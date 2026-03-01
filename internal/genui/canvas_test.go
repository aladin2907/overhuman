package genui

import (
	"strings"
	"testing"
)

func TestBuildCanvasHTML_ContainsLayout(t *testing.T) {
	layout := CanvasLayout{
		Canvas: "<div>Hello World</div>",
		Title:  "Test",
	}
	html := BuildCanvasHTML(layout)
	
	if html == "" {
		t.Fatal("should not return empty string for non-empty canvas")
	}
	if !strings.Contains(html, `class="layout"`) {
		t.Error("should contain layout class")
	}
	if !strings.Contains(html, `class="sidebar"`) {
		t.Error("should contain sidebar")
	}
	if !strings.Contains(html, `class="canvas"`) {
		t.Error("should contain canvas area")
	}
}

func TestBuildCanvasHTML_EmptyCanvas(t *testing.T) {
	layout := CanvasLayout{Canvas: ""}
	html := BuildCanvasHTML(layout)
	if html != "" {
		t.Error("empty canvas should return empty string")
	}
}

func TestBuildCanvasHTML_Title(t *testing.T) {
	layout := CanvasLayout{
		Canvas: "<div>test</div>",
		Title:  "My Dashboard",
	}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "<title>My Dashboard</title>") {
		t.Error("should contain the specified title")
	}
}

func TestBuildCanvasHTML_DefaultTitle(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>test</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "<title>Overhuman</title>") {
		t.Error("should use default title 'Overhuman'")
	}
}

func TestBuildCanvasHTML_DynamicExpand(t *testing.T) {
	layout := CanvasLayout{
		Canvas:        "<div>test</div>",
		DynamicExpand: true,
	}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "expanded") {
		t.Error("dynamic expand should add 'expanded' class")
	}
}

func TestBuildCanvasHTML_NoDynamicExpand(t *testing.T) {
	layout := CanvasLayout{
		Canvas:        "<div>test</div>",
		DynamicExpand: false,
	}
	html := BuildCanvasHTML(layout)
	// Should have layout class but NOT expanded class on the div
	if !strings.Contains(html, `class="layout"`) {
		t.Error("should have layout class")
	}
	// The CSS definition ".layout.expanded" exists, but the div should NOT have the expanded class.
	if strings.Contains(html, `class="layout expanded"`) {
		t.Error("should not have expanded class on layout div when DynamicExpand is false")
	}
}

func TestBuildCanvasHTML_ChatInput(t *testing.T) {
	layout := CanvasLayout{
		Canvas:    "<div>test</div>",
		ChatInput: true,
	}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, `class="chat-input"`) {
		t.Error("should contain chat input section")
	}
	if !strings.Contains(html, "chatInput") {
		t.Error("should contain chat input element")
	}
	if !strings.Contains(html, "sendMessage") {
		t.Error("should contain sendMessage function")
	}
}

func TestBuildCanvasHTML_NoChatInput(t *testing.T) {
	layout := CanvasLayout{
		Canvas:    "<div>test</div>",
		ChatInput: false,
	}
	html := BuildCanvasHTML(layout)
	if strings.Contains(html, `class="chat-input"`) {
		t.Error("should not contain chat input when disabled")
	}
}

func TestBuildCanvasHTML_CustomSidebar(t *testing.T) {
	layout := CanvasLayout{
		Canvas:  "<div>main</div>",
		Sidebar: `<div class="sidebar-section"><h2>Tasks</h2><ul><li>Task 1</li></ul></div>`,
	}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "Task 1") {
		t.Error("should contain custom sidebar content")
	}
}

func TestBuildCanvasHTML_DefaultSidebar(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>main</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "Overhuman") {
		t.Error("default sidebar should contain 'Overhuman'")
	}
}

func TestBuildCanvasHTML_DarkTheme(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>test</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "#0d1117") {
		t.Error("should use dark theme background color")
	}
	if !strings.Contains(html, "--accent: #00d4aa") {
		t.Error("should use accent color variable")
	}
}

func TestBuildCanvasHTML_ResponsiveCSS(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>test</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "@media (max-width: 768px)") {
		t.Error("should contain responsive media query")
	}
}

func TestBuildCanvasHTML_TouchOptimization(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>test</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "hover: none") {
		t.Error("should contain touch device media query")
	}
}

func TestBuildCanvasHTML_SidebarToggle(t *testing.T) {
	layout := CanvasLayout{Canvas: "<div>test</div>"}
	html := BuildCanvasHTML(layout)
	if !strings.Contains(html, "sidebar-toggle") {
		t.Error("should contain sidebar toggle button")
	}
}

func TestBuildCanvasForDevice_CLI(t *testing.T) {
	ui := &GeneratedUI{
		Code:   "\033[1mHello\033[0m",
		Format: FormatANSI,
	}
	result := BuildCanvasForDevice(ui, CLICapabilities())
	// CLI should return raw code, no canvas wrapper.
	if result != ui.Code {
		t.Errorf("CLI should return raw code, got %q", result)
	}
}

func TestBuildCanvasForDevice_Web(t *testing.T) {
	ui := &GeneratedUI{
		Code:   "<div>web content</div>",
		Format: FormatHTML,
	}
	result := BuildCanvasForDevice(ui, WebCapabilities(1280, 800))
	if !strings.Contains(result, `class="layout"`) {
		t.Error("web device should get canvas layout")
	}
	if !strings.Contains(result, "web content") {
		t.Error("should contain the UI code")
	}
}

func TestBuildCanvasForDevice_MobileAutoExpand(t *testing.T) {
	ui := &GeneratedUI{
		Code:   "<div>mobile</div>",
		Format: FormatHTML,
	}
	caps := WebCapabilities(375, 812) // Mobile
	result := BuildCanvasForDevice(ui, caps)
	if !strings.Contains(result, "expanded") {
		t.Error("narrow mobile device should auto-expand canvas")
	}
}

func TestBuildCanvasForDevice_Tablet(t *testing.T) {
	ui := &GeneratedUI{
		Code:   "<div>tablet</div>",
		Format: FormatHTML,
	}
	caps := TabletCapabilities(768, 1024)
	result := BuildCanvasForDevice(ui, caps)
	if !strings.Contains(result, `class="layout`) {
		t.Error("tablet should get canvas layout")
	}
	// Tablet < 1024 width + touch → dynamic expand
	if !strings.Contains(result, "expanded") {
		t.Error("narrow tablet should auto-expand")
	}
}

func TestBuildCanvasForDevice_Nil(t *testing.T) {
	result := BuildCanvasForDevice(nil, WebCapabilities(1280, 800))
	if result != "" {
		t.Error("nil UI should return empty string")
	}
}

func TestBuildCanvasForDevice_EmptyCode(t *testing.T) {
	ui := &GeneratedUI{Code: ""}
	result := BuildCanvasForDevice(ui, WebCapabilities(1280, 800))
	if result != "" {
		t.Error("empty code should return empty string")
	}
}

func TestBuildCanvasForDevice_Markdown(t *testing.T) {
	ui := &GeneratedUI{
		Code:   "# Hello\n\nWorld",
		Format: FormatMarkdown,
	}
	result := BuildCanvasForDevice(ui, DeviceCapabilities{Format: FormatMarkdown})
	// Markdown should return raw code, no canvas.
	if result != ui.Code {
		t.Errorf("markdown should return raw code")
	}
}
