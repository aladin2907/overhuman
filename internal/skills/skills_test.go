package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/overhuman/overhuman/internal/instruments"
	"github.com/overhuman/overhuman/internal/storage"
)

func newTestStore(t *testing.T) storage.Store {
	t.Helper()
	s, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Registration tests ---

func TestAllSkills(t *testing.T) {
	defs := AllSkills(Config{})
	if len(defs) != 20 {
		t.Errorf("AllSkills = %d, want 20", len(defs))
	}
	ids := make(map[string]bool)
	for _, d := range defs {
		if ids[d.ID] {
			t.Errorf("duplicate ID: %s", d.ID)
		}
		ids[d.ID] = true
		if d.Name == "" {
			t.Errorf("skill %s has empty name", d.ID)
		}
		if d.Executor == nil {
			t.Errorf("skill %s has nil executor", d.ID)
		}
	}
}

func TestRegisterAll(t *testing.T) {
	registry := instruments.NewSkillRegistry()
	count := RegisterAll(registry, Config{})
	if count != 20 {
		t.Errorf("RegisterAll = %d, want 20", count)
	}
	if registry.Count() != 20 {
		t.Errorf("registry.Count = %d", registry.Count())
	}

	// Second call should register 0 (no duplicates).
	count2 := RegisterAll(registry, Config{})
	if count2 != 0 {
		t.Errorf("second RegisterAll = %d, want 0", count2)
	}
}

// --- Stub skill tests ---

func TestStubSkill(t *testing.T) {
	s := NewStubSkill("test", "not configured")
	output, err := s.Execute(context.Background(), instruments.SkillInput{Goal: "do something"})
	if err != nil {
		t.Fatal(err)
	}
	if output.Success {
		t.Error("stub should not succeed")
	}
	if output.Error == "" {
		t.Error("stub should have error message")
	}
}

// --- Code Execution tests ---

func TestCodeExecSkill_NoSandbox(t *testing.T) {
	s := NewCodeExecSkill(nil)
	output, err := s.Execute(context.Background(), instruments.SkillInput{Goal: "print(1)"})
	if err == nil {
		t.Error("expected error with nil sandbox")
	}
	if output.Success {
		t.Error("should not succeed")
	}
}

// --- Git Skill tests ---

func TestGitSkill_ReadOnly(t *testing.T) {
	s := NewGitSkill("/tmp")
	output, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"command": "status"},
	})
	if !output.Success {
		t.Errorf("git status should succeed: %s", output.Error)
	}
}

func TestGitSkill_BlocksWrite(t *testing.T) {
	s := NewGitSkill("/tmp")
	output, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"command": "push origin main"},
	})
	if output.Success {
		t.Error("push should be blocked")
	}
}

// --- Testing Skill ---

func TestTestingSkill_NoCode(t *testing.T) {
	s := NewTestingSkill(nil)
	output, _ := s.Execute(context.Background(), instruments.SkillInput{})
	if output.Success {
		t.Error("should fail without test_code")
	}
}

func TestTestingSkill_NoSandbox(t *testing.T) {
	s := NewTestingSkill(nil)
	output, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"test_code": "assert True"},
	})
	if !output.Success {
		t.Errorf("should succeed in passthrough mode: %s", output.Error)
	}
}

// --- File Operations tests ---

func TestFileOpsSkill_ReadWrite(t *testing.T) {
	dir := t.TempDir()
	s := NewFileOpsSkill(dir)
	ctx := context.Background()

	// Write.
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{
			"action":  "write",
			"path":    "test.txt",
			"content": "hello world",
		},
	})
	if !out.Success {
		t.Fatalf("write failed: %s", out.Error)
	}

	// Read.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "read", "path": "test.txt"},
	})
	if !out.Success || out.Result != "hello world" {
		t.Errorf("read: success=%v, result=%q", out.Success, out.Result)
	}
}

func TestFileOpsSkill_List(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

	s := NewFileOpsSkill(dir)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "list", "pattern": "*.txt"},
	})
	if !out.Success {
		t.Fatalf("list failed: %s", out.Error)
	}
	if out.Result == "" {
		t.Error("empty list result")
	}
}

func TestFileOpsSkill_Stat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("test"), 0644)

	s := NewFileOpsSkill(dir)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "stat", "path": "f.txt"},
	})
	if !out.Success {
		t.Fatalf("stat failed: %s", out.Error)
	}
}

