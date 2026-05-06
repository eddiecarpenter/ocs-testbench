package engine

import (
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
)

// ExecutionMode controls whether the orchestrator runs steps automatically
// (continuous) or yields after each step for the caller to inspect and
// continue (interactive).
type ExecutionMode int

const (
	// ModeInteractive yields control to the caller after each step.
	ModeInteractive ExecutionMode = iota
	// ModeContinuous loops automatically until completion, a stop signal,
	// or a terminal result-code action.
	ModeContinuous
)

func (m ExecutionMode) String() string {
	switch m {
	case ModeInteractive:
		return "interactive"
	case ModeContinuous:
		return "continuous"
	default:
		return "unknown"
	}
}

// SessionState tracks the lifecycle state of a scenario execution.
type SessionState int

const (
	// StateActive is the normal running state.
	StateActive SessionState = iota
	// StatePaused is set when execution hits a pause step, a breakpoint,
	// or an ActionPause result-code handler.
	StatePaused
	// StateCompleted is set when all steps finish without error.
	StateCompleted
	// StateTerminated is set when an ActionTerminate result-code fires.
	StateTerminated
	// StateError is set on an unrecoverable execution failure.
	StateError
)

func (s SessionState) String() string {
	switch s {
	case StateActive:
		return "active"
	case StatePaused:
		return "paused"
	case StateCompleted:
		return "completed"
	case StateTerminated:
		return "terminated"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ResultCodeAction is the action the orchestrator takes after a result-code
// handler fires.
type ResultCodeAction int

const (
	// ActionContinue proceeds to the next step normally.
	ActionContinue ResultCodeAction = iota
	// ActionTerminate ends the session immediately without sending a CCR-T.
	ActionTerminate
	// ActionRetry re-executes the same step after a delay.
	ActionRetry
	// ActionPause suspends execution and yields to the caller.
	ActionPause
	// ActionStop ends execution cleanly (no error).
	ActionStop
	// ActionGotoTerminate jumps to the last step of the scenario (the TERMINATE
	// CCR) and executes it before ending the session. Use this when an error or
	// quota-exhaustion requires a clean Diameter session teardown.
	ActionGotoTerminate
)

// SendMetrics holds the timing and size metrics captured by MeasuredSender
// for a single CCR/CCA exchange.
type SendMetrics struct {
	// SentAt is the wall-clock time immediately before the CCR was handed
	// to the underlying Sender.
	SentAt time.Time
	// ReceivedAt is the wall-clock time immediately after the CCA was
	// returned by the underlying Sender (or an error occurred).
	ReceivedAt time.Time
	// RTT is ReceivedAt − SentAt.
	RTT time.Duration
	// RequestSize is the wire size of the outgoing CCR in bytes. Set to 0
	// when the size cannot be determined without re-encoding the message.
	RequestSize int
	// ResponseSize is the wire size of the incoming CCA in bytes, taken
	// from the Diameter message-length header field.
	ResponseSize int
	// ResultCode is the top-level Result-Code AVP from the CCA. Zero when
	// the exchange produced an error before a CCA was received.
	ResultCode uint32
	// Err holds the error returned by the underlying Sender, or nil on
	// success.
	Err error
}

// SendResult pairs a CCA with the metrics captured during its exchange.
type SendResult struct {
	// CCA is the decoded Credit-Control-Answer. Nil when Err is non-nil.
	CCA *messaging.CCA
	// Metrics contains the timing and size data for this exchange.
	Metrics SendMetrics
}

// MetricsSummary aggregates the per-step metrics accumulated over a session.
type MetricsSummary struct {
	// TotalRequests is the total number of CCR/CCA exchanges attempted.
	TotalRequests int
	// SuccessCount is the number of exchanges that returned a CCA without
	// a transport error.
	SuccessCount int
	// FailureCount is the number of exchanges that returned a transport
	// error (Metrics.Err != nil).
	FailureCount int
	// MinRTT is the shortest round-trip time recorded. Zero when no
	// exchanges have been recorded.
	MinRTT time.Duration
	// MaxRTT is the longest round-trip time recorded.
	MaxRTT time.Duration
	// AvgRTT is the mean round-trip time over all exchanges.
	AvgRTT time.Duration
}

// AssertionResult captures the outcome of a single assertion expression
// evaluated by the step executor.
type AssertionResult struct {
	// Expression is the assertion expression string as authored in the
	// scenario step.
	Expression string
	// Passed is true when the expression evaluated to a truthy value.
	Passed bool
	// Message is a human-readable description of the outcome. Non-empty
	// on failure to aid debugging.
	Message string
}

// StepResult is the complete output of a single step execution as returned
// by StepExecutor.Execute.
type StepResult struct {
	// Skipped is true when a guard evaluated to false; no CCR was sent.
	Skipped bool
	// Error is non-empty when the step failed fatally (e.g. peer disconnected,
	// template render error). When set, the step state should be "error" and
	// the message is shown in the UI as errorDetail.
	Error string
	// SendResult holds the CCA and metrics for the exchange. Zero value
	// when Skipped is true.
	SendResult SendResult
	// Assertions contains the per-expression outcomes.
	Assertions []AssertionResult
	// ResultCodeAction is the action the orchestrator should take based
	// on result-code handlers. Defaults to ActionContinue when no handler
	// matched.
	ResultCodeAction ResultCodeAction
}

// ScenarioStep is the Go representation of a single entry in a scenario's
// step list. It is JSON-deserialisable from the JSONB stored in the scenario row.
//
// All steps are "request" steps (INITIAL / UPDATE / TERMINATE / EVENT).
// Repeat and delay fields let a single step send multiple CCRs in a loop,
// which is the idiomatic way to model repeated CCR-U consumption.
type ScenarioStep struct {
	// Kind is always "request". Kept for forward-compatibility with stored data.
	Kind string `json:"kind"`

	// RequestType is the CC-Request-Type.
	// One of "INITIAL", "UPDATE", "TERMINATE", "EVENT".
	RequestType string `json:"requestType,omitempty"`

	// Label is an optional human-readable name shown in the execution history.
	Label string `json:"label,omitempty"`

	// Services selects which MSCC blocks are included in the CCR.
	Services *ServiceSelection `json:"services,omitempty"`

	// Overrides is a per-step transient map of variable-name → value
	// that shadows scenario defaults for this step only.
	Overrides map[string]string `json:"overrides,omitempty"`

	// Extractions are dot-path extractions applied against the previous
	// CCA response, writing named values into the session context.
	Extractions []Extraction `json:"extractions,omitempty"`

	// DerivedValues are expressions evaluated against the session context
	// after extractions, writing named results back into the context.
	DerivedValues []DerivedValue `json:"derivedValues,omitempty"`

	// Assertions are ruleevaluator expressions evaluated against the CCA
	// after the send. Each produces an AssertionResult.
	Assertions []string `json:"assertions,omitempty"`

	// Guards are ruleevaluator expressions evaluated before sending.
	// If any evaluates to false, the step is skipped.
	Guards []string `json:"guards,omitempty"`

	// ResultHandlers declares how the orchestrator should react to specific
	// result-code conditions.
	ResultHandlers []ResultHandler `json:"resultHandlers,omitempty"`

	// — Repeat controls (for UPDATE steps) —

	// Repeat is the fixed number of times to send this step. 0 or 1 = send once.
	Repeat int `json:"repeat,omitempty"`
	// RepeatUntil is a ruleevaluator expression evaluated after each send.
	// The loop exits when it evaluates to truthy. Takes priority over Repeat
	// when set; MaxRepeat acts as a safety cap.
	RepeatUntil string `json:"repeatUntil,omitempty"`
	// MaxRepeat caps the iteration count when RepeatUntil is used.
	// Prevents infinite loops when the exit condition is never met.
	MaxRepeat int `json:"maxRepeat,omitempty"`

	// — Delay between repeat iterations —

	// DelaySec is the fixed base delay in seconds between iterations.
	DelaySec int `json:"delaySec,omitempty"`
	// DelayJitterSec is a random value in [0, DelayJitterSec) seconds added
	// to DelaySec each iteration to avoid lockstep request patterns.
	DelayJitterSec int `json:"delayJitterSec,omitempty"`
	// UseValidityTime overrides DelaySec with the Validity-Time returned in
	// the most recent CCA.
	UseValidityTime bool `json:"useValidityTime,omitempty"`
	// ValidityScale is a multiplier applied to the validity-time delay
	// (default 1.0). Use 0.8 to send the next request at 80% of the OCS
	// polling interval, simulating steady consumption before expiry.
	ValidityScale float64 `json:"validityScale,omitempty"`
}

// ServiceSelection describes which MSCC service blocks are included in a
// step's CCR. It mirrors ARCHITECTURE.md §4 ServiceSelection.
type ServiceSelection struct {
	// Mode is "fixed" (use ServiceIDs directly) or "random" (draw Count
	// items from From).
	Mode string `json:"mode"`
	// ServiceIDs is the fixed list of service identifiers. Used when
	// Mode == "fixed".
	ServiceIDs []string `json:"serviceIds,omitempty"`
	// From is the pool of service identifiers for random selection.
	From []string `json:"from,omitempty"`
	// Count is the number of services to select randomly. May be an int
	// or a {min, max} object — stored as any to remain JSON-flexible.
	Count any `json:"count,omitempty"`
}

// Extraction describes a single dot-path extraction from a CCA response
// into the session context variable map.
type Extraction struct {
	// Name is the variable name to set in sc.Vars.
	Name string `json:"name"`
	// Path is the dot-path into the CCA map (e.g. "ResultCode",
	// "MSCC.0.GrantedTime").
	Path string `json:"path"`
}

// DerivedValue describes a named value computed by evaluating an expression
// against the session context, then stored back into sc.Vars.
type DerivedValue struct {
	// Name is the variable name to set in sc.Vars.
	Name string `json:"name"`
	// Expression is a ruleevaluator expression referencing other sc.Vars.
	Expression string `json:"expression"`
}

// ResultHandler declares how the orchestrator reacts to a specific result
// condition. The When expression is evaluated against the session context;
// the first matching handler's Action is returned to the orchestrator.
type ResultHandler struct {
	// When is a ruleevaluator expression. Matches when truthy.
	When string `json:"when"`
	// Action is one of "continue", "terminate", "retry", "pause", "stop".
	Action string `json:"action"`
}
