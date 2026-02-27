package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/instruments"
)

func TestMCPSkillExecutor_Execute(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "search result: Go programming"},
		},
	})

	executor := NewMCPSkillExecutor(r, "mock-server", "search")
	output, err := executor.Execute(context.Background(), instruments.SkillInput{
		Goal:    "search for Go",
		Context: "programming",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !output.Success {
		t.Error("expected success")
	}
	if output.Result != "search result: Go programming" {
		t.Errorf("Result = %q", output.Result)
	}
	if output.ElapsedMs < 0 {
		t.Errorf("ElapsedMs = %d", output.ElapsedMs)
	}
}

func TestMCPSkillExecutor_Execute_MultipleBlocks(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "line 1"},
			{Type: "text", Text: "line 2"},
			{Type: "image"}, // Non-text block, should be skipped.
		},
	})

	executor := NewMCPSkillExecutor(r, "mock-server", "multi")
	output, _ := executor.Execute(context.Background(), instruments.SkillInput{Goal: "test"})
	if output.Result != "line 1\nline 2" {
		t.Errorf("Result = %q", output.Result)
	}
}

func TestMCPSkillExecutor_Execute_ToolError(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "error details"}},
		IsError: true,
	})

	executor := NewMCPSkillExecutor(r, "mock-server", "broken")
	output, err := executor.Execute(context.Background(), instruments.SkillInput{Goal: "test"})
	if err != nil {
		t.Fatal(err) // Transport worked, error is in result.
	}
	if output.Success {
		t.Error("should not be successful when IsError=true")
	}
}

func TestMCPSkillExecutor_Execute_ServerError(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetError(MethodToolsCall, ErrCodeInternal, "crash")

	executor := NewMCPSkillExecutor(r, "mock-server", "search")
	output, err := executor.Execute(context.Background(), instruments.SkillInput{Goal: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if output == nil {
		t.Fatal("output should not be nil even on error")
	}
	if output.Success {
		t.Error("should not be successful")
	}
}

func TestMCPSkillExecutor_WithParameters(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "ok"}},
	})

	executor := NewMCPSkillExecutor(r, "mock-server", "search")
	output, _ := executor.Execute(context.Background(), instruments.SkillInput{
		Goal:       "test",
		Parameters: map[string]string{"limit": "10", "sort": "date"},
	})
	if !output.Success {
		t.Error("expected success")
	}
}

func TestRegisterMCPTools(t *testing.T) {
	r, _ := testableRegistry(t) // Has 2 tools: "search", "calc"
	skills := instruments.NewSkillRegistry()

	count := RegisterMCPTools(r, skills)
	if count != 2 {
		t.Errorf("registered = %d, want 2", count)
	}
	if skills.Count() != 2 {
		t.Errorf("skills.Count = %d", skills.Count())
	}

	// Verify skill IDs.
	if skills.Get("mcp_mock-server_search") == nil {
		t.Error("search skill not found")
	}
	if skills.Get("mcp_mock-server_calc") == nil {
		t.Error("calc skill not found")
	}
}

func TestRegisterMCPTools_NoDuplicates(t *testing.T) {
	r, _ := testableRegistry(t)
	skills := instruments.NewSkillRegistry()

	RegisterMCPTools(r, skills)
	count2 := RegisterMCPTools(r, skills) // Second call should skip existing.
	if count2 != 0 {
		t.Errorf("second registration = %d, want 0", count2)
	}
}

func TestToolsToLLMFormat(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		{
			Name:        "calc",
			Description: "Calculator",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}

	llmTools := ToolsToLLMFormat(tools)
	if len(llmTools) != 2 {
		t.Fatalf("llmTools = %d", len(llmTools))
	}
	if llmTools[0].Name != "search" {
		t.Errorf("Name = %q", llmTools[0].Name)
	}
	if llmTools[0].Description != "Search the web" {
		t.Errorf("Description = %q", llmTools[0].Description)
	}
}

func TestLLMToolCallToMCP(t *testing.T) {
	tc := brain.ToolCall{
		ID:    "tc_1",
		Name:  "search",
		Input: json.RawMessage(`{"query":"hello","limit":10}`),
	}

	name, args, err := LLMToolCallToMCP(tc)
	if err != nil {
		t.Fatal(err)
	}
	if name != "search" {
		t.Errorf("name = %q", name)
	}
	if args["query"] != "hello" {
		t.Errorf("query = %v", args["query"])
	}
}

func TestLLMToolCallToMCP_EmptyInput(t *testing.T) {
	tc := brain.ToolCall{Name: "ping"}
	name, args, err := LLMToolCallToMCP(tc)
	if err != nil {
		t.Fatal(err)
	}
	if name != "ping" {
		t.Errorf("name = %q", name)
	}
	if args != nil {
		t.Errorf("args = %v", args)
	}
}

func TestLLMToolCallToMCP_InvalidJSON(t *testing.T) {
	tc := brain.ToolCall{
		Name:  "bad",
		Input: json.RawMessage(`{invalid`),
	}
	_, _, err := LLMToolCallToMCP(tc)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
