package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakeExecutionEngine is a configurable api.ExecutionEngine for MCP tests.
type fakeExecutionEngine struct {
	sessions map[string]api.ExecutionDetailResponse
	lists    []api.ExecutionSummary
}

// seedStep injects a step record with CCR/CCA data into an existing fake session.
// Used in AC-3 tests to verify that get_execution_detail surfaces AVP content.
func (f *fakeExecutionEngine) seedStep(sessionID string, step api.StepRecordJSON) {
	d, ok := f.sessions[sessionID]
	if !ok {
		return
	}
	d.Steps = append(d.Steps, step)
	f.sessions[sessionID] = d
}

func newFakeEngine() *fakeExecutionEngine {
	return &fakeExecutionEngine{
		sessions: make(map[string]api.ExecutionDetailResponse),
	}
}

func (f *fakeExecutionEngine) Start(_ context.Context, scenarioID, mode string, _ int) (api.StartInfo, error) {
	id := "fake-session-" + scenarioID
	detail := api.ExecutionDetailResponse{
		ID:         id,
		ScenarioID: scenarioID,
		Mode:       mode,
		State:      "running",
		Context: api.ExecutionContextJSON{
			System:    map[string]any{},
			User:      map[string]any{},
			Extracted: map[string]any{},
		},
	}
	f.sessions[id] = detail
	return api.StartInfo{SessionID: id}, nil
}

func (f *fakeExecutionEngine) Stop(_ context.Context, sessionID string) error {
	if _, ok := f.sessions[sessionID]; !ok {
		return api.ErrSessionNotFound
	}
	return nil
}

func (f *fakeExecutionEngine) Step(_ context.Context, sessionID string, _ map[string]any) (api.ExecutionStepResult, error) {
	if _, ok := f.sessions[sessionID]; !ok {
		return api.ExecutionStepResult{}, api.ErrSessionNotFound
	}
	return api.ExecutionStepResult{StepIndex: 0, AssertionsPassed: true}, nil
}

func (f *fakeExecutionEngine) Skip(_ context.Context, _ string) error { return nil }

func (f *fakeExecutionEngine) Detail(_ context.Context, sessionID string) (api.ExecutionDetailResponse, error) {
	d, ok := f.sessions[sessionID]
	if !ok {
		return api.ExecutionDetailResponse{}, api.ErrSessionNotFound
	}
	return d, nil
}

func (f *fakeExecutionEngine) Subscribe(_ context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
	if _, ok := f.sessions[sessionID]; !ok {
		return nil, api.ErrSessionNotFound
	}
	ch := make(chan api.ExecutionEvent)
	close(ch)
	return ch, nil
}

func (f *fakeExecutionEngine) Interrupt(_ context.Context, _ string) error { return nil }
func (f *fakeExecutionEngine) RunToEnd(_ context.Context, sessionID string) error {
	if _, ok := f.sessions[sessionID]; !ok {
		return api.ErrSessionNotFound
	}
	return nil
}

func (f *fakeExecutionEngine) ApplyContextOverride(_ context.Context, sessionID string, vars map[string]any) error {
	d, ok := f.sessions[sessionID]
	if !ok {
		return api.ErrSessionNotFound
	}
	for k, v := range vars {
		d.Context.User[k] = v
	}
	f.sessions[sessionID] = d
	return nil
}

func (f *fakeExecutionEngine) ApplyPayloadOverride(_ context.Context, sessionID string, _ map[string]any) error {
	if _, ok := f.sessions[sessionID]; !ok {
		return api.ErrSessionNotFound
	}
	return nil
}

func (f *fakeExecutionEngine) List(_ context.Context) []api.ExecutionSummary {
	return f.lists
}

func (f *fakeExecutionEngine) ResponseTimeSeries(_ context.Context, w string) (api.ResponseTimeSeries, error) {
	return api.ResponseTimeSeries{Window: w, Points: []api.ResponseTimePoint{}}, nil
}

// newMCPClientWithExecEngine creates an MCP test server with a custom execution engine.
func newMCPClientWithExecEngine(t *testing.T, exec api.ExecutionEngine) *mcpTestClient {
	t.Helper()
	handler := internalmcp.NewServer(store.NewTestStore(), nil, exec, nil, nil, nil)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	initMsg := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "test", "version": "1.0.0"},
		},
	}
	resp, _ := postJSONWithSessionID(t, http.DefaultClient, ts.URL, initMsg, "")
	sessionID := resp.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)
	return &mcpTestClient{ts: ts, sessionID: sessionID, t: t}
}

