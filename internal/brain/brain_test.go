package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Claude Provider Tests ---

func TestClaudeProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers.
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version header")
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %s", r.URL.Path)
		}

		// Verify request body.
		var req claudeRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.System == "" {
			t.Error("expected system prompt")
		}
		if len(req.Messages) == 0 {
			t.Error("expected messages")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeResponse{
			ID:    "msg_123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text,omitempty"`
				ID    string          `json:"id,omitempty"`
				Name  string          `json:"name,omitempty"`
				Input json.RawMessage `json:"input,omitempty"`
			}{
				{Type: "text", Text: "Hello, world!"},
			},
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 100, OutputTokens: 50},
		})
	}))
	defer srv.Close()

	p := NewClaudeProvider("test-key", WithClaudeBaseURL(srv.URL))

	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "Hello, world!" {
		t.Errorf("content = %q, want %q", resp.Content, "Hello, world!")
	}
	if resp.InputTokens != 100 {
		t.Errorf("input tokens = %d, want 100", resp.InputTokens)
	}
	if resp.OutputTokens != 50 {
		t.Errorf("output tokens = %d, want 50", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want end_turn", resp.StopReason)
	}
	if resp.LatencyMs < 0 {
		t.Error("latency should be >= 0")
	}
}

func TestClaudeProvider_CostCalculation(t *testing.T) {
	tests := []struct {
		model  string
		input  int
		output int
		want   float64
	}{
		{"claude-haiku-3-5-20241022", 1_000_000, 1_000_000, 0.25 + 1.25},
		{"claude-sonnet-4-20250514", 1_000_000, 1_000_000, 3.0 + 15.0},
		{"claude-opus-4-20250514", 1_000_000, 1_000_000, 15.0 + 75.0},
		{"claude-sonnet-4-20250514", 100, 50, 0.0003 + 0.00075},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := claudeCalculateCost(tt.model, tt.input, tt.output)
			if fmt.Sprintf("%.6f", got) != fmt.Sprintf("%.6f", tt.want) {
				t.Errorf("cost = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestClaudeProvider_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "model not found",
			},
		})
	}))
	defer srv.Close()

	p := NewClaudeProvider("test-key", WithClaudeBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error should contain 'model not found', got: %v", err)
	}
}

func TestClaudeProvider_ToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(claudeResponse{
			Model: "claude-sonnet-4-20250514",
			Content: []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text,omitempty"`
				ID    string          `json:"id,omitempty"`
				Name  string          `json:"name,omitempty"`
				Input json.RawMessage `json:"input,omitempty"`
			}{
				{Type: "tool_use", ID: "call_1", Name: "search", Input: json.RawMessage(`{"query":"test"}`)},
			},
			StopReason: "tool_use",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 50, OutputTokens: 30},
		})
	}))
	defer srv.Close()

	p := NewClaudeProvider("test-key", WithClaudeBaseURL(srv.URL))
	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "search"}},
		Tools: []Tool{
			{Name: "search", Description: "Search the web", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("tool name = %q, want search", resp.ToolCalls[0].Name)
	}
}

// --- OpenAI Provider Tests ---

func TestOpenAIProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer auth")
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResponse{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []struct {
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
			}{
				{
					Index:        0,
					FinishReason: "stop",
					Message: struct {
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
					}{Role: "assistant", Content: "Response from GPT"},
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{PromptTokens: 80, CompletionTokens: 40, TotalTokens: 120},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key", WithOpenAIBaseURL(srv.URL))
	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "Response from GPT" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.InputTokens != 80 {
		t.Errorf("input = %d, want 80", resp.InputTokens)
	}
}

func TestOpenAIProvider_CostCalculation(t *testing.T) {
	got := openaiCalculateCost("gpt-4o", 1_000_000, 1_000_000)
	want := 2.50 + 10.0
	if fmt.Sprintf("%.2f", got) != fmt.Sprintf("%.2f", want) {
		t.Errorf("cost = %f, want %f", got, want)
	}

	gotMini := openaiCalculateCost("gpt-4o-mini", 1_000_000, 1_000_000)
	wantMini := 0.15 + 0.60
	if fmt.Sprintf("%.2f", gotMini) != fmt.Sprintf("%.2f", wantMini) {
		t.Errorf("mini cost = %f, want %f", gotMini, wantMini)
	}
}

// --- Router Tests ---

func TestModelRouter_SelectByComplexity(t *testing.T) {
	r := NewModelRouter()

	simple := r.Select("simple", 100.0)
	if !strings.Contains(simple, "haiku") && !strings.Contains(simple, "mini") {
		t.Errorf("simple should select cheap model, got %s", simple)
	}

	moderate := r.Select("moderate", 100.0)
	if !strings.Contains(moderate, "sonnet") && !strings.Contains(moderate, "gpt-4o") {
		t.Errorf("moderate should select mid model, got %s", moderate)
	}

	complex := r.Select("complex", 100.0)
	if !strings.Contains(complex, "opus") {
		t.Errorf("complex should select powerful model, got %s", complex)
	}
}

func TestModelRouter_BudgetDowngrade(t *testing.T) {
	r := NewModelRouter()

	// Low budget should force cheap.
	got := r.Select("complex", 0.05)
	if strings.Contains(got, "opus") || strings.Contains(got, "sonnet") {
		t.Errorf("low budget should force cheap, got %s", got)
	}

	// Moderate budget should downgrade powerful to mid.
	got2 := r.Select("complex", 0.50)
	if strings.Contains(got2, "opus") {
		t.Errorf("moderate budget should downgrade powerful, got %s", got2)
	}
}

// --- ContextAssembler Tests ---

func TestContextAssembler_AllLayers(t *testing.T) {
	ca := NewContextAssembler()

	msgs := ca.Assemble(ContextLayers{
		SystemPrompt:    "You are a helpful assistant.",
		TaskDescription: "Summarize this document",
		Tools:           []Tool{{Name: "search", Description: "Web search"}},
		RelevantMemory:  []string{"Previous summary was good"},
		RecentHistory: []Message{
			{Role: "user", Content: "Prev question"},
			{Role: "assistant", Content: "Prev answer"},
		},
		SKBInsights: []string{"Insight from another agent"},
	})

	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}

	// First should be system message combining all system parts.
	if msgs[0].Role != "system" {
		t.Errorf("first message should be system, got %s", msgs[0].Role)
	}
	if !strings.Contains(msgs[0].Content, "You are a helpful assistant.") {
		t.Error("system should contain soul")
	}
	if !strings.Contains(msgs[0].Content, "search") {
		t.Error("system should contain tools")
	}
	if !strings.Contains(msgs[0].Content, "Previous summary") {
		t.Error("system should contain memory")
	}
	if !strings.Contains(msgs[0].Content, "Insight from another agent") {
		t.Error("system should contain SKB insights")
	}

	// Should have non-system messages.
	foundTask := false
	foundHistory := false
	for _, m := range msgs[1:] {
		if strings.Contains(m.Content, "Summarize this document") {
			foundTask = true
		}
		if m.Content == "Prev question" {
			foundHistory = true
		}
	}
	if !foundTask {
		t.Error("should contain task description")
	}
	if !foundHistory {
		t.Error("should contain history")
	}
}

func TestContextAssembler_Truncation(t *testing.T) {
	ca := NewContextAssemblerWithLimit(10) // Very small limit.

	msgs := ca.Assemble(ContextLayers{
		SystemPrompt:    "Short soul",
		TaskDescription: "A very long task description that should definitely exceed our tiny token limit",
		RelevantMemory:  []string{"Some memory that should get truncated"},
		SKBInsights:     []string{"This should definitely get dropped entirely"},
	})

	// Should still have at least the system prompt.
	if len(msgs) == 0 {
		t.Fatal("should have at least some messages")
	}
	if msgs[0].Role != "system" {
		t.Error("first should be system")
	}
}

func TestContextAssembler_EmptyLayers(t *testing.T) {
	ca := NewContextAssembler()
	msgs := ca.Assemble(ContextLayers{})

	if len(msgs) != 0 {
		t.Errorf("empty layers should produce no messages, got %d", len(msgs))
	}
}

func TestContextAssembler_SystemPromptOnly(t *testing.T) {
	ca := NewContextAssembler()
	msgs := ca.Assemble(ContextLayers{
		SystemPrompt: "You are an AI.",
	})

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Error("should be system")
	}
	if msgs[0].Content != "You are an AI." {
		t.Errorf("content = %q", msgs[0].Content)
	}
}

// --- Provider Interface Tests ---

func TestClaudeProvider_ImplementsInterface(t *testing.T) {
	var _ LLMProvider = (*ClaudeProvider)(nil)
}

