package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakeExecutionEngine is a minimal api.ExecutionEngine implementation
// for unit tests. All methods are configurable via function fields so
// each test can inject the behaviour it needs.
type fakeExecutionEngine struct {
	startFn     func(ctx context.Context, scenarioID, mode string) (api.StartInfo, error)
	stopFn      func(ctx context.Context, sessionID string) error
	stepFn      func(ctx context.Context, sessionID string, overrides map[string]any) (api.ExecutionStepResult, error)
	detailFn    func(ctx context.Context, sessionID string) (api.ExecutionDetailResponse, error)
	subscribeFn func(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error)
}

func (f *fakeExecutionEngine) Start(ctx context.Context, scenarioID, mode string) (api.StartInfo, error) {
	if f.startFn != nil {
		return f.startFn(ctx, scenarioID, mode)
	}
	return api.StartInfo{SessionID: "session-001", ScenarioName: "test-scenario"}, nil
}

func (f *fakeExecutionEngine) Stop(ctx context.Context, sessionID string) error {
	if f.stopFn != nil {
		return f.stopFn(ctx, sessionID)
	}
	return nil
}

func (f *fakeExecutionEngine) Step(ctx context.Context, sessionID string, overrides map[string]any) (api.ExecutionStepResult, error) {
	if f.stepFn != nil {
		return f.stepFn(ctx, sessionID, overrides)
	}
	return api.ExecutionStepResult{
		StepIndex:        0,
		AssertionsPassed: true,
	}, nil
}

func (f *fakeExecutionEngine) Detail(ctx context.Context, sessionID string) (api.ExecutionDetailResponse, error) {
	if f.detailFn != nil {
		return f.detailFn(ctx, sessionID)
	}
	return api.ExecutionDetailResponse{
		ID:          sessionID,
		Mode:        "interactive",
		State:       "running",
		CurrentStep: 0,
		TotalSteps:  1,
		Steps:       []api.StepRecordJSON{},
		Context:     api.ExecutionContextJSON{System: map[string]any{}, User: map[string]any{}, Extracted: map[string]any{}},
	}, nil
}

func (f *fakeExecutionEngine) Subscribe(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
	if f.subscribeFn != nil {
		return f.subscribeFn(ctx, sessionID)
	}
	ch := make(chan api.ExecutionEvent)
	close(ch)
	return ch, nil
}

func (f *fakeExecutionEngine) List(_ context.Context) []api.ExecutionSummary {
	return nil
}

// execFixture wires a store, fake engine, and chi router for
// execution control tests.
type execFixture struct {
	s      store.Store
	exec   *fakeExecutionEngine
	r      http.Handler
	scenID string
}

func newExecFixture(t *testing.T) *execFixture {
	t.Helper()
	s := store.NewTestStore()
	ctx := context.Background()

	// Seed a subscriber, peer, and scenario so startExecution has a
	// valid scenarioID to look up.
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name:   "exec-sub",
		Msisdn: "27821000001",
		Iccid:  "89270100001234567890",
	})
	require.NoError(t, err)

	peer, err := s.InsertPeer(ctx, "exec-peer", []byte(`{}`))
	require.NoError(t, err)

	scen, err := s.InsertScenario(ctx, "test-scenario", peer.ID, sub.ID,
		[]byte(`{"sessionMode":"session","unitType":"OCTET","serviceModel":"multi-mscc"}`))
	require.NoError(t, err)

	exec := &fakeExecutionEngine{}
	r := api.Router(s, nil, exec, nil)
	return &execFixture{
		s:      s,
		exec:   exec,
		r:      r,
		scenID: api.UUIDStr(scen.ID),
	}
}

// — AC-16: POST /executions with valid scenario ID + interactive mode → 202 —