func TestFileOpsSkill_Search(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("goodbye"), 0644)

	s := NewFileOpsSkill(dir)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "search", "query": "hello"},
	})
	if !out.Success {
		t.Fatalf("search failed: %s", out.Error)
	}
}

func TestFileOpsSkill_NoAction(t *testing.T) {
	s := NewFileOpsSkill(".")
	out, _ := s.Execute(context.Background(), instruments.SkillInput{})
	if out.Success {
		t.Error("should fail without action")
	}
}

// --- Data Analysis tests ---

func TestDataAnalysis_Statistics(t *testing.T) {
	s := NewDataAnalysisSkill()
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{
			"action": "statistics",
			"data":   "1 2 3 4 5 6 7 8 9 10",
		},
	})
	if !out.Success {
		t.Fatalf("statistics failed: %s", out.Error)
	}
	if out.Result == "" {
		t.Error("empty result")
	}
}

func TestDataAnalysis_CSV(t *testing.T) {
	s := NewDataAnalysisSkill()
	csv := "name,age,score\nalice,30,85\nbob,25,92\ncharlie,35,78"
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "csv_stats", "data": csv},
	})
	if !out.Success {
		t.Fatalf("csv_stats failed: %s", out.Error)
	}
}

func TestDataAnalysis_JSON(t *testing.T) {
	s := NewDataAnalysisSkill()
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{
			"action": "json_query",
			"data":   `{"name":"test","value":42}`,
		},
	})
	if !out.Success {
		t.Fatalf("json_query failed: %s", out.Error)
	}
}

func TestDataAnalysis_NoNumbers(t *testing.T) {
	s := NewDataAnalysisSkill()
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"data": "no numbers here"},
	})
	if out.Success {
		t.Error("should fail with no numbers")
	}
}

func TestComputeStats(t *testing.T) {
	s := computeStats([]float64{1, 2, 3, 4, 5})
	if s.count != 5 {
		t.Errorf("count = %d", s.count)
	}
	if s.mean != 3 {
		t.Errorf("mean = %f", s.mean)
	}
	if s.median != 3 {
		t.Errorf("median = %f", s.median)
	}
	if s.min != 1 {
		t.Errorf("min = %f", s.min)
	}
	if s.max != 5 {
		t.Errorf("max = %f", s.max)
	}
}

func TestComputeStats_Empty(t *testing.T) {
	s := computeStats(nil)
	if s.count != 0 {
		t.Errorf("count = %d", s.count)
	}
}

// --- API Integration tests ---

func TestAPIIntegration_NoURL(t *testing.T) {
	s := NewAPIIntegrationSkill()
	out, _ := s.Execute(context.Background(), instruments.SkillInput{})
	if out.Success {
		t.Error("should fail without URL")
	}
}

func TestWebSearch_NoQuery(t *testing.T) {
	s := NewWebSearchSkill()
	out, _ := s.Execute(context.Background(), instruments.SkillInput{})
	if out.Success {
		t.Error("should fail without query")
	}
}

// --- Scheduler tests ---

func TestScheduler_AddList(t *testing.T) {
	store := newTestStore(t)
	s := NewSchedulerSkill(store)
	ctx := context.Background()

	// Add a task.
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{
			"action":      "add",
			"name":        "daily cleanup",
			"schedule":    "0 2 * * *",
			"task_action": "clean temp files",
		},
	})
	if !out.Success {
		t.Fatalf("add failed: %s", out.Error)
	}

	// List tasks.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "list"},
	})
	if !out.Success {
		t.Fatalf("list failed: %s", out.Error)
	}
}

func TestScheduler_Remove(t *testing.T) {
	s := NewSchedulerSkill(nil)
	ctx := context.Background()

	s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "add", "name": "test"},
	})

	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "remove", "task_id": "sched_1"},
	})
	if !out.Success {
		t.Fatalf("remove failed: %s", out.Error)
	}
}

func TestScheduler_RemoveNotFound(t *testing.T) {
	s := NewSchedulerSkill(nil)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "remove", "task_id": "nope"},
	})
	if out.Success {
		t.Error("should fail for missing task")
	}
}

// --- Audit tests ---

