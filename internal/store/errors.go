// Package store — typed error values returned by every Store
// implementation.
//
// The Store interface promises that callers can distinguish three
// failure shapes by errors.Is comparison alone:
//
//   - ErrNotFound        — the entity does not exist
//   - ErrDuplicateName   — a uniqueness constraint was violated
//   - ErrForeignKey      — a referenced row (template_id / peer_id)
//                          does not exist
//
// The production store maps pgconn error codes onto these sentinels;
// the in-memory test store enforces the same invariants and returns
// the same sentinels so unit tests can assert on them without caring
// which implementation is in play.

package store

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a Get / Update / Delete targets a row
// that does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrDuplicateName is returned when an Insert or Update would
// violate a unique-name constraint. The schema declares unique
// constraints on peer.name, avp_template.name, scenario.name, and
// custom_dictionary.name. Subscriber names are NOT unique by design
// and never trigger this sentinel.
var ErrDuplicateName = errors.New("store: duplicate name")

// ErrForeignKey is returned when an Insert or Update on the
// scenario table references a template_id or peer_id that does not
// exist (or when a Delete on peer / avp_template would leave a
// scenario dangling — schema-level ON DELETE RESTRICT triggers the
// same sentinel via the production wrapper).
var ErrForeignKey = errors.New("store: foreign key violation")

// EntityError wraps one of the sentinels above with the entity name
// and the offending key. It is opaque to errors.Is comparison
// against the sentinel — callers can still write
// `errors.Is(err, store.ErrNotFound)` — but the formatted message
// includes enough context to be useful in logs and tests.
type EntityError struct {
	// Sentinel is one of ErrNotFound / ErrDuplicateName /
	// ErrForeignKey. Never nil for a non-nil EntityError.
	Sentinel error
	// Entity is the lower-case table name, e.g. "peer", "scenario".
	Entity string
	// Key is the offending lookup key (id string, name, or
	// referenced foreign-key value). Empty if not applicable.
	Key string
}

// Error renders the wrapped sentinel together with the entity and
// offending key for log readability.
func (e *EntityError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("%s: %s", e.Entity, e.Sentinel.Error())
	}
	return fmt.Sprintf("%s %q: %s", e.Entity, e.Key, e.Sentinel.Error())
}

// Unwrap returns the wrapped sentinel so errors.Is can match on
// ErrNotFound / ErrDuplicateName / ErrForeignKey.
func (e *EntityError) Unwrap() error { return e.Sentinel }

// notFound is a small constructor used by every Store implementation
// to surface a uniform not-found error.
func notFound(entity, key string) error {
	return &EntityError{Sentinel: ErrNotFound, Entity: entity, Key: key}
}

// duplicate is a small constructor used by every Store implementation
// to surface a uniform duplicate-name error.
func duplicate(entity, name string) error {
	return &EntityError{Sentinel: ErrDuplicateName, Entity: entity, Key: name}
}

// foreignKey is a small constructor used by every Store implementation
// to surface a uniform foreign-key violation. The key is the
// referenced id that does not resolve.
func foreignKey(entity, ref string) error {
	return &EntityError{Sentinel: ErrForeignKey, Entity: entity, Key: ref}
}
