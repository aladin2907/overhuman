// Package versioning implements the observation window and auto-rollback
// mechanism for mutable entities (soul, skills, policies).
//
// After any change, the system enters an "observation window" of N runs.
// If metrics degrade during the window, the change is automatically rolled back.
package versioning

import (
	"fmt"
	"sync"
	"time"
)

// ChangeType identifies what kind of entity was changed.
type ChangeType string

const (
	ChangeSoul   ChangeType = "SOUL"
	ChangeSkill  ChangeType = "SKILL"
	ChangePolicy ChangeType = "POLICY"
)

// ChangeStatus tracks where a change is in its lifecycle.
type ChangeStatus string

const (
	StatusObserving ChangeStatus = "OBSERVING"
	StatusAccepted  ChangeStatus = "ACCEPTED"
	StatusRolledBack ChangeStatus = "ROLLED_BACK"
)

// Change records a single mutation with its observation window.
type Change struct {
	ID          string       `json:"id"`
	Type        ChangeType   `json:"type"`
	EntityID    string       `json:"entity_id"`    // e.g., soul path or skill ID
	Description string       `json:"description"`
	Status      ChangeStatus `json:"status"`

	// Metrics before/after.
	BaselineQuality float64 `json:"baseline_quality"` // Average quality before change
	BaselineCost    float64 `json:"baseline_cost"`    // Average cost before change
	CurrentQuality  float64 `json:"current_quality"`
	CurrentCost     float64 `json:"current_cost"`

	// Observation tracking.
	WindowSize   int       `json:"window_size"`    // How many runs to observe
	RunsObserved int       `json:"runs_observed"`
	Threshold    float64   `json:"threshold"`      // Minimum acceptable quality ratio (e.g., 0.9 = 90% of baseline)
	CreatedAt    time.Time `json:"created_at"`
	DecidedAt    time.Time `json:"decided_at,omitempty"`

	// Rollback info.
	RollbackData string `json:"rollback_data,omitempty"` // Serialized previous state
}

// Controller manages observation windows and auto-rollback.
type Controller struct {
	mu      sync.RWMutex
	changes map[string]*Change
	nextID  int

	// Defaults.
	defaultWindow    int
	defaultThreshold float64
}

// New creates a version controller.
func New() *Controller {
	return &Controller{
		changes:          make(map[string]*Change),
		defaultWindow:    5,
		defaultThreshold: 0.9, // Rollback if quality drops below 90% of baseline.
	}
}

// SetDefaultWindow sets the default observation window size.
func (c *Controller) SetDefaultWindow(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > 0 {
		c.defaultWindow = n
	}
}

// SetDefaultThreshold sets the default quality threshold ratio.
func (c *Controller) SetDefaultThreshold(t float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultThreshold = t
}

// RecordChange registers a new change and starts the observation window.
func (c *Controller) RecordChange(
	changeType ChangeType,
	entityID string,
	description string,
	baselineQuality float64,
	baselineCost float64,
	rollbackData string,
) *Change {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	ch := &Change{
		ID:              fmt.Sprintf("change_%d", c.nextID),
		Type:            changeType,
		EntityID:        entityID,
		Description:     description,
		Status:          StatusObserving,
		BaselineQuality: baselineQuality,
		BaselineCost:    baselineCost,
		WindowSize:      c.defaultWindow,
		Threshold:       c.defaultThreshold,
		CreatedAt:       time.Now(),
		RollbackData:    rollbackData,
	}
	c.changes[ch.ID] = ch
	return ch
}

// ObserveRun records a run's metrics against all active observation windows
// for the given entity. Returns any change IDs that should be rolled back.
func (c *Controller) ObserveRun(entityID string, quality, cost float64) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var rollbacks []string

	for _, ch := range c.changes {
		if ch.Status != StatusObserving || ch.EntityID != entityID {
			continue
		}

		// Running average of observed metrics.
		n := float64(ch.RunsObserved)
		ch.CurrentQuality = (ch.CurrentQuality*n + quality) / (n + 1)
		ch.CurrentCost = (ch.CurrentCost*n + cost) / (n + 1)
		ch.RunsObserved++

		// Check if observation window is complete.
		if ch.RunsObserved >= ch.WindowSize {
			if ch.BaselineQuality > 0 && ch.CurrentQuality/ch.BaselineQuality < ch.Threshold {
				ch.Status = StatusRolledBack
				ch.DecidedAt = time.Now()
				rollbacks = append(rollbacks, ch.ID)
			} else {
				ch.Status = StatusAccepted
				ch.DecidedAt = time.Now()
			}
		}
	}

	return rollbacks
}

// Get returns a change by ID.
func (c *Controller) Get(id string) *Change {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.changes[id]
}

// ActiveChanges returns all changes currently being observed.
func (c *Controller) ActiveChanges() []*Change {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Change
	for _, ch := range c.changes {
		if ch.Status == StatusObserving {
			result = append(result, ch)
		}
	}
	return result
}

// RolledBack returns all changes that were rolled back.
func (c *Controller) RolledBack() []*Change {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Change
	for _, ch := range c.changes {
		if ch.Status == StatusRolledBack {
			result = append(result, ch)
		}
	}
	return result
}

// ForceAccept immediately accepts a change (skipping remaining observation).
func (c *Controller) ForceAccept(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.changes[id]
	if !ok {
		return fmt.Errorf("change %q not found", id)
	}
	ch.Status = StatusAccepted
	ch.DecidedAt = time.Now()
	return nil
}

// ForceRollback immediately triggers rollback.
func (c *Controller) ForceRollback(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.changes[id]
	if !ok {
		return fmt.Errorf("change %q not found", id)
	}
	ch.Status = StatusRolledBack
	ch.DecidedAt = time.Now()
	return nil
}

// Count returns total number of tracked changes.
func (c *Controller) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.changes)
}
