package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// ── Anthropic wire types ──────────────────────────────────────────────────────

// anthropicCacheControl marks a content block as a prompt-cache breakpoint.
// Anthropic supports up to 4 breakpoints per request; we use two:
//   - the system prompt (stable across all turns)
//   - the last tool definition (stable as long as the tool list doesn't change)
//
// Both use type "ephemeral" — the only supported cache type at time of writing.
type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ephemeralCache is the singleton cache-control value used on every breakpoint.
var ephemeralCache = &anthropicCacheControl{Type: "ephemeral"}

// anthropicSystemBlock is the extended system-prompt content format.
// Using an array of blocks (instead of a plain string) is required to attach
// cache_control to the system prompt.
type anthropicSystemBlock struct {
	Type         string                 `json:"type"` // always "text"
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    any                `json:"system,omitempty"` // string or []anthropicSystemBlock
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	ID           string                 `json:"id,omitempty"`          // tool_use
	Name         string                 `json:"name,omitempty"`        // tool_use
	Input        json.RawMessage        `json:"input,omitempty"`       // tool_use
	ToolUseID    string                 `json:"tool_use_id,omitempty"` // tool_result
	Content      string                 `json:"content,omitempty"`     // tool_result
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]any         `json:"input_schema"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

// Non-streaming response.
type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
}

// Streaming SSE event shapes.
type anthropicEvent struct {
	Type         string                `json:"type"`
	Index        int                   `json:"index"`
	ContentBlock anthropicContent      `json:"content_block"`
	Delta        anthropicDelta        `json:"delta"`
	Message      anthropicEventMessage `json:"message"` // message_start
	Usage        anthropicUsage        `json:"usage"`   // message_delta
}

type anthropicDelta struct {
	Type        string `json:"type"` // text_delta | input_json_delta
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicMessage (in SSE events) carries start-of-message metadata.
type anthropicEventMessage struct {
	Usage anthropicUsage `json:"usage"`
}

// ── Client ────────────────────────────────────────────────────────────────────

// AnthropicLLMClient implements LLMClient against Anthropic's native
// /v1/messages API (not the OpenAI-compat layer). It translates between the
// OpenAI-shaped types used internally and Anthropic's JSON wire format.
// Required because Anthropic's OAuth tokens (sk-ant-oat*) are not accepted by
// the OpenAI-compat /v1/chat/completions endpoint.
type AnthropicLLMClient struct {
	endpoint string
	model    string
	apiKey   string
	http     *http.Client
}

func newAnthropicLLMClient(cfg baseconfig.AIConfig) *AnthropicLLMClient {
	return &AnthropicLLMClient{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		http:     &http.Client{},
	}
}

// ── Request translation ───────────────────────────────────────────────────────

// buildAnthropicRequest converts an OpenAI-format CompletionRequest to the
// Anthropic /v1/messages wire format. System messages are extracted from the
// messages array and placed in the top-level system field. Consecutive tool
// results are merged into a single user message. Tool call arguments (JSON
// strings) are parsed to JSON objects for the input field.
func buildAnthropicRequest(req CompletionRequest, model string, stream bool) (anthropicRequest, error) {
	ar := anthropicRequest{
		Model:     req.Model,
		MaxTokens: 8096,
		Stream:    stream,
	}
	if ar.Model == "" {
		ar.Model = model
	}

	// Convert tools.
	for _, t := range req.Tools {
		params := t.Function.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		ar.Tools = append(ar.Tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: params,
		})
	}
	// Mark the last tool as a cache breakpoint. The tool list is stable across
	// turns (same set of MCP tools), so this yields a high cache-hit rate and
	// avoids re-processing the schema definitions on every call.
	if len(ar.Tools) > 0 {
		ar.Tools[len(ar.Tools)-1].CacheControl = ephemeralCache
	}

	// Convert messages. Tool result messages (role:"tool") must be grouped into
	// user messages following the assistant turn that requested the tool calls.
	var msgs []anthropicMessage
	var pendingToolResults []anthropicContent

	flushToolResults := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		msgs = append(msgs, anthropicMessage{Role: "user", Content: pendingToolResults})
		pendingToolResults = nil
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			// Use the extended block format so we can attach cache_control.
			// The system prompt is completely stable — caching it avoids
			// re-processing the large instruction text on every turn.
			ar.System = []anthropicSystemBlock{{
				Type:         "text",
				Text:         m.Content,
				CacheControl: ephemeralCache,
			}}

		case "user":
			flushToolResults()
			msgs = append(msgs, anthropicMessage{
				Role:    "user",
				Content: []anthropicContent{{Type: "text", Text: m.Content}},
			})

		case "assistant":
			flushToolResults()
			var content []anthropicContent
			if m.Content != "" {
				content = append(content, anthropicContent{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				// Arguments is a JSON string; Anthropic wants a JSON object.
				raw := json.RawMessage(tc.Function.Arguments)
				if !json.Valid(raw) {
					raw = json.RawMessage("{}")
				}
				content = append(content, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: raw,
				})
			}
			if len(content) == 0 {
				content = []anthropicContent{{Type: "text", Text: ""}}
			}
			msgs = append(msgs, anthropicMessage{Role: "assistant", Content: content})

		case "tool":
			// Accumulate; will be flushed as a single user message.
			pendingToolResults = append(pendingToolResults, anthropicContent{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			})
		}
	}
	flushToolResults()
	ar.Messages = msgs
	return ar, nil
}

// ── Response translation ──────────────────────────────────────────────────────

// anthropicResponseToCompletion converts an Anthropic non-streaming response
// to our internal CompletionResponse.
func anthropicResponseToCompletion(ar anthropicResponse) *CompletionResponse {
	var textBuf strings.Builder
	var toolCalls []ToolCall

	for _, c := range ar.Content {
		switch c.Type {
		case "text":
			textBuf.WriteString(c.Text)
		case "tool_use":
			args := "{}"
			if len(c.Input) > 0 {
				args = string(c.Input)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   c.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      c.Name,
					Arguments: args,
				},
			})
		}
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &CompletionResponse{
		Choices: []Choice{{
			Message: Message{
				Role:      "assistant",
				Content:   textBuf.String(),
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}},
	}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (c *AnthropicLLMClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.apiKey)
	return c.http.Do(req)
}

// ── LLMClient implementation ──────────────────────────────────────────────────

func (c *AnthropicLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	ar, err := buildAnthropicRequest(req, c.model, false)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return nil, &LLMError{StatusCode: resp.StatusCode, Body: buf.String()}
	}

	var ar2 anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar2); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}
	return anthropicResponseToCompletion(ar2), nil
}

func (c *AnthropicLLMClient) CompleteStream(ctx context.Context, req CompletionRequest, onToken func(string), onUsage func(Usage)) (*CompletionResponse, error) {
	ar, err := buildAnthropicRequest(req, c.model, true)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal stream request: %w", err)
	}

	slog.Info("llm: sending request",
		"model", ar.Model,
		"messages", len(ar.Messages),
		"tools", len(ar.Tools),
		"stream", true,
	)

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		slog.Error("llm: request failed", "err", err)
		return nil, fmt.Errorf("anthropic: send stream request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		slog.Error("llm: non-2xx response", "status", resp.StatusCode, "body", buf.String())
		return nil, &LLMError{StatusCode: resp.StatusCode, Body: buf.String()}
	}

	// Accumulate state across SSE events.
	var textBuf strings.Builder
	var usage Usage
	// content_block index → in-progress tool call
	toolBlocks := map[int]*ToolCall{}
	toolArgsBuf := map[int]*strings.Builder{}
	finishReason := "stop"

	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	for scanner.Scan() {
		line := scanner.Text()

		// Anthropic SSE uses "event:" and "data:" lines.
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")

		var ev anthropicEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}

		switch eventType {
		case "message_start":
			// Input token count arrives at the very start of the stream.
			usage.InputTokens = ev.Message.Usage.InputTokens
			if onUsage != nil && usage.InputTokens > 0 {
				onUsage(Usage{InputTokens: usage.InputTokens})
			}

		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				toolBlocks[ev.Index] = &ToolCall{
					ID:   ev.ContentBlock.ID,
					Type: "function",
					Function: FunctionCall{
						Name: ev.ContentBlock.Name,
					},
				}
				toolArgsBuf[ev.Index] = &strings.Builder{}
			}

		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				textBuf.WriteString(ev.Delta.Text)
				onToken(ev.Delta.Text)
			case "input_json_delta":
				if buf, ok := toolArgsBuf[ev.Index]; ok {
					buf.WriteString(ev.Delta.PartialJSON)
				}
			}

		case "message_delta":
			// Output token count arrives here.
			if ev.Usage.OutputTokens > 0 {
				usage.OutputTokens = ev.Usage.OutputTokens
			}
			if ev.Delta.StopReason != "" {
				switch ev.Delta.StopReason {
				case "tool_use":
					finishReason = "tool_calls"
				case "end_turn":
					finishReason = "stop"
				default:
					finishReason = ev.Delta.StopReason
				}
			}

		case "message_stop":
			// Stream complete.
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		slog.Error("llm: stream read error", "err", err)
		return nil, fmt.Errorf("anthropic: read stream: %w", err)
	}

	slog.Info("llm: response received",
		"model", ar.Model,
		"finish_reason", finishReason,
		"input_tokens", usage.InputTokens,
		"output_tokens", usage.OutputTokens,
	)

	// Assemble tool calls in ascending index order. Collect only entries with
	// a non-empty ID — Anthropic content-block indices include text blocks
	// (index 0) so the toolBlocks map may have gaps that should be skipped.
	indices := make([]int, 0, len(toolBlocks))
	for idx := range toolBlocks {
		indices = append(indices, idx)
	}
	// Sort ascending so tool calls appear in the order the model declared them.
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
	var toolCalls []ToolCall
	for _, idx := range indices {
		tc := toolBlocks[idx]
		if tc.ID == "" {
			continue // skip zero-value / non-tool-use content blocks
		}
		args := "{}"
		if buf, ok := toolArgsBuf[idx]; ok && buf.Len() > 0 {
			args = buf.String()
		}
		tc.Function.Arguments = args
		toolCalls = append(toolCalls, *tc)
	}

	return &CompletionResponse{
		Choices: []Choice{{
			Message: Message{
				Role:      "assistant",
				Content:   textBuf.String(),
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}},
		Usage: usage,
	}, nil
}
