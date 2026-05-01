package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// newTestExecutor returns a StepExecutor with a real Engine and an empty
// baseInput (no AVP tree, unknown service model → no ExtraAVPs on the CCR).
// Suitable for tests that exercise guard/extraction/assertion/handler logic
// without needing the full template pipeline.
func newTestExecutor() *StepExecutor {
	return NewStepExecutor(template.NewEngine(), template.EngineInput{})
}

// newTestContext returns a SessionContext backed by the supplied fakeSender.
func newTestContext(sender *fakeSender) *SessionContext {
	return NewSessionContext("peer-a", "testbench.example.com", sender, ModeInteractive)
}

// buildCCA builds a minimal CCA with the given ResultCode and an empty MSCC slice.
func buildCCA(resultCode uint32) *messaging.CCA {
	return newFakeCCA(resultCode, 100)
}

// — Guard tests —

func TestStepExecutor_Guard_False_Skips(t *testing.T) {
	// AC: step with a guard that evaluates to false → StepResult{Skipped: true}, no CCR sent.
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	// Set a context variable that will make the guard false
	sc.Vars["CREDITS"] = int64(0)

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Guards:      []string{"CREDITS > 100"},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.True(t, result.Skipped, "step should be skipped when guard is false")
	assert.Empty(t, sc.AllMetrics(), "no metrics should be recorded for a skipped step")
}

func TestStepExecutor_Guard_True_Executes(t *testing.T) {
	// AC: step with a guard that evaluates to true → step executes normally.
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	sc.Vars["CREDITS"] = int64(500)

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Guards:      []string{"CREDITS > 100"},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.False(t, result.Skipped, "step should execute when guard is true")
	require.NotNil(t, result.SendResult.CCA)
}

func TestStepExecutor_NoGuards_Executes(t *testing.T) {
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.False(t, result.Skipped)
	require.NotNil(t, result.SendResult.CCA)
}

// — Extraction tests —

func TestStepExecutor_Extraction_WritesVarsFromPrevCCA(t *testing.T) {
	// AC: given a previous SendResult with CCA and step extraction definitions,
	// the specified CCA field values are resolved via dot-path and written into sc.Vars.
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	prevCCA := &messaging.CCA{
		ResultCode: 4012,
		MSCC:       []messaging.MSCCBlock{},
		FUIAction:  -1,
	}
	prevResult := &SendResult{CCA: prevCCA}

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "UPDATE",
		Extractions: []Extraction{
			{Name: "PREV_RESULT_CODE", Path: "ResultCode"},
		},
	}

	_, err := executor.Execute(context.Background(), sc, prevResult, step)

	require.NoError(t, err)
	// ccaToMap stores numeric values as int64 for ruleevaluator type compatibility.
	assert.Equal(t, int64(4012), sc.Vars["PREV_RESULT_CODE"], "extracted value must be in sc.Vars")
}

func TestStepExecutor_Extraction_PathMissing_DoesNotError(t *testing.T) {
	// Non-existent paths are silently skipped; the variable is left unchanged.
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	sc.Vars["MY_VAR"] = "original"
	executor := newTestExecutor()

	prevCCA := &messaging.CCA{FUIAction: -1}
	prevResult := &SendResult{CCA: prevCCA}

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Extractions: []Extraction{
			{Name: "MY_VAR", Path: "NonExistentField.Deep.Path"},
		},
	}

	_, err := executor.Execute(context.Background(), sc, prevResult, step)

	require.NoError(t, err)
	assert.Equal(t, "original", sc.Vars["MY_VAR"], "variable should be unchanged when path is not found")
}

func TestStepExecutor_Extraction_NilPrevResult_NoExtractionApplied(t *testing.T) {
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Extractions: []Extraction{
			{Name: "SHOULD_NOT_SET", Path: "ResultCode"},
		},
	}

	_, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	_, ok := sc.Vars["SHOULD_NOT_SET"]
	assert.False(t, ok, "extraction should not run when prevResult is nil")
}

// — Derived value tests —

