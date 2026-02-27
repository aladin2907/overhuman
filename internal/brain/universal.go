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

// ---------------------------------------------------------------------------
// UniversalProvider — works with ANY OpenAI-compatible API endpoint.
//
// Supported backends (anything that speaks OpenAI /v1/chat/completions):
//   - OpenAI          (https://api.openai.com)
//   - Anthropic       (https://api.anthropic.com — via OpenAI compat layer)
//   - Ollama          (http://localhost:11434)
//   - LM Studio       (http://localhost:1234)
//   - Together AI     (https://api.together.xyz)
//   - Groq            (https://api.groq.com/openai)
//   - OpenRouter      (https://openrouter.ai/api)
//   - vLLM/TGI        (http://localhost:8000)
//   - Any local/remote OpenAI-compatible server
// ---------------------------------------------------------------------------

// ProviderConfig describes how to connect to an LLM provider.
type ProviderConfig struct {
	// Name is a human-readable label for this provider (e.g., "openai", "ollama", "local").
	Name string `json:"name"`

	// BaseURL is the API base URL (e.g., "https://api.openai.com", "http://localhost:11434").
	// The /v1/chat/completions path is appended automatically.
	BaseURL string `json:"base_url"`

	// APIKey is the bearer token. Empty for local models (Ollama, LM Studio).
	APIKey string `json:"api_key,omitempty"`

	// DefaultModel is used when the request doesn't specify a model.
	DefaultModel string `json:"default_model"`

	// Models lists available models with their tiers and costs.
	// If empty, a single model entry is created from DefaultModel.
	Models []ModelConfig `json:"models,omitempty"`

	// AuthHeader overrides the authorization header name.
	// Default: "Authorization" with "Bearer <key>" value.
	// Set to "x-api-key" for Anthropic native API.
	AuthHeader string `json:"auth_header,omitempty"`

	// AuthPrefix overrides the auth value prefix. Default: "Bearer ".
	AuthPrefix string `json:"auth_prefix,omitempty"`

	// ExtraHeaders are sent with every request (e.g., "anthropic-version: 2023-06-01").
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`

	// CompletionsPath overrides the API path. Default: "/v1/chat/completions".
	CompletionsPath string `json:"completions_path,omitempty"`

	// TimeoutSeconds overrides the HTTP timeout. Default: 120.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// MaxTokensDefault is the default max_tokens if not specified in request.
	// Default: 4096.
	MaxTokensDefault int `json:"max_tokens_default,omitempty"`
}

// ModelConfig describes a single model available from a provider.
type ModelConfig struct {
	ID        string  `json:"id"`                  // Model identifier (e.g., "gpt-4o", "llama3.3:70b")
	Tier      string  `json:"tier"`                // "cheap", "mid", "powerful"
	CostPer1K float64 `json:"cost_per_1k"`         // Approximate cost per 1K tokens (0 for local)
	InputCostPerM  float64 `json:"input_cost_per_m,omitempty"`  // Input cost per 1M tokens
	OutputCostPerM float64 `json:"output_cost_per_m,omitempty"` // Output cost per 1M tokens
}

// UniversalProvider implements LLMProvider for any OpenAI-compatible endpoint.
type UniversalProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewUniversalProvider creates a provider from config.
func NewUniversalProvider(cfg ProviderConfig) *UniversalProvider {
	if cfg.CompletionsPath == "" {
		cfg.CompletionsPath = "/v1/chat/completions"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}
	if cfg.MaxTokensDefault <= 0 {
		cfg.MaxTokensDefault = 4096
	}
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	if cfg.AuthPrefix == "" {
		cfg.AuthPrefix = "Bearer "
	}
	// Ensure at least one model entry.
	if len(cfg.Models) == 0 && cfg.DefaultModel != "" {
		cfg.Models = []ModelConfig{{
			ID:   cfg.DefaultModel,
			Tier: "mid",
		}}
	}

	return &UniversalProvider{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
	}
}

// Name returns the provider name.
func (p *UniversalProvider) Name() string { return p.config.Name }

// Models returns the list of available model IDs.
func (p *UniversalProvider) Models() []string {
	var ids []string
	for _, m := range p.config.Models {
		ids = append(ids, m.ID)
	}
	return ids
}

// ModelEntries returns models as ModelEntry for the router.
func (p *UniversalProvider) ModelEntries() []ModelEntry {
	var entries []ModelEntry
	for _, m := range p.config.Models {
		tier := TierMid
		switch m.Tier {
		case "cheap":
			tier = TierCheap
		case "powerful":
			tier = TierPowerful
		}
		entries = append(entries, ModelEntry{
			ID:        m.ID,
			Provider:  p.config.Name,
			Tier:      tier,
			CostPer1K: m.CostPer1K,
		})
	}
	return entries
}

// Complete sends a chat completion request.
func (p *UniversalProvider) Complete(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.config.MaxTokensDefault
	}

	// Build messages.
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
	if maxTokens > 0 {
		or.MaxTokens = &maxTokens
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
		return nil, fmt.Errorf("%s: marshal request: %w", p.config.Name, err)
	}

	url := strings.TrimRight(p.config.BaseURL, "/") + p.config.CompletionsPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", p.config.Name, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Auth header.
	if p.config.APIKey != "" {
		httpReq.Header.Set(p.config.AuthHeader, p.config.AuthPrefix+p.config.APIKey)
	}

	// Extra headers.
	for k, v := range p.config.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: http request: %w", p.config.Name, err)
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read response: %w", p.config.Name, err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openaiErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("%s: API error %d: %s: %s",
				p.config.Name, resp.StatusCode, errResp.Error.Type, errResp.Error.Message)
		}
		return nil, fmt.Errorf("%s: API error %d: %s", p.config.Name, resp.StatusCode, string(respBody))
	}

	var or2 openaiResponse
	if err := json.Unmarshal(respBody, &or2); err != nil {
		return nil, fmt.Errorf("%s: unmarshal response: %w", p.config.Name, err)
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
	result.CostUSD = p.calculateCost(model, result.InputTokens, result.OutputTokens)

	return result, nil
}

// calculateCost computes cost based on model config.
func (p *UniversalProvider) calculateCost(model string, inputTokens, outputTokens int) float64 {
	for _, m := range p.config.Models {
		if m.ID == model || strings.Contains(model, m.ID) {
			if m.InputCostPerM > 0 || m.OutputCostPerM > 0 {
				return float64(inputTokens)/1_000_000*m.InputCostPerM +
					float64(outputTokens)/1_000_000*m.OutputCostPerM
			}
			if m.CostPer1K > 0 {
				return float64(inputTokens+outputTokens) / 1000 * m.CostPer1K
			}
			return 0 // Free (local model)
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Preset configs for popular providers
// ---------------------------------------------------------------------------

// OpenAIConfig returns a ProviderConfig for OpenAI.
func OpenAIConfig(apiKey string) ProviderConfig {
	return ProviderConfig{
		Name:         "openai",
		BaseURL:      "https://api.openai.com",
		APIKey:       apiKey,
		DefaultModel: "o4-mini",
		Models: []ModelConfig{
			{ID: "o4-mini", Tier: "cheap", InputCostPerM: 1.10, OutputCostPerM: 4.40},
			{ID: "o3", Tier: "mid", InputCostPerM: 2.00, OutputCostPerM: 8.00},
			{ID: "o3-pro", Tier: "powerful", InputCostPerM: 20.0, OutputCostPerM: 80.0},
			{ID: "gpt-4.1-nano", Tier: "cheap", InputCostPerM: 0.10, OutputCostPerM: 0.40},
			{ID: "gpt-4.1-mini", Tier: "cheap", InputCostPerM: 0.40, OutputCostPerM: 1.60},
			{ID: "gpt-4.1", Tier: "mid", InputCostPerM: 2.00, OutputCostPerM: 8.00},
		},
	}
}

// AnthropicConfig returns a ProviderConfig for Anthropic Claude.
func AnthropicConfig(apiKey string) ProviderConfig {
	return ProviderConfig{
		Name:            "claude",
		BaseURL:         "https://api.anthropic.com",
		APIKey:          apiKey,
		DefaultModel:    "claude-sonnet-4-20250514",
		AuthHeader:      "x-api-key",
		AuthPrefix:      "",
		CompletionsPath: "/v1/messages",
		ExtraHeaders:    map[string]string{"anthropic-version": "2023-06-01"},
		Models: []ModelConfig{
			{ID: "claude-haiku-4-5-20251015", Tier: "cheap", InputCostPerM: 0.80, OutputCostPerM: 4.0},
			{ID: "claude-sonnet-4-20250514", Tier: "mid", InputCostPerM: 3.0, OutputCostPerM: 15.0},
			{ID: "claude-sonnet-4-6-20260217", Tier: "mid", InputCostPerM: 3.0, OutputCostPerM: 15.0},
			{ID: "claude-opus-4-20250514", Tier: "powerful", InputCostPerM: 15.0, OutputCostPerM: 75.0},
			{ID: "claude-opus-4-6-20260205", Tier: "powerful", InputCostPerM: 15.0, OutputCostPerM: 75.0},
		},
	}
}

// OllamaConfig returns a ProviderConfig for Ollama (local).
func OllamaConfig(model string) ProviderConfig {
	if model == "" {
		model = "llama3.3"
	}
	return ProviderConfig{
		Name:         "ollama",
		BaseURL:      "http://localhost:11434",
		DefaultModel: model,
		Models: []ModelConfig{
			{ID: model, Tier: "mid", CostPer1K: 0}, // Free, local
		},
	}
}

// LMStudioConfig returns a ProviderConfig for LM Studio (local).
func LMStudioConfig(model string) ProviderConfig {
	if model == "" {
		model = "local-model"
	}
	return ProviderConfig{
		Name:         "lmstudio",
		BaseURL:      "http://localhost:1234",
		DefaultModel: model,
		Models: []ModelConfig{
			{ID: model, Tier: "mid", CostPer1K: 0},
		},
	}
}

// OpenRouterConfig returns a ProviderConfig for OpenRouter.
func OpenRouterConfig(apiKey string) ProviderConfig {
	return ProviderConfig{
		Name:         "openrouter",
		BaseURL:      "https://openrouter.ai/api",
		APIKey:       apiKey,
		DefaultModel: "anthropic/claude-sonnet-4-6-20260217",
		Models: []ModelConfig{
			{ID: "anthropic/claude-haiku-4-5-20251015", Tier: "cheap", InputCostPerM: 0.80, OutputCostPerM: 4.0},
			{ID: "anthropic/claude-sonnet-4-6-20260217", Tier: "mid", InputCostPerM: 3.0, OutputCostPerM: 15.0},
			{ID: "anthropic/claude-opus-4-6-20260205", Tier: "powerful", InputCostPerM: 15.0, OutputCostPerM: 75.0},
			{ID: "openai/o4-mini", Tier: "cheap", InputCostPerM: 1.10, OutputCostPerM: 4.40},
			{ID: "openai/o3", Tier: "mid", InputCostPerM: 2.00, OutputCostPerM: 8.00},
			{ID: "google/gemini-2.5-pro", Tier: "powerful", InputCostPerM: 1.25, OutputCostPerM: 10.0},
		},
	}
}

// GroqConfig returns a ProviderConfig for Groq.
func GroqConfig(apiKey string) ProviderConfig {
	return ProviderConfig{
		Name:         "groq",
		BaseURL:      "https://api.groq.com/openai",
		APIKey:       apiKey,
		DefaultModel: "llama-3.3-70b-versatile",
		Models: []ModelConfig{
			{ID: "llama-3.3-70b-versatile", Tier: "mid", InputCostPerM: 0.59, OutputCostPerM: 0.79},
			{ID: "llama-3.1-8b-instant", Tier: "cheap", InputCostPerM: 0.05, OutputCostPerM: 0.08},
			{ID: "qwen-qwq-32b", Tier: "mid", InputCostPerM: 0.29, OutputCostPerM: 0.39},
			{ID: "deepseek-r1-distill-llama-70b", Tier: "mid", InputCostPerM: 0.75, OutputCostPerM: 0.99},
		},
	}
}

// TogetherConfig returns a ProviderConfig for Together AI.
func TogetherConfig(apiKey string) ProviderConfig {
	return ProviderConfig{
		Name:         "together",
		BaseURL:      "https://api.together.xyz",
		APIKey:       apiKey,
		DefaultModel: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
		Models: []ModelConfig{
			{ID: "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo", Tier: "cheap", InputCostPerM: 0.18, OutputCostPerM: 0.18},
			{ID: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", Tier: "mid", InputCostPerM: 0.88, OutputCostPerM: 0.88},
		},
	}
}

// CustomConfig returns a ProviderConfig for a custom OpenAI-compatible endpoint.
func CustomConfig(name, baseURL, apiKey, model string) ProviderConfig {
	return ProviderConfig{
		Name:         name,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		DefaultModel: model,
		Models: []ModelConfig{
			{ID: model, Tier: "mid", CostPer1K: 0},
		},
	}
}
