package conn

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/fiorix/go-diameter/v4/diam/sm"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// dialFunc is the seam between PeerConnection and go-diameter. The
// production implementation uses sm.Client.DialNetwork /
// DialTLSConfig; tests substitute a fake that returns a
// pre-arranged diam.Conn (or an error). The seam is local to this
// file and intentionally narrow.
type dialFunc func(ctx context.Context, cli *sm.Client, cfg diameter.PeerConfig) (diam.Conn, error)

// realDial is the production dialFunc. It honours the peer's
// transport selection (tcp / tls) and applies the InsecureSkipVerify
// flag for TLS connections that do not point at a CA-trusted server
// (e.g. internal test peers).
func realDial(_ context.Context, cli *sm.Client, cfg diameter.PeerConfig) (diam.Conn, error) {
	addr := cfg.Address()
	switch strings.ToLower(strings.TrimSpace(cfg.Transport)) {
	case "", diameter.TransportTCP:
		return cli.DialNetwork("tcp", addr)
	case diameter.TransportTLS:
		// We delegate to diam.DialTLSConfig directly so we can
		// supply our own tls.Config — sm.Client's DialTLS path
		// rejects an empty certFile/keyFile pair when the operator
		// has chosen InsecureSkipVerify but provides no client
		// certificate, which is the common case for TLS to an
		// internal OCS testbed.
		tlsCfg := &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify, //nolint:gosec // operator-controlled per-peer
			MinVersion:         tls.VersionTLS12,
		}
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
			if err != nil {
				return nil, fmt.Errorf("load client tls keypair: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		return diam.DialTLSConfig(addr, "", "", cli.Handler, cli.Dict, tlsCfg)
	default:
		return nil, fmt.Errorf("%w: unsupported transport %q (want tcp or tls)", diameter.ErrInvalidPeerConfig, cfg.Transport)
	}
}

// PeerConnection is the per-peer connection lifecycle.
//
// The zero value is not useful — construct via New. Once
// constructed:
//
//   - Connect spawns the lifecycle goroutine which dials, runs the
//     handshake, and (on drop) reconnects with exponential
//     backoff. Connect returns once the goroutine is started; the
//     caller observes connect outcomes via Subscribe() events. To
//     await the first transition synchronously, see WaitFor.
//   - Disconnect cancels the lifecycle goroutine and closes any
//     active connection. Idempotent.
//   - State returns the current state; cheap, mutex-RLock guarded.
//   - Subscribe returns a typed StateEvent channel for fan-out.
//   - Conn returns the live diam.Conn while connected (used by the
//     Sender in task 4); returns nil otherwise.
//   - HandleFunc registers a per-command incoming-message handler
//     (e.g. "CCA") that is reapplied to every fresh sm.Client
//     across reconnects. The Sender (task 4) uses this to install
//     a CCA correlator before any messages are sent.
//
// Concurrency: every public method is goroutine-safe.
type PeerConnection struct {
	cfg    diameter.PeerConfig
	parser *dict.Parser

	state *stateBox
	subs  *subscriberSet

	// muLife protects the lifecycle fields below.
	muLife sync.Mutex
	// cancel cancels the lifecycle goroutine when set; nil while
	// the connection is in StateDisconnected and not running.
	cancel context.CancelFunc
	// wg tracks the lifecycle goroutine so Disconnect can wait for
	// a clean teardown.
	wg sync.WaitGroup
	// activeConn is the current go-diameter connection while
	// StateConnected; nil otherwise. Held under muLife so the
	// Sender (task 4) can read it under the same lock that the
	// lifecycle goroutine uses to swap it in.
	activeConn diam.Conn
	// activeMgr is the sm.StateMachine bound to activeConn. The
	// HandleFunc method applies user-registered handlers to it
	// immediately so a Sender registering "CCA" after the
	// connection is up still sees responses.
	activeMgr *sm.StateMachine
	// handlers is the slice of (command, handler) pairs the
	// caller has registered via HandleFunc. Re-applied to every
	// fresh sm.Client constructed in dialOnce so a reconnect
	// preserves the user's handler installation.
	handlers []handlerEntry

	// Tunables — defaults applied in New. Exposed in the package
	// (lower-case) so the test seam can override them without
	// public surface area.
	backoffInitial time.Duration
	backoffCap     time.Duration
	backoffFactor  float64
	nowFn          func() time.Time

	// dial is the seam used by tests; production uses realDial.
	dial dialFunc
}

// handlerEntry is one user-registered command handler. The handler
// is invoked by the underlying sm.StateMachine (which is itself a
// ServeMux) when an incoming message matches the command.
type handlerEntry struct {
	cmd     string
	handler diam.HandlerFunc
}

// HandleFunc registers a handler invoked for every incoming
// Diameter message whose short command name equals cmd (e.g.
// "CCA", "RAR"). The handler is reapplied to every fresh sm.Client
// created across reconnects — callers register once at
// construction time and the connection layer takes care of
// re-installing on each dial attempt.
//
// HandleFunc is safe to call from any state. If the connection is
// currently connected and a handler with the same command already
// exists, the existing entry is replaced; otherwise the new
// handler is appended. This matches the behaviour callers want
// when they subscribe to a command after a reconnect.
//
// CER, CEA, DWR, DWA are reserved by go-diameter's StateMachine —
// registering a handler for those commands will be silently
// shadowed by the StateMachine's built-in handlers. The Sender
// (task 4) registers "CCA" only; protocol-mandated handlers for
// out-of-band CCRs (CCR-Terminate from FUI) flow back through the
// same Sender path and do not need a separate registration.
func (pc *PeerConnection) HandleFunc(cmd string, handler diam.HandlerFunc) {
	pc.muLife.Lock()
	defer pc.muLife.Unlock()
	found := false
	for i := range pc.handlers {
		if pc.handlers[i].cmd == cmd {
			pc.handlers[i].handler = handler
			found = true
			break
		}
	}
	if !found {
		pc.handlers = append(pc.handlers, handlerEntry{cmd: cmd, handler: handler})
	}
	// If a live StateMachine exists (the connection is up or
	// connecting), apply the handler now so callers registering
	// after Connect still see incoming messages.
	if pc.activeMgr != nil {
		pc.activeMgr.HandleFunc(cmd, handler)
	}
}

// New constructs a PeerConnection bound to the given config and
// dictionary parser. Pass dict.Default in production — the manager
// (task 3) does this — so the connection's CER and any later CCRs
// resolve AVP names against the same parser the dictionary loader
// (task 1) populated.
//
// Panics on a nil parser; the lifecycle goroutine cannot operate
// without one and an early panic is the right failure mode for
// what is unconditionally a wiring bug.
func New(cfg diameter.PeerConfig, parser *dict.Parser) *PeerConnection {
	if parser == nil {
		panic("conn.New: parser must not be nil")
	}
	return &PeerConnection{
		cfg:            cfg,
		parser:         parser,
		state:          &stateBox{v: diameter.StateDisconnected},
		subs:           newSubscriberSet(),
		backoffInitial: diameter.DefaultBackoffInitial,
		backoffCap:     diameter.DefaultBackoffCap,
		backoffFactor:  diameter.DefaultBackoffFactor,
		nowFn:          func() time.Time { return time.Now().UTC() },
		dial:           realDial,
	}
}

// Config returns the PeerConfig the connection was constructed with.
func (pc *PeerConnection) Config() diameter.PeerConfig { return pc.cfg }

// State returns the current ConnectionState. Constant time, RLock-only.
func (pc *PeerConnection) State() diameter.ConnectionState { return pc.state.load() }

// Subscribe registers a new subscriber and returns the receive
// channel. The channel is closed when Disconnect tears down the
// connection or when the parent context is cancelled. Concurrent
// subscribers each see every event from the moment they subscribe;
// historical events are not replayed.
func (pc *PeerConnection) Subscribe() <-chan diameter.StateEvent {
	return pc.subs.add()
}

// Unsubscribe deregisters a subscriber. Optional — when the
// connection tears down, every subscriber's channel is closed
// regardless. Use this when a consumer wants to stop receiving
// events while the connection is still live.
func (pc *PeerConnection) Unsubscribe(ch <-chan diameter.StateEvent) {
	pc.subs.remove(ch)
}

// Conn returns the current diam.Conn while the peer is connected,
// or nil otherwise. The Sender (task 4) reads this to acquire the
// connection it writes a CCR over. The returned value is only
// valid until the next state transition; callers must re-read on
// each send rather than caching.
func (pc *PeerConnection) Conn() diam.Conn {
	pc.muLife.Lock()
	defer pc.muLife.Unlock()
	if pc.state.load() != diameter.StateConnected {
		return nil
	}
	return pc.activeConn
}

// Connect spawns the lifecycle goroutine. Returns
// ErrPeerAlreadyConnected if a goroutine is already running for
// this peer (idempotent on the start state, not on the running
// state).
//
// The supplied context bounds the lifecycle goroutine — cancelling
// it has the same effect as Disconnect. Production wiring passes
// the application's root context so Lifecycle shutdown drains
// every PeerConnection cleanly.
func (pc *PeerConnection) Connect(ctx context.Context) error {
	if err := validateConfig(pc.cfg); err != nil {
		return err
	}

	pc.muLife.Lock()
	if pc.cancel != nil {
		pc.muLife.Unlock()
		return diameter.ErrPeerAlreadyConnected
	}
	lifeCtx, cancel := context.WithCancel(ctx)
	pc.cancel = cancel
	pc.muLife.Unlock()

	pc.wg.Add(1)
	go pc.run(lifeCtx)
	return nil
}

// Disconnect cancels the lifecycle goroutine and waits for it to
// exit. Idempotent — safe to call from any state, including
// already-disconnected. After Disconnect, every subscriber's
// channel is closed and the underlying go-diameter connection (if
// any) is torn down.
func (pc *PeerConnection) Disconnect() {
	pc.muLife.Lock()
	cancel := pc.cancel
	pc.cancel = nil
	pc.muLife.Unlock()
	if cancel == nil {
		// Never connected (or already disconnected). Still
		// transition to disconnected so a stray "error" state from
		// validate-fail is normalised.
		_ = pc.transitionTo(diameter.StateDisconnected, "manual disconnect (idempotent)")
		return
	}
	cancel()
	pc.wg.Wait()
	pc.subs.closeAll()
	pc.subs = newSubscriberSet()
}

// validateConfig performs minimal config validation. The store layer
// is the source of truth; this is a defence-in-depth check
// targeting wiring bugs that produce zero-valued configs.
func validateConfig(cfg diameter.PeerConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("%w: peer name is empty", diameter.ErrInvalidPeerConfig)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is empty", diameter.ErrInvalidPeerConfig)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("%w: port %d is out of range", diameter.ErrInvalidPeerConfig, cfg.Port)
	}
	if strings.TrimSpace(cfg.OriginHost) == "" {
		return fmt.Errorf("%w: origin-host is empty", diameter.ErrInvalidPeerConfig)
	}
	if strings.TrimSpace(cfg.OriginRealm) == "" {
		return fmt.Errorf("%w: origin-realm is empty", diameter.ErrInvalidPeerConfig)
	}
	tr := strings.ToLower(strings.TrimSpace(cfg.Transport))
	if tr != "" && tr != diameter.TransportTCP && tr != diameter.TransportTLS {
		return fmt.Errorf("%w: unsupported transport %q", diameter.ErrInvalidPeerConfig, cfg.Transport)
	}
	return nil
}

