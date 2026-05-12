package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/api"
	"github.com/eddiecarpenter/ocs-testbench/internal/diameter/messaging"
	"github.com/eddiecarpenter/ocs-testbench/internal/store"
	tmpl "github.com/eddiecarpenter/ocs-testbench/internal/template"
)

// fakeSender is a minimal messaging.Sender for unit tests.
type fakeSender struct{}

func (f *fakeSender) Send(_ context.Context, _ string, _ *messaging.CCR) (*messaging.CCA, error) {
	return nil, nil
}

// fakeDictionary is a minimal template.Dictionary for unit tests.
type fakeDictionary struct{}

func (f *fakeDictionary) Lookup(name string) (tmpl.AVPMetadata, error) {
	return tmpl.AVPMetadata{Code: 999}, nil
}

// newTestSessionManager creates a SessionManager backed by an in-memory store.
func newTestSessionManager() *api.SessionManager {
	return api.NewSessionManager(
		store.NewTestStore(),
		newFakePeerManager(),
		&fakeSender{},
		&fakeDictionary{},
	)
}

// TestApplyContextOverride_SessionNotFound_ReturnsErr verifies that
// ApplyContextOverride returns ErrSessionNotFound for unknown sessions.
func TestApplyContextOverride_SessionNotFound_ReturnsErr(t *testing.T) {
	sm := newTestSessionManager()
	err := sm.ApplyContextOverride(context.Background(), "no-such-session", map[string]any{"K": "V"})
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrSessionNotFound, "unknown session must return ErrSessionNotFound")
}

// TestApplyPayloadOverride_SessionNotFound_ReturnsErr verifies that
// ApplyPayloadOverride returns ErrSessionNotFound for unknown sessions.
func TestApplyPayloadOverride_SessionNotFound_ReturnsErr(t *testing.T) {
	sm := newTestSessionManager()
	err := sm.ApplyPayloadOverride(context.Background(), "no-such-session", map[string]any{"K": "V"})
	require.Error(t, err)
	assert.ErrorIs(t, err, api.ErrSessionNotFound, "unknown session must return ErrSessionNotFound")
}

// TestApplyContextOverride_PersistsIntoDetail verifies AC-5 at the engine level:
// after calling ApplyContextOverride on a paused interactive session, the
// overridden variable must appear in the context returned by Detail.
//
// A nil PeerManager bypasses the connectivity gate in SessionManager.Start,
// allowing the test to run without a Diameter peer.
func TestApplyContextOverride_PersistsIntoDetail(t *testing.T) {
	ctx := context.Background()
	s := store.NewTestStore()
	// nil PeerManager: session_manager.go skips the connectivity check when mgr == nil,
	// letting unit tests exercise the override write path without a live peer.
	sm := api.NewSessionManager(s, nil, &fakeSender{}, &fakeDictionary{})

	// Seed a peer with the minimal identity body required by loadPeerInfo.
	peerBody := []byte(`{"originHost":"test.ocs.local","originRealm":"ocs.local","destRealm":"ocs.local"}`)
	peer, err := s.InsertPeer(ctx, "test-override-peer", peerBody)
	require.NoError(t, err, "InsertPeer must succeed")

	// Seed a minimal scenario linked to the peer. No subscriber needed.
	// The body must decode as scenarioBodyExec; empty JSON arrays satisfy each field.
	scenBody := []byte(`{
		"serviceModel":"root",
		"serviceContextId":"32251@3gpp.org",
		"serviceType":"DATA",
		"avpTree":[],
		"services":[],
		"variables":[],
		"steps":[{"kind":"request","requestType":"INITIAL"}]
	}`)
	scenario, err := s.InsertScenario(ctx, "test-override-scenario", peer.ID, pgtype.UUID{}, scenBody)
	require.NoError(t, err, "InsertScenario must succeed")

	// Start an interactive session. Interactive mode starts paused (per session_manager.go).
	scenarioIDStr := uuid.UUID(scenario.ID.Bytes).String()
	info, err := sm.Start(ctx, scenarioIDStr, "interactive", 0)
	require.NoError(t, err, "Start must succeed with a seeded scenario and nil PeerManager")
	sessionID := info.SessionID

	// Apply context override — succeeds because the session is paused.
	err = sm.ApplyContextOverride(ctx, sessionID, map[string]any{"MY_OVERRIDE": "overridden"})
	require.NoError(t, err, "ApplyContextOverride must succeed on a paused interactive session")

	// Detail must reflect the override in context.user (non-system vars map to the User bucket).
	detail, err := sm.Detail(ctx, sessionID)
	require.NoError(t, err, "Detail must succeed for a known session")
	assert.Equal(t, "overridden", detail.Context.User["MY_OVERRIDE"],
		"ApplyContextOverride must persist the variable into the session context visible via Detail")
}
