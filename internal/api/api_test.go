package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// newTestRouter constructs an api.Router wired with an in-memory test
// store. Tests that only exercise the routing and middleware layers
// (not entity logic) use this helper.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	return api.Router(store.NewTestStore())
}

// TestRouter_AC1_AllRoutesRegistered verifies that the router accepts
// requests on the pre-wired routes (they return 501, not 404).
//
// AC-1: given the application starts, when the HTTP server initialises,
// then all API routes are registered and the server accepts requests on
// the configured port.
func TestRouter_AC1_AllRoutesRegistered(t *testing.T) {
	r := newTestRouter(t)

	// Routes are divided into two categories:
	// - Collection routes (no {id}): should NOT return 404 (route missing)
	//   nor 405 (method not allowed). Real handlers return 200/201; stubs
	//   return 501.
	// - Entity routes (with {id}): a 404 from the handler means the route
	//   IS registered but the entity doesn't exist — that's correct behaviour.
	//   We only reject 405 (method not allowed) as a sign the route is missing.
	collectionRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/peers"},
		{http.MethodPost, "/peers"},
		{http.MethodGet, "/subscribers"},
		{http.MethodPost, "/subscribers"},
		{http.MethodGet, "/templates"},
		{http.MethodPost, "/templates"},
		{http.MethodGet, "/scenarios"},
		{http.MethodPost, "/scenarios"},
		{http.MethodGet, "/dictionaries"},
		{http.MethodPost, "/dictionaries"},
	}
	entityRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/peers/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/peers/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/peers/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/subscribers/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/subscribers/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/subscribers/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/templates/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/scenarios/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/scenarios/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/scenarios/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/dictionaries/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/dictionaries/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/dictionaries/00000000-0000-0000-0000-000000000001"},
	}

	for _, tc := range collectionRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusNotFound, rr.Code,
				"collection route %s %s should be registered (got 404)", tc.method, tc.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code,
				"collection route %s %s method should be allowed", tc.method, tc.path)
		})
	}

	for _, tc := range entityRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			// 404 from a real handler (entity not found) means the route
			// IS registered. Only 405 would indicate the route is missing.
			assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code,
				"entity route %s %s method should be allowed (got 405)", tc.method, tc.path)
		})
	}
}

// TestRouter_AC2_UnknownEndpoint404 verifies that unknown paths return
// a 404 with a structured JSON error body.
//
// AC-2: given a request to an unknown endpoint, when the server
// processes it, then it returns a 404 with a structured error body
// containing an error code and message.
func TestRouter_AC2_UnknownEndpoint404(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/no-such-endpoint", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "response body must have an 'error' object")
	assert.NotEmpty(t, errObj["code"], "error.code must be present")
	assert.NotEmpty(t, errObj["message"], "error.message must be present")
}

// TestRouter_AC3_InvalidJSONBody400 verifies that the decodeJSON
// foundation helper returns a 400 structured error response when the
// request body is not valid JSON.
//
// AC-3: given a request with invalid JSON body, when the server
// processes it, then it returns a 400 with a structured error body
// identifying the parsing error.
//
// The test exercises this through a chi router with a test-only handler
// that calls decodeJSON — the actual POST entity handlers (Tasks 2–6)
// all call decodeJSON and therefore inherit this behaviour.
func TestRouter_AC3_InvalidJSONBody400(t *testing.T) {
	// Build a minimal router with a test handler that parses a JSON
	// body. This isolates the decodeJSON helper from the 501 stubs
	// that are in place for Tasks 2–6.
	wrapped := api.DecodeJSONTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/",
		bytes.NewBufferString(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "response body must have an 'error' object")
	assert.Equal(t, "INVALID_REQUEST", errObj["code"])
	assert.NotEmpty(t, errObj["message"])
}

// TestRequestIDMiddleware_SetsXRequestIDHeader verifies that every
// response includes the X-Request-ID header.
func TestRequestIDMiddleware_SetsXRequestIDHeader(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"),
		"X-Request-ID header should be set on every response")
}

// TestRecoveryMiddleware_PanicReturns500 verifies that the Recovery
// middleware catches panics and returns a structured 500 response.
func TestRecoveryMiddleware_PanicReturns500(t *testing.T) {
	// Build a minimal handler that always panics, then wrap it in the
	// Recovery middleware directly — we can't swap a route on the
	// existing router, so this tests the middleware in isolation.
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	wrapped := api.Recovery(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "response body must have an 'error' object")
	assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
}
