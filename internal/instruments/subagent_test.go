package instruments

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockRunner implements TaskRunner for testing.
type mockRunner struct {
	result  *DelegationResult
	err     error
	calls   int32 // atomic
	delay   time.Duration
}

func (m *mockRunner) RunTask(ctx context.Context, agentID string, task DelegatedTask) (*DelegationResult, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func successResult() *DelegationResult {
	return &DelegationResult{
		Output:    "done",
		Success:   true,
		Quality:   0.9,
		CostUSD:   0.01,
		ElapsedMs: 100,
	}
}

func TestNewSubagentManager(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	s := mgr.Stats()
	if s.Total != 0 {
		t.Fatal("expected 0 delegations")
	}
}

func TestDelegate_Success(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	result, err := mgr.Delegate(context.Background(), "parent-1", "child-1", DelegatedTask{
		Goal: "test task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Output != "done" {
		t.Fatalf("expected 'done', got %s", result.Output)
	}

	s := mgr.Stats()
	if s.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", s.Completed)
	}
}

func TestDelegate_Failure(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("agent crashed")}
	mgr := NewSubagentManager(runner)

	_, err := mgr.Delegate(context.Background(), "p", "c", DelegatedTask{Goal: "fail"})
	if err == nil {
		t.Fatal("expected error")
	}

	s := mgr.Stats()
	if s.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", s.Failed)
	}
}

