package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eddiecarpenter/ruleevaluator"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// StepExecutor is a stateless processor that takes a single step definition
// plus a session context and produces a StepResult.
//
// Instantiated once per scenario execution with the scenario-level template
// input; each Execute call overrides only the Values field per step.
//
// StepExecutor is NOT goroutine-safe for concurrent Execute calls on the same
// *SessionContext (the context is inherently single-threaded per execution).
type StepExecutor struct {
	templateEngine *template.Engine
	// baseInput carries the scenario-level template data (Tree, MSCC,
	// Dictionary, UnitType, ServiceModel). Values is overridden per step.
	baseInput template.EngineInput
}

// NewStepExecutor creates a StepExecutor with the given template engine and
// scenario-level template input. baseInput.Values is ignored (overridden at
// execute time from the session context).
func NewStepExecutor(engine *template.Engine, baseInput template.EngineInput) *StepExecutor {
	return &StepExecutor{
		templateEngine: engine,
		baseInput:      baseInput,
	}
}

// Execute processes a single step and returns a StepResult.
//
// Execution order per step:
//  1. Guards — evaluate against sc.Vars; skip if any is false.
//  2. Extractions — apply step.Extractions from prevResult.CCA into sc.Vars.
//  3. Derived values — evaluate step.DerivedValues expressions into sc.Vars.
//  4. Build CCR — render AVP tree via template engine with merged vars.
//  5. Send — call sc.Send(ctx, req).
//  6. Record metrics — call sc.RecordStep(result).
//  7. Auto-update sc.Vars from the CCA (RESULT_CODE, MSCC values).
//  8. Assert — evaluate step.Assertions against updated sc.Vars.
//  9. Result code handlers — return first matching action.
//  10. Increment CCRequestNumber and update CC_REQUEST_NUMBER in sc.Vars.
func (e *StepExecutor) Execute(
	ctx context.Context,
	sc *SessionContext,
	prevResult *SendResult,
	step ScenarioStep,
) (StepResult, error) {
	// — Step 1: Guard evaluation —
	skip, err := e.evaluateGuards(sc.Vars, step.Guards)
	if err != nil {
		return StepResult{}, fmt.Errorf("step executor: evaluate guard: %w", err)
	}
	if skip {
		return StepResult{Skipped: true}, nil
	}

	// — Step 2: Extractions from previous CCA —
	if prevResult != nil && prevResult.CCA != nil {
		if err := e.applyExtractions(sc, prevResult.CCA, step.Extractions); err != nil {
			return StepResult{}, fmt.Errorf("step executor: apply extractions: %w", err)
		}
	}

	// — Step 3: Derived values —
	if err := e.applyDerivedValues(sc, step.DerivedValues); err != nil {
		return StepResult{}, fmt.Errorf("step executor: apply derived values: %w", err)
	}

	// — Step 4: Build CCR via template engine —
	// Merge sc.Vars with step-level overrides for this send only.
	mergedVars := mergeVars(sc.Vars, step.Overrides)

	input := e.baseInput
	input.Values = mergedVars

	avps, err := e.templateEngine.Render(ctx, input)
	if err != nil {
		return StepResult{}, fmt.Errorf("step executor: render CCR: %w", err)
	}

	ccReqType := mapRequestType(step.RequestType)
	req := &messaging.CCR{
		SessionID:       sc.SessionID,
		CCRequestType:   ccReqType,
		CCRequestNumber: sc.CCRequestNumber,
		ExtraAVPs:       avps,
	}

	// — Step 5: Send —
	result, sendErr := sc.Send(ctx, req)

	// — Step 6: Record metrics —
	// Always record, even on error, so failure-rate metrics are accurate.
	sc.RecordStep(result)

	if sendErr != nil {
		return StepResult{}, fmt.Errorf("step executor: send CCR: %w", sendErr)
	}

	// — Step 7: Auto-update sc.Vars from the CCA —
	if result.CCA != nil {
		autoUpdateVarsFromCCA(sc.Vars, result.CCA)
	}

	// — Step 8: Assertions —
	assertions := e.evaluateAssertions(sc.Vars, step.Assertions)

	// — Step 9: Result code handlers —
	action := e.evaluateResultHandlers(sc.Vars, step.ResultHandlers)

	// — Step 10: Increment CCRequestNumber —
	sc.CCRequestNumber++
	sc.Vars["CC_REQUEST_NUMBER"] = sc.CCRequestNumber

	return StepResult{
		Skipped:          false,
		SendResult:       result,
		Assertions:       assertions,
		ResultCodeAction: action,
	}, nil
}

// — Helpers —

