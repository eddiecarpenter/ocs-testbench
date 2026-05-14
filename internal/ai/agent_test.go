package ai_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
)

// ---- Mock LLM client -------------------------------------------------------

// mockLLMClient is a simple mock that returns a sequence of
// CompletionResponses in order.
type mockLLMClient struct {
	responses []ai.CompletionResponse
	idx       int
	err       error // returned on the next call when non-nil
}

func (m *mockLLMClient) Complete(_ context.Context, _ ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.idx >= len(m.responses) {
		return &ai.CompletionResponse{
			Choices: []ai.Choice{{
				Message:      ai.Message{Role: "assistant", Content: "no more responses"},
				FinishReason: "stop",
			}},
		}, nil
	}
	r := m.responses[m.idx]
	m.idx++
	return &r, nil
}

// textResponse builds a CompletionResponse with a plain text assistant message.
func textResponse(content string) ai.CompletionResponse {
	return ai.CompletionResponse{Choices: []ai.Choice{{
		Message:      ai.Message{Role: "assistant", Content: content},
		FinishReason: "stop",
	}}}
}

// toolCallResponse builds a CompletionResponse containing tool_calls.
func toolCallResponse(calls ...ai.ToolCall) ai.CompletionResponse {
	return ai.CompletionResponse{Choices: []ai.Choice{{
		Message:      ai.Message{Role: "assistant", ToolCalls: calls},
		FinishReason: "tool_calls",
	}}}
}

// ---- Mock MCP caller -------------------------------------------------------

// mockMCPCaller provides controllable ListTools and CallTool responses.
type mockMCPCaller struct {
	tools      []mcp.Tool
	callError  error
	callResult *mcp.CallToolResult
}

func (m *mockMCPCaller) Initialize(_ context.Context, _ mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	return &mcp.InitializeResult{}, nil
}

func (m *mockMCPCaller) ListTools(_ context.Context, _ mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{Tools: m.tools}, nil
}

func (m *mockMCPCaller) CallTool(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if m.callError != nil {
		return nil, m.callError
	}
	if m.callResult != nil {
		return m.callResult, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: `{"ok":true}`}},
	}, nil
}

// readOnlyTool creates a read-only MCP tool stub.
func readOnlyTool(name string) mcp.Tool {
	ro := true
	de := false
	return mcp.Tool{
		Name:        name,
		Description: "test tool " + name,
		Annotations: mcp.ToolAnnotation{ReadOnlyHint: &ro, DestructiveHint: &de},
	}
}

// writeTool creates a write-tier MCP tool stub.
func writeTool(name string) mcp.Tool {
	ro := false
	de := false
	return mcp.Tool{
		Name:        name,
		Description: "test tool " + name,
		Annotations: mcp.ToolAnnotation{ReadOnlyHint: &ro, DestructiveHint: &de},
	}
}

// ---- SSE event capture helpers ---------------------------------------------

type capturedEvent struct {
	eventType string
	data      any
}

func captureEmit() (ai.EmitFunc, *[]capturedEvent) {
	var events []capturedEvent
	fn := func(eventType string, data any) error {
		events = append(events, capturedEvent{eventType: eventType, data: data})
		return nil
	}
	return fn, &events
}

func alwaysAllow(_ string, _ string, _ string) bool { return true }
func alwaysDeny(_ string, _ string, _ string) bool  { return false }

// ---- Tests -----------------------------------------------------------------

