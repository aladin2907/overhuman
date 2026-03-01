package genui

import (
	"context"
	"math"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

func TestDefaultABTestConfig(t *testing.T) {
	cfg := DefaultABTestConfig()

	if cfg.TestProbability != 0.1 {
		t.Errorf("TestProbability = %f, want 0.1", cfg.TestProbability)
	}
	if cfg.MinSampleSize != 3 {
		t.Errorf("MinSampleSize = %d, want 3", cfg.MinSampleSize)
	}
	if cfg.ScoreDiffThreshold != 0.15 {
		t.Errorf("ScoreDiffThreshold = %f, want 0.15", cfg.ScoreDiffThreshold)
	}
}

func TestNewABTestEngine(t *testing.T) {
	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	if eng == nil {
		t.Fatal("NewABTestEngine returned nil")
	}
	if eng.active == nil {
		t.Error("active map should be initialized")
	}
	if eng.rng == nil {
		t.Error("rng should be initialized")
	}
	if len(eng.active) != 0 {
		t.Errorf("active should be empty, got %d", len(eng.active))
	}
	if len(eng.history) != 0 {
		t.Errorf("history should be empty, got %d", len(eng.history))
	}
}

func TestABTestEngine_ShouldTest(t *testing.T) {
	// Set probability to 1.0 so it always triggers.
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	if !eng.ShouldTest("fp-1") {
		t.Error("ShouldTest should return true with probability=1.0")
	}

	// Set probability to 0.0 so it never triggers.
	cfg2 := ABTestConfig{
		TestProbability:    0.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	eng2 := NewABTestEngine(cfg2, mem)

	if eng2.ShouldTest("fp-2") {
		t.Error("ShouldTest should return false with probability=0.0")
	}
}

func TestABTestEngine_ShouldTest_AlreadyActive(t *testing.T) {
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	// Manually insert an active test for "fp-active".
	eng.active["fp-active"] = &ABTest{
		ID:          "ab_existing",
		Fingerprint: "fp-active",
	}

	if eng.ShouldTest("fp-active") {
		t.Error("ShouldTest should return false when active test exists for fingerprint")
	}
}

func TestABTestEngine_CreateTest(t *testing.T) {
	callCount := 0
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		callCount++
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	result := pipeline.RunResult{
		TaskID:       "task-ab-1",
		Success:      true,
		Result:       "Test result for A/B",
		QualityScore: 0.85,
		Fingerprint:  "fp-ab-create",
	}
	caps := CLICapabilities()

	test, err := eng.CreateTest(context.Background(), gen, result, caps, nil, nil)
	if err != nil {
		t.Fatalf("CreateTest: %v", err)
	}
	if test == nil {
		t.Fatal("CreateTest returned nil test")
	}
	if test.VariantA == nil {
		t.Error("VariantA should not be nil")
	}
	if test.VariantB == nil {
		t.Error("VariantB should not be nil")
	}
	if test.Fingerprint != "fp-ab-create" {
		t.Errorf("Fingerprint = %q, want fp-ab-create", test.Fingerprint)
	}
	if test.ID == "" {
		t.Error("test ID should not be empty")
	}

	// Verify variant titles are prefixed.
	if test.VariantA.Meta.Title == "" {
		t.Error("VariantA title should not be empty")
	}
	if test.VariantB.Meta.Title == "" {
		t.Error("VariantB title should not be empty")
	}

	// CreateTest should have called GenerateWithThought twice (once per variant).
	if callCount < 2 {
		t.Errorf("expected at least 2 LLM calls, got %d", callCount)
	}

	// Test should be in active map.
	active := eng.ActiveTests()
	if len(active) != 1 {
		t.Errorf("expected 1 active test, got %d", len(active))
	}
}

func TestABTestEngine_PickVariant(t *testing.T) {
	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	test := &ABTest{
		ID:          "ab_pick",
		Fingerprint: "fp-pick",
		VariantA:    &GeneratedUI{Code: "variant-a-code"},
		VariantB:    &GeneratedUI{Code: "variant-b-code"},
	}

	// Run many times and verify both variants are selected.
	sawA, sawB := false, false
	for i := 0; i < 100; i++ {
		ui, variant := eng.PickVariant(test)
		if variant == VariantA {
			sawA = true
			if ui.Code != "variant-a-code" {
				t.Errorf("VariantA UI code mismatch")
			}
		} else if variant == VariantB {
			sawB = true
			if ui.Code != "variant-b-code" {
				t.Errorf("VariantB UI code mismatch")
			}
		} else {
			t.Errorf("unexpected variant: %q", variant)
		}
	}

	if !sawA {
		t.Error("should have picked VariantA at least once in 100 runs")
	}
	if !sawB {
		t.Error("should have picked VariantB at least once in 100 runs")
	}
}

func TestABTestEngine_RecordResult(t *testing.T) {
	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	// Insert an active test.
	eng.active["fp-rec"] = &ABTest{
		ID:          "ab_rec",
		Fingerprint: "fp-rec",
		VariantA:    &GeneratedUI{},
		VariantB:    &GeneratedUI{},
	}

	// Record a good result for variant A (not dismissed, has actions).
	reflection := UIReflection{
		TaskID:      "task-rec",
		Dismissed:   false,
		ActionsUsed: []string{"apply", "view"},
	}
	eng.RecordResult("fp-rec", VariantA, reflection)

	test := eng.active["fp-rec"]
	if test.ScoreA == 0 {
		t.Error("ScoreA should be updated after recording result")
	}
	if test.ScoreB != 0 {
		t.Error("ScoreB should remain 0 since no result was recorded for B")
	}
}

func TestABTestEngine_CheckWinner_NotReady(t *testing.T) {
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	// Insert a test with scores that are close together.
	eng.active["fp-close"] = &ABTest{
		ID:          "ab_close",
		Fingerprint: "fp-close",
		ScoreA:      0.50,
		ScoreB:      0.52, // diff = 0.02, below threshold 0.15
	}

	winner, resolved := eng.CheckWinner("fp-close")
	if resolved {
		t.Error("should not be resolved when scores are close")
	}
	if winner != "" {
		t.Errorf("winner should be empty, got %q", winner)
	}
}

func TestABTestEngine_CheckWinner_AIsWinner(t *testing.T) {
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	eng.active["fp-a-wins"] = &ABTest{
		ID:          "ab_a_wins",
		Fingerprint: "fp-a-wins",
		ScoreA:      0.85,
		ScoreB:      0.40, // diff = 0.45, above threshold 0.15
	}

	winner, resolved := eng.CheckWinner("fp-a-wins")
	if !resolved {
		t.Error("should be resolved when score diff exceeds threshold")
	}
	if winner != VariantA {
		t.Errorf("winner = %q, want %q", winner, VariantA)
	}
}

func TestABTestEngine_CheckWinner_BIsWinner(t *testing.T) {
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	eng.active["fp-b-wins"] = &ABTest{
		ID:          "ab_b_wins",
		Fingerprint: "fp-b-wins",
		ScoreA:      0.30,
		ScoreB:      0.80, // diff = 0.50, above threshold
	}

	winner, resolved := eng.CheckWinner("fp-b-wins")
	if !resolved {
		t.Error("should be resolved when score diff exceeds threshold")
	}
	if winner != VariantB {
		t.Errorf("winner = %q, want %q", winner, VariantB)
	}
}

func TestABTestEngine_CheckWinner_MovesToHistory(t *testing.T) {
	cfg := ABTestConfig{
		TestProbability:    1.0,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	eng.active["fp-hist"] = &ABTest{
		ID:          "ab_hist",
		Fingerprint: "fp-hist",
		ScoreA:      0.90,
		ScoreB:      0.30,
	}

	_, resolved := eng.CheckWinner("fp-hist")
	if !resolved {
		t.Fatal("should be resolved")
	}

	// Active map should no longer contain the test.
	if _, exists := eng.active["fp-hist"]; exists {
		t.Error("resolved test should be removed from active map")
	}

	// History should contain the resolved test.
	history := eng.History()
	if len(history) != 1 {
		t.Fatalf("history should have 1 entry, got %d", len(history))
	}
	if history[0].ID != "ab_hist" {
		t.Errorf("history entry ID = %q, want ab_hist", history[0].ID)
	}
	if history[0].Winner != VariantA {
		t.Errorf("history winner = %q, want A", history[0].Winner)
	}
	if history[0].ResolvedAt.IsZero() {
		t.Error("ResolvedAt should be set")
	}
}

func TestABTestEngine_ActiveTests(t *testing.T) {
	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	// Start with empty.
	if len(eng.ActiveTests()) != 0 {
		t.Error("expected 0 active tests initially")
	}

	// Add two active tests.
	eng.active["fp-1"] = &ABTest{ID: "ab_1", Fingerprint: "fp-1"}
	eng.active["fp-2"] = &ABTest{ID: "ab_2", Fingerprint: "fp-2"}

	active := eng.ActiveTests()
	if len(active) != 2 {
		t.Errorf("expected 2 active tests, got %d", len(active))
	}

	// Verify they are copies (not pointers to internal state).
	ids := map[string]bool{}
	for _, a := range active {
		ids[a.ID] = true
	}
	if !ids["ab_1"] || !ids["ab_2"] {
		t.Errorf("expected ab_1 and ab_2 in active tests, got %v", ids)
	}
}

func TestABTestEngine_History(t *testing.T) {
	cfg := DefaultABTestConfig()
	mem := NewUIMemory(10)
	eng := NewABTestEngine(cfg, mem)

	// Start with empty history.
	if len(eng.History()) != 0 {
		t.Error("expected 0 history entries initially")
	}

	// Add history entries manually.
	eng.history = append(eng.history,
		ABTest{ID: "ab_h1", Winner: VariantA},
		ABTest{ID: "ab_h2", Winner: VariantB},
	)

	hist := eng.History()
	if len(hist) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(hist))
	}
	if hist[0].ID != "ab_h1" {
		t.Errorf("first history entry ID = %q, want ab_h1", hist[0].ID)
	}
	if hist[1].ID != "ab_h2" {
		t.Errorf("second history entry ID = %q, want ab_h2", hist[1].ID)
	}

	// Verify returned slice is a copy.
	hist[0].ID = "modified"
	origHist := eng.History()
	if origHist[0].ID == "modified" {
		t.Error("History should return a copy, not a reference to internal state")
	}
}

func TestRunningAvg(t *testing.T) {
	// First value: when old is 0, returns the new value.
	result := runningAvg(0, 1.0)
	if result != 1.0 {
		t.Errorf("runningAvg(0, 1.0) = %f, want 1.0", result)
	}

	// EMA: 0.7*old + 0.3*new.
	result2 := runningAvg(1.0, 0.0)
	expected := 1.0*0.7 + 0.0*0.3
	if math.Abs(result2-expected) > 1e-9 {
		t.Errorf("runningAvg(1.0, 0.0) = %f, want %f", result2, expected)
	}

	// Multiple updates converge.
	avg := runningAvg(0, 0.8)
	avg = runningAvg(avg, 0.8)
	avg = runningAvg(avg, 0.8)
	if avg < 0.7 || avg > 0.9 {
		t.Errorf("repeated runningAvg(x, 0.8) should converge near 0.8, got %f", avg)
	}
}
