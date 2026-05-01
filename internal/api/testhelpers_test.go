// Test helpers shared across all api_test files.
// This file is compiled only during test builds.

package api_test

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// pgTextFrom converts a string to a non-null pgtype.Text. Used in
// tests that populate optional text fields directly on the store.
func pgTextFrom(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}
