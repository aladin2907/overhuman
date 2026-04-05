package genui

import (
	"context"
	"strings"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// ── Detection Tests ──────────────────────────────────────────────────

func TestDetect_JSON_Object(t *testing.T) {
	ct := detectContentType(`{"name": "Alice", "age": 30}`)
	if ct != ContentJSON {
		t.Errorf("got %q, want json", ct)
	}
}

func TestDetect_JSON_Array(t *testing.T) {
	ct := detectContentType(`[1, 2, 3, "hello"]`)
	if ct != ContentJSON {
		t.Errorf("got %q, want json", ct)
	}
}

func TestDetect_JSON_Invalid(t *testing.T) {
	ct := detectContentType(`{broken json`)
	if ct == ContentJSON {
		t.Error("broken JSON should not match")
	}
}

func TestDetect_Error_WithColon(t *testing.T) {
	ct := detectContentType("Error: connection refused\nat line 42")
	if ct != ContentError {
		t.Errorf("got %q, want error", ct)
	}
}

func TestDetect_Error_FailedTo(t *testing.T) {
	ct := detectContentType("failed to connect to database: timeout")
	if ct != ContentError {
		t.Errorf("got %q, want error", ct)
	}
}

func TestDetect_Error_GoStackTrace(t *testing.T) {
	ct := detectContentType("goroutine 1 [running]:\nmain.go:42\nruntime/panic.go:100")
	if ct != ContentError {
		t.Errorf("got %q, want error", ct)
	}
}

func TestDetect_Table_PipeSeparated(t *testing.T) {
	content := "| Name | Age |\n|------|-----|\n| Alice | 30 |\n| Bob | 25 |"
	ct := detectContentType(content)
	if ct != ContentTable {
		t.Errorf("got %q, want table", ct)
	}
}

func TestDetect_Table_TabSeparated(t *testing.T) {
	content := "Name\tAge\nAlice\t30\nBob\t25"
	ct := detectContentType(content)
	if ct != ContentTable {
		t.Errorf("got %q, want table", ct)
	}
}

func TestDetect_Code_BacktickFence(t *testing.T) {
	content := "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```"
	ct := detectContentType(content)
	if ct != ContentCode {
		t.Errorf("got %q, want code", ct)
	}
}

func TestDetect_Code_IndentedWithKeywords(t *testing.T) {
	content := "    func main() {\n        var x := 10\n        return x\n        const y = 20\n    }"
	ct := detectContentType(content)
	if ct != ContentCode {
		t.Errorf("got %q, want code", ct)
	}
}

func TestDetect_List_Dashes(t *testing.T) {
	content := "- First item\n- Second item\n- Third item"
	ct := detectContentType(content)
	if ct != ContentList {
		t.Errorf("got %q, want list", ct)
	}
}

func TestDetect_List_Numbered(t *testing.T) {
	content := "1. First step\n2. Second step\n3. Third step"
	ct := detectContentType(content)
	if ct != ContentList {
		t.Errorf("got %q, want list", ct)
	}
}

func TestDetect_List_Bullets(t *testing.T) {
	content := "• Alpha\n• Beta\n• Gamma"
	ct := detectContentType(content)
	if ct != ContentList {
		t.Errorf("got %q, want list", ct)
	}
}

func TestDetect_KeyValue_Colon(t *testing.T) {
	content := "Name: Alice\nAge: 30\nCity: New York"
	ct := detectContentType(content)
	if ct != ContentKeyValue {
		t.Errorf("got %q, want key_value", ct)
	}
}

func TestDetect_KeyValue_Equals(t *testing.T) {
	content := "host = localhost\nport = 5432\ndb = mydb"
	ct := detectContentType(content)
	if ct != ContentKeyValue {
		t.Errorf("got %q, want key_value", ct)
	}
}

func TestDetect_Short_PlainText(t *testing.T) {
	ct := detectContentType("Hello world")
	if ct != ContentShort {
		t.Errorf("got %q, want short", ct)
	}
}

func TestDetect_Unknown_LongProse(t *testing.T) {
	content := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 10)
	ct := detectContentType(content)
	if ct != ContentUnknown {
		t.Errorf("got %q, want unknown for long prose", ct)
	}
}

// ── ANSI Rendering Tests ─────────────────────────────────────────────

