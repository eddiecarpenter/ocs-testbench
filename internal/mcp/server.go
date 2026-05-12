// Package mcp implements the MCP (Model Context Protocol) server for the
// OCS Testbench. It exposes the testbench's operations as discoverable MCP
// tools over Streamable HTTP, allowing MCP-capable AI clients (Claude Desktop,
// Goose) to drive testing workflows directly.
//
// The server is a thin adapter over the same store/PeerManager/ExecutionEngine
// interfaces used by the REST API. It does not import internal/diameter directly.
package mcp

import (
	"net/http"

	"github.com/fiorix/go-diameter/v4/diam/dict"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	tmpl "github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// Server holds the dependencies injected into each tool handler.
type Server struct {
	store      store.Store
	peerMgr    api.PeerManager
	execEngine api.ExecutionEngine
	dict       tmpl.Dictionary
	parser     *dict.Parser // for AVP enumeration (list_avps); may be nil in tests
	cfg        *baseconfig.Config
}

// NewServer constructs the MCP http.Handler. It creates an MCPServer,
// registers all 32 tools with their annotations, and wraps it in a
// Streamable HTTP transport.
//
// parser is used only by list_avps to enumerate all loaded AVPs from the
// Diameter dictionary. Pass dict.Default in production; pass nil in tests
// (list_avps will return an empty list when parser is nil).
//
// The returned handler is mounted at /mcp in cmd/ocs-testbench/main.go.
func NewServer(
	s store.Store,
	mgr api.PeerManager,
	exec api.ExecutionEngine,
	dictArg tmpl.Dictionary,
	parser *dict.Parser,
	cfg *baseconfig.Config,
) http.Handler {
	srv := &Server{
		store:      s,
		peerMgr:    mgr,
		execEngine: exec,
		dict:       dictArg,
		parser:     parser,
		cfg:        cfg,
	}

	mcpServer := server.NewMCPServer(
		"ocs-testbench",
		"1.0.0",
	)

	// Register all tool groups.
	registerPeerTools(mcpServer, srv)
	registerSubscriberTools(mcpServer, srv)
	registerScenarioTools(mcpServer, srv)
	registerExecutionTools(mcpServer, srv)
	registerAVPTools(mcpServer, srv)
	registerSystemTools(mcpServer, srv)

	return server.NewStreamableHTTPServer(mcpServer)
}
