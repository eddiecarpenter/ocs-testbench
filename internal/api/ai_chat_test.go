package api_test

import (
	"bufio"
	"bytes"
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
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// readSSEEvents reads lines from a bufio.Scanner and returns the first N
// complete SSE data payloads (lines starting with "data: ").
func readSSEEvents(t *testing.T, scanner *bufio.Scanner, count int, timeout time.Duration) []map[string]any {
	t.Helper()
	deadline := time.After(timeout)
	var events []map[string]any
	var dataLine string
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE events (got %d of %d)", len(events), count)
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
		}
		if line == "" && dataLine != "" {
			var payload map[string]any
			if err := json.Unmarshal([]byte(dataLine), &payload); err == nil {
				events = append(events, payload)
				dataLine = ""
				if len(events) >= count {
					return events
				}
			}
		}
	}
	return events
}

// TestAIChat_ContentTypeTextEventStream verifies that POST /v1/ai/chat
// returns text/event-stream.
func TestAIChat_ContentTypeTextEventStream(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	go r.ServeHTTP(rr, req)

	// Give the handler a moment to write headers.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, "text/event-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rr.Header().Get("Cache-Control"))
}

// TestAIChat_EmitsThinkingEventFirst verifies that the first event from
// the mock is a "thinking" event.
func TestAIChat_EmitsThinkingEventFirst(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 1, 5*time.Second)

	require.GreaterOrEqual(t, len(events), 1)
	assert.Equal(t, "thinking", events[0]["type"])
	data, ok := events[0]["data"].(map[string]any)
	require.True(t, ok, "data field must be an object")
	assert.Contains(t, data["content"], "Analysing")
}

// TestAIChat_EmitsTokenEvents verifies that token events are emitted
// after the thinking event.
func TestAIChat_EmitsTokenEvents(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"test"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read enough events to get past thinking + at least one token.
	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 2, 10*time.Second)

	// Verify at least one token event is present after thinking.
	var tokenCount int
	for _, e := range events {
		if e["type"] == "token" {
			tokenCount++
		}
	}
	assert.GreaterOrEqual(t, tokenCount, 1, "expected at least one token event")
}

// TestAIChat_EmitsPermissionRequiredForWriteTool verifies that a
// permission_required event is emitted for the write-tier tool call.
func TestAIChat_EmitsPermissionRequiredForWriteTool(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"dup"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read enough events to get to the permission_required event.
	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 10, 20*time.Second)

	var foundPermission bool
	for _, e := range events {
		if e["type"] == "permission_required" {
			foundPermission = true
			data, ok := e["data"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "write", data["tier"])
			assert.Equal(t, "duplicate_scenario", data["toolName"])
			break
		}
	}
	assert.True(t, foundPermission, "expected permission_required event for write tool")
}

// TestAIChat_EmitsDoneEvent verifies that the stream terminates with a
// "done" event.
func TestAIChat_EmitsDoneEvent(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	body := bytes.NewBufferString(`{"messages":[{"role":"user","content":"go"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/ai/chat", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read until done event.
	scanner := bufio.NewScanner(resp.Body)
	events := readSSEEvents(t, scanner, 20, 30*time.Second)

	var foundDone bool
	for _, e := range events {
		if e["type"] == "done" {
			foundDone = true
			break
		}
	}
	assert.True(t, foundDone, "expected done event at end of stream")
}

// TestAIChat_ClientDisconnect verifies that the handler exits cleanly
// when the client disconnects mid-stream.
func TestAIChat_ClientDisconnect(t *testing.T) {
	s := store.NewTestStore()
	r := api.Router(s, nil, nil, nil, nil, "test")

	// Cancel the context immediately to simulate a disconnect.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body := bytes.NewBufferString(`{"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/ai/chat", body).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after client disconnect")
	}
}
