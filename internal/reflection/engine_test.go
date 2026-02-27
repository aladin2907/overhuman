package reflection

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
)

// mockLLM creates a test server that returns a meso-formatted response.
func mockLLM(t *testing.T, responseText string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		type contentBlock struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		type resp struct {
			ID      string         `json:"id"`
			Type    string         `json:"type"`
			Role    string         `json:"role"`
			Model   string         `json:"model"`
			Content []contentBlock `json:"content"`
			StopReason string     `json:"stop_reason"`
			Usage   struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		r2 := resp{
			ID:    "msg_refl",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-haiku-3-5-20241022",
			Content: []contentBlock{
				{Type: "text", Text: responseText},
			},
			StopReason: "end_turn",
		}
		r2.Usage.InputTokens = 30
		r2.Usage.OutputTokens = 20

		json.NewEncoder(w).Encode(r2)
	}))
}

func setupEngine(t *testing.T, srvURL string) (*Engine, *memory.LongTermMemory) {
	t.Helper()

	dir, err := os.MkdirTemp("", "refl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	ltm, err := memory.NewLongTermMemory(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ltm.Close() })

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srvURL))
	router := brain.NewModelRouter()
	ca := brain.NewContextAssembler()

	return NewEngine(llm, router, ca, ltm), ltm
}

func TestMeso_BasicReflection(t *testing.T) {
	srv := mockLLM(t, `WENT_WELL: fast execution, correct result
IMPROVEMENTS: could cache results, reduce LLM calls
SOUL_SUGGESTION: Add caching strategy
SKILL_SUGGESTION: Create summarization cache skill`)
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	summary := RunSummary{
		TaskID:       "task_123",
		Goal:         "Summarize article",
		Result:       "The article discusses...",
		QualityScore: 0.85,
		ReviewNotes:  "Good quality, could be more concise",
		CostUSD:      0.005,
		ElapsedMs:    1200,
		Fingerprint:  "fp_abc",
	}

	insight, cost, err := engine.Meso(context.Background(), "You are a helpful assistant.", summary)
	if err != nil {
		t.Fatalf("Meso: %v", err)
	}

	if cost <= 0 {
		t.Error("cost should be > 0")
	}
	if insight.TaskID != "task_123" {
		t.Errorf("TaskID = %q", insight.TaskID)
	}
	if len(insight.WentWell) != 2 {
		t.Errorf("WentWell count = %d, want 2, got %v", len(insight.WentWell), insight.WentWell)
	}
	if len(insight.Improvements) != 2 {
		t.Errorf("Improvements count = %d, want 2, got %v", len(insight.Improvements), insight.Improvements)
	}
	if insight.SoulSuggestion == "" {
		t.Error("SoulSuggestion should not be empty")
	}
	if insight.SkillSuggestion == "" {
		t.Error("SkillSuggestion should not be empty")
	}
}

func TestMeso_StoresInLongTermMemory(t *testing.T) {
	srv := mockLLM(t, `WENT_WELL: good
IMPROVEMENTS: none
SOUL_SUGGESTION: NONE
SKILL_SUGGESTION: NONE`)
	defer srv.Close()

	engine, ltm := setupEngine(t, srv.URL)

	summary := RunSummary{
		TaskID:       "task_mem",
		Goal:         "Test memory storage",
		QualityScore: 0.9,
	}

	_, _, err := engine.Meso(context.Background(), "soul", summary)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := ltm.Search("meso", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Error("expected at least 1 long-term entry from reflection")
	}
}

func TestMeso_NoSuggestionWhenNone(t *testing.T) {
	srv := mockLLM(t, `WENT_WELL: everything
IMPROVEMENTS: nothing
SOUL_SUGGESTION: NONE
SKILL_SUGGESTION: NONE`)
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	insight, _, err := engine.Meso(context.Background(), "soul", RunSummary{
		TaskID: "task_none",
		Goal:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	if insight.SoulSuggestion != "" {
		t.Errorf("SoulSuggestion should be empty, got %q", insight.SoulSuggestion)
	}
	if insight.SkillSuggestion != "" {
		t.Errorf("SkillSuggestion should be empty, got %q", insight.SkillSuggestion)
	}
}

func TestMacroThreshold(t *testing.T) {
	srv := mockLLM(t, "WENT_WELL: ok\nIMPROVEMENTS: none\nSOUL_SUGGESTION: NONE\nSKILL_SUGGESTION: NONE")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	engine.SetMacroThreshold(3)

	if engine.ShouldRunMacro() {
		t.Error("should not trigger macro yet")
	}

	for i := 0; i < 3; i++ {
		_, _, err := engine.Meso(context.Background(), "soul", RunSummary{
			TaskID: "task_" + string(rune('a'+i)),
			Goal:   "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if !engine.ShouldRunMacro() {
		t.Error("should trigger macro after 3 runs")
	}

	engine.ResetMacroCounter()
	if engine.ShouldRunMacro() {
		t.Error("should not trigger after reset")
	}
	if engine.RunsSinceMacro() != 0 {
		t.Error("counter should be 0 after reset")
	}
}

func TestParseMesoResponse(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantWell int
		wantImp  int
		wantSoul bool
		wantSkill bool
	}{
		{
			name:    "full response",
			text:    "WENT_WELL: a, b, c\nIMPROVEMENTS: x, y\nSOUL_SUGGESTION: do something\nSKILL_SUGGESTION: build something",
			wantWell: 3,
			wantImp:  2,
			wantSoul: true,
			wantSkill: true,
		},
		{
			name:    "none suggestions",
			text:    "WENT_WELL: good\nIMPROVEMENTS: none\nSOUL_SUGGESTION: NONE\nSKILL_SUGGESTION: none",
			wantWell: 1,
			wantImp:  1,
			wantSoul: false,
			wantSkill: false,
		},
		{
			name:    "empty response",
			text:    "some random text",
			wantWell: 0,
			wantImp:  0,
			wantSoul: false,
			wantSkill: false,
		},
		{
			name:    "partial response",
			text:    "WENT_WELL: speed\nsome other text",
			wantWell: 1,
			wantImp:  0,
			wantSoul: false,
			wantSkill: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insight := parseMesoResponse("task_test", tt.text)

			if len(insight.WentWell) != tt.wantWell {
				t.Errorf("WentWell = %d, want %d", len(insight.WentWell), tt.wantWell)
			}
			if len(insight.Improvements) != tt.wantImp {
				t.Errorf("Improvements = %d, want %d", len(insight.Improvements), tt.wantImp)
			}
			hasSoul := insight.SoulSuggestion != ""
			if hasSoul != tt.wantSoul {
				t.Errorf("SoulSuggestion present = %v, want %v (val=%q)", hasSoul, tt.wantSoul, insight.SoulSuggestion)
			}
			hasSkill := insight.SkillSuggestion != ""
			if hasSkill != tt.wantSkill {
				t.Errorf("SkillSuggestion present = %v, want %v (val=%q)", hasSkill, tt.wantSkill, insight.SkillSuggestion)
			}
		})
	}
}

func TestMeso_LLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"type":"server_error","message":"fail"}}`))
	}))
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)

	_, _, err := engine.Meso(context.Background(), "soul", RunSummary{
		TaskID: "task_err",
		Goal:   "fail",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "meso reflection") {
		t.Errorf("error should mention meso reflection, got: %v", err)
	}
}
