// Package genui implements Level 3 Fully Generated UI for Overhuman.
// LLM generates raw ANSI (CLI) or HTML/CSS/JS (web/tablet) from scratch.
package genui

import (
	"context"
	"fmt"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// UIFormat describes the output format for generated UI.
type UIFormat string

const (
	FormatANSI     UIFormat = "ansi"     // CLI: ANSI escape + box drawing
	FormatHTML     UIFormat = "html"     // Web/Tablet: HTML + CSS + JS
	FormatReact    UIFormat = "react"    // Web: React + Tailwind (compiled in browser)
	FormatMarkdown UIFormat = "markdown" // Fallback: plain markdown
)

// GeneratedUI is a fully LLM-generated UI for a single pipeline response.
type GeneratedUI struct {
	TaskID  string            `json:"task_id"`
	Format  UIFormat          `json:"format"`
	Code    string            `json:"code"`              // full UI code
	Actions []GeneratedAction `json:"actions,omitempty"`
	Meta    UIMeta            `json:"meta,omitempty"`
	Thought *ThoughtLog       `json:"thought,omitempty"` // pipeline thought chain
	Sandbox bool              `json:"sandbox,omitempty"` // wrap in sandboxed iframe
}

// GeneratedAction is an interactive action embedded in generated UI.
type GeneratedAction struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Callback string `json:"callback"` // callback ID for daemon
}

// UIMeta holds metadata about the generated UI.
type UIMeta struct {
	Title     string `json:"title,omitempty"`
	Summary   string `json:"summary,omitempty"` // TL;DR for progressive disclosure
	Streaming bool   `json:"streaming,omitempty"`
}

// DeviceCapabilities describes what the rendering device supports.
type DeviceCapabilities struct {
	Format      UIFormat `json:"format"`
	Width       int      `json:"width"`       // chars (CLI) or pixels (web)
	Height      int      `json:"height"`
	ColorDepth  int      `json:"color_depth"` // 1, 8, 256, 16M
	Interactive bool     `json:"interactive"`
	JavaScript  bool     `json:"javascript"`
	SVG         bool     `json:"svg"`
	Animation   bool     `json:"animation"`
	TouchScreen bool     `json:"touch_screen"`
}

// CLICapabilities returns terminal device capabilities.
func CLICapabilities() DeviceCapabilities {
	return DeviceCapabilities{
		Format:      FormatANSI,
		Width:       80,
		Height:      24,
		ColorDepth:  256,
		Interactive: true,
		JavaScript:  false,
		SVG:         false,
		Animation:   false,
		TouchScreen: false,
	}
}

// WebCapabilities returns web browser capabilities.
func WebCapabilities(w, h int) DeviceCapabilities {
	return DeviceCapabilities{
		Format:      FormatHTML,
		Width:       w,
		Height:      h,
		ColorDepth:  16777216, // 24-bit
		Interactive: true,
		JavaScript:  true,
		SVG:         true,
		Animation:   true,
		TouchScreen: false,
	}
}

// TabletCapabilities returns tablet kiosk capabilities.
func TabletCapabilities(w, h int) DeviceCapabilities {
	return DeviceCapabilities{
		Format:      FormatHTML,
		Width:       w,
		Height:      h,
		ColorDepth:  16777216,
		Interactive: true,
		JavaScript:  true,
		SVG:         true,
		Animation:   true,
		TouchScreen: true,
	}
}

// ThoughtLog records pipeline stages for UI display.
type ThoughtLog struct {
	Stages    []ThoughtStage `json:"stages"`
	TotalMs   int64          `json:"total_ms"`
	TotalCost float64        `json:"total_cost"`
}

// ThoughtStage is one step in the pipeline thought chain.
type ThoughtStage struct {
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	DurMs   int64  `json:"duration_ms"`
}

// UIChunk is a streaming fragment of generated UI.
type UIChunk struct {
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done"`
	Error   error  `json:"-"`
}

// UIReflection records how the user interacted with generated UI.
type UIReflection struct {
	TaskID       string   `json:"task_id"`
	UIFormat     UIFormat `json:"format"`
	ActionsShown []string `json:"actions_shown"`
	ActionsUsed  []string `json:"actions_used"`
	TimeToAction int64    `json:"time_to_action_ms"`
	Scrolled     bool     `json:"scrolled"`
	Dismissed    bool     `json:"dismissed"`
}

