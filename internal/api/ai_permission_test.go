package api_test

import (
	"context"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
	"github.com/eddiecarpenter/ocs-testbench/internal/api"
)

// buildPermissionRequest builds an HTTP request for the permission endpoint.
func buildPermissionRequest(t *testing.T, sessionID, callID, decision string) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"callId":   callID,
		"decision": decision,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat/permission", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	return req
}

// makePermissionRouter builds a chi Router with the permission endpoint wired.
// Intentionally calls the exported wiring helper introduced in Task 5.
// Since Task 4 creates the file without wiring into Router yet, we test
// the handler directly via the export_test bridge.
func makePermissionRouter(sessions *ai.SessionManager) http.Handler {
	return api.PermissionTestRouter(sessions)
}

// TestAIPermission_AllowOnce verifies that an allow_once decision is
// delivered to the session's PermChan and the endpoint returns 204.
func TestAIPermission_AllowOnce(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-allow")
	// Create a buffered PermChan to simulate an in-flight permission prompt.
	sess.PermChan = make(chan ai.PermissionDecision, 1)

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-allow", "call-1", "allow_once"))

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify the decision was delivered to PermChan.
	var decision ai.PermissionDecision
	select {
	case decision = <-sess.PermChan:
	default:
		t.Fatal("PermChan must have received a decision")
	}
	assert.Equal(t, "call-1", decision.CallID)
	assert.Equal(t, "allow_once", decision.Decision)
}

// TestAIPermission_AllowAlways verifies allow_always decision is delivered.
func TestAIPermission_AllowAlways(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-always")
	sess.PermChan = make(chan ai.PermissionDecision, 1)

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-always", "call-2", "allow_always"))

	assert.Equal(t, http.StatusNoContent, rr.Code)

	select {
	case d := <-sess.PermChan:
		assert.Equal(t, "allow_always", d.Decision)
	default:
		t.Fatal("expected decision in PermChan")
	}
}

// TestAIPermission_Deny verifies deny decision is delivered.
func TestAIPermission_Deny(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-deny")
	sess.PermChan = make(chan ai.PermissionDecision, 1)

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-deny", "call-3", "deny"))

	assert.Equal(t, http.StatusNoContent, rr.Code)

	select {
	case d := <-sess.PermChan:
		assert.Equal(t, "deny", d.Decision)
	default:
		t.Fatal("expected decision in PermChan")
	}
}

// TestAIPermission_SessionNotFound verifies 404 when session is unknown.
func TestAIPermission_SessionNotFound(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "no-such-session", "call-x", "allow_once"))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestAIPermission_NilPermChan_Returns409 verifies that 409 Conflict is
// returned when no permission prompt is currently in flight.
func TestAIPermission_NilPermChan_Returns409(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sm.GetOrCreate(context.Background(), "sess-no-perm")
	// PermChan is nil — no prompt in flight.

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-no-perm", "call-y", "allow_once"))

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestAIPermission_FullPermChan_Returns409 verifies 409 when the buffer
// already holds a decision (second send would block).
func TestAIPermission_FullPermChan_Returns409(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-full")
	sess.PermChan = make(chan ai.PermissionDecision, 1)
	// Fill the buffer.
	sess.PermChan <- ai.PermissionDecision{CallID: "old", Decision: "allow_once"}

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-full", "call-new", "allow_once"))

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// TestAIPermission_MissingSessionID_Returns400 verifies 400 when the
// X-Session-ID header is absent.
func TestAIPermission_MissingSessionID_Returns400(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	// No session ID header.
	r.ServeHTTP(rr, buildPermissionRequest(t, "", "call-x", "allow_once"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestAIPermission_InvalidDecision_Returns400 verifies 400 for unrecognised
// decision values.
func TestAIPermission_InvalidDecision_Returns400(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-invalid")
	sess.PermChan = make(chan ai.PermissionDecision, 1)

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-invalid", "call-x", "maybe"))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestAIPermission_HandlerIsNonBlocking verifies that the handler returns
// immediately without blocking on the agent goroutine. The permission
// handler uses a non-blocking select so it never hangs regardless of
// agent goroutine state.
func TestAIPermission_HandlerIsNonBlocking(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sess := sm.GetOrCreate(context.Background(), "sess-nonblock")
	sess.PermChan = make(chan ai.PermissionDecision, 1)

	r := makePermissionRouter(sm)
	rr := httptest.NewRecorder()

	// ServeHTTP must return synchronously (handler is non-blocking).
	r.ServeHTTP(rr, buildPermissionRequest(t, "sess-nonblock", "c1", "allow_once"))
	assert.Equal(t, http.StatusNoContent, rr.Code)
}
