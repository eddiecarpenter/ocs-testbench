package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// newTestRouterWithStore constructs an api.Router wired with the
// given store and no PeerManager, for tests that need to
// pre-populate the store before issuing requests.
func newTestRouterWithStore(t *testing.T, s store.Store) http.Handler {
	t.Helper()
	return api.Router(s, nil)
}

// seedPeer creates a peer via POST /peers. Fails the test if not 201.
func seedPeer(t *testing.T, r http.Handler, name string, body json.RawMessage) map[string]any {
	t.Helper()
	reqBody, err := json.Marshal(map[string]any{"name": name, "body": body})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedPeer: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// assertErrorCode verifies the JSON response body contains the expected
// error code in the {"error": {"code": ..., "message": ...}} envelope.
func assertErrorCode(t *testing.T, body *bytes.Buffer, expectedCode string) {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&resp))
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok, "response must have an 'error' object, got: %v", resp)
	assert.Equal(t, expectedCode, errObj["code"])
}

// TestPeer_CreatePeer_ValidBody_Returns201 tests AC-4 (create with
// required fields returns 201 with the created entity and generated ID).
func TestPeer_CreatePeer_ValidBody_Returns201(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	body := json.RawMessage(`{"host":"10.0.1.1","port":3868}`)
	reqBody, err := json.Marshal(map[string]any{"name": "peer-a", "body": body})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["id"], "id must be generated")
	assert.Equal(t, "peer-a", resp["name"])
}

// TestPeer_CreatePeer_MissingName_Returns400 tests AC-5 (create with
// missing required field returns 400 with structured error).
func TestPeer_CreatePeer_MissingName_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, err := json.Marshal(map[string]any{"body": json.RawMessage(`{}`)})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestPeer_CreatePeer_DuplicateName_Returns409 tests that duplicate
// name returns 409 Conflict.
func TestPeer_CreatePeer_DuplicateName_Returns409(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	seedPeer(t, r, "peer-dup", json.RawMessage(`{}`))

	reqBody, _ := json.Marshal(map[string]any{"name": "peer-dup", "body": json.RawMessage(`{}`)})
	req := httptest.NewRequest(http.MethodPost, "/peers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assertErrorCode(t, rr.Body, "CONFLICT")
}

// TestPeer_ListPeers_Returns200WithArray tests AC-8 (list returns 200
// with an array of all entities of that type).
func TestPeer_ListPeers_Returns200WithArray(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	seedPeer(t, r, "peer-x", json.RawMessage(`{}`))
	seedPeer(t, r, "peer-y", json.RawMessage(`{}`))

	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestPeer_GetPeer_Existing_Returns200 tests AC-6 (get by existing ID
// returns 200 with the entity).
func TestPeer_GetPeer_Existing_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedPeer(t, r, "peer-get", json.RawMessage(`{"host":"x"}`))
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/peers/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, id, resp["id"])
	assert.Equal(t, "peer-get", resp["name"])
}

// TestPeer_GetPeer_NonExistent_Returns404 tests AC-7 (get by non-existent
// ID returns 404).
func TestPeer_GetPeer_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/peers/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assertErrorCode(t, rr.Body, "NOT_FOUND")
}

// TestPeer_GetPeer_MalformedUUID_Returns400 verifies that a non-UUID
// path parameter returns 400.
func TestPeer_GetPeer_MalformedUUID_Returns400(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/peers/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestPeer_UpdatePeer_ValidBody_Returns200 tests AC-9 (valid update
// returns 200 with the updated entity).
func TestPeer_UpdatePeer_ValidBody_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedPeer(t, r, "peer-upd", json.RawMessage(`{"host":"old"}`))
	id := created["id"].(string)

	reqBody, _ := json.Marshal(map[string]any{"name": "peer-upd-new", "body": json.RawMessage(`{"host":"new"}`)})
	req := httptest.NewRequest(http.MethodPut, "/peers/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "peer-upd-new", resp["name"])
}

// TestPeer_DeletePeer_Existing_Returns204 tests AC-10 (delete existing
// entity returns 204).
func TestPeer_DeletePeer_Existing_Returns204(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedPeer(t, r, "peer-del", json.RawMessage(`{}`))
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/peers/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestPeer_DeletePeer_FKReferenced_Returns409 tests that deleting a
// peer referenced by a scenario returns 409 Conflict.
func TestPeer_DeletePeer_FKReferenced_Returns409(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()

	peer, err := s.InsertPeer(ctx, "peer-fk", []byte(`{}`))
	require.NoError(t, err)
	tpl, err := s.InsertAVPTemplate(ctx, "tpl-fk", []byte(`{}`))
	require.NoError(t, err)
	_, err = s.InsertScenario(ctx, "scen-fk", tpl.ID, peer.ID, []byte(`{}`))
	require.NoError(t, err)

	r := newTestRouterWithStore(t, s)
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/peers/%s", api.UUIDStr(peer.ID)), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assertErrorCode(t, rr.Body, "CONFLICT")
}
