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
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
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
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "cli":
		runCLI()
	case "start":
		runDaemon()
	case "version":
		fmt.Printf("%s v%s\n", appName, version)
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
  cli        Interactive CLI mode (stdin/stdout)
  start      Start daemon (HTTP API + heartbeat timer)
  status     Check daemon health (requires running daemon)
  version    Print version

Environment variables:
  ANTHROPIC_API_KEY   Claude API key
  OPENAI_API_KEY      OpenAI API key
  OVERHUMAN_DATA      Data directory (default: ~/.overhuman)
  OVERHUMAN_API_ADDR  API listen address (default: 127.0.0.1:9090)
  OVERHUMAN_NAME      Agent name (default: Overhuman)

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

	apiAddr := os.Getenv("OVERHUMAN_API_ADDR")
	if apiAddr == "" {
		apiAddr = "127.0.0.1:9090"
	}

	agentName := os.Getenv("OVERHUMAN_NAME")
	if agentName == "" {
		agentName = "Overhuman"
	}

	return Config{
		DataDir:     dataDir,
		AgentName:   agentName,
		APIAddr:     apiAddr,
		ClaudeKey:   os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIKey:   os.Getenv("OPENAI_API_KEY"),
		DefaultSpec: "general",
	}
}

// bootstrap initializes all subsystems and returns the pipeline dependencies.
func bootstrap(cfg Config) (pipeline.Dependencies, *reflection.Engine, error) {
	// Ensure data directory exists.
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return pipeline.Dependencies{}, nil, fmt.Errorf("create data dir: %w", err)
	}

	// Soul.
	s := soul.New(cfg.DataDir, cfg.AgentName, cfg.DefaultSpec)
	if err := s.Initialize(); err != nil {
		// Already initialized is fine.
		if _, readErr := s.Read(); readErr != nil {
			return pipeline.Dependencies{}, nil, fmt.Errorf("soul: %w", err)
		}
	}
	log.Printf("[bootstrap] soul initialized: %s", cfg.AgentName)

	// LLM provider.
	var llm brain.LLMProvider
	var providerName string
	if cfg.ClaudeKey != "" {
		llm = brain.NewClaudeProvider(cfg.ClaudeKey)
		providerName = "claude"
		log.Printf("[bootstrap] LLM: Claude")
	} else if cfg.OpenAIKey != "" {
		llm = brain.NewOpenAIProvider(cfg.OpenAIKey)
		providerName = "openai"
		log.Printf("[bootstrap] LLM: OpenAI")
	} else {
		return pipeline.Dependencies{}, nil, fmt.Errorf("no API key set (ANTHROPIC_API_KEY or OPENAI_API_KEY)")
	}

	// Memory.
	dbPath := filepath.Join(cfg.DataDir, "overhuman.db")
	ltm, err := memory.NewLongTermMemory(dbPath)
	if err != nil {
		return pipeline.Dependencies{}, nil, fmt.Errorf("long-term memory: %w", err)
	}
	log.Printf("[bootstrap] long-term memory: %s", dbPath)

	pt, err := memory.NewPatternTracker(ltm.DB())
	if err != nil {
		ltm.Close()
		return pipeline.Dependencies{}, nil, fmt.Errorf("pattern tracker: %w", err)
	}
	log.Printf("[bootstrap] pattern tracker ready")

	stm := memory.NewShortTermMemory(100)

	// Brain.
	router := brain.NewModelRouter()
	router.SetProvider(providerName)
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
	}

	log.Printf("[bootstrap] all subsystems ready")
	return deps, reflEngine, nil
}

// runCLI starts the agent in interactive CLI mode.
func runCLI() {
	cfg := loadConfig()
	deps, _, err := bootstrap(cfg)
	if err != nil {
		log.Fatalf("[cli] bootstrap: %v", err)
	}

	p := pipeline.New(deps)
	cli := senses.NewCLISense(os.Stdin, os.Stdout)

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
		}
	}
}

// runDaemon starts the full daemon with HTTP API and heartbeat timer.
func runDaemon() {
	cfg := loadConfig()
	deps, _, err := bootstrap(cfg)
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

	log.Printf("[daemon] %s v%s started", cfg.AgentName, version)

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
			}
		}
	}()

	// Wait for shutdown signal.
	<-sigCh
	log.Printf("[daemon] shutting down...")
	cancel()

	// Graceful shutdown.
	api.Stop()
	deps.LongTerm.Close()
	log.Printf("[daemon] shutdown complete")
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
