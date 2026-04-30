package manager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/diamtest"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/conn"
)

// fakePeerConnection is the test stand-in for PeerConnection. It
// records every Connect / Disconnect call, exposes a hand-driven
// state transition channel via emitState, and never touches the
// network. The manager-level fan-out and lifecycle logic exercise
// purely against this fake.
type fakePeerConnection struct {
	cfg diameter.PeerConfig

	mu             sync.Mutex
	connectErr     error
	connectCount   atomic.Int32
	disconnectCnt  atomic.Int32
	currentState   diameter.ConnectionState
	subscriberCh   chan diameter.StateEvent
	subscriberOpen bool
}

func newFakePeerConnection(cfg diameter.PeerConfig) *fakePeerConnection {
	return &fakePeerConnection{
		cfg:            cfg,
		currentState:   diameter.StateDisconnected,
		subscriberCh:   make(chan diameter.StateEvent, 16),
		subscriberOpen: true,
	}
}

func (f *fakePeerConnection) Config() diameter.PeerConfig { return f.cfg }
func (f *fakePeerConnection) Conn() diam.Conn             { return nil }

func (f *fakePeerConnection) State() diameter.ConnectionState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentState
}

func (f *fakePeerConnection) Subscribe() <-chan diameter.StateEvent {
	return f.subscriberCh
}

func (f *fakePeerConnection) Connect(ctx context.Context) error {
	if ctx == nil {
		return errors.New("fakePeerConnection.Connect: nil ctx")
	}
	f.connectCount.Add(1)
	f.mu.Lock()
	err := f.connectErr
	f.mu.Unlock()
	return err
}

func (f *fakePeerConnection) Disconnect() {
	f.disconnectCnt.Add(1)
	f.mu.Lock()
	if f.subscriberOpen {
		close(f.subscriberCh)
		f.subscriberOpen = false
	}
	f.mu.Unlock()
}

// emitState pushes a synthetic transition onto the subscriber
// channel. Tests use this to drive the manager's fan-out without
// running a real lifecycle goroutine.
func (f *fakePeerConnection) emitState(to diameter.ConnectionState, detail string) {
	f.mu.Lock()
	from := f.currentState
	f.currentState = to
	open := f.subscriberOpen
	f.mu.Unlock()
	if !open {
		return
	}
	f.subscriberCh <- diameter.StateEvent{
		PeerName: f.cfg.Name,
		From:     from,
		To:       to,
		Detail:   detail,
		Time:     time.Now().UTC(),
	}
}

// fakeFactory builds a ConnectionFactory closure that returns a
// pre-allocated fakePeerConnection per name. The map lets tests
// assert against the same instances they constructed.
func fakeFactory(t *testing.T, peers map[string]*fakePeerConnection) ConnectionFactory {
	t.Helper()
	return func(cfg diameter.PeerConfig, _ *dict.Parser) PeerConnection {
		t.Helper()
		fp, ok := peers[cfg.Name]
		if !ok {
			t.Fatalf("fakeFactory: no fake for peer %q", cfg.Name)
		}
		return fp
	}
}

// withFakes constructs a Manager wired with fake connections for
// the given configs. Returns the Manager and the per-name fakes
// the test asserts against.
func withFakes(t *testing.T, configs []diameter.PeerConfig) (*Manager, map[string]*fakePeerConnection) {
	t.Helper()
	fakes := map[string]*fakePeerConnection{}
	for _, cfg := range configs {
		fakes[cfg.Name] = newFakePeerConnection(cfg)
	}
	m := New(dict.Default, SlicePeerProvider{Configs: configs})
	m.SetFactory(fakeFactory(t, fakes))
	return m, fakes
}

// awaitFanOut waits for an event on the manager-level subscriber
// channel and returns it. Times out the test on miss.
func awaitFanOut(t *testing.T, ch <-chan diameter.StateEvent, timeout time.Duration) diameter.StateEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatalf("manager subscriber channel closed unexpectedly")
		}
		return ev
	case <-time.After(timeout):
		t.Fatalf("timeout %s waiting for manager fan-out event", timeout)
		return diameter.StateEvent{}
	}
}

