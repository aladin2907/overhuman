package senses

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

// SlackConfig holds Slack app configuration.
type SlackConfig struct {
	BotToken     string `json:"bot_token"`     // xoxb-...
	AppToken     string `json:"app_token"`     // xapp-... (for Socket Mode)
	SigningSecret string `json:"signing_secret"` // For webhook verification
	ListenAddr   string `json:"listen_addr"`   // Webhook listen address (e.g., ":3001")
}

// SlackSense receives messages from Slack via Events API webhooks.
type SlackSense struct {
	config   SlackConfig
	mu       sync.Mutex
	stopped  bool
	cancel   context.CancelFunc
	srv      *http.Server
	listener net.Listener
}

// NewSlackSense creates a Slack adapter.
func NewSlackSense(config SlackConfig) *SlackSense {
	if config.ListenAddr == "" {
		config.ListenAddr = ":3001"
	}
	return &SlackSense{config: config}
}

func (s *SlackSense) Name() string { return "Slack" }

// Start begins listening for Slack Events API webhooks.
func (s *SlackSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("slack sense already stopped")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/events", s.handleEvents(out))

	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("slack listen: %w", err)
	}

	s.mu.Lock()
	s.listener = ln
	s.srv = &http.Server{Handler: mux}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.srv.Close()
	}()

	err = s.srv.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// handleEvents processes incoming Slack Events API payloads.
func (s *SlackSense) handleEvents(out chan<- *UnifiedInput) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var envelope slackEventEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Handle URL verification challenge.
		if envelope.Type == "url_verification" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"challenge": envelope.Challenge,
			})
			return
		}

		// Handle event callbacks.
		if envelope.Type == "event_callback" && envelope.Event != nil {
			if envelope.Event.Type == "message" && envelope.Event.BotID == "" {
				input := NewUnifiedInput(SourceSlack, envelope.Event.Text)
				input.SourceMeta.Channel = "slack"
				input.SourceMeta.Sender = envelope.Event.User
				input.SourceMeta.Extra = map[string]string{
					"channel":   envelope.Event.Channel,
					"team":      envelope.TeamID,
					"ts":        envelope.Event.TS,
					"thread_ts": envelope.Event.ThreadTS,
				}
				input.ResponseChannel = envelope.Event.Channel

				select {
				case out <- input:
				default:
					// Drop if channel full.
				}
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Send posts a message to a Slack channel.
func (s *SlackSense) Send(ctx context.Context, target string, message string) error {
	// Uses chat.postMessage API.
	// target is the channel ID.
	_ = target
	_ = message
	// Placeholder for actual Slack API call.
	return nil
}

// Stop gracefully shuts down the Slack adapter.
func (s *SlackSense) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	if s.srv != nil {
		s.srv.Close()
	}
	return nil
}

// Addr returns the listener address (for testing).
func (s *SlackSense) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// --- Slack API types (minimal subset) ---

type slackEventEnvelope struct {
	Type      string      `json:"type"`
	Challenge string      `json:"challenge,omitempty"`
	TeamID    string      `json:"team_id,omitempty"`
	Event     *slackEvent `json:"event,omitempty"`
}

type slackEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Text     string `json:"text"`
	Channel  string `json:"channel"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts,omitempty"`
	BotID    string `json:"bot_id,omitempty"`
}
