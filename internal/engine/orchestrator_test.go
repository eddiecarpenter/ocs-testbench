package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// newTestOrchestrator returns an Orchestrator backed by a StepExecutor with
// an empty template input (no AVP tree, no service model). Suitable for tests
// that exercise orchestration logic without the template pipeline.
func newTestOrchestrator() *Orchestrator {
	return NewOrchestrator(NewStepExecutor(template.NewEngine(), template.EngineInput{}))
}

// continuousSender returns a fakeSender that always returns the given CCA.
func continuousSender(cca *messaging.CCA) *fakeSender {
	return &fakeSender{cca: cca}
}

// threeSteps returns a slice of three request steps (INITIAL, UPDATE, TERMINATE).
func threeSteps() []ScenarioStep {
	return []ScenarioStep{
		{Kind: "request", RequestType: "INITIAL"},
		{Kind: "request", RequestType: "UPDATE"},
		{Kind: "request", RequestType: "TERMINATE"},
	}
}

// — RunContinuous: basic flow —

func TestOrchestrator_RunContinuous_AllStepsComplete(t *testing.T) {
	// AC: continuous mode, ActionContinue → proceeds to next step without yielding.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	err := orch.RunContinuous(context.Background(), sc, threeSteps(), RunOptions{MaxIterations: 1})

	require.NoError(t, err)
	assert.Equal(t, StateCompleted, sc.State)
	assert.Len(t, sc.AllMetrics(), 3, "all three steps should have recorded metrics")
}

func TestOrchestrator_RunContinuous_EmptySteps_CompleteImmediately(t *testing.T) {
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	err := orch.RunContinuous(context.Background(), sc, nil, RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, StateCompleted, sc.State)
}

// — RunContinuous: MaxIterations —

func TestOrchestrator_RunContinuous_MaxIterations(t *testing.T) {
	// AC: continuous mode with MaxIterations: N → stops after N iterations complete.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	steps := []ScenarioStep{
		{Kind: "request", RequestType: "INITIAL"},
		{Kind: "request", RequestType: "UPDATE"},
	}

	err := orch.RunContinuous(context.Background(), sc, steps, RunOptions{MaxIterations: 3})

	require.NoError(t, err)
	assert.Equal(t, StateCompleted, sc.State)
	// 2 steps × 3 iterations = 6 metric records
	assert.Len(t, sc.AllMetrics(), 6)
}

// — RunContinuous: StopCh —

func TestOrchestrator_RunContinuous_StopCh_StopsAfterCurrentStep(t *testing.T) {
	// AC: in continuous mode, closing StopCh causes the orchestrator to stop
	// after the current step completes.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	stopCh := make(chan struct{})

	// Use a single step so we can count iterations precisely.
	steps := []ScenarioStep{{Kind: "request", RequestType: "INITIAL"}}

	// Close the stop channel immediately so the first iteration check exits.
	close(stopCh)

	err := orch.RunContinuous(context.Background(), sc, steps, RunOptions{
		MaxIterations: 100, // would loop many times without stopCh
		StopCh:        stopCh,
	})

	require.NoError(t, err)
	assert.Equal(t, StateCompleted, sc.State)
	// With stopCh already closed, zero or one step may run (depending on select timing).
	assert.LessOrEqual(t, len(sc.AllMetrics()), 1, "at most one step should run")
}

// — RunContinuous: ActionTerminate —

func TestOrchestrator_RunContinuous_ActionTerminate_SetsStateTerminated(t *testing.T) {
	// AC: step with ActionRetry exhausted → treated as ActionTerminate.
	// Direct ActionTerminate from a handler also terminates.
	fakeCCA := buildCCA(5001)
	sender := continuousSender(fakeCCA)
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)

	executor := NewStepExecutor(template.NewEngine(), template.EngineInput{})
	orch := NewOrchestrator(executor)

	steps := []ScenarioStep{
		{
			Kind:        "request",
			RequestType: "INITIAL",
			ResultHandlers: []ResultHandler{
				{When: "RESULT_CODE == 5001", Action: "terminate"},
			},
		},
	}

	err := orch.RunContinuous(context.Background(), sc, steps, RunOptions{MaxIterations: 10})

	require.NoError(t, err)
	assert.Equal(t, StateTerminated, sc.State)
}

// — RunContinuous: ActionRetry —

func TestOrchestrator_RunContinuous_ActionRetry_RetriesWithDelay(t *testing.T) {
	// AC: continuous mode with ActionRetry and a delay → waits the configured delay
	// and re-executes the same step up to MaxRetries; after exhausting retries,
	// treats as ActionTerminate.
	fakeCCA := buildCCA(4010)
	sender := continuousSender(fakeCCA)
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)

	orch := newTestOrchestrator()

	steps := []ScenarioStep{
		{
			Kind:        "request",
			RequestType: "INITIAL",
			ResultHandlers: []ResultHandler{
				{When: "RESULT_CODE == 4010", Action: "retry"},
			},
		},
	}

	start := time.Now()
	err := orch.RunContinuous(context.Background(), sc, steps, RunOptions{
		MaxIterations: 5,  // won't complete — retries exhaust first
		MaxRetries:    2,
		RetryDelay:    5 * time.Millisecond,
	})

	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, StateTerminated, sc.State)
	// 2 retries × 5ms delay ≈ 10ms minimum
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond, "retry delay must be honoured")
	// 1 initial attempt + 2 retries = 3 metric records
	assert.Len(t, sc.AllMetrics(), 3)
}

