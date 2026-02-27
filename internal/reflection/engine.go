// Package reflection implements the 4-level reflection system for Overhuman.
//
// The reflection hierarchy:
//   - Micro:  per pipeline step  (Phase 4)
//   - Meso:   per task run       (Phase 1 — this file)
//   - Macro:  per N runs / timer (Phase 3)
//   - Mega:   reflection on reflection (Phase 4)
//
// Meso-reflection runs after each completed pipeline run. It evaluates what
// went well and what could be improved, then stores actionable insights in
// long-term memory and optionally suggests updates to the Soul.
package reflection

import (
	"context"
	"fmt"
	"strings"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
)

// RunSummary contains the data from one completed pipeline run that the
// reflection engine needs to evaluate.
type RunSummary struct {
	TaskID        string
	Goal          string
	Result        string
	QualityScore  float64
	ReviewNotes   string
	CostUSD       float64
	ElapsedMs     int64
	Fingerprint   string
	SourceChannel string
}

// MesoInsight is the output of a meso-reflection cycle.
type MesoInsight struct {
	TaskID      string   `json:"task_id"`
	WentWell    []string `json:"went_well"`
	Improvements []string `json:"improvements"`
	SoulSuggestion string `json:"soul_suggestion,omitempty"` // If non-empty, suggests a soul update.
	SkillSuggestion string `json:"skill_suggestion,omitempty"` // If non-empty, suggests a new skill.
}

// Engine orchestrates all reflection loops.
type Engine struct {
	llm     brain.LLMProvider
	router  *brain.ModelRouter
	ctx     *brain.ContextAssembler
	longMem *memory.LongTermMemory

	// macroThreshold triggers macro-reflection every N runs.
	macroThreshold int
	runsSinceMacro int
}

// NewEngine creates a reflection engine.
func NewEngine(
	llm brain.LLMProvider,
	router *brain.ModelRouter,
	ca *brain.ContextAssembler,
	longMem *memory.LongTermMemory,
) *Engine {
	return &Engine{
		llm:            llm,
		router:         router,
		ctx:            ca,
		longMem:        longMem,
		macroThreshold: 10,
	}
}

// SetMacroThreshold configures how many runs between macro-reflection cycles.
func (e *Engine) SetMacroThreshold(n int) {
	if n > 0 {
		e.macroThreshold = n
	}
}

// Meso performs per-run reflection.
// It asks the LLM to analyze the run result and produces structured insights.
func (e *Engine) Meso(ctx context.Context, soulContent string, summary RunSummary) (*MesoInsight, float64, error) {
	prompt := fmt.Sprintf(`Reflect on this completed task run.

Task: %s
Result quality: %.2f
Review notes: %s
Cost: $%.4f
Time: %dms

Analyze and respond in EXACTLY this format:
WENT_WELL: <comma-separated list>
IMPROVEMENTS: <comma-separated list>
SOUL_SUGGESTION: <one-line suggestion for the soul/strategy update, or NONE>
SKILL_SUGGESTION: <one-line suggestion for a new skill to build, or NONE>`,
		summary.Goal,
		summary.QualityScore,
		summary.ReviewNotes,
		summary.CostUSD,
		summary.ElapsedMs,
	)

	messages := e.ctx.Assemble(brain.ContextLayers{
		SystemPrompt:    soulContent,
		TaskDescription: prompt,
	})

	// Use cheapest model for reflection.
	model := e.router.Select("simple", 100.0)
	resp, err := e.llm.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 512,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("meso reflection: %w", err)
	}

	insight := parseMesoResponse(summary.TaskID, resp.Content)

	// Store insight in long-term memory.
	var tags []string
	tags = append(tags, "reflection", "meso")
	if summary.Fingerprint != "" {
		tags = append(tags, summary.Fingerprint)
	}

	e.longMem.Store(memory.LongTermEntry{
		ID:          summary.TaskID + "_meso",
		Summary:     fmt.Sprintf("Meso-reflection: well=[%s] improve=[%s]", strings.Join(insight.WentWell, "; "), strings.Join(insight.Improvements, "; ")),
		Tags:        tags,
		SourceRunID: summary.TaskID,
	})

	// Track runs for macro-reflection trigger.
	e.runsSinceMacro++

	return insight, resp.CostUSD, nil
}

// ShouldRunMacro returns true if enough runs have passed to warrant macro-reflection.
func (e *Engine) ShouldRunMacro() bool {
	return e.runsSinceMacro >= e.macroThreshold
}

// ResetMacroCounter resets the run counter after macro-reflection.
func (e *Engine) ResetMacroCounter() {
	e.runsSinceMacro = 0
}

// RunsSinceMacro returns the current count.
func (e *Engine) RunsSinceMacro() int {
	return e.runsSinceMacro
}

// parseMesoResponse extracts structured insight from the LLM response text.
func parseMesoResponse(taskID, text string) *MesoInsight {
	insight := &MesoInsight{TaskID: taskID}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "WENT_WELL:"):
			raw := strings.TrimPrefix(line, "WENT_WELL:")
			for _, item := range strings.Split(raw, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					insight.WentWell = append(insight.WentWell, item)
				}
			}
		case strings.HasPrefix(line, "IMPROVEMENTS:"):
			raw := strings.TrimPrefix(line, "IMPROVEMENTS:")
			for _, item := range strings.Split(raw, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					insight.Improvements = append(insight.Improvements, item)
				}
			}
		case strings.HasPrefix(line, "SOUL_SUGGESTION:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "SOUL_SUGGESTION:"))
			if val != "" && val != "NONE" && val != "none" {
				insight.SoulSuggestion = val
			}
		case strings.HasPrefix(line, "SKILL_SUGGESTION:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "SKILL_SUGGESTION:"))
			if val != "" && val != "NONE" && val != "none" {
				insight.SkillSuggestion = val
			}
		}
	}

	return insight
}