// run is the lifecycle goroutine. It loops until ctx is cancelled,
// dialling, observing drops, and re-dialling with exponential
// backoff. State transitions are emitted to subscribers as they
// happen.
func (pc *PeerConnection) run(ctx context.Context) {
	defer pc.wg.Done()
	defer func() {
		// On exit, ensure no live conn is leaked.
		pc.muLife.Lock()
		if pc.activeConn != nil {
			pc.activeConn.Close()
			pc.activeConn = nil
		}
		pc.activeMgr = nil
		pc.muLife.Unlock()
	}()

	backoff := pc.backoffInitial
	for {
		if ctx.Err() != nil {
			pc.transitionTo(diameter.StateDisconnected, "manual disconnect")
			return
		}

		pc.transitionTo(diameter.StateConnecting, "dialling "+pc.cfg.Address())
		c, err := pc.dialOnce(ctx)
		if err != nil {
			detail := fmt.Sprintf("dial failed: %v", err)
			pc.transitionTo(diameter.StateDisconnected, detail)
			logging.Warn(
				"diameter/conn: dial failed; backing off",
				"peer", pc.cfg.Name,
				"address", pc.cfg.Address(),
				"error", err.Error(),
				"backoff", backoff.String(),
			)
			if !sleepCtx(ctx, backoff) {
				pc.transitionTo(diameter.StateDisconnected, "manual disconnect")
				return
			}
			backoff = nextBackoff(backoff, pc.backoffFactor, pc.backoffCap)
			continue
		}

		// Connect succeeded — store the live conn, transition, reset
		// backoff. On the next drop or cancel we go round.
		pc.muLife.Lock()
		pc.activeConn = c
		pc.muLife.Unlock()
		pc.transitionTo(diameter.StateConnected, "CER/CEA OK")
		backoff = pc.backoffInitial

		dropped := pc.waitForDrop(ctx, c)

		// Tear down the conn on this side regardless of cause.
		pc.muLife.Lock()
		if pc.activeConn == c {
			pc.activeConn = nil
		}
		pc.muLife.Unlock()
		c.Close()

		if !dropped {
			// Context cancelled — clean exit.
			pc.transitionTo(diameter.StateDisconnected, "manual disconnect")
			return
		}
		pc.transitionTo(diameter.StateDisconnected, "connection dropped")
		// Loop and reconnect with reset backoff.
	}
}

