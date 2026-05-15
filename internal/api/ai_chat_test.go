package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// readSSEEvents reads lines from a bufio.Scanner and returns the first N
// complete SSE data payloads (lines starting with "data: ").
func readSSEEvents(t *testing.T, scanner *bufio.Scanner, count int, timeout time.Duration) []map[string]any {
	t.Helper()
	deadline := time.After(timeout)
	var events []map[string]any
	var dataLine string
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE events (got %d of %d)", len(events), count)
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
		}
		if line == "" && dataLine != "" {
			var payload map[string]any
			if err := json.Unmarshal([]byte(dataLine), &payload); err == nil {
				events = append(events, payload)
				dataLine = ""
				if len(events) >= count {
					return events
				}
			}
		}
	}
	return events
}

// newLLMServer creates a minimal httptest.Server that responds to
// POST /v1/chat/completions with the given scripted JSON responses in order.
// Each call to the server returns the next response in the sequence.
// When the request body includes "stream":true the response is returned as
// SSE (text/event-stream) so CompleteStream can parse it.
func newLLMServer(t *testing.T, responses []map[string]any) *httptest.Server {
	t.Helper()
	idx := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}

		// Detect streaming request.
		var reqBody struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		if !reqBody.Stream {
			w.Header().Set("Content-Type", "application/json")
			if idx >= len(responses) {
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{}}) //nolint:errcheck
				return
			}
			json.NewEncoder(w).Encode(responses[idx]) //nolint:errcheck
			idx++
			return
		}

		// SSE streaming response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		if idx >= len(responses) {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n") //nolint:errcheck
			fmt.Fprintf(w, "data: [DONE]\n\n")                                                    //nolint:errcheck
			return
		}
		resp := responses[idx]
		idx++

		choices, _ := resp["choices"].([]map[string]any)
		if len(choices) == 0 {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n") //nolint:errcheck
			fmt.Fprintf(w, "data: [DONE]\n\n")                                                    //nolint:errcheck
			return
		}
		choice := choices[0]
		finishReason, _ := choice["finish_reason"].(string)

		if msg, ok := choice["message"].(map[string]any); ok {
			if content, ok := msg["content"].(string); ok && content != "" {
				// Emit content as a single delta chunk.
				chunk := map[string]any{
					"choices": []map[string]any{{
						"delta":         map[string]any{"content": content},
						"finish_reason": "",
					}},
				}
				b, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
			}
			if toolCalls, ok := msg["tool_calls"].([]map[string]any); ok {
				// Emit each tool call as a delta.
				for i, tc := range toolCalls {
					fn, _ := tc["function"].(map[string]any)
					fnName, _ := fn["name"].(string)
					fnArgs, _ := fn["arguments"].(string)
					chunk := map[string]any{
						"choices": []map[string]any{{
							"delta": map[string]any{
								"tool_calls": []map[string]any{{
									"index": i,
									"id":    tc["id"],
									"type":  "function",
									"function": map[string]any{
										"name":      fnName,
										"arguments": fnArgs,
									},
								}},
							},
							"finish_reason": "",
						}},
					}
					b, _ := json.Marshal(chunk)
					fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
				}
			}
		}

		// Final chunk with finish_reason.
		finalChunk := map[string]any{
			"choices": []map[string]any{{
				"delta":         map[string]any{},
				"finish_reason": finishReason,
			}},
		}
		b, _ := json.Marshal(finalChunk)
		fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
		fmt.Fprintf(w, "data: [DONE]\n\n") //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

// llmTextResponse builds an OpenAI-shaped response with a plain text message.
func llmTextResponse(content string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
	}
}

// llmToolCallResponse builds an OpenAI-shaped response with one tool call.
func llmToolCallResponse(callID, name, argsJSON string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{{
			"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id":   callID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": argsJSON,
					},
				}},
			},
			"finish_reason": "tool_calls",
		}},
	}
}