func TestDelegate_Timeout(t *testing.T) {
	runner := &mockRunner{result: successResult(), delay: 2 * time.Second}
	mgr := NewSubagentManager(runner)

	_, err := mgr.Delegate(context.Background(), "p", "c", DelegatedTask{
		Goal:    "slow task",
		Timeout: 50 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestDelegateAsync_Execute(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	id := mgr.DelegateAsync("p", "c", DelegatedTask{Goal: "async"})
	d := mgr.GetDelegation(id)
	if d.Status != DelegationPending {
		t.Fatalf("expected PENDING, got %s", d.Status)
	}

	if err := mgr.Execute(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	d = mgr.GetDelegation(id)
	if d.Status != DelegationCompleted {
		t.Fatalf("expected COMPLETED, got %s", d.Status)
	}
	if !d.Result.Success {
		t.Fatal("expected success result")
	}
}

func TestDelegateAsync_ExecuteNotFound(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)
	if err := mgr.Execute(context.Background(), "nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFanOut(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	agents := []string{"c1", "c2", "c3"}
	results := mgr.FanOut(context.Background(), "p", agents, DelegatedTask{Goal: "parallel"})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Success {
			t.Fatalf("result %d should be success", i)
		}
	}

	calls := int(atomic.LoadInt32(&runner.calls))
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestFanOut_PartialFailure(t *testing.T) {
	// This runner alternates success/failure based on call count.
	callCount := int32(0)
	runner := &mockRunner{}
	mgr := NewSubagentManager(&alternatingRunner{})

	agents := []string{"c1", "c2"}
	results := mgr.FanOut(context.Background(), "p", agents, DelegatedTask{Goal: "partial"})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	_ = callCount
	_ = runner
}

// alternatingRunner succeeds for even calls, fails for odd.
type alternatingRunner struct {
	calls int32
}

func (r *alternatingRunner) RunTask(ctx context.Context, agentID string, task DelegatedTask) (*DelegationResult, error) {
	n := atomic.AddInt32(&r.calls, 1)
	if n%2 == 0 {
		return nil, fmt.Errorf("failure on call %d", n)
	}
	return &DelegationResult{
		Output:  fmt.Sprintf("ok from call %d", n),
		Success: true,
		Quality: 0.8,
	}, nil
}

func TestBestOfN(t *testing.T) {
	mgr := NewSubagentManager(&qualityRunner{})

	agents := []string{"low", "high", "mid"}
	best := mgr.BestOfN(context.Background(), "p", agents, DelegatedTask{Goal: "compete"})

	if best == nil {
		t.Fatal("expected a result")
	}
	if best.Quality != 0.95 {
		t.Fatalf("expected quality 0.95, got %.2f", best.Quality)
	}
}

// qualityRunner returns different quality based on agent ID.
type qualityRunner struct{}

func (r *qualityRunner) RunTask(ctx context.Context, agentID string, task DelegatedTask) (*DelegationResult, error) {
	var q float64
	switch agentID {
	case "low":
		q = 0.3
	case "high":
		q = 0.95
	case "mid":
		q = 0.6
	}
	return &DelegationResult{
		Output:  agentID,
		Success: true,
		Quality: q,
	}, nil
}

func TestBestOfN_AllFail(t *testing.T) {
	runner := &mockRunner{err: fmt.Errorf("fail")}
	mgr := NewSubagentManager(runner)

	agents := []string{"c1", "c2"}
	best := mgr.BestOfN(context.Background(), "p", agents, DelegatedTask{Goal: "all fail"})

	if best == nil {
		t.Fatal("expected a failure result, not nil")
	}
	if best.Success {
		t.Fatal("expected non-success")
	}
}

func TestActiveDelegations(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	mgr.DelegateAsync("p", "c1", DelegatedTask{Goal: "t1"})
	mgr.DelegateAsync("p", "c2", DelegatedTask{Goal: "t2"})

	active := mgr.ActiveDelegations()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %d", len(active))
	}
}

func TestDelegationsByAgent(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	mgr.DelegateAsync("p", "c1", DelegatedTask{Goal: "t1"})
	mgr.DelegateAsync("p", "c1", DelegatedTask{Goal: "t2"})
	mgr.DelegateAsync("p", "c2", DelegatedTask{Goal: "t3"})

	c1Delegs := mgr.DelegationsByAgent("c1")
	if len(c1Delegs) != 2 {
		t.Fatalf("expected 2 for c1, got %d", len(c1Delegs))
	}
}

func TestCancelDelegation(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	id := mgr.DelegateAsync("p", "c1", DelegatedTask{Goal: "to cancel"})
	if err := mgr.CancelDelegation(id); err != nil {
		t.Fatal(err)
	}

	d := mgr.GetDelegation(id)
	if d.Status != DelegationCancelled {
		t.Fatalf("expected CANCELLED, got %s", d.Status)
	}
}

func TestCancelDelegation_NotPending(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	result, _ := mgr.Delegate(context.Background(), "p", "c", DelegatedTask{Goal: "done"})
	_ = result

	// Find the completed delegation.
	for _, d := range mgr.delegations {
		if d.Status == DelegationCompleted {
			if err := mgr.CancelDelegation(d.ID); err == nil {
				t.Fatal("expected error cancelling completed delegation")
			}
			return
		}
	}
}

func TestCancelDelegation_NotFound(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)
	if err := mgr.CancelDelegation("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCleanup(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	// Create and complete a delegation.
	mgr.Delegate(context.Background(), "p", "c", DelegatedTask{Goal: "old"})

	// Manually set the completion time to the past.
	for _, d := range mgr.delegations {
		d.CompletedAt = time.Now().UTC().Add(-2 * time.Hour)
	}

	removed := mgr.Cleanup(1 * time.Hour)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if mgr.Stats().Total != 0 {
		t.Fatal("expected 0 after cleanup")
	}
}

func TestCleanup_KeepsRecent(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	mgr.Delegate(context.Background(), "p", "c", DelegatedTask{Goal: "recent"})

	removed := mgr.Cleanup(1 * time.Hour)
	if removed != 0 {
		t.Fatalf("expected 0 removed (too recent), got %d", removed)
	}
}

func TestStats(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)

	mgr.Delegate(context.Background(), "p", "c1", DelegatedTask{Goal: "t1"})
	mgr.Delegate(context.Background(), "p", "c2", DelegatedTask{Goal: "t2"})
	mgr.DelegateAsync("p", "c3", DelegatedTask{Goal: "pending"})

	s := mgr.Stats()
	if s.Total != 3 {
		t.Fatalf("expected 3 total, got %d", s.Total)
	}
	if s.Completed != 2 {
		t.Fatalf("expected 2 completed, got %d", s.Completed)
	}
	if s.Pending != 1 {
		t.Fatalf("expected 1 pending, got %d", s.Pending)
	}
	if s.AvgQuality != 0.9 {
		t.Fatalf("expected avg quality 0.9, got %.2f", s.AvgQuality)
	}
}

func TestGetDelegation_Missing(t *testing.T) {
	runner := &mockRunner{result: successResult()}
	mgr := NewSubagentManager(runner)
	if mgr.GetDelegation("nope") != nil {
		t.Fatal("expected nil")
	}
}
