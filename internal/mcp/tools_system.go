package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/baseconfig"
)

// registerSystemTools registers the 3 system tools.
func registerSystemTools(s *server.MCPServer, srv *Server) {
	// get_health — read-only
	s.AddTool(
		mcp.NewTool("get_health",
			mcp.WithDescription("Get the health status of the testbench: store reachability and per-peer connection states."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleGetHealth,
	)

	// get_config — read-only
	s.AddTool(
		mcp.NewTool("get_config",
			mcp.WithDescription("Get the current runtime-editable configuration fields "+
				"(server address, metrics address, logging format, headless mode)."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleGetConfig,
	)

	// update_config — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_config",
			mcp.WithDescription("Update runtime-editable configuration fields in-memory. "+
				"WARNING: Changes apply only to the running process and are not persisted across restarts. "+
				"Excluded fields: database_url, peers (not runtime-editable)."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("logging_format",
				mcp.Description("Logging format: 'text' or 'json'. Optional."),
			),
			mcp.WithBoolean("headless",
				mcp.Description("Headless mode toggle. Optional."),
			),
		),
		srv.handleUpdateConfig,
	)
}

// — health response —

// healthJSON is the MCP response shape for get_health.
type healthJSON struct {
	StoreReachable bool            `json:"store_reachable"`
	Peers          []peerStateJSON `json:"peers"`
}

// peerStateJSON is one entry in the health peers list.
type peerStateJSON struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// — config response —

// configJSON is the MCP response shape for get_config.
type configJSON struct {
	ServerAddr    string `json:"server_addr"`
	MetricsAddr   string `json:"metrics_addr"`
	LoggingFormat string `json:"logging_format"`
	LoggingLevel  string `json:"logging_level"`
	Headless      bool   `json:"headless"`
}

// — handlers —

// handleGetHealth returns store reachability and per-peer connection states.
func (srv *Server) handleGetHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Probe store reachability via a lightweight list call.
	storeOK := false
	var peerStates []peerStateJSON

	if srv.store != nil {
		peers, err := srv.store.ListPeers(ctx)
		if err == nil {
			storeOK = true
			peerStates = make([]peerStateJSON, len(peers))
			for i, p := range peers {
				peerStates[i] = peerStateJSON{
					Name:  p.Name,
					State: srv.peerStatus(p.Name),
				}
			}
		}
	}

	return mcp.NewToolResultJSON(healthJSON{
		StoreReachable: storeOK,
		Peers:          peerStates,
	})
}

// handleGetConfig returns runtime-editable configuration fields.
func (srv *Server) handleGetConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.cfg == nil {
		return mcp.NewToolResultError("configuration not available"), nil
	}
	return mcp.NewToolResultJSON(configJSON{
		ServerAddr:    srv.cfg.Server.Addr,
		MetricsAddr:   srv.cfg.Metrics.Addr,
		LoggingFormat: srv.cfg.Logging.Format,
		LoggingLevel:  srv.cfg.Logging.Level,
		Headless:      srv.cfg.Headless,
	})
}

// handleUpdateConfig updates in-memory configuration fields.
// Changes do not persist across restarts.
func (srv *Server) handleUpdateConfig(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.cfg == nil {
		return mcp.NewToolResultError("configuration not available"), nil
	}

	changed := []string{}

	loggingFormat := req.GetString("logging_format", "")
	if loggingFormat != "" {
		if loggingFormat != "text" && loggingFormat != "json" {
			return mcp.NewToolResultError(fmt.Sprintf("logging_format must be 'text' or 'json', got %q", loggingFormat)), nil
		}
		srv.cfg.Logging.Format = loggingFormat
		changed = append(changed, "logging_format="+loggingFormat)
	}

	// headless: only update when explicitly provided as a non-default value.
	args := req.GetArguments()
	if headlessVal, ok := args["headless"].(bool); ok {
		srv.cfg.Headless = headlessVal
		changed = append(changed, fmt.Sprintf("headless=%v", headlessVal))
	}

	return mcp.NewToolResultJSON(map[string]any{
		"status":  "updated",
		"changed": changed,
		"warning": "Changes apply only to the running process and are not persisted across restarts.",
	})
}

// MetricsConfig is accessed via baseconfig.Config.Metrics.Addr.
// Declared here to avoid package-level import cycle; the real type is in baseconfig.
var _ = (*baseconfig.Config)(nil) // compile-time assertion that baseconfig.Config is visible
