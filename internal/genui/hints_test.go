package genui

import (
	"strings"
	"testing"
)

func TestNewHintBuilder(t *testing.T) {
	t.Run("default_maxHints", func(t *testing.T) {
		hb := NewHintBuilder(nil, nil, 0)
		if hb == nil {
			t.Fatal("expected non-nil HintBuilder")
		}
		if hb.maxHints != 10 {
			t.Errorf("expected default maxHints=10, got %d", hb.maxHints)
		}
	})

	t.Run("negative_maxHints", func(t *testing.T) {
		hb := NewHintBuilder(nil, nil, -1)
		if hb.maxHints != 10 {
			t.Errorf("expected default maxHints=10 for negative input, got %d", hb.maxHints)
		}
	})

	t.Run("custom_maxHints", func(t *testing.T) {
		hb := NewHintBuilder(nil, nil, 5)
		if hb.maxHints != 5 {
			t.Errorf("expected maxHints=5, got %d", hb.maxHints)
		}
	})

	t.Run("stores_memory_and_reflection", func(t *testing.T) {
		mem := NewUIMemory(50)
		refl := NewReflectionStore()
		hb := NewHintBuilder(mem, refl, 10)
		if hb.memory != mem {
			t.Error("expected memory to be stored")
		}
		if hb.reflection != refl {
			t.Error("expected reflection to be stored")
		}
	})
}

func TestHintBuilder_Build_NoData(t *testing.T) {
	mem := NewUIMemory(50)
	refl := NewReflectionStore()
	hb := NewHintBuilder(mem, refl, 10)

	hints := hb.Build("fp-empty")
	if len(hints) != 0 {
		t.Errorf("expected no hints for empty memory/reflection, got %d: %v", len(hints), hints)
	}
}

func TestHintBuilder_Build_MemoryOnly(t *testing.T) {
	mem := NewUIMemory(50)
	// Add enough entries to trigger hints (>2 required for scroll/dismiss patterns).
	for i := 0; i < 5; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-mem",
			Format:      FormatHTML,
			ActionsUsed: []string{"save"},
			Score:       0.85,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-mem")

	if len(hints) == 0 {
		t.Fatal("expected hints from memory data")
	}

	// Should have at least action hint and high score hint.
	hasActionHint := false
	for _, h := range hints {
		if strings.Contains(h, "save") {
			hasActionHint = true
		}
	}
	if !hasActionHint {
		t.Errorf("expected action hint mentioning 'save', got: %v", hints)
	}
}

func TestHintBuilder_Build_ReflectionOnly(t *testing.T) {
	refl := NewReflectionStore()
	// Add >5 records with high dismiss rate to trigger global hint.
	for i := 0; i < 8; i++ {
		refl.Record(UIReflection{
			TaskID:    "task-refl",
			UIFormat:  FormatANSI,
			Dismissed: true,
		})
	}

	hb := NewHintBuilder(nil, refl, 10)
	hints := hb.Build("fp-refl")

	if len(hints) == 0 {
		t.Fatal("expected hints from reflection data")
	}

	hasDismissHint := false
	for _, h := range hints {
		if strings.Contains(h, "dismisses UI frequently") {
			hasDismissHint = true
		}
	}
	if !hasDismissHint {
		t.Errorf("expected global dismiss hint, got: %v", hints)
	}
}

func TestHintBuilder_Build_Combined(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 5; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-combo",
			Format:      FormatHTML,
			ActionsUsed: []string{"deploy"},
			Score:       0.85,
		})
	}

	refl := NewReflectionStore()
	for i := 0; i < 8; i++ {
		refl.Record(UIReflection{
			TaskID:    "task-combo",
			UIFormat:  FormatANSI,
			Dismissed: true,
		})
	}

	hb := NewHintBuilder(mem, refl, 20)
	hints := hb.Build("fp-combo")

	if len(hints) < 2 {
		t.Fatalf("expected hints from both sources, got %d: %v", len(hints), hints)
	}

	hasMemoryHint := false
	hasReflectionHint := false
	for _, h := range hints {
		if strings.Contains(h, "deploy") {
			hasMemoryHint = true
		}
		if strings.Contains(h, "dismisses UI frequently") {
			hasReflectionHint = true
		}
	}

	if !hasMemoryHint {
		t.Errorf("expected memory-based hint, got: %v", hints)
	}
	if !hasReflectionHint {
		t.Errorf("expected reflection-based hint, got: %v", hints)
	}
}