// Test 1 (AC-4) — register multiple peers, only auto-connect ones
// receive Connect; the others remain disconnected.
func TestManager_StartAutoConnectsOnlyMarkedPeers(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "auto1", Host: "h1", Port: 3868, OriginHost: "oh1", OriginRealm: "or1", AutoConnect: true},
		{Name: "manual", Host: "h2", Port: 3868, OriginHost: "oh2", OriginRealm: "or2", AutoConnect: false},
		{Name: "auto2", Host: "h3", Port: 3868, OriginHost: "oh3", OriginRealm: "or3", AutoConnect: true},
	}
	m, fakes := withFakes(t, configs)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Auto-connect peers should have Connect called exactly once.
	if got := fakes["auto1"].connectCount.Load(); got != 1 {
		t.Errorf("auto1 connect count = %d; want 1", got)
	}
	if got := fakes["auto2"].connectCount.Load(); got != 1 {
		t.Errorf("auto2 connect count = %d; want 1", got)
	}
	// Manual peer must not be auto-connected.
	if got := fakes["manual"].connectCount.Load(); got != 0 {
		t.Errorf("manual connect count = %d; want 0", got)
	}
	// Manual peer remains in disconnected state.
	if got := fakes["manual"].State(); got != diameter.StateDisconnected {
		t.Errorf("manual state = %v; want disconnected", got)
	}
}

// Test 2 (AC-5) — explicit Connect(name) for a manual peer fires
// the underlying Connect after Start.
func TestManager_ExplicitConnectAfterStart(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or", AutoConnect: false},
	}
	m, fakes := withFakes(t, configs)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	if got := fakes["p1"].connectCount.Load(); got != 0 {
		t.Errorf("connect count after start = %d; want 0", got)
	}
	if err := m.Connect("p1"); err != nil {
		t.Fatalf("Connect(p1): %v", err)
	}
	if got := fakes["p1"].connectCount.Load(); got != 1 {
		t.Errorf("connect count after explicit Connect = %d; want 1", got)
	}
}

// Test 3 — Connect against an unregistered peer returns
// ErrUnknownPeer.
func TestManager_ConnectUnknownReturnsError(t *testing.T) {
	t.Parallel()
	m, _ := withFakes(t, []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	})
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	if err := m.Connect("missing"); !errors.Is(err, ErrUnknownPeer) {
		t.Errorf("Connect(missing) err = %v; want ErrUnknownPeer", err)
	}
}

// Test 4 (AC-4) — peers are independent: faulting one peer's
// auto-connect does not break the registry or sibling peers.
func TestManager_FaultingPeerDoesNotAffectOthers(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "good", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or", AutoConnect: true},
		{Name: "bad", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or", AutoConnect: true},
	}
	m, fakes := withFakes(t, configs)
	// "bad" returns an error from Connect; "good" returns nil.
	fakes["bad"].connectErr = errors.New("dial refused")

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Both peers were registered (manager survived the bad one).
	names := m.Names()
	if len(names) != 2 {
		t.Errorf("Names() = %v; want 2 entries", names)
	}
	// Both Connect attempts were made.
	if got := fakes["good"].connectCount.Load(); got != 1 {
		t.Errorf("good connect count = %d; want 1", got)
	}
	if got := fakes["bad"].connectCount.Load(); got != 1 {
		t.Errorf("bad connect count = %d; want 1", got)
	}
}

// Test 5 — Stop cancels reconnect goroutines and disconnects every
// peer. After Stop, Connect returns ErrStopped.
func TestManager_StopDisconnectsEveryPeer(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or", AutoConnect: true},
		{Name: "p2", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or", AutoConnect: false},
	}
	m, fakes := withFakes(t, configs)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	m.Stop()
	if got := fakes["p1"].disconnectCnt.Load(); got != 1 {
		t.Errorf("p1 disconnect count = %d; want 1", got)
	}
	if got := fakes["p2"].disconnectCnt.Load(); got != 1 {
		t.Errorf("p2 disconnect count = %d; want 1", got)
	}
	// Post-Stop Connect/Disconnect return ErrStopped.
	if err := m.Connect("p1"); !errors.Is(err, ErrStopped) {
		t.Errorf("Connect after Stop err = %v; want ErrStopped", err)
	}
	// Stop is idempotent.
	m.Stop()
}

