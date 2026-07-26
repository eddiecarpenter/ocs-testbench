package mcp_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// scenarioFixture holds pre-created store entities for scenario tests.
type scenarioFixture struct {
	s      store.Store
	peerID pgtype.UUID
	subID  pgtype.UUID
	scID   string // UUID string of the first scenario
}

// newScenarioFixture inserts a peer, subscriber, and one scenario.
func newScenarioFixture(t *testing.T) scenarioFixture {
	t.Helper()
	s := store.NewTestStore()
	ctx := context.Background()

	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1"}`))
	require.NoError(t, err)

	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)

	sc, err := s.InsertScenario(ctx, "test-scenario", peer.ID, sub.ID,
		[]byte(`{"sessionMode":"session","serviceModel":"gy","steps":[]}`))
	require.NoError(t, err)

	return scenarioFixture{
		s:      s,
		peerID: peer.ID,
		subID:  sub.ID,
		scID:   uuidToStr(sc.ID.Bytes),
	}
}

// uuidFromStr parses a UUID string into a pgtype.UUID.
func uuidFromStr(s string) pgtype.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic("uuidFromStr: " + err.Error())
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// TestHandleListScenarios_ReturnsAll verifies list_scenarios returns all scenarios.
func TestHandleListScenarios_ReturnsAll(t *testing.T) {
	f := newScenarioFixture(t)
	ctx := context.Background()
	// Insert a second scenario.
	_, err := f.s.InsertScenario(ctx, "second-scenario", f.peerID, f.subID, []byte(`{}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("list_scenarios", nil)

	var envelope map[string]any
	toolResult(t, resp, &envelope)
	scenarios, _ := envelope["scenarios"].([]any)
	assert.Len(t, scenarios, 2, "list_scenarios must return 2 scenarios")
}

// TestHandleScenario_Get_Existing_ReturnsIt verifies scenario op=get with valid ID.
func TestHandleScenario_Get_Existing_ReturnsIt(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{"op": "get", "id": f.scID})

	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "test-scenario", result["name"])
	assert.Equal(t, "session", result["sessionMode"])
}

// TestHandleScenario_Get_Missing_ReturnsToolError verifies scenario op=get with unknown ID.
func TestHandleScenario_Get_Missing_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("scenario", map[string]any{
		"op": "get",
		"id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "missing scenario must return isError: true")
}

// TestHandleScenario_Create_ValidInput_ReturnsScenario verifies scenario op=create.
func TestHandleScenario_Create_ValidInput_ReturnsScenario(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{
		"op":            "create",
		"name":          "new-scenario",
		"peer_id":       uuidToStr(f.peerID.Bytes),
		"subscriber_id": uuidToStr(f.subID.Bytes),
		"body":          map[string]any{"sessionMode": "session", "serviceModel": "gy"},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "new-scenario", result["name"])
	assert.NotEmpty(t, result["id"])
}

// TestHandleScenario_Create_EmptyGroupedAvp_Succeeds verifies that a grouped
// AVP with an empty children array is accepted — it must not be rejected as a
// leaf missing its valueRef.
func TestHandleScenario_Create_EmptyGroupedAvp_Succeeds(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{
		"op":            "create",
		"name":          "empty-group-scenario",
		"peer_id":       uuidToStr(f.peerID.Bytes),
		"subscriber_id": uuidToStr(f.subID.Bytes),
		"body": map[string]any{
			"sessionMode":  "event",
			"serviceModel": "root",
			"avpTree": []any{
				map[string]any{"name": "Service-Information", "code": 873, "children": []any{}},
				map[string]any{"name": "USSD-Detail", "code": 20654, "valueRef": "USSD_DETAIL"},
			},
		},
	})
	assert.False(t, isToolError(resp), "empty grouped AVP should be accepted")
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "empty-group-scenario", result["name"])
}

// TestHandleScenario_Create_LeafWithoutValueRef_ReturnsToolError verifies that a
// true leaf (no children key) still requires a valueRef.
func TestHandleScenario_Create_LeafWithoutValueRef_ReturnsToolError(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{
		"op":            "create",
		"name":          "bad-leaf-scenario",
		"peer_id":       uuidToStr(f.peerID.Bytes),
		"subscriber_id": uuidToStr(f.subID.Bytes),
		"body": map[string]any{
			"sessionMode":  "event",
			"serviceModel": "root",
			"avpTree": []any{
				map[string]any{"name": "USSD-Detail", "code": 20654},
			},
		},
	})
	assert.True(t, isToolError(resp), "leaf without valueRef must return isError: true")
}

// TestHandleScenario_Create_MissingName_ReturnsToolError verifies missing required param.
func TestHandleScenario_Create_MissingName_ReturnsToolError(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{
		"op":            "create",
		"peer_id":       uuidToStr(f.peerID.Bytes),
		"subscriber_id": uuidToStr(f.subID.Bytes),
	})
	assert.True(t, isToolError(resp), "missing name must return isError: true")
}

// TestHandleScenario_Delete_Existing_Deletes verifies scenario op=delete.
func TestHandleScenario_Delete_Existing_Deletes(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{"op": "delete", "id": f.scID})

	var result map[string]string
	toolResult(t, resp, &result)
	assert.Equal(t, "deleted", result["status"])
}

// TestHandleScenario_Duplicate_ExistingSource_CreatesCopy verifies scenario op=duplicate.
func TestHandleScenario_Duplicate_ExistingSource_CreatesCopy(t *testing.T) {
	f := newScenarioFixture(t)

	client := newMCPTestClient(t, f.s)
	resp := client.callTool("scenario", map[string]any{
		"op":        "duplicate",
		"source_id": f.scID,
		"new_name":  "copied-scenario",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "copied-scenario", result["name"])
	assert.NotEqual(t, f.scID, result["id"], "copy must have a different ID")
}

// TestHandleScenario_Duplicate_MissingSource_ReturnsToolError verifies missing source.
func TestHandleScenario_Duplicate_MissingSource_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("scenario", map[string]any{
		"op":        "duplicate",
		"source_id": "00000000-0000-0000-0000-000000000000",
		"new_name":  "copy",
	})
	assert.True(t, isToolError(resp), "missing source must return isError: true")
}
