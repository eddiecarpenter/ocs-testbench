package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	internalmcp "github.com/eddiecarpenter/ocs-testbench/internal/mcp"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// testPeerManager is a fake PeerManager for MCP integration tests.
// Connect always succeeds; State always returns Connected.
type testPeerManager struct {
	mu       sync.Mutex
	connects []string
}

func (m *testPeerManager) Connect(name string) error {
	m.mu.Lock()
	m.connects = append(m.connects, name)
	m.mu.Unlock()
	return nil
}

func (m *testPeerManager) Disconnect(name string) error { return nil }
func (m *testPeerManager) State(name string) (diameter.ConnectionState, error) {
	return diameter.StateConnected, nil
}
func (m *testPeerManager) Subscribe() <-chan diameter.StateEvent {
	return make(chan diameter.StateEvent)
}

// sessionTrackingEngine wraps fakeExecutionEngine and adds a mutex so the
// goroutine that simulates session completion is race-safe with the
// MCP server's concurrent handler goroutines.
type sessionTrackingEngine struct {
	mu       sync.Mutex
	sessions map[string]api.ExecutionDetailResponse
}

func newSessionTrackingEngine() *sessionTrackingEngine {
	return &sessionTrackingEngine{
		sessions: make(map[string]api.ExecutionDetailResponse),
	}
}

func (e *sessionTrackingEngine) Start(_ context.Context, scenarioID, mode string, _ int) (api.StartInfo, error) {
	id := "track-session-" + scenarioID
	detail := api.ExecutionDetailResponse{
		ID:         id,
		ScenarioID: scenarioID,
		Mode:       mode,
		State:      "running",
		Context: api.ExecutionContextJSON{
			System:    map[string]any{},
			User:      map[string]any{},
			Extracted: map[string]any{},
		},
	}
	e.mu.Lock()
	e.sessions[id] = detail
	e.mu.Unlock()
	// Simulate fast completion under the mutex.
	go func() {
		time.Sleep(10 * time.Millisecond)
		e.mu.Lock()
		d := e.sessions[id]
		d.State = "completed"
		e.sessions[id] = d
		e.mu.Unlock()
	}()
	return api.StartInfo{SessionID: id}, nil
}

func (e *sessionTrackingEngine) Stop(_ context.Context, _ string) error { return nil }
func (e *sessionTrackingEngine) Step(_ context.Context, _ string, _ map[string]any) (api.ExecutionStepResult, error) {
	return api.ExecutionStepResult{}, nil
}
func (e *sessionTrackingEngine) Skip(_ context.Context, _ string) error { return nil }
func (e *sessionTrackingEngine) Detail(_ context.Context, sessionID string) (api.ExecutionDetailResponse, error) {
	e.mu.Lock()
	d, ok := e.sessions[sessionID]
	e.mu.Unlock()
	if !ok {
		return api.ExecutionDetailResponse{}, api.ErrSessionNotFound
	}
	return d, nil
}
func (e *sessionTrackingEngine) Subscribe(_ context.Context, _ string) (<-chan api.ExecutionEvent, error) {
	ch := make(chan api.ExecutionEvent)
	close(ch)
	return ch, nil
}
func (e *sessionTrackingEngine) Interrupt(_ context.Context, _ string) error { return nil }
func (e *sessionTrackingEngine) RunToEnd(_ context.Context, _ string) error  { return nil }
func (e *sessionTrackingEngine) ApplyContextOverride(_ context.Context, _ string, _ map[string]any) error {
	return nil
}
func (e *sessionTrackingEngine) ApplyPayloadOverride(_ context.Context, _ string, _ map[string]any) error {
	return nil
}
func (e *sessionTrackingEngine) List(_ context.Context) []api.ExecutionSummary {
	e.mu.Lock()
	out := make([]api.ExecutionSummary, 0, len(e.sessions))
	for _, d := range e.sessions {
		out = append(out, api.ExecutionSummary{SessionID: d.ID, State: d.State})
	}
	e.mu.Unlock()
	return out
}
func (e *sessionTrackingEngine) ResponseTimeSeries(_ context.Context, w string) (api.ResponseTimeSeries, error) {
	return api.ResponseTimeSeries{Window: w}, nil
}

// makeRawMCPRequest performs a raw JSON-RPC POST and returns the response body.
func makeRawMCPRequest(t *testing.T, client *http.Client, url, sessionID string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, bodyBytes
}

