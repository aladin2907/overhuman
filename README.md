<p align="center">
  <img src="docs/assets/banner.svg" alt="Overhuman — Self-evolving AI daemon with fully generative UI" width="800">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go 1.25">&ensp;
  <img src="https://img.shields.io/badge/tests-981-success?style=for-the-badge" alt="Tests">&ensp;
  <img src="https://img.shields.io/badge/deps-3-blue?style=for-the-badge" alt="Dependencies">&ensp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" alt="MIT License"></a>
</p>

<br>

<p align="center">
  <strong>Overhuman</strong> is an always-on AI assistant daemon that processes tasks through a 10-stage pipeline<br>
  and <strong>generates a unique visual interface for every response</strong> — not from a component catalog, but from scratch.<br>
  It connects to any channel you already use, remembers everything, learns from repetition,<br>
  and gets cheaper over time by auto-generating code skills that replace LLM calls.
</p>

<p align="center">
  <em>If you want a personal AI daemon that thinks, learns, and renders — this is it.</em>
</p>

<br>

<p align="center">
  <a href="docs/SPEC.md">Spec</a> ·
  <a href="docs/SPEC_DYNAMIC_UI.md">GenUI Spec</a> ·
  <a href="docs/ARCHITECTURE.md">Architecture</a> ·
  <a href="docs/PHASES.md">Phases</a> ·
  <a href="#-quick-start">Quick Start</a>
</p>

<br>

<p align="center">
  <img src="docs/assets/architecture.svg" alt="Architecture: Input → 10-Stage Pipeline → Generative UI" width="800">
</p>

---

## 🎨 Generative UI — The Core Feature

Most AI assistants return plain text. Some use pre-built component catalogs. Overhuman takes a fundamentally different approach:

> **The LLM generates complete, self-contained UI code for every response.**

```
 "Analyze server logs"  →  Interactive dashboard with latency charts, error heatmap, filterable table
 "Compare Q1 vs Q2"     →  Side-by-side cards with sparklines and delta highlights
 "Draft an email"       →  Rich editor with tone slider and preview pane
 "Explain this code"    →  Syntax-highlighted walkthrough with collapsible sections
```

No component registry. No JSON schema. The agent decides the best visualization — charts, tables, forms, games, timelines, card grids — whatever fits the data.

<br>

### How it compares

| Approach | Who does it | Agent freedom | Overhuman |
|----------|-------------|---------------|:---------:|
| **Level 1 — Controlled** | Vercel AI SDK, AG-UI | Agent picks from developer-built components | — |
| **Level 2 — Declarative** | Google A2UI, Yandex DivKit | Agent outputs JSON, client renders from catalog | — |
| **Level 3 — Fully Generated** | Gemini Dynamic View, Claude Artifacts | Agent writes full UI code from scratch | **✓** |

> [!NOTE]
> Level 1-2 limit the agent to what a developer pre-built. Level 3 means **infinite UI surface** — the agent can create any visualization it can imagine. The tradeoff is sandboxing (solved) and non-determinism (solved via self-healing + reflection).

<br>

### Three rendering targets

| Target | Format | Use case |
|--------|--------|----------|
| 🖥️ **Terminal** | ANSI escape codes + box drawing | CLI over SSH, no browser needed |
| 🌐 **Browser** | HTML + CSS + JS via WebSocket | Sandboxed iframe — no network, no data leak |
| 📺 **Kiosk** | Full-screen SPA on dedicated screen | Companion display for tablet / wall mount / desktop |

<br>

### Kiosk: the companion display

The Kiosk is a full-screen web app designed for a dedicated screen — tablet on your desk, monitor on the wall, or a browser window you keep open. It connects via WebSocket and shows:

| Component | Description |
|-----------|-------------|
| **Pipeline HUD** | Real-time progress through all 10 stages as tasks execute |
| **Generated UI** | Each response rendered as a rich HTML app inside a sandboxed iframe |
| **Neural canvas** | Animated particle background that reacts to pipeline activity |
| **Agent status ring** | Visual heartbeat of the daemon |
| **Metrics panel** | Tasks processed, skills generated, memory entries |
| **Theme system** | Sci-fi (default) · cyberpunk · clean — via CSS custom properties |
| **Sound engine** | Synthesized audio feedback via Web Audio API (zero external files) |
| **CRT mode** | Scanline overlay + text glow for the retro aesthetic |