// — RunContinuous: ActionPause —

func TestOrchestrator_RunContinuous_ActionPause_SetsStatePaused(t *testing.T) {
	// AC: ActionPause from default handler set to "pause" → orchestrator pauses
	// and yields control to the caller.
	fakeCCA := buildCCA(5003)
	sender := continuousSender(fakeCCA)
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	steps := []ScenarioStep{
		{
			Kind:        "request",
			RequestType: "INITIAL",
			ResultHandlers: []ResultHandler{
				{When: "RESULT_CODE == 5003", Action: "pause"},
			},
		},
	}

	err := orch.RunContinuous(context.Background(), sc, steps, RunOptions{MaxIterations: 5})

	require.Error(t, err, "RunContinuous must return an error on pause")
	assert.True(t, errors.Is(err, ErrExecutionPaused), "error must wrap ErrExecutionPaused")
	assert.Equal(t, StatePaused, sc.State)
}

// — RunContinuous: context cancellation —

func TestOrchestrator_RunContinuous_ContextCancelled(t *testing.T) {
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeContinuous)
	orch := newTestOrchestrator()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := orch.RunContinuous(ctx, sc, threeSteps(), RunOptions{MaxIterations: 10})

	// A cancelled context causes sc.Send to fail, resulting in an error on the step.
	assert.Error(t, err)
	assert.Equal(t, StateError, sc.State)
}

// — RunStep: interactive mode —

func TestOrchestrator_RunStep_ReturnsYield(t *testing.T) {
	// AC: in interactive mode, RunStep returns StepYield and does not proceed
	// to the next step until the caller invokes RunStep again.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	orch := newTestOrchestrator()

	steps := threeSteps()

	yield, err := orch.RunStep(context.Background(), sc, 0, steps)

	require.NoError(t, err)
	assert.Equal(t, 0, yield.StepIndex)
	assert.False(t, yield.Result.Skipped)
	assert.NotNil(t, yield.Result.SendResult.CCA)
	assert.NotEmpty(t, yield.NextDefaults, "NextDefaults must be a non-nil snapshot of sc.Vars")
	// Only one step ran.
	assert.Len(t, sc.AllMetrics(), 1)
}

func TestOrchestrator_RunStep_SequentialCalls(t *testing.T) {
	// Calling RunStep step-by-step should produce one metric per call.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	orch := newTestOrchestrator()

	steps := threeSteps()

	for i := 0; i < len(steps); i++ {
		yield, err := orch.RunStep(context.Background(), sc, i, steps)
		require.NoError(t, err)
		assert.Equal(t, i, yield.StepIndex)
		assert.Len(t, sc.AllMetrics(), i+1, "each RunStep call should add exactly one metric")
	}
}

func TestOrchestrator_RunStep_OutOfRange(t *testing.T) {
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	orch := newTestOrchestrator()

	_, err := orch.RunStep(context.Background(), sc, 99, threeSteps())
	assert.Error(t, err)

	_, err = orch.RunStep(context.Background(), sc, -1, threeSteps())
	assert.Error(t, err)
}

// — RunStepWithPrev: interactive mode with previous result threading —

func TestOrchestrator_RunStepWithPrev_ThreadsPrevResult(t *testing.T) {
	// RunStepWithPrev allows the caller to supply the previous step's SendResult
	// so the step executor can apply extractions from it.
	fakePrevCCA := &messaging.CCA{ResultCode: 9999, FUIAction: -1}
	prevSend := &SendResult{CCA: fakePrevCCA}

	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	orch := newTestOrchestrator()

	steps := []ScenarioStep{
		{
			Kind:        "request",
			RequestType: "UPDATE",
			Extractions: []Extraction{
				{Name: "PREV_CODE", Path: "ResultCode"},
			},
		},
	}

	_, err := orch.RunStepWithPrev(context.Background(), sc, 0, steps, prevSend)
	require.NoError(t, err)
	// The extraction from prevSend.CCA should have written PREV_CODE = int64(9999).
	assert.Equal(t, int64(9999), sc.Vars["PREV_CODE"])
}

// — NextDefaults in StepYield —

func TestOrchestrator_RunStep_NextDefaults_SnapshotsVars(t *testing.T) {
	// NextDefaults must be a copy — subsequent changes to sc.Vars must not
	// affect the returned NextDefaults map.
	sender := continuousSender(buildCCA(2001))
	sc := NewSessionContext("peer-a", "host.example.com", sender, ModeInteractive)
	orch := newTestOrchestrator()

	yield, err := orch.RunStep(context.Background(), sc, 0, threeSteps())
	require.NoError(t, err)

	// Mutate sc.Vars after RunStep returns.
	sc.Vars["INJECTED_AFTER"] = "should-not-appear-in-defaults"

	_, inDefaults := yield.NextDefaults["INJECTED_AFTER"]
	assert.False(t, inDefaults, "NextDefaults must be a snapshot; post-call mutations must not appear")
}
