package main

import (
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Point to empty temp dir so real ~/.overhuman/config.json is not read.
	t.Setenv("OVERHUMAN_DATA", t.TempDir())
	t.Setenv("OVERHUMAN_API_ADDR", "")
	t.Setenv("OVERHUMAN_NAME", "")

	cfg := loadConfig()

	if cfg.APIAddr != "127.0.0.1:9090" {
		t.Errorf("APIAddr = %q, want 127.0.0.1:9090", cfg.APIAddr)
	}
	if cfg.AgentName != "Overhuman" {
		t.Errorf("AgentName = %q, want Overhuman", cfg.AgentName)
	}
	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if cfg.DefaultSpec != "general" {
		t.Errorf("DefaultSpec = %q, want general", cfg.DefaultSpec)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("OVERHUMAN_DATA", "/tmp/test-overhuman")
	t.Setenv("OVERHUMAN_API_ADDR", "0.0.0.0:8888")
	t.Setenv("OVERHUMAN_NAME", "TestBot")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-123")

	cfg := loadConfig()

	if cfg.DataDir != "/tmp/test-overhuman" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.APIAddr != "0.0.0.0:8888" {
		t.Errorf("APIAddr = %q", cfg.APIAddr)
	}
	if cfg.AgentName != "TestBot" {
		t.Errorf("AgentName = %q", cfg.AgentName)
	}
	if cfg.ClaudeKey != "sk-test-123" {
		t.Errorf("ClaudeKey = %q", cfg.ClaudeKey)
	}
}

func TestBootstrap_NoAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	dir := t.TempDir()
	cfg := Config{
		DataDir:     dir,
		AgentName:   "TestAgent",
		DefaultSpec: "general",
	}

	_, _, err := bootstrap(cfg)
	if err == nil {
		t.Fatal("expected error when no API key is set")
	}
}

func TestBootstrap_WithClaudeKey(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		DataDir:     dir,
		AgentName:   "TestAgent",
		DefaultSpec: "general",
		ClaudeKey:   "test-key",
	}

	deps, reflEngine, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer deps.LongTerm.Close()

	if deps.Soul == nil {
		t.Error("Soul should not be nil")
	}
	if deps.LLM == nil {
		t.Error("LLM should not be nil")
	}
	if deps.Router == nil {
		t.Error("Router should not be nil")
	}
	if deps.Context == nil {
		t.Error("Context should not be nil")
	}
	if deps.ShortTerm == nil {
		t.Error("ShortTerm should not be nil")
	}
	if deps.LongTerm == nil {
		t.Error("LongTerm should not be nil")
	}
	if deps.Patterns == nil {
		t.Error("Patterns should not be nil")
	}
	if deps.AutoThreshold != 3 {
		t.Errorf("AutoThreshold = %d, want 3", deps.AutoThreshold)
	}
	if reflEngine == nil {
		t.Error("reflEngine should not be nil")
	}
	if deps.Reflection == nil {
		t.Error("deps.Reflection should not be nil (must be wired into pipeline)")
	}
	if deps.Reflection != reflEngine {
		t.Error("deps.Reflection should be the same instance as reflEngine")
	}
}

func TestBootstrap_WithOpenAIKey(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		DataDir:     dir,
		AgentName:   "TestAgent",
		DefaultSpec: "general",
		OpenAIKey:   "test-openai-key",
	}

	deps, _, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer deps.LongTerm.Close()

	if deps.LLM.Name() != "openai" {
		t.Errorf("LLM name = %q, want openai", deps.LLM.Name())
	}
}

func TestBootstrap_SoulReinitialization(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		DataDir:     dir,
		AgentName:   "TestAgent",
		DefaultSpec: "general",
		ClaudeKey:   "test-key",
	}

	// First bootstrap creates the soul.
	deps1, _, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	deps1.LongTerm.Close()

	// Second bootstrap should not fail (soul already exists).
	deps2, _, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	deps2.LongTerm.Close()
}
