package genui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// TestIntegration_EndToEnd_ANSI tests the full flow:
// mock LLM -> Generate -> CLIRenderer.Render -> verify ANSI output.
func TestIntegration_EndToEnd_ANSI(t *testing.T) {
	// 1. Create mock LLM that returns valid ANSI with box-drawing.
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      "\033[1mResult\033[0m\n\033[36m\u250c\u2500\u2500\u2500\u2500\u2500\u2510\033[0m\n\033[36m\u2502 OK  \u2502\033[0m\n\033[36m\u2514\u2500\u2500\u2500\u2500\u2500\u2518\033[0m",
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	// 2. Generate UI.
	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t1",
		Success:      true,
		Result:       "All good",
		QualityScore: 0.95,
	}
	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if ui.Format != FormatANSI {
		t.Errorf("Format = %q, want ansi", ui.Format)
	}
	if ui.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", ui.TaskID)
	}

	// 3. Render through CLIRenderer.
	var buf bytes.Buffer
	renderer := NewCLIRenderer(&buf, nil)
	err = renderer.Render(ui)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Result") {
		t.Error("rendered output should contain 'Result'")
	}
	if !strings.Contains(output, "\u250c") { // box-drawing ┌
		t.Error("rendered output should contain box-drawing characters")
	}
	if !strings.Contains(output, "OK") {
		t.Error("rendered output should contain 'OK'")
	}
	// Verify ANSI escape codes survived sanitization (SGR is allowed).
	if !strings.Contains(output, "\033[1m") {
		t.Error("rendered output should contain bold ANSI escape")
	}
	if !strings.Contains(output, "\033[0m") {
		t.Error("rendered output should contain reset ANSI escape")
	}
}

// TestIntegration_EndToEnd_HTML tests the full flow:
// mock LLM -> Generate with WebCapabilities -> verify HTML output.
func TestIntegration_EndToEnd_HTML(t *testing.T) {
	htmlResponse := `<!DOCTYPE html>
<html>
<head><title>Dashboard</title>
<style>
body { background: #1a1a2e; color: #e0e0e0; font-family: sans-serif; }
.card { border: 1px solid #333; padding: 16px; border-radius: 8px; }
</style>
</head>
<body>
<div class="card">
<h1>Analysis Complete</h1>
<p>Quality: 92%</p>
</div>
</body>
</html>`

	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      htmlResponse,
			Model:        "mock-model",
			InputTokens:  120,
			OutputTokens: 200,
		}, nil
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_html_1",
		Success:      true,
		Result:       "Analysis complete with high confidence",
		QualityScore: 0.92,
	}

	ui, err := gen.Generate(context.Background(), result, WebCapabilities(1280, 800))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if ui.Format != FormatHTML {
		t.Errorf("Format = %q, want html", ui.Format)
	}
	if ui.TaskID != "t_html_1" {
		t.Errorf("TaskID = %q, want t_html_1", ui.TaskID)
	}
	if !strings.Contains(ui.Code, "<!DOCTYPE html>") {
		t.Error("HTML output should contain DOCTYPE")
	}
	if !strings.Contains(ui.Code, "<html>") {
		t.Error("HTML output should contain <html> tag")
	}
	if !strings.Contains(ui.Code, "</html>") {
		t.Error("HTML output should contain closing </html>")
	}
	if !strings.Contains(ui.Code, "<style>") {
		t.Error("HTML output should contain inline CSS")
	}
	if !strings.Contains(ui.Code, "Analysis Complete") {
		t.Error("HTML output should contain result text")
	}
	// Verify SanitizeHTML did not flag anything (no network calls).
	sanitized := SanitizeHTML(ui.Code)
	if strings.Contains(sanitized, "BLOCKED") {
		t.Error("HTML output should not trigger sanitizer blocks")
	}
}

// TestIntegration_Fallback tests: LLM returns error -> Generate fails -> RenderPlainText fallback.
func TestIntegration_Fallback(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return nil, errors.New("service unavailable")
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_fallback",
		Success:      true,
		Result:       "Important result data",
		QualityScore: 0.85,
	}

	// Generate should fail.
	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err == nil {
		t.Fatal("expected error from Generate when LLM fails")
	}
	if ui != nil {
		t.Error("UI should be nil on error")
	}
	if !strings.Contains(err.Error(), "service unavailable") {
		t.Errorf("error should propagate LLM error, got: %v", err)
	}

	// Fallback: use RenderPlainText to show raw result.
	var buf bytes.Buffer
	renderer := NewCLIRenderer(&buf, nil)
	err = renderer.RenderPlainText(result.Result)
	if err != nil {
		t.Fatalf("RenderPlainText failed: %v", err)
	}

	output := buf.String()
	if output != "Important result data" {
		t.Errorf("fallback output = %q, want %q", output, "Important result data")
	}
}

