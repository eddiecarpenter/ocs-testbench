package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// seedTemplate creates an AVP template via POST /templates.
func seedTemplate(t *testing.T, r http.Handler, name string) map[string]any {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]any{
		"name": name,
		"body": json.RawMessage(`{"avps":[]}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/templates", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedTemplate: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// TestTemplate_CreateTemplate_ValidBody_Returns201 tests AC-4.
func TestTemplate_CreateTemplate_ValidBody_Returns201(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"name": "tpl-a", "body": json.RawMessage(`{"avps":[]}`)})
	req := httptest.NewRequest(http.MethodPost, "/v1/templates", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "tpl-a", resp["name"])
}

// TestTemplate_CreateTemplate_MissingName_Returns400 tests AC-5.
func TestTemplate_CreateTemplate_MissingName_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"body": json.RawMessage(`{}`)})
	req := httptest.NewRequest(http.MethodPost, "/v1/templates", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestTemplate_CreateTemplate_DuplicateName_Returns409.
func TestTemplate_CreateTemplate_DuplicateName_Returns409(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	seedTemplate(t, r, "tpl-dup")

	reqBody, _ := json.Marshal(map[string]any{"name": "tpl-dup", "body": json.RawMessage(`{}`)})
	req := httptest.NewRequest(http.MethodPost, "/v1/templates", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestTemplate_ListTemplates_Returns200 tests AC-8.
func TestTemplate_ListTemplates_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	seedTemplate(t, r, "tpl-1")
	seedTemplate(t, r, "tpl-2")

	req := httptest.NewRequest(http.MethodGet, "/v1/templates", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestTemplate_GetTemplate_Existing_Returns200 tests AC-6.
func TestTemplate_GetTemplate_Existing_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	created := seedTemplate(t, r, "tpl-get")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/v1/templates/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, id, resp["id"])
}

// TestTemplate_GetTemplate_NonExistent_Returns404 tests AC-7.
func TestTemplate_GetTemplate_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/templates/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestTemplate_UpdateTemplate_ValidBody_Returns200 tests AC-9.
func TestTemplate_UpdateTemplate_ValidBody_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	created := seedTemplate(t, r, "tpl-upd")
	id := created["id"].(string)

	reqBody, _ := json.Marshal(map[string]any{"name": "tpl-upd-new", "body": json.RawMessage(`{"avps":[1,2]}`)})
	req := httptest.NewRequest(http.MethodPut, "/v1/templates/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "tpl-upd-new", resp["name"])
}

// TestTemplate_DeleteTemplate_NoFKRef_Returns204 tests AC-10.
func TestTemplate_DeleteTemplate_NoFKRef_Returns204(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	created := seedTemplate(t, r, "tpl-del")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/v1/templates/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestTemplate_DeleteTemplate_WithScenario_Returns204 verifies that deleting
// a template succeeds even when scenarios exist (template_id is no longer FK'd).
func TestTemplate_DeleteTemplate_WithScenario_Returns204(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()

	tpl, err := s.InsertAVPTemplate(ctx, "tpl-fk", []byte(`{}`))
	require.NoError(t, err)
	peer, err := s.InsertPeer(ctx, "peer-for-tpl", []byte(`{}`))
	require.NoError(t, err)
	_, err = s.InsertScenario(ctx, "scen-for-tpl", peer.ID, pgtype.UUID{}, []byte(`{}`))
	require.NoError(t, err)

	r := newTestRouterWithStore(t, s)
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/v1/templates/%s", api.UUIDStr(tpl.ID)), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}
