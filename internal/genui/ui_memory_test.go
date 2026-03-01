package genui

import (
	"math"
	"sync"
	"testing"
)

func TestNewUIMemory(t *testing.T) {
	t.Run("default_maxPerFP", func(t *testing.T) {
		m := NewUIMemory(0)
		if m == nil {
			t.Fatal("expected non-nil UIMemory")
		}
		if m.maxPerFP != 50 {
			t.Errorf("expected default maxPerFP=50, got %d", m.maxPerFP)
		}
	})

	t.Run("negative_maxPerFP", func(t *testing.T) {
		m := NewUIMemory(-5)
		if m.maxPerFP != 50 {
			t.Errorf("expected default maxPerFP=50 for negative input, got %d", m.maxPerFP)
		}
	})

	t.Run("custom_maxPerFP", func(t *testing.T) {
		m := NewUIMemory(10)
		if m.maxPerFP != 10 {
			t.Errorf("expected maxPerFP=10, got %d", m.maxPerFP)
		}
	})

	t.Run("entries_initialized", func(t *testing.T) {
		m := NewUIMemory(5)
		if m.entries == nil {
			t.Fatal("expected entries map to be initialized")
		}
		if len(m.entries) != 0 {
			t.Errorf("expected empty entries map, got %d", len(m.entries))
		}
	})
}

func TestUIMemory_Record(t *testing.T) {
	m := NewUIMemory(50)

	entry := UIMemoryEntry{
		Fingerprint: "fp-weather",
		Format:      FormatANSI,
		PromptUsed:  "show weather",
		Score:       0.8,
	}
	m.Record(entry)

	entries := m.Lookup("fp-weather")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Fingerprint != "fp-weather" {
		t.Errorf("expected fingerprint fp-weather, got %s", entries[0].Fingerprint)
	}
	if entries[0].Format != FormatANSI {
		t.Errorf("expected format %s, got %s", FormatANSI, entries[0].Format)
	}
	if entries[0].Score != 0.8 {
		t.Errorf("expected score 0.8, got %f", entries[0].Score)
	}
	if entries[0].CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set automatically")
	}
}

func TestUIMemory_RecordEmpty(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{
		Fingerprint: "",
		Format:      FormatHTML,
		Score:       0.9,
	})

	if m.EntryCount() != 0 {
		t.Errorf("expected 0 entries for empty fingerprint, got %d", m.EntryCount())
	}
}

func TestUIMemory_RecordFromReflection(t *testing.T) {
	m := NewUIMemory(50)

	r := UIReflection{
		TaskID:       "task-42",
		UIFormat:     FormatHTML,
		ActionsUsed:  []string{"save", "preview"},
		TimeToAction: 2000,
		Scrolled:     false,
		Dismissed:    false,
	}

	m.RecordFromReflection("fp-editor", r, FormatHTML)

	entries := m.Lookup("fp-editor")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Fingerprint != "fp-editor" {
		t.Errorf("expected fingerprint fp-editor, got %s", e.Fingerprint)
	}
	if e.Format != FormatHTML {
		t.Errorf("expected format %s, got %s", FormatHTML, e.Format)
	}
	if len(e.ActionsUsed) != 2 {
		t.Errorf("expected 2 actions, got %d", len(e.ActionsUsed))
	}

	// Score should be computed from reflection: base 0.5 + actions 0.2 + fast(<3s) 0.15 = 0.85
	expectedScore := computeUIScore(r)
	if math.Abs(e.Score-expectedScore) > 0.001 {
		t.Errorf("expected score %f, got %f", expectedScore, e.Score)
	}
	if e.Score < 0.8 {
		t.Errorf("expected high score for engaged user, got %f", e.Score)
	}
}

func TestUIMemory_LookupSortedByScore(t *testing.T) {
	m := NewUIMemory(50)

	scores := []float64{0.3, 0.9, 0.1, 0.7, 0.5}
	for _, s := range scores {
		m.Record(UIMemoryEntry{
			Fingerprint: "fp-sort",
			Format:      FormatANSI,
			Score:       s,
		})
	}

	entries := m.Lookup("fp-sort")
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].Score > entries[i-1].Score {
			t.Errorf("entries not sorted descending at index %d: %f > %f",
				i, entries[i].Score, entries[i-1].Score)
		}
	}
}

