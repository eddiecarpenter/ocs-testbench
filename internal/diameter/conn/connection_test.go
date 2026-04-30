package conn

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/diamtest"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
)

// settingsServer returns a *sm.Settings suitable for the in-process
// diamtest server used by every connection test. Constants taken
// from go-diameter's own client_test.go fixtures so the CER/CEA
// negotiation succeeds against the library's default state machine
// without surprises.
func settingsServer(originHost string) *sm.Settings {
	return &sm.Settings{
		OriginHost:  diameterIdent(originHost),
		OriginRealm: diameterIdent("test"),
		VendorID:    13,
		ProductName: "go-diameter-test-srv",
	}
}

// diameterIdent renders a string as the go-diameter typed
// DiameterIdentity AVP value. Centralised here so the tests don't
// repeat the import dance in every fixture.
func diameterIdent(s string) datatypeIdentity {
	return datatypeIdentity(s)
}

// datatypeIdentity is a local alias for datatype.DiameterIdentity
// so the helper above does not need to import datatype directly
// from every test function. The alias has the same underlying
// type and is interchangeable.
type datatypeIdentity = datatype_DiameterIdentity

// We re-export the underlying go-diameter datatype.DiameterIdentity
// as a local typedef so tests stay short and only the helper
// function above touches the library type. Defined as the same
// underlying string type so assignment is implicit.
//
// (Defined in this file to keep the test fixtures self-contained.)
//
//nolint:revive // tightly scoped helper alias

type datatype_DiameterIdentity = datatype.DiameterIdentity

// makeServer spins up a loopback diamtest server with a default
// state machine. Returns the server (caller defers Close) and the
// dial address string the client will connect to.
func makeServer(t *testing.T, originHost string) (*diamtest.Server, string) {
	t.Helper()
	srv := diamtest.NewServer(sm.New(settingsServer(originHost)), dict.Default)
	return srv, srv.Addr
}

// awaitState reads from the subscriber channel until target is
// observed or timeout elapses. Returns the slice of all events
// seen up to and including target. Helps the tests wait
// deterministically without sleeping for arbitrary durations.
func awaitState(
	t *testing.T,
	ch <-chan diameter.StateEvent,
	target diameter.ConnectionState,
	timeout time.Duration,
) []diameter.StateEvent {
	t.Helper()
	var seen []diameter.StateEvent
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed before reaching state %s; saw %v", target, seen)
			}
			seen = append(seen, ev)
			if ev.To == target {
				return seen
			}
		case <-deadline.C:
			t.Fatalf("timeout %s waiting for state %s; saw %v", timeout, target, seen)
		}
	}
}

// peerConfigForServer renders a PeerConfig that points at addr.
// Always uses TCP and a tiny watchdog interval so DWR-related
// tests don't have to wait the production default.
func peerConfigForServer(name, addr string) diameter.PeerConfig {
	host, port := splitHostPort(addr)
	return diameter.PeerConfig{
		Name:               name,
		Host:               host,
		Port:               port,
		OriginHost:         "client.ocs-testbench",
		OriginRealm:        "test",
		Transport:          diameter.TransportTCP,
		WatchdogInterval:   100 * time.Millisecond,
		AuthApplicationIDs: []uint32{diameter.AppIDCreditControl},
	}
}

// splitHostPort is a permissive splitter — it returns "host" and
// integer port. diamtest gives us 127.0.0.1:NNNNN so a manual
// rsplit is fine; net.SplitHostPort would be more robust if we
// expected v6 addresses, but the test fixture is always v4.
func splitHostPort(addr string) (string, int) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			port := 0
			for _, c := range addr[i+1:] {
				port = port*10 + int(c-'0')
			}
			return addr[:i], port
		}
	}
	return addr, 0
}

