package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/dict"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/conn"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// PeerProvider is the read-only source the Manager consumes at
// Start time to discover the peers it should register. The
// orchestrator (cmd/ocs-testbench/main.go) supplies a production
// implementation that decodes the peer rows out of internal/store;
// tests supply a fixed slice via SlicePeerProvider.
//
// ListPeers is called exactly once per Start. The Manager does not
// subscribe to provider changes — adding or removing a peer at
// runtime is out of scope for Feature #17 and would land as part
// of the future API layer (Feature #8).
type PeerProvider interface {
	// ListPeers returns the peer configs the Manager should
	// register. Returning an error halts Start with that error
	// propagated to the caller. An empty slice is valid and
	// produces a Manager that runs but registers zero peers.
	ListPeers(ctx context.Context) ([]diameter.PeerConfig, error)
}

// SlicePeerProvider is the trivial PeerProvider used by tests and by
// any caller that already has the list of configs in hand. The
// production wiring uses a thin adapter over internal/store.Store
// (see cmd/ocs-testbench/main.go).
type SlicePeerProvider struct {
	Configs []diameter.PeerConfig
}

// ListPeers returns the embedded slice. Always nil error.
func (s SlicePeerProvider) ListPeers(_ context.Context) ([]diameter.PeerConfig, error) {
	return append([]diameter.PeerConfig(nil), s.Configs...), nil
}

// PeerConnection is the slice of conn.PeerConnection the Manager
// hands out via Get. Defined as its own interface so tests can
// substitute a fake without spinning up a goroutine and a real
// go-diameter dialler — the manager-level registry / fan-out
// semantics are independent of the underlying connection's
// go-diameter behaviour. The interface is also the contract Task 4's
// messaging.Sender will consume.
//
// The surface mirrors the public methods of *conn.PeerConnection
// the Manager and its callers actually call. Conn returns the live
// go-diameter connection while the peer is in StateConnected — the
// messaging Sender uses it to write a CCR; the manager itself never
// reaches for it.
type PeerConnection interface {
	Connect(ctx context.Context) error
	Disconnect()
	State() diameter.ConnectionState
	Subscribe() <-chan diameter.StateEvent
	Unsubscribe(ch <-chan diameter.StateEvent)
	Config() diameter.PeerConfig
	Conn() diam.Conn
	HandleFunc(cmd string, handler diam.HandlerFunc)
}

// ConnectionFactory is the seam tests use to substitute a fake
// peer-connection. Production wiring uses the closure constructed
// by New that delegates to conn.New.
type ConnectionFactory func(cfg diameter.PeerConfig, parser *dict.Parser) PeerConnection

// Manager is the multi-peer connection registry.
//
// Construct via New. Lifecycle:
//
//   - New constructs an empty Manager bound to a dictionary parser
//     (typically dict.Default after the loader has populated it).
//   - Start(ctx) reads peers from the PeerProvider, builds one
//     PeerConnection per peer, and fires Connect on every peer
//     marked autoConnect=true. Returns once registration is
//     complete; auto-connect outcomes flow asynchronously through
//     the subscriber channel.
//   - Connect(name) / Disconnect(name) operate on a single peer
//     post-Start (typically used by the future REST layer #8).
//   - Get(name) returns the *conn.PeerConnection (production type)
//     for callers that need the live diam.Conn — the messaging
//     Sender (Task 4) uses this.
//   - Subscribe() returns a manager-level event channel that fans
//     out StateEvents from every registered peer.
//   - Stop() cancels the manager-level context, waits for every
//     PeerConnection's lifecycle goroutine to drain, closes every
//     subscriber channel, and clears the registry.
//
// Concurrency: every public method is goroutine-safe.
type Manager struct {
	parser   *dict.Parser
	provider PeerProvider
	factory  ConnectionFactory

	mu      sync.Mutex
	peers   map[string]*peerEntry
	started bool
	stopped bool

	// rootCtx is the lifecycle context shared by every
	// PeerConnection's Connect call; populated by Start.
	// rootCancel cancels it.
	rootCtx    context.Context
	rootCancel context.CancelFunc
	// fanCtx, fanCancel control the manager's fan-out goroutines —
	// one per registered peer. fanCancel fires before PeerConnection
	// Disconnect so the fan-out stops reading from the per-peer
	// subscriber channel before the peer closes it.
	fanCtx    context.Context
	fanCancel context.CancelFunc
	fanWG     sync.WaitGroup

	// subs holds the manager-level subscribers (fan-out).
	subs *subscriberSet
}