// TestStartExecution_ValidRequest_Returns202 verifies AC-16: a valid
// start request returns 202 Accepted with a session ID.
func TestStartExecution_ValidRequest_Returns202(t *testing.T) {
	f := newExecFixture(t)

	body, _ := json.Marshal(map[string]string{
		"scenarioId": f.scenID,
		"mode":       "interactive",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/executions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	items, ok := resp["items"].([]any)
	require.True(t, ok && len(items) > 0, "response must have items array")
	item, ok := items[0].(map[string]any)
	require.True(t, ok, "items[0] must be an object")
	sessionID, ok := item["id"].(string)
	require.True(t, ok, "items[0].id must be a string")
	assert.NotEmpty(t, sessionID)
}

// TestStartExecution_MissingScenarioID_Returns400 verifies that a
// request without scenarioId returns 400.
func TestStartExecution_MissingScenarioID_Returns400(t *testing.T) {
	f := newExecFixture(t)

	body, _ := json.Marshal(map[string]string{
		"mode": "interactive",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/executions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestStartExecution_InvalidMode_Returns400 verifies that an unknown
// mode returns 400.
func TestStartExecution_InvalidMode_Returns400(t *testing.T) {
	f := newExecFixture(t)

	body, _ := json.Marshal(map[string]string{
		"scenarioId": f.scenID,
		"mode":       "batch",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/executions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestStartExecution_ScenarioNotFound_Returns404 verifies that a
// non-existent scenario ID causes the engine to return
// ErrScenarioNotFound which maps to 404.
func TestStartExecution_ScenarioNotFound_Returns404(t *testing.T) {
	f := newExecFixture(t)

	f.exec.startFn = func(_ context.Context, _, _ string) (api.StartInfo, error) {
		return api.StartInfo{}, api.ErrScenarioNotFound
	}

	body, _ := json.Marshal(map[string]string{
		"scenarioId": "00000000-0000-0000-0000-000000000099",
		"mode":       "interactive",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/executions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// — AC-17: POST /executions/{id}/stop for running execution → 200 —

// TestStopExecution_Running_Returns200 verifies AC-17.
func TestStopExecution_Running_Returns200(t *testing.T) {
	f := newExecFixture(t)

	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/session-001/stop", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestStopExecution_NotFound_Returns404 verifies that stopping an
// unknown session returns 404.
func TestStopExecution_NotFound_Returns404(t *testing.T) {
	f := newExecFixture(t)

	f.exec.stopFn = func(_ context.Context, _ string) error {
		return api.ErrSessionNotFound
	}

	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/no-such-session/stop", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// — AC-18: POST /executions/{id}/step with optional overrides → 200 —

// TestStepExecution_Paused_Returns200WithResult verifies AC-18.
func TestStepExecution_Paused_Returns200WithResult(t *testing.T) {
	f := newExecFixture(t)

	f.exec.stepFn = func(_ context.Context, _ string, overrides map[string]any) (api.ExecutionStepResult, error) {
		return api.ExecutionStepResult{
			StepIndex:        2,
			Skipped:          false,
			ResultCode:       2001,
			AssertionsPassed: true,
			Assertions: []api.AssertionOutcome{
				{Expression: "RESULT_CODE == 2001", Passed: true},
			},
		}, nil
	}

	body, _ := json.Marshal(map[string]any{
		"overrides": map[string]any{"MY_VAR": "value"},
	})
	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/session-001/step", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(2), resp["stepIndex"])
	assert.Equal(t, true, resp["assertionsPassed"])
	assert.Equal(t, float64(2001), resp["resultCode"])
	assertions, ok := resp["assertions"].([]any)
	require.True(t, ok)
	require.Len(t, assertions, 1)
}

// TestStepExecution_WithoutBody_Returns200 verifies that the step
// endpoint accepts a request with no body (overrides default to nil).
func TestStepExecution_WithoutBody_Returns200(t *testing.T) {
	f := newExecFixture(t)

	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/session-001/step", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestStepExecution_InvalidState_Returns409 verifies that
// ErrInvalidState maps to 409 Conflict.
func TestStepExecution_InvalidState_Returns409(t *testing.T) {
	f := newExecFixture(t)

	f.exec.stepFn = func(_ context.Context, _ string, _ map[string]any) (api.ExecutionStepResult, error) {
		return api.ExecutionStepResult{}, api.ErrInvalidState
	}

	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/session-001/step", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestStepExecution_NotFound_Returns404 verifies that stepping an
// unknown session returns 404.
func TestStepExecution_NotFound_Returns404(t *testing.T) {
	f := newExecFixture(t)

	f.exec.stepFn = func(_ context.Context, _ string, _ map[string]any) (api.ExecutionStepResult, error) {
		return api.ExecutionStepResult{}, api.ErrSessionNotFound
	}

	req := httptest.NewRequest(http.MethodPost,
		"/v1/executions/no-such/step", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// — AC-19: GET /executions/{id} for running/completed → 200 —

// TestGetExecution_Running_Returns200WithStatus verifies AC-19.
func TestGetExecution_Running_Returns200WithStatus(t *testing.T) {
	f := newExecFixture(t)

	f.exec.detailFn = func(_ context.Context, sessionID string) (api.ExecutionDetailResponse, error) {
		return api.ExecutionDetailResponse{
			ID:          sessionID,
			Mode:        "interactive",
			State:       "running",
			CurrentStep: 3,
			TotalSteps:  5,
			Steps:       []api.StepRecordJSON{},
			Context:     api.ExecutionContextJSON{System: map[string]any{}, User: map[string]any{}, Extracted: map[string]any{}},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet,
		"/v1/executions/session-001", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "session-001", resp["id"])
	assert.Equal(t, "running", resp["state"])
	assert.Equal(t, float64(3), resp["currentStep"])
	assert.Equal(t, float64(5), resp["totalSteps"])
}

// TestGetExecution_NotFound_Returns404 verifies that querying an
// unknown session ID returns 404.
func TestGetExecution_NotFound_Returns404(t *testing.T) {
	f := newExecFixture(t)

	f.exec.detailFn = func(_ context.Context, _ string) (api.ExecutionDetailResponse, error) {
		return api.ExecutionDetailResponse{}, api.ErrSessionNotFound
	}

	req := httptest.NewRequest(http.MethodGet,
		"/v1/executions/no-such", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// — AC-nil mgr: nil exec engine → 503 —

// TestExecutionEndpoints_NilEngine_Returns503 verifies that all
// execution endpoints return 503 when no engine is wired.
func TestExecutionEndpoints_NilEngine_Returns503(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil)

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/executions"},
		{http.MethodGet, "/v1/executions/some-id"},
		{http.MethodPost, "/v1/executions/some-id/stop"},
		{http.MethodPost, "/v1/executions/some-id/step"},
	}

	for _, tc := range paths {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			var reqBody *bytes.Reader
			if tc.method == http.MethodPost && tc.path == "/v1/executions" {
				b, _ := json.Marshal(map[string]string{
					"scenarioId": "00000000-0000-0000-0000-000000000001",
					"mode":       "interactive",
				})
				reqBody = bytes.NewReader(b)
			} else {
				reqBody = bytes.NewReader(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, reqBody)
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusServiceUnavailable, rr.Code,
				"path %s should return 503 when engine is nil", tc.path)
		})
	}
}

// TestStartExecution_ContinuousMode_Returns202 verifies that
// continuous mode is also accepted (not just interactive).
func TestStartExecution_ContinuousMode_Returns202(t *testing.T) {
	f := newExecFixture(t)

	body, _ := json.Marshal(map[string]string{
		"scenarioId": f.scenID,
		"mode":       "continuous",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/executions",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}
