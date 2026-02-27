package observability

import (
	"math"
	"testing"
	"time"
)

func TestNewMetricsCollector(t *testing.T) {
	c := NewMetricsCollector(100)
	if c.Len() != 0 {
		t.Errorf("Len = %d", c.Len())
	}
}

func TestNewMetricsCollector_ZeroSize(t *testing.T) {
	c := NewMetricsCollector(0) // Should default.
	if c.maxSize != 10000 {
		t.Errorf("maxSize = %d, want 10000", c.maxSize)
	}
}

func TestMetricsCollector_Record(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Record(MetricQuality, 0.85, Labels{"task": "t1"})
	c.Record(MetricQuality, 0.90, Labels{"task": "t2"})
	c.Record(MetricCost, 0.003, nil)

	if c.Len() != 3 {
		t.Errorf("Len = %d, want 3", c.Len())
	}
}

func TestMetricsCollector_Record_RingBuffer(t *testing.T) {
	c := NewMetricsCollector(3) // Tiny buffer.

	for i := 0; i < 5; i++ {
		c.Record(MetricRuns, float64(i), nil)
	}

	// Should have only 3 most recent.
	if c.Len() != 3 {
		t.Errorf("Len = %d, want 3", c.Len())
	}

	points := c.Query(MetricRuns, time.Time{})
	if len(points) != 3 {
		t.Fatalf("Query = %d, want 3", len(points))
	}
	// Oldest should be 2, newest 4.
	if points[0].Value != 2 {
		t.Errorf("oldest = %f, want 2", points[0].Value)
	}
	if points[2].Value != 4 {
		t.Errorf("newest = %f, want 4", points[2].Value)
	}
}

func TestMetricsCollector_Counter(t *testing.T) {
	c := NewMetricsCollector(100)

	c.Increment("runs")
	c.Increment("runs")
	c.Increment("errors")
	c.IncrementBy("cost_micros", 300)

	if c.Counter("runs") != 2 {
		t.Errorf("runs = %d", c.Counter("runs"))
	}
	if c.Counter("errors") != 1 {
		t.Errorf("errors = %d", c.Counter("errors"))
	}
	if c.Counter("cost_micros") != 300 {
		t.Errorf("cost_micros = %d", c.Counter("cost_micros"))
	}
	if c.Counter("missing") != 0 {
		t.Errorf("missing counter = %d", c.Counter("missing"))
	}
}

func TestMetricsCollector_Query(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Record(MetricQuality, 0.8, nil)
	c.Record(MetricCost, 0.01, nil)
	c.Record(MetricQuality, 0.9, nil)

	qPoints := c.Query(MetricQuality, time.Time{})
	if len(qPoints) != 2 {
		t.Errorf("quality points = %d, want 2", len(qPoints))
	}

	cPoints := c.Query(MetricCost, time.Time{})
	if len(cPoints) != 1 {
		t.Errorf("cost points = %d, want 1", len(cPoints))
	}
}

func TestMetricsCollector_Query_TimeSince(t *testing.T) {
	c := NewMetricsCollector(100)

	// Record a point, sleep briefly, record another.
	c.Record(MetricQuality, 0.5, nil)
	midpoint := time.Now()
	time.Sleep(2 * time.Millisecond)
	c.Record(MetricQuality, 0.9, nil)

	recent := c.Query(MetricQuality, midpoint)
	if len(recent) != 1 {
		t.Errorf("recent = %d, want 1", len(recent))
	}
	if len(recent) > 0 && recent[0].Value != 0.9 {
		t.Errorf("recent value = %f", recent[0].Value)
	}
}

func TestMetricsCollector_QueryWithLabel(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Record(MetricFitness, 0.8, Labels{"skill_id": "sk_1"})
	c.Record(MetricFitness, 0.6, Labels{"skill_id": "sk_2"})
	c.Record(MetricFitness, 0.9, Labels{"skill_id": "sk_1"})
	c.Record(MetricFitness, 0.7, nil) // No labels.

	results := c.QueryWithLabel(MetricFitness, "skill_id", "sk_1")
	if len(results) != 2 {
		t.Errorf("sk_1 results = %d, want 2", len(results))
	}
}

