package agent

import (
	"testing"
)

func TestFactory_SpawnRoot(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	root, err := f.SpawnRoot("TestAgent", "general")
	if err != nil {
		t.Fatal(err)
	}

	if root.Name != "TestAgent" {
		t.Fatalf("expected TestAgent, got %s", root.Name)
	}
	if root.Specialization != "general" {
		t.Fatalf("expected general, got %s", root.Specialization)
	}
	if root.Level != 0 {
		t.Fatal("root should be level 0")
	}
	if root.ParentAgentID != "" {
		t.Fatal("root should have no parent")
	}
	if reg.Count() != 1 {
		t.Fatal("registry should have 1 agent")
	}
}

func TestFactory_SpawnChild(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	parent.DefaultSkillset = []string{"coding", "writing"}
	parent.ToolAccess = []string{"git", "docker"}
	parent.SafetyPolicy = SafetyPolicy{MaxConcurrentRuns: 5}

	child, err := f.SpawnChild(parent, SpawnConfig{
		Role:           "coder",
		Specialization: "Go development",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check child properties.
	if child.ParentAgentID != parent.AgentID {
		t.Fatal("child should reference parent")
	}
	if child.Level != 1 {
		t.Fatalf("expected level 1, got %d", child.Level)
	}
	if child.Name != "Parent/coder" {
		t.Fatalf("expected Parent/coder, got %s", child.Name)
	}
	if child.Specialization != "Go development" {
		t.Fatal("wrong specialization")
	}

	// Inherited fields.
	if len(child.DefaultSkillset) != 2 {
		t.Fatal("should inherit skillset from parent")
	}
	if len(child.ToolAccess) != 2 {
		t.Fatal("should inherit tool access from parent")
	}
	if child.SafetyPolicy.MaxConcurrentRuns != 5 {
		t.Fatal("should inherit safety policy from parent")
	}

	// Parent should have subagent ref.
	if len(parent.Subagents) != 1 {
		t.Fatal("parent should have 1 subagent")
	}
	if parent.Subagents[0].Role != "coder" {
		t.Fatal("subagent role mismatch")
	}

	if reg.Count() != 2 {
		t.Fatalf("expected 2 agents, got %d", reg.Count())
	}
}

func TestFactory_SpawnChildCustomName(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	child, _ := f.SpawnChild(parent, SpawnConfig{
		Role:           "reviewer",
		Specialization: "code review",
		Name:           "ReviewBot",
	})

	if child.Name != "ReviewBot" {
		t.Fatalf("expected ReviewBot, got %s", child.Name)
	}
}

func TestFactory_SpawnChildOverridePolicies(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	parent.SafetyPolicy = SafetyPolicy{MaxConcurrentRuns: 10}
	parent.ReviewPolicy = ReviewPolicy{Enabled: true, MinScore: 0.7}

	customSafety := SafetyPolicy{MaxConcurrentRuns: 2, RequireApproval: true}
	customReview := ReviewPolicy{Enabled: false}

	child, _ := f.SpawnChild(parent, SpawnConfig{
		Role:           "tester",
		Specialization: "testing",
		SafetyPolicy:   &customSafety,
		ReviewPolicy:   &customReview,
	})

	if child.SafetyPolicy.MaxConcurrentRuns != 2 {
		t.Fatal("safety policy should be overridden")
	}
	if !child.SafetyPolicy.RequireApproval {
		t.Fatal("require approval should be true")
	}
	if child.ReviewPolicy.Enabled {
		t.Fatal("review policy should be disabled")
	}
}

func TestFactory_SpawnChildNilParent(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)
	_, err := f.SpawnChild(nil, SpawnConfig{Role: "x", Specialization: "x"})
	if err == nil {
		t.Fatal("expected error for nil parent")
	}
}

func TestFactory_SpawnChildEmptyRole(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)
	parent, _ := f.SpawnRoot("P", "g")
	_, err := f.SpawnChild(parent, SpawnConfig{Specialization: "x"})
	if err == nil {
		t.Fatal("expected error for empty role")
	}
}

func TestFactory_SpawnChildInheritsLLMConfig(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("P", "g")
	parent.LLMProviderConfig = &LLMProviderConfig{
		Provider:    "claude",
		Model:       "sonnet",
		MaxTokens:   8192,
		Temperature: 0.3,
	}

	child, _ := f.SpawnChild(parent, SpawnConfig{
		Role:           "worker",
		Specialization: "tasks",
	})

	if child.LLMProviderConfig == nil {
		t.Fatal("child should inherit LLM config")
	}
	if child.LLMProviderConfig.Model != "sonnet" {
		t.Fatal("wrong inherited model")
	}
	// Should be a copy, not the same pointer.
	if child.LLMProviderConfig == parent.LLMProviderConfig {
		t.Fatal("LLM config should be a copy, not same pointer")
	}
}

