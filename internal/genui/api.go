package genui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// UIAPIHandler provides REST API endpoints for UI generation.
type UIAPIHandler struct {
	generator *UIGenerator
	wsServer  *WSServer
	mu        sync.RWMutex
	cache     map[string]*GeneratedUI // keyed by task_id
}

// NewUIAPIHandler creates a new UI API handler.
func NewUIAPIHandler(gen *UIGenerator, ws *WSServer) *UIAPIHandler {
	return &UIAPIHandler{
		generator: gen,
		wsServer:  ws,
		cache:     make(map[string]*GeneratedUI),
	}
}

// RegisterRoutes registers UI API routes on the given ServeMux.
// Routes: POST /api/ui/generate, GET /api/ui/{task_id}, GET /api/ui/ws/status
func (h *UIAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ui/generate", h.handleGenerate)
	mux.HandleFunc("GET /api/ui/last", h.handleGetLast)
	mux.HandleFunc("GET /api/ui/ws/status", h.handleWSStatus)
}

// apiGenerateRequest is the JSON body for POST /api/ui/generate.
type apiGenerateRequest struct {
	TaskID       string            `json:"task_id"`
	Result       string            `json:"result"`
	QualityScore float64           `json:"quality_score"`
	Caps         *DeviceCapabilities `json:"caps,omitempty"`
}

// apiGenerateResponse is the JSON body returned from POST /api/ui/generate.
type apiGenerateResponse struct {
	UI    *GeneratedUI `json:"ui,omitempty"`
	Error string       `json:"error,omitempty"`
}

// apiWSStatusResponse describes WebSocket server status.
type apiWSStatusResponse struct {
	Clients    int    `json:"clients"`
	Addr       string `json:"addr"`
	HasCachedUI bool  `json:"has_cached_ui"`
}

// handleGenerate handles POST /api/ui/generate.
func (h *UIAPIHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req apiGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Result == "" {
		writeJSONError(w, "result is required", http.StatusBadRequest)
		return
	}

	caps := WebCapabilities(1280, 800)
	if req.Caps != nil {
		caps = *req.Caps
	}

	// Build a minimal pipeline.RunResult for generation.
	// We import the type inline to avoid circular deps — use the generator directly.
	ui, err := h.generator.Generate(r.Context(), makeAPIRunResult(req), caps)
	if err != nil {
		log.Printf("[ui-api] generate error: %v", err)
		writeJSONError(w, fmt.Sprintf("generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Cache the generated UI.
	h.mu.Lock()
	h.cache[ui.TaskID] = ui
	h.mu.Unlock()

	// Also broadcast via WebSocket if connected clients exist.
	if h.wsServer != nil && h.wsServer.ClientCount() > 0 {
		if err := h.wsServer.BroadcastUI(ui); err != nil {
			log.Printf("[ui-api] broadcast error: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiGenerateResponse{UI: ui})
}

// handleGetLast handles GET /api/ui/last — returns last generated UI.
func (h *UIAPIHandler) handleGetLast(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	var last *GeneratedUI
	for _, ui := range h.cache {
		last = ui // Return any cached UI (last written wins due to map iteration)
	}
	h.mu.RUnlock()

	if last == nil {
		writeJSONError(w, "no UI available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiGenerateResponse{UI: last})
}

// handleWSStatus handles GET /api/ui/ws/status.
func (h *UIAPIHandler) handleWSStatus(w http.ResponseWriter, r *http.Request) {
	status := apiWSStatusResponse{
		HasCachedUI: false,
	}

	if h.wsServer != nil {
		status.Clients = h.wsServer.ClientCount()
		status.Addr = h.wsServer.Addr()
		h.wsServer.mu.RLock()
		status.HasCachedUI = h.wsServer.lastUI != nil
		h.wsServer.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// CacheUI stores a GeneratedUI in the API cache.
func (h *UIAPIHandler) CacheUI(ui *GeneratedUI) {
	if ui == nil {
		return
	}
	h.mu.Lock()
	h.cache[ui.TaskID] = ui
	h.mu.Unlock()
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