// peerEntry is the manager-level record for one registered peer.
type peerEntry struct {
	conn PeerConnection //nolint:revive // intentional field name; matches manager.PeerConnection interface
	cfg  diameter.PeerConfig
}

// Sentinel errors raised by the Manager. Callers branch on these via
// errors.Is.
var (
	// ErrUnknownPeer is returned by Connect/Disconnect/Get when the
	// supplied name is not registered.
	ErrUnknownPeer = errors.New("manager: unknown peer")
	// ErrAlreadyStarted is returned by Start when the Manager has
	// already been started.
	ErrAlreadyStarted = errors.New("manager: already started")
	// ErrNotStarted is returned by Connect/Disconnect/Get when the
	// Manager has not yet been started.
	ErrNotStarted = errors.New("manager: not started")
	// ErrStopped is returned by Connect/Disconnect after Stop has
	// drained the registry.
	ErrStopped = errors.New("manager: stopped")
	// ErrDuplicatePeer is returned by Start when the provider
	// supplies two configs with the same Name.
	ErrDuplicatePeer = errors.New("manager: duplicate peer name")
)

// New constructs a Manager bound to the given dictionary parser and
// PeerProvider. Pass dict.Default in production so the manager's
// PeerConnections share the dictionary the loader (Task 1)
// populated; tests pass a fresh parser to keep dict.Default
// unmutated.
//
// Panics on a nil parser or a nil provider — both are wiring bugs
// that should fail loudly at startup.
func New(parser *dict.Parser, provider PeerProvider) *Manager {
	if parser == nil {
		panic("manager.New: parser must not be nil")
	}
	if provider == nil {
		panic("manager.New: provider must not be nil")
	}
	return &Manager{
		parser:   parser,
		provider: provider,
		factory:  productionFactory,
		peers:    map[string]*peerEntry{},
		subs:     newSubscriberSet(),
	}
}

// productionFactory is the ConnectionFactory used in production. It
// delegates to conn.New, which spins up a real PeerConnection
// against go-diameter.
func productionFactory(cfg diameter.PeerConfig, parser *dict.Parser) PeerConnection {
	return conn.New(cfg, parser)
}

// SetFactory replaces the connection factory. Tests use this to
// swap in a fake PeerConnection that doesn't dial. Returns the
// previous factory so the caller can restore it via defer.
//
// Production code MUST NOT call this — the default factory
// (productionFactory) is what makes the manager actually open
// connections.
func (m *Manager) SetFactory(f ConnectionFactory) ConnectionFactory {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := m.factory
	if f != nil {
		m.factory = f
	}
	return prev
}

