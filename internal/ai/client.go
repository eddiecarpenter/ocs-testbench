package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// xmlToolCallRE matches <tool>…</tool> and <tool_call>…</tool_call> blocks
// that some models (e.g. Qwen3) emit in the content stream instead of using
// the OpenAI delta.tool_calls field.
var xmlToolCallRE = regexp.MustCompile(`(?s)<tool(?:_call)?>\s*(\{.*?})\s*</tool(?:_call)?>`)

// parseXMLToolCalls extracts tool calls embedded as XML in content.
// Returns nil when none are found.
func parseXMLToolCalls(content string) []ToolCall {
	matches := xmlToolCallRE.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	calls := make([]ToolCall, 0, len(matches))
	for i, m := range matches {
		var obj struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(m[1]), &obj); err != nil || obj.Name == "" {
			continue
		}
		args := string(obj.Arguments)
		if args == "" || args == "null" {
			args = "{}"
		}
		calls = append(calls, ToolCall{
			ID:   fmt.Sprintf("call_xml_%d", i),
			Type: "function",
			Function: FunctionCall{
				Name:      obj.Name,
				Arguments: args,
			},
		})
	}
	return calls
}

// setAuthHeaders applies the appropriate authentication headers for the given
// endpoint.
//   - Anthropic API keys (sk-ant-api*): x-api-key header
//   - Anthropic OAuth tokens (sk-ant-oat*): Authorization: Bearer header
//   - All other providers: Authorization: Bearer header
//
// Both Anthropic token types also require the anthropic-version header.
func SetAuthHeaders(req *http.Request, endpoint, apiKey string) {
	if apiKey == "" {
		return
	}
	if strings.Contains(endpoint, "anthropic.com") {
		req.Header.Set("anthropic-version", "2023-06-01")
		if strings.HasPrefix(apiKey, "sk-ant-oat") {
			// Claude Code OAuth access token — uses Bearer scheme.
			req.Header.Set("Authorization", "Bearer "+apiKey)
		} else {
			// Standard Anthropic API key — uses x-api-key scheme.
			req.Header.Set("x-api-key", apiKey)
		}
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

// LLMClient is the interface for sending chat completion requests to an
// OpenAI-compatible API.
type LLMClient interface {
	// Complete sends a non-streaming chat completion request and returns
	// the full response atomically. Used for tool-call turns where the
	// complete tool_calls array must be available before execution begins.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// CompleteStream sends a streaming chat completion request. Each
	// content delta is passed to onToken as it arrives so the caller can
	// relay it to the browser immediately. onUsage is called as soon as
	// token-usage figures become available mid-stream (for Anthropic this
	// is right at the start; for OpenAI it is the final chunk). The method
	// blocks until the stream ends and returns the fully-assembled
	// CompletionResponse for history tracking.
	// onToken is never called for tool-call turns (content is empty).
	CompleteStream(ctx context.Context, req CompletionRequest, onToken func(string), onUsage func(Usage)) (*CompletionResponse, error)
}

// LLMError is returned by OpenAILLMClient.Complete when the LLM endpoint
// responds with a non-2xx HTTP status. The StatusCode and Body fields
// let callers inspect the upstream error without re-parsing.
type LLMError struct {
	StatusCode int
	Body       string
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("llm: HTTP %d: %s", e.StatusCode, e.Body)
}

// OpenAILLMClient implements LLMClient via HTTP calls to an
// OpenAI-compatible chat completions endpoint
// (POST {endpoint}/v1/chat/completions).
type OpenAILLMClient struct {
	endpoint string
	model    string
	apiKey   string
	http     *http.Client
}

// MakeAIConfig builds an AIConfig pointing at the given endpoint with a
// default model. Intended for tests that spin up a local HTTP server to
// simulate an OpenAI-compatible LLM endpoint.
func MakeAIConfig(endpoint string) baseconfig.AIConfig {
	return baseconfig.AIConfig{Endpoint: endpoint, Model: "gpt-4o"}
}

// MakeAIConfigWithKey builds an AIConfig with an endpoint and API key.
// Intended for tests that need to verify key masking behaviour.
func MakeAIConfigWithKey(endpoint, apiKey string) baseconfig.AIConfig {
	return baseconfig.AIConfig{Endpoint: endpoint, Model: "gpt-4o", APIKey: apiKey}
}

// NewLLMClient constructs a production LLMClient from the given AIConfig.
// For Anthropic endpoints (api.anthropic.com) it returns an AnthropicLLMClient
// that speaks the native /v1/messages API, which accepts both API keys and
// OAuth Bearer tokens (sk-ant-oat*) from Claude Code. All other endpoints use
// the OpenAI-compatible /v1/chat/completions client.
// Returns an error when cfg.Endpoint is empty.
func NewLLMClient(cfg baseconfig.AIConfig) (LLMClient, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("ai: LLM endpoint is not configured")
	}
	if strings.Contains(cfg.Endpoint, "anthropic.com") {
		return newAnthropicLLMClient(cfg), nil
	}
	return &OpenAILLMClient{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		http:     &http.Client{},
	}, nil
}

// Complete implements LLMClient. It serialises the request to JSON,
// POSTs it to {endpoint}/v1/chat/completions, and deserialises the
// CompletionResponse. Non-2xx responses are returned as *LLMError so
// callers can inspect the status code and body.
func (c *OpenAILLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	SetAuthHeaders(httpReq, c.endpoint, c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return nil, &LLMError{StatusCode: resp.StatusCode, Body: buf.String()}
	}

	var completion CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("ai: decode response: %w", err)
	}
	return &completion, nil
}

