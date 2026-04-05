package senses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Telegram Send tests
// ---------------------------------------------------------------------------

func TestTelegramSend_Basic(t *testing.T) {
	var received struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			t.Errorf("expected /sendMessage path, got %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		json.Unmarshal(body, &received)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer srv.Close()

	tg := NewTelegramSense(TelegramConfig{Token: "test-token"})
	tg.apiBase = srv.URL + "/bot" + tg.config.Token

	err := tg.Send(context.Background(), "12345", "Hello from daemon")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if received.ChatID != "12345" {
		t.Errorf("chat_id = %q, want 12345", received.ChatID)
	}
	if received.Text != "Hello from daemon" {
		t.Errorf("text = %q, want 'Hello from daemon'", received.Text)
	}
}

func TestTelegramSend_Chunking(t *testing.T) {
	var messages []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
		mu.Lock()
		messages = append(messages, payload["text"])
		mu.Unlock()
		w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer srv.Close()

	tg := NewTelegramSense(TelegramConfig{Token: "tok"})
	tg.apiBase = srv.URL + "/bottok"

	// Create message > 4096 chars (Telegram limit).
	longMsg := strings.Repeat("A", 4096+100)
	err := tg.Send(context.Background(), "1", longMsg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(messages) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(messages))
	}
	if len(messages[0]) != 4096 {
		t.Errorf("first chunk = %d chars, want 4096", len(messages[0]))
	}
	if len(messages[1]) != 100 {
		t.Errorf("second chunk = %d chars, want 100", len(messages[1]))
	}
}

func TestTelegramSend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"error_code":403,"description":"bot was blocked"}`))
	}))
	defer srv.Close()

	tg := NewTelegramSense(TelegramConfig{Token: "tok"})
	tg.apiBase = srv.URL + "/bottok"

	err := tg.Send(context.Background(), "1", "hello")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "bot was blocked") {
		t.Errorf("error = %q, want to contain 'bot was blocked'", err.Error())
	}
}

func TestTelegramSend_EmptyMessage(t *testing.T) {
	tg := NewTelegramSense(TelegramConfig{Token: "tok"})
	err := tg.Send(context.Background(), "1", "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

// ---------------------------------------------------------------------------
// Slack Send tests
// ---------------------------------------------------------------------------

func TestSlackSend_Basic(t *testing.T) {
	var received struct {
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}
	var mu sync.Mutex
	var authHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeader = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		mu.Unlock()
		w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1234.5678"}`))
	}))
	defer srv.Close()

	sl := NewSlackSense(SlackConfig{BotToken: "xoxb-test"})
	sl.apiBase = srv.URL

	err := sl.Send(context.Background(), "C123", "Hello Slack")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if authHeader != "Bearer xoxb-test" {
		t.Errorf("Authorization = %q, want 'Bearer xoxb-test'", authHeader)
	}
	if received.Channel != "C123" {
		t.Errorf("channel = %q, want C123", received.Channel)
	}
	if received.Text != "Hello Slack" {
		t.Errorf("text = %q, want 'Hello Slack'", received.Text)
	}
}

func TestSlackSend_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	defer srv.Close()

	sl := NewSlackSense(SlackConfig{BotToken: "tok"})
	sl.apiBase = srv.URL

	err := sl.Send(context.Background(), "C999", "hello")
	if err == nil {
		t.Fatal("expected error for Slack API failure")
	}
	if !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("error = %q, want to contain 'channel_not_found'", err.Error())
	}
}

