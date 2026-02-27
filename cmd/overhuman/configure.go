package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	fmt.Println("Select your LLM provider:")
	fmt.Println()
	providers := []struct {
		key  string
		name string
		desc string
	}{
		{"openai", "OpenAI", "GPT-4o, GPT-4o-mini (requires API key)"},
		{"claude", "Anthropic Claude", "Claude Sonnet, Haiku, Opus (requires API key)"},
		{"ollama", "Ollama", "Local models — llama3, mistral, etc. (free, no API key)"},
		{"lmstudio", "LM Studio", "Local models via LM Studio (free, no API key)"},
		{"groq", "Groq", "Fast inference — Llama, Mixtral (requires API key)"},
		{"together", "Together AI", "Open-source models hosted (requires API key)"},
		{"openrouter", "OpenRouter", "Multi-model gateway (requires API key)"},
		{"custom", "Custom endpoint", "Any OpenAI-compatible API"},
	}

	for i, p := range providers {
		marker := "  "
		if existing.Provider == p.key {
			marker = "→ "
		}
		fmt.Printf("  %s%d) %-20s %s\n", marker, i+1, p.name, p.desc)
	}
	fmt.Println()

	defaultChoice := ""
	if existing.Provider != "" {
		for i, p := range providers {
			if p.key == existing.Provider {
				defaultChoice = fmt.Sprintf("%d", i+1)
				break
			}
		}
	}

	provider := promptChoice(reader, "Choose provider", defaultChoice, len(providers))
	selectedProvider := providers[provider-1]
	fmt.Printf("\n  ✓ Selected: %s\n\n", selectedProvider.name)

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

	// Step 4: Model (optional override).
	defaultModel := ""
	switch selectedProvider.key {
	case "openai":
		defaultModel = "gpt-4o"
	case "claude":
		defaultModel = "claude-sonnet-4-20250514"
	case "ollama":
		defaultModel = "llama3.3"
	case "groq":
		defaultModel = "llama-3.3-70b-versatile"
	}
	if existing.Model != "" {
		defaultModel = existing.Model
	}

	model := promptString(reader, "Default model", defaultModel)
	cfg.Model = model
	fmt.Printf("  ✓ Model: %s\n\n", model)

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

// --- Terminal helpers ---

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

// promptChoice asks for a numbered choice.
func promptChoice(reader *bufio.Reader, prompt, defaultVal string, max int) int {
	for {
		if defaultVal != "" {
			fmt.Printf("  %s [%s]: ", prompt, defaultVal)
		} else {
			fmt.Printf("  %s: ", prompt)
		}

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && defaultVal != "" {
			line = defaultVal
		}

		var choice int
		if _, err := fmt.Sscanf(line, "%d", &choice); err == nil && choice >= 1 && choice <= max {
			return choice
		}
		fmt.Printf("  Please enter a number between 1 and %d.\n", max)
	}
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
