package observability

import (
	"sort"
	"sync"
	"time"
)

// MetricType categorizes what is being measured.
type MetricType string

const (
	MetricRuns       MetricType = "runs"
	MetricQuality    MetricType = "quality"
	MetricCost       MetricType = "cost"
	MetricLatency    MetricType = "latency_ms"
	MetricFitness    MetricType = "fitness"
	MetricReflection MetricType = "reflection"
	MetricErrors     MetricType = "errors"
	MetricPatterns   MetricType = "patterns"
)

// MetricPoint is a single recorded data point.
type MetricPoint struct {
	Type      MetricType `json:"type"`
	Value     float64    `json:"value"`
	Labels    Labels     `json:"labels,omitempty"` // e.g., {"skill_id": "sk_1"}
	Timestamp time.Time  `json:"timestamp"`
}

// Labels are key-value metadata on a metric.
type Labels map[string]string

// MetricsCollector collects in-memory metrics with rolling window.
type MetricsCollector struct {
	mu       sync.RWMutex
	points   []MetricPoint
	maxSize  int // Ring buffer capacity
	counters map[string]int64
}

// NewMetricsCollector creates a collector with a max ring buffer size.
func NewMetricsCollector(maxSize int) *MetricsCollector {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &MetricsCollector{
		points:   make([]MetricPoint, 0, maxSize),
		maxSize:  maxSize,
		counters: make(map[string]int64),
	}
}

// Record adds a metric data point.
func (c *MetricsCollector) Record(mt MetricType, value float64, labels Labels) {
	c.mu.Lock()
	defer c.mu.Unlock()

	point := MetricPoint{
		Type:      mt,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}

	if len(c.points) >= c.maxSize {
		// Shift left (drop oldest).
		copy(c.points, c.points[1:])
		c.points[len(c.points)-1] = point
	} else {
		c.points = append(c.points, point)
	}
}

// Increment increments a named counter.
func (c *MetricsCollector) Increment(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters[name]++
}

// IncrementBy increments a named counter by n.
func (c *MetricsCollector) IncrementBy(name string, n int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters[name] += n
}

// Counter returns the current value of a counter.
func (c *MetricsCollector) Counter(name string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.counters[name]
}

// Query returns metric points matching type and optional time window.
// If since is zero, returns all points of this type.
func (c *MetricsCollector) Query(mt MetricType, since time.Time) []MetricPoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []MetricPoint
	for _, p := range c.points {
		if p.Type != mt {
			continue
		}
		if !since.IsZero() && p.Timestamp.Before(since) {
			continue
		}
		result = append(result, p)
	}
	return result
}

// QueryWithLabel returns points matching type and a label key=value.
func (c *MetricsCollector) QueryWithLabel(mt MetricType, key, value string) []MetricPoint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []MetricPoint
	for _, p := range c.points {
		if p.Type != mt {
			continue
		}
		if p.Labels != nil && p.Labels[key] == value {
			result = append(result, p)
		}
	}
	return result
}

// Summary computes aggregate statistics for a metric type.
type Summary struct {
	Count  int     `json:"count"`
	Sum    float64 `json:"sum"`
	Mean   float64 `json:"mean"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	P50    float64 `json:"p50"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
}

// Summarize returns aggregate statistics for a metric type.
func (c *MetricsCollector) Summarize(mt MetricType, since time.Time) Summary {
	points := c.Query(mt, since)
	if len(points) == 0 {
		return Summary{}
	}

	values := make([]float64, len(points))
	sum := 0.0
	for i, p := range points {
		values[i] = p.Value
		sum += p.Value
	}
	sort.Float64s(values)

	return Summary{
		Count: len(values),
		Sum:   sum,
		Mean:  sum / float64(len(values)),
		Min:   values[0],
		Max:   values[len(values)-1],
		P50:   percentile(values, 0.50),
		P95:   percentile(values, 0.95),
		P99:   percentile(values, 0.99),
	}
}

// Len returns total number of recorded points.
func (c *MetricsCollector) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.points)
}

// Reset clears all points and counters.
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.points = c.points[:0]
	c.counters = make(map[string]int64)
}

// Snapshot returns a copy of current counters.
func (c *MetricsCollector) Snapshot() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := make(map[string]int64, len(c.counters))
	for k, v := range c.counters {
		snap[k] = v
	}
	return snap
}

// percentile computes the p-th percentile from sorted values.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
