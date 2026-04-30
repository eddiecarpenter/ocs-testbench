package manager

import (
	"sort"
	"sync"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// subscriberBufferSize is the per-subscriber channel capacity for
// manager-level subscribers. The fan-out interleaves events from
// every peer so the burst size is N peers × per-peer burst — sized
// generously to absorb a coordinated reconnect storm without
// dropping events. A subscriber that cannot keep up loses the
// oldest event with a logged WARN, mirroring the per-connection
// fan-out policy.
const subscriberBufferSize = 64

// subscriberSet tracks the live manager-level subscribers. Mirrors
// conn/state.go's subscriberSet semantics, with a bigger default
// channel buffer because the manager fans out from N peers onto a
// single channel.
type subscriberSet struct {
	mu       sync.Mutex
	channels map[chan diameter.StateEvent]struct{}
}

// newSubscriberSet constructs an empty set. Allocation lives behind
// a constructor so map-initialisation conventions are uniform with
// the rest of the package.
func newSubscriberSet() *subscriberSet {
	return &subscriberSet{channels: map[chan diameter.StateEvent]struct{}{}}
}

// add registers a new subscriber and returns the read end.
func (s *subscriberSet) add() <-chan diameter.StateEvent {
	ch := make(chan diameter.StateEvent, subscriberBufferSize)
	s.mu.Lock()
	s.channels[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

// remove unregisters a subscriber and closes its channel.
// Idempotent — a second remove on the same channel is a no-op.
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

// closeAll closes every registered subscriber and clears the set.
func (s *subscriberSet) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.channels {
		close(c)
	}
	s.channels = map[chan diameter.StateEvent]struct{}{}
}

// count returns the number of live subscribers. Used by tests.
func (s *subscriberSet) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.channels)
}

// emit fans one StateEvent out to every subscriber. A subscriber
// whose channel is full has the oldest event dropped (oldest-drop)
// and the new event delivered, with a WARN log line so operators
// see slow subscribers in their telemetry. The producer never
// blocks.
func (s *subscriberSet) emit(ev diameter.StateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.channels {
		select {
		case ch <- ev:
		default:
			// Drop the oldest cached event and retry once.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
				logging.Warn(
					"manager: state-event subscriber overflow; dropping new event",
					"peer", ev.PeerName,
					"from", ev.From.String(),
					"to", ev.To.String(),
				)
			}
		}
	}
}

// sortStrings is a tiny helper around sort.Strings so the manager's
// public Names() returns a deterministic order without the call site
// reaching into the sort package.
func sortStrings(s []string) {
	sort.Strings(s)
}
