package genui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestCLI_RenderANSI(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	ui := &GeneratedUI{
		TaskID: "test-1",
		Format: FormatANSI,
		Code:   "\033[1mHello\033[0m World",
	}

	err := r.Render(ui)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hello") {
		t.Errorf("expected output to contain 'Hello', got %q", output)
	}
	if !strings.Contains(output, "World") {
		t.Errorf("expected output to contain 'World', got %q", output)
	}
	// SGR sequences should be preserved
	if !strings.Contains(output, "\033[1m") {
		t.Errorf("expected bold SGR sequence to be preserved, got %q", output)
	}
}

func TestCLI_RenderANSI_Sanitized(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	// Include dangerous sequences: cursor home (\033[H), erase display (\033[2J),
	// cursor movement (\033[5A), along with safe SGR bold (\033[1m) and reset (\033[0m).
	ui := &GeneratedUI{
		TaskID: "test-sanitized",
		Format: FormatANSI,
		Code:   "\033[H\033[2J\033[1mSafe text\033[0m\033[5A",
	}

	err := r.Render(ui)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// Dangerous sequences must be removed
	if strings.Contains(output, "\033[H") {
		t.Errorf("cursor home sequence should be removed, got %q", output)
	}
	if strings.Contains(output, "\033[2J") {
		t.Errorf("erase display sequence should be removed, got %q", output)
	}
	if strings.Contains(output, "\033[5A") {
		t.Errorf("cursor up sequence should be removed, got %q", output)
	}
	// Safe SGR and text must remain
	if !strings.Contains(output, "\033[1m") {
		t.Errorf("bold SGR should be preserved, got %q", output)
	}
	if !strings.Contains(output, "Safe text") {
		t.Errorf("text content should be preserved, got %q", output)
	}
}

func TestCLI_RenderFallbackMarkdown(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	ui := &GeneratedUI{
		TaskID: "test-md",
		Format: FormatMarkdown,
		Code:   "# Title\n\nSome **bold** text",
	}

	err := r.Render(ui)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// Markdown format should render as plain text (no ANSI processing)
	if !strings.Contains(output, "# Title") {
		t.Errorf("expected markdown header preserved, got %q", output)
	}
	if !strings.Contains(output, "**bold**") {
		t.Errorf("expected markdown bold preserved, got %q", output)
	}
	// Code is output exactly as-is for markdown
	if output != ui.Code {
		t.Errorf("expected exact code output for markdown, got %q vs %q", output, ui.Code)
	}
}

func TestCLI_RenderWithActions(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	ui := &GeneratedUI{
		TaskID: "test-actions",
		Format: FormatANSI,
		Code:   "\033[1mPick one\033[0m",
		Actions: []GeneratedAction{
			{ID: "a1", Label: "Approve", Callback: "cb-approve"},
			{ID: "a2", Label: "Reject", Callback: "cb-reject"},
			{ID: "a3", Label: "Skip", Callback: "cb-skip"},
		},
	}

	err := r.Render(ui)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// All three numbered actions should appear
	if !strings.Contains(output, "[1]") {
		t.Errorf("expected [1] action number, got %q", output)
	}
	if !strings.Contains(output, "Approve") {
		t.Errorf("expected 'Approve' label, got %q", output)
	}
	if !strings.Contains(output, "[2]") {
		t.Errorf("expected [2] action number, got %q", output)
	}
	if !strings.Contains(output, "Reject") {
		t.Errorf("expected 'Reject' label, got %q", output)
	}
	if !strings.Contains(output, "[3]") {
		t.Errorf("expected [3] action number, got %q", output)
	}
	if !strings.Contains(output, "Skip") {
		t.Errorf("expected 'Skip' label, got %q", output)
	}
}

func TestCLI_RenderWithSummary(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	ui := &GeneratedUI{
		TaskID: "test-summary",
		Format: FormatANSI,
		Code:   "\033[1mResult\033[0m",
		Meta: UIMeta{
			Summary: "Completed in 42ms",
		},
	}

	err := r.Render(ui)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// Should display expand hints [d] and [t]
	if !strings.Contains(output, "[d]") {
		t.Errorf("expected [d] details hint, got %q", output)
	}
	if !strings.Contains(output, "[t]") {
		t.Errorf("expected [t] thought log hint, got %q", output)
	}
	if !strings.Contains(output, "Details") {
		t.Errorf("expected 'Details' label, got %q", output)
	}
	if !strings.Contains(output, "Thought log") {
		t.Errorf("expected 'Thought log' label, got %q", output)
	}
}

