package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Transport abstracts how we communicate with an MCP server.
type Transport interface {
	// Send sends a JSON-RPC request and returns the response.
	Send(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error)
	// Close shuts down the transport.
	Close() error
}

// StdioTransport communicates with an MCP server over stdin/stdout.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
}

// NewStdioTransport spawns a process and connects via stdio.
func NewStdioTransport(command string, args ...string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Discard stderr to prevent blocking.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command %q: %w", command, err)
	}

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Send sends a JSON-RPC request and reads the response line.
func (t *StdioTransport) Send(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := t.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	// Read response with context timeout.
	type readResult struct {
		line []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := t.stdout.ReadBytes('\n')
		ch <- readResult{line, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("read from stdout: %w", r.err)
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal(r.line, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		return &resp, nil
	}
}

// Close terminates the child process.
func (t *StdioTransport) Close() error {
	t.stdin.Close()
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	return t.cmd.Wait()
}

// Client connects to a single MCP server and invokes tools.
type Client struct {
	transport Transport
	nextID    atomic.Int64
	info      ServerInfo
	tools     []ToolDefinition
	mu        sync.RWMutex
	timeout   time.Duration
}

// NewClient creates an MCP client over the given transport.
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
		timeout:   30 * time.Second,
	}
}

// SetTimeout sets the default request timeout.
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Initialize performs the MCP handshake.
func (c *Client) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: ClientInfo{
			Name:    "overhuman",
			Version: "1.0.0",
		},
	}

	resp, err := c.call(ctx, MethodInitialize, params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	c.mu.Lock()
	c.info = result.ServerInfo
	c.mu.Unlock()

	return nil
}

// DiscoverTools fetches the list of tools from the server.
func (c *Client) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	resp, err := c.call(ctx, MethodToolsList, nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse tools list: %w", err)
	}

	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()

	return result.Tools, nil
}

// CallTool invokes a tool by name with the given arguments.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal arguments: %w", err)
	}

	params := ToolCallParams{
		Name:      name,
		Arguments: argsJSON,
	}

	resp, err := c.call(ctx, MethodToolsCall, params)
	if err != nil {
		return nil, fmt.Errorf("tools/call %q: %w", name, err)
	}

	var result ToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse tool result: %w", err)
	}

	return &result, nil
}

// Ping sends a ping to check server health.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.call(ctx, MethodPing, nil)
	return err
}

// ServerInfo returns the server's info from initialization.
func (c *Client) ServerInfo() ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info
}

// Tools returns the cached list of discovered tools.
func (c *Client) Tools() []ToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

// Close shuts down the transport connection.
func (c *Client) Close() error {
	return c.transport.Close()
}

// call makes a JSON-RPC call and returns the result.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	req, err := NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("server error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}
