package genui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// --- BuildThoughtLog Tests ---

func TestBuildThoughtLog_WithStages(t *testing.T) {
	stages := []ThoughtStage{
		{Number: 1, Name: "Parse", Summary: "Parsed input", DurMs: 50},
		{Number: 2, Name: "Think", Summary: "Generated response", DurMs: 120},
		{Number: 3, Name: "Format", Summary: "Formatted output", DurMs: 30},
	}

	log := BuildThoughtLog(stages)
	if log == nil {
		t.Fatal("expected non-nil ThoughtLog")
	}
	if len(log.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(log.Stages))
	}
	expectedTotal := int64(50 + 120 + 30)
	if log.TotalMs != expectedTotal {
		t.Fatalf("expected TotalMs=%d, got %d", expectedTotal, log.TotalMs)
	}
}

func TestBuildThoughtLog_Empty(t *testing.T) {
	log := BuildThoughtLog(nil)
	if log == nil {
		t.Fatal("expected non-nil ThoughtLog even for nil input")
	}
	if len(log.Stages) != 0 {
		t.Fatalf("expected 0 stages, got %d", len(log.Stages))
	}
	if log.TotalMs != 0 {
		t.Fatalf("expected TotalMs=0, got %d", log.TotalMs)
	}

	// Also test with empty slice.
	log2 := BuildThoughtLog([]ThoughtStage{})
	if log2 == nil {
		t.Fatal("expected non-nil ThoughtLog for empty slice")
	}
	if log2.TotalMs != 0 {
		t.Fatalf("expected TotalMs=0, got %d", log2.TotalMs)
	}
}

func TestBuildThoughtLog_SingleStage(t *testing.T) {
	stages := []ThoughtStage{
		{Number: 1, Name: "Analyze", Summary: "Did stuff", DurMs: 77},
	}

	log := BuildThoughtLog(stages)
	if log == nil {
		t.Fatal("expected non-nil ThoughtLog")
	}
	if len(log.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(log.Stages))
	}
	if log.TotalMs != 77 {
		t.Fatalf("expected TotalMs=77, got %d", log.TotalMs)
	}
	if log.Stages[0].Name != "Analyze" {
		t.Fatalf("expected stage name 'Analyze', got %q", log.Stages[0].Name)
	}
}

// --- FormatThoughtLogANSI Tests ---

func TestFormatThoughtLogANSI_WithStages(t *testing.T) {
	log := &ThoughtLog{
		Stages: []ThoughtStage{
			{Number: 1, Name: "Parse", Summary: "Parsed input", DurMs: 50},
			{Number: 2, Name: "Think", Summary: "Generated answer", DurMs: 200},
		},
		TotalMs: 250,
	}

	out := FormatThoughtLogANSI(log)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	// Should contain stage names.
	if !strings.Contains(out, "Parse") {
		t.Fatalf("output should contain stage name 'Parse': %q", out)
	}
	if !strings.Contains(out, "Think") {
		t.Fatalf("output should contain stage name 'Think': %q", out)
	}
	// Should contain durations.
	if !strings.Contains(out, "50ms") {
		t.Fatalf("output should contain '50ms': %q", out)
	}
	if !strings.Contains(out, "200ms") {
		t.Fatalf("output should contain '200ms': %q", out)
	}
	// Should contain total.
	if !strings.Contains(out, "250ms") {
		t.Fatalf("output should contain total '250ms': %q", out)
	}
}

func TestFormatThoughtLogANSI_Empty(t *testing.T) {
	// nil log
	out := FormatThoughtLogANSI(nil)
	if out != "" {
		t.Fatalf("expected empty string for nil log, got %q", out)
	}
	// empty stages
	out = FormatThoughtLogANSI(&ThoughtLog{})
	if out != "" {
		t.Fatalf("expected empty string for empty stages, got %q", out)
	}
}

