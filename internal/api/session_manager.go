package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/engine"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	"github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// — Scenario body JSON shapes (wire format stored in DB) —

type scenarioBodyExec struct {
	UnitType     string          `json:"unitType"`
	ServiceModel string          `json:"serviceModel"`
	AvpTree      json.RawMessage `json:"avpTree"`
	Services     json.RawMessage `json:"services"`
	Variables    json.RawMessage `json:"variables"`
	Steps        json.RawMessage `json:"steps"`
}

type avpNodeJSON struct {
	Name     string        `json:"name"`
	VendorID int64         `json:"vendorId,omitempty"`
	ValueRef string        `json:"valueRef,omitempty"`
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
	Name   string            `json:"name"`
	Source variableSrcJSON   `json:"source"`
}

type variableSrcJSON struct {
	Kind     string         `json:"kind"`
	From     string         `json:"from,omitempty"`
	Field    string         `json:"field,omitempty"`
	Strategy string         `json:"strategy,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// peerIdentityBody is the minimal subset of the peer body needed for
// extracting origin_host / origin_realm.
type peerIdentityBody struct {
	Identity struct {
		OriginHost  string `json:"origin_host"`
		OriginRealm string `json:"origin_realm"`
	} `json:"identity"`
}

// — Session record —

type sessionRecord struct {
	id    string
	sc    *engine.SessionContext
	orc   *engine.Orchestrator
	steps []engine.ScenarioStep

	mu         sync.RWMutex
	state      engine.SessionState
	stepIdx    int
	prevResult *engine.SendResult

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
func (m *SessionManager) Start(ctx context.Context, scenarioID string, mode string) (string, error) {
	scUID, err := parseScenarioUUID(scenarioID)
	if err != nil {
		return "", ErrScenarioNotFound
	}

	scRow, err := m.store.GetScenario(ctx, scUID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrScenarioNotFound
		}
		return "", fmt.Errorf("session manager: load scenario: %w", err)
	}

	var body scenarioBodyExec
	if err := json.Unmarshal(scRow.Body, &body); err != nil {
		return "", fmt.Errorf("session manager: parse scenario body: %w", err)
	}

	var avpNodes []avpNodeJSON
	if err := json.Unmarshal(body.AvpTree, &avpNodes); err != nil {
		return "", fmt.Errorf("session manager: parse avpTree: %w", err)
	}

	var services []serviceJSON
	if err := json.Unmarshal(body.Services, &services); err != nil {
		return "", fmt.Errorf("session manager: parse services: %w", err)
	}

	var variables []variableJSON
	if err := json.Unmarshal(body.Variables, &variables); err != nil {
		return "", fmt.Errorf("session manager: parse variables: %w", err)
	}

	var steps []engine.ScenarioStep
	if err := json.Unmarshal(body.Steps, &steps); err != nil {
		return "", fmt.Errorf("session manager: parse steps: %w", err)
	}

	peerName, originHost, originRealm, err := m.loadPeerInfo(ctx, scRow.PeerID)
	if err != nil {
		return "", fmt.Errorf("session manager: load peer: %w", err)
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
		UnitType:     template.UnitType(body.UnitType),
		ServiceModel: template.ServiceModel(body.ServiceModel),
	}

	execMode := engine.ModeInteractive
	if mode == "continuous" {
		execMode = engine.ModeContinuous
	}

	sc := engine.NewSessionContext(peerName, originHost, m.sender, execMode)
	for k, v := range values {
		sc.Vars[k] = v
	}
	sc.Vars["SESSION_ID"] = sc.SessionID

	stepExec := engine.NewStepExecutor(m.tmpl, baseInput)
	orc := engine.NewOrchestrator(stepExec)

	sessionID := uuid.New().String()
	stopCh := make(chan struct{})
	runCtx, cancel := context.WithCancel(context.Background())

	rec := &sessionRecord{
		id:     sessionID,
		sc:     sc,
		orc:    orc,
		steps:  steps,
		state:  engine.StateActive,
		cancel: cancel,
		stopCh: stopCh,
		done:   make(chan struct{}),
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

	return sessionID, nil
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
	if rec.state != engine.StateActive {
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

	rec.mu.Lock()
	rec.stepIdx++
	if stepErr == nil {
		rec.prevResult = &yield.Result.SendResult
		if rec.stepIdx >= len(rec.steps) {
			rec.state = engine.StateCompleted
		}
	} else {
		rec.state = engine.StateError
	}
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

// Status implements ExecutionEngine.
func (m *SessionManager) Status(_ context.Context, sessionID string) (ExecutionStatus, error) {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return ExecutionStatus{}, err
	}

	rec.mu.RLock()
	state := rec.state
	stepIdx := rec.stepIdx
	rec.mu.RUnlock()

	summary := rec.sc.Summary()

	return ExecutionStatus{
		SessionID:   sessionID,
		State:       state.String(),
		CurrentStep: stepIdx,
		Metrics:     smMetricsFromSummary(summary),
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
	return peer.Name, body.Identity.OriginHost, body.Identity.OriginRealm, nil
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
	vals := make(map[string]any, len(vars)+8)
	vals["ORIGIN_HOST"] = originHost
	vals["ORIGIN_REALM"] = originRealm
	if sub != nil {
		vals["MSISDN"] = sub.Msisdn
		vals["ICCID"] = sub.Iccid
		if sub.Imei.Valid {
			vals["IMEI"] = sub.Imei.String
		}
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

func smConvertAvpTree(nodes []avpNodeJSON) []template.AVPNode {
	out := make([]template.AVPNode, len(nodes))
	for i, n := range nodes {
		out[i] = smConvertAvpNode(n)
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
		node.Value = "{{" + n.ValueRef + "}}"
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