// evaluateGuards evaluates all guard expressions against vars. Returns true
// (skip) if any guard expression evaluates to a falsy value. Returns false
// (proceed) when all guards pass or the list is empty.
func (e *StepExecutor) evaluateGuards(vars map[string]any, guards []string) (bool, error) {
	for _, expr := range guards {
		result, err := evalExpr(vars, expr)
		if err != nil {
			return false, fmt.Errorf("guard %q: %w", expr, err)
		}
		if !isTruthy(result) {
			return true, nil // skip this step
		}
	}
	return false, nil
}

// applyExtractions resolves each extraction's dot-path against the CCA map
// and writes the result into sc.Vars.
func (e *StepExecutor) applyExtractions(sc *SessionContext, cca *messaging.CCA, extractions []Extraction) error {
	if len(extractions) == 0 {
		return nil
	}
	ccaMap := ccaToMap(cca)
	ev := ruleevaluator.NewRuleEvaluator(ccaMap)
	for _, ex := range extractions {
		val, err := ev.Evaluate(ex.Path)
		if err != nil || val == nil {
			// Non-fatal: missing path or nil result → leave the variable
			// unchanged. ARCHITECTURE.md §5: "if a later CCA lacks the
			// path, the previous value persists."
			continue
		}
		sc.Vars[ex.Name] = val
	}
	return nil
}

// applyDerivedValues evaluates each derived value expression against sc.Vars
// and writes the result back into sc.Vars.
func (e *StepExecutor) applyDerivedValues(sc *SessionContext, derivedValues []DerivedValue) error {
	for _, dv := range derivedValues {
		result, err := evalExpr(sc.Vars, dv.Expression)
		if err != nil {
			return fmt.Errorf("derived value %q (expr %q): %w", dv.Name, dv.Expression, err)
		}
		sc.Vars[dv.Name] = result
	}
	return nil
}

// evaluateAssertions evaluates each assertion expression against vars and
// returns an ordered list of AssertionResult.
func (e *StepExecutor) evaluateAssertions(vars map[string]any, assertions []string) []AssertionResult {
	if len(assertions) == 0 {
		return nil
	}
	results := make([]AssertionResult, 0, len(assertions))
	for _, expr := range assertions {
		result, err := evalExpr(vars, expr)
		var ar AssertionResult
		ar.Expression = expr
		if err != nil {
			ar.Passed = false
			ar.Message = fmt.Sprintf("evaluation error: %v", err)
		} else if isTruthy(result) {
			ar.Passed = true
		} else {
			ar.Passed = false
			ar.Message = fmt.Sprintf("assertion failed: %q evaluated to %v", expr, result)
		}
		results = append(results, ar)
	}
	return results
}

// evaluateResultHandlers walks handlers in order and returns the action of
// the first handler whose When expression evaluates to truthy. Returns
// ActionContinue when no handler matches or the list is empty.
func (e *StepExecutor) evaluateResultHandlers(vars map[string]any, handlers []ResultHandler) ResultCodeAction {
	for _, h := range handlers {
		result, err := evalExpr(vars, h.When)
		if err != nil {
			continue // evaluation error — skip handler
		}
		if isTruthy(result) {
			return parseAction(h.Action)
		}
	}
	return ActionContinue
}

// — Package-level utilities —

// evalExpr evaluates a ruleevaluator expression against a variable map.
func evalExpr(vars map[string]any, expr string) (any, error) {
	ev := ruleevaluator.NewRuleEvaluator(vars)
	return ev.Evaluate(expr)
}

// isTruthy converts a ruleevaluator result to a Go boolean. Any non-nil,
// non-zero, non-false, non-empty-string value is considered truthy.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	case string:
		return val != ""
	}
	return true
}

