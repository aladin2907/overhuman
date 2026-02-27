package memory

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ShortTermMemory tests
// ---------------------------------------------------------------------------

func TestShortTermMemory_AddAndGetRecent(t *testing.T) {
	stm := NewShortTermMemory(10)

	stm.Add("user", "hello", nil)
	stm.Add("assistant", "hi there", map[string]string{"model": "gpt-4"})
	stm.Add("user", "how are you?", nil)

	if stm.Len() != 3 {
		t.Fatalf("expected Len()=3, got %d", stm.Len())
	}

	recent := stm.GetRecent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
	// Should be chronological: second-to-last, then last.
	if recent[0].Content != "hi there" {
		t.Errorf("expected recent[0].Content='hi there', got %q", recent[0].Content)
	}
	if recent[1].Content != "how are you?" {
		t.Errorf("expected recent[1].Content='how are you?', got %q", recent[1].Content)
	}
}

func TestShortTermMemory_GetRecentMoreThanAvailable(t *testing.T) {
	stm := NewShortTermMemory(10)
	stm.Add("user", "one", nil)

	recent := stm.GetRecent(100)
	if len(recent) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(recent))
	}
}

func TestShortTermMemory_RingBufferOverflow(t *testing.T) {
	stm := NewShortTermMemory(3)

	stm.Add("user", "msg-1", nil)
	stm.Add("user", "msg-2", nil)
	stm.Add("user", "msg-3", nil)
	// Buffer full — next add should overwrite the oldest.
	stm.Add("user", "msg-4", nil)

	if stm.Len() != 3 {
		t.Fatalf("expected Len()=3 after overflow, got %d", stm.Len())
	}

	all := stm.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}

	// Oldest entry ("msg-1") should have been evicted.
	contents := make([]string, len(all))
	for i, e := range all {
		contents[i] = e.Content
	}
	if contents[0] != "msg-2" || contents[1] != "msg-3" || contents[2] != "msg-4" {
		t.Errorf("unexpected order after overflow: %v", contents)
	}
}

func TestShortTermMemory_Clear(t *testing.T) {
	stm := NewShortTermMemory(10)
	stm.Add("user", "something", nil)
	stm.Clear()

	if stm.Len() != 0 {
		t.Fatalf("expected Len()=0 after Clear, got %d", stm.Len())
	}
	if len(stm.GetAll()) != 0 {
		t.Fatal("expected empty slice after Clear")
	}
}

func TestShortTermMemory_ConcurrentAccess(t *testing.T) {
	stm := NewShortTermMemory(100)
	var wg sync.WaitGroup

	// Spin up writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				stm.Add("user", "concurrent", nil)
			}
		}()
	}

	// Spin up readers.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = stm.GetRecent(10)
				_ = stm.GetAll()
				_ = stm.Len()
			}
		}()
	}

	wg.Wait()
	// No data race = pass.
}

func TestShortTermMemory_DefaultMaxSize(t *testing.T) {
	stm := NewShortTermMemory(0)
	// Should default to 50.
	for i := 0; i < 60; i++ {
		stm.Add("user", "x", nil)
	}
	if stm.Len() != 50 {
		t.Fatalf("expected Len()=50 with default max, got %d", stm.Len())
	}
}

// ---------------------------------------------------------------------------
// LongTermMemory tests
// ---------------------------------------------------------------------------

func tempDBPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.db")
}

func TestLongTermMemory_StoreAndGetAll(t *testing.T) {
	ltm, err := NewLongTermMemory(tempDBPath(t))
	if err != nil {
		t.Fatalf("NewLongTermMemory: %v", err)
	}
	defer ltm.Close()

	entry := LongTermEntry{
		ID:          "lt-1",
		Summary:     "User prefers concise answers",
		Tags:        []string{"preference", "style"},
		SourceRunID: "run-001",
		CreatedAt:   time.Now(),
	}
	if err := ltm.Store(entry); err != nil {
		t.Fatalf("Store: %v", err)
	}

	entries, err := ltm.GetAll(10)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Summary != entry.Summary {
		t.Errorf("summary mismatch: %q vs %q", entries[0].Summary, entry.Summary)
	}
	if len(entries[0].Tags) != 2 || entries[0].Tags[0] != "preference" {
		t.Errorf("tags mismatch: %v", entries[0].Tags)
	}
}

