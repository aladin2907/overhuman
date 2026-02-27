package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockTransport implements Transport for testing.
type mockTransport struct {
	mu        sync.Mutex
	responses map[string]*JSONRPCResponse
	calls     []string
	closed    bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses: make(map[string]*JSONRPCResponse),
	}
}

func (m *mockTransport) SetResponse(method string, result any) {
	raw, _ := json.Marshal(result)
	m.responses[method] = &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  raw,
	}
}

func (m *mockTransport) SetError(method string, code int, msg string) {
	m.responses[method] = &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
}

func (m *mockTransport) Send(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, req.Method)

	resp, ok := m.responses[req.Method]
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`{}`),
		}, nil
	}
	resp.ID = req.ID
	return resp, nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func TestClient_Initialize(t *testing.T) {
	mt := newMockTransport()
	mt.SetResponse(MethodInitialize, InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "test-server",
			Version: "1.0.0",
		},
	})

	client := NewClient(mt)
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}

	info := client.ServerInfo()
	if info.Name != "test-server" {
		t.Errorf("Name = %q", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %q", info.Version)
	}
}

func TestClient_DiscoverTools(t *testing.T) {
	mt := newMockTransport()
	mt.SetResponse(MethodToolsList, ToolsListResult{
		Tools: []ToolDefinition{
			{Name: "search", Description: "Search the web"},
			{Name: "calculator", Description: "Math operations"},
		},
	})

	client := NewClient(mt)
	tools, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	if tools[0].Name != "search" {
		t.Errorf("tools[0].Name = %q", tools[0].Name)
	}

	cached := client.Tools()
	if len(cached) != 2 {
		t.Errorf("cached tools = %d", len(cached))
	}
}

func TestClient_CallTool(t *testing.T) {
	mt := newMockTransport()
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "42"},
		},
	})

	client := NewClient(mt)
	result, err := client.CallTool(context.Background(), "calculator", map[string]any{
		"expression": "6 * 7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content blocks = %d", len(result.Content))
	}
	if result.Content[0].Text != "42" {
		t.Errorf("text = %q", result.Content[0].Text)
	}
}

func TestClient_CallTool_ServerError(t *testing.T) {
	mt := newMockTransport()
	mt.SetError(MethodToolsCall, ErrCodeInternal, "tool crashed")

	client := NewClient(mt)
	_, err := client.CallTool(context.Background(), "broken", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_Ping(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}

	mt.mu.Lock()
	found := false
	for _, c := range mt.calls {
		if c == MethodPing {
			found = true
		}
	}
	mt.mu.Unlock()
	if !found {
		t.Error("ping was not called")
	}
}

func TestClient_Close(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if !mt.closed {
		t.Error("transport should be closed")
	}
}

func TestClient_SetTimeout(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)
	client.SetTimeout(5 * time.Second)
	if client.timeout != 5*time.Second {
		t.Errorf("timeout = %v", client.timeout)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	// Create a transport that blocks until context cancelled.
	slowTransport := &blockingTransport{}
	client := NewClient(slowTransport)
	client.SetTimeout(0) // No default timeout.

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.CallTool(ctx, "slow", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

type blockingTransport struct{}

func (t *blockingTransport) Send(ctx context.Context, req *JSONRPCRequest) (*JSONRPCResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (t *blockingTransport) Close() error { return nil }

// --- Registry tests (using mock transport) ---

func TestRegistry_Add(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "test-server", Command: "echo"})

	if r.Count() != 1 {
		t.Errorf("Count = %d", r.Count())
	}

	entry := r.Get("test-server")
	if entry == nil {
		t.Fatal("Get returned nil")
	}
	if entry.Status != ServerStatusDisconnected {
		t.Errorf("Status = %q", entry.Status)
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "s1"})
	r.Add(ServerConfig{Name: "s2"})

	if err := r.Remove("s1"); err != nil {
		t.Fatal(err)
	}
	if r.Count() != 1 {
		t.Errorf("Count = %d", r.Count())
	}
}

func TestRegistry_Remove_NotFound(t *testing.T) {
	r := NewRegistry()
	if err := r.Remove("nope"); err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	if r.Get("missing") != nil {
		t.Error("expected nil")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "a"})
	r.Add(ServerConfig{Name: "b"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("List = %d", len(list))
	}
}

func TestRegistry_ConnectedCount_Empty(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "a"})
	if r.ConnectedCount() != 0 {
		t.Errorf("ConnectedCount = %d", r.ConnectedCount())
	}
}

func TestRegistry_FindTool_NotConnected(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "s1"})
	_, _, found := r.FindTool("search")
	if found {
		t.Error("should not find tool on disconnected server")
	}
}

func TestRegistry_AllTools_Empty(t *testing.T) {
	r := NewRegistry()
	all := r.AllTools()
	if len(all) != 0 {
		t.Errorf("AllTools = %d", len(all))
	}
}

func TestRegistry_FlatTools_Empty(t *testing.T) {
	r := NewRegistry()
	flat := r.FlatTools()
	if len(flat) != 0 {
		t.Errorf("FlatTools = %d", len(flat))
	}
}

// testableRegistry creates a registry with a mock-connected server.
func testableRegistry(t *testing.T) (*Registry, *mockTransport) {
	r := NewRegistry()
	mt := newMockTransport()

	r.Add(ServerConfig{Name: "mock-server"})

	// Manually inject a connected client.
	client := NewClient(mt)
	r.mu.Lock()
	entry := r.servers["mock-server"]
	entry.client = client
	entry.Status = ServerStatusReady
	entry.Tools = []ToolDefinition{
		{Name: "search", Description: "Search tool"},
		{Name: "calc", Description: "Calculator"},
	}
	r.mu.Unlock()

	return r, mt
}