func TestFactory_RetireChild(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	child, _ := f.SpawnChild(parent, SpawnConfig{
		Role:           "worker",
		Specialization: "tasks",
	})

	if err := f.RetireChild(parent, child.AgentID, false); err != nil {
		t.Fatal(err)
	}

	if reg.Get(child.AgentID) != nil {
		t.Fatal("child should be removed from registry")
	}
	if len(parent.Subagents) != 0 {
		t.Fatal("parent should have no subagents")
	}
}

func TestFactory_RetireChildRecursive(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	root, _ := f.SpawnRoot("Root", "general")
	child, _ := f.SpawnChild(root, SpawnConfig{Role: "mid", Specialization: "mid"})
	grandchild, _ := f.SpawnChild(child, SpawnConfig{Role: "leaf", Specialization: "leaf"})

	if reg.Count() != 3 {
		t.Fatalf("expected 3, got %d", reg.Count())
	}

	if err := f.RetireChild(root, child.AgentID, true); err != nil {
		t.Fatal(err)
	}

	if reg.Get(grandchild.AgentID) != nil {
		t.Fatal("grandchild should be removed")
	}
	if reg.Get(child.AgentID) != nil {
		t.Fatal("child should be removed")
	}
	if reg.Count() != 1 {
		t.Fatalf("expected 1 (root), got %d", reg.Count())
	}
}

func TestFactory_RetireChildNotFound(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("P", "g")
	if err := f.RetireChild(parent, "nonexistent", false); err == nil {
		t.Fatal("expected error for nonexistent child")
	}
}

func TestFactory_Promote(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	child, _ := f.SpawnChild(parent, SpawnConfig{Role: "worker", Specialization: "tasks"})

	if err := f.Promote(child.AgentID); err != nil {
		t.Fatal(err)
	}

	if child.ParentAgentID != "" {
		t.Fatal("promoted child should have no parent")
	}
	if child.Level != 0 {
		t.Fatal("promoted child should be level 0")
	}
	if len(parent.Subagents) != 0 {
		t.Fatal("parent should have no subagents after promotion")
	}
	// Both should be roots now.
	roots := reg.Roots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}

func TestFactory_PromoteAlreadyRoot(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	root, _ := f.SpawnRoot("Root", "g")
	if err := f.Promote(root.AgentID); err == nil {
		t.Fatal("expected error promoting a root")
	}
}

func TestFactory_PromoteNotFound(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)
	if err := f.Promote("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_PromoteRelevelDescendants(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	root, _ := f.SpawnRoot("Root", "g")
	child, _ := f.SpawnChild(root, SpawnConfig{Role: "mid", Specialization: "m"})
	grandchild, _ := f.SpawnChild(child, SpawnConfig{Role: "leaf", Specialization: "l"})

	if grandchild.Level != 2 {
		t.Fatalf("expected level 2, got %d", grandchild.Level)
	}

	// Promote child → it becomes root (level 0), grandchild → level 1.
	if err := f.Promote(child.AgentID); err != nil {
		t.Fatal(err)
	}

	if child.Level != 0 {
		t.Fatalf("expected level 0, got %d", child.Level)
	}
	if grandchild.Level != 1 {
		t.Fatalf("expected grandchild level 1, got %d", grandchild.Level)
	}
}

func TestFactory_MultipleChildren(t *testing.T) {
	reg := NewRegistry()
	f := NewFactory(reg)

	parent, _ := f.SpawnRoot("Parent", "general")
	_, _ = f.SpawnChild(parent, SpawnConfig{Role: "coder", Specialization: "go"})
	_, _ = f.SpawnChild(parent, SpawnConfig{Role: "tester", Specialization: "testing"})
	_, _ = f.SpawnChild(parent, SpawnConfig{Role: "reviewer", Specialization: "review"})

	if len(parent.Subagents) != 3 {
		t.Fatalf("expected 3 subagents, got %d", len(parent.Subagents))
	}
	if reg.Count() != 4 {
		t.Fatalf("expected 4 agents total, got %d", reg.Count())
	}

	children := reg.Children(parent.AgentID)
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}
