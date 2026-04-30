package conn

import (
	"sync"
	"time"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// subscriberBufferSize is the per-subscriber channel capacity.
// Sized to absorb a small burst of transitions — connect-attempt,
// connect-failed, reconnect-attempt — without forcing the
// connection goroutine to block. A subscriber that cannot keep up
// loses the oldest event with a logged WARN.
const subscriberBufferSize = 16

// stateBox is a small mutex-protected holder for ConnectionState.
// Exposed as its own type rather than open-coded on PeerConnection
// so the connection's own mutex isn't held while transitions are
// observed by subscribers — fan-out is fire-and-forget.
type stateBox struct {
	mu sync.RWMutex
	v  diameter.ConnectionState
}

// load returns the current state.
func (b *stateBox) load() diameter.ConnectionState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.v
}

// store sets the state and returns the previous value.
func (b *stateBox) store(next diameter.ConnectionState) diameter.ConnectionState {
	b.mu.Lock()
	defer b.mu.Unlock()
	prev := b.v
	b.v = next
	return prev
}

// subscriberSet tracks the live subscriber channels. A subscriber
// is added on Subscribe and removed when its channel is closed via
// Unsubscribe (or the connection's lifecycle goroutine exits).
//
// The set's mutex is held only briefly during add/remove and during
// emit; emit performs a non-blocking send on every subscriber
// channel so a slow receiver does not wedge the producer.
type subscriberSet struct {
	mu       sync.Mutex
	channels map[chan diameter.StateEvent]struct{}
}

func newSubscriberSet() *subscriberSet {
	return &subscriberSet{channels: map[chan diameter.StateEvent]struct{}{}}
}

// add registers a new subscriber and returns the channel it should
// read from. The channel is created with capacity subscriberBufferSize.
func (s *subscriberSet) add() <-chan diameter.StateEvent {
	ch := make(chan diameter.StateEvent, subscriberBufferSize)
	s.mu.Lock()
	s.channels[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// remove unregisters a subscriber. The channel is closed so any
// blocked reader returns the zero value. Idempotent — calling
// remove twice on the same channel is safe.
func (s *subscriberSet) remove(ch <-chan diameter.StateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.channels {
		if c == ch {
			delete(s.channels, c)
			close(c)
			return
		}
	}
}

// closeAll closes every registered subscriber channel and clears
// the set. Called when the connection is permanently torn down.
func (s *subscriberSet) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.channels {
		close(c)
	}
	s.channels = map[chan diameter.StateEvent]struct{}{}
}

// count returns the live subscriber count. Used by tests.
func (s *subscriberSet) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.channels)
}

// emit fans out one event to every subscriber. A subscriber whose
// buffer is full has the oldest event dropped (oldest-drop) and the
// new event delivered, with a WARN log line so operators see slow
// subscribers in their telemetry. The producer never blocks.
//
// Oldest-drop is implemented as: try to send non-blocking; on
// failure read one event from the channel (best-effort dequeue),
// then try the send again non-blocking. If the second send still
// fails (the subscriber is reading concurrently), the new event is
// dropped — better than wedging the connection.
func (s *subscriberSet) emit(ev diameter.StateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.channels {
		select {
		case ch <- ev:
		default:
			// Drop the oldest cached event and retry.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
				logging.Warn(
					"diameter/conn: state-event subscriber overflow; dropping new event",
					"peer", ev.PeerName,
					"from", ev.From.String(),
					"to", ev.To.String(),
				)
			}
		}
	}
}

// transition is a tiny helper that builds a StateEvent timestamped
// at the call site so the event reflects when the transition
// actually happened, not when the subscriber processed it.
func transition(peer string, from, to diameter.ConnectionState, detail string, now func() time.Time) diameter.StateEvent {
	return diameter.StateEvent{
		PeerName: peer,
		From:     from,
		To:       to,
		Detail:   detail,
		Time:     now(),
	}
}
