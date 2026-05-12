package api_test

import (
	"context"
	"testing"

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