func TestAudit_LogAndQuery(t *testing.T) {
	store := newTestStore(t)
	s := NewAuditSkill(store)
	ctx := context.Background()

	// Log an entry.
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Goal: "user logged in",
		Parameters: map[string]string{
			"action":       "log",
			"actor":        "admin",
			"audit_action": "login",
		},
	})
	if !out.Success {
		t.Fatalf("log failed: %s", out.Error)
	}

	// Count.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "count"},
	})
	if !out.Success {
		t.Fatalf("count failed: %s", out.Error)
	}
}

func TestAudit_NoStore(t *testing.T) {
	s := NewAuditSkill(nil)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "count"},
	})
	if !out.Success {
		t.Error("count without store should succeed with 0")
	}
}

// --- Credential tests ---

func TestCredential_StoreGetList(t *testing.T) {
	store := newTestStore(t)
	s := NewCredentialSkill(store)
	ctx := context.Background()

	// Store.
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{
			"action": "store",
			"name":   "OPENAI_KEY",
			"value":  "sk-abcdef1234567890xyz",
			"type":   "api_key",
		},
	})
	if !out.Success {
		t.Fatalf("store failed: %s", out.Error)
	}

	// Get (should mask).
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "get", "name": "OPENAI_KEY"},
	})
	if !out.Success {
		t.Fatalf("get failed: %s", out.Error)
	}
	// Should not contain full key.
	if out.Result == "sk-abcdef1234567890xyz" {
		t.Error("credential value should be masked")
	}

	// List.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "list"},
	})
	if !out.Success {
		t.Fatalf("list failed: %s", out.Error)
	}
}

func TestCredential_Delete(t *testing.T) {
	store := newTestStore(t)
	s := NewCredentialSkill(store)
	ctx := context.Background()

	s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "store", "name": "k", "value": "v"},
	})
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "delete", "name": "k"},
	})
	if !out.Success {
		t.Fatalf("delete failed: %s", out.Error)
	}
}

func TestCredential_NotFound(t *testing.T) {
	store := newTestStore(t)
	s := NewCredentialSkill(store)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "get", "name": "missing"},
	})
	if out.Success {
		t.Error("should fail for missing credential")
	}
}

func TestMaskValue(t *testing.T) {
	if m := maskValue("short"); m != "****" {
		t.Errorf("short mask = %q", m)
	}
	if m := maskValue("sk-abcdef1234567890xyz"); m == "sk-abcdef1234567890xyz" {
		t.Error("long value should be masked")
	}
	m := maskValue("sk-abcdef1234567890xyz")
	if m[:4] != "sk-a" || m[len(m)-4:] != "0xyz" {
		t.Errorf("mask = %q", m)
	}
}

// --- Knowledge Base tests ---

func TestKnowledge_StoreAndSearch(t *testing.T) {
	store := newTestStore(t)
	s := NewKnowledgeSearchSkill(store)
	ctx := context.Background()

	// Store a document.
	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{
			"action":  "store",
			"key":     "go-concurrency",
			"content": "Go uses goroutines and channels for concurrency patterns",
			"type":    "article",
		},
	})
	if !out.Success {
		t.Fatalf("store failed: %s", out.Error)
	}

	// Search for it.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "search", "query": "goroutines"},
	})
	if !out.Success {
		t.Fatalf("search failed: %s", out.Error)
	}

	// List documents.
	out, _ = s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "list"},
	})
	if !out.Success {
		t.Fatalf("list failed: %s", out.Error)
	}
}

func TestKnowledge_Get(t *testing.T) {
	store := newTestStore(t)
	s := NewKnowledgeSearchSkill(store)
	ctx := context.Background()

	s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{
			"action":  "store",
			"key":     "test-doc",
			"content": "test content",
		},
	})

	out, _ := s.Execute(ctx, instruments.SkillInput{
		Parameters: map[string]string{"action": "get", "key": "test-doc"},
	})
	if !out.Success || out.Result != "test content" {
		t.Errorf("get: success=%v, result=%q", out.Success, out.Result)
	}
}

func TestKnowledge_NotFound(t *testing.T) {
	store := newTestStore(t)
	s := NewKnowledgeSearchSkill(store)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{
		Parameters: map[string]string{"action": "get", "key": "nope"},
	})
	if out.Success {
		t.Error("should fail for missing doc")
	}
}

func TestKnowledge_NoStore(t *testing.T) {
	s := NewKnowledgeSearchSkill(nil)
	out, _ := s.Execute(context.Background(), instruments.SkillInput{Goal: "test"})
	if out.Success {
		t.Error("should fail without store")
	}
}
