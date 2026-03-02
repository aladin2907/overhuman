// Package main is the entry point for the Overhuman daemon.
//
// Usage:
//
//	overhuman cli              — interactive CLI mode
//	overhuman start            — daemon mode (HTTP API + heartbeat)
//	overhuman version          — print version
//	overhuman status           — check daemon health
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/deploy"
	"github.com/overhuman/overhuman/internal/genui"
	"github.com/overhuman/overhuman/internal/memory"
	"github.com/overhuman/overhuman/internal/pipeline"
	"github.com/overhuman/overhuman/internal/reflection"
	"github.com/overhuman/overhuman/internal/senses"
	"github.com/overhuman/overhuman/internal/soul"
)

const (
	version = "0.1.0"
	appName = "overhuman"
)

// Config holds the daemon configuration.
type Config struct {
	DataDir     string
	AgentName   string
	APIAddr     string
	ClaudeKey   string
	OpenAIKey   string
	DefaultSpec string

	// Universal provider settings.
	LLMProvider  string // "openai", "claude", "ollama", "lmstudio", "groq", "together", "openrouter", "custom"
	LLMBaseURL   string // Custom base URL (for "custom" or override)
	LLMModel     string // Default model override
	LLMAPIKey    string // API key (for custom provider)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "cli":
		ensureConfigured()
		runCLI()
	case "start":
		ensureConfigured()
		runDaemon()
	case "configure", "config", "setup":
		runConfigure()
	case "doctor":
		runDoctor()
	case "version":
		fmt.Printf("%s v%s\n", appName, version)
	case "install":
		runInstall()
	case "uninstall":
		runUninstall()
	case "stop":
		runStop()
	case "update":
		runUpdate()
	case "logs":
		runLogs()
	case "status":
		runStatus()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `%s v%s — self-evolving universal assistant

Usage:
  %s <command>

Commands:
  configure  Interactive setup wizard (API keys, provider, model)
  cli        Interactive CLI mode (stdin/stdout)
  start      Start daemon (HTTP API + heartbeat timer)
  stop       Stop the running daemon (sends SIGTERM)
  status     Check daemon health (requires running daemon)
  install    Install as an OS service (launchd/systemd)
  uninstall  Remove the OS service
  update     Check for and apply updates
  logs       Tail the daemon log file
  doctor     Diagnose configuration issues
  version    Print version

Environment variables (override config.json):
  ANTHROPIC_API_KEY   Claude API key (auto-detected)
  OPENAI_API_KEY      OpenAI API key (auto-detected)
  OVERHUMAN_DATA      Data directory (default: ~/.overhuman)
  OVERHUMAN_API_ADDR  API listen address (default: 127.0.0.1:9090)
  OVERHUMAN_NAME      Agent name (default: Overhuman)
  LLM_PROVIDER        Provider: openai, claude, ollama, lmstudio, groq, together, openrouter, custom
  LLM_BASE_URL        Custom API base URL (e.g., http://localhost:11434 for Ollama)
  LLM_MODEL           Default model override (e.g., llama3.3, gpt-4o, claude-sonnet-4-20250514)
  LLM_API_KEY         API key for custom/groq/together/openrouter providers

`, appName, version, appName)
}

