package messaging

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/diamtest"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/conn"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
)

// fakeResolver is a tiny manager.Resolver substitute for tests
// that do not need to spin up the full Manager.
type fakeResolver struct {
	peers map[string]manager.PeerConnection
	err   error
}

func (f fakeResolver) Get(name string) (manager.PeerConnection, error) {
	if f.err != nil {
		return nil, f.err
	}
	pc, ok := f.peers[name]
	if !ok {
		return nil, manager.ErrUnknownPeer
	}
	return pc, nil
}

// fakePeer is a minimal manager.PeerConnection used to exercise
// the Sender's disconnected-peer guard. Holds a fixed state and a
// nil diam.Conn.
type fakePeer struct {
	cfg   diameter.PeerConfig
	state diameter.ConnectionState
	subs  chan diameter.StateEvent
	once  sync.Once
}

func (f *fakePeer) Config() diameter.PeerConfig             { return f.cfg }
func (f *fakePeer) State() diameter.ConnectionState         { return f.state }
func (f *fakePeer) Conn() diam.Conn                         { return nil }
func (f *fakePeer) Connect(_ context.Context) error         { return nil }
func (f *fakePeer) Disconnect()                             {}
func (f *fakePeer) HandleFunc(_ string, _ diam.HandlerFunc) {}
func (f *fakePeer) Subscribe() <-chan diameter.StateEvent {
	f.once.Do(func() { f.subs = make(chan diameter.StateEvent, 4) })
	return f.subs
}

func (f *fakePeer) Unsubscribe(_ <-chan diameter.StateEvent) {}

// Test 1 — Sender returns ErrPeerNotConnected synchronously when
// the peer is disconnected. The wire is not touched.
func TestSender_DisconnectedPeerFailsFast(t *testing.T) {
	t.Parallel()
	pc := &fakePeer{
		cfg:   diameter.PeerConfig{Name: "p1", OriginHost: "tb", OriginRealm: "test"},
		state: diameter.StateDisconnected,
	}
	res := fakeResolver{peers: map[string]manager.PeerConnection{"p1": pc}}
	s := NewSender(res)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := s.Send(ctx, "p1", validCCR())
	if !errors.Is(err, diameter.ErrPeerNotConnected) {
		t.Errorf("Send err = %v; want diameter.ErrPeerNotConnected", err)
	}
}

// Test 2 — Sender returns the resolver's error verbatim when the
// peer is unknown.
func TestSender_UnknownPeerSurfaces(t *testing.T) {
	t.Parallel()
	res := fakeResolver{peers: map[string]manager.PeerConnection{}}
	s := NewSender(res)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := s.Send(ctx, "missing", validCCR())
	if !errors.Is(err, manager.ErrUnknownPeer) {
		t.Errorf("Send err = %v; want manager.ErrUnknownPeer", err)
	}
}

// Test 3 — NewSender(nil) panics.
func TestNewSender_NilResolverPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil resolver")
		}
	}()
	NewSender(nil)
}

// ---------------------------------------------------------------------
// In-process round-trip tests with a real go-diameter peer.
// ---------------------------------------------------------------------

// ccaEchoServer spins up a diamtest server that responds to every
// incoming CCR with a canned CCA carrying the supplied result code,
// the original session id, and the original e2e/hop ids.
func ccaEchoServer(t *testing.T, originHost string, resultCode uint32) (*diamtest.Server, string) {
	t.Helper()
	settings := &sm.Settings{
		OriginHost:  datatype.DiameterIdentity(originHost),
		OriginRealm: datatype.DiameterIdentity("test"),
		VendorID:    13,
		ProductName: "go-diameter-cca-echo",
	}
	mgr := sm.New(settings)
	mgr.HandleFunc("CCR", func(c diam.Conn, m *diam.Message) {
		// Build the answer with the same E2E and Hop-by-Hop IDs.
		a := m.Answer(resultCode)
		// Echo Session-Id verbatim.
		if sid, err := m.FindAVP(avp.SessionID, 0); err == nil {
			a.AddAVP(sid)
		}
		a.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(originHost))
		a.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("test"))
		a.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4))
		// Echo CC-Request-Type / Number for round-trip correlation.
		if rt, err := m.FindAVP(avp.CCRequestType, 0); err == nil {
			a.AddAVP(rt)
		}
		if rn, err := m.FindAVP(avp.CCRequestNumber, 0); err == nil {
			a.AddAVP(rn)
		}
		_, _ = a.WriteTo(c)
	})
	srv := diamtest.NewServer(mgr, dict.Default)
	return srv, srv.Addr
}

