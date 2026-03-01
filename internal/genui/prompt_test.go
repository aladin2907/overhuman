package genui

import (
	"strings"
	"testing"
)

func TestANSIPrompt_ContainsFormattingRules(t *testing.T) {
	required := []string{
		"ANSI escape codes",
		"\\033[1m",
		"\\033[0m",
		"box drawing",
	}
	for _, r := range required {
		if !strings.Contains(SystemPromptANSI, r) {
			t.Errorf("ANSI prompt should contain %q", r)
		}
	}
}

func TestANSIPrompt_OutputFormat(t *testing.T) {
	if !strings.Contains(SystemPromptANSI, "RESPOND WITH ONLY THE ANSI TEXT") {
		t.Error("ANSI prompt should instruct to respond with only ANSI text")
	}
}

func TestHTMLPrompt_OutputFormat(t *testing.T) {
	if !strings.Contains(SystemPromptHTML, "RESPOND WITH ONLY THE HTML CODE") {
		t.Error("HTML prompt should instruct to respond with only HTML code")
	}
}

func TestReactPrompt_OutputFormat(t *testing.T) {
	if !strings.Contains(SystemPromptReact, "RESPOND WITH ONLY THE JSX CODE") {
		t.Error("React prompt should instruct to respond with only JSX code")
	}
}

func TestHTMLPrompt_DarkThemeColors(t *testing.T) {
	colors := []string{"#1a1a2e", "#e0e0e0", "#00d4aa"}
	for _, c := range colors {
		if !strings.Contains(SystemPromptHTML, c) {
			t.Errorf("HTML prompt should contain dark theme color %s", c)
		}
	}
}

func TestReactPrompt_DarkThemeClasses(t *testing.T) {
	classes := []string{"bg-gray-900", "text-gray-100"}
	for _, c := range classes {
		if !strings.Contains(SystemPromptReact, c) {
			t.Errorf("React prompt should contain Tailwind class %s", c)
		}
	}
}

func TestHTMLPrompt_SecurityRules(t *testing.T) {
	if !strings.Contains(SystemPromptHTML, "NO fetch") || !strings.Contains(SystemPromptHTML, "XMLHttpRequest") {
		t.Error("HTML prompt should forbid network access")
	}
	if !strings.Contains(SystemPromptHTML, "NO external dependencies") {
		t.Error("HTML prompt should forbid external deps")
	}
}

func TestReactPrompt_OnActionPattern(t *testing.T) {
	if !strings.Contains(SystemPromptReact, "onAction") {
		t.Error("React prompt should mention onAction callback")
	}
}

func TestHTMLPrompt_PostMessageCallbackPattern(t *testing.T) {
	if !strings.Contains(SystemPromptHTML, "window.parent.postMessage") {
		t.Error("HTML prompt should contain postMessage pattern")
	}
}