// Start registers every peer the provider returns, wires their
// per-peer subscriber channels into the manager-level fan-out, and
// fires Connect for every peer marked AutoConnect=true.
//
// Start is single-shot — calling it twice returns
// ErrAlreadyStarted.
//
// The supplied ctx becomes the root context for every
// PeerConnection's lifecycle goroutine. Cancelling it has the same
// effect as Stop(). Production wiring passes the application's
// root context so a SIGTERM tears the whole stack down cleanly.
func (m *Manager) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("manager.Start: ctx is nil")
	}

	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return ErrAlreadyStarted
	}
	m.started = true
	rootCtx, rootCancel := context.WithCancel(ctx)
	m.rootCtx = rootCtx
	m.rootCancel = rootCancel
	fanCtx, fanCancel := context.WithCancel(rootCtx)
	m.fanCtx = fanCtx
	m.fanCancel = fanCancel
	factory := m.factory
	m.mu.Unlock()

	configs, err := m.provider.ListPeers(ctx)
	if err != nil {
		// Best-effort cleanup so a failed Start leaves the manager
		// in a fresh-construct equivalent state.
		m.mu.Lock()
		m.started = false
		m.rootCtx = nil
		m.rootCancel = nil
		m.fanCtx = nil
		m.fanCancel = nil
		m.mu.Unlock()
		rootCancel()
		return fmt.Errorf("manager: list peers: %w", err)
	}

	// Detect duplicate names early — registering two peers under
	// the same key is a wiring bug the manager refuses to mask.
	seen := make(map[string]struct{}, len(configs))
	for _, cfg := range configs {
		if _, exists := seen[cfg.Name]; exists {
			m.mu.Lock()
			m.started = false
			m.rootCancel = nil
			m.fanCtx = nil
			m.fanCancel = nil
			m.mu.Unlock()
			rootCancel()
			return fmt.Errorf("%w: %q", ErrDuplicatePeer, cfg.Name)
		}
		seen[cfg.Name] = struct{}{}
	}

	// Register every peer first so a partial-Start failure can be
	// torn down via Stop without leaking a half-built registry.
	autoConnect := make([]string, 0, len(configs))
	for _, cfg := range configs {
		entry := &peerEntry{
			cfg:  cfg,
			conn: factory(cfg, m.parser),
		}
		m.mu.Lock()
		m.peers[cfg.Name] = entry
		m.mu.Unlock()

		// Wire the per-peer subscriber into the manager-level
		// fan-out goroutine. The subscription is taken before any
		// Connect so we do not miss the first transitions.
		ch := entry.conn.Subscribe()
		m.fanWG.Add(1)
		go m.fanOut(fanCtx, cfg.Name, ch)

		if cfg.AutoConnect {
			autoConnect = append(autoConnect, cfg.Name)
		}
	}

	// Auto-connect after every peer is registered so a peer's
	// connect-failure cannot prevent another peer from being
	// registered.
	for _, name := range autoConnect {
		m.mu.Lock()
		entry := m.peers[name]
		m.mu.Unlock()
		if entry == nil {
			continue
		}
		if err := entry.conn.Connect(rootCtx); err != nil {
			// Auto-connect errors are surfaced as warnings — the
			// peer stays registered (operator can retry via
			// Connect(name)) but the Start does not fail. AC-4
			// requires peer independence: a faulting peer must
			// not break Start.
			logging.Warn(
				"manager: auto-connect failed",
				"peer", name,
				"error", err.Error(),
			)
		}
	}

	logging.Info(
		"manager: started",
		"peers_total", len(configs),
		"peers_auto_connect", len(autoConnect),
	)
	return nil
}

// Stop cancels the manager-level context, waits for every
// PeerConnection's lifecycle goroutine to drain, closes every
// manager-level subscriber channel, and clears the registry.
//
// Idempotent — calling Stop twice is safe; the second call is a
// no-op.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.started || m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	rootCancel := m.rootCancel
	fanCancel := m.fanCancel
	peers := m.peers
	m.peers = map[string]*peerEntry{}
	m.mu.Unlock()

	// Step 1: Disconnect every peer first. This makes the
	// per-peer subscriber channels close, which is what the
	// fan-out goroutines watch as a termination signal.
	for _, entry := range peers {
		entry.conn.Disconnect()
	}

	// Step 2: Cancel the manager-level fan-out context (in case
	// any fan-out goroutine is blocked on a never-closing
	// channel) and wait for them to drain.
	if fanCancel != nil {
		fanCancel()
	}
	m.fanWG.Wait()

	// Step 3: Cancel the root context so any in-flight Connect
	// goroutine exits.
	if rootCancel != nil {
		rootCancel()
	}

	// Step 4: Close manager-level subscribers and reset.
	m.subs.closeAll()

	logging.Info("manager: stopped")
}

