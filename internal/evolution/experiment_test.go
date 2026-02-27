package evolution

import (
	"math"
	"testing"
)

func TestExperimentManager_StartExperiment(t *testing.T) {
	m := NewExperimentManager()

	exp := m.StartExperiment(
		"Cheaper model saves cost",
		"haiku",
		"sonnet",
		"cost",
	)

	if exp.ID != "exp_1" {
		t.Errorf("ID = %q, want exp_1", exp.ID)
	}
	if exp.Status != ExperimentRunning {
		t.Errorf("Status = %q, want RUNNING", exp.Status)
	}
	if exp.Hypothesis != "Cheaper model saves cost" {
		t.Errorf("Hypothesis mismatch")
	}
	if exp.MinSamples != 10 {
		t.Errorf("MinSamples = %d, want 10", exp.MinSamples)
	}

	// Second experiment gets incremented ID.
	exp2 := m.StartExperiment("h2", "a", "b", "quality")
	if exp2.ID != "exp_2" {
		t.Errorf("second ID = %q, want exp_2", exp2.ID)
	}
}

func TestExperimentManager_RecordSample(t *testing.T) {
	m := NewExperimentManager()
	exp := m.StartExperiment("h", "a", "b", "quality")

	if err := m.RecordSample(exp.ID, "A", 0.8); err != nil {
		t.Fatal(err)
	}
	if err := m.RecordSample(exp.ID, "B", 0.9); err != nil {
		t.Fatal(err)
	}

	if len(exp.SamplesA) != 1 || exp.SamplesA[0] != 0.8 {
		t.Errorf("SamplesA = %v", exp.SamplesA)
	}
	if len(exp.SamplesB) != 1 || exp.SamplesB[0] != 0.9 {
		t.Errorf("SamplesB = %v", exp.SamplesB)
	}
}

func TestExperimentManager_RecordSample_NotFound(t *testing.T) {
	m := NewExperimentManager()
	err := m.RecordSample("nope", "A", 1.0)
	if err == nil {
		t.Fatal("expected error for missing experiment")
	}
}

func TestExperimentManager_RecordSample_InvalidVariant(t *testing.T) {
	m := NewExperimentManager()
	exp := m.StartExperiment("h", "a", "b", "q")

	err := m.RecordSample(exp.ID, "C", 1.0)
	if err == nil {
		t.Fatal("expected error for invalid variant")
	}
}

func TestExperimentManager_RecordSample_NotRunning(t *testing.T) {
	m := NewExperimentManager()
	exp := m.StartExperiment("h", "a", "b", "q")
	m.Abort(exp.ID)

	err := m.RecordSample(exp.ID, "A", 1.0)
	if err == nil {
		t.Fatal("expected error for aborted experiment")
	}
}