// buildChatRequest returns a POST /v1/ai/chat http.Request with the given
// message content and optional X-Session-ID header.
func buildChatRequest(t *testing.T, sessionID, content string) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": content}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	return req
}

// newChatRouter builds a Router with a real Agent backed by a mock LLM
// server and a minimal MCP server (empty tool list). Used for tests that
// exercise the SSE streaming path without real Diameter tools.
func newChatRouter(t *testing.T, llmResponses []map[string]any) (http.Handler, *ai.SessionManager) {
	t.Helper()

	llmSrv := newLLMServer(t, llmResponses)

	// Spin up a minimal MCP server with no tools. The agent calls ListTools
	// first; an empty list is valid and causes the LLM to respond without
	// tool calls (as scripted by llmResponses).
	_, mcpURL := newEmptyMCPServer(t)

	llmCfg := ai.MakeAIConfig(llmSrv.URL)
	agent := ai.NewAgent(llmCfg, mcpURL)
	require.NotNil(t, agent)

	sessions := ai.NewSessionManager(nil)
	t.Cleanup(sessions.Stop)

	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, agent, sessions, "test")
	return r, sessions
}

// TestAIChat_NilAgent_Returns503 verifies that POST /v1/ai/chat returns 503
// when the Router is built without an AI agent (feature not configured).
func TestAIChat_NilAgent_Returns503(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, nil, nil, "test")

	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat",
		bytes.NewBufferString(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// TestAIChat_ContentTypeTextEventStream verifies that POST /v1/ai/chat
// returns text/event-stream when the agent is configured.
func TestAIChat_ContentTypeTextEventStream(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{llmTextResponse("hello")})

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
}

// TestAIChat_SessionIDGeneratedWhenAbsent verifies that the response includes
// an X-Session-ID header when the request did not supply one.
func TestAIChat_SessionIDGeneratedWhenAbsent(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{llmTextResponse("hi")})

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEmpty(t, resp.Header.Get("X-Session-ID"),
		"X-Session-ID must be present in response even when not supplied in request")
}

// TestAIChat_SessionIDEchoedWhenSupplied verifies that the response echoes
// back the same X-Session-ID the client sent.
func TestAIChat_SessionIDEchoedWhenSupplied(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{llmTextResponse("ok")})

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"ok"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", "my-fixed-session")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "my-fixed-session", resp.Header.Get("X-Session-ID"))
}

// TestAIChat_EmitsTokenEvents verifies that a plain-text LLM response emits
// token events and a done event.
func TestAIChat_EmitsTokenEvents(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{
		llmTextResponse("The OCS is responding correctly."),
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"check OCS"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 10, 8*time.Second)

	var tokenCount int
	var foundDone bool
	for _, e := range events {
		switch e["type"] {
		case "token":
			tokenCount++
		case "done":
			foundDone = true
		}
	}
	assert.Greater(t, tokenCount, 0, "expected at least one token event")
	assert.True(t, foundDone, "expected done event at end of stream")
}

// TestAIChat_EmitsDoneEvent verifies that the stream terminates with a
// "done" event when the LLM produces a final text response.
func TestAIChat_EmitsDoneEvent(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{
		llmTextResponse("Done."),
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"go"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 20, 10*time.Second)

	var foundDone bool
	for _, e := range events {
		if e["type"] == "done" {
			foundDone = true
			break
		}
	}
	assert.True(t, foundDone, "expected done event at end of stream")
}

// TestAIChat_LLMError_EmitsErrorEvent verifies that an LLM HTTP 500 emits an
// "error" SSE event and does not hang.
func TestAIChat_LLMError_EmitsErrorEvent(t *testing.T) {
	// LLM server returns 500.
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	t.Cleanup(errSrv.Close)

	llmCfg := ai.MakeAIConfig(errSrv.URL)
	agent := ai.NewAgent(llmCfg, errSrv.URL)
	require.NotNil(t, agent)

	sessions := ai.NewSessionManager(nil)
	t.Cleanup(sessions.Stop)

	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, agent, sessions, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"test"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 5, 6*time.Second)

	var foundError bool
	for _, e := range events {
		if e["type"] == "error" {
			foundError = true
			break
		}
	}
	assert.True(t, foundError, "expected error SSE event when LLM returns 500")
}

