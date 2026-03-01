package genui

import (
	"context"
	"fmt"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/pipeline"
)

// StreamConfig configures streaming behavior.
type StreamConfig struct {
	MaxChunkSize  int           // Max bytes per chunk (default 512)
	FlushInterval time.Duration // Max wait before flushing buffer (default 100ms)
	Timeout       time.Duration // Total generation timeout (default 5 min)
}

// DefaultStreamConfig returns default streaming configuration.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		MaxChunkSize:  512,
		FlushInterval: 100 * time.Millisecond,
		Timeout:       5 * time.Minute,
	}
}

// StreamGenerate generates UI in streaming mode, sending chunks to the returned channel.
// The channel is closed when generation is complete or on error.
func (g *UIGenerator) StreamGenerate(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities, config StreamConfig) (<-chan UIChunk, error) {
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = 512
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 100 * time.Millisecond
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}

	format := g.selectFormat(caps)
	prompt := g.buildPrompt(result, format, caps, nil, nil)
	model := g.router.Select("simple", 100.0)

	chunks := make(chan UIChunk, 16)

	go func() {
		defer close(chunks)

		ctx, cancel := context.WithTimeout(ctx, config.Timeout)
		defer cancel()

		resp, err := g.llm.Complete(ctx, brain.LLMRequest{
			Messages: prompt,
			Model:    model,
		})
		if err != nil {
			chunks <- UIChunk{Error: fmt.Errorf("stream generate: %w", err)}
			return
		}

		// Simulate streaming by chunking the response.
		// In a real implementation, the LLM provider would stream tokens.
		content := resp.Content
		for i := 0; i < len(content); i += config.MaxChunkSize {
			end := i + config.MaxChunkSize
			if end > len(content) {
				end = len(content)
			}

			select {
			case <-ctx.Done():
				chunks <- UIChunk{Error: ctx.Err()}
				return
			case chunks <- UIChunk{Content: content[i:end], Done: end >= len(content)}:
			}

			// Small delay between chunks to simulate streaming.
			if end < len(content) {
				time.Sleep(config.FlushInterval / 4)
			}
		}
	}()

	return chunks, nil
}

// StreamGenerateWithCallback is an alternate streaming interface that calls
// a callback for each chunk instead of returning a channel.
func (g *UIGenerator) StreamGenerateWithCallback(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities, config StreamConfig, callback func(chunk UIChunk)) error {
	chunks, err := g.StreamGenerate(ctx, result, caps, config)
	if err != nil {
		return err
	}

	for chunk := range chunks {
		if chunk.Error != nil {
			return chunk.Error
		}
		callback(chunk)
	}
	return nil
}
