package genui

import (
	"fmt"
	"html"
	"strings"
)

// WrapInSandbox wraps generated HTML code in a sandboxed iframe.
// The sandbox prevents network access, navigation, and DOM access to parent.
// Only inline scripts and styles are allowed.
func WrapInSandbox(htmlCode string) string {
	csp := GenerateCSP()
	escaped := html.EscapeString(htmlCode)

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Overhuman UI</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0d1117; overflow: hidden; }
iframe { width: 100%%; height: 100vh; border: none; }
</style>
</head>
<body>
<iframe
  sandbox="allow-scripts"
  srcdoc="%s"
  csp="%s"
  style="width:100%%;height:100vh;border:none;"
></iframe>
<script>
// Listen for action callbacks from sandboxed iframe.
window.addEventListener('message', function(e) {
  if (e.data && e.data.action) {
    // Forward to WebSocket connection if available.
    if (window._overhuman_ws && window._overhuman_ws.readyState === 1) {
      window._overhuman_ws.send(JSON.stringify({
        type: 'action',
        payload: e.data
      }));
    }
  }
});
</script>
</body>
</html>`, escaped, csp)
}

// WrapInSandboxRaw wraps HTML without escaping (when HTML is already safe/sanitized).
func WrapInSandboxRaw(htmlCode string) string {
	csp := GenerateCSP()
	// Use a different approach: base64 or direct srcdoc with careful quoting.
	// For srcdoc, we need to escape double quotes and ampersands.
	safe := strings.ReplaceAll(htmlCode, "&", "&amp;")
	safe = strings.ReplaceAll(safe, "\"", "&quot;")

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Overhuman UI</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0d1117; overflow: hidden; }
iframe { width: 100%%; height: 100vh; border: none; }
</style>
</head>
<body>
<iframe
  sandbox="allow-scripts"
  srcdoc="%s"
  csp="%s"
  style="width:100%%;height:100vh;border:none;"
></iframe>
<script>
window.addEventListener('message', function(e) {
  if (e.data && e.data.action) {
    if (window._overhuman_ws && window._overhuman_ws.readyState === 1) {
      window._overhuman_ws.send(JSON.stringify({
        type: 'action',
        payload: e.data
      }));
    }
  }
});
</script>
</body>
</html>`, safe, csp)
}

// SandboxAttributes returns the recommended iframe sandbox attribute value.
func SandboxAttributes() string {
	return "allow-scripts"
}

// ValidateSandboxSafety checks that HTML code doesn't try to escape the sandbox.
func ValidateSandboxSafety(code string) []string {
	var warnings []string
	lower := strings.ToLower(code)

	// Check for sandbox escape attempts.
	escapePatterns := []struct {
		pattern string
		warning string
	}{
		{"allow-same-origin", "attempts to set allow-same-origin (sandbox escape)"},
		{"allow-top-navigation", "attempts to set allow-top-navigation"},
		{"allow-popups", "attempts to set allow-popups"},
		{"document.domain", "attempts to modify document.domain"},
		{"parent.document", "attempts to access parent document"},
		{"top.document", "attempts to access top document"},
		{"window.parent.document", "attempts to access parent document via window"},
		{"document.cookie", "attempts to access cookies"},
		{"localstorage", "attempts to access localStorage"},
		{"sessionstorage", "attempts to access sessionStorage"},
		{"indexeddb", "attempts to access IndexedDB"},
	}

	for _, p := range escapePatterns {
		if strings.Contains(lower, p.pattern) {
			warnings = append(warnings, p.warning)
		}
	}

	return warnings
}
