package protocol

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// DefaultReAuthMargin is the cushion applied when scheduling a
// CCR-Update against a CCA's Validity-Time. The timer fires
// `Validity-Time - DefaultReAuthMargin` seconds after the CCA
// arrives so the re-auth lands before the grant actually expires.
//
// Five seconds matches the suggestion in the design plan and is a
// reasonable trade-off between re-auth latency and clock skew. A
// shorter margin risks the engine missing the validity window;
// longer means re-auth fires more often than strictly necessary.
const DefaultReAuthMargin = 5 * time.Second

// minScheduleDelay is the floor applied when validity time minus
// the re-auth margin would produce a non-positive duration. We
// schedule the re-auth immediately rather than not at all so an
// abnormally short Validity-Time doesn't silently disable
// re-authorisation.
const minScheduleDelay = 100 * time.Millisecond

// resultCodePermanentFailureLow / High are the bounds of the
// permanent-failure range per RFC 6733 §7.1.7.
const (
	resultCodePermanentFailureLow  uint32 = 5000
	resultCodePermanentFailureHigh uint32 = 5999
)

// ErrSessionTerminated is returned by Send when the session
// identified by req.SessionID was previously marked terminated
// (FUI-TERMINATE seen, 5xxx seen). Callers branch on this via
// errors.Is.
var ErrSessionTerminated = errors.New("protocol: session terminated")

// CCRUpdateBuilder builds the CCR the Behaviour issues to refresh
// a grant approaching its Validity-Time expiry. The Behaviour
// supplies the session id and the CC-Request-Number it expects
// the next request to carry (one greater than the most recent
// Send for that session); the builder fills in the rest of the
// CCR (Service-Context-Id, MSCC, Subscription-Id, …).
//
// Production wiring supplies a closure that consults the engine's
// per-session state. Tests supply a stub that asserts on the
// session id.
type CCRUpdateBuilder func(sessionID string, ccRequestNumber uint32) *messaging.CCR

// CCRTerminateBuilder builds the CCR-Terminate the Behaviour
// issues automatically when a CCA carries FUI = TERMINATE. Same
// pattern as CCRUpdateBuilder; the Behaviour fills in the
// CC-Request-Type as Terminate before sending.
type CCRTerminateBuilder func(sessionID string, ccRequestNumber uint32) *messaging.CCR

// Options configures the Behaviour. Every field is optional; the
// zero value is acceptable (re-auth scheduling and CCR-T emission
// will be no-ops without builders).
type Options struct {
	// ReAuthMargin overrides the default cushion applied to
	// Validity-Time when scheduling re-auth. Zero means
	// DefaultReAuthMargin.
	ReAuthMargin time.Duration

	// CCRUpdate is the builder the Behaviour invokes to construct
	// the re-auth CCR-Update fired before a Validity-Time
	// expires. Nil means re-auth is not scheduled.
	CCRUpdate CCRUpdateBuilder

	// CCRTerminate is the builder the Behaviour invokes to
	// construct the auto CCR-Terminate fired when FUI = TERMINATE
	// is observed. Nil means CCR-T is not emitted (the session
	// is still marked terminated).
	CCRTerminate CCRTerminateBuilder

	// PeerNameOf returns the peer name to send out-of-band CCRs
	// against, given a session id. The Behaviour records the peer
	// for every Send it observes, so this hook is only consulted
	// when a session is being terminated/re-authed before it has
	// been sent on; in normal operation the recorded peer is
	// used.
	PeerNameOf func(sessionID string) string

	// Now is the clock the Behaviour uses for re-auth scheduling.
	// Zero means time.Now.
	Now func() time.Time

	// AfterFunc is the seam the Behaviour uses to schedule timers.
	// Zero means time.AfterFunc. Tests substitute a synchronous
	// scheduler so re-auth fires immediately.
	AfterFunc func(d time.Duration, fn func()) Timer
}

// Timer is the small surface of time.Timer the Behaviour uses.
// Defined as its own interface so tests can substitute a fake
// timer that fires synchronously.
type Timer interface {
	// Stop cancels the timer. Returns true if the call stopped
	// the timer, false if the timer has already expired or been
	// stopped.
	Stop() bool
}

// realTimer wraps *time.Timer to satisfy the Timer interface.
type realTimer struct{ t *time.Timer }

func (r *realTimer) Stop() bool { return r.t.Stop() }

