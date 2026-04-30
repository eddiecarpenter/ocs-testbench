// Template loader — assembles an EngineInput from the store, the
// execution context, and injectable generator/dictionary providers.

package template

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// UnitType selects the inner CC-* AVP for service-unit encoding per
// docs/ARCHITECTURE.md §3 "Unit type".
type UnitType string

// Unit type constants mirror the scenario.unitType domain values.
const (
	// UnitTypeOctet selects CC-Total-Octets as the inner AVP.
	UnitTypeOctet UnitType = "OCTET"
	// UnitTypeTime selects CC-Time as the inner AVP.
	UnitTypeTime UnitType = "TIME"
	// UnitTypeUnits selects CC-Service-Specific-Units as the inner AVP.
	UnitTypeUnits UnitType = "UNITS"
)

// ServiceModel determines how MSCC blocks are structured and whether
// the Multiple-Services-Indicator AVP is emitted per
// docs/ARCHITECTURE.md §4 "Services".
type ServiceModel string

// Service model constants mirror the scenario.serviceModel domain values.
const (
	// ServiceModelRoot places RSU/USU directly under the CCR root
	// with no MSCC block and no MSI AVP.
	ServiceModelRoot ServiceModel = "root"
	// ServiceModelSingleMSCC wraps RSU/USU in a single MSCC block
	// and emits MSI=0.
	ServiceModelSingleMSCC ServiceModel = "single-mscc"
	// ServiceModelMultiMSCC emits one MSCC block per service entry
	// and adds MSI=1.
	ServiceModelMultiMSCC ServiceModel = "multi-mscc"
)

// Dictionary is the AVP lookup surface used by the engine to validate
// AVP names and determine the correct Diameter encoding type.
//
// Defined at the consumer point (here, per Go interface convention) so
// the engine can be unit-tested with a fake without importing
// go-diameter's dict.Parser. The production wiring adapts
// dict.Parser to this interface in cmd/ocs-testbench/main.go.
type Dictionary interface {
	// Lookup resolves an AVP name to its metadata. Returns
	// UNKNOWN_AVP if the name is not found in the loaded dictionary.
	Lookup(name string) (AVPMetadata, error)
}

// AVPMetadata is the dictionary information about a single AVP
// returned by Dictionary.Lookup.
type AVPMetadata struct {
	// Code is the AVP code as defined in the Diameter dictionary.
	Code uint32
	// VendorID is the vendor identifier (0 for IETF base AVPs).
	VendorID uint32
	// DataType is the Diameter data type name as reported by the
	// dictionary (e.g. "Unsigned32", "UTF8String", "Grouped").
	DataType string
	// Grouped is true when the AVP is a grouped AVP (contains child
	// AVPs rather than a primitive value). Derived from
	// DataType == "Grouped".
	Grouped bool
}

// GeneratorProvider auto-generates values for system-managed tokens
// (SESSION_ID, CHARGING_ID, CC_REQUEST_NUMBER, EVENT_TIMESTAMP, etc.).
//
// The interface is injectable so unit tests can supply deterministic
// values and avoid non-reproducible test assertions. The production
// implementation (NewGeneratorProvider) uses crypto/rand and
// wall-clock time.
type GeneratorProvider interface {
	// Generate returns the current value for the named token.
	// Returns an error for unrecognised token names.
	Generate(name string) (any, error)
}

// EngineInput is the contract between the Loader and the Engine (and
// any caller that builds the input directly).
//
// The Loader produces an EngineInput by reading the avp_template row,
// resolving the value map, and attaching the dictionary and unit
// metadata. Any caller can also construct an EngineInput directly —
// the Engine has no store dependency and does not distinguish between
// the two construction paths (AC-13).
type EngineInput struct {
	// Tree is the parsed AVP tree from the template body.
	Tree []AVPNode

	// MSCC is the parsed MSCC block list from the template body.
	MSCC []MSCCTemplateBlock

	// Values is the fully-resolved value map assembled by the Loader:
	// generated values (lowest priority), then static template
	// defaults, then execution-context variables, then step-level
	// overrides (highest priority). The Engine substitutes
	// {{PLACEHOLDER}} tokens from this map.
	Values map[string]any

	// Dictionary is the AVP lookup surface. The Engine validates
	// every AVP name via Lookup before encoding. May be nil during
	// unit tests that exercise only loader behaviour (the Engine
	// will error if Dictionary is nil when it runs).
	Dictionary Dictionary

	// UnitType selects the inner CC-* AVP for service-unit encoding.
	UnitType UnitType

	// ServiceModel determines MSCC placement and MSI emission.
	ServiceModel ServiceModel
}

