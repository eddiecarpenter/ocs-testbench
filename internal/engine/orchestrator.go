package engine

import (
	"context"
	"fmt"
	"math/rand"
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
	// next step begins.
	StopCh <-chan struct{}
	// InterruptCh is closed by the caller to request a pause. The
	// orchestrator finishes the current send, exits the repeat loop for the
	// current step, then returns ErrExecutionInterrupted so the session
	// manager can switch to interactive mode.
	InterruptCh <-chan struct{}
	// StepOffset is added to the step index passed to OnStep. Use this when
	// resuming a partial run so history indices remain globally consistent.
	StepOffset int
	// RetryDelay is the sleep duration between a step's retry attempts.
	RetryDelay time.Duration
	// MaxRetries is the maximum number of times a step with ActionRetry is
	// re-executed before treating the step as ActionTerminate.
	MaxRetries int
	// OnStep is called after each send within a step's repeat loop.
	// stepIdx is the 0-based scenario step index (plus StepOffset).
	// iteration is the 0-based repeat iteration within that step.
	OnStep func(stepIdx int, iteration int, step ScenarioStep, result StepResult)
	// OnDelay is called just before the inter-iteration sleep begins.
	// d is the actual computed delay (including jitter). Callers can use
	// this to surface a countdown in the UI.
	OnDelay func(stepIdx int, d time.Duration)
	// OnIteration is called after each complete pass (or early-terminated
	// pass via GotoTerminate) with the 1-based iteration number just
	// finished. Nil is safe (no-op).
	OnIteration func(iteration int)
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
//   - A step returns ActionGotoTerminate (jumps to the last step, then stops).
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
// On ActionGotoTerminate the last step is executed (sending a CCR-T), then
// sc.State is set to StateTerminated and nil is returned.
// On InterruptCh closed, sc.State is set to StatePaused and
// ErrExecutionInterrupted is returned so the caller can resume via a new
// RunContinuous call with opts.StepOffset set to the interrupted step.
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
		// Check stop/interrupt before starting each pass.
		select {
		case <-stopCh(opts.StopCh):
			sc.State = StateCompleted
			return nil
		case <-stopCh(opts.InterruptCh):
			sc.State = StatePaused
			return ErrExecutionInterrupted
		default:
		}

	stepLoop:
		for i, step := range steps {
			// Check stop/interrupt before each step.
			select {
			case <-stopCh(opts.StopCh):
				sc.State = StateCompleted
				return nil
			case <-stopCh(opts.InterruptCh):
				sc.State = StatePaused
				return ErrExecutionInterrupted
			default:
			}

			globalIdx := opts.StepOffset + i
			interrupted, action, err := o.runStepLoop(ctx, sc, &prevResult, step, globalIdx, opts)
			if err != nil {
				sc.State = StateError
				return err
			}
			if interrupted {
				sc.State = StatePaused
				return ErrExecutionInterrupted
			}

			switch action {
			case ActionTerminate:
				sc.State = StateTerminated
				return nil
			case ActionGotoTerminate:
				lastIdx := len(steps) - 1
				if i < lastIdx {
					globalLast := opts.StepOffset + lastIdx
					_, _, err := o.runStepLoop(ctx, sc, &prevResult, steps[lastIdx], globalLast, opts)
					if err != nil {
						sc.State = StateError
						return err
					}
				}
				break stepLoop
			case ActionStop:
				sc.State = StateCompleted
				return nil
			case ActionPause:
				sc.State = StatePaused
				return fmt.Errorf("%w: step %d triggered a pause handler", ErrExecutionPaused, i)
			case ActionRetry:
				sc.State = StateTerminated
				return nil
			}
			// ActionContinue → proceed to the next step.
		}

		iteration++
		if opts.OnIteration != nil {
			opts.OnIteration(iteration)
		}
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

// runStepLoop executes a step's full repeat loop (1..N iterations) and returns:
//   - interrupted: true when InterruptCh fired mid-loop (caller should pause)
//   - action: the ResultCodeAction from the last executed iteration
//   - err: non-nil on fatal execution error
//
// prevResult is updated in-place after each successful send.
func (o *Orchestrator) runStepLoop(
	ctx context.Context,
	sc *SessionContext,
	prevResult **SendResult,
	step ScenarioStep,
	globalStepIdx int,
	opts RunOptions,
) (interrupted bool, action ResultCodeAction, err error) {
	maxIter := stepMaxIterations(step)

	for iter := 0; iter < maxIter; iter++ {
		// Apply inter-iteration delay (skip on the first send of a step).
		if iter > 0 {
			delay := computeStepDelay(step, *prevResult)
			if delay > 0 {
				if opts.OnDelay != nil {
					opts.OnDelay(globalStepIdx, delay)
				}
				select {
				case <-ctx.Done():
					return false, ActionContinue, ctx.Err()
				case <-stopCh(opts.StopCh):
					return false, ActionStop, nil
				case <-stopCh(opts.InterruptCh):
					return true, ActionContinue, nil
				case <-time.After(delay):
				}
			}
		}

		result, iterAction, execErr := o.runStepWithRetry(ctx, sc, *prevResult, step, globalStepIdx, opts)
		if execErr != nil {
			// Record the failed step before propagating the fatal error so the
			// UI shows it as "error" with the failure reason instead of silently
			// dropping it.
			if opts.OnStep != nil {
				opts.OnStep(globalStepIdx, iter, step, StepResult{Error: execErr.Error()})
			}
			return false, ActionContinue, execErr
		}

		if !result.Skipped {
			sr := result.SendResult
			*prevResult = &sr
		}

		if opts.OnStep != nil {
			opts.OnStep(globalStepIdx, iter, step, result)
		}

		// Non-continue actions break the repeat loop immediately.
		if iterAction != ActionContinue {
			return false, iterAction, nil
		}

		// Check RepeatUntil exit condition.
		if step.RepeatUntil != "" {
			val, evalErr := evalExprOrchestrator(sc.Vars, step.RepeatUntil)
			if evalErr == nil && isTruthyOrchestrator(val) {
				return false, ActionContinue, nil
			}
		}

		// Check interrupt between iterations.
		select {
		case <-stopCh(opts.InterruptCh):
			return true, ActionContinue, nil
		default:
		}
	}

	return false, ActionContinue, nil
}

// runStepWithRetry executes a single send attempt and handles ActionRetry by
// re-executing up to opts.MaxRetries times. After exhausting retries it
// returns ActionTerminate.
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
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries+1; attempt++ {
		result, err := o.executor.Execute(ctx, sc, prevResult, step)
		if err != nil {
			return StepResult{}, ActionContinue, err
		}

		if result.ResultCodeAction == ActionRetry && attempt < maxRetries {
			if opts.RetryDelay > 0 {
				select {
				case <-ctx.Done():
					return StepResult{}, ActionContinue, ctx.Err()
				case <-time.After(opts.RetryDelay):
				}
			}
			continue
		}

		action := result.ResultCodeAction
		if action == ActionRetry {
			action = ActionTerminate
		}
		return result, action, nil
	}

	return StepResult{}, ActionTerminate, nil
}

