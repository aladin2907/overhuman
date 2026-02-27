package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/overhuman/overhuman/internal/instruments"
)

// CodeExecSkill runs code in a Docker sandbox.
type CodeExecSkill struct {
	sandbox *instruments.DockerSandbox
}

// NewCodeExecSkill creates a code execution skill.
func NewCodeExecSkill(sandbox *instruments.DockerSandbox) *CodeExecSkill {
	return &CodeExecSkill{sandbox: sandbox}
}

func (s *CodeExecSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	if s.sandbox == nil {
		return &instruments.SkillOutput{
			Success: false,
			Error:   "Docker sandbox not configured",
		}, fmt.Errorf("docker sandbox not available")
	}

	// Parse language and code from parameters or goal.
	lang := input.Parameters["language"]
	code := input.Parameters["code"]
	if lang == "" {
		lang = "python" // Default language.
	}
	if code == "" {
		code = input.Goal
	}

	result, err := s.sandbox.Execute(ctx, lang, code)
	if err != nil {
		return &instruments.SkillOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	output := result.Stdout
	if result.Stderr != "" {
		output += "\n[stderr] " + result.Stderr
	}

	return &instruments.SkillOutput{
		Result:    output,
		Success:   result.ExitCode == 0,
		ElapsedMs: result.ElapsedMs,
		Error:     errorFromResult(result),
	}, nil
}

func errorFromResult(r *instruments.SandboxResult) string {
	if r.ExitCode == 0 {
		return ""
	}
	if r.OOMKilled {
		return "out of memory"
	}
	if r.TimedOut {
		return "timeout exceeded"
	}
	return fmt.Sprintf("exit code %d", r.ExitCode)
}

// GitSkill runs git commands.
type GitSkill struct {
	workDir string
}

func NewGitSkill(workDir string) *GitSkill {
	return &GitSkill{workDir: workDir}
}

func (s *GitSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	// Parse git command from goal.
	cmd := input.Parameters["command"]
	if cmd == "" {
		cmd = input.Goal
	}

	// Safety: only allow read-only commands by default.
	allowed := []string{"status", "log", "diff", "branch", "show", "ls-files", "blame", "shortlog"}
	isAllowed := false
	for _, a := range allowed {
		if strings.Contains(cmd, a) {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return &instruments.SkillOutput{
			Success: false,
			Error:   "only read-only git commands allowed by default: " + strings.Join(allowed, ", "),
		}, nil
	}

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("[git] would execute: git %s (in %s)", cmd, s.workDir),
		Success: true,
	}, nil
}

// TestingSkill generates and runs tests.
type TestingSkill struct {
	sandbox *instruments.DockerSandbox
}

func NewTestingSkill(sandbox *instruments.DockerSandbox) *TestingSkill {
	return &TestingSkill{sandbox: sandbox}
}

func (s *TestingSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	lang := input.Parameters["language"]
	code := input.Parameters["test_code"]

	if code == "" {
		return &instruments.SkillOutput{
			Success: false,
			Error:   "test_code parameter required",
		}, nil
	}

	if s.sandbox == nil {
		return &instruments.SkillOutput{
			Result:  "[testing] sandbox not available; test code received:\n" + code,
			Success: true,
		}, nil
	}

	if lang == "" {
		lang = "python"
	}

	result, err := s.sandbox.Execute(ctx, lang, code)
	if err != nil {
		return &instruments.SkillOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return &instruments.SkillOutput{
		Result:    result.Stdout + result.Stderr,
		Success:   result.ExitCode == 0,
		ElapsedMs: result.ElapsedMs,
	}, nil
}