func TestLongTermMemory_SearchFTS5(t *testing.T) {
	ltm, err := NewLongTermMemory(tempDBPath(t))
	if err != nil {
		t.Fatalf("NewLongTermMemory: %v", err)
	}
	defer ltm.Close()

	entries := []LongTermEntry{
		{ID: "lt-a", Summary: "Learned Go concurrency patterns", Tags: []string{"go", "concurrency"}, SourceRunID: "run-1", CreatedAt: time.Now()},
		{ID: "lt-b", Summary: "Python data analysis pipeline", Tags: []string{"python", "data"}, SourceRunID: "run-2", CreatedAt: time.Now()},
		{ID: "lt-c", Summary: "Go testing best practices", Tags: []string{"go", "testing"}, SourceRunID: "run-3", CreatedAt: time.Now()},
	}
	for _, e := range entries {
		if err := ltm.Store(e); err != nil {
			t.Fatalf("Store(%s): %v", e.ID, err)
		}
	}

	// Search for "Go" — should match lt-a and lt-c.
	results, err := ltm.Search("Go", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'Go', got %d", len(results))
	}

	// Search for "Python" — should match lt-b only.
	results, err = ltm.Search("Python", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'Python', got %d", len(results))
	}
	if results[0].ID != "lt-b" {
		t.Errorf("expected ID 'lt-b', got %q", results[0].ID)
	}
}

func TestLongTermMemory_Close(t *testing.T) {
	ltm, err := NewLongTermMemory(tempDBPath(t))
	if err != nil {
		t.Fatalf("NewLongTermMemory: %v", err)
	}
	if err := ltm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// After close, operations should fail.
	err = ltm.Store(LongTermEntry{ID: "x", Summary: "x", CreatedAt: time.Now()})
	if err == nil {
		t.Fatal("expected error after Close, got nil")
	}
}

// ---------------------------------------------------------------------------
// PatternTracker tests
// ---------------------------------------------------------------------------

func newTestPatternTracker(t *testing.T) *PatternTracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "patterns.db")
	ltm, err := NewLongTermMemory(dbPath)
	if err != nil {
		t.Fatalf("NewLongTermMemory for patterns: %v", err)
	}
	t.Cleanup(func() { ltm.Close() })

	pt, err := NewPatternTracker(ltm.DB())
	if err != nil {
		t.Fatalf("NewPatternTracker: %v", err)
	}
	return pt
}

func TestPatternTracker_FingerprintDeterministic(t *testing.T) {
	pt := newTestPatternTracker(t)

	fp1 := pt.ComputeFingerprint("deploy service", "devops")
	fp2 := pt.ComputeFingerprint("deploy service", "devops")
	if fp1 != fp2 {
		t.Fatalf("fingerprints should be deterministic: %q != %q", fp1, fp2)
	}

	fp3 := pt.ComputeFingerprint("deploy service", "infra")
	if fp1 == fp3 {
		t.Fatal("different inputs should produce different fingerprints")
	}
}