func TestMetricsCollector_Summarize(t *testing.T) {
	c := NewMetricsCollector(100)
	// Values: 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0
	for i := 1; i <= 10; i++ {
		c.Record(MetricQuality, float64(i)/10, nil)
	}

	s := c.Summarize(MetricQuality, time.Time{})
	if s.Count != 10 {
		t.Errorf("Count = %d", s.Count)
	}
	if math.Abs(s.Mean-0.55) > 0.001 {
		t.Errorf("Mean = %f, want ~0.55", s.Mean)
	}
	if s.Min != 0.1 {
		t.Errorf("Min = %f", s.Min)
	}
	if s.Max != 1.0 {
		t.Errorf("Max = %f", s.Max)
	}
	// P50 of [0.1..1.0] ≈ 0.55
	if math.Abs(s.P50-0.55) > 0.01 {
		t.Errorf("P50 = %f, want ~0.55", s.P50)
	}
	// P95 should be near 0.955
	if s.P95 < 0.9 {
		t.Errorf("P95 = %f, too low", s.P95)
	}
}

func TestMetricsCollector_Summarize_Empty(t *testing.T) {
	c := NewMetricsCollector(100)
	s := c.Summarize(MetricQuality, time.Time{})
	if s.Count != 0 {
		t.Errorf("Count = %d", s.Count)
	}
}

func TestMetricsCollector_Summarize_SinglePoint(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Record(MetricCost, 0.42, nil)

	s := c.Summarize(MetricCost, time.Time{})
	if s.Count != 1 {
		t.Errorf("Count = %d", s.Count)
	}
	if s.Mean != 0.42 {
		t.Errorf("Mean = %f", s.Mean)
	}
	if s.P50 != 0.42 {
		t.Errorf("P50 = %f", s.P50)
	}
}

func TestMetricsCollector_Reset(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Record(MetricQuality, 0.5, nil)
	c.Increment("runs")

	c.Reset()
	if c.Len() != 0 {
		t.Errorf("Len after reset = %d", c.Len())
	}
	if c.Counter("runs") != 0 {
		t.Errorf("Counter after reset = %d", c.Counter("runs"))
	}
}

func TestMetricsCollector_Snapshot(t *testing.T) {
	c := NewMetricsCollector(100)
	c.Increment("a")
	c.IncrementBy("b", 5)

	snap := c.Snapshot()
	if snap["a"] != 1 {
		t.Errorf("a = %d", snap["a"])
	}
	if snap["b"] != 5 {
		t.Errorf("b = %d", snap["b"])
	}

	// Modifying snapshot shouldn't affect collector.
	snap["a"] = 999
	if c.Counter("a") != 1 {
		t.Errorf("Counter a changed after snapshot mutation")
	}
}

func TestPercentile(t *testing.T) {
	if p := percentile(nil, 0.5); p != 0 {
		t.Errorf("nil percentile = %f", p)
	}

	vals := []float64{10, 20, 30, 40, 50}
	if p := percentile(vals, 0.0); p != 10 {
		t.Errorf("p0 = %f", p)
	}
	if p := percentile(vals, 1.0); p != 50 {
		t.Errorf("p100 = %f", p)
	}
	if p := percentile(vals, 0.5); p != 30 {
		t.Errorf("p50 = %f", p)
	}
}

func TestMetricTypes(t *testing.T) {
	// Verify all metric type constants exist and are distinct.
	types := []MetricType{
		MetricRuns, MetricQuality, MetricCost, MetricLatency,
		MetricFitness, MetricReflection, MetricErrors, MetricPatterns,
	}
	seen := make(map[MetricType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate metric type: %s", mt)
		}
		seen[mt] = true
	}
}