func TestRegistry_FindTool_Connected(t *testing.T) {
	r, _ := testableRegistry(t)

	server, tool, found := r.FindTool("search")
	if !found {
		t.Fatal("should find tool")
	}
	if server != "mock-server" {
		t.Errorf("server = %q", server)
	}
	if tool.Name != "search" {
		t.Errorf("tool = %q", tool.Name)
	}
}

func TestRegistry_AllTools_Connected(t *testing.T) {
	r, _ := testableRegistry(t)

	all := r.AllTools()
	if len(all) != 1 {
		t.Fatalf("AllTools servers = %d", len(all))
	}
	tools, ok := all["mock-server"]
	if !ok {
		t.Fatal("mock-server not found")
	}
	if len(tools) != 2 {
		t.Errorf("tools = %d", len(tools))
	}
}

func TestRegistry_FlatTools_Connected(t *testing.T) {
	r, _ := testableRegistry(t)
	flat := r.FlatTools()
	if len(flat) != 2 {
		t.Errorf("FlatTools = %d", len(flat))
	}
}

func TestRegistry_CallTool_Connected(t *testing.T) {
	r, mt := testableRegistry(t)
	mt.SetResponse(MethodToolsCall, ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "result"}},
	})

	result, err := r.CallTool(context.Background(), "mock-server", "search", map[string]any{"q": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "result" {
		t.Errorf("text = %q", result.Content[0].Text)
	}
}

func TestRegistry_CallTool_ServerNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.CallTool(context.Background(), "nope", "tool", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_CallTool_NotReady(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "s1"}) // Disconnected.

	_, err := r.CallTool(context.Background(), "s1", "tool", nil)
	if err == nil {
		t.Error("expected error for disconnected server")
	}
}

func TestRegistry_Disconnect(t *testing.T) {
	r, mt := testableRegistry(t)
	if err := r.Disconnect("mock-server"); err != nil {
		t.Fatal(err)
	}

	entry := r.Get("mock-server")
	if entry.Status != ServerStatusDisconnected {
		t.Errorf("Status = %q", entry.Status)
	}

	mt.mu.Lock()
	defer mt.mu.Unlock()
	if !mt.closed {
		t.Error("transport should be closed")
	}
}

func TestRegistry_DisconnectAll(t *testing.T) {
	r, _ := testableRegistry(t)

	// Add another connected server.
	mt2 := newMockTransport()
	r.Add(ServerConfig{Name: "s2"})
	r.mu.Lock()
	entry2 := r.servers["s2"]
	entry2.client = NewClient(mt2)
	entry2.Status = ServerStatusReady
	r.mu.Unlock()

	r.DisconnectAll()

	if r.ConnectedCount() != 0 {
		t.Errorf("ConnectedCount = %d", r.ConnectedCount())
	}
}

func TestRegistry_Connect_NotRegistered(t *testing.T) {
	r := NewRegistry()
	err := r.Connect(context.Background(), "nope")
	if err == nil {
		t.Error("expected error")
	}
}

func TestRegistry_Disconnect_NotRegistered(t *testing.T) {
	r := NewRegistry()
	err := r.Disconnect("nope")
	if err == nil {
		t.Error("expected error")
	}
}

func TestServerStatus_Constants(t *testing.T) {
	statuses := []ServerStatus{
		ServerStatusDisconnected,
		ServerStatusConnecting,
		ServerStatusReady,
		ServerStatusError,
	}
	seen := make(map[ServerStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
		if s == "" {
			t.Error("empty status constant")
		}
	}
}

// Verify mockTransport tracks calls.
func TestMockTransport_CallTracking(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)

	client.Ping(context.Background())
	client.Ping(context.Background())

	mt.mu.Lock()
	defer mt.mu.Unlock()
	if len(mt.calls) != 2 {
		t.Errorf("calls = %d, want 2", len(mt.calls))
	}
	for _, c := range mt.calls {
		if c != MethodPing {
			t.Errorf("call = %q", c)
		}
	}
}

// Verify atomic ID increments.
func TestClient_IDIncrement(t *testing.T) {
	mt := newMockTransport()
	client := NewClient(mt)

	for i := 0; i < 5; i++ {
		client.Ping(context.Background())
	}

	// IDs should have been 1, 2, 3, 4, 5.
	if got := client.nextID.Load(); got != 5 {
		t.Errorf("nextID = %d, want 5", got)
	}
}

func TestRegistry_ConnectAll_NoAutoConnect(t *testing.T) {
	r := NewRegistry()
	r.Add(ServerConfig{Name: "s1", AutoConnect: false})
	r.Add(ServerConfig{Name: "s2", AutoConnect: false})

	errs := r.ConnectAll(context.Background())
	// No servers should attempt connection.
	if len(errs) != 0 {
		// Errors from actual connection attempts would show up,
		// but with AutoConnect=false, nothing should be attempted.
		t.Logf("unexpected errors: %v", errs)
	}
	if r.ConnectedCount() != 0 {
		t.Errorf("ConnectedCount = %d", r.ConnectedCount())
	}
}

func TestRegistry_ConnectFail(t *testing.T) {
	r := NewRegistry()
	// Register a server with an invalid command.
	r.Add(ServerConfig{
		Name:    "bad",
		Command: fmt.Sprintf("/nonexistent-binary-%d", time.Now().UnixNano()),
	})

	err := r.Connect(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for invalid command")
	}

	entry := r.Get("bad")
	if entry.Status != ServerStatusError {
		t.Errorf("Status = %q, want ERROR", entry.Status)
	}
}
