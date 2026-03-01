package genui

import (
	"strings"
	"testing"
)

func TestDefaultStylePreference(t *testing.T) {
	pref := DefaultStylePreference()

	if pref.Density != "normal" {
		t.Errorf("Density = %q, want normal", pref.Density)
	}
	if pref.Complexity != "standard" {
		t.Errorf("Complexity = %q, want standard", pref.Complexity)
	}
	if pref.ColorScheme != "dark" {
		t.Errorf("ColorScheme = %q, want dark", pref.ColorScheme)
	}
	if pref.ActionStyle != "buttons" {
		t.Errorf("ActionStyle = %q, want buttons", pref.ActionStyle)
	}
	if pref.PreferCharts {
		t.Error("PreferCharts should be false by default")
	}
	if pref.Confidence != 0.0 {
		t.Errorf("Confidence = %f, want 0.0", pref.Confidence)
	}
}

func TestNewStyleEvolution(t *testing.T) {
	se := NewStyleEvolution()
	if se == nil {
		t.Fatal("NewStyleEvolution returned nil")
	}

	pref := se.Preference()
	defaults := DefaultStylePreference()

	if pref.Density != defaults.Density {
		t.Errorf("Density = %q, want %q", pref.Density, defaults.Density)
	}
	if pref.Complexity != defaults.Complexity {
		t.Errorf("Complexity = %q, want %q", pref.Complexity, defaults.Complexity)
	}
	if pref.ColorScheme != defaults.ColorScheme {
		t.Errorf("ColorScheme = %q, want %q", pref.ColorScheme, defaults.ColorScheme)
	}
	if pref.ActionStyle != defaults.ActionStyle {
		t.Errorf("ActionStyle = %q, want %q", pref.ActionStyle, defaults.ActionStyle)
	}
	if se.SignalCount() != 0 {
		t.Errorf("SignalCount = %d, want 0", se.SignalCount())
	}
	if se.maxSignals != 100 {
		t.Errorf("maxSignals = %d, want 100", se.maxSignals)
	}
}

func TestStyleEvolution_LearnFrom_Single(t *testing.T) {
	se := NewStyleEvolution()

	r := UIReflection{
		TaskID:      "task-s1",
		Scrolled:    false,
		Dismissed:   false,
		ActionsUsed: []string{"apply"},
	}
	se.LearnFrom(r, FormatANSI)

	if se.SignalCount() != 1 {
		t.Errorf("SignalCount = %d, want 1", se.SignalCount())
	}

	pref := se.Preference()
	// With 1 signal, confidence = 1/100 = 0.01.
	if pref.Confidence < 0.005 || pref.Confidence > 0.02 {
		t.Errorf("Confidence = %f, want ~0.01", pref.Confidence)
	}
}

func TestStyleEvolution_LearnFrom_Scrolling(t *testing.T) {
	se := NewStyleEvolution()

	// Send 10 signals, all with scrolling.
	for i := 0; i < 10; i++ {
		r := UIReflection{
			TaskID:   "task-scroll",
			Scrolled: true,
		}
		se.LearnFrom(r, FormatANSI)
	}

	pref := se.Preference()
	// scrollRate = 10/10 = 1.0 > 0.6 → compact.
	if pref.Density != "compact" {
		t.Errorf("Density = %q, want compact (high scroll rate)", pref.Density)
	}
}

func TestStyleEvolution_LearnFrom_Dismissed(t *testing.T) {
	se := NewStyleEvolution()

	// Send 10 signals, all dismissed.
	for i := 0; i < 10; i++ {
		r := UIReflection{
			TaskID:    "task-dismiss",
			Dismissed: true,
		}
		se.LearnFrom(r, FormatANSI)
	}

	pref := se.Preference()
	// dismissRate = 10/10 = 1.0 > 0.4 → minimal.
	if pref.Complexity != "minimal" {
		t.Errorf("Complexity = %q, want minimal (high dismiss rate)", pref.Complexity)
	}
}

func TestStyleEvolution_LearnFrom_HighActions(t *testing.T) {
	se := NewStyleEvolution()

	// Send 10 signals with 4 actions each.
	for i := 0; i < 10; i++ {
		r := UIReflection{
			TaskID:      "task-actions",
			Dismissed:   false,
			ActionsUsed: []string{"a1", "a2", "a3", "a4"},
		}
		se.LearnFrom(r, FormatANSI)
	}

	pref := se.Preference()
	// avgActions = 4.0 > 2.0 → rich complexity.
	if pref.Complexity != "rich" {
		t.Errorf("Complexity = %q, want rich (high action usage)", pref.Complexity)
	}
	// avgActions = 4.0 > 3.0 → cards action style.
	if pref.ActionStyle != "cards" {
		t.Errorf("ActionStyle = %q, want cards (high action usage)", pref.ActionStyle)
	}
}

func TestStyleEvolution_Preference(t *testing.T) {
	se := NewStyleEvolution()

	pref := se.Preference()
	if pref.Density != "normal" {
		t.Errorf("Density = %q, want normal", pref.Density)
	}

	// After learning, preference should update.
	for i := 0; i < 10; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-p", Scrolled: true}, FormatANSI)
	}
	pref2 := se.Preference()
	if pref2.Density != "compact" {
		t.Errorf("after scrolling signals, Density = %q, want compact", pref2.Density)
	}
}

