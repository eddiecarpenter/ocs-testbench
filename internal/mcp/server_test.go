package mcp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// jsonRPCResponse mirrors the JSON-RPC 2.0 response shape.
type jsonRPCResponse struct {
	Jsonrpc string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  map[string]any `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// newTestMCPServer creates the MCP http.Handler backed by an in-memory store.
// All optional dependencies are nil to keep unit tests database-free.
func newTestMCPServer() http.Handler {
	return internalmcp.NewServer(
		store.NewTestStore(),
		nil, // PeerManager — nil is valid; tools return errors gracefully
		nil, // ExecutionEngine — nil is valid
		nil, // Dictionary — nil is valid
		nil, // dict.Parser — nil means list_avps returns empty (acceptable in tests)
		nil, // Config — nil is valid
	)
}

// postJSONWithSessionID sends a JSON-RPC POST with the given session ID header.
// Callers that have no session ID (e.g. initialize) pass an empty string.
func postJSONWithSessionID(t *testing.T, client *http.Client, url string, body any, sessionID string) (*http.Response, jsonRPCResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err, "marshal JSON-RPC request")

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := client.Do(req)
	require.NoError(t, err, "send JSON-RPC request")
	defer resp.Body.Close()

	body2, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var rpcResp jsonRPCResponse
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"expected 200 OK from MCP endpoint, body: %s", string(body2))
	require.NoError(t, json.Unmarshal(body2, &rpcResp), "decode JSON-RPC response: %s", string(body2))
	return resp, rpcResp
}

// TestNewServer_ToolListReturns34Tools verifies AC-1: the tool-list request
// returns exactly 34 tools with non-empty names and correct hint annotations.
func TestNewServer_ToolListReturns34Tools(t *testing.T) {
	handler := newTestMCPServer()
	require.NotNil(t, handler, "NewServer must return a non-nil handler")

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Step 1: Initialize the session (no session ID sent — server generates one).
	initMsg := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	initHTTPResp, initRPCResp := postJSONWithSessionID(t, ts.Client(), ts.URL, initMsg, "")
	assert.Nil(t, initRPCResp.Error, "initialize must succeed without error")
	assert.Equal(t, mcp.LATEST_PROTOCOL_VERSION, initRPCResp.Result["protocolVersion"],
		"protocolVersion must match")

	// Capture the session ID assigned by the server.
	sessionID := initHTTPResp.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID, "server must return Mcp-Session-Id after initialize")

	// Step 2: Request the tool list using the session ID from initialize.
	listMsg := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	_, listRPCResp := postJSONWithSessionID(t, ts.Client(), ts.URL, listMsg, sessionID)
	assert.Nil(t, listRPCResp.Error, "tools/list must succeed without error")

	tools, ok := listRPCResp.Result["tools"].([]any)
	require.True(t, ok, "result.tools must be an array, got: %T", listRPCResp.Result["tools"])
	// Peers: 7 → 3 (list_peers, peer, peer_connection).
	// Subscribers: 5 → 2 (list_subscribers, subscriber).
	// Scenarios: 6 → 2 (list_scenarios, scenario).
	assert.Len(t, tools, 23, "exactly 23 tools must be registered")

	// Build a name→annotations map for spot-check assertions.
	type toolAnnotations struct {
		readOnly    bool
		destructive bool
	}
	toolMap := make(map[string]toolAnnotations, len(tools))
	for _, rawTool := range tools {
		m, ok := rawTool.(map[string]any)
		require.True(t, ok, "each tool entry must be an object")
		name, _ := m["name"].(string)
		require.NotEmpty(t, name, "tool name must be non-empty")

		ta := toolAnnotations{}
		if ann, ok := m["annotations"].(map[string]any); ok {
			ta.readOnly, _ = ann["readOnlyHint"].(bool)
			ta.destructive, _ = ann["destructiveHint"].(bool)
		}
		toolMap[name] = ta
	}

	// Spot-check read-only tools.
	readOnlyTools := []string{
		"list_peers",
		"list_subscribers",
		"list_scenarios",
		"list_executions", "get_execution", "get_execution_detail",
		"list_avps", "get_avp_info",
		"get_health", "get_config",
	}
	for _, name := range readOnlyTools {
		ta, exists := toolMap[name]
		require.True(t, exists, "tool %q must be registered", name)
		assert.True(t, ta.readOnly, "tool %q must have readOnlyHint=true", name)
		assert.False(t, ta.destructive, "tool %q must have destructiveHint=false", name)
	}

	// Spot-check destructive tools.
	destructiveTools := []string{"stop_execution"}
	for _, name := range destructiveTools {
		ta, exists := toolMap[name]
		require.True(t, exists, "tool %q must be registered", name)
		assert.False(t, ta.readOnly, "tool %q must have readOnlyHint=false", name)
		assert.True(t, ta.destructive, "tool %q must have destructiveHint=true", name)
	}

	// Spot-check write non-destructive tools.
	writeTools := []string{
		"peer", "peer_connection",
		"subscriber",
		"scenario",
		"start_execution", "resume_execution", "step_execution",
		"apply_execution_context_override", "apply_execution_payload_override",
		"update_config",
	}
	for _, name := range writeTools {
		ta, exists := toolMap[name]
		require.True(t, exists, "tool %q must be registered", name)
		assert.False(t, ta.readOnly, "tool %q must have readOnlyHint=false", name)
		assert.False(t, ta.destructive, "tool %q must have destructiveHint=false", name)
	}
}