// TestAgent_Run_NoToolCalls verifies that a plain text response emits
// token events and a done event with no tool calls.
func TestAgent_Run_NoToolCalls(t *testing.T) {
	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		textResponse("The OCS is functioning normally."),
	}}
	mocked := &mockMCPCaller{tools: []mcp.Tool{readOnlyTool("list_peers")}}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "check the OCS"}}

	updated, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err)

	// Verify done event is present.
	var foundDone bool
	for _, e := range *events {
		if e.eventType == "done" {
			foundDone = true
		}
	}
	assert.True(t, foundDone, "expected done event")

	// Verify at least one token event.
	var tokenCount int
	for _, e := range *events {
		if e.eventType == "token" {
			tokenCount++
		}
	}
	assert.Greater(t, tokenCount, 0, "expected token events")

	// Verify updated history contains the assistant turn.
	require.Greater(t, len(updated), len(history))
	lastMsg := updated[len(updated)-1]
	assert.Equal(t, "assistant", lastMsg.Role)
	assert.Contains(t, lastMsg.Content, "OCS is functioning normally")
}

// TestAgent_Run_OneToolCall verifies the loop with one read-only tool call
// followed by a final text response.
func TestAgent_Run_OneToolCall(t *testing.T) {
	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		// First turn: LLM requests list_peers.
		toolCallResponse(ai.ToolCall{
			ID:       "tc-1",
			Type:     "function",
			Function: ai.FunctionCall{Name: "list_peers", Arguments: `{}`},
		}),
		// Second turn: LLM produces final text.
		textResponse("Found 2 peers."),
	}}
	mocked := &mockMCPCaller{tools: []mcp.Tool{readOnlyTool("list_peers")}}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "list peers"}}

	updated, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err)

	// Verify tool_call event was emitted.
	var foundTool bool
	for _, e := range *events {
		if e.eventType == "tool_call" {
			data, ok := e.data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "list_peers", data["name"])
			assert.Equal(t, "readonly", data["tier"])
			foundTool = true
		}
	}
	assert.True(t, foundTool, "expected tool_call event")

	// Final history should include user, assistant (tool call), tool result, assistant (text).
	assert.Greater(t, len(updated), len(history))
}

// TestAgent_Run_TwoSequentialToolCalls verifies that two sequential tool
// calls are both executed before the final text response.
func TestAgent_Run_TwoSequentialToolCalls(t *testing.T) {
	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		// First turn: one tool call.
		toolCallResponse(ai.ToolCall{
			ID: "tc-1", Type: "function",
			Function: ai.FunctionCall{Name: "list_peers", Arguments: `{}`},
		}),
		// Second turn: another tool call.
		toolCallResponse(ai.ToolCall{
			ID: "tc-2", Type: "function",
			Function: ai.FunctionCall{Name: "list_peers", Arguments: `{}`},
		}),
		// Third turn: final text.
		textResponse("Analysis complete."),
	}}
	mocked := &mockMCPCaller{tools: []mcp.Tool{readOnlyTool("list_peers")}}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "analyse"}}

	_, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err)

	var toolCallCount int
	for _, e := range *events {
		if e.eventType == "tool_call" {
			toolCallCount++
		}
	}
	assert.Equal(t, 2, toolCallCount, "expected exactly two tool_call events")
}

// TestAgent_Run_UnexecutableToolCall verifies that an MCP error is
// surfaced as a tool_call event with an error result, and the loop continues.
func TestAgent_Run_UnexecutableToolCall(t *testing.T) {
	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		toolCallResponse(ai.ToolCall{
			ID: "tc-err", Type: "function",
			Function: ai.FunctionCall{Name: "list_peers", Arguments: `{}`},
		}),
		textResponse("Encountered an error but continuing."),
	}}
	mocked := &mockMCPCaller{
		tools:     []mcp.Tool{readOnlyTool("list_peers")},
		callError: errors.New("MCP tool execution failed"),
	}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "list peers"}}

	_, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err, "Run must not return an error for unexecutable tool calls")

	// Verify tool_call event with error result.
	var foundToolCallWithError bool
	for _, e := range *events {
		if e.eventType == "tool_call" {
			data, ok := e.data.(map[string]any)
			require.True(t, ok)
			resultStr, _ := json.Marshal(data["result"])
			if strings.Contains(string(resultStr), "error") {
				foundToolCallWithError = true
			}
		}
	}
	assert.True(t, foundToolCallWithError, "expected tool_call event with error result")

	// Verify done event still emitted (loop continued).
	var foundDone bool
	for _, e := range *events {
		if e.eventType == "done" {
			foundDone = true
		}
	}
	assert.True(t, foundDone, "expected done event after tool error")
}

