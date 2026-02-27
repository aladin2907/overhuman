package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistedConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	cfg := &persistedConfig{
		Provider: "openai",
		APIKey:   "sk-test-key-12345",
		Model:    "gpt-4o",
		Name:     "TestBot",
	}

	if err := savePersistedConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Check file exists with correct permissions.
	path := filepath.Join(dir, "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perms := info.Mode().Perm()
	if perms != 0o600 {
		t.Errorf("permissions = %o, want 600", perms)
	}

	// Load and verify.
	loaded, err := loadPersistedConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded config is nil")
	}
	if loaded.Provider != "openai" {
		t.Errorf("provider = %q", loaded.Provider)
	}
	if loaded.APIKey != "sk-test-key-12345" {
		t.Errorf("api_key = %q", loaded.APIKey)
	}
	if loaded.Model != "gpt-4o" {
		t.Errorf("model = %q", loaded.Model)
	}
	if loaded.Name != "TestBot" {
		t.Errorf("name = %q", loaded.Name)
	}
}

func TestPersistedConfig_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	cfg, err := loadPersistedConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil for missing config")
	}
}

func TestPersistedConfig_LoadInvalid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	// Write invalid JSON.
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("not json{"), 0o600)

	_, err := loadPersistedConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_FromConfigJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	// Clear env vars.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("OVERHUMAN_NAME", "")
	t.Setenv("OVERHUMAN_API_ADDR", "")

	// Write config.json.
	cfg := persistedConfig{
		Provider: "openai",
		APIKey:   "sk-from-config",
		Model:    "gpt-4o-mini",
		Name:     "ConfigBot",
		APIAddr:  "0.0.0.0:7070",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)

	// Load config — should pick up from config.json.
	loaded := loadConfig()

	if loaded.LLMProvider != "openai" {
		t.Errorf("provider = %q, want openai", loaded.LLMProvider)
	}
	if loaded.LLMAPIKey != "sk-from-config" {
		t.Errorf("api_key = %q, want sk-from-config", loaded.LLMAPIKey)
	}
	if loaded.OpenAIKey != "sk-from-config" {
		t.Errorf("openai_key = %q, want sk-from-config (backward compat)", loaded.OpenAIKey)
	}
	if loaded.LLMModel != "gpt-4o-mini" {
		t.Errorf("model = %q, want gpt-4o-mini", loaded.LLMModel)
	}
	if loaded.AgentName != "ConfigBot" {
		t.Errorf("name = %q, want ConfigBot", loaded.AgentName)
	}
	if loaded.APIAddr != "0.0.0.0:7070" {
		t.Errorf("api_addr = %q, want 0.0.0.0:7070", loaded.APIAddr)
	}
}

func TestLoadConfig_EnvOverridesConfigJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	// Write config.json with openai.
	cfg := persistedConfig{
		Provider: "openai",
		APIKey:   "sk-from-config",
		Model:    "gpt-4o",
		Name:     "ConfigBot",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)

	// Set env vars — these should override.
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-from-env")
	t.Setenv("LLM_MODEL", "claude-opus-4-20250514")
	t.Setenv("OVERHUMAN_NAME", "EnvBot")

	loaded := loadConfig()

	if loaded.LLMProvider != "claude" {
		t.Errorf("provider = %q, want claude (env override)", loaded.LLMProvider)
	}
	if loaded.ClaudeKey != "sk-ant-from-env" {
		t.Errorf("claude_key = %q, want sk-ant-from-env", loaded.ClaudeKey)
	}
	if loaded.LLMModel != "claude-opus-4-20250514" {
		t.Errorf("model = %q, want claude-opus-4-20250514", loaded.LLMModel)
	}
	if loaded.AgentName != "EnvBot" {
		t.Errorf("name = %q, want EnvBot", loaded.AgentName)
	}
}

func TestLoadConfig_ClaudeFromConfigJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	// Clear env vars.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("OVERHUMAN_NAME", "")
	t.Setenv("OVERHUMAN_API_ADDR", "")

	// Write config for Claude.
	cfg := persistedConfig{
		Provider: "claude",
		APIKey:   "sk-ant-from-config",
		Model:    "claude-sonnet-4-20250514",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)

	loaded := loadConfig()

	if loaded.LLMProvider != "claude" {
		t.Errorf("provider = %q, want claude", loaded.LLMProvider)
	}
	if loaded.ClaudeKey != "sk-ant-from-config" {
		t.Errorf("claude_key = %q, want sk-ant-from-config", loaded.ClaudeKey)
	}
	if loaded.LLMAPIKey != "sk-ant-from-config" {
		t.Errorf("llm_api_key = %q, want sk-ant-from-config", loaded.LLMAPIKey)
	}
}

func TestLoadConfig_OllamaFromConfigJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OVERHUMAN_DATA", dir)

	// Clear env vars.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_PROVIDER", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("OVERHUMAN_NAME", "")
	t.Setenv("OVERHUMAN_API_ADDR", "")

	// Write config for Ollama (no API key).
	cfg := persistedConfig{
		Provider: "ollama",
		Model:    "llama3.3",
		BaseURL:  "http://localhost:11434",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600)

	loaded := loadConfig()

	if loaded.LLMProvider != "ollama" {
		t.Errorf("provider = %q, want ollama", loaded.LLMProvider)
	}
	if loaded.LLMBaseURL != "http://localhost:11434" {
		t.Errorf("base_url = %q", loaded.LLMBaseURL)
	}
	if loaded.LLMModel != "llama3.3" {
		t.Errorf("model = %q, want llama3.3", loaded.LLMModel)
	}
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("OVERHUMAN_DATA", "/tmp/test-overhuman")
	path := configFilePath()
	if path != "/tmp/test-overhuman/config.json" {
		t.Errorf("path = %q, want /tmp/test-overhuman/config.json", path)
	}
}

func TestTestProviderConnection_InvalidURL(t *testing.T) {
	cfg := &persistedConfig{
		Provider: "custom",
		BaseURL:  "", // Empty = error.
	}
	err := testProviderConnection(cfg)
	if err == nil {
		t.Error("expected error for custom with no base URL")
	}
}
