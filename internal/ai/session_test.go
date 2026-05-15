package ai_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eddiecarpenter/ocs-testbench/internal/ai"
)

// TestSessionManager_CreateNewSession verifies that GetOrCreate creates a
// new session when the session ID is unknown.
func TestSessionManager_CreateNewSession(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	s := sm.GetOrCreate(context.Background(), "sess-1")
	require.NotNil(t, s)
	assert.NotNil(t, s.AllowAlways, "AllowAlways must be initialised")
	assert.Empty(t, s.History, "new session must have empty history")
}

// TestSessionManager_GetExistingSession verifies that GetOrCreate returns
// the same Session on subsequent calls with the same ID.
func TestSessionManager_GetExistingSession(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	s1 := sm.GetOrCreate(context.Background(), "sess-same")
	s1.History = []ai.Message{{Role: "user", Content: "hello"}}

	s2 := sm.GetOrCreate(context.Background(), "sess-same")
	require.Equal(t, s1, s2, "same session must be returned for same ID")
	assert.Len(t, s2.History, 1)
}

// TestSessionManager_GetNotExist verifies that Get returns nil for unknown IDs.
func TestSessionManager_GetNotExist(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	s := sm.Get("no-such-session")
	assert.Nil(t, s)
}

// TestSessionManager_Get_ReturnsExistingSession verifies that Get returns the
// session when it exists without creating a new one.
func TestSessionManager_Get_ReturnsExistingSession(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sm.GetOrCreate(context.Background(), "existing-sess")
	s := sm.Get("existing-sess")
	require.NotNil(t, s)
}

// TestSessionManager_UpdateHistory_ReplacesHistory verifies that
// UpdateHistory replaces the existing conversation history.
func TestSessionManager_UpdateHistory_ReplacesHistory(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sm.GetOrCreate(context.Background(), "sess-update")
	newHistory := []ai.Message{
		{Role: "user", Content: "query"},
		{Role: "assistant", Content: "response"},
	}
	sm.UpdateHistory("sess-update", newHistory)

	s := sm.Get("sess-update")
	require.NotNil(t, s)
	assert.Equal(t, newHistory, s.History)
}

// TestSessionManager_UpdateHistory_CreatesSession verifies that
// UpdateHistory creates a session when it does not exist.
func TestSessionManager_UpdateHistory_CreatesSession(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	msgs := []ai.Message{{Role: "user", Content: "hi"}}
	sm.UpdateHistory("new-sess", msgs)

	s := sm.Get("new-sess")
	require.NotNil(t, s)
	assert.Equal(t, msgs, s.History)
}

// TestSessionManager_ConcurrentAccess verifies that concurrent GetOrCreate
// and UpdateHistory calls do not race.
func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()
			id := "concurrent-sess"
			_ = sm.GetOrCreate(context.Background(), id)
			sm.UpdateHistory(id, []ai.Message{{Role: "user", Content: "msg"}})
			_ = sm.Get(id)
		}(i)
	}
	wg.Wait()
	// If we get here without a data-race report (run with -race), the test passes.
}

// TestSessionManager_TTLEviction verifies that sessions idle beyond the TTL
// are evicted by the cleanup logic.
func TestSessionManager_TTLEviction(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	// Create a session.
	sm.GetOrCreate(context.Background(), "evict-me")

	// Trigger eviction with a "now" that is 31 minutes in the future,
	// which puts the session past the 30-minute TTL.
	ai.EvictExpiredAt(sm, time.Now().Add(31*time.Minute))

	s := sm.Get("evict-me")
	assert.Nil(t, s, "session should have been evicted after TTL")
}

// TestSessionManager_TTLEviction_KeepsRecentSession verifies that a session
// accessed recently is NOT evicted.
func TestSessionManager_TTLEviction_KeepsRecentSession(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	sm.GetOrCreate(context.Background(), "keep-me")

	// Trigger eviction with a "now" only 5 minutes in the future — well
	// within the 30-minute TTL.
	ai.EvictExpiredAt(sm, time.Now().Add(5*time.Minute))

	s := sm.Get("keep-me")
	assert.NotNil(t, s, "recently accessed session must NOT be evicted")
}

// TestSessionManager_AllowAlwaysRegistration verifies that AllowAlways can
// be populated and read correctly.
func TestSessionManager_AllowAlwaysRegistration(t *testing.T) {
	sm := ai.NewSessionManager(nil)
	defer sm.Stop()

	s := sm.GetOrCreate(context.Background(), "allow-always-sess")
	s.AllowAlways["list_peers"] = true

	s2 := sm.GetOrCreate(context.Background(), "allow-always-sess")
	assert.True(t, s2.AllowAlways["list_peers"], "AllowAlways entry must persist across GetOrCreate calls")
	assert.False(t, s2.AllowAlways["unknown_tool"])
}

// TestGenerateSessionID_Unique verifies that GenerateSessionID returns
// unique IDs on each call.
func TestGenerateSessionID_Unique(t *testing.T) {
	ids := make(map[string]bool, 100)
	for range 100 {
		id := ai.GenerateSessionID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "session ID must be unique")
		ids[id] = true
	}
}