func TestFormatThoughtLogANSI_TreeFormat(t *testing.T) {
	log := &ThoughtLog{
		Stages: []ThoughtStage{
			{Number: 1, Name: "First", Summary: "A", DurMs: 10},
			{Number: 2, Name: "Middle", Summary: "B", DurMs: 20},
			{Number: 3, Name: "Last", Summary: "C", DurMs: 30},
		},
		TotalMs: 60,
	}

	out := FormatThoughtLogANSI(log)

	// Non-last stages use tree branch character.
	if !strings.Contains(out, "\u251C\u2500") { // ├─
		t.Fatalf("expected tree branch character in output: %q", out)
	}
	// Last stage uses tree end character.
	if !strings.Contains(out, "\u2514\u2500") { // └─
		t.Fatalf("expected tree end character in output: %q", out)
	}

	// Verify counts: 2 branch lines (First, Middle) and 1 end line (Last).
	lines := strings.Split(out, "\n")
	var branchCount, endCount int
	for _, line := range lines {
		if strings.Contains(line, "\u251C\u2500") {
			branchCount++
		}
		if strings.Contains(line, "\u2514\u2500") {
			endCount++
		}
	}
	if branchCount != 2 {
		t.Fatalf("expected 2 branch lines, got %d", branchCount)
	}
	if endCount != 1 {
		t.Fatalf("expected 1 end line, got %d", endCount)
	}
}

// --- Progressive Disclosure / GenerateWithThought Tests ---

func TestThoughtLog_IncludedInPrompt(t *testing.T) {
	var capturedContent string
	mock := newMockLLM(func(_ context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// Concatenate all message contents for inspection.
		for _, msg := range req.Messages {
			capturedContent += msg.Content + "\n"
		}
		return &brain.LLMResponse{Content: "\033[36mResult\033[0m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	thought := &ThoughtLog{
		Stages: []ThoughtStage{
			{Number: 1, Name: "Classify", Summary: "Classified as question", DurMs: 15},
			{Number: 2, Name: "Retrieve", Summary: "Found 3 docs", DurMs: 80},
			{Number: 3, Name: "Synthesize", Summary: "Built answer", DurMs: 150},
		},
		TotalMs: 245,
	}

	result := pipeline.RunResult{
		TaskID:       "thought-test-1",
		Success:      true,
		Result:       "Answer to your question",
		QualityScore: 0.88,
	}

	_, err := gen.GenerateWithThought(context.Background(), result, CLICapabilities(), thought, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.requestCount() == 0 {
		t.Fatal("LLM was not called")
	}

	// Check that the prompt contains each stage's info.
	for _, stage := range thought.Stages {
		if !strings.Contains(capturedContent, stage.Name) {
			t.Fatalf("prompt should contain stage name %q", stage.Name)
		}
		if !strings.Contains(capturedContent, stage.Summary) {
			t.Fatalf("prompt should contain stage summary %q", stage.Summary)
		}
		durStr := fmt.Sprintf("%dms", stage.DurMs)
		if !strings.Contains(capturedContent, durStr) {
			t.Fatalf("prompt should contain stage duration %q", durStr)
		}
	}

	// Should mention collapsible thought log.
	if !strings.Contains(capturedContent, "Thought Log") {
		t.Fatalf("prompt should mention 'Thought Log'")
	}
}

func TestProgressiveDisclosure_Summary(t *testing.T) {
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "\033[36mResult\033[0m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	thought := &ThoughtLog{
		Stages: []ThoughtStage{
			{Number: 1, Name: "Work", Summary: "Did work", DurMs: 100},
		},
		TotalMs: 100,
	}

	result := pipeline.RunResult{
		TaskID:       "summary-test",
		Success:      true,
		Result:       "Some result",
		QualityScore: 0.75,
	}

	ui, err := gen.GenerateWithThought(context.Background(), result, CLICapabilities(), thought, []string{"be concise"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Meta.Summary should be set based on TotalMs.
	if ui.Meta.Summary == "" {
		t.Fatal("expected Meta.Summary to be set")
	}
	expectedSummary := fmt.Sprintf("Completed in %dms", thought.TotalMs)
	if ui.Meta.Summary != expectedSummary {
		t.Fatalf("expected summary %q, got %q", expectedSummary, ui.Meta.Summary)
	}

	// Thought should be attached to UI.
	if ui.Thought == nil {
		t.Fatal("expected Thought to be attached to GeneratedUI")
	}
	if ui.Thought.TotalMs != 100 {
		t.Fatalf("expected Thought.TotalMs=100, got %d", ui.Thought.TotalMs)
	}
}
