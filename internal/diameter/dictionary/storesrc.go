package dictionary

import (
	"context"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// StoreLister is the slice of internal/store.Store the dictionary
// loader actually uses. Defined as its own interface here so the
// adapter does not pull in the full Store interface — and so tests
// at the integration level can pass a mock that satisfies just this
// method shape.
type StoreLister interface {
	ListCustomDictionaries(ctx context.Context) ([]store.CustomDictionary, error)
}

// StoreSource adapts the production internal/store.Store to the
// loader's Source interface.
//
// The adapter projects the sqlc-generated row (which uses pgtype
// wrappers and includes columns the loader does not need — id,
// description, timestamps) onto the flat Record shape. The
// projection is intentionally narrow: the loader only needs name,
// XML, and the active flag.
//
// Use NewStoreSource to construct. The orchestrator wires
//
//	loader := dictionary.NewLoader(dictionary.NewStoreSource(s))
//
// at startup; everything else flows from there.
type StoreSource struct {
	store StoreLister
}

// NewStoreSource constructs a StoreSource over the given lister. A
// nil lister panics — wiring a loader against a nil store is a
// programming error and should fail loudly at startup, not silently
// when Load is called.
func NewStoreSource(s StoreLister) *StoreSource {
	if s == nil {
		panic("dictionary.NewStoreSource: store must not be nil")
	}
	return &StoreSource{store: s}
}

// ListActiveCustomDictionaries reads every custom_dictionary row
// from the underlying store and returns the active ones projected
// onto the loader's Record shape.
//
// Filtering on IsActive is performed here rather than at the SQL
// layer because internal/store.Store.ListCustomDictionaries returns
// every row in name order; adding a separate "list active only"
// query would duplicate the surface for one optional WHERE clause.
// The loader itself also tolerates inactive rows in the result, so
// the filter is defence-in-depth.
//
// Error contract: any store error is propagated as-is so the caller
// can compare against store.ErrNotFound or wrap it with whatever
// context they have.
func (s *StoreSource) ListActiveCustomDictionaries(ctx context.Context) ([]Record, error) {
	rows, err := s.store.ListCustomDictionaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		if !row.IsActive {
			continue
		}
		out = append(out, Record{
			Name:       row.Name,
			XMLContent: row.XmlContent,
			IsActive:   row.IsActive,
		})
	}
	return out, nil
}
