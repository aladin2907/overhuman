package brain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultMaxContextTokens is the default maximum context size in tokens.
const DefaultMaxContextTokens = 100_000

// ContextLayers holds the 6 prioritized layers of context for prompt assembly.
type ContextLayers struct {
	// Layer 1: System prompt (from soul). Highest priority.
	SystemPrompt string

	// Layer 2: Task description.
	TaskDescription string

	// Layer 3: Available tools (MCP tool definitions).
	Tools []Tool

	// Layer 4: Relevant memory (from long-term memory search).
	RelevantMemory []string

	// Layer 5: Recent history (from short-term memory).
	RecentHistory []Message

	// Layer 6: SKB insights (cross-agent knowledge). Lowest priority.
	SKBInsights []string
}

// ContextAssembler builds the final prompt from prioritized context layers.
// It handles truncation when total context exceeds the configured maximum.
type ContextAssembler struct {
	maxTokens int
}

// NewContextAssembler creates a new assembler with default settings.
func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{
		maxTokens: DefaultMaxContextTokens,
	}
}

// NewContextAssemblerWithLimit creates a new assembler with a custom token limit.
func NewContextAssemblerWithLimit(maxTokens int) *ContextAssembler {
	return &ContextAssembler{
		maxTokens: maxTokens,
	}
}

// Assemble builds an ordered []Message from the context layers.
// Priority order (highest first): system prompt, task, tools, memory, history, SKB.
// When total estimated tokens exceed maxTokens, lower-priority layers are truncated first.
func (ca *ContextAssembler) Assemble(layers ContextLayers) []Message {
	// Build all potential content blocks with their priority (lower number = higher priority).
	var blocks []block

	// Layer 1: System prompt.
	if layers.SystemPrompt != "" {
		blocks = append(blocks, block{priority: 1, role: "system", content: layers.SystemPrompt, isSystem: true})
	}

	// Layer 2: Task description.
	if layers.TaskDescription != "" {
		blocks = append(blocks, block{priority: 2, role: "user", content: fmt.Sprintf("[Task]\n%s", layers.TaskDescription)})
	}

	// Layer 3: Tools - serialize to a readable format in a system-like message.
	if len(layers.Tools) > 0 {
		toolsJSON, err := json.Marshal(layers.Tools)
		if err == nil {
			blocks = append(blocks, block{priority: 3, role: "system", content: fmt.Sprintf("[Available Tools]\n%s", string(toolsJSON)), isSystem: true})
		}
	}

	// Layer 4: Relevant memory.
	if len(layers.RelevantMemory) > 0 {
		memContent := "[Relevant Memory]\n" + strings.Join(layers.RelevantMemory, "\n---\n")
		blocks = append(blocks, block{priority: 4, role: "system", content: memContent, isSystem: true})
	}

	// Layer 5: Recent history (already Message-shaped, we serialize them as individual blocks).
	// These go in as-is with their original roles.
	for i, msg := range layers.RecentHistory {
		blocks = append(blocks, block{priority: 5, role: msg.Role, content: msg.Content, isSystem: msg.Role == "system"})
		_ = i
	}

	// Layer 6: SKB insights.
	if len(layers.SKBInsights) > 0 {
		skbContent := "[SKB Insights]\n" + strings.Join(layers.SKBInsights, "\n---\n")
		blocks = append(blocks, block{priority: 6, role: "system", content: skbContent, isSystem: true})
	}

	// Estimate total tokens (rough: 1 token ~ 4 characters).
	totalEstTokens := 0
	for _, b := range blocks {
		totalEstTokens += estimateTokens(b.content)
	}

	// If within budget, return all blocks as messages.
	if totalEstTokens <= ca.maxTokens {
		return blocksToMessages(blocks)
	}

	// Truncation: remove from lowest priority first.
	// Sort by priority descending to truncate lowest priority first.
	remaining := ca.maxTokens

	// First pass: allocate tokens by priority (highest priority gets full allocation first).
	type allocation struct {
		idx    int
		tokens int
	}
	allocations := make([]allocation, len(blocks))

	// Allocate in priority order (lowest number = highest priority).
	for i, b := range blocks {
		needed := estimateTokens(b.content)
		if remaining >= needed {
			allocations[i] = allocation{idx: i, tokens: needed}
			remaining -= needed
		} else if remaining > 0 {
			allocations[i] = allocation{idx: i, tokens: remaining}
			remaining = 0
		} else {
			allocations[i] = allocation{idx: i, tokens: 0}
		}
	}

	// Build truncated blocks.
	var truncated []block
	for i, alloc := range allocations {
		if alloc.tokens <= 0 {
			continue
		}
		b := blocks[i]
		needed := estimateTokens(b.content)
		if alloc.tokens < needed {
			// Truncate content to fit.
			maxChars := alloc.tokens * 4
			if maxChars < len(b.content) {
				b.content = b.content[:maxChars] + "\n...[truncated]"
			}
		}
		truncated = append(truncated, b)
	}

	return blocksToMessages(truncated)
}

// blocksToMessages converts internal blocks to a []Message slice.
// It merges consecutive system blocks into a single system message at the start.
func blocksToMessages(blocks []block) []Message {
	if len(blocks) == 0 {
		return nil
	}

	var messages []Message

	// Collect system messages first, then non-system in order.
	var systemParts []string
	var nonSystem []Message

	for _, b := range blocks {
		if b.isSystem {
			systemParts = append(systemParts, b.content)
		} else {
			nonSystem = append(nonSystem, Message{Role: b.role, Content: b.content})
		}
	}

	// If there are system parts, combine them into one system message at the beginning.
	if len(systemParts) > 0 {
		messages = append(messages, Message{
			Role:    "system",
			Content: strings.Join(systemParts, "\n\n"),
		})
	}

	messages = append(messages, nonSystem...)

	return messages
}

// estimateTokens gives a rough token count estimate (~4 chars per token).
func estimateTokens(s string) int {
	tokens := len(s) / 4
	if tokens == 0 && len(s) > 0 {
		tokens = 1
	}
	return tokens
}

// block is a helper type used internally by ContextAssembler.
type block struct {
	priority int
	role     string
	content  string
	isSystem bool
}