func TestStyleEvolution_StyleHints_LowConfidence(t *testing.T) {
	se := NewStyleEvolution()

	// No signals → confidence = 0 → no hints.
	hints := se.StyleHints()
	if hints != nil {
		t.Errorf("expected nil hints with no signals, got %v", hints)
	}

	// Add only 2 signals (below 5 minimum).
	for i := 0; i < 2; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-low", Scrolled: true}, FormatANSI)
	}
	hints = se.StyleHints()
	if hints != nil {
		t.Errorf("expected nil hints with fewer than 5 signals, got %v", hints)
	}
}

func TestStyleEvolution_StyleHints_Compact(t *testing.T) {
	se := NewStyleEvolution()

	// Need at least 5 signals and confidence >= 0.3 (i.e., 30 signals for maxSignals=100).
	for i := 0; i < 35; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-compact", Scrolled: true}, FormatANSI)
	}

	hints := se.StyleHints()
	found := false
	for _, h := range hints {
		if strings.Contains(h, "compact") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected compact density hint, got %v", hints)
	}
}

func TestStyleEvolution_StyleHints_Minimal(t *testing.T) {
	se := NewStyleEvolution()

	// 35 dismissed signals → high dismiss rate → minimal complexity.
	for i := 0; i < 35; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-min", Dismissed: true}, FormatANSI)
	}

	hints := se.StyleHints()
	found := false
	for _, h := range hints {
		if strings.Contains(h, "minimal") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected minimal complexity hint, got %v", hints)
	}
}

func TestStyleEvolution_StyleHints_Rich(t *testing.T) {
	se := NewStyleEvolution()

	// 35 signals with many actions → rich complexity.
	for i := 0; i < 35; i++ {
		se.LearnFrom(UIReflection{
			TaskID:      "task-rich",
			Dismissed:   false,
			ActionsUsed: []string{"a1", "a2", "a3"},
		}, FormatANSI)
	}

	hints := se.StyleHints()
	found := false
	for _, h := range hints {
		if strings.Contains(h, "rich") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rich complexity hint, got %v", hints)
	}
}

func TestStyleEvolution_Reset(t *testing.T) {
	se := NewStyleEvolution()

	// Add some signals.
	for i := 0; i < 10; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-reset", Scrolled: true}, FormatANSI)
	}
	if se.SignalCount() != 10 {
		t.Fatalf("expected 10 signals before reset, got %d", se.SignalCount())
	}

	se.Reset()

	if se.SignalCount() != 0 {
		t.Errorf("SignalCount after reset = %d, want 0", se.SignalCount())
	}

	pref := se.Preference()
	defaults := DefaultStylePreference()
	if pref.Density != defaults.Density {
		t.Errorf("after reset Density = %q, want %q", pref.Density, defaults.Density)
	}
	if pref.Complexity != defaults.Complexity {
		t.Errorf("after reset Complexity = %q, want %q", pref.Complexity, defaults.Complexity)
	}
	if pref.Confidence != 0.0 {
		t.Errorf("after reset Confidence = %f, want 0.0", pref.Confidence)
	}
}

func TestStyleEvolution_SignalCount(t *testing.T) {
	se := NewStyleEvolution()

	if se.SignalCount() != 0 {
		t.Errorf("initial SignalCount = %d, want 0", se.SignalCount())
	}

	for i := 1; i <= 5; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-count"}, FormatANSI)
		if se.SignalCount() != i {
			t.Errorf("after %d signals, SignalCount = %d", i, se.SignalCount())
		}
	}
}

func TestStyleEvolution_String(t *testing.T) {
	se := NewStyleEvolution()

	s := se.String()
	if !strings.Contains(s, "density=normal") {
		t.Errorf("String should contain 'density=normal', got %q", s)
	}
	if !strings.Contains(s, "complexity=standard") {
		t.Errorf("String should contain 'complexity=standard', got %q", s)
	}
	if !strings.Contains(s, "colors=dark") {
		t.Errorf("String should contain 'colors=dark', got %q", s)
	}
	if !strings.Contains(s, "actions=buttons") {
		t.Errorf("String should contain 'actions=buttons', got %q", s)
	}
	if !strings.Contains(s, "charts=false") {
		t.Errorf("String should contain 'charts=false', got %q", s)
	}
	if !strings.Contains(s, "confidence=0%") {
		t.Errorf("String should contain 'confidence=0%%', got %q", s)
	}
}

func TestStyleEvolution_MaxSignals(t *testing.T) {
	se := NewStyleEvolution()

	// Feed 120 signals — should be trimmed to maxSignals (100).
	for i := 0; i < 120; i++ {
		se.LearnFrom(UIReflection{TaskID: "task-max"}, FormatANSI)
	}

	if se.SignalCount() != 100 {
		t.Errorf("SignalCount = %d, want 100 (capped at maxSignals)", se.SignalCount())
	}

	// Confidence should be capped at 1.0.
	pref := se.Preference()
	if pref.Confidence > 1.0 {
		t.Errorf("Confidence = %f, should be capped at 1.0", pref.Confidence)
	}
}

func TestStyleEvolution_ChartPreference(t *testing.T) {
	se := NewStyleEvolution()

	// Feed signals with fast timeToAction (< 5000ms) to trigger PreferCharts.
	// Need > 3 signals with timeToAction > 0.
	for i := 0; i < 10; i++ {
		se.LearnFrom(UIReflection{
			TaskID:       "task-chart",
			TimeToAction: 2000, // 2 seconds — fast interaction
			ActionsUsed:  []string{"click"},
		}, FormatANSI)
	}

	pref := se.Preference()
	if !pref.PreferCharts {
		t.Error("PreferCharts should be true when avg timeToAction < 5000ms")
	}
}
