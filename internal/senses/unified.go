// Package senses defines the normalized input model and interface for all
// incoming signal adapters in the Overhuman system.
package senses

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// SourceType — enum for the origin of a signal.
// ---------------------------------------------------------------------------

// SourceType identifies the kind of input channel that produced a signal.
type SourceType string

const (
	SourceText     SourceType = "TEXT"
	SourceJSON     SourceType = "JSON"
	SourceWebhook  SourceType = "WEBHOOK"
	SourceFile     SourceType = "FILE"
	SourceTimer    SourceType = "TIMER"
	SourceTelegram SourceType = "TELEGRAM"
	SourceSlack    SourceType = "SLACK"
	SourceDiscord  SourceType = "DISCORD"
	SourceEmail    SourceType = "EMAIL"
	SourceAPI      SourceType = "API"
)

// ---------------------------------------------------------------------------
// Priority — enum for urgency levels.
// ---------------------------------------------------------------------------

// Priority represents how urgently a signal should be processed.
type Priority int

const (
	PriorityLow      Priority = 0
	PriorityNormal   Priority = 1
	PriorityHigh     Priority = 2
	PriorityCritical Priority = 3
)

// String returns the human-readable label for a priority level.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityNormal:
		return "NORMAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// MarshalJSON encodes a Priority as its string label.
func (p Priority) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON decodes a Priority from its string label.
func (p *Priority) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "LOW":
		*p = PriorityLow
	case "NORMAL":
		*p = PriorityNormal
	case "HIGH":
		*p = PriorityHigh
	case "CRITICAL":
		*p = PriorityCritical
	default:
		return fmt.Errorf("unknown priority: %s", s)
	}
	return nil
}

// ---------------------------------------------------------------------------
// SourceMeta — metadata about where a signal came from.
// ---------------------------------------------------------------------------

// SourceMeta carries contextual metadata about the origin of a signal.
type SourceMeta struct {
	Timestamp time.Time         `json:"timestamp"`
	Channel   string            `json:"channel,omitempty"`
	Sender    string            `json:"sender,omitempty"`
	URL       string            `json:"url,omitempty"`
	Path      string            `json:"path,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// ---------------------------------------------------------------------------
// Attachment — a file or binary blob attached to a signal.
// ---------------------------------------------------------------------------

// Attachment describes a file or binary blob that accompanies a signal.
type Attachment struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

// ---------------------------------------------------------------------------
// UnifiedInput — the canonical representation of every incoming signal.
// ---------------------------------------------------------------------------

// UnifiedInput is the normalized format for ALL incoming signals in the
// Overhuman system. Every adapter converts its native format into this
// structure before handing it off to the processing pipeline.
type UnifiedInput struct {
	InputID         string       `json:"input_id"`
	SourceType      SourceType   `json:"source_type"`
	SourceMeta      SourceMeta   `json:"source_meta"`
	Payload         string       `json:"payload"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	Priority        Priority     `json:"priority"`
	CorrelationID   string       `json:"correlation_id,omitempty"`
	ResponseChannel string       `json:"response_channel,omitempty"`
}

// ---------------------------------------------------------------------------
// Factory methods
// ---------------------------------------------------------------------------

// newUUID generates a random UUID v4 string.
func newUUID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

// NewFromText creates a UnifiedInput from a plain text string.
func NewFromText(text string) *UnifiedInput {
	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: SourceText,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
			Channel:   "text",
		},
		Payload:  text,
		Priority: PriorityNormal,
	}
}

// NewFromJSON creates a UnifiedInput from a raw JSON byte slice.
// The entire byte slice is stored as the Payload string.
func NewFromJSON(data []byte) *UnifiedInput {
	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: SourceJSON,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
			Channel:   "json",
		},
		Payload:  string(data),
		Priority: PriorityNormal,
	}
}

// NewHeartbeat creates a timer-based heartbeat signal with CRITICAL priority.
// Heartbeats are internal pulses used to drive periodic self-checks and
// autonomous behavior.
func NewHeartbeat() *UnifiedInput {
	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: SourceTimer,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
			Channel:   "heartbeat",
		},
		Payload:  "heartbeat",
		Priority: PriorityCritical,
	}
}

// NewFromWebhook creates a UnifiedInput from a webhook payload and source
// identifier.
func NewFromWebhook(payload []byte, source string) *UnifiedInput {
	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: SourceWebhook,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
			Channel:   "webhook",
			Sender:    source,
		},
		Payload:  string(payload),
		Priority: PriorityNormal,
	}
}

// NewUnifiedInput creates a UnifiedInput for a given source type and payload.
func NewUnifiedInput(sourceType SourceType, payload string) *UnifiedInput {
	return &UnifiedInput{
		InputID:    newUUID(),
		SourceType: sourceType,
		SourceMeta: SourceMeta{
			Timestamp: time.Now(),
		},
		Payload:  payload,
		Priority: PriorityNormal,
	}
}