func TestUIMemory_BestEntry(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{Fingerprint: "fp-best", Format: FormatANSI, Score: 0.3})
	m.Record(UIMemoryEntry{Fingerprint: "fp-best", Format: FormatHTML, Score: 0.95})
	m.Record(UIMemoryEntry{Fingerprint: "fp-best", Format: FormatANSI, Score: 0.6})

	best := m.BestEntry("fp-best")
	if best == nil {
		t.Fatal("expected non-nil best entry")
	}
	if best.Score != 0.95 {
		t.Errorf("expected best score 0.95, got %f", best.Score)
	}
	if best.Format != FormatHTML {
		t.Errorf("expected format %s, got %s", FormatHTML, best.Format)
	}
}

func TestUIMemory_BestEntry_Empty(t *testing.T) {
	m := NewUIMemory(50)

	best := m.BestEntry("nonexistent")
	if best != nil {
		t.Errorf("expected nil for unknown fingerprint, got %+v", best)
	}
}

func TestUIMemory_AverageScore(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{Fingerprint: "fp-avg", Score: 0.4})
	m.Record(UIMemoryEntry{Fingerprint: "fp-avg", Score: 0.6})
	m.Record(UIMemoryEntry{Fingerprint: "fp-avg", Score: 0.8})

	avg := m.AverageScore("fp-avg")
	expected := 0.6 // (0.4 + 0.6 + 0.8) / 3
	if math.Abs(avg-expected) > 0.001 {
		t.Errorf("expected average %f, got %f", expected, avg)
	}
}

func TestUIMemory_AverageScore_Empty(t *testing.T) {
	m := NewUIMemory(50)

	avg := m.AverageScore("unknown-fp")
	if avg != 0 {
		t.Errorf("expected 0 for unknown fingerprint, got %f", avg)
	}
}

func TestUIMemory_TrimToMax(t *testing.T) {
	maxPerFP := 3
	m := NewUIMemory(maxPerFP)

	// Record 5 entries with varying scores.
	scores := []float64{0.2, 0.8, 0.1, 0.9, 0.5}
	for _, s := range scores {
		m.Record(UIMemoryEntry{
			Fingerprint: "fp-trim",
			Format:      FormatANSI,
			Score:       s,
		})
	}

	entries := m.Lookup("fp-trim")
	if len(entries) != maxPerFP {
		t.Fatalf("expected %d entries after trim, got %d", maxPerFP, len(entries))
	}

	// Verify lowest-scored entries were evicted (0.1 and 0.2 should be gone).
	for _, e := range entries {
		if e.Score < 0.5 {
			t.Errorf("expected lowest scores to be evicted, found score %f", e.Score)
		}
	}

	// Verify the kept entries are the highest-scored ones.
	expectedKept := []float64{0.9, 0.8, 0.5}
	for i, want := range expectedKept {
		if math.Abs(entries[i].Score-want) > 0.001 {
			t.Errorf("entry[%d]: expected score %f, got %f", i, want, entries[i].Score)
		}
	}
}

func TestUIMemory_AllFingerprints(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{Fingerprint: "charlie", Score: 0.5})
	m.Record(UIMemoryEntry{Fingerprint: "alpha", Score: 0.5})
	m.Record(UIMemoryEntry{Fingerprint: "bravo", Score: 0.5})
	m.Record(UIMemoryEntry{Fingerprint: "alpha", Score: 0.7}) // duplicate fingerprint

	fps := m.AllFingerprints()
	if len(fps) != 3 {
		t.Fatalf("expected 3 fingerprints, got %d: %v", len(fps), fps)
	}

	expected := []string{"alpha", "bravo", "charlie"}
	for i, want := range expected {
		if fps[i] != want {
			t.Errorf("fingerprint[%d]: expected %s, got %s", i, want, fps[i])
		}
	}
}

func TestUIMemory_EntryCount(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{Fingerprint: "fp-a", Score: 0.5})
	m.Record(UIMemoryEntry{Fingerprint: "fp-a", Score: 0.6})
	m.Record(UIMemoryEntry{Fingerprint: "fp-b", Score: 0.7})

	count := m.EntryCount()
	if count != 3 {
		t.Errorf("expected 3 total entries, got %d", count)
	}
}

func TestUIMemory_Clear(t *testing.T) {
	m := NewUIMemory(50)

	m.Record(UIMemoryEntry{Fingerprint: "fp-x", Score: 0.5})
	m.Record(UIMemoryEntry{Fingerprint: "fp-y", Score: 0.6})

	if m.EntryCount() != 2 {
		t.Fatalf("expected 2 entries before clear, got %d", m.EntryCount())
	}

	m.Clear()

	if m.EntryCount() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", m.EntryCount())
	}
	if len(m.AllFingerprints()) != 0 {
		t.Errorf("expected no fingerprints after clear, got %v", m.AllFingerprints())
	}
}

