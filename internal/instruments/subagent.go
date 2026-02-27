// Package instruments — subagent.go implements the SubagentManager which
// handles task delegation to child agents, result collection, and lifecycle
// management. It's the bridge between the parent pipeline and fractal agents.
package instruments

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Delegation types
// ---------------------------------------------------------------------------

// DelegationStatus represents the lifecycle of a delegated task.
type DelegationStatus string

const (
	DelegationPending   DelegationStatus = "PENDING"
	DelegationRunning   DelegationStatus = "RUNNING"
	DelegationCompleted DelegationStatus = "COMPLETED"
	DelegationFailed    DelegationStatus = "FAILED"
	DelegationCancelled DelegationStatus = "CANCELLED"
)

// Delegation represents a task delegated to a subagent.
type Delegation struct {
	ID            string           `json:"id"`
	ParentAgentID string           `json:"parent_agent_id"`
	ChildAgentID  string           `json:"child_agent_id"`
	Task          DelegatedTask    `json:"task"`
	Status        DelegationStatus `json:"status"`
	Result        *DelegationResult `json:"result,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	CompletedAt   time.Time        `json:"completed_at,omitempty"`
}

// DelegatedTask is the work sent to a subagent.
type DelegatedTask struct {
	Goal       string            `json:"goal"`
	Context    string            `json:"context"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Priority   int               `json:"priority"` // 0=normal, 1=high, 2=critical
	Timeout    time.Duration     `json:"timeout,omitempty"`
}

// DelegationResult is the work returned from a subagent.
type DelegationResult struct {
	Output    string  `json:"output"`
	Success   bool    `json:"success"`
	Quality   float64 `json:"quality"`
	CostUSD   float64 `json:"cost_usd"`
	ElapsedMs int64   `json:"elapsed_ms"`
	Error     string  `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// TaskRunner interface — abstraction for running tasks on agents
// ---------------------------------------------------------------------------

// TaskRunner is an abstraction that allows the SubagentManager to run tasks
// on any agent without depending on the pipeline package (avoid circular deps).
type TaskRunner interface {
	// RunTask executes a task on the given agent and returns the result.
	RunTask(ctx context.Context, agentID string, task DelegatedTask) (*DelegationResult, error)
}

// ---------------------------------------------------------------------------
// SubagentManager
// ---------------------------------------------------------------------------

// SubagentManager manages task delegation to child agents. It tracks active
// delegations and provides coordination primitives for parallel execution.
type SubagentManager struct {
	mu          sync.RWMutex
	runner      TaskRunner
	delegations map[string]*Delegation // delegationID → Delegation
	nextID      int
}

// NewSubagentManager creates a SubagentManager with the given TaskRunner.
func NewSubagentManager(runner TaskRunner) *SubagentManager {
	return &SubagentManager{
		runner:      runner,
		delegations: make(map[string]*Delegation),
		nextID:      1,
	}
}

// ---------------------------------------------------------------------------
// Delegation API
// ---------------------------------------------------------------------------

// Delegate sends a task to a specific subagent and blocks until completion
// or context cancellation.
func (m *SubagentManager) Delegate(ctx context.Context, parentID, childID string, task DelegatedTask) (*DelegationResult, error) {
	d := m.createDelegation(parentID, childID, task)

	// Apply task-level timeout if set.
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	m.setStatus(d.ID, DelegationRunning)

	result, err := m.runner.RunTask(ctx, childID, task)
	now := time.Now().UTC()

	if err != nil {
		m.mu.Lock()
		d.Status = DelegationFailed
		d.CompletedAt = now
		d.Result = &DelegationResult{
			Success:   false,
			Error:     err.Error(),
			ElapsedMs: now.Sub(d.CreatedAt).Milliseconds(),
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("delegation to %q failed: %w", childID, err)
	}

	m.mu.Lock()
	d.Status = DelegationCompleted
	d.CompletedAt = now
	d.Result = result
	m.mu.Unlock()

	return result, nil
}

// DelegateAsync sends a task to a subagent without blocking. Returns the
// delegation ID immediately. Use WaitFor to collect the result later.
func (m *SubagentManager) DelegateAsync(parentID, childID string, task DelegatedTask) string {
	d := m.createDelegation(parentID, childID, task)
	return d.ID
}

// Execute runs a previously-created async delegation. This should be called
// from a goroutine. It updates the delegation status on completion.
func (m *SubagentManager) Execute(ctx context.Context, delegationID string) error {
	m.mu.RLock()
	d, ok := m.delegations[delegationID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("delegation %q not found", delegationID)
	}

	// Apply task-level timeout if set.
	if d.Task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.Task.Timeout)
		defer cancel()
	}

	m.setStatus(delegationID, DelegationRunning)

	result, err := m.runner.RunTask(ctx, d.ChildAgentID, d.Task)
	now := time.Now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	d.CompletedAt = now
	if err != nil {
		d.Status = DelegationFailed
		d.Result = &DelegationResult{
			Success:   false,
			Error:     err.Error(),
			ElapsedMs: now.Sub(d.CreatedAt).Milliseconds(),
		}
		return err
	}

	d.Status = DelegationCompleted
	d.Result = result
	return nil
}

// FanOut delegates the same task to multiple subagents in parallel and
// collects all results. Returns results in the same order as agentIDs.
// Individual failures don't cancel other goroutines.
func (m *SubagentManager) FanOut(ctx context.Context, parentID string, agentIDs []string, task DelegatedTask) []*DelegationResult {
	results := make([]*DelegationResult, len(agentIDs))
	var wg sync.WaitGroup

	for i, aid := range agentIDs {
		wg.Add(1)
		go func(idx int, childID string) {
			defer wg.Done()
			res, err := m.Delegate(ctx, parentID, childID, task)
			if err != nil {
				results[idx] = &DelegationResult{
					Success: false,
					Error:   err.Error(),
				}
				return
			}
			results[idx] = res
		}(i, aid)
	}

	wg.Wait()
	return results
}

// BestOfN fans out to N agents and returns the result with highest quality.
func (m *SubagentManager) BestOfN(ctx context.Context, parentID string, agentIDs []string, task DelegatedTask) *DelegationResult {
	results := m.FanOut(ctx, parentID, agentIDs, task)

	var best *DelegationResult
	for _, r := range results {
		if r == nil {
			continue
		}
		if !r.Success {
			continue
		}
		if best == nil || r.Quality > best.Quality {
			best = r
		}
	}

	// If no success, return first failure.
	if best == nil && len(results) > 0 {
		for _, r := range results {
			if r != nil {
				return r
			}
		}
	}

	return best
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// GetDelegation returns a delegation by ID.
func (m *SubagentManager) GetDelegation(id string) *Delegation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.delegations[id]
}

// ActiveDelegations returns all delegations in PENDING or RUNNING state.
func (m *SubagentManager) ActiveDelegations() []*Delegation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Delegation
	for _, d := range m.delegations {
		if d.Status == DelegationPending || d.Status == DelegationRunning {
			out = append(out, d)
		}
	}
	return out
}

// DelegationsByAgent returns all delegations for a specific child agent.
func (m *SubagentManager) DelegationsByAgent(childID string) []*Delegation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Delegation
	for _, d := range m.delegations {
		if d.ChildAgentID == childID {
			out = append(out, d)
		}
	}
	return out
}

// Stats returns aggregate statistics for all delegations.
func (m *SubagentManager) Stats() DelegationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var s DelegationStats
	s.Total = len(m.delegations)

	for _, d := range m.delegations {
		switch d.Status {
		case DelegationPending:
			s.Pending++
		case DelegationRunning:
			s.Running++
		case DelegationCompleted:
			s.Completed++
			if d.Result != nil {
				s.TotalCostUSD += d.Result.CostUSD
				s.TotalQuality += d.Result.Quality
				s.QualityCount++
			}
		case DelegationFailed:
			s.Failed++
		case DelegationCancelled:
			s.Cancelled++
		}
	}

	if s.QualityCount > 0 {
		s.AvgQuality = s.TotalQuality / float64(s.QualityCount)
	}

	return s
}

// DelegationStats holds aggregate delegation statistics.
type DelegationStats struct {
	Total        int     `json:"total"`
	Pending      int     `json:"pending"`
	Running      int     `json:"running"`
	Completed    int     `json:"completed"`
	Failed       int     `json:"failed"`
	Cancelled    int     `json:"cancelled"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	AvgQuality   float64 `json:"avg_quality"`
	TotalQuality float64 `json:"-"`
	QualityCount int     `json:"-"`
}

