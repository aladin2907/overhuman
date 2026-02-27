package instruments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
)

// CodeSpec describes what a generated code-skill should do.
type CodeSpec struct {
	Goal        string   `json:"goal"`         // What the skill should accomplish
	InputDesc   string   `json:"input_desc"`   // Description of expected inputs
	OutputDesc  string   `json:"output_desc"`  // Description of expected outputs
	Examples    []string `json:"examples"`      // Example input→output pairs
	Language    string   `json:"language"`      // Target language ("python", "javascript", "bash", "go")
	Fingerprint string  `json:"fingerprint"`   // Pattern fingerprint this skill is for
}

// GeneratedCode holds the output of code generation.
type GeneratedCode struct {
	Code     string `json:"code"`
	Tests    string `json:"tests"`
	Language string `json:"language"`
}

// Generator creates code-skills from specifications using an LLM.
type Generator struct {
	llm    brain.LLMProvider
	router *brain.ModelRouter
	ctx    *brain.ContextAssembler
}

// NewGenerator creates a code-skill generator.
func NewGenerator(llm brain.LLMProvider, router *brain.ModelRouter, ca *brain.ContextAssembler) *Generator {
	return &Generator{
		llm:    llm,
		router: router,
		ctx:    ca,
	}
}

// Generate creates code and tests from a specification.
func (g *Generator) Generate(ctx context.Context, spec CodeSpec) (*GeneratedCode, float64, error) {
	lang := spec.Language
	if lang == "" {
		lang = "python" // Default to Python for widest compatibility.
	}

	prompt := fmt.Sprintf(`Generate a %s function that accomplishes this goal.

Goal: %s
Input: %s
Output: %s

Requirements:
- The function must be self-contained (no external dependencies beyond stdlib)
- Handle errors gracefully
- Return a clear result string

Respond in EXACTLY this format (no markdown fences):

CODE_START
<your function code here>
CODE_END

TESTS_START
<your test code here>
TESTS_END`, lang, spec.Goal, spec.InputDesc, spec.OutputDesc)

	if len(spec.Examples) > 0 {
		prompt += "\n\nExamples:\n" + strings.Join(spec.Examples, "\n")
	}

	messages := g.ctx.Assemble(brain.ContextLayers{
		SystemPrompt:    "You are a code generation expert. Write clean, tested, production-quality code.",
		TaskDescription: prompt,
	})

	// Use a strong model for code generation.
	model := g.router.Select("moderate", 100.0)
	resp, err := g.llm.Complete(ctx, brain.LLMRequest{
		Messages:  messages,
		Model:     model,
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("generate code: %w", err)
	}

	code := extractBlock(resp.Content, "CODE_START", "CODE_END")
	tests := extractBlock(resp.Content, "TESTS_START", "TESTS_END")

	if code == "" {
		return nil, resp.CostUSD, fmt.Errorf("generate: no code block found in response")
	}

	return &GeneratedCode{
		Code:     code,
		Tests:    tests,
		Language: lang,
	}, resp.CostUSD, nil
}

// GenerateAndRegister generates a code-skill and registers it in the registry.
func (g *Generator) GenerateAndRegister(
	ctx context.Context,
	spec CodeSpec,
	registry *SkillRegistry,
) (*Skill, float64, error) {
	generated, cost, err := g.Generate(ctx, spec)
	if err != nil {
		return nil, cost, err
	}

	// Create a code skill that wraps the generated code as a string result.
	// In Phase 3, this will actually compile/execute the code in a Docker sandbox.
	codeSkill := NewCodeSkill(
		func(ctx context.Context, input SkillInput) (*SkillOutput, error) {
			// Phase 2: return the generated code as a "skill template".
			// Phase 3: will execute in Docker container.
			return &SkillOutput{
				Result:  fmt.Sprintf("[Generated %s skill]\n%s", generated.Language, generated.Code),
				Success: true,
			}, nil
		},
		generated.Language,
		generated.Code,
	)

	now := time.Now()
	skill := &Skill{
		Meta: SkillMeta{
			ID:          fmt.Sprintf("skill_gen_%d", now.UnixNano()),
			Name:        fmt.Sprintf("Auto: %s", truncate(spec.Goal, 50)),
			Description: spec.Goal,
			Type:        SkillTypeCode,
			Status:      SkillStatusTrial,
			Fingerprint: spec.Fingerprint,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Executor: codeSkill,
	}

	registry.Register(skill)
	return skill, cost, nil
}

// extractBlock extracts text between start/end markers.
func extractBlock(text, startMarker, endMarker string) string {
	startIdx := strings.Index(text, startMarker)
	if startIdx < 0 {
		return ""
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(text[startIdx:], endMarker)
	if endIdx < 0 {
		return ""
	}

	return strings.TrimSpace(text[startIdx : startIdx+endIdx])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
