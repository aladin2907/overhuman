package genui

import (
	"fmt"
	"sort"
	"strings"
)

// HintBuilder generates UI generation hints from memory and reflection data.
type HintBuilder struct {
	memory     *UIMemory
	reflection *ReflectionStore
	maxHints   int
}

// NewHintBuilder creates a new hint builder.
func NewHintBuilder(mem *UIMemory, refl *ReflectionStore, maxHints int) *HintBuilder {
	if maxHints <= 0 {
		maxHints = 10
	}
	return &HintBuilder{
		memory:     mem,
		reflection: refl,
		maxHints:   maxHints,
	}
}

// Build generates hints for a given fingerprint. Combines memory patterns with recent reflections.
func (b *HintBuilder) Build(fingerprint string) []string {
	var hints []string

	// Memory-based hints (fingerprint-specific history).
	if b.memory != nil {
		hints = append(hints, b.memoryHints(fingerprint)...)
	}

	// Reflection-based hints (recent interactions).
	if b.reflection != nil {
		hints = append(hints, b.reflectionHints(fingerprint)...)
	}

	// Deduplicate.
	hints = deduplicateHints(hints)

	// Trim to max.
	if len(hints) > b.maxHints {
		hints = hints[:b.maxHints]
	}

	return hints
}

// memoryHints generates hints from UI memory patterns.
func (b *HintBuilder) memoryHints(fingerprint string) []string {
	entries := b.memory.Lookup(fingerprint)
	if len(entries) == 0 {
		return nil
	}

	var hints []string

	// Best format hint.
	formatScores := make(map[UIFormat]float64)
	formatCounts := make(map[UIFormat]int)
	for _, e := range entries {
		formatScores[e.Format] += e.Score
		formatCounts[e.Format]++
	}
	var bestFormat UIFormat
	var bestAvg float64
	for f, total := range formatScores {
		avg := total / float64(formatCounts[f])
		if avg > bestAvg {
			bestAvg = avg
			bestFormat = f
		}
	}
	if bestFormat != "" && bestAvg > 0.6 {
		hints = append(hints, fmt.Sprintf("Format %q has worked well for this task type (avg score: %.0f%%)", bestFormat, bestAvg*100))
	}

	// Most-used actions hint.
	actionCounts := make(map[string]int)
	for _, e := range entries {
		for _, a := range e.ActionsUsed {
			actionCounts[a]++
		}
	}
	if len(actionCounts) > 0 {
		type ac struct {
			action string
			count  int
		}
		var sorted []ac
		for a, c := range actionCounts {
			sorted = append(sorted, ac{a, c})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})
		top := sorted
		if len(top) > 3 {
			top = top[:3]
		}
		var names []string
		for _, a := range top {
			names = append(names, a.action)
		}
		hints = append(hints, "User frequently uses these actions: "+strings.Join(names, ", "))
	}

	// Scroll pattern hint.
	scrollCount := 0
	for _, e := range entries {
		if e.Scrolled {
			scrollCount++
		}
	}
	if len(entries) > 2 && float64(scrollCount)/float64(len(entries)) > 0.5 {
		hints = append(hints, "User often scrolls — consider more compact layout with collapsible sections")
	}

	// Dismiss pattern hint.
	dismissCount := 0
	for _, e := range entries {
		if e.Dismissed {
			dismissCount++
		}
	}
	if len(entries) > 2 && float64(dismissCount)/float64(len(entries)) > 0.3 {
		hints = append(hints, "User frequently dismisses UI for this task type — try a simpler, more focused layout")
	}

	// Speed hint.
	var totalTTA int64
	var ttaCount int
	for _, e := range entries {
		if e.TimeToAction > 0 {
			totalTTA += e.TimeToAction
			ttaCount++
		}
	}
	if ttaCount > 0 {
		avgTTA := totalTTA / int64(ttaCount)
		if avgTTA < 3000 {
			hints = append(hints, "User responds quickly — current layout is intuitive")
		} else if avgTTA > 15000 {
			hints = append(hints, "User takes long to interact — consider clearer calls to action")
		}
	}

	// Average score hint.
	avgScore := b.memory.AverageScore(fingerprint)
	if avgScore < 0.4 {
		hints = append(hints, fmt.Sprintf("Historical UI effectiveness is low (%.0f%%) — try a significantly different approach", avgScore*100))
	} else if avgScore > 0.8 {
		hints = append(hints, fmt.Sprintf("Historical UI effectiveness is high (%.0f%%) — keep the current style", avgScore*100))
	}

	return hints
}

// reflectionHints generates hints from recent reflections (not fingerprint-specific).
func (b *HintBuilder) reflectionHints(fingerprint string) []string {
	records := b.reflection.Records()
	if len(records) == 0 {
		return nil
	}

	// Use only recent records (last 20).
	if len(records) > 20 {
		records = records[len(records)-20:]
	}

	var hints []string

	// Global dismiss rate.
	dismissed := 0
	for _, r := range records {
		if r.Dismissed {
			dismissed++
		}
	}
	if len(records) > 5 && float64(dismissed)/float64(len(records)) > 0.4 {
		hints = append(hints, "User dismisses UI frequently across all tasks — prefer concise output")
	}

	// Global scroll rate.
	scrolled := 0
	for _, r := range records {
		if r.Scrolled {
			scrolled++
		}
	}
	if len(records) > 5 && float64(scrolled)/float64(len(records)) > 0.6 {
		hints = append(hints, "User often scrolls across all tasks — reduce content density")
	}

	return hints
}

// deduplicateHints removes duplicate hints (case-insensitive comparison).
func deduplicateHints(hints []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, h := range hints {
		key := strings.ToLower(h)
		if !seen[key] {
			seen[key] = true
			result = append(result, h)
		}
	}
	return result
}
