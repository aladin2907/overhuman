package genui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
)

func TestUIAPIHandler_Generate(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: genHtmlFullPage, Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	handler := NewUIAPIHandler(gen, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(apiGenerateRequest{
		TaskID:       "api_test_1",
		Result:       "Analysis complete.",
		QualityScore: 0.9,
	})

	req := httptest.NewRequest("POST", "/api/ui/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}

	var resp apiGenerateResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.UI == nil {
		t.Fatal("UI should not be nil")
	}
	if resp.UI.TaskID != "api_test_1" {
		t.Errorf("TaskID = %q, want api_test_1", resp.UI.TaskID)
	}
	if resp.UI.Format != FormatHTML {
		t.Errorf("Format = %q, want html", resp.UI.Format)
	}
}

func TestUIAPIHandler_Generate_MissingResult(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(apiGenerateRequest{TaskID: "t1"})
	req := httptest.NewRequest("POST", "/api/ui/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 400 {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUIAPIHandler_Generate_InvalidJSON(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("POST", "/api/ui/generate", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 400 {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestUIAPIHandler_GetLast_Empty(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/ui/last", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 404 {
		t.Errorf("status = %d, want 404 for empty cache", rr.Code)
	}
}

func TestUIAPIHandler_GetLast_WithCache(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil)

	// Pre-cache a UI.
	handler.CacheUI(&GeneratedUI{
		TaskID: "cached_task",
		Format: FormatHTML,
		Code:   "<div>cached</div>",
	})

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/ui/last", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp apiGenerateResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.UI == nil {
		t.Fatal("UI should not be nil")
	}
	if resp.UI.TaskID != "cached_task" {
		t.Errorf("TaskID = %q, want cached_task", resp.UI.TaskID)
	}
}

func TestUIAPIHandler_WSStatus_NoServer(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil) // nil wsServer

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/ui/ws/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var status apiWSStatusResponse
	json.NewDecoder(rr.Body).Decode(&status)
	if status.Clients != 0 {
		t.Errorf("Clients = %d, want 0 with nil server", status.Clients)
	}
}

func TestUIAPIHandler_WSStatus_WithServer(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	ws := NewWSServer(":0")
	handler := NewUIAPIHandler(gen, ws)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest("GET", "/api/ui/ws/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var status apiWSStatusResponse
	json.NewDecoder(rr.Body).Decode(&status)
	if status.HasCachedUI {
		t.Error("HasCachedUI should be false initially")
	}
}

func TestUIAPIHandler_CacheUI(t *testing.T) {
	gen := NewUIGenerator(nil, brain.NewModelRouter())
	handler := NewUIAPIHandler(gen, nil)

	handler.CacheUI(&GeneratedUI{TaskID: "t1", Code: "a"})
	handler.CacheUI(&GeneratedUI{TaskID: "t2", Code: "b"})

	// Nil should not panic.
	handler.CacheUI(nil)
}

func TestUIAPIHandler_Generate_WithCustomCaps(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: genAnsiSimpleText, Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	handler := NewUIAPIHandler(gen, nil)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	caps := CLICapabilities()
	body, _ := json.Marshal(apiGenerateRequest{
		TaskID:       "custom_caps",
		Result:       "test",
		QualityScore: 0.8,
		Caps:         &caps,
	})

	req := httptest.NewRequest("POST", "/api/ui/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp apiGenerateResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.UI.Format != FormatANSI {
		t.Errorf("Format = %q, want ansi (from custom caps)", resp.UI.Format)
	}
}
