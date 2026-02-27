package reflection

import (
	"context"
	"fmt"
	"strings"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
)

// MacroSummary aggregates data across multiple runs for macro-reflection.
type MacroSummary struct {
	TotalRuns       int
	AvgQuality      float64
	AvgCostUSD      float64
	TopPatterns     []string // Most repeated fingerprints
	RecentInsights  []string // From recent meso-reflections
	SkillCount      int
	GoalsPending    int
	GoalsCompleted  int
}

// MacroInsight is the output of a macro-reflection cycle.
type MacroInsight struct {
	StrategyChanges  []string `json:"strategy_changes"`
	SoulUpdates      []string `json:"soul_updates"`
	NewGoals         []string `json:"new_goals"`
	SkillsToGenerate []string `json:"skills_to_generate"`
	ThresholdChanges []string `json:"threshold_changes"`
}

// Macro performs meta-reflection over recent agent performance.
// It evaluates whether the agent's strategies are still effective
// and suggests higher-level adjustments.
func (e *Engine) Macro(ctx context.Context, soulContent string, summary MacroSummary) (*MacroInsight, float64, error) {
	prompt := fmt.Sprintf(`You are performing MACRO-REFLECTION on this agent's performance over the last %d runs.

Performance Summary:
- Average quality: %.2f
- Average cost per run: $%.4f
- Active skills: %d
- Pending goals: %d / Completed: %d

Top patterns: %s
Recent meso-reflection insights: %s

As a meta-analyst, evaluate:
1. Are the current strategies effective?
2. Should the soul/identity be updated?
3. What new goals should be set?
4. Which skills should be generated or improved?
5. Should any thresholds be adjusted?

Respond in EXACTLY this format:
STRATEGY_CHANGES: <comma-separated list, or NONE>
SOUL_UPDATES: <comma-separated list, or NONE>
NEW_GOALS: <comma-separated list, or NONE>
SKILLS_TO_GENERATE: <comma-separated list, or NONE>
THRESHOLD_CHANGES: <comma-separated list, or NONE>`,
		summary.TotalRuns,
		summary.AvgQuality,
		summary.AvgCostUSD,
		summary.SkillCount,
		summary.GoalsPending,
		summary.GoalsCompleted,
		strings.Join(summary.TopPatterns, ", "),
		strings.Join(summary.RecentInsights, "; "),
	)

	messages := e.ctx.Assemble(brain.ContextLayers{
		SystemPrompt:    soulContent,
		TaskDescription: prompt,
	})

	// Macro-reflection uses a mid-tier model for deeper analysis.
	model := e.router.Select("moderate", 100.0)
	resp, err := e.llm.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("macro reflection: %w", err)
	}

	insight := parseMacroResponse(resp.Content)

	// Store in long-term memory.
	e.longMem.Store(memory.LongTermEntry{
		ID:      fmt.Sprintf("macro_%d", e.runsSinceMacro),
		Summary: fmt.Sprintf("Macro-reflection: strategies=[%s] goals=[%s]", strings.Join(insight.StrategyChanges, "; "), strings.Join(insight.NewGoals, "; ")),
		Tags:    []string{"reflection", "macro"},
	})

	e.ResetMacroCounter()
	return insight, resp.CostUSD, nil
}

func parseMacroResponse(text string) *MacroInsight {
	insight := &MacroInsight{}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		parseList := func(prefix string) []string {
			raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if raw == "" || raw == "NONE" || raw == "none" {
				return nil
			}
			var items []string
			for _, item := range strings.Split(raw, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					items = append(items, item)
				}
			}
			return items
		}

		switch {
		case strings.HasPrefix(line, "STRATEGY_CHANGES:"):
			insight.StrategyChanges = parseList("STRATEGY_CHANGES:")
		case strings.HasPrefix(line, "SOUL_UPDATES:"):
			insight.SoulUpdates = parseList("SOUL_UPDATES:")
		case strings.HasPrefix(line, "NEW_GOALS:"):
			insight.NewGoals = parseList("NEW_GOALS:")
		case strings.HasPrefix(line, "SKILLS_TO_GENERATE:"):
			insight.SkillsToGenerate = parseList("SKILLS_TO_GENERATE:")
		case strings.HasPrefix(line, "THRESHOLD_CHANGES:"):
			insight.ThresholdChanges = parseList("THRESHOLD_CHANGES:")
		}
	}

	return insight
}