// TestIntegration_ActionCallback tests: generate UI with actions -> WaitForAction -> correct action.
func TestIntegration_ActionCallback(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genAnsiWithActions,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 70,
		}, nil
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_actions",
		Success:      true,
		Result:       "Deploy completed successfully.",
		QualityScore: 0.92,
	}

	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Attach actions to the UI (generator produces code, but actions are set externally).
	ui.Actions = []GeneratedAction{
		{ID: "a1", Label: "View Logs", Callback: "view_logs"},
		{ID: "a2", Label: "Rollback", Callback: "rollback"},
		{ID: "a3", Label: "Open Dashboard", Callback: "open_dash"},
	}

	// Render with actions.
	var outBuf bytes.Buffer
	inReader := strings.NewReader("2\n") // simulate user selecting action 2
	renderer := NewCLIRenderer(&outBuf, inReader)
	err = renderer.Render(ui)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Verify actions are rendered.
	rendered := outBuf.String()
	if !strings.Contains(rendered, "[1]") {
		t.Error("rendered output should contain [1] for first action")
	}
	if !strings.Contains(rendered, "View Logs") {
		t.Error("rendered output should show 'View Logs' label")
	}
	if !strings.Contains(rendered, "[2]") {
		t.Error("rendered output should contain [2] for second action")
	}
	if !strings.Contains(rendered, "Rollback") {
		t.Error("rendered output should show 'Rollback' label")
	}

	// WaitForAction reads user input and matches to action 2.
	action := renderer.WaitForAction(ui)
	if action == nil {
		t.Fatal("WaitForAction returned nil, expected action 2")
	}
	if action.ID != "a2" {
		t.Errorf("action.ID = %q, want a2", action.ID)
	}
	if action.Label != "Rollback" {
		t.Errorf("action.Label = %q, want Rollback", action.Label)
	}
	if action.Callback != "rollback" {
		t.Errorf("action.Callback = %q, want rollback", action.Callback)
	}
}

// TestIntegration_SelfHeal_EndToEnd tests self-healing:
// LLM returns invalid ANSI first (no reset), valid ANSI second -> Generate succeeds -> Render works.
func TestIntegration_SelfHeal_EndToEnd(t *testing.T) {
	var callNum int32
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		n := atomic.AddInt32(&callNum, 1)
		if n == 1 {
			// Invalid: ANSI escape opens but no reset \033[0m.
			return &brain.LLMResponse{
				Content:      "\033[36mBroken UI with no reset\033[31mStill broken",
				Model:        "mock-model",
				InputTokens:  50,
				OutputTokens: 20,
			}, nil
		}
		// Second call: valid ANSI.
		return &brain.LLMResponse{
			Content:      "\033[1m\033[36mHealed Result\033[0m\nAll fixed.\n\033[90m---\033[0m",
			Model:        "mock-model",
			InputTokens:  60,
			OutputTokens: 30,
		}, nil
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_heal",
		Success:      true,
		Result:       "Self-healing test",
		QualityScore: 0.88,
	}

	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate should succeed after self-heal: %v", err)
	}

	// Verify retry happened (at least 2 LLM calls).
	if mock.requestCount() < 2 {
		t.Errorf("expected at least 2 LLM calls for self-heal, got %d", mock.requestCount())
	}

	// Verify the error message was fed back in the retry prompt.
	lastReq := mock.lastRequest()
	found := false
	for _, msg := range lastReq.Messages {
		if strings.Contains(msg.Content, "error") || strings.Contains(msg.Content, "Fix it") {
			found = true
			break
		}
	}
	if !found {
		t.Error("retry prompt should contain the validation error feedback")
	}

	// Render the healed output.
	var buf bytes.Buffer
	renderer := NewCLIRenderer(&buf, nil)
	err = renderer.Render(ui)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Healed Result") {
		t.Error("rendered output should contain 'Healed Result' from healed response")
	}
	if !strings.Contains(output, "All fixed") {
		t.Error("rendered output should contain 'All fixed'")
	}
}

