package protocol

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
)

// fakeSender is the test stand-in for messaging.Sender. Records
// every Send call and returns a canned response per session id /
// CC-Request-Type.
type fakeSender struct {
	mu    sync.Mutex
	calls []sendCall
	resp  func(req *messaging.CCR) (*messaging.CCA, error)
}

type sendCall struct {
	peer string
	req  *messaging.CCR
}

func (f *fakeSender) Send(_ context.Context, peer string, req *messaging.CCR) (*messaging.CCA, error) {
	f.mu.Lock()
	f.calls = append(f.calls, sendCall{peer: peer, req: copyCCR(req)})
	resp := f.resp
	f.mu.Unlock()
	if resp == nil {
		return &messaging.CCA{SessionID: req.SessionID, ResultCode: 2001, FUIAction: -1}, nil
	}
	return resp(req)
}

// copyCCR returns a deep-enough copy of req for assertions —
// captures session id, CC-Request-Type / Number.
func copyCCR(req *messaging.CCR) *messaging.CCR {
	if req == nil {
		return nil
	}
	c := *req
	return &c
}

// callCount returns the number of recorded Send calls.
func (f *fakeSender) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// callsFor returns the recorded calls whose req.SessionID matches
// sid.
func (f *fakeSender) callsFor(sid string) []sendCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sendCall, 0)
	for _, c := range f.calls {
		if c.req != nil && c.req.SessionID == sid {
			out = append(out, c)
		}
	}
	return out
}

// fakeTimer is a Timer that fires synchronously when the test
// drives it, or never if the test doesn't.
type fakeTimer struct {
	fn      func()
	stopped atomic.Bool
}

func (t *fakeTimer) Stop() bool {
	already := t.stopped.Swap(true)
	return !already
}

// fire invokes the timer's callback if it has not been stopped.
func (t *fakeTimer) fire() {
	if t.stopped.Load() {
		return
	}
	t.fn()
}

// fakeScheduler captures every AfterFunc call so the test can
// inspect / fire timers deterministically.
type fakeScheduler struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

func (s *fakeScheduler) AfterFunc() func(d time.Duration, fn func()) Timer {
	return func(_ time.Duration, fn func()) Timer {
		t := &fakeTimer{fn: fn}
		s.mu.Lock()
		s.timers = append(s.timers, t)
		s.mu.Unlock()
		return t
	}
}

// last returns the most recently scheduled fakeTimer.
func (s *fakeScheduler) last() *fakeTimer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.timers) == 0 {
		return nil
	}
	return s.timers[len(s.timers)-1]
}

// count returns how many timers were scheduled.
func (s *fakeScheduler) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.timers)
}

// validCCR returns a minimal CCR with a fixed Session-Id and
// Initial CC-Request-Type. Tests adjust as needed.
func validCCR() *messaging.CCR {
	return &messaging.CCR{
		SessionID:        "sess-1",
		ServiceContextID: "32251@3gpp.org",
		DestinationRealm: "test.local",
		CCRequestType:    messaging.CCRTypeInitial,
		CCRequestNumber:  0,
	}
}

// Test 1 (AC-12) — FUI=TERMINATE marks the session terminated and
// fires CCR-Terminate exactly once.
func TestBehaviour_FUITerminateEmitsCCRT(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			// On the original CCR-Initial, return CCA with
			// FUI=TERMINATE. On the CCR-Terminate, return a
			// 2001 CCA.
			if req.CCRequestType == messaging.CCRTypeInitial {
				return &messaging.CCA{
					SessionID:  req.SessionID,
					ResultCode: 2001,
					FUIAction:  messaging.FUIActionTerminate,
				}, nil
			}
			return &messaging.CCA{
				SessionID:  req.SessionID,
				ResultCode: 2001,
				FUIAction:  -1,
			}, nil
		},
	}

	var ccrtBuilds atomic.Int32
	b := New(inner, Options{
		CCRTerminate: func(sessionID string, ccRequestNumber uint32) *messaging.CCR {
			ccrtBuilds.Add(1)
			if sessionID != "sess-1" {
				t.Errorf("CCRTerminate sessionID = %q; want sess-1", sessionID)
			}
			return &messaging.CCR{
				SessionID:        sessionID,
				DestinationRealm: "test.local",
				ServiceContextID: "32251@3gpp.org",
				CCRequestNumber:  ccRequestNumber,
			}
		},
	})
	defer b.Stop()

	cca, err := b.Send(context.Background(), "p1", validCCR())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cca.FUIAction != messaging.FUIActionTerminate {
		t.Errorf("CCA FUIAction = %d; want TERMINATE", cca.FUIAction)
	}

	// The CCR-T is sent in a goroutine — wait briefly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ccrtBuilds.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ccrtBuilds.Load() != 1 {
		t.Errorf("CCR-Terminate builder calls = %d; want 1", ccrtBuilds.Load())
	}

	// Session should be marked terminated.
	if !b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated(sess-1) = false; want true")
	}

	// A subsequent Send should fail with ErrSessionTerminated.
	_, err = b.Send(context.Background(), "p1", validCCR())
	if !errors.Is(err, ErrSessionTerminated) {
		t.Errorf("Send after terminate err = %v; want ErrSessionTerminated", err)
	}
}

