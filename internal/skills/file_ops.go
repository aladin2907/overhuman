package skills

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/overhuman/overhuman/internal/instruments"
)

// FileOpsSkill provides file system operations.
type FileOpsSkill struct {
	baseDir string
}

func NewFileOpsSkill(baseDir string) *FileOpsSkill {
	if baseDir == "" {
		baseDir = "."
	}
	return &FileOpsSkill{baseDir: baseDir}
}

func (s *FileOpsSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]
	path := input.Parameters["path"]

	// Ensure path is within base directory.
	if path != "" {
		path = filepath.Join(s.baseDir, filepath.Clean(path))
	}

	switch action {
	case "read":
		return s.readFile(path)
	case "write":
		content := input.Parameters["content"]
		return s.writeFile(path, content)
	case "list":
		pattern := input.Parameters["pattern"]
		return s.listFiles(path, pattern)
	case "stat":
		return s.statFile(path)
	case "search":
		query := input.Parameters["query"]
		return s.searchFiles(path, query)
	default:
		return &instruments.SkillOutput{
			Success: false,
			Error:   "action required: read, write, list, stat, search",
		}, nil
	}
}

func (s *FileOpsSkill) readFile(path string) (*instruments.SkillOutput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	// Limit to 100KB.
	content := string(data)
	if len(content) > 100*1024 {
		content = content[:100*1024] + "\n... (truncated)"
	}
	return &instruments.SkillOutput{Result: content, Success: true}, nil
}

func (s *FileOpsSkill) writeFile(path, content string) (*instruments.SkillOutput, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("written %d bytes to %s", len(content), path),
		Success: true,
	}, nil
}

func (s *FileOpsSkill) listFiles(dir, pattern string) (*instruments.SkillOutput, error) {
	if dir == "" {
		dir = s.baseDir
	}
	if pattern == "" {
		pattern = "*"
	}

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors.
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, rel)
		}
		if len(files) > 500 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Found %d files:\n%s", len(files), strings.Join(files, "\n")),
		Success: true,
	}, nil
}

func (s *FileOpsSkill) statFile(path string) (*instruments.SkillOutput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result: fmt.Sprintf("Name: %s\nSize: %d\nDir: %v\nModified: %s\nMode: %s",
			info.Name(), info.Size(), info.IsDir(), info.ModTime().Format("2006-01-02 15:04:05"), info.Mode()),
		Success: true,
	}, nil
}

func (s *FileOpsSkill) searchFiles(dir, query string) (*instruments.SkillOutput, error) {
	if dir == "" {
		dir = s.baseDir
	}
	var matches []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Size() > 1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), query) {
			rel, _ := filepath.Rel(dir, path)
			matches = append(matches, rel)
		}
		if len(matches) > 50 {
			return filepath.SkipAll
		}
		return nil
	})
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Found %d files containing %q:\n%s", len(matches), query, strings.Join(matches, "\n")),
		Success: true,
	}, nil
}

// --- Data Analysis Skill ---

// DataAnalysisSkill processes CSV/JSON data and computes statistics.
type DataAnalysisSkill struct{}

func NewDataAnalysisSkill() *DataAnalysisSkill {
	return &DataAnalysisSkill{}
}

func (s *DataAnalysisSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]
	data := input.Parameters["data"]
	if data == "" {
		data = input.Goal
	}

	switch action {
	case "csv_stats":
		return s.csvStats(data)
	case "json_query":
		return s.jsonQuery(data, input.Parameters["query"])
	case "statistics":
		return s.statistics(data)
	default:
		return s.statistics(data) // Default to statistics.
	}
}

func (s *DataAnalysisSkill) csvStats(data string) (*instruments.SkillOutput, error) {
	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: "CSV parse error: " + err.Error()}, nil
	}
	if len(records) == 0 {
		return &instruments.SkillOutput{Success: false, Error: "empty CSV"}, nil
	}

	result := fmt.Sprintf("Rows: %d\nColumns: %d\n", len(records), len(records[0]))
	if len(records) > 0 {
		result += "Headers: " + strings.Join(records[0], ", ") + "\n"
	}
	// Try to compute numeric stats for each column.
	for col := 0; col < len(records[0]); col++ {
		var vals []float64
		for row := 1; row < len(records); row++ {
			if col < len(records[row]) {
				var v float64
				if _, err := fmt.Sscanf(records[row][col], "%f", &v); err == nil {
					vals = append(vals, v)
				}
			}
		}
		if len(vals) > 0 {
			stats := computeStats(vals)
			result += fmt.Sprintf("\nColumn %q: count=%d, mean=%.2f, min=%.2f, max=%.2f, stddev=%.2f",
				records[0][col], stats.count, stats.mean, stats.min, stats.max, stats.stddev)
		}
	}

	return &instruments.SkillOutput{Result: result, Success: true}, nil
}

func (s *DataAnalysisSkill) jsonQuery(data, query string) (*instruments.SkillOutput, error) {
	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return &instruments.SkillOutput{Success: false, Error: "JSON parse error: " + err.Error()}, nil
	}
	pretty, _ := json.MarshalIndent(parsed, "", "  ")
	return &instruments.SkillOutput{
		Result:  string(pretty),
		Success: true,
	}, nil
}

func (s *DataAnalysisSkill) statistics(data string) (*instruments.SkillOutput, error) {
	// Parse numbers from input.
	var vals []float64
	for _, field := range strings.Fields(data) {
		var v float64
		if _, err := fmt.Sscanf(field, "%f", &v); err == nil {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return &instruments.SkillOutput{Success: false, Error: "no numeric values found"}, nil
	}

	stats := computeStats(vals)
	return &instruments.SkillOutput{
		Result: fmt.Sprintf("Count: %d\nMean: %.4f\nMedian: %.4f\nMin: %.4f\nMax: %.4f\nStddev: %.4f\nSum: %.4f",
			stats.count, stats.mean, stats.median, stats.min, stats.max, stats.stddev, stats.sum),
		Success: true,
	}, nil
}

type statsResult struct {
	count  int
	mean   float64
	median float64
	min    float64
	max    float64
	stddev float64
	sum    float64
}

func computeStats(vals []float64) statsResult {
	n := len(vals)
	if n == 0 {
		return statsResult{}
	}

	sorted := make([]float64, n)
	copy(sorted, vals)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(n)

	var median float64
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	if n > 1 {
		variance /= float64(n - 1)
	}

	return statsResult{
		count:  n,
		mean:   mean,
		median: median,
		min:    sorted[0],
		max:    sorted[n-1],
		stddev: math.Sqrt(variance),
		sum:    sum,
	}
}
