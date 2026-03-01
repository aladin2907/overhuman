package genui

import (
	"fmt"
	"strings"
)

// CanvasLayout defines the web UI layout structure with sidebar + main canvas.
type CanvasLayout struct {
	Sidebar       string // HTML for sidebar (task list, status, health)
	Canvas        string // Main content area (generated UI)
	ChatInput     bool   // Show bottom chat input
	DynamicExpand bool   // Collapse sidebar, canvas takes 100%
	Title         string // Page title
}

// BuildCanvasHTML generates the full HTML page with canvas layout.
// For CLI devices, it returns just the canvas content (no layout wrapper).
func BuildCanvasHTML(layout CanvasLayout) string {
	if layout.Canvas == "" {
		return ""
	}

	title := layout.Title
	if title == "" {
		title = "Overhuman"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
:root {
  --bg-primary: #0d1117;
  --bg-secondary: #161b22;
  --bg-tertiary: #21262d;
  --text-primary: #e6edf3;
  --text-secondary: #8b949e;
  --accent: #00d4aa;
  --accent-hover: #00f0c0;
  --border: #30363d;
  --error: #f85149;
  --warning: #d29922;
  --success: #3fb950;
  --sidebar-width: 260px;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { height: 100%%; background: var(--bg-primary); color: var(--text-primary); font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif; }

.layout {
  display: grid;
  grid-template-columns: var(--sidebar-width) 1fr;
  grid-template-rows: 1fr auto;
  height: 100vh;
  gap: 0;
}

.sidebar {
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  padding: 16px;
  overflow-y: auto;
  grid-row: 1 / -1;
}
.sidebar h2 {
  font-size: 14px;
  font-weight: 600;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 12px;
}
.sidebar-section {
  margin-bottom: 20px;
}

.canvas {
  overflow-y: auto;
  padding: 0;
  position: relative;
}
.canvas iframe {
  width: 100%%;
  height: 100%%;
  border: none;
}

.chat-input {
  grid-column: 2;
  background: var(--bg-secondary);
  border-top: 1px solid var(--border);
  padding: 12px 16px;
  display: flex;
  gap: 8px;
  align-items: center;
}
.chat-input input {
  flex: 1;
  background: var(--bg-tertiary);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
}
.chat-input input:focus {
  border-color: var(--accent);
}
.chat-input button {
  background: var(--accent);
  color: var(--bg-primary);
  border: none;
  border-radius: 8px;
  padding: 10px 18px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s;
}
.chat-input button:hover { background: var(--accent-hover); }

.sidebar-toggle {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: 100;
  background: var(--bg-tertiary);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  padding: 6px 10px;
  cursor: pointer;
  display: none;
  font-size: 18px;
}

/* Dynamic expand: sidebar collapsed */
.layout.expanded {
  grid-template-columns: 1fr;
}
.layout.expanded .sidebar { display: none; }
.layout.expanded .sidebar-toggle { display: block; }
.layout.expanded .chat-input { grid-column: 1; }

/* Responsive: single column on mobile */
@media (max-width: 768px) {
  .layout {
    grid-template-columns: 1fr;
  }
  .sidebar { display: none; }
  .sidebar-toggle { display: block; }
  .chat-input { grid-column: 1; }
}

/* Touch device optimizations */
@media (hover: none) and (pointer: coarse) {
  .chat-input input { font-size: 16px; padding: 12px 16px; }
  .chat-input button { padding: 12px 20px; min-height: 44px; }
}
</style>
</head>
<body>
`, title))

	expandClass := ""
	if layout.DynamicExpand {
		expandClass = " expanded"
	}
	b.WriteString(fmt.Sprintf(`<div class="layout%s">`, expandClass))

	// Sidebar toggle button (visible when sidebar is collapsed or on mobile).
	b.WriteString(`<button class="sidebar-toggle" onclick="document.querySelector('.layout').classList.toggle('expanded')">☰</button>`)

	// Sidebar.
	b.WriteString(`<div class="sidebar">`)
	if layout.Sidebar != "" {
		b.WriteString(layout.Sidebar)
	} else {
		b.WriteString(`<div class="sidebar-section"><h2>Overhuman</h2><p style="color:var(--text-secondary);font-size:13px;">Self-evolving AI assistant</p></div>`)
	}
	b.WriteString(`</div>`)

	// Main canvas.
	b.WriteString(`<div class="canvas">`)
	b.WriteString(layout.Canvas)
	b.WriteString(`</div>`)

	// Chat input (optional).
	if layout.ChatInput {
		b.WriteString(`<div class="chat-input">
<input type="text" id="chatInput" placeholder="Ask anything..." onkeydown="if(event.key==='Enter')sendMessage()">
<button onclick="sendMessage()">Send</button>
</div>
<script>
function sendMessage() {
  var input = document.getElementById('chatInput');
  var text = input.value.trim();
  if (!text) return;
  input.value = '';
  if (window._overhuman_ws && window._overhuman_ws.readyState === 1) {
    window._overhuman_ws.send(JSON.stringify({type:'input',payload:{text:text}}));
  }
}
</script>`)
	}

	b.WriteString(`</div></body></html>`)
	return b.String()
}

// BuildCanvasForDevice creates appropriate canvas layout based on device capabilities.
func BuildCanvasForDevice(ui *GeneratedUI, caps DeviceCapabilities) string {
	if ui == nil || ui.Code == "" {
		return ""
	}

	// CLI devices don't use canvas layout.
	if caps.Format == FormatANSI || caps.Format == FormatMarkdown {
		return ui.Code
	}

	layout := CanvasLayout{
		Canvas:    ui.Code,
		ChatInput: caps.Interactive,
		Title:     ui.Meta.Title,
	}

	// If the device is narrow (mobile), use dynamic expand by default.
	if caps.Width > 0 && caps.Width < 768 {
		layout.DynamicExpand = true
	}

	// Tablet devices get touch-optimized layout.
	if caps.TouchScreen {
		layout.DynamicExpand = caps.Width < 1024
	}

	return BuildCanvasHTML(layout)
}
