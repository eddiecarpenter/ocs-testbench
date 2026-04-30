package dictionary

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam/dict"

	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
)

// fakeSource is a static stand-in for the production StoreSource.
// It returns a fixed slice of records and an optional error so each
// test case can drive Loader.Load down a specific branch.
type fakeSource struct {
	records []Record
	err     error
}

func (f fakeSource) ListActiveCustomDictionaries(_ context.Context) ([]Record, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.records, nil
}

// validCustomXML is a minimal-but-real Diameter dictionary fragment
// that registers a single AVP code under a synthetic application id.
// The application id (16777299) is the one go-diameter uses for its
// "Diameter Sy" testdata fixture, but the AVP code (98765) is unique
// enough that it cannot collide with any built-in or 3GPP-defined
// AVP. The vendor-id (99999) is similarly synthetic so the test
// asserts on data that no library upgrade will accidentally start
// shipping for us.
const validCustomXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
    <application id="16777299">
        <avp name="OCS-Testbench-Custom-AVP" code="98765" must="-" may="P" must-not="V" may-encrypt="N" vendor-id="99999">
            <data type="UTF8String"/>
        </avp>
    </application>
</diameter>`

// secondValidCustomXML registers a different synthetic AVP. Used in
// tests that exercise multiple successful loads in one Load() call.
const secondValidCustomXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
    <application id="16777299">
        <avp name="OCS-Testbench-Second-AVP" code="98766" must="-" may="P" must-not="V" may-encrypt="N" vendor-id="99999">
            <data type="UTF8String"/>
        </avp>
    </application>
</diameter>`

// invalidCustomXML is malformed XML — the closing tag does not
// match. dict.Parser.Load surfaces an XML decode error; the loader
// must log + skip rather than abort.
const invalidCustomXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
    <application id="16777299">
        <avp name="Bad" code="99999"
    </application>
</diameter>`

// Test 1 (AC-7) — built-in dictionaries are present in dict.Default
// after go-diameter's package init. Verifying via dict.Default
// directly documents the dependency on the library's init
// behaviour. Each dictionary is sentinel-checked at the right
// application id: Origin-Host at app 0 (RFC 6733 base),
// Service-Context-Id at app 4 (RFC 4006 credit-control), and a
// 3GPP-specific AVP via the same app 4 to confirm TS 32.299 is
// layered on top.
func TestBuiltInsAvailableInDefaultParser(t *testing.T) {
	t.Parallel()

	originHost, err := dict.Default.FindAVP(uint32(0), AVPCodeOriginHost)
	if err != nil {
		t.Fatalf("Origin-Host (264) not resolvable in dict.Default: %v", err)
	}
	if originHost.Name != "Origin-Host" {
		t.Errorf("expected AVP name Origin-Host, got %q", originHost.Name)
	}

	svcCtx, err := dict.Default.FindAVP(diameter.AppIDCreditControl, AVPCodeServiceContextID)
	if err != nil {
		t.Fatalf("Service-Context-Id (461) not resolvable in app 4: %v", err)
	}
	if svcCtx.Name != "Service-Context-Id" {
		t.Errorf("expected AVP name Service-Context-Id, got %q", svcCtx.Name)
	}

	// Service-Information (873, vendor 10415) is the canonical
	// 3GPP TS 32.299 AVP. Its presence in dict.Default proves the
	// TGPP dictionary loaded.
	svcInfo, err := dict.Default.FindAVP(diameter.AppIDCreditControl, uint32(873))
	if err != nil {
		t.Fatalf("Service-Information (873) not resolvable in app 4: %v", err)
	}
	if svcInfo.Name != "Service-Information" {
		t.Errorf("expected AVP name Service-Information, got %q", svcInfo.Name)
	}
}

// freshParserWithBase returns a brand-new dict.Parser pre-loaded
// with the same base XML that dict.Default carries. This lets each
// test case run against an isolated parser, so loading a custom XML
// in one test cannot collide with the same load in another.
//
// We pull the base XML out of dict.Default by looking up its first
// loaded application — but the simpler and more deterministic path
// is to ship a tiny inline base just covering the two sentinel AVPs
// the loader checks for. That keeps the test parser lean and
// independent of dict.Default's contents.
func freshParserWithBase(t *testing.T) *dict.Parser {
	t.Helper()
	p, err := dict.NewParser()
	if err != nil {
		t.Fatalf("dict.NewParser: %v", err)
	}
	const baseXML = `<?xml version="1.0" encoding="UTF-8"?>
