package goals

import (
	"testing"
	"time"
)

func TestEngine_Add(t *testing.T) {
	e := New()

	g := e.Add("Generate code-skill for pattern X", GoalSourcePattern, GoalPriorityHigh)

	if g.ID == "" {
		t.Error("ID should not be empty")
	}
	if g.Description != "Generate code-skill for pattern X" {
		t.Errorf("Description = %q", g.Description)
	}
	if g.Source != GoalSourcePattern {
		t.Errorf("Source = %q", g.Source)
	}
	if g.Priority != GoalPriorityHigh {
		t.Errorf("Priority = %d", g.Priority)
	}
	if g.Status != GoalStatusPending {
		t.Errorf("Status = %q, want PENDING", g.Status)
	}
	if g.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", g.MaxAttempts)
	}
	if g.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestEngine_AddWithMeta(t *testing.T) {
	e := New()

	meta := map[string]string{"fingerprint": "fp_abc", "task_type": "summarize"}
	g := e.AddWithMeta("Improve summarization", GoalSourceReflection, GoalPriorityNormal, meta)

	if g.Metadata["fingerprint"] != "fp_abc" {
		t.Errorf("metadata = %v", g.Metadata)
	}
}

func TestEngine_Get(t *testing.T) {
	e := New()
	g := e.Add("test", GoalSourceUser, GoalPriorityNormal)

	got := e.Get(g.ID)
	if got == nil {
		t.Fatal("expected to find goal")
	}
	if got.ID != g.ID {
		t.Errorf("ID mismatch: %q vs %q", got.ID, g.ID)
	}

	if e.Get("nonexistent") != nil {
		t.Error("should return nil for unknown ID")
	}
}

func TestEngine_NextPending(t *testing.T) {
	e := New()

	// No goals — should return nil.
	if e.NextPending() != nil {
		t.Error("expected nil when no goals")
	}

	low := e.Add("low priority", GoalSourceHeartbeat, GoalPriorityLow)
	high := e.Add("high priority", GoalSourcePattern, GoalPriorityHigh)
	normal := e.Add("normal priority", GoalSourceReflection, GoalPriorityNormal)

	next := e.NextPending()
	if next == nil {
		t.Fatal("expected a pending goal")
	}
	if next.ID != high.ID {
		t.Errorf("expected high priority goal, got %s (priority %d)", next.ID, next.Priority)
	}

	_ = low
	_ = normal
}

func TestEngine_NextPending_SamePriorityOldestFirst(t *testing.T) {
	e := New()

	first := e.Add("first", GoalSourceUser, GoalPriorityNormal)
	time.Sleep(time.Millisecond)
	_ = e.Add("second", GoalSourceUser, GoalPriorityNormal)

	next := e.NextPending()
	if next.ID != first.ID {
		t.Errorf("same priority should pick oldest, got %s", next.ID)
	}
}