func TestSlackSend_EmptyMessage(t *testing.T) {
	sl := NewSlackSense(SlackConfig{BotToken: "tok"})
	err := sl.Send(context.Background(), "C123", "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

// ---------------------------------------------------------------------------
// Discord Send tests
// ---------------------------------------------------------------------------

func TestDiscordSend_Basic(t *testing.T) {
	var received struct {
		Content string `json:"content"`
	}
	var mu sync.Mutex
	var authHeader, urlPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeader = r.Header.Get("Authorization")
		urlPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		mu.Unlock()
		w.Write([]byte(`{"id":"1","content":"ok"}`))
	}))
	defer srv.Close()

	dc := NewDiscordSense(DiscordConfig{BotToken: "bot-token"})
	dc.apiBase = srv.URL

	err := dc.Send(context.Background(), "CH456", "Hello Discord")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if authHeader != "Bot bot-token" {
		t.Errorf("Authorization = %q, want 'Bot bot-token'", authHeader)
	}
	if !strings.Contains(urlPath, "CH456") {
		t.Errorf("URL path = %q, want to contain channel ID 'CH456'", urlPath)
	}
	if received.Content != "Hello Discord" {
		t.Errorf("content = %q, want 'Hello Discord'", received.Content)
	}
}

func TestDiscordSend_Chunking(t *testing.T) {
	var messages []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
		mu.Lock()
		messages = append(messages, payload["content"])
		mu.Unlock()
		w.Write([]byte(`{"id":"1","content":"ok"}`))
	}))
	defer srv.Close()

	dc := NewDiscordSense(DiscordConfig{BotToken: "tok"})
	dc.apiBase = srv.URL

	// Discord limit is 2000 chars.
	longMsg := strings.Repeat("B", 2000+500)
	err := dc.Send(context.Background(), "CH1", longMsg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(messages) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(messages))
	}
	if len(messages[0]) != 2000 {
		t.Errorf("first chunk = %d chars, want 2000", len(messages[0]))
	}
	if len(messages[1]) != 500 {
		t.Errorf("second chunk = %d chars, want 500", len(messages[1]))
	}
}

func TestDiscordSend_EmptyMessage(t *testing.T) {
	dc := NewDiscordSense(DiscordConfig{BotToken: "tok"})
	err := dc.Send(context.Background(), "CH1", "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

// ---------------------------------------------------------------------------
// SenseRegistry tests
// ---------------------------------------------------------------------------

func TestSenseRegistry_GetBySourceType(t *testing.T) {
	reg := NewSenseRegistry()

	tg := NewTelegramSense(TelegramConfig{Token: "tok"})
	sl := NewSlackSense(SlackConfig{BotToken: "tok"})
	dc := NewDiscordSense(DiscordConfig{BotToken: "tok"})

	reg.Register(tg)
	reg.Register(sl)
	reg.Register(dc)

	tests := []struct {
		st   SourceType
		want string
	}{
		{SourceTelegram, "Telegram"},
		{SourceSlack, "Slack"},
		{SourceDiscord, "Discord"},
		{SourceEmail, ""}, // not registered
	}

	for _, tt := range tests {
		s := reg.GetBySourceType(tt.st)
		if tt.want == "" {
			if s != nil {
				t.Errorf("GetBySourceType(%s) = %v, want nil", tt.st, s)
			}
		} else {
			if s == nil {
				t.Errorf("GetBySourceType(%s) = nil, want %s", tt.st, tt.want)
			} else if s.Name() != tt.want {
				t.Errorf("GetBySourceType(%s).Name() = %q, want %q", tt.st, s.Name(), tt.want)
			}
		}
	}
}

func TestSenseRegistry_PrimarySense(t *testing.T) {
	reg := NewSenseRegistry()
	tg := NewTelegramSense(TelegramConfig{Token: "tok"})
	reg.Register(tg)
	reg.SetPrimary("Telegram", "12345")

	s, target := reg.GetPrimary()
	if s == nil {
		t.Fatal("GetPrimary() returned nil")
	}
	if s.Name() != "Telegram" {
		t.Errorf("primary sense = %q, want Telegram", s.Name())
	}
	if target != "12345" {
		t.Errorf("primary target = %q, want 12345", target)
	}
}

func TestSenseRegistry_PrimaryNotSet(t *testing.T) {
	reg := NewSenseRegistry()
	s, target := reg.GetPrimary()
	if s != nil {
		t.Errorf("expected nil primary, got %v", s)
	}
	if target != "" {
		t.Errorf("expected empty target, got %q", target)
	}
}
