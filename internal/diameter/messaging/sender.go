package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fiorix/go-diameter/v4/diam"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// Sender is the §14 abstraction the engine and any other caller
// uses to send a CCR and obtain the corresponding CCA. Returning
// an error means the request did not produce a CCA — the wire was
// not touched (peer disconnected) or the connection dropped during
// the wait.
//
// Implementations MUST be safe for concurrent use.
type Sender interface {
	// Send blocks until the CCA correlated to req's E2E ID
	// arrives or ctx is cancelled. Returns
	// diameter.ErrPeerNotConnected when peerName is unknown or
	// in disconnected state — fail-fast per AC-14.
	Send(ctx context.Context, peerName string, req *CCR) (*CCA, error)
}

// Resolver is the slice of manager.Manager the Sender consumes.
// Defined as its own interface here so tests can supply a mock
// without spinning up the full manager.
type Resolver interface {
	// Get returns the PeerConnection bound to peerName, or an
	// error — typically manager.ErrUnknownPeer.
	Get(name string) (manager.PeerConnection, error)
}

// peerCorrelator is the per-peer registry of in-flight CCRs
// keyed by end-to-end ID. The handler installed on the
// PeerConnection consults this map when a CCA arrives and
// dispatches the message onto the matching channel.
//
// One peerCorrelator per peer (the Sender holds a sync.Map keyed
// by peer name). The handler sees the *PeerConnection it is
// installed on, but our handler closure captures the
// peerCorrelator pointer at registration time so the dispatch
// path doesn't need a back-pointer.
type peerCorrelator struct {
	mu      sync.Mutex
	pending map[uint32]chan *diam.Message
}

func newPeerCorrelator() *peerCorrelator {
	return &peerCorrelator{pending: map[uint32]chan *diam.Message{}}
}

// register adds a pending request keyed by e2eID. The returned
// channel is closed when the connection drops, or receives one
// message when the matching CCA arrives.
func (c *peerCorrelator) register(e2eID uint32) chan *diam.Message {
	ch := make(chan *diam.Message, 1)
	c.mu.Lock()
	c.pending[e2eID] = ch
	c.mu.Unlock()
	return ch
}

// deregister removes a pending entry without closing its channel.
// Used by the Send path when ctx is cancelled and the caller
// wants to abandon the wait.
func (c *peerCorrelator) deregister(e2eID uint32) {
	c.mu.Lock()
	delete(c.pending, e2eID)
	c.mu.Unlock()
}

// dispatch is invoked by the CCA handler with an incoming
// message. Looks up the matching pending entry and forwards the
// message; if no entry exists (the request timed out / was
// cancelled before the response arrived) the message is dropped
// with a WARN log.
func (c *peerCorrelator) dispatch(m *diam.Message) {
	if m == nil || m.Header == nil {
		return
	}
	e2eID := m.Header.EndToEndID
	c.mu.Lock()
	ch, ok := c.pending[e2eID]
	if ok {
		delete(c.pending, e2eID)
	}
	c.mu.Unlock()
	if !ok {
		logging.Warn(
			"messaging: dropping CCA with no matching pending request",
			"end_to_end_id", e2eID,
		)
		return
	}
	// Non-blocking send — the channel is buffered (cap 1) and only
	// one message is ever delivered per e2eID, so this select is
	// belt-and-braces against a buggy double-dispatch.
	select {
	case ch <- m:
	default:
	}
}

// closeAll closes every pending channel. Called by the Sender
// when the peer disconnects so blocked Send calls return cleanly
// with ErrCorrelatorClosed.
func (c *peerCorrelator) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

// concreteSender is the production implementation of Sender.
// Constructed via NewSender.
type concreteSender struct {
	resolver Resolver

	mu       sync.Mutex
	perPeer  map[string]*peerCorrelator
	stateSub map[string]<-chan diameter.StateEvent
}

// NewSender constructs a Sender that resolves peers via the
// supplied Resolver (typically a *manager.Manager). Panics on a
// nil resolver — wiring a Sender against a nil manager is a
// programming error.
func NewSender(r Resolver) Sender {
	if r == nil {
		panic("messaging.NewSender: resolver must not be nil")
	}
	return &concreteSender{
		resolver: r,
		perPeer:  map[string]*peerCorrelator{},
		stateSub: map[string]<-chan diameter.StateEvent{},
	}
}

