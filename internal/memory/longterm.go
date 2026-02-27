package memory

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// LongTermEntry represents a summarised memory stored persistently.
type LongTermEntry struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Tags        []string  `json:"tags"`
	SourceRunID string    `json:"source_run_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// LongTermMemory provides SQLite-backed persistent memory with FTS5 full-text search.
type LongTermMemory struct {
	db *sql.DB
}

// NewLongTermMemory opens (or creates) a SQLite database at dbPath and
// ensures the required tables and FTS5 virtual table exist.
func NewLongTermMemory(dbPath string) (*LongTermMemory, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	createSQL := `
	CREATE TABLE IF NOT EXISTS long_term_memory (
		id          TEXT PRIMARY KEY,
		summary     TEXT NOT NULL,
		tags        TEXT NOT NULL DEFAULT '',
		source_run_id TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS long_term_memory_fts USING fts5(
		id UNINDEXED,
		summary,
		tags,
		content=long_term_memory,
		content_rowid=rowid
	);

	CREATE TRIGGER IF NOT EXISTS long_term_memory_ai AFTER INSERT ON long_term_memory BEGIN
		INSERT INTO long_term_memory_fts(rowid, id, summary, tags)
		VALUES (new.rowid, new.id, new.summary, new.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS long_term_memory_ad AFTER DELETE ON long_term_memory BEGIN
		INSERT INTO long_term_memory_fts(long_term_memory_fts, rowid, id, summary, tags)
		VALUES ('delete', old.rowid, old.id, old.summary, old.tags);
	END;

	CREATE TRIGGER IF NOT EXISTS long_term_memory_au AFTER UPDATE ON long_term_memory BEGIN
		INSERT INTO long_term_memory_fts(long_term_memory_fts, rowid, id, summary, tags)
		VALUES ('delete', old.rowid, old.id, old.summary, old.tags);
		INSERT INTO long_term_memory_fts(rowid, id, summary, tags)
		VALUES (new.rowid, new.id, new.summary, new.tags);
	END;
	`

	if _, err := db.Exec(createSQL); err != nil {
		db.Close()
		return nil, err
	}

	return &LongTermMemory{db: db}, nil
}

// Store persists a LongTermEntry into the database.
func (l *LongTermMemory) Store(entry LongTermEntry) error {
	tags := strings.Join(entry.Tags, ",")
	_, err := l.db.Exec(
		`INSERT OR REPLACE INTO long_term_memory (id, summary, tags, source_run_id, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		entry.ID, entry.Summary, tags, entry.SourceRunID, entry.CreatedAt,
	)
	return err
}

// Search performs a full-text search using FTS5 MATCH and returns up to limit results.
func (l *LongTermMemory) Search(query string, limit int) ([]LongTermEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := l.db.Query(
		`SELECT m.id, m.summary, m.tags, m.source_run_id, m.created_at
		 FROM long_term_memory m
		 JOIN long_term_memory_fts f ON m.id = f.id
		 WHERE long_term_memory_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLongTermRows(rows)
}

// GetAll returns up to limit entries ordered by creation time descending.
func (l *LongTermMemory) GetAll(limit int) ([]LongTermEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := l.db.Query(
		`SELECT id, summary, tags, source_run_id, created_at
		 FROM long_term_memory
		 ORDER BY created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLongTermRows(rows)
}

// Close closes the underlying database connection.
func (l *LongTermMemory) Close() error {
	return l.db.Close()
}

// DB returns the underlying *sql.DB so other subsystems (e.g. PatternTracker)
// can share the same database connection.
func (l *LongTermMemory) DB() *sql.DB {
	return l.db
}

func scanLongTermRows(rows *sql.Rows) ([]LongTermEntry, error) {
	var entries []LongTermEntry
	for rows.Next() {
		var e LongTermEntry
		var tags string
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.Summary, &tags, &e.SourceRunID, &createdAt); err != nil {
			return nil, err
		}
		if tags != "" {
			e.Tags = strings.Split(tags, ",")
		}
		e.CreatedAt = createdAt
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