func TestRenderANSI_Short_HasEscapeCodes(t *testing.T) {
	result := renderShortANSI("Hello world")
	if !strings.Contains(result, "\033[") {
		t.Error("short ANSI should contain escape codes")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("short ANSI should contain reset")
	}
	if !strings.Contains(result, "Result") {
		t.Error("short ANSI should contain 'Result' header")
	}
	if !strings.Contains(result, "Hello world") {
		t.Error("short ANSI should contain content")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_Error_RedColor(t *testing.T) {
	result := renderErrorANSI("Error: something broke")
	if !strings.Contains(result, "\033[31m") {
		t.Error("error ANSI should contain red color")
	}
	if !strings.Contains(result, "Error") {
		t.Error("error ANSI should contain 'Error' label")
	}
	if !strings.Contains(result, "┌") {
		t.Error("error ANSI should contain box drawing")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_Code_HasBorder(t *testing.T) {
	result := renderCodeANSI("```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```")
	if !strings.Contains(result, "│") {
		t.Error("code ANSI should contain │ border")
	}
	if !strings.Contains(result, "func main") {
		t.Error("code ANSI should contain code content")
	}
	if !strings.Contains(result, "go") {
		t.Error("code ANSI should contain language label")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_Table_BoxDrawing(t *testing.T) {
	content := "| Name | Score |\n|------|-------|\n| Alice | 95 |\n| Bob | 87 |"
	result := renderTableANSI(content)
	for _, ch := range []string{"┌", "┐", "└", "┘", "│", "─", "├", "┤", "┬", "┴", "┼"} {
		if !strings.Contains(result, ch) {
			t.Errorf("table should contain box-drawing char %q", ch)
		}
	}
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Error("table should contain data")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_List_CyanBullets(t *testing.T) {
	content := "- First item\n- Second item\n- Third item"
	result := renderListANSI(content)
	if !strings.Contains(result, "\033[36m") {
		t.Error("list ANSI should contain cyan color")
	}
	if !strings.Contains(result, "•") {
		t.Error("list ANSI should contain bullet chars")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_KV_Aligned(t *testing.T) {
	content := "Name: Alice\nAge: 30"
	result := renderKVANSI(content)
	if !strings.Contains(result, "\033[36m") {
		t.Error("KV ANSI should contain cyan keys")
	}
	if !strings.Contains(result, "│") {
		t.Error("KV ANSI should contain separator")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

func TestRenderANSI_JSON_PrettyPrinted(t *testing.T) {
	result := renderJSONANSI(`{"name":"Alice","age":30}`)
	if !strings.Contains(result, "│") {
		t.Error("JSON ANSI should contain code border")
	}
	if !strings.Contains(result, "name") {
		t.Error("JSON ANSI should contain key")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass ANSI validation: %v", err)
	}
}

// ── HTML Rendering Tests ─────────────────────────────────────────────

func TestRenderHTML_Short_CompletePage(t *testing.T) {
	result := renderShortHTML("Hello world")
	assertValidHTML(t, result)
	if !strings.Contains(result, "Hello world") {
		t.Error("HTML should contain content")
	}
}

func TestRenderHTML_Error_RedPanel(t *testing.T) {
	result := renderErrorHTML("Error: broken pipe")
	assertValidHTML(t, result)
	if !strings.Contains(result, "ff3b30") || !strings.Contains(result, "error") {
		t.Error("error HTML should have red styling")
	}
}

func TestRenderHTML_Code_MonospaceBlock(t *testing.T) {
	result := renderCodeHTML("```python\nprint('hello')\n```")
	assertValidHTML(t, result)
	if !strings.Contains(result, "<pre>") {
		t.Error("code HTML should contain <pre> tag")
	}
	if !strings.Contains(result, "python") {
		t.Error("code HTML should show language")
	}
}

func TestRenderHTML_Table_StyledTable(t *testing.T) {
	content := "| Name | Score |\n|------|-------|\n| Alice | 95 |"
	result := renderTableHTML(content)
	assertValidHTML(t, result)
	if !strings.Contains(result, "<table>") {
		t.Error("table HTML should contain <table>")
	}
	if !strings.Contains(result, "<th>") {
		t.Error("table HTML should contain <th> headers")
	}
}

func TestRenderHTML_List_StyledList(t *testing.T) {
	content := "- Item one\n- Item two"
	result := renderListHTML(content)
	assertValidHTML(t, result)
	if !strings.Contains(result, "<li>") {
		t.Error("list HTML should contain <li> elements")
	}
}

func TestRenderHTML_KV_Grid(t *testing.T) {
	content := "Name: Alice\nAge: 30"
	result := renderKVHTML(content)
	assertValidHTML(t, result)
	if !strings.Contains(result, "kv-grid") {
		t.Error("KV HTML should use grid layout")
	}
}

func TestRenderHTML_JSON_Formatted(t *testing.T) {
	result := renderJSONHTML(`{"key":"value"}`)
	assertValidHTML(t, result)
	if !strings.Contains(result, "<pre>") {
		t.Error("JSON HTML should contain <pre> block")
	}
}

func TestRenderHTML_NoDarkThemeMissing(t *testing.T) {
	result := renderShortHTML("test")
	if !strings.Contains(result, "#1a1a2e") {
		t.Error("HTML should use dark theme background")
	}
	if !strings.Contains(result, "#e0e0e0") {
		t.Error("HTML should use light text color")
	}
	if !strings.Contains(result, "#00d4aa") {
		t.Error("HTML should use accent color")
	}
}

func TestRenderHTML_NoExternalDeps(t *testing.T) {
	cases := []string{
		renderShortHTML("test"),
		renderCodeHTML("```go\nfoo()\n```"),
		renderTableHTML("| a | b |\n|---|---|\n| 1 | 2 |"),
		renderErrorHTML("Error: x"),
	}
	forbidden := []string{"cdn.jsdelivr", "unpkg.com", "cdnjs.cloudflare"}
	for i, html := range cases {
		for _, f := range forbidden {
			if strings.Contains(html, f) {
				t.Errorf("case %d: HTML should not contain %q", i, f)
			}
		}
	}
}

// ── TryFastPath Integration ──────────────────────────────────────────

func TestFastPath_ANSI_MatchesCode(t *testing.T) {
	fp := TryFastPath("```go\nfunc main() {}\n```", FormatANSI)
	if !fp.Matched {
		t.Fatal("expected fast path to match code")
	}
	if fp.Type != ContentCode {
		t.Errorf("Type = %q, want code", fp.Type)
	}
	if !strings.Contains(fp.Code, "│") {
		t.Error("ANSI code should have border")
	}
}

func TestFastPath_HTML_MatchesTable(t *testing.T) {
	fp := TryFastPath("| A | B |\n|---|---|\n| 1 | 2 |", FormatHTML)
	if !fp.Matched {
		t.Fatal("expected fast path to match table")
	}
	if fp.Type != ContentTable {
		t.Errorf("Type = %q, want table", fp.Type)
	}
	if !strings.Contains(fp.Code, "<table>") {
		t.Error("HTML table should contain <table>")
	}
}

func TestFastPath_ReactFormat_NeverMatches(t *testing.T) {
	fp := TryFastPath("- item1\n- item2", FormatReact)
	if fp.Matched {
		t.Error("React format should never match fast path")
	}
}

func TestFastPath_MarkdownFormat_NeverMatches(t *testing.T) {
	fp := TryFastPath("hello", FormatMarkdown)
	if fp.Matched {
		t.Error("Markdown format should never match fast path")
	}
}

func TestFastPath_EmptyContent(t *testing.T) {
	fp := TryFastPath("", FormatANSI)
	if !fp.Matched {
		t.Fatal("empty content should match fast path")
	}
	if fp.Type != ContentShort {
		t.Errorf("Type = %q, want short", fp.Type)
	}
}

func TestFastPath_LongProse_FallsThrough(t *testing.T) {
	content := strings.Repeat("This is a long paragraph of text that does not match any pattern. ", 10)
	fp := TryFastPath(content, FormatANSI)
	if fp.Matched {
		t.Error("long prose should NOT match fast path")
	}
}

func TestFastPath_Generate_UseFastPath(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		t.Error("LLM should NOT be called when fast path matches")
		return &brain.LLMResponse{Content: "x\033[0m", Model: "m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router) // fast path enabled by default

	result := pipeline.RunResult{
		TaskID:       "fp_test",
		Success:      true,
		Result:       "- item one\n- item two\n- item three",
		QualityScore: 0.9,
	}
	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if ui.Source != "fastpath" {
		t.Errorf("Source = %q, want fastpath", ui.Source)
	}
	if mock.requestCount() != 0 {
		t.Errorf("LLM call count = %d, want 0", mock.requestCount())
	}
}

func TestFastPath_Generate_FallsThrough(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content: genAnsiSimpleText,
			Model:   "mock-model",
		}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	content := strings.Repeat("Lorem ipsum dolor sit amet. ", 20)
	result := pipeline.RunResult{
		TaskID:       "fp_fall",
		Success:      true,
		Result:       content,
		QualityScore: 0.8,
	}
	ui, err := gen.Generate(context.Background(), result, CLICapabilities())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if ui.Source != "llm" {
		t.Errorf("Source = %q, want llm", ui.Source)
	}
	if mock.requestCount() == 0 {
		t.Error("LLM should have been called for unknown content")
	}
}

func TestFastPath_GenerateWithThought_IncludesThought(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		t.Error("LLM should not be called")
		return &brain.LLMResponse{Content: "x\033[0m", Model: "m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	thought := &ThoughtLog{
		TotalMs: 200,
		Stages: []ThoughtStage{
			{Number: 1, Name: "parse", Summary: "Parsed", DurMs: 100},
			{Number: 2, Name: "exec", Summary: "Done", DurMs: 100},
		},
	}
	result := pipeline.RunResult{
		TaskID:       "fp_thought",
		Success:      true,
		Result:       "Hello world",
		QualityScore: 0.9,
	}
	ui, err := gen.GenerateWithThought(context.Background(), result, CLICapabilities(), thought, nil)
	if err != nil {
		t.Fatalf("GenerateWithThought: %v", err)
	}
	if ui.Source != "fastpath" {
		t.Errorf("Source = %q, want fastpath", ui.Source)
	}
	if ui.Thought == nil {
		t.Fatal("Thought should be set")
	}
	if ui.Thought.TotalMs != 200 {
		t.Errorf("TotalMs = %d, want 200", ui.Thought.TotalMs)
	}
	if !strings.Contains(ui.Code, "Stage 1") {
		t.Error("fast path code should include thought log for ANSI")
	}
	if ui.Meta.Summary != "Completed in 200ms" {
		t.Errorf("Summary = %q, want 'Completed in 200ms'", ui.Meta.Summary)
	}
}

// ── Edge Cases ───────────────────────────────────────────────────────

func TestDetect_MixedContent_PicksDominant(t *testing.T) {
	// Content with both list markers and key-value — list has higher priority.
	content := "- Name: Alice\n- Age: 30\n- City: NYC"
	ct := detectContentType(content)
	if ct != ContentList {
		t.Errorf("got %q, want list (higher priority than KV)", ct)
	}
}

func TestDetect_SingleLine_NotList(t *testing.T) {
	ct := detectContentType("- just one bullet")
	// Only 1 line with bullet — not a list (needs >= 2).
	if ct == ContentList {
		t.Error("single bullet should not be a list")
	}
}

func TestDetect_Unicode_Content(t *testing.T) {
	content := "名前: アリス\n年齢: 30\n市: 東京"
	ct := detectContentType(content)
	if ct != ContentKeyValue {
		t.Errorf("got %q, want key_value for Unicode KV", ct)
	}
}

func TestRenderANSI_Table_EmptyCells(t *testing.T) {
	content := "| A | B |\n|---|---|\n| 1 |  |\n|  | 2 |"
	result := renderTableANSI(content)
	if !strings.Contains(result, "┌") {
		t.Error("table with empty cells should still render")
	}
	if err := validateANSI(result); err != nil {
		t.Errorf("should pass validation: %v", err)
	}
}

// ── Helpers ──────────────────────────────────────────────────────────

func assertValidHTML(t *testing.T, code string) {
	t.Helper()
	if !strings.Contains(code, "<!DOCTYPE html>") {
		t.Error("HTML should contain DOCTYPE")
	}
	if !strings.Contains(code, "<html>") {
		t.Error("HTML should contain <html>")
	}
	if !strings.Contains(code, "</html>") {
		t.Error("HTML should contain </html>")
	}
	if !strings.Contains(code, "<body>") {
		t.Error("HTML should contain <body>")
	}
	if !strings.Contains(code, "<style>") {
		t.Error("HTML should contain <style>")
	}
	if err := validateHTML(code); err != nil {
		t.Errorf("should pass HTML validation: %v", err)
	}
}
