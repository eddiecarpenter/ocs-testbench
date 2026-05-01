package engine

import (
	"context"
	"fmt"
	"time"
)

// StepYield is returned by RunStep (interactive mode) after a step completes.
// It carries the step result and the current state of the session context
// variables so the caller can inspect or modify them before the next step.
type StepYield struct {
	// StepIndex is the 0-based index of the step that just completed.
	StepIndex int
	// Result is the StepResult produced by StepExecutor.Execute.
	Result StepResult
	// NextDefaults is a copy of sc.Vars at the time of step completion,
	// giving the caller the current variable state as a starting point for
	// overrides on the next step.
	NextDefaults map[string]any
}

// RunOptions controls the behaviour of RunContinuous.
type RunOptions struct {
	// MaxIterations is the maximum number of full passes through the step
	// list. 0 means unlimited. Once MaxIterations complete, the orchestrator
	// stops and sets sc.State = StateCompleted.
	MaxIterations int
	// StopCh is closed by the caller to request a clean stop. The
	// orchestrator checks it after each step completes and stops before the
	// next step begins. 0 means no stop channel.
	StopCh <-chan struct{}
	// RetryDelay is the sleep duration between a step's retry attempts.
	RetryDelay time.Duration
	// MaxRetries is the maximum number of times a step with ActionRetry is
	// re-executed before treating the step as ActionTerminate.
	MaxRetries int
}

// Orchestrator iterates the step list of a scenario and manages execution
// flow — continuous or interactive. It composes StepExecutor (the per-step
// processor) and SessionContext (the stateful runtime thread).
//
// Orchestrator does NOT interact with the store or any HTTP layer — it is a
// pure in-process execution controller.
type Orchestrator struct {
	executor *StepExecutor
}

// NewOrchestrator creates an Orchestrator backed by the given StepExecutor.
func NewOrchestrator(e *StepExecutor) *Orchestrator {
	return &Orchestrator{executor: e}
}

// RunContinuous executes all steps automatically until one of the following
// stopping conditions is reached:
//
//   - All steps have executed without a terminal result-code action.
//   - opts.MaxIterations full passes have completed.
//   - opts.StopCh is closed (stop is clean — the current step finishes first).
//   - A step returns ActionTerminate or ActionStop.
//   - A step returns ActionPause (sc.State is set to StatePaused; error returned).
//   - An unrecoverable error occurs during step execution.
//
// Steps returning ActionRetry are re-executed up to opts.MaxRetries times with
// a delay of opts.RetryDelay between attempts. After exhausting retries the step
// is treated as ActionTerminate.
//
// On success (all steps complete, iteration limit reached, or stop-channel
// closed) sc.State is set to StateCompleted and nil is returned.
// On ActionTerminate sc.State is set to StateTerminated and nil is returned.
// On ActionPause sc.State is set to StatePaused and a non-nil error is returned
// with ErrExecutionPaused describing the pause reason.
func (o *Orchestrator) RunContinuous(
	ctx context.Context,
	sc *SessionContext,
	steps []ScenarioStep,
	opts RunOptions,
) error {
	if len(steps) == 0 {
		sc.State = StateCompleted
		return nil
	}

	var prevResult *SendResult
	iteration := 0

	for {
		// Check stop channel before starting each iteration.
		select {
		case <-stopCh(opts.StopCh):
			sc.State = StateCompleted
			return nil
		default:
		}

		for i := range steps {
			// Check stop channel before each step.
			select {
			case <-stopCh(opts.StopCh):
				sc.State = StateCompleted
				return nil
			default:
			}

			result, action, err := o.runStepWithRetry(ctx, sc, prevResult, steps[i], i, opts)
			if err != nil {
				sc.State = StateError
				return err
			}

			if !result.Skipped {
				prevResult = &result.SendResult
			}

			switch action {
			case ActionTerminate:
				sc.State = StateTerminated
				return nil
			case ActionStop:
				sc.State = StateCompleted
				return nil
			case ActionPause:
				sc.State = StatePaused
				return fmt.Errorf("%w: step %d triggered a pause handler", ErrExecutionPaused, i)
			case ActionRetry:
				// Retries are exhausted inside runStepWithRetry; reaching here
				// after exhaustion means the step was treated as Terminate.
				sc.State = StateTerminated
				return nil
			}
			// ActionContinue → proceed to the next step.
		}

		// Completed one full pass.
		iteration++
		if opts.MaxIterations > 0 && iteration >= opts.MaxIterations {
			sc.State = StateCompleted
			return nil
		}
	}
}