func TestPatternTracker_RecordIncrementsCount(t *testing.T) {
	pt := newTestPatternTracker(t)

	fp := pt.ComputeFingerprint("write tests", "testing")

	e1, err := pt.Record(fp, "writing unit tests", 0.8)
	if err != nil {
		t.Fatalf("Record #1: %v", err)
	}
	if e1.Count != 1 {
		t.Fatalf("expected count=1, got %d", e1.Count)
	}

	e2, err := pt.Record(fp, "writing unit tests", 0.9)
	if err != nil {
		t.Fatalf("Record #2: %v", err)
	}
	if e2.Count != 2 {
		t.Fatalf("expected count=2, got %d", e2.Count)
	}

	e3, err := pt.Record(fp, "writing unit tests", 1.0)
	if err != nil {
		t.Fatalf("Record #3: %v", err)
	}
	if e3.Count != 3 {
		t.Fatalf("expected count=3, got %d", e3.Count)
	}
}

func TestPatternTracker_AvgQualityCalculation(t *testing.T) {
	pt := newTestPatternTracker(t)

	fp := pt.ComputeFingerprint("refactor code", "engineering")

	// Record three observations: 0.6, 0.8, 1.0 => average should be 0.8.
	pt.Record(fp, "refactoring", 0.6)
	pt.Record(fp, "refactoring", 0.8)
	entry, err := pt.Record(fp, "refactoring", 1.0)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	expected := 0.8
	if math.Abs(entry.AvgQuality-expected) > 0.01 {
		t.Fatalf("expected avg_quality ~%.2f, got %.4f", expected, entry.AvgQuality)
	}
}

func TestPatternTracker_GetAutomatable(t *testing.T) {
	pt := newTestPatternTracker(t)

	fp1 := pt.ComputeFingerprint("task A", "type1")
	fp2 := pt.ComputeFingerprint("task B", "type2")

	// fp1: 3 occurrences
	pt.Record(fp1, "task A", 0.9)
	pt.Record(fp1, "task A", 0.9)
	pt.Record(fp1, "task A", 0.9)

	// fp2: 1 occurrence
	pt.Record(fp2, "task B", 0.5)

	// Threshold = 3: only fp1 should be returned.
	automatable, err := pt.GetAutomatable(3)
	if err != nil {
		t.Fatalf("GetAutomatable: %v", err)
	}
	if len(automatable) != 1 {
		t.Fatalf("expected 1 automatable pattern, got %d", len(automatable))
	}
	if automatable[0].Fingerprint != fp1 {
		t.Errorf("expected fingerprint %q, got %q", fp1, automatable[0].Fingerprint)
	}
}

func TestPatternTracker_LinkSkillExcludesFromAutomatable(t *testing.T) {
	pt := newTestPatternTracker(t)

	fp := pt.ComputeFingerprint("repeated task", "ops")
	pt.Record(fp, "repeated task", 0.9)
	pt.Record(fp, "repeated task", 0.9)
	pt.Record(fp, "repeated task", 0.9)

	// Before linking, it should be automatable.
	automatable, err := pt.GetAutomatable(3)
	if err != nil {
		t.Fatalf("GetAutomatable: %v", err)
	}
	if len(automatable) != 1 {
		t.Fatalf("expected 1 automatable, got %d", len(automatable))
	}

	// Link a skill.
	if err := pt.LinkSkill(fp, "skill-42"); err != nil {
		t.Fatalf("LinkSkill: %v", err)
	}

	// After linking, it should no longer appear.
	automatable, err = pt.GetAutomatable(3)
	if err != nil {
		t.Fatalf("GetAutomatable after link: %v", err)
	}
	if len(automatable) != 0 {
		t.Fatalf("expected 0 automatable after linking skill, got %d", len(automatable))
	}

	// Verify the skill ID is stored.
	entry, err := pt.Get(fp)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.SkillID != "skill-42" {
		t.Errorf("expected SkillID='skill-42', got %q", entry.SkillID)
	}
}

func TestPatternTracker_LinkSkillNotFound(t *testing.T) {
	pt := newTestPatternTracker(t)

	err := pt.LinkSkill("nonexistent", "skill-1")
	if err == nil {
		t.Fatal("expected error linking skill to nonexistent fingerprint")
	}
}

// Ensure temp files are cleaned up properly.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