// Test 1 (AC-1) — connect happy path. Spin up an in-process
// go-diameter server, point a PeerConnection at it, observe the
// disconnected → connecting → connected sequence on the subscriber
// channel.
func TestPeerConnection_ConnectHappyPath(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "srv1")
	defer srv.Close()

	cfg := peerConfigForServer("p1", addr)
	pc := New(cfg, dict.Default)
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	seen := awaitState(t, ch, diameter.StateConnected, 5*time.Second)
	// We expect at least:
	//   disconnected → connecting   ("dialling …")
	//   connecting   → connected    ("CER/CEA OK")
	if len(seen) < 2 {
		t.Fatalf("expected at least two transitions; got %v", seen)
	}
	if seen[0].From != diameter.StateDisconnected || seen[0].To != diameter.StateConnecting {
		t.Errorf("first transition = %v→%v; want disconnected→connecting", seen[0].From, seen[0].To)
	}
	last := seen[len(seen)-1]
	if last.From != diameter.StateConnecting || last.To != diameter.StateConnected {
		t.Errorf("last transition = %v→%v; want connecting→connected", last.From, last.To)
	}
	if pc.State() != diameter.StateConnected {
		t.Errorf("State() = %v; want connected", pc.State())
	}
	if pc.Conn() == nil {
		t.Errorf("Conn() = nil; want a live diam.Conn")
	}
}

// Test 2 (AC-2) — DWR/DWA watchdog round-trip. The server's state
// machine answers DWR with DWA automatically; we verify the
// connection stays up across the watchdog interval. Indirect
// assertion: the connection remains in StateConnected for several
// watchdog cycles without a drop.
func TestPeerConnection_WatchdogKeepsConnectionAlive(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "srv-wd")
	defer srv.Close()

	cfg := peerConfigForServer("wd", addr)
	cfg.WatchdogInterval = 50 * time.Millisecond
	pc := New(cfg, dict.Default)
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	awaitState(t, ch, diameter.StateConnected, 5*time.Second)

	// Wait several watchdog cycles. If DWR/DWA are not running the
	// peer would still stay up (TCP keepalive isn't tied to DWR);
	// the load-bearing assertion is "no drop event observed in the
	// window". We drain unscheduled events and ensure none of them
	// is a connected→* transition.
	deadline := time.NewTimer(400 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed unexpectedly")
			}
			if ev.From == diameter.StateConnected {
				t.Fatalf("unexpected drop during watchdog window: %+v", ev)
			}
		case <-deadline.C:
			if pc.State() != diameter.StateConnected {
				t.Errorf("State() = %v; want still connected", pc.State())
			}
			return
		}
	}
}

// Test 3 (AC-3) — drop-then-reconnect. Connect to a fresh server,
// close the live conn from our side to simulate a drop, observe
// connected → disconnected, then observe at least one reconnect
// attempt (the address is gone so reconnects fail, but the loop
// must keep ticking).
//
// Closing the client's diam.Conn directly (rather than the
// diamtest server) is the deterministic path: diam.Server.Close
// only closes listeners, leaving accepted connections alive, so it
// is not a reliable drop-trigger.
func TestPeerConnection_DropThenReconnect(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "srv-r1")
	defer srv.Close()
	cfg := peerConfigForServer("r", addr)
	pc := New(cfg, dict.Default)
	pc.backoffInitial = 50 * time.Millisecond
	pc.backoffCap = 200 * time.Millisecond
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	awaitState(t, ch, diameter.StateConnected, 5*time.Second)

	// Simulate drop by closing our side. CloseNotify fires for both
	// peers when either side closes the underlying TCP connection.
	live := pc.Conn()
	if live == nil {
		t.Fatalf("Conn() = nil while connected")
	}
	live.Close()

	awaitState(t, ch, diameter.StateDisconnected, 5*time.Second)

	// Expect a subsequent reconnecting attempt — the loop must keep
	// trying until Disconnect.
	gotConnecting := false
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for !gotConnecting {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed unexpectedly")
			}
			if ev.To == diameter.StateConnecting {
				gotConnecting = true
			}
		case <-deadline.C:
			t.Fatalf("expected at least one reconnect-attempt transition; got none")
		}
	}
}