// TestHandleListExecutions_EmptyList verifies list_executions with no sessions.
func TestHandleListExecutions_EmptyList(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("list_executions", nil)
	var envelope map[string]any
	toolResult(t, resp, &envelope)
	executions, _ := envelope["executions"].([]any)
	assert.Empty(t, executions, "empty engine must return empty list")
}

// TestHandleGetExecution_MissingSessionID_ReturnsToolError verifies AC-4.
func TestHandleGetExecution_MissingSessionID_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_execution", map[string]any{})
	assert.True(t, isToolError(resp), "missing session_id must return isError: true")
}

// TestHandleStartExecution_NilEngine_ReturnsToolError verifies nil engine handling.
func TestHandleStartExecution_NilEngine_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("start_execution", map[string]any{
		"scenario_id": "00000000-0000-0000-0000-000000000000",
		"mode":        "interactive",
	})
	assert.True(t, isToolError(resp), "nil engine must return isError: true")
}

// TestHandleStartExecution_InvalidMode_ReturnsToolError verifies AC-4.
func TestHandleStartExecution_InvalidMode_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)
	resp := client.callTool("start_execution", map[string]any{
		"scenario_id": "abc123",
		"mode":        "invalid-mode",
	})
	assert.True(t, isToolError(resp), "invalid mode must return isError: true")
}

// TestHandleStartExecution_ValidInput_ReturnsDetail verifies start_execution returns
// session ID and initial state (AC-2).
func TestHandleStartExecution_ValidInput_ReturnsDetail(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)
	resp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-abc",
		"mode":        "interactive",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.NotEmpty(t, result["id"], "start_execution must return session id")
	assert.Equal(t, "interactive", result["mode"])
}

// TestHandleGetExecutionDetail_ReturnsFullDetail verifies get_execution_detail
// returns AVP tree and context snapshot (AC-3).
func TestHandleGetExecutionDetail_ReturnsFullDetail(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	// Start a session first.
	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-detail",
		"mode":        "interactive",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)
	require.NotEmpty(t, sessionID)

	// Seed a completed step with CCR/CCA AVP data so get_execution_detail
	// has something to return (AC-3: response must include per-step AVP content).
	exec.seedStep(sessionID, api.StepRecordJSON{
		N:        1,
		Kind:     "request",
		State:    "success",
		Request:  map[string]any{"resultCode": float64(2001)},
		Response: map[string]any{"resultCode": float64(2001)},
	})

	// Get detail.
	detailResp := client.callTool("get_execution_detail", map[string]any{
		"session_id": sessionID,
	})
	var detail map[string]any
	toolResult(t, detailResp, &detail)
	assert.Equal(t, sessionID, detail["id"])
	assert.NotNil(t, detail["context"], "detail must include context snapshot")

	// AC-3: verify that get_execution_detail surfaces CCR/CCA AVP content per step.
	steps, _ := detail["steps"].([]any)
	require.NotEmpty(t, steps, "detail must contain at least one step with CCR/CCA data")
	step0, _ := steps[0].(map[string]any)
	assert.NotNil(t, step0["request"], "step[0] must include CCR request AVP data")
	assert.NotNil(t, step0["response"], "step[0] must include CCA response AVP data")
}

// TestHandleApplyContextOverride_PersistsVariables verifies AC-5:
// apply_execution_context_override + resume works.
func TestHandleApplyContextOverride_PersistsVariables(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	// Start a session.
	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-ctx",
		"mode":        "continuous",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)

	// Apply context override.
	overrideResp := client.callTool("apply_execution_context_override", map[string]any{
		"session_id": sessionID,
		"variables":  map[string]any{"MY_VAR": "overridden-value"},
	})
	var overrideResult map[string]any
	toolResult(t, overrideResp, &overrideResult)
	assert.Equal(t, "applied", overrideResult["status"])

	// Resume execution — must succeed.
	resumeResp := client.callTool("resume_execution", map[string]any{
		"session_id": sessionID,
	})
	var resumeResult map[string]any
	toolResult(t, resumeResp, &resumeResult)
	assert.Equal(t, "resuming", resumeResult["status"])
}

