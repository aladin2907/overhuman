package genui

import (
	"strings"
	"sync"
	"testing"
)

func TestReflectionStore_Record(t *testing.T) {
	store := NewReflectionStore()

	r := UIReflection{
		TaskID:       "task-1",
		UIFormat:     FormatANSI,
		ActionsShown: []string{"apply_fix", "show_diff"},
		ActionsUsed:  []string{"apply_fix"},
		TimeToAction: 1500,
	}

	store.Record(r)

	records := store.Records()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", records[0].TaskID)
	}
	if records[0].UIFormat != FormatANSI {
		t.Errorf("expected format %s, got %s", FormatANSI, records[0].UIFormat)
	}
	if len(records[0].ActionsUsed) != 1 || records[0].ActionsUsed[0] != "apply_fix" {
		t.Errorf("unexpected ActionsUsed: %v", records[0].ActionsUsed)
	}
	if records[0].TimeToAction != 1500 {
		t.Errorf("expected TimeToAction 1500, got %d", records[0].TimeToAction)
	}
}

func TestReflectionStore_MultipleRecords(t *testing.T) {
	store := NewReflectionStore()

	for i, id := range []string{"task-a", "task-b", "task-c"} {
		store.Record(UIReflection{
			TaskID:       id,
			UIFormat:     FormatHTML,
			ActionsShown: []string{"action-" + id},
			TimeToAction: int64((i + 1) * 1000),
		})
	}

	records := store.Records()
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	ids := map[string]bool{}
	for _, r := range records {
		ids[r.TaskID] = true
	}
	for _, want := range []string{"task-a", "task-b", "task-c"} {
		if !ids[want] {
			t.Errorf("missing record with TaskID %s", want)
		}
	}
}

func TestReflectionStore_NoAction_Dismissed(t *testing.T) {
	store := NewReflectionStore()

	store.Record(UIReflection{
		TaskID:    "task-dismissed",
		UIFormat:  FormatANSI,
		Dismissed: true,
	})

	hints := store.BuildHints("any-fingerprint")
	if len(hints) == 0 {
		t.Fatal("expected at least one hint for dismissed UI")
	}

	found := false
	for _, h := range hints {
		if strings.Contains(h, "simpler layout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected hint mentioning 'simpler layout', got: %v", hints)
	}
}

func TestReflectionStore_ActionUsed(t *testing.T) {
	store := NewReflectionStore()

	store.Record(UIReflection{
		TaskID:       "task-action",
		UIFormat:     FormatHTML,
		ActionsShown: []string{"deploy", "rollback"},
		ActionsUsed:  []string{"deploy"},
		TimeToAction: 800,
	})

	hints := store.BuildHints("any-fingerprint")
	if len(hints) == 0 {
		t.Fatal("expected at least one hint for action used")
	}

	found := false
	for _, h := range hints {
		if strings.Contains(h, "deploy") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected hint mentioning 'deploy', got: %v", hints)
	}
}

func TestReflectionStore_Scrolled(t *testing.T) {
	store := NewReflectionStore()

	store.Record(UIReflection{
		TaskID:   "task-scroll",
		UIFormat: FormatANSI,
		Scrolled: true,
	})

	hints := store.BuildHints("any-fingerprint")
	if len(hints) == 0 {
		t.Fatal("expected at least one hint for scrolled UI")
	}

	found := false
	for _, h := range hints {
		if strings.Contains(h, "compact layout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected hint mentioning 'compact layout', got: %v", hints)
	}
}

func TestReflectionStore_BuildHints_Empty(t *testing.T) {
	store := NewReflectionStore()

	hints := store.BuildHints("no-records")
	if len(hints) != 0 {
		t.Errorf("expected no hints for empty store, got %d: %v", len(hints), hints)
	}
}

func TestReflectionStore_BuildHints_Combined(t *testing.T) {
	store := NewReflectionStore()

	// Record with dismissed flag
	store.Record(UIReflection{
		TaskID:    "task-1",
		UIFormat:  FormatANSI,
		Dismissed: true,
	})

	// Record with actions used
	store.Record(UIReflection{
		TaskID:       "task-2",
		UIFormat:     FormatHTML,
		ActionsShown: []string{"save", "cancel"},
		ActionsUsed:  []string{"save"},
		TimeToAction: 500,
	})

	// Record with scrolled flag
	store.Record(UIReflection{
		TaskID:   "task-3",
		UIFormat: FormatANSI,
		Scrolled: true,
	})

	hints := store.BuildHints("combined")

	if len(hints) < 3 {
		t.Fatalf("expected at least 3 hints, got %d: %v", len(hints), hints)
	}

	var hasSimpler, hasAction, hasCompact bool
	for _, h := range hints {
		if strings.Contains(h, "simpler layout") {
			hasSimpler = true
		}
		if strings.Contains(h, "save") {
			hasAction = true
		}
		if strings.Contains(h, "compact layout") {
			hasCompact = true
		}
	}

	if !hasSimpler {
		t.Error("missing hint about simpler layout (dismissed)")
	}
	if !hasAction {
		t.Error("missing hint about action 'save'")
	}
	if !hasCompact {
		t.Error("missing hint about compact layout (scrolled)")
	}
}

func TestReflectionStore_ThreadSafe(t *testing.T) {
	store := NewReflectionStore()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			store.Record(UIReflection{
				TaskID:       "task-concurrent",
				UIFormat:     FormatANSI,
				ActionsShown: []string{"action"},
				ActionsUsed:  []string{"action"},
				Scrolled:     n%2 == 0,
				Dismissed:    n%3 == 0,
			})
		}(i)
	}

	wg.Wait()

	records := store.Records()
	if len(records) != goroutines {
		t.Errorf("expected %d records, got %d", goroutines, len(records))
	}

	// Also verify BuildHints doesn't panic under concurrent read after writes
	var wg2 sync.WaitGroup
	wg2.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg2.Done()
			_ = store.BuildHints("fp")
		}()
	}
	wg2.Wait()
}