> [!TIP]
> Device-adaptive rendering: phone strips down to essentials (no HUD, overlay sidebar), tablet becomes a control pad, desktop is a full command center.

<br>

### Self-healing and reflection

```
LLM generates HTML → Render in sandbox → Error?
                                           ├─ Yes → Feed error back to LLM → Retry (max 2)
                                           │         Still broken? → Fall back to plain text
                                           └─ No  → Track user interactions
                                                     └─ What they click, scroll past, ignore
                                                        └─ Feed back into UI generation prompt
                                                           └─ Next UI is better
```

> UI cost: ~$0.001 per generation (gpt-4.1-nano). Skipped for short text answers.

<br>

### Why this matters

The industry is converging on AI companions — ambient displays, smart glasses, wearable pendants — all needing a software brain that renders dynamic UI on any surface. Overhuman's architecture — a central daemon that pushes generated UI to any connected client via WebSocket — is exactly this pattern, running today, in pure Go with 3 dependencies.

---

## ⚡ Features

| | Feature | Description |
|-|---------|-------------|
| 📡 | **6 Channels** | CLI · Telegram · Slack · Discord · Email · HTTP API — one brain behind all |
| 🧠 | **Memory** | Short-term (dialog) + long-term (SQLite FTS5) + pattern tracking |
| 🔄 | **Self-learning** | 3x repeat → auto code skill → LLM replaced with deterministic code |
| 🤖 | **Any model** | OpenAI · Claude · Ollama · Groq · Together · OpenRouter + any compatible |
| 🌳 | **Fractal agents** | Tree hierarchy, delegation, best-of-N competitions, per-agent memory |
| 🪞 | **4-level reflection** | Micro → Meso → Macro → Mega self-improvement loop |
| 🛠️ | **20 skills** | Code gen, search, translate, summarize, email + 9 stubs for external services |
| 🔐 | **Security** | AES-256-GCM · prompt injection protection · audit trail · code sandbox |
| ⏰ | **Always on** | OS service (launchd/systemd) · heartbeat every 30 min · proactive goals |
| 🔌 | **MCP tools** | Model Context Protocol client for external tool integration |

---

## 🚀 Quick Start

```bash
# Build
go build -o overhuman ./cmd/overhuman/

# Configure (interactive wizard — provider selection, API key, model)
./overhuman configure

# Start in chat mode
./overhuman cli

# Or as a daemon with HTTP API + WebSocket + Kiosk UI
./overhuman start
```

On first run, `overhuman cli` will automatically prompt the setup wizard if no keys are configured.

> [!TIP]
> **Zero-config local mode** — no API key needed:
> ```bash
> LLM_PROVIDER=ollama ./overhuman cli
> ```

---

## 🖥️ Deployment

```bash
overhuman doctor       # diagnostics — check config, connection, database
overhuman install      # install as OS service (launchd on macOS, systemd on Linux)
overhuman status       # check if daemon is running
overhuman stop         # graceful shutdown (SIGTERM)
overhuman logs         # tail last 50 lines of log
overhuman update       # check & apply updates (SHA256 verified)
overhuman uninstall    # remove OS service
```

The daemon opens 3 ports (configurable via `OVERHUMAN_API_ADDR`):

| Port | Service | Description |
|:----:|---------|-------------|
| `9090` | **HTTP API** | REST API for integrations (`/input`, `/input/sync`, `/health`) |
| `9091` | **WebSocket** | Real-time UI streaming (RFC 6455, pure stdlib) |
| `9092` | **Kiosk** | Full-screen web app (tablet/desktop companion display) |

> File drop: place files in `~/.overhuman/inbox/` — the daemon picks them up automatically.
> Logs: stdout + `~/.overhuman/logs/overhuman.log`.

---

