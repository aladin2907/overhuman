package reflection

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/overhuman/overhuman/internal/brain"
)

// StepName identifies a pipeline stage for micro-reflection.
type StepName string

const (
	StepClarify  StepName = "clarify"
	StepPlan     StepName = "plan"
	StepExecute  StepName = "execute"
	StepReview   StepName = "review"
)

// StepResult captures the outcome of one pipeline step.
type StepResult struct {
	Step     StepName
	Input    string  // What was fed into the step
	Output   string  // What the step produced
	CostUSD  float64
	ElapsedMs int64
}

// MicroVerdict is the outcome of a micro-reflection check.
type MicroVerdict struct {
	Step       StepName `json:"step"`
	OK         bool     `json:"ok"`          // True = step succeeded, proceed
	Confidence float64  `json:"confidence"`  // 0-1 confidence in verdict
	Issue      string   `json:"issue"`       // Non-empty if problem detected
	Suggestion string   `json:"suggestion"`  // How to fix / what to retry
}

// MicroReflector performs lightweight per-step quality checks.
// Only triggers on critical steps (clarify, execute, review) and uses
// the cheapest model possible to minimize cost overhead.
type MicroReflector struct {
	llm    brain.LLMProvider
	router *brain.ModelRouter
	ctx    *brain.ContextAssembler

	mu       sync.RWMutex
	enabled  map[StepName]bool // Which steps to check
	minQuality float64        // Skip micro-reflection if overall quality above this
}

// NewMicroReflector creates a micro-reflector.
func NewMicroReflector(
	llm brain.LLMProvider,
	router *brain.ModelRouter,
	ca *brain.ContextAssembler,
) *MicroReflector {
	return &MicroReflector{
		llm:    llm,
		router: router,
		ctx:    ca,
		enabled: map[StepName]bool{
			StepClarify: true,
			StepExecute: true,
			StepReview:  true,
		},
		minQuality: 0.95, // Don't micro-reflect if quality is very high
	}
}

// SetEnabled controls which steps get micro-reflection.
func (m *MicroReflector) SetEnabled(step StepName, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled[step] = on
}

// SetMinQuality sets the quality threshold above which micro-reflection is skipped.
func (m *MicroReflector) SetMinQuality(q float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.minQuality = q
}

// IsEnabled returns whether micro-reflection is active for a step.
func (m *MicroReflector) IsEnabled(step StepName) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled[step]
}

// Check evaluates a pipeline step result and returns a verdict.
// Returns (verdict, cost, error). If the step is not enabled, returns a passing verdict at zero cost.
func (m *MicroReflector) Check(ctx context.Context, goal string, result StepResult) (*MicroVerdict, float64, error) {
	m.mu.RLock()
	enabled := m.enabled[result.Step]
	m.mu.RUnlock()

	if !enabled {
		return &MicroVerdict{Step: result.Step, OK: true, Confidence: 1.0}, 0, nil
	}

	prompt := fmt.Sprintf(`Quickly evaluate this pipeline step result.

Step: %s
Task goal: %s
Step output (first 500 chars): %.500s

Is this step output adequate for the task goal? Respond in EXACTLY this format:
OK: YES or NO
CONFIDENCE: <0.0-1.0>
ISSUE: <brief issue description, or NONE>
SUGGESTION: <what to fix or retry, or NONE>`, result.Step, goal, result.Output)

	messages := m.ctx.Assemble(brain.ContextLayers{
		SystemPrompt:    "You are a quality checker. Be concise and fast.",
		TaskDescription: prompt,
	})

	model := m.router.Select("simple", 100.0)
	resp, err := m.llm.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 128,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("micro reflection (%s): %w", result.Step, err)
	}

	verdict := parseMicroResponse(result.Step, resp.Content)
	return verdict, resp.CostUSD, nil
}

// parseMicroResponse extracts a MicroVerdict from LLM text.
func parseMicroResponse(step StepName, text string) *MicroVerdict {
	v := &MicroVerdict{Step: step, OK: true, Confidence: 0.5}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "OK:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "OK:"))
			v.OK = strings.EqualFold(val, "YES")
		case strings.HasPrefix(line, "CONFIDENCE:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			fmt.Sscanf(val, "%f", &v.Confidence)
		case strings.HasPrefix(line, "ISSUE:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "ISSUE:"))
			if val != "" && val != "NONE" && val != "none" {
				v.Issue = val
			}
		case strings.HasPrefix(line, "SUGGESTION:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "SUGGESTION:"))
			if val != "" && val != "NONE" && val != "none" {
				v.Suggestion = val
			}
		}
	}

	return v
}