// Connect opens (or re-arms) the lifecycle goroutine for the named
// peer. Returns ErrUnknownPeer if name is not registered;
// ErrNotStarted if the Manager has not been started.
//
// Idempotent on the start state — Connect against a peer whose
// goroutine is already running returns ErrPeerAlreadyConnected
// (from the underlying conn package).
func (m *Manager) Connect(name string) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return ErrNotStarted
	}
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	entry, ok := m.peers[name]
	rootCtx := m.rootCtx
	fanCtx := m.fanCtx
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownPeer, name)
	}
	if rootCtx == nil {
		return ErrStopped
	}
	// Subscribe before Connect so no early events are missed. A new
	// subscriber channel is created by the connection on each Connect
	// call (the previous one was closed by Disconnect). We then start
	// a fresh fan-out goroutine for it.
	ch := entry.conn.Subscribe()
	if err := entry.conn.Connect(rootCtx); err != nil {
		entry.conn.Unsubscribe(ch)
		return err
	}
	m.fanWG.Add(1)
	go m.fanOut(fanCtx, name, ch)
	return nil
}

// Disconnect cancels the lifecycle goroutine for the named peer.
// Returns ErrUnknownPeer if name is not registered.
//
// Idempotent — safe to call against an already-disconnected peer.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return ErrNotStarted
	}
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	entry, ok := m.peers[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownPeer, name)
	}
	entry.conn.Disconnect()
	return nil
}

// Get returns the underlying peer-connection for the named peer.
// The Sender (Task 4) calls this to acquire the *conn.PeerConnection
// it then sends a CCR over.
//
// Returns nil and ErrUnknownPeer if name is not registered. The
// returned interface points at the production *conn.PeerConnection
// in normal operation; tests that swap in a fake via SetFactory get
// their fake back.
func (m *Manager) Get(name string) (PeerConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil, ErrNotStarted
	}
	if m.stopped {
		return nil, ErrStopped
	}
	entry, ok := m.peers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownPeer, name)
	}
	return entry.conn, nil
}

// State returns the current state for the named peer. Convenience
// over Get().State().
func (m *Manager) State(name string) (diameter.ConnectionState, error) {
	pc, err := m.Get(name)
	if err != nil {
		return diameter.StateDisconnected, err
	}
	return pc.State(), nil
}

// Names returns the peer names currently registered, in
// alphabetical order. Used by callers (and tests) that want to
// enumerate the registry.
func (m *Manager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.peers))
	for name := range m.peers {
		out = append(out, name)
	}
	sortStrings(out)
	return out
}

// Subscribe returns a manager-level StateEvent channel that fans
// out events from every registered peer. The channel is closed
// when Stop drains the registry.
func (m *Manager) Subscribe() <-chan diameter.StateEvent {
	return m.subs.add()
}

// Unsubscribe deregisters a previously-returned subscriber channel.
// Optional — Stop will close every subscriber channel anyway.
func (m *Manager) Unsubscribe(ch <-chan diameter.StateEvent) {
	m.subs.remove(ch)
}

// fanOut watches a per-peer subscriber channel and forwards every
// event to the manager-level subscriber set. Exits when the
// per-peer channel is closed (peer disconnected) or fanCtx is
// cancelled (manager stopped).
func (m *Manager) fanOut(ctx context.Context, peerName string, ch <-chan diameter.StateEvent) {
	defer m.fanWG.Done()
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			// Defence in depth: the channel is per-peer so PeerName
			// is already populated by the conn layer. We do not
			// rewrite it.
			_ = peerName
			m.subs.emit(ev)
		case <-ctx.Done():
			return
		}
	}
}
