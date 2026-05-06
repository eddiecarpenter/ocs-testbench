package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/eddiecarpenter/ruleevaluator"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// templateBracesRe matches {{VARNAME}} tokens used in AVP value fields.
// Expressions in repeatUntil / assertions / guards may use either notation;
// stripping braces before evaluation makes both forms equivalent.
var templateBracesRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// stringLiteralRe matches single- or double-quoted string literals so that
// identifiers inside them are not mistaken for variable references.
var stringLiteralRe = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// identRe extracts word tokens that could be variable names.
var identRe = regexp.MustCompile(`\b([A-Za-z_]\w*)\b`)

// exprKeywords are tokens that ruleevaluator treats as operators or literals,
// not as variable lookups. They are excluded from unknown-variable checks.
var exprKeywords = map[string]bool{
	"true": true, "false": true, "null": true, "nil": true,
	"is": true, "not": true, "and": true, "or": true,
}

// systemVars is the fixed set of variables the engine auto-provisions into
// every session context. Names here are never flagged as unknown.
var systemVars = map[string]bool{
	"SESSION_ID":        true,
	"MSISDN":            true,
	"CC_REQUEST_NUMBER": true,
	"RESULT_CODE":       true,
	"FUI_ACTION":        true,
	"TOTAL_USU":         true,
}

// toInt64 coerces common numeric types to int64 for arithmetic.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	}
	return 0
}

// rgVarRe matches per-rating-group variables (RG1_GRANTED, RG2_VALIDITY, …).
var rgVarRe = regexp.MustCompile(`^RG\d+_(GRANTED|GRANTED_OCTETS|VALIDITY|RESULT_CODE|FUI_ACTION)$`)

// isKnownVar reports whether name is a declared or system-provided variable.
func isKnownVar(name string, declared map[string]bool) bool {
	return declared[name] || systemVars[name] || rgVarRe.MatchString(name)
}

// unknownVars returns the set of identifier tokens in expr that are not
// present in vars and are not operator/literal keywords.
// expr must already have {{}} notation stripped.
func unknownVars(vars map[string]any, expr string) []string {
	// Blank out string literals so we don't match identifiers inside them.
	noStrings := stringLiteralRe.ReplaceAllString(expr, "''")
	var missing []string
	seen := map[string]bool{}
	for _, m := range identRe.FindAllString(noStrings, -1) {
		if seen[m] || exprKeywords[strings.ToLower(m)] {
			continue
		}
		seen[m] = true
		if _, ok := vars[m]; !ok {
			missing = append(missing, m)
		}
	}
	return missing
}

