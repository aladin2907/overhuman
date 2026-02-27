package evolution

import (
	"testing"

	"github.com/overhuman/overhuman/internal/instruments"
)

func TestComputeFitness_PerfectSkill(t *testing.T) {
	e := New()
	meta := instruments.SkillMeta{
		TotalRuns:   100,
		SuccessRate: 1.0,
		AvgQuality:  1.0,
		AvgCostUSD:  0.0, // Code skill = free
		AvgElapsedMs: 1,  // 1ms
	}
	fitness := e.ComputeFitness(meta)
	if fitness < 0.9 {
		t.Errorf("perfect skill fitness = %f, want > 0.9", fitness)
	}
}

func TestComputeFitness_TerribleSkill(t *testing.T) {
	e := New()
	meta := instruments.SkillMeta{
		TotalRuns:    100,
		SuccessRate:  0.1,
		AvgQuality:   0.1,
		AvgCostUSD:   0.20,
		AvgElapsedMs: 50000,
	}
	fitness := e.ComputeFitness(meta)
	if fitness > 0.3 {
		t.Errorf("terrible skill fitness = %f, want < 0.3", fitness)
	}
}

func TestComputeFitness_UntestedSkill(t *testing.T) {
	e := New()
	meta := instruments.SkillMeta{TotalRuns: 0}
	fitness := e.ComputeFitness(meta)
	if fitness != 0.5 {
		t.Errorf("untested skill fitness = %f, want 0.5", fitness)
	}
}

func TestComputeFitness_CodeVsLLM(t *testing.T) {
	e := New()

	code := instruments.SkillMeta{
		TotalRuns: 50, SuccessRate: 0.95, AvgQuality: 0.8,
		AvgCostUSD: 0, AvgElapsedMs: 5,
	}
	llm := instruments.SkillMeta{
		TotalRuns: 50, SuccessRate: 0.95, AvgQuality: 0.8,
		AvgCostUSD: 0.05, AvgElapsedMs: 2000,
	}

	codeFitness := e.ComputeFitness(code)
	llmFitness := e.ComputeFitness(llm)

	if codeFitness <= llmFitness {
		t.Errorf("code skill (%f) should be fitter than LLM skill (%f)", codeFitness, llmFitness)
	}
}

func TestShouldDeprecate(t *testing.T) {
	e := New()
	e.SetDeprecateThreshold(0.3)
	e.SetObservationRuns(5)

	// Not enough runs.
	young := instruments.SkillMeta{TotalRuns: 2, SuccessRate: 0}
	if e.ShouldDeprecate(young) {
		t.Error("should not deprecate with insufficient runs")
	}

	// Bad skill with enough runs.
	bad := instruments.SkillMeta{
		TotalRuns: 10, SuccessRate: 0.1, AvgQuality: 0.05,
		AvgCostUSD: 0.15, AvgElapsedMs: 30000,
	}
	if !e.ShouldDeprecate(bad) {
		t.Errorf("should deprecate bad skill (fitness=%f)", e.ComputeFitness(bad))
	}

	// Good skill.
	good := instruments.SkillMeta{
		TotalRuns: 10, SuccessRate: 0.95, AvgQuality: 0.9,
		AvgCostUSD: 0.001, AvgElapsedMs: 10,
	}
	if e.ShouldDeprecate(good) {
		t.Errorf("should NOT deprecate good skill (fitness=%f)", e.ComputeFitness(good))
	}
}

func TestABTest_Lifecycle(t *testing.T) {
	e := New()
	e.SetObservationRuns(3)

	reg := instruments.NewSkillRegistry()
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{
		ID: "inc", TotalRuns: 10, SuccessRate: 0.8, AvgQuality: 0.7,
		AvgCostUSD: 0.05, AvgElapsedMs: 1000,
	}})
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{
		ID: "chal", TotalRuns: 10, SuccessRate: 0.95, AvgQuality: 0.9,
		AvgCostUSD: 0.001, AvgElapsedMs: 5,
	}})

	test := e.StartABTest("inc", "chal", "fp_1")
	if test.ID == "" {
		t.Fatal("test ID should not be empty")
	}

	// Record runs.
	for i := 0; i < 3; i++ {
		e.RecordABRun(test.ID, "inc")
		e.RecordABRun(test.ID, "chal")
	}

	winner, loser, decided := e.EvaluateABTest(test.ID, reg)
	if !decided {
		t.Fatal("should be decided after enough runs")
	}
	if winner != "chal" {
		t.Errorf("winner = %q, want 'chal' (better fitness)", winner)
	}
	if loser != "inc" {
		t.Errorf("loser = %q, want 'inc'", loser)
	}
}

func TestABTest_NotEnoughRuns(t *testing.T) {
	e := New()
	e.SetObservationRuns(5)

	reg := instruments.NewSkillRegistry()
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{ID: "a", TotalRuns: 10}})
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{ID: "b", TotalRuns: 10}})

	test := e.StartABTest("a", "b", "fp")
	e.RecordABRun(test.ID, "a")
	e.RecordABRun(test.ID, "b")

	_, _, decided := e.EvaluateABTest(test.ID, reg)
	if decided {
		t.Error("should not decide with insufficient runs")
	}
}

func TestABTest_ErrorCases(t *testing.T) {
	e := New()

	if err := e.RecordABRun("nonexistent", "x"); err == nil {
		t.Error("expected error for nonexistent test")
	}

	test := e.StartABTest("a", "b", "fp")
	if err := e.RecordABRun(test.ID, "unknown_skill"); err == nil {
		t.Error("expected error for unknown skill in test")
	}
}

func TestActiveTests(t *testing.T) {
	e := New()
	e.StartABTest("a", "b", "fp1")
	e.StartABTest("c", "d", "fp2")

	active := e.ActiveTests()
	if len(active) != 2 {
		t.Errorf("active = %d, want 2", len(active))
	}
}

func TestEvaluateAll(t *testing.T) {
	e := New()
	e.SetDeprecateThreshold(0.3)
	e.SetObservationRuns(3)

	reg := instruments.NewSkillRegistry()
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{
		ID: "good", Status: instruments.SkillStatusActive,
		TotalRuns: 10, SuccessRate: 0.95, AvgQuality: 0.9,
	}})
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{
		ID: "bad", Status: instruments.SkillStatusActive,
		TotalRuns: 10, SuccessRate: 0.05, AvgQuality: 0.02,
		AvgCostUSD: 0.20, AvgElapsedMs: 50000,
	}})
	reg.Register(&instruments.Skill{Meta: instruments.SkillMeta{
		ID: "already_dep", Status: instruments.SkillStatusDeprecated,
		TotalRuns: 10, SuccessRate: 0.01,
	}})

	deprecated := e.EvaluateAll(reg)
	if len(deprecated) != 1 || deprecated[0] != "bad" {
		t.Errorf("deprecated = %v, want [bad]", deprecated)
	}
}

func TestSetWeights(t *testing.T) {
	e := New()
	custom := FitnessWeights{SuccessRate: 1.0, Quality: 0, CostSaving: 0, Speed: 0}
	e.SetWeights(custom)

	meta := instruments.SkillMeta{TotalRuns: 10, SuccessRate: 1.0}
	fitness := e.ComputeFitness(meta)
	if fitness != 1.0 {
		t.Errorf("with 100%% weight on success rate and 1.0 rate, fitness = %f", fitness)
	}
}
