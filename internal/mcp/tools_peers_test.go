package mcp_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mcpTestClient wraps an httptest.Server and manages a session for
// making tool calls in tests.
type mcpTestClient struct {
	ts        *httptest.Server
	sessionID string
	t         *testing.T
}

// newMCPTestClient creates a test MCP server with the given store and
// returns an initialised client with a valid session ID.
func newMCPTestClient(t *testing.T, s store.Store) *mcpTestClient {
	t.Helper()
	handler := internalmcp.NewServer(s, nil, nil, nil, nil, nil)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	// Initialize the session.
	initMsg := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "test", "version": "1.0.0"},
		},
	}
	resp, _ := postJSONWithSessionID(t, ts.Client(), ts.URL, initMsg, "")
	sessionID := resp.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID, "initialize must return a session ID")

	return &mcpTestClient{ts: ts, sessionID: sessionID, t: t}
}

// callTool invokes a tool by name with the given arguments and returns
// the parsed JSON-RPC response.
func (c *mcpTestClient) callTool(name string, args map[string]any) jsonRPCResponse {
	c.t.Helper()
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	_, resp := postJSONWithSessionID(c.t, c.ts.Client(), c.ts.URL, msg, c.sessionID)
	return resp
}

// toolResult extracts the content[0].text field from a tool call response
// and unmarshals it into the given target.
func toolResult(t *testing.T, resp jsonRPCResponse, target any) {
	t.Helper()
	require.Nil(t, resp.Error, "JSON-RPC error: %+v", resp.Error)
	content, ok := resp.Result["content"].([]any)
	require.True(t, ok, "result.content must be an array")
	require.NotEmpty(t, content, "content must not be empty")
	first, ok := content[0].(map[string]any)
	require.True(t, ok, "content[0] must be an object")
	text, ok := first["text"].(string)
	require.True(t, ok, "content[0].text must be a string")
	require.NoError(t, json.Unmarshal([]byte(text), target), "unmarshal tool result")
}

// toolError extracts the tool-level error message from a tool call response.
func toolError(t *testing.T, resp jsonRPCResponse) string {
	t.Helper()
	require.Nil(t, resp.Error, "unexpected JSON-RPC error: %+v", resp.Error)
	content, ok := resp.Result["content"].([]any)
	require.True(t, ok, "result.content must be an array")
	require.NotEmpty(t, content)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	text, _ := first["text"].(string)
	return text
}

// isToolError returns true when the response carries isError: true.
func isToolError(resp jsonRPCResponse) bool {
	isError, _ := resp.Result["isError"].(bool)
	return isError
}

// TestHandleListPeers_ReturnsPeers verifies list_peers returns all peers.
func TestHandleListPeers_ReturnsPeers(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1","port":3868}`))
	require.NoError(t, err)
	_, err = s.InsertPeer(ctx, "ocs-02", []byte(`{"host":"10.0.0.2","port":3868}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("list_peers", nil)

	var envelope map[string]any
	toolResult(t, resp, &envelope)
	peers, _ := envelope["peers"].([]any)
	assert.Len(t, peers, 2, "list_peers must return 2 peers")
	assert.Equal(t, "ocs-01", peers[0].(map[string]any)["name"])
	assert.Equal(t, "ocs-02", peers[1].(map[string]any)["name"])
}

// TestHandlePeer_Get_ExistingPeer_ReturnsIt verifies peer op=get with a valid ID.
func TestHandlePeer_Get_ExistingPeer_ReturnsIt(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1","port":3868}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("peer", map[string]any{
		"op": "get",
		"id": uuidToStr(peer.ID.Bytes),
	})

	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-01", result["name"])
	assert.Equal(t, "10.0.0.1", result["host"])
}

// TestHandlePeer_Get_MissingPeer_ReturnsToolError verifies peer op=get with
// an unknown ID returns a structured tool error.
func TestHandlePeer_Get_MissingPeer_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("peer", map[string]any{
		"op": "get",
		"id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "missing peer must return isError: true")
	errMsg := toolError(t, resp)
	assert.Contains(t, errMsg, "not found", "error message must mention not found")
}

// TestHandlePeer_Get_ByName_ReturnsIt verifies peer op=get resolves by name.
func TestHandlePeer_Get_ByName_ReturnsIt(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1","port":3868}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("peer", map[string]any{"op": "get", "name": "ocs-01"})

	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-01", result["name"])
	assert.Equal(t, "10.0.0.1", result["host"])
}

// TestHandlePeer_Get_MissingRequiredParam_ReturnsToolError verifies that
// peer op=get with neither id nor name returns a structured tool error.
func TestHandlePeer_Get_MissingRequiredParam_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("peer", map[string]any{"op": "get"})
	assert.True(t, isToolError(resp), "missing id and name must return isError: true")
}

// TestHandlePeer_Create_ValidInput_ReturnsPeer verifies peer op=create.
func TestHandlePeer_Create_ValidInput_ReturnsPeer(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("peer", map[string]any{
		"op":     "create",
		"name":   "ocs-new",
		"config": map[string]any{"host": "10.0.0.5", "port": float64(3868)},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-new", result["name"])
	assert.NotEmpty(t, result["id"], "created peer must have an id")
}

// TestHandlePeer_Create_DuplicateName_ReturnsToolError verifies duplicate
// name returns a structured tool error.
func TestHandlePeer_Create_DuplicateName_ReturnsToolError(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertPeer(ctx, "ocs-01", []byte(`{}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("peer", map[string]any{
		"op":     "create",
		"name":   "ocs-01",
		"config": map[string]any{},
	})
	assert.True(t, isToolError(resp), "duplicate name must return isError: true")
}

// TestHandlePeer_Update_ValidInput_ReturnsPeer verifies peer op=update.
func TestHandlePeer_Update_ValidInput_ReturnsPeer(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1"}`))
	require.NoError(t, err)
	peerID := uuidToStr(peer.ID.Bytes)

	client := newMCPTestClient(t, s)
	resp := client.callTool("peer", map[string]any{
		"op":     "update",
		"id":     peerID,
		"name":   "ocs-01-updated",
		"config": map[string]any{"host": "10.0.0.9"},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-01-updated", result["name"])
}

// TestHandlePeer_Delete_ExistingPeer_Deletes verifies peer op=delete.
func TestHandlePeer_Delete_ExistingPeer_Deletes(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-del", []byte(`{}`))
	require.NoError(t, err)
	peerID := uuidToStr(peer.ID.Bytes)

	client := newMCPTestClient(t, s)
	resp := client.callTool("peer", map[string]any{"op": "delete", "id": peerID})
	var result map[string]string
	toolResult(t, resp, &result)
	assert.Equal(t, "deleted", result["status"])

	_, err = s.GetPeer(ctx, peer.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// TestHandlePeerConnection_NilManager_ReturnsToolError verifies peer_connection
// with no PeerManager returns a tool error.
func TestHandlePeerConnection_NilManager_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("peer_connection", map[string]any{"action": "connect", "name": "ocs-01"})
	assert.True(t, isToolError(resp), "nil manager must return isError: true")

	resp = client.callTool("peer_connection", map[string]any{"action": "disconnect", "name": "ocs-01"})
	assert.True(t, isToolError(resp), "nil manager must return isError: true")
}
