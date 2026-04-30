// Production GeneratorProvider — auto-generates system-managed token
// values for use in the template loader's value-map assembly.
//
// Token semantics follow docs/ARCHITECTURE.md §5 (system variables)
// and §7 (engine-managed AVPs):
//
//   - SESSION_ID    — RFC 6733 §8.8 format, generated once per
//                     provider construction (refresh: once).
//   - CHARGING_ID   — 32-bit unsigned int, fresh per construction.
//   - CC_REQUEST_NUMBER — per-call incrementer starting at 0.
//   - EVENT_TIMESTAMP  — wall-clock time at Generate call.
//
// The provider is injectable; unit tests use a deterministic fake
// to keep test assertions stable.

package template

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Known generator token names — callers should use these constants
// rather than raw strings to avoid typos.
const (
	// TokenSessionID is the Session-Id system variable name.
	TokenSessionID = "SESSION_ID"
	// TokenChargingID is the Charging-Id system variable name.
	TokenChargingID = "CHARGING_ID"
	// TokenCCRequestNumber is the CC-Request-Number system variable name.
	TokenCCRequestNumber = "CC_REQUEST_NUMBER"
	// TokenEventTimestamp is the Event-Timestamp system variable name.
	TokenEventTimestamp = "EVENT_TIMESTAMP"
)

// productionGeneratorProvider is the live GeneratorProvider backed
// by crypto/rand and the system clock.
type productionGeneratorProvider struct {
	sessionID  string
	chargingID uint32

	// requestNumber is accessed via sync/atomic to keep Generate
	// goroutine-safe without a per-call mutex.
	requestNumber atomic.Uint32

	// mu protects the session-id and charging-id generation during
	// construction; once constructed, those fields are read-only.
	mu  sync.Mutex
	now func() time.Time
}

// NewGeneratorProvider constructs the production GeneratorProvider.
// SESSION_ID and CHARGING_ID are generated once at construction time
// (refresh: once per execution context). CC_REQUEST_NUMBER starts at
// 0 and increments on every Generate("CC_REQUEST_NUMBER") call.
// EVENT_TIMESTAMP returns l.now() on every call.
//
// originHost is the Diameter Origin-Host inserted into the Session-Id.
// Pass an empty string to fall back to "ocs-testbench.local".
//
// nowFn is the wall-clock source. Pass nil to use time.Now().UTC —
// callers that want deterministic timestamps in tests inject their own.
func NewGeneratorProvider(originHost string, nowFn func() time.Time) GeneratorProvider {
	if originHost == "" {
		originHost = "ocs-testbench.local"
	}
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	g := &productionGeneratorProvider{now: nowFn}
	g.sessionID = generateSessionID(originHost)
	g.chargingID = randomUint32()
	return g
}

// Generate returns the current value for the named token.
// Returns an error for unrecognised names.
func (g *productionGeneratorProvider) Generate(name string) (any, error) {
	switch name {
	case TokenSessionID:
		return g.sessionID, nil
	case TokenChargingID:
		return g.chargingID, nil
	case TokenCCRequestNumber:
		// Add returns the new value after increment; we want the
		// pre-increment value for sequence continuity (first call → 0).
		// Load-then-add gives us the "current then advance" behaviour.
		v := g.requestNumber.Load()
		g.requestNumber.Add(1)
		return v, nil
	case TokenEventTimestamp:
		return g.now(), nil
	default:
		return nil, fmt.Errorf("generator: unknown token %q", name)
	}
}

// generateSessionID builds a Diameter Session-Id per RFC 6733 §8.8:
//
//	<Origin-Host>;<hi32>;<lo32>
//
// where hi32 and lo32 are the high and low 32-bit halves of a random
// 64-bit value (analogous to the two UUID halves).
func generateSessionID(originHost string) string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	hi := binary.BigEndian.Uint32(buf[0:4])
	lo := binary.BigEndian.Uint32(buf[4:8])
	return fmt.Sprintf("%s;%d;%d", originHost, hi, lo)
}

// randomUint32 generates a cryptographically random 32-bit unsigned
// integer for use as a Charging-Id.
func randomUint32() uint32 {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return binary.BigEndian.Uint32(buf[:])
}
