package ai

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// sessionTTL is the idle duration after which a session is evicted.
	sessionTTL = 30 * time.Minute
	// cleanupInterval is how often the TTL sweep runs.
	cleanupInterval = 5 * time.Minute
)

// Session holds per-session conversation state for a single browser tab.
//
// History accumulates messages across multiple POST /v1/ai/chat calls.
// PermChan receives permission decisions for in-flight write-tier tool
// calls; it is created lazily on the first tool call that requires
// approval and closed when the agent goroutine exits.
// AllowAlways records tool names that the operator approved permanently
// for the lifetime of this session.
// LastAccess is updated by the session manager on every GetOrCreate call;
// the TTL sweep uses it to evict idle sessions.
//
// Per-session mutation is single-goroutine (one HTTP handler at a time
// per session ID) so the fields do not need a dedicated mutex; the
// session map itself is protected by SessionManager's RWMutex.
type Session struct {
	History     []Message
	PermChan    chan PermissionDecision
	AllowAlways map[string]bool
	LastAccess  time.Time
}

// SessionManager is a goroutine-safe in-memory store of active sessions
// keyed by session ID. It starts a background TTL-cleanup goroutine that
// evicts sessions idle for more than sessionTTL.
//
// SessionManager is safe for concurrent use.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stopCh   chan struct{}
}

// NewSessionManager creates a new SessionManager and starts its
// TTL-cleanup goroutine.
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
	}
	go sm.cleanupLoop()
	return sm
}

// GenerateSessionID returns a new random session ID. Called by the HTTP
// handler when the X-Session-ID header is absent.
func GenerateSessionID() string {
	return uuid.New().String()
}

// GetOrCreate returns the existing Session for sessionID, or creates a new
// one if it does not exist. It updates LastAccess on every call.
func (sm *SessionManager) GetOrCreate(sessionID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[sessionID]
	if !ok {
		s = &Session{
			AllowAlways: make(map[string]bool),
		}
		sm.sessions[sessionID] = s
	}
	s.LastAccess = time.Now()
	return s
}

// Get returns the Session for sessionID, or nil if the session does not
// exist. It does NOT update LastAccess — use for read-only lookups such
// as the permission endpoint.
func (sm *SessionManager) Get(sessionID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[sessionID]
}

// UpdateHistory replaces the conversation history for sessionID. If the
// session does not exist it is created.
func (sm *SessionManager) UpdateHistory(sessionID string, messages []Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[sessionID]
	if !ok {
		s = &Session{
			AllowAlways: make(map[string]bool),
		}
		sm.sessions[sessionID] = s
	}
	s.History = messages
	s.LastAccess = time.Now()
}

// Stop terminates the background cleanup goroutine. Call during graceful
// shutdown to avoid goroutine leaks in tests and production.
func (sm *SessionManager) Stop() {
	close(sm.stopCh)
}

// cleanupLoop runs on a background goroutine and evicts sessions that
// have been idle for longer than sessionTTL.
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.evictExpired(time.Now())
		}
	}
}

// evictExpired removes sessions whose LastAccess is older than ttl before now.
// It is called by cleanupLoop and is also exported (via evictExpiredAt) for
// testing with an injected clock.
func (sm *SessionManager) evictExpired(now time.Time) {
	threshold := now.Add(-sessionTTL)
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for id, s := range sm.sessions {
		if s.LastAccess.Before(threshold) {
			delete(sm.sessions, id)
		}
	}
}
