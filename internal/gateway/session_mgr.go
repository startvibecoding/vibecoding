package gateway

import (
	"sync"
	"time"

	"github.com/startvibecoding/vibecoding/internal/agent"
	"github.com/startvibecoding/vibecoding/internal/session"
	"github.com/startvibecoding/vibecoding/internal/tools"
)

// GatewaySession holds state for a single gateway session.
type GatewaySession struct {
	ID       string
	WorkDir  string
	Manager  *session.Manager
	Registry *tools.Registry
	AgentMgr *agent.AgentManager // nil unless sub-agents enabled
	Mode     string             // session-level mode override
	LastUsed time.Time
	mu       sync.Mutex // serializes requests within this session

	// ForceCompact is set by /compact command and consumed by the next agent run.
	ForceCompact bool
}

// Lock acquires the session lock (one request at a time per session).
func (s *GatewaySession) Lock()   { s.mu.Lock() }
// Unlock releases the session lock.
func (s *GatewaySession) Unlock() { s.mu.Unlock() }

// Touch updates the last-used timestamp.
func (s *GatewaySession) Touch() { s.LastUsed = time.Now() }

// SessionPool manages multiple concurrent gateway sessions.
type SessionPool struct {
	mu       sync.RWMutex
	sessions map[string]*GatewaySession
	maxSess  int
	idleTTL  time.Duration
	stopCh   chan struct{}
}

// NewSessionPool creates a session pool.
func NewSessionPool(maxSessions int, idleTimeout time.Duration) *SessionPool {
	p := &SessionPool{
		sessions: make(map[string]*GatewaySession),
		maxSess:  maxSessions,
		idleTTL:  idleTimeout,
		stopCh:   make(chan struct{}),
	}
	if idleTimeout > 0 {
		go p.cleanupLoop()
	}
	return p
}

// Get returns an existing session by ID, or nil.
func (p *SessionPool) Get(id string) *GatewaySession {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessions[id]
}

// Put adds a session to the pool. Returns an error if the pool is at capacity.
func (p *SessionPool) Put(s *GatewaySession) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.maxSess > 0 && len(p.sessions) >= p.maxSess {
		// Check if we have an existing entry (replace is OK)
		if _, exists := p.sessions[s.ID]; !exists {
			return &PoolFullError{Max: p.maxSess}
		}
	}
	s.Touch()
	p.sessions[s.ID] = s
	return nil
}

// Remove removes a session by ID.
func (p *SessionPool) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sessions, id)
}

// Count returns the number of active sessions.
func (p *SessionPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sessions)
}

// List returns all session IDs.
func (p *SessionPool) List() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Stop shuts down the cleanup goroutine.
func (p *SessionPool) Stop() {
	close(p.stopCh)
}

// cleanupLoop periodically removes idle sessions.
func (p *SessionPool) cleanupLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.evictIdle()
		}
	}
}

func (p *SessionPool) evictIdle() {
	if p.idleTTL <= 0 {
		return
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, s := range p.sessions {
		if now.Sub(s.LastUsed) > p.idleTTL {
			delete(p.sessions, id)
		}
	}
}

// PoolFullError is returned when the session pool is at capacity.
type PoolFullError struct {
	Max int
}

func (e *PoolFullError) Error() string {
	return "session pool is at capacity"
}
