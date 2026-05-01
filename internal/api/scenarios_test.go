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

// scenarioFixture holds a pre-created peer, template, and router for
// scenario-related tests.
type scenarioFixture struct {
	s      store.Store
	r      http.Handler
	peerID string
	tplID  string
}

// newScenarioFixture creates a store with a peer and template pre-seeded
// for scenario tests.
func newScenarioFixture(t *testing.T) *scenarioFixture {
	t.Helper()
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "peer-scen", []byte(`{}`))
	require.NoError(t, err)
	tpl, err := s.InsertAVPTemplate(ctx, "tpl-scen", []byte(`{}`))
	require.NoError(t, err)
	return &scenarioFixture{
		s:      s,
		r:      api.Router(s),
		peerID: api.UUIDStr(peer.ID),
		tplID:  api.UUIDStr(tpl.ID),
	}
}

// seedScenario creates a scenario via POST /scenarios.
func (f *scenarioFixture) seedScenario(t *testing.T, name string) map[string]any {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]any{
		"name":       name,
		"templateId": f.tplID,
		"peerId":     f.peerID,
		"body":       json.RawMessage(`{"steps":[]}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedScenario: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// TestScenario_CreateScenario_ValidBody_Returns201 tests AC-4.
func TestScenario_CreateScenario_ValidBody_Returns201(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-a")
	assert.NotEmpty(t, created["id"])
	assert.Equal(t, "scen-a", created["name"])
	assert.Equal(t, f.tplID, created["templateId"])
	assert.Equal(t, f.peerID, created["peerId"])
}

// TestScenario_CreateScenario_MissingName_Returns400 tests AC-5.
func TestScenario_CreateScenario_MissingName_Returns400(t *testing.T) {
	f := newScenarioFixture(t)
	reqBody, _ := json.Marshal(map[string]any{
		"templateId": f.tplID,
		"peerId":     f.peerID,
	})
	req := httptest.NewRequest(http.MethodPost, "/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestScenario_CreateScenario_MissingTemplateID_Returns400 tests AC-5.
func TestScenario_CreateScenario_MissingTemplateID_Returns400(t *testing.T) {
	f := newScenarioFixture(t)
	reqBody, _ := json.Marshal(map[string]any{
		"name":   "scen-no-tpl",
		"peerId": f.peerID,
	})
	req := httptest.NewRequest(http.MethodPost, "/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestScenario_CreateScenario_NonExistentTemplateID_Returns409 tests
// AC-11 (non-existent FK returns 409).
func TestScenario_CreateScenario_NonExistentTemplateID_Returns409(t *testing.T) {
	f := newScenarioFixture(t)
	reqBody, _ := json.Marshal(map[string]any{
		"name":       "scen-bad-tpl",
		"templateId": "00000000-0000-0000-0000-000000000099",
		"peerId":     f.peerID,
		"body":       json.RawMessage(`{}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
	assertErrorCode(t, rr.Body, "CONFLICT")
}

// TestScenario_CreateScenario_DuplicateName_Returns409.
func TestScenario_CreateScenario_DuplicateName_Returns409(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-dup")

	reqBody, _ := json.Marshal(map[string]any{
		"name":       "scen-dup",
		"templateId": f.tplID,
		"peerId":     f.peerID,
		"body":       json.RawMessage(`{}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestScenario_ListScenarios_Returns200 tests AC-8.
func TestScenario_ListScenarios_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-list-1")
	f.seedScenario(t, "scen-list-2")

	req := httptest.NewRequest(http.MethodGet, "/scenarios", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestScenario_GetScenario_Existing_Returns200 tests AC-6.
func TestScenario_GetScenario_Existing_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-get")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/scenarios/"+id, nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestScenario_GetScenario_NonExistent_Returns404 tests AC-7.
func TestScenario_GetScenario_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/scenarios/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestScenario_UpdateScenario_ValidBody_Returns200 tests AC-9.
func TestScenario_UpdateScenario_ValidBody_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-upd")
	id := created["id"].(string)

	reqBody, _ := json.Marshal(map[string]any{
		"name":       "scen-upd-new",
		"templateId": f.tplID,
		"peerId":     f.peerID,
		"body":       json.RawMessage(`{"steps":[1]}`),
	})
	req := httptest.NewRequest(http.MethodPut, "/scenarios/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "scen-upd-new", resp["name"])
}

// TestScenario_DeleteScenario_Returns204 tests AC-10.
func TestScenario_DeleteScenario_Returns204(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-del")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/scenarios/"+id, nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestScenario_DeletePeer_WithScenarioFK_Returns409 verifies that
// deleting a peer referenced by a scenario returns 409 (AC-11 -
// FK constraint tested from peer perspective).
func TestScenario_DeletePeer_WithScenarioFK_Returns409(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-fk2")

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/peers/%s", f.peerID), nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}
