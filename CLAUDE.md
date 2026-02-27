# Overhuman — Claude Code Project Guide

## What is this

Self-evolving universal assistant daemon in Go. Learns from repetition, auto-generates code-skills to replace expensive LLM calls, and continuously improves through 4-level reflection.

## Quick commands

```bash
go test ./...                          # Run all tests (526 tests, 18 packages)
go run ./cmd/overhuman/ cli            # Interactive CLI mode
go run ./cmd/overhuman/ start          # Daemon mode (HTTP :9090)
go run ./cmd/overhuman/ status         # Health check
go build -o overhuman ./cmd/overhuman/ # Build binary
```

## Architecture

10-stage pipeline: Intake → Clarify → Plan → Agent Select → Execute → Review → Memory → Patterns → Reflect → Goals

```
internal/
├── soul/          # Agent identity (markdown DNA, versioned, immutable anchors)
├── agent/         # Agent model, fractal hierarchy (registry, factory)
├── pipeline/      # 10-stage orchestrator + DAG executor
├── brain/         # LLM integration (Claude, OpenAI), model routing, context assembly
├── senses/        # Input adapters (CLI, HTTP API, Webhook, Telegram, Slack, Discord, Email)
├── instruments/   # Skill system (LLM/Code/Hybrid), code generator, Docker sandbox, subagent manager
├── memory/        # Short-term + Long-term (SQLite FTS5) + Patterns + SKB
├── reflection/    # Micro + Meso + Macro + Mega (4-level reflection)
├── evolution/     # Fitness scoring, A/B testing, deprecation, experimentation
├── observability/ # Structured logging (slog) + metrics collector
├── goals/         # Proactive goal engine
├── budget/        # Cost tracking, daily/monthly limits
├── versioning/    # Observation windows, auto-rollback
├── mcp/           # MCP client, server registry, skill bridge (JSON-RPC 2.0)
├── storage/       # Persistent KV store (SQLite-backed, FTS5 search, TTL)
├── skills/        # 20 starter skills (dev, file, automation, knowledge, stubs)
└── security/      # Sanitizer, audit logger, encryption (AES-256-GCM), skill validator, policy enforcer
```

## Key patterns

- **Nil-safe dependencies**: Phase 2/3 deps in pipeline are optional (nil checks everywhere)
- **Lock-free inner functions**: Public methods lock, call lock-free `computeXxx()` internally (see evolution engine)
- **Thread safety**: All registries use `sync.RWMutex`
- **Running averages**: Skill metrics use incremental mean (no full recompute)
- **Mock LLM in tests**: `httptest.Server` returning Claude API format responses

## Environment variables

```
ANTHROPIC_API_KEY  — Claude API key (primary)
OPENAI_API_KEY     — OpenAI fallback
OVERHUMAN_DATA     — Data directory (default: ~/.overhuman)
OVERHUMAN_API_ADDR — API listen address (default: 127.0.0.1:9090)
OVERHUMAN_NAME     — Agent name (default: Overhuman)
```

## Dependencies

Minimal: only `google/uuid` and `modernc.org/sqlite` (pure Go SQLite).

## Implementation status

Full spec: `docs/SPEC.md`
Phase tracking: `docs/PHASES.md`

### Completed
- **Phase 1**: Soul, Agent, Pipeline, Brain (Claude+OpenAI), Senses (CLI+API+Webhook+Timer), Memory (short+long+patterns), Meso-reflection
- **Phase 2**: Skill system (LLM/Code/Hybrid), Code generator, GoalEngine, Budget tracker, Channel adapters (Telegram+Slack+Discord+Email)
- **Phase 3**: Evolution engine (fitness+A/B+deprecation), Version control (observation+rollback), Macro-reflection, DAG executor, Pipeline integration
- **Phase 4**: SKB, Mega-reflection, Micro-reflection, Experimentation (Welch's t-test), Docker sandbox, Observability (slog+metrics), Pipeline Phase 4 integration
- **Infrastructure**: MCP client+registry+bridge (JSON-RPC 2.0), Storage abstraction (SQLite KV+FTS5)

- **Starter Skills**: 20 skills in 5 categories (dev, communication, research, files, automation) — `internal/skills/`
- **Fractal Agents**: Registry, Factory (spawn/retire/promote), SubagentManager (delegate/fan-out/best-of-N), Pipeline integration
- **Security Architecture**: Input sanitizer (prompt injection), audit logger, AES-256-GCM encryption, skill validator (manifests+signatures), policy enforcer, rate limiter

### Next
- Real IMAP/SMTP implementation for Email adapter