// Test 4 (AC-3) — backoff loop ticks repeatedly. After a drop and
// subsequent dial failure (server has been closed), the lifecycle
// goroutine must keep emitting connecting events; the count is
// the assertion that backoff is doubling-with-cap, not collapsing
// to a single attempt and giving up.
func TestPeerConnection_BackoffLoopKeepsTicking(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "stable-1")
	cfg := peerConfigForServer("stable", addr)
	cfg.WatchdogInterval = 50 * time.Millisecond
	pc := New(cfg, dict.Default)
	pc.backoffInitial = 30 * time.Millisecond
	pc.backoffCap = 60 * time.Millisecond
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	awaitState(t, ch, diameter.StateConnected, 5*time.Second)

	// Stop the server's listener so subsequent reconnect attempts
	// fail (the listener is the only thing accepting new connects),
	// then close the live conn so the lifecycle goroutine observes
	// a drop and starts the backoff loop.
	srv.Close()
	pc.Conn().Close()

	awaitState(t, ch, diameter.StateDisconnected, 5*time.Second)

	connectingAttempts := 0
	deadline := time.NewTimer(800 * time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("subscriber channel closed unexpectedly")
			}
			if ev.To == diameter.StateConnecting {
				connectingAttempts++
			}
		case <-deadline.C:
			if connectingAttempts < 2 {
				t.Errorf("expected at least 2 reconnect attempts; got %d", connectingAttempts)
			}
			return
		}
	}
}

// Test 5 — manual Disconnect halts the reconnect loop. After
// Disconnect, no further state events arrive and the goroutine
// has exited (channel closed).
func TestPeerConnection_DisconnectHaltsReconnect(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "halt-1")
	cfg := peerConfigForServer("halt", addr)
	cfg.WatchdogInterval = 50 * time.Millisecond
	pc := New(cfg, dict.Default)
	pc.backoffInitial = 50 * time.Millisecond
	pc.backoffCap = 100 * time.Millisecond
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	awaitState(t, ch, diameter.StateConnected, 5*time.Second)
	// Force a drop locally and tear the server's listener down so
	// reconnect attempts fail.
	pc.Conn().Close()
	srv.Close()
	awaitState(t, ch, diameter.StateDisconnected, 5*time.Second)

	// Wait briefly to allow at least one reconnect attempt.
	time.Sleep(150 * time.Millisecond)
	pc.Disconnect()

	// After Disconnect, the subscriber channel is closed.
	deadline := time.NewTimer(1 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				// Channel closed — goroutine has exited cleanly.
				if pc.State() != diameter.StateDisconnected {
					t.Errorf("State() = %v; want disconnected after Disconnect", pc.State())
				}
				return
			}
			// Drain any remaining transitions; they are valid
			// (final disconnected emission).
		case <-deadline.C:
			t.Fatalf("subscriber channel never closed after Disconnect")
		}
	}
}

// Test 6 (AC-5) — TLS handshake against a self-signed test cert.
// Uses diamtest's StartTLS which loads its bundled localhost
// keypair; the client uses InsecureSkipVerify=true so the
// self-signed cert is accepted.
func TestPeerConnection_TLSHandshake(t *testing.T) {
	t.Parallel()

	srv := diamtest.NewUnstartedServer(sm.New(settingsServer("srv-tls")), dict.Default)
	srv.StartTLS()
	defer srv.Close()

	cfg := peerConfigForServer("tls-peer", srv.Addr)
	cfg.Transport = diameter.TransportTLS
	cfg.TLSInsecureSkipVerify = true

	pc := New(cfg, dict.Default)
	ch := pc.Subscribe()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	awaitState(t, ch, diameter.StateConnected, 5*time.Second)
	if pc.State() != diameter.StateConnected {
		t.Errorf("State() = %v; want connected over TLS", pc.State())
	}
}

// Test 7 (AC-6) — concurrent subscribers all see every transition.
// Two subscribers, both should observe disconnected → connecting →
// connected.
func TestPeerConnection_SubscribeFanout(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "fanout")
	defer srv.Close()

	cfg := peerConfigForServer("fanout", addr)
	pc := New(cfg, dict.Default)

	a := pc.Subscribe()
	b := pc.Subscribe()
	if got := pc.subs.count(); got != 2 {
		t.Fatalf("subscriber count = %d; want 2", got)
	}

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	var wg sync.WaitGroup
	wg.Add(2)
	var aCount, bCount atomic.Int32
	check := func(ch <-chan diameter.StateEvent, counter *atomic.Int32) {
		defer wg.Done()
		seen := awaitState(t, ch, diameter.StateConnected, 5*time.Second)
		counter.Store(int32(len(seen)))
	}
	go check(a, &aCount)
	go check(b, &bCount)
	wg.Wait()

	if aCount.Load() < 2 || bCount.Load() < 2 {
		t.Errorf("subscribers saw mismatched event counts: a=%d, b=%d", aCount.Load(), bCount.Load())
	}
}

