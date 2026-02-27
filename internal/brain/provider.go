package brain

import (
	"context"
	"encoding/json"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// LLMRequest holds parameters for an LLM completion call.
type LLMRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// Tool represents a callable tool (MCP compatible).
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolCall represents a tool invocation by the LLM.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// LLMResponse holds the response from an LLM call.
type LLMResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	CostUSD      float64    `json:"cost_usd"`
	LatencyMs    int64      `json:"latency_ms"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	StopReason   string     `json:"stop_reason"`
}

// LLMProvider is the abstract interface for LLM backends.
type LLMProvider interface {
	Complete(ctx context.Context, req LLMRequest) (*LLMResponse, error)
	Name() string
	Models() []string
}
