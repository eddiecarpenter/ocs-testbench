package diameter

import (
	"errors"
	"fmt"
	"time"
)

// Default backoff parameters used by the per-peer reconnect loop.
// Exposed at package scope (rather than buried inside the conn
// package) so tests and any future per-peer override path can read
// the canonical defaults.
const (
	// DefaultBackoffInitial is the first sleep on a transient failure
	// (CER/CEA refused, dial error, drop without explicit close).
	DefaultBackoffInitial = 1 * time.Second
	// DefaultBackoffCap is the upper bound applied to the doubling
	// backoff. Sustained failures back off no slower than this — the
	// reconnect loop polls at most once per cap duration.
	DefaultBackoffCap = 60 * time.Second
	// DefaultBackoffFactor is the multiplier applied between attempts.
	DefaultBackoffFactor = 2.0
	// DefaultWatchdogInterval is the DWR/DWA cadence applied when
	// the peer config does not specify one. RFC 3539 §3.4.1 suggests
	// 30s; we mirror that.
	DefaultWatchdogInterval = 30 * time.Second
)

// TransportTCP and TransportTLS are the two transport modes the
// testbench supports for MVP. SCTP is explicitly out of scope per
// docs/ARCHITECTURE.md §13.
const (
	TransportTCP = "tcp"
	TransportTLS = "tls"
)

// AppIDCreditControl is the application id assigned to the Diameter
// Credit-Control application by RFC 4006 §3. Exposed at the
// package root so the connection layer can reference it without
// importing internal/diameter/dictionary (which transitively pulls
// in internal/store and is the wrong dependency direction).
const AppIDCreditControl uint32 = 4

// PeerConfig is the connection-time configuration for a single
// Diameter peer. The orchestrator decodes the JSONB peer.body row
// (owned by feature #16) into this shape and feeds it into the
// diameter package family. The diameter package itself never reads
// SQL — see docs/ARCHITECTURE.md §14 core-separation invariant.
//
// Defaults (applied by the connection layer when the corresponding
// field is the zero value):
//   - Transport: TransportTCP
//   - WatchdogInterval: DefaultWatchdogInterval
//   - VendorID: 0 (set explicitly when known)
//   - ProductName: "ocs-testbench"
//
// Validation is intentionally light here: the diameter package
// trusts the orchestrator to supply a well-formed config. The
// store layer enforces the semantic invariants (host/port present,
// origin-host non-empty, etc.) at row write time.
type PeerConfig struct {
	// Name is the unique peer name; used as the key in the manager
	// registry and surfaced in StateEvent.PeerName.
	Name string

	// Host is the resolvable hostname or IP literal of the OCS peer.
	Host string
	// Port is the TCP port the peer accepts Diameter connections on
	// (typically 3868 for plaintext, 5868 for TLS).
	Port int

	// OriginHost is the local FQDN advertised in CER (Origin-Host
	// AVP, RFC 6733 §6.4). Each peer has its own Origin-Host so
	// upstream OCS peers can disambiguate concurrent testbench
	// connections.
	OriginHost string
	// OriginRealm is the local realm advertised in CER.
	OriginRealm string

	// Transport selects the wire transport. Empty is treated as
	// TransportTCP. SCTP is rejected by the connection layer.
	Transport string

	// WatchdogInterval is the DWR cadence. Zero is treated as
	// DefaultWatchdogInterval.
	WatchdogInterval time.Duration

	// AutoConnect requests that the manager open this connection on
	// startup; consumed by the manager (task 3), not by the
	// PeerConnection itself.
	AutoConnect bool

	// TLSCertFile and TLSKeyFile are the optional client certificate
	// pair for mutual TLS. Both must be set or both must be empty.
	// Used only when Transport == TransportTLS.
	TLSCertFile string
	TLSKeyFile  string
	// TLSInsecureSkipVerify disables server-certificate validation
	// when true. Permitted only against test fixtures and operators
	// that explicitly opt in. The server's go-diameter library uses
	// InsecureSkipVerify=true by default for its bare DialTLS path,
	// which we mirror.
	TLSInsecureSkipVerify bool

	// VendorID is the Vendor-Id AVP advertised in CER. Defaults to
	// 0 ("Unknown / IETF") when unset.
	VendorID uint32
	// ProductName is the Product-Name AVP. Defaults to
	// "ocs-testbench" when empty.
	ProductName string

	// AuthApplicationIDs is the list of Auth-Application-Id AVPs
	// advertised in CER. The Credit-Control application id (4) is
	// the one this testbench cares about.
	AuthApplicationIDs []uint32
}

// Address returns the dial-target address (host:port). Returns
// "host:0" if Port is unset, which the dial layer surfaces as an
// error rather than silently choosing 0.
func (p PeerConfig) Address() string {
	return fmt.Sprintf("%s:%d", p.Host, p.Port)
}

// ConnectionState is the per-peer state.
//
//	disconnected → connecting → connected
//	     ▲             │             │
//	     └─────────────┴─────────────┘   (drop or manual disconnect)
//
// The error state is reserved for unrecoverable failures (e.g.
// invalid config rejected at validate time). Transient failures
// surface as disconnected with the reason in StateEvent.Detail —
// the reconnect loop owns retry policy.
type ConnectionState int

// Connection state values.
const (
	// StateDisconnected — no live connection; the reconnect loop
	// may be sleeping between attempts.
	StateDisconnected ConnectionState = iota
	// StateConnecting — the dial-or-handshake is in flight.
	StateConnecting
	// StateConnected — CER/CEA has succeeded and DWR/DWA is
	// optionally running.
	StateConnected
	// StateError — unrecoverable; the operator must intervene.
	StateError
)

// String renders a ConnectionState for log lines.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateError:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// StateEvent is one transition observed by Subscribe() consumers.
// Time is set by the emitter so subscribers can reconstruct
// ordering even when the channel is buffered.
type StateEvent struct {
	// PeerName identifies the peer the event belongs to. Always
	// equal to the corresponding PeerConfig.Name.
	PeerName string
	// From is the prior state.
	From ConnectionState
	// To is the new state.
	To ConnectionState
	// Detail is a free-form short description, e.g. "dial failed:
	// connection refused" or "manual disconnect". May be empty.
	Detail string
	// Time records the emitter's wall clock when the transition
	// occurred. Set by the connection layer, not the subscriber.
	Time time.Time
}

// Sentinel errors raised by the diameter package and its sub-packages.
//
// Callers can branch on these via errors.Is. The connection layer
// wraps the underlying I/O error with one of these to keep the
// interface narrow.
var (
	// ErrPeerNotConnected is returned by send/disconnect operations
	// invoked while the peer is not in StateConnected. The Sender
	// (task 4) returns this when a CCR is requested for a peer
	// whose connection has dropped — see AC-14.
	ErrPeerNotConnected = errors.New("diameter: peer not connected")
	// ErrPeerAlreadyConnected is returned by Connect when invoked on
	// a peer whose lifecycle is already running. Connect is
	// idempotent at the start state but rejects double-claim of the
	// reconnect loop.
	ErrPeerAlreadyConnected = errors.New("diameter: peer already connected")
	// ErrInvalidPeerConfig is returned by Connect when the config
	// fails validation (empty host, unknown transport, …).
	ErrInvalidPeerConfig = errors.New("diameter: invalid peer config")
)
