package template

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// ---- test helpers --------------------------------------------------

// fakeDict is a no-op Dictionary used in loader tests. The loader
// does not call Lookup; it just attaches the dictionary to EngineInput.
type fakeDict struct{}

func (fakeDict) Lookup(name string) (AVPMetadata, error) {
	return AVPMetadata{}, fmt.Errorf("fakeDict: Lookup not expected in loader tests")
}

// fakeGenerator returns deterministic values for all known token names.
type fakeGenerator struct {
	counter uint32
}

func (g *fakeGenerator) Generate(name string) (any, error) {
	switch name {
	case TokenSessionID:
		return "testhost;11111111;22222222", nil
	case TokenChargingID:
		return uint32(12345), nil
	case TokenCCRequestNumber:
		v := g.counter
		g.counter++
		return v, nil
	case TokenEventTimestamp:
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil
	default:
		return nil, fmt.Errorf("fakeGenerator: unknown token %q", name)
	}
}

// failGenerator always returns an error, to test error propagation.
type failGenerator struct{ reason string }

func (f *failGenerator) Generate(name string) (any, error) {
	return nil, fmt.Errorf("failGenerator: %s", f.reason)
}

// makeTemplate creates an avp_template row in the test store and
// returns its pgtype.UUID.
func makeTemplate(t *testing.T, s store.Store, name string, body TemplateBody) pgtype.UUID {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	row, err := s.InsertAVPTemplate(context.Background(), name, raw)
	require.NoError(t, err)
	return row.ID
}

// newTestLoader creates a Loader wired to the in-memory test store.
func newTestLoader(s store.Store, gen GeneratorProvider) *Loader {
	return NewLoader(s, gen, fakeDict{})
}

// ---- tests ---------------------------------------------------------

// TestLoader_Load_StaticOnlyTemplate verifies the success path when
// the template has only static values (no variables, no generators).
func TestLoader_Load_StaticOnlyTemplate(t *testing.T) {
	s := store.NewTestStore()
	body := TemplateBody{
		Version: 1,
		AVPs: []AVPNode{
			{Name: "Origin-Host", Value: "{{ORIGIN_HOST}}"},
		},
		StaticValues: map[string]string{
			"ORIGIN_HOST": "ocs-testbench.local",
		},
	}
	id := makeTemplate(t, s, "static-only", body)

	l := newTestLoader(s, &fakeGenerator{})
	input, err := l.Load(context.Background(), id, ScenarioCtx{
		UnitType:     UnitTypeOctet,
		ServiceModel: ServiceModelMultiMSCC,
	})
	require.NoError(t, err)

	assert.Equal(t, "ocs-testbench.local", input.Values["ORIGIN_HOST"])
	assert.Equal(t, UnitTypeOctet, input.UnitType)
	assert.Equal(t, ServiceModelMultiMSCC, input.ServiceModel)
	assert.IsType(t, fakeDict{}, input.Dictionary)
}

// TestLoader_Load_AllFourValueSources verifies that all four value
// sources are assembled with correct precedence.
func TestLoader_Load_AllFourValueSources(t *testing.T) {
	s := store.NewTestStore()
	body := TemplateBody{
		Version: 1,
		AVPs:    []AVPNode{{Name: "Origin-Host", Value: "{{ORIGIN_HOST}}"}},
		StaticValues: map[string]string{
			"STATIC_KEY":    "static-value",
			"OVERRIDE_KEY":  "will-be-overridden-by-variable",
			"OVERRIDE_KEY2": "will-be-overridden-by-step",
		},
		GeneratedValues: []string{TokenSessionID, TokenChargingID},
	}
	id := makeTemplate(t, s, "all-sources", body)

	l := newTestLoader(s, &fakeGenerator{})
	input, err := l.Load(context.Background(), id, ScenarioCtx{
		Variables: map[string]string{
			"OVERRIDE_KEY":  "variable-value",  // overrides static
			"OVERRIDE_KEY2": "variable-value-2", // will be overridden by step
			"VARIABLE_ONLY": "from-context",
		},
		Overrides: map[string]string{
			"OVERRIDE_KEY2": "step-override-value", // highest priority
		},
		UnitType:     UnitTypeTime,
		ServiceModel: ServiceModelSingleMSCC,
	})
	require.NoError(t, err)

	// Static value — no override
	assert.Equal(t, "static-value", input.Values["STATIC_KEY"])

	// Variable overrides static
	assert.Equal(t, "variable-value", input.Values["OVERRIDE_KEY"])

	// Step override overrides variable
	assert.Equal(t, "step-override-value", input.Values["OVERRIDE_KEY2"])

	// Variable-only key
	assert.Equal(t, "from-context", input.Values["VARIABLE_ONLY"])

	// Generated values are present (from fakeGenerator)
	assert.Equal(t, "testhost;11111111;22222222", input.Values[TokenSessionID])
	assert.Equal(t, uint32(12345), input.Values[TokenChargingID])
}

