package senses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// WebhookSense implements the Sense interface for incoming HTTP POST webhooks.
// It listens on a configurable path and converts webhook payloads into
// UnifiedInput messages.
type WebhookSense struct {
	addr string
	path string // Default: "/webhook"
	srv  *http.Server
	out  chan<- *UnifiedInput

	mu       sync.Mutex
	listener net.Listener
	stopped  bool
}

// NewWebhookSense creates a webhook receiver.
// addr is the listen address (e.g. ":8081"), path is the URL path (e.g. "/webhook").
func NewWebhookSense(addr, path string) *WebhookSense {
	if path == "" {
		path = "/webhook"
	}
	return &WebhookSense{
		addr: addr,
		path: path,
	}
}

// Name returns the sense name.
func (w *WebhookSense) Name() string { return "Webhook" }

// Start launches the webhook HTTP server.
func (w *WebhookSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	w.mu.Lock()
	w.out = out

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+w.path, w.handleWebhook)
	mux.HandleFunc("GET "+w.path, func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{"status": "ok", "path": w.path})
	})

	w.srv = &http.Server{
		Addr:              w.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", w.addr)
	if err != nil {
		w.mu.Unlock()
		return fmt.Errorf("webhook sense: listen: %w", err)
	}
	w.listener = ln
	w.mu.Unlock()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		w.srv.Shutdown(shutdownCtx)
	}()

	if err := w.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webhook sense: serve: %w", err)
	}
	return nil
}

func (w *WebhookSense) handleWebhook(rw http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB max
	if err != nil {
		http.Error(rw, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	source := r.Header.Get("X-Webhook-Source")
	if source == "" {
		source = r.RemoteAddr
	}

	input := NewFromWebhook(body, source)

	// Check for priority header.
	if p := r.Header.Get("X-Priority"); p != "" {
		switch p {
		case "LOW":
			input.Priority = PriorityLow
		case "HIGH":
			input.Priority = PriorityHigh
		case "CRITICAL":
			input.Priority = PriorityCritical
		}
	}

	select {
	case w.out <- input:
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusAccepted)
		json.NewEncoder(rw).Encode(map[string]string{
			"input_id": input.InputID,
			"status":   "accepted",
		})
	default:
		http.Error(rw, `{"error":"pipeline busy"}`, http.StatusServiceUnavailable)
	}
}

// Send sends a response back via webhook. For webhooks, this is a no-op
// since webhooks are typically fire-and-forget.
func (w *WebhookSense) Send(ctx context.Context, target string, message string) error {
	// Webhooks are fire-and-forget — no response channel.
	return nil
}

// Stop gracefully stops the webhook server.
func (w *WebhookSense) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.stopped = true
	if w.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return w.srv.Shutdown(ctx)
	}
	return nil
}

// Addr returns the listener address.
func (w *WebhookSense) Addr() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.listener != nil {
		return w.listener.Addr().String()
	}
	return w.addr
}