func loadConfig() Config {
	dataDir := os.Getenv("OVERHUMAN_DATA")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot determine home directory: %v", err)
		}
		dataDir = filepath.Join(home, ".overhuman")
	}

	// Defaults.
	cfg := Config{
		DataDir:     dataDir,
		AgentName:   "Overhuman",
		APIAddr:     "127.0.0.1:9090",
		DefaultSpec: "general",
	}

	// Layer 1: Load from config.json (persistent settings).
	if persisted, err := loadPersistedConfig(); err == nil && persisted != nil {
		if persisted.Provider != "" {
			cfg.LLMProvider = persisted.Provider
		}
		if persisted.APIKey != "" {
			cfg.LLMAPIKey = persisted.APIKey
			// Also set provider-specific keys for backward compat.
			switch persisted.Provider {
			case "claude", "anthropic":
				cfg.ClaudeKey = persisted.APIKey
			case "openai":
				cfg.OpenAIKey = persisted.APIKey
			}
		}
		if persisted.Model != "" {
			cfg.LLMModel = persisted.Model
		}
		if persisted.BaseURL != "" {
			cfg.LLMBaseURL = persisted.BaseURL
		}
		if persisted.Name != "" {
			cfg.AgentName = persisted.Name
		}
		if persisted.APIAddr != "" {
			cfg.APIAddr = persisted.APIAddr
		}
	}

	// Layer 2: Environment variables override config.json.
	if v := os.Getenv("OVERHUMAN_API_ADDR"); v != "" {
		cfg.APIAddr = v
	}
	if v := os.Getenv("OVERHUMAN_NAME"); v != "" {
		cfg.AgentName = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.ClaudeKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAIKey = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		cfg.LLMProvider = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		cfg.LLMBaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLMModel = v
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLMAPIKey = v
	}

	return cfg
}

// ensureConfigured checks if the system is configured and guides the user if not.
func ensureConfigured() {
	cfg := loadConfig()

	// Check if any provider is configured.
	hasProvider := cfg.LLMProvider != "" || cfg.ClaudeKey != "" || cfg.OpenAIKey != ""
	if hasProvider {
		return
	}

	// No provider configured — check if config.json exists at all.
	persisted, _ := loadPersistedConfig()
	if persisted != nil && persisted.Provider != "" {
		return // Config exists, provider set.
	}

	// First run — no config, no env vars.
	fmt.Printf("\n👋 Welcome to %s v%s!\n\n", appName, version)
	fmt.Println("  No LLM provider configured. Let's set one up.")
	fmt.Println()
	fmt.Println("  Quick options:")
	fmt.Println("    1) Run the setup wizard:  overhuman configure")
	fmt.Println("    2) Set an env variable:   export OPENAI_API_KEY=sk-...")
	fmt.Println("    3) Use a local model:     export LLM_PROVIDER=ollama")
	fmt.Println()

	// Auto-start wizard if running interactively.
	if isTerminal() {
		fmt.Print("  Start setup wizard now? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || line == "y" || line == "yes" {
			fmt.Println()
			runConfigure()
			return
		}
	}

	fmt.Fprintf(os.Stderr, "  Run '%s configure' to set up your API key.\n\n", appName)
	os.Exit(1)
}

// isTerminal returns true if stdin is a terminal.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// bootstrap initializes all subsystems and returns the pipeline dependencies.
func bootstrap(cfg Config) (pipeline.Dependencies, *reflection.Engine, *genui.UIGenerator, error) {
	// Ensure data directory exists.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return pipeline.Dependencies{}, nil, nil, fmt.Errorf("create data dir: %w", err)
	}

	// Soul.
	s := soul.New(cfg.DataDir, cfg.AgentName, cfg.DefaultSpec)
	if err := s.Initialize(); err != nil {
		// Already initialized is fine.
		if _, readErr := s.Read(); readErr != nil {
			return pipeline.Dependencies{}, nil, nil, fmt.Errorf("soul: %w", err)
		}
	}
	log.Printf("[bootstrap] soul initialized: %s", cfg.AgentName)

	// LLM provider — universal, supports any OpenAI-compatible endpoint.
	llm, providerName, err := createLLMProvider(cfg)
	if err != nil {
		return pipeline.Dependencies{}, nil, nil, err
	}
	log.Printf("[bootstrap] LLM: %s", providerName)

	// Memory.
	dbPath := filepath.Join(cfg.DataDir, "overhuman.db")
	ltm, err := memory.NewLongTermMemory(dbPath)
	if err != nil {
		return pipeline.Dependencies{}, nil, nil, fmt.Errorf("long-term memory: %w", err)
	}
	log.Printf("[bootstrap] long-term memory: %s", dbPath)

	pt, err := memory.NewPatternTracker(ltm.DB())
	if err != nil {
		ltm.Close()
		return pipeline.Dependencies{}, nil, nil, fmt.Errorf("pattern tracker: %w", err)
	}
	log.Printf("[bootstrap] pattern tracker ready")

	stm := memory.NewShortTermMemory(100)

	// Brain — model router uses models from the active provider.
	var router *brain.ModelRouter
	if up, ok := llm.(*brain.UniversalProvider); ok {
		router = brain.NewModelRouterWithModels(up.ModelEntries())
	} else {
		router = brain.NewModelRouter()
		router.SetProvider(providerName)
	}
	log.Printf("[bootstrap] model router: provider=%s", providerName)
	ca := brain.NewContextAssembler()

	// Reflection engine.
	reflEngine := reflection.NewEngine(llm, router, ca, ltm)

	deps := pipeline.Dependencies{
		Soul:          s,
		LLM:           llm,
		Router:        router,
		Context:       ca,
		ShortTerm:     stm,
		LongTerm:      ltm,
		Patterns:      pt,
		AutoThreshold: 3,
		Reflection:    reflEngine,
	}

	// UI generator — separate LLM call for visual representation.
	uiGen := genui.NewUIGenerator(llm, router)

	log.Printf("[bootstrap] all subsystems ready")
	return deps, reflEngine, uiGen, nil
}

