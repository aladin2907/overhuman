package agent

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 1. Creating a new agent with defaults
// ---------------------------------------------------------------------------

func TestNew_Defaults(t *testing.T) {
	a := New("agent-001", "Alpha")

	// 7.1 Identity
	if a.AgentID != "agent-001" {
		t.Fatalf("AgentID = %q, want %q", a.AgentID, "agent-001")
	}
	if a.Name != "Alpha" {
		t.Fatalf("Name = %q, want %q", a.Name, "Alpha")
	}
	if a.Specialization != "" {
		t.Fatalf("Specialization should be empty for a new agent, got %q", a.Specialization)
	}
	if len(a.SpecializationHistory) != 0 {
		t.Fatalf("SpecializationHistory should be empty, got %d entries", len(a.SpecializationHistory))
	}

	// 7.2 Hierarchy
	if len(a.Subagents) != 0 {
		t.Fatalf("Subagents should be empty")
	}
	if a.ParentAgentID != "" {
		t.Fatalf("ParentAgentID should be empty")
	}
	if a.Level != 0 {
		t.Fatalf("Level should be 0")
	}

	// 7.3 Skills
	if len(a.Skills) != 0 {
		t.Fatalf("Skills should be empty")
	}
	if len(a.DefaultSkillset) != 0 {
		t.Fatalf("DefaultSkillset should be empty")
	}

	// 7.4 Memory & Experience
	if len(a.RunHistory) != 0 {
		t.Fatalf("RunHistory should be empty")
	}
	if len(a.FeedbackHistory) != 0 {
		t.Fatalf("FeedbackHistory should be empty")
	}

	// 7.5 Pattern Statistics
	if len(a.TaskPatterns) != 0 {
		t.Fatalf("TaskPatterns should be empty")
	}
	if a.PatternCounts == nil {
		t.Fatalf("PatternCounts map should be initialised")
	}

	// 7.7 Policies & Triggers
	if a.AutomationThreshold != DefaultAutomationThreshold {
		t.Fatalf("AutomationThreshold = %d, want %d", a.AutomationThreshold, DefaultAutomationThreshold)
	}
	if DefaultAutomationThreshold != 3 {
		t.Fatalf("DefaultAutomationThreshold = %d, want 3", DefaultAutomationThreshold)
	}

	// 7.8 Interfaces
	if len(a.ToolAccess) != 0 {
		t.Fatalf("ToolAccess should be empty")
	}

	// Timestamps
	if a.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should not be zero")
	}
	if a.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should not be zero")
	}
}

// ---------------------------------------------------------------------------
// 2. Adding / removing subagents
// ---------------------------------------------------------------------------

func TestAddSubagent(t *testing.T) {
	a := New("root", "Root")

	ref := SubagentRef{AgentID: "sub-1", Role: "coder", Description: "writes code"}
	if err := a.AddSubagent(ref); err != nil {
		t.Fatalf("AddSubagent: unexpected error: %v", err)
	}
	if len(a.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(a.Subagents))
	}
	if a.Subagents[0].AgentID != "sub-1" {
		t.Fatalf("subagent AgentID = %q, want %q", a.Subagents[0].AgentID, "sub-1")
	}
}

func TestAddSubagent_Duplicate(t *testing.T) {
	a := New("root", "Root")

	ref := SubagentRef{AgentID: "sub-1", Role: "coder", Description: "writes code"}
	_ = a.AddSubagent(ref)

	if err := a.AddSubagent(ref); err == nil {
		t.Fatal("AddSubagent should return an error for a duplicate agent_id")
	}
}

func TestRemoveSubagent(t *testing.T) {
	a := New("root", "Root")

	_ = a.AddSubagent(SubagentRef{AgentID: "sub-1", Role: "coder", Description: "writes code"})
	_ = a.AddSubagent(SubagentRef{AgentID: "sub-2", Role: "reviewer", Description: "reviews code"})

	if err := a.RemoveSubagent("sub-1"); err != nil {
		t.Fatalf("RemoveSubagent: unexpected error: %v", err)
	}
	if len(a.Subagents) != 1 {
		t.Fatalf("expected 1 subagent after removal, got %d", len(a.Subagents))
	}
	if a.Subagents[0].AgentID != "sub-2" {
		t.Fatalf("remaining subagent should be sub-2, got %q", a.Subagents[0].AgentID)
	}
}

