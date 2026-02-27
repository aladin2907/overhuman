// Package agent defines the core Agent model for the Overhuman system.
//
// The Agent is the central entity that represents an autonomous unit capable of
// executing tasks, managing subagents, accumulating skills, and evolving its
// specialization over time. This file implements sections 7.1–7.8 of the ТЗ.
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// SkillType describes how a skill is implemented.
type SkillType string

const (
	SkillTypeLLM    SkillType = "LLM"
	SkillTypeCode   SkillType = "CODE"
	SkillTypeHybrid SkillType = "HYBRID"
)

// SkillStatus represents the lifecycle stage of a skill.
type SkillStatus string

const (
	SkillStatusActive     SkillStatus = "ACTIVE"
	SkillStatusChallenger SkillStatus = "CHALLENGER"
	SkillStatusDeprecated SkillStatus = "DEPRECATED"
	SkillStatusTrial      SkillStatus = "TRIAL"
)

// TaskStatus represents the lifecycle stage of a task.
type TaskStatus string

const (
	TaskStatusDraft     TaskStatus = "DRAFT"
	TaskStatusClarified TaskStatus = "CLARIFIED"
	TaskStatusPlanned   TaskStatus = "PLANNED"
	TaskStatusExecuting TaskStatus = "EXECUTING"
	TaskStatusReviewing TaskStatus = "REVIEWING"
	TaskStatusCompleted TaskStatus = "COMPLETED"
	TaskStatusFailed    TaskStatus = "FAILED"
)

// ---------------------------------------------------------------------------
// Supporting types – 7.1 Identity & Role
// ---------------------------------------------------------------------------

// SpecializationChange records a single change of the agent's specialization.
type SpecializationChange struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.2 Hierarchy
// ---------------------------------------------------------------------------

