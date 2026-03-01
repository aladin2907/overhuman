package genui

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/overhuman/overhuman/internal/pipeline"
)

// ABVariant identifies one of two UI variants.
type ABVariant string

const (
	VariantA ABVariant = "A"
	VariantB ABVariant = "B"
)

// ABTest represents an active A/B test for UI generation.
type ABTest struct {
	ID          string       `json:"id"`
	Fingerprint string       `json:"fingerprint"`
	VariantA    *GeneratedUI `json:"variant_a"`
	VariantB    *GeneratedUI `json:"variant_b"`
	Winner      ABVariant    `json:"winner,omitempty"`
	ScoreA      float64      `json:"score_a"`
	ScoreB      float64      `json:"score_b"`
	CreatedAt   time.Time    `json:"created_at"`
	ResolvedAt  time.Time    `json:"resolved_at,omitempty"`
}

// ABTestConfig configures A/B testing behavior.
type ABTestConfig struct {
	// Probability of triggering an A/B test (0.0-1.0). Default: 0.1 (10%).
	TestProbability float64

	// MinSampleSize before declaring a winner. Default: 3.
	MinSampleSize int

	// ScoreDiffThreshold to declare a winner. Default: 0.15 (15%).
	ScoreDiffThreshold float64
}

// DefaultABTestConfig returns sensible defaults.
func DefaultABTestConfig() ABTestConfig {
	return ABTestConfig{
		TestProbability:    0.1,
		MinSampleSize:      3,
		ScoreDiffThreshold: 0.15,
	}
}

// ABTestEngine manages UI A/B tests.
type ABTestEngine struct {
	mu      sync.Mutex
	config  ABTestConfig
	active  map[string]*ABTest // by fingerprint
	history []ABTest
	memory  *UIMemory
	rng     *rand.Rand
}

// NewABTestEngine creates a new A/B test engine.
func NewABTestEngine(cfg ABTestConfig, memory *UIMemory) *ABTestEngine {
	return &ABTestEngine{
		config: cfg,
		active: make(map[string]*ABTest),
		memory: memory,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ShouldTest returns true if we should run an A/B test for this fingerprint.
func (e *ABTestEngine) ShouldTest(fingerprint string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Don't run if there's already an active test for this fingerprint.
	if _, exists := e.active[fingerprint]; exists {
		return false
	}

	return e.rng.Float64() < e.config.TestProbability
}

// CreateTest generates two UI variants for the same input.
func (e *ABTestEngine) CreateTest(
	ctx context.Context,
	gen *UIGenerator,
	result pipeline.RunResult,
	caps DeviceCapabilities,
	thought *ThoughtLog,
	hints []string,
) (*ABTest, error) {
	// Generate variant A (with standard prompt).
	uiA, err := gen.GenerateWithThought(ctx, result, caps, thought, hints)
	if err != nil {
		return nil, fmt.Errorf("variant A: %w", err)
	}
	uiA.Meta.Title = "[A] " + uiA.Meta.Title

	// Generate variant B (with variation hint appended).
	hintsB := append(hints, "Try a different layout approach: if the previous was list-based, try cards; if it was verbose, try minimal; vary the visual hierarchy")
	uiB, err := gen.GenerateWithThought(ctx, result, caps, thought, hintsB)
	if err != nil {
		return nil, fmt.Errorf("variant B: %w", err)
	}
	uiB.Meta.Title = "[B] " + uiB.Meta.Title

	test := &ABTest{
		ID:          fmt.Sprintf("ab_%d", time.Now().UnixNano()),
		Fingerprint: result.Fingerprint,
		VariantA:    uiA,
		VariantB:    uiB,
		CreatedAt:   time.Now(),
	}

	e.mu.Lock()
	e.active[result.Fingerprint] = test
	e.mu.Unlock()

	log.Printf("[ab] created test %s for fingerprint %s", test.ID, test.Fingerprint)
	return test, nil
}

// PickVariant randomly selects which variant to show.
func (e *ABTestEngine) PickVariant(test *ABTest) (*GeneratedUI, ABVariant) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.rng.Float64() < 0.5 {
		return test.VariantA, VariantA
	}
	return test.VariantB, VariantB
}

// RecordResult records feedback for a variant in an active test.
func (e *ABTestEngine) RecordResult(fingerprint string, variant ABVariant, reflection UIReflection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	test, exists := e.active[fingerprint]
	if !exists {
		return
	}

	score := computeUIScore(reflection)
	switch variant {
	case VariantA:
		test.ScoreA = runningAvg(test.ScoreA, score)
	case VariantB:
		test.ScoreB = runningAvg(test.ScoreB, score)
	}
}

// CheckWinner evaluates if a winner can be declared for a fingerprint.
// Returns the winning variant and true if resolved, or empty and false if still undecided.
func (e *ABTestEngine) CheckWinner(fingerprint string) (ABVariant, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	test, exists := e.active[fingerprint]
	if !exists {
		return "", false
	}

	diff := test.ScoreA - test.ScoreB
	if diff < 0 {
		diff = -diff
	}

	if diff < e.config.ScoreDiffThreshold {
		return "", false // not enough difference yet
	}

	var winner ABVariant
	if test.ScoreA > test.ScoreB {
		winner = VariantA
	} else {
		winner = VariantB
	}

	test.Winner = winner
	test.ResolvedAt = time.Now()

	// Move to history.
	e.history = append(e.history, *test)
	delete(e.active, fingerprint)

	log.Printf("[ab] test %s resolved: winner=%s (A=%.2f, B=%.2f)", test.ID, winner, test.ScoreA, test.ScoreB)
	return winner, true
}

// ActiveTests returns all currently active tests.
func (e *ABTestEngine) ActiveTests() []ABTest {
	e.mu.Lock()
	defer e.mu.Unlock()

	tests := make([]ABTest, 0, len(e.active))
	for _, t := range e.active {
		tests = append(tests, *t)
	}
	return tests
}

// History returns all resolved tests.
func (e *ABTestEngine) History() []ABTest {
	e.mu.Lock()
	defer e.mu.Unlock()

	cp := make([]ABTest, len(e.history))
	copy(cp, e.history)
	return cp
}

// runningAvg computes a simple exponential moving average.
func runningAvg(old, new float64) float64 {
	if old == 0 {
		return new
	}
	return old*0.7 + new*0.3
}
