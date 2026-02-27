package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServerStatus tracks the health of an MCP server connection.
type ServerStatus string

const (
	ServerStatusDisconnected ServerStatus = "DISCONNECTED"
	ServerStatusConnecting   ServerStatus = "CONNECTING"
	ServerStatusReady        ServerStatus = "READY"
	ServerStatusError        ServerStatus = "ERROR"
)

// ServerConfig describes how to connect to an MCP server.
type ServerConfig struct {
	Name    string   `json:"name"`              // Unique identifier
	Command string   `json:"command"`           // Executable (e.g., "npx", "python")
	Args    []string `json:"args,omitempty"`    // Command arguments
	Env     []string `json:"env,omitempty"`     // Extra environment variables
	AutoConnect bool `json:"auto_connect"`      // Connect on registry startup
}

// ServerEntry is a managed MCP server instance.
type ServerEntry struct {
	Config  ServerConfig     `json:"config"`
	Status  ServerStatus     `json:"status"`
	Info    ServerInfo       `json:"info,omitempty"`
	Tools   []ToolDefinition `json:"tools,omitempty"`
	Error   string           `json:"error,omitempty"`
	ConnectedAt time.Time    `json:"connected_at,omitempty"`

	client *Client // Active client connection.
}

// Registry manages multiple MCP server connections.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*ServerEntry
}

// NewRegistry creates an MCP server registry.
func NewRegistry() *Registry {
	return &Registry{
		servers: make(map[string]*ServerEntry),
	}
}

// Add registers an MCP server config without connecting.
func (r *Registry) Add(config ServerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.servers[config.Name] = &ServerEntry{
		Config: config,
		Status: ServerStatusDisconnected,
	}
}

// Connect establishes a connection to a registered server.
func (r *Registry) Connect(ctx context.Context, name string) error {
	r.mu.Lock()
	entry, ok := r.servers[name]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("server %q not registered", name)
	}
	entry.Status = ServerStatusConnecting
	cfg := entry.Config
	r.mu.Unlock()

	// Create transport and client outside the lock.
	transport, err := NewStdioTransport(cfg.Command, cfg.Args...)
	if err != nil {
		r.mu.Lock()
		entry.Status = ServerStatusError
		entry.Error = err.Error()
		r.mu.Unlock()
		return fmt.Errorf("connect %q: %w", name, err)
	}

	client := NewClient(transport)

	// Initialize handshake.
	if err := client.Initialize(ctx); err != nil {
		transport.Close()
		r.mu.Lock()
		entry.Status = ServerStatusError
		entry.Error = err.Error()
		r.mu.Unlock()
		return fmt.Errorf("initialize %q: %w", name, err)
	}

	// Discover tools.
	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		transport.Close()
		r.mu.Lock()
		entry.Status = ServerStatusError
		entry.Error = err.Error()
		r.mu.Unlock()
		return fmt.Errorf("discover tools %q: %w", name, err)
	}

	r.mu.Lock()
	entry.client = client
	entry.Status = ServerStatusReady
	entry.Info = client.ServerInfo()
	entry.Tools = tools
	entry.Error = ""
	entry.ConnectedAt = time.Now()
	r.mu.Unlock()

	return nil
}

// Disconnect closes the connection to a server.
func (r *Registry) Disconnect(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.servers[name]
	if !ok {
		return fmt.Errorf("server %q not registered", name)
	}

	if entry.client != nil {
		entry.client.Close()
		entry.client = nil
	}
	entry.Status = ServerStatusDisconnected
	entry.Tools = nil
	return nil
}

// Remove unregisters and disconnects a server.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.servers[name]
	if !ok {
		return fmt.Errorf("server %q not registered", name)
	}

	if entry.client != nil {
		entry.client.Close()
	}
	delete(r.servers, name)
	return nil
}

// CallTool invokes a tool on a specific server.
func (r *Registry) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*ToolResult, error) {
	r.mu.RLock()
	entry, ok := r.servers[serverName]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("server %q not registered", serverName)
	}
	if entry.Status != ServerStatusReady || entry.client == nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("server %q is not ready (status: %s)", serverName, entry.Status)
	}
	client := entry.client
	r.mu.RUnlock()

	return client.CallTool(ctx, toolName, args)
}

// FindTool searches all connected servers for a tool by name.
// Returns (serverName, toolDef, found).
func (r *Registry) FindTool(toolName string) (string, *ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, entry := range r.servers {
		if entry.Status != ServerStatusReady {
			continue
		}
		for i := range entry.Tools {
			if entry.Tools[i].Name == toolName {
				return name, &entry.Tools[i], true
			}
		}
	}
	return "", nil, false
}

// AllTools returns all tools from all connected servers.
// Returns a map of serverName → tools.
func (r *Registry) AllTools() map[string][]ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]ToolDefinition)
	for name, entry := range r.servers {
		if entry.Status == ServerStatusReady && len(entry.Tools) > 0 {
			result[name] = entry.Tools
		}
	}
	return result
}

// FlatTools returns all tools as a flat list (for LLM tool definitions).
func (r *Registry) FlatTools() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []ToolDefinition
	for _, entry := range r.servers {
		if entry.Status == ServerStatusReady {
			all = append(all, entry.Tools...)
		}
	}
	return all
}

// Get returns a server entry by name.
func (r *Registry) Get(name string) *ServerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers[name]
}

// List returns all registered server entries.
func (r *Registry) List() []*ServerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ServerEntry, 0, len(r.servers))
	for _, entry := range r.servers {
		result = append(result, entry)
	}
	return result
}

// ConnectedCount returns the number of servers in READY state.
func (r *Registry) ConnectedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, entry := range r.servers {
		if entry.Status == ServerStatusReady {
			count++
		}
	}
	return count
}

// Count returns total registered servers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.servers)
}

// ConnectAll connects to all servers marked as auto_connect.
func (r *Registry) ConnectAll(ctx context.Context) []error {
	r.mu.RLock()
	var toConnect []string
	for name, entry := range r.servers {
		if entry.Config.AutoConnect && entry.Status == ServerStatusDisconnected {
			toConnect = append(toConnect, name)
		}
	}
	r.mu.RUnlock()

	var errs []error
	for _, name := range toConnect {
		if err := r.Connect(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// DisconnectAll disconnects all connected servers.
func (r *Registry) DisconnectAll() {
	r.mu.RLock()
	names := make([]string, 0, len(r.servers))
	for name, entry := range r.servers {
		if entry.Status == ServerStatusReady {
			names = append(names, name)
		}
	}
	r.mu.RUnlock()

	for _, name := range names {
		r.Disconnect(name)
	}
}
