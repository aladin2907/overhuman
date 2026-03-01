package genui

import (
	"encoding/json"
	"fmt"
)

// WSMessageType defines WebSocket message types.
type WSMessageType string

const (
	// Server → Client message types.
	WSMsgUIFull      WSMessageType = "ui_full"       // Full GeneratedUI delivery
	WSMsgUIStream    WSMessageType = "ui_stream"     // Streaming chunk
	WSMsgActionResult WSMessageType = "action_result" // Response to client action
	WSMsgError       WSMessageType = "error"         // Error notification
	WSMsgPong        WSMessageType = "pong"          // Keepalive response

	// Client → Server message types.
	WSMsgAction     WSMessageType = "action"      // User clicked an action button
	WSMsgInput      WSMessageType = "input"       // User text input
	WSMsgPing       WSMessageType = "ping"        // Keepalive request
	WSMsgUIFeedback WSMessageType = "ui_feedback" // UI interaction data for reflection
	WSMsgCancel     WSMessageType = "cancel"      // Emergency stop
)

// WSMessage is the top-level WebSocket message envelope.
type WSMessage struct {
	Type    WSMessageType   `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WSUIFullPayload is the payload for WSMsgUIFull messages.
type WSUIFullPayload struct {
	TaskID  string            `json:"task_id"`
	HTML    string            `json:"html"`
	Actions []GeneratedAction `json:"actions,omitempty"`
	Meta    UIMeta            `json:"meta,omitempty"`
	Thought *ThoughtLog       `json:"thought,omitempty"`
}

// WSUIStreamPayload is the payload for WSMsgUIStream messages.
type WSUIStreamPayload struct {
	Chunk string `json:"chunk"`
	Done  bool   `json:"done"`
}

// WSActionPayload is the payload for WSMsgAction messages (client → server).
type WSActionPayload struct {
	ActionID string          `json:"action_id"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// WSActionResultPayload is the payload for WSMsgActionResult messages.
type WSActionResultPayload struct {
	ActionID string `json:"action_id"`
	Success  bool   `json:"success"`
	Result   string `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}

// WSInputPayload is the payload for WSMsgInput messages.
type WSInputPayload struct {
	Text string `json:"text"`
}

// WSErrorPayload is the payload for WSMsgError messages.
type WSErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WSCancelPayload is the payload for WSMsgCancel messages.
type WSCancelPayload struct {
	Reason string `json:"reason,omitempty"`
}

// WSUIFeedbackPayload is the payload for WSMsgUIFeedback messages.
type WSUIFeedbackPayload struct {
	TaskID       string   `json:"task_id"`
	Scrolled     bool     `json:"scrolled"`
	TimeToAction int64    `json:"time_to_action_ms"`
	ActionsUsed  []string `json:"actions_used"`
	Dismissed    bool     `json:"dismissed"`
}

// ParseWSMessage decodes a raw JSON byte slice into a WSMessage.
func ParseWSMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse ws message: %w", err)
	}
	if msg.Type == "" {
		return nil, fmt.Errorf("parse ws message: missing type field")
	}
	return &msg, nil
}

// EncodeWSMessage encodes a WSMessage to JSON bytes.
func EncodeWSMessage(msg *WSMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// NewWSMessage creates a WSMessage with the given type and payload.
func NewWSMessage(msgType WSMessageType, payload interface{}) (*WSMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode ws payload: %w", err)
	}
	return &WSMessage{Type: msgType, Payload: raw}, nil
}

// NewUIFullMessage creates a WSMsgUIFull message from a GeneratedUI.
func NewUIFullMessage(ui *GeneratedUI) (*WSMessage, error) {
	payload := WSUIFullPayload{
		TaskID:  ui.TaskID,
		HTML:    ui.Code,
		Actions: ui.Actions,
		Meta:    ui.Meta,
		Thought: ui.Thought,
	}
	return NewWSMessage(WSMsgUIFull, payload)
}

// NewUIStreamMessage creates a WSMsgUIStream message for a single chunk.
func NewUIStreamMessage(chunk string, done bool) (*WSMessage, error) {
	return NewWSMessage(WSMsgUIStream, WSUIStreamPayload{
		Chunk: chunk,
		Done:  done,
	})
}

// NewErrorMessage creates a WSMsgError message.
func NewErrorMessage(code int, message string) (*WSMessage, error) {
	return NewWSMessage(WSMsgError, WSErrorPayload{
		Code:    code,
		Message: message,
	})
}

// ParseActionPayload extracts WSActionPayload from a WSMessage.
func ParseActionPayload(msg *WSMessage) (*WSActionPayload, error) {
	var p WSActionPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, fmt.Errorf("parse action payload: %w", err)
	}
	return &p, nil
}

// ParseInputPayload extracts WSInputPayload from a WSMessage.
func ParseInputPayload(msg *WSMessage) (*WSInputPayload, error) {
	var p WSInputPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, fmt.Errorf("parse input payload: %w", err)
	}
	return &p, nil
}

// ParseCancelPayload extracts WSCancelPayload from a WSMessage.
func ParseCancelPayload(msg *WSMessage) (*WSCancelPayload, error) {
	var p WSCancelPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, fmt.Errorf("parse cancel payload: %w", err)
	}
	return &p, nil
}

// ParseUIFeedbackPayload extracts WSUIFeedbackPayload from a WSMessage.
func ParseUIFeedbackPayload(msg *WSMessage) (*WSUIFeedbackPayload, error) {
	var p WSUIFeedbackPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, fmt.Errorf("parse ui feedback payload: %w", err)
	}
	return &p, nil
}
