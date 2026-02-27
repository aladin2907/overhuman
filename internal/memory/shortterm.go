package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryEntry represents a single interaction stored in short-term memory.
type MemoryEntry struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"` // "user", "assistant", "system"
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// ShortTermMemory is a thread-safe ring buffer that holds the last N interactions.
type ShortTermMemory struct {
	mu      sync.RWMutex
	entries []MemoryEntry
	maxSize int
	head    int // write position (next slot to overwrite)
	count   int // how many entries currently stored
}

// NewShortTermMemory creates a new ring buffer with the given maximum capacity.
// If maxSize <= 0, it defaults to 50.
func NewShortTermMemory(maxSize int) *ShortTermMemory {
	if maxSize <= 0 {
		maxSize = 50
	}
	return &ShortTermMemory{
		entries: make([]MemoryEntry, maxSize),
		maxSize: maxSize,
	}
}

// Add inserts a new memory entry into the ring buffer.
func (s *ShortTermMemory) Add(role, content string, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := MemoryEntry{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	s.entries[s.head] = entry
	s.head = (s.head + 1) % s.maxSize
	if s.count < s.maxSize {
		s.count++
	}
}

// GetRecent returns the most recent n entries in chronological order (oldest first).
// If n exceeds the number of stored entries, all entries are returned.
func (s *ShortTermMemory) GetRecent(n int) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 {
		return nil
	}
	if n > s.count {
		n = s.count
	}

	result := make([]MemoryEntry, n)
	// The most recent entry is at (head-1), the one before at (head-2), etc.
	// We want chronological order, so oldest of the n first.
	for i := 0; i < n; i++ {
		idx := (s.head - n + i + s.maxSize) % s.maxSize
		result[i] = s.entries[idx]
	}
	return result
}

// GetAll returns all stored entries in chronological order (oldest first).
func (s *ShortTermMemory) GetAll() []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getAll()
}

func (s *ShortTermMemory) getAll() []MemoryEntry {
	result := make([]MemoryEntry, s.count)
	for i := 0; i < s.count; i++ {
		var idx int
		if s.count < s.maxSize {
			idx = i
		} else {
			idx = (s.head + i) % s.maxSize
		}
		result[i] = s.entries[idx]
	}
	return result
}

// Clear removes all entries from the buffer.
func (s *ShortTermMemory) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make([]MemoryEntry, s.maxSize)
	s.head = 0
	s.count = 0
}

// Len returns the number of entries currently stored.
func (s *ShortTermMemory) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.count
}