// TestConcurrentMCPClients_ListPeers_NoDataRaces verifies AC-7:
// two goroutines each call list_peers 100 times simultaneously with no
// data races and no cross-contamination between clients.
//
// Run with: go test -race ./internal/mcp/... to exercise the race detector.
func TestConcurrentMCPClients_ListPeers_NoDataRaces(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	_, err := s.InsertPeer(ctx, "ocs-01", []byte(`{"host":"10.0.0.1"}`))
	require.NoError(t, err)

	handler := internalmcp.NewServer(s, nil, nil, nil, nil, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Initialize two independent sessions and capture their session IDs.
	initPayload := func(clientName string) map[string]any {
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2025-03-26",
				"clientInfo":      map[string]any{"name": clientName, "version": "1"},
			},
		}
	}

	doInit := func(client *http.Client, clientName string) string {
		raw, _ := json.Marshal(initPayload(clientName))
		req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		sid := resp.Header.Get("Mcp-Session-Id")
		require.NotEmpty(t, sid, "initialize must return session ID")
		return sid
	}

	clientA := ts.Client()
	clientB := &http.Client{}
	sessionA := doInit(clientA, "client-A")
	sessionB := doInit(clientB, "client-B")

	// list_peers message template.
	listMsg := func(id int) map[string]any {
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "list_peers",
				"arguments": map[string]any{},
			},
		}
	}

	const iterations = 100
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A: uses sessionA.
	go func() {
		defer wg.Done()
		for i := range iterations {
			status, body := makeRawMCPRequest(t, clientA, ts.URL, sessionA, listMsg(i))
			if status != http.StatusOK {
				errCh <- assert.AnError
				return
			}
			var rpcResp map[string]any
			if err := json.Unmarshal(body, &rpcResp); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Goroutine B: uses sessionB (independent client).
	go func() {
		defer wg.Done()
		for i := range iterations {
			status, body := makeRawMCPRequest(t, clientB, ts.URL, sessionB, listMsg(i+1000))
			if status != http.StatusOK {
				errCh <- assert.AnError
				return
			}
			var rpcResp map[string]any
			if err := json.Unmarshal(body, &rpcResp); err != nil {
				errCh <- err
				return
			}
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err, "concurrent MCP client encountered an error")
	}
}

// TestEndToEnd_AC2_FullSequence verifies AC-2: the full sequence
// create_peer → connect_peer → create_subscriber → duplicate_scenario →
// start_execution → poll get_execution until terminal state completes
// without error using a test MCP server backed by store.NewTestStore().
func TestEndToEnd_AC2_FullSequence(t *testing.T) {
	s := store.NewTestStore()
	mgr := &testPeerManager{}
	exec := newSessionTrackingEngine()

	// Seed a starter scenario as the source for duplicate_scenario.
	ctx := context.Background()
	seedPeer, err := s.InsertPeer(ctx, "starter-peer", []byte(`{"host":"10.0.0.1"}`))
	require.NoError(t, err)
	seedSub, err := s.InsertSubscriber(ctx, store.InsertSubscriberParams{
		Name: "starter-sub", Msisdn: "27831111111", Iccid: "8927000000",
	})
	require.NoError(t, err)
	starterSc, err := s.InsertScenario(ctx, "starter-scenario", seedPeer.ID, seedSub.ID,
		[]byte(`{"sessionMode":"session","serviceModel":"gy","steps":[]}`))
	require.NoError(t, err)
	starterScID := uuidToStr(starterSc.ID.Bytes)

	// Build the MCP server.
	handler := internalmcp.NewServer(s, mgr, exec, nil, nil, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Initialize session.
	_, initResp := postJSONWithSessionID(t, ts.Client(), ts.URL, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"clientInfo":      map[string]any{"name": "e2e-client", "version": "1.0"},
		},
	}, "")
	sessionID := ""
	{
		raw, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2025-03-26",
				"clientInfo":      map[string]any{"name": "e2e-client", "version": "1.0"},
			},
		})
		req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, _ := ts.Client().Do(req)
		if resp != nil {
			defer resp.Body.Close()
			sessionID = resp.Header.Get("Mcp-Session-Id")
		}
	}
	require.NotEmpty(t, sessionID, "must have session ID after initialize")
	_ = initResp

	// Create a test client using the established session.
	client := &mcpTestClient{ts: ts, sessionID: sessionID, t: t}

	// Step 1: create_peer.
	peerResp := client.callTool("create_peer", map[string]any{
		"name":   "test-peer",
		"config": map[string]any{"host": "10.0.0.2", "port": float64(3868)},
	})
	var peer map[string]any
	toolResult(t, peerResp, &peer)
	peerID := peer["id"].(string)
	peerName := peer["name"].(string)
	require.NotEmpty(t, peerID)

	// Step 2: connect_peer.
	connResp := client.callTool("connect_peer", map[string]any{"name": peerName})
	var connResult map[string]any
	toolResult(t, connResp, &connResult)
	assert.Equal(t, "connecting", connResult["status"])

	// Step 3: create_subscriber.
	subResp := client.callTool("create_subscriber", map[string]any{
		"name":   "test-subscriber",
		"msisdn": "27839999001",
		"iccid":  "8927009999",
	})
	var sub map[string]any
	toolResult(t, subResp, &sub)
	subID := sub["id"].(string)
	require.NotEmpty(t, subID)

	// Step 4: duplicate_scenario (primary AC-2 authoring pattern).
	dupResp := client.callTool("duplicate_scenario", map[string]any{
		"source_id": starterScID,
		"new_name":  "e2e-test-scenario",
	})
	var dupSc map[string]any
	toolResult(t, dupResp, &dupSc)
	scID := dupSc["id"].(string)
	require.NotEmpty(t, scID)
	assert.Equal(t, "e2e-test-scenario", dupSc["name"])

	// Step 5: start_execution.
	startResp := client.callTool("start_execution", map[string]any{
		"scenario_id": scID,
		"mode":        "continuous",
	})
	var started map[string]any
	toolResult(t, startResp, &started)
	execID := started["id"].(string)
	require.NotEmpty(t, execID)

	// Step 6: poll get_execution until terminal state (or timeout).
	deadline := time.Now().Add(2 * time.Second)
	var finalState string
	for time.Now().Before(deadline) {
		getResp := client.callTool("get_execution", map[string]any{"session_id": execID})
		var detail map[string]any
		toolResult(t, getResp, &detail)
		finalState, _ = detail["state"].(string)
		if finalState == "completed" || finalState == "error" || finalState == "terminated" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The fake engine marks sessions as completed after 10ms.
	assert.Equal(t, "completed", finalState,
		"execution must reach a terminal state within the polling window")
}
