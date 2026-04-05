package senses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// DiscordConfig holds Discord bot configuration.
type DiscordConfig struct {
	BotToken    string `json:"bot_token"`
	AppID       string `json:"app_id"`
	PublicKey   string `json:"public_key"` // For interaction verification
	ListenAddr  string `json:"listen_addr"` // Webhook endpoint address
}

// discordMaxMessageLen is the Discord message length limit.
const discordMaxMessageLen = 2000

// DiscordSense receives messages from Discord via Interactions endpoint.
type DiscordSense struct {
	config   DiscordConfig
	mu       sync.Mutex
	stopped  bool
	cancel   context.CancelFunc
	srv      *http.Server
	listener net.Listener
	client   *http.Client

	// apiBase is the Discord API base URL. Override in tests.
	apiBase string
}

// NewDiscordSense creates a Discord adapter.
func NewDiscordSense(config DiscordConfig) *DiscordSense {
	if config.ListenAddr == "" {
		config.ListenAddr = ":3002"
	}
	return &DiscordSense{
		config:  config,
		client:  &http.Client{Timeout: 30 * time.Second},
		apiBase: "https://discord.com/api/v10",
	}
}

func (s *DiscordSense) Name() string { return "Discord" }

// Start begins listening for Discord interactions.
func (s *DiscordSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("discord sense already stopped")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/discord/interactions", s.handleInteraction(out))

	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("discord listen: %w", err)
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

// handleInteraction processes Discord interaction webhooks.
func (s *DiscordSense) handleInteraction(out chan<- *UnifiedInput) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var interaction discordInteraction
		if err := json.Unmarshal(body, &interaction); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Handle ping (type 1).
		if interaction.Type == 1 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{"type": 1})
			return
		}

		// Handle message/slash commands.
		if interaction.Type == 2 && interaction.Data != nil {
			// Slash command.
			var text string
			for _, opt := range interaction.Data.Options {
				if opt.Name == "message" || opt.Name == "query" || opt.Name == "input" {
					text = fmt.Sprintf("%v", opt.Value)
					break
				}
			}
			if text == "" {
				text = interaction.Data.Name
			}

			input := NewUnifiedInput(SourceDiscord, text)
			input.SourceMeta.Channel = "discord"
			input.SourceMeta.Sender = interaction.Member.User.ID
			input.SourceMeta.Extra = map[string]string{
				"guild_id":    interaction.GuildID,
				"channel_id":  interaction.ChannelID,
				"command":     interaction.Data.Name,
				"username":    interaction.Member.User.Username,
				"interaction": interaction.ID,
			}
			input.ResponseChannel = interaction.ChannelID

			select {
			case out <- input:
			default:
			}

			// Acknowledge with deferred response.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{"type": 5}) // DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Send sends a message to a Discord channel via REST API.
// Long messages are automatically split into chunks of discordMaxMessageLen.
func (s *DiscordSense) Send(ctx context.Context, target string, message string) error {
	if message == "" {
		return fmt.Errorf("discord: empty message")
	}

	// Split into chunks for Discord's 2000-char limit.
	for len(message) > 0 {
		chunk := message
		if len(chunk) > discordMaxMessageLen {
			chunk = message[:discordMaxMessageLen]
			message = message[discordMaxMessageLen:]
		} else {
			message = ""
		}

		if err := s.sendChunk(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

// sendChunk sends a single message chunk to Discord.
func (s *DiscordSense) sendChunk(ctx context.Context, channelID, content string) error {
	payload, _ := json.Marshal(map[string]string{
		"content": content,
	})

	url := fmt.Sprintf("%s/channels/%s/messages", s.apiBase, channelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("discord: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+s.config.BotToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord: API error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Stop gracefully shuts down the Discord adapter.
func (s *DiscordSense) Stop() error {
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
func (s *DiscordSense) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// --- Discord API types (minimal subset) ---

type discordInteraction struct {
	ID        string                  `json:"id"`
	Type      int                     `json:"type"` // 1=PING, 2=APPLICATION_COMMAND
	Data      *discordInteractionData `json:"data,omitempty"`
	GuildID   string                  `json:"guild_id,omitempty"`
	ChannelID string                  `json:"channel_id,omitempty"`
	Member    discordMember           `json:"member"`
}

type discordInteractionData struct {
	Name    string                    `json:"name"`
	Options []discordCommandOption    `json:"options,omitempty"`
}

type discordCommandOption struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type discordMember struct {
	User discordUser `json:"user"`
}

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
