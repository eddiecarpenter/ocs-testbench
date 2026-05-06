package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
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
	n                int
	kind             string
	requestType      string
	label            string
	state            string
	startedAt        string
	finishedAt       string
	durationMs       int64
	errorDetail      string
	request          map[string]any
	response         map[string]any
	requestText      string
	responseText     string
	assertionResults []assertionJSON
}

type sessionRecord struct {
	id                  string
	scenarioID          string
	scenarioName        string
	mode                string
	repeats             int
	completedIterations int
	startedAt           string
	sc                  *engine.SessionContext
	orc                 *engine.Orchestrator
	steps               []engine.ScenarioStep

	mu          sync.RWMutex
	state       engine.SessionState
	stepIdx     int
	prevResult  *engine.SendResult
	stepHistory []stepHistoryRecord

	cancel      context.CancelFunc
	stopCh      chan struct{}
	interruptCh chan struct{}
	done        chan struct{}

	subsMu sync.Mutex
	subs   []chan ExecutionEvent
	// sleepUntil is non-zero while the engine is sleeping between
	// iterations. Used to re-emit the sleeping event to late subscribers.
	sleepStepIdx int
	sleepUntil   time.Time
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
	mgr    PeerManager
	sender messaging.Sender
	dict   template.Dictionary
	tmpl   *template.Engine

	mu       sync.Mutex
	sessions map[string]*sessionRecord
}

// ErrPeerNotConnected is returned by Start when the scenario's peer is not in
// the Connected state. The caller should surface this as a 409 so the UI can
// prompt the user to connect the peer before running.
var ErrPeerNotConnected = errors.New("peer not connected")

// NewSessionManager creates a SessionManager.
func NewSessionManager(s store.Store, mgr PeerManager, sender messaging.Sender, dict template.Dictionary) *SessionManager {
	return &SessionManager{
		store:    s,
		mgr:      mgr,
		sender:   sender,
		dict:     dict,
		tmpl:     template.NewEngine(),
		sessions: make(map[string]*sessionRecord),
	}
}

// Start implements ExecutionEngine.
func (m *SessionManager) Start(ctx context.Context, scenarioID string, mode string, repeats int) (StartInfo, error) {
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

	if m.mgr != nil {
		state, stateErr := m.mgr.State(peerName)
		if stateErr != nil || state != diameter.StateConnected {
			return StartInfo{}, ErrPeerNotConnected
		}
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
		repeats:      repeats,
		startedAt:    time.Now().UTC().Format(time.RFC3339),
		sc:           sc,
		orc:          orc,
		steps:        steps,
		state:        initialState,
		cancel:       cancel,
		stopCh:       stopCh,
		interruptCh:  make(chan struct{}),
		done:         make(chan struct{}),
	}

	m.mu.Lock()
	m.sessions[sessionID] = rec
	m.mu.Unlock()

	if execMode == engine.ModeContinuous {
		go m.runContinuous(runCtx, rec, 0)
	} else {
		close(rec.done)
		cancel()
	}

	return StartInfo{SessionID: sessionID, ScenarioName: scRow.Name}, nil
}