// createLLMProvider creates the appropriate LLM provider based on config.
// Priority: LLM_PROVIDER env > ANTHROPIC_API_KEY > OPENAI_API_KEY.
func createLLMProvider(cfg Config) (brain.LLMProvider, string, error) {
	// Explicit provider selection via LLM_PROVIDER.
	if cfg.LLMProvider != "" {
		return createNamedProvider(cfg)
	}

	// Auto-detect from legacy env vars (backward compatible).
	if cfg.ClaudeKey != "" {
		p := brain.NewUniversalProvider(brain.AnthropicConfig(cfg.ClaudeKey))
		return p, "claude", nil
	}
	if cfg.OpenAIKey != "" {
		pcfg := brain.OpenAIConfig(cfg.OpenAIKey)
		if cfg.LLMModel != "" {
			pcfg.DefaultModel = cfg.LLMModel
		}
		p := brain.NewUniversalProvider(pcfg)
		return p, "openai", nil
	}

	return nil, "", fmt.Errorf("no LLM provider configured.\n\nSet one of:\n" +
		"  export OPENAI_API_KEY=sk-...          # OpenAI\n" +
		"  export ANTHROPIC_API_KEY=sk-ant-...   # Claude\n" +
		"  export LLM_PROVIDER=ollama            # Local Ollama\n" +
		"  export LLM_PROVIDER=custom LLM_BASE_URL=http://... LLM_MODEL=...\n")
}

// createNamedProvider creates a provider by name.
func createNamedProvider(cfg Config) (brain.LLMProvider, string, error) {
	apiKey := cfg.LLMAPIKey
	model := cfg.LLMModel

	var pcfg brain.ProviderConfig

	switch cfg.LLMProvider {
	case "openai":
		if apiKey == "" {
			apiKey = cfg.OpenAIKey
		}
		if apiKey == "" {
			return nil, "", fmt.Errorf("openai: set OPENAI_API_KEY or LLM_API_KEY")
		}
		pcfg = brain.OpenAIConfig(apiKey)

	case "claude", "anthropic":
		if apiKey == "" {
			apiKey = cfg.ClaudeKey
		}
		if apiKey == "" {
			return nil, "", fmt.Errorf("claude: set ANTHROPIC_API_KEY or LLM_API_KEY")
		}
		// Note: Claude native API uses different message format.
		// Use the dedicated ClaudeProvider for full compatibility.
		p := brain.NewClaudeProvider(apiKey)
		return p, "claude", nil

	case "ollama":
		pcfg = brain.OllamaConfig(model)
		if cfg.LLMBaseURL != "" {
			pcfg.BaseURL = cfg.LLMBaseURL
		}

	case "lmstudio":
		pcfg = brain.LMStudioConfig(model)
		if cfg.LLMBaseURL != "" {
			pcfg.BaseURL = cfg.LLMBaseURL
		}

	case "groq":
		if apiKey == "" {
			return nil, "", fmt.Errorf("groq: set LLM_API_KEY")
		}
		pcfg = brain.GroqConfig(apiKey)

	case "together":
		if apiKey == "" {
			return nil, "", fmt.Errorf("together: set LLM_API_KEY")
		}
		pcfg = brain.TogetherConfig(apiKey)

	case "openrouter":
		if apiKey == "" {
			return nil, "", fmt.Errorf("openrouter: set LLM_API_KEY")
		}
		pcfg = brain.OpenRouterConfig(apiKey)

	case "custom":
		if cfg.LLMBaseURL == "" {
			return nil, "", fmt.Errorf("custom: set LLM_BASE_URL")
		}
		if model == "" {
			model = "default"
		}
		pcfg = brain.CustomConfig("custom", cfg.LLMBaseURL, apiKey, model)

	default:
		return nil, "", fmt.Errorf("unknown LLM_PROVIDER: %q (use: openai, claude, ollama, lmstudio, groq, together, openrouter, custom)", cfg.LLMProvider)
	}

	if model != "" && pcfg.DefaultModel != model {
		pcfg.DefaultModel = model
	}

	p := brain.NewUniversalProvider(pcfg)
	return p, pcfg.Name, nil
}

