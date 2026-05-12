package mcp_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// newMCPClientWithCfg creates an MCP test client backed by the given config.
func newMCPClientWithCfg(t *testing.T, cfg *baseconfig.Config) *mcpTestClient {
	t.Helper()
	handler := internalmcp.NewServer(store.NewTestStore(), nil, nil, nil, nil, cfg)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

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
	return &mcpTestClient{ts: ts, sessionID: sessionID, t: t}
}

// TestHandleListAVPs_NilParser_ReturnsEmpty verifies list_avps with nil parser returns empty.
func TestHandleListAVPs_NilParser_ReturnsEmpty(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("list_avps", nil)

	var result []any
	toolResult(t, resp, &result)
	assert.Empty(t, result, "nil parser must return empty AVP list")
}

// TestHandleGetAVPInfo_NilDict_ReturnsToolError verifies get_avp_info with nil dict.
func TestHandleGetAVPInfo_NilDict_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_avp_info", map[string]any{"name": "Origin-Host"})
	assert.True(t, isToolError(resp), "nil dict must return isError: true")
}

// TestHandleGetAVPInfo_MissingName_ReturnsToolError verifies AC-4 for get_avp_info.
func TestHandleGetAVPInfo_MissingName_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_avp_info", map[string]any{})
	assert.True(t, isToolError(resp), "missing name must return isError: true")
}

// TestHandleGetHealth_ReturnsStoreReachableField verifies get_health has expected shape.
func TestHandleGetHealth_ReturnsStoreReachableField(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_health", nil)

	var result map[string]any
	toolResult(t, resp, &result)
	_, hasPeers := result["peers"]
	assert.True(t, hasPeers, "get_health must return peers field")
	_, hasStoreOK := result["store_reachable"]
	assert.True(t, hasStoreOK, "get_health must return store_reachable field")
}

// TestHandleGetConfig_WithConfig_ReturnsFields verifies get_config returns correct fields.
func TestHandleGetConfig_WithConfig_ReturnsFields(t *testing.T) {
	cfg := &baseconfig.Config{
		BaseConfig: baseconfig.BaseConfig{
			Logging: baseconfig.LogConfig{Format: "json", Level: "debug"},
		},
		Server:   baseconfig.ServerConfig{Addr: ":8080"},
		Headless: true,
	}
	client := newMCPClientWithCfg(t, cfg)
	resp := client.callTool("get_config", nil)

	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, ":8080", result["server_addr"])
	assert.Equal(t, "json", result["logging_format"])
	assert.Equal(t, "debug", result["logging_level"])
	assert.Equal(t, true, result["headless"])
}

// TestHandleGetConfig_NilConfig_ReturnsToolError verifies get_config with nil config.
func TestHandleGetConfig_NilConfig_ReturnsToolError(t *testing.T) {
	client := newMCPTestClient(t, store.NewTestStore())
	resp := client.callTool("get_config", nil)
	assert.True(t, isToolError(resp), "nil config must return isError: true")
}

// TestHandleUpdateConfig_ValidInput_UpdatesFields verifies update_config applies changes.
func TestHandleUpdateConfig_ValidInput_UpdatesFields(t *testing.T) {
	cfg := &baseconfig.Config{
		BaseConfig: baseconfig.BaseConfig{
			Logging: baseconfig.LogConfig{Format: "text"},
		},
		Server: baseconfig.ServerConfig{Addr: ":8080"},
	}
	client := newMCPClientWithCfg(t, cfg)
	resp := client.callTool("update_config", map[string]any{
		"logging_format": "json",
	})
	var result map[string]any
	toolResult(t, resp, &result)
	assert.Equal(t, "updated", result["status"])
	assert.Equal(t, "json", cfg.Logging.Format, "config must be updated in-memory")
}

// TestHandleUpdateConfig_InvalidLoggingFormat_ReturnsToolError verifies AC-4.
func TestHandleUpdateConfig_InvalidLoggingFormat_ReturnsToolError(t *testing.T) {
	cfg := &baseconfig.Config{}
	client := newMCPClientWithCfg(t, cfg)
	resp := client.callTool("update_config", map[string]any{
		"logging_format": "xml",
	})
	assert.True(t, isToolError(resp), "invalid format must return isError: true")
}
