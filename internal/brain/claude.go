package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// claudePricing maps model family to (input, output) cost per 1M tokens in USD.
var claudePricing = map[string][2]float64{
	"haiku":  {0.25, 1.25},
	"sonnet": {3.0, 15.0},
	"opus":   {15.0, 75.0},
}

// ClaudeOption configures a ClaudeProvider.
type ClaudeOption func(*ClaudeProvider)

// WithClaudeBaseURL overrides the API base URL (useful for testing).
func WithClaudeBaseURL(url string) ClaudeOption {
	return func(p *ClaudeProvider) {
		p.baseURL = url
	}
}

// WithClaudeHTTPClient sets a custom HTTP client.
func WithClaudeHTTPClient(c *http.Client) ClaudeOption {
	return func(p *ClaudeProvider) {
		p.client = c
	}
}

// WithClaudeDefaultModel sets the default model when none is specified in the request.
func WithClaudeDefaultModel(model string) ClaudeOption {
	return func(p *ClaudeProvider) {
		p.defaultModel = model
	}
}

// ClaudeProvider implements LLMProvider for the Anthropic Claude API.
type ClaudeProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	defaultModel string
}

// NewClaudeProvider creates a new Claude provider.
func NewClaudeProvider(apiKey string, opts ...ClaudeOption) *ClaudeProvider {
	p := &ClaudeProvider{
		apiKey:       apiKey,
		baseURL:      "https://api.anthropic.com",
		client:       &http.Client{Timeout: 120 * time.Second},
		defaultModel: "claude-sonnet-4-20250514",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the provider name.
func (p *ClaudeProvider) Name() string { return "claude" }

// Models returns the list of supported models.
func (p *ClaudeProvider) Models() []string {
	return []string{
		"claude-haiku-3-5-20241022",
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
	}
}

// claudeRequest is the Anthropic API request body.
type claudeRequest struct {
	Model       string         `json:"model"`
	MaxTokens   int            `json:"max_tokens"`
	Messages    []claudeMsg    `json:"messages"`
	System      string         `json:"system,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	Tools       []claudeTool   `json:"tools,omitempty"`
}

type claudeMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// claudeResponse is the Anthropic API response body.
type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// claudeErrorResponse is used to parse API errors.
type claudeErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends a completion request to the Claude API.
func (p *ClaudeProvider) Complete(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Separate system message from user/assistant messages.
	var systemPrompt string
	var msgs []claudeMsg
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			msgs = append(msgs, claudeMsg{Role: m.Role, Content: m.Content})
		}
	}

	cr := claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  msgs,
		System:    systemPrompt,
	}

	if req.Temperature > 0 {
		t := req.Temperature
		cr.Temperature = &t
	}

	for _, tool := range req.Tools {
		cr.Tools = append(cr.Tools, claudeTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	body, err := json.Marshal(cr)
	if err != nil {
		return nil, fmt.Errorf("claude: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude: http request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("claude: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp claudeErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("claude: API error %d: %s: %s", resp.StatusCode, errResp.Error.Type, errResp.Error.Message)
		}
		return nil, fmt.Errorf("claude: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var cr2 claudeResponse
	if err := json.Unmarshal(respBody, &cr2); err != nil {
		return nil, fmt.Errorf("claude: unmarshal response: %w", err)
	}

	result := &LLMResponse{
		Model:        cr2.Model,
		InputTokens:  cr2.Usage.InputTokens,
		OutputTokens: cr2.Usage.OutputTokens,
		LatencyMs:    latency,
		StopReason:   cr2.StopReason,
	}

	// Extract text content and tool calls.
	var textParts []string
	for _, block := range cr2.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	result.Content = strings.Join(textParts, "")

	// Calculate cost.
	result.CostUSD = claudeCalculateCost(cr2.Model, cr2.Usage.InputTokens, cr2.Usage.OutputTokens)

	return result, nil
}

// claudeCalculateCost computes USD cost based on model and token counts.
func claudeCalculateCost(model string, inputTokens, outputTokens int) float64 {
	var pricing [2]float64
	found := false
	for family, p := range claudePricing {
		if strings.Contains(model, family) {
			pricing = p
			found = true
			break
		}
	}
	if !found {
		// Default to sonnet pricing for unknown models.
		pricing = claudePricing["sonnet"]
	}

	inputCost := float64(inputTokens) / 1_000_000 * pricing[0]
	outputCost := float64(outputTokens) / 1_000_000 * pricing[1]
	return inputCost + outputCost
}
