package messaging

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// validCCR returns a CCR that should encode without error.
func validCCR() *CCR {
	return &CCR{
		SessionID:         "test;1234;0;abcd",
		OriginHost:        "tb.test.local",
		OriginRealm:       "test.local",
		DestinationRealm:  "ocs.test.local",
		AuthApplicationID: 4,
		ServiceContextID:  "32251@3gpp.org",
		CCRequestType:     CCRTypeInitial,
		CCRequestNumber:   0,
		EventTimestamp:    time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
	}
}

// Test 1 — happy path encoding produces a Diameter request
// with the correct command code and application id.
func TestBuildCCRMessage_HappyPath(t *testing.T) {
	t.Parallel()
	m, err := BuildCCRMessage(dict.Default, validCCR())
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	if m.Header.CommandCode != diam.CreditControl {
		t.Errorf("command code = %d; want %d", m.Header.CommandCode, diam.CreditControl)
	}
	if m.Header.ApplicationID != 4 {
		t.Errorf("application id = %d; want 4", m.Header.ApplicationID)
	}
	if m.Header.CommandFlags&diam.RequestFlag == 0 {
		t.Errorf("RequestFlag not set on header")
	}
	// Spot-check that key AVPs are present.
	for _, code := range []uint32{
		avp.SessionID,
		avp.OriginHost,
		avp.OriginRealm,
		avp.DestinationRealm,
		avp.AuthApplicationID,
		avp.ServiceContextID,
		avp.CCRequestType,
		avp.CCRequestNumber,
		avp.EventTimestamp,
	} {
		if _, err := m.FindAVP(code, 0); err != nil {
			t.Errorf("AVP code %d missing: %v", code, err)
		}
	}
}

// Test 2 — destination realm empty is rejected.
func TestBuildCCRMessage_DestinationRealmRequired(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.DestinationRealm = ""
	if _, err := BuildCCRMessage(dict.Default, r); !errors.Is(err, ErrInvalidCCR) {
		t.Errorf("err = %v; want ErrInvalidCCR", err)
	}
}

// Test 3 — service-context-id empty is rejected.
func TestBuildCCRMessage_ServiceContextIDRequired(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.ServiceContextID = ""
	if _, err := BuildCCRMessage(dict.Default, r); !errors.Is(err, ErrInvalidCCR) {
		t.Errorf("err = %v; want ErrInvalidCCR", err)
	}
}

// Test 4 — out-of-range CC-Request-Type is rejected.
func TestBuildCCRMessage_RequestTypeRange(t *testing.T) {
	t.Parallel()
	for _, badType := range []uint32{0, 5, 99} {
		r := validCCR()
		r.CCRequestType = badType
		if _, err := BuildCCRMessage(dict.Default, r); !errors.Is(err, ErrInvalidCCR) {
			t.Errorf("CCRequestType=%d err = %v; want ErrInvalidCCR", badType, err)
		}
	}
}

// Test 5 — Session-ID auto-generated when empty.
func TestBuildCCRMessage_SessionIDDefaulted(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.SessionID = ""
	m, err := BuildCCRMessage(dict.Default, r)
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	a, err := m.FindAVP(avp.SessionID, 0)
	if err != nil {
		t.Fatalf("Session-Id AVP not present: %v", err)
	}
	v, ok := a.Data.(datatype.UTF8String)
	if !ok {
		t.Fatalf("Session-Id type = %T; want UTF8String", a.Data)
	}
	if !strings.HasPrefix(string(v), r.OriginHost) {
		t.Errorf("Session-Id %q does not begin with origin host %q", v, r.OriginHost)
	}
}

// Test 6 — AuthApplicationID defaults to 4 when zero.
func TestBuildCCRMessage_AuthApplicationIDDefaulted(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.AuthApplicationID = 0
	m, err := BuildCCRMessage(dict.Default, r)
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	if m.Header.ApplicationID != 4 {
		t.Errorf("application id = %d; want 4", m.Header.ApplicationID)
	}
	a, err := m.FindAVP(avp.AuthApplicationID, 0)
	if err != nil {
		t.Fatalf("Auth-Application-Id AVP missing: %v", err)
	}
	if v, ok := a.Data.(datatype.Unsigned32); !ok || uint32(v) != 4 {
		t.Errorf("Auth-Application-Id AVP = %v; want 4", a.Data)
	}
}

// Test 7 — Destination-Host included when present, omitted when
// empty.
func TestBuildCCRMessage_DestinationHostOptional(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.DestinationHost = "ocs-edge.test.local"
	m, err := BuildCCRMessage(dict.Default, r)
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	if _, err := m.FindAVP(avp.DestinationHost, 0); err != nil {
		t.Errorf("Destination-Host AVP missing: %v", err)
	}

	r2 := validCCR()
	m2, err := BuildCCRMessage(dict.Default, r2)
	if err != nil {
		t.Fatalf("BuildCCRMessage 2: %v", err)
	}
	if _, err := m2.FindAVP(avp.DestinationHost, 0); err == nil {
		t.Errorf("Destination-Host AVP unexpectedly present in default-config CCR")
	}
}

// Test 8 — ExtraAVPs are appended verbatim.
func TestBuildCCRMessage_ExtraAVPsAppended(t *testing.T) {
	t.Parallel()
	r := validCCR()
	r.ExtraAVPs = []*diam.AVP{
		diam.NewAVP(avp.UserName, avp.Mbit, 0, datatype.UTF8String("alice")),
	}
	m, err := BuildCCRMessage(dict.Default, r)
	if err != nil {
		t.Fatalf("BuildCCRMessage: %v", err)
	}
	a, err := m.FindAVP(avp.UserName, 0)
	if err != nil {
		t.Fatalf("UserName AVP missing: %v", err)
	}
	if v, ok := a.Data.(datatype.UTF8String); !ok || string(v) != "alice" {
		t.Errorf("UserName AVP data = %v; want \"alice\"", a.Data)
	}
}

// Test 9 — distinct E2E IDs across consecutive builds (defensive
// against go-diameter regressions).
func TestBuildCCRMessage_DistinctE2EIDs(t *testing.T) {
	t.Parallel()
	m1, err := BuildCCRMessage(dict.Default, validCCR())
	if err != nil {
		t.Fatalf("BuildCCRMessage 1: %v", err)
	}
	m2, err := BuildCCRMessage(dict.Default, validCCR())
	if err != nil {
		t.Fatalf("BuildCCRMessage 2: %v", err)
	}
	if m1.Header.EndToEndID == m2.Header.EndToEndID {
		t.Errorf("identical end-to-end IDs: %d", m1.Header.EndToEndID)
	}
}

// Test 10 — nil parser / nil ccr rejected.
func TestBuildCCRMessage_NilGuards(t *testing.T) {
	t.Parallel()
	if _, err := BuildCCRMessage(nil, validCCR()); !errors.Is(err, ErrInvalidCCR) {
		t.Errorf("nil parser: err = %v; want ErrInvalidCCR", err)
	}
	if _, err := BuildCCRMessage(dict.Default, nil); !errors.Is(err, ErrInvalidCCR) {
		t.Errorf("nil ccr: err = %v; want ErrInvalidCCR", err)
	}
}
