package dictionary

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/fiorix/go-diameter/v4/diam/dict"

	"github.com/eddiecarpenter/ocs-testbench/internal/logging"
)

// Built-in AVP code constants exposed for callers that want to
// reference canonical codes by name rather than a magic number.
//
// Source: RFC 6733 §4.5 (base) and RFC 4006 §8 (credit-control).
//
// Origin-Host (264) is also the loader's sentinel for "RFC 6733 base
// is present" — it sits in application id 0 (Base) and is therefore
// resolvable via FindAVP(0, 264) without touching any child app.
// Service-Context-Id (461) belongs to application id 4
// (Credit-Control); the loader does not use it as a sentinel because
// FindAVP(0, 461) does not walk into child apps. Callers that want
// to assert the credit-control dictionary is present should resolve
// it via FindAVP(diameter.AppIDCreditControl, AVPCodeServiceContextID).
const (
	// AVPCodeOriginHost is the AVP code for Origin-Host (RFC 6733
	// base, application id 0).
	AVPCodeOriginHost = uint32(264)
	// AVPCodeServiceContextID is the AVP code for Service-Context-Id
	// (RFC 4006 credit-control, application id 4).
	AVPCodeServiceContextID = uint32(461)
)

// Record is the loader's view of a single custom_dictionary row.
//
// Flattened from the sqlc-generated store row (which uses pgtype
// wrappers for nullability) so the loader and its tests do not need
// to depend on pgtype. The orchestrator wires a thin adapter that
// projects the store row onto this shape — see StoreSource for the
// production adapter.
//
// IsActive is captured here rather than filtered at the source
// boundary so the loader's own filter logic (skip inactive rows) is
// observable in test inputs.
type Record struct {
	Name       string // unique custom-dictionary name; used in log lines
	XMLContent string // raw <diameter>…</diameter> XML payload
	IsActive   bool   // when false the loader ignores the row entirely
}

// Source is the read-only dependency the loader needs to discover
// custom dictionary records. The production wiring satisfies it via
// internal/store.Store; tests use a small in-memory implementation.
//
// Implementations MUST be safe for one call per loader invocation —
// the loader does not retry. Errors are propagated to the caller as
// fatal because the loader cannot proceed without the list (an empty
// list is a valid result and falls through to a base-only load).
type Source interface {
	// ListActiveCustomDictionaries returns the records the loader
	// should attempt to apply on top of dict.Default. Implementations
	// MAY return rows where IsActive is false; the loader filters
	// them out. Returning an error halts the load with that error
	// propagated to the caller.
	ListActiveCustomDictionaries(ctx context.Context) ([]Record, error)
}

// LoadResult summarises one Loader.Load run for the caller. The
// production caller (the application bootstrap) uses these counters
// to log a single info line summarising the dictionary subsystem
// state at startup; tests assert on the fields to verify the
// fail-soft branches behave.
//
// Skipped records are listed by name so the operator can find the
// offending row in the custom_dictionary table without grepping
// logs.
type LoadResult struct {
	// LoadedNames is the names of every custom dictionary that
	// successfully extended the base parser, in the order they were
	// applied.
	LoadedNames []string
	// SkippedNames is the names of every custom dictionary that
	// failed to parse and was logged-and-skipped.
	SkippedNames []string
}

// ErrBuiltInsMissing is returned by Load when the built-in
// dictionaries shipped by go-diameter are not present in
// dict.Default. In normal operation this cannot fire — go-diameter
// loads them in its package init() — but keeping the explicit check
// fails fast and documented if a future library upgrade ever drops
// one of the bundled dictionaries.
var ErrBuiltInsMissing = errors.New(
	"dictionary: go-diameter built-in dictionaries (RFC 6733 / RFC 4006) not present in dict.Default")

// Loader extends the package-level go-diameter dictionary
// (dict.Default) with active custom XML records read from a Source.
//
// A Loader is single-use per startup: its Load method is called
// exactly once after the store is up but before any peer connection
// is opened. Re-running Load on the same parser would attempt to
// re-load the same custom XML and the underlying dict.Parser rejects
// duplicate command codes, so re-running is unsafe.
//
// The parser pointer is exposed via the Parser field so tests
// (and downstream packages) can construct a Loader against a fresh
// parser rather than mutating dict.Default — this keeps unit tests
// hermetic. Production constructs the loader via NewLoader (which
// uses dict.Default) so the rest of the diameter package family,
// which transitively reads dict.Default, sees the loaded customs.
type Loader struct {
	// Parser is the dictionary the loader extends. Set to
	// dict.Default by NewLoader so production code paths share the
	// dictionary that go-diameter's networking layer queries.
	Parser *dict.Parser

	// Source supplies the active custom_dictionary records. Always
	// non-nil after construction.
	Source Source
}

