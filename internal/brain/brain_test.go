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