// CancelDelegation marks a delegation as cancelled. Only PENDING delegations
// can be cancelled.
func (m *SubagentManager) CancelDelegation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.delegations[id]
	if !ok {
		return fmt.Errorf("delegation %q not found", id)
	}
	if d.Status != DelegationPending {
		return fmt.Errorf("cannot cancel delegation in state %s", d.Status)
	}
	d.Status = DelegationCancelled
	d.CompletedAt = time.Now().UTC()
	return nil
}

// Cleanup removes completed/failed/cancelled delegations older than the given
// duration. Returns the number of removed entries.
func (m *SubagentManager) Cleanup(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().UTC().Add(-olderThan)
	removed := 0
	for id, d := range m.delegations {
		if d.Status == DelegationCompleted || d.Status == DelegationFailed || d.Status == DelegationCancelled {
			if d.CompletedAt.Before(cutoff) {
				delete(m.delegations, id)
				removed++
			}
		}
	}
	return removed
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m *SubagentManager) createDelegation(parentID, childID string, task DelegatedTask) *Delegation {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("deleg-%d", m.nextID)
	m.nextID++
	d := &Delegation{
		ID:            id,
		ParentAgentID: parentID,
		ChildAgentID:  childID,
		Task:          task,
		Status:        DelegationPending,
		CreatedAt:     time.Now().UTC(),
	}
	m.delegations[id] = d
	return d
}

func (m *SubagentManager) setStatus(id string, status DelegationStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.delegations[id]; ok {
		d.Status = status
	}
}
