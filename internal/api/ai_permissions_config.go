package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	internalaiai "github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// mountAIPermissionsConfig registers the AI tool permission management
// endpoints under /config/ai/permissions.
//
//	GET    /config/ai/permissions            — merged list of MCP tools + DB decisions
//	PATCH  /config/ai/permissions/{tool}     — set decision for a tool
//	DELETE /config/ai/permissions/{tool}     — reset tool decision to "ask"
//
// s is required (permission records live in the database).
// agent may be nil — when nil, GET returns only DB rows (no MCP tools).
func mountAIPermissionsConfig(r chi.Router, s store.Store, agent *internalaiai.Agent) {
	r.Get("/config/ai/permissions", getAIPermissions(s, agent))
	r.Patch("/config/ai/permissions/{tool}", setAIPermission(s))
	r.Delete("/config/ai/permissions/{tool}", deleteAIPermission(s))
}

// aiToolPermissionResponse is the on-wire shape for a single tool row.
type aiToolPermissionResponse struct {
	ToolName    string `json:"toolName"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
	Decision    string `json:"decision"` // "ask" | "allow" | "deny"
}

// aiPermissionInput is the request body for PATCH /config/ai/permissions/{tool}.
type aiPermissionInput struct {
	Decision string `json:"decision"` // "ask" | "allow" | "deny"
}

// getAIPermissions implements GET /v1/config/ai/permissions.
//
// It fetches the live MCP tool list (if agent is non-nil) and merges it
// with the persisted DB decisions. Every known MCP tool appears in the
// result; DB-only rows (tools no longer registered with MCP) are also
// included so stored decisions are visible. If MCP is unavailable, only
// DB rows are returned.
func getAIPermissions(s store.Store, agent *internalaiai.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Load DB decisions.
		dbPerms, err := s.ListAIPermissions(ctx)
		if err != nil {
			respondError(w, http.StatusInternalServerError, CodeInternalError, "failed to load permissions: "+err.Error())
			return
		}
		// Build a lookup map: toolName → decision from DB.
		dbByName := make(map[string]string, len(dbPerms))
		for _, p := range dbPerms {
			dbByName[p.ToolName] = p.Decision
		}

		// Build the result list starting with MCP tools (if available).
		var out []aiToolPermissionResponse
		seen := make(map[string]bool)

		if agent != nil {
			mcpTools, mcpErr := agent.ListMCPTools(ctx)
			if mcpErr == nil && len(mcpTools) > 0 {
				for _, t := range mcpTools {
					decision := "ask"
					if d, ok := dbByName[t.Name]; ok {
						decision = d
					}
					out = append(out, aiToolPermissionResponse{
						ToolName:    t.Name,
						Description: t.Description,
						Tier:        t.Tier,
						Decision:    decision,
					})
					seen[t.Name] = true
				}
			}
		}

		// Append DB-only rows (tools that had decisions but are no longer
		// registered with MCP, or MCP was unavailable).
		for _, p := range dbPerms {
			if seen[p.ToolName] {
				continue
			}
			out = append(out, aiToolPermissionResponse{
				ToolName: p.ToolName,
				Decision: p.Decision,
			})
		}

		if out == nil {
			out = []aiToolPermissionResponse{}
		}
		respondJSON(w, http.StatusOK, out)
	}
}

// setAIPermission implements PATCH /v1/config/ai/permissions/{tool}.
//
// Accepts {"decision": "allow"|"deny"|"ask"}. When decision is "ask" the
// record is deleted (reset to default). Otherwise the decision is upserted.
func setAIPermission(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		toolName := chi.URLParam(r, "tool")
		if toolName == "" {
			respondInvalidRequest(w, "tool name is required")
			return
		}

		var input aiPermissionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			respondInvalidRequest(w, "invalid request body: "+err.Error())
			return
		}
		switch input.Decision {
		case "ask", "allow", "deny":
			// valid
		default:
			respondInvalidRequest(w, `decision must be one of "ask", "allow", "deny"`)
			return
		}

		ctx := r.Context()
		var opErr error
		if input.Decision == "ask" {
			opErr = s.DeleteAIPermission(ctx, toolName)
		} else {
			opErr = s.UpsertAIPermission(ctx, toolName, input.Decision)
		}
		if opErr != nil {
			respondError(w, http.StatusInternalServerError, CodeInternalError, "failed to save permission: "+opErr.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteAIPermission implements DELETE /v1/config/ai/permissions/{tool}.
//
// Deletes the stored decision, effectively resetting the tool to "ask".
func deleteAIPermission(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		toolName := chi.URLParam(r, "tool")
		if toolName == "" {
			respondInvalidRequest(w, "tool name is required")
			return
		}
		if err := s.DeleteAIPermission(r.Context(), toolName); err != nil {
			respondError(w, http.StatusInternalServerError, CodeInternalError, "failed to delete permission: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