func (m *SessionManager) runContinuous(ctx context.Context, rec *sessionRecord, stepOffset int) {
	defer close(rec.done)

	steps := rec.steps[stepOffset:]

	opts := engine.RunOptions{
		StopCh:        rec.stopCh,
		InterruptCh:   rec.interruptCh,
		StepOffset:    stepOffset,
		MaxIterations: rec.repeats,
		OnStep: func(stepIdx int, iteration int, step engine.ScenarioStep, result engine.StepResult) {
			smRecordStep(rec, stepIdx, iteration, step, result, nil)
			rec.mu.Lock()
			rec.stepIdx = stepIdx + 1
			rec.sleepUntil = time.Time{} // clear any active sleep
			rec.mu.Unlock()
			rec.broadcast(ExecutionEvent{
				Type:      "progress",
				SessionID: rec.id,
				State:     "running",
				Step:      stepIdx,
				Metrics:   smMetricsFromSummary(rec.sc.Summary()),
			})
		},
		OnDelay: func(stepIdx int, d time.Duration) {
			until := time.Now().Add(d)
			rec.mu.Lock()
			rec.sleepStepIdx = stepIdx
			rec.sleepUntil = until
			rec.mu.Unlock()
			rec.broadcast(ExecutionEvent{
				Type:      "progress",
				SessionID: rec.id,
				State:     "sleeping",
				Step:      stepIdx,
				DelaySec:  int(d.Seconds()),
				Metrics:   smMetricsFromSummary(rec.sc.Summary()),
			})
		},
		OnIteration: func(n int) {
			rec.mu.Lock()
			rec.completedIterations = n
			rec.mu.Unlock()
		},
	}
	err := rec.orc.RunContinuous(ctx, rec.sc, steps, opts)

	rec.mu.Lock()
	if rec.stepIdx < len(rec.steps) {
		rec.stepIdx = len(rec.steps)
	}

	switch {
	case errors.Is(err, engine.ErrExecutionInterrupted):
		// Interrupted — stay in StatePaused (set by orchestrator); do NOT
		// broadcast completed. The UI will reflect the paused state via SSE.
		rec.mu.Unlock()
		rec.broadcast(ExecutionEvent{
			Type:      "progress",
			SessionID: rec.id,
			State:     "paused",
			Step:      rec.stepIdx,
		})
		return
	case err != nil && !errors.Is(err, context.Canceled):
		rec.state = engine.StateError
	default:
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
			SessionID:           rec.id,
			ScenarioID:          rec.scenarioID,
			ScenarioName:        rec.scenarioName,
			Mode:                rec.mode,
			State:               smMapState(rec.state),
			StartedAt:           rec.startedAt,
			Repeats:             rec.repeats,
			CompletedIterations: rec.completedIterations,
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

// Interrupt implements ExecutionEngine — signals a running continuous session
// to pause after the current send completes. The session transitions to
// StatePaused and can be resumed with RunToEnd.
func (m *SessionManager) Interrupt(_ context.Context, sessionID string) error {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	rec.mu.RLock()
	state := rec.state
	mode := rec.mode
	rec.mu.RUnlock()

	if mode != "continuous" || state != engine.StateActive {
		return ErrInvalidState
	}

	select {
	case <-rec.interruptCh:
		// Already interrupted — idempotent.
	default:
		close(rec.interruptCh)
	}
	return nil
}

// RunToEnd implements ExecutionEngine — resumes an interrupted continuous
// session from its current step position, running to completion.
func (m *SessionManager) RunToEnd(ctx context.Context, sessionID string) error {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	rec.mu.Lock()
	if rec.state != engine.StatePaused || rec.mode != "continuous" {
		rec.mu.Unlock()
		return ErrInvalidState
	}

	// Reset interrupt channel and done channel for the new run.
	rec.interruptCh = make(chan struct{})
	rec.done = make(chan struct{})
	rec.state = engine.StateActive
	stepOffset := rec.stepIdx
	rec.mu.Unlock()

	runCtx, cancel := context.WithCancel(context.Background())
	rec.cancel = cancel

	go m.runContinuous(runCtx, rec, stepOffset)
	return nil
}

// Skip implements ExecutionEngine — advances past the current step without
// sending a CCR. Records the step as "skipped" in history.
func (m *SessionManager) Skip(_ context.Context, sessionID string) error {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	rec.mu.Lock()
	if rec.state != engine.StatePaused {
		rec.mu.Unlock()
		return ErrInvalidState
	}
	if rec.stepIdx >= len(rec.steps) {
		rec.mu.Unlock()
		return ErrInvalidState
	}
	stepIdx := rec.stepIdx
	step := rec.steps[stepIdx]
	rec.stepHistory = append(rec.stepHistory, stepHistoryRecord{
		n:     len(rec.stepHistory) + 1,
		kind:  step.Kind,
		label: smStepLabel(step),
		state: "skipped",
	})
	rec.stepIdx++
	if rec.stepIdx >= len(rec.steps) {
		rec.state = engine.StateCompleted
	}
	rec.mu.Unlock()

	rec.broadcast(ExecutionEvent{
		Type:      "progress",
		SessionID: rec.id,
		State:     rec.state.String(),
		Step:      rec.stepIdx,
	})
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

	// Record history before updating session state so Detail sees it immediately.
	smRecordStep(rec, stepIdx, 0, step, yield.Result, stepErr)

	rec.mu.Lock()
	rec.stepIdx++
	if stepErr != nil {
		rec.state = engine.StateError
	} else {
		rec.prevResult = &yield.Result.SendResult
		if rec.stepIdx >= len(rec.steps) {
			rec.state = engine.StateCompleted
		} else {
			// Stay paused between steps in interactive mode.
			rec.state = engine.StatePaused
		}
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

// Detail implements ExecutionEngine.
func (m *SessionManager) Detail(_ context.Context, sessionID string) (ExecutionDetailResponse, error) {
	rec, err := m.getSession(sessionID)
	if err != nil {
		return ExecutionDetailResponse{}, err
	}

	rec.mu.RLock()
	state := rec.state
	stepIdx := rec.stepIdx
	totalSteps := len(rec.steps)
	history := make([]stepHistoryRecord, len(rec.stepHistory))
	copy(history, rec.stepHistory)
	scenarioSteps := rec.steps
	rec.mu.RUnlock()

	// Executed history first, then remaining scenario steps as pending rows.
	// This gives the UI a complete picture of what has run and what is next.
	steps := make([]StepRecordJSON, 0, len(history)+len(scenarioSteps)-stepIdx)
	for _, h := range history {
		steps = append(steps, StepRecordJSON{
			N:                h.n,
			Kind:             h.kind,
			RequestType:      h.requestType,
			Label:            h.label,
			State:            h.state,
			StartedAt:        h.startedAt,
			FinishedAt:       h.finishedAt,
			DurationMs:       h.durationMs,
			ErrorDetail:      h.errorDetail,
			Response:         h.response,
			RequestText:      h.requestText,
			ResponseText:     h.responseText,
			AssertionResults: h.assertionResults,
		})
	}
	// Append exactly one pending row — the step about to execute — so the
	// UI always shows what's next without exposing the full scenario ahead.
	if stepIdx < len(scenarioSteps) {
		s := scenarioSteps[stepIdx]
		steps = append(steps, StepRecordJSON{
			N:           len(history) + 1,
			Kind:        s.Kind,
			RequestType: s.RequestType,
			Label:       smStepLabel(s),
			State:       "pending",
		})
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
		TotalSteps:   totalSteps,
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

	// If the engine is currently sleeping, send the sleep event immediately
	// so subscribers who connect mid-sleep see the countdown right away.
	rec.mu.RLock()
	sleepUntil := rec.sleepUntil
	sleepStepIdx := rec.sleepStepIdx
	rec.mu.RUnlock()
	if !sleepUntil.IsZero() {
		remainingSec := int(time.Until(sleepUntil).Seconds())
		if remainingSec > 0 {
			ch <- ExecutionEvent{
				Type:      "progress",
				SessionID: rec.id,
				State:     "sleeping",
				Step:      sleepStepIdx,
				DelaySec:  remainingSec,
			}
		}
	}

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

// smRecordStep appends a history record for a completed step. Shared between
// interactive (Step method) and continuous (OnStep callback) paths.
func smRecordStep(rec *sessionRecord, stepIdx int, iteration int, step engine.ScenarioStep, result engine.StepResult, stepErr error) {
	finishedAt := time.Now().UTC()
	stepState := "success"
	if stepErr != nil || result.Error != "" {
		stepState = "error"
	} else if result.Skipped {
		stepState = "skipped"
	} else if !smAllPassed(result.Assertions) {
		stepState = "failure"
	}

	durationMs := int64(0)
	startTs := finishedAt
	if !result.Skipped && stepErr == nil && result.SendResult.Metrics.RTT > 0 {
		startTs = finishedAt.Add(-result.SendResult.Metrics.RTT)
		durationMs = finishedAt.Sub(startTs).Milliseconds()
	}

	label := step.Label
	if label == "" {
		label = smStepLabel(step)
	}
	if iteration > 0 {
		label = fmt.Sprintf("%s #%d", label, iteration+1)
	}

	assertions := make([]assertionJSON, len(result.Assertions))
	for i, a := range result.Assertions {
		assertions[i] = assertionJSON{Expression: a.Expression, Passed: a.Passed, Message: a.Message}
	}

	errDetail := ""
	if stepErr != nil {
		errDetail = stepErr.Error()
	} else if result.Error != "" {
		errDetail = result.Error
	}

	cca := result.SendResult.CCA
	rec.mu.Lock()
	rec.stepHistory = append(rec.stepHistory, stepHistoryRecord{
		n:                len(rec.stepHistory) + 1,
		kind:             step.Kind,
		requestType:      step.RequestType,
		label:            label,
		state:            stepState,
		startedAt:        startTs.Format(time.RFC3339),
		finishedAt:       finishedAt.Format(time.RFC3339),
		durationMs:       durationMs,
		errorDetail:      errDetail,
		response:         smCCAToMap(cca),
		requestText:      messaging.FormatDiameterMessage(smSentMessage(cca)),
		responseText:     messaging.FormatDiameterMessage(smRawMessage(cca)),
		assertionResults: assertions,
	})
	rec.mu.Unlock()
}

// smStepLabel derives a display label from the step definition when the
// scenario author has not provided an explicit label.
func smStepLabel(step engine.ScenarioStep) string {
	if step.Kind != "request" {
		return strings.ToUpper(step.Kind[:1]) + step.Kind[1:]
	}
	switch strings.ToUpper(step.RequestType) {
	case "INITIAL":
		return "CCR-INITIAL"
	case "UPDATE":
		return "CCR-UPDATE"
	case "TERMINATE":
		return "CCR-TERMINATE"
	case "EVENT":
		return "CCR-EVENT"
	}
	return "CCR"
}

func smSentMessage(cca *messaging.CCA) *diam.Message {
	if cca == nil {
		return nil
	}
	return cca.SentMessage
}

func smRawMessage(cca *messaging.CCA) *diam.Message {
	if cca == nil {
		return nil
	}
	return cca.Raw
}

// smCCAToMap converts a CCA to a JSON-serialisable map for the step history.
func smCCAToMap(cca *messaging.CCA) map[string]any {
	if cca == nil {
		return nil
	}
	m := map[string]any{
		"resultCode":      cca.ResultCode,
		"sessionId":       cca.SessionID,
		"originHost":      cca.OriginHost,
		"originRealm":     cca.OriginRealm,
		"ccRequestType":   cca.CCRequestType,
		"ccRequestNumber": cca.CCRequestNumber,
		"fuiAction":       cca.FUIAction,
	}
	if len(cca.MSCC) > 0 {
		mscc := make([]map[string]any, len(cca.MSCC))
		for i, b := range cca.MSCC {
			mscc[i] = map[string]any{
				"ratingGroup":        b.RatingGroup,
				"serviceIdentifier":  b.ServiceIdentifier,
				"resultCode":         b.ResultCode,
				"grantedTime":        b.GrantedTime,
				"grantedTotalOctets": b.GrantedTotalOctets,
				"validityTime":       b.ValidityTime,
				"fuiAction":          b.FUIAction,
			}
		}
		m["mscc"] = mscc
	}
	return m
}

// ResponseTimeSeries implements ExecutionEngine. It collects durationMs values
// from all step history records that fall within the requested window, groups
// them into ~12 time buckets, and returns p50/p95/p99 per bucket.
func (m *SessionManager) ResponseTimeSeries(_ context.Context, window string) (ResponseTimeSeries, error) {
	windowDur, err := parseISO8601Duration(window)
	if err != nil {
		return ResponseTimeSeries{}, fmt.Errorf("invalid window %q: %w", window, err)
	}

	// Choose a bucket size that gives ~12 data points.
	bucketDur := windowDur / 12
	if bucketDur < time.Minute {
		bucketDur = time.Minute
	}
	// Round to whole minutes for clean labels.
	bucketDur = bucketDur.Round(time.Minute)
	if bucketDur == 0 {
		bucketDur = time.Minute
	}

	nBuckets := int(math.Ceil(float64(windowDur) / float64(bucketDur)))
	now := time.Now().UTC()
	cutoff := now.Add(-windowDur)

	// Bucket index 0 = oldest.
	type bucket struct{ samples []float64 }
	buckets := make([]bucket, nBuckets)

	m.mu.Lock()
	for _, rec := range m.sessions {
		rec.mu.RLock()
		for _, h := range rec.stepHistory {
			if h.durationMs <= 0 || h.finishedAt == "" {
				continue
			}
			ts, err := time.Parse(time.RFC3339, h.finishedAt)
			if err != nil || ts.Before(cutoff) {
				continue
			}
			age := now.Sub(ts)
			idx := nBuckets - 1 - int(age/bucketDur)
			if idx < 0 || idx >= nBuckets {
				continue
			}
			buckets[idx].samples = append(buckets[idx].samples, float64(h.durationMs))
		}
		rec.mu.RUnlock()
	}
	m.mu.Unlock()

	points := make([]ResponseTimePoint, nBuckets)
	for i, b := range buckets {
		t := now.Add(-time.Duration(nBuckets-1-i) * bucketDur).Truncate(bucketDur)
		pt := ResponseTimePoint{T: t.Format(time.RFC3339)}
		if len(b.samples) > 0 {
			sorted := make([]float64, len(b.samples))
			copy(sorted, b.samples)
			sort.Float64s(sorted)
			pt.P50 = percentile(sorted, 50)
			pt.P95 = percentile(sorted, 95)
			pt.P99 = percentile(sorted, 99)
		}
		points[i] = pt
	}

	bucketLabel := formatISO8601Duration(bucketDur)
	return ResponseTimeSeries{Window: window, BucketSize: bucketLabel, Points: points}, nil
}

// parseISO8601Duration parses a subset of ISO 8601 durations: PTxH, PTxM,
// PTxHyM. Returns an error for any other form.
func parseISO8601Duration(s string) (time.Duration, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "PT") {
		return 0, fmt.Errorf("only time-only durations (PT…) are supported")
	}
	rest := s[2:]
	var total time.Duration
	for len(rest) > 0 {
		var n int
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			n = n*10 + int(rest[i]-'0')
			i++
		}
		if i == len(rest) {
			return 0, fmt.Errorf("missing unit in %q", s)
		}
		switch rest[i] {
		case 'H':
			total += time.Duration(n) * time.Hour
		case 'M':
			total += time.Duration(n) * time.Minute
		case 'S':
			total += time.Duration(n) * time.Second
		default:
			return 0, fmt.Errorf("unknown unit %q in %q", string(rest[i]), s)
		}
		rest = rest[i+1:]
	}
	if total == 0 {
		return 0, fmt.Errorf("zero duration")
	}
	return total, nil
}

func formatISO8601Duration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	parts := "PT"
	if h > 0 {
		parts += fmt.Sprintf("%dH", h)
	}
	if m > 0 {
		parts += fmt.Sprintf("%dM", m)
	}
	if s > 0 {
		parts += fmt.Sprintf("%dS", s)
	}
	if parts == "PT" {
		parts = "PT0S"
	}
	return parts
}

// percentile returns the p-th percentile (0–100) of a pre-sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