// ScenarioCtx carries the execution-time context supplied by the
// caller when loading a template.
type ScenarioCtx struct {
	// Variables are the execution-context variable values resolved at
	// execution start — user-declared scenario variables and system
	// variables such as MSISDN, ORIGIN_HOST, SERVICE_CONTEXT_ID.
	// These override the template's StaticValues.
	Variables map[string]string

	// Overrides are per-step transient substitutions that take
	// precedence over Variables for this request only. Applied last
	// in the value-map assembly.
	Overrides map[string]string

	// UnitType selects the inner CC-* AVP.
	UnitType UnitType

	// ServiceModel determines MSCC structure and MSI emission.
	ServiceModel ServiceModel
}

// Loader reads an avp_template row from the store, assembles the
// resolved value map, and returns an EngineInput ready for the
// Engine to consume.
//
// Construct via NewLoader; the zero value is not usable.
type Loader struct {
	store store.Store
	gen   GeneratorProvider
	dict  Dictionary
}

// NewLoader constructs a Loader. All three dependencies are required:
//   - s     — store.Store for fetching avp_template rows
//   - gen   — GeneratorProvider for auto-generated values
//   - dict  — Dictionary attached verbatim to each EngineInput
func NewLoader(s store.Store, gen GeneratorProvider, dict Dictionary) *Loader {
	return &Loader{store: s, gen: gen, dict: dict}
}

// Load fetches the avp_template row identified by templateID, parses
// its body, assembles the resolved value map, and returns an
// EngineInput.
//
// Value-map assembly order (later layers override earlier):
//  1. Generated values — auto-generated for each token in
//     TemplateBody.GeneratedValues.
//  2. Static values — from TemplateBody.StaticValues (override generated).
//  3. Variable values — from scenCtx.Variables (override static).
//  4. Step-level overrides — from scenCtx.Overrides (override all).
//
// Returns store.ErrNotFound (via errors.Is) when the template does
// not exist.
func (l *Loader) Load(
	ctx context.Context,
	templateID pgtype.UUID,
	scenCtx ScenarioCtx,
) (EngineInput, error) {
	row, err := l.store.GetAVPTemplate(ctx, templateID)
	if err != nil {
		return EngineInput{}, fmt.Errorf("template loader: fetch template: %w", err)
	}

	body, err := ParseTemplateBody(row.Body)
	if err != nil {
		return EngineInput{}, fmt.Errorf("template loader: parse body: %w", err)
	}

	values, err := l.assembleValues(body, scenCtx)
	if err != nil {
		return EngineInput{}, fmt.Errorf("template loader: assemble values: %w", err)
	}

	return EngineInput{
		Tree:         body.AVPs,
		MSCC:         body.MSCC,
		Values:       values,
		Dictionary:   l.dict,
		UnitType:     scenCtx.UnitType,
		ServiceModel: scenCtx.ServiceModel,
	}, nil
}

// assembleValues builds the resolved value map by merging the four
// sources in priority order (generated → static → variable →
// override).
func (l *Loader) assembleValues(body TemplateBody, scenCtx ScenarioCtx) (map[string]any, error) {
	result := make(map[string]any, len(body.StaticValues)+len(scenCtx.Variables)+len(scenCtx.Overrides))

	// 1. Generated values — lowest priority. Called for every key
	//    declared in GeneratedValues; static and variable layers will
	//    override any of these that have an explicit value.
	for _, key := range body.GeneratedValues {
		val, err := l.gen.Generate(key)
		if err != nil {
			return nil, fmt.Errorf("generate %q: %w", key, err)
		}
		result[key] = val
	}

	// 2. Static values from the template body — override generated.
	for k, v := range body.StaticValues {
		result[k] = v
	}

	// 3. Execution-context variables — override static.
	for k, v := range scenCtx.Variables {
		result[k] = v
	}

	// 4. Per-step overrides — highest priority.
	for k, v := range scenCtx.Overrides {
		result[k] = v
	}

	return result, nil
}
