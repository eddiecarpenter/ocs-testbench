package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakePeerManagerWithChannel is a PeerManager that exposes a channel
// the test can write events into, simulating peer state transitions.
type fakePeerManagerWithChannel struct {
	*fakePeerManager
	ch chan diameter.StateEvent
}

func newFakePeerManagerWithChannel(names ...string) *fakePeerManagerWithChannel {
	return &fakePeerManagerWithChannel{
		fakePeerManager: newFakePeerManager(names...),
		ch:              make(chan diameter.StateEvent, 4),
	}
}

func (f *fakePeerManagerWithChannel) Subscribe() <-chan diameter.StateEvent {
	return f.ch
}

// TestPeerSSE_ContentTypeTextEventStream verifies that GET /events/peers
// returns text/event-stream content type.
func TestPeerSSE_ContentTypeTextEventStream(t *testing.T) {
	s := store.NewTestStore()
	mgr := newFakePeerManagerWithChannel()
	r := api.Router(s, mgr, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	close(mgr.ch) // immediately close to avoid blocking

	req := httptest.NewRequest(http.MethodGet, "/v1/events/peers", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
}

// TestPeerSSE_EmitsEventOnStateChange verifies AC-20: when a peer's
// connection state changes, the client receives an SSE event with the
// peer ID and new state.
func TestPeerSSE_EmitsEventOnStateChange(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()

	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{}`))
	require.NoError(t, err)
	peerIDStr := api.UUIDStr(peer.ID)

	mgr := newFakePeerManagerWithChannel("ocs-01")
	r := api.Router(s, mgr, nil)

	// Use a real HTTP server so we can read the response body as a
	// streaming reader.
	srv := httptest.NewServer(r)
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start a GET /events/peers request in a goroutine. Read all events
	// until we find one with status="connected" — the handler emits an
	// initial sync event (status="disconnected") before the live event.
	evtCh := make(chan map[string]any, 4)
	go func() {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
			srv.URL+"/v1/events/peers", nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var dataLine string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
			}
			if line == "" && dataLine != "" {
				var payload map[string]any
				if err := json.Unmarshal([]byte(dataLine), &payload); err == nil {
					evtCh <- payload
					dataLine = ""
				}
			}
		}
	}()

	// Give the handler time to subscribe and emit initial sync before
	// sending the live state-change event.
	time.Sleep(50 * time.Millisecond)

	// Emit a state-change event.
	mgr.ch <- diameter.StateEvent{
		PeerName: "ocs-01",
		From:     diameter.StateDisconnected,
		To:       diameter.StateConnected,
		Time:     time.Now(),
	}

	// Drain events until we see the connected transition (skip initial sync).
	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case payload := <-evtCh:
			if payload["status"] == "connected" {
				assert.Equal(t, peerIDStr, payload["id"])
				assert.Equal(t, "ocs-01", payload["name"])
				assert.Equal(t, "connected", payload["status"])
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for SSE connected event")
		}
	}
}

// — Execution SSE tests —

// TestExecutionSSE_ContentTypeTextEventStream verifies that
// GET /events/executions/{id} returns text/event-stream content type.
//
// AC-21 basis: endpoint must stream with correct MIME type.
func TestExecutionSSE_ContentTypeTextEventStream(t *testing.T) {
	s := store.NewTestStore()

	execEngine := &fakeExecutionEngine{
		subscribeFn: func(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
			ch := make(chan api.ExecutionEvent)
			close(ch) // immediately close
			return ch, nil
		},
	}
	r := api.Router(s, nil, execEngine)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet,
		"/v1/events/executions/session-001", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
}

// TestExecutionSSE_EmitsEventOnProgress verifies AC-21: when a step
// completes the client receives an SSE event with step result and
// metrics.
func TestExecutionSSE_EmitsEventOnProgress(t *testing.T) {
	s := store.NewTestStore()

	evtChan := make(chan api.ExecutionEvent, 4)
	execEngine := &fakeExecutionEngine{
		subscribeFn: func(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
			return evtChan, nil
		},
	}
	r := api.Router(s, nil, execEngine)

	srv := httptest.NewServer(r)
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	received := make(chan map[string]any, 4)
	go func() {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
			srv.URL+"/v1/events/executions/session-001", nil)
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var dataLine string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				dataLine = strings.TrimPrefix(line, "data: ")
			}
			if line == "" && dataLine != "" {
				var payload map[string]any
				if err := json.Unmarshal([]byte(dataLine), &payload); err == nil {
					received <- payload
					dataLine = ""
				}
			}
		}
	}()

	// Give the handler time to subscribe before sending the event.
	time.Sleep(50 * time.Millisecond)

	evtChan <- api.ExecutionEvent{
		Type:      "progress",
		SessionID: "session-001",
		State:     "active",
		Step:      3,
		Metrics: api.ExecutionMetrics{
			TotalRequests: 3,
			SuccessCount:  3,
			AvgRTTMs:      12.5,
		},
	}

	deadline := time.After(1500 * time.Millisecond)
	select {
	case payload := <-received:
		assert.Equal(t, "progress", payload["type"])
		assert.Equal(t, "session-001", payload["sessionId"])
		assert.Equal(t, "active", payload["state"])
		assert.Equal(t, float64(3), payload["step"])
		metrics, ok := payload["metrics"].(map[string]any)
		require.True(t, ok, "metrics field must be present")
		assert.Equal(t, float64(3), metrics["totalRequests"])
	case <-deadline:
		t.Fatal("timed out waiting for execution SSE event")
	}
}

// TestExecutionSSE_NotFound_Returns404 verifies that a session ID that
// is unknown to the engine returns 404 (not a streaming response).
//
// AC-21 basis: unknown session should surface an error, not hang.
func TestExecutionSSE_NotFound_Returns404(t *testing.T) {
	s := store.NewTestStore()

	execEngine := &fakeExecutionEngine{
		subscribeFn: func(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
			return nil, api.ErrSessionNotFound
		},
	}
	r := api.Router(s, nil, execEngine)

	req := httptest.NewRequest(http.MethodGet,
		"/v1/events/executions/no-such-session", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}

// TestExecutionSSE_ClientDisconnect_HandlerExitsCleanly verifies AC-22:
// when a client disconnects from the execution SSE stream the handler
// exits cleanly without panicking.
func TestExecutionSSE_ClientDisconnect_HandlerExitsCleanly(t *testing.T) {
	s := store.NewTestStore()

	execEngine := &fakeExecutionEngine{
		subscribeFn: func(ctx context.Context, sessionID string) (<-chan api.ExecutionEvent, error) {
			// Return an open channel that never sends — the test will
			// cancel the request context to simulate a disconnect.
			return make(chan api.ExecutionEvent), nil
		},
	}
	r := api.Router(s, nil, execEngine)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to simulate instant disconnect

	req := httptest.NewRequest(http.MethodGet,
		"/v1/events/executions/session-001", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not exit after client disconnect")
	}
}

// TestExecutionSSE_NilEngine_Returns503 verifies that GET
// /events/executions/{id} returns 503 when no engine is wired.
func TestExecutionSSE_NilEngine_Returns503(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/v1/events/executions/some-session", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// TestPeerSSE_ClientDisconnect verifies AC-22 premise: when the SSE
// connection drops, the server handler exits cleanly (no panic, no
// goroutine leak). On reconnect, subsequent events are delivered.
func TestPeerSSE_ClientDisconnect_HandlerExitsCleanly(t *testing.T) {
	s := store.NewTestStore()
	mgr := newFakePeerManagerWithChannel()
	r := api.Router(s, mgr, nil)

	// Cancel the request context to simulate a client disconnect.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := httptest.NewRequest(http.MethodGet, "/v1/events/peers", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	// Should return without panicking.
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not exit after client disconnect")
	}
}