// UIGenerator generates UI code from pipeline results using LLM.
type UIGenerator struct {
	llm    brain.LLMProvider
	router *brain.ModelRouter
}

// NewUIGenerator creates a new UIGenerator.
func NewUIGenerator(llm brain.LLMProvider, router *brain.ModelRouter) *UIGenerator {
	return &UIGenerator{llm: llm, router: router}
}

// Generate creates a GeneratedUI from a pipeline result.
func (g *UIGenerator) Generate(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities) (*GeneratedUI, error) {
	format := g.selectFormat(caps)
	prompt := g.buildPrompt(result, format, caps, nil, nil)

	code, err := g.generateWithRetry(ctx, prompt, format, 2)
	if err != nil {
		return nil, err
	}

	ui := &GeneratedUI{
		TaskID: result.TaskID,
		Format: format,
		Code:   code,
	}
	return ui, nil
}

// GenerateWithThought creates a GeneratedUI with ThoughtLog included.
func (g *UIGenerator) GenerateWithThought(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities, thought *ThoughtLog, hints []string) (*GeneratedUI, error) {
	format := g.selectFormat(caps)
	prompt := g.buildPrompt(result, format, caps, thought, hints)

	code, err := g.generateWithRetry(ctx, prompt, format, 2)
	if err != nil {
		return nil, err
	}

	ui := &GeneratedUI{
		TaskID:  result.TaskID,
		Format:  format,
		Code:    code,
		Thought: thought,
	}
	if thought != nil {
		ui.Meta.Summary = fmt.Sprintf("Completed in %dms", thought.TotalMs)
	}
	return ui, nil
}

// selectFormat picks the best format for the device.
func (g *UIGenerator) selectFormat(caps DeviceCapabilities) UIFormat {
	// If the device explicitly requests a format, use it.
	if caps.Format == FormatReact || caps.Format == FormatANSI || caps.Format == FormatMarkdown {
		return caps.Format
	}
	// For HTML-capable devices with JavaScript, prefer React for richer UI.
	// Plain HTML is always the safe default for web devices.
	return caps.Format
}

// buildPrompt assembles the LLM prompt for UI generation.
func (g *UIGenerator) buildPrompt(result pipeline.RunResult, format UIFormat, caps DeviceCapabilities, thought *ThoughtLog, hints []string) []brain.Message {
	var sysPrompt string
	switch format {
	case FormatANSI:
		sysPrompt = SystemPromptANSI
	case FormatHTML:
		sysPrompt = SystemPromptHTML
	case FormatReact:
		sysPrompt = SystemPromptReact
	default:
		sysPrompt = SystemPromptANSI
	}

	userContent := fmt.Sprintf("Task result (quality: %.0f%%):\n\n%s", result.QualityScore*100, result.Result)

	if thought != nil && len(thought.Stages) > 0 {
		userContent += "\n\nPipeline thought chain:\n"
		for _, s := range thought.Stages {
			userContent += fmt.Sprintf("  Stage %d (%s): %s [%dms]\n", s.Number, s.Name, s.Summary, s.DurMs)
		}
		userContent += "\nInclude a collapsible 'Thought Log' section in the UI."
	}

	for _, h := range hints {
		userContent += "\nUI HINT: " + h
	}

	userContent += fmt.Sprintf("\n\nDevice: %s, %dx%d", format, caps.Width, caps.Height)

	return []brain.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userContent},
	}
}

// generateWithRetry generates UI with self-healing: validate → retry → fallback.
func (g *UIGenerator) generateWithRetry(ctx context.Context, prompt []brain.Message, format UIFormat, maxRetries int) (string, error) {
	var lastErr string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		messages := make([]brain.Message, len(prompt))
		copy(messages, prompt)

		if lastErr != "" {
			messages = append(messages, brain.Message{
				Role:    "user",
				Content: fmt.Sprintf("The previous UI code had an error:\n%s\n\nFix it and regenerate.", lastErr),
			})
		}

		model := g.router.Select("simple", 100.0)
		resp, err := g.llm.Complete(ctx, brain.LLMRequest{
			Messages: messages,
			Model:    model,
		})
		if err != nil {
			return "", err
		}

		if validationErr := Validate(resp.Content, format); validationErr != nil {
			lastErr = validationErr.Error()
			continue
		}

		return resp.Content, nil
	}

	return "", fmt.Errorf("UI generation failed after %d attempts: %s", maxRetries+1, lastErr)
}
