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

// registerPeerTools registers the 3 peer tools on the MCP server.
//
//	list_peers      — list all peers (readonly)
//	peer            — CRUD: get · create · update · delete
//	peer_connection — lifecycle: connect · disconnect
func registerPeerTools(s *server.MCPServer, srv *Server) {
	// list_peers — readonly, no parameters
	s.AddTool(
		mcp.NewTool("list_peers",
			mcp.WithDescription("List all configured Diameter peers with their live connection status."),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		srv.handleListPeers,
	)

	// peer — CRUD operations
	s.AddTool(
		mcp.NewTool("peer",
			mcp.WithDescription(`Diameter peer CRUD.
op=get     id or name                          → single peer
op=create  name*, config*                      → new peer
op=update  id*, name*, config*                 → replace config
op=delete  id*                                 → remove (fails if used by a scenario)
config: {host, port, originHost, originRealm, transport, watchdogIntervalSeconds, autoConnect, originIp?, originPort?, destHost?, destRealm?}`),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false), // delete is the only destructive op; annotated at runtime
			mcp.WithString("op",
				mcp.Required(),
				mcp.Description("get | create | update | delete"),
			),
			mcp.WithString("id",
				mcp.Description("Peer UUID — required for update, delete, and get-by-id."),
			),
			mcp.WithString("name",
				mcp.Description("Peer name — required for create, update, and get-by-name."),
			),
			mcp.WithObject("config",
				mcp.Description("Peer config body — required for create and update."),
			),
		),
		srv.handlePeer,
	)

	// peer_connection — lifecycle operations
	s.AddTool(
		mcp.NewTool("peer_connection",
			mcp.WithDescription(`Control the live Diameter connection for a peer.
action=connect    name*  → initiate connection
action=disconnect name*  → gracefully close connection`),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("connect | disconnect"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the peer."),
			),
		),
		srv.handlePeerConnection,
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

// handlePeer dispatches CRUD operations for the consolidated "peer" tool.
func (srv *Server) handlePeer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	op, err := req.RequireString("op")
	if err != nil {
		return mcp.NewToolResultError("op is required (get | create | update | delete)"), nil
	}
	switch op {
	case "get":
		return srv.handleGetPeer(ctx, req)
	case "create":
		return srv.handleCreatePeer(ctx, req)
	case "update":
		return srv.handleUpdatePeer(ctx, req)
	case "delete":
		return srv.handleDeletePeer(ctx, req)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown op %q — use get | create | update | delete", op)), nil
	}
}

// handleGetPeer returns a single peer by UUID or name.
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
	config, ok := req.GetArguments()["config"].(map[string]any)
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
	config, ok := req.GetArguments()["config"].(map[string]any)
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

// handlePeerConnection dispatches connect/disconnect lifecycle operations.
func (srv *Server) handlePeerConnection(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if srv.peerMgr == nil {
		return mcp.NewToolResultError("peer manager not available"), nil
	}
	action, err := req.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("action is required (connect | disconnect)"), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}
	switch action {
	case "connect":
		if err := srv.peerMgr.Connect(name); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("connect failed: %v", err)), nil
		}
		return mcp.NewToolResultJSON(map[string]string{"status": "connecting", "peer": name})
	case "disconnect":
		if err := srv.peerMgr.Disconnect(name); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("disconnect failed: %v", err)), nil
		}
		return mcp.NewToolResultJSON(map[string]string{"status": "disconnected", "peer": name})
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown action %q — use connect | disconnect", action)), nil
	}
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