func TestComputeUIScore_Dismissed(t *testing.T) {
	r := UIReflection{Dismissed: true}
	score := computeUIScore(r)
	if math.Abs(score-0.1) > 0.001 {
		t.Errorf("expected 0.1 for dismissed, got %f", score)
	}
}

func TestComputeUIScore_Base(t *testing.T) {
	r := UIReflection{} // no actions, no scroll, no dismiss
	score := computeUIScore(r)
	if math.Abs(score-0.5) > 0.001 {
		t.Errorf("expected base score 0.5, got %f", score)
	}
}

func TestComputeUIScore_ActionsUsed(t *testing.T) {
	t.Run("one_action", func(t *testing.T) {
		r := UIReflection{ActionsUsed: []string{"save"}}
		score := computeUIScore(r)
		// base 0.5 + actions 0.2 = 0.7
		if math.Abs(score-0.7) > 0.001 {
			t.Errorf("expected 0.7, got %f", score)
		}
	})

	t.Run("three_or_more_actions", func(t *testing.T) {
		r := UIReflection{ActionsUsed: []string{"save", "preview", "deploy"}}
		score := computeUIScore(r)
		// base 0.5 + actions 0.2 + deep 0.1 = 0.8
		if math.Abs(score-0.8) > 0.001 {
			t.Errorf("expected 0.8, got %f", score)
		}
	})
}

func TestComputeUIScore_QuickInteraction(t *testing.T) {
	t.Run("very_fast", func(t *testing.T) {
		r := UIReflection{TimeToAction: 1500} // < 3s
		score := computeUIScore(r)
		// base 0.5 + fast 0.15 = 0.65
		if math.Abs(score-0.65) > 0.001 {
			t.Errorf("expected 0.65, got %f", score)
		}
	})

	t.Run("moderate", func(t *testing.T) {
		r := UIReflection{TimeToAction: 5000} // < 10s
		score := computeUIScore(r)
		// base 0.5 + moderate 0.05 = 0.55
		if math.Abs(score-0.55) > 0.001 {
			t.Errorf("expected 0.55, got %f", score)
		}
	})

	t.Run("slow", func(t *testing.T) {
		r := UIReflection{TimeToAction: 20000} // > 10s, no bonus
		score := computeUIScore(r)
		// base 0.5 only
		if math.Abs(score-0.5) > 0.001 {
			t.Errorf("expected 0.5, got %f", score)
		}
	})
}

func TestComputeUIScore_Scrolled(t *testing.T) {
	r := UIReflection{Scrolled: true}
	score := computeUIScore(r)
	// base 0.5 - scroll 0.05 = 0.45
	if math.Abs(score-0.45) > 0.001 {
		t.Errorf("expected 0.45, got %f", score)
	}
}

func TestComputeUIScore_FullEngagement(t *testing.T) {
	r := UIReflection{
		ActionsUsed:  []string{"save", "preview", "deploy", "rollback"},
		TimeToAction: 1000, // very fast
		Scrolled:     false,
		Dismissed:    false,
	}
	score := computeUIScore(r)
	// base 0.5 + actions 0.2 + deep 0.1 + fast 0.15 = 0.95
	if math.Abs(score-0.95) > 0.001 {
		t.Errorf("expected 0.95, got %f", score)
	}
	if score < 0.9 {
		t.Errorf("expected high score for full engagement, got %f", score)
	}
}

func TestUIMemory_ConcurrentAccess(t *testing.T) {
	m := NewUIMemory(100)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Concurrent writes.
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			m.Record(UIMemoryEntry{
				Fingerprint: "fp-concurrent",
				Format:      FormatANSI,
				Score:       float64(n) / float64(goroutines),
			})
		}(i)
	}

	// Concurrent reads.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = m.Lookup("fp-concurrent")
			_ = m.BestEntry("fp-concurrent")
			_ = m.AverageScore("fp-concurrent")
			_ = m.AllFingerprints()
			_ = m.EntryCount()
		}()
	}

	wg.Wait()

	count := m.EntryCount()
	if count != goroutines {
		t.Errorf("expected %d entries, got %d", goroutines, count)
	}
}