## 🧩 Supported LLMs

| Provider | API Key | Models |
|----------|:-------:|--------|
| **OpenAI** | Required | o3, o4-mini, GPT-4.1 |
| **Anthropic Claude** | Required | Claude Sonnet, Haiku, Opus |
| **Ollama** | — | Local models — llama3, mistral, etc. Free |
| **LM Studio** | — | Local models via GUI |
| **Groq** | Required | Fast inference — Llama, DeepSeek, Qwen |
| **Together AI** | Required | Open-source models hosted |
| **OpenRouter** | Required | All models through a single key |
| **Custom** | Optional | Any OpenAI-compatible server |

The configure wizard fetches available models directly from provider APIs. Config is stored in `~/.overhuman/config.json` (permissions 600). Environment variables override config — useful for Docker/CI.

---

## 🔬 How It Works

<details>
<summary><strong>10-Stage Pipeline</strong> — every request goes through a full processing cycle</summary>

<br>

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

</details>

<details>
<summary><strong>Self-Learning</strong> — recurring tasks get compiled into code</summary>

<br>

Overhuman tracks recurring tasks via fingerprinting. When a pattern repeats K times (default 3):

1. Generates a code skill based on accumulated examples
2. Registers it as a deterministic alternative to LLM calls
3. On the next occurrence, uses code instead of LLM
4. If the code breaks — automatic fallback to LLM

Result: each cycle makes the system cheaper (code vs API), faster (ms vs seconds), and more reliable (determinism vs stochastic).

</details>

<details>
<summary><strong>4 Levels of Reflection</strong> — self-improvement at every scale</summary>

<br>

| Level | When | What it does |
|-------|------|-------------|
| **Micro** | Each pipeline step | Adjusts the next step |
| **Meso** | After each task | Updates memory, skills, patterns |
| **Macro** | Every N tasks | Reevaluates strategies and goals |
| **Mega** | Rarely | Evaluates the reflection process itself |

</details>

<details>
<summary><strong>Fractal Agents</strong> — tree of specialized sub-agents</summary>

<br>

Agents form a tree. A parent creates specialized children (coder, reviewer, researcher), delegates tasks, runs competitions (best-of-N), and fires/promotes based on results. Each agent has its own identity, memory, and skills.

</details>

---

## 🌐 HTTP API

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

---

## ⚙️ Configuration

<details>
<summary>Environment variables (override config.json)</summary>

<br>

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

</details>

---

## 🏗️ Technical Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Language | **Go** | Daemon-first, goroutines, single binary 15MB, <10MB RAM |
| Storage | **SQLite + files** | Self-contained, human-readable, FTS5 for search |
| Dependencies | **3 total** | `google/uuid`, `modernc.org/sqlite`, `golang.org/x/term` |
| Tools | **MCP** | Industry standard (Anthropic + OpenAI + Google + Microsoft) |
| Sandbox | **Docker** | Isolation for auto-generated code |
| Encryption | **AES-256-GCM** | Authenticated encryption for keys |

---

## 📁 Project Structure

<details>
<summary>21 packages, single binary</summary>

<br>

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
├── genui/           — generative UI (LLM → ANSI/HTML, self-healing, reflection)
├── deploy/          — PID management, OS service templates, auto-update
├── skills/          — 20 starter skills
└── observability/   — structured logs and metrics
```

</details>

---

## 🧪 Tests

```bash
go test ./...         # 981 tests, 21 packages
go test ./... -race   # race condition checks
```

All tests run with a mock LLM server — no API keys needed.

---

## 📚 Docs

| Document | Description |
|----------|-------------|
| [`docs/SPEC.md`](docs/SPEC.md) | Full specification (700+ lines) |
| [`docs/SPEC_DYNAMIC_UI.md`](docs/SPEC_DYNAMIC_UI.md) | Generative UI specification (1186 lines) |
| [`docs/PHASES.md`](docs/PHASES.md) | Implementation tracker |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture overview |

---

<p align="center">
  <sub>MIT License · Built with Go · 3 dependencies · Zero JS frameworks</sub>
</p>
