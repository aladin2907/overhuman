package main

import (
	"testing"

	"github.com/overhuman/overhuman/internal/genui"
	"github.com/overhuman/overhuman/internal/pipeline"
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

	_, _, _, err := bootstrap(cfg)
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

	deps, reflEngine, uiGen, err := bootstrap(cfg)
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
	if uiGen == nil {
		t.Error("uiGen should not be nil")
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

	deps, _, uiGen, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer deps.LongTerm.Close()

	if deps.LLM.Name() != "openai" {
		t.Errorf("LLM name = %q, want openai", deps.LLM.Name())
	}
	if uiGen == nil {
		t.Error("uiGen should not be nil")
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
	deps1, _, _, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	deps1.LongTerm.Close()

	// Second bootstrap should not fail (soul already exists).
	deps2, _, _, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	deps2.LongTerm.Close()
}

// TestBootstrap_CreatesUIGenerator verifies bootstrap returns non-nil UIGenerator.
func TestBootstrap_CreatesUIGenerator(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		DataDir:     dir,
		AgentName:   "TestAgent",
		DefaultSpec: "general",
		ClaudeKey:   "test-key",
	}

	deps, _, uiGen, err := bootstrap(cfg)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer deps.LongTerm.Close()

	if uiGen == nil {
		t.Fatal("UIGenerator should not be nil after bootstrap")
	}
}

// TestUIGenerator_CLICapabilities verifies CLICapabilities returns correct defaults.
func TestUIGenerator_CLICapabilities(t *testing.T) {
	caps := genui.CLICapabilities()
	if caps.Format != genui.FormatANSI {
		t.Errorf("Format = %q, want ansi", caps.Format)
	}
	if caps.JavaScript {
		t.Error("CLI should not have JavaScript")
	}
	if caps.Width <= 0 {
		t.Errorf("Width = %d, should be positive", caps.Width)
	}
}

// TestUIGenerator_ThoughtLogFromStageLogs converts pipeline.StageLog to genui.ThoughtStage.
func TestUIGenerator_ThoughtLogFromStageLogs(t *testing.T) {
	// Simulate what runCLI does: convert pipeline.StageLog -> genui.ThoughtStage
	stageLogs := []pipeline.StageLog{
		{Number: 1, Name: "intake", Summary: "task_id=abc", DurMs: 5},
		{Number: 2, Name: "clarify", DurMs: 120},
		{Number: 5, Name: "execute", Summary: "LLM call", DurMs: 800},
	}

	stages := make([]genui.ThoughtStage, len(stageLogs))
	for i, sl := range stageLogs {
		stages[i] = genui.ThoughtStage{
			Number:  sl.Number,
			Name:    sl.Name,
			Summary: sl.Summary,
			DurMs:   sl.DurMs,
		}
	}
	thought := genui.BuildThoughtLog(stages)

	if len(thought.Stages) != 3 {
		t.Fatalf("got %d stages, want 3", len(thought.Stages))
	}
	if thought.TotalMs != 925 {
		t.Errorf("TotalMs = %d, want 925", thought.TotalMs)
	}
	if thought.Stages[0].Name != "intake" {
		t.Errorf("first stage name = %q, want intake", thought.Stages[0].Name)
	}
}

// TestUIReflectionStore_BasicFlow tests recording an interaction and building hints.
func TestUIReflectionStore_BasicFlow(t *testing.T) {
	store := genui.NewReflectionStore()

	store.Record(genui.UIReflection{
		TaskID:       "t1",
		UIFormat:     genui.FormatANSI,
		ActionsShown: []string{"apply", "cancel"},
		ActionsUsed:  []string{"apply"},
		Scrolled:     true,
	})

	hints := store.BuildHints("fingerprint1")
	if len(hints) == 0 {
		t.Error("expected hints after recording interaction")
	}

	// Check that hints contain non-empty strings.
	found := false
	for _, h := range hints {
		if len(h) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("hints should contain non-empty strings")
	}
}
