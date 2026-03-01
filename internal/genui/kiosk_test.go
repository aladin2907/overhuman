package genui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// KioskConfig & defaults
// ---------------------------------------------------------------------------

func TestDefaultKioskConfig(t *testing.T) {
	cfg := DefaultKioskConfig()

	if cfg.WSAddr != "127.0.0.1:9091" {
		t.Errorf("WSAddr = %q, want %q", cfg.WSAddr, "127.0.0.1:9091")
	}
	if cfg.Title != "Overhuman" {
		t.Errorf("Title = %q, want %q", cfg.Title, "Overhuman")
	}
	if !cfg.DarkMode {
		t.Error("DarkMode = false, want true")
	}
	if !cfg.ShowSidebar {
		t.Error("ShowSidebar = false, want true")
	}
	if cfg.TouchMode {
		t.Error("TouchMode = true, want false")
	}
	if !cfg.EmergencyStop {
		t.Error("EmergencyStop = false, want true")
	}
}

func TestKioskConfigCustom(t *testing.T) {
	cfg := KioskConfig{
		WSAddr:        "10.0.0.1:8080",
		Title:         "Custom Title",
		DarkMode:      false,
		ShowSidebar:   false,
		TouchMode:     true,
		EmergencyStop: false,
	}

	if cfg.WSAddr != "10.0.0.1:8080" {
		t.Errorf("WSAddr = %q, want %q", cfg.WSAddr, "10.0.0.1:8080")
	}
	if cfg.Title != "Custom Title" {
		t.Errorf("Title = %q, want %q", cfg.Title, "Custom Title")
	}
	if cfg.DarkMode {
		t.Error("DarkMode = true, want false")
	}
	if cfg.ShowSidebar {
		t.Error("ShowSidebar = true, want false")
	}
	if !cfg.TouchMode {
		t.Error("TouchMode = false, want true")
	}
	if cfg.EmergencyStop {
		t.Error("EmergencyStop = true, want false")
	}
}

// ---------------------------------------------------------------------------
// KioskHandler construction
// ---------------------------------------------------------------------------

func TestNewKioskHandler(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if h == nil {
		t.Fatal("NewKioskHandler returned nil")
	}
	if h.html == "" {
		t.Error("rendered html is empty")
	}
}

func TestKioskHandler_ConfigInjected(t *testing.T) {
	cfg := KioskConfig{
		WSAddr:        "192.168.1.1:7777",
		Title:         "TestDashboard",
		DarkMode:      true,
		ShowSidebar:   true,
		TouchMode:     false,
		EmergencyStop: true,
	}
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "ws://192.168.1.1:7777/ws") {
		t.Error("rendered HTML does not contain expected WS URL")
	}
	if !strings.Contains(h.html, "TestDashboard") {
		t.Error("rendered HTML does not contain injected title")
	}
	if !strings.Contains(h.html, "darkMode: true") {
		t.Error("rendered HTML does not contain darkMode: true")
	}
	if !strings.Contains(h.html, "showSidebar: true") {
		t.Error("rendered HTML does not contain showSidebar: true")
	}
	if !strings.Contains(h.html, "touchMode: false") {
		t.Error("rendered HTML does not contain touchMode: false")
	}
	if !strings.Contains(h.html, "emergencyStop: true") {
		t.Error("rendered HTML does not contain emergencyStop: true")
	}
}

// ---------------------------------------------------------------------------
// HTTP serving
// ---------------------------------------------------------------------------

func TestKioskHandler_ServeHTTP_Root(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestKioskHandler_ServeHTTP_Kiosk(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	req := httptest.NewRequest(http.MethodGet, "/kiosk", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestKioskHandler_ServeHTTP_KioskSlash(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	req := httptest.NewRequest(http.MethodGet, "/kiosk/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestKioskHandler_ServeHTTP_NotFound(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestKioskHandler_SecurityHeaders(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	headers := map[string]string{
		"Cache-Control":       "no-cache, no-store, must-revalidate",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
	}
	for name, want := range headers {
		got := rec.Header().Get(name)
		if got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes
// ---------------------------------------------------------------------------

func TestKioskHandler_RegisterRoutes(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	paths := []string{"/", "/kiosk", "/kiosk/"}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want %d", p, rec.Code, http.StatusOK)
		}
	}
}

// ---------------------------------------------------------------------------
// HTML Content Validation
// ---------------------------------------------------------------------------

func TestKioskHTML_HasDoctype(t *testing.T) {
	if !strings.HasPrefix(KioskHTML, "<!DOCTYPE html>") {
		t.Error("KioskHTML does not start with <!DOCTYPE html>")
	}
}

func TestKioskHTML_HasTemplateVars(t *testing.T) {
	vars := []string{
		"{{WS_URL}}",
		"{{TITLE}}",
		"{{TITLE2}}",
		"{{DARK_MODE}}",
		"{{SHOW_SIDEBAR}}",
		"{{TOUCH_MODE}}",
		"{{EMERGENCY_STOP}}",
	}
	for _, v := range vars {
		if !strings.Contains(KioskHTML, v) {
			t.Errorf("KioskHTML missing template variable %s", v)
		}
	}
}

func TestKioskHTML_NoTemplateVarsAfterRender(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if strings.Contains(h.html, "{{") {
		t.Error("rendered HTML still contains {{ placeholder")
	}
	if strings.Contains(h.html, "}}") {
		t.Error("rendered HTML still contains }} placeholder")
	}
}

func TestKioskHTML_HasWebSocketClient(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "WebSocket") {
		t.Error("rendered HTML does not contain WebSocket")
	}
}

func TestKioskHTML_HasSandboxIframe(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "sandbox") {
		t.Error("rendered HTML does not contain sandbox")
	}
	if !strings.Contains(h.html, "allow-scripts") {
		t.Error("rendered HTML does not contain allow-scripts")
	}
}

func TestKioskHTML_HasDarkTheme(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	vars := []string{"--bg-primary", "--accent", "--danger"}
	for _, v := range vars {
		if !strings.Contains(h.html, v) {
			t.Errorf("rendered HTML does not contain CSS variable %s", v)
		}
	}
}

func TestKioskHTML_HasEmergencyStop(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "Stop") {
		t.Error("rendered HTML does not contain Stop button text")
	}
	if !strings.Contains(h.html, "cancel") {
		t.Error("rendered HTML does not contain cancel message type")
	}
}

