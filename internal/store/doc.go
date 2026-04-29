// Package store is the persistence wrapper for the OCS Testbench.
//
// It exposes a Store interface that every later application layer
// (engine, API, executions) couples against. Two constructors are
// provided: NewStore wraps the sqlc-generated bindings over a
// pgxpool.Pool for production use, and NewTestStore returns an
// in-memory implementation for unit tests.
//
// The implementation is split across:
//
//   - this package — the Store interface, public types, and
//     constructors;
//   - internal/store/sqlc — the sqlc-generated CRUD bindings;
//   - internal/store/queries — the .sql input files that sqlc
//     compiles into Go.
//
// At Task 1 of the data-model Feature only the package shell exists;
// the interface and constructors are introduced in Task 3.
package store
