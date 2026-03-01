package genui

import (
	"sort"
	"sync"
	"time"
)

// UIMemoryEntry records a UI generation and its effectiveness.
type UIMemoryEntry struct {
	Fingerprint  string    `json:"fingerprint"`
	Format       UIFormat  `json:"format"`
	PromptUsed   string    `json:"prompt_used"`
	ActionsUsed  []string  `json:"actions_used"`
	TimeToAction int64     `json:"time_to_action_ms"`
	Scrolled     bool      `json:"scrolled"`
	Dismissed    bool      `json:"dismissed"`
	Score        float64   `json:"score"`
	CreatedAt    time.Time `json:"created_at"`
}

// UIMemory stores historical UI generation patterns.
type UIMemory struct {
	mu       sync.Mutex
	entries  map[string][]UIMemoryEntry // keyed by fingerprint
	maxPerFP int                        // max entries per fingerprint
}

// NewUIMemory creates a new UI memory store.
func NewUIMemory(maxPerFingerprint int) *UIMemory {
	if maxPerFingerprint <= 0 {
		maxPerFingerprint = 50
	}
	return &UIMemory{
		entries:  make(map[string][]UIMemoryEntry),
		maxPerFP: maxPerFingerprint,
	}
}

// Record stores a new UI memory entry.
func (m *UIMemory) Record(entry UIMemoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.Fingerprint == "" {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	entries := m.entries[entry.Fingerprint]
	entries = append(entries, entry)

	// Trim to maxPerFP, keeping highest-scoring entries.
	if len(entries) > m.maxPerFP {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Score > entries[j].Score
		})
		entries = entries[:m.maxPerFP]
	}

	m.entries[entry.Fingerprint] = entries
}

// RecordFromReflection converts a UIReflection into a UIMemoryEntry and records it.
func (m *UIMemory) RecordFromReflection(fingerprint string, r UIReflection, format UIFormat) {
	entry := UIMemoryEntry{
		Fingerprint:  fingerprint,
		Format:       format,
		ActionsUsed:  r.ActionsUsed,
		TimeToAction: r.TimeToAction,
		Scrolled:     r.Scrolled,
		Dismissed:    r.Dismissed,
		Score:        computeUIScore(r),
		CreatedAt:    time.Now(),
	}
	m.Record(entry)
}

// Lookup returns all entries for a fingerprint, sorted by score descending.
func (m *UIMemory) Lookup(fingerprint string) []UIMemoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.entries[fingerprint]
	if len(entries) == 0 {
		return nil
	}

	cp := make([]UIMemoryEntry, len(entries))
	copy(cp, entries)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Score > cp[j].Score
	})
	return cp
}

// BestEntry returns the highest-scoring entry for a fingerprint, or nil.
func (m *UIMemory) BestEntry(fingerprint string) *UIMemoryEntry {
	entries := m.Lookup(fingerprint)
	if len(entries) == 0 {
		return nil
	}
	return &entries[0]
}

// AverageScore returns the average effectiveness score for a fingerprint.
func (m *UIMemory) AverageScore(fingerprint string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.entries[fingerprint]
	if len(entries) == 0 {
		return 0
	}
	var total float64
	for _, e := range entries {
		total += e.Score
	}
	return total / float64(len(entries))
}

// AllFingerprints returns all known fingerprints.
func (m *UIMemory) AllFingerprints() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	fps := make([]string, 0, len(m.entries))
	for fp := range m.entries {
		fps = append(fps, fp)
	}
	sort.Strings(fps)
	return fps
}

// EntryCount returns total number of entries across all fingerprints.
func (m *UIMemory) EntryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := 0
	for _, entries := range m.entries {
		total += len(entries)
	}
	return total
}

// Clear removes all entries.
func (m *UIMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string][]UIMemoryEntry)
}

// computeUIScore calculates effectiveness score from reflection data.
// Higher is better. Range: 0.0 to 1.0.
func computeUIScore(r UIReflection) float64 {
	score := 0.5 // base score

	// Dismissed = bad (user didn't engage)
	if r.Dismissed {
		return 0.1
	}

	// Actions used = good (user interacted)
	if len(r.ActionsUsed) > 0 {
		score += 0.2
		if len(r.ActionsUsed) >= 3 {
			score += 0.1 // deeply engaged
		}
	}

	// Quick interaction = good (UI was intuitive)
	if r.TimeToAction > 0 {
		switch {
		case r.TimeToAction < 3000: // < 3s
			score += 0.15
		case r.TimeToAction < 10000: // < 10s
			score += 0.05
		}
	}

	// Scrolled = slight negative (content was too long)
	if r.Scrolled {
		score -= 0.05
	}

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}
