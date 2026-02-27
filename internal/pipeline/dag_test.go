package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDAGExecutor_LinearChain(t *testing.T) {
	// A → B → C: must execute in order.
	var order []string
	var mu sync.Mutex

	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		mu.Lock()
		order = append(order, sub.ID)
		mu.Unlock()
		return "done:" + sub.ID, nil
	})

	subtasks := []SubtaskSpec{
		{ID: "a", Goal: "first", Status: TaskStatusDraft},
		{ID: "b", Goal: "second", DependsOn: []string{"a"}, Status: TaskStatusDraft},
		{ID: "c", Goal: "third", DependsOn: []string{"b"}, Status: TaskStatusDraft},
	}

	results, err := exec.Execute(context.Background(), subtasks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}

	// Verify order.
	if order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("order = %v, want [a b c]", order)
	}

	// Verify all completed.
	for _, r := range results {
		if r.Status != TaskStatusCompleted {
			t.Errorf("subtask %s status = %q, want completed", r.ID, r.Status)
		}
		if r.Result != "done:"+r.ID {
			t.Errorf("subtask %s result = %q", r.ID, r.Result)
		}
	}
}

func TestDAGExecutor_ParallelIndependent(t *testing.T) {
	// A, B, C have no dependencies — should run in parallel.
	var concurrent int64
	var maxConcurrent int64

	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		cur := atomic.AddInt64(&concurrent, 1)
		// Track max concurrency.
		for {
			old := atomic.LoadInt64(&maxConcurrent)
			if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // Ensure overlap.
		atomic.AddInt64(&concurrent, -1)
		return "ok", nil
	})

	subtasks := []SubtaskSpec{
		{ID: "a", Goal: "parallel 1", Status: TaskStatusDraft},
		{ID: "b", Goal: "parallel 2", Status: TaskStatusDraft},
		{ID: "c", Goal: "parallel 3", Status: TaskStatusDraft},
	}

	results, err := exec.Execute(context.Background(), subtasks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}

	if maxConcurrent < 2 {
		t.Errorf("maxConcurrent = %d, want >= 2 (should run in parallel)", maxConcurrent)
	}
}

func TestDAGExecutor_DiamondDependency(t *testing.T) {
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	var order []string
	var mu sync.Mutex

	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		mu.Lock()
		order = append(order, sub.ID)
		mu.Unlock()
		return "ok", nil
	})

	subtasks := []SubtaskSpec{
		{ID: "a", Goal: "root", Status: TaskStatusDraft},
		{ID: "b", Goal: "left", DependsOn: []string{"a"}, Status: TaskStatusDraft},
		{ID: "c", Goal: "right", DependsOn: []string{"a"}, Status: TaskStatusDraft},
		{ID: "d", Goal: "join", DependsOn: []string{"b", "c"}, Status: TaskStatusDraft},
	}

	results, err := exec.Execute(context.Background(), subtasks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// A must be first, D must be last.
	if order[0] != "a" {
		t.Errorf("first = %q, want 'a'", order[0])
	}
	if order[3] != "d" {
		t.Errorf("last = %q, want 'd'", order[3])
	}

	for _, r := range results {
		if r.Status != TaskStatusCompleted {
			t.Errorf("subtask %s status = %q", r.ID, r.Status)
		}
	}
}

func TestDAGExecutor_ErrorStopsExecution(t *testing.T) {
	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		if sub.ID == "b" {
			return "", fmt.Errorf("b failed")
		}
		return "ok", nil
	})

	subtasks := []SubtaskSpec{
		{ID: "a", Goal: "ok", Status: TaskStatusDraft},
		{ID: "b", Goal: "will fail", DependsOn: []string{"a"}, Status: TaskStatusDraft},
		{ID: "c", Goal: "depends on b", DependsOn: []string{"b"}, Status: TaskStatusDraft},
	}

	results, err := exec.Execute(context.Background(), subtasks)
	if err == nil {
		t.Fatal("expected error")
	}

	// B should be failed.
	if results[1].Status != TaskStatusFailed {
		t.Errorf("b status = %q, want failed", results[1].Status)
	}
	// C should NOT have executed (still draft because b failed and loop breaks).
	if results[2].Status == TaskStatusCompleted {
		t.Error("c should not have completed after b failed")
	}
}

func TestDAGExecutor_EmptySubtasks(t *testing.T) {
	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		return "ok", nil
	})

	results, err := exec.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if results != nil {
		t.Errorf("results = %v, want nil", results)
	}
}

func TestDAGExecutor_SingleTask(t *testing.T) {
	exec := NewDAGExecutor(func(ctx context.Context, sub *SubtaskSpec) (string, error) {
		return "single result", nil
	})

	subtasks := []SubtaskSpec{
		{ID: "only", Goal: "single", Status: TaskStatusDraft},
	}

	results, err := exec.Execute(context.Background(), subtasks)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Result != "single result" {
		t.Errorf("result = %q", results[0].Result)
	}
}

// --- TopologicalOrder Tests ---

func TestTopologicalOrder_LinearChain(t *testing.T) {
	subtasks := []SubtaskSpec{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "a"},
	}

	order, err := TopologicalOrder(subtasks)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	// a must come before b, b before c.
	idxOf := make(map[string]int)
	for i, id := range order {
		idxOf[id] = i
	}
	if idxOf["a"] > idxOf["b"] || idxOf["b"] > idxOf["c"] {
		t.Errorf("order = %v, want a before b before c", order)
	}
}

func TestTopologicalOrder_Parallel(t *testing.T) {
	subtasks := []SubtaskSpec{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}

	order, err := TopologicalOrder(subtasks)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(order) != 3 {
		t.Errorf("order = %v, want 3 elements", order)
	}
}

func TestTopologicalOrder_CycleDetection(t *testing.T) {
	subtasks := []SubtaskSpec{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	_, err := TopologicalOrder(subtasks)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopologicalOrder_Diamond(t *testing.T) {
	subtasks := []SubtaskSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}

	order, err := TopologicalOrder(subtasks)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}

	idxOf := make(map[string]int)
	for i, id := range order {
		idxOf[id] = i
	}
	if idxOf["a"] > idxOf["b"] || idxOf["a"] > idxOf["c"] {
		t.Error("a should come before b and c")
	}
	if idxOf["b"] > idxOf["d"] || idxOf["c"] > idxOf["d"] {
		t.Error("b and c should come before d")
	}
}

func TestTopologicalOrder_Empty(t *testing.T) {
	order, err := TopologicalOrder(nil)
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("order = %v, want empty", order)
	}
}
