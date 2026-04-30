package messaging

import (
	"errors"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
)

// CC-Request-Type values defined by RFC 4006 §8.3. Re-exposed at
// the package root so callers can build a CCR without importing
// the diam library directly.
const (
	// CCRTypeInitial is CC-Request-Type = 1 (INITIAL_REQUEST).
	CCRTypeInitial uint32 = 1
	// CCRTypeUpdate is CC-Request-Type = 2 (UPDATE_REQUEST).
	CCRTypeUpdate uint32 = 2
	// CCRTypeTerminate is CC-Request-Type = 3 (TERMINATION_REQUEST).
	CCRTypeTerminate uint32 = 3
	// CCRTypeEvent is CC-Request-Type = 4 (EVENT_REQUEST).
	CCRTypeEvent uint32 = 4
)

// CCR is the Go-native shape of a Credit-Control-Request the
// caller hands to the Sender. Required fields (per RFC 4006 §3.1)
// are surfaced as their own properties so the encoder can validate
// them; ExtraAVPs is the catch-all for anything else (Subscription-
// Id, User-Equipment-Info, Multiple-Services-Credit-Control,
// Service-Information sub-tree, vendor AVPs, …).
//
// The Sender populates Session-Id and the Origin-Host /
// Origin-Realm from the peer's PeerConfig if the caller leaves
// them empty — both are peer-scoped per ARCHITECTURE §4 and
// pre-filling them from the connection is the natural default.
//
// CCR is internal — it is NOT a contract serialised on the wire.
// The wire form is the *diam.Message produced by BuildCCRMessage.
// Field renames here do not break consumers of the wire protocol.
type CCR struct {
	// SessionID is the Session-Id AVP (RFC 6733 §8.8). The Sender
	// generates one when empty; setting it lets the caller chain
	// related CCRs (Initial → Update → Terminate) under the same
	// session.
	SessionID string

	// OriginHost / OriginRealm are the local peer's identity. The
	// Sender pre-fills these from PeerConfig when empty.
	OriginHost  string
	OriginRealm string

	// DestinationRealm is mandatory per RFC 4006. If empty, the
	// Sender uses PeerConfig.OriginRealm as a sensible default
	// (typical OCS deployments live in the same realm as the
	// testbench).
	DestinationRealm string
	// DestinationHost is optional (proxy/agent routing).
	DestinationHost string

	// AuthApplicationID is the Auth-Application-Id AVP. Defaults
	// to 4 (Credit-Control) when zero.
	AuthApplicationID uint32

	// ServiceContextID is the Service-Context-Id AVP. Required by
	// RFC 4006; the Sender does not invent a default — leaving it
	// empty produces a CCR that the OCS will reject with a 5xxx
	// permanent failure.
	ServiceContextID string

	// CCRequestType is the CC-Request-Type AVP. One of
	// CCRTypeInitial / CCRTypeUpdate / CCRTypeTerminate /
	// CCRTypeEvent.
	CCRequestType uint32
	// CCRequestNumber is the CC-Request-Number AVP. Caller-managed
	// so a session's numbering scheme stays consistent.
	CCRequestNumber uint32

	// EventTimestamp is the Event-Timestamp AVP. When zero, the
	// Sender stamps it with the current time at encode time.
	EventTimestamp time.Time

	// ExtraAVPs is the slice of additional AVPs to inject into the
	// CCR. The Sender appends these after the required AVPs are
	// laid down. Order in this slice is preserved on the wire.
	//
	// Typical contents: Subscription-Id, User-Equipment-Info,
	// Multiple-Services-Indicator, Multiple-Services-Credit-
	// Control (one or more), Service-Information sub-tree,
	// vendor-specific AVPs.
	ExtraAVPs []*diam.AVP
}

