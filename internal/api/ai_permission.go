package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	internalaiai "github.com/eddiecarpenter/ocs-testbench/internal/ai"
)

// mountAIPermission registers POST /ai/chat/permission for the
// Deny/Once/Always permission guardrail.
//
// sessions is the shared SessionManager that also handles /ai/chat.
// When sessions is nil the endpoint returns 503 (AI agent not configured).
func mountAIPermission(r chi.Router, sessions *internalaiai.SessionManager) {
	r.Post("/ai/chat/permission", func(w http.ResponseWriter, req *http.Request) {
		if sessions == nil {
			respondError(w, http.StatusServiceUnavailable, CodeInternalError, "AI assistant not configured")
			return
		}
		handleAIPermission(w, req, sessions)
	})
}

// permissionRequest is the JSON body for POST /v1/ai/chat/permission.
type permissionRequest struct {
	CallID   string `json:"callId"`
	Decision string `json:"decision"` // "allow_once" | "allow_always" | "deny"
}

// handleAIPermission processes a Deny/Once/Always decision from the
// frontend and delivers it to the agent goroutine waiting on the session's
// PermChan. The agent goroutine's PermissionFunc closure is responsible
// for registering allow_always decisions in Session.AllowAlways.
//
// Returns 404 when the session does not exist, 409 when no permission
// prompt is currently in flight (PermChan is nil or the buffer is full),
// and 204 No Content on success.
func handleAIPermission(w http.ResponseWriter, r *http.Request, sessions *internalaiai.SessionManager) {
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, CodeInvalidRequest, "X-Session-ID header is required")
		return
	}

	s := sessions.Get(sessionID)
	if s == nil {
		respondError(w, http.StatusNotFound, CodeNotFound, "session not found")
		return
	}

	var body permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondInvalidRequest(w, "invalid request body: "+err.Error())
		return
	}
	if body.CallID == "" {
		respondInvalidRequest(w, "callId is required")
		return
	}
	switch body.Decision {
	case "allow_once", "allow_always", "deny":
		// valid decisions
	default:
		respondInvalidRequest(w, `decision must be one of "allow_once", "allow_always", "deny"`)
		return
	}

	// PermChan must be non-nil for there to be an in-flight permission prompt.
	if s.PermChan == nil {
		respondError(w, http.StatusConflict, CodeConflict, "no permission prompt in flight")
		return
	}

	// Non-blocking send so the HTTP handler never blocks waiting for the
	// agent goroutine to read. PermChan is buffered(1), so one pending
	// decision always fits. If the buffer is full (a decision was already
	// queued, or the agent has exited), return 409.
	select {
	case s.PermChan <- internalaiai.PermissionDecision{
		CallID:   body.CallID,
		Decision: body.Decision,
	}:
	default:
		respondError(w, http.StatusConflict, CodeConflict, "no permission prompt in flight")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
