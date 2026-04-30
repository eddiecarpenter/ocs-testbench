package messaging

import (
	"bytes"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// buildCCATestMessage assembles a CCA-shaped *diam.Message with
// the supplied result code, validity time, FUI, and one MSCC
// block. Used by the decoder tests.
func buildCCATestMessage(t *testing.T, sessionID string, resultCode uint32, validity uint32, fuiAction *int32, mscc *diam.GroupedAVP) *diam.Message {
	t.Helper()
	m := diam.NewMessage(diam.CreditControl, 0, 4, 1, 1, dict.Default)
	m.NewAVP(avp.SessionID, avp.Mbit, 0, datatype.UTF8String(sessionID))
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, datatype.DiameterIdentity("ocs.test"))
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, datatype.DiameterIdentity("test.local"))
	m.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4))
	m.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(resultCode))
	m.NewAVP(avp.CCRequestType, avp.Mbit, 0, datatype.Enumerated(CCRTypeInitial))
	m.NewAVP(avp.CCRequestNumber, avp.Mbit, 0, datatype.Unsigned32(0))
	if validity != 0 {
		m.NewAVP(avp.ValidityTime, avp.Mbit, 0, datatype.Unsigned32(validity))
	}
	if fuiAction != nil {
		m.NewAVP(avp.FinalUnitIndication, avp.Mbit, 0, &diam.GroupedAVP{
			AVP: []*diam.AVP{
				diam.NewAVP(avp.FinalUnitAction, avp.Mbit, 0, datatype.Enumerated(*fuiAction)),
			},
		})
	}
	if mscc != nil {
		m.NewAVP(avp.MultipleServicesCreditControl, avp.Mbit, 0, mscc)
	}
	return m
}

// Test 1 — happy path: decoder maps every headline field.
func TestDecodeCCAMessage_HappyPath(t *testing.T) {
	t.Parallel()
	m := buildCCATestMessage(t, "sess-1", 2001, 0, nil, nil)
	cca, err := DecodeCCAMessage(m)
	if err != nil {
		t.Fatalf("DecodeCCAMessage: %v", err)
	}
	if cca.SessionID != "sess-1" {
		t.Errorf("SessionID = %q; want sess-1", cca.SessionID)
	}
	if cca.OriginHost != "ocs.test" {
		t.Errorf("OriginHost = %q; want ocs.test", cca.OriginHost)
	}
	if cca.ResultCode != 2001 {
		t.Errorf("ResultCode = %d; want 2001", cca.ResultCode)
	}
	if cca.AuthApplicationID != 4 {
		t.Errorf("AuthApplicationID = %d; want 4", cca.AuthApplicationID)
	}
	if cca.CCRequestType != CCRTypeInitial {
		t.Errorf("CCRequestType = %d; want %d", cca.CCRequestType, CCRTypeInitial)
	}
	if cca.FUIAction != -1 {
		t.Errorf("FUIAction = %d; want -1 (absent)", cca.FUIAction)
	}
}

// Test 2 — Validity-Time present is surfaced.
func TestDecodeCCAMessage_ValidityTime(t *testing.T) {
	t.Parallel()
	m := buildCCATestMessage(t, "s", 2001, 1800, nil, nil)
	cca, err := DecodeCCAMessage(m)
	if err != nil {
		t.Fatalf("DecodeCCAMessage: %v", err)
	}
	if cca.ValidityTime != 1800 {
		t.Errorf("ValidityTime = %d; want 1800", cca.ValidityTime)
	}
}

// Test 3 — top-level FUI = TERMINATE is surfaced.
func TestDecodeCCAMessage_FUITerminate(t *testing.T) {
	t.Parallel()
	terminate := FUIActionTerminate
	m := buildCCATestMessage(t, "s", 2001, 0, &terminate, nil)
	cca, err := DecodeCCAMessage(m)
	if err != nil {
		t.Fatalf("DecodeCCAMessage: %v", err)
	}
	if cca.FUIAction != FUIActionTerminate {
		t.Errorf("FUIAction = %d; want TERMINATE (%d)", cca.FUIAction, FUIActionTerminate)
	}
}