// MSCCBlock is the structured view of one Multiple-Services-Credit-
// Control AVP from a CCA. The Sender returns these alongside the
// raw AVP so the engine can read individual fields without
// re-parsing.
type MSCCBlock struct {
	// ServiceIdentifier is the Service-Identifier AVP from this
	// MSCC block, when present. Zero indicates absent (RFC 4006
	// reserves 0 only for default; absent is more common).
	ServiceIdentifier uint32
	// RatingGroup is the Rating-Group AVP, when present.
	RatingGroup uint32
	// ResultCode is the per-MSCC Result-Code AVP. Zero indicates
	// absent.
	ResultCode uint32
	// GrantedTime is the granted CC-Time, in seconds, when the
	// Granted-Service-Unit AVP carries a CC-Time sub-AVP. Zero
	// indicates "not granted" (the AVP may still grant
	// volume/money via the other sub-fields, not surfaced here).
	GrantedTime uint32
	// GrantedTotalOctets is the granted CC-Total-Octets, in
	// octets.
	GrantedTotalOctets uint64
	// ValidityTime is the Validity-Time AVP under this MSCC, in
	// seconds. Zero indicates absent.
	ValidityTime uint32
	// FUIAction is the Final-Unit-Action sub-AVP under this
	// MSCC's Final-Unit-Indication, when present. -1 indicates
	// "no FUI present".
	FUIAction int32
	// Raw is the original *diam.AVP. Engine layers that need
	// access to fields not surfaced above can read them from here.
	Raw *diam.AVP
}

// FinalUnitAction values per RFC 4006 §8.34 Final-Unit-Action.
const (
	// FUIActionTerminate signals the session must be terminated
	// (CCR-T) — task 5 owns the auto-emit.
	FUIActionTerminate int32 = 0
	// FUIActionRedirect signals the user is redirected.
	FUIActionRedirect int32 = 1
	// FUIActionRestrictAccess signals access is restricted to a
	// list of filters.
	FUIActionRestrictAccess int32 = 2
)

// CCA is the Go-native shape of a Credit-Control-Answer returned
// by the Sender. Mirrors CCR's surface for the headline fields
// plus the response-specific MSCC blocks and the top-level
// Validity-Time / Final-Unit-Indication.
type CCA struct {
	// SessionID echoes the CCR's Session-Id so callers can
	// correlate without holding state outside the CCR.
	SessionID string

	// OriginHost / OriginRealm identify the responding peer (the
	// OCS).
	OriginHost  string
	OriginRealm string

	// AuthApplicationID echoes the CCR's value.
	AuthApplicationID uint32

	// ResultCode is the top-level Result-Code AVP. RFC 6733 §7.1
	// defines the ranges; task 5's protocol-behaviour layer
	// branches on the 5xxx range for permanent failure.
	ResultCode uint32

	// CCRequestType / CCRequestNumber echo the CCR's values.
	CCRequestType   uint32
	CCRequestNumber uint32

	// ValidityTime is the top-level Validity-Time AVP, in
	// seconds, when present at the message level. Zero indicates
	// absent. (Per-MSCC Validity-Time lives on each MSCCBlock.)
	ValidityTime uint32

	// FUIAction is the message-level Final-Unit-Indication's
	// Final-Unit-Action sub-AVP, when present. -1 indicates
	// "no top-level FUI present". Per-MSCC FUI lives on
	// MSCCBlock.FUIAction.
	FUIAction int32

	// MSCC is the ordered list of decoded
	// Multiple-Services-Credit-Control AVPs in the CCA.
	MSCC []MSCCBlock

	// Raw is the *diam.Message returned by go-diameter, exposed
	// so callers that need a field not surfaced above can dig it
	// out via FindAVP / FindAVPs.
	Raw *diam.Message
}

// Sentinel errors raised by this package. Callers branch via
// errors.Is.
var (
	// ErrInvalidCCR is returned by the encoder when a required
	// field on the CCR is zero (Session-Id derivation aside).
	ErrInvalidCCR = errors.New("messaging: invalid CCR")
	// ErrCorrelatorClosed is returned by Send when the correlator
	// channel was closed before a response was received — signals
	// the connection dropped mid-request.
	ErrCorrelatorClosed = errors.New("messaging: response channel closed (peer dropped)")
)
