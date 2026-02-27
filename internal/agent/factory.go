// Package agent — factory.go provides the AgentFactory for creating child
// agents in the fractal hierarchy. Child agents inherit a subset of the
// parent's properties (specialization baseline, default skillset, policies)
// while getting their own identity, memory references, and soul directory.
package agent

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// Factory creates agents in the fractal hierarchy. It wires parent-child
// relationships and registers new agents in the shared registry.
type Factory struct {
	registry *Registry
	nextID   int // simple monotonic counter; callers may override AgentID.
}

// NewFactory creates a factory bound to the given registry.
func NewFactory(registry *Registry) *Factory {
	return &Factory{
		registry: registry,
		nextID:   1,
	}
}

// SpawnConfig holds parameters for creating a child agent.
type SpawnConfig struct {
	// Required
	Role           string // role descriptor (e.g., "coder", "reviewer", "researcher")
	Specialization string // initial specialization text

	// Optional overrides (zero values = inherit from parent)
	Name              string       // defaults to "<parent.Name>/<Role>"
	DefaultSkillset   []string     // defaults to parent's DefaultSkillset
	ToolAccess        []string     // defaults to parent's ToolAccess
	LLMProviderConfig *LLMProviderConfig
	SafetyPolicy      *SafetyPolicy
	ReviewPolicy      *ReviewPolicy
}

// SpawnChild creates a new child agent under the given parent and registers
// it in both the registry and the parent's Subagents list.
func (f *Factory) SpawnChild(parent *Agent, cfg SpawnConfig) (*Agent, error) {
	if parent == nil {
		return nil, fmt.Errorf("parent agent is nil")
	}
	if cfg.Role == "" {
		return nil, fmt.Errorf("role is required")
	}

	childID := fmt.Sprintf("%s/child-%d", parent.AgentID, f.nextID)
	f.nextID++

	childName := cfg.Name
	if childName == "" {
		childName = fmt.Sprintf("%s/%s", parent.Name, cfg.Role)
	}

	// Inherit skillset from parent if not explicitly set.
	skillset := cfg.DefaultSkillset
	if len(skillset) == 0 && len(parent.DefaultSkillset) > 0 {
		skillset = make([]string, len(parent.DefaultSkillset))
		copy(skillset, parent.DefaultSkillset)
	}

	// Inherit tool access from parent if not explicitly set.
	tools := cfg.ToolAccess
	if len(tools) == 0 && len(parent.ToolAccess) > 0 {
		tools = make([]string, len(parent.ToolAccess))
		copy(tools, parent.ToolAccess)
	}

	child := New(childID, childName)
	child.Specialization = cfg.Specialization
	child.ParentAgentID = parent.AgentID
	child.Level = parent.Level + 1
	child.DefaultSkillset = skillset
	child.ToolAccess = tools
	child.AutomationThreshold = parent.AutomationThreshold

	// Inherit policies from parent when not overridden.
	if cfg.SafetyPolicy != nil {
		child.SafetyPolicy = *cfg.SafetyPolicy
	} else {
		child.SafetyPolicy = parent.SafetyPolicy
	}
	if cfg.ReviewPolicy != nil {
		child.ReviewPolicy = *cfg.ReviewPolicy
	} else {
		child.ReviewPolicy = parent.ReviewPolicy
	}
	if cfg.LLMProviderConfig != nil {
		child.LLMProviderConfig = cfg.LLMProviderConfig
	} else if parent.LLMProviderConfig != nil {
		// shallow copy — Extra map is shared.
		cp := *parent.LLMProviderConfig
		child.LLMProviderConfig = &cp
	}

	// Memory references point to child-specific stores.
	child.MemoryShortTermRef = fmt.Sprintf("mem:short:%s", childID)
	child.MemoryLongTermRef = fmt.Sprintf("mem:long:%s", childID)

	// Register child in the shared registry.
	if err := f.registry.Register(child); err != nil {
		return nil, fmt.Errorf("register child: %w", err)
	}

	// Add to parent's subagent list.
	ref := SubagentRef{
		AgentID:     childID,
		Role:        cfg.Role,
		Description: cfg.Specialization,
	}
	if err := parent.AddSubagent(ref); err != nil {
		// Rollback registry if parent rejects (shouldn't happen, but defensive).
		_ = f.registry.Remove(childID)
		return nil, fmt.Errorf("add subagent ref: %w", err)
	}

	return child, nil
}

// SpawnRoot creates a new top-level (root) agent and registers it.
func (f *Factory) SpawnRoot(name, specialization string) (*Agent, error) {
	id := fmt.Sprintf("agent-%d", f.nextID)
	f.nextID++

	root := New(id, name)
	root.Specialization = specialization
	root.Level = 0
	root.MemoryShortTermRef = fmt.Sprintf("mem:short:%s", id)
	root.MemoryLongTermRef = fmt.Sprintf("mem:long:%s", id)

	if err := f.registry.Register(root); err != nil {
		return nil, fmt.Errorf("register root: %w", err)
	}

	return root, nil
}

// RetireChild removes a child agent and cleans up the parent's reference.
// If recursive=true, also retires all descendants (bottom-up).
func (f *Factory) RetireChild(parent *Agent, childID string, recursive bool) error {
	if parent == nil {
		return fmt.Errorf("parent agent is nil")
	}

	child := f.registry.Get(childID)
	if child == nil {
		return fmt.Errorf("child %q not found in registry", childID)
	}

	// If recursive, retire grandchildren first.
	if recursive {
		for _, sub := range child.Subagents {
			if err := f.RetireChild(child, sub.AgentID, true); err != nil {
				return fmt.Errorf("retire descendant %q: %w", sub.AgentID, err)
			}
		}
	}

	// Remove from parent's subagent list.
	_ = parent.RemoveSubagent(childID) // ignore error if already removed

	// Remove from registry.
	return f.registry.Remove(childID)
}

// Promote detaches a child from its parent and makes it a root agent.
// The child keeps its soul, skills, and memory — it just becomes independent.
func (f *Factory) Promote(childID string) error {
	child := f.registry.Get(childID)
	if child == nil {
		return fmt.Errorf("agent %q not found", childID)
	}
	if child.ParentAgentID == "" {
		return fmt.Errorf("agent %q is already a root", childID)
	}

	// Remove from parent's subagent list.
	parent := f.registry.Get(child.ParentAgentID)
	if parent != nil {
		_ = parent.RemoveSubagent(childID)
	}

	child.ParentAgentID = ""
	child.Level = 0
	child.UpdatedAt = time.Now().UTC()

	// Re-level descendants.
	f.relevelDescendants(childID, 0)

	return nil
}

// relevelDescendants updates Level for all descendants after a promotion.
func (f *Factory) relevelDescendants(parentID string, parentLevel int) {
	children := f.registry.Children(parentID)
	for _, c := range children {
		c.Level = parentLevel + 1
		c.UpdatedAt = time.Now().UTC()
		f.relevelDescendants(c.AgentID, c.Level)
	}
}
