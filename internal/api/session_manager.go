package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/engine"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// — Scenario body JSON shapes (wire format stored in DB) —

type scenarioBodyExec struct {
	ServiceModel     string          `json:"serviceModel"`
	ServiceContextID string          `json:"serviceContextId"`
	ServiceType      string          `json:"serviceType"`
	ServiceProfile   string          `json:"serviceProfile"`
	AvpTree          json.RawMessage `json:"avpTree"`
	Services         json.RawMessage `json:"services"`
	Variables        json.RawMessage `json:"variables"`
	Steps            json.RawMessage `json:"steps"`
}

type avpNodeJSON struct {
	Name     string        `json:"name"`
	Code     uint32        `json:"code,omitempty"`
	VendorID int64         `json:"vendorId,omitempty"`
	ValueRef string        `json:"valueRef,omitempty"`
	Locked   bool          `json:"locked,omitempty"`
	Children []avpNodeJSON `json:"children,omitempty"`
}

type serviceJSON struct {
	ID                string `json:"id"`
	RatingGroup       string `json:"ratingGroup,omitempty"`
	ServiceIdentifier string `json:"serviceIdentifier,omitempty"`
	RequestedUnits    string `json:"requestedUnits"`
	UsedUnits         string `json:"usedUnits,omitempty"`
}

type variableJSON struct {
	Name   string          `json:"name"`
	Source variableSrcJSON `json:"source"`
}

