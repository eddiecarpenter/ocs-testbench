package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// registerPeerTools registers all 7 peer management tools on the MCP server.
func registerPeerTools(s *server.MCPServer, srv *Server) {
	// list_peers — read-only
	s.AddTool(
		mcp.NewTool("list_peers",
			mcp.WithDescription("List all configured Diameter peers."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListPeers,
	)

	// get_peer — read-only
	s.AddTool(
		mcp.NewTool("get_peer",
			mcp.WithDescription("Get a single Diameter peer by UUID or name. Provide either 'id' or 'name'; if both are given, 'id' takes precedence."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Description("UUID of the peer to retrieve."),
			),
			mcp.WithString("name",
				mcp.Description("Unique name of the peer to retrieve (e.g. 'OpenBSS')."),
			),
		),
		srv.handleGetPeer,
	)

	// create_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("create_peer",
			mcp.WithDescription("Create a new Diameter peer configuration."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Unique name for the peer."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Peer configuration object (host, port, originHost, originRealm, etc.)."),
			),
		),
		srv.handleCreatePeer,
	)

	// update_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("update_peer",
			mcp.WithDescription("Update an existing Diameter peer configuration."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the peer to update."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("New unique name for the peer."),
			),
			mcp.WithObject("config",
				mcp.Required(),
				mcp.Description("Updated peer configuration object."),
			),
		),
		srv.handleUpdatePeer,
	)

	// delete_peer — destructive
	s.AddTool(
		mcp.NewTool("delete_peer",
			mcp.WithDescription("Delete a Diameter peer configuration. Fails if the peer is referenced by any scenario."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("UUID of the peer to delete."),
			),
		),
		srv.handleDeletePeer,
	)

	// connect_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("connect_peer",
			mcp.WithDescription("Initiate the Diameter connection lifecycle for the named peer."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the peer to connect."),
			),
		),
		srv.handleConnectPeer,
	)

	// disconnect_peer — write, non-destructive
	s.AddTool(
		mcp.NewTool("disconnect_peer",
			mcp.WithDescription("Gracefully close the Diameter connection for the named peer."),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the peer to disconnect."),
			),
		),
		srv.handleDisconnectPeer,
	)
}

// — peer response shape —

// peerJSON is the MCP response shape for a Peer resource.
type peerJSON struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	OriginHost              string `json:"originHost"`
	OriginRealm             string `json:"originRealm"`
	OriginIP                string `json:"originIp"`
	OriginPort              int    `json:"originPort"`
	DestHost                string `json:"destHost,omitempty"`
	DestRealm               string `json:"destRealm,omitempty"`
	Transport               string `json:"transport"`
	WatchdogIntervalSeconds int    `json:"watchdogIntervalSeconds"`
	AutoConnect             bool   `json:"autoConnect"`
	Status                  string `json:"status"`
}

// peerBody is the JSONB payload persisted in the store (mirrors api.peerBody).
type peerBodyJSON struct {
	Host                    string `json:"host"`
	Port                    int    `json:"port"`
	OriginHost              string `json:"originHost"`
	OriginRealm             string `json:"originRealm"`
	OriginIP                string `json:"originIp"`
	OriginPort              int    `json:"originPort"`
	DestHost                string `json:"destHost,omitempty"`
	DestRealm               string `json:"destRealm,omitempty"`
	Transport               string `json:"transport"`
	WatchdogIntervalSeconds int    `json:"watchdogIntervalSeconds"`
	AutoConnect             bool   `json:"autoConnect"`
}

// toPeerJSON converts a store.Peer to a peerJSON response. The status
// comes from the live PeerManager state.
func (srv *Server) toPeerJSON(p store.Peer) peerJSON {
	resp := peerJSON{
		ID:     uuidStr(p.ID),
		Name:   p.Name,
		Status: srv.peerStatus(p.Name),
	}
	var b peerBodyJSON
	if len(p.Body) > 0 {
		_ = json.Unmarshal(p.Body, &b)
	}
	resp.Host = b.Host
	resp.Port = b.Port
	resp.OriginHost = b.OriginHost
	resp.OriginRealm = b.OriginRealm
	resp.OriginIP = b.OriginIP
	resp.OriginPort = b.OriginPort
	resp.DestHost = b.DestHost
	resp.DestRealm = b.DestRealm
	resp.Transport = b.Transport
	resp.WatchdogIntervalSeconds = b.WatchdogIntervalSeconds
	resp.AutoConnect = b.AutoConnect
	return resp
}

