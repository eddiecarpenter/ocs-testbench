// This file exports internal symbols for use in external test packages.
// It is compiled only during test runs (the _test.go suffix triggers
// Go's test build constraint).

package api

import (
	"encoding/json"
	"net/http"
)

// DecodeJSONTestHandler returns an http.Handler that calls the internal
// decodeJSON helper and returns 200 OK when parsing succeeds, or the
// 400 error response that decodeJSON writes on failure.
//
// Tests use this to verify AC-3 (invalid JSON body → 400 structured
// error) against the foundation helper, independent of the entity-
// specific handlers that are implemented in later tasks.
func DecodeJSONTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if !decodeJSON(w, r, &body) {
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
}
