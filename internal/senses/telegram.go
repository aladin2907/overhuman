package senses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// TelegramConfig holds Telegram bot configuration.
type TelegramConfig struct {
	Token       string        `json:"token"`        // Bot API token
	WebhookURL  string        `json:"webhook_url"`  // If set, use webhook instead of polling
	PollTimeout time.Duration `json:"poll_timeout"`
	AllowedIDs  []int64       `json:"allowed_ids"`  // Whitelist of user/chat IDs (empty = allow all)
}

// TelegramSense connects to Telegram Bot API.
// Supports both long-polling and webhook modes.
type TelegramSense struct {
	config  TelegramConfig
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
	out     chan<- *UnifiedInput
	client  *http.Client

	// apiBase is the Telegram Bot API base URL. Override in tests.
	apiBase string

	// Response routing: chatID → pending messages.
	responses map[string]string
}

// NewTelegramSense creates a Telegram adapter.
// telegramMaxMessageLen is the Telegram Bot API message length limit.
const telegramMaxMessageLen = 4096

func NewTelegramSense(config TelegramConfig) *TelegramSense {
	if config.PollTimeout == 0 {
		config.PollTimeout = 30 * time.Second
	}
	token := config.Token
	return &TelegramSense{
		config:    config,
		client:    &http.Client{Timeout: 30 * time.Second},
		apiBase:   "https://api.telegram.org/bot" + token,
		responses: make(map[string]string),
	}
}

func (s *TelegramSense) Name() string { return "Telegram" }

// Start begins listening for Telegram messages via long-polling.
func (s *TelegramSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("telegram sense already stopped")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.out = out
	s.mu.Unlock()

	return s.poll(ctx, out)
}

// poll implements the Telegram getUpdates long-polling loop.
func (s *TelegramSense) poll(ctx context.Context, out chan<- *UnifiedInput) error {
	offset := 0
	client := &http.Client{Timeout: s.config.PollTimeout + 5*time.Second}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=%d",
			s.config.Token, offset, int(s.config.PollTimeout.Seconds()))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(2 * time.Second) // Backoff on error.
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var result telegramResponse
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		for _, update := range result.Result {
			if update.Message == nil || update.Message.Text == "" {
				offset = update.UpdateID + 1
				continue
			}

			// Whitelist check.
			if len(s.config.AllowedIDs) > 0 && !s.isAllowed(update.Message.From.ID) {
				offset = update.UpdateID + 1
				continue
			}

			input := NewUnifiedInput(SourceTelegram, update.Message.Text)
			input.SourceMeta.Channel = "telegram"
			input.SourceMeta.Sender = fmt.Sprintf("%d", update.Message.From.ID)
			input.SourceMeta.Extra = map[string]string{
				"chat_id":    fmt.Sprintf("%d", update.Message.Chat.ID),
				"message_id": fmt.Sprintf("%d", update.Message.MessageID),
				"username":   update.Message.From.Username,
				"first_name": update.Message.From.FirstName,
			}
			input.ResponseChannel = fmt.Sprintf("%d", update.Message.Chat.ID)

			select {
			case out <- input:
			case <-ctx.Done():
				return ctx.Err()
			}

			offset = update.UpdateID + 1
		}
	}
}

// Send sends a message back to a Telegram chat.
// Long messages are automatically split into chunks of telegramMaxMessageLen.
func (s *TelegramSense) Send(ctx context.Context, target string, message string) error {
	if message == "" {
		return fmt.Errorf("telegram: empty message")
	}

	// Split into chunks for Telegram's 4096-char limit.
	for len(message) > 0 {
		chunk := message
		if len(chunk) > telegramMaxMessageLen {
			chunk = message[:telegramMaxMessageLen]
			message = message[telegramMaxMessageLen:]
		} else {
			message = ""
		}

		if err := s.sendChunk(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

// sendChunk sends a single message chunk to Telegram Bot API.
func (s *TelegramSense) sendChunk(ctx context.Context, chatID, text string) error {
	payload, _ := json.Marshal(map[string]string{
		"chat_id": chatID,
		"text":    text,
	})

	url := s.apiBase + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code,omitempty"`
		Description string `json:"description,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("telegram: parse response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram: API error %d: %s", result.ErrorCode, result.Description)
	}
	return nil
}

func (s *TelegramSense) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *TelegramSense) isAllowed(userID int64) bool {
	for _, id := range s.config.AllowedIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// --- Telegram API types (minimal subset) ---

type telegramResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *telegramMessage `json:"message,omitempty"`
}

type telegramMessage struct {
	MessageID int          `json:"message_id"`
	From      telegramUser `json:"from"`
	Chat      telegramChat `json:"chat"`
	Text      string       `json:"text"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