// peerStatus returns the live connection state string for a named peer.
func (srv *Server) peerStatus(name string) string {
	if srv.peerMgr == nil {
		return "stopped"
	}
	state, err := srv.peerMgr.State(name)
	if err != nil {
		return "stopped"
	}
	return state.String()
}

// peerBodyFromConfig marshals the config map[string]any from the tool
// input into the store JSONB body bytes.
func peerBodyFromConfig(config map[string]any) ([]byte, error) {
	return json.Marshal(config)
}

// — handlers —

// handleListPeers returns all configured peers with their live connection state.
func (srv *Server) handleListPeers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	peers, err := srv.store.ListPeers(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list peers failed: %v", err)), nil
	}
	out := make([]peerJSON, len(peers))
	for i, p := range peers {
		out[i] = srv.toPeerJSON(p)
	}
	return mcp.NewToolResultJSON(map[string]any{"peers": out})
}

// handleGetPeer returns a single peer by UUID or name.
// If 'id' is provided it takes precedence; otherwise 'name' is used.
func (srv *Server) handleGetPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr := req.GetString("id", "")
	name := req.GetString("name", "")

	if idStr == "" && name == "" {
		return mcp.NewToolResultError("either 'id' or 'name' is required"), nil
	}

	if idStr != "" {
		id, err := parseUUID(idStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid peer id %q: %v", idStr, err)), nil
		}
		peer, err := srv.store.GetPeer(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return mcp.NewToolResultError(fmt.Sprintf("peer %q not found", idStr)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("get peer failed: %v", err)), nil
		}
		return mcp.NewToolResultJSON(srv.toPeerJSON(peer))
	}

	// Lookup by name.
	peer, err := srv.store.GetPeerByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("peer %q not found", name)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("get peer failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(srv.toPeerJSON(peer))
}

// handleCreatePeer creates a new peer configuration.
func (srv *Server) handleCreatePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	args := req.GetArguments()
	config, ok := args["config"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("config is required and must be an object"), nil
	}
	body, err := peerBodyFromConfig(config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid config: %v", err)), nil
	}
	peer, err := srv.store.InsertPeer(ctx, name, body)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			return mcp.NewToolResultError(fmt.Sprintf("peer name %q is already taken", name)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("create peer failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(srv.toPeerJSON(peer))
}

// handleUpdatePeer updates an existing peer configuration.
func (srv *Server) handleUpdatePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid peer id %q: %v", idStr, err)), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	args := req.GetArguments()
	config, ok := args["config"].(map[string]any)
	if !ok {
		return mcp.NewToolResultError("config is required and must be an object"), nil
	}
	body, err := peerBodyFromConfig(config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid config: %v", err)), nil
	}
	peer, err := srv.store.UpdatePeer(ctx, id, name, body)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("peer %q not found", idStr)), nil
		}
		if errors.Is(err, store.ErrDuplicateName) {
			return mcp.NewToolResultError(fmt.Sprintf("peer name %q is already taken", name)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("update peer failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(srv.toPeerJSON(peer))
}

// handleDeletePeer deletes a peer configuration.
func (srv *Server) handleDeletePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid peer id %q: %v", idStr, err)), nil
	}
	if err := srv.store.DeletePeer(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return mcp.NewToolResultError(fmt.Sprintf("peer %q not found", idStr)), nil
		}
		if errors.Is(err, store.ErrForeignKey) {
			return mcp.NewToolResultError("peer cannot be deleted: it is referenced by one or more scenarios"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("delete peer failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "deleted"})
}

// handleConnectPeer initiates connection for a named peer.
func (srv *Server) handleConnectPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.peerMgr == nil {
		return mcp.NewToolResultError("peer manager not available"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	if err := srv.peerMgr.Connect(name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("connect failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "connecting", "peer": name})
}

// handleDisconnectPeer gracefully closes a named peer connection.
func (srv *Server) handleDisconnectPeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.peerMgr == nil {
		return mcp.NewToolResultError("peer manager not available"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	if err := srv.peerMgr.Disconnect(name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("disconnect failed: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]string{"status": "disconnected", "peer": name})
}

// — UUID helpers (not re-exported from internal/api) —

// uuidStr converts a pgtype.UUID to its string representation.
func uuidStr(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}

// parseUUID parses a UUID string and returns a pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