func TestRemoveSubagent_NotFound(t *testing.T) {
	a := New("root", "Root")
	if err := a.RemoveSubagent("nonexistent"); err == nil {
		t.Fatal("RemoveSubagent should return an error when subagent not found")
	}
}

// ---------------------------------------------------------------------------
// 3. Adding skills
// ---------------------------------------------------------------------------

func TestAddSkill(t *testing.T) {
	a := New("agent-001", "Alpha")

	skill := SkillRef{
		SkillID:     "skill-go-codegen",
		Type:        SkillTypeLLM,
		Description: "Go code generation",
		Version:     "1.0.0",
		Status:      SkillStatusActive,
	}
	if err := a.AddSkill(skill); err != nil {
		t.Fatalf("AddSkill: unexpected error: %v", err)
	}
	if len(a.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(a.Skills))
	}
	if a.Skills[0].Type != SkillTypeLLM {
		t.Fatalf("skill Type = %q, want %q", a.Skills[0].Type, SkillTypeLLM)
	}
	if a.Skills[0].Status != SkillStatusActive {
		t.Fatalf("skill Status = %q, want %q", a.Skills[0].Status, SkillStatusActive)
	}
}

func TestAddSkill_Duplicate(t *testing.T) {
	a := New("agent-001", "Alpha")

	skill := SkillRef{SkillID: "skill-1", Type: SkillTypeCode, Description: "test", Version: "1.0", Status: SkillStatusTrial}
	_ = a.AddSkill(skill)

	if err := a.AddSkill(skill); err == nil {
		t.Fatal("AddSkill should return an error for a duplicate skill_id")
	}
}

func TestSkillTypes(t *testing.T) {
	// Ensure all enum values are distinct and non-empty.
	types := []SkillType{SkillTypeLLM, SkillTypeCode, SkillTypeHybrid}
	seen := make(map[SkillType]bool)
	for _, st := range types {
		if st == "" {
			t.Fatal("SkillType value must not be empty")
		}
		if seen[st] {
			t.Fatalf("duplicate SkillType value: %q", st)
		}
		seen[st] = true
	}
}

func TestSkillStatuses(t *testing.T) {
	statuses := []SkillStatus{SkillStatusActive, SkillStatusChallenger, SkillStatusDeprecated, SkillStatusTrial}
	seen := make(map[SkillStatus]bool)
	for _, ss := range statuses {
		if ss == "" {
			t.Fatal("SkillStatus value must not be empty")
		}
		if seen[ss] {
			t.Fatalf("duplicate SkillStatus value: %q", ss)
		}
		seen[ss] = true
	}
}

func TestTaskStatuses(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusDraft, TaskStatusClarified, TaskStatusPlanned,
		TaskStatusExecuting, TaskStatusReviewing, TaskStatusCompleted, TaskStatusFailed,
	}
	seen := make(map[TaskStatus]bool)
	for _, ts := range statuses {
		if ts == "" {
			t.Fatal("TaskStatus value must not be empty")
		}
		if seen[ts] {
			t.Fatalf("duplicate TaskStatus value: %q", ts)
		}
		seen[ts] = true
	}
}

// ---------------------------------------------------------------------------
// 4. Recording a run
// ---------------------------------------------------------------------------

func TestRecordRun(t *testing.T) {
	a := New("agent-001", "Alpha")

	rec := RunRecord{
		RunID:       "run-001",
		Timestamp:   time.Now().UTC(),
		InputRef:    "input://task-42",
		TaskSpecRef: "spec://task-42",
		Outputs:     []string{"output://result-1"},
		Metrics:     map[string]string{"tokens": "1500", "iterations": "2"},
		Artifacts:   []string{"artifact://code-diff"},
	}
	a.RecordRun(rec)

	if len(a.RunHistory) != 1 {
		t.Fatalf("expected 1 run, got %d", len(a.RunHistory))
	}
	if a.RunHistory[0].RunID != "run-001" {
		t.Fatalf("RunID = %q, want %q", a.RunHistory[0].RunID, "run-001")
	}
	if len(a.RunHistory[0].Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(a.RunHistory[0].Outputs))
	}
	if a.RunHistory[0].Metrics["tokens"] != "1500" {
		t.Fatalf("Metrics[tokens] = %q, want %q", a.RunHistory[0].Metrics["tokens"], "1500")
	}
}

