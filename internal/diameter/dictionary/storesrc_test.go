package dictionary

import (
	"context"
	"errors"
	"testing"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakeStoreLister implements StoreLister with a static list and an
// optional error. Defined separately from fakeSource so the
// store-projection layer can be exercised without going through the
// loader.
type fakeStoreLister struct {
	rows []store.CustomDictionary
	err  error
}

func (f fakeStoreLister) ListCustomDictionaries(_ context.Context) ([]store.CustomDictionary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

// TestStoreSource_FiltersInactive confirms the adapter projects
// only active rows onto the loader Record shape.
func TestStoreSource_FiltersInactive(t *testing.T) {
	t.Parallel()

	rows := []store.CustomDictionary{
		{Name: "active-1", XmlContent: "<x/>", IsActive: true},
		{Name: "inactive-1", XmlContent: "<y/>", IsActive: false},
		{Name: "active-2", XmlContent: "<z/>", IsActive: true},
	}
	src := NewStoreSource(fakeStoreLister{rows: rows})
	got, err := src.ListActiveCustomDictionaries(context.Background())
	if err != nil {
		t.Fatalf("ListActiveCustomDictionaries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records; want 2", len(got))
	}
	if got[0].Name != "active-1" || got[1].Name != "active-2" {
		t.Errorf("unexpected names: %v", []string{got[0].Name, got[1].Name})
	}
	for _, r := range got {
		if !r.IsActive {
			t.Errorf("inactive row leaked into active set: %s", r.Name)
		}
	}
}

// TestStoreSource_PropagatesError ensures store errors are surfaced
// untouched to the loader, which is the contract relied on for
// AC-9's source-error branch.
func TestStoreSource_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("simulated store failure")
	src := NewStoreSource(fakeStoreLister{err: sentinel})
	_, err := src.ListActiveCustomDictionaries(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("error chain does not include sentinel: %v", err)
	}
}

// TestNewStoreSource_NilStorePanics — wiring a StoreSource over a
// nil store is a programming error and must fail loudly at startup.
func TestNewStoreSource_NilStorePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil store")
		}
	}()
	_ = NewStoreSource(nil)
}