// TestIntegration_ThoughtLogVisible tests:
// GenerateWithThought with stages -> Render -> output contains stage info.
func TestIntegration_ThoughtLogVisible(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// Verify thought stages appear in the prompt.
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "Stage 1 (intake)") {
			t.Error("prompt should mention stage 1 (intake)")
		}
		if !strings.Contains(userMsg, "Stage 2 (execute)") {
			t.Error("prompt should mention stage 2 (execute)")
		}
		if !strings.Contains(userMsg, "collapsible 'Thought Log'") {
			t.Error("prompt should instruct collapsible thought log")
		}
		return &brain.LLMResponse{
			Content:      "\033[1m\033[36mTask Output\033[0m\nProcessed successfully.\n\033[0m",
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 60,
		}, nil
	})

	thought := BuildThoughtLog([]ThoughtStage{
		{Number: 1, Name: "intake", Summary: "Parsed user input", DurMs: 12},
		{Number: 2, Name: "execute", Summary: "Ran LLM call", DurMs: 230},
		{Number: 3, Name: "review", Summary: "Quality checked", DurMs: 45},
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_thought",
		Success:      true,
		Result:       "Processed successfully",
		QualityScore: 0.91,
	}

	ui, err := gen.GenerateWithThought(context.Background(), result, CLICapabilities(), thought, nil)
	if err != nil {
		t.Fatalf("GenerateWithThought failed: %v", err)
	}

	// Verify ThoughtLog is attached.
	if ui.Thought == nil {
		t.Fatal("ui.Thought should not be nil")
	}
	if len(ui.Thought.Stages) != 3 {
		t.Errorf("Thought.Stages len = %d, want 3", len(ui.Thought.Stages))
	}
	if ui.Thought.TotalMs != 287 { // 12 + 230 + 45
		t.Errorf("Thought.TotalMs = %d, want 287", ui.Thought.TotalMs)
	}

	// Format the thought log as ANSI.
	thoughtANSI := FormatThoughtLogANSI(ui.Thought)
	if thoughtANSI == "" {
		t.Fatal("FormatThoughtLogANSI returned empty string")
	}
	if !strings.Contains(thoughtANSI, "287ms") {
		t.Error("thought log ANSI should contain total duration '287ms'")
	}
	if !strings.Contains(thoughtANSI, "intake") {
		t.Error("thought log ANSI should contain stage name 'intake'")
	}
	if !strings.Contains(thoughtANSI, "execute") {
		t.Error("thought log ANSI should contain stage name 'execute'")
	}
	if !strings.Contains(thoughtANSI, "review") {
		t.Error("thought log ANSI should contain stage name 'review'")
	}
	// Verify tree-drawing characters.
	if !strings.Contains(thoughtANSI, "\u251c\u2500") { // ├─
		t.Error("thought log should contain tree branch character ├─")
	}
	if !strings.Contains(thoughtANSI, "\u2514\u2500") { // └─
		t.Error("thought log should contain tree leaf character └─")
	}

	// Render the UI itself.
	var buf bytes.Buffer
	renderer := NewCLIRenderer(&buf, nil)
	err = renderer.Render(ui)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Task Output") {
		t.Error("rendered output should contain 'Task Output'")
	}
}

// TestIntegration_ProgressiveDisclosure tests:
// GenerateWithThought with Summary -> Render -> output shows [d] Details hint.
func TestIntegration_ProgressiveDisclosure(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      "\033[1m\033[36mSummary View\033[0m\nTask done in 150ms.\n\033[0m",
			Model:        "mock-model",
			InputTokens:  80,
			OutputTokens: 40,
		}, nil
	})

	thought := BuildThoughtLog([]ThoughtStage{
		{Number: 1, Name: "parse", Summary: "Parsed input", DurMs: 30},
		{Number: 2, Name: "plan", Summary: "Created plan", DurMs: 50},
		{Number: 3, Name: "execute", Summary: "Executed task", DurMs: 70},
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	result := pipeline.RunResult{
		TaskID:       "t_progressive",
		Success:      true,
		Result:       "Compact result",
		QualityScore: 0.90,
	}

	ui, err := gen.GenerateWithThought(context.Background(), result, CLICapabilities(), thought, nil)
	if err != nil {
		t.Fatalf("GenerateWithThought failed: %v", err)
	}

	// GenerateWithThought sets Meta.Summary when thought is provided.
	if ui.Meta.Summary == "" {
		t.Fatal("Meta.Summary should be set from thought log")
	}
	if !strings.Contains(ui.Meta.Summary, "150ms") {
		t.Errorf("Meta.Summary = %q, should contain '150ms'", ui.Meta.Summary)
	}

	// Render: when Meta.Summary is non-empty, CLIRenderer adds [d] Details hint.
	var buf bytes.Buffer
	renderer := NewCLIRenderer(&buf, nil)
	err = renderer.Render(ui)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[d] Details") {
		t.Error("progressive disclosure output should contain '[d] Details' hint")
	}
	if !strings.Contains(output, "[t] Thought log") {
		t.Error("progressive disclosure output should contain '[t] Thought log' hint")
	}
	// The hint line should be dim (\033[90m).
	if !strings.Contains(output, "\033[90m") {
		t.Error("progressive disclosure hint should use dim ANSI color")
	}
}