// runCLI starts the agent in interactive CLI mode.
func runCLI() {
	cfg := loadConfig()
	deps, _, uiGen, err := bootstrap(cfg)
	if err != nil {
		log.Fatalf("[cli] bootstrap: %v", err)
	}

	p := pipeline.New(deps)
	cli := senses.NewCLISense(os.Stdin, os.Stdout)
	uiRenderer := genui.NewCLIRenderer(os.Stdout, os.Stdin)
	uiReflection := genui.NewReflectionStore()
	caps := genui.CLICapabilities()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Catch signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nshutting down...")
		cancel()
	}()

	out := make(chan *senses.UnifiedInput, 10)

	// Start CLI sense in background.
	go func() {
		if err := cli.Start(ctx, out); err != nil && ctx.Err() == nil {
			log.Printf("[cli] sense error: %v", err)
		}
		cancel() // EOF → shutdown
	}()

	fmt.Printf("%s v%s — interactive mode (type /quit to exit)\n\n", appName, version)

	for {
		select {
		case <-ctx.Done():
			return
		case input, ok := <-out:
			if !ok {
				return
			}

			result, err := p.Run(ctx, *input)
			if err != nil {
				cli.Send(ctx, "", fmt.Sprintf("Error: %v", err))
				continue
			}

			// Build ThoughtLog from pipeline stage logs.
			var thought *genui.ThoughtLog
			if len(result.StageLogs) > 0 {
				stages := make([]genui.ThoughtStage, len(result.StageLogs))
				for i, sl := range result.StageLogs {
					stages[i] = genui.ThoughtStage{
						Number:  sl.Number,
						Name:    sl.Name,
						Summary: sl.Summary,
						DurMs:   sl.DurMs,
					}
				}
				thought = genui.BuildThoughtLog(stages)
				thought.TotalCost = result.CostUSD
			}

			// Generate UI (separate LLM call).
			hints := uiReflection.BuildHints(result.Fingerprint)
			ui, uiErr := uiGen.GenerateWithThought(ctx, *result, caps, thought, hints)
			if uiErr != nil {
				// Fallback: plain text output.
				output := fmt.Sprintf("[task: %s | quality: %.0f%% | cost: $%.4f | time: %dms]\n%s",
					result.TaskID,
					result.QualityScore*100,
					result.CostUSD,
					result.ElapsedMs,
					result.Result,
				)
				if result.AutomationTriggered {
					output += "\n⚡ Pattern detected — automation triggered"
				}
				cli.Send(ctx, "", output)
				continue
			}

			// Render generated UI.
			if renderErr := uiRenderer.Render(ui); renderErr != nil {
				cli.Send(ctx, "", result.Result) // ultimate fallback
			}
			fmt.Println() // newline after UI
		}
	}
}

