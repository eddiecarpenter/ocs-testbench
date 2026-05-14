package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// LLMClient is the interface for sending chat completion requests to an
// OpenAI-compatible API. The agent uses only non-streaming calls so the
// full response (including tool_calls) is available atomically before
// the tool execution loop begins.
type LLMClient interface {
	// Complete sends a chat completion request and returns the full
	// response. The request must include the model and message list;
	// tools are optional. Returns a typed error for non-2xx responses.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}

// LLMError is returned by HTTPLLMClient.Complete when the LLM endpoint
// responds with a non-2xx HTTP status. The StatusCode and Body fields
// let callers inspect the upstream error without re-parsing.
type LLMError struct {
	StatusCode int
	Body       string
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("llm: HTTP %d: %s", e.StatusCode, e.Body)
}

// HTTPLLMClient implements LLMClient via HTTP calls to an
// OpenAI-compatible chat completions endpoint
// (POST {endpoint}/v1/chat/completions).
type HTTPLLMClient struct {
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

// NewHTTPLLMClient constructs a production LLMClient from the given
// AIConfig. Returns an error when cfg.Endpoint is empty — an empty
// endpoint is treated as disabled, not as a valid configuration.
func NewHTTPLLMClient(cfg baseconfig.AIConfig) (LLMClient, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("ai: LLM endpoint is not configured")
	}
	return &HTTPLLMClient{
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
func (c *HTTPLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	// Ensure the configured model is used when the caller has not set one.
	if req.Model == "" {
		req.Model = c.model
	}
	// Non-streaming calls so the full response including tool_calls is
	// available atomically.
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
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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