// Test 8 — Connect rejects a config with no host.
func TestPeerConnection_ValidationRejectsBadConfig(t *testing.T) {
	t.Parallel()

	cfg := diameter.PeerConfig{
		Name:        "bad",
		Host:        "",
		Port:        3868,
		OriginHost:  "x",
		OriginRealm: "y",
	}
	pc := New(cfg, dict.Default)
	err := pc.Connect(context.Background())
	if err == nil {
		t.Fatalf("Connect: expected ErrInvalidPeerConfig; got nil")
	}
	// Disconnect should be safe even though we never started.
	pc.Disconnect()
}

// Test 9 — Connect rejects a second concurrent Connect.
func TestPeerConnection_DoubleConnectRejected(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "dbl")
	defer srv.Close()

	cfg := peerConfigForServer("dbl", addr)
	pc := New(cfg, dict.Default)
	defer pc.Disconnect()

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	if err := pc.Connect(context.Background()); err == nil {
		t.Fatalf("second Connect: expected ErrPeerAlreadyConnected; got nil")
	}
}

// Test 10 — context cancellation tears the lifecycle down cleanly.
func TestPeerConnection_ContextCancel(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "ctxc")
	defer srv.Close()

	cfg := peerConfigForServer("ctxc", addr)
	pc := New(cfg, dict.Default)
	ch := pc.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	if err := pc.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	awaitState(t, ch, diameter.StateConnected, 5*time.Second)
	cancel()

	// The lifecycle goroutine should exit; subsequent Disconnect
	// is a no-op.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	done := make(chan struct{})
	go func() {
		pc.Disconnect()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-deadline.C:
		t.Fatalf("Disconnect after ctx cancel did not complete")
	}
	if pc.State() != diameter.StateDisconnected {
		t.Errorf("State() = %v; want disconnected", pc.State())
	}
}

// Test 11 — nextBackoff doubles to cap and stays there.
func TestNextBackoff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cur, want time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{30 * time.Second, 60 * time.Second},
		{60 * time.Second, 60 * time.Second},
		{120 * time.Second, 60 * time.Second},
	}
	for _, c := range cases {
		got := nextBackoff(c.cur, 2.0, 60*time.Second)
		if got != c.want {
			t.Errorf("nextBackoff(%s) = %s; want %s", c.cur, got, c.want)
		}
	}
}

// Test 12 — sleepCtx returns false when ctx is cancelled mid-sleep.
func TestSleepCtx(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	if sleepCtx(ctx, 200*time.Millisecond) {
		t.Errorf("sleepCtx returned true despite cancellation")
	}
}

// Test 13 — Unsubscribe stops a specific subscriber from receiving
// further events without affecting others.
func TestPeerConnection_Unsubscribe(t *testing.T) {
	t.Parallel()

	srv, addr := makeServer(t, "unsub")
	defer srv.Close()

	cfg := peerConfigForServer("unsub", addr)
	pc := New(cfg, dict.Default)

	a := pc.Subscribe()
	b := pc.Subscribe()
	pc.Unsubscribe(a)

	if got := pc.subs.count(); got != 1 {
		t.Fatalf("subscriber count after unsubscribe = %d; want 1", got)
	}
	// a should be closed
	select {
	case _, ok := <-a:
		if ok {
			t.Errorf("a yielded a value; expected channel closed")
		}
	default:
		// Channel may not have been closed yet (Unsubscribe is synchronous, so
		// it must be closed by now). The select default catches the racey-
		// channel scenario; if we hit it the test should report.
		t.Errorf("a not closed after Unsubscribe")
	}

	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()

	// b should still see events.
	awaitState(t, b, diameter.StateConnected, 5*time.Second)
}

// _ explicit reference so the datatype import is recorded as used
// even if a future refactor drops the helper alias.
var _ = datatype.DiameterIdentity("")