func TestRecordRun_Multiple(t *testing.T) {
	a := New("agent-001", "Alpha")

	for i := range 5 {
		a.RecordRun(RunRecord{
			RunID:     fmt.Sprintf("run-%03d", i),
			Timestamp: time.Now().UTC(),
		})
	}
	if len(a.RunHistory) != 5 {
		t.Fatalf("expected 5 runs, got %d", len(a.RunHistory))
	}
}

// ---------------------------------------------------------------------------
// 5. Updating quality metrics
// ---------------------------------------------------------------------------

func TestUpdateQualityMetrics(t *testing.T) {
	a := New("agent-001", "Alpha")

	m := QualityMetrics{
		SuccessRate:   0.95,
		ErrorRate:     0.05,
		AvgIterations: 2.3,
		AvgCost:       0.012,
		AvgLatency:    350 * time.Millisecond,
	}
	beforeUpdate := a.UpdatedAt
	// small sleep to ensure UpdatedAt changes
	time.Sleep(time.Millisecond)

	a.UpdateQualityMetrics(m)

	if a.QualityMetrics.SuccessRate != 0.95 {
		t.Fatalf("SuccessRate = %f, want 0.95", a.QualityMetrics.SuccessRate)
	}
	if a.QualityMetrics.ErrorRate != 0.05 {
		t.Fatalf("ErrorRate = %f, want 0.05", a.QualityMetrics.ErrorRate)
	}
	if a.QualityMetrics.AvgIterations != 2.3 {
		t.Fatalf("AvgIterations = %f, want 2.3", a.QualityMetrics.AvgIterations)
	}
	if a.QualityMetrics.AvgCost != 0.012 {
		t.Fatalf("AvgCost = %f, want 0.012", a.QualityMetrics.AvgCost)
	}
	if a.QualityMetrics.AvgLatency != 350*time.Millisecond {
		t.Fatalf("AvgLatency = %v, want %v", a.QualityMetrics.AvgLatency, 350*time.Millisecond)
	}
	if !a.UpdatedAt.After(beforeUpdate) {
		t.Fatal("UpdatedAt should advance after UpdateQualityMetrics")
	}
}

// ---------------------------------------------------------------------------
// 6. Updating specialization (with history)
// ---------------------------------------------------------------------------

func TestUpdateSpecialization(t *testing.T) {
	a := New("agent-001", "Alpha")

	// First specialization
	a.UpdateSpecialization("backend-go", "assigned to backend team")
	if a.Specialization != "backend-go" {
		t.Fatalf("Specialization = %q, want %q", a.Specialization, "backend-go")
	}
	if len(a.SpecializationHistory) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(a.SpecializationHistory))
	}
	if a.SpecializationHistory[0].From != "" {
		t.Fatalf("first change From = %q, want empty", a.SpecializationHistory[0].From)
	}
	if a.SpecializationHistory[0].To != "backend-go" {
		t.Fatalf("first change To = %q, want %q", a.SpecializationHistory[0].To, "backend-go")
	}
	if a.SpecializationHistory[0].Reason != "assigned to backend team" {
		t.Fatalf("first change Reason = %q, want %q", a.SpecializationHistory[0].Reason, "assigned to backend team")
	}

	// Second specialization
	a.UpdateSpecialization("frontend-ts", "reassigned to frontend")
	if a.Specialization != "frontend-ts" {
		t.Fatalf("Specialization = %q, want %q", a.Specialization, "frontend-ts")
	}
	if len(a.SpecializationHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(a.SpecializationHistory))
	}
	if a.SpecializationHistory[1].From != "backend-go" {
		t.Fatalf("second change From = %q, want %q", a.SpecializationHistory[1].From, "backend-go")
	}
	if a.SpecializationHistory[1].To != "frontend-ts" {
		t.Fatalf("second change To = %q, want %q", a.SpecializationHistory[1].To, "frontend-ts")
	}
}