func TestHintBuilder_FormatHint(t *testing.T) {
	mem := NewUIMemory(50)
	// All entries with HTML format and high scores (avg > 0.6).
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-format",
			Format:      FormatHTML,
			Score:       0.8,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-format")

	hasFormatHint := false
	for _, h := range hints {
		if strings.Contains(h, "Format") && strings.Contains(h, "html") && strings.Contains(h, "worked well") {
			hasFormatHint = true
		}
	}
	if !hasFormatHint {
		t.Errorf("expected format hint for high-scoring HTML, got: %v", hints)
	}
}

func TestHintBuilder_ActionHint(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-actions",
			Format:      FormatANSI,
			ActionsUsed: []string{"approve", "reject", "defer"},
			Score:       0.7,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-actions")

	hasActionHint := false
	for _, h := range hints {
		if strings.Contains(h, "frequently uses these actions") {
			hasActionHint = true
			// Verify top actions are mentioned.
			if !strings.Contains(h, "approve") {
				t.Errorf("expected 'approve' in action hint: %s", h)
			}
		}
	}
	if !hasActionHint {
		t.Errorf("expected action hint, got: %v", hints)
	}
}

func TestHintBuilder_ScrollHint(t *testing.T) {
	mem := NewUIMemory(50)
	// >50% scrolled entries (4 out of 5).
	for i := 0; i < 5; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-scroll",
			Format:      FormatANSI,
			Scrolled:    i < 4, // 4 out of 5 scrolled
			Score:       0.5,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-scroll")

	hasScrollHint := false
	for _, h := range hints {
		if strings.Contains(h, "often scrolls") && strings.Contains(h, "compact layout") {
			hasScrollHint = true
		}
	}
	if !hasScrollHint {
		t.Errorf("expected scroll pattern hint, got: %v", hints)
	}
}

func TestHintBuilder_DismissHint(t *testing.T) {
	mem := NewUIMemory(50)
	// >30% dismissed entries (3 out of 5).
	for i := 0; i < 5; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-dismiss",
			Format:      FormatANSI,
			Dismissed:   i < 3, // 3 out of 5 dismissed = 60%
			Score:       0.3,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-dismiss")

	hasDismissHint := false
	for _, h := range hints {
		if strings.Contains(h, "frequently dismisses") && strings.Contains(h, "simpler") {
			hasDismissHint = true
		}
	}
	if !hasDismissHint {
		t.Errorf("expected dismiss pattern hint, got: %v", hints)
	}
}

func TestHintBuilder_SpeedHint_Fast(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint:  "fp-fast",
			Format:       FormatANSI,
			TimeToAction: 1500, // < 3s
			Score:        0.7,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-fast")

	hasFastHint := false
	for _, h := range hints {
		if strings.Contains(h, "responds quickly") && strings.Contains(h, "intuitive") {
			hasFastHint = true
		}
	}
	if !hasFastHint {
		t.Errorf("expected fast interaction hint, got: %v", hints)
	}
}

func TestHintBuilder_SpeedHint_Slow(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint:  "fp-slow",
			Format:       FormatANSI,
			TimeToAction: 20000, // > 15s
			Score:        0.5,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-slow")

	hasSlowHint := false
	for _, h := range hints {
		if strings.Contains(h, "takes long") && strings.Contains(h, "clearer calls to action") {
			hasSlowHint = true
		}
	}
	if !hasSlowHint {
		t.Errorf("expected slow interaction hint, got: %v", hints)
	}
}

func TestHintBuilder_LowScoreHint(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-low",
			Format:      FormatANSI,
			Score:       0.2,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-low")

	hasLowHint := false
	for _, h := range hints {
		if strings.Contains(h, "effectiveness is low") && strings.Contains(h, "different approach") {
			hasLowHint = true
		}
	}
	if !hasLowHint {
		t.Errorf("expected low effectiveness hint, got: %v", hints)
	}
}

