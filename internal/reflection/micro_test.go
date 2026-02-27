package reflection

import (
	"context"
	"strings"
	"testing"
)

func TestMicro_CheckPassing(t *testing.T) {
	srv := mockLLM(t, "OK: YES\nCONFIDENCE: 0.9\nISSUE: NONE\nSUGGESTION: NONE")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	micro := NewMicroReflector(engine.llm, engine.router, engine.ctx)

	verdict, cost, err := micro.Check(context.Background(), "Summarize article", StepResult{
		Step:   StepExecute,
		Output: "The article discusses recent advances in AI...",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !verdict.OK {
		t.Error("expected OK=true")
	}
	if verdict.Confidence < 0.5 {
		t.Errorf("Confidence = %f, want >= 0.5", verdict.Confidence)
	}
	if verdict.Issue != "" {
		t.Errorf("Issue should be empty, got %q", verdict.Issue)
	}
	if cost <= 0 {
		t.Error("cost should be > 0")
	}
}

func TestMicro_CheckFailing(t *testing.T) {
	srv := mockLLM(t, "OK: NO\nCONFIDENCE: 0.8\nISSUE: Output is empty\nSUGGESTION: Retry with more context")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	micro := NewMicroReflector(engine.llm, engine.router, engine.ctx)

	verdict, _, err := micro.Check(context.Background(), "Test task", StepResult{
		Step:   StepClarify,
		Output: "",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if verdict.OK {
		t.Error("expected OK=false")
	}
	if verdict.Issue == "" {
		t.Error("Issue should not be empty")
	}
	if verdict.Suggestion == "" {
		t.Error("Suggestion should not be empty")
	}
}

func TestMicro_DisabledStep(t *testing.T) {
	srv := mockLLM(t, "should not be called")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	micro := NewMicroReflector(engine.llm, engine.router, engine.ctx)
	micro.SetEnabled(StepPlan, false) // Plan is not enabled by default, but set explicitly

	verdict, cost, err := micro.Check(context.Background(), "test", StepResult{
		Step:   StepPlan,
		Output: "some plan",
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !verdict.OK {
		t.Error("disabled step should return OK")
	}
	if cost != 0 {
		t.Error("disabled step should cost nothing")
	}
}

func TestMicro_EnableDisable(t *testing.T) {
	srv := mockLLM(t, "")
	defer srv.Close()

	engine, _ := setupEngine(t, srv.URL)
	micro := NewMicroReflector(engine.llm, engine.router, engine.ctx)

	if !micro.IsEnabled(StepExecute) {
		t.Error("execute should be enabled by default")
	}

	micro.SetEnabled(StepExecute, false)
	if micro.IsEnabled(StepExecute) {
		t.Error("execute should be disabled after SetEnabled(false)")
	}

	micro.SetEnabled(StepExecute, true)
	if !micro.IsEnabled(StepExecute) {
		t.Error("execute should be re-enabled")
	}
}

func TestMicro_LLMError(t *testing.T) {
	engine, _ := setupEngine(t, "http://127.0.0.1:1")
	micro := NewMicroReflector(engine.llm, engine.router, engine.ctx)

	_, _, err := micro.Check(context.Background(), "test", StepResult{
		Step:   StepExecute,
		Output: "something",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "micro reflection") {
		t.Errorf("error should mention micro reflection, got: %v", err)
	}
}

func TestParseMicroResponse(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantOK     bool
		wantIssue  bool
		wantSugg   bool
	}{
		{
			name:   "passing",
			text:   "OK: YES\nCONFIDENCE: 0.95\nISSUE: NONE\nSUGGESTION: NONE",
			wantOK: true,
		},
		{
			name:      "failing",
			text:      "OK: NO\nCONFIDENCE: 0.7\nISSUE: bad output\nSUGGESTION: retry",
			wantOK:    false,
			wantIssue: true,
			wantSugg:  true,
		},
		{
			name:   "empty",
			text:   "random text",
			wantOK: true, // default is OK=true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := parseMicroResponse(StepExecute, tt.text)
			if v.OK != tt.wantOK {
				t.Errorf("OK = %v, want %v", v.OK, tt.wantOK)
			}
			if (v.Issue != "") != tt.wantIssue {
				t.Errorf("Issue present = %v, want %v", v.Issue != "", tt.wantIssue)
			}
			if (v.Suggestion != "") != tt.wantSugg {
				t.Errorf("Suggestion present = %v, want %v", v.Suggestion != "", tt.wantSugg)
			}
		})
	}
}
