package senses

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// APISense implements the Sense interface for HTTP REST API input.
// It starts an HTTP server that accepts POST /input requests with JSON payloads
// and emits UnifiedInput messages to the pipeline.
type APISense struct {
	addr string
	srv  *http.Server
	out  chan<- *UnifiedInput

	mu       sync.Mutex
	listener net.Listener
	stopped  bool

	// responses stores pending response channels keyed by correlation ID.
	responses   map[string]chan string
	responsesMu sync.RWMutex
}

// apiRequest is the JSON body for POST /input.
type apiRequest struct {
	Payload  string            `json:"payload"`
	Priority string            `json:"priority,omitempty"` // "LOW", "NORMAL", "HIGH", "CRITICAL"
	Sender   string            `json:"sender,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// apiResponse is the JSON body returned for POST /input.
type apiResponse struct {
	InputID string `json:"input_id"`
	Status  string `json:"status"`
}

// apiHealthResponse is the JSON body for GET /health.
type apiHealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// NewAPISense creates an HTTP API sense adapter.
// addr is the listen address, e.g. ":8080" or "127.0.0.1:9000".
func NewAPISense(addr string) *APISense {
	return &APISense{
		addr:      addr,
		responses: make(map[string]chan string),
	}
}

// Name returns the sense name.
func (a *APISense) Name() string { return "API" }

// Start launches the HTTP server and blocks until ctx is cancelled.
func (a *APISense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	a.mu.Lock()
	a.out = out

	startTime := time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiHealthResponse{
			Status: "ok",
			Uptime: time.Since(startTime).String(),
		})
	})
	mux.HandleFunc("POST /input", a.handleInput)
	mux.HandleFunc("POST /input/sync", a.handleInputSync)

	a.srv = &http.Server{
		Addr:              a.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", a.addr)
	if err != nil {
		a.mu.Unlock()
		return fmt.Errorf("api sense: listen: %w", err)
	}
	a.listener = ln
	a.mu.Unlock()

	// Graceful shutdown on context cancellation.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.srv.Shutdown(shutdownCtx)
	}()

	if err := a.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api sense: serve: %w", err)
	}
	return nil
}

// handleInput handles async POST /input — fire-and-forget.
func (a *APISense) handleInput(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.Payload == "" {
		http.Error(w, `{"error":"payload required"}`, http.StatusBadRequest)
		return
	}

	input := a.buildInput(req)

	select {
	case a.out <- input:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(apiResponse{
			InputID: input.InputID,
			Status:  "accepted",
		})
	default:
		http.Error(w, `{"error":"pipeline busy"}`, http.StatusServiceUnavailable)
	}
}

// handleInputSync handles POST /input/sync — waits for response (with timeout).
func (a *APISense) handleInputSync(w http.ResponseWriter, r *http.Request) {
	var req apiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.Payload == "" {
		http.Error(w, `{"error":"payload required"}`, http.StatusBadRequest)
		return
	}

	input := a.buildInput(req)
	input.CorrelationID = input.InputID
	input.ResponseChannel = "api"

	// Register response channel.
	ch := make(chan string, 1)
	a.responsesMu.Lock()
	a.responses[input.InputID] = ch
	a.responsesMu.Unlock()

	defer func() {
		a.responsesMu.Lock()
		delete(a.responses, input.InputID)
		a.responsesMu.Unlock()
	}()

	select {
	case a.out <- input:
	default:
		http.Error(w, `{"error":"pipeline busy"}`, http.StatusServiceUnavailable)
		return
	}

	// Wait for response with timeout.
	timeout := 60 * time.Second
	select {
	case msg := <-ch:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"input_id": input.InputID,
			"status":   "completed",
			"result":   msg,
		})
	case <-time.After(timeout):
		http.Error(w, `{"error":"timeout"}`, http.StatusGatewayTimeout)
	case <-r.Context().Done():
		return
	}
}

func (a *APISense) buildInput(req apiRequest) *UnifiedInput {
	priority := PriorityNormal
	switch req.Priority {
	case "LOW":
		priority = PriorityLow
	case "HIGH":
		priority = PriorityHigh
	case "CRITICAL":
		priority = PriorityCritical
	}

	sender := req.Sender
	if sender == "" {
		sender = "api_user"
	}

	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: SourceAPI,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
			Channel:   "api",
			Sender:    sender,
			Extra:     req.Metadata,
		},
		Payload:  req.Payload,
		Priority: priority,
	}
}

// Send delivers a response back. For the API sense, if the target matches
// a pending sync request's correlation ID, it delivers the response.
func (a *APISense) Send(ctx context.Context, target string, message string) error {
	a.responsesMu.RLock()
	ch, ok := a.responses[target]
	a.responsesMu.RUnlock()

	if ok {
		select {
		case ch <- message:
		default:
		}
		return nil
	}

	// No waiting request — that's fine for async mode.
	return nil
}

// Stop gracefully stops the HTTP server.
func (a *APISense) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stopped = true
	if a.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.srv.Shutdown(ctx)
	}
	return nil
}

// Addr returns the listener address. Useful for tests to get the actual port
// when ":0" is used. Returns empty string if not started.
func (a *APISense) Addr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listener != nil {
		return a.listener.Addr().String()
	}
	return a.addr
}
