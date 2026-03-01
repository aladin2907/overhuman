package genui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// selfHealResult returns a simple pipeline result for self-healing tests.
func selfHealResult() pipeline.RunResult {
	return pipeline.RunResult{
		TaskID:       "sh-test-1",
		Success:      true,
		Result:       "Hello world",
		QualityScore: 0.95,
	}
}

func TestSelfHeal_ValidOnFirstTry(t *testing.T) {
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "\033[31mHello\033[0m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	ui, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 1 {
		t.Fatalf("expected 1 LLM call, got %d", mock.requestCount())
	}
	if ui.Code != "\033[31mHello\033[0m" {
		t.Fatalf("unexpected code: %q", ui.Code)
	}
}

func TestSelfHeal_InvalidThenValid(t *testing.T) {
	callNum := 0
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		callNum++
		if callNum == 1 {
			return &brain.LLMResponse{Content: "\033[31mBroken"}, nil // no reset
		}
		return &brain.LLMResponse{Content: "\033[31mFixed\033[0m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	ui, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", mock.requestCount())
	}
	if ui.Code != "\033[31mFixed\033[0m" {
		t.Fatalf("unexpected code: %q", ui.Code)
	}
}

func TestSelfHeal_InvalidTwiceThenValid(t *testing.T) {
	callNum := 0
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		callNum++
		switch callNum {
		case 1:
			return &brain.LLMResponse{Content: "\033[31mBad1"}, nil
		case 2:
			return &brain.LLMResponse{Content: "\033[32mBad2"}, nil
		default:
			return &brain.LLMResponse{Content: "\033[33mGood\033[0m"}, nil
		}
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	ui, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", mock.requestCount())
	}
	if ui.Code != "\033[33mGood\033[0m" {
		t.Fatalf("unexpected code: %q", ui.Code)
	}
}

func TestSelfHeal_AllRetrysFail_Error(t *testing.T) {
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "\033[31mAlways broken"}, nil // never has reset
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	_, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}
	if mock.requestCount() != 3 {
		t.Fatalf("expected 3 LLM calls (1 initial + 2 retries), got %d", mock.requestCount())
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSelfHeal_ErrorMessageInPrompt(t *testing.T) {
	callNum := 0
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		callNum++
		if callNum == 1 {
			return &brain.LLMResponse{Content: "\033[31mBroken"}, nil // no reset
		}
		return &brain.LLMResponse{Content: "\033[31mFixed\033[0m"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	_, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", mock.requestCount())
	}

	// The second request should contain the validation error in a message.
	mock.mu.Lock()
	secondReq := mock.captured[1]
	mock.mu.Unlock()

	found := false
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "no reset") && strings.Contains(msg.Content, "Fix it") {
			found = true
			break
		}
	}
	if !found {
		var contents []string
		for _, msg := range secondReq.Messages {
			contents = append(contents, fmt.Sprintf("[%s] %s", msg.Role, msg.Content))
		}
		t.Fatalf("retry prompt should contain the validation error about 'no reset';\nmessages:\n%s",
			strings.Join(contents, "\n---\n"))
	}
}

func TestSelfHeal_LLMErrorNotRetried(t *testing.T) {
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		return nil, fmt.Errorf("API rate limit exceeded")
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	_, err := gen.Generate(context.Background(), selfHealResult(), CLICapabilities())
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
	if mock.requestCount() != 1 {
		t.Fatalf("expected 1 LLM call (no retry on LLM error), got %d", mock.requestCount())
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelfHeal_ValidHTML_NoRetry(t *testing.T) {
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "<div>Hello</div>"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	caps := WebCapabilities(1024, 768)
	ui, err := gen.Generate(context.Background(), selfHealResult(), caps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 1 {
		t.Fatalf("expected 1 LLM call, got %d", mock.requestCount())
	}
	if ui.Code != "<div>Hello</div>" {
		t.Fatalf("unexpected code: %q", ui.Code)
	}
	if ui.Format != FormatHTML {
		t.Fatalf("expected HTML format, got %s", ui.Format)
	}
}

func TestSelfHeal_InvalidHTML_Retry(t *testing.T) {
	callNum := 0
	mock := newMockLLM(func(_ context.Context, _ brain.LLMRequest) (*brain.LLMResponse, error) {
		callNum++
		if callNum == 1 {
			return &brain.LLMResponse{Content: "Just plain text no tags"}, nil // invalid: no <
		}
		return &brain.LLMResponse{Content: "<div>Valid HTML</div>"}, nil
	})
	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)

	caps := WebCapabilities(1024, 768)
	ui, err := gen.Generate(context.Background(), selfHealResult(), caps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.requestCount() != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", mock.requestCount())
	}
	if ui.Code != "<div>Valid HTML</div>" {
		t.Fatalf("unexpected code: %q", ui.Code)
	}
}
