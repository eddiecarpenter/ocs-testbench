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

// seedSubscriber creates a subscriber via POST /subscribers. Fails if
// not 201.
func seedSubscriber(t *testing.T, r http.Handler, msisdn, iccid string) map[string]any {
	t.Helper()
	reqBody, err := json.Marshal(map[string]any{
		"name":   "sub-" + msisdn,
		"msisdn": msisdn,
		"iccid":  iccid,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/subscribers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code,
		"seedSubscriber: expected 201, got %d: %s", rr.Code, rr.Body.String())
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp
}

// TestSubscriber_CreateSubscriber_AllRequiredFields_Returns201 tests
// AC-4 (create with required fields returns 201).
func TestSubscriber_CreateSubscriber_AllRequiredFields_Returns201(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{
		"name": "alice", "msisdn": "27821234567", "iccid": "8927010000123456789",
	})
	req := httptest.NewRequest(http.MethodPost, "/subscribers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "27821234567", resp["msisdn"])
}

// TestSubscriber_CreateSubscriber_MissingMsisdn_Returns400 tests
// AC-5 (missing required field returns 400).
func TestSubscriber_CreateSubscriber_MissingMsisdn_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"iccid": "8927010000123456789"})
	req := httptest.NewRequest(http.MethodPost, "/subscribers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestSubscriber_CreateSubscriber_MissingIccid_Returns400 tests
// AC-5 (missing iccid returns 400).
func TestSubscriber_CreateSubscriber_MissingIccid_Returns400(t *testing.T) {
	r := newTestRouter(t)
	reqBody, _ := json.Marshal(map[string]any{"msisdn": "27821234567"})
	req := httptest.NewRequest(http.MethodPost, "/subscribers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assertErrorCode(t, rr.Body, "INVALID_REQUEST")
}

// TestSubscriber_ListSubscribers_Returns200 tests AC-8.
func TestSubscriber_ListSubscribers_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	seedSubscriber(t, r, "27821111111", "8927010000000000001")
	seedSubscriber(t, r, "27822222222", "8927010000000000002")

	req := httptest.NewRequest(http.MethodGet, "/subscribers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var items []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// TestSubscriber_GetSubscriber_Existing_Returns200 tests AC-6.
func TestSubscriber_GetSubscriber_Existing_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedSubscriber(t, r, "27820000001", "8927010000000000099")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/subscribers/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, id, resp["id"])
}

// TestSubscriber_GetSubscriber_NonExistent_Returns404 tests AC-7.
func TestSubscriber_GetSubscriber_NonExistent_Returns404(t *testing.T) {
	r := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/subscribers/00000000-0000-0000-0000-000000000099", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assertErrorCode(t, rr.Body, "NOT_FOUND")
}

// TestSubscriber_UpdateSubscriber_ValidBody_Returns200 tests AC-9.
func TestSubscriber_UpdateSubscriber_ValidBody_Returns200(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedSubscriber(t, r, "27820000002", "8927010000000000088")
	id := created["id"].(string)

	reqBody, _ := json.Marshal(map[string]any{
		"name": "updated", "msisdn": "27820000099", "iccid": "8927010000000000088",
	})
	req := httptest.NewRequest(http.MethodPut, "/subscribers/"+id, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "27820000099", resp["msisdn"])
}

// TestSubscriber_DeleteSubscriber_Existing_Returns204 tests AC-10.
func TestSubscriber_DeleteSubscriber_Existing_Returns204(t *testing.T) {
	s := store.NewTestStore()
	r := newTestRouterWithStore(t, s)

	created := seedSubscriber(t, r, "27820000003", "8927010000000000077")
	id := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/subscribers/"+id, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestSubscriber_OptionalFields_StoredCorrectly verifies that optional
// fields (imei, deviceMake, deviceModel) are stored and returned.
func TestSubscriber_OptionalFields_StoredCorrectly(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()

	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name:        "device-user",
		Msisdn:      "27820000004",
		Iccid:       "8927010000000000066",
		Imei:        pgTextFrom("123456789012345"),
		DeviceMake:  pgTextFrom("Apple"),
		DeviceModel: pgTextFrom("iPhone 14"),
	})
	require.NoError(t, err)

	r := newTestRouterWithStore(t, s)
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/subscribers/%s", api.UUIDStr(sub.ID)), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "123456789012345", resp["imei"])
	assert.Equal(t, "Apple", resp["deviceMake"])
	assert.Equal(t, "iPhone 14", resp["deviceModel"])
}