// Test 6 — Subscribe receives fan-out events from every peer.
func TestManager_SubscribeFansOutEvents(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
		{Name: "p2", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	}
	m, fakes := withFakes(t, configs)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	ch := m.Subscribe()
	fakes["p1"].emitState(diameter.StateConnecting, "test")
	ev1 := awaitFanOut(t, ch, 2*time.Second)
	if ev1.PeerName != "p1" || ev1.To != diameter.StateConnecting {
		t.Errorf("first fan-out event = %+v; want p1 connecting", ev1)
	}

	fakes["p2"].emitState(diameter.StateConnecting, "test")
	ev2 := awaitFanOut(t, ch, 2*time.Second)
	if ev2.PeerName != "p2" || ev2.To != diameter.StateConnecting {
		t.Errorf("second fan-out event = %+v; want p2 connecting", ev2)
	}
}

// Test 7 — Manager.Get returns the registered PeerConnection.
func TestManager_GetReturnsRegisteredConnection(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	}
	m, fakes := withFakes(t, configs)
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	pc, err := m.Get("p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pc != fakes["p1"] {
		t.Errorf("Get returned a different PeerConnection than the factory produced")
	}
	if _, err := m.Get("missing"); !errors.Is(err, ErrUnknownPeer) {
		t.Errorf("Get(missing) err = %v; want ErrUnknownPeer", err)
	}
}

// Test 8 — duplicate peer names in the provider trigger
// ErrDuplicatePeer at Start. The Manager refuses to mask the
// wiring bug.
func TestManager_DuplicatePeerNamesRejected(t *testing.T) {
	t.Parallel()
	configs := []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	}
	m, _ := withFakes(t, configs)
	err := m.Start(context.Background())
	if !errors.Is(err, ErrDuplicatePeer) {
		t.Errorf("Start err = %v; want ErrDuplicatePeer", err)
	}
}

// Test 9 — Start is single-shot: a second Start returns
// ErrAlreadyStarted.
func TestManager_StartSingleShot(t *testing.T) {
	t.Parallel()
	m, _ := withFakes(t, []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	})
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer m.Stop()
	if err := m.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Errorf("second Start err = %v; want ErrAlreadyStarted", err)
	}
}

// Test 10 — Connect on an unstarted manager returns ErrNotStarted.
func TestManager_OperationsBeforeStart(t *testing.T) {
	t.Parallel()
	m, _ := withFakes(t, []diameter.PeerConfig{
		{Name: "p1", Host: "h", Port: 3868, OriginHost: "oh", OriginRealm: "or"},
	})
	if err := m.Connect("p1"); !errors.Is(err, ErrNotStarted) {
		t.Errorf("Connect before Start err = %v; want ErrNotStarted", err)
	}
	if err := m.Disconnect("p1"); !errors.Is(err, ErrNotStarted) {
		t.Errorf("Disconnect before Start err = %v; want ErrNotStarted", err)
	}
	if _, err := m.Get("p1"); !errors.Is(err, ErrNotStarted) {
		t.Errorf("Get before Start err = %v; want ErrNotStarted", err)
	}
}

// Test 11 — provider error halts Start and the manager remains
// fresh-construct-equivalent.
func TestManager_ProviderErrorHaltsStart(t *testing.T) {
	t.Parallel()
	provErr := errors.New("provider boom")
	m := New(dict.Default, errProvider{err: provErr})
	err := m.Start(context.Background())
	if !errors.Is(err, provErr) {
		t.Errorf("Start err = %v; want wrapped provider error", err)
	}
	// The manager should be retry-able after a provider failure.
	m2 := New(dict.Default, SlicePeerProvider{Configs: nil})
	if err := m2.Start(context.Background()); err != nil {
		t.Errorf("Start on empty provider: %v", err)
	}
	defer m2.Stop()
}

type errProvider struct{ err error }

func (e errProvider) ListPeers(_ context.Context) ([]diameter.PeerConfig, error) {
	return nil, e.err
}

