// Package goals implements the GoalEngine — the proactive goal-setting
// system for Overhuman. Instead of only reacting to input, the agent
// identifies things it should do and schedules them.
//
// Goals come from:
//   - Meso-reflection insights (soul/skill suggestions)
//   - Pattern detection (automatable patterns → code-skill generation)
//   - Macro-reflection (strategy improvements)
//   - Heartbeat timer (periodic self-check)
package goals

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// GoalStatus tracks where a goal is in its lifecycle.
type GoalStatus string

const (
	GoalStatusPending    GoalStatus = "PENDING"
	GoalStatusInProgress GoalStatus = "IN_PROGRESS"
	GoalStatusCompleted  GoalStatus = "COMPLETED"
	GoalStatusFailed     GoalStatus = "FAILED"
	GoalStatusCancelled  GoalStatus = "CANCELLED"
)

// GoalPriority determines execution order.
type GoalPriority int

const (
	GoalPriorityLow      GoalPriority = 0
	GoalPriorityNormal   GoalPriority = 1
	GoalPriorityHigh     GoalPriority = 2
	GoalPriorityCritical GoalPriority = 3
)

// GoalSource indicates what triggered the goal.
type GoalSource string

const (
	GoalSourceReflection GoalSource = "REFLECTION"
	GoalSourcePattern    GoalSource = "PATTERN"
	GoalSourceHeartbeat  GoalSource = "HEARTBEAT"
	GoalSourceUser       GoalSource = "USER"
	GoalSourceEvolution  GoalSource = "EVOLUTION"
)

// Goal represents a proactive objective the agent should pursue.
type Goal struct {
	ID          string       `json:"id"`
	Description string       `json:"description"`
	Source      GoalSource   `json:"source"`
	Priority    GoalPriority `json:"priority"`
	Status      GoalStatus   `json:"status"`
	TaskID      string       `json:"task_id,omitempty"` // If executing, the pipeline task ID
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Attempts    int          `json:"attempts"`
	MaxAttempts int          `json:"max_attempts"` // 0 = unlimited
}

// Engine manages the lifecycle of proactive goals.
type Engine struct {
	mu    sync.RWMutex
	goals map[string]*Goal

	// Counter for generating IDs.
	nextID int
}

// New creates a new GoalEngine.
func New() *Engine {
	return &Engine{
		goals: make(map[string]*Goal),
	}
}

// Add registers a new goal.
func (e *Engine) Add(description string, source GoalSource, priority GoalPriority) *Goal {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextID++
	now := time.Now()
	g := &Goal{
		ID:          fmt.Sprintf("goal_%d", e.nextID),
		Description: description,
		Source:      source,
		Priority:    priority,
		Status:      GoalStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		MaxAttempts: 3,
	}
	e.goals[g.ID] = g
	return g
}

// AddWithMeta registers a goal with additional metadata.
func (e *Engine) AddWithMeta(description string, source GoalSource, priority GoalPriority, meta map[string]string) *Goal {
	g := e.Add(description, source, priority)
	e.mu.Lock()
	g.Metadata = meta
	e.mu.Unlock()
	return g
}

// Get returns a goal by ID.
func (e *Engine) Get(id string) *Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.goals[id]
}

// NextPending returns the highest-priority pending goal, or nil if none.
func (e *Engine) NextPending() *Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var pending []*Goal
	for _, g := range e.goals {
		if g.Status == GoalStatusPending {
			pending = append(pending, g)
		}
	}

	if len(pending) == 0 {
		return nil
	}

	// Sort by priority (highest first), then by creation time (oldest first).
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Priority != pending[j].Priority {
			return pending[i].Priority > pending[j].Priority
		}
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})

	return pending[0]
}

// MarkInProgress transitions a goal to IN_PROGRESS.
func (e *Engine) MarkInProgress(id, taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %q not found", id)
	}
	g.Status = GoalStatusInProgress
	g.TaskID = taskID
	g.Attempts++
	g.UpdatedAt = time.Now()
	return nil
}

// MarkCompleted transitions a goal to COMPLETED.
func (e *Engine) MarkCompleted(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %q not found", id)
	}
	g.Status = GoalStatusCompleted
	g.UpdatedAt = time.Now()
	return nil
}

// MarkFailed transitions a goal to FAILED or back to PENDING if retries remain.
func (e *Engine) MarkFailed(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %q not found", id)
	}

	if g.MaxAttempts > 0 && g.Attempts >= g.MaxAttempts {
		g.Status = GoalStatusFailed
	} else {
		g.Status = GoalStatusPending // Retry later
	}
	g.UpdatedAt = time.Now()
	return nil
}

// Cancel transitions a goal to CANCELLED.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	g, ok := e.goals[id]
	if !ok {
		return fmt.Errorf("goal %q not found", id)
	}
	g.Status = GoalStatusCancelled
	g.UpdatedAt = time.Now()
	return nil
}

// ListByStatus returns all goals with the given status.
func (e *Engine) ListByStatus(status GoalStatus) []*Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Goal
	for _, g := range e.goals {
		if g.Status == status {
			result = append(result, g)
		}
	}
	return result
}

// ListAll returns all goals.
func (e *Engine) ListAll() []*Goal {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Goal, 0, len(e.goals))
	for _, g := range e.goals {
		result = append(result, g)
	}
	return result
}

// Count returns total number of goals.
func (e *Engine) Count() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.goals)
}

// PendingCount returns the number of pending goals.
func (e *Engine) PendingCount() int {
	return len(e.ListByStatus(GoalStatusPending))
}

// CleanupCompleted removes all completed and cancelled goals older than maxAge.
func (e *Engine) CleanupCompleted(maxAge time.Duration) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, g := range e.goals {
		if (g.Status == GoalStatusCompleted || g.Status == GoalStatusCancelled) && g.UpdatedAt.Before(cutoff) {
			delete(e.goals, id)
			removed++
		}
	}
	return removed
}