type variableSrcJSON struct {
	Kind     string         `json:"kind"`
	From     string         `json:"from,omitempty"`
	Field    string         `json:"field,omitempty"`
	Strategy string         `json:"strategy,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// peerIdentityBody is the minimal subset of the peer body needed for
// extracting identity fields. The stored JSON uses camelCase flat structure.
type peerIdentityBody struct {
	OriginHost  string `json:"originHost"`
	OriginRealm string `json:"originRealm"`
}

// — Session record —

type stepHistoryRecord struct {
	n          int
	kind       string
	label      string
	state      string
	startedAt  string
	finishedAt string
	durationMs int64
}

type sessionRecord struct {
	id           string
	scenarioID   string
	scenarioName string
	mode         string
	startedAt    string
	sc           *engine.SessionContext
	orc          *engine.Orchestrator
	steps        []engine.ScenarioStep

	mu          sync.RWMutex
	state       engine.SessionState
	stepIdx     int
	prevResult  *engine.SendResult
	stepHistory []stepHistoryRecord

	cancel context.CancelFunc
	stopCh chan struct{}
	done   chan struct{}

	subsMu sync.Mutex
	subs   []chan ExecutionEvent
}

func (r *sessionRecord) broadcast(ev ExecutionEvent) {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()
	for _, ch := range r.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (r *sessionRecord) addSub(ch chan ExecutionEvent) {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()
	r.subs = append(r.subs, ch)
}

func (r *sessionRecord) removeSub(ch chan ExecutionEvent) {
	r.subsMu.Lock()
	defer r.subsMu.Unlock()
	out := r.subs[:0]
	for _, s := range r.subs {
		if s != ch {
			out = append(out, s)
		}
	}
	r.subs = out
}

// — SessionManager —

// SessionManager implements ExecutionEngine by bridging engine.Orchestrator
// with the store, Diameter sender, and template engine.
type SessionManager struct {
	store  store.Store
	sender messaging.Sender
	dict   template.Dictionary
	tmpl   *template.Engine

	mu       sync.Mutex
	sessions map[string]*sessionRecord
}

// NewSessionManager creates a SessionManager.
func NewSessionManager(s store.Store, sender messaging.Sender, dict template.Dictionary) *SessionManager {
	return &SessionManager{
		store:    s,
		sender:   sender,
		dict:     dict,
		tmpl:     template.NewEngine(),
		sessions: make(map[string]*sessionRecord),
	}
}

// Start implements ExecutionEngine.
func (m *SessionManager) Start(ctx context.Context, scenarioID string, mode string) (StartInfo, error) {
	scUID, err := parseScenarioUUID(scenarioID)
	if err != nil {
		return StartInfo{}, ErrScenarioNotFound
	}

	scRow, err := m.store.GetScenario(ctx, scUID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return StartInfo{}, ErrScenarioNotFound
		}
		return StartInfo{}, fmt.Errorf("session manager: load scenario: %w", err)
	}

	var body scenarioBodyExec
	if err := json.Unmarshal(scRow.Body, &body); err != nil {
		return StartInfo{}, fmt.Errorf("session manager: parse scenario body: %w", err)
	}

	var avpNodes []avpNodeJSON
	if err := json.Unmarshal(body.AvpTree, &avpNodes); err != nil {
		return StartInfo{}, fmt.Errorf("session manager: parse avpTree: %w", err)
	}

	var services []serviceJSON
	if err := json.Unmarshal(body.Services, &services); err != nil {
		return StartInfo{}, fmt.Errorf("session manager: parse services: %w", err)
	}

	var variables []variableJSON
	if err := json.Unmarshal(body.Variables, &variables); err != nil {
		return StartInfo{}, fmt.Errorf("session manager: parse variables: %w", err)
	}

	var steps []engine.ScenarioStep
	if err := json.Unmarshal(body.Steps, &steps); err != nil {
		return StartInfo{}, fmt.Errorf("session manager: parse steps: %w", err)
	}

	peerName, originHost, originRealm, err := m.loadPeerInfo(ctx, scRow.PeerID)
	if err != nil {
		return StartInfo{}, fmt.Errorf("session manager: load peer: %w", err)
	}

	var sub *store.Subscriber
	if scRow.SubscriberID.Valid {
		row, err := m.store.GetSubscriber(ctx, scRow.SubscriberID)
		if err == nil {
			sub = &row
		}
	}

	values := smResolveVariables(variables, sub, originHost, originRealm)

	baseInput := template.EngineInput{
		Tree:         smConvertAvpTree(avpNodes),
		MSCC:         smConvertServices(services, values),
		Dictionary:   m.dict,
		UnitType:     template.UnitType(smDeriveUnitType(body.ServiceType)),
		ServiceModel: template.ServiceModel(body.ServiceModel),
	}

	execMode := engine.ModeInteractive
	if mode == "continuous" {
		execMode = engine.ModeContinuous
	}

	sc := engine.NewSessionContext(peerName, originHost, m.sender, execMode)
	svcCtxID := body.ServiceContextID
	if svcCtxID == "" {
		svcCtxID = "32251@3gpp.org"
	}
	sc.ServiceContextID = svcCtxID
	for k, v := range values {
		sc.Vars[k] = v
	}
	sc.Vars["SESSION_ID"] = sc.SessionID
	sc.Vars["SERVICE_CONTEXT_ID"] = svcCtxID

	stepExec := engine.NewStepExecutor(m.tmpl, baseInput)
	orc := engine.NewOrchestrator(stepExec)

	sessionID := uuid.New().String()
	stopCh := make(chan struct{})
	runCtx, cancel := context.WithCancel(context.Background())

	// Interactive sessions start paused so the UI shows step controls
	// immediately. Continuous sessions start active (running).
	initialState := engine.StateActive
	if execMode == engine.ModeInteractive {
		initialState = engine.StatePaused
	}

	rec := &sessionRecord{
		id:           sessionID,
		scenarioID:   scenarioID,
		scenarioName: scRow.Name,
		mode:         mode,
		startedAt:    time.Now().UTC().Format(time.RFC3339),
		sc:           sc,
		orc:          orc,
		steps:        steps,
		state:        initialState,
		cancel:       cancel,
		stopCh:       stopCh,
		done:         make(chan struct{}),
	}

	m.mu.Lock()
	m.sessions[sessionID] = rec
	m.mu.Unlock()

	if execMode == engine.ModeContinuous {
		go m.runContinuous(runCtx, rec)
	} else {
		close(rec.done)
		cancel()
	}

	return StartInfo{SessionID: sessionID, ScenarioName: scRow.Name}, nil
}

func (m *SessionManager) runContinuous(ctx context.Context, rec *sessionRecord) {
	defer close(rec.done)

	opts := engine.RunOptions{StopCh: rec.stopCh}
	err := rec.orc.RunContinuous(ctx, rec.sc, rec.steps, opts)

	rec.mu.Lock()
	if err != nil && !errors.Is(err, context.Canceled) {
		rec.state = engine.StateError
	} else {
		rec.state = engine.StateCompleted
	}
	rec.mu.Unlock()

	rec.broadcast(ExecutionEvent{
		Type:      "completed",
		SessionID: rec.id,
		State:     rec.sc.State.String(),
		Metrics:   smMetricsFromSummary(rec.sc.Summary()),
	})
}

// List implements ExecutionEngine.
func (m *SessionManager) List(_ context.Context) []ExecutionSummary {
	m.mu.Lock()
	out := make([]ExecutionSummary, 0, len(m.sessions))
	for _, rec := range m.sessions {
		rec.mu.RLock()
		out = append(out, ExecutionSummary{
			SessionID:    rec.id,
			ScenarioID:   rec.scenarioID,
			ScenarioName: rec.scenarioName,
			Mode:         rec.mode,
			State:        smMapState(rec.state),
			StartedAt:    rec.startedAt,
		})
		rec.mu.RUnlock()
	}
	m.mu.Unlock()
	return out
}

// Stop implements ExecutionEngine.
func (m *SessionManager) Stop(_ context.Context, sessionID string) error {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	select {
	case <-rec.stopCh:
	default:
		close(rec.stopCh)
	}
	rec.cancel()

	rec.mu.Lock()
	if rec.state == engine.StateActive || rec.state == engine.StatePaused {
		rec.state = engine.StateTerminated
	}
	rec.mu.Unlock()

	return nil
}

// Step implements ExecutionEngine (interactive mode only).
func (m *SessionManager) Step(ctx context.Context, sessionID string, overrides map[string]any) (ExecutionStepResult, error) {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return ExecutionStepResult{}, err
	}

	if rec.sc.Mode != engine.ModeInteractive {
		return ExecutionStepResult{}, ErrInvalidState
	}

	rec.mu.Lock()
	if rec.state != engine.StateActive && rec.state != engine.StatePaused {
		rec.mu.Unlock()
		return ExecutionStepResult{}, ErrInvalidState
	}
	if rec.stepIdx >= len(rec.steps) {
		rec.state = engine.StateCompleted
		rec.mu.Unlock()
		return ExecutionStepResult{}, ErrInvalidState
	}
	stepIdx := rec.stepIdx
	step := rec.steps[stepIdx]
	prev := rec.prevResult
	rec.mu.Unlock()

	if len(overrides) > 0 {
		strOverrides := make(map[string]string, len(overrides))
		for k, v := range overrides {
			strOverrides[k] = fmt.Sprintf("%v", v)
		}
		step.Overrides = strOverrides
	}

	yield, stepErr := rec.orc.RunStepWithPrev(ctx, rec.sc, stepIdx, rec.steps, prev)

	finishedAt := time.Now().UTC()
	rec.mu.Lock()
	rec.stepIdx++
	stepState := "success"
	if stepErr != nil {
		stepState = "error"
		rec.state = engine.StateError
	} else {
		rec.prevResult = &yield.Result.SendResult
		if !yield.Result.Skipped && !smAllPassed(yield.Result.Assertions) {
			stepState = "failure"
		}
		if yield.Result.Skipped {
			stepState = "skipped"
		}
		if rec.stepIdx >= len(rec.steps) {
			rec.state = engine.StateCompleted
		} else {
			// Stay paused between steps in interactive mode.
			rec.state = engine.StatePaused
		}
	}
	durationMs := int64(0)
	startTs := finishedAt.Add(-time.Duration(yield.Result.SendResult.Metrics.RTT))
	if !yield.Result.Skipped && stepErr == nil {
		durationMs = finishedAt.Sub(startTs).Milliseconds()
	}
	rec.stepHistory = append(rec.stepHistory, stepHistoryRecord{
		n:          stepIdx + 1,
		kind:       "request",
		state:      stepState,
		startedAt:  startTs.Format(time.RFC3339),
		finishedAt: finishedAt.Format(time.RFC3339),
		durationMs: durationMs,
	})
	rec.mu.Unlock()

	if stepErr != nil {
		return ExecutionStepResult{}, fmt.Errorf("session manager: step: %w", stepErr)
	}

	assertions := make([]AssertionOutcome, len(yield.Result.Assertions))
	for i, a := range yield.Result.Assertions {
		assertions[i] = AssertionOutcome{
			Expression: a.Expression,
			Passed:     a.Passed,
			Message:    a.Message,
		}
	}

	rec.broadcast(ExecutionEvent{
		Type:      "progress",
		SessionID: rec.id,
		State:     rec.sc.State.String(),
		Step:      stepIdx,
		Metrics:   smMetricsFromSummary(rec.sc.Summary()),
	})

	return ExecutionStepResult{
		StepIndex:        stepIdx,
		Skipped:          yield.Result.Skipped,
		ResultCode:       yield.Result.SendResult.Metrics.ResultCode,
		AssertionsPassed: smAllPassed(yield.Result.Assertions),
		Assertions:       assertions,
	}, nil
}

// Detail implements ExecutionEngine.
func (m *SessionManager) Detail(_ context.Context, sessionID string) (ExecutionDetailResponse, error) {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return ExecutionDetailResponse{}, err
	}

	rec.mu.RLock()
	state := rec.state
	stepIdx := rec.stepIdx
	history := make([]stepHistoryRecord, len(rec.stepHistory))
	copy(history, rec.stepHistory)
	rec.mu.RUnlock()

	steps := make([]StepRecordJSON, len(history))
	for i, h := range history {
		steps[i] = StepRecordJSON{
			N:          h.n,
			Kind:       h.kind,
			Label:      h.label,
			State:      h.state,
			StartedAt:  h.startedAt,
			FinishedAt: h.finishedAt,
			DurationMs: h.durationMs,
		}
	}

	ctx := smBuildContext(rec.sc.Vars)

	return ExecutionDetailResponse{
		ID:           sessionID,
		ScenarioID:   rec.scenarioID,
		ScenarioName: rec.scenarioName,
		Mode:         rec.mode,
		State:        smMapState(state),
		StartedAt:    rec.startedAt,
		CurrentStep:  stepIdx,
		TotalSteps:   len(rec.steps),
		Steps:        steps,
		Context:      ctx,
	}, nil
}

// Subscribe implements ExecutionEngine.
func (m *SessionManager) Subscribe(ctx context.Context, sessionID string) (<-chan ExecutionEvent, error) {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	ch := make(chan ExecutionEvent, 16)
	rec.addSub(ch)

	go func() {
		select {
		case <-ctx.Done():
		case <-rec.done:
		}
		rec.removeSub(ch)
		close(ch)
	}()

	return ch, nil
}

// — Private helpers —

func (m *SessionManager) getSession(sessionID string) (*sessionRecord, error) {
	m.mu.Lock()
	rec, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	return rec, nil
}

func (m *SessionManager) loadPeerInfo(ctx context.Context, peerID pgtype.UUID) (name, originHost, originRealm string, err error) {
	if !peerID.Valid {
		return "", "", "", fmt.Errorf("scenario has no peer assigned")
	}
	peer, err := m.store.GetPeer(ctx, peerID)
	if err != nil {
		return "", "", "", err
	}
	var body peerIdentityBody
	_ = json.Unmarshal(peer.Body, &body)
	return peer.Name, body.OriginHost, body.OriginRealm, nil
}

func parseScenarioUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	var p pgtype.UUID
	p.Bytes = id
	p.Valid = true
	return p, nil
}

func smResolveVariables(
	vars []variableJSON,
	sub *store.Subscriber,
	originHost, originRealm string,
) map[string]any {
	vals := make(map[string]any, len(vars)+12)
	vals["ORIGIN_HOST"] = originHost
	vals["ORIGIN_REALM"] = originRealm
	// Destination-Realm defaults to the peer's own realm (same domain).
	vals["DEST_REALM"] = originRealm
	// Auth-Application-Id is always 4 (Diameter Credit-Control / Gy).
	vals["AUTH_APP_ID"] = uint32(4)
	// END_USER_E164 (0) — constant for MSISDN-based Subscription-Id-Type AVP.
	vals["SUB_ID_TYPE"] = int32(0)
	if sub != nil {
		vals["MSISDN"] = sub.Msisdn
		vals["ICCID"] = sub.Iccid
		if sub.Imei.Valid {
			vals["IMEI"] = sub.Imei.String
		}
		// Auto-seed IMS calling-party from subscriber MSISDN so VOICE scenarios
		// get a valid SIP URI without requiring an explicit variable definition.
		vals["CALLING_PARTY_ADDRESS"] = sub.Msisdn
	}
	for _, v := range vars {
		switch v.Source.Kind {
		case "generator":
			smResolveGenerator(vals, v.Name, v.Source)
		case "bound":
			smResolveBound(vals, v.Name, v.Source, sub, originHost, originRealm)
		}
	}
	return vals
}

func smResolveGenerator(vals map[string]any, name string, src variableSrcJSON) {
	switch src.Strategy {
	case "literal":
		if src.Params != nil {
			vals[name] = src.Params["value"]
		}
	case "uuid":
		vals[name] = uuid.New().String()
	}
}

func smResolveBound(vals map[string]any, name string, src variableSrcJSON, sub *store.Subscriber, originHost, originRealm string) {
	switch src.From {
	case "subscriber":
		if sub == nil {
			return
		}
		switch src.Field {
		case "msisdn":
			vals[name] = sub.Msisdn
		case "iccid":
			vals[name] = sub.Iccid
		case "imei":
			if sub.Imei.Valid {
				vals[name] = sub.Imei.String
			}
		}
	case "peer":
		switch src.Field {
		case "originHost":
			vals[name] = originHost
		case "originRealm":
			vals[name] = originRealm
		}
	}
}

// smDeriveUnitType returns the Diameter unit type implied by the service type.
func smDeriveUnitType(serviceType string) string {
	switch strings.ToUpper(serviceType) {
	case "VOICE", "USSD2_SESSION":
		return "TIME"
	case "DATA":
		return "VOLUME"
	default: // SMS, USSD1_EVENT, USSD1_SESSION
		return "EVENT"
	}
}

// smServiceInfoForProfile returns the Service-Information child AVP list for
// the given (profile, serviceType) combination. Profile defaults to 3GPP when
// empty.
func smServiceInfoForProfile(profile, serviceType string) []avpNodeJSON {
	switch strings.ToUpper(profile) {
	case "HUAWEI":
		return smHuaweiServiceInfoAvps(serviceType)
	default:
		return sm3GPPServiceInfoAvps(serviceType)
	}
}

func sm3GPPServiceInfoAvps(serviceType string) []avpNodeJSON {
	switch strings.ToUpper(serviceType) {
	case "VOICE":
		return []avpNodeJSON{{
			Name:     "IMS-Information",
			VendorID: 10415,
			Children: []avpNodeJSON{
				// Node-Functionality (862): required by spec; "0" = S-CSCF.
				{Name: "Node-Functionality", VendorID: 10415, ValueRef: "0"},
				{Name: "Role-Of-Node", VendorID: 10415, ValueRef: "ROLE_OF_NODE"},
				{Name: "Calling-Party-Address", VendorID: 10415, ValueRef: "CALLING_PARTY_ADDRESS"},
				{Name: "Called-Party-Address", VendorID: 10415, ValueRef: "CALLED_PARTY_ADDRESS"},
			},
		}}
	case "DATA":
		return []avpNodeJSON{{Name: "PS-Information", VendorID: 10415}}
	case "SMS":
		return []avpNodeJSON{{Name: "SMS-Information", VendorID: 10415}}
	case "USSD1_EVENT", "USSD1_SESSION", "USSD2_SESSION":
		return []avpNodeJSON{{Name: "USSD-Information", VendorID: 10415}}
	default:
		return nil
	}
}

func smHuaweiServiceInfoAvps(serviceType string) []avpNodeJSON {
	switch strings.ToUpper(serviceType) {
	case "VOICE":
		return []avpNodeJSON{{Name: "IN_INFORMATION", VendorID: 2011}}
	case "DATA":
		return []avpNodeJSON{{Name: "PS-Information", VendorID: 10415}}
	case "SMS":
		return []avpNodeJSON{{Name: "SMS_INFORMATION", VendorID: 2011}}
	case "USSD1_EVENT", "USSD1_SESSION", "USSD2_SESSION":
		return []avpNodeJSON{{Name: "DCD_INFORMATION", VendorID: 2011}}
	default:
		return nil
	}
}

// ccrBuilderCodes are AVP codes that must not be included in ExtraAVPs
// because the CCR builder already injects them as mandatory fields.
// Including them again would produce duplicate AVPs on the wire.
var ccrBuilderCodes = map[uint32]bool{
	263: true, // Session-Id
	264: true, // Origin-Host
	296: true, // Origin-Realm
	283: true, // Destination-Realm
	258: true, // Auth-Application-Id
	461: true, // Service-Context-Id
}

func smConvertAvpTree(nodes []avpNodeJSON) []template.AVPNode {
	out := make([]template.AVPNode, 0, len(nodes))
	for _, n := range nodes {
		// Skip AVPs whose wire values are already injected by the CCR builder.
		if ccrBuilderCodes[n.Code] {
			continue
		}
		out = append(out, smConvertAvpNode(n))
	}
	return out
}

func smConvertAvpNode(n avpNodeJSON) template.AVPNode {
	node := template.AVPNode{
		Name:     n.Name,
		VendorID: n.VendorID,
	}
	if len(n.Children) > 0 {
		node.AVPs = smConvertAvpTree(n.Children)
	} else if n.ValueRef != "" {
		// Pure numeric literals are passed through directly so that
		// Enumerated AVPs like Subscription-Id-Type can be given a
		// literal value (e.g. "0" for END_USER_E164) without needing a
		// named variable.  Non-numeric refs are wrapped as {{TOKEN}}.
		if _, err := strconv.ParseFloat(n.ValueRef, 64); err == nil {
			node.Value = n.ValueRef
		} else {
			node.Value = "{{" + n.ValueRef + "}}"
		}
	}
	return node
}

func smConvertServices(services []serviceJSON, values map[string]any) []template.MSCCTemplateBlock {
	out := make([]template.MSCCTemplateBlock, len(services))
	for i, s := range services {
		block := template.MSCCTemplateBlock{
			Requested: "{{" + s.RequestedUnits + "}}",
		}
		if s.UsedUnits != "" {
			block.Used = "{{" + s.UsedUnits + "}}"
		}
		if s.RatingGroup != "" {
			block.RatingGroup = smResolveUint32(s.RatingGroup, values)
		}
		if s.ServiceIdentifier != "" {
			block.ServiceIdentifier = smResolveUint32(s.ServiceIdentifier, values)
		}
		if block.ServiceIdentifier == 0 {
			if n, err := strconv.ParseUint(s.ID, 10, 32); err == nil {
				block.ServiceIdentifier = uint32(n)
			}
		}
		out[i] = block
	}
	return out
}

func smResolveUint32(ref string, values map[string]any) uint32 {
	val, ok := values[ref]
	if !ok {
		if n, err := strconv.ParseUint(ref, 10, 32); err == nil {
			return uint32(n)
		}
		return 0
	}
	switch v := val.(type) {
	case uint32:
		return v
	case int:
		return uint32(v)
	case int64:
		return uint32(v)
	case float64:
		return uint32(v)
	case json.Number:
		if n, err := strconv.ParseUint(v.String(), 10, 32); err == nil {
			return uint32(n)
		}
	case string:
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			return uint32(n)
		}
	}
	return 0
}

// smMapState maps internal engine state strings to the OpenAPI ExecutionState enum.
func smMapState(s engine.SessionState) string {
	switch s {
	case engine.StateActive:
		return "running"
	case engine.StatePaused:
		return "paused"
	case engine.StateCompleted:
		return "success"
	case engine.StateTerminated:
		return "aborted"
	case engine.StateError:
		return "error"
	default:
		return "error"
	}
}

// smBuildContext partitions the session vars into the three OpenAPI context buckets.
func smBuildContext(vars map[string]any) ExecutionContextJSON {
	systemKeys := map[string]bool{
		"SESSION_ID": true, "CC_REQUEST_NUMBER": true,
		"ORIGIN_HOST": true, "ORIGIN_REALM": true,
	}
	ctx := ExecutionContextJSON{
		System:    make(map[string]any),
		User:      make(map[string]any),
		Extracted: make(map[string]any),
	}
	for k, v := range vars {
		if systemKeys[k] {
			ctx.System[k] = v
		} else {
			ctx.User[k] = v
		}
	}
	return ctx
}

func smMetricsFromSummary(s engine.MetricsSummary) ExecutionMetrics {
	return ExecutionMetrics{
		TotalRequests: s.TotalRequests,
		SuccessCount:  s.SuccessCount,
		FailureCount:  s.FailureCount,
		MinRTTMs:      float64(s.MinRTT.Milliseconds()),
		MaxRTTMs:      float64(s.MaxRTT.Milliseconds()),
		AvgRTTMs:      float64(s.AvgRTT.Milliseconds()),
	}
}

func smAllPassed(assertions []engine.AssertionResult) bool {
	for _, a := range assertions {
		if !a.Passed {
			return false
		}
	}
	return true
}