// TestAIChat_ClientDisconnect verifies that the handler exits cleanly when
// the client disconnects mid-stream (context cancelled before response completes).
func TestAIChat_ClientDisconnect(t *testing.T) {
	r, _ := newChatRouter(t, []map[string]any{llmTextResponse("long response")})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled immediately

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not exit after client disconnect")
	}
}

// TestAIChat_EndToEnd_OneToolCall exercises the full request → agentic loop
// → SSE response path via a real httptest MCP server. The mock LLM first
// returns a tool_call (list_peers, readonly) then a text response.
// Expects: tool_call SSE event followed by token events and done.
func TestAIChat_EndToEnd_OneToolCall(t *testing.T) {
	// Spin up a minimal httptest server that serves the LLM responses.
	llmSrv := newLLMServer(t, []map[string]any{
		// First LLM call: request list_peers tool
		llmToolCallResponse("tc-1", "list_peers", `{}`),
		// Second LLM call: final text response
		llmTextResponse("Peers retrieved successfully."),
	})

	// Spin up a real MCP server with a list_peers tool.
	_, mcpURL := newEndToEndMCPServer(t)

	llmCfg := ai.MakeAIConfig(llmSrv.URL)
	agent := ai.NewAgent(llmCfg, mcpURL)
	require.NotNil(t, agent)

	sessions := ai.NewSessionManager(nil)
	t.Cleanup(sessions.Stop)

	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, agent, sessions, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"list peers"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 20, 12*time.Second)

	var foundToolCall, foundToken, foundDone bool
	for _, e := range events {
		switch e["type"] {
		case "tool_call":
			foundToolCall = true
			data, ok := e["data"].(map[string]any)
			require.True(t, ok, "tool_call data must be an object")
			assert.Equal(t, "list_peers", data["name"])
		case "token":
			foundToken = true
		case "done":
			foundDone = true
		}
	}
	assert.True(t, foundToolCall, "expected tool_call SSE event")
	assert.True(t, foundToken, "expected at least one token event")
	assert.True(t, foundDone, "expected done event")
}

// TestAIChat_EndToEnd_UnexecutableTool verifies that when the MCP server
// returns an error for a tool call, the SSE stream contains a tool_call
// event with an error result and the loop continues to done.
func TestAIChat_EndToEnd_UnexecutableTool(t *testing.T) {
	llmSrv := newLLMServer(t, []map[string]any{
		llmToolCallResponse("tc-err", "list_peers", `{}`),
		llmTextResponse("Encountered error but continuing."),
	})

	// MCP server that always returns an error for tool calls.
	mcpSrv, mcpURL := newEndToEndMCPServerWithError(t)
	_ = mcpSrv

	llmCfg := ai.MakeAIConfig(llmSrv.URL)
	agent := ai.NewAgent(llmCfg, mcpURL)
	require.NotNil(t, agent)

	sessions := ai.NewSessionManager(nil)
	t.Cleanup(sessions.Stop)

	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, agent, sessions, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"test error"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 20, 12*time.Second)

	var foundToolCallWithError, foundDone bool
	for _, e := range events {
		switch e["type"] {
		case "tool_call":
			data, ok := e["data"].(map[string]any)
			require.True(t, ok)
			resultBytes, _ := json.Marshal(data["result"])
			if strings.Contains(string(resultBytes), "error") {
				foundToolCallWithError = true
			}
		case "done":
			foundDone = true
		}
	}
	assert.True(t, foundToolCallWithError, "expected tool_call SSE event with error result")
	assert.True(t, foundDone, "expected done event after tool error")
}