// mergeVars returns a new map with sc.Vars values overlaid by step overrides.
// The original sc.Vars map is not modified.
func mergeVars(base map[string]any, overrides map[string]string) map[string]any {
	merged := make(map[string]any, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// mapRequestType converts a step requestType string to the Diameter
// CC-Request-Type AVP value (RFC 4006 §8.3).
func mapRequestType(s string) uint32 {
	switch strings.ToUpper(s) {
	case "INITIAL":
		return messaging.CCRTypeInitial
	case "UPDATE":
		return messaging.CCRTypeUpdate
	case "TERMINATE":
		return messaging.CCRTypeTerminate
	case "EVENT":
		return messaging.CCRTypeEvent
	default:
		return messaging.CCRTypeInitial
	}
}

// parseAction converts a result handler action string to a ResultCodeAction.
func parseAction(s string) ResultCodeAction {
	switch strings.ToLower(s) {
	case "terminate":
		return ActionTerminate
	case "retry":
		return ActionRetry
	case "pause":
		return ActionPause
	case "stop":
		return ActionStop
	default:
		return ActionContinue
	}
}

// ccaToMap converts a *messaging.CCA to a flat map[string]any suitable for
// dot-path extraction via ruleevaluator. Uses encoding/json round-trip as the
// fallback (per design plan) after an explicit field projection pass.
//
// Fields are exported under their Go names (e.g. "ResultCode", "SessionID").
// MSCC blocks are exported as a slice under the "MSCC" key.
func ccaToMap(cca *messaging.CCA) map[string]any {
	if cca == nil {
		return map[string]any{}
	}

	// Preferred: explicit projection — gives stable, predictable key names
	// without depending on json struct tags or reflection.
	//
	// All numeric values are stored as int64 so ruleevaluator can compare
	// them against integer literals (parsed as int64) without type errors.
	m := map[string]any{
		"SessionID":         cca.SessionID,
		"OriginHost":        cca.OriginHost,
		"OriginRealm":       cca.OriginRealm,
		"AuthApplicationID": int64(cca.AuthApplicationID),
		"ResultCode":        int64(cca.ResultCode),
		"CCRequestType":     int64(cca.CCRequestType),
		"CCRequestNumber":   int64(cca.CCRequestNumber),
		"ValidityTime":      int64(cca.ValidityTime),
		"FUIAction":         int64(cca.FUIAction),
	}

	msccList := make([]map[string]any, len(cca.MSCC))
	for i, block := range cca.MSCC {
		msccList[i] = map[string]any{
			"ServiceIdentifier":  int64(block.ServiceIdentifier),
			"RatingGroup":        int64(block.RatingGroup),
			"ResultCode":         int64(block.ResultCode),
			"GrantedTime":        int64(block.GrantedTime),
			"GrantedTotalOctets": int64(block.GrantedTotalOctets),
			"ValidityTime":       int64(block.ValidityTime),
			"FUIAction":          int64(block.FUIAction),
		}
	}
	m["MSCC"] = msccList

	return m
}

// ccaToMapFallback is the encoding/json round-trip fallback used when the
// explicit projection in ccaToMap is insufficient (e.g. for vendor AVPs or
// fields added in future CCA versions). Not called in production — reserved
// for callers that need a full JSON representation of the CCA struct fields.
func ccaToMapFallback(cca *messaging.CCA) map[string]any {
	type ccaJSON struct {
		SessionID         string `json:"SessionID"`
		OriginHost        string `json:"OriginHost"`
		OriginRealm       string `json:"OriginRealm"`
		AuthApplicationID uint32 `json:"AuthApplicationID"`
		ResultCode        uint32 `json:"ResultCode"`
		CCRequestType     uint32 `json:"CCRequestType"`
		CCRequestNumber   uint32 `json:"CCRequestNumber"`
		ValidityTime      uint32 `json:"ValidityTime"`
		FUIAction         int32  `json:"FUIAction"`
	}
	proxy := ccaJSON{
		SessionID:         cca.SessionID,
		OriginHost:        cca.OriginHost,
		OriginRealm:       cca.OriginRealm,
		AuthApplicationID: cca.AuthApplicationID,
		ResultCode:        cca.ResultCode,
		CCRequestType:     cca.CCRequestType,
		CCRequestNumber:   cca.CCRequestNumber,
		ValidityTime:      cca.ValidityTime,
		FUIAction:         cca.FUIAction,
	}
	b, err := json.Marshal(proxy)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

// autoUpdateVarsFromCCA writes auto-provisioned system variables from the CCA
// into the session context variable map after each send. This implements the
// implicit extractions described in ARCHITECTURE.md §10 (Level 2 evaluation).
//
// All numeric values are stored as int64 so that ruleevaluator can compare
// them against integer literals (which are parsed as int64) without a type
// mismatch error.
func autoUpdateVarsFromCCA(vars map[string]any, cca *messaging.CCA) {
	if cca == nil {
		return
	}
	vars["RESULT_CODE"] = int64(cca.ResultCode)
	vars["SESSION_ID"] = cca.SessionID
	// Per-MSCC auto-provisioned variables: RG<n>_GRANTED, RG<n>_VALIDITY.
	for _, block := range cca.MSCC {
		rg := block.RatingGroup
		prefix := fmt.Sprintf("RG%d", rg)
		vars[prefix+"_GRANTED"] = int64(block.GrantedTime)
		if block.GrantedTotalOctets > 0 {
			vars[prefix+"_GRANTED_OCTETS"] = int64(block.GrantedTotalOctets)
		}
		if block.ValidityTime > 0 {
			vars[prefix+"_VALIDITY"] = int64(block.ValidityTime)
		}
	}
}
