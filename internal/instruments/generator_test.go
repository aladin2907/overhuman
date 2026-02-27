package instruments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
)

func mockCodeLLM(t *testing.T, responseText string) *httptest.Server {
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
			ID:    "msg_gen",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []contentBlock{
				{Type: "text", Text: responseText},
			},
			StopReason: "end_turn",
		}
		r2.Usage.InputTokens = 100
		r2.Usage.OutputTokens = 200

		json.NewEncoder(w).Encode(r2)
	}))
}

func TestGenerator_Generate(t *testing.T) {
	srv := mockCodeLLM(t, `Here's the generated code:

CODE_START
def summarize(text):
    words = text.split()
    if len(words) <= 10:
        return text
    return ' '.join(words[:10]) + '...'
CODE_END

TESTS_START
def test_summarize():
    assert summarize("short") == "short"
    assert summarize("a " * 20) == "a a a a a a a a a a..."
TESTS_END`)
	defer srv.Close()

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srv.URL))
	gen := NewGenerator(llm, brain.NewModelRouter(), brain.NewContextAssembler())

	spec := CodeSpec{
		Goal:       "Summarize text to first 10 words",
		InputDesc:  "A string of text",
		OutputDesc: "Shortened text with ellipsis if truncated",
		Language:   "python",
	}

	generated, cost, err := gen.Generate(context.Background(), spec)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if cost <= 0 {
		t.Error("cost should be > 0")
	}
	if generated.Code == "" {
		t.Error("code should not be empty")
	}
	if generated.Tests == "" {
		t.Error("tests should not be empty")
	}
	if generated.Language != "python" {
		t.Errorf("language = %q", generated.Language)
	}
}

func TestGenerator_Generate_DefaultLanguage(t *testing.T) {
	srv := mockCodeLLM(t, "CODE_START\nprint('hello')\nCODE_END\nTESTS_START\nassert True\nTESTS_END")
	defer srv.Close()

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srv.URL))
	gen := NewGenerator(llm, brain.NewModelRouter(), brain.NewContextAssembler())

	generated, _, err := gen.Generate(context.Background(), CodeSpec{Goal: "test"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if generated.Language != "python" {
		t.Errorf("default language should be python, got %q", generated.Language)
	}
}

func TestGenerator_Generate_NoCodeBlock(t *testing.T) {
	srv := mockCodeLLM(t, "I can't generate code for this request.")
	defer srv.Close()

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srv.URL))
	gen := NewGenerator(llm, brain.NewModelRouter(), brain.NewContextAssembler())

	_, _, err := gen.Generate(context.Background(), CodeSpec{Goal: "impossible"})
	if err == nil {
		t.Fatal("expected error when no code block found")
	}
}

func TestGenerator_GenerateAndRegister(t *testing.T) {
	srv := mockCodeLLM(t, "CODE_START\ndef solve(): return 42\nCODE_END\nTESTS_START\nassert solve() == 42\nTESTS_END")
	defer srv.Close()

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srv.URL))
	gen := NewGenerator(llm, brain.NewModelRouter(), brain.NewContextAssembler())
	registry := NewSkillRegistry()

	spec := CodeSpec{
		Goal:        "Solve the ultimate question",
		Language:    "python",
		Fingerprint: "fp_42",
	}

	skill, cost, err := gen.GenerateAndRegister(context.Background(), spec, registry)
	if err != nil {
		t.Fatalf("GenerateAndRegister: %v", err)
	}
	if cost <= 0 {
		t.Error("cost should be > 0")
	}
	if skill.Meta.Type != SkillTypeCode {
		t.Errorf("type = %q, want CODE", skill.Meta.Type)
	}
	if skill.Meta.Status != SkillStatusTrial {
		t.Errorf("status = %q, want TRIAL", skill.Meta.Status)
	}
	if skill.Meta.Fingerprint != "fp_42" {
		t.Errorf("fingerprint = %q", skill.Meta.Fingerprint)
	}

	// Should be in the registry.
	if registry.Count() != 1 {
		t.Errorf("registry count = %d, want 1", registry.Count())
	}
	found := registry.FindByFingerprint("fp_42")
	if len(found) != 1 {
		t.Errorf("found %d skills by fingerprint", len(found))
	}

	// Execute the registered skill.
	output, err := skill.Executor.Execute(context.Background(), SkillInput{Goal: "test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !output.Success {
		t.Error("expected success")
	}
}

func TestGenerator_LLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"type":"server_error","message":"boom"}}`))
	}))
	defer srv.Close()

	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srv.URL))
	gen := NewGenerator(llm, brain.NewModelRouter(), brain.NewContextAssembler())

	_, _, err := gen.Generate(context.Background(), CodeSpec{Goal: "fail"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractBlock(t *testing.T) {
	tests := []struct {
		text  string
		start string
		end   string
		want  string
	}{
		{"before CODE_START\nhello\nCODE_END after", "CODE_START", "CODE_END", "hello"},
		{"no markers here", "CODE_START", "CODE_END", ""},
		{"CODE_START only", "CODE_START", "CODE_END", ""},
		{"CODE_START\n  spaced  \nCODE_END", "CODE_START", "CODE_END", "spaced"},
	}

	for _, tt := range tests {
		got := extractBlock(tt.text, tt.start, tt.end)
		if got != tt.want {
			t.Errorf("extractBlock(%q, %q, %q) = %q, want %q", tt.text, tt.start, tt.end, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
	got := truncate("a very long string", 5)
	if got != "a ver..." {
		t.Errorf("truncate = %q", got)
	}
}