func TestStepExecutor_DerivedValue_ComputedAndStoredInVars(t *testing.T) {
	// AC: derived value expression referencing an extracted variable is evaluated
	// and the result substituted (stored in sc.Vars for use in subsequent steps).
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	// Simulate that "GRANTED" was extracted from a previous step
	sc.Vars["GRANTED"] = int64(1000)

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "UPDATE",
		DerivedValues: []DerivedValue{
			// Expression: GRANTED > 500 → bool (demonstrates derived value from context)
			{Name: "HAS_SUFFICIENT_GRANT", Expression: "GRANTED > 500"},
		},
	}

	_, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.Equal(t, true, sc.Vars["HAS_SUFFICIENT_GRANT"])
}

// — Assertion tests —

func TestStepExecutor_Assertion_Pass(t *testing.T) {
	// AC: assertions evaluated against the CCA produce per-assertion AssertionResult entries.
	fakeCCA := buildCCA(2001)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	// After the send, autoUpdateVarsFromCCA sets RESULT_CODE = 2001
	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Assertions:  []string{"RESULT_CODE == 2001"},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	require.Len(t, result.Assertions, 1)
	assert.True(t, result.Assertions[0].Passed)
	assert.Equal(t, "RESULT_CODE == 2001", result.Assertions[0].Expression)
}

func TestStepExecutor_Assertion_Fail(t *testing.T) {
	fakeCCA := buildCCA(4012)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Assertions:  []string{"RESULT_CODE == 2001"},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	require.Len(t, result.Assertions, 1)
	assert.False(t, result.Assertions[0].Passed)
	assert.NotEmpty(t, result.Assertions[0].Message)
}

func TestStepExecutor_MultipleAssertions_AllEvaluated(t *testing.T) {
	fakeCCA := buildCCA(2001)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		Assertions: []string{
			"RESULT_CODE == 2001",
			"RESULT_CODE != 4012",
			"RESULT_CODE == 9999", // will fail
		},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	require.Len(t, result.Assertions, 3)
	assert.True(t, result.Assertions[0].Passed)
	assert.True(t, result.Assertions[1].Passed)
	assert.False(t, result.Assertions[2].Passed)
}

// — Result code handler tests —

func TestStepExecutor_ResultHandler_Terminate_ReturnsActionTerminate(t *testing.T) {
	// AC: CCA with result code matching handler action "terminate" → ActionTerminate.
	fakeCCA := buildCCA(4012)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		ResultHandlers: []ResultHandler{
			{When: "RESULT_CODE == 4012", Action: "terminate"},
		},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.Equal(t, ActionTerminate, result.ResultCodeAction)
}

func TestStepExecutor_ResultHandler_NoMatch_ReturnsActionContinue(t *testing.T) {
	// AC: no handler matches → ActionContinue (default).
	fakeCCA := buildCCA(2001)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		ResultHandlers: []ResultHandler{
			{When: "RESULT_CODE == 4012", Action: "terminate"},
		},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.Equal(t, ActionContinue, result.ResultCodeAction)
}

func TestStepExecutor_ResultHandler_FirstMatchWins(t *testing.T) {
	fakeCCA := buildCCA(5001)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "INITIAL",
		ResultHandlers: []ResultHandler{
			{When: "RESULT_CODE == 5001", Action: "pause"},
			{When: "RESULT_CODE == 5001", Action: "terminate"}, // shadowed by first match
		},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.Equal(t, ActionPause, result.ResultCodeAction)
}

func TestStepExecutor_NoHandlers_ReturnsActionContinue(t *testing.T) {
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.Equal(t, ActionContinue, result.ResultCodeAction)
}

// — CCRequestNumber increment test —

func TestStepExecutor_IncrementsccRequestNumber(t *testing.T) {
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	assert.Equal(t, uint32(0), sc.CCRequestNumber)

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	_, err := executor.Execute(context.Background(), sc, nil, step)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), sc.CCRequestNumber)

	_, err = executor.Execute(context.Background(), sc, nil, step)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), sc.CCRequestNumber)
}

func TestStepExecutor_CCRequestNumber_SyncedToVars(t *testing.T) {
	sender := &fakeSender{cca: buildCCA(2001)}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	_, err := executor.Execute(context.Background(), sc, nil, step)
	require.NoError(t, err)

	assert.Equal(t, sc.CCRequestNumber, sc.Vars["CC_REQUEST_NUMBER"])
}

