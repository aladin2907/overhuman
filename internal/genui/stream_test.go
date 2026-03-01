package genui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
)

func TestDefaultStreamConfig(t *testing.T) {
	cfg := DefaultStreamConfig()
	if cfg.MaxChunkSize != 512 {
		t.Errorf("MaxChunkSize = %d, want 512", cfg.MaxChunkSize)
	}
	if cfg.FlushInterval != 100*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 100ms", cfg.FlushInterval)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", cfg.Timeout)
	}
}

func TestStreamGenerate_ProducesChunks(t *testing.T) {
	content := "<div>Hello, this is a moderately long HTML content for testing streaming output.</div>"
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: content, Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("test stream", 0.9)
	caps := WebCapabilities(1280, 800)

	config := StreamConfig{
		MaxChunkSize:  20,
		FlushInterval: 10 * time.Millisecond,
		Timeout:       5 * time.Second,
	}

	chunks, err := gen.StreamGenerate(context.Background(), result, caps, config)
	if err != nil {
		t.Fatalf("StreamGenerate: %v", err)
	}

	var allContent string
	var chunkCount int
	var lastChunkDone bool
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		allContent += chunk.Content
		chunkCount++
		lastChunkDone = chunk.Done
	}

	if allContent != content {
		t.Errorf("reassembled content doesn't match original\ngot:  %q\nwant: %q", allContent, content)
	}
	if chunkCount < 2 {
		t.Errorf("expected multiple chunks for content len %d with chunk size 20, got %d", len(content), chunkCount)
	}
	if !lastChunkDone {
		t.Error("last chunk should have Done=true")
	}
}

func TestStreamGenerate_SmallContent(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "Hi", Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("short", 0.9)
	caps := CLICapabilities()

	config := StreamConfig{
		MaxChunkSize:  512,
		FlushInterval: 10 * time.Millisecond,
		Timeout:       5 * time.Second,
	}

	chunks, err := gen.StreamGenerate(context.Background(), result, caps, config)
	if err != nil {
		t.Fatalf("StreamGenerate: %v", err)
	}

	var count int
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		count++
		if chunk.Content != "Hi" {
			t.Errorf("Content = %q, want Hi", chunk.Content)
		}
		if !chunk.Done {
			t.Error("single-chunk content should have Done=true")
		}
	}
	if count != 1 {
		t.Errorf("expected 1 chunk, got %d", count)
	}
}

func TestStreamGenerate_LLMError(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return nil, errors.New("LLM down")
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("fail", 0.5)
	caps := CLICapabilities()

	chunks, err := gen.StreamGenerate(context.Background(), result, caps, DefaultStreamConfig())
	if err != nil {
		t.Fatalf("StreamGenerate should not return error immediately: %v", err)
	}

	var gotError bool
	for chunk := range chunks {
		if chunk.Error != nil {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected error chunk when LLM fails")
	}
}

func TestStreamGenerate_ContextCancellation(t *testing.T) {
	// LLM takes a long time.
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &brain.LLMResponse{Content: "late", Model: "mock"}, nil
		}
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("cancel me", 0.8)
	caps := CLICapabilities()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	chunks, err := gen.StreamGenerate(ctx, result, caps, DefaultStreamConfig())
	if err != nil {
		t.Fatalf("StreamGenerate: %v", err)
	}

	var gotError bool
	for chunk := range chunks {
		if chunk.Error != nil {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected error when context is cancelled")
	}
}

func TestStreamGenerate_DefaultConfigFallbacks(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "ok\033[0m", Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("defaults", 0.8)
	caps := CLICapabilities()

	// Zero config should use defaults.
	config := StreamConfig{}
	chunks, err := gen.StreamGenerate(context.Background(), result, caps, config)
	if err != nil {
		t.Fatalf("StreamGenerate: %v", err)
	}

	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
	}
}

func TestStreamGenerateWithCallback(t *testing.T) {
	mock := newMockLLM(func(ctx context.Context, req brain.LLMRequest) (*brain.LLMResponse, error) {
		return &brain.LLMResponse{Content: "callback test content here\033[0m", Model: "mock"}, nil
	})

	router := brain.NewModelRouter()
	gen := NewUIGenerator(mock, router)
	result := genSimpleResult("callback", 0.9)
	caps := CLICapabilities()

	var chunks []UIChunk
	err := gen.StreamGenerateWithCallback(context.Background(), result, caps, StreamConfig{
		MaxChunkSize:  10,
		FlushInterval: 5 * time.Millisecond,
		Timeout:       5 * time.Second,
	}, func(chunk UIChunk) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("StreamGenerateWithCallback: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
	// Last chunk should be Done.
	if !chunks[len(chunks)-1].Done {
		t.Error("last chunk should have Done=true")
	}
}