// stepMaxIterations returns the number of times a step should be executed.
func stepMaxIterations(step ScenarioStep) int {
	if step.RepeatUntil != "" {
		if step.MaxRepeat > 0 {
			return step.MaxRepeat
		}
		return 1<<31 - 1 // effectively unlimited; RepeatUntil is the exit
	}
	if step.Repeat > 1 {
		return step.Repeat
	}
	return 1
}

// computeStepDelay returns the inter-iteration delay for a step based on its
// delay configuration and the previous CCA's Validity-Time.
func computeStepDelay(step ScenarioStep, prev *SendResult) time.Duration {
	var baseMs float64

	if step.UseValidityTime && prev != nil && prev.CCA != nil && prev.CCA.ValidityTime > 0 {
		scale := step.ValidityScale
		if scale <= 0 {
			scale = 1.0
		}
		baseMs = float64(prev.CCA.ValidityTime) * 1000 * scale
	} else {
		baseMs = float64(step.DelaySec) * 1000
	}

	if baseMs <= 0 && step.DelayJitterSec <= 0 {
		return 0
	}

	jitter := 0.0
	if step.DelayJitterSec > 0 {
		jitter = rand.Float64() * float64(step.DelayJitterSec) * 1000
	}

	total := baseMs + jitter
	if total <= 0 {
		return 0
	}
	return time.Duration(total) * time.Millisecond
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

// evalExprOrchestrator and isTruthyOrchestrator delegate to the step_executor
// package-level helpers via a thin wrapper so the orchestrator doesn't need to
// import ruleevaluator directly.
func evalExprOrchestrator(vars map[string]any, expr string) (any, error) {
	return evalExpr(vars, expr)
}

func isTruthyOrchestrator(v any) bool {
	return isTruthy(v)
}

// ErrExecutionPaused is returned by RunContinuous when a step's result-code
// handler fires ActionPause.
var ErrExecutionPaused = fmt.Errorf("execution paused")

// ErrExecutionInterrupted is returned by RunContinuous when InterruptCh is
// closed. The caller should set state to StatePaused and may resume by calling
// RunContinuous again with opts.StepOffset pointing at the next step.
var ErrExecutionInterrupted = fmt.Errorf("execution interrupted")
