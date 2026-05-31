package agent

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	agentpkg "github.com/startvibecoding/vibecoding/agent"
)

// ManagedAgentStatus captures scheduling state for an agent managed by AgentManager.
type ManagedAgentStatus struct {
	ID        agentpkg.AgentID
	ParentID  agentpkg.AgentID
	State     string
	Result    string
	Error     string
	StartedAt time.Time
	UpdatedAt time.Time
}

// AgentManager manages the lifecycle of all agent instances.
type AgentManager struct {
	mu       sync.RWMutex
	agents   map[agentpkg.AgentID]agentpkg.Agent
	parentOf map[agentpkg.AgentID]agentpkg.AgentID
	children map[agentpkg.AgentID][]agentpkg.AgentID
	statuses map[agentpkg.AgentID]ManagedAgentStatus
	factory  *AgentFactory
	counter  int64
}

// NewAgentManager creates a new agent manager.
func NewAgentManager(factory *AgentFactory) *AgentManager {
	return &AgentManager{
		agents:   make(map[agentpkg.AgentID]agentpkg.Agent),
		parentOf: make(map[agentpkg.AgentID]agentpkg.AgentID),
		children: make(map[agentpkg.AgentID][]agentpkg.AgentID),
		statuses: make(map[agentpkg.AgentID]ManagedAgentStatus),
		factory:  factory,
	}
}

// Register adds an already-created top-level agent to the manager.
func (m *AgentManager) Register(a agentpkg.Agent) {
	if a == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	id := a.ID()
	m.agents[id] = a
	if a.ParentID() != "" {
		m.parentOf[id] = a.ParentID()
		m.children[a.ParentID()] = appendUniqueAgentID(m.children[a.ParentID()], id)
	}
	now := time.Now()
	m.statuses[id] = ManagedAgentStatus{
		ID:        id,
		ParentID:  a.ParentID(),
		State:     "ready",
		StartedAt: now,
		UpdatedAt: now,
	}
}

// Create creates a new agent and registers it.
// If opts.ParentID is set, validates the parent exists and is a top-level agent.
func (m *AgentManager) Create(opts AgentOptions) (agentpkg.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided
	if opts.ID == "" {
		opts.ID = agentpkg.AgentID(fmt.Sprintf("agent-%d", atomic.AddInt64(&m.counter, 1)))
	}
	if opts.Mode == "" {
		opts.Mode = "agent"
	}

	// Validate parent
	if opts.ParentID != "" {
		parent, ok := m.agents[opts.ParentID]
		if !ok {
			return nil, fmt.Errorf("parent agent %s not found", opts.ParentID)
		}
		// Decision 5: sub-agents cannot nest (only top-level agents can spawn)
		if parent.ParentID() != "" {
			return nil, fmt.Errorf("parent agent %s is itself a sub-agent; nesting is not allowed", opts.ParentID)
		}
		policy := DefaultSubAgentPolicy()
		if err := policy.Validate(string(opts.ParentID), opts.Mode, len(m.children[opts.ParentID])); err != nil {
			return nil, err
		}
	}

	a := m.factory.Create(opts)
	m.agents[opts.ID] = a
	if opts.ParentID != "" {
		m.parentOf[opts.ID] = opts.ParentID
		m.children[opts.ParentID] = append(m.children[opts.ParentID], opts.ID)
	}
	now := time.Now()
	m.statuses[opts.ID] = ManagedAgentStatus{
		ID:        opts.ID,
		ParentID:  opts.ParentID,
		State:     "ready",
		StartedAt: now,
		UpdatedAt: now,
	}

	return a, nil
}

// Get returns an agent by ID.
func (m *AgentManager) Get(id agentpkg.AgentID) (agentpkg.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[id]
	return a, ok
}

// Destroy stops and removes an agent and all its children.
func (m *AgentManager) Destroy(id agentpkg.AgentID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[id]
	if !ok {
		return fmt.Errorf("agent %s not found", id)
	}

	// Recursively destroy children first
	children := m.children[id]
	for _, childID := range children {
		m.destroyLocked(childID)
	}

	// Abort the agent
	a.Abort()

	// Remove from parent's children list
	if parentID, hasParent := m.parentOf[id]; hasParent {
		siblings := m.children[parentID]
		filtered := make([]agentpkg.AgentID, 0, len(siblings))
		for _, sid := range siblings {
			if sid != id {
				filtered = append(filtered, sid)
			}
		}
		m.children[parentID] = filtered
	}

	// Remove self
	delete(m.agents, id)
	delete(m.parentOf, id)
	delete(m.children, id)
	delete(m.statuses, id)

	return nil
}

// destroyLocked destroys an agent without locking (caller must hold lock).
func (m *AgentManager) destroyLocked(id agentpkg.AgentID) {
	// Destroy children recursively
	for _, childID := range m.children[id] {
		m.destroyLocked(childID)
	}
	if a, ok := m.agents[id]; ok {
		a.Abort()
	}
	delete(m.agents, id)
	delete(m.parentOf, id)
	delete(m.children, id)
	delete(m.statuses, id)
}

// MarkRunning records that an agent has started processing a task.
func (m *AgentManager) MarkRunning(id agentpkg.AgentID) {
	m.updateStatus(id, "running", "", "")
}

// MarkDone records successful completion and the last reported result.
func (m *AgentManager) MarkDone(id agentpkg.AgentID, result string) {
	m.updateStatus(id, "done", result, "")
}

// MarkError records an agent failure.
func (m *AgentManager) MarkError(id agentpkg.AgentID, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	m.updateStatus(id, "error", "", msg)
}

func (m *AgentManager) updateStatus(id agentpkg.AgentID, state, result, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.statuses[id]
	st.ID = id
	if st.StartedAt.IsZero() {
		st.StartedAt = time.Now()
	}
	if parentID, ok := m.parentOf[id]; ok {
		st.ParentID = parentID
	}
	st.State = state
	if result != "" {
		st.Result = result
	}
	if errMsg != "" {
		st.Error = errMsg
	}
	st.UpdatedAt = time.Now()
	m.statuses[id] = st
}

// Status returns a copy of the tracked status for an agent.
func (m *AgentManager) Status(id agentpkg.AgentID) (ManagedAgentStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.statuses[id]
	return st, ok
}

// List returns all agent IDs.
func (m *AgentManager) List() []agentpkg.AgentID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]agentpkg.AgentID, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids
}

func appendUniqueAgentID(ids []agentpkg.AgentID, id agentpkg.AgentID) []agentpkg.AgentID {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

// Children returns the children of an agent.
func (m *AgentManager) Children(id agentpkg.AgentID) []agentpkg.AgentID {
	m.mu.RLock()
	defer m.mu.RUnlock()
	children := m.children[id]
	if children == nil {
		return nil
	}
	result := make([]agentpkg.AgentID, len(children))
	copy(result, children)
	return result
}

// Parent returns the parent ID of an agent.
func (m *AgentManager) Parent(id agentpkg.AgentID) (agentpkg.AgentID, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pid, ok := m.parentOf[id]
	return pid, ok
}

// Count returns the number of active agents.
func (m *AgentManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}