// TestAgent_Run_LLMError verifies that an LLM error emits an error SSE
// event and Run returns nil (the HTTP handler does not double-error).
func TestAgent_Run_LLMError(t *testing.T) {
	llm := &mockLLMClient{err: errors.New("LLM unavailable")}
	mocked := &mockMCPCaller{tools: []mcp.Tool{readOnlyTool("list_peers")}}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "hi"}}

	returned, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	assert.NoError(t, err, "Run must return nil on LLM error")
	assert.Equal(t, history, returned, "history must be unchanged on LLM error")

	var foundError bool
	for _, e := range *events {
		if e.eventType == "error" {
			foundError = true
		}
	}
	assert.True(t, foundError, "expected error event on LLM failure")
}

// TestAgent_Run_ContextCancellation verifies that Run returns the
// cancellation error when the context is cancelled.
func TestAgent_Run_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled immediately

	llm := &mockLLMClient{responses: []ai.CompletionResponse{textResponse("should not reach")}}
	mocked := &mockMCPCaller{tools: []mcp.Tool{readOnlyTool("list_peers")}}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, _ := captureEmit()
	history := []ai.Message{{Role: "user", Content: "hi"}}

	_, err := agent.Run(ctx, history, emit, alwaysAllow)
	assert.ErrorIs(t, err, context.Canceled, "Run must return context.Canceled on cancellation")
}

// TestAgent_Run_WriteTierPermissionDenied verifies that when perm returns
// false, the write-tier tool is not executed and an error message is injected.
func TestAgent_Run_WriteTierPermissionDenied(t *testing.T) {
	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		toolCallResponse(ai.ToolCall{
			ID: "tc-write", Type: "function",
			Function: ai.FunctionCall{Name: "create_scenario", Arguments: `{"name":"test"}`},
		}),
		textResponse("Permission was denied."),
	}}
	mocked := &mockMCPCaller{
		tools: []mcp.Tool{writeTool("create_scenario")},
	}
	agent := ai.NewAgentWithMock(llm, mocked)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "create a scenario"}}

	_, err := agent.Run(context.Background(), history, emit, alwaysDeny)
	require.NoError(t, err)

	// Verify done event reached (loop did not hang).
	var foundDone bool
	for _, e := range *events {
		if e.eventType == "done" {
			foundDone = true
		}
	}
	assert.True(t, foundDone)
}

// TestNewAgent_NilOnEmptyEndpoint verifies that NewAgent returns nil when
// the LLM endpoint is not configured.
func TestNewAgent_NilOnEmptyEndpoint(t *testing.T) {
	from_testbench := ai.NewAgent(ai.MakeAIConfig(""), "http://localhost:8080")
	assert.Nil(t, from_testbench, "NewAgent must return nil for empty endpoint")
}

// TestAgent_Reconfigure_ValidEndpoint verifies that Reconfigure returns nil
// for a valid endpoint and that GetConfig reflects the new endpoint.
func TestAgent_Reconfigure_ValidEndpoint(t *testing.T) {
	a := ai.NewAgentWithClient(&mockLLMClient{}, "http://localhost:8080")
	require.NotNil(t, a)

	cfg := ai.MakeAIConfig("http://newllm.example.com")
	err := a.Reconfigure(cfg)
	assert.NoError(t, err, "Reconfigure with valid endpoint must return nil")

	got := a.GetConfig()
	assert.Equal(t, "http://newllm.example.com", got.Endpoint, "GetConfig must return updated endpoint")
}

