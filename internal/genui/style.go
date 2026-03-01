package genui

import (
	"fmt"
	"sync"
)

// StylePreference represents a learned user style preference.
type StylePreference struct {
	// Density: "compact", "normal", "spacious"
	Density string `json:"density"`

	// Complexity: "minimal", "standard", "rich"
	Complexity string `json:"complexity"`

	// ColorScheme: "dark", "light", "auto"
	ColorScheme string `json:"color_scheme"`

	// ActionStyle: "buttons", "links", "cards"
	ActionStyle string `json:"action_style"`

	// PreferCharts indicates user prefers data visualizations.
	PreferCharts bool `json:"prefer_charts"`

	// Confidence is how sure we are about these preferences (0.0-1.0).
	Confidence float64 `json:"confidence"`
}

// DefaultStylePreference returns neutral defaults.
func DefaultStylePreference() StylePreference {
	return StylePreference{
		Density:      "normal",
		Complexity:   "standard",
		ColorScheme:  "dark",
		ActionStyle:  "buttons",
		PreferCharts: false,
		Confidence:   0.0,
	}
}

// StyleEvolution learns and evolves user style preferences from interaction data.
type StyleEvolution struct {
	mu         sync.Mutex
	preference StylePreference
	signals    []styleSignal
	maxSignals int
}

// styleSignal captures a single data point about user style preference.
type styleSignal struct {
	scrolled     bool
	dismissed    bool
	actionsUsed  int
	timeToAction int64
	format       UIFormat
}

// NewStyleEvolution creates a new style evolution engine.
func NewStyleEvolution() *StyleEvolution {
	return &StyleEvolution{
		preference: DefaultStylePreference(),
		maxSignals: 100,
	}
}

// LearnFrom processes a UI reflection to update style preferences.
func (se *StyleEvolution) LearnFrom(r UIReflection, format UIFormat) {
	se.mu.Lock()
	defer se.mu.Unlock()

	sig := styleSignal{
		scrolled:     r.Scrolled,
		dismissed:    r.Dismissed,
		actionsUsed:  len(r.ActionsUsed),
		timeToAction: r.TimeToAction,
		format:       format,
	}

	se.signals = append(se.signals, sig)
	if len(se.signals) > se.maxSignals {
		se.signals = se.signals[len(se.signals)-se.maxSignals:]
	}

	se.recompute()
}

// Preference returns the current style preference.
func (se *StyleEvolution) Preference() StylePreference {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.preference
}

// StyleHints generates prompt hints based on learned preferences.
func (se *StyleEvolution) StyleHints() []string {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.preference.Confidence < 0.3 || len(se.signals) < 5 {
		return nil // not enough data
	}

	var hints []string

	switch se.preference.Density {
	case "compact":
		hints = append(hints, "User prefers compact layouts — minimize whitespace and use concise text")
	case "spacious":
		hints = append(hints, "User prefers spacious layouts — use generous padding and clear visual sections")
	}

	switch se.preference.Complexity {
	case "minimal":
		hints = append(hints, "User prefers minimal UI — show only essential information and primary actions")
	case "rich":
		hints = append(hints, "User prefers rich UI — include details, secondary actions, and visual elements")
	}

	switch se.preference.ActionStyle {
	case "buttons":
		hints = append(hints, "User engages more with button-style actions")
	case "links":
		hints = append(hints, "User prefers link-style actions over buttons")
	case "cards":
		hints = append(hints, "User interacts well with card-based layouts")
	}

	if se.preference.PreferCharts {
		hints = append(hints, "User engages with data visualizations — include charts/graphs when relevant")
	}

	return hints
}

// Reset clears all learned preferences.
func (se *StyleEvolution) Reset() {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.preference = DefaultStylePreference()
	se.signals = nil
}

// SignalCount returns the number of collected signals.
func (se *StyleEvolution) SignalCount() int {
	se.mu.Lock()
	defer se.mu.Unlock()
	return len(se.signals)
}

// String returns a human-readable summary of current preferences.
func (se *StyleEvolution) String() string {
	se.mu.Lock()
	defer se.mu.Unlock()
	return fmt.Sprintf("density=%s complexity=%s colors=%s actions=%s charts=%v confidence=%.0f%%",
		se.preference.Density,
		se.preference.Complexity,
		se.preference.ColorScheme,
		se.preference.ActionStyle,
		se.preference.PreferCharts,
		se.preference.Confidence*100,
	)
}

// recompute recalculates preferences from all signals. Called under lock.
func (se *StyleEvolution) recompute() {
	n := len(se.signals)
	if n == 0 {
		return
	}

	// Density: based on scroll rate.
	scrollCount := 0
	for _, s := range se.signals {
		if s.scrolled {
			scrollCount++
		}
	}
	scrollRate := float64(scrollCount) / float64(n)
	if scrollRate > 0.6 {
		se.preference.Density = "compact"
	} else if scrollRate < 0.2 {
		se.preference.Density = "spacious"
	} else {
		se.preference.Density = "normal"
	}

	// Complexity: based on dismiss rate and action usage.
	dismissCount := 0
	totalActions := 0
	for _, s := range se.signals {
		if s.dismissed {
			dismissCount++
		}
		totalActions += s.actionsUsed
	}
	dismissRate := float64(dismissCount) / float64(n)
	avgActions := float64(totalActions) / float64(n)

	if dismissRate > 0.4 {
		se.preference.Complexity = "minimal" // they keep dismissing complex UIs
	} else if avgActions > 2.0 {
		se.preference.Complexity = "rich" // they engage with many actions
	} else {
		se.preference.Complexity = "standard"
	}

	// ActionStyle: based on action engagement rate.
	if avgActions > 3.0 {
		se.preference.ActionStyle = "cards" // deep engagement suggests card layouts work
	} else if avgActions > 1.0 {
		se.preference.ActionStyle = "buttons"
	} else {
		se.preference.ActionStyle = "links" // minimal interaction style
	}

	// Speed preference: fast interaction suggests charts/viz work.
	var totalTTA int64
	var ttaCount int
	for _, s := range se.signals {
		if s.timeToAction > 0 {
			totalTTA += s.timeToAction
			ttaCount++
		}
	}
	if ttaCount > 3 {
		avgTTA := totalTTA / int64(ttaCount)
		se.preference.PreferCharts = avgTTA < 5000 // quick responses = visual data works
	}

	// Confidence increases with more data.
	se.preference.Confidence = float64(n) / float64(se.maxSignals)
	if se.preference.Confidence > 1.0 {
		se.preference.Confidence = 1.0
	}
}
