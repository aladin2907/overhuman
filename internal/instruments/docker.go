// Package instruments — Docker sandbox for code-skill execution.
//
// Manages container lifecycle for running untrusted generated code safely:
//   - Resource limits (CPU, memory, timeout)
//   - Network isolation (no outbound by default)
//   - Volume mounting for input/output
//   - Automatic cleanup after execution
package instruments

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SandboxConfig controls container resource limits.
type SandboxConfig struct {
	Image        string        // Docker image (default: "overhuman-skill-base")
	MemoryMB     int           // Memory limit in MB (default: 256)
	CPUs         float64       // CPU limit (default: 0.5)
	Timeout      time.Duration // Execution timeout (default: 30s)
	NetworkMode  string        // "none" (default), "bridge", "host"
	WorkDir      string        // Working directory inside container
}

// DefaultSandboxConfig returns safe defaults.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Image:       "overhuman-skill-base",
		MemoryMB:    256,
		CPUs:        0.5,
		Timeout:     30 * time.Second,
		NetworkMode: "none",
		WorkDir:     "/workspace",
	}
}

// SandboxResult captures the output of a container execution.
type SandboxResult struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ElapsedMs int64  `json:"elapsed_ms"`
	OOMKilled bool   `json:"oom_killed"` // Out of memory
	TimedOut  bool   `json:"timed_out"`
}

// DockerSandbox manages container-based code execution.
type DockerSandbox struct {
	mu     sync.RWMutex
	config SandboxConfig

	// Stats.
	totalRuns   int
	totalErrors int
}

// NewDockerSandbox creates a sandbox manager.
func NewDockerSandbox(config SandboxConfig) *DockerSandbox {
	return &DockerSandbox{config: config}
}

// SetConfig updates the sandbox configuration.
func (d *DockerSandbox) SetConfig(config SandboxConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
}

// Config returns the current sandbox configuration.
func (d *DockerSandbox) Config() SandboxConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// IsAvailable checks if Docker is installed and accessible.
func (d *DockerSandbox) IsAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// Execute runs code in a Docker container.
// The code string is passed via stdin to the container's interpreter.
func (d *DockerSandbox) Execute(ctx context.Context, language, code string) (*SandboxResult, error) {
	d.mu.RLock()
	cfg := d.config
	d.mu.RUnlock()

	interpreter, err := languageInterpreter(language)
	if err != nil {
		return nil, err
	}

	// Build docker run command.
	args := []string{
		"run", "--rm",
		"--memory", fmt.Sprintf("%dm", cfg.MemoryMB),
		"--cpus", fmt.Sprintf("%.1f", cfg.CPUs),
		"--network", cfg.NetworkMode,
		"--workdir", cfg.WorkDir,
		// Security: drop all capabilities, read-only root.
		"--cap-drop=ALL",
		"--read-only",
		// Tmpfs for /tmp (skills may need temp files).
		"--tmpfs", "/tmp:size=64m",
		cfg.Image,
		interpreter[0],
	}
	args = append(args, interpreter[1:]...)

	// Create context with timeout.
	execCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", args...)
	cmd.Stdin = strings.NewReader(code)

	start := time.Now()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	d.mu.Lock()
	d.totalRuns++
	if runErr != nil {
		d.totalErrors++
	}
	d.mu.Unlock()

	result := &SandboxResult{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		ElapsedMs: elapsed,
	}

	if execCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, nil
	}

	if exitErr, ok := runErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		// Check for OOM (exit code 137 = SIGKILL, often OOM).
		if result.ExitCode == 137 {
			result.OOMKilled = true
		}
		return result, nil
	}

	if runErr != nil {
		return nil, fmt.Errorf("docker run: %w", runErr)
	}

	result.ExitCode = 0
	return result, nil
}

// Stats returns execution statistics.
func (d *DockerSandbox) Stats() (runs, errors int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.totalRuns, d.totalErrors
}

// CreateSkillExecutor wraps a DockerSandbox as a SkillExecutor for a specific code string.
func (d *DockerSandbox) CreateSkillExecutor(language, code string) SkillExecutor {
	return &dockerSkillExecutor{
		sandbox:  d,
		language: language,
		code:     code,
	}
}

type dockerSkillExecutor struct {
	sandbox  *DockerSandbox
	language string
	code     string
}

func (e *dockerSkillExecutor) Execute(ctx context.Context, input SkillInput) (*SkillOutput, error) {
	// Inject input parameters into the code environment via stdin.
	fullCode := fmt.Sprintf("# Input: goal=%s\n%s", input.Goal, e.code)

	result, err := e.sandbox.Execute(ctx, e.language, fullCode)
	if err != nil {
		return &SkillOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	if result.TimedOut {
		return &SkillOutput{
			Success:   false,
			Error:     "execution timed out",
			ElapsedMs: result.ElapsedMs,
		}, nil
	}

	if result.OOMKilled {
		return &SkillOutput{
			Success:   false,
			Error:     "out of memory",
			ElapsedMs: result.ElapsedMs,
		}, nil
	}

	if result.ExitCode != 0 {
		return &SkillOutput{
			Success:   false,
			Error:     fmt.Sprintf("exit code %d: %s", result.ExitCode, result.Stderr),
			ElapsedMs: result.ElapsedMs,
		}, nil
	}

	return &SkillOutput{
		Result:    result.Stdout,
		Success:   true,
		CostUSD:   0, // Code execution is free.
		ElapsedMs: result.ElapsedMs,
	}, nil
}

// languageInterpreter returns the command to run code for a given language.
func languageInterpreter(lang string) ([]string, error) {
	switch strings.ToLower(lang) {
	case "python", "py":
		return []string{"python3", "-c", "/dev/stdin"}, nil
	case "javascript", "js", "node":
		return []string{"node", "-e", "/dev/stdin"}, nil
	case "bash", "sh":
		return []string{"bash", "-s"}, nil
	case "go":
		return []string{"sh", "-c", "cat > /tmp/main.go && go run /tmp/main.go"}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}
