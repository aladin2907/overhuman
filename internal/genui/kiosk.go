package genui

import (
	"fmt"
	"net/http"
	"strings"
)

// KioskConfig configures the kiosk web application.
type KioskConfig struct {
	// WSAddr is the WebSocket server address to connect to (e.g., "127.0.0.1:9091").
	WSAddr string

	// Title is displayed in the browser tab and kiosk header.
	Title string

	// DarkMode enables the dark color scheme (default: true).
	DarkMode bool

	// ShowSidebar shows the sidebar panel on start (default: true).
	ShowSidebar bool

	// TouchMode enables larger touch targets for tablet use (default: auto-detect).
	TouchMode bool

	// EmergencyStop shows the emergency stop button (default: true).
	EmergencyStop bool
}

// DefaultKioskConfig returns the default kiosk configuration.
func DefaultKioskConfig() KioskConfig {
	return KioskConfig{
		WSAddr:        "127.0.0.1:9091",
		Title:         "Overhuman",
		DarkMode:      true,
		ShowSidebar:   true,
		TouchMode:     false, // auto-detect in JS
		EmergencyStop: true,
	}
}

// KioskHandler serves the kiosk web application.
type KioskHandler struct {
	config KioskConfig
	html   string // cached rendered HTML
}

// NewKioskHandler creates a new kiosk handler with the given configuration.
func NewKioskHandler(cfg KioskConfig) *KioskHandler {
	h := &KioskHandler{config: cfg}
	h.html = h.renderHTML()
	return h
}

// ServeHTTP serves the kiosk single-page application.
func (h *KioskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only serve the root path and /kiosk.
	path := r.URL.Path
	if path != "/" && path != "/kiosk" && path != "/kiosk/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, h.html)
}

// RegisterRoutes registers kiosk routes on the given ServeMux.
func (h *KioskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/", h)
	mux.Handle("/kiosk", h)
	mux.Handle("/kiosk/", h)
}

// renderHTML generates the full kiosk HTML page with injected configuration.
func (h *KioskHandler) renderHTML() string {
	html := KioskHTML

	// Inject configuration.
	wsProtocol := "ws"
	wsURL := fmt.Sprintf("%s://%s/ws", wsProtocol, h.config.WSAddr)

	html = strings.ReplaceAll(html, "{{WS_URL}}", wsURL)
	html = strings.ReplaceAll(html, "{{TITLE}}", h.config.Title)
	html = strings.ReplaceAll(html, "{{TITLE2}}", h.config.Title)
	html = strings.ReplaceAll(html, "{{DARK_MODE}}", boolStr(h.config.DarkMode))
	html = strings.ReplaceAll(html, "{{SHOW_SIDEBAR}}", boolStr(h.config.ShowSidebar))
	html = strings.ReplaceAll(html, "{{TOUCH_MODE}}", boolStr(h.config.TouchMode))
	html = strings.ReplaceAll(html, "{{EMERGENCY_STOP}}", boolStr(h.config.EmergencyStop))

	return html
}

// boolStr converts a bool to a JavaScript string "true" or "false".
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