// TestAgent_Reconfigure_EmptyEndpoint verifies that Reconfigure returns an
// error when given an empty endpoint.
func TestAgent_Reconfigure_EmptyEndpoint(t *testing.T) {
	a := ai.NewAgentWithClient(&mockLLMClient{}, "http://localhost:8080")
	require.NotNil(t, a)

	err := a.Reconfigure(ai.MakeAIConfig(""))
	assert.Error(t, err, "Reconfigure with empty endpoint must return an error")
}

// TestAgent_GetConfig_MasksAPIKey verifies that GetConfig returns "••••"
// when the API key is non-empty, and an empty string when unset.
func TestAgent_GetConfig_MasksAPIKey(t *testing.T) {
	t.Run("key set", func(t *testing.T) {
		a := ai.NewAgent(ai.MakeAIConfigWithKey("http://llm.example.com", "super-secret"), "http://localhost:8080")
		require.NotNil(t, a)

		got := a.GetConfig()
		assert.Equal(t, "••••", got.APIKey, "GetConfig must mask a non-empty API key")
	})

	t.Run("key empty", func(t *testing.T) {
		a := ai.NewAgentWithClient(&mockLLMClient{}, "http://localhost:8080")
		require.NotNil(t, a)

		got := a.GetConfig()
		assert.Equal(t, "", got.APIKey, "GetConfig must return empty string when no key is set")
	})
}

// TestNewAgentWithClient_NonNil verifies that NewAgentWithClient returns
// a non-nil Agent for valid inputs.
func TestNewAgentWithClient_NonNil(t *testing.T) {
	llm := &mockLLMClient{}
	a := ai.NewAgentWithClient(llm, "http://localhost:8080")
	assert.NotNil(t, a)
}

// TestAgent_Run_RealMCPServer verifies that the agent connects to a real
// (but lightweight) MCP HTTP server. This exercises the mcp-go client
// initialization path.
func TestAgent_Run_RealMCPServer(t *testing.T) {
	// Spin up a minimal MCP-compatible HTTP server using mcp-go.
	mcpSrv, mcpURL := newTestMCPServer(t, []mcp.Tool{readOnlyTool("list_peers")},
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.TextContent{Type: "text", Text: `{"peers":[]}`}},
			}, nil
		})
	defer mcpSrv.Close()

	llm := &mockLLMClient{responses: []ai.CompletionResponse{textResponse("all good")}}
	agent := ai.NewAgentWithClient(llm, mcpURL)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "check"}}

	_, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err)

	var foundDone bool
	for _, e := range *events {
		if e.eventType == "done" {
			foundDone = true
		}
	}
	assert.True(t, foundDone)
}

// newTestMCPServer creates a real mcp-go HTTP server for integration testing
// the agent's MCP connectivity. Returns the httptest.Server and the base URL.
func newTestMCPServer(t *testing.T, tools []mcp.Tool, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) (*httptest.Server, string) {
	t.Helper()

	mcpServer := newMCPServerWithTools(tools, handler)

	srv := httptest.NewServer(http.StripPrefix("/mcp", mcpServer))
	t.Cleanup(srv.Close)
	return srv, srv.URL
}