// NewLoader constructs a Loader that extends the package-level
// dict.Default. Pass the production store (via the StoreSource
// adapter) so the loaded customs are visible to the rest of the
// diameter package family.
//
// Panics on a nil source — the loader cannot operate without one and
// returning an error from a constructor here would push the same
// guard into every call site for no benefit.
func NewLoader(source Source) *Loader {
	if source == nil {
		panic("dictionary.NewLoader: source must not be nil")
	}
	return &Loader{Parser: dict.Default, Source: source}
}

// NewLoaderForParser is the test-friendly constructor: the caller
// supplies the parser the loader should extend. Used by unit tests
// to keep dict.Default unmutated, which lets test cases run in
// arbitrary order without polluting each other.
//
// Production code should not call this — the rest of the diameter
// package family reads dict.Default and would not see customs
// loaded into a side parser.
func NewLoaderForParser(source Source, parser *dict.Parser) *Loader {
	if source == nil {
		panic("dictionary.NewLoaderForParser: source must not be nil")
	}
	if parser == nil {
		panic("dictionary.NewLoaderForParser: parser must not be nil")
	}
	return &Loader{Parser: parser, Source: source}
}

// Load applies every active custom dictionary row from the loader's
// Source on top of the loader's Parser, returning a summary of the
// run.
//
// Behaviour:
//
//   - The built-in dictionaries shipped by go-diameter are NOT
//     loaded here — go-diameter's package init() loads them into
//     dict.Default. The loader verifies a sentinel base AVP
//     (Origin-Host) is resolvable on the parser and returns
//     ErrBuiltInsMissing if it is not. This catches the only scenario
//     where the verification fails: the caller passed a fresh
//     dict.NewParser() (test path) without first pre-loading the
//     base dictionary.
//
//   - Source errors abort the load and are returned to the caller
//     with severity = ERROR.
//
//   - Inactive rows are silently skipped.
//
//   - An invalid active row (XML parser error, duplicate command,
//     unsupported AVP type) is logged at WARN with its name and the
//     parser error and added to LoadResult.SkippedNames. The loader
//     continues with the next row; success is the default outcome
//     even if every active row fails.
//
//   - Successful rows are appended to LoadResult.LoadedNames in
//     application order.
//
// Concurrency: the underlying dict.Parser.Load is mutex-guarded, so
// the loader is safe to call while other goroutines are reading the
// parser, but it is NOT designed to be called more than once on the
// same parser (duplicate command code errors).
func (l *Loader) Load(ctx context.Context) (LoadResult, error) {
	if l == nil || l.Parser == nil || l.Source == nil {
		return LoadResult{}, errors.New("dictionary.Loader: misconfigured (nil receiver, parser, or source)")
	}

	// Built-ins sentinel check. Resolving Origin-Host (the canonical
	// base-AVP, application id 0) confirms RFC 6733 was loaded into
	// the parser by go-diameter's package init(). RFC 4006 and
	// 3GPP TS 32.299 are loaded into application-specific scopes
	// (app id 4) and are therefore not findable via a base-app
	// lookup; rather than poke into specific app indices here, we
	// rely on the same init() that loads RFC 6733 also loading the
	// other two — if go-diameter ever stopped bundling them, the
	// downstream connection / messaging tests would fail loudly.
	//
	// AC-7 of feature #17 calls for the dictionaries to be loaded
	// "at startup". The library loads them in init() and the
	// sentinel here proves the init() ran for the parser this loader
	// is bound to.
	if _, err := l.Parser.FindAVP(uint32(0), AVPCodeOriginHost); err != nil {
		return LoadResult{}, fmt.Errorf("%w: Origin-Host (264) lookup failed: %v", ErrBuiltInsMissing, err)
	}

	records, err := l.Source.ListActiveCustomDictionaries(ctx)
	if err != nil {
		return LoadResult{}, fmt.Errorf("dictionary: list custom dictionaries: %w", err)
	}

	result := LoadResult{}
	for _, rec := range records {
		if !rec.IsActive {
			// Inactive rows are out of the active set; the loader
			// is permitted to ignore them entirely. Sources that
			// pre-filter on is_active will never produce these,
			// but the contract of Source allows them.
			continue
		}
		if err := l.Parser.Load(bytes.NewReader([]byte(rec.XMLContent))); err != nil {
			// Fail-soft: log + skip. The Feature explicitly forbids
			// fail-fast here (AC-9). The next iteration applies the
			// next custom row to the same parser unaffected.
			logging.Warn(
				"dictionary: skipping invalid custom dictionary",
				"name", rec.Name,
				"error", err.Error(),
			)
			result.SkippedNames = append(result.SkippedNames, rec.Name)
			continue
		}
		result.LoadedNames = append(result.LoadedNames, rec.Name)
	}

	logging.Info(
		"dictionary: loaded",
		"customs_loaded", len(result.LoadedNames),
		"customs_skipped", len(result.SkippedNames),
	)
	return result, nil
}
