# Overhuman

**Self-evolving universal assistant daemon in Go.**

Learns from repetition, auto-generates code-skills to replace expensive LLM calls, and continuously improves through 4-level reflection.

```
Tasks → LLM executes → Reflection → Patterns → Code-skills
  ↑                                                  ↓
  └──── Cheaper, faster, more reliable ◄──── Code replaces LLM
```

## Why

Current AI assistants are reactive chatbots or one-shot code generators. They don't learn between sessions, don't improve over time, and don't reduce costs through automation. Overhuman closes that gap:

- **Memory across sessions** — short-term + long-term + pattern tracking
- **Self-improvement** — 4 nested reflection loops (micro → meso → macro → mega)
- **LLM→Code flywheel** — repeated tasks auto-generate deterministic code-skills
- **Darwinian selection** — skills compete via fitness metrics, A/B testing, auto-deprecation
- **Multi-channel** — one brain behind CLI, HTTP API, Telegram, Slack, Discord, Email, Webhooks
- **Fractal agents** — hierarchical sub-agents with own identity, task delegation, competitive execution
- **Security-first** — prompt injection defense, encrypted credentials, audit trail, skill validation

## Architecture

10-stage pipeline: **Intake → Clarify → Plan → Agent Select → Execute → Review → Memory → Patterns → Reflect → Goals**

```
internal/
├── soul/          # Agent identity (markdown DNA, versioned, immutable anchors)
├── agent/         # Fractal agent hierarchy (registry, factory, spawn/retire/promote)
├── pipeline/      # 10-stage orchestrator + DAG executor
├── brain/         # LLM integration (Claude, OpenAI), model routing, context assembly
├── senses/        # Input adapters (CLI, HTTP, Telegram, Slack, Discord, Email, Webhook)
├── instruments/   # Skills (LLM/Code/Hybrid), code generator, Docker sandbox, subagent manager
├── memory/        # Short-term + Long-term (SQLite FTS5) + Patterns + Shared Knowledge Base
├── reflection/    # Micro + Meso + Macro + Mega (4-level reflection)
├── evolution/     # Fitness scoring, A/B testing, deprecation, experimentation (Welch's t-test)
├── observability/ # Structured logging (slog) + metrics collector (ring buffer, percentiles)
├── goals/         # Proactive goal engine
├── budget/        # Cost tracking, daily/monthly limits, model routing by budget
├── versioning/    # Observation windows, auto-rollback on degradation
├── mcp/           # MCP client + registry + bridge (JSON-RPC 2.0, industry standard)
├── storage/       # Persistent KV store (SQLite, WAL, FTS5, TTL)
├── skills/        # 20 starter skills (dev, file ops, automation, knowledge, stubs)
└── security/      # Sanitizer, audit, AES-256-GCM encryption, skill validator, policy enforcer
```

## Quick Start

```bash
# Build
go build -o overhuman ./cmd/overhuman/

# First-time setup — interactive wizard for API keys
./overhuman configure

# Interactive CLI
./overhuman cli

# Daemon mode (HTTP API on :9090)
./overhuman start

# Health check
./overhuman status

# Diagnose configuration issues
./overhuman doctor
```

On first run, `overhuman cli` or `overhuman start` will detect missing configuration and offer to run the setup wizard automatically.

### Supported LLM Providers

| Provider | API Key Required | Command |
|----------|-----------------|---------|
| **OpenAI** | Yes | `overhuman configure` → select OpenAI |
| **Anthropic Claude** | Yes | `overhuman configure` → select Claude |
| **Ollama** (local) | No | `overhuman configure` → select Ollama |
| **LM Studio** (local) | No | `overhuman configure` → select LM Studio |
| **Groq** | Yes | `overhuman configure` → select Groq |
| **Together AI** | Yes | `overhuman configure` → select Together |
| **OpenRouter** | Yes | `overhuman configure` → select OpenRouter |
| **Custom endpoint** | Optional | Any OpenAI-compatible API |

Configuration is stored in `~/.overhuman/config.json` (chmod 600). Environment variables override the config file for CI/Docker use.

### Environment Variables (optional override)

```
ANTHROPIC_API_KEY   — Claude API key
OPENAI_API_KEY      — OpenAI API key
LLM_PROVIDER        — Provider override (openai, claude, ollama, etc.)
LLM_API_KEY         — API key for any provider
LLM_MODEL           — Default model override
LLM_BASE_URL        — Custom API base URL
OVERHUMAN_DATA      — Data directory (default: ~/.overhuman)
OVERHUMAN_API_ADDR  — API listen address (default: 127.0.0.1:9090)
OVERHUMAN_NAME      — Agent name (default: Overhuman)
```

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Language | **Go** | Daemon-first (goroutines), single binary, <10MB RAM, 100k+ RPS ceiling |
| Storage | **SQLite + files** | Self-contained, human-readable, git-versionable, FTS5 for search |
| Dependencies | **2 total** | `google/uuid` + `modernc.org/sqlite` (pure Go). Zero C dependencies |
| Tool protocol | **MCP** | Industry standard (Anthropic + OpenAI + Google + Microsoft) |
| Sandbox | **Docker** | Container isolation, --cap-drop=ALL, --read-only, resource limits |
| Encryption | **AES-256-GCM** | Authenticated encryption for credentials at rest |

