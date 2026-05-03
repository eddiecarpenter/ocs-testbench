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

// scenarioFixture holds a pre-created peer and router for scenario tests.
type scenarioFixture struct {
	s      store.Store
	r      http.Handler
	peerID string
}

// newScenarioFixture creates a store with a peer pre-seeded for scenario tests.
func newScenarioFixture(t *testing.T) *scenarioFixture {
	t.Helper()
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "peer-scen", []byte(`{}`))
	require.NoError(t, err)
	return &scenarioFixture{
		s:      s,
		r:      api.Router(s, nil),
		peerID: api.UUIDStr(peer.ID),
	}
}

// minimalScenarioBody returns a minimal valid ScenarioInput body for seeding.
func minimalScenarioBody(name, peerID string) map[string]any {
	return map[string]any{
		"name":         name,
		"peerId":       peerID,
		"unitType":     "OCTET",
		"sessionMode":  "continuous",
		"serviceModel": "single-mscc",
		"avpTree":      []any{},
		"services":     []any{},
		"variables":    []any{},
		"steps":        []any{},
	}
}

// seedScenario creates a scenario via POST /scenarios.
func (f *scenarioFixture) seedScenario(t *testing.T, name string) map[string]any {
	t.Helper()
	reqBody, _ := json.Marshal(minimalScenarioBody(name, f.peerID))
	req := httptest.NewRequest(http.MethodPost, "/v1/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedScenario: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// TestScenario_CreateScenario_ValidBody_Returns201.
func TestScenario_CreateScenario_ValidBody_Returns201(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-a")
	assert.NotEmpty(t, created["id"])
	assert.Equal(t, "scen-a", created["name"])
	assert.Equal(t, f.peerID, created["peerId"])
	assert.Equal(t, "OCTET", created["unitType"])
	assert.Equal(t, "user", created["origin"])
	assert.EqualValues(t, 0, created["stepCount"])
}

// TestScenario_CreateScenario_MissingName_Returns400.
func TestScenario_CreateScenario_MissingName_Returns400(t *testing.T) {
	f := newScenarioFixture(t)
	reqBody, _ := json.Marshal(map[string]any{
		"peerId":       f.peerID,
		"unitType":     "OCTET",
		"sessionMode":  "continuous",
		"serviceModel": "single-mscc",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestScenario_CreateScenario_DuplicateName_Returns409.
func TestScenario_CreateScenario_DuplicateName_Returns409(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-dup")

	reqBody, _ := json.Marshal(minimalScenarioBody("scen-dup", f.peerID))
	req := httptest.NewRequest(http.MethodPost, "/v1/scenarios", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestScenario_ListScenarios_Returns200.
func TestScenario_ListScenarios_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-list-1")
	f.seedScenario(t, "scen-list-2")

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestScenario_GetScenario_Existing_Returns200.
func TestScenario_GetScenario_Existing_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-get")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios/"+id, nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	// Full response includes body arrays.
	assert.Contains(t, resp, "avpTree")
	assert.Contains(t, resp, "services")
	assert.Contains(t, resp, "steps")
}

// TestScenario_GetScenario_NonExistent_Returns404.
func TestScenario_GetScenario_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestScenario_UpdateScenario_ValidBody_Returns200.
func TestScenario_UpdateScenario_ValidBody_Returns200(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-upd")
	id := created["id"].(string)

	updated := minimalScenarioBody("scen-upd-new", f.peerID)
	updated["steps"] = []any{"step1"}
	reqBody, _ := json.Marshal(updated)
	req := httptest.NewRequest(http.MethodPut, "/v1/scenarios/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "scen-upd-new", resp["name"])
	assert.EqualValues(t, 1, resp["stepCount"])
}

// TestScenario_DeleteScenario_Returns204.
func TestScenario_DeleteScenario_Returns204(t *testing.T) {
	f := newScenarioFixture(t)
	created := f.seedScenario(t, "scen-del")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/v1/scenarios/"+id, nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestScenario_DeletePeer_WithScenarioFK_Returns409 verifies that
// deleting a peer referenced by a scenario returns 409.
func TestScenario_DeletePeer_WithScenarioFK_Returns409(t *testing.T) {
	f := newScenarioFixture(t)
	f.seedScenario(t, "scen-fk2")

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/v1/peers/%s", f.peerID), nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}