// Test 4 — MSCC block decoding: per-MSCC Result-Code, granted
// units, validity, FUI.
func TestDecodeCCAMessage_MSCCBlock(t *testing.T) {
	t.Parallel()
	mscc := &diam.GroupedAVP{
		AVP: []*diam.AVP{
			diam.NewAVP(avp.RatingGroup, avp.Mbit, 0, datatype.Unsigned32(7)),
			diam.NewAVP(avp.ServiceIdentifier, avp.Mbit, 0, datatype.Unsigned32(13)),
			diam.NewAVP(avp.ResultCode, avp.Mbit, 0, datatype.Unsigned32(2001)),
			diam.NewAVP(avp.ValidityTime, avp.Mbit, 0, datatype.Unsigned32(900)),
			diam.NewAVP(avp.GrantedServiceUnit, avp.Mbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.CCTime, avp.Mbit, 0, datatype.Unsigned32(60)),
					diam.NewAVP(avp.CCTotalOctets, avp.Mbit, 0, datatype.Unsigned64(1<<20)),
				},
			}),
			diam.NewAVP(avp.FinalUnitIndication, avp.Mbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.FinalUnitAction, avp.Mbit, 0, datatype.Enumerated(FUIActionTerminate)),
				},
			}),
		},
	}
	m := buildCCATestMessage(t, "s", 2001, 0, nil, mscc)
	cca, err := DecodeCCAMessage(m)
	if err != nil {
		t.Fatalf("DecodeCCAMessage: %v", err)
	}
	if len(cca.MSCC) != 1 {
		t.Fatalf("MSCC count = %d; want 1", len(cca.MSCC))
	}
	b := cca.MSCC[0]
	if b.RatingGroup != 7 {
		t.Errorf("RatingGroup = %d; want 7", b.RatingGroup)
	}
	if b.ServiceIdentifier != 13 {
		t.Errorf("ServiceIdentifier = %d; want 13", b.ServiceIdentifier)
	}
	if b.ResultCode != 2001 {
		t.Errorf("Per-MSCC ResultCode = %d; want 2001", b.ResultCode)
	}
	if b.ValidityTime != 900 {
		t.Errorf("ValidityTime = %d; want 900", b.ValidityTime)
	}
	if b.GrantedTime != 60 {
		t.Errorf("GrantedTime = %d; want 60", b.GrantedTime)
	}
	if b.GrantedTotalOctets != 1<<20 {
		t.Errorf("GrantedTotalOctets = %d; want %d", b.GrantedTotalOctets, 1<<20)
	}
	if b.FUIAction != FUIActionTerminate {
		t.Errorf("Per-MSCC FUIAction = %d; want TERMINATE", b.FUIAction)
	}
	if b.Raw == nil {
		t.Errorf("Raw AVP = nil; want non-nil")
	}
}

// Test 5 — nil message rejected.
func TestDecodeCCAMessage_NilMessage(t *testing.T) {
	t.Parallel()
	_, err := DecodeCCAMessage(nil)
	if err == nil {
		t.Errorf("nil message: expected error; got nil")
	}
}

// Test 6 — encode-decode round trip via go-diameter
// serialization. Goes through the full Marshal → bytes → ReadMessage
// pipeline so the test catches regressions in either direction.
func TestEncodeDecodeRoundtrip(t *testing.T) {
	t.Parallel()
	enc, err := BuildCCRMessage(dict.Default, validCCR())
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	raw, err := enc.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	parsed, err := diam.ReadMessage(bytes.NewReader(raw), dict.Default)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	// Round-tripped E2E IDs must match.
	if parsed.Header.EndToEndID != enc.Header.EndToEndID {
		t.Errorf("E2E mismatch: parsed=%d enc=%d", parsed.Header.EndToEndID, enc.Header.EndToEndID)
	}
}
