package memory

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

// PatternEntry represents a recognised behavioural pattern.
type PatternEntry struct {
	Fingerprint string    `json:"fingerprint"`
	Description string    `json:"description"`
	Count       int       `json:"count"`
	AvgQuality  float64   `json:"avg_quality"`
	LastSeen    time.Time `json:"last_seen"`
	SkillID     string    `json:"skill_id,omitempty"` // linked code-skill if any
}

// PatternTracker fingerprints recurring tasks and tracks repetition counts
// so the system can identify candidates for automation.
type PatternTracker struct {
	db *sql.DB
}

// NewPatternTracker creates the patterns table if it does not exist and
// returns a ready-to-use tracker.
func NewPatternTracker(db *sql.DB) (*PatternTracker, error) {
	createSQL := `
	CREATE TABLE IF NOT EXISTS patterns (
		fingerprint TEXT PRIMARY KEY,
		description TEXT NOT NULL,
		count       INTEGER NOT NULL DEFAULT 0,
		avg_quality REAL    NOT NULL DEFAULT 0,
		last_seen   DATETIME NOT NULL,
		skill_id    TEXT NOT NULL DEFAULT ''
	);`

	if _, err := db.Exec(createSQL); err != nil {
		return nil, fmt.Errorf("pattern tracker: create table: %w", err)
	}

	return &PatternTracker{db: db}, nil
}

// ComputeFingerprint returns a deterministic SHA-256 hex digest for the
// combination of goal and taskType.
func (p *PatternTracker) ComputeFingerprint(goal, taskType string) string {
	h := sha256.New()
	h.Write([]byte(goal))
	h.Write([]byte("|"))
	h.Write([]byte(taskType))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Record upserts a pattern observation. If the fingerprint already exists the
// count is incremented and the running average quality is updated. The
// resulting PatternEntry is returned.
func (p *PatternTracker) Record(fingerprint, description string, quality float64) (*PatternEntry, error) {
	now := time.Now()

	// Use an upsert (INSERT … ON CONFLICT … DO UPDATE).
	upsertSQL := `
	INSERT INTO patterns (fingerprint, description, count, avg_quality, last_seen)
	VALUES (?, ?, 1, ?, ?)
	ON CONFLICT(fingerprint) DO UPDATE SET
		description = excluded.description,
		count       = patterns.count + 1,
		avg_quality = (patterns.avg_quality * patterns.count + excluded.avg_quality) / (patterns.count + 1),
		last_seen   = excluded.last_seen;`

	if _, err := p.db.Exec(upsertSQL, fingerprint, description, quality, now); err != nil {
		return nil, fmt.Errorf("pattern tracker: record: %w", err)
	}

	return p.Get(fingerprint)
}

// GetAutomatable returns patterns whose count is >= threshold and that have
// no linked skill yet (skill_id is empty).
func (p *PatternTracker) GetAutomatable(threshold int) ([]PatternEntry, error) {
	rows, err := p.db.Query(
		`SELECT fingerprint, description, count, avg_quality, last_seen, skill_id
		 FROM patterns
		 WHERE count >= ? AND skill_id = ''
		 ORDER BY count DESC`,
		threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("pattern tracker: get automatable: %w", err)
	}
	defer rows.Close()

	return scanPatternRows(rows)
}

// LinkSkill associates a code-skill ID with the given fingerprint.
func (p *PatternTracker) LinkSkill(fingerprint, skillID string) error {
	res, err := p.db.Exec(
		`UPDATE patterns SET skill_id = ? WHERE fingerprint = ?`,
		skillID, fingerprint,
	)
	if err != nil {
		return fmt.Errorf("pattern tracker: link skill: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("pattern tracker: fingerprint %q not found", fingerprint)
	}
	return nil
}

// Get retrieves a single pattern by fingerprint.
func (p *PatternTracker) Get(fingerprint string) (*PatternEntry, error) {
	row := p.db.QueryRow(
		`SELECT fingerprint, description, count, avg_quality, last_seen, skill_id
		 FROM patterns
		 WHERE fingerprint = ?`,
		fingerprint,
	)

	var e PatternEntry
	var lastSeen time.Time
	if err := row.Scan(&e.Fingerprint, &e.Description, &e.Count, &e.AvgQuality, &lastSeen, &e.SkillID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pattern tracker: fingerprint %q not found", fingerprint)
		}
		return nil, fmt.Errorf("pattern tracker: get: %w", err)
	}
	e.LastSeen = lastSeen
	return &e, nil
}

func scanPatternRows(rows *sql.Rows) ([]PatternEntry, error) {
	var entries []PatternEntry
	for rows.Next() {
		var e PatternEntry
		var lastSeen time.Time
		if err := rows.Scan(&e.Fingerprint, &e.Description, &e.Count, &e.AvgQuality, &lastSeen, &e.SkillID); err != nil {
			return nil, err
		}
		e.LastSeen = lastSeen
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