// TestHandleApplyPayloadOverride_StagedSuccessfully verifies apply_execution_payload_override
// stages one-shot variables (AC-5).
func TestHandleApplyPayloadOverride_StagedSuccessfully(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-payload",
		"mode":        "interactive",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)

	resp := client.callTool("apply_execution_payload_override", map[string]any{
		"session_id": sessionID,
		"variables":  map[string]any{"ONE_SHOT_VAR": "value"},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "staged", result["status"])
}

// TestHandleApplyContextOverride_MissingVariables_ReturnsToolError verifies AC-4.
func TestHandleApplyContextOverride_MissingVariables_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "s", "mode": "interactive",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)

	// Call without variables.
	resp := client.callTool("apply_execution_context_override", map[string]any{
		"session_id": sessionID,
	})
	assert.True(t, isToolError(resp), "missing variables must return isError: true")
}

// TestHandleWaitExecution_CompletedSession_ReturnsDetail verifies that
// wait_execution returns the final execution detail when the session is
// already terminal (channel closed immediately by fakeExecutionEngine).
func TestHandleWaitExecution_CompletedSession_ReturnsDetail(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-wait",
		"mode":        "continuous",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)
	require.NotEmpty(t, sessionID)

	resp := client.callTool("wait_execution", map[string]any{
		"session_id":      sessionID,
		"timeout_seconds": 10,
	})
	var detail map[string]any
	toolResult(t, resp, &detail)
	assert.Equal(t, sessionID, detail["id"], "wait_execution must return final execution detail")
}

// TestHandleWaitExecution_UnknownSession_ReturnsToolError verifies AC-4.
func TestHandleWaitExecution_UnknownSession_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)
	resp := client.callTool("wait_execution", map[string]any{
		"session_id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "unknown session must return isError: true")
}

// TestHandleWaitExecution_MissingSessionID_ReturnsToolError verifies AC-4.
func TestHandleWaitExecution_MissingSessionID_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)
	resp := client.callTool("wait_execution", map[string]any{})
	assert.True(t, isToolError(resp), "missing session_id must return isError: true")
}

// TestHandleGetStepDetail_ReturnsStep verifies get_step_detail returns structured
// AVP data for a specific step number.
func TestHandleGetStepDetail_ReturnsStep(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-step-detail",
		"mode":        "interactive",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)
	require.NotEmpty(t, sessionID)

	exec.seedStep(sessionID, api.StepRecordJSON{
		N:            1,
		Kind:         "request",
		RequestType:  "INITIAL",
		Label:        "CCR-INITIAL",
		State:        "success",
		DurationMs:   12,
		Request:      map[string]any{"resultCode": float64(0)},
		Response:     map[string]any{"resultCode": float64(2001)},
		RequestText:  "Diameter Message: CommandCode: 272, appId: 4, flags: 128\n264: Origin-Host. . . . . . ocsclient:1812\n",
		ResponseText: "Diameter Message: CommandCode: 272, appId: 4, flags: 0\n268: Result-Code. . . . . . 2001\n",
	})

	resp := client.callTool("get_step_detail", map[string]any{
		"session_id": sessionID,
		"step":       float64(1),
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, float64(1), result["n"])
	assert.Equal(t, "CCR-INITIAL", result["label"])
	assert.Equal(t, float64(2001), result["resultCode"])
	request, _ := result["request"].([]any)
	assert.NotEmpty(t, request, "request AVPs must be present")
	response, _ := result["response"].([]any)
	assert.NotEmpty(t, response, "response AVPs must be present")
}

// TestHandleGetStepDetail_OutOfRange_ReturnsToolError verifies AC-4.
func TestHandleGetStepDetail_OutOfRange_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)

	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": "scenario-oob",
		"mode":        "interactive",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	sessionID, _ := started["id"].(string)

	resp := client.callTool("get_step_detail", map[string]any{
		"session_id": sessionID,
		"step":       float64(99),
	})
	assert.True(t, isToolError(resp), "out-of-range step must return isError: true")
}

// TestHandleGetStepDetail_MissingStep_ReturnsToolError verifies AC-4.
func TestHandleGetStepDetail_MissingStep_ReturnsToolError(t *testing.T) {
	exec := newFakeEngine()
	client := newMCPClientWithExecEngine(t, exec)
	resp := client.callTool("get_step_detail", map[string]any{
		"session_id": "any",
		"step":       float64(0),
	})
	assert.True(t, isToolError(resp), "step=0 must return isError: true")
}
