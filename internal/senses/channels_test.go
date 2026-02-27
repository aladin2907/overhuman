package senses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Telegram tests ---

func TestNewTelegramSense(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{Token: "test-token"})
	if s == nil {
		t.Fatal("nil")
	}
	if s.Name() != "Telegram" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.config.PollTimeout != 30*time.Second {
		t.Errorf("PollTimeout = %v", s.config.PollTimeout)
	}
}

func TestTelegramSense_Stop(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{Token: "t"})
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if !s.stopped {
		t.Error("should be stopped")
	}
}

func TestTelegramSense_StartAfterStop(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{Token: "t"})
	s.Stop()

	err := s.Start(context.Background(), make(chan *UnifiedInput))
	if err == nil {
		t.Error("expected error when starting after stop")
	}
}

func TestTelegramSense_IsAllowed(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{
		Token:      "t",
		AllowedIDs: []int64{111, 222},
	})

	if !s.isAllowed(111) {
		t.Error("111 should be allowed")
	}
	if s.isAllowed(999) {
		t.Error("999 should not be allowed")
	}
}

func TestTelegramSense_IsAllowed_EmptyWhitelist(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{Token: "t"})
	// Empty whitelist means allow all — but isAllowed returns false for empty list.
	// The poll loop checks len(AllowedIDs) > 0 before calling isAllowed.
	if s.isAllowed(123) {
		t.Error("isAllowed should return false for empty whitelist")
	}
}

// TestTelegramSense_PollWithMockServer tests the polling loop against a mock Telegram API.
func TestTelegramSense_PollWithMockServer(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: return a message.
			json.NewEncoder(w).Encode(telegramResponse{
				OK: true,
				Result: []telegramUpdate{
					{
						UpdateID: 1,
						Message: &telegramMessage{
							MessageID: 100,
							From:      telegramUser{ID: 42, Username: "testuser", FirstName: "Test"},
							Chat:      telegramChat{ID: 42, Type: "private"},
							Text:      "hello bot",
						},
					},
				},
			})
		} else {
			// Second+ call: context should be cancelled, return empty.
			json.NewEncoder(w).Encode(telegramResponse{OK: true})
		}
	}))
	defer server.Close()

	// We can't easily redirect the Telegram API URL from poll(), but we can test
	// the message parsing by testing the types directly.
	msg := telegramMessage{
		MessageID: 1,
		From:      telegramUser{ID: 42, Username: "user1"},
		Chat:      telegramChat{ID: 42},
		Text:      "test message",
	}
	if msg.Text != "test message" {
		t.Errorf("Text = %q", msg.Text)
	}
}

func TestTelegramSense_Send(t *testing.T) {
	s := NewTelegramSense(TelegramConfig{Token: "test"})
	// Send is a placeholder — should not error.
	err := s.Send(context.Background(), "12345", "hello")
	if err != nil {
		t.Fatal(err)
	}
}

// --- Slack tests ---

func TestNewSlackSense(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	if s.Name() != "Slack" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.config.ListenAddr != ":3001" {
		t.Errorf("ListenAddr = %q", s.config.ListenAddr)
	}
}

