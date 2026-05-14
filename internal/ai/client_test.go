package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// buildLLMResponse returns a minimal OpenAI-format completion response JSON.
func buildLLMResponse(content string, toolCalls []ai.ToolCall) []byte {
	msg := ai.Message{Role: "assistant", Content: content}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
		msg.Content = ""
	}
	resp := ai.CompletionResponse{
		Choices: []ai.Choice{
			{Message: msg, FinishReason: "stop"},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// TestNewHTTPLLMClient_EmptyEndpoint verifies that an empty endpoint returns
// an error rather than a client.
func TestNewHTTPLLMClient_EmptyEndpoint(t *testing.T) {
	_, err := ai.NewHTTPLLMClient(baseconfig.AIConfig{Endpoint: ""})
	assert.Error(t, err, "empty endpoint must return an error")
}

// TestHTTPLLMClient_Complete_Success verifies a happy-path completion call.
func TestHTTPLLMClient_Complete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		// Decode the request to verify model is passed correctly.
		var req ai.CompletionRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "gpt-4o", req.Model)
		assert.False(t, req.Stream, "Stream must be false for non-streaming calls")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildLLMResponse("Hello!", nil))
	}))
	defer srv.Close()

	client, err := ai.NewHTTPLLMClient(baseconfig.AIConfig{
		Endpoint: srv.URL,
		Model:    "gpt-4o",
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello!", resp.Choices[0].Message.Content)
}

// TestHTTPLLMClient_Complete_NonTwoXX verifies that a non-2xx response
// returns an *LLMError with the status code and body.
func TestHTTPLLMClient_Complete_NonTwoXX(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"server error", http.StatusInternalServerError},
		{"rate limit", http.StatusTooManyRequests},
		{"unauthorized", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":"test error"}`, tc.statusCode)
			}))
			defer srv.Close()

			client, err := ai.NewHTTPLLMClient(baseconfig.AIConfig{Endpoint: srv.URL, Model: "gpt-4o"})
			require.NoError(t, err)

			_, err = client.Complete(context.Background(), ai.CompletionRequest{
				Messages: []ai.Message{{Role: "user", Content: "hi"}},
			})
			require.Error(t, err)

			var llmErr *ai.LLMError
			require.ErrorAs(t, err, &llmErr, "error must be *LLMError")
			assert.Equal(t, tc.statusCode, llmErr.StatusCode)
			assert.NotEmpty(t, llmErr.Body)
		})
	}
}

// TestHTTPLLMClient_Complete_NetworkTimeout verifies that a request timeout
// returns an error (not a successful response).
func TestHTTPLLMClient_Complete_NetworkTimeout(t *testing.T) {
	// Server that sleeps for longer than the client's deadline.
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := ai.NewHTTPLLMClient(baseconfig.AIConfig{Endpoint: srv.URL, Model: "gpt-4o"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for the server to start handling the request before the timeout fires.
	go func() { <-started }()

	_, err = client.Complete(ctx, ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err, "timed-out request must return an error")
}

// TestHTTPLLMClient_Complete_ToolCalls verifies that tool_calls in the
// response are correctly parsed.
func TestHTTPLLMClient_Complete_ToolCalls(t *testing.T) {
	expectedToolCalls := []ai.ToolCall{
		{
			ID:   "call-1",
			Type: "function",
			Function: ai.FunctionCall{
				Name:      "list_peers",
				Arguments: `{}`,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildLLMResponse("", expectedToolCalls))
	}))
	defer srv.Close()

	client, err := ai.NewHTTPLLMClient(baseconfig.AIConfig{Endpoint: srv.URL, Model: "gpt-4o"})
	require.NoError(t, err)

	resp, err := client.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: "user", Content: "list my peers"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	require.Len(t, resp.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "call-1", resp.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "list_peers", resp.Choices[0].Message.ToolCalls[0].Function.Name)
}
