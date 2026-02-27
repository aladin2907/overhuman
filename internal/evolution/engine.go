// Package evolution implements Darwinian selection for skills.
//
// The evolution engine:
//   - Computes fitness scores from success rate, cost, speed, quality
//   - Runs A/B tests between challenger and incumbent skills
//   - Deprecates underperforming skills
//   - Triggers auto-generation of improved skills
package evolution

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/overhuman/overhuman/internal/instruments"
)

// FitnessWeights controls how fitness is computed.
type FitnessWeights struct {
	SuccessRate float64 // Weight for success rate (0-1)
	Quality     float64 // Weight for average quality (0-1)
	CostSaving  float64 // Weight for cost efficiency (0-1)
	Speed       float64 // Weight for execution speed (0-1)
}

// DefaultWeights returns balanced default weights.
func DefaultWeights() FitnessWeights {
	return FitnessWeights{
		SuccessRate: 0.35,
		Quality:     0.30,
		CostSaving:  0.20,
		Speed:       0.15,
	}
}

// ABTest tracks an ongoing competition between two skills.
type ABTest struct {
	ID            string    `json:"id"`
	IncumbentID   string    `json:"incumbent_id"`
	ChallengerID  string    `json:"challenger_id"`
	Fingerprint   string    `json:"fingerprint"`
	StartedAt     time.Time `json:"started_at"`
	MinRuns       int       `json:"min_runs"`       // Minimum runs before deciding
	IncumbentRuns int       `json:"incumbent_runs"`
	ChallengerRuns int     `json:"challenger_runs"`
	Decided       bool      `json:"decided"`
	WinnerID      string    `json:"winner_id,omitempty"`
}

// Engine manages skill evolution.
type Engine struct {
	mu      sync.RWMutex
	weights FitnessWeights
	tests   map[string]*ABTest // keyed by test ID

	// Thresholds.
	deprecateBelow  float64 // Fitness score below which a skill gets deprecated
	observationRuns int     // How many runs to observe before evaluating
	nextTestID      int
}

// New creates an evolution engine.
func New() *Engine {
	return &Engine{
		weights:         DefaultWeights(),
		tests:           make(map[string]*ABTest),
		deprecateBelow:  0.3,
		observationRuns: 5,
	}
}

// SetWeights configures fitness weights.
func (e *Engine) SetWeights(w FitnessWeights) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.weights = w
}

// SetDeprecateThreshold sets the minimum fitness score.
func (e *Engine) SetDeprecateThreshold(threshold float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.deprecateBelow = threshold
}

// SetObservationRuns sets how many runs before evaluation.
func (e *Engine) SetObservationRuns(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.observationRuns = n
}

// ComputeFitness scores a skill from 0.0 (worst) to 1.0 (best).
func (e *Engine) ComputeFitness(meta instruments.SkillMeta) float64 {
	e.mu.RLock()
	w := e.weights
	e.mu.RUnlock()

	return computeFitness(w, meta)
}

// computeFitness is the lock-free inner implementation.
func computeFitness(w FitnessWeights, meta instruments.SkillMeta) float64 {
	if meta.TotalRuns == 0 {
		return 0.5 // Neutral for untested skills.
	}

	// Success rate component (already 0-1).
	successComponent := meta.SuccessRate

	// Quality component (already 0-1).
	qualityComponent := meta.AvgQuality

	// Cost saving: $0 = perfect, $0.10+ = worst. Normalize inversely.
	costComponent := 1.0 - math.Min(meta.AvgCostUSD/0.10, 1.0)

	// Speed: <100ms = perfect, >10000ms = worst. Log scale.
	speedComponent := 1.0 - math.Min(math.Log10(math.Max(meta.AvgElapsedMs, 1))/4.0, 1.0)

	fitness := w.SuccessRate*successComponent +
		w.Quality*qualityComponent +
		w.CostSaving*costComponent +
		w.Speed*speedComponent

	return math.Max(0, math.Min(1, fitness))
}

// ShouldDeprecate returns true if a skill's fitness is below threshold.
func (e *Engine) ShouldDeprecate(meta instruments.SkillMeta) bool {
	e.mu.RLock()
	minRuns := e.observationRuns
	threshold := e.deprecateBelow
	w := e.weights
	e.mu.RUnlock()

	if meta.TotalRuns < minRuns {
		return false // Not enough data.
	}
	return computeFitness(w, meta) < threshold
}

// StartABTest begins a competition between incumbent and challenger.
func (e *Engine) StartABTest(incumbentID, challengerID, fingerprint string) *ABTest {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextTestID++
	test := &ABTest{
		ID:           fmt.Sprintf("ab_%d", e.nextTestID),
		IncumbentID:  incumbentID,
		ChallengerID: challengerID,
		Fingerprint:  fingerprint,
		StartedAt:    time.Now(),
		MinRuns:      e.observationRuns,
	}
	e.tests[test.ID] = test
	return test
}

// RecordABRun records a run for one participant in an A/B test.
func (e *Engine) RecordABRun(testID, skillID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	test, ok := e.tests[testID]
	if !ok {
		return fmt.Errorf("A/B test %q not found", testID)
	}
	if test.Decided {
		return fmt.Errorf("A/B test %q already decided", testID)
	}

	switch skillID {
	case test.IncumbentID:
		test.IncumbentRuns++
	case test.ChallengerID:
		test.ChallengerRuns++
	default:
		return fmt.Errorf("skill %q not in test %q", skillID, testID)
	}
	return nil
}

// EvaluateABTest decides the winner if enough runs have been collected.
// Returns (winnerID, loserID, decided).
func (e *Engine) EvaluateABTest(testID string, registry *instruments.SkillRegistry) (string, string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	test, ok := e.tests[testID]
	if !ok || test.Decided {
		return "", "", false
	}

	if test.IncumbentRuns < test.MinRuns || test.ChallengerRuns < test.MinRuns {
		return "", "", false // Not enough data.
	}

	incumbent := registry.Get(test.IncumbentID)
	challenger := registry.Get(test.ChallengerID)

	if incumbent == nil || challenger == nil {
		return "", "", false
	}

	w := e.weights
	incFitness := computeFitness(w, incumbent.Meta)
	chalFitness := computeFitness(w, challenger.Meta)

	test.Decided = true

	if chalFitness > incFitness {
		test.WinnerID = test.ChallengerID
		return test.ChallengerID, test.IncumbentID, true
	}
	test.WinnerID = test.IncumbentID
	return test.IncumbentID, test.ChallengerID, true
}

// GetTest returns an A/B test by ID.
func (e *Engine) GetTest(id string) *ABTest {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tests[id]
}

// ActiveTests returns all undecided A/B tests.
func (e *Engine) ActiveTests() []*ABTest {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*ABTest
	for _, t := range e.tests {
		if !t.Decided {
			result = append(result, t)
		}
	}
	return result
}

// EvaluateAll checks all skills in the registry and returns IDs that should be deprecated.
func (e *Engine) EvaluateAll(registry *instruments.SkillRegistry) []string {
	skills := registry.List()
	var deprecated []string

	for _, s := range skills {
		if s.Meta.Status == instruments.SkillStatusDeprecated {
			continue
		}
		if e.ShouldDeprecate(s.Meta) {
			deprecated = append(deprecated, s.Meta.ID)
		}
	}
	return deprecated
}
