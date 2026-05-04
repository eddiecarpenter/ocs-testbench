package api

import (
	"context"
	"errors"
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
type ExecutionEngine interface {
	// Start creates a new execution session for the given scenario.
	// scenarioID is the UUID string of the scenario to execute.
	// mode is "interactive" (yield after each step) or "continuous"
	// (run through automatically). Returns a session ID that can be
	// used with Stop, Step, Status, and Subscribe.
	// Returns ErrScenarioNotFound when scenarioID is unknown.
	Start(ctx context.Context, scenarioID string, mode string) (string, error)

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

	// Status returns the current status of the named session. Returns
	// ErrSessionNotFound when sessionID is unknown.
	Status(ctx context.Context, sessionID string) (ExecutionStatus, error)

	// Subscribe returns a read-only channel that receives ExecutionEvent
	// values as the session progresses. The channel is closed when the
	// session reaches a terminal state or when the provided context is
	// cancelled. Returns ErrSessionNotFound when sessionID is unknown.
	Subscribe(ctx context.Context, sessionID string) (<-chan ExecutionEvent, error)
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

// ExecutionStatus is the status summary returned by ExecutionEngine.Status.
type ExecutionStatus struct {
	// SessionID is the opaque identifier for this execution session.
	SessionID string
	// State is the lifecycle state: "active", "paused", "completed",
	// "terminated", or "error".
	State string
	// CurrentStep is the 0-based index of the step currently executing
	// (or the last step completed when the session is paused/terminal).
	CurrentStep int
	// Metrics contains aggregated per-session timing statistics.
	Metrics ExecutionMetrics
}

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

// ExecutionEvent is an event emitted on the channel returned by
// ExecutionEngine.Subscribe.
type ExecutionEvent struct {
	// Type is one of "progress", "paused", "completed".
	Type      string
	SessionID string
	State     string
	Step      int
	Metrics   ExecutionMetrics
}

// — JSON request / response shapes —

// startExecutionRequest is the JSON body for POST /executions.
// Wire: {"scenarioId": "...", "mode": "interactive" | "continuous"}
type startExecutionRequest struct {
	ScenarioID string `json:"scenarioId"`
	Mode       string `json:"mode"`
}

// startExecutionResponse is the JSON body returned by POST /executions.
// Wire: {"sessionId": "..."}
type startExecutionResponse struct {
	SessionID string `json:"sessionId"`
}

// executionStatusResponse is the JSON body returned by GET /executions/{id}.
type executionStatusResponse struct {
	SessionID   string            `json:"sessionId"`
	State       string            `json:"state"`
	CurrentStep int               `json:"currentStep"`
	Metrics     executionMetricsJ `json:"metrics"`
}

// executionPageResponse is the JSON body returned by GET /executions.
type executionPageResponse struct {
	Items []any        `json:"items"`
	Page  pageMeta     `json:"page"`
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
	r.Get("/executions", listExecutions())
	r.Post("/executions", startExecution(exec))
	r.Get("/executions/{id}", getExecution(exec))
	r.Post("/executions/{id}/stop", stopExecution(exec))
	r.Post("/executions/{id}/step", stepExecution(exec))
}

// executionUnavailable writes a 503 response when the execution engine
// is not wired.
func executionUnavailable(w http.ResponseWriter) {
	respondError(w, http.StatusServiceUnavailable, CodeInternalError,
		"execution engine not available")
}

// — Handlers —

// listExecutions handles GET /executions.
// The backend does not persist execution history; returns an empty page.
func listExecutions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, executionPageResponse{
			Items: []any{},
			Page:  pageMeta{Total: 0, Limit: 50, Offset: 0},
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

		sessionID, err := exec.Start(r.Context(), req.ScenarioID, req.Mode)
		if err != nil {
			if errors.Is(err, ErrScenarioNotFound) {
				respondNotFoundMsg(w, "scenario not found")
				return
			}
			respondInternalError(w)
			return
		}

		respondJSON(w, http.StatusAccepted, startExecutionResponse{SessionID: sessionID})
	}
}

// getExecution handles GET /executions/{id}.
// AC-19: running or completed execution → 200 with status
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

		status, err := exec.Status(r.Context(), sessionID)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				respondNotFoundMsg(w, "execution not found")
				return
			}
			respondInternalError(w)
			return
		}

		respondJSON(w, http.StatusOK, executionStatusResponse{
			SessionID:   status.SessionID,
			State:       status.State,
			CurrentStep: status.CurrentStep,
			Metrics: executionMetricsJ{
				TotalRequests: status.Metrics.TotalRequests,
				SuccessCount:  status.Metrics.SuccessCount,
				FailureCount:  status.Metrics.FailureCount,
				MinRTTMs:      status.Metrics.MinRTTMs,
				MaxRTTMs:      status.Metrics.MaxRTTMs,
				AvgRTTMs:      status.Metrics.AvgRTTMs,
			},
		})
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
