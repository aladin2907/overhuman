// Package instruments implements the skill system for Overhuman.
//
// Skills are the "hands" of the agent — executable capabilities that
// perform actions. There are three types:
//
//   - LLM Skill: delegates to an LLM provider (flexible, expensive)
//   - Code Skill: runs deterministic code (fast, cheap, reliable)
//   - Hybrid Skill: tries code first, falls back to LLM
//
// The LLM→Code flywheel: as the agent detects repeated patterns,
// it auto-generates Code Skills to replace expensive LLM calls.
package instruments

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SkillType identifies the implementation strategy of a skill.
type SkillType string

const (
	SkillTypeLLM    SkillType = "LLM"
	SkillTypeCode   SkillType = "CODE"
	SkillTypeHybrid SkillType = "HYBRID"
)

// SkillStatus tracks the lifecycle state of a skill.
type SkillStatus string

const (
	SkillStatusActive     SkillStatus = "ACTIVE"
	SkillStatusChallenger SkillStatus = "CHALLENGER" // A/B testing against incumbent
	SkillStatusTrial      SkillStatus = "TRIAL"      // Newly generated, under observation
	SkillStatusDeprecated SkillStatus = "DEPRECATED"
)

// SkillInput is the standardized input to any skill execution.
type SkillInput struct {
	Goal       string            `json:"goal"`
	Context    string            `json:"context,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// SkillOutput is the standardized output from any skill execution.
type SkillOutput struct {
	Result    string  `json:"result"`
	Success   bool    `json:"success"`
	CostUSD   float64 `json:"cost_usd"`
	ElapsedMs int64   `json:"elapsed_ms"`
	Error     string  `json:"error,omitempty"`
}

// SkillExecutor is the interface that all skill implementations must satisfy.
type SkillExecutor interface {
	Execute(ctx context.Context, input SkillInput) (*SkillOutput, error)
}

// SkillMeta holds metadata about a registered skill.
type SkillMeta struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        SkillType   `json:"type"`
	Status      SkillStatus `json:"status"`
	Fingerprint string      `json:"fingerprint,omitempty"` // Links to the pattern that spawned it
	Version     int         `json:"version"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`

	// Fitness metrics (populated by Evolution engine in Phase 3).
	TotalRuns     int     `json:"total_runs"`
	SuccessRate   float64 `json:"success_rate"`
	AvgQuality    float64 `json:"avg_quality"`
	AvgCostUSD    float64 `json:"avg_cost_usd"`
	AvgElapsedMs  float64 `json:"avg_elapsed_ms"`
}

// Skill combines metadata with an executor.
type Skill struct {
	Meta     SkillMeta
	Executor SkillExecutor
}

// RecordRun updates fitness metrics after a skill execution.
func (s *Skill) RecordRun(output *SkillOutput) {
	n := float64(s.Meta.TotalRuns)
	s.Meta.TotalRuns++
	newN := float64(s.Meta.TotalRuns)

	// Running averages.
	if output.Success {
		s.Meta.SuccessRate = (s.Meta.SuccessRate*n + 1.0) / newN
	} else {
		s.Meta.SuccessRate = (s.Meta.SuccessRate * n) / newN
	}
	s.Meta.AvgCostUSD = (s.Meta.AvgCostUSD*n + output.CostUSD) / newN
	s.Meta.AvgElapsedMs = (s.Meta.AvgElapsedMs*n + float64(output.ElapsedMs)) / newN
	s.Meta.UpdatedAt = time.Now()
}

// -------------------------------------------------------------------
// LLMSkill — delegates execution to an LLM provider.
// -------------------------------------------------------------------

// LLMSkillFunc is a function that calls the LLM and returns output.
type LLMSkillFunc func(ctx context.Context, input SkillInput) (*SkillOutput, error)

// LLMSkill wraps an LLM call as a skill.
type LLMSkill struct {
	fn LLMSkillFunc
}

// NewLLMSkill creates a new LLM-based skill.
func NewLLMSkill(fn LLMSkillFunc) *LLMSkill {
	return &LLMSkill{fn: fn}
}

// Execute runs the LLM skill.
func (s *LLMSkill) Execute(ctx context.Context, input SkillInput) (*SkillOutput, error) {
	return s.fn(ctx, input)
}

// -------------------------------------------------------------------
// CodeSkill — executes deterministic code.
// -------------------------------------------------------------------

// CodeSkillFunc is a pure function that produces output without LLM calls.
type CodeSkillFunc func(ctx context.Context, input SkillInput) (*SkillOutput, error)

// CodeSkill wraps a code function as a skill.
type CodeSkill struct {
	fn       CodeSkillFunc
	Language string // "go", "python", "javascript", "bash"
	Source   string // source code (for inspection/versioning)
}

// NewCodeSkill creates a new code-based skill.
func NewCodeSkill(fn CodeSkillFunc, language, source string) *CodeSkill {
	return &CodeSkill{fn: fn, Language: language, Source: source}
}

// Execute runs the code skill.
func (s *CodeSkill) Execute(ctx context.Context, input SkillInput) (*SkillOutput, error) {
	start := time.Now()
	output, err := s.fn(ctx, input)
	if err != nil {
		return &SkillOutput{
			Success:   false,
			Error:     err.Error(),
			ElapsedMs: time.Since(start).Milliseconds(),
		}, err
	}
	output.ElapsedMs = time.Since(start).Milliseconds()
	output.CostUSD = 0 // Code skills are free.
	return output, nil
}

// -------------------------------------------------------------------
// HybridSkill — tries code first, falls back to LLM.
// -------------------------------------------------------------------

// HybridSkill first tries a CodeSkill, falls back to LLMSkill on failure.
type HybridSkill struct {
	code *CodeSkill
	llm  *LLMSkill
}

// NewHybridSkill creates a hybrid skill with code-first, LLM-fallback.
func NewHybridSkill(code *CodeSkill, llm *LLMSkill) *HybridSkill {
	return &HybridSkill{code: code, llm: llm}
}

// Execute tries code first, falls back to LLM.
func (s *HybridSkill) Execute(ctx context.Context, input SkillInput) (*SkillOutput, error) {
	output, err := s.code.Execute(ctx, input)
	if err == nil && output.Success {
		return output, nil
	}

	// Fallback to LLM.
	return s.llm.Execute(ctx, input)
}

// -------------------------------------------------------------------
// SkillRegistry — manages all registered skills.
// -------------------------------------------------------------------

// SkillRegistry stores and retrieves skills. Thread-safe.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*Skill // keyed by skill ID

	// Index: fingerprint → skill IDs (for pattern-based lookup).
	byFingerprint map[string][]string
}

// NewSkillRegistry creates an empty registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills:        make(map[string]*Skill),
		byFingerprint: make(map[string][]string),
	}
}

// Register adds a skill to the registry.
func (r *SkillRegistry) Register(skill *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[skill.Meta.ID] = skill
	if skill.Meta.Fingerprint != "" {
		r.byFingerprint[skill.Meta.Fingerprint] = append(
			r.byFingerprint[skill.Meta.Fingerprint], skill.Meta.ID,
		)
	}
}

// Get retrieves a skill by ID.
func (r *SkillRegistry) Get(id string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[id]
}

// FindByFingerprint returns all skills linked to a pattern fingerprint.
func (r *SkillRegistry) FindByFingerprint(fingerprint string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byFingerprint[fingerprint]
	var result []*Skill
	for _, id := range ids {
		if s := r.skills[id]; s != nil {
			result = append(result, s)
		}
	}
	return result
}

// FindActive returns the best active skill for a fingerprint.
// Prefers CODE over HYBRID over LLM. Among same type, picks highest success rate.
func (r *SkillRegistry) FindActive(fingerprint string) *Skill {
	skills := r.FindByFingerprint(fingerprint)
	if len(skills) == 0 {
		return nil
	}

	var best *Skill
	for _, s := range skills {
		if s.Meta.Status == SkillStatusDeprecated {
			continue
		}
		if best == nil {
			best = s
			continue
		}
		// Prefer CODE > HYBRID > LLM.
		if typePriority(s.Meta.Type) > typePriority(best.Meta.Type) {
			best = s
		} else if s.Meta.Type == best.Meta.Type && s.Meta.SuccessRate > best.Meta.SuccessRate {
			best = s
		}
	}
	return best
}

func typePriority(t SkillType) int {
	switch t {
	case SkillTypeCode:
		return 3
	case SkillTypeHybrid:
		return 2
	case SkillTypeLLM:
		return 1
	default:
		return 0
	}
}

// List returns all registered skills.
func (r *SkillRegistry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// Count returns the total number of skills.
func (r *SkillRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// UpdateStatus changes a skill's status.
func (r *SkillRegistry) UpdateStatus(id string, status SkillStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.skills[id]
	if !ok {
		return fmt.Errorf("skill %q not found", id)
	}
	s.Meta.Status = status
	s.Meta.UpdatedAt = time.Now()
	return nil
}

// Remove removes a skill from the registry.
func (r *SkillRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.skills[id]
	if !ok {
		return
	}

	// Remove from fingerprint index.
	if s.Meta.Fingerprint != "" {
		ids := r.byFingerprint[s.Meta.Fingerprint]
		for i, sid := range ids {
			if sid == id {
				r.byFingerprint[s.Meta.Fingerprint] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	delete(r.skills, id)
}

// MarshalMeta returns JSON of all skill metadata (without executors).
func (r *SkillRegistry) MarshalMeta() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metas := make([]SkillMeta, 0, len(r.skills))
	for _, s := range r.skills {
		metas = append(metas, s.Meta)
	}
	return json.Marshal(metas)
}
