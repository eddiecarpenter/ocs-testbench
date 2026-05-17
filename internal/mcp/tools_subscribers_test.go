package mcp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// TestHandleListSubscribers_ReturnsSubscribers verifies list_subscribers
// returns all subscribers.
func TestHandleListSubscribers_ReturnsSubscribers(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)
	_, err = s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Bob", Msisdn: "27837654321", Iccid: "8927002222",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("list_subscribers", nil)

	var envelope map[string]any
	toolResult(t, resp, &envelope)
	subs, _ := envelope["subscribers"].([]any)
	assert.Len(t, subs, 2, "list_subscribers must return 2 subscribers")
}

// TestHandleSubscriber_Get_Existing_ReturnsIt verifies subscriber op=get with valid ID.
func TestHandleSubscriber_Get_Existing_ReturnsIt(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("subscriber", map[string]any{
		"op": "get",
		"id": uuidToStr(sub.ID.Bytes),
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "Alice", result["name"])
	assert.Equal(t, "27831234567", result["msisdn"])
}

// TestHandleSubscriber_Get_Missing_ReturnsToolError verifies subscriber op=get
// with unknown ID returns a structured tool error.
func TestHandleSubscriber_Get_Missing_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("subscriber", map[string]any{
		"op": "get",
		"id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "missing subscriber must return isError: true")
}

// TestHandleSubscriber_Create_ValidInput_ReturnsSubscriber verifies subscriber op=create.
func TestHandleSubscriber_Create_ValidInput_ReturnsSubscriber(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("subscriber", map[string]any{
		"op":     "create",
		"name":   "Carol",
		"msisdn": "27839876543",
		"iccid":  "8927003333",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "Carol", result["name"])
	assert.Equal(t, "27839876543", result["msisdn"])
	assert.NotEmpty(t, result["id"])
}

// TestHandleSubscriber_Create_MissingMsisdn_ReturnsToolError verifies missing
// required param returns a structured tool error.
func TestHandleSubscriber_Create_MissingMsisdn_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("subscriber", map[string]any{
		"op":    "create",
		"name":  "Carol",
		"iccid": "8927003333",
		// msisdn intentionally omitted
	})
	assert.True(t, isToolError(resp), "missing msisdn must return isError: true")
}

// TestHandleSubscriber_Update_ValidInput_ReturnsUpdated verifies subscriber op=update.
func TestHandleSubscriber_Update_ValidInput_ReturnsUpdated(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("subscriber", map[string]any{
		"op":     "update",
		"id":     uuidToStr(sub.ID.Bytes),
		"name":   "Alice Updated",
		"msisdn": "27831234567",
		"iccid":  "8927001111",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "Alice Updated", result["name"])
}

// TestHandleSubscriber_Delete_Existing_Deletes verifies subscriber op=delete.
func TestHandleSubscriber_Delete_Existing_Deletes(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Del", Msisdn: "27839999999", Iccid: "8927009999",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("subscriber", map[string]any{
		"op": "delete",
		"id": uuidToStr(sub.ID.Bytes),
	})
	var result map[string]string
	toolResult(t, resp, &result)
	assert.Equal(t, "deleted", result["status"])

	_, err = s.GetSubscriber(ctx, sub.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// uuidToStr converts a 16-byte UUID array to hyphenated string form.
// This is a test-local helper to avoid importing internal/api.
func uuidToStr(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
