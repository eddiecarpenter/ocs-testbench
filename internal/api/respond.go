package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// jsonRawOrNil is an alias for []byte that encodes/decodes as raw JSON.
// Used for opaque JSONB body fields that pass through the API layer
// without interpretation.
type jsonRawOrNil = json.RawMessage

// respondJSON writes v as a JSON response with the given HTTP status
// code. Encoding errors are logged and a 500 is returned instead.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status header is already sent; we can only log the error.
		// Callers must not rely on the body when encoding fails.
		_ = fmt.Errorf("api: encode response: %w", err)
	}
}

// parseUUIDParam extracts a chi URL parameter by name, parses it as
// a UUID, and converts it to a pgtype.UUID. On parse failure it writes
// a 400 response and returns ok=false. The caller must check ok before
// using the returned UUID.
func parseUUIDParam(w http.ResponseWriter, r *http.Request, param string) (pgtype.UUID, bool) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil {
		respondInvalidRequest(w, fmt.Sprintf("invalid %s: must be a valid UUID", param))
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{Bytes: id, Valid: true}, true
}

// decodeJSON reads and decodes the request body into v. On failure it
// writes a 400 response and returns false. The caller must check the
// return value before using v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		respondInvalidRequest(w, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// uuidToString converts a pgtype.UUID to its canonical string
// representation. Returns the zero-value UUID string when not valid.
func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return "00000000-0000-0000-0000-000000000000"
	}
	return uuid.UUID(id.Bytes).String()
}

// mapStoreError maps a store error to the appropriate HTTP response.
// Returns the original error so callers can short-circuit with
// `if err := mapStoreError(w, err); err != nil { return }`.
// Returns nil when err is nil (no response written).
func mapStoreError(w http.ResponseWriter, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		respondNotFound(w)
	case errors.Is(err, store.ErrDuplicateName):
		respondConflict(w, err.Error())
	case errors.Is(err, store.ErrForeignKey):
		respondConflict(w, err.Error())
	default:
		respondInternalError(w)
	}
	return err
}