// Send writes the CCR to the named peer and blocks until the
// matching CCA arrives, ctx is cancelled, or the peer drops.
//
// AC-14 enforcement: when the peer is in any non-connected state
// at the moment Send is called, the call returns
// diameter.ErrPeerNotConnected immediately without writing
// anything to the wire. The check is point-in-time — a peer that
// drops between the check and the write surfaces as
// ErrCorrelatorClosed when the close-all cascade fires.
func (s *concreteSender) Send(ctx context.Context, peerName string, req *CCR) (*CCA, error) {
	pc, err := s.resolver.Get(peerName)
	if err != nil {
		return nil, fmt.Errorf("messaging: resolve peer %q: %w", peerName, err)
	}
	if pc.State() != diameter.StateConnected {
		return nil, fmt.Errorf("%w: peer %q is %s",
			diameter.ErrPeerNotConnected, peerName, pc.State())
	}
	conn := pc.Conn()
	if conn == nil {
		// State drifted between the State() and Conn() reads. The
		// connection laid out by conn.PeerConnection guarantees
		// activeConn is non-nil while StateConnected, but we
		// defend belt-and-braces.
		return nil, fmt.Errorf("%w: peer %q connection went away",
			diameter.ErrPeerNotConnected, peerName)
	}

	// Pre-fill peer-scoped fields from the connection's PeerConfig
	// so the encoder produces a well-formed CCR without forcing
	// every caller to re-state the local identity.
	cfg := pc.Config()
	enriched := *req
	if enriched.OriginHost == "" {
		enriched.OriginHost = cfg.OriginHost
	}
	if enriched.OriginRealm == "" {
		enriched.OriginRealm = cfg.OriginRealm
	}
	if enriched.DestinationRealm == "" {
		enriched.DestinationRealm = cfg.OriginRealm
	}

	correlator := s.correlatorFor(peerName, pc)

	msg, err := BuildCCRMessage(conn.Dictionary(), &enriched)
	if err != nil {
		return nil, fmt.Errorf("messaging: build CCR for peer %q: %w", peerName, err)
	}

	// Capture the correlator key before sending. diam.NewRequest
	// has already populated EndToEndID and HopByHopID inside
	// BuildCCRMessage; we simply read them out.
	e2eID := msg.Header.EndToEndID
	respCh := correlator.register(e2eID)

	if _, err := msg.WriteTo(conn); err != nil {
		correlator.deregister(e2eID)
		return nil, fmt.Errorf("messaging: write CCR to peer %q: %w", peerName, err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("messaging: peer %q dropped during request: %w",
				peerName, ErrCorrelatorClosed)
		}
		return DecodeCCAMessage(resp)
	case <-ctx.Done():
		correlator.deregister(e2eID)
		return nil, ctx.Err()
	}
}

// correlatorFor returns the per-peer correlator, lazily creating
// one and registering the CCA handler on the connection on first
// use. Subsequent calls return the existing instance.
func (s *concreteSender) correlatorFor(peerName string, pc manager.PeerConnection) *peerCorrelator {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.perPeer[peerName]; ok {
		return c
	}
	c := newPeerCorrelator()
	s.perPeer[peerName] = c

	// Capture the correlator pointer in the closure so the
	// handler's dispatch path doesn't need to look the peer up.
	pc.HandleFunc("CCA", func(_ diam.Conn, m *diam.Message) {
		c.dispatch(m)
	})

	// Watch the peer's state so we can close pending channels on
	// drop. The state subscription runs in its own goroutine —
	// one per peer — until the connection's subscriber channel
	// is closed (PeerConnection.Disconnect tears it down).
	stateCh := pc.Subscribe()
	s.stateSub[peerName] = stateCh
	go s.watchPeerState(peerName, stateCh, c)

	return c
}

// watchPeerState consumes state transitions for one peer and
// closes pending correlator entries every time the peer leaves
// StateConnected. This unblocks any Send() call that was
// waiting for a response when the connection dropped.
func (s *concreteSender) watchPeerState(peerName string, ch <-chan diameter.StateEvent, c *peerCorrelator) {
	for ev := range ch {
		if ev.From == diameter.StateConnected && ev.To != diameter.StateConnected {
			c.closeAll()
		}
	}
	// Channel closed — final cleanup.
	c.closeAll()
	s.mu.Lock()
	delete(s.stateSub, peerName)
	s.mu.Unlock()
}

// noOpDiscard is a sentinel error variable that lets Sender
// callers branch on errors.Is(err, diameter.ErrPeerNotConnected)
// for both the resolve and not-connected paths in tests.
var _ = errors.Is