// Test 2 (AC-12 — no builder) — FUI=TERMINATE without a builder
// still marks the session terminated; no CCR-T is fired.
func TestBehaviour_FUITerminateWithoutBuilder(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:  req.SessionID,
				ResultCode: 2001,
				FUIAction:  messaging.FUIActionTerminate,
			}, nil
		},
	}
	b := New(inner, Options{})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Give any leaked goroutine time to fire — we expect none.
	time.Sleep(50 * time.Millisecond)

	if !b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated = false; want true")
	}
	// Only the original Send should have hit the inner sender.
	if got := inner.callCount(); got != 1 {
		t.Errorf("inner Send calls = %d; want 1", got)
	}
}

// Test 3 (AC-13) — 5xxx Result-Code marks the session terminated;
// no CCR-T fires; subsequent Send returns ErrSessionTerminated.
func TestBehaviour_PermanentFailureTerminates(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:  req.SessionID,
				ResultCode: 5012, // DIAMETER_UNABLE_TO_COMPLY
				FUIAction:  -1,
			}, nil
		},
	}
	var ccrtBuilds atomic.Int32
	b := New(inner, Options{
		CCRTerminate: func(string, uint32) *messaging.CCR {
			ccrtBuilds.Add(1)
			return nil
		},
	})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated = false; want true")
	}
	// 5xxx must NOT trigger CCR-T — that's only FUI's path.
	time.Sleep(50 * time.Millisecond)
	if got := ccrtBuilds.Load(); got != 0 {
		t.Errorf("CCR-Terminate builder calls = %d; want 0 (5xxx is not FUI)", got)
	}

	// Subsequent Send should fail.
	_, err := b.Send(context.Background(), "p1", validCCR())
	if !errors.Is(err, ErrSessionTerminated) {
		t.Errorf("Send after 5xxx err = %v; want ErrSessionTerminated", err)
	}
}

// Test 4 (AC-13) — 4010 (Credit-Limit-Reached, transient) is NOT
// in the 5xxx range and must not terminate the session.
func TestBehaviour_TransientFailureDoesNotTerminate(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:  req.SessionID,
				ResultCode: 4010, // CREDIT_LIMIT_REACHED — transient
				FUIAction:  -1,
			}, nil
		},
	}
	b := New(inner, Options{})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated for 4010 = true; want false")
	}
}

// Test 5 (AC-11) — Validity-Time schedules a re-auth timer and
// firing it issues a CCR-Update via the builder.
func TestBehaviour_ValidityTimeSchedulesReAuth(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			// Initial CCR returns Validity-Time = 30s.
			// CCR-Update returns Validity-Time = 0 (no further
			// re-auth).
			validity := uint32(0)
			if req.CCRequestType == messaging.CCRTypeInitial {
				validity = 30
			}
			return &messaging.CCA{
				SessionID:    req.SessionID,
				ResultCode:   2001,
				FUIAction:    -1,
				ValidityTime: validity,
			}, nil
		},
	}

	sched := &fakeScheduler{}
	var reauthBuilds atomic.Int32
	b := New(inner, Options{
		AfterFunc: sched.AfterFunc(),
		CCRUpdate: func(sessionID string, n uint32) *messaging.CCR {
			reauthBuilds.Add(1)
			if sessionID != "sess-1" {
				t.Errorf("CCRUpdate sessionID = %q; want sess-1", sessionID)
			}
			return &messaging.CCR{
				SessionID:        sessionID,
				DestinationRealm: "test.local",
				ServiceContextID: "32251@3gpp.org",
				CCRequestNumber:  n,
			}
		},
	})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := sched.count(); got != 1 {
		t.Fatalf("scheduled timers = %d; want 1", got)
	}

	// Fire the timer — should produce a CCR-Update.
	sched.last().fire()

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if reauthBuilds.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reauthBuilds.Load() != 1 {
		t.Errorf("CCRUpdate builder calls = %d; want 1", reauthBuilds.Load())
	}

	// The re-auth Send must have hit the inner sender.
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if inner.callCount() == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := inner.callCount(); got != 2 {
		t.Errorf("inner Send calls after reauth = %d; want 2", got)
	}
	// And the second call should have been CCR-Update.
	calls := inner.callsFor("sess-1")
	if len(calls) < 2 {
		t.Fatalf("calls for sess-1 = %d; want at least 2", len(calls))
	}
	second := calls[1]
	if second.req.CCRequestType != messaging.CCRTypeUpdate {
		t.Errorf("second call CCRequestType = %d; want Update (%d)",
			second.req.CCRequestType, messaging.CCRTypeUpdate)
	}
}