func TestHintBuilder_HighScoreHint(t *testing.T) {
	mem := NewUIMemory(50)
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-high",
			Format:      FormatANSI,
			Score:       0.9,
		})
	}

	hb := NewHintBuilder(mem, nil, 10)
	hints := hb.Build("fp-high")

	hasHighHint := false
	for _, h := range hints {
		if strings.Contains(h, "effectiveness is high") && strings.Contains(h, "keep the current style") {
			hasHighHint = true
		}
	}
	if !hasHighHint {
		t.Errorf("expected high effectiveness hint, got: %v", hints)
	}
}

func TestHintBuilder_ReflectionDismissRate(t *testing.T) {
	refl := NewReflectionStore()
	// >5 records, >40% dismissed.
	for i := 0; i < 10; i++ {
		refl.Record(UIReflection{
			TaskID:    "task-gdismiss",
			UIFormat:  FormatANSI,
			Dismissed: i < 6, // 6 out of 10 = 60%
		})
	}

	hb := NewHintBuilder(nil, refl, 10)
	hints := hb.Build("fp-any")

	hasDismissRate := false
	for _, h := range hints {
		if strings.Contains(h, "dismisses UI frequently across all tasks") {
			hasDismissRate = true
		}
	}
	if !hasDismissRate {
		t.Errorf("expected global dismiss rate hint, got: %v", hints)
	}
}

func TestHintBuilder_ReflectionScrollRate(t *testing.T) {
	refl := NewReflectionStore()
	// >5 records, >60% scrolled.
	for i := 0; i < 10; i++ {
		refl.Record(UIReflection{
			TaskID:   "task-gscroll",
			UIFormat: FormatANSI,
			Scrolled: i < 8, // 8 out of 10 = 80%
		})
	}

	hb := NewHintBuilder(nil, refl, 10)
	hints := hb.Build("fp-any")

	hasScrollRate := false
	for _, h := range hints {
		if strings.Contains(h, "often scrolls across all tasks") {
			hasScrollRate = true
		}
	}
	if !hasScrollRate {
		t.Errorf("expected global scroll rate hint, got: %v", hints)
	}
}

func TestHintBuilder_MaxHints(t *testing.T) {
	mem := NewUIMemory(50)
	refl := NewReflectionStore()

	// Fill memory with data that generates many hints.
	for i := 0; i < 10; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint:  "fp-max",
			Format:       FormatHTML,
			ActionsUsed:  []string{"a1", "a2", "a3"},
			TimeToAction: 20000,
			Scrolled:     true,
			Dismissed:    i%3 == 0,
			Score:        0.3,
		})
	}

	// Fill reflection to add more hints.
	for i := 0; i < 10; i++ {
		refl.Record(UIReflection{
			TaskID:   "task-max",
			UIFormat: FormatANSI,
			Scrolled: true,
		})
	}

	maxHints := 3
	hb := NewHintBuilder(mem, refl, maxHints)
	hints := hb.Build("fp-max")

	if len(hints) > maxHints {
		t.Errorf("expected at most %d hints, got %d: %v", maxHints, len(hints), hints)
	}
}

func TestHintBuilder_Deduplication(t *testing.T) {
	mem := NewUIMemory(50)

	// Create entries that produce the same action hint text.
	for i := 0; i < 4; i++ {
		mem.Record(UIMemoryEntry{
			Fingerprint: "fp-dedup",
			Format:      FormatANSI,
			ActionsUsed: []string{"save"},
			Score:       0.7,
		})
	}

	hb := NewHintBuilder(mem, nil, 20)
	hints := hb.Build("fp-dedup")

	// Count occurrences of each hint (case-insensitive).
	seen := make(map[string]int)
	for _, h := range hints {
		key := strings.ToLower(h)
		seen[key]++
	}

	for hint, count := range seen {
		if count > 1 {
			t.Errorf("hint duplicated %d times: %s", count, hint)
		}
	}
}