func TestExperimentManager_Evaluate_NotEnoughSamples(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(5)
	exp := m.StartExperiment("h", "a", "b", "q")

	// Only 3 samples per variant — not enough.
	for i := 0; i < 3; i++ {
		m.RecordSample(exp.ID, "A", 0.5)
		m.RecordSample(exp.ID, "B", 0.5)
	}

	concluded, err := m.Evaluate(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if concluded {
		t.Error("should not conclude with insufficient samples")
	}
	if exp.Status != ExperimentRunning {
		t.Errorf("Status = %q, want RUNNING", exp.Status)
	}
}

func TestExperimentManager_Evaluate_BWins(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(5)
	m.SetSignificanceThreshold(0.5) // Lenient threshold for test.

	exp := m.StartExperiment("B is better", "control", "treatment", "quality")

	// A: low values, B: high values — clear difference.
	for i := 0; i < 10; i++ {
		m.RecordSample(exp.ID, "A", 0.3+float64(i)*0.01)
		m.RecordSample(exp.ID, "B", 0.9+float64(i)*0.01)
	}

	concluded, err := m.Evaluate(exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !concluded {
		t.Fatal("should have concluded")
	}
	if exp.Status != ExperimentConcluded {
		t.Errorf("Status = %q", exp.Status)
	}
	if exp.Winner != "B" {
		t.Errorf("Winner = %q, want B", exp.Winner)
	}
}

func TestExperimentManager_Evaluate_AWins(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(5)
	m.SetSignificanceThreshold(0.5)

	exp := m.StartExperiment("A is better", "control", "treatment", "quality")

	for i := 0; i < 10; i++ {
		m.RecordSample(exp.ID, "A", 0.9+float64(i)*0.01)
		m.RecordSample(exp.ID, "B", 0.3+float64(i)*0.01)
	}

	concluded, _ := m.Evaluate(exp.ID)
	if !concluded {
		t.Fatal("should have concluded")
	}
	if exp.Winner != "A" {
		t.Errorf("Winner = %q, want A", exp.Winner)
	}
}

func TestExperimentManager_Evaluate_Inconclusive(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(5)
	m.SetSignificanceThreshold(0.01) // Very strict threshold.

	exp := m.StartExperiment("no diff", "control", "treatment", "quality")

	// Very similar values — should be inconclusive.
	for i := 0; i < 10; i++ {
		m.RecordSample(exp.ID, "A", 0.5+float64(i)*0.01)
		m.RecordSample(exp.ID, "B", 0.5+float64(i)*0.01)
	}

	concluded, _ := m.Evaluate(exp.ID)
	if !concluded {
		t.Fatal("should have concluded (enough samples)")
	}
	if exp.Winner != "inconclusive" {
		t.Errorf("Winner = %q, want inconclusive", exp.Winner)
	}
}

func TestExperimentManager_Evaluate_NotFound(t *testing.T) {
	m := NewExperimentManager()
	_, err := m.Evaluate("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExperimentManager_Abort(t *testing.T) {
	m := NewExperimentManager()
	exp := m.StartExperiment("h", "a", "b", "q")

	if err := m.Abort(exp.ID); err != nil {
		t.Fatal(err)
	}
	if exp.Status != ExperimentAborted {
		t.Errorf("Status = %q, want ABORTED", exp.Status)
	}
	if exp.ConcludedAt.IsZero() {
		t.Error("ConcludedAt should be set")
	}
}

func TestExperimentManager_Abort_NotFound(t *testing.T) {
	m := NewExperimentManager()
	err := m.Abort("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExperimentManager_Get(t *testing.T) {
	m := NewExperimentManager()
	exp := m.StartExperiment("h", "a", "b", "q")

	got := m.Get(exp.ID)
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.ID != exp.ID {
		t.Errorf("ID mismatch")
	}

	if m.Get("missing") != nil {
		t.Error("Get should return nil for missing ID")
	}
}

func TestExperimentManager_Running(t *testing.T) {
	m := NewExperimentManager()
	m.StartExperiment("h1", "a", "b", "q")
	exp2 := m.StartExperiment("h2", "a", "b", "q")
	m.StartExperiment("h3", "a", "b", "q")

	m.Abort(exp2.ID)

	running := m.Running()
	if len(running) != 2 {
		t.Errorf("Running = %d, want 2", len(running))
	}
}

func TestExperimentManager_Concluded(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(2)
	m.SetSignificanceThreshold(0.99) // Accept anything.

	exp := m.StartExperiment("h", "a", "b", "q")
	m.StartExperiment("h2", "a", "b", "q") // Not concluded.

	for i := 0; i < 5; i++ {
		m.RecordSample(exp.ID, "A", 0.5)
		m.RecordSample(exp.ID, "B", 0.5)
	}
	m.Evaluate(exp.ID)

	concluded := m.Concluded()
	if len(concluded) != 1 {
		t.Errorf("Concluded = %d, want 1", len(concluded))
	}
}

func TestMean(t *testing.T) {
	if m := mean(nil); m != 0 {
		t.Errorf("mean(nil) = %f", m)
	}
	if m := mean([]float64{1, 2, 3}); m != 2.0 {
		t.Errorf("mean = %f, want 2", m)
	}
}

func TestVariance(t *testing.T) {
	if v := variance(nil); v != 0 {
		t.Errorf("variance(nil) = %f", v)
	}
	if v := variance([]float64{1}); v != 0 {
		t.Errorf("variance(1 elem) = %f", v)
	}
	// Variance of {2,4,6}: mean=4, var = ((4+0+4)/2) = 4
	v := variance([]float64{2, 4, 6})
	if math.Abs(v-4.0) > 0.001 {
		t.Errorf("variance = %f, want 4", v)
	}
}

func TestWelchSignificance(t *testing.T) {
	// Identical samples → p = 1.0
	a := []float64{1, 1, 1, 1, 1}
	b := []float64{1, 1, 1, 1, 1}
	if p := welchTStatSignificance(a, b); p != 1.0 {
		t.Errorf("identical → p = %f, want 1.0", p)
	}

	// Very different → p close to 0
	a = []float64{0.1, 0.11, 0.12, 0.09, 0.1}
	b = []float64{0.9, 0.91, 0.92, 0.89, 0.9}
	p := welchTStatSignificance(a, b)
	if p > 0.01 {
		t.Errorf("very different → p = %f, want < 0.01", p)
	}

	// Not enough data → 1.0
	if p := welchTStatSignificance([]float64{1}, []float64{2}); p != 1.0 {
		t.Errorf("1 sample → p = %f, want 1.0", p)
	}
}

func TestExperimentManager_SetMinSamples_Zero(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(0)  // Should be ignored.
	m.SetMinSamples(-1) // Should be ignored.

	exp := m.StartExperiment("h", "a", "b", "q")
	if exp.MinSamples != 10 {
		t.Errorf("MinSamples = %d, want 10 (default)", exp.MinSamples)
	}
}

func TestExperimentManager_Evaluate_AlreadyConcluded(t *testing.T) {
	m := NewExperimentManager()
	m.SetMinSamples(2)
	exp := m.StartExperiment("h", "a", "b", "q")

	for i := 0; i < 5; i++ {
		m.RecordSample(exp.ID, "A", 0.5)
		m.RecordSample(exp.ID, "B", 0.9)
	}

	m.Evaluate(exp.ID) // First evaluation concludes.

	concluded, err := m.Evaluate(exp.ID) // Second should be no-op.
	if err != nil {
		t.Fatal(err)
	}
	if concluded {
		t.Error("should not conclude again")
	}
}
