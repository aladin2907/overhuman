package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite-backed store.
// Use ":memory:" for an in-memory database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Create tables.
	schema := `
	CREATE TABLE IF NOT EXISTS kv_store (
		key        TEXT PRIMARY KEY,
		value      BLOB NOT NULL,
		metadata   TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		expires_at TEXT
	);
	CREATE VIRTUAL TABLE IF NOT EXISTS kv_fts USING fts5(
		key, value, content='kv_store', content_rowid='rowid'
	);
	CREATE TRIGGER IF NOT EXISTS kv_ai AFTER INSERT ON kv_store BEGIN
		INSERT INTO kv_fts(rowid, key, value) VALUES (new.rowid, new.key, new.value);
	END;
	CREATE TRIGGER IF NOT EXISTS kv_ad AFTER DELETE ON kv_store BEGIN
		INSERT INTO kv_fts(kv_fts, rowid, key, value) VALUES ('delete', old.rowid, old.key, old.value);
	END;
	CREATE TRIGGER IF NOT EXISTS kv_au AFTER UPDATE ON kv_store BEGIN
		INSERT INTO kv_fts(kv_fts, rowid, key, value) VALUES ('delete', old.rowid, old.key, old.value);
		INSERT INTO kv_fts(rowid, key, value) VALUES (new.rowid, new.key, new.value);
	END;`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Get retrieves a record by key.
func (s *SQLiteStore) Get(ctx context.Context, key string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value []byte
	var metaJSON sql.NullString
	var createdAt, updatedAt string
	var expiresAt sql.NullString

	err := s.db.QueryRowContext(ctx,
		"SELECT value, metadata, created_at, updated_at, expires_at FROM kv_store WHERE key = ?",
		key,
	).Scan(&value, &metaJSON, &createdAt, &updatedAt, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}

	rec := &Record{
		Key:   key,
		Value: value,
	}
	rec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rec.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if expiresAt.Valid && expiresAt.String != "" {
		rec.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt.String)
		// Check if expired.
		if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
			return nil, nil
		}
	}
	if metaJSON.Valid && metaJSON.String != "" {
		json.Unmarshal([]byte(metaJSON.String), &rec.Metadata)
	}

	return rec, nil
}

// Put stores or updates a record.
func (s *SQLiteStore) Put(ctx context.Context, rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	rec.UpdatedAt = time.Now().UTC()

	var metaJSON *string
	if len(rec.Metadata) > 0 {
		data, _ := json.Marshal(rec.Metadata)
		s := string(data)
		metaJSON = &s
	}

	var expiresAt *string
	if !rec.ExpiresAt.IsZero() {
		s := rec.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &s
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kv_store (key, value, metadata, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at`,
		rec.Key, rec.Value, metaJSON,
		rec.CreatedAt.UTC().Format(time.RFC3339), now, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("put %q: %w", rec.Key, err)
	}
	return nil
}

// Delete removes a record by key.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM kv_store WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// List returns keys matching a prefix.
func (s *SQLiteStore) List(ctx context.Context, prefix string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	query := "SELECT key FROM kv_store WHERE key LIKE ? ORDER BY key LIMIT ?"
	rows, err := s.db.QueryContext(ctx, query, prefix+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("list prefix %q: %w", prefix, err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// Search performs full-text search.
func (s *SQLiteStore) Search(ctx context.Context, query string, limit int) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	// Sanitize query for FTS5.
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil, nil
	}
	ftsQuery := strings.Join(terms, " OR ")

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.key, s.value, s.metadata, s.created_at, s.updated_at, s.expires_at
		FROM kv_fts f
		JOIN kv_store s ON s.rowid = f.rowid
		WHERE kv_fts MATCH ?
		ORDER BY rank
		LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		var metaJSON sql.NullString
		var createdAt, updatedAt string
		var expiresAt sql.NullString

		if err := rows.Scan(&rec.Key, &rec.Value, &metaJSON, &createdAt, &updatedAt, &expiresAt); err != nil {
			return nil, err
		}

		rec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		rec.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if expiresAt.Valid {
			rec.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt.String)
		}
		if metaJSON.Valid {
			json.Unmarshal([]byte(metaJSON.String), &rec.Metadata)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// Count returns the total number of stored records.
func (s *SQLiteStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM kv_store").Scan(&count)
	return count, err
}

// Close shuts down the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