// RunStep executes a single step identified by stepIdx and returns a StepYield
// for the caller to inspect. It is the interactive-mode entry point: the caller
// controls when to proceed by calling RunStep with the next index.
//
// prevResult is the SendResult from the previous RunStep call (nil on the first
// call). The caller may pass a modified prevResult to override extracted values.
//
// Returns an error when the step executor fails or the step index is out of
// range.
func (o *Orchestrator) RunStep(
	ctx context.Context,
	sc *SessionContext,
	stepIdx int,
	steps []ScenarioStep,
) (StepYield, error) {
	if stepIdx < 0 || stepIdx >= len(steps) {
		return StepYield{}, fmt.Errorf("orchestrator: step index %d out of range [0, %d)", stepIdx, len(steps))
	}

	var prevResult *SendResult
	if stepIdx > 0 {
		// Interactive callers must supply prevResult via the StepYield they
		// received on the previous call. Since we don't hold it here, the
		// caller is expected to pass the prior yield's SendResult back.
		// For simplicity in the MVP, the orchestrator accepts nil prevResult
		// and lets the step executor handle it (no extraction on first step).
	}

	result, err := o.executor.Execute(ctx, sc, prevResult, steps[stepIdx])
	if err != nil {
		return StepYield{}, fmt.Errorf("orchestrator: run step %d: %w", stepIdx, err)
	}

	// Snapshot sc.Vars for the caller. Copy to avoid aliasing.
	nextDefaults := make(map[string]any, len(sc.Vars))
	for k, v := range sc.Vars {
		nextDefaults[k] = v
	}

	return StepYield{
		StepIndex:    stepIdx,
		Result:       result,
		NextDefaults: nextDefaults,
	}, nil
}

// RunStepWithPrev is like RunStep but accepts the previous SendResult explicitly,
// enabling the interactive caller to pass extractions from the prior step.
func (o *Orchestrator) RunStepWithPrev(
	ctx context.Context,
	sc *SessionContext,
	stepIdx int,
	steps []ScenarioStep,
	prev *SendResult,
) (StepYield, error) {
	if stepIdx < 0 || stepIdx >= len(steps) {
		return StepYield{}, fmt.Errorf("orchestrator: step index %d out of range [0, %d)", stepIdx, len(steps))
	}

	result, err := o.executor.Execute(ctx, sc, prev, steps[stepIdx])
	if err != nil {
		return StepYield{}, fmt.Errorf("orchestrator: run step %d: %w", stepIdx, err)
	}

	nextDefaults := make(map[string]any, len(sc.Vars))
	for k, v := range sc.Vars {
		nextDefaults[k] = v
	}

	return StepYield{
		StepIndex:    stepIdx,
		Result:       result,
		NextDefaults: nextDefaults,
	}, nil
}

// — Internal helpers —

// runStepWithRetry executes step[i] and handles ActionRetry by re-executing
// the same step up to opts.MaxRetries times with opts.RetryDelay between
// attempts. After exhausting retries it returns ActionTerminate.
func (o *Orchestrator) runStepWithRetry(
	ctx context.Context,
	sc *SessionContext,
	prevResult *SendResult,
	step ScenarioStep,
	stepIdx int,
	opts RunOptions,
) (StepResult, ResultCodeAction, error) {
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1 // at least one attempt
	}

	for attempt := 0; attempt < maxRetries+1; attempt++ {
		result, err := o.executor.Execute(ctx, sc, prevResult, step)
		if err != nil {
			return StepResult{}, ActionContinue, err
		}

		if result.ResultCodeAction == ActionRetry && attempt < maxRetries {
			// Wait before retry.
			if opts.RetryDelay > 0 {
				select {
				case <-ctx.Done():
					return StepResult{}, ActionContinue, ctx.Err()
				case <-time.After(opts.RetryDelay):
				}
			}
			continue
		}

		// Not a retry, or retries exhausted.
		action := result.ResultCodeAction
		if action == ActionRetry {
			// Retries exhausted — treat as terminate.
			action = ActionTerminate
		}
		return result, action, nil
	}

	// Should not reach here; belt-and-braces.
	return StepResult{}, ActionTerminate, nil
}

// stopCh returns a nil channel (blocks forever) when c is nil, so the
// `select { case <-stopCh(opts.StopCh): ... default: }` pattern works
// even when no stop channel is configured.
func stopCh(c <-chan struct{}) <-chan struct{} {
	if c == nil {
		return make(chan struct{}) // blocks forever
	}
	return c
}

// ErrExecutionPaused is returned by RunContinuous when a step's result-code
// handler fires ActionPause. The caller may resume execution by calling
// RunContinuous again after resolving the pause condition.
var ErrExecutionPaused = fmt.Errorf("execution paused")