func TestEngine_Lifecycle(t *testing.T) {
	e := New()

	g := e.Add("test goal", GoalSourceUser, GoalPriorityNormal)

	// Mark in progress.
	if err := e.MarkInProgress(g.ID, "task_123"); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	got := e.Get(g.ID)
	if got.Status != GoalStatusInProgress {
		t.Errorf("Status = %q, want IN_PROGRESS", got.Status)
	}
	if got.TaskID != "task_123" {
		t.Errorf("TaskID = %q", got.TaskID)
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", got.Attempts)
	}

	// Mark completed.
	if err := e.MarkCompleted(g.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if e.Get(g.ID).Status != GoalStatusCompleted {
		t.Error("expected COMPLETED")
	}
}

func TestEngine_FailWithRetry(t *testing.T) {
	e := New()

	g := e.Add("retryable", GoalSourcePattern, GoalPriorityNormal)
	// MaxAttempts = 3 by default.

	// First attempt: fail → should go back to PENDING.
	e.MarkInProgress(g.ID, "t1")
	e.MarkFailed(g.ID)
	if e.Get(g.ID).Status != GoalStatusPending {
		t.Error("should be PENDING after first failure (1/3 attempts)")
	}

	// Second attempt: fail.
	e.MarkInProgress(g.ID, "t2")
	e.MarkFailed(g.ID)
	if e.Get(g.ID).Status != GoalStatusPending {
		t.Error("should be PENDING after second failure (2/3 attempts)")
	}

	// Third attempt: fail → should be FAILED (max attempts reached).
	e.MarkInProgress(g.ID, "t3")
	e.MarkFailed(g.ID)
	if e.Get(g.ID).Status != GoalStatusFailed {
		t.Errorf("should be FAILED after 3 failures, got %q", e.Get(g.ID).Status)
	}
}

func TestEngine_Cancel(t *testing.T) {
	e := New()
	g := e.Add("cancel me", GoalSourceUser, GoalPriorityLow)

	if err := e.Cancel(g.ID); err != nil {
		t.Fatal(err)
	}
	if e.Get(g.ID).Status != GoalStatusCancelled {
		t.Error("expected CANCELLED")
	}
}

func TestEngine_ErrorOnUnknownID(t *testing.T) {
	e := New()

	if err := e.MarkInProgress("bad", "t"); err == nil {
		t.Error("expected error")
	}
	if err := e.MarkCompleted("bad"); err == nil {
		t.Error("expected error")
	}
	if err := e.MarkFailed("bad"); err == nil {
		t.Error("expected error")
	}
	if err := e.Cancel("bad"); err == nil {
		t.Error("expected error")
	}
}

func TestEngine_ListByStatus(t *testing.T) {
	e := New()
	e.Add("a", GoalSourceUser, GoalPriorityNormal)
	g2 := e.Add("b", GoalSourceUser, GoalPriorityNormal)
	e.Add("c", GoalSourceUser, GoalPriorityNormal)

	e.MarkInProgress(g2.ID, "t")
	e.MarkCompleted(g2.ID)

	pending := e.ListByStatus(GoalStatusPending)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	completed := e.ListByStatus(GoalStatusCompleted)
	if len(completed) != 1 {
		t.Errorf("expected 1 completed, got %d", len(completed))
	}
}

func TestEngine_Count(t *testing.T) {
	e := New()
	if e.Count() != 0 {
		t.Error("empty engine should have 0 goals")
	}

	e.Add("a", GoalSourceUser, GoalPriorityNormal)
	e.Add("b", GoalSourceUser, GoalPriorityNormal)
	if e.Count() != 2 {
		t.Errorf("Count = %d, want 2", e.Count())
	}
}

func TestEngine_PendingCount(t *testing.T) {
	e := New()
	e.Add("a", GoalSourceUser, GoalPriorityNormal)
	g := e.Add("b", GoalSourceUser, GoalPriorityNormal)
	e.MarkInProgress(g.ID, "t")

	if e.PendingCount() != 1 {
		t.Errorf("PendingCount = %d, want 1", e.PendingCount())
	}
}

func TestEngine_CleanupCompleted(t *testing.T) {
	e := New()
	g1 := e.Add("old", GoalSourceUser, GoalPriorityNormal)
	g2 := e.Add("new", GoalSourceUser, GoalPriorityNormal)
	e.Add("pending", GoalSourceUser, GoalPriorityNormal)

	e.MarkInProgress(g1.ID, "t1")
	e.MarkCompleted(g1.ID)
	// Hack the timestamp to be old.
	e.mu.Lock()
	e.goals[g1.ID].UpdatedAt = time.Now().Add(-2 * time.Hour)
	e.mu.Unlock()

	e.Cancel(g2.ID)
	// Keep g2 recent.

	removed := e.CleanupCompleted(1 * time.Hour)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if e.Count() != 2 {
		t.Errorf("Count = %d, want 2 (recent cancelled + pending)", e.Count())
	}
}

func TestEngine_ListAll(t *testing.T) {
	e := New()
	e.Add("a", GoalSourceUser, GoalPriorityNormal)
	e.Add("b", GoalSourcePattern, GoalPriorityHigh)

	all := e.ListAll()
	if len(all) != 2 {
		t.Errorf("ListAll = %d, want 2", len(all))
	}
}
