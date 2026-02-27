package security

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Audit events
// ---------------------------------------------------------------------------

// AuditEventType categorizes audit events.
type AuditEventType string

const (
	AuditSkillExec     AuditEventType = "SKILL_EXEC"
	AuditSkillCreate   AuditEventType = "SKILL_CREATE"
	AuditSkillDelete   AuditEventType = "SKILL_DELETE"
	AuditCredAccess    AuditEventType = "CRED_ACCESS"
	AuditCredModify    AuditEventType = "CRED_MODIFY"
	AuditAgentSpawn    AuditEventType = "AGENT_SPAWN"
	AuditAgentRetire   AuditEventType = "AGENT_RETIRE"
	AuditPolicyChange  AuditEventType = "POLICY_CHANGE"
	AuditInjectionWarn AuditEventType = "INJECTION_WARN"
	AuditRateLimit     AuditEventType = "RATE_LIMIT"
	AuditAuthAttempt   AuditEventType = "AUTH_ATTEMPT"
	AuditSoulModify    AuditEventType = "SOUL_MODIFY"
	AuditInputBlocked  AuditEventType = "INPUT_BLOCKED"
	AuditExecDenied    AuditEventType = "EXEC_DENIED"
)

// AuditSeverity indicates the importance of an audit event.
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "INFO"
	SeverityWarn     AuditSeverity = "WARN"
	SeverityCritical AuditSeverity = "CRITICAL"
)

// AuditEvent is a single immutable audit record.
type AuditEvent struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Type      AuditEventType    `json:"type"`
	Severity  AuditSeverity     `json:"severity"`
	AgentID   string            `json:"agent_id"`
	Actor     string            `json:"actor"`    // who triggered (user/agent/system)
	Action    string            `json:"action"`   // what happened
	Resource  string            `json:"resource"`  // what was affected
	Details   map[string]string `json:"details,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Audit logger (append-only, immutable)
// ---------------------------------------------------------------------------

// AuditStore is an abstraction for persistent audit storage.
type AuditStore interface {
	Append(event AuditEvent) error
	Query(filter AuditFilter) ([]AuditEvent, error)
	Count() (int, error)
}

// AuditFilter defines criteria for querying audit events.
type AuditFilter struct {
	Since    time.Time      // Events after this time
	Until    time.Time      // Events before this time
	Type     AuditEventType // Filter by type (empty = all)
	Severity AuditSeverity  // Filter by severity (empty = all)
	AgentID  string         // Filter by agent (empty = all)
	Limit    int            // Max results (0 = default 100)
}

// AuditLogger provides an append-only audit trail for security-sensitive
// operations. Events are immutable once recorded.
type AuditLogger struct {
	mu     sync.Mutex
	store  AuditStore
	nextID int
}

// NewAuditLogger creates an AuditLogger with the given store.
func NewAuditLogger(store AuditStore) *AuditLogger {
	return &AuditLogger{
		store:  store,
		nextID: 1,
	}
}

// Log records an audit event. Returns the event ID.
func (a *AuditLogger) Log(eventType AuditEventType, severity AuditSeverity, agentID, actor, action, resource string, success bool, details map[string]string) string {
	a.mu.Lock()
	id := fmt.Sprintf("audit-%d", a.nextID)
	a.nextID++
	a.mu.Unlock()

	event := AuditEvent{
		ID:        id,
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Severity:  severity,
		AgentID:   agentID,
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Details:   details,
		Success:   success,
	}

	if a.store != nil {
		_ = a.store.Append(event) // Audit logging should never block operations.
	}

	return id
}

// LogError records a failed operation audit event.
func (a *AuditLogger) LogError(eventType AuditEventType, agentID, actor, action, resource, errMsg string, details map[string]string) string {
	a.mu.Lock()
	id := fmt.Sprintf("audit-%d", a.nextID)
	a.nextID++
	a.mu.Unlock()

	event := AuditEvent{
		ID:        id,
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Severity:  SeverityWarn,
		AgentID:   agentID,
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Details:   details,
		Success:   false,
		Error:     errMsg,
	}

	if a.store != nil {
		_ = a.store.Append(event)
	}

	return id
}

// Query retrieves audit events matching the filter.
func (a *AuditLogger) Query(filter AuditFilter) ([]AuditEvent, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no audit store configured")
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	return a.store.Query(filter)
}

// Count returns total number of audit events.
func (a *AuditLogger) Count() (int, error) {
	if a.store == nil {
		return 0, nil
	}
	return a.store.Count()
}

// ---------------------------------------------------------------------------
// In-memory audit store (for testing and small deployments)
// ---------------------------------------------------------------------------

// MemoryAuditStore is a simple in-memory audit store backed by a slice.
type MemoryAuditStore struct {
	mu     sync.RWMutex
	events []AuditEvent
}

// NewMemoryAuditStore creates a MemoryAuditStore.
func NewMemoryAuditStore() *MemoryAuditStore {
	return &MemoryAuditStore{
		events: make([]AuditEvent, 0, 1024),
	}
}

// Append adds an event (append-only).
func (s *MemoryAuditStore) Append(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

// Query returns events matching the filter.
func (s *MemoryAuditStore) Query(filter AuditFilter) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []AuditEvent
	for i := len(s.events) - 1; i >= 0; i-- {
		e := s.events[i]

		// Time filters.
		if !filter.Since.IsZero() && e.Timestamp.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && e.Timestamp.After(filter.Until) {
			continue
		}
		// Type filter.
		if filter.Type != "" && e.Type != filter.Type {
			continue
		}
		// Severity filter.
		if filter.Severity != "" && e.Severity != filter.Severity {
			continue
		}
		// Agent filter.
		if filter.AgentID != "" && e.AgentID != filter.AgentID {
			continue
		}

		results = append(results, e)
		if len(results) >= filter.Limit {
			break
		}
	}
	return results, nil
}

// Count returns the total number of events.
func (s *MemoryAuditStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events), nil
}

// MarshalJSON serializes the audit store for export/backup.
func (s *MemoryAuditStore) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.events)
}
