package genui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// mockLLM implements brain.LLMProvider for all genui tests.
// It delegates Complete calls to a callback function and captures
// every request for later assertion.
type mockLLM struct {
	mu         sync.Mutex
	fn         func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error)
	captured   []brain.LLMRequest
	callCount  int
}

// newMockLLM creates a mockLLM that delegates to the given callback.
func newMockLLM(fn func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error)) *mockLLM {
	return &mockLLM{fn: fn}
}

func (m *mockLLM) Complete(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
	m.mu.Lock()
	m.callCount++
	m.captured = append(m.captured, req)
	m.mu.Unlock()
	return m.fn(ctx, req)
}

func (m *mockLLM) Name() string    { return "mock" }
func (m *mockLLM) Models() []string { return []string{"mock-model"} }

// requestCount returns the number of LLM calls made so far.
func (m *mockLLM) requestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// lastRequest returns the most recent captured request.
func (m *mockLLM) lastRequest() brain.LLMRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.captured) == 0 {
		return brain.LLMRequest{}
	}
	return m.captured[len(m.captured)-1]
}

// --- Helper to build RunResult values ---

func genSimpleResult(text string, quality float64) pipeline.RunResult {
	return pipeline.RunResult{
		TaskID:       "task_test_123",
		Success:      true,
		Result:       text,
		QualityScore: quality,
	}
}

func genErrorResult(errText string) pipeline.RunResult {
	return pipeline.RunResult{
		TaskID:       "task_err_456",
		Success:      false,
		Result:       errText,
		QualityScore: 0.1,
	}
}

// --- Valid ANSI responses for mock ---

const genAnsiSimpleText = "\033[1m\033[36m\u2501\u2501\u2501 Result \u2501\u2501\u2501\033[0m\n\nHello, this is a simple text result.\n\033[90m\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\033[0m\n"

const genAnsiCodeResult = "\033[1m\033[36m Code \033[0m\n\033[90m\u2502\033[0m func main() {\n\033[90m\u2502\033[0m     fmt.Println(\"hello\")\n\033[90m\u2502\033[0m }\n\033[0m"

const genAnsiTableData = "\033[1m\033[36m Data Table \033[0m\n\u250c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u252c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2510\n\u2502 \033[1mName\033[0m     \u2502 \033[1mScore\033[0m \u2502\n\u251c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u253c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2524\n\u2502 Alice    \u2502 95    \u2502\n\u2502 Bob      \u2502 87    \u2502\n\u2514\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2534\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2518\n\033[0m"

const genAnsiErrorBlock = "\033[1m\033[31m\u2717 Error\033[0m\n\033[31m\u250c\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2510\033[0m\n\033[31m\u2502 Connection refused: ECONNREFUSED \u2502\033[0m\n\033[31m\u2514\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2518\033[0m\n"

const genAnsiWithActions = "\033[1m\033[36m Result \033[0m\nDeploy completed successfully.\n\n\033[33mActions:\033[0m\n  \033[1m1.\033[0m View Logs\n  \033[1m2.\033[0m Rollback\n  \033[1m3.\033[0m Open Dashboard\n\033[0m"

// --- Valid HTML responses for mock ---

const genHtmlFullPage = `<!DOCTYPE html>
<html>
<head><title>Result</title>
<style>
body { background: #1a1a2e; color: #e0e0e0; font-family: sans-serif; }
.container { max-width: 800px; margin: 0 auto; padding: 20px; }
.accent { color: #00d4aa; }
</style>
</head>
<body>
<div class="container">
<h1 class="accent">Task Result</h1>
<p>The analysis is complete. Here are the findings.</p>
</div>
</body>
</html>`

const genHtmlWithActions = `<!DOCTYPE html>
<html>
<head><title>Actions</title>
<style>
body { background: #1a1a2e; color: #e0e0e0; font-family: sans-serif; }
.btn { padding: 10px 20px; background: #00d4aa; border: none; cursor: pointer; }
</style>
</head>
<body>
<div>
<h1>Deploy Complete</h1>
<button class="btn" onclick="window.parent.postMessage({action: 'view_logs', data: {}}, '*')">View Logs</button>
<button class="btn" onclick="window.parent.postMessage({action: 'rollback', data: {}}, '*')">Rollback</button>
</div>
</body>
</html>`