func TestKioskHTML_HasChatInput(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "chat-field") {
		t.Error("rendered HTML does not contain chat-field input element")
	}
}

func TestKioskHTML_HasOfflineCache(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "localStorage") {
		t.Error("rendered HTML does not contain localStorage references")
	}
}

func TestKioskHTML_HasConnectionStatus(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "conn-dot") {
		t.Error("rendered HTML does not contain conn-dot")
	}
	if !strings.Contains(h.html, "connected") {
		t.Error("rendered HTML does not contain connection status class")
	}
}

func TestKioskHTML_HasTaskHistory(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "task-list") {
		t.Error("rendered HTML does not contain task-list")
	}
	if !strings.Contains(h.html, "taskHistory") {
		t.Error("rendered HTML does not contain taskHistory references")
	}
}

func TestKioskHTML_HasFeedback(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "ui_feedback") {
		t.Error("rendered HTML does not contain ui_feedback message type")
	}
}

func TestKioskHTML_HasPostMessageBridge(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "postMessage") {
		t.Error("rendered HTML does not contain postMessage")
	}
	if !strings.Contains(h.html, "iframe_action") {
		t.Error("rendered HTML does not contain iframe_action")
	}
}

func TestKioskHTML_HasReconnect(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "reconnectDelay") {
		t.Error("rendered HTML does not contain reconnectDelay")
	}
	if !strings.Contains(h.html, "reconnectMax") {
		t.Error("rendered HTML does not contain reconnectMax")
	}
}

func TestKioskHTML_HasPingKeepalive(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "pingInterval") {
		t.Error("rendered HTML does not contain pingInterval")
	}
}

func TestKioskHTML_HasResponsiveLayout(t *testing.T) {
	h := NewKioskHandler(DefaultKioskConfig())
	if !strings.Contains(h.html, "@media") {
		t.Error("rendered HTML does not contain @media CSS query")
	}
}

// ---------------------------------------------------------------------------
// boolStr helper
// ---------------------------------------------------------------------------

func TestBoolStr_True(t *testing.T) {
	if got := boolStr(true); got != "true" {
		t.Errorf("boolStr(true) = %q, want %q", got, "true")
	}
}

func TestBoolStr_False(t *testing.T) {
	if got := boolStr(false); got != "false" {
		t.Errorf("boolStr(false) = %q, want %q", got, "false")
	}
}

// ---------------------------------------------------------------------------
// Template injection
// ---------------------------------------------------------------------------

func TestKioskHandler_CustomWSAddr(t *testing.T) {
	cfg := DefaultKioskConfig()
	cfg.WSAddr = "10.20.30.40:5555"
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "ws://10.20.30.40:5555/ws") {
		t.Error("rendered HTML does not contain custom WS address")
	}
}

func TestKioskHandler_CustomTitle(t *testing.T) {
	cfg := DefaultKioskConfig()
	cfg.Title = "MyKiosk"
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "<title>MyKiosk</title>") {
		t.Error("rendered HTML does not contain custom title in <title> tag")
	}
}

func TestKioskHandler_DarkModeDisabled(t *testing.T) {
	cfg := DefaultKioskConfig()
	cfg.DarkMode = false
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "darkMode: false") {
		t.Error("rendered HTML does not contain darkMode: false")
	}
}

func TestKioskHandler_SidebarHidden(t *testing.T) {
	cfg := DefaultKioskConfig()
	cfg.ShowSidebar = false
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "showSidebar: false") {
		t.Error("rendered HTML does not contain showSidebar: false")
	}
}

func TestKioskHandler_TouchModeEnabled(t *testing.T) {
	cfg := DefaultKioskConfig()
	cfg.TouchMode = true
	h := NewKioskHandler(cfg)

	if !strings.Contains(h.html, "touchMode: true") {
		t.Error("rendered HTML does not contain touchMode: true")
	}
}
