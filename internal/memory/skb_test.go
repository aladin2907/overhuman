package memory

import (
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupSKB(t *testing.T) *SharedKnowledgeBase {
	t.Helper()

	dir, err := os.MkdirTemp("", "skb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	ltm, err := NewLongTermMemory(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ltm.Close() })

	skb, err := NewSharedKnowledgeBase(ltm.DB())
	if err != nil {
		t.Fatal(err)
	}
	return skb
}

func TestSKB_StoreAndSearch(t *testing.T) {
	skb := setupSKB(t)

	entry := SKBEntry{
		ID:          "skb_1",
		Type:        SKBPattern,
		SourceAgent: "agent_main",
		Content:     "Summarization tasks work best with code-skill caching",
		Tags:        []string{"summarization", "caching"},
		Fitness:     0.85,
		CreatedAt:   time.Now(),
	}

	if err := skb.Store(entry); err != nil {
		t.Fatalf("Store: %v", err)
	}

	results, err := skb.Search("summarization", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search results = %d, want 1", len(results))
	}
	if results[0].ID != "skb_1" {
		t.Errorf("ID = %q", results[0].ID)
	}
	if results[0].Fitness != 0.85 {
		t.Errorf("Fitness = %f", results[0].Fitness)
	}
}

func TestSKB_SearchByTags(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "a", Type: SKBInsight, SourceAgent: "x", Content: "insight A", Tags: []string{"quality"}, Fitness: 0.7, CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "b", Type: SKBInsight, SourceAgent: "x", Content: "insight B", Tags: []string{"cost"}, Fitness: 0.9, CreatedAt: time.Now()})

	results, err := skb.Search("quality", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "a" {
		t.Errorf("expected entry 'a', got %v", results)
	}
}

func TestSKB_FindByType(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "p1", Type: SKBPattern, SourceAgent: "a", Content: "pattern", CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "i1", Type: SKBInsight, SourceAgent: "a", Content: "insight", CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "p2", Type: SKBPattern, SourceAgent: "a", Content: "another pattern", CreatedAt: time.Now()})

	patterns, err := skb.FindByType(SKBPattern, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 {
		t.Errorf("patterns = %d, want 2", len(patterns))
	}
}

func TestSKB_FindByAgent(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "1", Type: SKBInsight, SourceAgent: "alice", Content: "from alice", CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "2", Type: SKBInsight, SourceAgent: "bob", Content: "from bob", CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "3", Type: SKBInsight, SourceAgent: "alice", Content: "more alice", CreatedAt: time.Now()})

	results, err := skb.FindByAgent("alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("alice entries = %d, want 2", len(results))
	}
}

func TestSKB_TopEntries(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "low", Type: SKBInsight, SourceAgent: "a", Content: "low", Fitness: 0.2, CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "high", Type: SKBInsight, SourceAgent: "a", Content: "high", Fitness: 0.9, CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "mid", Type: SKBInsight, SourceAgent: "a", Content: "mid", Fitness: 0.5, CreatedAt: time.Now()})

	top, err := skb.TopEntries(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 {
		t.Fatalf("top = %d, want 2", len(top))
	}
	if top[0].ID != "high" {
		t.Errorf("top[0] = %q, want 'high'", top[0].ID)
	}
}

func TestSKB_RecordUsage(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "s1", Type: SKBSkill, SourceAgent: "a", Content: "skill", Fitness: 0.5, UsageCount: 0, CreatedAt: time.Now()})

	if err := skb.RecordUsage("s1", 0.8); err != nil {
		t.Fatal(err)
	}

	results, _ := skb.Search("skill", 1)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].UsageCount != 1 {
		t.Errorf("UsageCount = %d, want 1", results[0].UsageCount)
	}
}

func TestSKB_Delete(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "del1", Type: SKBInsight, SourceAgent: "a", Content: "will delete", CreatedAt: time.Now()})

	count, _ := skb.Count()
	if count != 1 {
		t.Fatalf("Count = %d, want 1", count)
	}

	if err := skb.Delete("del1"); err != nil {
		t.Fatal(err)
	}

	count, _ = skb.Count()
	if count != 0 {
		t.Errorf("Count after delete = %d, want 0", count)
	}
}

func TestSKB_Count(t *testing.T) {
	skb := setupSKB(t)

	count, err := skb.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("empty SKB count = %d", count)
	}

	skb.Store(SKBEntry{ID: "a", Type: SKBPattern, SourceAgent: "x", Content: "a", CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "b", Type: SKBPattern, SourceAgent: "x", Content: "b", CreatedAt: time.Now()})

	count, _ = skb.Count()
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}
}

func TestSKB_Upsert(t *testing.T) {
	skb := setupSKB(t)

	skb.Store(SKBEntry{ID: "up1", Type: SKBInsight, SourceAgent: "a", Content: "version 1", Fitness: 0.5, CreatedAt: time.Now()})
	skb.Store(SKBEntry{ID: "up1", Type: SKBInsight, SourceAgent: "a", Content: "version 2", Fitness: 0.8, CreatedAt: time.Now()})

	count, _ := skb.Count()
	if count != 1 {
		t.Errorf("Count after upsert = %d, want 1", count)
	}

	results, _ := skb.Search("version", 10)
	if len(results) != 1 {
		t.Fatal("expected 1")
	}
	if results[0].Content != "version 2" {
		t.Errorf("Content = %q, want 'version 2'", results[0].Content)
	}
	if results[0].Fitness != 0.8 {
		t.Errorf("Fitness = %f, want 0.8", results[0].Fitness)
	}
}

func TestSKB_Propagate(t *testing.T) {
	source := setupSKB(t)
	target := setupSKB(t)

	source.Store(SKBEntry{ID: "s1", Type: SKBInsight, SourceAgent: "child", Content: "good insight", Fitness: 0.9, CreatedAt: time.Now()})
	source.Store(SKBEntry{ID: "s2", Type: SKBInsight, SourceAgent: "child", Content: "bad insight", Fitness: 0.2, CreatedAt: time.Now()})
	source.Store(SKBEntry{ID: "s3", Type: SKBInsight, SourceAgent: "child", Content: "ok insight", Fitness: 0.7, CreatedAt: time.Now()})

	count, err := source.Propagate(target, SKBDirUp, 0.6, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("propagated = %d, want 2 (fitness >= 0.6)", count)
	}

	targetCount, _ := target.Count()
	if targetCount != 2 {
		t.Errorf("target count = %d, want 2", targetCount)
	}
}

func TestSKB_SearchNoResults(t *testing.T) {
	skb := setupSKB(t)

	results, err := skb.Search("nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
