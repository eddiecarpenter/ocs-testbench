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

	var peers []map[string]any
	toolResult(t, resp, &peers)
	assert.Len(t, peers, 2, "list_peers must return 2 peers")
	assert.Equal(t, "ocs-01", peers[0]["name"])
	assert.Equal(t, "ocs-02", peers[1]["name"])
}

// TestHandleGetPeer_ExistingPeer_ReturnsIt verifies get_peer with a valid ID.
func TestHandleGetPeer_ExistingPeer_ReturnsIt(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1","port":3868}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("get_peer", map[string]any{
		"id": uuidToStr(peer.ID.Bytes),
	})

	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-01", result["name"])
	assert.Equal(t, "10.0.0.1", result["host"])
}

// TestHandleGetPeer_MissingPeer_ReturnsToolError verifies get_peer with
// an unknown ID returns a structured tool error (AC-4).
func TestHandleGetPeer_MissingPeer_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_peer", map[string]any{
		"id": "00000000-0000-0000-0000-000000000000",
	})
	assert.True(t, isToolError(resp), "missing peer must return isError: true")
	errMsg := toolError(t, resp)
	assert.Contains(t, errMsg, "not found", "error message must mention not found")
}

// TestHandleGetPeer_MissingRequiredParam_ReturnsToolError verifies AC-4:
// missing required params return structured tool errors (not panics).
func TestHandleGetPeer_MissingRequiredParam_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	// Call without id parameter.
	resp := client.callTool("get_peer", map[string]any{})
	assert.True(t, isToolError(resp), "missing id param must return isError: true")
}

// TestHandleCreatePeer_ValidInput_ReturnsPeer verifies create_peer creates a peer.
func TestHandleCreatePeer_ValidInput_ReturnsPeer(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("create_peer", map[string]any{
		"name":   "ocs-new",
		"config": map[string]any{"host": "10.0.0.5", "port": float64(3868)},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-new", result["name"])
	assert.NotEmpty(t, result["id"], "created peer must have an id")
}

// TestHandleCreatePeer_DuplicateName_ReturnsToolError verifies duplicate
// name returns a structured tool error.
func TestHandleCreatePeer_DuplicateName_ReturnsToolError(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertPeer(ctx, "ocs-01", []byte(`{}`))
	require.NoError(t, err)

	client := newMCPTestClient(t, s)
	resp := client.callTool("create_peer", map[string]any{
		"name":   "ocs-01",
		"config": map[string]any{},
	})
	assert.True(t, isToolError(resp), "duplicate name must return isError: true")
}

// TestHandleUpdatePeer_ValidInput_ReturnsPeer verifies update_peer updates a peer.
func TestHandleUpdatePeer_ValidInput_ReturnsPeer(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1"}`))
	require.NoError(t, err)
	peerID := uuidToStr(peer.ID.Bytes)

	client := newMCPTestClient(t, s)
	resp := client.callTool("update_peer", map[string]any{
		"id":     peerID,
		"name":   "ocs-01-updated",
		"config": map[string]any{"host": "10.0.0.9"},
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "ocs-01-updated", result["name"])
}

// TestHandleDeletePeer_ExistingPeer_Deletes verifies delete_peer removes a peer.
func TestHandleDeletePeer_ExistingPeer_Deletes(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-del", []byte(`{}`))
	require.NoError(t, err)
	peerID := uuidToStr(peer.ID.Bytes)

	client := newMCPTestClient(t, s)
	resp := client.callTool("delete_peer", map[string]any{"id": peerID})
	var result map[string]string
	toolResult(t, resp, &result)
	assert.Equal(t, "deleted", result["status"])

	// Verify it's gone.
	_, err = s.GetPeer(ctx, peer.ID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// TestHandleConnectPeer_NilManager_ReturnsToolError verifies connect_peer
// with no PeerManager returns a tool error.
func TestHandleConnectPeer_NilManager_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("connect_peer", map[string]any{"name": "ocs-01"})
	assert.True(t, isToolError(resp), "nil manager must return isError: true")
}

// TestHandleDisconnectPeer_NilManager_ReturnsToolError verifies disconnect_peer
// with no PeerManager returns a tool error.
func TestHandleDisconnectPeer_NilManager_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("disconnect_peer", map[string]any{"name": "ocs-01"})
	assert.True(t, isToolError(resp), "nil manager must return isError: true")
}
