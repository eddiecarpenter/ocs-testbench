package messaging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
)

// BuildCCRMessage encodes a CCR onto a *diam.Message. The caller
// provides the dictionary parser that should be used (typically
// dict.Default after the loader has run); the output message
// carries that parser so it can be serialised to the wire.
//
// Validation:
//
//   - DestinationRealm must be non-empty (RFC 4006 §3.1).
//   - ServiceContextID must be non-empty (RFC 4006 §3.1).
//   - CCRequestType must be one of CCRTypeInitial / Update /
//     Terminate / Event.
//
// Defaults the encoder applies when the corresponding field is
// zero:
//
//   - SessionID — generated as "<originHost>;<unix-nanos>;<rand>".
//   - AuthApplicationID — 4 (Credit-Control).
//   - EventTimestamp — current time at encode time.
//
// The CCR's E2E and Hop-by-Hop IDs are populated by go-diameter's
// diam.NewRequest — the encoder does not assign them; instead the
// Sender uses them as the correlator key.
func BuildCCRMessage(parser *dict.Parser, req *CCR) (*diam.Message, error) {
	if parser == nil {
		return nil, fmt.Errorf("%w: parser is nil", ErrInvalidCCR)
	}
	if req == nil {
		return nil, fmt.Errorf("%w: ccr is nil", ErrInvalidCCR)
	}
	if strings.TrimSpace(req.DestinationRealm) == "" {
		return nil, fmt.Errorf("%w: destination-realm is empty", ErrInvalidCCR)
	}
	if strings.TrimSpace(req.ServiceContextID) == "" {
		return nil, fmt.Errorf("%w: service-context-id is empty", ErrInvalidCCR)
	}
	if req.CCRequestType < CCRTypeInitial || req.CCRequestType > CCRTypeEvent {
		return nil, fmt.Errorf("%w: cc-request-type %d out of range", ErrInvalidCCR, req.CCRequestType)
	}

	auth := req.AuthApplicationID
	if auth == 0 {
		auth = diameter.AppIDCreditControl
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = generateSessionID(req.OriginHost)
	}
	ts := req.EventTimestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	m := diam.NewRequest(diam.CreditControl, auth, parser)

	// Ordering follows the canonical RFC 4006 §3.1 layout. The
	// receiver does not enforce ordering, but staying close to the
	// AVP order in the standard makes pcap-debugging easier.
	m.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity(req.OriginHost))
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity(req.OriginRealm))
	m.NewAVP(avp.DestinationRealm, avp.Mbit, 0, datatype.DiameterIdentity(req.DestinationRealm))
	if strings.TrimSpace(req.DestinationHost) != "" {
		m.NewAVP(avp.DestinationHost, avp.Mbit, 0, datatype.DiameterIdentity(req.DestinationHost))
	}
	m.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(auth))
	m.NewAVP(avp.ServiceContextID, avp.Mbit, 0, datatype.UTF8String(req.ServiceContextID))
	m.NewAVP(avp.CCRequestType, avp.Mbit, 0, datatype.Enumerated(req.CCRequestType))
	m.NewAVP(avp.CCRequestNumber, avp.Mbit, 0, datatype.Unsigned32(req.CCRequestNumber))
	m.NewAVP(avp.EventTimestamp, avp.Mbit, 0, datatype.Time(ts))

	// Caller-supplied AVPs trail the canonical block. The encoder
	// does not deduplicate against the canonical AVPs above —
	// callers are responsible for not double-supplying Session-Id
	// etc. via ExtraAVPs. The OCS will reject duplicates with a
	// 5xxx anyway.
	for _, a := range req.ExtraAVPs {
		if a == nil {
			continue
		}
		m.AddAVP(a)
	}

	return m, nil
}

// generateSessionID renders an RFC 6733 §8.8-conformant Session-Id
// of the shape "<originHost>;<unix-secs>;<unix-nanos>;<rand-hex>".
// Falls back to a static prefix when originHost is empty.
func generateSessionID(originHost string) string {
	if strings.TrimSpace(originHost) == "" {
		originHost = "ocs-testbench.local"
	}
	now := time.Now().UTC()
	var randBytes [4]byte
	_, _ = rand.Read(randBytes[:])
	return fmt.Sprintf("%s;%d;%d;%s",
		originHost,
		now.Unix(),
		now.UnixNano(),
		hex.EncodeToString(randBytes[:]))
}
