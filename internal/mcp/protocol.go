// Package mcp implements the Model Context Protocol client.
//
// MCP is the industry-standard protocol for connecting LLM applications
// to external tools and data sources. Overhuman uses MCP to:
//   - Connect to external tool servers (Brave Search, GitHub, etc.)
//   - Expose auto-generated code-skills as MCP tools
//   - Enable tool use through LLM providers (Claude, OpenAI)
//
// Protocol: JSON-RPC 2.0 over stdio or HTTP+SSE.
package mcp

import "encoding/json"

// --- JSON-RPC 2.0 types ---

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"` // int or string; nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	ErrCodeParse      = -32700
	ErrCodeInvalidReq = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// NewRequest creates a JSON-RPC 2.0 request.
func NewRequest(id any, method string, params any) (*JSONRPCRequest, error) {
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
	}
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		req.Params = raw
	}
	return req, nil
}

// --- MCP Protocol types ---

// ServerInfo describes an MCP server's capabilities.
type ServerInfo struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

// Capabilities advertises what the server supports.
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability  `json:"resources,omitempty"`
	Prompts   *PromptCapability    `json:"prompts,omitempty"`
}

// ToolsCapability indicates the server exposes tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"` // Server sends notifications on tool list changes.
}

// ResourceCapability indicates the server exposes resources.
type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapability indicates the server exposes prompt templates.
type PromptCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema
}

// ToolCallParams is sent with tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolResult is returned from tools/call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a piece of tool output.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}

// --- MCP Method names ---

const (
	MethodInitialize  = "initialize"
	MethodToolsList   = "tools/list"
	MethodToolsCall   = "tools/call"
	MethodPing        = "ping"
)

// InitializeParams is sent during handshake.
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      ClientInfo `json:"clientInfo"`
	Capabilities    struct{}   `json:"capabilities"`
}

// ClientInfo identifies the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the server's response to initialize.
type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

// ToolsListResult is the server's response to tools/list.
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}