// realAfterFunc is the production AfterFunc — defers to
// time.AfterFunc.
func realAfterFunc(d time.Duration, fn func()) Timer {
	return &realTimer{t: time.AfterFunc(d, fn)}
}

// Behaviour is the protocol-mandated CCA observer. It wraps a
// messaging.Sender and itself implements messaging.Sender so
// callers can swap it in transparently.
//
// Construct via New. Lifecycle:
//
//   - Send delegates to the wrapped Sender, then observes the
//     returned CCA: marks the session terminated on
//     FUI-TERMINATE or 5xxx, schedules / refreshes re-auth on
//     Validity-Time, and tracks the most recent CC-Request-Number
//     so the builder hook can produce the next CCR.
//   - SessionTerminated reports whether a session has been
//     terminated by the Behaviour (test / introspection hook).
//   - Stop cancels every outstanding re-auth timer and clears the
//     session map. Idempotent.
//
// Concurrency: every public method is goroutine-safe.
type Behaviour struct {
	inner messaging.Sender
	opt   Options

	mu       sync.Mutex
	sessions map[string]*sessionState
	stopped  bool
}

// sessionState is the Behaviour's per-session bookkeeping.
type sessionState struct {
	peerName        string
	ccRequestNumber uint32
	terminated      bool
	timer           Timer
}

// New constructs a Behaviour decorating inner. Panics on a nil
// inner — wiring a Behaviour against a nil Sender is a
// programming error.
func New(inner messaging.Sender, opts Options) *Behaviour {
	if inner == nil {
		panic("protocol.New: inner sender must not be nil")
	}
	if opts.ReAuthMargin <= 0 {
		opts.ReAuthMargin = DefaultReAuthMargin
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.AfterFunc == nil {
		opts.AfterFunc = realAfterFunc
	}
	return &Behaviour{
		inner:    inner,
		opt:      opts,
		sessions: map[string]*sessionState{},
	}
}

// Send is the messaging.Sender implementation. It delegates to
// the wrapped sender, observes the response, and applies
// protocol-mandated actions before returning the CCA verbatim.
func (b *Behaviour) Send(ctx context.Context, peerName string, req *messaging.CCR) (*messaging.CCA, error) {
	if req != nil && req.SessionID != "" {
		b.mu.Lock()
		state := b.sessions[req.SessionID]
		terminated := state != nil && state.terminated
		b.mu.Unlock()
		if terminated {
			return nil, fmt.Errorf("%w: session %q", ErrSessionTerminated, req.SessionID)
		}
	}

	cca, err := b.inner.Send(ctx, peerName, req)
	if err != nil {
		return nil, err
	}

	b.observeCCA(peerName, req, cca)
	return cca, nil
}

// observeCCA inspects the CCA and updates session state. Called
// after every successful Send.
//
// Order of operations matters: 5xxx and FUI-TERMINATE are checked
// before Validity-Time scheduling, because a terminated session
// does not need a re-auth timer.
func (b *Behaviour) observeCCA(peerName string, req *messaging.CCR, cca *messaging.CCA) {
	if cca == nil {
		return
	}
	sessionID := cca.SessionID
	if sessionID == "" && req != nil {
		sessionID = req.SessionID
	}
	if sessionID == "" {
		return
	}

	b.mu.Lock()
	state := b.sessions[sessionID]
	if state == nil {
		state = &sessionState{peerName: peerName}
		b.sessions[sessionID] = state
	}
	state.peerName = peerName
	if req != nil && req.CCRequestNumber > state.ccRequestNumber {
		state.ccRequestNumber = req.CCRequestNumber
	}

	// Permanent failure (5xxx) — terminate the session.
	if isPermanentFailure(cca.ResultCode) {
		state.terminated = true
		b.cancelTimerLocked(state)
		b.mu.Unlock()
		logging.Info(
			"protocol: session terminated by 5xxx Result-Code",
			"session_id", sessionID,
			"result_code", cca.ResultCode,
		)
		return
	}

	// FUI = TERMINATE — terminate the session AND fire CCR-T.
	if cca.FUIAction == messaging.FUIActionTerminate {
		state.terminated = true
		b.cancelTimerLocked(state)
		nextRN := state.ccRequestNumber + 1
		state.ccRequestNumber = nextRN
		builder := b.opt.CCRTerminate
		b.mu.Unlock()
		logging.Info(
			"protocol: FUI=TERMINATE → emitting CCR-Terminate",
			"session_id", sessionID,
			"cc_request_number", nextRN,
		)
		if builder != nil {
			b.emitTerminate(peerName, sessionID, nextRN, builder)
		}
		return
	}

	// Validity-Time present — schedule (or refresh) re-auth.
	if cca.ValidityTime > 0 {
		b.cancelTimerLocked(state)
		delay := time.Duration(cca.ValidityTime)*time.Second - b.opt.ReAuthMargin
		if delay < minScheduleDelay {
			delay = minScheduleDelay
		}
		afterFunc := b.opt.AfterFunc
		state.timer = afterFunc(delay, func() {
			b.fireReAuth(sessionID)
		})
		b.mu.Unlock()
		logging.Info(
			"protocol: scheduled re-auth",
			"session_id", sessionID,
			"validity_time_seconds", cca.ValidityTime,
			"reauth_in", delay.String(),
		)
		return
	}

	b.mu.Unlock()
}

// fireReAuth is the timer callback. Issues a CCR-Update via the
// inner Sender if the session is still active.
func (b *Behaviour) fireReAuth(sessionID string) {
	b.mu.Lock()
	state := b.sessions[sessionID]
	if state == nil || state.terminated || b.stopped {
		b.mu.Unlock()
		return
	}
	peerName := state.peerName
	nextRN := state.ccRequestNumber + 1
	state.ccRequestNumber = nextRN
	state.timer = nil
	builder := b.opt.CCRUpdate
	b.mu.Unlock()

	if builder == nil {
		return
	}
	logging.Info(
		"protocol: firing scheduled re-auth",
		"session_id", sessionID,
		"cc_request_number", nextRN,
	)
	req := builder(sessionID, nextRN)
	if req == nil {
		return
	}
	req.SessionID = sessionID
	req.CCRequestType = messaging.CCRTypeUpdate
	req.CCRequestNumber = nextRN

	// Re-auth happens with a fresh background context; the
	// engine's caller context cannot be the bound context here
	// because the re-auth is asynchronous.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := b.Send(ctx, peerName, req); err != nil {
			logging.Warn(
				"protocol: scheduled re-auth failed",
				"session_id", sessionID,
				"peer", peerName,
				"error", err.Error(),
			)
		}
	}()
}

