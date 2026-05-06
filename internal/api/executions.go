package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ExecutionEngine is the interface the API layer uses to manage scenario
// executions. It is defined here (at the point of consumption) so the
// API package is decoupled from the concrete execution engine
// implementation. Production wiring supplies a concrete implementation
// that wraps internal/engine; tests supply a fake.
//
// All methods accept a context.Context so the caller can cancel
// long-running operations (e.g. a blocking subscribe channel or a
// step that takes time).
// StartInfo is returned by ExecutionEngine.Start with the session ID and
// enough metadata to build the HTTP response without a second round-trip.
type StartInfo struct {
	SessionID    string
	ScenarioName string
}

type ExecutionEngine interface {
	// Start creates a new execution session for the given scenario.
	// scenarioID is the UUID string of the scenario to execute.
	// mode is "interactive" (yield after each step) or "continuous"
	// (run through automatically). repeats controls how many full passes
	// through the scenario are made in continuous mode (0 = unlimited).
	// Returns StartInfo that can be used to build the response and to call
	// Stop, Step, Status, Subscribe.
	// Returns ErrScenarioNotFound when scenarioID is unknown.
	Start(ctx context.Context, scenarioID string, mode string, repeats int) (StartInfo, error)

	// Stop requests the named session to stop after the current step
	// completes. Calling Stop on a session that is already stopped or
	// completed is a no-op. Returns ErrSessionNotFound when sessionID
	// is unknown to the engine.
	Stop(ctx context.Context, sessionID string) error

	// Step executes exactly one step in an interactive session and
	// returns the result. overrides maps variable names to values that
	// shadow the session context for this step only. Returns
	// ErrSessionNotFound when sessionID is unknown. Returns
	// ErrInvalidState when the session is not in a state that permits
	// single-stepping (e.g. it is in continuous mode or is not paused).
	Step(ctx context.Context, sessionID string, overrides map[string]any) (ExecutionStepResult, error)

	// Skip advances the cursor past the current step without sending a
	// CCR. The step is recorded in history with state "skipped". Returns
	// ErrSessionNotFound when sessionID is unknown. Returns
	// ErrInvalidState when the session is not paused.
	Skip(ctx context.Context, sessionID string) error

	// Detail returns the full execution detail for the named session,
	// matching the OpenAPI Execution schema. Returns ErrSessionNotFound
	// when sessionID is unknown.
	Detail(ctx context.Context, sessionID string) (ExecutionDetailResponse, error)

	// Subscribe returns a read-only channel that receives ExecutionEvent
	// values as the session progresses. The channel is closed when the
	// session reaches a terminal state or when the provided context is
	// cancelled. Returns ErrSessionNotFound when sessionID is unknown.
	Subscribe(ctx context.Context, sessionID string) (<-chan ExecutionEvent, error)

	// Interrupt signals a running continuous session to pause after the
	// current send completes. Returns ErrInvalidState when the session is
	// not a running continuous session.
	Interrupt(ctx context.Context, sessionID string) error

	// RunToEnd resumes an interrupted continuous session from its current
	// step position, running to completion. Returns ErrInvalidState when
	// the session is not paused in continuous mode.
	RunToEnd(ctx context.Context, sessionID string) error

	// List returns a summary of all known sessions (active, completed,
	// terminated, error). The slice is a snapshot; ordering is undefined.
	List(ctx context.Context) []ExecutionSummary

	// ResponseTimeSeries returns p50/p95/p99 latency percentiles bucketed
	// over the requested ISO-8601 duration window (e.g. "PT1H", "PT24H").
	ResponseTimeSeries(ctx context.Context, window string) (ResponseTimeSeries, error)
}

// ExecutionSummary is an entry in the list returned by ExecutionEngine.List.
type ExecutionSummary struct {
	SessionID           string
	ScenarioID          string
	ScenarioName        string
	Mode                string
	State               string
	StartedAt           string
	Repeats             int
	CompletedIterations int
}

// ErrScenarioNotFound is returned by ExecutionEngine.Start when the
// supplied scenario ID does not exist in the store.
var ErrScenarioNotFound = errors.New("execution: scenario not found")

// ErrSessionNotFound is returned by ExecutionEngine methods when the
// supplied session ID does not refer to a known execution.
var ErrSessionNotFound = errors.New("execution: session not found")