// ValidateExpr checks that every variable referenced in expr is listed in
// declaredVars or is a well-known system variable. Both {{VAR}} and bare VAR
// notation are accepted. Returns an error naming the unknown identifiers.
func ValidateExpr(declaredVars []string, expr string) error {
	if expr == "" {
		return nil
	}
	stripped := templateBracesRe.ReplaceAllString(expr, "$1")
	noStrings := stringLiteralRe.ReplaceAllString(stripped, "''")

	known := make(map[string]bool, len(declaredVars))
	for _, v := range declaredVars {
		known[v] = true
	}

	var bad []string
	seen := map[string]bool{}
	for _, m := range identRe.FindAllString(noStrings, -1) {
		if seen[m] || exprKeywords[strings.ToLower(m)] {
			continue
		}
		seen[m] = true
		if !isKnownVar(m, known) {
			bad = append(bad, m)
		}
	}
	if len(bad) != 0 {
		return fmt.Errorf("unknown variable(s): %s", strings.Join(bad, ", "))
	}
	return nil
}

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
//     9b/c/d. Built-in checks — goto_terminate on non-2xxx, all-MSCC-exhausted, FUI=TERMINATE.
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
	// Derive the request type first so it can be injected into the value map
	// before rendering — the engine uses CC_REQUEST_TYPE to apply §7 RSU/USU
	// presence rules (INITIAL→no USU, TERMINATE→no RSU, etc.).
	ccReqType := mapRequestType(step.RequestType)

	// Merge sc.Vars with step-level overrides for this send only.
	mergedVars := mergeVars(sc.Vars, step.Overrides)
	mergedVars["CC_REQUEST_TYPE"] = ccReqType

	input := e.baseInput
	input.Values = mergedVars

	avps, err := e.templateEngine.Render(ctx, input)
	if err != nil {
		return StepResult{}, fmt.Errorf("step executor: render CCR: %w", err)
	}
	req := &messaging.CCR{
		SessionID:        sc.SessionID,
		CCRequestType:    ccReqType,
		CCRequestNumber:  sc.CCRequestNumber,
		ServiceContextID: sc.ServiceContextID,
		ExtraAVPs:        avps,
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

	// Accumulate used service units into TOTAL_USU after each UPDATE or
	// TERMINATE so repeatUntil / assertions can reference the running total.
	if step.RequestType == "UPDATE" || step.RequestType == "TERMINATE" {
		usu := toInt64(mergedVars["USU_TOTAL"])
		prev, _ := sc.Vars["TOTAL_USU"].(int64)
		sc.Vars["TOTAL_USU"] = prev + usu
	}

	// — Step 8: Assertions —
	assertions := e.evaluateAssertions(sc.Vars, step.Assertions)

	// — Step 9: Result code handlers —
	action := e.evaluateResultHandlers(sc.Vars, step.ResultHandlers)

	// — Steps 9b/9c: Built-in termination checks —
	// These fire only when no scenario result handler has overridden the action.
	// All cases use ActionGotoTerminate so the session ends with a proper
	// CCR-T (the last step) rather than dropping the Diameter session cold.
	if action == ActionContinue && result.CCA != nil {
		// 9b: Root-level non-success result code.
		// A 4xxx or 5xxx code means the OCS rejected the request; reporting
		// usage after this is a protocol violation (e.g. 5012 USED_MORE_THAN_GRANTED).
		rc := result.CCA.ResultCode
		if rc != 0 && (rc < 2000 || rc > 2999) {
			action = ActionGotoTerminate
		}

		// 9c: All MSCC blocks denied or quota-exhausted (FUI=TERMINATE).
		// When every rating group the OCS responded to has either a non-2xxx
		// result code or FUI=TERMINATE, there is no grant left to continue with.
		if action == ActionContinue && isAllMSCCExhausted(result.CCA) {
			action = ActionGotoTerminate
		}

		// 9d: Root-level FUI=TERMINATE.
		// The OCS signals the last quota grant; the session must terminate
		// per RFC 4006 §5.6 once that quota is exhausted.
		if action == ActionContinue && isFUITerminate(result.CCA) {
			action = ActionGotoTerminate
		}
	}

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
// {{VARNAME}} tokens are stripped to bare VARNAME before evaluation so
// both the template-style {{VAR}} notation and bare VAR notation work.
func evalExpr(vars map[string]any, expr string) (any, error) {
	expr = templateBracesRe.ReplaceAllString(expr, "$1")
	ev := ruleevaluator.NewRuleEvaluator(vars)
	return ev.Evaluate(expr)
}

// EvalExprPublic is the exported entry point for the API expression-evaluate
// endpoint. It applies the same {{}} normalisation as the execution engine and
// additionally rejects expressions that reference variables not present in vars,
// catching typos before they silently evaluate to nil.
func EvalExprPublic(vars map[string]any, expr string) (any, error) {
	stripped := templateBracesRe.ReplaceAllString(expr, "$1")
	if missing := unknownVars(vars, stripped); len(missing) != 0 {
		// Also allow system vars and RGn_* that the caller may not have seeded.
		var reallyMissing []string
		for _, m := range missing {
			if !isKnownVar(m, nil) {
				reallyMissing = append(reallyMissing, m)
			}
		}
		if len(reallyMissing) != 0 {
			return nil, fmt.Errorf("unknown variable(s): %s", strings.Join(reallyMissing, ", "))
		}
	}
	ev := ruleevaluator.NewRuleEvaluator(vars)
	return ev.Evaluate(stripped)
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
	case "goto_terminate":
		return ActionGotoTerminate
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
	// Root-level FUI_ACTION: -1 means no FUI present in the CCA.
	vars["FUI_ACTION"] = int64(cca.FUIAction)

	// Zero out all previously-set RG variables before re-populating.
	// This ensures that when a CCA omits a rating group, its variables
	// reset to 0 rather than carrying stale values into the next CCR.
	for k := range vars {
		if strings.HasPrefix(k, "RG") &&
			(strings.HasSuffix(k, "_GRANTED") ||
				strings.HasSuffix(k, "_GRANTED_OCTETS") ||
				strings.HasSuffix(k, "_VALIDITY") ||
				strings.HasSuffix(k, "_RESULT_CODE") ||
				strings.HasSuffix(k, "_FUI_ACTION")) {
			vars[k] = int64(0)
		}
	}

	// Per-MSCC auto-provisioned variables.
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
		// Per-MSCC result code: 0 means absent (OCS omitted it).
		vars[prefix+"_RESULT_CODE"] = int64(block.ResultCode)
		// Per-MSCC FUI action: -1 means no FUI present in this block.
		vars[prefix+"_FUI_ACTION"] = int64(block.FUIAction)
	}
}

// isFUITerminate reports whether the CCA carries a root-level
// Final-Unit-Action=TERMINATE (0). Per RFC 4006 §5.6 the session must
// terminate once the granted quota is exhausted.
func isFUITerminate(cca *messaging.CCA) bool {
	return cca.FUIAction == messaging.FUIActionTerminate
}

// isAllMSCCExhausted reports whether every MSCC block the OCS returned has
// either a non-2xxx result code (quota denied) or FUI=TERMINATE (last grant).
// Returns false when the CCA contains no MSCC blocks (root service model).
func isAllMSCCExhausted(cca *messaging.CCA) bool {
	if len(cca.MSCC) == 0 {
		return false
	}
	for _, block := range cca.MSCC {
		rc := block.ResultCode
		rcFailed := rc != 0 && (rc < 2000 || rc > 2999)
		fuiTerminate := block.FUIAction == messaging.FUIActionTerminate
		if !rcFailed && !fuiTerminate {
			return false // at least one block is still active
		}
	}
	return true
}
