package genui

import (
	"fmt"
	"io"
	"strings"
)

// CLIRenderer renders GeneratedUI to a terminal.
type CLIRenderer struct {
	out io.Writer
	in  io.Reader
}

// NewCLIRenderer creates a CLI renderer writing to out, reading from in.
func NewCLIRenderer(out io.Writer, in io.Reader) *CLIRenderer {
	return &CLIRenderer{out: out, in: in}
}

// Render outputs GeneratedUI to the terminal.
func (r *CLIRenderer) Render(ui *GeneratedUI) error {
	if ui == nil {
		return fmt.Errorf("nil UI")
	}

	switch ui.Format {
	case FormatANSI:
		return r.renderANSI(ui)
	case FormatMarkdown:
		return r.renderPlain(ui)
	default:
		return r.renderPlain(ui)
	}
}

// renderANSI outputs ANSI-formatted UI.
func (r *CLIRenderer) renderANSI(ui *GeneratedUI) error {
	sanitized := SanitizeANSI(ui.Code)
	_, err := fmt.Fprint(r.out, sanitized)
	if err != nil {
		return err
	}

	// Progressive disclosure: if summary exists, add expand hint
	if ui.Meta.Summary != "" {
		fmt.Fprintf(r.out, "\n\033[90m[d] Details  [t] Thought log\033[0m\n")
	}

	// Render actions as numbered options
	if len(ui.Actions) > 0 {
		fmt.Fprintln(r.out)
		for i, a := range ui.Actions {
			fmt.Fprintf(r.out, "  \033[36m[%d]\033[0m %s\n", i+1, a.Label)
		}
	}

	return nil
}

// renderPlain outputs plain text fallback.
func (r *CLIRenderer) renderPlain(ui *GeneratedUI) error {
	_, err := fmt.Fprint(r.out, ui.Code)
	return err
}

// RenderPlainText renders a raw string (fallback when UI generation fails).
func (r *CLIRenderer) RenderPlainText(text string) error {
	_, err := fmt.Fprint(r.out, text)
	return err
}

// WaitForAction reads user input and returns the selected action, or nil.
func (r *CLIRenderer) WaitForAction(ui *GeneratedUI) *GeneratedAction {
	if len(ui.Actions) == 0 {
		return nil
	}

	var input string
	_, err := fmt.Fscan(r.in, &input)
	if err != nil {
		return nil
	}

	input = strings.TrimSpace(input)

	// Check for numbered action
	for i, a := range ui.Actions {
		if input == fmt.Sprintf("%d", i+1) {
			return &a
		}
	}

	return nil
}

// RenderStream outputs UI chunks as they arrive.
func (r *CLIRenderer) RenderStream(chunks <-chan UIChunk) error {
	for chunk := range chunks {
		if chunk.Error != nil {
			return chunk.Error
		}
		if chunk.Content != "" {
			fmt.Fprint(r.out, chunk.Content)
		}
	}
	return nil
}