// --- Generator Tests ---

func TestGenerate_ANSI_SimpleText(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// Verify system prompt is the ANSI one.
		if len(req.Messages) < 2 {
			t.Errorf("expected at least 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("first message role = %q, want system", req.Messages[0].Role)
		}
		if !strings.Contains(req.Messages[0].Content, "terminal UI generator") {
			t.Error("system prompt should contain 'terminal UI generator'")
		}
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Hello, this is a simple text result.", 0.9)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if ui.TaskID != "task_test_123" {
		t.Errorf("TaskID = %q, want task_test_123", ui.TaskID)
	}
	if ui.Format != FormatANSI {
		t.Errorf("Format = %q, want ansi", ui.Format)
	}
	if !strings.Contains(ui.Code, "\033[") {
		t.Error("ANSI output should contain escape codes")
	}
	if !strings.Contains(ui.Code, "\033[0m") {
		t.Error("ANSI output should contain reset codes")
	}
	if !strings.Contains(ui.Code, "Result") {
		t.Error("output should contain result text")
	}
}

func TestGenerate_ANSI_CodeResult(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genAnsiCodeResult,
			Model:        "mock-model",
			InputTokens:  80,
			OutputTokens: 40,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```", 0.95)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if ui.Format != FormatANSI {
		t.Errorf("Format = %q, want ansi", ui.Format)
	}
	// Should contain box-drawing border character for code.
	if !strings.Contains(ui.Code, "\u2502") { // │
		t.Error("code result should contain box-drawing border character")
	}
	// Should contain ANSI formatting.
	if !strings.Contains(ui.Code, "\033[") {
		t.Error("code result should contain ANSI escape codes")
	}
	if !strings.Contains(ui.Code, "func main") {
		t.Error("code result should contain the code content")
	}
}

func TestGenerate_ANSI_TableData(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genAnsiTableData,
			Model:        "mock-model",
			InputTokens:  90,
			OutputTokens: 60,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Name: Alice, Score: 95; Name: Bob, Score: 87", 0.88)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if ui.Format != FormatANSI {
		t.Errorf("Format = %q, want ansi", ui.Format)
	}
	// Should contain ASCII table box-drawing characters.
	for _, ch := range []string{
		"\u250c", // ┌
		"\u2510", // ┐
		"\u2514", // └
		"\u2518", // ┘
		"\u2502", // │
		"\u2500", // ─
		"\u251c", // ├
		"\u2524", // ┤
	} {
		if !strings.Contains(ui.Code, ch) {
			t.Errorf("table should contain box-drawing char U+%04X", []rune(ch)[0])
		}
	}
	if !strings.Contains(ui.Code, "Alice") {
		t.Error("table should contain data 'Alice'")
	}
	if !strings.Contains(ui.Code, "Bob") {
		t.Error("table should contain data 'Bob'")
	}
}

func TestGenerate_ANSI_ErrorResult(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// Verify the user content mentions error.
		userMsg := req.Messages[1].Content
		if !strings.Contains(userMsg, "Connection refused") {
			t.Error("user prompt should contain the error text")
		}
		return &brain.LLMResponse{
			Content:      genAnsiErrorBlock,
			Model:        "mock-model",
			InputTokens:  70,
			OutputTokens: 30,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genErrorResult("Connection refused: ECONNREFUSED")
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should contain red ANSI color code \033[31m.
	if !strings.Contains(ui.Code, "\033[31m") {
		t.Error("error result should contain red ANSI code \\033[31m")
	}
	if !strings.Contains(ui.Code, "Error") {
		t.Error("error result should contain 'Error' label")
	}
}

func TestGenerate_ANSI_WithActions(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genAnsiWithActions,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 70,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Deploy completed successfully.", 0.92)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should contain numbered action options.
	if !strings.Contains(ui.Code, "1.") {
		t.Error("should contain numbered action '1.'")
	}
	if !strings.Contains(ui.Code, "2.") {
		t.Error("should contain numbered action '2.'")
	}
	if !strings.Contains(ui.Code, "View Logs") {
		t.Error("should contain action label 'View Logs'")
	}
	if !strings.Contains(ui.Code, "Rollback") {
		t.Error("should contain action label 'Rollback'")
	}
}

func TestGenerate_HTML_FullPage(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		// Verify system prompt is the HTML one.
		if !strings.Contains(req.Messages[0].Content, "UI generator") {
			t.Error("system prompt should contain 'UI generator'")
		}
		if !strings.Contains(req.Messages[0].Content, "HTML") {
			t.Error("system prompt should mention HTML")
		}
		return &brain.LLMResponse{
			Content:      genHtmlFullPage,
			Model:        "mock-model",
			InputTokens:  120,
			OutputTokens: 200,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("The analysis is complete. Here are the findings.", 0.85)
	caps := WebCapabilities(1280, 800)

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if ui.Format != FormatHTML {
		t.Errorf("Format = %q, want html", ui.Format)
	}
	if !strings.Contains(ui.Code, "<!DOCTYPE html>") {
		t.Error("HTML output should contain DOCTYPE")
	}
	if !strings.Contains(ui.Code, "<html>") {
		t.Error("HTML output should contain <html> tag")
	}
	if !strings.Contains(ui.Code, "</html>") {
		t.Error("HTML output should contain closing </html> tag")
	}
	if !strings.Contains(ui.Code, "<body>") {
		t.Error("HTML output should contain <body> tag")
	}
	if !strings.Contains(ui.Code, "</body>") {
		t.Error("HTML output should contain closing </body> tag")
	}
}

func TestGenerate_HTML_ContainsCSS(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genHtmlFullPage,
			Model:        "mock-model",
			InputTokens:  120,
			OutputTokens: 200,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Styled content.", 0.88)
	caps := WebCapabilities(1280, 800)

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(ui.Code, "<style>") {
		t.Error("HTML output should contain inline <style> tag")
	}
	if !strings.Contains(ui.Code, "</style>") {
		t.Error("HTML output should contain closing </style> tag")
	}
	// Verify it has actual CSS rules.
	if !strings.Contains(ui.Code, "background") {
		t.Error("CSS should contain styling rules like 'background'")
	}
	if !strings.Contains(ui.Code, "color") {
		t.Error("CSS should contain 'color' property")
	}
}

func TestGenerate_HTML_NoExternalDeps(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genHtmlFullPage,
			Model:        "mock-model",
			InputTokens:  120,
			OutputTokens: 200,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Self-contained content.", 0.9)
	caps := WebCapabilities(1280, 800)

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The system prompt forbids CDN/import/fetch. Verify the output is clean.
	forbidden := []string{
		"cdn.jsdelivr.net",
		"unpkg.com",
		"cdnjs.cloudflare.com",
		"googleapis.com/ajax",
		"import ",
	}
	for _, pattern := range forbidden {
		if strings.Contains(ui.Code, pattern) {
			t.Errorf("HTML should not contain external dependency pattern %q", pattern)
		}
	}

	// Also verify via SanitizeHTML: no fetch or XMLHttpRequest.
	sanitized := SanitizeHTML(ui.Code)
	if strings.Contains(sanitized, "BLOCKED") {
		t.Error("HTML output should not trigger sanitizer blocks")
	}
}

func TestGenerate_HTML_WithActions(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genHtmlWithActions,
			Model:        "mock-model",
			InputTokens:  130,
			OutputTokens: 180,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Deploy complete with actions available.", 0.91)
	caps := WebCapabilities(1280, 800)

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should contain postMessage buttons.
	if !strings.Contains(ui.Code, "postMessage") {
		t.Error("HTML with actions should contain postMessage calls")
	}
	if !strings.Contains(ui.Code, "<button") {
		t.Error("HTML with actions should contain <button> elements")
	}
	if !strings.Contains(ui.Code, "view_logs") {
		t.Error("should contain action callback 'view_logs'")
	}
	if !strings.Contains(ui.Code, "rollback") {
		t.Error("should contain action callback 'rollback'")
	}
}

func TestGenerate_FallbackOnError(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return nil, errors.New("LLM service unavailable")
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Some text.", 0.8)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
	if ui != nil {
		t.Errorf("UI should be nil on error, got %+v", ui)
	}
	if !strings.Contains(err.Error(), "LLM service unavailable") {
		t.Errorf("error should contain LLM error message, got: %v", err)
	}
}

func TestGenerate_RespectsCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		caps       DeviceCapabilities
		wantFormat UIFormat
		wantPrompt string // substring expected in system prompt
	}{
		{
			name:       "CLI capabilities select ANSI",
			caps:       CLICapabilities(),
			wantFormat: FormatANSI,
			wantPrompt: "terminal UI generator",
		},
		{
			name:       "Web capabilities select HTML",
			caps:       WebCapabilities(1280, 800),
			wantFormat: FormatHTML,
			wantPrompt: "COMPLETE, SELF-CONTAINED HTML",
		},
		{
			name:       "Tablet capabilities select HTML",
			caps:       TabletCapabilities(1024, 768),
			wantFormat: FormatHTML,
			wantPrompt: "COMPLETE, SELF-CONTAINED HTML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPrompt string
			mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
				capturedPrompt = req.Messages[0].Content

				// Return format-appropriate content.
				var content string
				if tt.wantFormat == FormatANSI {
					content = genAnsiSimpleText
				} else {
					content = genHtmlFullPage
				}
				return &brain.LLMResponse{
					Content:      content,
					Model:        "mock-model",
					InputTokens:  100,
					OutputTokens: 50,
				}, nil
			})

			router := brain.NewModelRouter()
			gen := NewUIGenerator(mock, router)
			result := genSimpleResult("Test content.", 0.85)

			ui, err := gen.Generate(context.Background(), result, tt.caps)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}

			if ui.Format != tt.wantFormat {
				t.Errorf("Format = %q, want %q", ui.Format, tt.wantFormat)
			}
			if !strings.Contains(capturedPrompt, tt.wantPrompt) {
				limit := len(capturedPrompt)
				if limit > 200 {
					limit = 200
				}
				t.Errorf("system prompt should contain %q, got:\n%s", tt.wantPrompt, capturedPrompt[:limit])
			}

			// Verify device dimensions are included in user content.
			lastReq := mock.lastRequest()
			userMsg := lastReq.Messages[1].Content
			if tt.caps.Format == FormatANSI {
				if !strings.Contains(userMsg, "80x24") {
					t.Errorf("user prompt should contain CLI dimensions '80x24', got: %s", userMsg)
				}
			} else if tt.name == "Web capabilities select HTML" {
				if !strings.Contains(userMsg, "1280x800") {
					t.Errorf("user prompt should contain web dimensions '1280x800', got: %s", userMsg)
				}
			}
		})
	}
}

func TestGenerate_UsesHintsFromMemory(t *testing.T) {
	var capturedUserContent string
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		capturedUserContent = req.Messages[1].Content
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	result := genSimpleResult("Result with UI hints.", 0.87)
	caps := CLICapabilities()
	hints := []string{
		"User prefers compact tables",
		"User prefers dark theme with green accents",
	}
	thought := &ThoughtLog{
		TotalMs:   150,
		TotalCost: 0.002,
		Stages: []ThoughtStage{
			{Number: 1, Name: "parse", Summary: "Parsed input", DurMs: 30},
			{Number: 2, Name: "plan", Summary: "Created plan", DurMs: 50},
			{Number: 3, Name: "execute", Summary: "Executed task", DurMs: 70},
		},
	}

	ui, err := gen.GenerateWithThought(context.Background(), result, caps, thought, hints)
	if err != nil {
		t.Fatalf("GenerateWithThought: %v", err)
	}

	// Verify hints are in the user prompt.
	if !strings.Contains(capturedUserContent, "UI HINT: User prefers compact tables") {
		t.Error("prompt should contain hint about compact tables")
	}
	if !strings.Contains(capturedUserContent, "UI HINT: User prefers dark theme with green accents") {
		t.Error("prompt should contain hint about dark theme")
	}

	// Verify thought stages are in the prompt.
	if !strings.Contains(capturedUserContent, "Stage 1 (parse)") {
		t.Error("prompt should contain stage 1 info")
	}
	if !strings.Contains(capturedUserContent, "Stage 2 (plan)") {
		t.Error("prompt should contain stage 2 info")
	}
	if !strings.Contains(capturedUserContent, "Stage 3 (execute)") {
		t.Error("prompt should contain stage 3 info")
	}

	// Verify collapsible hint is present.
	if !strings.Contains(capturedUserContent, "collapsible 'Thought Log'") {
		t.Error("prompt should instruct to include collapsible thought log")
	}

	// Verify ThoughtLog is attached to result.
	if ui.Thought == nil {
		t.Fatal("ui.Thought should not be nil")
	}
	if ui.Thought.TotalMs != 150 {
		t.Errorf("Thought.TotalMs = %d, want 150", ui.Thought.TotalMs)
	}
	if len(ui.Thought.Stages) != 3 {
		t.Errorf("Thought.Stages len = %d, want 3", len(ui.Thought.Stages))
	}

	// Verify Meta.Summary is set from thought.
	if ui.Meta.Summary != "Completed in 150ms" {
		t.Errorf("Meta.Summary = %q, want 'Completed in 150ms'", ui.Meta.Summary)
	}
}

// --- Additional edge case tests ---

func TestGenerate_ValidationRetry(t *testing.T) {
	// First call returns invalid ANSI (no reset), second returns valid.
	callNum := 0
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		callNum++
		if callNum == 1 {
			// Invalid: has escape opens but no reset.
			return &brain.LLMResponse{
				Content:      "\033[36mNo reset here\033[31mStill no reset",
				Model:        "mock-model",
				InputTokens:  50,
				OutputTokens: 20,
			}, nil
		}
		// Valid on retry.
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  50,
			OutputTokens: 20,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("Test retry.", 0.8)
	caps := CLICapabilities()

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate should succeed after retry: %v", err)
	}
	if ui.Code != genAnsiSimpleText {
		t.Error("should return the valid response from retry")
	}
	if mock.requestCount() < 2 {
		t.Errorf("expected at least 2 LLM calls (initial + retry), got %d", mock.requestCount())
	}
}

func TestGenerate_ValidationExhausted(t *testing.T) {
	// Always returns invalid ANSI -- should exhaust retries and fail.
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      "\033[31mBad ANSI with no reset ever",
			Model:        "mock-model",
			InputTokens:  50,
			OutputTokens: 20,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("Test exhaust.", 0.8)
	caps := CLICapabilities()

	_, err := gen.Generate(context.Background(), result, caps)
	if err == nil {
		t.Fatal("expected error when all retries are exhausted")
	}
	if !strings.Contains(err.Error(), "UI generation failed after") {
		t.Errorf("error should mention failed attempts, got: %v", err)
	}
	// Should have tried 3 times total (initial + 2 retries).
	if mock.requestCount() != 3 {
		t.Errorf("expected 3 LLM calls, got %d", mock.requestCount())
	}
}

func TestGenerate_QualityScoreInPrompt(t *testing.T) {
	var capturedUserContent string
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		capturedUserContent = req.Messages[1].Content
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("Test quality display.", 0.73)
	caps := CLICapabilities()

	_, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Quality 0.73 should appear as "73%" in the prompt.
	if !strings.Contains(capturedUserContent, "73%") {
		t.Errorf("user prompt should contain quality '73%%', got: %s", capturedUserContent)
	}
}

func TestGenerate_WithThought_NilThought(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        "mock-model",
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("No thought provided.", 0.9)
	caps := CLICapabilities()

	ui, err := gen.GenerateWithThought(context.Background(), result, caps, nil, nil)
	if err != nil {
		t.Fatalf("GenerateWithThought: %v", err)
	}

	if ui.Thought != nil {
		t.Error("Thought should be nil when nil is passed")
	}
	if ui.Meta.Summary != "" {
		t.Errorf("Meta.Summary should be empty when no thought, got %q", ui.Meta.Summary)
	}
}

func TestGenerate_ModelRouterSelection(t *testing.T) {
	var capturedModel string
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		capturedModel = req.Model
		return &brain.LLMResponse{
			Content:      genAnsiSimpleText,
			Model:        req.Model,
			InputTokens:  100,
			OutputTokens: 50,
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("Test model selection.", 0.85)
	caps := CLICapabilities()

	_, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// generateWithRetry calls router.Select("simple", 100.0) which should pick a cheap model.
	if capturedModel == "" {
		t.Error("model should be set in the request")
	}
	// Should be a cheap tier model (haiku or mini).
	if !strings.Contains(capturedModel, "haiku") && !strings.Contains(capturedModel, "mini") {
		t.Errorf("expected cheap model for 'simple' complexity, got %q", capturedModel)
	}
}
