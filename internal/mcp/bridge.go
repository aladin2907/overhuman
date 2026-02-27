package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/instruments"
)

// MCPSkillExecutor wraps an MCP tool as a SkillExecutor.
type MCPSkillExecutor struct {
	registry   *Registry
	serverName string
	toolName   string
}

// NewMCPSkillExecutor creates a SkillExecutor that delegates to an MCP tool.
func NewMCPSkillExecutor(registry *Registry, serverName, toolName string) *MCPSkillExecutor {
	return &MCPSkillExecutor{
		registry:   registry,
		serverName: serverName,
		toolName:   toolName,
	}
}

// Execute invokes the MCP tool and returns SkillOutput.
func (e *MCPSkillExecutor) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	start := time.Now()

	// Build arguments from skill input.
	args := map[string]any{
		"goal":    input.Goal,
		"context": input.Context,
	}
	for k, v := range input.Parameters {
		args[k] = v
	}

	result, err := e.registry.CallTool(ctx, e.serverName, e.toolName, args)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &instruments.SkillOutput{
			Success:   false,
			Error:     err.Error(),
			ElapsedMs: elapsed,
		}, err
	}

	// Combine content blocks into result text.
	var text strings.Builder
	for _, block := range result.Content {
		if block.Text != "" {
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(block.Text)
		}
	}

	return &instruments.SkillOutput{
		Result:    text.String(),
		Success:   !result.IsError,
		CostUSD:   0, // MCP tools don't report cost.
		ElapsedMs: elapsed,
	}, nil
}

// RegisterMCPTools discovers tools from all connected MCP servers and
// registers them as skills in the skill registry.
func RegisterMCPTools(registry *Registry, skills *instruments.SkillRegistry) int {
	allTools := registry.AllTools()
	count := 0

	for serverName, tools := range allTools {
		for _, tool := range tools {
			skillID := fmt.Sprintf("mcp_%s_%s", serverName, tool.Name)

			// Skip if already registered.
			if skills.Get(skillID) != nil {
				continue
			}

			executor := NewMCPSkillExecutor(registry, serverName, tool.Name)
			skill := &instruments.Skill{
				Executor: executor,
				Meta: instruments.SkillMeta{
					ID:     skillID,
					Name:   fmt.Sprintf("[MCP:%s] %s", serverName, tool.Name),
					Type:   instruments.SkillTypeLLM, // MCP tools behave like external calls.
					Status: instruments.SkillStatusActive,
				},
			}
			skills.Register(skill)
			count++
		}
	}
	return count
}

// ToolsToLLMFormat converts MCP tool definitions to the LLM brain.Tool format.
func ToolsToLLMFormat(tools []ToolDefinition) []brain.Tool {
	result := make([]brain.Tool, len(tools))
	for i, t := range tools {
		result[i] = brain.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return result
}

// LLMToolCallToMCP converts an LLM tool call to MCP format.
func LLMToolCallToMCP(tc brain.ToolCall) (string, map[string]any, error) {
	var args map[string]any
	if len(tc.Input) > 0 {
		if err := json.Unmarshal(tc.Input, &args); err != nil {
			return tc.Name, nil, fmt.Errorf("unmarshal tool call args: %w", err)
		}
	}
	return tc.Name, args, nil
}
