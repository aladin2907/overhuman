// Package agent — registry.go provides a thread-safe, hierarchical agent
// registry that supports fractal agent patterns (parent-child relationships,
// lineage queries, specialization lookup).
package agent

import (
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is a thread-safe store for Agent instances.
// It provides hierarchy-aware lookups: children, parent, lineage, roots.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*Agent // agentID → *Agent
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Agent),
	}
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// Register adds an agent. Returns error if an agent with the same ID exists.
func (r *Registry) Register(a *Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[a.AgentID]; ok {
		return fmt.Errorf("agent %q already registered", a.AgentID)
	}
	r.agents[a.AgentID] = a
	return nil
}

// Get retrieves an agent by ID. Returns nil if not found.
func (r *Registry) Get(id string) *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[id]
}

// Remove deletes an agent from the registry.
// It does NOT remove it from the parent's Subagents slice — the caller
// should call parent.RemoveSubagent() separately.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[id]; !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	delete(r.agents, id)
	return nil
}

// ---------------------------------------------------------------------------
// Hierarchy queries
// ---------------------------------------------------------------------------

// Children returns all agents whose ParentAgentID equals the given id.
func (r *Registry) Children(parentID string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Agent
	for _, a := range r.agents {
		if a.ParentAgentID == parentID {
			out = append(out, a)
		}
	}
	return out
}

// Parent returns the parent agent, or nil if the agent is a root.
func (r *Registry) Parent(agentID string) *Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[agentID]
	if !ok || a.ParentAgentID == "" {
		return nil
	}
	return r.agents[a.ParentAgentID]
}

// Lineage returns the ancestry chain from the given agent up to (and
// including) the root. The first element is the agent itself, the last is
// the root. Returns nil if the agent is not found.
func (r *Registry) Lineage(agentID string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[agentID]
	if !ok {
		return nil
	}
	chain := []*Agent{a}
	for a.ParentAgentID != "" {
		p, ok := r.agents[a.ParentAgentID]
		if !ok {
			break
		}
		chain = append(chain, p)
		a = p
	}
	return chain
}

// Roots returns all agents that have no parent (top-level agents).
func (r *Registry) Roots() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Agent
	for _, a := range r.agents {
		if a.ParentAgentID == "" {
			out = append(out, a)
		}
	}
	return out
}

// Descendants returns ALL descendants of the given agent (children,
// grandchildren, etc.) via BFS. Does not include the agent itself.
func (r *Registry) Descendants(agentID string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Agent
	queue := []string{agentID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, a := range r.agents {
			if a.ParentAgentID == cur {
				result = append(result, a)
				queue = append(queue, a.AgentID)
			}
		}
	}
	return result
}

// FindBySpecialization returns all agents whose Specialization contains
// the given substring (case-sensitive).
func (r *Registry) FindBySpecialization(spec string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Agent
	for _, a := range r.agents {
		if a.Specialization == spec {
			out = append(out, a)
		}
	}
	return out
}

// FindByRole returns all agents registered as subagents with the given role
// across the entire registry.
func (r *Registry) FindByRole(role string) []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make(map[string]bool)
	for _, a := range r.agents {
		for _, sub := range a.Subagents {
			if sub.Role == role {
				ids[sub.AgentID] = true
			}
		}
	}
	var out []*Agent
	for id := range ids {
		if a, ok := r.agents[id]; ok {
			out = append(out, a)
		}
	}
	return out
}

// All returns every registered agent.
func (r *Registry) All() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}

// Count returns the number of registered agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// Depth returns the maximum depth of the agent hierarchy (0 = no agents,
// 1 = only roots, 2 = roots with children, etc.).
func (r *Registry) Depth() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	maxLevel := 0
	for _, a := range r.agents {
		if a.Level+1 > maxLevel {
			maxLevel = a.Level + 1
		}
	}
	return maxLevel
}
