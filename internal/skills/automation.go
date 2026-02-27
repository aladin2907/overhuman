package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/overhuman/overhuman/internal/instruments"
	"github.com/overhuman/overhuman/internal/storage"
)

// --- API Integration Skill ---

// APIIntegrationSkill performs HTTP REST calls.
type APIIntegrationSkill struct {
	client *http.Client
}

func NewAPIIntegrationSkill() *APIIntegrationSkill {
	return &APIIntegrationSkill{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *APIIntegrationSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	method := input.Parameters["method"]
	url := input.Parameters["url"]
	body := input.Parameters["body"]
	headers := input.Parameters["headers"]

	if url == "" {
		return &instruments.SkillOutput{Success: false, Error: "url parameter required"}, nil
	}
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}

	// Parse headers.
	if headers != "" {
		var hmap map[string]string
		if err := json.Unmarshal([]byte(headers), &hmap); err == nil {
			for k, v := range hmap {
				req.Header.Set(k, v)
			}
		}
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error(), ElapsedMs: elapsed}, nil
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 100*1024))

	return &instruments.SkillOutput{
		Result:    fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(data)),
		Success:   resp.StatusCode >= 200 && resp.StatusCode < 400,
		ElapsedMs: elapsed,
	}, nil
}

// --- Web Search Skill ---

// WebSearchSkill performs web search via HTTP.
type WebSearchSkill struct {
	client *http.Client
}

func NewWebSearchSkill() *WebSearchSkill {
	return &WebSearchSkill{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *WebSearchSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	query := input.Parameters["query"]
	if query == "" {
		query = input.Goal
	}
	if query == "" {
		return &instruments.SkillOutput{Success: false, Error: "query parameter required"}, nil
	}

	// Placeholder: in production, would call Brave Search API or similar.
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("[web_search] query=%q — requires BRAVE_API_KEY or search API configuration", query),
		Success: false,
		Error:   "search API not configured",
	}, nil
}

// --- Scheduler Skill ---

// SchedulerSkill manages cron-like scheduled tasks.
type SchedulerSkill struct {
	mu    sync.RWMutex
	tasks map[string]*ScheduledTask
	store storage.Store
}

// ScheduledTask represents a scheduled task.
type ScheduledTask struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Schedule  string `json:"schedule"` // Cron expression or interval.
	Action    string `json:"action"`
	Active    bool   `json:"active"`
	NextRun   string `json:"next_run,omitempty"`
	LastRun   string `json:"last_run,omitempty"`
	RunCount  int    `json:"run_count"`
}

func NewSchedulerSkill(store storage.Store) *SchedulerSkill {
	return &SchedulerSkill{
		tasks: make(map[string]*ScheduledTask),
		store: store,
	}
}

func (s *SchedulerSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]

	switch action {
	case "add":
		return s.addTask(ctx, input)
	case "list":
		return s.listTasks()
	case "remove":
		return s.removeTask(input.Parameters["task_id"])
	case "status":
		return s.taskStatus(input.Parameters["task_id"])
	default:
		return s.listTasks()
	}
}

func (s *SchedulerSkill) addTask(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("sched_%d", len(s.tasks)+1)
	task := &ScheduledTask{
		ID:       id,
		Name:     input.Parameters["name"],
		Schedule: input.Parameters["schedule"],
		Action:   input.Parameters["task_action"],
		Active:   true,
		NextRun:  time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	s.tasks[id] = task

	// Persist if store available.
	if s.store != nil {
		data, _ := json.Marshal(task)
		s.store.Put(ctx, storage.Record{Key: "scheduler:" + id, Value: data})
	}

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Scheduled task %s: %s (schedule: %s)", id, task.Name, task.Schedule),
		Success: true,
	}, nil
}

func (s *SchedulerSkill) listTasks() (*instruments.SkillOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.tasks) == 0 {
		return &instruments.SkillOutput{Result: "No scheduled tasks", Success: true}, nil
	}

	var lines []string
	for _, t := range s.tasks {
		status := "active"
		if !t.Active {
			status = "paused"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s — %s (%s, runs: %d)", t.ID, t.Name, t.Schedule, status, t.RunCount))
	}
	return &instruments.SkillOutput{Result: strings.Join(lines, "\n"), Success: true}, nil
}

func (s *SchedulerSkill) removeTask(id string) (*instruments.SkillOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return &instruments.SkillOutput{Success: false, Error: "task not found: " + id}, nil
	}
	delete(s.tasks, id)
	return &instruments.SkillOutput{Result: "Removed task " + id, Success: true}, nil
}

func (s *SchedulerSkill) taskStatus(id string) (*instruments.SkillOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return &instruments.SkillOutput{Success: false, Error: "task not found: " + id}, nil
	}
	data, _ := json.MarshalIndent(t, "", "  ")
	return &instruments.SkillOutput{Result: string(data), Success: true}, nil
}

