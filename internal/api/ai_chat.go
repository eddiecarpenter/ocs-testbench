package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// mountAIChat registers the AI assistant chat endpoint.
//
// POST /ai/chat — scripted SSE stream simulating an agentic loop with
// thinking, token streaming, tool calls, and permission prompts.  This
// is a development stub; Feature 3 replaces it with a real LLM runtime.
func mountAIChat(r chi.Router) {
	r.Post("/ai/chat", handleAIChat)
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

// tokenData carries a single streamed text chunk.
type tokenData struct {
	Chunk string `json:"chunk"`
}

// thinkingData carries planning block content.
type thinkingData struct {
	Content string `json:"content"`
}

// toolCallData carries a tool invocation and its result.
type toolCallData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tier        string `json:"tier"`
	Result      any    `json:"result"`
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
// It streams a scripted SSE sequence that exercises all three permission
// tiers so the frontend guardrail can be developed without a live LLM.
//
// Scripted sequence:
//  1. thinking — "Analysing request, selecting tools…"
//  2. token stream — introductory assistant text (5 chunks)
//  3. tool_call — list_peers (read-only tier, result inline)
//  4. tool_call — duplicate_scenario (write tier) → permission_required
//  5. token stream — continuation text after approval (4 chunks)
//  6. tool_call — start_execution (write tier, running state)
//  7. done
//
// Stream delay: 150 ms between token chunks to simulate real streaming.
func handleAIChat(w http.ResponseWriter, r *http.Request) {
	// Decode and discard the request body — the mock doesn't use it, but
	// we read it to respect the HTTP contract (avoid broken-pipe on the client).
	var req chatRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Set SSE headers — from this point on we are streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondInternalError(w)
		return
	}

	emit := func(evtType string, data any) bool {
		evt := chatSSEEvent{Type: evtType, Data: data}
		b, err := json.Marshal(evt)
		if err != nil {
			return false
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
		return true
	}

	delay := func() bool {
		select {
		case <-r.Context().Done():
			return false
		case <-time.After(150 * time.Millisecond):
			return true
		}
	}

	// 1 — thinking block
	if !emit("thinking", thinkingData{Content: "Analysing request, selecting tools…"}) {
		return
	}
	if !delay() {
		return
	}

	// 2 — introductory token stream
	introTokens := []string{
		"I'll help you with that. ",
		"Let me first check the connected peers ",
		"to understand the current topology, ",
		"then duplicate the scenario ",
		"and start an execution for you.",
	}
	for _, chunk := range introTokens {
		if !emit("token", tokenData{Chunk: chunk}) {
			return
		}
		if !delay() {
			return
		}
	}

	// 3 — read-only tool call: list_peers (no permission prompt)
	if !emit("tool_call", toolCallData{
		ID:          "tc-list-peers",
		Name:        "list_peers",
		Description: "List all registered Diameter peers and their connection state",
		Tier:        "readonly",
		Result:      []map[string]string{{"name": "ocs-01", "state": "connected"}, {"name": "ocs-02", "state": "disconnected"}},
	}) {
		return
	}
	if !delay() {
		return
	}

	// 4 — write tool call: duplicate_scenario → needs permission prompt
	if !emit("permission_required", permissionRequiredData{
		CallID:      "tc-dup-scenario",
		ToolName:    "duplicate_scenario",
		Description: "Create a copy of scenario 'Gy — data single MSCC'",
		Tier:        "write",
	}) {
		return
	}
	// Stream pauses here; the client resumes it after the user approves or
	// denies.  For the scripted mock, we continue automatically after a
	// short pause to exercise the continuation path.
	if !delay() {
		return
	}
	if !delay() {
		return
	}

	if !emit("tool_call", toolCallData{
		ID:          "tc-dup-scenario",
		Name:        "duplicate_scenario",
		Description: "Create a copy of scenario 'Gy — data single MSCC'",
		Tier:        "write",
		Result:      map[string]string{"id": "scn-copy-001", "name": "Copy of Gy — data single MSCC"},
	}) {
		return
	}
	if !delay() {
		return
	}

	// 5 — continuation tokens
	continuationTokens := []string{
		"\n\nScenario duplicated. ",
		"Now starting an execution ",
		"against peer ocs-01.",
	}
	for _, chunk := range continuationTokens {
		if !emit("token", tokenData{Chunk: chunk}) {
			return
		}
		if !delay() {
			return
		}
	}

	// 6 — write tool call: start_execution (running state)
	if !emit("permission_required", permissionRequiredData{
		CallID:      "tc-start-exec",
		ToolName:    "start_execution",
		Description: "Run the copied scenario against peer ocs-01 (1 repeat, interactive mode)",
		Tier:        "write",
	}) {
		return
	}
	if !delay() {
		return
	}
	if !delay() {
		return
	}

	if !emit("tool_call", toolCallData{
		ID:          "tc-start-exec",
		Name:        "start_execution",
		Description: "Run the copied scenario against peer ocs-01 (1 repeat, interactive mode)",
		Tier:        "write",
		Result:      map[string]string{"sessionId": "session-mock-001", "state": "running"},
	}) {
		return
	}
	if !delay() {
		return
	}

	// 7 — done
	emit("done", map[string]any{}) //nolint:errcheck
}