<diameter>
    <application id="0" name="Base">
        <avp name="Origin-Host" code="264" must="M" may="P" must-not="V" may-encrypt="N">
            <data type="DiameterIdentity"/>
        </avp>
    </application>
    <application id="4" name="Diameter Credit Control">
        <avp name="Service-Context-Id" code="461" must="M" may="P" must-not="V" may-encrypt="N">
            <data type="UTF8String"/>
        </avp>
    </application>
</diameter>`
	if err := p.Load(bytes.NewReader([]byte(baseXML))); err != nil {
		t.Fatalf("freshParserWithBase: load base: %v", err)
	}
	return p
}

// Test 2 (AC-7+AC-8) — a custom XML extends the base parser. After
// Load, the AVP defined in the custom XML is resolvable; the
// pre-loaded base AVP remains resolvable.
func TestLoad_CustomExtendsBase(t *testing.T) {
	t.Parallel()

	p := freshParserWithBase(t)
	src := fakeSource{records: []Record{
		{Name: "ocs-custom-1", XMLContent: validCustomXML, IsActive: true},
	}}
	loader := NewLoaderForParser(src, p)

	res, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if want, got := []string{"ocs-custom-1"}, res.LoadedNames; !equalStrings(got, want) {
		t.Errorf("LoadedNames = %v; want %v", got, want)
	}
	if len(res.SkippedNames) != 0 {
		t.Errorf("SkippedNames = %v; want empty", res.SkippedNames)
	}

	if _, err := p.FindAVP(uint32(16777299), uint32(98765)); err != nil {
		t.Errorf("custom AVP 98765 not resolvable after Load: %v", err)
	}
	if _, err := p.FindAVP(uint32(0), AVPCodeOriginHost); err != nil {
		t.Errorf("base AVP Origin-Host disappeared after custom load: %v", err)
	}
}

// Test 3 (AC-9) — invalid custom XML is logged-and-skipped. The
// loader returns success, the bad row is in SkippedNames, and the
// remaining good rows still extend the base.
func TestLoad_InvalidCustomSkipped(t *testing.T) {
	t.Parallel()

	p := freshParserWithBase(t)
	src := fakeSource{records: []Record{
		{Name: "ocs-good-1", XMLContent: validCustomXML, IsActive: true},
		{Name: "ocs-bad-1", XMLContent: invalidCustomXML, IsActive: true},
		{Name: "ocs-good-2", XMLContent: secondValidCustomXML, IsActive: true},
	}}
	loader := NewLoaderForParser(src, p)

	res, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: must succeed when only some customs are bad: %v", err)
	}
	if want, got := []string{"ocs-good-1", "ocs-good-2"}, res.LoadedNames; !equalStrings(got, want) {
		t.Errorf("LoadedNames = %v; want %v", got, want)
	}
	if want, got := []string{"ocs-bad-1"}, res.SkippedNames; !equalStrings(got, want) {
		t.Errorf("SkippedNames = %v; want %v", got, want)
	}
	if _, err := p.FindAVP(uint32(16777299), uint32(98765)); err != nil {
		t.Errorf("good custom AVP 98765 not resolvable: %v", err)
	}
	if _, err := p.FindAVP(uint32(16777299), uint32(98766)); err != nil {
		t.Errorf("good custom AVP 98766 not resolvable: %v", err)
	}
}

// Test 4 (AC-9 edge) — every active custom record is invalid. Load
// still returns success and the base dictionary remains usable.
func TestLoad_AllCustomsInvalid_BaseStillUsable(t *testing.T) {
	t.Parallel()

	p := freshParserWithBase(t)
	src := fakeSource{records: []Record{
		{Name: "bad-1", XMLContent: invalidCustomXML, IsActive: true},
		{Name: "bad-2", XMLContent: invalidCustomXML, IsActive: true},
	}}
	loader := NewLoaderForParser(src, p)

	res, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: must succeed even when every custom is bad: %v", err)
	}
	if len(res.LoadedNames) != 0 {
		t.Errorf("LoadedNames = %v; want empty", res.LoadedNames)
	}
	if want, got := []string{"bad-1", "bad-2"}, res.SkippedNames; !equalStrings(got, want) {
		t.Errorf("SkippedNames = %v; want %v", got, want)
	}

	if _, err := p.FindAVP(uint32(0), AVPCodeOriginHost); err != nil {
		t.Errorf("base Origin-Host not resolvable after all-customs-failed load: %v", err)
	}
	if _, err := p.FindAVP(diameter.AppIDCreditControl, AVPCodeServiceContextID); err != nil {
		t.Errorf("base Service-Context-Id not resolvable after all-customs-failed load: %v", err)
	}
}

// Test 5 — store error during list propagates as a fatal error.
func TestLoad_SourceErrorPropagated(t *testing.T) {
	t.Parallel()

	p := freshParserWithBase(t)
	sentinel := errors.New("simulated db connection refused")
	src := fakeSource{err: sentinel}
	loader := NewLoaderForParser(src, p)

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatalf("Load: expected source error to propagate; got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("Load: returned error does not wrap source sentinel; got %v", err)
	}
	if !strings.Contains(err.Error(), "list custom dictionaries") {
		t.Errorf("Load: returned error missing context prefix; got %q", err.Error())
	}
}

// Test 6 — inactive rows are skipped before any parsing is
// attempted. Even an XML payload that would fail to parse is
// inert when IsActive is false.
func TestLoad_InactiveRowsIgnored(t *testing.T) {
	t.Parallel()

	p := freshParserWithBase(t)
	src := fakeSource{records: []Record{
		// Bad XML but inactive — must NOT appear in SkippedNames or
		// trigger a warn log; the loader filters before calling
		// Parser.Load.
		{Name: "inactive-bad", XMLContent: invalidCustomXML, IsActive: false},
		{Name: "active-good", XMLContent: validCustomXML, IsActive: true},
	}}
	loader := NewLoaderForParser(src, p)

	res, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want, got := []string{"active-good"}, res.LoadedNames; !equalStrings(got, want) {
		t.Errorf("LoadedNames = %v; want %v", got, want)
	}
	if len(res.SkippedNames) != 0 {
		t.Errorf("SkippedNames must be empty (inactive rows are ignored, not skipped); got %v", res.SkippedNames)
	}
}

// Test 7 (sanity) — the production NewLoader constructor wires
// dict.Default and an empty source produces a zero-customs success.
// Run sequentially (not in parallel) because it touches dict.Default.
func TestLoad_NewLoaderProductionConstructor(t *testing.T) {
	src := fakeSource{records: nil}
	loader := NewLoader(src)
	if loader.Parser != dict.Default {
		t.Fatalf("NewLoader: Parser = %p; want dict.Default %p", loader.Parser, dict.Default)
	}
	res, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.LoadedNames) != 0 || len(res.SkippedNames) != 0 {
		t.Errorf("expected empty result with empty source; got %+v", res)
	}
}

// Test 8 — the built-ins-missing sentinel fires when the parser
// passed in does not have RFC 6733 / RFC 4006 base loaded. This
// covers the test-only path NewLoaderForParser(p) where p is a
// fresh dict.NewParser(). The error wraps ErrBuiltInsMissing so
// callers can branch on it via errors.Is.
func TestLoad_BuiltInsMissingFromFreshParser(t *testing.T) {
	t.Parallel()

	emptyParser, err := dict.NewParser()
	if err != nil {
		t.Fatalf("dict.NewParser: %v", err)
	}
	loader := NewLoaderForParser(fakeSource{}, emptyParser)
	_, err = loader.Load(context.Background())
	if err == nil {
		t.Fatalf("Load: expected ErrBuiltInsMissing; got nil")
	}
	if !errors.Is(err, ErrBuiltInsMissing) {
		t.Errorf("Load: error not ErrBuiltInsMissing; got %v", err)
	}
}

// Test 9 — NewLoader rejects a nil source by panicking. The constructor
// guard prevents a NullPointer panic deeper in Load when a wiring bug
// passes a typed-nil interface.
func TestNewLoader_NilSourcePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("NewLoader(nil): expected panic; got none")
		}
	}()
	_ = NewLoader(nil)
}

// Test 10 — NewLoaderForParser rejects nil parser and nil source.
func TestNewLoaderForParser_NilArgsPanic(t *testing.T) {
	t.Parallel()

	t.Run("nil-source", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic on nil source")
			}
		}()
		_ = NewLoaderForParser(nil, &dict.Parser{})
	})

	t.Run("nil-parser", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic on nil parser")
			}
		}()
		_ = NewLoaderForParser(fakeSource{}, nil)
	})
}

// equalStrings compares two []string slices by ordered content.
// nil and []string{} are treated as equal (both length 0).
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