// waitForDrop returns true when the conn drops, false when ctx is
// cancelled. Either way the conn is torn down by the caller.
func (pc *PeerConnection) waitForDrop(ctx context.Context, c diam.Conn) bool {
	notifier, ok := c.(diam.CloseNotifier)
	if !ok {
		// Conservatively treat unknown types as never-dropping.
		// In practice every go-diameter connection implements
		// CloseNotifier; this is here purely so an unexpected
		// implementation does not panic the lifecycle goroutine.
		<-ctx.Done()
		return false
	}
	select {
	case <-notifier.CloseNotify():
		return true
	case <-ctx.Done():
		return false
	}
}

// dialOnce performs a single CER/CEA-establishing dial. The peer's
// dictionary, sm.Settings, and sm.Client are constructed fresh per
// attempt because the underlying StateMachine is not safe to reuse
// across reconnects (handshake-channel state is per-attempt).
//
// User-registered handlers (added via HandleFunc) are re-applied
// to the fresh StateMachine before the dial — this is what gives
// callers a stable handler set across reconnects.
func (pc *PeerConnection) dialOnce(ctx context.Context) (diam.Conn, error) {
	settings := buildSettings(pc.cfg, pc.parser)
	mgr := sm.New(settings)
	pc.muLife.Lock()
	for _, h := range pc.handlers {
		mgr.HandleFunc(h.cmd, h.handler)
	}
	pc.activeMgr = mgr
	pc.muLife.Unlock()
	client := &sm.Client{
		Dict:               pc.parser,
		Handler:            mgr,
		MaxRetransmits:     0,
		RetransmitInterval: time.Second,
		EnableWatchdog:     true,
		WatchdogInterval:   pc.cfg.WatchdogInterval,
		AuthApplicationID:  buildAuthAppAVPs(pc.cfg.AuthApplicationIDs),
	}
	if client.WatchdogInterval == 0 {
		client.WatchdogInterval = diameter.DefaultWatchdogInterval
	}
	return pc.dial(ctx, client, pc.cfg)
}

