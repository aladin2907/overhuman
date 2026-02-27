package budget

import (
	"strings"
	"testing"
)

func TestTracker_Record(t *testing.T) {
	tr := New(10.0, 100.0)

	tr.Record("task_1", 0.05)
	tr.Record("task_1", 0.03)
	tr.Record("task_2", 0.10)

	if tr.DailySpend() != 0.18 {
		t.Errorf("DailySpend = %f, want 0.18", tr.DailySpend())
	}
	if tr.MonthlySpend() != 0.18 {
		t.Errorf("MonthlySpend = %f, want 0.18", tr.MonthlySpend())
	}
	if tr.TotalSpend() != 0.18 {
		t.Errorf("TotalSpend = %f, want 0.18", tr.TotalSpend())
	}
	if tr.TaskSpend("task_1") != 0.08 {
		t.Errorf("TaskSpend(task_1) = %f, want 0.08", tr.TaskSpend("task_1"))
	}
	if tr.TaskSpend("task_2") != 0.10 {
		t.Errorf("TaskSpend(task_2) = %f, want 0.10", tr.TaskSpend("task_2"))
	}
}

func TestTracker_CanSpend(t *testing.T) {
	tr := New(1.0, 10.0)

	if !tr.CanSpend(0.5) {
		t.Error("should be able to spend 0.5")
	}

	tr.Record("t", 0.8)

	if !tr.CanSpend(0.1) {
		t.Error("should still be able to spend 0.1 (total 0.9 < 1.0)")
	}
	if tr.CanSpend(0.3) {
		t.Error("should NOT be able to spend 0.3 (total 1.1 > 1.0)")
	}
}

func TestTracker_CanSpend_MonthlyLimit(t *testing.T) {
	tr := New(0, 0.50) // No daily limit, $0.50 monthly.

	tr.Record("t", 0.40)

	if !tr.CanSpend(0.05) {
		t.Error("should be able to spend 0.05 (0.45 < 0.50)")
	}
	if tr.CanSpend(0.20) {
		t.Error("should NOT be able to spend 0.20 (0.60 > 0.50)")
	}
}

func TestTracker_CanSpend_Unlimited(t *testing.T) {
	tr := New(0, 0) // No limits.

	tr.Record("t", 999.0)
	if !tr.CanSpend(999.0) {
		t.Error("unlimited budget should always allow spending")
	}
}

func TestTracker_RemainingDaily(t *testing.T) {
	tr := New(5.0, 100.0)

	if tr.RemainingDaily() != 5.0 {
		t.Errorf("remaining = %f, want 5.0", tr.RemainingDaily())
	}

	tr.Record("t", 3.0)
	if tr.RemainingDaily() != 2.0 {
		t.Errorf("remaining = %f, want 2.0", tr.RemainingDaily())
	}

	tr.Record("t", 3.0) // Over limit.
	if tr.RemainingDaily() != 0 {
		t.Errorf("remaining = %f, want 0", tr.RemainingDaily())
	}
}

func TestTracker_RemainingDaily_Unlimited(t *testing.T) {
	tr := New(0, 0)
	if tr.RemainingDaily() != -1 {
		t.Errorf("remaining = %f, want -1 (unlimited)", tr.RemainingDaily())
	}
}

func TestTracker_RemainingMonthly(t *testing.T) {
	tr := New(10.0, 50.0)

	tr.Record("t", 20.0)
	if tr.RemainingMonthly() != 30.0 {
		t.Errorf("remaining = %f, want 30.0", tr.RemainingMonthly())
	}
}

func TestTracker_ShouldDowngrade(t *testing.T) {
	tr := New(1.0, 10.0)

	if tr.ShouldDowngrade() {
		t.Error("should not downgrade when fresh")
	}

	tr.Record("t", 0.85) // 85% of daily.
	if !tr.ShouldDowngrade() {
		t.Error("should downgrade at 85% of daily limit")
	}
}

func TestTracker_ShouldDowngrade_Monthly(t *testing.T) {
	tr := New(0, 10.0) // No daily limit.

	tr.Record("t", 8.5) // 85% of monthly.
	if !tr.ShouldDowngrade() {
		t.Error("should downgrade at 85% of monthly limit")
	}
}

func TestTracker_EffectiveBudget(t *testing.T) {
	tr := New(5.0, 30.0)

	if tr.EffectiveBudget() != 5.0 {
		t.Errorf("effective = %f, want 5.0 (min of daily and monthly)", tr.EffectiveBudget())
	}

	tr.Record("t", 3.0)
	if tr.EffectiveBudget() != 2.0 {
		t.Errorf("effective = %f, want 2.0", tr.EffectiveBudget())
	}
}

func TestTracker_EffectiveBudget_Unlimited(t *testing.T) {
	tr := New(0, 0)
	if tr.EffectiveBudget() != 1000.0 {
		t.Errorf("effective = %f, want 1000.0 (unlimited)", tr.EffectiveBudget())
	}
}

func TestTracker_EffectiveBudget_OnlyDaily(t *testing.T) {
	tr := New(5.0, 0)
	if tr.EffectiveBudget() != 5.0 {
		t.Errorf("effective = %f, want 5.0", tr.EffectiveBudget())
	}
}

func TestTracker_EffectiveBudget_OnlyMonthly(t *testing.T) {
	tr := New(0, 20.0)
	if tr.EffectiveBudget() != 20.0 {
		t.Errorf("effective = %f, want 20.0", tr.EffectiveBudget())
	}
}

func TestTracker_BudgetStatus(t *testing.T) {
	tr := New(5.0, 50.0)
	tr.Record("t", 1.5)

	status := tr.BudgetStatus()
	if !strings.Contains(status, "daily=") {
		t.Errorf("status should contain 'daily=': %s", status)
	}
	if !strings.Contains(status, "monthly=") {
		t.Errorf("status should contain 'monthly=': %s", status)
	}
	if !strings.Contains(status, "total=") {
		t.Errorf("status should contain 'total=': %s", status)
	}
}

func TestTracker_BudgetStatus_Unlimited(t *testing.T) {
	tr := New(0, 0)
	status := tr.BudgetStatus()
	if !strings.Contains(status, "unlimited") {
		t.Errorf("status should say unlimited: %s", status)
	}
}

func TestTracker_TaskSpend_Unknown(t *testing.T) {
	tr := New(0, 0)
	if tr.TaskSpend("nonexistent") != 0 {
		t.Error("unknown task should return 0")
	}
}