// CompleteStream implements LLMClient using stream: true.
//
// It opens the SSE stream from the LLM, calls onToken for every content
// delta (so the caller can relay tokens to the browser immediately), and
// assembles the full CompletionResponse from the accumulated deltas before
// returning. Tool-call turns produce no content deltas so onToken is never
// called for them.
func (c *OpenAILLMClient) CompleteStream(ctx context.Context, req CompletionRequest, onToken func(string), onUsage func(Usage)) (*CompletionResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	req.Stream = true
	req.StreamOptions = &streamOptions{IncludeUsage: true}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: build stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	SetAuthHeaders(httpReq, c.endpoint, c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai: send stream request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return nil, &LLMError{StatusCode: resp.StatusCode, Body: buf.String()}
	}

	// Accumulate content, tool_call deltas, and usage as they arrive.
	var contentBuf strings.Builder
	var usage Usage
	// tool call assembly: map from delta index → in-progress ToolCall
	toolCallMap := map[int]*ToolCall{}
	finishReason := "stop"

	// sniffBuf holds the first sniffLen bytes of content so we can decide
	// whether this is a plain-text response (emit tokens immediately) or a
	// tool-call response encoded as XML in the content (buffer and don't emit).
	// Some models (e.g. Qwen3) use <tool>…</tool> in content instead of
	// delta.tool_calls.
	const sniffLen = 64
	type sniffState int
	const (
		sniffPending  sniffState = iota // still collecting prefix
		sniffText                       // confirmed text — stream to onToken
		sniffToolCall                   // confirmed XML tool call — buffer only
	)
	sstate := sniffPending
	var sniffBuf strings.Builder

	emitContent := func(delta string) {
		contentBuf.WriteString(delta)
		switch sstate {
		case sniffPending:
			sniffBuf.WriteString(delta)
			if sniffBuf.Len() >= sniffLen {
				if strings.HasPrefix(strings.TrimSpace(sniffBuf.String()), "<tool") {
					sstate = sniffToolCall
				} else {
					sstate = sniffText
					onToken(sniffBuf.String()) // flush buffered prefix
				}
			}
		case sniffText:
			onToken(delta)
		case sniffToolCall:
			// buffer only — don't emit XML tool-call markup as tokens
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip blank lines and non-data lines.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // skip malformed chunks
		}
		// Usage-only chunk (sent after [DONE] by some providers when
		// stream_options.include_usage=true; others send it before [DONE]).
		if len(chunk.Choices) == 0 {
			if chunk.Usage != nil {
				usage = Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				}
				if onUsage != nil {
					onUsage(usage)
				}
			}
			continue
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}

		// Content delta — sniff then relay or buffer.
		if choice.Delta.Content != "" {
			emitContent(choice.Delta.Content)
		}

		// Tool-call deltas — assemble by index.
		for _, tcd := range choice.Delta.ToolCalls {
			tc, ok := toolCallMap[tcd.Index]
			if !ok {
				tc = &ToolCall{Type: "function"}
				toolCallMap[tcd.Index] = tc
			}
			if tcd.ID != "" {
				tc.ID = tcd.ID
			}
			if tcd.Function.Name != "" {
				tc.Function.Name = tcd.Function.Name
			}
			tc.Function.Arguments += tcd.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return nil, fmt.Errorf("ai: read stream: %w", err)
	}

	// Flush sniff buffer for short responses that never reached sniffLen.
	if sstate == sniffPending && sniffBuf.Len() > 0 {
		if strings.HasPrefix(strings.TrimSpace(sniffBuf.String()), "<tool") {
			sstate = sniffToolCall
		} else {
			onToken(sniffBuf.String())
		}
	}

	// Fallback: if the model emitted tool calls as XML content instead of
	// delta.tool_calls, parse them now and clear the content buffer.
	if len(toolCallMap) == 0 && sstate == sniffToolCall {
		if xmlCalls := parseXMLToolCalls(contentBuf.String()); len(xmlCalls) > 0 {
			for i := range xmlCalls {
				idx := i
				toolCallMap[idx] = &xmlCalls[idx]
			}
			contentBuf.Reset()
			finishReason = "tool_calls"
		}
	}

	// Build the assembled tool-call slice in ascending index order, skipping
	// any entries with an empty ID (gaps from malformed or missing deltas).
	indices := make([]int, 0, len(toolCallMap))
	for idx := range toolCallMap {
		indices = append(indices, idx)
	}
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	var toolCalls []ToolCall
	for _, idx := range indices {
		tc := toolCallMap[idx]
		if tc.ID != "" {
			toolCalls = append(toolCalls, *tc)
		}
	}

	return &CompletionResponse{
		Choices: []Choice{{
			Message: Message{
				Role:      "assistant",
				Content:   contentBuf.String(),
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}},
		Usage: usage,
	}, nil
}