## Features in Detail

### LLM→Code Flywheel
When the agent detects a repeating task pattern (via fingerprinting), it generates a deterministic code-skill to replace the LLM call. Each cycle makes the system cheaper (code vs API call), faster (ms vs seconds), and more reliable (deterministic vs stochastic).

### 4-Level Reflection
| Loop | When | Question | Changes |
|------|------|----------|---------|
| **Micro** | Every pipeline step | "Did this step succeed?" | Adjusts next step |
| **Meso** | After each run | "Was the task solved well?" | Memory, skills, patterns |
| **Macro** | Every N runs | "Are my strategies adequate?" | Soul, thresholds, goals |
| **Mega** | Rare / triggered | "Is my reflection process working?" | The reflection process itself |

### Fractal Agents
Agents form a tree hierarchy. A parent can spawn specialized children (coder, reviewer, researcher), delegate tasks via fan-out or best-of-N competition, and retire/promote children based on performance. Each agent has its own identity (soul), memory, and skill set, while sharing knowledge through the Shared Knowledge Base (SKB).

### Security Architecture
- **Prompt injection defense**: 14 regex patterns detecting common attack vectors
- **Input sanitization**: UTF-8 validation, control character stripping, length limits, blocklists
- **Credential encryption**: AES-256-GCM with SHA-256 key derivation and random nonces
- **Skill validation**: Manifests with SHA-256 signatures, resource limits, trusted author lists
- **Audit trail**: Append-only log with 14 event types and severity levels
- **Policy enforcement**: Concurrent run limits, forbidden tools, approval gates
- **Output masking**: Automatic secret detection and masking in pipeline output

### 20 Starter Skills
Fully implemented: Code Execution, Git Management, Testing & QA, File Operations, Data Analysis, Knowledge Base Search, API Integration, Web Search, Scheduled Tasks, Audit & Logging, Credential Management.

Stubs (need external services): Browser Automation, Database Query, Email Management, Calendar Integration, Messaging, Document Collaboration, PDF Analysis, Data Aggregation, Real-time Monitoring.

## Testing

```bash
go test ./...         # 586 tests across 19 packages
go test ./... -v      # Verbose output
go test ./... -race   # Race condition detection
```

All tests use mocked LLM responses (`httptest.Server`) and mock transports — no API keys needed to run the test suite. Includes 11 end-to-end integration tests covering the full pipeline flow.

## Project Stats

| Metric | Value |
|--------|-------|
| Go files | 100+ |
| Lines of Go | ~27,000 |
| Tests | 586 |
| Packages | 19 |
| External deps | 3 (`google/uuid`, `modernc.org/sqlite`, `x/term`) |
| Test coverage | All packages |

## Implementation Status

- [x] **Phase 1**: Soul, Agent, Pipeline, Brain, Senses, Memory, Meso-reflection
- [x] **Phase 2**: Skill system, Code generator, GoalEngine, Budget, Channel adapters
- [x] **Phase 3**: Evolution engine, Version control, Macro-reflection, DAG executor
- [x] **Phase 4**: SKB, Mega-reflection, Micro-reflection, Experimentation, Docker sandbox, Observability
- [x] **Infrastructure**: MCP (JSON-RPC 2.0), Storage (SQLite KV + FTS5)
- [x] **Starter Skills**: 20 skills (11 full implementations, 9 stubs)
- [x] **Fractal Agents**: Registry, Factory, SubagentManager, Pipeline integration
- [x] **Security**: Sanitizer, Audit, Encryption, Validator, Policy Enforcer
- [x] **Real IMAP/SMTP**: Full email adapter (IMAP4rev1 client + SMTP sender, stdlib only)
- [x] **Universal Provider**: Any OpenAI-compatible endpoint (Ollama, LM Studio, Groq, Together, OpenRouter, custom)
- [x] **Interactive Setup**: `overhuman configure` wizard, first-run detection, `overhuman doctor`
- [x] **E2E Tests**: 11 integration tests covering full pipeline, HTTP API, concurrency, error recovery

## Spec & Docs

- `docs/SPEC.md` — Full conceptual specification (717 lines)
- `docs/PHASES.md` — Phase-by-phase implementation tracker
- `docs/ARCHITECTURE.md` — Architecture overview

## License

MIT