// buildSettings constructs an sm.Settings from a PeerConfig. The
// Vendor-Id and Product-Name fall back to documented defaults so a
// minimal config still produces a valid CER.
func buildSettings(cfg diameter.PeerConfig, parser *dict.Parser) *sm.Settings {
	productName := cfg.ProductName
	if productName == "" {
		productName = "ocs-testbench"
	}
	settings := &sm.Settings{
		OriginHost:    datatype.DiameterIdentity(cfg.OriginHost),
		OriginRealm:   datatype.DiameterIdentity(cfg.OriginRealm),
		VendorID:      datatype.Unsigned32(cfg.VendorID),
		ProductName:   datatype.UTF8String(productName),
		OriginStateID: datatype.Unsigned32(time.Now().Unix()),
		Dict:          parser,
	}
	return settings
}

// buildAuthAppAVPs renders the configured Auth-Application-Id list
// into a slice of *diam.AVP suitable for sm.Client.AuthApplicationID.
// The credit-control application id (4) is always included so the
// CER advertises support for it — the testbench is a Gy-app client
// regardless of what the operator configured.
func buildAuthAppAVPs(ids []uint32) []*diam.AVP {
	have := make(map[uint32]bool, len(ids)+1)
	have[diameter.AppIDCreditControl] = false
	for _, id := range ids {
		have[id] = true
	}
	avps := make([]*diam.AVP, 0, len(have))
	avps = append(avps, diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(diameter.AppIDCreditControl)))
	for id := range have {
		if id == diameter.AppIDCreditControl {
			continue
		}
		avps = append(avps, diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(id)))
	}
	return avps
}

// transitionTo updates the state and emits the corresponding
// StateEvent. Returns true when the state actually changed; a
// no-op call (already in the target state) does not emit an event,
// preventing duplicate "disconnected → disconnected" pings during
// shutdown.
func (pc *PeerConnection) transitionTo(next diameter.ConnectionState, detail string) bool {
	prev := pc.state.store(next)
	if prev == next {
		return false
	}
	pc.subs.emit(transition(pc.cfg.Name, prev, next, detail, pc.nowFn))
	logging.Info(
		"diameter/conn: state transition",
		"peer", pc.cfg.Name,
		"from", prev.String(),
		"to", next.String(),
		"detail", detail,
	)
	return true
}

// sleepCtx blocks for d or until ctx is cancelled. Returns true if
// the full sleep elapsed; false on cancellation.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// nextBackoff applies the doubling-with-cap policy. Pure function
// so the caller does not need to inline the math.
func nextBackoff(cur time.Duration, factor float64, cap time.Duration) time.Duration {
	if factor <= 1 {
		factor = 2
	}
	next := time.Duration(float64(cur) * factor)
	if next > cap {
		return cap
	}
	if next <= 0 {
		return cap
	}
	return next
}
