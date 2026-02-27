package versioning

import (
	"testing"
)

func TestRecordChange(t *testing.T) {
	c := New()
	ch := c.RecordChange(ChangeSoul, "soul.md", "Updated strategy", 0.85, 0.03, "old content")

	if ch.ID == "" {
		t.Error("ID should not be empty")
	}
	if ch.Status != StatusObserving {
		t.Errorf("Status = %q, want OBSERVING", ch.Status)
	}
	if ch.Type != ChangeSoul {
		t.Errorf("Type = %q", ch.Type)
	}
	if ch.BaselineQuality != 0.85 {
		t.Errorf("BaselineQuality = %f", ch.BaselineQuality)
	}
	if ch.RollbackData != "old content" {
		t.Error("rollback data should be preserved")
	}
}

func TestObserveRun_AcceptsGoodChange(t *testing.T) {
	c := New()
	c.SetDefaultWindow(3)
	c.SetDefaultThreshold(0.9)

	ch := c.RecordChange(ChangeSkill, "skill_1", "New code skill", 0.80, 0.05, "")

	// Observe 3 good runs (quality >= 90% of baseline).
	for i := 0; i < 3; i++ {
		rollbacks := c.ObserveRun("skill_1", 0.85, 0.04)
		if len(rollbacks) > 0 {
			t.Error("should not rollback good change")
		}
	}

	got := c.Get(ch.ID)
	if got.Status != StatusAccepted {
		t.Errorf("Status = %q, want ACCEPTED", got.Status)
	}
	if got.RunsObserved != 3 {
		t.Errorf("RunsObserved = %d, want 3", got.RunsObserved)
	}
}

func TestObserveRun_RollsBackBadChange(t *testing.T) {
	c := New()
	c.SetDefaultWindow(3)
	c.SetDefaultThreshold(0.9)

	ch := c.RecordChange(ChangeSkill, "skill_1", "Bad update", 0.80, 0.05, "old version")

	// Observe 3 bad runs (quality < 90% of 0.80 = 0.72).
	for i := 0; i < 3; i++ {
		rollbacks := c.ObserveRun("skill_1", 0.50, 0.10)
		if i == 2 && len(rollbacks) == 0 {
			t.Error("should trigger rollback on 3rd run")
		}
	}

	got := c.Get(ch.ID)
	if got.Status != StatusRolledBack {
		t.Errorf("Status = %q, want ROLLED_BACK", got.Status)
	}
}

func TestObserveRun_IgnoresOtherEntities(t *testing.T) {
	c := New()
	c.SetDefaultWindow(2)

	c.RecordChange(ChangeSkill, "skill_1", "change A", 0.80, 0.05, "")

	// Observing a different entity should not affect skill_1.
	rollbacks := c.ObserveRun("skill_2", 0.10, 0.50)
	if len(rollbacks) > 0 {
		t.Error("should not affect unrelated entities")
	}

	// skill_1's change should still be observing.
	active := c.ActiveChanges()
	if len(active) != 1 {
		t.Errorf("active = %d, want 1", len(active))
	}
}

func TestObserveRun_ZeroBaseline(t *testing.T) {
	c := New()
	c.SetDefaultWindow(2)

	c.RecordChange(ChangeSkill, "new_skill", "Brand new", 0, 0, "")

	// Zero baseline → should accept (can't be worse than 0).
	c.ObserveRun("new_skill", 0.5, 0.01)
	c.ObserveRun("new_skill", 0.5, 0.01)

	active := c.ActiveChanges()
	if len(active) != 0 {
		t.Error("should be decided with zero baseline")
	}
}

func TestForceAccept(t *testing.T) {
	c := New()
	ch := c.RecordChange(ChangeSkill, "skill_1", "test", 0.8, 0.05, "")

	if err := c.ForceAccept(ch.ID); err != nil {
		t.Fatalf("ForceAccept: %v", err)
	}

	got := c.Get(ch.ID)
	if got.Status != StatusAccepted {
		t.Errorf("Status = %q, want ACCEPTED", got.Status)
	}
	if got.DecidedAt.IsZero() {
		t.Error("DecidedAt should be set")
	}
}

func TestForceAccept_NotFound(t *testing.T) {
	c := New()
	if err := c.ForceAccept("nonexistent"); err == nil {
		t.Error("expected error for nonexistent change")
	}
}

func TestForceRollback(t *testing.T) {
	c := New()
	ch := c.RecordChange(ChangePolicy, "policy_1", "test", 0.9, 0.02, "old policy")

	if err := c.ForceRollback(ch.ID); err != nil {
		t.Fatalf("ForceRollback: %v", err)
	}

	got := c.Get(ch.ID)
	if got.Status != StatusRolledBack {
		t.Errorf("Status = %q, want ROLLED_BACK", got.Status)
	}
}

func TestForceRollback_NotFound(t *testing.T) {
	c := New()
	if err := c.ForceRollback("nonexistent"); err == nil {
		t.Error("expected error for nonexistent change")
	}
}

func TestRolledBack(t *testing.T) {
	c := New()
	c.SetDefaultWindow(1)
	c.SetDefaultThreshold(0.9)

	c.RecordChange(ChangeSkill, "s1", "bad change", 0.80, 0.05, "old")
	c.ObserveRun("s1", 0.10, 0.20) // Very bad → rollback

	rolled := c.RolledBack()
	if len(rolled) != 1 {
		t.Errorf("RolledBack() = %d, want 1", len(rolled))
	}
}

func TestCount(t *testing.T) {
	c := New()
	if c.Count() != 0 {
		t.Errorf("Count = %d, want 0", c.Count())
	}

	c.RecordChange(ChangeSoul, "soul", "a", 0.8, 0.03, "")
	c.RecordChange(ChangeSkill, "skill", "b", 0.7, 0.04, "")

	if c.Count() != 2 {
		t.Errorf("Count = %d, want 2", c.Count())
	}
}

func TestMultipleChanges_SameEntity(t *testing.T) {
	c := New()
	c.SetDefaultWindow(2)
	c.SetDefaultThreshold(0.9)

	ch1 := c.RecordChange(ChangeSkill, "skill_1", "first change", 0.80, 0.05, "v1")
	ch2 := c.RecordChange(ChangeSkill, "skill_1", "second change", 0.80, 0.05, "v2")

	// Both should be observing the same entity.
	c.ObserveRun("skill_1", 0.85, 0.04)
	c.ObserveRun("skill_1", 0.85, 0.04)

	got1 := c.Get(ch1.ID)
	got2 := c.Get(ch2.ID)

	if got1.Status != StatusAccepted {
		t.Errorf("ch1 Status = %q, want ACCEPTED", got1.Status)
	}
	if got2.Status != StatusAccepted {
		t.Errorf("ch2 Status = %q, want ACCEPTED", got2.Status)
	}
}