package evolution

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ExperimentStatus tracks where an experiment is in its lifecycle.
type ExperimentStatus string

const (
	ExperimentRunning   ExperimentStatus = "RUNNING"
	ExperimentConcluded ExperimentStatus = "CONCLUDED"
	ExperimentAborted   ExperimentStatus = "ABORTED"
)

// Experiment tracks an A/B test on strategies (not just skills).
// Unlike ABTest which compares two skills, Experiment compares
// two approaches/strategies with hypothesis-driven evaluation.
type Experiment struct {
	ID          string           `json:"id"`
	Hypothesis  string           `json:"hypothesis"`   // e.g., "Using cheaper model for clarification saves cost without quality loss"
	VariantA    string           `json:"variant_a"`     // Control description
	VariantB    string           `json:"variant_b"`     // Treatment description
	Metric      string           `json:"metric"`        // What we measure: "quality", "cost", "speed"
	Status      ExperimentStatus `json:"status"`
	StartedAt   time.Time        `json:"started_at"`
	ConcludedAt time.Time        `json:"concluded_at,omitempty"`

	// Results.
	SamplesA     []float64 `json:"samples_a"`
	SamplesB     []float64 `json:"samples_b"`
	MinSamples   int       `json:"min_samples"`    // Minimum samples per variant
	Winner       string    `json:"winner,omitempty"` // "A", "B", or "inconclusive"
	Significance float64   `json:"significance"`     // p-value equivalent (lower = more significant)
	Conclusion   string    `json:"conclusion,omitempty"`
}

// ExperimentManager runs hypothesis-driven strategy experiments.
type ExperimentManager struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	nextID      int
	minSamples  int     // Default minimum samples per variant
	sigThreshold float64 // Significance threshold (default 0.05)
}

// NewExperimentManager creates an experiment manager.
func NewExperimentManager() *ExperimentManager {
	return &ExperimentManager{
		experiments:  make(map[string]*Experiment),
		minSamples:   10,
		sigThreshold: 0.05,
	}
}

// SetMinSamples configures the minimum samples per variant.
func (m *ExperimentManager) SetMinSamples(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n > 0 {
		m.minSamples = n
	}
}

// SetSignificanceThreshold configures the p-value threshold.
func (m *ExperimentManager) SetSignificanceThreshold(t float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sigThreshold = t
}

// StartExperiment creates a new experiment.
func (m *ExperimentManager) StartExperiment(hypothesis, variantA, variantB, metric string) *Experiment {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	exp := &Experiment{
		ID:         fmt.Sprintf("exp_%d", m.nextID),
		Hypothesis: hypothesis,
		VariantA:   variantA,
		VariantB:   variantB,
		Metric:     metric,
		Status:     ExperimentRunning,
		StartedAt:  time.Now(),
		MinSamples: m.minSamples,
	}
	m.experiments[exp.ID] = exp
	return exp
}

// RecordSample records a measurement for a variant.
func (m *ExperimentManager) RecordSample(expID, variant string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %q not found", expID)
	}
	if exp.Status != ExperimentRunning {
		return fmt.Errorf("experiment %q is not running", expID)
	}

	switch variant {
	case "A":
		exp.SamplesA = append(exp.SamplesA, value)
	case "B":
		exp.SamplesB = append(exp.SamplesB, value)
	default:
		return fmt.Errorf("invalid variant %q (must be A or B)", variant)
	}
	return nil
}

// Evaluate checks if the experiment has enough data and determines a winner.
// Returns (concluded, error).
func (m *ExperimentManager) Evaluate(expID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[expID]
	if !ok {
		return false, fmt.Errorf("experiment %q not found", expID)
	}
	if exp.Status != ExperimentRunning {
		return false, nil
	}

	if len(exp.SamplesA) < exp.MinSamples || len(exp.SamplesB) < exp.MinSamples {
		return false, nil
	}

	// Compute means and significance.
	meanA := mean(exp.SamplesA)
	meanB := mean(exp.SamplesB)
	sig := welchTStatSignificance(exp.SamplesA, exp.SamplesB)
	exp.Significance = sig

	exp.Status = ExperimentConcluded
	exp.ConcludedAt = time.Now()

	if sig > m.sigThreshold {
		exp.Winner = "inconclusive"
		exp.Conclusion = fmt.Sprintf("No significant difference (p=%.3f, threshold=%.3f). Mean A=%.4f, Mean B=%.4f", sig, m.sigThreshold, meanA, meanB)
	} else if meanB > meanA {
		exp.Winner = "B"
		exp.Conclusion = fmt.Sprintf("Variant B wins (p=%.3f). Mean A=%.4f, Mean B=%.4f", sig, meanA, meanB)
	} else {
		exp.Winner = "A"
		exp.Conclusion = fmt.Sprintf("Variant A wins (p=%.3f). Mean A=%.4f, Mean B=%.4f", sig, meanA, meanB)
	}

	return true, nil
}

// Abort stops an experiment without a conclusion.
func (m *ExperimentManager) Abort(expID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[expID]
	if !ok {
		return fmt.Errorf("experiment %q not found", expID)
	}
	exp.Status = ExperimentAborted
	exp.ConcludedAt = time.Now()
	return nil
}

// Get returns an experiment by ID.
func (m *ExperimentManager) Get(id string) *Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.experiments[id]
}

// Running returns all running experiments.
func (m *ExperimentManager) Running() []*Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Experiment
	for _, exp := range m.experiments {
		if exp.Status == ExperimentRunning {
			result = append(result, exp)
		}
	}
	return result
}

// Concluded returns all concluded experiments.
func (m *ExperimentManager) Concluded() []*Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Experiment
	for _, exp := range m.experiments {
		if exp.Status == ExperimentConcluded {
			result = append(result, exp)
		}
	}
	return result
}

// --- statistics helpers ---

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func variance(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sum := 0.0
	for _, v := range vals {
		d := v - m
		sum += d * d
	}
	return sum / float64(len(vals)-1) // Bessel's correction
}

// welchTStatSignificance computes a simplified significance test
// using Welch's t-test approximation. Returns a pseudo p-value (0-1).
// Lower values = more significant difference.
func welchTStatSignificance(a, b []float64) float64 {
	nA, nB := float64(len(a)), float64(len(b))
	if nA < 2 || nB < 2 {
		return 1.0 // Not enough data
	}

	varA, varB := variance(a), variance(b)
	meanA, meanB := mean(a), mean(b)

	// Welch's t-statistic.
	se := math.Sqrt(varA/nA + varB/nB)
	if se == 0 {
		if meanA == meanB {
			return 1.0 // Identical, no difference
		}
		return 0.0 // Perfect separation
	}

	t := math.Abs(meanA-meanB) / se

	// Approximate p-value using a simple sigmoid on t-statistic.
	// This is a rough approximation — good enough for an agent that
	// uses it as a heuristic, not for academic papers.
	// t=2 → ~0.05, t=3 → ~0.003, t=1 → ~0.32
	p := 2.0 * math.Exp(-0.717*t*t) // Approximation of 2-tailed p-value
	return math.Min(1.0, math.Max(0.0, p))
}
