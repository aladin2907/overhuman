package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewRequest(t *testing.T) {
	req, err := NewRequest(1, "tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q", req.JSONRPC)
	}
	if req.Method != "tools/list" {
		t.Errorf("Method = %q", req.Method)
	}
	if req.Params != nil {
		t.Error("Params should be nil")
	}
}

func TestNewRequest_WithParams(t *testing.T) {
	params := ToolCallParams{
		Name:      "search",
		Arguments: json.RawMessage(`{"query":"hello"}`),
	}
	req, err := NewRequest(42, "tools/call", params)
	if err != nil {
		t.Fatal(err)
	}
	if req.ID != 42 {
		t.Errorf("ID = %v", req.ID)
	}
	if req.Params == nil {
		t.Fatal("Params should not be nil")
	}

	// Verify params round-trip.
	var decoded ToolCallParams
	if err := json.Unmarshal(req.Params, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "search" {
		t.Errorf("decoded.Name = %q", decoded.Name)
	}
}

func TestJSONRPCRequest_Marshal(t *testing.T) {
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s == "" {
		t.Error("empty JSON")
	}

	var decoded JSONRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Method != "ping" {
		t.Errorf("Method = %q", decoded.Method)
	}
}

func TestJSONRPCResponse_WithError(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &JSONRPCError{
			Code:    ErrCodeMethodNotFound,
			Message: "method not found",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded JSONRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if decoded.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("Code = %d", decoded.Error.Code)
	}
}

func TestToolDefinition_Marshal(t *testing.T) {
	td := ToolDefinition{
		Name:        "search",
		Description: "Search the web",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
	}
	data, err := json.Marshal(td)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ToolDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "search" {
		t.Errorf("Name = %q", decoded.Name)
	}
}

func TestToolResult_Marshal(t *testing.T) {
	tr := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Hello, world!"},
		},
	}
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Content) != 1 {
		t.Fatalf("Content len = %d", len(decoded.Content))
	}
	if decoded.Content[0].Text != "Hello, world!" {
		t.Errorf("Text = %q", decoded.Content[0].Text)
	}
}

func TestToolResult_IsError(t *testing.T) {
	tr := ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	data, _ := json.Marshal(tr)
	var decoded ToolResult
	json.Unmarshal(data, &decoded)
	if !decoded.IsError {
		t.Error("IsError should be true")
	}
}

func TestInitializeParams_Marshal(t *testing.T) {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo:      ClientInfo{Name: "overhuman", Version: "1.0.0"},
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty JSON")
	}
}

func TestServerInfo_Capabilities(t *testing.T) {
	info := ServerInfo{
		Name:    "test-server",
		Version: "0.1.0",
		Capabilities: Capabilities{
			Tools: &ToolsCapability{ListChanged: true},
		},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ServerInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Capabilities.Tools == nil {
		t.Fatal("Tools capability should not be nil")
	}
	if !decoded.Capabilities.Tools.ListChanged {
		t.Error("ListChanged should be true")
	}
}

func TestMethodConstants(t *testing.T) {
	if MethodInitialize != "initialize" {
		t.Errorf("MethodInitialize = %q", MethodInitialize)
	}
	if MethodToolsList != "tools/list" {
		t.Errorf("MethodToolsList = %q", MethodToolsList)
	}
	if MethodToolsCall != "tools/call" {
		t.Errorf("MethodToolsCall = %q", MethodToolsCall)
	}
	if MethodPing != "ping" {
		t.Errorf("MethodPing = %q", MethodPing)
	}
}