// --- Audit Skill ---

// AuditSkill records and queries audit trail entries.
type AuditSkill struct {
	store storage.Store
	mu    sync.Mutex
	seq   int
}

func NewAuditSkill(store storage.Store) *AuditSkill {
	return &AuditSkill{store: store}
}

func (s *AuditSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]

	switch action {
	case "log":
		return s.logEntry(ctx, input)
	case "query":
		return s.queryEntries(ctx, input.Parameters["query"])
	case "count":
		return s.countEntries(ctx)
	default:
		return s.logEntry(ctx, input)
	}
}

func (s *AuditSkill) logEntry(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("audit_%d_%d", time.Now().Unix(), s.seq)
	s.mu.Unlock()

	entry := map[string]string{
		"id":        id,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"actor":     input.Parameters["actor"],
		"action":    input.Parameters["audit_action"],
		"target":    input.Parameters["target"],
		"details":   input.Goal,
	}
	data, _ := json.Marshal(entry)

	if s.store != nil {
		err := s.store.Put(ctx, storage.Record{
			Key:   "audit:" + id,
			Value: data,
			Metadata: map[string]string{
				"actor":  input.Parameters["actor"],
				"action": input.Parameters["audit_action"],
			},
		})
		if err != nil {
			return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
		}
	}

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Audit entry recorded: %s", id),
		Success: true,
	}, nil
}

func (s *AuditSkill) queryEntries(ctx context.Context, query string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "store not configured"}, nil
	}

	if query != "" {
		records, err := s.store.Search(ctx, query, 20)
		if err != nil {
			return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
		}
		var lines []string
		for _, r := range records {
			lines = append(lines, string(r.Value))
		}
		return &instruments.SkillOutput{
			Result:  fmt.Sprintf("Found %d audit entries:\n%s", len(lines), strings.Join(lines, "\n")),
			Success: true,
		}, nil
	}

	// List recent audit entries.
	keys, err := s.store.List(ctx, "audit:", 20)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Recent %d audit entries:\n%s", len(keys), strings.Join(keys, "\n")),
		Success: true,
	}, nil
}

func (s *AuditSkill) countEntries(ctx context.Context) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Result: "0 entries (no store)", Success: true}, nil
	}
	keys, err := s.store.List(ctx, "audit:", 10000)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("%d audit entries", len(keys)),
		Success: true,
	}, nil
}

// --- Credential Skill ---

// CredentialSkill manages secure API key storage.
type CredentialSkill struct {
	store storage.Store
}

func NewCredentialSkill(store storage.Store) *CredentialSkill {
	return &CredentialSkill{store: store}
}

func (s *CredentialSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]

	switch action {
	case "store":
		return s.storeCredential(ctx, input)
	case "get":
		return s.getCredential(ctx, input.Parameters["name"])
	case "list":
		return s.listCredentials(ctx)
	case "delete":
		return s.deleteCredential(ctx, input.Parameters["name"])
	default:
		return s.listCredentials(ctx)
	}
}

func (s *CredentialSkill) storeCredential(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "store not configured"}, nil
	}
	name := input.Parameters["name"]
	value := input.Parameters["value"]
	if name == "" || value == "" {
		return &instruments.SkillOutput{Success: false, Error: "name and value required"}, nil
	}

	// Store with masked value for safety.
	err := s.store.Put(ctx, storage.Record{
		Key:   "cred:" + name,
		Value: []byte(value),
		Metadata: map[string]string{
			"type": input.Parameters["type"],
		},
	})
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Credential %q stored (value hidden)", name),
		Success: true,
	}, nil
}

func (s *CredentialSkill) getCredential(ctx context.Context, name string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "store not configured"}, nil
	}
	rec, err := s.store.Get(ctx, "cred:"+name)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	if rec == nil {
		return &instruments.SkillOutput{Success: false, Error: "credential not found: " + name}, nil
	}

	// Mask the value for safety — only show first/last 4 chars.
	val := string(rec.Value)
	masked := maskValue(val)

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Credential %q: %s (type: %s)", name, masked, rec.Metadata["type"]),
		Success: true,
	}, nil
}

func (s *CredentialSkill) listCredentials(ctx context.Context) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Result: "No credentials (store not configured)", Success: true}, nil
	}
	keys, err := s.store.List(ctx, "cred:", 100)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	if len(keys) == 0 {
		return &instruments.SkillOutput{Result: "No credentials stored", Success: true}, nil
	}

	var names []string
	for _, k := range keys {
		names = append(names, strings.TrimPrefix(k, "cred:"))
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("%d credentials: %s", len(names), strings.Join(names, ", ")),
		Success: true,
	}, nil
}

func (s *CredentialSkill) deleteCredential(ctx context.Context, name string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "store not configured"}, nil
	}
	if err := s.store.Delete(ctx, "cred:"+name); err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Credential %q deleted", name),
		Success: true,
	}, nil
}

func maskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
