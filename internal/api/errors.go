// Package api is the HTTP API layer for the OCS Testbench.
//
// The package is thin by design — every handler validates its input,
// delegates to a core library (store, diameter manager, execution
// engine), maps errors to HTTP status codes, and encodes the response.
// No business logic lives here.
package api

import (
	"encoding/json"
	"net/http"
)

// ErrorCode is a stable string identifier for an API error.
// Callers can branch on these codes without string parsing.
type ErrorCode string

// Standard error codes returned by all API handlers.
const (
	// CodeInvalidRequest is returned when the request body cannot be
	// parsed or contains invalid field values.
	CodeInvalidRequest ErrorCode = "INVALID_REQUEST"
	// CodeNotFound is returned when the requested resource does not
	// exist.
	CodeNotFound ErrorCode = "NOT_FOUND"
	// CodeConflict is returned when a uniqueness constraint or a
	// foreign-key constraint is violated.
	CodeConflict ErrorCode = "CONFLICT"
	// CodeInternalError is returned for unexpected server-side
	// failures.
	CodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// errorPayload is the JSON envelope for all error responses.
// Wire shape: {"error": {"code": "NOT_FOUND", "message": "peer not found"}}
type errorPayload struct {
	Err errorBody `json:"error"`
}

// errorBody carries the code and human-readable message.
type errorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// respondError writes a JSON error response with the given HTTP status,
// code, and message.
func respondError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorPayload{
		Err: errorBody{Code: code, Message: message},
	})
}

// respondNotFound writes a 404 error response.
func respondNotFound(w http.ResponseWriter) {
	respondError(w, http.StatusNotFound, CodeNotFound, "resource not found")
}

// respondNotFoundMsg writes a 404 error response with a custom message.
func respondNotFoundMsg(w http.ResponseWriter, msg string) {
	respondError(w, http.StatusNotFound, CodeNotFound, msg)
}

// respondConflict writes a 409 error response with a custom message.
func respondConflict(w http.ResponseWriter, msg string) {
	respondError(w, http.StatusConflict, CodeConflict, msg)
}

// respondInvalidRequest writes a 400 error response with a custom
// message describing which field or value was invalid.
func respondInvalidRequest(w http.ResponseWriter, msg string) {
	respondError(w, http.StatusBadRequest, CodeInvalidRequest, msg)
}

// respondInternalError writes a 500 error response. The message is
// intentionally generic — internal details must not leak to clients.
func respondInternalError(w http.ResponseWriter) {
	respondError(w, http.StatusInternalServerError, CodeInternalError, "internal server error")
}
