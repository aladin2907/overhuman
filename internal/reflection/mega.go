package reflection

import (
	"context"
	"fmt"
	"strings"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
)

// MegaSummary aggregates data about how well the reflection process itself works.
type MegaSummary struct {
	TotalMesoRuns      int
	TotalMacroRuns     int
	MesoInsightsActedOn int      // How many meso insights led to real changes
	MacroInsightsActedOn int     // How many macro insights led to real changes
	RecentMesoInsights []string  // Last N meso insight summaries
	RecentMacroInsights []string // Last N macro insight summaries
	QualityTrend       string    // "improving", "stable", "degrading"
	CostTrend          string    // "decreasing", "stable", "increasing"
}

// MegaInsight is the output of a mega-reflection cycle.
type MegaInsight struct {
	ReflectionEffectiveness string   `json:"reflection_effectiveness"` // Assessment of reflection quality
	MesoAdjustments         []string `json:"meso_adjustments"`         // Changes to meso-reflection process
	MacroAdjustments        []string `json:"macro_adjustments"`        // Changes to macro-reflection process
	ThresholdAdjustments    []string `json:"threshold_adjustments"`    // Changes to triggers/thresholds
	ProcessChanges          []string `json:"process_changes"`          // Higher-level process improvements
}

// Mega performs reflection on the reflection process itself.
// This is the highest-level feedback loop — it asks "Are my reflection loops
// actually helping me improve?" and adjusts the reflection process.
func (e *Engine) Mega(ctx context.Context, soulContent string, summary MegaSummary) (*MegaInsight, float64, error) {
	prompt := fmt.Sprintf(`You are performing MEGA-REFLECTION — reflecting on the reflection process itself.

Reflection Process Stats:
- Total meso-reflections run: %d
- Total macro-reflections run: %d
- Meso insights that led to real changes: %d
- Macro insights that led to real changes: %d
- Quality trend: %s
- Cost trend: %s

Recent meso insights: %s
Recent macro insights: %s

Evaluate:
1. Is the meso-reflection process (per-run) producing useful insights?
2. Is the macro-reflection process (per-N-runs) leading to real improvements?
3. Are the reflection thresholds (when to trigger) appropriate?
4. What changes to the reflection process itself would improve outcomes?

Respond in EXACTLY this format:
EFFECTIVENESS: <one-line assessment>
MESO_ADJUSTMENTS: <comma-separated list, or NONE>
MACRO_ADJUSTMENTS: <comma-separated list, or NONE>
THRESHOLD_ADJUSTMENTS: <comma-separated list, or NONE>
PROCESS_CHANGES: <comma-separated list, or NONE>`,
		summary.TotalMesoRuns,
		summary.TotalMacroRuns,
		summary.MesoInsightsActedOn,
		summary.MacroInsightsActedOn,
		summary.QualityTrend,
		summary.CostTrend,
		strings.Join(summary.RecentMesoInsights, "; "),
		strings.Join(summary.RecentMacroInsights, "; "),
	)

	messages := e.ctx.Assemble(brain.ContextLayers{
		SystemPrompt:    soulContent,
		TaskDescription: prompt,
	})

	// Mega-reflection uses a strong model for deep analysis.
	model := e.router.Select("complex", 100.0)
	resp, err := e.llm.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("mega reflection: %w", err)
	}

	insight := parseMegaResponse(resp.Content)

	// Store in long-term memory.
	e.longMem.Store(memory.LongTermEntry{
		ID:      fmt.Sprintf("mega_%d_%d", summary.TotalMesoRuns, summary.TotalMacroRuns),
		Summary: fmt.Sprintf("Mega-reflection: effectiveness=[%s] process_changes=[%s]", insight.ReflectionEffectiveness, strings.Join(insight.ProcessChanges, "; ")),
		Tags:    []string{"reflection", "mega"},
	})

	return insight, resp.CostUSD, nil
}

// parseMegaResponse extracts a MegaInsight from LLM text.
func parseMegaResponse(text string) *MegaInsight {
	insight := &MegaInsight{}

	parseList := func(line, prefix string) []string {
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

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "EFFECTIVENESS:"):
			insight.ReflectionEffectiveness = strings.TrimSpace(strings.TrimPrefix(line, "EFFECTIVENESS:"))
		case strings.HasPrefix(line, "MESO_ADJUSTMENTS:"):
			insight.MesoAdjustments = parseList(line, "MESO_ADJUSTMENTS:")
		case strings.HasPrefix(line, "MACRO_ADJUSTMENTS:"):
			insight.MacroAdjustments = parseList(line, "MACRO_ADJUSTMENTS:")
		case strings.HasPrefix(line, "THRESHOLD_ADJUSTMENTS:"):
			insight.ThresholdAdjustments = parseList(line, "THRESHOLD_ADJUSTMENTS:")
		case strings.HasPrefix(line, "PROCESS_CHANGES:"):
			insight.ProcessChanges = parseList(line, "PROCESS_CHANGES:")
		}
	}

	return insight
}