func TestCLI_RenderNilUI(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	err := r.Render(nil)
	if err == nil {
		t.Fatal("expected error for nil UI, got nil")
	}
	if !strings.Contains(err.Error(), "nil UI") {
		t.Errorf("expected 'nil UI' in error, got %q", err.Error())
	}
}

func TestCLI_HandleAction_ValidChoice(t *testing.T) {
	input := strings.NewReader("1\n")
	r := NewCLIRenderer(nil, input)

	ui := &GeneratedUI{
		Actions: []GeneratedAction{
			{ID: "a1", Label: "First", Callback: "cb1"},
			{ID: "a2", Label: "Second", Callback: "cb2"},
		},
	}

	action := r.WaitForAction(ui)
	if action == nil {
		t.Fatal("expected action, got nil")
	}
	if action.ID != "a1" {
		t.Errorf("expected action ID 'a1', got %q", action.ID)
	}
	if action.Label != "First" {
		t.Errorf("expected label 'First', got %q", action.Label)
	}
	if action.Callback != "cb1" {
		t.Errorf("expected callback 'cb1', got %q", action.Callback)
	}
}

func TestCLI_HandleAction_SecondChoice(t *testing.T) {
	input := strings.NewReader("2\n")
	r := NewCLIRenderer(nil, input)

	ui := &GeneratedUI{
		Actions: []GeneratedAction{
			{ID: "a1", Label: "First", Callback: "cb1"},
			{ID: "a2", Label: "Second", Callback: "cb2"},
			{ID: "a3", Label: "Third", Callback: "cb3"},
		},
	}

	action := r.WaitForAction(ui)
	if action == nil {
		t.Fatal("expected action, got nil")
	}
	if action.ID != "a2" {
		t.Errorf("expected action ID 'a2', got %q", action.ID)
	}
	if action.Label != "Second" {
		t.Errorf("expected label 'Second', got %q", action.Label)
	}
	if action.Callback != "cb2" {
		t.Errorf("expected callback 'cb2', got %q", action.Callback)
	}
}

func TestCLI_HandleAction_InvalidChoice(t *testing.T) {
	input := strings.NewReader("abc\n")
	r := NewCLIRenderer(nil, input)

	ui := &GeneratedUI{
		Actions: []GeneratedAction{
			{ID: "a1", Label: "First", Callback: "cb1"},
		},
	}

	action := r.WaitForAction(ui)
	if action != nil {
		t.Errorf("expected nil for invalid choice, got %+v", action)
	}
}

func TestCLI_HandleAction_NoActions(t *testing.T) {
	input := strings.NewReader("1\n")
	r := NewCLIRenderer(nil, input)

	ui := &GeneratedUI{
		Actions: nil,
	}

	action := r.WaitForAction(ui)
	if action != nil {
		t.Errorf("expected nil for no actions, got %+v", action)
	}
}

func TestCLI_RenderPlainText(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	text := "Raw fallback output\nWith newlines"
	err := r.RenderPlainText(text)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if output != text {
		t.Errorf("expected exact text output, got %q vs %q", output, text)
	}
}

func TestCLI_StreamRender(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	ch := make(chan UIChunk, 4)
	go func() {
		ch <- UIChunk{Content: "Hello "}
		ch <- UIChunk{Content: "streaming "}
		ch <- UIChunk{Content: "world"}
		ch <- UIChunk{Done: true}
		close(ch)
	}()

	err := r.RenderStream(ch)
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	expected := "Hello streaming world"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestCLI_StreamRender_WithError(t *testing.T) {
	var buf bytes.Buffer
	r := NewCLIRenderer(&buf, nil)

	streamErr := fmt.Errorf("LLM connection lost")
	ch := make(chan UIChunk, 3)
	go func() {
		ch <- UIChunk{Content: "partial "}
		ch <- UIChunk{Error: streamErr}
		ch <- UIChunk{Content: "should not appear"}
		close(ch)
	}()

	err := r.RenderStream(ch)
	if err == nil {
		t.Fatal("expected error from stream, got nil")
	}
	if err.Error() != "LLM connection lost" {
		t.Errorf("expected 'LLM connection lost', got %q", err.Error())
	}

	// Partial content before the error should have been written
	output := buf.String()
	if !strings.Contains(output, "partial") {
		t.Errorf("expected partial output before error, got %q", output)
	}
	// Content after error chunk should not appear
	if strings.Contains(output, "should not appear") {
		t.Errorf("content after error chunk should not be written, got %q", output)
	}
}