// ---------------------------------------------------------------------------
// 7. JSON serialization / deserialization
// ---------------------------------------------------------------------------

func TestJSON_RoundTrip(t *testing.T) {
	a := New("agent-007", "Bond")
	a.UpdateSpecialization("espionage", "MI6 assignment")
	a.ParentAgentID = "agent-M"
	a.Level = 2
	a.MemoryShortTermRef = "mem://st/007"
	a.MemoryLongTermRef = "mem://lt/007"
	a.DefaultSkillset = []string{"stealth", "combat"}

	_ = a.AddSubagent(SubagentRef{AgentID: "sub-Q", Role: "gadgeteer", Description: "provides gadgets"})
	_ = a.AddSkill(SkillRef{
		SkillID:     "skill-disguise",
		Type:        SkillTypeHybrid,
		Description: "master of disguise",
		Version:     "3.0.0",
		Status:      SkillStatusActive,
	})

	a.RecordRun(RunRecord{
		RunID:       "run-goldfinger",
		Timestamp:   time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		InputRef:    "input://goldfinger",
		TaskSpecRef: "spec://goldfinger",
		Outputs:     []string{"output://gold-secured"},
		Metrics:     map[string]string{"martinis": "3"},
		Artifacts:   []string{"artifact://aston-martin"},
	})

	a.FeedbackHistory = []FeedbackRecord{
		{ReviewerID: "M", Score: 9.5, Comment: "excellent work", Timestamp: time.Now().UTC()},
	}

	a.TaskPatterns = []PatternEntry{
		{PatternID: "pat-1", Description: "infiltration", Fingerprint: "abc123", Count: 7,
			LastSeen: time.Now().UTC(), Examples: []string{"goldfinger", "skyfall"}},
	}
	a.PatternCounts["infiltration"] = 7

	a.UpdateQualityMetrics(QualityMetrics{
		SuccessRate:   0.99,
		ErrorRate:     0.01,
		AvgIterations: 1.5,
		AvgCost:       0.05,
		AvgLatency:    200 * time.Millisecond,
	})

	now := time.Now().UTC()
	a.LastReview = &ReviewRecord{ReviewID: "rev-1", Score: 9.0, Summary: "top agent", Timestamp: now}

	a.EscalationRules = []EscalationRule{
		{Condition: "error_rate > 0.2", Action: "add_subagent", Description: "bring backup"},
	}
	a.ReviewPolicy = ReviewPolicy{Enabled: true, MinScore: 7.0, ReviewInterval: "24h"}
	a.SafetyPolicy = SafetyPolicy{MaxConcurrentRuns: 5, ForbiddenTools: []string{"nuclear_launch"}, RequireApproval: true}

	a.InputSchema = &SchemaRef{SchemaID: "schema-in-007", Version: "1.0"}
	a.OutputSchema = &SchemaRef{SchemaID: "schema-out-007", Version: "1.0"}
	a.ToolAccess = []string{"gun", "car", "phone"}
	a.LLMProviderConfig = &LLMProviderConfig{
		Provider: "anthropic", Model: "claude-opus-4-6", MaxTokens: 4096, Temperature: 0.7,
		Extra: map[string]string{"system_prompt": "you are a spy"},
	}

	// Marshal
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal into a fresh struct
	var b Agent
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify key fields survived the round-trip
	if b.AgentID != a.AgentID {
		t.Fatalf("AgentID mismatch: %q vs %q", b.AgentID, a.AgentID)
	}
	if b.Name != a.Name {
		t.Fatalf("Name mismatch")
	}
	if b.Specialization != "espionage" {
		t.Fatalf("Specialization mismatch: %q", b.Specialization)
	}
	if len(b.SpecializationHistory) != 1 {
		t.Fatalf("SpecializationHistory len = %d, want 1", len(b.SpecializationHistory))
	}
	if b.ParentAgentID != "agent-M" {
		t.Fatalf("ParentAgentID mismatch")
	}
	if b.Level != 2 {
		t.Fatalf("Level mismatch")
	}
	if len(b.Subagents) != 1 || b.Subagents[0].AgentID != "sub-Q" {
		t.Fatal("Subagents mismatch")
	}
	if len(b.Skills) != 1 || b.Skills[0].SkillID != "skill-disguise" {
		t.Fatal("Skills mismatch")
	}
	if b.Skills[0].Type != SkillTypeHybrid {
		t.Fatalf("Skill Type = %q, want HYBRID", b.Skills[0].Type)
	}
	if len(b.DefaultSkillset) != 2 {
		t.Fatalf("DefaultSkillset len = %d, want 2", len(b.DefaultSkillset))
	}
	if b.MemoryShortTermRef != "mem://st/007" {
		t.Fatalf("MemoryShortTermRef mismatch")
	}
	if b.MemoryLongTermRef != "mem://lt/007" {
		t.Fatalf("MemoryLongTermRef mismatch")
	}
	if len(b.RunHistory) != 1 || b.RunHistory[0].RunID != "run-goldfinger" {
		t.Fatal("RunHistory mismatch")
	}
	if b.RunHistory[0].Metrics["martinis"] != "3" {
		t.Fatal("RunHistory Metrics mismatch")
	}
	if len(b.FeedbackHistory) != 1 {
		t.Fatal("FeedbackHistory mismatch")
	}
	if len(b.TaskPatterns) != 1 || b.TaskPatterns[0].PatternID != "pat-1" {
		t.Fatal("TaskPatterns mismatch")
	}
	if b.PatternCounts["infiltration"] != 7 {
		t.Fatal("PatternCounts mismatch")
	}
	if b.QualityMetrics.SuccessRate != 0.99 {
		t.Fatalf("QualityMetrics.SuccessRate mismatch")
	}
	if b.QualityMetrics.AvgLatency != 200*time.Millisecond {
		t.Fatalf("QualityMetrics.AvgLatency = %v, want 200ms", b.QualityMetrics.AvgLatency)
	}
	if b.LastReview == nil || b.LastReview.ReviewID != "rev-1" {
		t.Fatal("LastReview mismatch")
	}
	if b.AutomationThreshold != 3 {
		t.Fatalf("AutomationThreshold = %d, want 3", b.AutomationThreshold)
	}
	if len(b.EscalationRules) != 1 {
		t.Fatal("EscalationRules mismatch")
	}
	if !b.ReviewPolicy.Enabled || b.ReviewPolicy.MinScore != 7.0 {
		t.Fatal("ReviewPolicy mismatch")
	}
	if b.SafetyPolicy.MaxConcurrentRuns != 5 || !b.SafetyPolicy.RequireApproval {
		t.Fatal("SafetyPolicy mismatch")
	}
	if len(b.SafetyPolicy.ForbiddenTools) != 1 {
		t.Fatal("SafetyPolicy.ForbiddenTools mismatch")
	}
	if b.InputSchema == nil || b.InputSchema.SchemaID != "schema-in-007" {
		t.Fatal("InputSchema mismatch")
	}
	if b.OutputSchema == nil || b.OutputSchema.SchemaID != "schema-out-007" {
		t.Fatal("OutputSchema mismatch")
	}
	if len(b.ToolAccess) != 3 {
		t.Fatalf("ToolAccess len = %d, want 3", len(b.ToolAccess))
	}
	if b.LLMProviderConfig == nil || b.LLMProviderConfig.Provider != "anthropic" {
		t.Fatal("LLMProviderConfig mismatch")
	}
	if b.LLMProviderConfig.Model != "claude-opus-4-6" {
		t.Fatalf("LLMProviderConfig.Model = %q", b.LLMProviderConfig.Model)
	}
	if b.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should not be zero after deserialization")
	}
}

func TestJSON_MinimalAgent(t *testing.T) {
	a := New("min-1", "Minimal")

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var b Agent
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if b.AgentID != "min-1" {
		t.Fatalf("AgentID = %q, want %q", b.AgentID, "min-1")
	}
	// Slices should deserialise as non-nil (we marshal empty slices, not nil).
	if b.Subagents == nil {
		t.Fatal("Subagents should not be nil after round-trip")
	}
	if b.Skills == nil {
		t.Fatal("Skills should not be nil after round-trip")
	}
	if b.RunHistory == nil {
		t.Fatal("RunHistory should not be nil after round-trip")
	}
}
