package instruments

import (
	"context"
	"testing"
	"time"
)

func TestDefaultSandboxConfig(t *testing.T) {
	cfg := DefaultSandboxConfig()
	if cfg.Image != "overhuman-skill-base" {
		t.Errorf("Image = %q", cfg.Image)
	}
	if cfg.MemoryMB != 256 {
		t.Errorf("MemoryMB = %d", cfg.MemoryMB)
	}
	if cfg.CPUs != 0.5 {
		t.Errorf("CPUs = %f", cfg.CPUs)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if cfg.NetworkMode != "none" {
		t.Errorf("NetworkMode = %q", cfg.NetworkMode)
	}
	if cfg.WorkDir != "/workspace" {
		t.Errorf("WorkDir = %q", cfg.WorkDir)
	}
}

func TestNewDockerSandbox(t *testing.T) {
	cfg := DefaultSandboxConfig()
	sb := NewDockerSandbox(cfg)
	if sb == nil {
		t.Fatal("NewDockerSandbox returned nil")
	}

	got := sb.Config()
	if got.Image != cfg.Image {
		t.Errorf("Config().Image = %q", got.Image)
	}
}

func TestDockerSandbox_SetConfig(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())

	newCfg := SandboxConfig{
		Image:       "custom-image",
		MemoryMB:    512,
		CPUs:        1.0,
		Timeout:     60 * time.Second,
		NetworkMode: "bridge",
		WorkDir:     "/app",
	}
	sb.SetConfig(newCfg)

	got := sb.Config()
	if got.Image != "custom-image" {
		t.Errorf("Image = %q", got.Image)
	}
	if got.MemoryMB != 512 {
		t.Errorf("MemoryMB = %d", got.MemoryMB)
	}
	if got.CPUs != 1.0 {
		t.Errorf("CPUs = %f", got.CPUs)
	}
	if got.NetworkMode != "bridge" {
		t.Errorf("NetworkMode = %q", got.NetworkMode)
	}
}

func TestDockerSandbox_Stats(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())
	runs, errs := sb.Stats()
	if runs != 0 || errs != 0 {
		t.Errorf("Stats = (%d, %d), want (0, 0)", runs, errs)
	}
}

func TestLanguageInterpreter(t *testing.T) {
	tests := []struct {
		lang    string
		wantCmd string
		wantErr bool
	}{
		{"python", "python3", false},
		{"py", "python3", false},
		{"Python", "python3", false},
		{"javascript", "node", false},
		{"js", "node", false},
		{"node", "node", false},
		{"bash", "bash", false},
		{"sh", "bash", false},
		{"go", "sh", false},
		{"Go", "sh", false},
		{"ruby", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			cmd, err := languageInterpreter(tt.lang)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd[0] != tt.wantCmd {
				t.Errorf("cmd[0] = %q, want %q", cmd[0], tt.wantCmd)
			}
		})
	}
}

func TestDockerSandbox_Execute_UnsupportedLanguage(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())
	_, err := sb.Execute(context.Background(), "ruby", "puts 'hello'")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestDockerSandbox_CreateSkillExecutor(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())
	executor := sb.CreateSkillExecutor("python", "print('hello')")
	if executor == nil {
		t.Fatal("CreateSkillExecutor returned nil")
	}

	// Verify it's a dockerSkillExecutor.
	dse, ok := executor.(*dockerSkillExecutor)
	if !ok {
		t.Fatal("expected *dockerSkillExecutor")
	}
	if dse.language != "python" {
		t.Errorf("language = %q", dse.language)
	}
	if dse.code != "print('hello')" {
		t.Errorf("code mismatch")
	}
}

func TestSandboxResult_Fields(t *testing.T) {
	r := &SandboxResult{
		ExitCode:  0,
		Stdout:    "hello",
		Stderr:    "",
		ElapsedMs: 100,
		OOMKilled: false,
		TimedOut:  false,
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d", r.ExitCode)
	}
	if r.Stdout != "hello" {
		t.Errorf("Stdout = %q", r.Stdout)
	}
}

// TestDockerSandbox_Execute_NoDocker verifies that Execute fails gracefully
// when Docker is not available (common in CI/test environments).
func TestDockerSandbox_Execute_NoDocker(t *testing.T) {
	sb := NewDockerSandbox(SandboxConfig{
		Image:       "nonexistent-image-xyz",
		MemoryMB:    64,
		CPUs:        0.1,
		Timeout:     2 * time.Second,
		NetworkMode: "none",
		WorkDir:     "/workspace",
	})

	// If Docker is not available, this should return an error.
	// If Docker IS available, it will fail on the nonexistent image.
	// Either way, we shouldn't panic.
	result, err := sb.Execute(context.Background(), "python", "print('test')")

	// We accept both outcomes: either an error (Docker not available)
	// or a result with non-zero exit code (Docker available but image missing).
	if err != nil {
		// Docker not available or command failed — that's fine.
		t.Logf("Execute returned error (expected in CI): %v", err)
		return
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code with nonexistent image")
	}
	t.Logf("Execute returned exit code %d (Docker available but image missing)", result.ExitCode)
}

// TestDockerSandbox_IsAvailable just runs the check — it may return true or false
// depending on the environment.
func TestDockerSandbox_IsAvailable(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())
	available := sb.IsAvailable()
	t.Logf("Docker available: %v", available)
	// No assertion — depends on environment.
}

func TestDockerSkillExecutor_Execute_UnsupportedLanguage(t *testing.T) {
	sb := NewDockerSandbox(DefaultSandboxConfig())
	executor := sb.CreateSkillExecutor("ruby", "puts 'hi'")

	output, err := executor.Execute(context.Background(), SkillInput{Goal: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if output == nil {
		t.Fatal("output should not be nil even on error")
	}
	if output.Success {
		t.Error("output.Success should be false")
	}
}