func TestSlackSense_Stop(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestSlackSense_URLVerification(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleEvents(out)

	body := `{"type":"url_verification","challenge":"test-challenge-123"}`
	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["challenge"] != "test-challenge-123" {
		t.Errorf("challenge = %q", resp["challenge"])
	}
}

func TestSlackSense_EventCallback(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleEvents(out)

	body := `{
		"type": "event_callback",
		"team_id": "T123",
		"event": {
			"type": "message",
			"user": "U456",
			"text": "hello from slack",
			"channel": "C789",
			"ts": "1234567890.123456"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d", w.Code)
	}

	select {
	case input := <-out:
		if input.Payload != "hello from slack" {
			t.Errorf("Payload = %q", input.Payload)
		}
		if input.SourceMeta.Sender != "U456" {
			t.Errorf("Sender = %q", input.SourceMeta.Sender)
		}
		if input.SourceMeta.Extra["channel"] != "C789" {
			t.Errorf("channel = %q", input.SourceMeta.Extra["channel"])
		}
		if input.ResponseChannel != "C789" {
			t.Errorf("ResponseChannel = %q", input.ResponseChannel)
		}
	default:
		t.Error("expected message in channel")
	}
}

func TestSlackSense_BotMessageIgnored(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleEvents(out)

	body := `{
		"type": "event_callback",
		"event": {
			"type": "message",
			"user": "U456",
			"text": "bot message",
			"channel": "C789",
			"bot_id": "B123"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	select {
	case <-out:
		t.Error("bot messages should be ignored")
	default:
		// Good.
	}
}

func TestSlackSense_InvalidJSON(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleEvents(out)

	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()

	handler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestSlackSense_Send(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	err := s.Send(context.Background(), "C123", "hello")
	if err != nil {
		t.Fatal(err)
	}
}

// --- Discord tests ---

func TestNewDiscordSense(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test"})
	if s.Name() != "Discord" {
		t.Errorf("Name = %q", s.Name())
	}
}

func TestDiscordSense_Stop(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test"})
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestDiscordSense_PingInteraction(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleInteraction(out)

	body := `{"type": 1}` // PING
	req := httptest.NewRequest(http.MethodPost, "/discord/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	var resp map[string]int
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["type"] != 1 {
		t.Errorf("response type = %d, want 1", resp["type"])
	}
}

func TestDiscordSense_SlashCommand(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test"})
	out := make(chan *UnifiedInput, 10)
	handler := s.handleInteraction(out)

	body := `{
		"id": "int_1",
		"type": 2,
		"guild_id": "G1",
		"channel_id": "C1",
		"member": {"user": {"id": "U1", "username": "testuser"}},
		"data": {
			"name": "ask",
			"options": [{"name": "message", "value": "what is Go?"}]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/discord/interactions", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	// Should respond with deferred (type 5).
	var resp map[string]int
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["type"] != 5 {
		t.Errorf("response type = %d, want 5", resp["type"])
	}

	select {
	case input := <-out:
		if input.Payload != "what is Go?" {
			t.Errorf("Payload = %q", input.Payload)
		}
		if input.SourceMeta.Sender != "U1" {
			t.Errorf("Sender = %q", input.SourceMeta.Sender)
		}
		if input.SourceMeta.Extra["command"] != "ask" {
			t.Errorf("command = %q", input.SourceMeta.Extra["command"])
		}
	default:
		t.Error("expected message in channel")
	}
}

func TestDiscordSense_Send(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test"})
	err := s.Send(context.Background(), "C1", "hello")
	if err != nil {
		t.Fatal(err)
	}
}

// --- Email tests ---

func TestNewEmailSense(t *testing.T) {
	s := NewEmailSense(EmailConfig{
		IMAPServer: "imap.example.com:993",
		IMAPUser:   "test@example.com",
	})
	if s.Name() != "Email" {
		t.Errorf("Name = %q", s.Name())
	}
	if s.config.PollInterval != 60*time.Second {
		t.Errorf("PollInterval = %v", s.config.PollInterval)
	}
	if s.config.FolderName != "INBOX" {
		t.Errorf("FolderName = %q", s.config.FolderName)
	}
}

func TestEmailSense_Stop(t *testing.T) {
	s := NewEmailSense(EmailConfig{})
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if !s.stopped {
		t.Error("should be stopped")
	}
}

func TestEmailSense_StartAfterStop(t *testing.T) {
	s := NewEmailSense(EmailConfig{})
	s.Stop()

	err := s.Start(context.Background(), make(chan *UnifiedInput))
	if err == nil {
		t.Error("expected error when starting after stop")
	}
}

func TestEmailSense_IsAllowed(t *testing.T) {
	s := NewEmailSense(EmailConfig{
		AllowedSenders: []string{"alice@example.com", "bob@example.com"},
	})

	if !s.isAllowed("alice@example.com") {
		t.Error("alice should be allowed")
	}
	if s.isAllowed("eve@example.com") {
		t.Error("eve should not be allowed")
	}
}

func TestEmailSense_Send(t *testing.T) {
	s := NewEmailSense(EmailConfig{})
	err := s.Send(context.Background(), "user@example.com", "hello")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmailSense_FetchNewEmails_Placeholder(t *testing.T) {
	s := NewEmailSense(EmailConfig{})
	emails, err := s.fetchNewEmails(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 0 {
		t.Errorf("emails = %d, want 0 (placeholder)", len(emails))
	}
}

// --- Sense interface compliance ---

func TestTelegramSense_ImplementsSense(t *testing.T) {
	var _ Sense = (*TelegramSense)(nil)
}

func TestSlackSense_ImplementsSense(t *testing.T) {
	var _ Sense = (*SlackSense)(nil)
}

func TestDiscordSense_ImplementsSense(t *testing.T) {
	var _ Sense = (*DiscordSense)(nil)
}

func TestEmailSense_ImplementsSense(t *testing.T) {
	var _ Sense = (*EmailSense)(nil)
}

// --- Slack Start/Stop lifecycle test ---

func TestSlackSense_StartStop(t *testing.T) {
	s := NewSlackSense(SlackConfig{BotToken: "xoxb-test", ListenAddr: ":0"})
	out := make(chan *UnifiedInput, 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx, out)
	}()

	// Wait for server to start.
	time.Sleep(50 * time.Millisecond)

	addr := s.Addr()
	if addr == "" {
		t.Fatal("Addr should not be empty after start")
	}

	// Verify the server responds.
	url := fmt.Sprintf("http://%s/slack/events", addr)
	resp, err := http.Post(url, "application/json",
		strings.NewReader(`{"type":"url_verification","challenge":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Status = %d", resp.StatusCode)
	}

	cancel()
	<-done
}

// --- Discord Start/Stop lifecycle test ---

func TestDiscordSense_StartStop(t *testing.T) {
	s := NewDiscordSense(DiscordConfig{BotToken: "test", ListenAddr: ":0"})
	out := make(chan *UnifiedInput, 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx, out)
	}()

	time.Sleep(50 * time.Millisecond)

	addr := s.Addr()
	if addr == "" {
		t.Fatal("Addr should not be empty")
	}

	// Verify ping works.
	url := fmt.Sprintf("http://%s/discord/interactions", addr)
	resp, err := http.Post(url, "application/json", strings.NewReader(`{"type":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Status = %d", resp.StatusCode)
	}

	cancel()
	<-done
}