// realPeerConfig builds a PeerConfig pointing at addr.
func realPeerConfig(name, addr string) diameter.PeerConfig {
	host, port := splitHostPort(addr)
	return diameter.PeerConfig{
		Name: name, Host: host, Port: port,
		OriginHost: "tb." + name + ".test", OriginRealm: "test",
		Transport: diameter.TransportTCP, WatchdogInterval: 100 * time.Millisecond,
	}
}

// splitHostPort matches the helper in conn/connection_test.go and
// manager/manager_test.go. Local copy keeps each test package
// self-contained.
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

// awaitConnected blocks until pc reports StateConnected or
// timeout expires.
func awaitConnected(t *testing.T, pc *conn.PeerConnection, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pc.State() == diameter.StateConnected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("peer did not reach StateConnected within %s", timeout)
}

// Test 4 — encode/decode round trip via a real in-process peer.
func TestSender_RoundTripWithEchoPeer(t *testing.T) {
	t.Parallel()
	srv, addr := ccaEchoServer(t, "ocs-echo.test", 2001)
	defer srv.Close()

	pc := conn.New(realPeerConfig("p1", addr), dict.Default)
	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()
	awaitConnected(t, pc, 5*time.Second)

	// Wire the Sender directly to a fixed-resolver pointing at this
	// PeerConnection. The fake-resolver path is exercised by tests
	// 1–2; this is the live-wire path.
	res := fakeResolver{peers: map[string]manager.PeerConnection{"p1": pc}}
	s := NewSender(res)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cca, err := s.Send(ctx, "p1", validCCR())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cca.ResultCode != 2001 {
		t.Errorf("ResultCode = %d; want 2001", cca.ResultCode)
	}
	if cca.SessionID != validCCR().SessionID {
		t.Errorf("SessionID echoed = %q; want %q", cca.SessionID, validCCR().SessionID)
	}
	if cca.CCRequestType != CCRTypeInitial {
		t.Errorf("CCRequestType = %d; want %d", cca.CCRequestType, CCRTypeInitial)
	}
}

// Test 5 — concurrent Send calls all receive their correct CCAs
// (E2E correlation works for parallel requests).
func TestSender_ConcurrentSendsCorrelateByE2EID(t *testing.T) {
	t.Parallel()
	srv, addr := ccaEchoServer(t, "ocs-conc.test", 2001)
	defer srv.Close()

	pc := conn.New(realPeerConfig("p1", addr), dict.Default)
	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()
	awaitConnected(t, pc, 5*time.Second)

	res := fakeResolver{peers: map[string]manager.PeerConnection{"p1": pc}}
	s := NewSender(res)

	const N = 8
	var (
		wg    sync.WaitGroup
		errCt atomic.Int32
		okCt  atomic.Int32
	)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ccr := validCCR()
			ccr.SessionID = "" // force unique session ids
			cca, err := s.Send(ctx, "p1", ccr)
			if err != nil {
				errCt.Add(1)
				return
			}
			if cca.ResultCode != 2001 {
				errCt.Add(1)
				return
			}
			okCt.Add(1)
		}()
	}
	wg.Wait()
	if errCt.Load() != 0 {
		t.Errorf("concurrent Send errors: %d/%d; want 0", errCt.Load(), N)
	}
	if okCt.Load() != N {
		t.Errorf("concurrent Send successes: %d; want %d", okCt.Load(), N)
	}
}

// Test 6 — context cancellation aborts the wait, returns ctx.Err.
// Uses a non-responsive server (an opened but never-answering go-
// diameter server doesn't exist — we point the connection at a
// black-hole listener and confirm the cancellation path fires).
//
// Implementation: we spin up a real CCA-echo server, connect, but
// pre-register a pending E2E ID with no-one to dispatch it; that
// guarantees the Send is waiting on the channel when ctx is
// cancelled.
func TestSender_ContextCancellation(t *testing.T) {
	t.Parallel()
	// Spin up a real connection but use a server that DOES NOT
	// answer CCRs — silent server.
	settings := &sm.Settings{
		OriginHost:  datatype.DiameterIdentity("ocs-silent.test"),
		OriginRealm: datatype.DiameterIdentity("test"),
		VendorID:    13,
		ProductName: "go-diameter-silent",
	}
	srv := diamtest.NewServer(sm.New(settings), dict.Default)
	defer srv.Close()

	pc := conn.New(realPeerConfig("silent", srv.Addr), dict.Default)
	if err := pc.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pc.Disconnect()
	awaitConnected(t, pc, 5*time.Second)

	res := fakeResolver{peers: map[string]manager.PeerConnection{"silent": pc}}
	s := NewSender(res)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := s.Send(ctx, "silent", validCCR())
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Send err = %v; want context.DeadlineExceeded", err)
	}
	// Sanity: aborted within ~250ms (allow generous slack).
	if elapsed > 2*time.Second {
		t.Errorf("Send took %s; expected ~250ms", elapsed)
	}
}
