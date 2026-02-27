package reflection

import (
	"context"
	"strings"
	"testing"
)

func TestMega_BasicReflection(t *testing.T) {
	srv := mockLLM(t, `EFFECTIVENESS: Meso insights are useful but macro needs improvement
MESO_ADJUSTMENTS: focus more on cost analysis, reduce verbosity
MACRO_ADJUSTMENTS: increase frequency, add skill-specific analysis
THRESHOLD_ADJUSTMENTS: lower macro trigger from 10 to 7 runs
PROCESS_CHANGES: add automated tracking of insight-to-action ratio`)
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	summary := MegaSummary{
		TotalMesoRuns:       50,
		TotalMacroRuns:      5,
		MesoInsightsActedOn: 15,
		MacroInsightsActedOn: 2,
		RecentMesoInsights:  []string{"caching helpful", "LLM costs high"},
		RecentMacroInsights: []string{"strategy shift to code-first"},
		QualityTrend:        "improving",
		CostTrend:           "decreasing",
	}

	insight, cost, err := engine.Mega(context.Background(), "You are Overhuman.", summary)
	if err != nil {
		t.Fatalf("Mega: %v", err)
	}

	if cost <= 0 {
		t.Error("cost should be > 0")
	}
	if insight.ReflectionEffectiveness == "" {
		t.Error("effectiveness should not be empty")
	}
	if len(insight.MesoAdjustments) != 2 {
		t.Errorf("MesoAdjustments = %d, want 2", len(insight.MesoAdjustments))
	}
	if len(insight.MacroAdjustments) != 2 {
		t.Errorf("MacroAdjustments = %d, want 2", len(insight.MacroAdjustments))
	}
	if len(insight.ThresholdAdjustments) != 1 {
		t.Errorf("ThresholdAdjustments = %d, want 1", len(insight.ThresholdAdjustments))
	}
	if len(insight.ProcessChanges) != 1 {
		t.Errorf("ProcessChanges = %d, want 1", len(insight.ProcessChanges))
	}
}

func TestMega_AllNone(t *testing.T) {
	srv := mockLLM(t, `EFFECTIVENESS: Everything works well
MESO_ADJUSTMENTS: NONE
MACRO_ADJUSTMENTS: NONE
THRESHOLD_ADJUSTMENTS: NONE
PROCESS_CHANGES: NONE`)
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	insight, _, err := engine.Mega(context.Background(), "soul", MegaSummary{
		TotalMesoRuns: 100,
		QualityTrend:  "stable",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(insight.MesoAdjustments) != 0 {
		t.Errorf("MesoAdjustments should be empty, got %v", insight.MesoAdjustments)
	}
	if len(insight.ProcessChanges) != 0 {
		t.Errorf("ProcessChanges should be empty, got %v", insight.ProcessChanges)
	}
}

func TestMega_StoresInLTM(t *testing.T) {
	srv := mockLLM(t, "EFFECTIVENESS: good\nMESO_ADJUSTMENTS: NONE\nMACRO_ADJUSTMENTS: NONE\nTHRESHOLD_ADJUSTMENTS: NONE\nPROCESS_CHANGES: NONE")
	defer srv.Close()

	engine, ltm := setupEngine(t, srv.URL)

	_, _, err := engine.Mega(context.Background(), "soul", MegaSummary{TotalMesoRuns: 10})
	if err != nil {
		t.Fatal(err)
	}

	entries, err := ltm.Search("mega", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Error("expected at least 1 long-term entry from mega-reflection")
	}
}

func TestMega_LLMError(t *testing.T) {
	engine, _ := setupEngine(t, "http://127.0.0.1:1")

	_, _, err := engine.Mega(context.Background(), "soul", MegaSummary{TotalMesoRuns: 10})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mega reflection") {
		t.Errorf("error should mention mega reflection, got: %v", err)
	}
}

func TestParseMegaResponse(t *testing.T) {
	tests := []struct {
		name            string
		text            string
		wantEffective   bool
		wantMeso        int
		wantMacro       int
		wantThreshold   int
		wantProcess     int
	}{
		{
			name:          "full",
			text:          "EFFECTIVENESS: good\nMESO_ADJUSTMENTS: a, b\nMACRO_ADJUSTMENTS: c\nTHRESHOLD_ADJUSTMENTS: d, e\nPROCESS_CHANGES: f",
			wantEffective: true,
			wantMeso:      2,
			wantMacro:     1,
			wantThreshold: 2,
			wantProcess:   1,
		},
		{
			name:          "empty",
			text:          "random text",
			wantEffective: false,
		},
		{
			name:          "partial",
			text:          "EFFECTIVENESS: decent\nPROCESS_CHANGES: improve logging",
			wantEffective: true,
			wantProcess:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insight := parseMegaResponse(tt.text)
			hasEff := insight.ReflectionEffectiveness != ""
			if hasEff != tt.wantEffective {
				t.Errorf("effectiveness present = %v, want %v", hasEff, tt.wantEffective)
			}
			if len(insight.MesoAdjustments) != tt.wantMeso {
				t.Errorf("MesoAdjustments = %d, want %d", len(insight.MesoAdjustments), tt.wantMeso)
			}
			if len(insight.MacroAdjustments) != tt.wantMacro {
				t.Errorf("MacroAdjustments = %d, want %d", len(insight.MacroAdjustments), tt.wantMacro)
			}
			if len(insight.ThresholdAdjustments) != tt.wantThreshold {
				t.Errorf("ThresholdAdjustments = %d, want %d", len(insight.ThresholdAdjustments), tt.wantThreshold)
			}
			if len(insight.ProcessChanges) != tt.wantProcess {
				t.Errorf("ProcessChanges = %d, want %d", len(insight.ProcessChanges), tt.wantProcess)
			}
		})
	}
}
