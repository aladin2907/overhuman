package reflection

import (
	"context"
	"strings"
	"testing"
)

func TestMacro_BasicReflection(t *testing.T) {
	srv := mockLLM(t, `STRATEGY_CHANGES: focus on caching, reduce LLM reliance
SOUL_UPDATES: add cost-efficiency principle
NEW_GOALS: build top-5 pattern skills, reduce avg cost below $0.01
SKILLS_TO_GENERATE: article-summarizer, date-formatter
THRESHOLD_CHANGES: raise quality threshold to 0.85`)
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	summary := MacroSummary{
		TotalRuns:      50,
		AvgQuality:     0.82,
		AvgCostUSD:     0.03,
		TopPatterns:    []string{"fp_summarize", "fp_format"},
		RecentInsights: []string{"caching helps", "LLM calls too expensive"},
		SkillCount:     5,
		GoalsPending:   2,
		GoalsCompleted: 8,
	}

	insight, cost, err := engine.Macro(context.Background(), "You are a helpful assistant.", summary)
	if err != nil {
		t.Fatalf("Macro: %v", err)
	}

	if cost <= 0 {
		t.Error("cost should be > 0")
	}
	if len(insight.StrategyChanges) != 2 {
		t.Errorf("StrategyChanges = %d, want 2, got %v", len(insight.StrategyChanges), insight.StrategyChanges)
	}
	if len(insight.SoulUpdates) != 1 {
		t.Errorf("SoulUpdates = %d, want 1", len(insight.SoulUpdates))
	}
	if len(insight.NewGoals) != 2 {
		t.Errorf("NewGoals = %d, want 2", len(insight.NewGoals))
	}
	if len(insight.SkillsToGenerate) != 2 {
		t.Errorf("SkillsToGenerate = %d, want 2", len(insight.SkillsToGenerate))
	}
	if len(insight.ThresholdChanges) != 1 {
		t.Errorf("ThresholdChanges = %d, want 1", len(insight.ThresholdChanges))
	}
}

func TestMacro_ResetsCounter(t *testing.T) {
	srv := mockLLM(t, "STRATEGY_CHANGES: NONE\nSOUL_UPDATES: NONE\nNEW_GOALS: NONE\nSKILS_TO_GENERATE: NONE\nTHRESHOLD_CHANGES: NONE")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	engine.SetMacroThreshold(3)

	// Simulate 3 meso runs.
	for i := 0; i < 3; i++ {
		_, _, _ = engine.Meso(context.Background(), "soul", RunSummary{
			TaskID: "t",
			Goal:   "test",
		})
	}

	if !engine.ShouldRunMacro() {
		t.Fatal("should trigger macro after 3 runs")
	}

	// Run macro — it should reset the counter.
	_, _, err := engine.Macro(context.Background(), "soul", MacroSummary{TotalRuns: 3})
	if err != nil {
		t.Fatal(err)
	}

	if engine.ShouldRunMacro() {
		t.Error("counter should be reset after macro")
	}
	if engine.RunsSinceMacro() != 0 {
		t.Errorf("RunsSinceMacro = %d, want 0", engine.RunsSinceMacro())
	}
}

func TestMacro_StoresInLongTermMemory(t *testing.T) {
	srv := mockLLM(t, "STRATEGY_CHANGES: improve caching\nSOUL_UPDATES: NONE\nNEW_GOALS: build cache skill\nSKILLS_TO_GENERATE: NONE\nTHRESHOLD_CHANGES: NONE")
	defer srv.Close()

	engine, ltm := setupEngine(t, srv.URL)

	_, _, err := engine.Macro(context.Background(), "soul", MacroSummary{TotalRuns: 10})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := ltm.Search("macro", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Error("expected at least 1 long-term entry from macro-reflection")
	}
}

func TestMacro_LLMError(t *testing.T) {
	srv := mockLLM(t, "") // We'll override with a failing server.
	srv.Close()

	// Use a closed server URL to force connection error.
	engine, _ := setupEngine(t, "http://127.0.0.1:1")

	_, _, err := engine.Macro(context.Background(), "soul", MacroSummary{TotalRuns: 10})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "macro reflection") {
		t.Errorf("error should mention macro reflection, got: %v", err)
	}
}

func TestParseMacroResponse(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		wantStrategy   int
		wantSoul       int
		wantGoals      int
		wantSkills     int
		wantThresholds int
	}{
		{
			name:           "full response",
			text:           "STRATEGY_CHANGES: a, b\nSOUL_UPDATES: c\nNEW_GOALS: d, e, f\nSKILLS_TO_GENERATE: g\nTHRESHOLD_CHANGES: h, i",
			wantStrategy:   2,
			wantSoul:       1,
			wantGoals:      3,
			wantSkills:     1,
			wantThresholds: 2,
		},
		{
			name:           "all NONE",
			text:           "STRATEGY_CHANGES: NONE\nSOUL_UPDATES: NONE\nNEW_GOALS: NONE\nSKILLS_TO_GENERATE: NONE\nTHRESHOLD_CHANGES: NONE",
			wantStrategy:   0,
			wantSoul:       0,
			wantGoals:      0,
			wantSkills:     0,
			wantThresholds: 0,
		},
		{
			name:           "empty response",
			text:           "some random text",
			wantStrategy:   0,
			wantSoul:       0,
			wantGoals:      0,
			wantSkills:     0,
			wantThresholds: 0,
		},
		{
			name:           "partial response",
			text:           "STRATEGY_CHANGES: focus\nNEW_GOALS: learn",
			wantStrategy:   1,
			wantSoul:       0,
			wantGoals:      1,
			wantSkills:     0,
			wantThresholds: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insight := parseMacroResponse(tt.text)

			if len(insight.StrategyChanges) != tt.wantStrategy {
				t.Errorf("StrategyChanges = %d, want %d", len(insight.StrategyChanges), tt.wantStrategy)
			}
			if len(insight.SoulUpdates) != tt.wantSoul {
				t.Errorf("SoulUpdates = %d, want %d", len(insight.SoulUpdates), tt.wantSoul)
			}
			if len(insight.NewGoals) != tt.wantGoals {
				t.Errorf("NewGoals = %d, want %d", len(insight.NewGoals), tt.wantGoals)
			}
			if len(insight.SkillsToGenerate) != tt.wantSkills {
				t.Errorf("SkillsToGenerate = %d, want %d", len(insight.SkillsToGenerate), tt.wantSkills)
			}
			if len(insight.ThresholdChanges) != tt.wantThresholds {
				t.Errorf("ThresholdChanges = %d, want %d", len(insight.ThresholdChanges), tt.wantThresholds)
			}
		})
	}
}