// — Fake sender produces identical StepResult shape as real sender —

func TestStepExecutor_FakeSender_IdenticalStepResultShape(t *testing.T) {
	// AC: fake messaging.Sender (test implementation) produces identical StepResult
	// shape — the Sender interface abstraction is the only difference.
	fakeCCA := buildCCA(4010)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{
		Kind:        "request",
		RequestType: "UPDATE",
		Assertions:  []string{"RESULT_CODE == 4010"},
		ResultHandlers: []ResultHandler{
			{When: "RESULT_CODE == 4010", Action: "retry"},
		},
	}

	result, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	assert.False(t, result.Skipped)
	require.NotNil(t, result.SendResult.CCA)
	assert.Equal(t, uint32(4010), result.SendResult.CCA.ResultCode)
	require.Len(t, result.Assertions, 1)
	assert.True(t, result.Assertions[0].Passed)
	assert.Equal(t, ActionRetry, result.ResultCodeAction)
}

// — Send error propagation —

func TestStepExecutor_SendError_ReturnsError(t *testing.T) {
	sender := &fakeSender{err: errors.New("peer disconnected")}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	_, err := executor.Execute(context.Background(), sc, nil, step)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send CCR")
	// Metrics should be recorded even on error
	assert.Len(t, sc.AllMetrics(), 1)
	assert.NotNil(t, sc.AllMetrics()[0].Err)
}

// — Auto-update vars from CCA —

func TestStepExecutor_AutoUpdateVars_SetsResultCode(t *testing.T) {
	fakeCCA := buildCCA(5003)
	sender := &fakeSender{cca: fakeCCA}
	sc := newTestContext(sender)
	executor := newTestExecutor()

	step := ScenarioStep{Kind: "request", RequestType: "INITIAL"}
	_, err := executor.Execute(context.Background(), sc, nil, step)

	require.NoError(t, err)
	// RESULT_CODE is stored as int64 so ruleevaluator can compare it with
	// integer literals (which parse as int64) without a type-mismatch error.
	assert.Equal(t, int64(5003), sc.Vars["RESULT_CODE"])
}

// — ccaToMap helper tests —

func TestCCAToMap_NilCCA(t *testing.T) {
	m := ccaToMap(nil)
	assert.Empty(t, m)
}

func TestCCAToMap_Fields(t *testing.T) {
	cca := &messaging.CCA{
		SessionID:  "host;123;456",
		ResultCode: 2001,
		OriginHost: "ocs.example.com",
		FUIAction:  -1,
	}
	m := ccaToMap(cca)
	assert.Equal(t, "host;123;456", m["SessionID"])
	// All numerics stored as int64 for ruleevaluator compatibility.
	assert.Equal(t, int64(2001), m["ResultCode"])
	assert.Equal(t, "ocs.example.com", m["OriginHost"])
	assert.Equal(t, int64(-1), m["FUIAction"])
}

func TestCCAToMap_MSCCSlice(t *testing.T) {
	cca := &messaging.CCA{
		FUIAction: -1,
		MSCC: []messaging.MSCCBlock{
			{RatingGroup: 10, GrantedTime: 3600, ResultCode: 2001},
			{RatingGroup: 20, GrantedTotalOctets: 1024 * 1024},
		},
	}
	m := ccaToMap(cca)
	msccList, ok := m["MSCC"].([]map[string]any)
	require.True(t, ok, "MSCC must be a []map[string]any")
	require.Len(t, msccList, 2)
	assert.Equal(t, int64(10), msccList[0]["RatingGroup"])
	assert.Equal(t, int64(3600), msccList[0]["GrantedTime"])
}

// — isTruthy edge cases —

func TestIsTruthy(t *testing.T) {
	cases := []struct {
		v    any
		want bool
	}{
		{nil, false},
		{false, false},
		{true, true},
		{int(0), false},
		{int(1), true},
		{int64(0), false},
		{int64(-1), true},
		{float64(0), false},
		{float64(0.1), true},
		{"", false},
		{"x", true},
		{struct{}{}, true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isTruthy(tc.v), "isTruthy(%T(%v))", tc.v, tc.v)
	}
}