// SubagentRef is a lightweight reference to a subagent.
type SubagentRef struct {
	AgentID     string `json:"agent_id"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.3 Skills
// ---------------------------------------------------------------------------

// SkillRef describes a skill attached to the agent.
type SkillRef struct {
	SkillID     string      `json:"skill_id"`
	Type        SkillType   `json:"type"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Status      SkillStatus `json:"status"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.4 Memory & Experience
// ---------------------------------------------------------------------------

// RunRecord is a single entry in the agent's execution journal.
type RunRecord struct {
	RunID       string            `json:"run_id"`
	Timestamp   time.Time         `json:"timestamp"`
	InputRef    string            `json:"input_ref"`
	TaskSpecRef string            `json:"task_spec_ref"`
	Outputs     []string          `json:"outputs"`
	Metrics     map[string]string `json:"metrics"`
	Artifacts   []string          `json:"artifacts"`
}

// FeedbackRecord stores a review assessment from a reviewer (human or agent).
type FeedbackRecord struct {
	ReviewerID string    `json:"reviewer_id"`
	Score      float64   `json:"score"`
	Comment    string    `json:"comment"`
	Timestamp  time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.5 Pattern Statistics
// ---------------------------------------------------------------------------

// PatternEntry describes a recognised task pattern.
type PatternEntry struct {
	PatternID   string    `json:"pattern_id"`
	Description string    `json:"description"`
	Fingerprint string    `json:"fingerprint"`
	Count       int       `json:"count"`
	LastSeen    time.Time `json:"last_seen"`
	Examples    []string  `json:"examples"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.6 Quality Metrics
// ---------------------------------------------------------------------------

// QualityMetrics holds aggregated quality indicators for the agent.
type QualityMetrics struct {
	SuccessRate   float64       `json:"success_rate"`
	ErrorRate     float64       `json:"error_rate"`
	AvgIterations float64       `json:"avg_iterations"`
	AvgCost       float64       `json:"avg_cost"`
	AvgLatency    time.Duration `json:"avg_latency"`
}

// ReviewRecord stores the result of the agent's last self-assessment.
type ReviewRecord struct {
	ReviewID  string    `json:"review_id"`
	Score     float64   `json:"score"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.7 Policies & Triggers
// ---------------------------------------------------------------------------

// EscalationRule defines when and how to add or replace subagents.
type EscalationRule struct {
	Condition   string `json:"condition"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// ReviewPolicy specifies mandatory review rules.
type ReviewPolicy struct {
	Enabled        bool    `json:"enabled"`
	MinScore       float64 `json:"min_score"`
	ReviewInterval string  `json:"review_interval"`
}

// SafetyPolicy defines execution restrictions.
type SafetyPolicy struct {
	MaxConcurrentRuns int      `json:"max_concurrent_runs"`
	ForbiddenTools    []string `json:"forbidden_tools"`
	RequireApproval   bool     `json:"require_approval"`
}

// ---------------------------------------------------------------------------
// Supporting types – 7.8 Interfaces
// ---------------------------------------------------------------------------

// SchemaRef is a reference to a JSON-Schema contract.
type SchemaRef struct {
	SchemaID string          `json:"schema_id"`
	Version  string          `json:"version"`
	Body     json.RawMessage `json:"body,omitempty"`
}

// LLMProviderConfig holds parameters for the LLM provider.
type LLMProviderConfig struct {
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
	Extra       map[string]string `json:"extra,omitempty"`
}

// ---------------------------------------------------------------------------
// Agent – the root model (sections 7.1–7.8)
// ---------------------------------------------------------------------------

// Agent is the core model of the Overhuman system representing an autonomous
// agent that can execute tasks, manage subagents, and evolve over time.
type Agent struct {
	// 7.1 Identity & Role
	AgentID               string                 `json:"agent_id"`
	Name                  string                 `json:"name"`
	Specialization        string                 `json:"specialization"`
	SpecializationHistory []SpecializationChange `json:"specialization_history"`

	// 7.2 Hierarchy
	Subagents     []SubagentRef `json:"subagents"`
	ParentAgentID string        `json:"parent_agent_id,omitempty"`
	Level         int           `json:"level"`

	// 7.3 Skills
	Skills          []SkillRef `json:"skills"`
	DefaultSkillset []string   `json:"default_skillset"`

	// 7.4 Memory & Experience
	MemoryShortTermRef string           `json:"memory_short_term_ref"`
	MemoryLongTermRef  string           `json:"memory_long_term_ref"`
	RunHistory         []RunRecord      `json:"run_history"`
	FeedbackHistory    []FeedbackRecord `json:"feedback_history"`

	// 7.5 Pattern Statistics
	TaskPatterns  []PatternEntry `json:"task_patterns"`
	PatternCounts map[string]int `json:"pattern_counts"`

	// 7.6 Quality Metrics
	QualityMetrics QualityMetrics `json:"quality_metrics"`
	LastReview     *ReviewRecord  `json:"last_review,omitempty"`

	// 7.7 Policies & Triggers
	AutomationThreshold int              `json:"automation_threshold"`
	EscalationRules     []EscalationRule `json:"escalation_rules"`
	ReviewPolicy        ReviewPolicy     `json:"review_policy"`
	SafetyPolicy        SafetyPolicy     `json:"safety_policy"`

	// 7.8 Interfaces
	InputSchema       *SchemaRef         `json:"input_schema,omitempty"`
	OutputSchema      *SchemaRef         `json:"output_schema,omitempty"`
	ToolAccess        []string           `json:"tool_access"`
	LLMProviderConfig *LLMProviderConfig `json:"llm_provider_config,omitempty"`

	// Internal bookkeeping
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// DefaultAutomationThreshold is the default repetition count that triggers
// automation of a pattern (section 7.7).
const DefaultAutomationThreshold = 3

// New creates a new Agent with sensible defaults.
func New(id, name string) *Agent {
	now := time.Now().UTC()
	return &Agent{
		AgentID:               id,
		Name:                  name,
		SpecializationHistory: make([]SpecializationChange, 0),
		Subagents:             make([]SubagentRef, 0),
		Skills:                make([]SkillRef, 0),
		DefaultSkillset:       make([]string, 0),
		RunHistory:            make([]RunRecord, 0),
		FeedbackHistory:       make([]FeedbackRecord, 0),
		TaskPatterns:          make([]PatternEntry, 0),
		PatternCounts:         make(map[string]int),
		AutomationThreshold:   DefaultAutomationThreshold,
		EscalationRules:       make([]EscalationRule, 0),
		ToolAccess:            make([]string, 0),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

// ---------------------------------------------------------------------------
// 7.1 – Specialization
// ---------------------------------------------------------------------------

// UpdateSpecialization changes the agent's specialization and appends an
// entry to the specialization history.
func (a *Agent) UpdateSpecialization(newSpec, reason string) {
	change := SpecializationChange{
		From:      a.Specialization,
		To:        newSpec,
		Reason:    reason,
		Timestamp: time.Now().UTC(),
	}
	a.SpecializationHistory = append(a.SpecializationHistory, change)
	a.Specialization = newSpec
	a.touch()
}

// ---------------------------------------------------------------------------
// 7.2 – Subagents
// ---------------------------------------------------------------------------

// AddSubagent adds a subagent reference. Returns an error if a subagent with
// the same agent_id already exists.
func (a *Agent) AddSubagent(ref SubagentRef) error {
	for _, s := range a.Subagents {
		if s.AgentID == ref.AgentID {
			return fmt.Errorf("subagent %q already exists", ref.AgentID)
		}
	}
	a.Subagents = append(a.Subagents, ref)
	a.touch()
	return nil
}

// RemoveSubagent removes a subagent by its agent_id. Returns an error if not
// found.
func (a *Agent) RemoveSubagent(agentID string) error {
	for i, s := range a.Subagents {
		if s.AgentID == agentID {
			a.Subagents = append(a.Subagents[:i], a.Subagents[i+1:]...)
			a.touch()
			return nil
		}
	}
	return fmt.Errorf("subagent %q not found", agentID)
}

// ---------------------------------------------------------------------------
// 7.3 – Skills
// ---------------------------------------------------------------------------

// AddSkill adds a skill reference. Returns an error if a skill with the same
// skill_id already exists.
func (a *Agent) AddSkill(ref SkillRef) error {
	for _, s := range a.Skills {
		if s.SkillID == ref.SkillID {
			return fmt.Errorf("skill %q already exists", ref.SkillID)
		}
	}
	a.Skills = append(a.Skills, ref)
	a.touch()
	return nil
}

// ---------------------------------------------------------------------------
// 7.4 – Run History
// ---------------------------------------------------------------------------

// RecordRun appends a RunRecord to the agent's execution journal.
func (a *Agent) RecordRun(rec RunRecord) {
	a.RunHistory = append(a.RunHistory, rec)
	a.touch()
}

// ---------------------------------------------------------------------------
// 7.6 – Quality Metrics
// ---------------------------------------------------------------------------

var (
	// ErrNoRuns is returned when quality metrics cannot be computed because
	// the agent has no run history.
	ErrNoRuns = errors.New("no runs recorded")
)

// UpdateQualityMetrics recomputes aggregated quality metrics from the
// supplied totals. This is intentionally a simple setter so that callers
// (e.g. a metrics pipeline) can supply pre-computed values.
func (a *Agent) UpdateQualityMetrics(m QualityMetrics) {
	a.QualityMetrics = m
	a.touch()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (a *Agent) touch() {
	a.UpdatedAt = time.Now().UTC()
}