// Test 6 (AC-11) — a later CCA carrying a fresh Validity-Time
// cancels the prior timer and reschedules.
func TestBehaviour_ValidityTimeRefreshCancelsPrior(t *testing.T) {
	t.Parallel()

	respValidity := uint32(30)
	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:    req.SessionID,
				ResultCode:   2001,
				FUIAction:    -1,
				ValidityTime: respValidity,
			}, nil
		},
	}
	sched := &fakeScheduler{}
	b := New(inner, Options{
		AfterFunc: sched.AfterFunc(),
		CCRUpdate: func(string, uint32) *messaging.CCR { return validCCR() },
	})
	defer b.Stop()

	// First Send schedules timer 1.
	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("first Send: %v", err)
	}
	t1 := sched.last()
	if t1 == nil {
		t.Fatalf("no timer scheduled after first Send")
	}

	// Second Send (e.g. an Update with a fresh Validity-Time)
	// schedules timer 2 and cancels timer 1.
	r2 := validCCR()
	r2.CCRequestType = messaging.CCRTypeUpdate
	r2.CCRequestNumber = 1
	if _, err := b.Send(context.Background(), "p1", r2); err != nil {
		t.Fatalf("second Send: %v", err)
	}
	if got := sched.count(); got != 2 {
		t.Fatalf("scheduled timers = %d; want 2", got)
	}
	if !t1.stopped.Load() {
		t.Errorf("first timer not stopped after refresh")
	}
}

// Test 7 — Validity-Time is ignored when the session was just
// terminated by the same CCA's FUI=TERMINATE.
func TestBehaviour_ValidityTimeIgnoredOnTerminatedCCA(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:    req.SessionID,
				ResultCode:   2001,
				FUIAction:    messaging.FUIActionTerminate,
				ValidityTime: 30, // would normally schedule
			}, nil
		},
	}
	sched := &fakeScheduler{}
	b := New(inner, Options{
		AfterFunc: sched.AfterFunc(),
		CCRUpdate: func(string, uint32) *messaging.CCR { return validCCR() },
	})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got := sched.count(); got != 0 {
		t.Errorf("scheduled timers on terminated CCA = %d; want 0", got)
	}
}

// Test 8 — Stop cancels every outstanding timer and marks all
// tracked sessions terminated.
func TestBehaviour_StopCancelsTimers(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:    req.SessionID,
				ResultCode:   2001,
				FUIAction:    -1,
				ValidityTime: 30,
			}, nil
		},
	}
	sched := &fakeScheduler{}
	b := New(inner, Options{
		AfterFunc: sched.AfterFunc(),
		CCRUpdate: func(string, uint32) *messaging.CCR { return validCCR() },
	})

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sched.count() != 1 {
		t.Fatalf("scheduled timers = %d; want 1", sched.count())
	}
	b.Stop()
	if !sched.last().stopped.Load() {
		t.Errorf("timer not stopped by Stop()")
	}
	if !b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated after Stop = false; want true")
	}
	// Stop is idempotent.
	b.Stop()
}

// Test 9 — inner sender error propagates without observing the
// (nil) CCA.
func TestBehaviour_InnerErrorPropagates(t *testing.T) {
	t.Parallel()

	want := errors.New("inner failed")
	inner := &fakeSender{
		resp: func(_ *messaging.CCR) (*messaging.CCA, error) {
			return nil, want
		},
	}
	b := New(inner, Options{})
	defer b.Stop()
	_, err := b.Send(context.Background(), "p1", validCCR())
	if !errors.Is(err, want) {
		t.Errorf("Send err = %v; want %v", err, want)
	}
	// Session must NOT be marked terminated by an inner error.
	if b.SessionTerminated("sess-1") {
		t.Errorf("SessionTerminated on inner error = true; want false")
	}
}

// Test 10 — nil inner Sender panics.
func TestNew_NilInnerPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil inner")
		}
	}()
	New(nil, Options{})
}

// Test 11 — isPermanentFailure boundary cases.
func TestIsPermanentFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code uint32
		want bool
	}{
		{0, false},
		{2001, false},
		{4010, false}, // transient
		{4999, false},
		{5000, true},
		{5012, true},
		{5999, true},
		{6000, false},
	}
	for _, tc := range cases {
		if got := isPermanentFailure(tc.code); got != tc.want {
			t.Errorf("isPermanentFailure(%d) = %v; want %v", tc.code, got, tc.want)
		}
	}
}

// Test 12 — short Validity-Time falls back to minScheduleDelay
// rather than producing a non-positive timer.
func TestBehaviour_ShortValidityTimeUsesFloor(t *testing.T) {
	t.Parallel()

	inner := &fakeSender{
		resp: func(req *messaging.CCR) (*messaging.CCA, error) {
			return &messaging.CCA{
				SessionID:    req.SessionID,
				ResultCode:   2001,
				FUIAction:    -1,
				ValidityTime: 1, // less than the 5s margin
			}, nil
		},
	}
	sched := &fakeScheduler{}
	b := New(inner, Options{
		AfterFunc:    sched.AfterFunc(),
		CCRUpdate:    func(string, uint32) *messaging.CCR { return validCCR() },
		ReAuthMargin: 5 * time.Second,
	})
	defer b.Stop()

	if _, err := b.Send(context.Background(), "p1", validCCR()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sched.count() != 1 {
		t.Errorf("scheduled timers = %d; want 1 (floor must apply)", sched.count())
	}
}
