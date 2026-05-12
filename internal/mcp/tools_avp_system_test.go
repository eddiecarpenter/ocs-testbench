package mcp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	tmpl "github.com/eddiecarpenter/ocs-testbench/internal/template"
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

// fakeMCPDictionary implements tmpl.Dictionary for MCP AVP-tool unit tests.
// It returns known metadata for "Origin-Host" and an error for all other names.
type fakeMCPDictionary struct{}

func (f *fakeMCPDictionary) Lookup(name string) (tmpl.AVPMetadata, error) {
	switch name {
	case "Origin-Host":
		return tmpl.AVPMetadata{Code: 264, VendorID: 0, DataType: "UTF8String"}, nil
	default:
		return tmpl.AVPMetadata{}, fmt.Errorf("AVP %q not found in dictionary", name)
	}
}

// newMCPClientWithDictionary creates an MCP test client backed by the given dictionary.
func newMCPClientWithDictionary(t *testing.T, dict tmpl.Dictionary) *mcpTestClient {
	t.Helper()
	handler := internalmcp.NewServer(store.NewTestStore(), nil, nil, dict, nil, nil)
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
	resp, _ := postJSONWithSessionID(t, http.DefaultClient, ts.URL, initMsg, "")
	sessionID := resp.Header.Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID, "initialize must return a session ID")
	return &mcpTestClient{ts: ts, sessionID: sessionID, t: t}
}

// TestHandleGetAVPInfo_KnownAVP_ReturnsAllFields verifies AC-6: get_avp_info must return
// populated code, vendorId, and dataType fields for a known AVP (Origin-Host, code 264).
func TestHandleGetAVPInfo_KnownAVP_ReturnsAllFields(t *testing.T) {
	client := newMCPClientWithDictionary(t, &fakeMCPDictionary{})
	resp := client.callTool("get_avp_info", map[string]any{"name": "Origin-Host"})

	var result map[string]any
	toolResult(t, resp, &result)

	assert.Equal(t, "Origin-Host", result["name"], "get_avp_info must return the AVP name")
	assert.Equal(t, float64(264), result["code"], "get_avp_info must return code 264 for Origin-Host")
	assert.Equal(t, "UTF8String", result["dataType"], "get_avp_info must return correct Diameter data type")
	// vendorId is omitted from the JSON when zero (IETF base AVP) — absence equals 0.
	if vid, ok := result["vendorId"]; ok {
		assert.Equal(t, float64(0), vid, "IETF AVP must have vendorId 0 or absent")
	}
}
