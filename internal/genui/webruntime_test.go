package genui

import (
	"context"
	"strings"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
)

func TestReactPrompt_ContainsRules(t *testing.T) {
	if !strings.Contains(SystemPromptReact, "React") {
		t.Error("React prompt should mention React")
	}
	if !strings.Contains(SystemPromptReact, "Tailwind") {
		t.Error("React prompt should mention Tailwind")
	}
	if !strings.Contains(SystemPromptReact, "export default function") {
		t.Error("React prompt should contain component structure")
	}
}

func TestReactPrompt_AllowedImports(t *testing.T) {
	allowed := []string{"react", "recharts", "lucide-react", "date-fns"}
	for _, imp := range allowed {
		if !strings.Contains(SystemPromptReact, imp) {
			t.Errorf("React prompt should allow import %q", imp)
		}
	}
}

func TestReactPrompt_BlockedImports(t *testing.T) {
	blocked := []string{"axios", "node-fetch", "fs", "child_process"}
	for _, imp := range blocked {
		if !strings.Contains(SystemPromptReact, imp) {
			t.Errorf("React prompt should mention blocked import %q", imp)
		}
	}
}

func TestReactPrompt_NoFetch(t *testing.T) {
	if !strings.Contains(SystemPromptReact, "NO fetch") {
		t.Error("React prompt should forbid fetch/axios")
	}
}

func TestHTMLPrompt_ContainsRules(t *testing.T) {
	if !strings.Contains(SystemPromptHTML, "HTML") {
		t.Error("HTML prompt should mention HTML")
	}
	if !strings.Contains(strings.ToLower(SystemPromptHTML), "dark theme") {
		t.Error("HTML prompt should mention dark theme")
	}
	if !strings.Contains(SystemPromptHTML, "NO external dependencies") {
		t.Error("HTML prompt should forbid external dependencies")
	}
}

func TestHTMLPrompt_PostMessagePattern(t *testing.T) {
	if !strings.Contains(SystemPromptHTML, "postMessage") {
		t.Error("HTML prompt should describe postMessage pattern for actions")
	}
}

func TestGenerate_ReactFormat(t *testing.T) {
	reactCode := `export default function Component({ data, onAction }) {
  return (
    <div className="min-h-screen bg-gray-900 text-gray-100 p-4">
      <h1>Hello</h1>
    </div>
  );
}`

	var capturedPrompt string
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		capturedPrompt = req.Messages[0].Content
		return &brain.LLMResponse{
			Content: reactCode,
			Model:   "mock",
		}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("React test", 0.9)

	// Create capabilities with React format.
	caps := DeviceCapabilities{
		Format:     FormatReact,
		Width:      1280,
		Height:     800,
		JavaScript: true,
	}

	ui, err := gen.Generate(context.Background(), result, caps)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if ui.Format != FormatReact {
		t.Errorf("Format = %q, want react", ui.Format)
	}

	// Verify React system prompt was used.
	if !strings.Contains(capturedPrompt, "React") {
		t.Error("should use React system prompt for react format")
	}
	if !strings.Contains(capturedPrompt, "Tailwind") {
		t.Error("React prompt should mention Tailwind")
	}
}

func TestSelectFormat_ANSI(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	caps := CLICapabilities()
	format := gen.selectFormat(caps)
	if format != FormatANSI {
		t.Errorf("selectFormat(CLI) = %q, want ansi", format)
	}
}

func TestSelectFormat_HTML(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	caps := WebCapabilities(1280, 800)
	format := gen.selectFormat(caps)
	if format != FormatHTML {
		t.Errorf("selectFormat(Web) = %q, want html", format)
	}
}

func TestSelectFormat_React(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	caps := DeviceCapabilities{Format: FormatReact, JavaScript: true}
	format := gen.selectFormat(caps)
	if format != FormatReact {
		t.Errorf("selectFormat(React) = %q, want react", format)
	}
}

func TestSelectFormat_Markdown(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	caps := DeviceCapabilities{Format: FormatMarkdown}
	format := gen.selectFormat(caps)
	if format != FormatMarkdown {
		t.Errorf("selectFormat(Markdown) = %q, want markdown", format)
	}
}

func TestBuildPrompt_IncludesDeviceDimensions(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	result := genSimpleResult("dim test", 0.8)
	caps := WebCapabilities(1920, 1080)

	msgs := gen.buildPrompt(result, FormatHTML, caps, nil, nil)
	if len(msgs) < 2 {
		t.Fatal("expected at least 2 messages")
	}
	userMsg := msgs[1].Content
	if !strings.Contains(userMsg, "1920x1080") {
		t.Errorf("user prompt should contain device dimensions '1920x1080', got: %s", userMsg)
	}
}

func TestBuildPrompt_ReactSystemPrompt(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	result := genSimpleResult("test", 0.8)
	caps := DeviceCapabilities{Format: FormatReact}

	msgs := gen.buildPrompt(result, FormatReact, caps, nil, nil)
	if !strings.Contains(msgs[0].Content, "React") {
		t.Error("React format should use SystemPromptReact")
	}
}
