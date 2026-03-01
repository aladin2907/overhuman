package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

// persistedConfig is the JSON structure stored in ~/.overhuman/config.json.
type persistedConfig struct {
	Provider string `json:"provider,omitempty"` // "openai", "claude", "ollama", etc.
	APIKey   string `json:"api_key,omitempty"`  // API key (stored with 0600 permissions)
	Model    string `json:"model,omitempty"`    // Model override
	BaseURL  string `json:"base_url,omitempty"` // Custom base URL
	Name     string `json:"name,omitempty"`     // Agent name
	APIAddr  string `json:"api_addr,omitempty"` // API listen address
}

// configFilePath returns the path to config.json.
func configFilePath() string {
	dataDir := os.Getenv("OVERHUMAN_DATA")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataDir = filepath.Join(home, ".overhuman")
	}
	return filepath.Join(dataDir, "config.json")
}

// loadPersistedConfig reads config.json if it exists.
func loadPersistedConfig() (*persistedConfig, error) {
	path := configFilePath()
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg persistedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// savePersistedConfig writes config.json with 0600 permissions.
func savePersistedConfig(cfg *persistedConfig) error {
	path := configFilePath()
	if path == "" {
		return fmt.Errorf("cannot determine config path")
	}

	// Ensure directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Write with restricted permissions (only owner can read/write).
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// runConfigure runs the interactive configuration wizard.
func runConfigure() {
	fmt.Printf("\n🔧 %s v%s — Configuration Wizard\n\n", appName, version)

	reader := bufio.NewReader(os.Stdin)

	// Load existing config if any.
	existing, _ := loadPersistedConfig()
	if existing == nil {
		existing = &persistedConfig{}
	}

	// Step 1: Choose provider.
	fmt.Println("Select your LLM provider (↑↓ to move, Enter to select):")
	fmt.Println()

	type providerEntry struct {
		key  string
		name string
		desc string
	}
	providers := []providerEntry{
		{"openai", "OpenAI", "Requires API key"},
		{"claude", "Anthropic Claude", "Requires API key"},
		{"ollama", "Ollama", "Local models, free, no API key"},
		{"lmstudio", "LM Studio", "Local models via GUI, free"},
		{"groq", "Groq", "Fast cloud inference, requires API key"},
		{"together", "Together AI", "Open-source models hosted, requires API key"},
		{"openrouter", "OpenRouter", "Multi-provider gateway, requires API key"},
		{"custom", "Custom endpoint", "Any OpenAI-compatible API"},
	}

	providerItems := make([]selectItem, len(providers))
	defaultProviderIdx := 0
	for i, p := range providers {
		providerItems[i] = selectItem{label: p.name, desc: p.desc}
		if existing.Provider == p.key {
			defaultProviderIdx = i
		}
	}

	providerIdx := interactiveSelect(providerItems, defaultProviderIdx)
	if providerIdx < 0 {
		fmt.Println("  Cancelled.")
		return
	}
	selectedProvider := providers[providerIdx]
	fmt.Printf("  ✓ %s\n\n", selectedProvider.name)

	cfg := &persistedConfig{
		Provider: selectedProvider.key,
	}

	// Step 2: API key (if needed).
	needsKey := selectedProvider.key != "ollama" && selectedProvider.key != "lmstudio"
	if needsKey {
		existingKey := existing.APIKey
		masked := ""
		if existingKey != "" {
			if len(existingKey) > 8 {
				masked = existingKey[:4] + "..." + existingKey[len(existingKey)-4:]
			} else {
				masked = "****"
			}
		}

		if masked != "" {
			fmt.Printf("  Current API key: %s\n", masked)
			fmt.Print("  Enter new API key (or press Enter to keep current): ")
		} else {
			fmt.Print("  Enter your API key: ")
		}

		key := readSecretLine(reader)
		if key == "" && existingKey != "" {
			key = existingKey
			fmt.Println("  ✓ Keeping existing key")
		} else if key != "" {
			fmt.Println("  ✓ API key saved")
		} else {
			fmt.Println("  ⚠ No API key provided. You can set it later.")
		}
		cfg.APIKey = key
		fmt.Println()
	}

	// Step 3: Base URL (for ollama, lmstudio, custom).
	needsURL := selectedProvider.key == "ollama" || selectedProvider.key == "lmstudio" || selectedProvider.key == "custom"
	if needsURL {
		defaultURL := ""
		switch selectedProvider.key {
		case "ollama":
			defaultURL = "http://localhost:11434"
		case "lmstudio":
			defaultURL = "http://localhost:1234"
		}
		if existing.BaseURL != "" {
			defaultURL = existing.BaseURL
		}

		url := promptString(reader, "Base URL", defaultURL)
		cfg.BaseURL = url
		fmt.Printf("  ✓ Base URL: %s\n\n", url)
	}

	// Step 4: Model selection.
	// Fetch models dynamically from the provider API.
	fmt.Print("  Connecting to provider... ")
	models := fetchModelsFromAPI(selectedProvider.key, cfg.APIKey, cfg.BaseURL)
	if len(models) > 0 {
		fmt.Printf("OK, %d models available\n\n", len(models))
		fmt.Println("Select default model (↑↓ to move, Enter to select):")
		fmt.Println()

		// Build select items. Last item = "Other (type manually)".
		modelItems := make([]selectItem, len(models)+1)
		defaultModelIdx := 0
		for i, m := range models {
			modelItems[i] = selectItem{label: m.id, desc: m.desc}
			if existing.Model == m.id {
				defaultModelIdx = i
			}
		}
		modelItems[len(models)] = selectItem{label: "Other...", desc: "enter model name manually"}

		modelIdx := interactiveSelect(modelItems, defaultModelIdx)
		if modelIdx < 0 {
			fmt.Println("  Cancelled.")
			return
		}

		if modelIdx == len(models) {
			// "Other" — free input.
			model := promptString(reader, "Model name", "")
			cfg.Model = model
		} else {
			cfg.Model = models[modelIdx].id
		}
		fmt.Printf("  ✓ Model: %s\n\n", cfg.Model)
	} else {
		fmt.Println("could not reach provider")
		fmt.Println("  Check your API key and network connection.")
		fmt.Println()
		defaultModel := ""
		if existing.Model != "" {
			defaultModel = existing.Model
		}
		model := promptString(reader, "Model name", defaultModel)
		cfg.Model = model
		fmt.Printf("  ✓ Model: %s\n\n", model)
	}

	// Step 5: Agent name.
	defaultName := "Overhuman"
	if existing.Name != "" {
		defaultName = existing.Name
	}
	name := promptString(reader, "Agent name", defaultName)
	cfg.Name = name
	fmt.Printf("  ✓ Agent name: %s\n\n", name)

	// Save.
	if err := savePersistedConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	path := configFilePath()
	fmt.Printf("  ✓ Configuration saved to %s\n\n", path)

	// Validate connection.
	fmt.Print("  Testing connection... ")
	if err := testProviderConnection(cfg); err != nil {
		fmt.Printf("⚠ %v\n", err)
		fmt.Println("  You can fix this later and re-run: overhuman configure")
	} else {
		fmt.Println("✓ Connected!")
	}

	fmt.Printf("\n  Ready! Run: %s cli\n\n", appName)
}

// testProviderConnection attempts a basic health check against the provider.
func testProviderConnection(cfg *persistedConfig) error {
	var url string
	switch cfg.Provider {
	case "openai":
		url = "https://api.openai.com/v1/models"
	case "claude", "anthropic":
		// Claude doesn't have a simple ping endpoint, try models list.
		url = "https://api.anthropic.com/v1/models"
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		url = baseURL + "/api/tags"
	case "lmstudio":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:1234"
		}
		url = baseURL + "/v1/models"
	case "groq":
		url = "https://api.groq.com/openai/v1/models"
	case "together":
		url = "https://api.together.xyz/v1/models"
	case "openrouter":
		url = "https://openrouter.ai/api/v1/models"
	case "custom":
		if cfg.BaseURL == "" {
			return fmt.Errorf("no base URL configured")
		}
		url = strings.TrimRight(cfg.BaseURL, "/") + "/v1/models"
	default:
		return fmt.Errorf("unknown provider")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Add auth.
	if cfg.APIKey != "" {
		switch cfg.Provider {
		case "claude", "anthropic":
			req.Header.Set("x-api-key", cfg.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		default:
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — check your API key", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// runDoctor checks the configuration for issues.
func runDoctor() {
	fmt.Printf("\n🔍 %s v%s — Doctor\n\n", appName, version)

	issues := 0
	checks := 0

	// Check 1: Data directory.
	checks++
	dataDir := os.Getenv("OVERHUMAN_DATA")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".overhuman")
	}
	if info, err := os.Stat(dataDir); err != nil {
		fmt.Printf("  ✗ Data directory: %s (does not exist)\n", dataDir)
		issues++
	} else if !info.IsDir() {
		fmt.Printf("  ✗ Data directory: %s (not a directory)\n", dataDir)
		issues++
	} else {
		fmt.Printf("  ✓ Data directory: %s\n", dataDir)
	}

	// Check 2: Config file.
	checks++
	cfgPath := configFilePath()
	cfg, err := loadPersistedConfig()
	if err != nil {
		fmt.Printf("  ✗ Config file: %s (%v)\n", cfgPath, err)
		issues++
	} else if cfg == nil {
		fmt.Printf("  ✗ Config file: not found — run: overhuman configure\n")
		issues++
	} else {
		// Check permissions.
		info, _ := os.Stat(cfgPath)
		perms := info.Mode().Perm()
		if perms&0o077 != 0 {
			fmt.Printf("  ⚠ Config file: %s (permissions %o — should be 600)\n", cfgPath, perms)
			issues++
		} else {
			fmt.Printf("  ✓ Config file: %s (permissions %o)\n", cfgPath, perms)
		}
	}

	// Check 3: Provider.
	checks++
	if cfg != nil && cfg.Provider != "" {
		fmt.Printf("  ✓ Provider: %s\n", cfg.Provider)
	} else {
		// Check env vars.
		if os.Getenv("LLM_PROVIDER") != "" {
			fmt.Printf("  ✓ Provider: %s (from env)\n", os.Getenv("LLM_PROVIDER"))
		} else if os.Getenv("ANTHROPIC_API_KEY") != "" {
			fmt.Printf("  ✓ Provider: claude (from env ANTHROPIC_API_KEY)\n")
		} else if os.Getenv("OPENAI_API_KEY") != "" {
			fmt.Printf("  ✓ Provider: openai (from env OPENAI_API_KEY)\n")
		} else {
			fmt.Printf("  ✗ Provider: not configured\n")
			issues++
		}
	}

	// Check 4: API key.
	checks++
	hasKey := false
	if cfg != nil && cfg.APIKey != "" {
		masked := cfg.APIKey[:4] + "..." + cfg.APIKey[len(cfg.APIKey)-4:]
		fmt.Printf("  ✓ API key: %s (from config)\n", masked)
		hasKey = true
	}
	if !hasKey {
		for _, envKey := range []string{"LLM_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
			if v := os.Getenv(envKey); v != "" {
				masked := v[:4] + "..." + v[len(v)-4:]
				fmt.Printf("  ✓ API key: %s (from env %s)\n", masked, envKey)
				hasKey = true
				break
			}
		}
	}
	if !hasKey && cfg != nil && (cfg.Provider == "ollama" || cfg.Provider == "lmstudio") {
		fmt.Printf("  ✓ API key: not needed (local provider)\n")
		hasKey = true
	}
	if !hasKey {
		fmt.Printf("  ✗ API key: not found\n")
		issues++
	}

	// Check 5: Connection test.
	checks++
	if cfg != nil && cfg.Provider != "" {
		fmt.Print("  … Testing connection... ")
		if err := testProviderConnection(cfg); err != nil {
			fmt.Printf("✗ %v\n", err)
			issues++
		} else {
			fmt.Println("✓")
		}
	}

	// Check 6: Soul file.
	checks++
	soulPath := filepath.Join(dataDir, "soul.md")
	if _, err := os.Stat(soulPath); err == nil {
		fmt.Printf("  ✓ Soul: %s\n", soulPath)
	} else {
		fmt.Printf("  … Soul: not initialized (will be created on first run)\n")
	}

	// Check 7: Database.
	checks++
	dbPath := filepath.Join(dataDir, "overhuman.db")
	if info, err := os.Stat(dbPath); err == nil {
		fmt.Printf("  ✓ Database: %s (%d KB)\n", dbPath, info.Size()/1024)
	} else {
		fmt.Printf("  … Database: not created yet (will be created on first run)\n")
	}

	fmt.Println()
	if issues == 0 {
		fmt.Printf("  All %d checks passed! ✓\n\n", checks)
	} else {
		fmt.Printf("  %d/%d checks passed, %d issue(s) found.\n\n", checks-issues, checks, issues)
	}
}

// --- Model discovery ---

type modelOption struct {
	id   string
	desc string
}

// fetchModelsFromAPI queries the provider's API for available models.
// Returns nil if the API is unreachable or returns an error.
func fetchModelsFromAPI(provider, apiKey, baseURL string) []modelOption {
	var reqURL string
	switch provider {
	case "openai":
		reqURL = "https://api.openai.com/v1/models"
	case "claude", "anthropic":
		reqURL = "https://api.anthropic.com/v1/models?limit=100"
	case "ollama":
		base := baseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		reqURL = strings.TrimRight(base, "/") + "/api/tags"
	case "lmstudio":
		base := baseURL
		if base == "" {
			base = "http://localhost:1234"
		}
		reqURL = strings.TrimRight(base, "/") + "/v1/models"
	case "groq":
		reqURL = "https://api.groq.com/openai/v1/models"
	case "together":
		reqURL = "https://api.together.xyz/v1/models"
	case "openrouter":
		reqURL = "https://openrouter.ai/api/v1/models"
	case "custom":
		if baseURL == "" {
			return nil
		}
		reqURL = strings.TrimRight(baseURL, "/") + "/v1/models"
	default:
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil
	}

	// Auth headers.
	if apiKey != "" {
		switch provider {
		case "claude", "anthropic":
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		default:
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return parseModelsResponse(provider, body)
}

// parseModelsResponse parses the JSON response from a provider's model list API.
func parseModelsResponse(provider string, body []byte) []modelOption {
	switch provider {
	case "ollama":
		return parseOllamaModels(body)
	case "claude", "anthropic":
		return parseAnthropicModels(body)
	case "together":
		return parseTogetherModels(body)
	case "openrouter":
		return parseOpenRouterModels(body)
	case "lmstudio":
		return parseLMStudioModels(body)
	default:
		// OpenAI, Groq, custom — all use OpenAI-compatible format.
		return parseOpenAIModels(body, provider)
	}
}

// parseOpenAIModels parses OpenAI-compatible model list (OpenAI, Groq, custom).
func parseOpenAIModels(body []byte, provider string) []modelOption {
	var resp struct {
		Data []struct {
			ID            string `json:"id"`
			OwnedBy       string `json:"owned_by"`
			Created       int64  `json:"created,omitempty"`
			ContextWindow int    `json:"context_window,omitempty"` // Groq extension
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	type modelWithTime struct {
		opt     modelOption
		created int64
	}

	var models []modelWithTime
	for _, m := range resp.Data {
		id := strings.ToLower(m.ID)

		// Filter out non-chat models.
		if strings.HasPrefix(id, "text-embedding") ||
			strings.HasPrefix(id, "whisper") ||
			strings.HasPrefix(id, "tts") ||
			strings.HasPrefix(id, "dall-e") ||
			strings.Contains(id, "embed") ||
			strings.Contains(id, "moderation") ||
			strings.HasPrefix(id, "babbage") ||
			strings.HasPrefix(id, "davinci") {
			continue
		}

		// Filter out old/deprecated/non-standard models (for OpenAI specifically).
		if provider == "openai" {
			if strings.HasPrefix(id, "gpt-3.5") ||
				strings.HasPrefix(id, "gpt-4-") || // old snapshots: gpt-4-0314, gpt-4-turbo-preview, etc.
				strings.HasPrefix(id, "gpt-4o-") || // old gpt-4o snapshots (gpt-4o-2024-05-13, etc.)
				strings.HasPrefix(id, "ft:") || // fine-tuned
				strings.HasPrefix(id, "chatgpt-") || // internal ChatGPT models
				strings.HasPrefix(id, "sora") || // video generation
				strings.HasPrefix(id, "gpt-image") || // image generation
				strings.Contains(id, "-realtime") || // realtime API
				strings.Contains(id, "-audio") || // audio models
				strings.Contains(id, "-transcribe") || // transcription models
				strings.Contains(id, "-tts") || // text-to-speech
				strings.Contains(id, "codex") || // code-only models
				strings.Contains(id, "-chat-latest") || // aliases
				strings.Contains(id, "deep-research") || // deep research mode
				strings.Contains(id, "-search") || // search models
				isDateStamped(id) || // date-stamped versions (gpt-5-2025-08-07)
				id == "gpt-4" || id == "gpt-4-turbo" || id == "gpt-4o" || id == "gpt-4o-mini" {
				continue
			}
		}

		desc := ""
		if m.ContextWindow > 0 {
			desc = fmt.Sprintf("%dk context", m.ContextWindow/1000)
		}
		if m.OwnedBy != "" && m.OwnedBy != "system" && m.OwnedBy != "openai" {
			if desc != "" {
				desc += ", "
			}
			desc += m.OwnedBy
		}
		models = append(models, modelWithTime{
			opt:     modelOption{id: m.ID, desc: desc},
			created: m.Created,
		})
	}

	// Sort by created timestamp: newest first.
	sort.Slice(models, func(i, j int) bool {
		return models[i].created > models[j].created
	})

	result := make([]modelOption, 0, len(models))
	for _, m := range models {
		result = append(result, m.opt)
	}

	return result
}

// isDateStamped returns true if the model ID ends with a date suffix like "-2025-08-07".
func isDateStamped(id string) bool {
	// Match pattern: -YYYY-MM-DD at the end.
	if len(id) < 11 {
		return false
	}
	suffix := id[len(id)-11:]
	if suffix[0] != '-' {
		return false
	}
	date := suffix[1:]
	// Check YYYY-MM-DD format.
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		return false
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if date[i] < '0' || date[i] > '9' {
			return false
		}
	}
	return true
}

// parseAnthropicModels parses Anthropic's model list response.
func parseAnthropicModels(body []byte) []modelOption {
	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var models []modelOption
	for _, m := range resp.Data {
		id := strings.ToLower(m.ID)

		// Filter out old/deprecated model families.
		if strings.HasPrefix(id, "claude-1") ||
			strings.HasPrefix(id, "claude-2") ||
			strings.HasPrefix(id, "claude-instant") {
			continue
		}

		models = append(models, modelOption{id: m.ID, desc: m.DisplayName})
	}

	// Anthropic API returns newest first — keep the order.
	return models
}

// parseOllamaModels parses Ollama's /api/tags response.
func parseOllamaModels(body []byte) []modelOption {
	var resp struct {
		Models []struct {
			Name    string `json:"name"`
			Details struct {
				ParameterSize    string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
				Family           string `json:"family"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var models []modelOption
	for _, m := range resp.Models {
		desc := ""
		if m.Details.ParameterSize != "" {
			desc = m.Details.ParameterSize
		}
		if m.Details.QuantizationLevel != "" {
			if desc != "" {
				desc += " "
			}
			desc += m.Details.QuantizationLevel
		}
		models = append(models, modelOption{id: m.Name, desc: desc})
	}
	return models
}

// parseLMStudioModels parses LM Studio's model list.
func parseLMStudioModels(body []byte) []modelOption {
	var resp struct {
		Data []struct {
			ID           string `json:"id"`
			Type         string `json:"type"`
			Arch         string `json:"arch"`
			Quantization string `json:"quantization"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var models []modelOption
	for _, m := range resp.Data {
		// Filter: only LLM and VLM, skip embeddings.
		if m.Type == "embeddings" {
			continue
		}
		desc := ""
		if m.Arch != "" {
			desc = m.Arch
		}
		if m.Quantization != "" {
			if desc != "" {
				desc += " "
			}
			desc += m.Quantization
		}
		models = append(models, modelOption{id: m.ID, desc: desc})
	}
	return models
}

// parseTogetherModels parses Together AI's model list (bare JSON array).
func parseTogetherModels(body []byte) []modelOption {
	var resp []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var models []modelOption
	for _, m := range resp {
		// Only chat models.
		if m.Type != "" && m.Type != "chat" && m.Type != "language" && m.Type != "code" {
			continue
		}
		desc := m.DisplayName
		models = append(models, modelOption{id: m.ID, desc: desc})
	}

	return models
}

// parseOpenRouterModels parses OpenRouter's model list.
func parseOpenRouterModels(body []byte) []modelOption {
	var resp struct {
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Architecture struct {
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			ContextLength int `json:"context_length"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var models []modelOption
	for _, m := range resp.Data {
		// Filter: only text output models.
		hasText := false
		for _, mod := range m.Architecture.OutputModalities {
			if mod == "text" {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}

		desc := m.Name
		if m.ContextLength > 0 {
			desc += fmt.Sprintf(" (%dk)", m.ContextLength/1000)
		}
		models = append(models, modelOption{id: m.ID, desc: desc})
	}

	return models
}

// --- Terminal helpers ---

// selectItem is one entry in an interactive selector.
type selectItem struct {
	label string
	desc  string
}

// interactiveSelect shows an arrow-key navigable menu.
// Returns the 0-based index of the selected item, or -1 if cancelled.
// If the terminal doesn't support raw mode, falls back to numbered input.
func interactiveSelect(items []selectItem, defaultIdx int) int {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fallbackSelect(items, defaultIdx)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fallbackSelect(items, defaultIdx)
	}
	defer term.Restore(fd, oldState)

	cursor := defaultIdx
	if cursor < 0 || cursor >= len(items) {
		cursor = 0
	}

	// First render — draw the full list from scratch.
	renderSelectFull(items, cursor)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			continue
		}

		switch {
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'):
			// Enter — confirm selection.
			// Move cursor below the list.
			fmt.Printf("\r\033[%dB", len(items)-cursor)
			fmt.Print("\r\n")
			return cursor

		case n == 1 && buf[0] == 3:
			// Ctrl+C — cancel.
			fmt.Printf("\r\033[%dB", len(items)-cursor)
			fmt.Print("\r\n")
			return -1

		case n == 1 && buf[0] == 'q':
			// q — cancel.
			fmt.Printf("\r\033[%dB", len(items)-cursor)
			fmt.Print("\r\n")
			return -1

		case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'A':
			// Arrow up.
			if cursor > 0 {
				cursor--
				renderSelect(items, cursor)
			}

		case n == 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'B':
			// Arrow down.
			if cursor < len(items)-1 {
				cursor++
				renderSelect(items, cursor)
			}

		case n == 1 && buf[0] == 'k':
			// vim: k = up.
			if cursor > 0 {
				cursor--
				renderSelect(items, cursor)
			}

		case n == 1 && buf[0] == 'j':
			// vim: j = down.
			if cursor < len(items)-1 {
				cursor++
				renderSelect(items, cursor)
			}
		}
	}
}

// renderSelectFull draws the menu for the first time (no cursor movement up).
func renderSelectFull(items []selectItem, cursor int) {
	for i, item := range items {
		fmt.Print("\r\033[K") // clear line
		if i == cursor {
			if item.desc != "" {
				fmt.Printf("  \033[1;36m→ %-38s\033[0m \033[90m%s\033[0m", item.label, item.desc)
			} else {
				fmt.Printf("  \033[1;36m→ %s\033[0m", item.label)
			}
		} else {
			if item.desc != "" {
				fmt.Printf("    %-38s \033[90m%s\033[0m", item.label, item.desc)
			} else {
				fmt.Printf("    %s", item.label)
			}
		}
		if i < len(items)-1 {
			fmt.Print("\n")
		}
	}
	// Move cursor back to selected line.
	if cursor < len(items)-1 {
		fmt.Printf("\033[%dA", len(items)-1-cursor)
	}
}

// renderSelect redraws the menu in-place (subsequent renders after first).
func renderSelect(items []selectItem, cursor int) {
	// Move to first item line.
	if cursor > 0 {
		fmt.Printf("\033[%dA", cursor) // move up to top of list
	}

	for i, item := range items {
		fmt.Print("\r\033[K") // clear line
		if i == cursor {
			if item.desc != "" {
				fmt.Printf("  \033[1;36m→ %-38s\033[0m \033[90m%s\033[0m", item.label, item.desc)
			} else {
				fmt.Printf("  \033[1;36m→ %s\033[0m", item.label)
			}
		} else {
			if item.desc != "" {
				fmt.Printf("    %-38s \033[90m%s\033[0m", item.label, item.desc)
			} else {
				fmt.Printf("    %s", item.label)
			}
		}
		if i < len(items)-1 {
			fmt.Print("\n")
		}
	}

	// Move cursor back to selected line.
	if cursor < len(items)-1 {
		fmt.Printf("\033[%dA", len(items)-1-cursor)
	}
}

// fallbackSelect is a numbered-input fallback for non-TTY environments.
func fallbackSelect(items []selectItem, defaultIdx int) int {
	reader := bufio.NewReader(os.Stdin)
	for i, item := range items {
		marker := "  "
		if i == defaultIdx {
			marker = "→ "
		}
		if item.desc != "" {
			fmt.Printf("  %s%d) %-38s %s\n", marker, i+1, item.label, item.desc)
		} else {
			fmt.Printf("  %s%d) %s\n", marker, i+1, item.label)
		}
	}
	fmt.Println()

	defaultStr := ""
	if defaultIdx >= 0 {
		defaultStr = fmt.Sprintf("%d", defaultIdx+1)
	}

	for {
		if defaultStr != "" {
			fmt.Printf("  Choose [%s]: ", defaultStr)
		} else {
			fmt.Print("  Choose: ")
		}

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && defaultStr != "" {
			line = defaultStr
		}

		var choice int
		if _, err := fmt.Sscanf(line, "%d", &choice); err == nil && choice >= 1 && choice <= len(items) {
			return choice - 1
		}
		fmt.Printf("  Enter a number between 1 and %d.\n", len(items))
	}
}

// promptString asks for a string input with a default value.
func promptString(reader *bufio.Reader, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}

	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// readSecretLine reads a line without echoing (for API keys).
func readSecretLine(reader *bufio.Reader) string {
	// Try terminal raw mode (no echo).
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err == nil {
			return strings.TrimSpace(string(password))
		}
	}

	// Fallback: read normally.
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