func TestOpenAIProvider_ImplementsInterface(t *testing.T) {
	var _ LLMProvider = (*OpenAIProvider)(nil)
}

func TestClaudeProvider_Name(t *testing.T) {
	p := NewClaudeProvider("key")
	if p.Name() != "claude" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAIProvider("key")
	if p.Name() != "openai" {
		t.Errorf("name = %q", p.Name())
	}
}

// --- Universal Provider Tests ---

func TestUniversalProvider_ImplementsInterface(t *testing.T) {
	var _ LLMProvider = (*UniversalProvider)(nil)
}

func TestUniversalProvider_Name(t *testing.T) {
	p := NewUniversalProvider(ProviderConfig{Name: "test-provider"})
	if p.Name() != "test-provider" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestUniversalProvider_Models(t *testing.T) {
	p := NewUniversalProvider(ProviderConfig{
		Name:         "test",
		DefaultModel: "model-a",
		Models: []ModelConfig{
			{ID: "model-a", Tier: "cheap"},
			{ID: "model-b", Tier: "mid"},
			{ID: "model-c", Tier: "powerful"},
		},
	})
	models := p.Models()
	if len(models) != 3 {
		t.Fatalf("models = %d, want 3", len(models))
	}
	if models[0] != "model-a" || models[1] != "model-b" || models[2] != "model-c" {
		t.Errorf("models = %v", models)
	}
}

func TestUniversalProvider_ModelEntries(t *testing.T) {
	p := NewUniversalProvider(ProviderConfig{
		Name: "myhost",
		Models: []ModelConfig{
			{ID: "fast", Tier: "cheap", CostPer1K: 0.001},
			{ID: "smart", Tier: "powerful", CostPer1K: 0.05},
		},
	})
	entries := p.ModelEntries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].Provider != "myhost" {
		t.Errorf("provider = %q", entries[0].Provider)
	}
	if entries[0].Tier != TierCheap {
		t.Errorf("tier = %q", entries[0].Tier)
	}
	if entries[1].Tier != TierPowerful {
		t.Errorf("tier = %q", entries[1].Tier)
	}
}

func TestUniversalProvider_DefaultModel(t *testing.T) {
	p := NewUniversalProvider(ProviderConfig{
		Name:         "test",
		DefaultModel: "my-model",
	})
	// Should auto-create a model entry.
	models := p.Models()
	if len(models) != 1 || models[0] != "my-model" {
		t.Errorf("models = %v", models)
	}
}

func TestUniversalProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("custom header = %q", r.Header.Get("X-Custom"))
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "resp-1",
			"model": "test-model",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]string{
						"role":    "assistant",
						"content": "Hello from universal provider!",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	p := NewUniversalProvider(ProviderConfig{
		Name:         "test-backend",
		BaseURL:      server.URL,
		APIKey:       "test-key",
		DefaultModel: "test-model",
		ExtraHeaders: map[string]string{"X-Custom": "value"},
		Models: []ModelConfig{
			{ID: "test-model", Tier: "mid", InputCostPerM: 1.0, OutputCostPerM: 2.0},
		},
	})

	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Content != "Hello from universal provider!" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Model != "test-model" {
		t.Errorf("model = %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("input tokens = %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("output tokens = %d", resp.OutputTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("stop reason = %q", resp.StopReason)
	}
}

func TestUniversalProvider_CompleteNoAuth(t *testing.T) {
	// Ollama-style: no API key needed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("should not have auth header for local provider")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "llama3.3",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "Local response"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3},
		})
	}))
	defer server.Close()

	p := NewUniversalProvider(OllamaConfig("llama3.3"))
	// Override base URL to mock server.
	p.config.BaseURL = server.URL

	resp, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Local response" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.CostUSD != 0 {
		t.Errorf("cost should be 0 for local, got %f", resp.CostUSD)
	}
}

func TestUniversalProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"type":    "auth_error",
				"message": "invalid key",
			},
		})
	}))
	defer server.Close()

	p := NewUniversalProvider(ProviderConfig{
		Name:         "test",
		BaseURL:      server.URL,
		APIKey:       "bad-key",
		DefaultModel: "model",
	})

	_, err := p.Complete(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid key") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestUniversalProvider_CostCalculation(t *testing.T) {
	p := NewUniversalProvider(ProviderConfig{
		Name: "test",
		Models: []ModelConfig{
			{ID: "model-a", Tier: "mid", InputCostPerM: 2.0, OutputCostPerM: 10.0},
			{ID: "model-b", Tier: "cheap", CostPer1K: 0.001},
		},
	})

	// Per-million pricing.
	cost := p.calculateCost("model-a", 1000, 500)
	expected := 1000.0/1_000_000*2.0 + 500.0/1_000_000*10.0
	if fmt.Sprintf("%.6f", cost) != fmt.Sprintf("%.6f", expected) {
		t.Errorf("cost = %f, want %f", cost, expected)
	}

	// Per-1K pricing.
	cost2 := p.calculateCost("model-b", 1000, 0)
	if cost2 != 0.001 {
		t.Errorf("cost = %f, want 0.001", cost2)
	}

	// Unknown model = free.
	cost3 := p.calculateCost("unknown", 1000, 1000)
	if cost3 != 0 {
		t.Errorf("unknown cost = %f", cost3)
	}
}

// --- Preset Config Tests ---

func TestOpenAIConfig(t *testing.T) {
	cfg := OpenAIConfig("sk-test")
	if cfg.Name != "openai" {
		t.Errorf("name = %q", cfg.Name)
	}
	if cfg.DefaultModel != "gpt-4o" {
		t.Errorf("model = %q", cfg.DefaultModel)
	}
	if len(cfg.Models) < 3 {
		t.Errorf("models = %d", len(cfg.Models))
	}
}

func TestOllamaConfig(t *testing.T) {
	cfg := OllamaConfig("mistral")
	if cfg.Name != "ollama" {
		t.Errorf("name = %q", cfg.Name)
	}
	if cfg.BaseURL != "http://localhost:11434" {
		t.Errorf("url = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		t.Error("ollama should have no API key")
	}
	if cfg.DefaultModel != "mistral" {
		t.Errorf("model = %q", cfg.DefaultModel)
	}
}

func TestGroqConfig(t *testing.T) {
	cfg := GroqConfig("gsk-test")
	if cfg.Name != "groq" {
		t.Errorf("name = %q", cfg.Name)
	}
	if !strings.Contains(cfg.BaseURL, "groq.com") {
		t.Errorf("url = %q", cfg.BaseURL)
	}
}

func TestCustomConfig(t *testing.T) {
	cfg := CustomConfig("myserver", "http://gpu-box:8080", "secret", "phi-3")
	if cfg.Name != "myserver" {
		t.Errorf("name = %q", cfg.Name)
	}
	if cfg.DefaultModel != "phi-3" {
		t.Errorf("model = %q", cfg.DefaultModel)
	}
}

// --- Router with Provider Filter Tests ---

func TestModelRouter_SetProvider(t *testing.T) {
	r := NewModelRouter()
	r.SetProvider("openai")

	got := r.Select("simple", 100.0)
	if !strings.Contains(got, "gpt") && !strings.Contains(got, "mini") {
		t.Errorf("openai filter should return openai model, got %s", got)
	}

	r.SetProvider("claude")
	got2 := r.Select("simple", 100.0)
	if !strings.Contains(got2, "claude") && !strings.Contains(got2, "haiku") {
		t.Errorf("claude filter should return claude model, got %s", got2)
	}
}

func TestModelRouter_ProviderWithEntries(t *testing.T) {
	entries := []ModelEntry{
		{ID: "local-fast", Provider: "ollama", Tier: TierCheap, CostPer1K: 0},
		{ID: "local-smart", Provider: "ollama", Tier: TierMid, CostPer1K: 0},
	}
	r := NewModelRouterWithModels(entries)

	got := r.Select("simple", 100.0)
	if got != "local-fast" {
		t.Errorf("got %s, want local-fast", got)
	}

	got2 := r.Select("moderate", 100.0)
	if got2 != "local-smart" {
		t.Errorf("got %s, want local-smart", got2)
	}
}

func TestEstimateTokens(t *testing.T) {
	if estimateTokens("") != 0 {
		t.Error("empty string should be 0 tokens")
	}
	if estimateTokens("hi") != 1 {
		t.Error("short string should be at least 1 token")
	}
	got := estimateTokens(strings.Repeat("a", 400))
	if got != 100 {
		t.Errorf("400 chars should be ~100 tokens, got %d", got)
	}
}
