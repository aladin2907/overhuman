// Package budget implements cost tracking and limiting for Overhuman.
//
// The Budget Engine tracks LLM costs per task, per skill, and globally.
// It enforces daily and monthly spending limits and can automatically
// downgrade model selection when approaching limits.
package budget

import (
	"fmt"
	"sync"
	"time"
)

// Tracker records spending and enforces limits. Thread-safe.
type Tracker struct {
	mu sync.RWMutex

	// Limits.
	dailyLimit   float64
	monthlyLimit float64

	// Running totals.
	dailySpend   float64
	monthlySpend float64
	totalSpend   float64

	// Tracking by task and date.
	taskSpend map[string]float64
	dayKey    string // "2006-01-02" — reset daily when date changes
	monthKey  string // "2006-01" — reset monthly when month changes
}

// New creates a budget tracker with the given limits.
// Pass 0 for no limit.
func New(dailyLimit, monthlyLimit float64) *Tracker {
	now := time.Now()
	return &Tracker{
		dailyLimit:   dailyLimit,
		monthlyLimit: monthlyLimit,
		taskSpend:    make(map[string]float64),
		dayKey:       now.Format("2006-01-02"),
		monthKey:     now.Format("2006-01"),
	}
}

// Record records a cost against a task ID.
func (t *Tracker) Record(taskID string, costUSD float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.maybeReset()

	t.dailySpend += costUSD
	t.monthlySpend += costUSD
	t.totalSpend += costUSD
	t.taskSpend[taskID] += costUSD
}

// CanSpend returns true if spending the given amount would stay within limits.
func (t *Tracker) CanSpend(amount float64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.dailyLimit > 0 && t.dailySpend+amount > t.dailyLimit {
		return false
	}
	if t.monthlyLimit > 0 && t.monthlySpend+amount > t.monthlyLimit {
		return false
	}
	return true
}

// RemainingDaily returns the remaining daily budget. Returns -1 if no limit.
func (t *Tracker) RemainingDaily() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.dailyLimit <= 0 {
		return -1
	}
	remaining := t.dailyLimit - t.dailySpend
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RemainingMonthly returns the remaining monthly budget. Returns -1 if no limit.
func (t *Tracker) RemainingMonthly() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.monthlyLimit <= 0 {
		return -1
	}
	remaining := t.monthlyLimit - t.monthlySpend
	if remaining < 0 {
		return 0
	}
	return remaining
}

// DailySpend returns current daily spending.
func (t *Tracker) DailySpend() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dailySpend
}

// MonthlySpend returns current monthly spending.
func (t *Tracker) MonthlySpend() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.monthlySpend
}

// TotalSpend returns all-time spending.
func (t *Tracker) TotalSpend() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalSpend
}

// TaskSpend returns spending for a specific task.
func (t *Tracker) TaskSpend(taskID string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.taskSpend[taskID]
}

// BudgetStatus returns a human-readable status string.
func (t *Tracker) BudgetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	daily := "unlimited"
	if t.dailyLimit > 0 {
		daily = fmt.Sprintf("$%.4f / $%.2f (%.0f%%)", t.dailySpend, t.dailyLimit, t.dailySpend/t.dailyLimit*100)
	}
	monthly := "unlimited"
	if t.monthlyLimit > 0 {
		monthly = fmt.Sprintf("$%.4f / $%.2f (%.0f%%)", t.monthlySpend, t.monthlyLimit, t.monthlySpend/t.monthlyLimit*100)
	}
	return fmt.Sprintf("daily=%s monthly=%s total=$%.4f", daily, monthly, t.totalSpend)
}

// ShouldDowngrade returns true when we're close to hitting a limit (>80%).
// Used by the model router to select cheaper models.
func (t *Tracker) ShouldDowngrade() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.dailyLimit > 0 && t.dailySpend/t.dailyLimit > 0.8 {
		return true
	}
	if t.monthlyLimit > 0 && t.monthlySpend/t.monthlyLimit > 0.8 {
		return true
	}
	return false
}

// EffectiveBudget returns the remaining budget for the model router.
// Returns the minimum of daily and monthly remaining, or a large number if unlimited.
func (t *Tracker) EffectiveBudget() float64 {
	daily := t.RemainingDaily()
	monthly := t.RemainingMonthly()

	if daily < 0 && monthly < 0 {
		return 1000.0 // Unlimited.
	}
	if daily < 0 {
		return monthly
	}
	if monthly < 0 {
		return daily
	}
	if daily < monthly {
		return daily
	}
	return monthly
}

// maybeReset resets daily/monthly counters when the period changes.
// Must be called with mu held.
func (t *Tracker) maybeReset() {
	now := time.Now()
	day := now.Format("2006-01-02")
	month := now.Format("2006-01")

	if day != t.dayKey {
		t.dailySpend = 0
		t.dayKey = day
	}
	if month != t.monthKey {
		t.monthlySpend = 0
		t.monthKey = month
	}
}
