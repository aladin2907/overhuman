package genui

import (
	"sync"
)

// ReflectionStore stores UI interaction patterns for learning.
type ReflectionStore struct {
	mu      sync.Mutex
	records []UIReflection
}

// NewReflectionStore creates a new reflection store.
func NewReflectionStore() *ReflectionStore {
	return &ReflectionStore{}
}

// Record saves a UI interaction for future learning.
func (s *ReflectionStore) Record(r UIReflection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

// BuildHints generates UI hints from interaction history for a given fingerprint.
func (s *ReflectionStore) BuildHints(fingerprint string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hints []string
	for _, r := range s.records {
		if r.Dismissed {
			hints = append(hints, "User dismissed UI without interaction — try a simpler layout")
		}
		if len(r.ActionsUsed) > 0 {
			for _, a := range r.ActionsUsed {
				hints = append(hints, "User frequently uses action: "+a)
			}
		}
		if r.Scrolled {
			hints = append(hints, "User had to scroll — try more compact layout")
		}
	}

	return hints
}

// Records returns all stored reflections.
func (s *ReflectionStore) Records() []UIReflection {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]UIReflection, len(s.records))
	copy(cp, s.records)
	return cp
}
