// Package storage provides a persistent key-value and document storage abstraction.
//
// The Store interface is the primary abstraction. SQLiteStore is the default
// implementation using pure-Go SQLite (modernc.org/sqlite).
//
// Storage is used by memory, SKB, and other subsystems that need persistence
// beyond in-memory state.
package storage

import (
	"context"
	"time"
)

// Record is a stored document with metadata.
type Record struct {
	Key       string            `json:"key"`
	Value     []byte            `json:"value"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"` // Zero means no expiry.
}

// Store is the persistent storage interface.
type Store interface {
	// Get retrieves a record by key. Returns nil if not found.
	Get(ctx context.Context, key string) (*Record, error)

	// Put stores a record (upsert).
	Put(ctx context.Context, rec Record) error

	// Delete removes a record by key.
	Delete(ctx context.Context, key string) error

	// List returns all keys matching a prefix.
	List(ctx context.Context, prefix string, limit int) ([]string, error)

	// Search performs full-text search on values. Returns matching keys.
	Search(ctx context.Context, query string, limit int) ([]Record, error)

	// Count returns the total number of records.
	Count(ctx context.Context) (int, error)

	// Close shuts down the store.
	Close() error
}
