// Package memory — SKB (Shared Knowledge Base) component.
//
// SKB enables inter-agent experience sharing:
//   - Propagation up: insight → parent agent
//   - Propagation down: best practice → child agents
//   - Horizontal: peer agents share patterns and skills
//
// Backed by SQLite for persistence.
package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SKBEntryType categorizes what kind of knowledge is being shared.
type SKBEntryType string

const (
	SKBPattern  SKBEntryType = "PATTERN"   // Recurring task pattern
	SKBInsight  SKBEntryType = "INSIGHT"   // Reflection insight
	SKBSkill    SKBEntryType = "SKILL"     // Skill metadata/template
	SKBStrategy SKBEntryType = "STRATEGY"  // Strategy or policy
)

// SKBDirection indicates the propagation direction.
type SKBDirection string

const (
	SKBDirUp         SKBDirection = "UP"         // Child → parent
	SKBDirDown       SKBDirection = "DOWN"       // Parent → children
	SKBDirHorizontal SKBDirection = "HORIZONTAL" // Peer → peer
)

// SKBEntry is a single piece of shared knowledge.
type SKBEntry struct {
	ID          string       `json:"id"`
	Type        SKBEntryType `json:"type"`
	SourceAgent string       `json:"source_agent"` // Agent that created this knowledge
	Content     string       `json:"content"`       // The knowledge itself
	Tags        []string     `json:"tags"`
	Fitness     float64      `json:"fitness"`       // How useful this has been (0-1)
	UsageCount  int          `json:"usage_count"`   // How many agents have used it
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// SharedKnowledgeBase manages inter-agent experience sharing.
type SharedKnowledgeBase struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewSharedKnowledgeBase creates a new SKB backed by a SQLite database.
func NewSharedKnowledgeBase(db *sql.DB) (*SharedKnowledgeBase, error) {
	skb := &SharedKnowledgeBase{db: db}
	if err := skb.initSchema(); err != nil {
		return nil, fmt.Errorf("SKB init: %w", err)
	}
	return skb, nil
}

func (s *SharedKnowledgeBase) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS skb_entries (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			source_agent TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT DEFAULT '',
			fitness REAL DEFAULT 0.5,
			usage_count INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_skb_type ON skb_entries(type);
		CREATE INDEX IF NOT EXISTS idx_skb_source ON skb_entries(source_agent);
		CREATE INDEX IF NOT EXISTS idx_skb_fitness ON skb_entries(fitness DESC);
	`)
	return err
}

// Store adds or updates an entry in the SKB.
func (s *SharedKnowledgeBase) Store(entry SKBEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	tags := joinTags(entry.Tags)

	_, err := s.db.Exec(`
		INSERT INTO skb_entries (id, type, source_agent, content, tags, fitness, usage_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			tags = excluded.tags,
			fitness = excluded.fitness,
			usage_count = excluded.usage_count,
			updated_at = excluded.updated_at
	`, entry.ID, entry.Type, entry.SourceAgent, entry.Content, tags, entry.Fitness, entry.UsageCount, entry.CreatedAt.Format(time.RFC3339), now)
	return err
}

// Search finds entries matching a query (substring match on content and tags).
func (s *SharedKnowledgeBase) Search(query string, limit int) ([]SKBEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, type, source_agent, content, tags, fitness, usage_count, created_at, updated_at
		FROM skb_entries
		WHERE content LIKE ? OR tags LIKE ?
		ORDER BY fitness DESC, usage_count DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSKBRows(rows)
}

// FindByType returns entries of a specific type.
func (s *SharedKnowledgeBase) FindByType(entryType SKBEntryType, limit int) ([]SKBEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, type, source_agent, content, tags, fitness, usage_count, created_at, updated_at
		FROM skb_entries
		WHERE type = ?
		ORDER BY fitness DESC, usage_count DESC
		LIMIT ?
	`, entryType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSKBRows(rows)
}

// FindByAgent returns entries from a specific source agent.
func (s *SharedKnowledgeBase) FindByAgent(agentID string, limit int) ([]SKBEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT id, type, source_agent, content, tags, fitness, usage_count, created_at, updated_at
		FROM skb_entries
		WHERE source_agent = ?
		ORDER BY fitness DESC
		LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSKBRows(rows)
}

// TopEntries returns the highest-fitness entries across all types.
func (s *SharedKnowledgeBase) TopEntries(limit int) ([]SKBEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.Query(`
		SELECT id, type, source_agent, content, tags, fitness, usage_count, created_at, updated_at
		FROM skb_entries
		ORDER BY fitness DESC, usage_count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSKBRows(rows)
}

// RecordUsage increments the usage count and updates fitness.
func (s *SharedKnowledgeBase) RecordUsage(id string, newFitness float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)

	// Running average of fitness.
	_, err := s.db.Exec(`
		UPDATE skb_entries
		SET usage_count = usage_count + 1,
		    fitness = (fitness * usage_count + ?) / (usage_count + 1),
		    updated_at = ?
		WHERE id = ?
	`, newFitness, now, id)
	return err
}

// Delete removes an entry.
func (s *SharedKnowledgeBase) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM skb_entries WHERE id = ?`, id)
	return err
}

// Count returns total entries in the SKB.
func (s *SharedKnowledgeBase) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM skb_entries`).Scan(&count)
	return count, err
}

// Propagate copies high-fitness entries to another SKB instance (e.g., parent or child).
// Returns the number of entries propagated.
func (s *SharedKnowledgeBase) Propagate(target *SharedKnowledgeBase, direction SKBDirection, minFitness float64, limit int) (int, error) {
	entries, err := s.TopEntries(limit)
	if err != nil {
		return 0, fmt.Errorf("propagate read: %w", err)
	}

	count := 0
	for _, e := range entries {
		if e.Fitness < minFitness {
			continue
		}
		// Tag with propagation direction.
		e.Tags = append(e.Tags, "propagated", string(direction))
		if err := target.Store(e); err != nil {
			return count, fmt.Errorf("propagate write: %w", err)
		}
		count++
	}
	return count, nil
}

// --- helpers ---

func scanSKBRows(rows *sql.Rows) ([]SKBEntry, error) {
	var entries []SKBEntry
	for rows.Next() {
		var e SKBEntry
		var tags, createdAt, updatedAt string
		if err := rows.Scan(&e.ID, &e.Type, &e.SourceAgent, &e.Content, &tags, &e.Fitness, &e.UsageCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		e.Tags = splitTags(tags)
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