// emitTerminate sends an automatic CCR-Terminate for a session
// that just observed FUI=TERMINATE. Runs in a fresh goroutine so
// the original Send returns to the caller immediately.
func (b *Behaviour) emitTerminate(peerName, sessionID string, nextRN uint32, builder CCRTerminateBuilder) {
	go func() {
		req := builder(sessionID, nextRN)
		if req == nil {
			return
		}
		req.SessionID = sessionID
		req.CCRequestType = messaging.CCRTypeTerminate
		req.CCRequestNumber = nextRN
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Bypass the Behaviour's own SessionTerminated guard by
		// going through the inner sender directly — the session
		// IS terminated, but we still need to put the
		// CCR-Terminate on the wire so the OCS knows to release
		// resources.
		if _, err := b.inner.Send(ctx, peerName, req); err != nil {
			logging.Warn(
				"protocol: auto CCR-Terminate failed",
				"session_id", sessionID,
				"peer", peerName,
				"error", err.Error(),
			)
		}
	}()
}

// cancelTimerLocked stops a session's pending re-auth timer.
// Caller must hold b.mu.
func (b *Behaviour) cancelTimerLocked(s *sessionState) {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

// SessionTerminated reports whether a session has been marked
// terminated by the Behaviour. Convenience for tests and
// introspection.
func (b *Behaviour) SessionTerminated(sessionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.sessions[sessionID]
	return s != nil && s.terminated
}

// Stop cancels every outstanding re-auth timer and clears the
// session map. Subsequent calls to Send return
// ErrSessionTerminated for any session that was tracked. Stop is
// idempotent.
func (b *Behaviour) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	b.stopped = true
	for id, s := range b.sessions {
		b.cancelTimerLocked(s)
		s.terminated = true
		_ = id
	}
}

// isPermanentFailure reports whether code is in the 5xxx
// permanent-failure range.
func isPermanentFailure(code uint32) bool {
	return code >= resultCodePermanentFailureLow && code <= resultCodePermanentFailureHigh
}
