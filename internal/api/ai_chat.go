package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	internalaiai "github.com/eddiecarpenter/ocs-testbench/internal/ai"
)

// mountAIChat registers the AI assistant chat endpoint.
//
// POST /ai/chat — real agentic SSE stream that calls the LLM, executes
// MCP tools on its behalf, and streams events to the browser. When agent
// is nil (LLM not configured) the endpoint returns 503.
func mountAIChat(r chi.Router, agent *internalaiai.Agent, sessions *internalaiai.SessionManager) {
	r.Post("/ai/chat", func(w http.ResponseWriter, req *http.Request) {
		if agent == nil || sessions == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError, "AI assistant not configured")
			return
		}
		handleAIChat(w, req, agent, sessions)
	})
}

// chatRequest is the request body for POST /ai/chat.
type chatRequest struct {
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatSSEEvent is the on-wire SSE payload for the AI chat stream.
type chatSSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// permissionRequiredData is emitted when a write/destructive tool needs approval.
type permissionRequiredData struct {
	CallID      string `json:"callId"`
	ToolName    string `json:"toolName"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
}

// handleAIChat implements POST /v1/ai/chat.
//
// It reads the X-Session-ID header (generating one if absent), loads the
// session's conversation history, and invokes the agentic loop via
// Agent.Run. SSE events (token, tool_call, permission_required, done,
// error) are streamed to the client as the loop progresses. On return
// the updated history is persisted back to the session.
func handleAIChat(w http.ResponseWriter, r *http.Request, agent *internalaiai.Agent, sessions *internalaiai.SessionManager) {
	// Resolve session ID — generate one if the client did not supply it.
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = internalaiai.GenerateSessionID()
	}

	// Load (or create) the session and get the current history.
	session := sessions.GetOrCreate(sessionID)

	// Decode the request body to extract the new user message.
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondInvalidRequest(w, "invalid request body: "+err.Error())
		return
	}

	// Convert the request messages to ai.Message and append to history.
	// The caller sends only the new turn; history already carries prior turns.
	history := append([]internalaiai.Message(nil), session.History...)
	for _, m := range req.Messages {
		history = append(history, internalaiai.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Set SSE headers — from this point on we are streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Session-ID", sessionID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondInternalError(w)
		return
	}

	// emit writes a single SSE event to the response writer and flushes.
	emit := func(evtType string, data any) error {
		evt := chatSSEEvent{Type: evtType, Data: data}
		b, err := json.Marshal(evt)
		if err != nil {
			return err
		}
		if _, werr := fmt.Fprintf(w, "data: %s\n\n", b); werr != nil {
			return werr
		}
		flusher.Flush()
		return nil
	}

	// Create a per-call PermChan so the permission endpoint can deliver
	// decisions to this handler goroutine. Buffered(1) so the HTTP handler
	// for the permission endpoint never blocks.
	permChan := make(chan internalaiai.PermissionDecision, 1)
	session.PermChan = permChan

	// perm implements PermissionFunc: checks AllowAlways first, then waits
	// for a decision from the permission endpoint (via PermChan), then
	// registers allow_always decisions for the session lifetime.
	perm := func(callID, toolName, tier string) bool {
		if session.AllowAlways[toolName] {
			return true
		}
		// Emit a permission_required event so the frontend can prompt the
		// operator, then block until the decision arrives or the context
		// is cancelled.
		_ = emit("permission_required", permissionRequiredData{
			CallID:   callID,
			ToolName: toolName,
			Tier:     tier,
		})
		select {
		case <-r.Context().Done():
			return false
		case decision := <-permChan:
			if decision.Decision == "allow_always" {
				session.AllowAlways[toolName] = true
			}
			return decision.Decision == "allow_once" || decision.Decision == "allow_always"
		}
	}

	// Run the agentic loop — this blocks until the LLM produces a final
	// response (no tool_calls), the context is cancelled, or a fatal error
	// occurs. The updated history is returned for persistence.
	updatedHistory, _ := agent.Run(r.Context(), history, emit, perm)

	// Clear the per-call PermChan so subsequent calls get a fresh channel.
	session.PermChan = nil

	// Persist the updated history back to the session.
	sessions.UpdateHistory(sessionID, updatedHistory)
}