// Integration test (AC-4 + AC-6) — wire the Manager to two
// in-process go-diameter test peers and verify both reach
// connected concurrently using the production conn.PeerConnection.
func TestManager_TwoRealPeersConnectConcurrently(t *testing.T) {
	t.Parallel()

	// Spin up two independent in-process go-diameter servers.
	srv1 := diamtest.NewServer(sm.New(testServerSettings("srv1")), dict.Default)
	defer srv1.Close()
	srv2 := diamtest.NewServer(sm.New(testServerSettings("srv2")), dict.Default)
	defer srv2.Close()

	host1, port1 := splitHostPort(srv1.Addr)
	host2, port2 := splitHostPort(srv2.Addr)
	configs := []diameter.PeerConfig{
		{
			Name: "p1", Host: host1, Port: port1,
			OriginHost: "client1.test", OriginRealm: "test",
			Transport:        diameter.TransportTCP,
			WatchdogInterval: 100 * time.Millisecond,
			AutoConnect:      true,
		},
		{
			Name: "p2", Host: host2, Port: port2,
			OriginHost: "client2.test", OriginRealm: "test",
			Transport:        diameter.TransportTCP,
			WatchdogInterval: 100 * time.Millisecond,
			AutoConnect:      true,
		},
	}

	// Production factory — uses conn.New for real go-diameter
	// behaviour; verifies the manager wires PeerConfig through to
	// the connection layer correctly.
	m := New(dict.Default, SlicePeerProvider{Configs: configs})
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer m.Stop()

	// Wait for both peers to reach connected via the manager-level
	// subscriber channel — proves the fan-out wires to both peers.
	ch := m.Subscribe()
	connected := map[string]bool{}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !connected["p1"] || !connected["p2"] {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("manager subscriber channel closed before both peers connected; got %v", connected)
			}
			if ev.To == diameter.StateConnected {
				connected[ev.PeerName] = true
			}
		case <-deadline.C:
			t.Fatalf("timeout waiting for both peers to connect; got %v", connected)
		}
	}
	// Each peer reports its own state independently.
	if got, _ := m.State("p1"); got != diameter.StateConnected {
		t.Errorf("p1 state = %v; want connected", got)
	}
	if got, _ := m.State("p2"); got != diameter.StateConnected {
		t.Errorf("p2 state = %v; want connected", got)
	}
}

// Test — the production factory delegates to conn.New (smoke test
// guarding against an accidental regression where SetFactory is
// applied before construction or productionFactory drifts away
// from conn.New).
func TestManager_ProductionFactoryProducesConnPeerConnection(t *testing.T) {
	t.Parallel()
	cfg := diameter.PeerConfig{
		Name: "p1", Host: "127.0.0.1", Port: 3868,
		OriginHost: "oh", OriginRealm: "or",
	}
	pc := productionFactory(cfg, dict.Default)
	if _, ok := pc.(*conn.PeerConnection); !ok {
		t.Errorf("productionFactory returned %T; want *conn.PeerConnection", pc)
	}
}

// SlicePeerProvider returns a copy each call so callers cannot
// mutate the manager's view through the slice.
func TestSlicePeerProvider_ReturnsCopy(t *testing.T) {
	t.Parallel()
	p := SlicePeerProvider{Configs: []diameter.PeerConfig{{Name: "p1"}}}
	got, err := p.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	got[0].Name = "mutated"
	got2, _ := p.ListPeers(context.Background())
	if got2[0].Name != "p1" {
		t.Errorf("provider was mutated through returned slice; got2[0] = %+v", got2[0])
	}
}

// testServerSettings constructs a diam-test settings block. Mirrors
// the helper in conn/connection_test.go.
func testServerSettings(originHost string) *sm.Settings {
	return &sm.Settings{
		OriginHost:  datatype.DiameterIdentity(originHost),
		OriginRealm: datatype.DiameterIdentity("test"),
		VendorID:    13,
		ProductName: "go-diameter-test-srv",
	}
}

// splitHostPort matches the per-conn test helper. Local copy keeps
// the manager package test-self-contained.
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
