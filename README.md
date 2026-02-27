# Overhuman

**An AI assistant that learns and gets cheaper with every request.**

Overhuman is a daemon process that accepts tasks from any channel (CLI, HTTP API, Telegram, Slack, Discord, Email), executes them via LLM, remembers results, and automatically generates code for recurring tasks. Over time, expensive LLM calls get replaced by deterministic code — faster, cheaper, more reliable.

```
Tasks → LLM executes → Reflection → Patterns → Code skills
  ↑                                                    ↓
  └──── Cheaper, faster, more reliable ◄──── Code replaces LLM
```

## Features

**Universal assistant** — not just for code. Marketing, planning, data analysis, communications, routine automation.

**Remembers everything** — short-term memory (current dialog) + long-term (SQLite with full-text search) + pattern tracking. No need to repeat yourself.

**Learns from repetition** — if you ask the same thing 3 times, Overhuman automatically generates a code skill and stops spending money on LLM calls for that task.

**Works with any model** — OpenAI, Claude, Ollama (local models), LM Studio, Groq, Together AI, OpenRouter, or any OpenAI-compatible endpoint. Switches with a single command. Models are fetched dynamically from provider APIs.

**Multi-channel** — one brain behind all channels. Task comes in from Telegram, response goes out to Slack. CLI for development, HTTP API for integrations, Email for automation.

**Always on** — runs as a system service. Heartbeat every 30 minutes for proactive tasks and self-improvement.

**Secure** — key encryption (AES-256-GCM), prompt injection protection, full audit trail, skill validation, code sandbox.

## Quick Start

```bash
# Build
go build -o overhuman ./cmd/overhuman/

# Configure (interactive wizard — provider selection, API key, model)
./overhuman configure

# Start in chat mode
./overhuman cli

# Or as a daemon with HTTP API
./overhuman start

# Health check
./overhuman doctor
```

On first run, `overhuman cli` will automatically prompt the setup wizard if no keys are configured.

## Supported LLMs

| Provider | API Key | Models |
|----------|---------|--------|
| **OpenAI** | Required | o3, o4-mini, GPT-4.1 |
| **Anthropic Claude** | Required | Claude Sonnet, Haiku, Opus |
| **Ollama** | Not needed | Local models — llama3, mistral, etc. Free |
| **LM Studio** | Not needed | Local models via GUI |
| **Groq** | Required | Fast inference — Llama, DeepSeek, Qwen |
| **Together AI** | Required | Open-source models hosted |
| **OpenRouter** | Required | All models through a single key |
| **Custom** | Optional | Any OpenAI-compatible server |

The configure wizard fetches available models directly from provider APIs. Config is stored in `~/.overhuman/config.json` (permissions 600). Environment variables override config — useful for Docker/CI.

## How It Works

### 10-Stage Pipeline

Every request goes through a full processing cycle:

1. **Intake** — normalize input from any channel into a unified format
2. **Clarification** — LLM asks clarifying questions if needed
3. **Planning** — decompose the task into subtasks (DAG)
4. **Agent selection** — pick a specialized sub-agent
5. **Execution** — parallel subtask execution
6. **Review** — mandatory quality check of the result
7. **Memory** — save to short-term and long-term memory
8. **Patterns** — track recurring tasks
9. **Reflection** — self-assessment and strategy adjustment
10. **Goals** — update proactive goals

### Self-Learning

Overhuman tracks recurring tasks via fingerprinting. When a pattern repeats K times (default 3), the system:

1. Generates a code skill based on accumulated examples
2. Registers it as a deterministic alternative to LLM calls
3. On the next occurrence, uses code instead of LLM
4. If the code breaks — automatic fallback to LLM

Result: each cycle makes the system cheaper (code vs API), faster (ms vs seconds), and more reliable (determinism vs stochastic).

### 4 Levels of Reflection

| Level | When | What it does |
|-------|------|-------------|
| **Micro** | Each pipeline step | Adjusts the next step |
| **Meso** | After each task | Updates memory, skills, patterns |
| **Macro** | Every N tasks | Reevaluates strategies and goals |
| **Mega** | Rarely | Evaluates the reflection process itself |

### Fractal Agents

Agents form a tree. A parent creates specialized children (coder, reviewer, researcher), delegates tasks, runs competitions (best-of-N), and fires/promotes based on results. Each agent has its own identity, memory, and skills.

## HTTP API

```bash
# Async request (fire-and-forget)
curl -X POST http://localhost:9090/input \
  -H "Content-Type: application/json" \
  -d '{"payload": "Analyze this CSV file", "sender": "user1"}'

# Sync request (waits for response)
curl -X POST http://localhost:9090/input/sync \
  -H "Content-Type: application/json" \
  -d '{"payload": "Translate to French: Hello world"}'

# Health check
curl http://localhost:9090/health
```

## Configuration

Environment variables (override config.json):

```
ANTHROPIC_API_KEY   — Claude key
OPENAI_API_KEY      — OpenAI key
LLM_PROVIDER        — provider: openai, claude, ollama, groq, together, openrouter, custom
LLM_API_KEY         — key for any provider
LLM_MODEL           — default model
LLM_BASE_URL        — URL for custom/ollama
OVERHUMAN_DATA      — data directory (default ~/.overhuman)
OVERHUMAN_API_ADDR  — API address (default 127.0.0.1:9090)
OVERHUMAN_NAME      — agent name
```

## Technical Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Language | **Go** | Daemon-first, goroutines, single binary 15MB, <10MB RAM |
| Storage | **SQLite + files** | Self-contained, human-readable, FTS5 for search |
| Dependencies | **3 total** | `google/uuid`, `modernc.org/sqlite`, `golang.org/x/term` |
| Tools | **MCP** | Industry standard (Anthropic + OpenAI + Google + Microsoft) |
| Sandbox | **Docker** | Isolation for auto-generated code |
| Encryption | **AES-256-GCM** | Authenticated encryption for keys |

## Project Structure

```
cmd/overhuman/       — entry point (daemon, CLI, configure, doctor)
internal/
├── soul/            — agent identity (markdown DNA, versioning)
├── agent/           — fractal agent hierarchy
├── pipeline/        — 10-stage orchestrator + DAG executor
├── brain/           — LLM integration, model routing, context assembly
├── senses/          — input channels (CLI, HTTP, Telegram, Slack, Discord, Email)
├── instruments/     — skill system (LLM/Code/Hybrid), code generator, Docker sandbox
├── memory/          — short-term + long-term memory + patterns + shared knowledge base
├── reflection/      — 4 levels of reflection
├── evolution/       — fitness metrics, A/B testing, skill culling
├── goals/           — proactive goal engine
├── budget/          — cost control, limits, budget-based routing
├── versioning/      — versioning with auto-rollback on degradation
├── security/        — sanitization, audit, encryption, validation
├── mcp/             — MCP client and registry (JSON-RPC 2.0)
├── storage/         — persistent KV store (SQLite, FTS5, TTL)
├── skills/          — 20 starter skills
└── observability/   — structured logs and metrics
```

## Tests

```bash
go test ./...         # 586 tests, 19 packages
go test ./... -race   # Race condition checks
```

All tests run with a mock LLM server — no API keys needed.

## Docs

- `docs/SPEC.md` — full specification (700+ lines)
- `docs/PHASES.md` — implementation tracker
- `docs/ARCHITECTURE.md` — architecture

## License

MIT