// TestLoader_Load_StepOverrideIsolation verifies that a step override
// targeting one placeholder (e.g. RG100_REQUESTED) does not affect
// other placeholders in the value map (AC-10).
func TestLoader_Load_StepOverrideIsolation(t *testing.T) {
	s := store.NewTestStore()
	body := TemplateBody{
		Version: 1,
		AVPs:    []AVPNode{{Name: "Origin-Host", Value: "{{ORIGIN_HOST}}"}},
		MSCC: []MSCCTemplateBlock{
			{RatingGroup: 100, ServiceIdentifier: 1, Requested: "{{RG100_REQUESTED}}", Used: "{{RG100_USED}}"},
			{RatingGroup: 200, ServiceIdentifier: 2, Requested: "{{RG200_REQUESTED}}", Used: "{{RG200_USED}}"},
		},
		StaticValues: map[string]string{
			"ORIGIN_HOST":    "ocs.local",
			"RG100_REQUESTED": "1024",
			"RG100_USED":     "512",
			"RG200_REQUESTED": "2048",
			"RG200_USED":     "1024",
		},
	}
	id := makeTemplate(t, s, "step-override-isolation", body)

	l := newTestLoader(s, &fakeGenerator{})
	input, err := l.Load(context.Background(), id, ScenarioCtx{
		// Override only RG100_REQUESTED — RG200 must be unchanged.
		Overrides:    map[string]string{"RG100_REQUESTED": "99999"},
		UnitType:     UnitTypeOctet,
		ServiceModel: ServiceModelMultiMSCC,
	})
	require.NoError(t, err)

	// Override target changed.
	assert.Equal(t, "99999", input.Values["RG100_REQUESTED"])

	// Other RG100 fields unchanged.
	assert.Equal(t, "512", input.Values["RG100_USED"])

	// RG200 completely untouched.
	assert.Equal(t, "2048", input.Values["RG200_REQUESTED"])
	assert.Equal(t, "1024", input.Values["RG200_USED"])
}

// TestLoader_Load_MissingTemplate verifies store.ErrNotFound is
// propagated when the template ID does not exist.
func TestLoader_Load_MissingTemplate_ReturnsNotFound(t *testing.T) {
	s := store.NewTestStore()
	l := newTestLoader(s, &fakeGenerator{})

	// Use a non-existent UUID.
	var id pgtype.UUID
	id.Bytes = [16]byte{0xff, 0xff}
	id.Valid = true

	_, err := l.Load(context.Background(), id, ScenarioCtx{})
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// TestLoader_Load_InvalidJSON verifies an error is returned when the
// template body is not valid JSON.
func TestLoader_Load_InvalidJSON_ReturnsError(t *testing.T) {
	s := store.NewTestStore()
	// Insert a template with malformed JSON body directly.
	row, err := s.InsertAVPTemplate(context.Background(), "bad-json", []byte("not-json"))
	require.NoError(t, err)

	l := newTestLoader(s, &fakeGenerator{})
	_, err = l.Load(context.Background(), row.ID, ScenarioCtx{})
	require.Error(t, err)
}

// TestLoader_Load_UnknownVersion verifies UNKNOWN_TEMPLATE_VERSION is
// returned when the body has an unsupported version.
func TestLoader_Load_UnknownVersion_ReturnsTemplateError(t *testing.T) {
	s := store.NewTestStore()
	raw, _ := json.Marshal(map[string]any{"version": 42, "avps": []any{}})
	row, err := s.InsertAVPTemplate(context.Background(), "unknown-version", raw)
	require.NoError(t, err)

	l := newTestLoader(s, &fakeGenerator{})
	_, err = l.Load(context.Background(), row.ID, ScenarioCtx{})
	require.Error(t, err)

	var te *TemplateError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, ErrCodeUnknownTemplateVersion, te.Code)
}

// TestLoader_Load_GeneratorError propagates errors from
// GeneratorProvider.Generate.
func TestLoader_Load_GeneratorError_Propagated(t *testing.T) {
	s := store.NewTestStore()
	body := TemplateBody{
		Version:         1,
		GeneratedValues: []string{TokenSessionID},
	}
	id := makeTemplate(t, s, "gen-error", body)

	l := NewLoader(s, &failGenerator{reason: "injected failure"}, fakeDict{})
	_, err := l.Load(context.Background(), id, ScenarioCtx{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected failure")
}

// TestLoader_Load_DeterministicOutput verifies the loader produces the
// same EngineInput for the same (templateID, scenCtx, generator) triple.
func TestLoader_Load_DeterministicOutput(t *testing.T) {
	s := store.NewTestStore()
	body := TemplateBody{
		Version: 1,
		AVPs:    []AVPNode{{Name: "Origin-Host", Value: "{{ORIGIN_HOST}}"}},
		StaticValues: map[string]string{
			"ORIGIN_HOST": "deterministic.local",
		},
	}
	id := makeTemplate(t, s, "deterministic", body)

	ctx := context.Background()
	sc := ScenarioCtx{UnitType: UnitTypeTime, ServiceModel: ServiceModelSingleMSCC}

	// Reset generator to same state for both calls by using two
	// independent fakeGenerators (they start at counter=0 each time).
	l1 := newTestLoader(s, &fakeGenerator{})
	l2 := newTestLoader(s, &fakeGenerator{})

	in1, err1 := l1.Load(ctx, id, sc)
	in2, err2 := l2.Load(ctx, id, sc)

	require.NoError(t, err1)
	require.NoError(t, err2)

	assert.Equal(t, in1.Values, in2.Values)
	assert.Equal(t, in1.UnitType, in2.UnitType)
	assert.Equal(t, in1.ServiceModel, in2.ServiceModel)
}
