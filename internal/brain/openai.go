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

// openaiPricing maps model identifier substrings to (input, output) cost per 1M tokens.
var openaiPricing = map[string][2]float64{
	"gpt-4o-mini": {0.15, 0.60},
	"gpt-4o":      {2.50, 10.0},
}

// OpenAIOption configures an OpenAIProvider.
type OpenAIOption func(*OpenAIProvider)

// WithOpenAIBaseURL overrides the API base URL.
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.baseURL = url
	}
}

// WithOpenAIHTTPClient sets a custom HTTP client.
func WithOpenAIHTTPClient(c *http.Client) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.client = c
	}
}

// WithOpenAIDefaultModel sets the default model.
func WithOpenAIDefaultModel(model string) OpenAIOption {
	return func(p *OpenAIProvider) {
		p.defaultModel = model
	}
}

// OpenAIProvider implements LLMProvider for the OpenAI API.
type OpenAIProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	defaultModel string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey string, opts ...OpenAIOption) *OpenAIProvider {
	p := &OpenAIProvider{
		apiKey:       apiKey,
		baseURL:      "https://api.openai.com",
		client:       &http.Client{Timeout: 120 * time.Second},
		defaultModel: "gpt-4o",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string { return "openai" }

// Models returns the list of supported models.
func (p *OpenAIProvider) Models() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
	}
}

// openaiRequest is the OpenAI chat completions request body.
type openaiRequest struct {
	Model               string           `json:"model"`
	Messages            []openaiMsg      `json:"messages"`
	Temperature         *float64         `json:"temperature,omitempty"`
	MaxTokens           *int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int             `json:"max_completion_tokens,omitempty"`
	Tools               []openaiToolDef  `json:"tools,omitempty"`
}

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiToolDef struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// openaiResponse is the OpenAI chat completions response body.
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiErrorResponse is used to parse API errors.
type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Complete sends a completion request to the OpenAI API.
func (p *OpenAIProvider) Complete(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	var msgs []openaiMsg
	for _, m := range req.Messages {
		msgs = append(msgs, openaiMsg{Role: m.Role, Content: m.Content})
	}

	or := openaiRequest{
		Model:    model,
		Messages: msgs,
	}

	if req.Temperature > 0 {
		t := req.Temperature
		or.Temperature = &t
	}

	if req.MaxTokens > 0 {
		mt := req.MaxTokens
		if useMaxCompletionTokens(model) {
			or.MaxCompletionTokens = &mt
		} else {
			or.MaxTokens = &mt
		}
	}

	for _, tool := range req.Tools {
		or.Tools = append(or.Tools, openaiToolDef{
			Type: "function",
			Function: openaiFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	body, err := json.Marshal(or)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openaiErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("openai: API error %d: %s: %s", resp.StatusCode, errResp.Error.Type, errResp.Error.Message)
		}
		return nil, fmt.Errorf("openai: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var or2 openaiResponse
	if err := json.Unmarshal(respBody, &or2); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	result := &LLMResponse{
		Model:        or2.Model,
		InputTokens:  or2.Usage.PromptTokens,
		OutputTokens: or2.Usage.CompletionTokens,
		LatencyMs:    latency,
	}

	if len(or2.Choices) > 0 {
		choice := or2.Choices[0]
		result.Content = choice.Message.Content
		result.StopReason = choice.FinishReason

		for _, tc := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	// Calculate cost.
	result.CostUSD = openaiCalculateCost(or2.Model, or2.Usage.PromptTokens, or2.Usage.CompletionTokens)

	return result, nil
}

// useMaxCompletionTokens returns true if the model requires max_completion_tokens
// instead of max_tokens. Newer OpenAI models (o-series, gpt-4.1+, gpt-5+) use this.
func useMaxCompletionTokens(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "o1") ||
		strings.HasPrefix(m, "o3") ||
		strings.HasPrefix(m, "o4") ||
		strings.HasPrefix(m, "gpt-4.1") ||
		strings.HasPrefix(m, "gpt-4o") ||
		strings.HasPrefix(m, "gpt-5")
}

// openaiCalculateCost computes USD cost based on model and token counts.
func openaiCalculateCost(model string, inputTokens, outputTokens int) float64 {
	var pricing [2]float64
	found := false

	// Check most specific match first (gpt-4o-mini before gpt-4o).
	for _, key := range []string{"gpt-4o-mini", "gpt-4o"} {
		if strings.Contains(model, key) {
			pricing = openaiPricing[key]
			found = true
			break
		}
	}
	if !found {
		// Default to gpt-4o pricing.
		pricing = openaiPricing["gpt-4o"]
	}

	inputCost := float64(inputTokens) / 1_000_000 * pricing[0]
	outputCost := float64(outputTokens) / 1_000_000 * pricing[1]
	return inputCost + outputCost
}
