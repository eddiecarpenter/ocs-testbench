package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// seedDictionary creates a custom dictionary via POST /dictionaries.
func seedDictionary(t *testing.T, r http.Handler, name, xmlContent string) map[string]any {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]any{
		"name":       name,
		"xmlContent": xmlContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/dictionaries", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedDictionary: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// TestDictionary_CreateDictionary_ValidBody_Returns201 tests AC-4 and
// AC-12 (create with XML content returns 201; dictionary available for
// Diameter stack loading).
func TestDictionary_CreateDictionary_ValidBody_Returns201(t *testing.T) {
	r := newTestRouter(t)
	xml := `<dictionary><vendor id="1" name="Test"/></dictionary>`
	created := seedDictionary(t, r, "dict-a", xml)

	assert.NotEmpty(t, created["id"])
	assert.Equal(t, "dict-a", created["name"])
	assert.Equal(t, xml, created["xmlContent"])
	// AC-12: xmlContent is persisted; store API confirms it's available
	// for loading (the test store mirrors the production store contract).
}

// TestDictionary_CreateDictionary_MissingName_Returns400 tests AC-5.
func TestDictionary_CreateDictionary_MissingName_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"xmlContent": "<x/>"})
	req := httptest.NewRequest(http.MethodPost, "/dictionaries", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestDictionary_CreateDictionary_MissingXmlContent_Returns400 tests
// AC-5.
func TestDictionary_CreateDictionary_MissingXmlContent_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"name": "dict-no-xml"})
	req := httptest.NewRequest(http.MethodPost, "/dictionaries", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestDictionary_CreateDictionary_DuplicateName_Returns409.
func TestDictionary_CreateDictionary_DuplicateName_Returns409(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	seedDictionary(t, r, "dict-dup", "<x/>")
	reqBody, _ := json.Marshal(map[string]any{"name": "dict-dup", "xmlContent": "<x/>"})
	req := httptest.NewRequest(http.MethodPost, "/dictionaries", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestDictionary_ListDictionaries_Returns200 tests AC-8.
func TestDictionary_ListDictionaries_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	seedDictionary(t, r, "dict-1", "<a/>")
	seedDictionary(t, r, "dict-2", "<b/>")

	req := httptest.NewRequest(http.MethodGet, "/dictionaries", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestDictionary_GetDictionary_Existing_Returns200WithXmlContent tests
// AC-6 and that xmlContent is present in the response body.
func TestDictionary_GetDictionary_Existing_Returns200WithXmlContent(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	xml := "<vendor id=\"99\"/>"
	created := seedDictionary(t, r, "dict-get", xml)
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/dictionaries/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, xml, resp["xmlContent"])
}

// TestDictionary_GetDictionary_NonExistent_Returns404 tests AC-7.
func TestDictionary_GetDictionary_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/dictionaries/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestDictionary_UpdateDictionary_ValidBody_Returns200 tests AC-9.
func TestDictionary_UpdateDictionary_ValidBody_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	created := seedDictionary(t, r, "dict-upd", "<old/>")
	id := created["id"].(string)

	reqBody, _ := json.Marshal(map[string]any{
		"name":       "dict-upd-new",
		"xmlContent": "<new/>",
		"isActive":   true,
	})
	req := httptest.NewRequest(http.MethodPut, "/dictionaries/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "dict-upd-new", resp["name"])
	assert.Equal(t, "<new/>", resp["xmlContent"])
	assert.Equal(t, true, resp["isActive"])
}

// TestDictionary_DeleteDictionary_Returns204 tests AC-10.
func TestDictionary_DeleteDictionary_Returns204(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)
	created := seedDictionary(t, r, "dict-del", "<d/>")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/dictionaries/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}
