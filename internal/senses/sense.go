package senses

import (
	"context"
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// Sense — the interface every input-channel adapter must implement.
// ---------------------------------------------------------------------------

// Sense is the interface that ALL input channel adapters must implement.
// Each Sense represents one way the Overhuman system can receive signals
// from the outside world (Telegram, CLI, webhooks, timers, etc.).
type Sense interface {
	// Name returns the human-readable name of this sense (e.g. "Telegram", "CLI").
	Name() string

	// Start begins listening for input. Received inputs are sent to the
	// provided channel. The method should block until ctx is cancelled or
	// an unrecoverable error occurs.
	Start(ctx context.Context, out chan<- *UnifiedInput) error

	// Send sends a response back through this channel.
	// target identifies the specific destination (e.g. a chat ID).
	Send(ctx context.Context, target string, message string) error

	// Stop gracefully stops the sense, releasing any resources.
	Stop() error
}

// ---------------------------------------------------------------------------
// SenseRegistry — manages multiple Sense implementations.
// ---------------------------------------------------------------------------

// SenseRegistry manages multiple Sense implementations and provides
// convenience methods for bulk operations (start all, stop all).
type SenseRegistry struct {
	mu            sync.RWMutex
	senses        map[string]Sense
	sourceMap     map[SourceType]string // SourceType → sense name
	primaryName   string               // primary sense name for notifications
	primaryTarget string               // primary target (e.g. chat_id)
}

// sourceTypeToSenseName maps SourceType constants to Sense.Name() values.
var sourceTypeToSenseName = map[SourceType]string{
	SourceTelegram: "Telegram",
	SourceSlack:    "Slack",
	SourceDiscord:  "Discord",
	SourceEmail:    "Email",
	SourceAPI:      "API",
}

// NewSenseRegistry creates a new, empty SenseRegistry.
func NewSenseRegistry() *SenseRegistry {
	return &SenseRegistry{
		senses:    make(map[string]Sense),
		sourceMap: make(map[SourceType]string),
	}
}

// Register adds a Sense to the registry, keyed by its Name().
// If a sense with the same name already exists it is replaced.
func (r *SenseRegistry) Register(sense Sense) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.senses[sense.Name()] = sense
}

// Get returns the Sense registered under the given name, or nil if not found.
func (r *SenseRegistry) Get(name string) Sense {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.senses[name]
}

// GetBySourceType returns the Sense that handles the given SourceType.
// Uses the sourceTypeToSenseName mapping to find the right adapter.
func (r *SenseRegistry) GetBySourceType(st SourceType) Sense {
	name, ok := sourceTypeToSenseName[st]
	if !ok {
		return nil
	}
	return r.Get(name)
}

// SetPrimary designates a sense as the primary notification channel.
// target is the default destination (e.g. Telegram chat_id).
func (r *SenseRegistry) SetPrimary(senseName string, target string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.primaryName = senseName
	r.primaryTarget = target
}

// GetPrimary returns the primary sense and its default target for notifications.
// Returns nil, "" if no primary is set.
func (r *SenseRegistry) GetPrimary() (Sense, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.primaryName == "" {
		return nil, ""
	}
	return r.senses[r.primaryName], r.primaryTarget
}

// StartAll starts every registered Sense in its own goroutine.
// All senses share the same output channel. If any sense fails to start
// an error describing the failure is returned, but other senses that
// started successfully keep running.
func (r *SenseRegistry) StartAll(ctx context.Context, out chan<- *UnifiedInput) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)

	for _, s := range r.senses {
		wg.Add(1)
		go func(s Sense) {
			defer wg.Done()
			if err := s.Start(ctx, out); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("sense %q: %w", s.Name(), err))
				mu.Unlock()
			}
		}(s)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("start errors: %v", errs)
	}
	return nil
}

// StopAll gracefully stops every registered Sense. If any sense fails to
// stop, the first error is returned but all senses are still attempted.
func (r *SenseRegistry) StopAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var firstErr error
	for _, s := range r.senses {
		if err := s.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
