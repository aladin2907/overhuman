package genui

import (
	"strings"
	"testing"
)

func TestWrapInSandbox_ContainsIframe(t *testing.T) {
	html := "<div>Hello World</div>"
	wrapped := WrapInSandbox(html)
	
	if !strings.Contains(wrapped, "<iframe") {
		t.Error("wrapped HTML should contain <iframe> tag")
	}
	if !strings.Contains(wrapped, `sandbox="allow-scripts"`) {
		t.Error("iframe should have sandbox='allow-scripts' attribute")
	}
}

func TestWrapInSandbox_EscapesHTML(t *testing.T) {
	html := `<div class="test">Hello & "World"</div>`
	wrapped := WrapInSandbox(html)
	
	// html.EscapeString converts < > & " to entities
	if strings.Contains(wrapped, `srcdoc="<div class="test"`) {
		t.Error("HTML inside srcdoc should be escaped")
	}
}

func TestWrapInSandbox_ContainsCSP(t *testing.T) {
	wrapped := WrapInSandbox("<div>test</div>")
	csp := GenerateCSP()
	
	if !strings.Contains(wrapped, csp) {
		t.Errorf("wrapped HTML should contain CSP: %s", csp)
	}
}

func TestWrapInSandbox_HasPostMessageListener(t *testing.T) {
	wrapped := WrapInSandbox("<div>test</div>")
	
	if !strings.Contains(wrapped, "addEventListener") {
		t.Error("wrapper should have message event listener")
	}
	// The wrapper listens for 'message' events from iframe (not postMessage — that's in the iframe content).
	if !strings.Contains(wrapped, "'message'") {
		t.Error("wrapper should listen for message events from iframe")
	}
}

func TestWrapInSandboxRaw_PreservesContent(t *testing.T) {
	html := `<div class="test"><p>Hello World</p></div>`
	wrapped := WrapInSandboxRaw(html)
	
	// Should contain the content (with " escaped to &quot;)
	if !strings.Contains(wrapped, "&quot;test&quot;") {
		t.Error("WrapInSandboxRaw should escape double quotes")
	}
	if !strings.Contains(wrapped, "<iframe") {
		t.Error("should contain iframe wrapper")
	}
}

func TestWrapInSandboxRaw_EscapesAmpersands(t *testing.T) {
	html := `<p>A &amp; B</p>`
	wrapped := WrapInSandboxRaw(html)
	
	// &amp; should become &amp;amp; (ampersand is escaped)
	if !strings.Contains(wrapped, "&amp;amp;") {
		t.Error("ampersands should be escaped for srcdoc")
	}
}

func TestSandboxAttributes(t *testing.T) {
	attrs := SandboxAttributes()
	if attrs != "allow-scripts" {
		t.Errorf("SandboxAttributes() = %q, want 'allow-scripts'", attrs)
	}
}

func TestValidateSandboxSafety_Clean(t *testing.T) {
	clean := `<div><script>console.log("hello")</script></div>`
	warnings := ValidateSandboxSafety(clean)
	if len(warnings) > 0 {
		t.Errorf("clean HTML should have no warnings, got %v", warnings)
	}
}

func TestValidateSandboxSafety_DetectsEscapeAttempts(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string // substring expected in one of the warnings
	}{
		{"allow-same-origin", `<iframe sandbox="allow-scripts allow-same-origin">`, "allow-same-origin"},
		{"document.cookie", `<script>var c = document.cookie;</script>`, "cookie"},
		{"localStorage", `<script>localStorage.setItem("x", "y");</script>`, "localStorage"},
		{"sessionStorage", `<script>sessionStorage.getItem("x");</script>`, "sessionStorage"},
		{"parent.document", `<script>parent.document.body</script>`, "parent document"},
		{"top.document", `<script>top.document.body</script>`, "top document"},
		{"document.domain", `<script>document.domain = "evil.com"</script>`, "document.domain"},
		{"indexedDB", `<script>var db = indexedDB.open("x")</script>`, "IndexedDB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateSandboxSafety(tt.code)
			if len(warnings) == 0 {
				t.Errorf("expected warnings for %s", tt.name)
				return
			}
			found := false
			for _, w := range warnings {
				if strings.Contains(strings.ToLower(w), strings.ToLower(tt.want)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("warnings %v should contain %q", warnings, tt.want)
			}
		})
	}
}

func TestGenerateCSP_Format(t *testing.T) {
	csp := GenerateCSP()
	
	required := []string{
		"default-src 'none'",
		"style-src 'unsafe-inline'",
		"script-src 'unsafe-inline'",
		"img-src data:",
	}
	
	for _, r := range required {
		if !strings.Contains(csp, r) {
			t.Errorf("CSP should contain %q, got: %s", r, csp)
		}
	}
}

func TestWrapInSandbox_NoAllowSameOrigin(t *testing.T) {
	wrapped := WrapInSandbox("<div>test</div>")
	
	// Sandbox must NOT include allow-same-origin (enables iframe to access parent).
	if strings.Contains(wrapped, "allow-same-origin") {
		t.Error("sandbox MUST NOT include allow-same-origin for security")
	}
}

func TestWrapInSandbox_NoAllowTopNavigation(t *testing.T) {
	wrapped := WrapInSandbox("<div>test</div>")
	
	if strings.Contains(wrapped, "allow-top-navigation") {
		t.Error("sandbox MUST NOT include allow-top-navigation")
	}
}
