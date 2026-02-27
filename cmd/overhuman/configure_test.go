package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// --- Model parsing tests ---

func TestParseOpenAIModels(t *testing.T) {
	body := []byte(`{
		"object": "list",
		"data": [
			{"id": "o4-mini", "object": "model", "owned_by": "openai"},
			{"id": "o3", "object": "model", "owned_by": "openai"},
			{"id": "text-embedding-3-small", "object": "model", "owned_by": "openai"},
			{"id": "whisper-1", "object": "model", "owned_by": "openai"},
			{"id": "tts-1", "object": "model", "owned_by": "openai"},
			{"id": "dall-e-3", "object": "model", "owned_by": "openai"},
			{"id": "gpt-4.1", "object": "model", "owned_by": "openai"}
		]
	}`)

	models := parseOpenAIModels(body, "openai")
	if len(models) != 3 {
		t.Fatalf("expected 3 chat models, got %d: %v", len(models), models)
	}

	// Should have filtered out embedding, whisper, tts, dall-e.
	ids := map[string]bool{}
	for _, m := range models {
		ids[m.id] = true
	}
	if ids["text-embedding-3-small"] || ids["whisper-1"] || ids["tts-1"] || ids["dall-e-3"] {
		t.Error("should have filtered non-chat models")
	}
	if !ids["o4-mini"] || !ids["o3"] || !ids["gpt-4.1"] {
		t.Error("missing expected chat models")
	}
}

func TestParseAnthropicModels(t *testing.T) {
	body := []byte(`{
		"data": [
			{"id": "claude-sonnet-4-6-20260217", "display_name": "Claude Sonnet 4.6", "type": "model"},
			{"id": "claude-opus-4-6-20260205", "display_name": "Claude Opus 4.6", "type": "model"}
		],
		"has_more": false
	}`)

	models := parseAnthropicModels(body)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].id != "claude-sonnet-4-6-20260217" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
	if models[0].desc != "Claude Sonnet 4.6" {
		t.Errorf("model[0].desc = %q", models[0].desc)
	}
}

func TestParseOllamaModels(t *testing.T) {
	body := []byte(`{
		"models": [
			{
				"name": "llama3.3:latest",
				"model": "llama3.3:latest",
				"details": {
					"parameter_size": "70B",
					"quantization_level": "Q4_K_M",
					"family": "llama"
				}
			},
			{
				"name": "deepseek-r1:latest",
				"details": {
					"parameter_size": "7.6B",
					"quantization_level": "Q4_K_M",
					"family": "qwen2"
				}
			}
		]
	}`)

	models := parseOllamaModels(body)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].id != "llama3.3:latest" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
	if models[0].desc != "70B Q4_K_M" {
		t.Errorf("model[0].desc = %q", models[0].desc)
	}
}

func TestParseLMStudioModels(t *testing.T) {
	body := []byte(`{
		"object": "list",
		"data": [
			{"id": "meta-llama-3.1-8b", "type": "llm", "arch": "llama", "quantization": "Q4_K_M"},
			{"id": "nomic-embed", "type": "embeddings", "arch": "nomic-bert", "quantization": "fp16"}
		]
	}`)

	models := parseLMStudioModels(body)
	if len(models) != 1 {
		t.Fatalf("expected 1 model (embeddings filtered), got %d", len(models))
	}
	if models[0].id != "meta-llama-3.1-8b" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
}

func TestParseTogetherModels(t *testing.T) {
	body := []byte(`[
		{"id": "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "display_name": "Llama 3.1 70B", "type": "chat"},
		{"id": "togethercomputer/m2-bert-80M-8k-retrieval", "display_name": "M2 BERT", "type": "embedding"},
		{"id": "black-forest-labs/FLUX.1-schnell", "display_name": "FLUX Schnell", "type": "image"}
	]`)

	models := parseTogetherModels(body)
	if len(models) != 1 {
		t.Fatalf("expected 1 chat model, got %d: %v", len(models), models)
	}
	if models[0].id != "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
}

func TestParseOpenRouterModels(t *testing.T) {
	body := []byte(`{
		"data": [
			{
				"id": "anthropic/claude-sonnet-4-6-20260217",
				"name": "Claude Sonnet 4.6",
				"architecture": {"output_modalities": ["text"]},
				"context_length": 200000
			},
			{
				"id": "openai/dall-e-3",
				"name": "DALL-E 3",
				"architecture": {"output_modalities": ["image"]},
				"context_length": 0
			}
		]
	}`)

	models := parseOpenRouterModels(body)
	if len(models) != 1 {
		t.Fatalf("expected 1 text model, got %d", len(models))
	}
	if models[0].id != "anthropic/claude-sonnet-4-6-20260217" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
	if !strings.Contains(models[0].desc, "200k") {
		t.Errorf("desc should contain context length, got %q", models[0].desc)
	}
}

func TestParseModelsResponse_InvalidJSON(t *testing.T) {
	// All parsers should return nil for invalid JSON.
	for _, provider := range []string{"openai", "claude", "ollama", "lmstudio", "together", "openrouter"} {
		models := parseModelsResponse(provider, []byte("not json{"))
		if models != nil {
			t.Errorf("%s: expected nil for invalid JSON, got %v", provider, models)
		}
	}
}

func TestFetchModelsFromAPI_InvalidProvider(t *testing.T) {
	models := fetchModelsFromAPI("nonexistent", "", "")
	if models != nil {
		t.Error("expected nil for unknown provider")
	}
}

func TestFetchModelsFromAPI_CustomNoBaseURL(t *testing.T) {
	models := fetchModelsFromAPI("custom", "", "")
	if models != nil {
		t.Error("expected nil for custom with no base URL")
	}
}

func TestParseGroqModels(t *testing.T) {
	body := []byte(`{
		"object": "list",
		"data": [
			{"id": "llama-3.3-70b-versatile", "object": "model", "owned_by": "Meta", "context_window": 131072},
			{"id": "whisper-large-v3", "object": "model", "owned_by": "OpenAI", "context_window": 0}
		]
	}`)

	models := parseOpenAIModels(body, "groq")
	if len(models) != 1 {
		t.Fatalf("expected 1 model (whisper filtered), got %d", len(models))
	}
	if models[0].id != "llama-3.3-70b-versatile" {
		t.Errorf("model[0].id = %q", models[0].id)
	}
	if !strings.Contains(models[0].desc, "131k") {
		t.Errorf("desc should contain context window, got %q", models[0].desc)
	}
}