// runDaemon starts the full daemon with HTTP API, WebSocket UI server, and heartbeat timer.
func runDaemon() {
	cfg := loadConfig()

	// PID file — ensures single instance and enables `stop` command.
	pf := deploy.NewPIDFile(cfg.DataDir)
	if err := pf.Guard(); err != nil {
		log.Fatalf("[daemon] %v", err)
	}
	defer pf.Remove()

	// Set up log tee: write to stdout AND to log file.
	logFile := setupLogTee(cfg.DataDir)
	if logFile != nil {
		defer logFile.Close()
	}

	deps, _, uiGen, err := bootstrap(cfg)
	if err != nil {
		log.Fatalf("[daemon] bootstrap: %v", err)
	}

	p := pipeline.New(deps)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Catch signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Shared input channel.
	out := make(chan *senses.UnifiedInput, 50)

	// Start HTTP API sense.
	api := senses.NewAPISense(cfg.APIAddr)
	go func() {
		log.Printf("[daemon] API listening on %s", cfg.APIAddr)
		if err := api.Start(ctx, out); err != nil && ctx.Err() == nil {
			log.Printf("[daemon] API error: %v", err)
		}
	}()

	// WebSocket UI server on derived port (API port + 1).
	wsAddr := deriveWSAddr(cfg.APIAddr)
	wsSrv := genui.NewWSServer(wsAddr)
	uiAPIHandler := genui.NewUIAPIHandler(uiGen, wsSrv)
	uiReflection := genui.NewReflectionStore()
	webCaps := genui.WebCapabilities(1280, 800)

	// Wire WS incoming messages → pipeline input or reflection store.
	wsSrv.OnMessage(func(connID string, msg *genui.WSMessage) {
		switch msg.Type {
		case genui.WSMsgInput:
			payload, err := genui.ParseInputPayload(msg)
			if err != nil {
				log.Printf("[ws] bad input payload from %s: %v", connID, err)
				return
			}
			input := senses.NewUnifiedInput(senses.SourceAPI, payload.Text)
			input.ResponseChannel = "ws"
			input.CorrelationID = connID
			select {
			case out <- input:
			default:
				log.Printf("[ws] pipeline busy, dropping input from %s", connID)
			}

		case genui.WSMsgUIFeedback:
			payload, err := genui.ParseUIFeedbackPayload(msg)
			if err != nil {
				log.Printf("[ws] bad feedback from %s: %v", connID, err)
				return
			}
			uiReflection.Record(genui.UIReflection{
				TaskID:       payload.TaskID,
				Scrolled:     payload.Scrolled,
				TimeToAction: payload.TimeToAction,
				ActionsUsed:  payload.ActionsUsed,
				Dismissed:    payload.Dismissed,
			})

		case genui.WSMsgCancel:
			log.Printf("[ws] cancel request from %s", connID)

		case genui.WSMsgAction:
			log.Printf("[ws] action from %s (not yet routed)", connID)
		}
	})

	// Also start standalone WS server on derived port for non-browser clients.
	go func() {
		log.Printf("[daemon] WebSocket also on %s (standalone)", wsAddr)
		if err := wsSrv.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[daemon] WS error: %v", err)
		}
	}()

	// Kiosk web server on derived port (API port + 2).
	// WebSocket /ws is registered on the SAME mux as kiosk to avoid
	// cross-port issues with browsers (same-origin policy for WS).
	kioskAddr := deriveKioskAddr(cfg.APIAddr)
	kioskCfg := genui.KioskConfig{
		WSAddr:        kioskAddr, // WS on same port as kiosk
		Title:         cfg.AgentName,
		DarkMode:      true,
		ShowSidebar:   true,
		EmergencyStop: true,
	}
	kioskHandler := genui.NewKioskHandler(kioskCfg)
	kioskMux := http.NewServeMux()
	kioskHandler.RegisterRoutes(kioskMux)
	uiAPIHandler.RegisterRoutes(kioskMux)
	wsSrv.RegisterRoutes(kioskMux, ctx) // WS on kiosk port

	kioskServer := &http.Server{
		Addr:    kioskAddr,
		Handler: kioskMux,
	}
	go func() {
		log.Printf("[daemon] Kiosk UI on http://%s", kioskAddr)
		if err := kioskServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[daemon] kiosk error: %v", err)
		}
	}()

	// File watcher sense — monitors ~/.overhuman/inbox/ for new files.
	inboxDir := filepath.Join(cfg.DataDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		log.Printf("[daemon] create inbox dir: %v", err)
	} else {
		fw := senses.NewFileWatcherSense(senses.FileWatcherConfig{
			WatchDir:     inboxDir,
			PollInterval: 5 * time.Second,
			Recursive:    true,
		})
		go func() {
			log.Printf("[daemon] file watcher: %s", inboxDir)
			if err := fw.Start(ctx, out); err != nil && ctx.Err() == nil {
				log.Printf("[daemon] file watcher error: %v", err)
			}
		}()
	}

	// Heartbeat timer (every 30 minutes).
	heartbeatTicker := time.NewTicker(30 * time.Minute)
	defer heartbeatTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				hb := senses.NewHeartbeat()
				select {
				case out <- hb:
					log.Printf("[daemon] heartbeat sent")
				default:
					log.Printf("[daemon] heartbeat skipped (pipeline busy)")
				}
			}
		}
	}()

	log.Printf("[daemon] %s v%s started (API=%s, WS=%s, Kiosk=http://%s, Inbox=%s)", cfg.AgentName, version, cfg.APIAddr, wsAddr, kioskAddr, inboxDir)

	// Main processing loop.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case input, ok := <-out:
				if !ok {
					return
				}
				result, err := p.Run(ctx, *input)
				if err != nil {
					log.Printf("[daemon] run error: %v", err)
					continue
				}

				log.Printf("[daemon] completed task=%s quality=%.0f%% cost=$%.4f time=%dms automation=%v",
					result.TaskID,
					result.QualityScore*100,
					result.CostUSD,
					result.ElapsedMs,
					result.AutomationTriggered,
				)

				// If the input has a response channel, try to send back.
				if input.CorrelationID != "" && input.ResponseChannel == "api" {
					api.Send(ctx, input.CorrelationID, result.Result)
				}

				// Generate UI and broadcast to connected WebSocket clients.
				if wsSrv.ClientCount() > 0 {
					var thought *genui.ThoughtLog
					if len(result.StageLogs) > 0 {
						stages := make([]genui.ThoughtStage, len(result.StageLogs))
						for i, sl := range result.StageLogs {
							stages[i] = genui.ThoughtStage{
								Number:  sl.Number,
								Name:    sl.Name,
								Summary: sl.Summary,
								DurMs:   sl.DurMs,
							}
						}
						thought = genui.BuildThoughtLog(stages)
						thought.TotalCost = result.CostUSD
					}

					hints := uiReflection.BuildHints(result.Fingerprint)
					ui, uiErr := uiGen.GenerateWithThought(ctx, *result, webCaps, thought, hints)
					if uiErr != nil {
						log.Printf("[daemon] UI generation failed: %v", uiErr)
					} else {
						ui.Sandbox = true
						uiAPIHandler.CacheUI(ui)
						if bErr := wsSrv.BroadcastUI(ui); bErr != nil {
							log.Printf("[daemon] UI broadcast error: %v", bErr)
						}
					}
				}
			}
		}
	}()

	// Wait for shutdown signal.
	<-sigCh
	log.Printf("[daemon] shutting down...")
	cancel()

	// Graceful shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := kioskServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[daemon] kiosk shutdown error: %v", err)
	}
	wsSrv.Stop()
	api.Stop()
	deps.LongTerm.Close()
	log.Printf("[daemon] shutdown complete")
}

