package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/manager"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
)

// fakePeerManager is a minimal PeerManager used in connection-control
// tests. It accepts connect and disconnect calls and records them.
type fakePeerManager struct {
	states    map[string]diameter.ConnectionState
	connectFn func(name string) error
}

func newFakePeerManager(names ...string) *fakePeerManager {
	f := &fakePeerManager{states: make(map[string]diameter.ConnectionState)}
	for _, name := range names {
		f.states[name] = diameter.StateDisconnected
	}
	return f
}

func (f *fakePeerManager) Connect(name string) error {
	if f.connectFn != nil {
		return f.connectFn(name)
	}
	if _, ok := f.states[name]; !ok {
		return fmt.Errorf("%w: %q", manager.ErrUnknownPeer, name)
	}
	f.states[name] = diameter.StateConnected
	return nil
}

func (f *fakePeerManager) Disconnect(name string) error {
	if _, ok := f.states[name]; !ok {
		return fmt.Errorf("%w: %q", manager.ErrUnknownPeer, name)
	}
	f.states[name] = diameter.StateDisconnected
	return nil
}

func (f *fakePeerManager) State(name string) (diameter.ConnectionState, error) {
	s, ok := f.states[name]
	if !ok {
		return diameter.StateDisconnected, fmt.Errorf("%w: %q", manager.ErrUnknownPeer, name)
	}
	return s, nil
}

func (f *fakePeerManager) Subscribe() <-chan diameter.StateEvent {
	ch := make(chan diameter.StateEvent)
	return ch
}

// peerControlFixture holds a store + manager + router for connection
// control tests.
type peerControlFixture struct {
	s      store.Store
	mgr    *fakePeerManager
	r      http.Handler
	peerID string
	peer   store.Peer
}

func newPeerControlFixture(t *testing.T) *peerControlFixture {
	t.Helper()
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "ocs-01", []byte(`{}`))
	require.NoError(t, err)

	mgr := newFakePeerManager("ocs-01")
	r := api.Router(s, mgr)
	return &peerControlFixture{
		s:      s,
		mgr:    mgr,
		r:      r,
		peerID: api.UUIDStr(peer.ID),
		peer:   peer,
	}
}

// TestPeerControl_Connect_DisconnectedPeer_Returns202 tests AC-13
// (connect request returns 202 Accepted).
func TestPeerControl_Connect_DisconnectedPeer_Returns202(t *testing.T) {
	f := newPeerControlFixture(t)

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/v1/peers/%s/connect", f.peerID), nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	// Verify Manager received the Connect call.
	state, err := f.mgr.State("ocs-01")
	require.NoError(t, err)
	assert.Equal(t, diameter.StateConnected, state)
}

// TestPeerControl_Connect_UnknownPeerID_Returns404 verifies that a
// peer ID that doesn't exist in the store returns 404.
func TestPeerControl_Connect_UnknownPeerID_Returns404(t *testing.T) {
	f := newPeerControlFixture(t)

	req := httptest.NewRequest(http.MethodPost,
		"/v1/peers/00000000-0000-0000-0000-000000000099/connect", nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TestPeerControl_Disconnect_ConnectedPeer_Returns200 tests AC-14
// (disconnect request returns 200).
func TestPeerControl_Disconnect_ConnectedPeer_Returns200(t *testing.T) {
	f := newPeerControlFixture(t)
	f.mgr.states["ocs-01"] = diameter.StateConnected

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/v1/peers/%s/disconnect", f.peerID), nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	state, _ := f.mgr.State("ocs-01")
	assert.Equal(t, diameter.StateDisconnected, state)
}

// TestPeerControl_Status_Returns200WithState tests AC-15 (status
// request returns current connection state).
func TestPeerControl_Status_Returns200WithState(t *testing.T) {
	f := newPeerControlFixture(t)
	f.mgr.states["ocs-01"] = diameter.StateConnected

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/v1/peers/%s/status", f.peerID), nil)
	rr := httptest.NewRecorder()
	f.r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "connected", resp["status"])
	assert.Equal(t, f.peerID, resp["id"])
}

// TestPeerControl_Status_UnregisteredInManager_ReturnsStoppedState
// verifies that a peer known to the store but not registered in the
// Manager returns status "stopped" (AC-15 — independent states).
func TestPeerControl_Status_UnregisteredInManager_ReturnsStoppedState(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "unregistered-peer", []byte(`{}`))
	require.NoError(t, err)

	// Manager has no peers registered.
	mgr := newFakePeerManager() // no "unregistered-peer" in states
	r := api.Router(s, mgr)

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/v1/peers/%s/status", api.UUIDStr(peer.ID)), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "stopped", resp["status"])
}

// TestPeerControl_NilManager_Returns503 verifies that a nil Manager
// causes connection-control endpoints to return 503.
func TestPeerControl_NilManager_Returns503(t *testing.T) {
	s := store.NewTestStore()
	ctx := context.Background()
	peer, err := s.InsertPeer(ctx, "test-peer", []byte(`{}`))
	require.NoError(t, err)

	r := api.Router(s, nil) // no manager

	for _, path := range []string{
		fmt.Sprintf("/v1/peers/%s/connect", api.UUIDStr(peer.ID)),
		fmt.Sprintf("/v1/peers/%s/disconnect", api.UUIDStr(peer.ID)),
		fmt.Sprintf("/v1/peers/%s/status", api.UUIDStr(peer.ID)),
	} {
		t.Run(path, func(t *testing.T) {
			method := http.MethodPost
			if path[len(path)-6:] == "status" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		})
	}
}
