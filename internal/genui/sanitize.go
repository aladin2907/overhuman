package genui

import (
	"fmt"
	"regexp"
	"strings"
)

// ansiSGR matches SGR (Select Graphic Rendition) sequences: \033[...m
// These are safe: colors, bold, italic, underline, reset.
var ansiSGR = regexp.MustCompile(`\x1b\[\d+(;\d+)*m`)

// ansiAny matches any ANSI escape sequence.
var ansiAny = regexp.MustCompile(`\x1b\[[\x20-\x3f]*[\x40-\x7e]`)

// dangerousANSI lists ANSI sequence types to block.
// H=cursor home, J=erase display, K=erase line, A-D=cursor move,
// r=scroll region, s/u=cursor save/restore, S/T=scroll up/down.
var dangerousANSI = regexp.MustCompile(`\x1b\[\d*[HJKABCDEFGrsuST]`)

// SanitizeANSI removes dangerous ANSI escape sequences, keeping only SGR (colors/formatting).
func SanitizeANSI(code string) string {
	// Replace all ANSI sequences: keep SGR, remove everything else.
	result := ansiAny.ReplaceAllStringFunc(code, func(seq string) string {
		if ansiSGR.MatchString(seq) {
			return seq // safe: SGR color/formatting
		}
		return "" // dangerous: cursor movement, screen clear, etc.
	})
	return result
}

// networkPatterns are JS patterns that indicate network access.
var networkPatterns = []string{
	"fetch(", "fetch (", "XMLHttpRequest", "new WebSocket",
	"navigator.sendBeacon", "EventSource",
	".ajax(", "$.get(", "$.post(",
}

// navigationPatterns are JS patterns for navigation/popup.
var navigationPatterns = []string{
	"window.open", "window.location", "document.location",
	"top.location", "parent.location",
}

// formActionPattern matches <form action="http..."> tags.
var formActionPattern = regexp.MustCompile(`(?i)<form[^>]*action\s*=\s*["']https?://`)

// SanitizeHTML removes dangerous patterns from generated HTML.
func SanitizeHTML(code string) string {
	result := code

	// Remove network access patterns
	for _, p := range networkPatterns {
		result = strings.ReplaceAll(result, p, "/* BLOCKED: "+p+" */")
	}

	// Remove navigation patterns
	for _, p := range navigationPatterns {
		result = strings.ReplaceAll(result, p, "/* BLOCKED: "+p+" */")
	}

	// Remove form actions pointing to external URLs
	result = formActionPattern.ReplaceAllString(result, `<form action="about:blank"`)

	return result
}

// Validate checks generated UI code for errors.
func Validate(code string, format UIFormat) error {
	if code == "" {
		return fmt.Errorf("empty UI code")
	}

	switch format {
	case FormatANSI:
		return validateANSI(code)
	case FormatHTML:
		return validateHTML(code)
	default:
		return nil
	}
}

// validateANSI checks ANSI output for unclosed escape sequences.
func validateANSI(code string) error {
	// Check for unclosed ANSI color sequences: every open must have a reset.
	opens := strings.Count(code, "\033[")
	resets := strings.Count(code, "\033[0m")
	// Allow some tolerance — not every open needs a reset (nested colors),
	// but if there are opens with zero resets, that's suspicious.
	if opens > 0 && resets == 0 {
		return fmt.Errorf("ANSI output has %d escape sequences but no reset (\\033[0m)", opens)
	}
	return nil
}

// validateHTML checks HTML output for basic structural validity.
func validateHTML(code string) error {
	lower := strings.ToLower(code)
	if !strings.Contains(lower, "<") {
		return fmt.Errorf("HTML output contains no tags")
	}
	// Check for basic tag balance
	opens := strings.Count(lower, "<div")
	closes := strings.Count(lower, "</div")
	if opens > 0 && closes == 0 {
		return fmt.Errorf("HTML has %d <div> but no </div>", opens)
	}
	return nil
}

// GenerateCSP returns the Content-Security-Policy for sandboxed iframe.
func GenerateCSP() string {
	return "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:;"
}