// TestIntegration_ReflectionLoop tests the full reflection loop:
// Generate -> Render -> Record interaction -> BuildHints -> hints include data.
func TestIntegration_ReflectionLoop(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// On second call, verify hints are included.
		userMsg := req.Messages[1].Content
		if strings.Contains(userMsg, "UI HINT:") {
			if !strings.Contains(userMsg, "User frequently uses action: deploy") {
				t.Error("second generate should include hint about deploy action")
			}
			if !strings.Contains(userMsg, "User had to scroll") {
				t.Error("second generate should include hint about scrolling")
			}
		}
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	gen := NewUIGenerator(mock, brain.NewModelRouter())
	store := NewReflectionStore()

	// --- First iteration: generate, render, record. ---
	result1 := pipeline.RunResult{
		TaskID:       "t_refl_1",
		Success:      true,
		Result:       "First iteration result",
		QualityScore: 0.88,
	}

	ui1, err := gen.Generate(context.Background(), result1, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate (iter 1) failed: %v", err)
	}

	var buf1 bytes.Buffer
	renderer1 := NewCLIRenderer(&buf1, nil)
	err = renderer1.Render(ui1)
	if err != nil {
		t.Fatalf("Render (iter 1) failed: %v", err)
	}

	// Record user interaction: used "deploy" action, scrolled.
	store.Record(UIReflection{
		TaskID:       "t_refl_1",
		UIFormat:     FormatANSI,
		ActionsShown: []string{"deploy", "rollback", "logs"},
		ActionsUsed:  []string{"deploy"},
		Scrolled:     true,
		Dismissed:    false,
	})

	// --- Build hints from reflection store. ---
	hints := store.BuildHints("fp_test")
	if len(hints) == 0 {
		t.Fatal("BuildHints should return at least one hint")
	}

	// Verify hints content.
	foundActionHint := false
	foundScrollHint := false
	for _, h := range hints {
		if strings.Contains(h, "deploy") {
			foundActionHint = true
		}
		if strings.Contains(h, "scroll") {
			foundScrollHint = true
		}
	}
	if !foundActionHint {
		t.Error("hints should mention frequently used action 'deploy'")
	}
	if !foundScrollHint {
		t.Error("hints should mention scrolling behavior")
	}

	// --- Second iteration: generate with hints from reflection. ---
	result2 := pipeline.RunResult{
		TaskID:       "t_refl_2",
		Success:      true,
		Result:       "Second iteration with hints",
		QualityScore: 0.92,
	}

	ui2, err := gen.GenerateWithThought(context.Background(), result2, CLICapabilities(), nil, hints)
	if err != nil {
		t.Fatalf("GenerateWithThought (iter 2) failed: %v", err)
	}

	var buf2 bytes.Buffer
	renderer2 := NewCLIRenderer(&buf2, nil)
	err = renderer2.Render(ui2)
	if err != nil {
		t.Fatalf("Render (iter 2) failed: %v", err)
	}

	// Record a second interaction: dismissed without action.
	store.Record(UIReflection{
		TaskID:    "t_refl_2",
		UIFormat:  FormatANSI,
		Dismissed: true,
	})

	// Verify the store accumulated both records.
	allRecords := store.Records()
	if len(allRecords) != 2 {
		t.Errorf("store should have 2 records, got %d", len(allRecords))
	}

	// Build hints again -- now should also include dismiss hint.
	hints2 := store.BuildHints("fp_test")
	foundDismissHint := false
	for _, h := range hints2 {
		if strings.Contains(h, "dismissed") {
			foundDismissHint = true
		}
	}
	if !foundDismissHint {
		t.Error("hints after second iteration should mention dismissed UI")
	}
}