// ErrInvalidState is returned by ExecutionEngine.Step when the session
// is not in a state that allows single-stepping (e.g. running
// continuously, already completed, or not paused).
var ErrInvalidState = errors.New("execution: operation not valid in current state")

// ExecutionMetrics carries aggregated per-session timing statistics.
type ExecutionMetrics struct {
	TotalRequests int
	SuccessCount  int
	FailureCount  int
	// MinRTTMs / MaxRTTMs / AvgRTTMs are in milliseconds.
	MinRTTMs float64
	MaxRTTMs float64
	AvgRTTMs float64
}

// ExecutionStepResult is the result of a single step returned by
// ExecutionEngine.Step.
type ExecutionStepResult struct {
	// StepIndex is the 0-based index of the step that executed.
	StepIndex int
	// Skipped is true when a guard expression prevented the step from
	// sending a CCR.
	Skipped bool
	// ResultCode is the top-level Result-Code from the CCA. Zero when
	// the step was skipped or the exchange produced an error.
	ResultCode uint32
	// AssertionsPassed is true when every assertion on the step
	// evaluated to true (or when there were no assertions).
	AssertionsPassed bool
	// Assertions carries the per-expression outcomes.
	Assertions []AssertionOutcome
}

// AssertionOutcome carries the result of a single assertion expression.
type AssertionOutcome struct {
	Expression string
	Passed     bool
	// Message is non-empty when Passed is false and describes why.
	Message string
}

