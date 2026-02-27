package senses

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock sense for registry tests
// ---------------------------------------------------------------------------

type mockSense struct {
	name    string
	started bool
	stopped bool
	mu      sync.Mutex
	sent    []string // records target:message pairs
}

func newMockSense(name string) *mockSense {
	return &mockSense{name: name}
}

func (m *mockSense) Name() string { return m.name }

func (m *mockSense) Start(_ context.Context, _ chan<- *UnifiedInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return nil
}

func (m *mockSense) Send(_ context.Context, target string, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, target+":"+message)
	return nil
}

func (m *mockSense) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}

// ---------------------------------------------------------------------------
// 1. NewFromText creates valid UnifiedInput
// ---------------------------------------------------------------------------

func TestNewFromText(t *testing.T) {
	input := NewFromText("hello world")

	if input.InputID == "" {
		t.Fatal("InputID must not be empty")
	}
	if input.SourceType != SourceText {
		t.Fatalf("expected SourceType TEXT, got %s", input.SourceType)
	}
	if input.Payload != "hello world" {
		t.Fatalf("expected payload 'hello world', got %q", input.Payload)
	}
	if input.Priority != PriorityNormal {
		t.Fatalf("expected NORMAL priority, got %s", input.Priority)
	}
	if input.SourceMeta.Timestamp.IsZero() {
		t.Fatal("timestamp must not be zero")
	}
	if input.SourceMeta.Channel != "text" {
		t.Fatalf("expected channel 'text', got %q", input.SourceMeta.Channel)
	}

	// InputIDs must be unique.
	input2 := NewFromText("another")
	if input.InputID == input2.InputID {
		t.Fatal("two calls to NewFromText must produce different InputIDs")
	}
}

// ---------------------------------------------------------------------------
// 2. NewFromJSON creates valid UnifiedInput
// ---------------------------------------------------------------------------

func TestNewFromJSON(t *testing.T) {
	raw := []byte(`{"key":"value","num":42}`)
	input := NewFromJSON(raw)

	if input.InputID == "" {
		t.Fatal("InputID must not be empty")
	}
	if input.SourceType != SourceJSON {
		t.Fatalf("expected SourceType JSON, got %s", input.SourceType)
	}
	if input.Payload != string(raw) {
		t.Fatalf("payload mismatch: got %q", input.Payload)
	}
	if input.Priority != PriorityNormal {
		t.Fatalf("expected NORMAL priority, got %s", input.Priority)
	}
	if input.SourceMeta.Channel != "json" {
		t.Fatalf("expected channel 'json', got %q", input.SourceMeta.Channel)
	}
}

// ---------------------------------------------------------------------------
// 3. NewHeartbeat has TIMER source type and CRITICAL priority
// ---------------------------------------------------------------------------

func TestNewHeartbeat(t *testing.T) {
	hb := NewHeartbeat()

	if hb.SourceType != SourceTimer {
		t.Fatalf("expected SourceType TIMER, got %s", hb.SourceType)
	}
	if hb.Priority != PriorityCritical {
		t.Fatalf("expected CRITICAL priority, got %s", hb.Priority)
	}
	if hb.Payload != "heartbeat" {
		t.Fatalf("expected payload 'heartbeat', got %q", hb.Payload)
	}
	if hb.SourceMeta.Channel != "heartbeat" {
		t.Fatalf("expected channel 'heartbeat', got %q", hb.SourceMeta.Channel)
	}
	if hb.InputID == "" {
		t.Fatal("InputID must not be empty")
	}
}

// ---------------------------------------------------------------------------
// 4. NewFromWebhook creates valid UnifiedInput
// ---------------------------------------------------------------------------

func TestNewFromWebhook(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	input := NewFromWebhook(payload, "github")

	if input.InputID == "" {
		t.Fatal("InputID must not be empty")
	}
	if input.SourceType != SourceWebhook {
		t.Fatalf("expected SourceType WEBHOOK, got %s", input.SourceType)
	}
	if input.Payload != string(payload) {
		t.Fatalf("payload mismatch: got %q", input.Payload)
	}
	if input.SourceMeta.Sender != "github" {
		t.Fatalf("expected sender 'github', got %q", input.SourceMeta.Sender)
	}
	if input.SourceMeta.Channel != "webhook" {
		t.Fatalf("expected channel 'webhook', got %q", input.SourceMeta.Channel)
	}
	if input.Priority != PriorityNormal {
		t.Fatalf("expected NORMAL priority, got %s", input.Priority)
	}
}

// ---------------------------------------------------------------------------
// 5. SenseRegistry Register / Get / StartAll / StopAll
// ---------------------------------------------------------------------------