// deriveWSAddr increments the port from the API address by 1 for the WebSocket server.
func deriveWSAddr(apiAddr string) string {
	host, portStr, err := net.SplitHostPort(apiAddr)
	if err != nil {
		return "127.0.0.1:9091"
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return net.JoinHostPort(host, fmt.Sprintf("%d", port+1))
}

// deriveKioskAddr increments the port from the API address by 2 for the kiosk HTTP server.
func deriveKioskAddr(apiAddr string) string {
	host, portStr, err := net.SplitHostPort(apiAddr)
	if err != nil {
		return "127.0.0.1:9092"
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return net.JoinHostPort(host, fmt.Sprintf("%d", port+2))
}

// runStatus checks if the daemon is running by hitting the health endpoint.
func runStatus() {
	cfg := loadConfig()
	addr := cfg.APIAddr

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", addr))
	if err != nil {
		fmt.Printf("daemon is NOT running at %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Printf("daemon is running at %s\n", addr)
	} else {
		fmt.Printf("daemon returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}

// runInstall installs overhuman as an OS service.
func runInstall() {
	cfg := loadConfig()

	// Find the binary path.
	binPath, err := os.Executable()
	if err != nil {
		log.Fatalf("[install] cannot determine binary path: %v", err)
	}

	svcCfg := deploy.ServiceConfig{
		BinaryPath: binPath,
		DataDir:    cfg.DataDir,
		APIAddr:    cfg.APIAddr,
		AgentName:  cfg.AgentName,
	}

	result, err := deploy.Install(svcCfg)
	if err != nil {
		log.Fatalf("[install] %v", err)
	}

	fmt.Println(result.Instructions)
}

// runUninstall removes the OS service.
func runUninstall() {
	result, err := deploy.Uninstall()
	if err != nil {
		log.Fatalf("[uninstall] %v", err)
	}
	fmt.Println(result.Instructions)
}

// runStop sends SIGTERM to the running daemon.
func runStop() {
	cfg := loadConfig()
	if err := deploy.StopDaemon(cfg.DataDir); err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("daemon stopped")
}

// runUpdate checks for and applies updates.
func runUpdate() {
	cfg := loadConfig()

	binPath, err := os.Executable()
	if err != nil {
		log.Fatalf("[update] cannot determine binary path: %v", err)
	}

	ucfg := deploy.UpdateConfig{
		CurrentVersion: version,
		DataDir:        cfg.DataDir,
		BinaryPath:     binPath,
	}

	fmt.Println("Checking for updates...")
	info, err := deploy.CheckUpdate(ucfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
		os.Exit(1)
	}

	if info == nil {
		fmt.Printf("Already up to date (v%s)\n", version)
		return
	}

	fmt.Printf("New version available: v%s (current: v%s)\n", info.Version, version)
	if info.ReleaseNotes != "" {
		fmt.Printf("Release notes:\n%s\n\n", info.ReleaseNotes)
	}

	fmt.Print("Apply update? [y/N]: ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" && answer != "yes" {
		fmt.Println("Update cancelled.")
		return
	}

	result, err := deploy.ApplyUpdate(ucfg, info)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.Message)
	if result.NeedsRestart {
		fmt.Println("Please restart the daemon to use the new version.")
	}
}

// setupLogTee configures log output to write to both stdout and a log file.
// Returns the log file handle (caller should defer Close) or nil on error.
func setupLogTee(dataDir string) *os.File {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Printf("[daemon] cannot create log dir: %v (logging to stdout only)", err)
		return nil
	}

	logPath := filepath.Join(logDir, "overhuman.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("[daemon] cannot open log file: %v (logging to stdout only)", err)
		return nil
	}

	// Tee: all log output goes to stdout AND the file.
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.Printf("[daemon] logging to %s", logPath)
	return f
}

// runLogs tails the daemon log file.
func runLogs() {
	cfg := loadConfig()
	logPath := filepath.Join(cfg.DataDir, "logs", "overhuman.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "no log file found at %s\n", logPath)
		fmt.Fprintf(os.Stderr, "hint: start the daemon with 'overhuman start' — it logs to stdout and file.\n")
		os.Exit(1)
	}

	// Read and print last 50 lines.
	data, err := os.ReadFile(logPath)
	if err != nil {
		log.Fatalf("[logs] read: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println(line)
		}
	}
}