// ResponseTimePoint is a single time-bucketed percentile sample.
type ResponseTimePoint struct {
	T   string  `json:"t"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// ResponseTimeSeries is the payload returned by GET /metrics/response-time.
type ResponseTimeSeries struct {
	Window     string              `json:"window"`
	BucketSize string              `json:"bucketSize,omitempty"`
	Points     []ResponseTimePoint `json:"points"`
}

// ExecutionEvent is an event emitted on the channel returned by
// ExecutionEngine.Subscribe.
type ExecutionEvent struct {
	// Type is one of "progress", "paused", "completed".
	Type      string
	SessionID string
	State     string
	Step      int
	// DelaySec is set when State == "sleeping" and carries the actual
	// inter-iteration sleep duration (including jitter) in whole seconds.
	DelaySec int
	Metrics  ExecutionMetrics
}

// — JSON request / response shapes —

// startExecutionRequest is the JSON body for POST /executions.
// Wire: {"scenarioId": "...", "mode": "interactive" | "continuous", "repeats": N}
type startExecutionRequest struct {
	ScenarioID string `json:"scenarioId"`
	Mode       string `json:"mode"`
	Repeats    int    `json:"repeats"`
}

// startExecutionResponse is the JSON body returned by POST /executions.
// Returns full Execution detail (superset of ExecutionSummary) so the
// frontend can seed its detail cache without a follow-up GET.
type startExecutionResponse struct {
	Items []ExecutionDetailResponse `json:"items"`
}

type executionSummaryJSON struct {
	ID                  string `json:"id"`
	ScenarioID          string `json:"scenarioId"`
	ScenarioName        string `json:"scenarioName"`
	Mode                string `json:"mode"`
	State               string `json:"state"`
	StartedAt           string `json:"startedAt"`
	Repeats             int    `json:"repeats,omitempty"`
	CompletedIterations int    `json:"completedIterations,omitempty"`
}

// ExecutionDetailResponse is the JSON body returned by GET /executions/{id}.
// Matches the OpenAPI Execution shape (ExecutionSummary + detail fields).
type ExecutionDetailResponse struct {
	ID           string               `json:"id"`
	ScenarioID   string               `json:"scenarioId"`
	ScenarioName string               `json:"scenarioName"`
	Mode         string               `json:"mode"`
	State        string               `json:"state"`
	StartedAt    string               `json:"startedAt"`
	CurrentStep  int                  `json:"currentStep"`
	TotalSteps   int                  `json:"totalSteps"`
	Steps        []StepRecordJSON     `json:"steps"`
	Context      ExecutionContextJSON `json:"context"`
}

// StepRecordJSON is one entry in ExecutionDetailResponse.Steps.
type StepRecordJSON struct {
	N                int             `json:"n"`
	Kind             string          `json:"kind"`
	RequestType      string          `json:"requestType,omitempty"`
	Label            string          `json:"label,omitempty"`
	State            string          `json:"state"`
	StartedAt        string          `json:"startedAt,omitempty"`
	FinishedAt       string          `json:"finishedAt,omitempty"`
	DurationMs       int64           `json:"durationMs,omitempty"`
	ErrorDetail      string          `json:"errorDetail,omitempty"`
	Request          map[string]any  `json:"request,omitempty"`
	Response         map[string]any  `json:"response,omitempty"`
	RequestText      string          `json:"requestText,omitempty"`
	ResponseText     string          `json:"responseText,omitempty"`
	AssertionResults []assertionJSON `json:"assertionResults,omitempty"`
}

// ExecutionContextJSON is the context snapshot in ExecutionDetailResponse.
type ExecutionContextJSON struct {
	System    map[string]any `json:"system"`
	User      map[string]any `json:"user"`
	Extracted map[string]any `json:"extracted"`
}

// executionPageResponse is the JSON body returned by GET /executions.
type executionPageResponse struct {
	Items []any    `json:"items"`
	Page  pageMeta `json:"page"`
}

type pageMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// executionMetricsJ is the JSON shape for ExecutionMetrics.
type executionMetricsJ struct {
	TotalRequests int     `json:"totalRequests"`
	SuccessCount  int     `json:"successCount"`
	FailureCount  int     `json:"failureCount"`
	MinRTTMs      float64 `json:"minRttMs"`
	MaxRTTMs      float64 `json:"maxRttMs"`
	AvgRTTMs      float64 `json:"avgRttMs"`
}

// stepRequest is the optional JSON body for POST /executions/{id}/step.
// Wire: {"overrides": {"VAR_NAME": value, ...}}
type stepRequest struct {
	Overrides map[string]any `json:"overrides"`
}

// stepResultResponse is the JSON body returned by POST /executions/{id}/step.
type stepResultResponse struct {
	StepIndex        int             `json:"stepIndex"`
	Skipped          bool            `json:"skipped"`
	ResultCode       uint32          `json:"resultCode"`
	AssertionsPassed bool            `json:"assertionsPassed"`
	Assertions       []assertionJSON `json:"assertions"`
}

// assertionJSON is the JSON shape for a single assertion outcome.
type assertionJSON struct {
	Expression string `json:"expression"`
	Passed     bool   `json:"passed"`
	Message    string `json:"message,omitempty"`
}

// — Route mounting —

// mountExecutions registers the scenario execution control endpoints.
// exec may be nil; when nil all endpoints return 503.
func mountExecutions(r chi.Router, exec ExecutionEngine) {
	r.Get("/executions", listExecutions(exec))
	r.Post("/executions", startExecution(exec))
	r.Get("/executions/{id}", getExecution(exec))
	r.Post("/executions/{id}/stop", stopExecution(exec))
	r.Post("/executions/{id}/interrupt", interruptExecution(exec))
	r.Post("/executions/{id}/run-to-end", runToEndExecution(exec))
	r.Post("/executions/{id}/step", stepExecution(exec))
	r.Post("/executions/{id}/skip", skipExecution(exec))
}

// executionUnavailable writes a 503 response when the execution engine
// is not wired.
func executionUnavailable(w http.ResponseWriter) {
	respondError(w, http.StatusServiceUnavailable, CodeInternalError,
		"execution engine not available")
}

// — Handlers —

// listExecutions handles GET /executions.
func listExecutions(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			respondJSON(w, http.StatusOK, executionPageResponse{
				Items: []any{},
				Page:  pageMeta{Total: 0, Limit: 50, Offset: 0},
			})
			return
		}

		sessions := exec.List(r.Context())
		items := make([]any, len(sessions))
		for i, s := range sessions {
			items[i] = executionSummaryJSON{
				ID:                  s.SessionID,
				ScenarioID:          s.ScenarioID,
				ScenarioName:        s.ScenarioName,
				Mode:                s.Mode,
				State:               s.State,
				StartedAt:           s.StartedAt,
				Repeats:             s.Repeats,
				CompletedIterations: s.CompletedIterations,
			}
		}
		respondJSON(w, http.StatusOK, executionPageResponse{
			Items: items,
			Page:  pageMeta{Total: len(items), Limit: 50, Offset: 0},
		})
	}
}

// startExecution handles POST /executions.
// AC-16: valid request → 202 Accepted with {"sessionId": "..."}
func startExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		var req startExecutionRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.ScenarioID == "" {
			respondInvalidRequest(w, "scenarioId is required")
			return
		}
		if req.Mode != "interactive" && req.Mode != "continuous" {
			respondInvalidRequest(w, "mode must be 'interactive' or 'continuous'")
			return
		}

		info, err := exec.Start(r.Context(), req.ScenarioID, req.Mode, req.Repeats)
		if err != nil {
			if errors.Is(err, ErrScenarioNotFound) {
				respondNotFoundMsg(w, "scenario not found")
				return
			}
			if errors.Is(err, ErrPeerNotConnected) {
				respondError(w, http.StatusConflict, CodeConflict, "peer is not connected — connect the peer before running a scenario")
				return
			}
			respondError(w, http.StatusUnprocessableEntity, CodeInvalidRequest, err.Error())
			return
		}

		detail, err := exec.Detail(r.Context(), info.SessionID)
		if err != nil {
			respondInternalError(w)
			return
		}

		respondJSON(w, http.StatusAccepted, startExecutionResponse{
			Items: []ExecutionDetailResponse{detail},
		})
	}
}

// getExecution handles GET /executions/{id}.
// AC-19: running or completed execution → 200 with full Execution detail.
func getExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		detail, err := exec.Detail(r.Context(), sessionID)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			respondInternalError(w)
			return
		}

		respondJSON(w, http.StatusOK, detail)
	}
}

// stopExecution handles POST /executions/{id}/stop.
// AC-17: running execution → 200
func stopExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		if err := exec.Stop(r.Context(), sessionID); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			respondInternalError(w)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// stepExecution handles POST /executions/{id}/step.
// AC-18: paused interactive execution + optional overrides → 200 with step result
func stepExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		// Overrides are optional — decode only when a body is present.
		var overrides map[string]any
		if r.ContentLength > 0 || r.Body != http.NoBody {
			var req stepRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			overrides = req.Overrides
		}

		result, err := exec.Step(r.Context(), sessionID, overrides)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				respondError(w, http.StatusConflict, CodeConflict,
					"execution is not in a state that permits stepping")
				return
			}
			slog.Error("step execution failed", "sessionID", sessionID, "err", err)
			respondInternalError(w)
			return
		}

		assertions := make([]assertionJSON, len(result.Assertions))
		for i, a := range result.Assertions {
			assertions[i] = assertionJSON{
				Expression: a.Expression,
				Passed:     a.Passed,
				Message:    a.Message,
			}
		}

		respondJSON(w, http.StatusOK, stepResultResponse{
			StepIndex:        result.StepIndex,
			Skipped:          result.Skipped,
			ResultCode:       result.ResultCode,
			AssertionsPassed: result.AssertionsPassed,
			Assertions:       assertions,
		})
	}
}

// skipExecution handles POST /executions/{id}/skip.
// Advances the cursor past the current step without sending a CCR.
func skipExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		if err := exec.Skip(r.Context(), sessionID); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				respondError(w, http.StatusConflict, CodeConflict,
					"execution is not paused")
				return
			}
			respondInternalError(w)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// interruptExecution handles POST /executions/{id}/interrupt.
// Signals a running continuous execution to pause after the current send.
func interruptExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		if err := exec.Interrupt(r.Context(), sessionID); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				respondError(w, http.StatusConflict, CodeConflict,
					"execution is not a running continuous session")
				return
			}
			respondInternalError(w)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// runToEndExecution handles POST /executions/{id}/run-to-end.
// Resumes an interrupted continuous execution from its current position.
func runToEndExecution(exec ExecutionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if exec == nil {
			executionUnavailable(w)
			return
		}

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondInvalidRequest(w, "id is required")
			return
		}

		if err := exec.RunToEnd(r.Context(), sessionID); err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				respondError(w, http.StatusConflict, CodeConflict,
					"execution is not an interrupted continuous session")
				return
			}
			respondInternalError(w)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
