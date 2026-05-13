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

// TestHandleGetSubscriber_Existing_ReturnsIt verifies get_subscriber with valid ID.
func TestHandleGetSubscriber_Existing_ReturnsIt(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("get_subscriber", map[string]any{
		"id": uuidToStr(sub.ID.Bytes),
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "Alice", result["name"])
	assert.Equal(t, "27831234567", result["msisdn"])
}

// TestHandleGetSubscriber_Missing_ReturnsToolError verifies get_subscriber
// with unknown ID returns a structured tool error (AC-4).
func TestHandleGetSubscriber_Missing_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_subscriber", map[string]any{
		"id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "missing subscriber must return isError: true")
}

// TestHandleCreateSubscriber_ValidInput_ReturnsSubscriber verifies create_subscriber.
func TestHandleCreateSubscriber_ValidInput_ReturnsSubscriber(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("create_subscriber", map[string]any{
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

// TestHandleCreateSubscriber_MissingMsisdn_ReturnsToolError verifies AC-4
// for missing required params.
func TestHandleCreateSubscriber_MissingMsisdn_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("create_subscriber", map[string]any{
		"name":  "Carol",
		"iccid": "8927003333",
		// msisdn intentionally omitted
	})
	assert.True(t, isToolError(resp), "missing msisdn must return isError: true")
}

// TestHandleUpdateSubscriber_ValidInput_ReturnsUpdated verifies update_subscriber.
func TestHandleUpdateSubscriber_ValidInput_ReturnsUpdated(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Alice", Msisdn: "27831234567", Iccid: "8927001111",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("update_subscriber", map[string]any{
		"id":     uuidToStr(sub.ID.Bytes),
		"name":   "Alice Updated",
		"msisdn": "27831234567",
		"iccid":  "8927001111",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "Alice Updated", result["name"])
}

// TestHandleDeleteSubscriber_Existing_Deletes verifies delete_subscriber removes it.
func TestHandleDeleteSubscriber_Existing_Deletes(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	sub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "Del", Msisdn: "27839999999", Iccid: "8927009999",
	})
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("delete_subscriber", map[string]any{
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