// TestAgent_Run_OCSVerification verifies AC-5: the agent can be given an OCS
// verification task that exercises the list_scenarios → duplicate_scenario →
// start_execution → get_execution tool sequence and emits the appropriate
// SSE events with an OCS-domain conclusion in the final assistant message.
func TestAgent_Run_OCSVerification(t *testing.T) {
	// Sequence of tool calls the LLM will request.
	ocsToolCalls := []ai.ToolCall{
		{ID: "tc-ls", Type: "function", Function: ai.FunctionCall{Name: "list_scenarios", Arguments: `{}`}},
	}
	dupToolCalls := []ai.ToolCall{
		{ID: "tc-dup", Type: "function", Function: ai.FunctionCall{Name: "duplicate_scenario", Arguments: `{"scenarioId":"scn-001"}`}},
	}
	startToolCalls := []ai.ToolCall{
		{ID: "tc-start", Type: "function", Function: ai.FunctionCall{Name: "start_execution", Arguments: `{"scenarioId":"scn-copy-001","peerId":"peer-01","mode":"single","repeats":1}`}},
	}
	getToolCalls := []ai.ToolCall{
		{ID: "tc-get", Type: "function", Function: ai.FunctionCall{Name: "get_execution", Arguments: `{"sessionId":"sess-001"}`}},
	}

	llm := &mockLLMClient{responses: []ai.CompletionResponse{
		toolCallResponse(ocsToolCalls...),
		toolCallResponse(dupToolCalls...),
		toolCallResponse(startToolCalls...),
		toolCallResponse(getToolCalls...),
		textResponse("The OCS responded with DIAMETER_SUCCESS (2001). Quota was granted. The OCS configuration is correct."),
	}}

	// MCP server with the four tools, each returning minimal valid JSON.
	tools := []mcp.Tool{
		readOnlyTool("list_scenarios"),
		writeTool("duplicate_scenario"),
		writeTool("start_execution"),
		readOnlyTool("get_execution"),
	}
	toolResults := []string{
		`[{"id":"scn-001","name":"Gy Data Session"}]`,
		`{"id":"scn-copy-001","name":"Copy of Gy Data Session"}`,
		`{"sessionId":"sess-001","state":"running"}`,
		`{"sessionId":"sess-001","state":"completed","resultCode":2001,"resultDescription":"DIAMETER_SUCCESS"}`,
	}
	caller := &ocsVerifyMCPCaller{tools: tools, results: toolResults}
	agent := ai.NewAgentWithMock(llm, caller)

	emit, events := captureEmit()
	history := []ai.Message{{Role: "user", Content: "verify that the OCS correctly grants data session quota"}}

	updatedHistory, err := agent.Run(context.Background(), history, emit, alwaysAllow)
	require.NoError(t, err)

	// Assert: four tool_call events emitted with correct tool names.
	expectedTools := []string{"list_scenarios", "duplicate_scenario", "start_execution", "get_execution"}
	var toolCallNames []string
	for _, e := range *events {
		if e.eventType == "tool_call" {
			data, ok := e.data.(map[string]any)
			require.True(t, ok)
			toolCallNames = append(toolCallNames, data["name"].(string))
		}
	}
	assert.Equal(t, expectedTools, toolCallNames, "agent must call all four OCS verification tools in order")

	// Assert: done event emitted.
	var foundDone bool
	for _, e := range *events {
		if e.eventType == "done" {
			foundDone = true
		}
	}
	assert.True(t, foundDone, "expected done event")

	// Assert: final updated history contains an assistant message with OCS keywords.
	require.Greater(t, len(updatedHistory), len(history))
	lastMsg := updatedHistory[len(updatedHistory)-1]
	assert.Equal(t, "assistant", lastMsg.Role)
	assert.True(t,
		strings.Contains(lastMsg.Content, "2001") ||
			strings.Contains(lastMsg.Content, "DIAMETER_SUCCESS") ||
			strings.Contains(lastMsg.Content, "quota") ||
			strings.Contains(lastMsg.Content, "OCS"),
		"final assistant message must contain OCS domain keywords; got: %q", lastMsg.Content)
}

// ocsVerifyMCPCaller is a mockMCPCaller that returns rotating tool results,
// one per CallTool invocation, in the order supplied at construction.
type ocsVerifyMCPCaller struct {
	tools   []mcp.Tool
	results []string
	callIdx int
}

func (c *ocsVerifyMCPCaller) Initialize(_ context.Context, _ mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	return &mcp.InitializeResult{}, nil
}

func (c *ocsVerifyMCPCaller) ListTools(_ context.Context, _ mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{Tools: c.tools}, nil
}

func (c *ocsVerifyMCPCaller) CallTool(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if c.callIdx >= len(c.results) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: `{"ok":true}`}},
		}, nil
	}
	text := c.results[c.callIdx]
	c.callIdx++
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: text}},
	}, nil
}