func TestSenseRegistry(t *testing.T) {
	reg := NewSenseRegistry()

	// Register two mock senses.
	m1 := newMockSense("telegram")
	m2 := newMockSense("cli")
	reg.Register(m1)
	reg.Register(m2)

	// Get returns the correct sense.
	if got := reg.Get("telegram"); got != m1 {
		t.Fatal("Get('telegram') did not return m1")
	}
	if got := reg.Get("cli"); got != m2 {
		t.Fatal("Get('cli') did not return m2")
	}
	if got := reg.Get("nonexistent"); got != nil {
		t.Fatal("Get for unknown name should return nil")
	}

	// StartAll starts every registered sense.
	ctx := context.Background()
	out := make(chan *UnifiedInput, 10)
	if err := reg.StartAll(ctx, out); err != nil {
		t.Fatalf("StartAll failed: %v", err)
	}

	m1.mu.Lock()
	if !m1.started {
		t.Fatal("m1 was not started")
	}
	m1.mu.Unlock()

	m2.mu.Lock()
	if !m2.started {
		t.Fatal("m2 was not started")
	}
	m2.mu.Unlock()

	// StopAll stops every registered sense.
	if err := reg.StopAll(); err != nil {
		t.Fatalf("StopAll failed: %v", err)
	}

	m1.mu.Lock()
	if !m1.stopped {
		t.Fatal("m1 was not stopped")
	}
	m1.mu.Unlock()

	m2.mu.Lock()
	if !m2.stopped {
		t.Fatal("m2 was not stopped")
	}
	m2.mu.Unlock()
}

// ---------------------------------------------------------------------------
// 6. UnifiedInput JSON serialization roundtrip
// ---------------------------------------------------------------------------

func TestUnifiedInputJSONRoundtrip(t *testing.T) {
	original := &UnifiedInput{
		InputID:    "test-id-123",
		SourceType: SourceTelegram,
		SourceMeta: SourceMeta{
			Timestamp: time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC),
			Channel:   "telegram",
			Sender:    "user42",
			URL:       "https://t.me/bot",
			Path:      "/webhook",
			Extra:     map[string]string{"chat_id": "12345"},
		},
		Payload: "Hello from Telegram!",
		Attachments: []Attachment{
			{Name: "photo.jpg", Type: "image/jpeg", Size: 1024, Path: "/tmp/photo.jpg"},
		},
		Priority:        PriorityHigh,
		CorrelationID:   "conv-abc",
		ResponseChannel: "telegram:12345",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify the JSON contains expected string representations.
	s := string(data)
	if !strings.Contains(s, `"TELEGRAM"`) {
		t.Fatalf("JSON should contain TELEGRAM source type, got: %s", s)
	}
	if !strings.Contains(s, `"HIGH"`) {
		t.Fatalf("JSON should contain HIGH priority, got: %s", s)
	}

	var decoded UnifiedInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare key fields.
	if decoded.InputID != original.InputID {
		t.Fatalf("InputID mismatch: %q vs %q", decoded.InputID, original.InputID)
	}
	if decoded.SourceType != original.SourceType {
		t.Fatalf("SourceType mismatch: %q vs %q", decoded.SourceType, original.SourceType)
	}
	if decoded.Payload != original.Payload {
		t.Fatalf("Payload mismatch: %q vs %q", decoded.Payload, original.Payload)
	}
	if decoded.Priority != original.Priority {
		t.Fatalf("Priority mismatch: %d vs %d", decoded.Priority, original.Priority)
	}
	if decoded.CorrelationID != original.CorrelationID {
		t.Fatalf("CorrelationID mismatch: %q vs %q", decoded.CorrelationID, original.CorrelationID)
	}
	if decoded.ResponseChannel != original.ResponseChannel {
		t.Fatalf("ResponseChannel mismatch: %q vs %q", decoded.ResponseChannel, original.ResponseChannel)
	}
	if decoded.SourceMeta.Sender != original.SourceMeta.Sender {
		t.Fatalf("Sender mismatch: %q vs %q", decoded.SourceMeta.Sender, original.SourceMeta.Sender)
	}
	if decoded.SourceMeta.URL != original.SourceMeta.URL {
		t.Fatalf("URL mismatch: %q vs %q", decoded.SourceMeta.URL, original.SourceMeta.URL)
	}
	if len(decoded.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(decoded.Attachments))
	}
	if decoded.Attachments[0].Name != "photo.jpg" {
		t.Fatalf("attachment name mismatch: %q", decoded.Attachments[0].Name)
	}
	if decoded.Attachments[0].Size != 1024 {
		t.Fatalf("attachment size mismatch: %d", decoded.Attachments[0].Size)
	}
	if decoded.SourceMeta.Extra["chat_id"] != "12345" {
		t.Fatalf("extra metadata mismatch: %v", decoded.SourceMeta.Extra)
	}
}

// ---------------------------------------------------------------------------
// 7. Priority ordering (LOW < NORMAL < HIGH < CRITICAL)
// ---------------------------------------------------------------------------

func TestPriorityOrdering(t *testing.T) {
	if PriorityLow >= PriorityNormal {
		t.Fatalf("LOW (%d) should be less than NORMAL (%d)", PriorityLow, PriorityNormal)
	}
	if PriorityNormal >= PriorityHigh {
		t.Fatalf("NORMAL (%d) should be less than HIGH (%d)", PriorityNormal, PriorityHigh)
	}
	if PriorityHigh >= PriorityCritical {
		t.Fatalf("HIGH (%d) should be less than CRITICAL (%d)", PriorityHigh, PriorityCritical)
	}

	// Verify String() labels.
	cases := []struct {
		p    Priority
		want string
	}{
		{PriorityLow, "LOW"},
		{PriorityNormal, "NORMAL"},
		{PriorityHigh, "HIGH"},
		{PriorityCritical, "CRITICAL"},
	}
	for _, tc := range cases {
		if tc.p.String() != tc.want {
			t.Fatalf("Priority(%d).String() = %q, want %q", tc.p, tc.p.String(), tc.want)
		}
	}
}
