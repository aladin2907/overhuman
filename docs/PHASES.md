# Overhuman — Phase Tracker

> Full spec: [SPEC.md](./SPEC.md)

---

## Phase 1: Minimal Living Agent ✅ COMPLETE

**Goal:** Accept text → execute via LLM → save → learn

| Component | Package | Status | Tests |
|-----------|---------|--------|-------|
| Soul (identity, versioning, anchors) | `internal/soul/` | ✅ | ✅ |
| Agent model (all fields from spec 7) | `internal/agent/` | ✅ | ✅ |
| UnifiedInput | `internal/senses/unified.go` | ✅ | ✅ |
| Senses: CLI + HTTP API + Timer | `internal/senses/` | ✅ | ✅ |
| LLMProvider: Claude + OpenAI | `internal/brain/` | ✅ | ✅ |
| Pipeline (10 stages, sequential) | `internal/pipeline/` | ✅ | ✅ |
| Memory: Short-term (ring buffer) | `internal/memory/shortterm.go` | ✅ | ✅ |
| Memory: Long-term (SQLite + FTS5) | `internal/memory/longterm.go` | ✅ | ✅ |
| Pattern Tracker (fingerprinting) | `internal/memory/patterns.go` | ✅ | ✅ |
| Meso-reflection | `internal/reflection/engine.go` | ✅ | ✅ |

---

## Phase 2: Channels + Automation ✅ COMPLETE

**Goal:** Multi-channel input + repeating tasks → code-skill

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| Telegram Sense | `internal/senses/telegram.go` | ✅ | ✅ | Bot API long-polling + webhook, whitelist |
| Slack Sense | `internal/senses/slack.go` | ✅ | ✅ | Events API webhook, URL verification, bot filter |
| Discord Sense | `internal/senses/discord.go` | ✅ | ✅ | Interactions endpoint, slash commands |
| Email Sense | `internal/senses/email.go` | ✅ | ✅ | Real IMAP4rev1 + SMTP (stdlib only) |
| Webhook Sense | `internal/senses/webhook.go` | ✅ | ✅ | |
| Skill System (LLM/Code/Hybrid) | `internal/instruments/skill.go` | ✅ | ✅ | |
| Code Generator | `internal/instruments/generator.go` | ✅ | ✅ | |
| GoalEngine | `internal/goals/engine.go` | ✅ | ✅ | |
| Heartbeat (timer) | `internal/senses/` | ✅ | ✅ | Via UnifiedInput timer |

---

## Phase 3: Evolution + Scaling ✅ COMPLETE

**Goal:** Competitive skill selection, budget, parallelism

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| Evolution Engine (fitness + A/B + deprecation) | `internal/evolution/` | ✅ | ✅ | Fixed deadlock in EvaluateABTest |
| Version Control (observation + rollback) | `internal/versioning/` | ✅ | ✅ | Fixed missing RollbackData |
| Budget Engine (per-task + global) | `internal/budget/` | ✅ | ✅ | |
| Macro-reflection | `internal/reflection/macro.go` | ✅ | ✅ | |
| DAG Executor (goroutines) | `internal/pipeline/dag.go` | ✅ | ✅ | Parallel subtask execution |
| Pipeline Phase 3 integration | `internal/pipeline/pipeline.go` | ✅ | ✅ | Evolution, reflection engine, version control, DAG |

---

## Phase 4: Maturity ✅ COMPLETE

**Goal:** Full autonomy, inter-agent experience, security

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| SKB (Shared Knowledge Base) | `internal/memory/skb.go` | ✅ | ✅ | SQLite-backed, propagation up/down/horizontal |
| Mega-reflection | `internal/reflection/mega.go` | ✅ | ✅ | Reflection on the reflection process |
| Micro-reflection | `internal/reflection/micro.go` | ✅ | ✅ | Per-step quality checks (clarify, execute, review) |
| Experimentation | `internal/evolution/experiment.go` | ✅ | ✅ | Hypothesis-driven A/B with Welch's t-test |
| Docker Sandbox | `internal/instruments/docker.go` | ✅ | ✅ | Container isolation, resource limits, multi-language |
| Observability | `internal/observability/` | ✅ | ✅ | Structured logging (slog) + metrics collector |
| Pipeline Phase 4 integration | `internal/pipeline/pipeline.go` | ✅ | ✅ | Micro-reflection, SKB, metrics, structured logging |

---

## Infrastructure (cross-cutting)

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| MCP Protocol (JSON-RPC 2.0) | `internal/mcp/protocol.go` | ✅ | ✅ | Types, tool definitions, content blocks |
| MCP Client | `internal/mcp/client.go` | ✅ | ✅ | Stdio transport, initialize, discover, call |
| MCP Server Registry | `internal/mcp/registry.go` | ✅ | ✅ | Multi-server management, connect/disconnect |
| MCP Skill Bridge | `internal/mcp/bridge.go` | ✅ | ✅ | MCP tools ↔ SkillExecutor, LLM format conversion |
| Storage Interface | `internal/storage/storage.go` | ✅ | ✅ | KV store with metadata, TTL, FTS search |
| SQLite Storage | `internal/storage/sqlite.go` | ✅ | ✅ | WAL mode, FTS5, upsert, prefix listing |

---

## Starter Skills (spec §13) ✅ COMPLETE

**Goal:** 20 skills in 5 categories — dev, communication, research, files, automation

| # | Skill | Type | Package | Status | Tests |
|---|-------|------|---------|--------|-------|
| 1 | Code Execution | Full | `internal/skills/code_exec.go` | ✅ | ✅ |
| 2 | Git Management | Full | `internal/skills/code_exec.go` | ✅ | ✅ |
| 3 | Testing & QA | Full | `internal/skills/code_exec.go` | ✅ | ✅ |
| 4 | Browser Automation | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 5 | Database Query | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 6 | Email Management | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 7 | Calendar Integration | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 8 | Messaging | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 9 | Document Collaboration | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 10 | Web Search | Full | `internal/skills/automation.go` | ✅ | ✅ |
| 11 | PDF & Document Analysis | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 12 | Data Aggregation | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 13 | Real-time Monitoring | Stub | `internal/skills/stubs.go` | ✅ | ✅ |
| 14 | File Operations | Full | `internal/skills/file_ops.go` | ✅ | ✅ |
| 15 | Data Analysis | Full | `internal/skills/file_ops.go` | ✅ | ✅ |
| 16 | Knowledge Base Search | Full | `internal/skills/knowledge.go` | ✅ | ✅ |
| 17 | API Integration | Full | `internal/skills/automation.go` | ✅ | ✅ |
| 18 | Scheduled Tasks | Full | `internal/skills/automation.go` | ✅ | ✅ |
| 19 | Audit & Logging | Full | `internal/skills/automation.go` | ✅ | ✅ |
| 20 | Credential Management | Full | `internal/skills/automation.go` | ✅ | ✅ |

**Full implementations:** 11 skills (code exec, git, testing, web search, file ops, data analysis, knowledge search, API integration, scheduler, audit, credentials)
**Stubs (need external services):** 9 skills (browser, DB, email mgmt, calendar, messaging, docs collab, PDF, data aggregation, monitoring)

---

## Fractal Agents (spec §3, §6.6) ✅ COMPLETE

**Goal:** Hierarchical sub-agents with own soul, task delegation, and knowledge propagation

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| Agent Registry | `internal/agent/registry.go` | ✅ | ✅ | Thread-safe, hierarchy queries (children, parent, lineage, roots, descendants) |
| Agent Factory | `internal/agent/factory.go` | ✅ | ✅ | SpawnChild, SpawnRoot, RetireChild (recursive), Promote, inheritance |
| SubagentManager | `internal/instruments/subagent.go` | ✅ | ✅ | Delegate, DelegateAsync, FanOut, BestOfN, Cancel, Cleanup, Stats |
| Pipeline Integration | `internal/pipeline/pipeline.go` | ✅ | ✅ | Stage 4 agent routing, Stage 5 subagent execution with LLM fallback |

**Key features:**
- Parent-child hierarchy with unlimited depth (fractal)
- Property inheritance: skillset, tool access, policies, LLM config
- Task delegation: sync, async, fan-out (parallel), best-of-N (competitive)
- Lifecycle: spawn → delegate → retire (recursive) or promote (become independent)
- SKB integration for inter-agent knowledge propagation (already in Phase 4)

---

## Security Architecture (spec §6.8) ✅ COMPLETE

**Goal:** Defense-in-depth security: sanitization, audit, encryption, validation, policy enforcement

| Component | Package | Status | Tests | Notes |
|-----------|---------|--------|-------|-------|
| Input Sanitizer | `internal/security/sanitizer.go` | ✅ | ✅ | Prompt injection detection (14 patterns), UTF-8 validation, control char stripping, blocklist |
| Rate Limiter | `internal/security/sanitizer.go` | ✅ | ✅ | Per-source sliding window, configurable limit/interval, cleanup |
| Audit Logger | `internal/security/audit.go` | ✅ | ✅ | Append-only, 14 event types, severity levels, in-memory store, filter queries |
| Credential Encryption | `internal/security/encryption.go` | ✅ | ✅ | AES-256-GCM, SHA-256 key derivation, random nonces, backward compat |
| Secret Masking | `internal/security/encryption.go` | ✅ | ✅ | MaskSecret, MaskInString, SecretRegistry for output sanitization |
| Skill Validator | `internal/security/validator.go` | ✅ | ✅ | Manifest validation, SHA-256 signatures, resource limits, blocklist, trusted authors |
| Policy Enforcer | `internal/security/validator.go` | ✅ | ✅ | Concurrent run limits, forbidden tools, approval gates, acquire/release tracking |
| Pipeline Integration | `internal/pipeline/pipeline.go` | ✅ | ✅ | Pre-stage sanitization, output secret masking, audit logging hooks |

**Key security features:**
- Prompt injection detection: 14 regex patterns for common attack vectors
- Credential encryption: AES-256-GCM with random nonces (different ciphertext each time)
- Skill validation: manifest-based, SHA-256 code signatures, resource limit enforcement
- Audit trail: immutable append-only log with 14 event types and severity levels
- Output sanitization: automatic masking of registered secrets in pipeline output
- Rate limiting: per-source sliding window to prevent abuse
- Policy enforcement: MaxConcurrentRuns, ForbiddenTools, RequireApproval

---

## Real IMAP/SMTP Email Adapter ✅ COMPLETE

**Goal:** Replace placeholder email adapter with real IMAP fetching and SMTP sending using only Go stdlib

| Component | File | Status | Tests | Notes |
|-----------|------|--------|-------|-------|
| IMAP Client | `internal/senses/imap.go` | ✅ | ✅ | Minimal IMAP4rev1: LOGIN, SELECT, SEARCH UNSEEN, FETCH, STORE +FLAGS, LOGOUT |
| SMTP Sender | `internal/senses/smtp.go` | ✅ | ✅ | STARTTLS, PlainAuth, RFC 2822 headers, multi-recipient |
| Email Adapter | `internal/senses/email.go` | ✅ | ✅ | Real IMAP polling, mark-as-seen, allowed sender filter, SMTP send |
| Mock IMAP Server | `internal/senses/email_test.go` | ✅ | ✅ | Full IMAP protocol mock (LOGIN, SELECT, SEARCH, FETCH, STORE, LOGOUT) |
| Mock SMTP Server | `internal/senses/email_test.go` | ✅ | ✅ | Full SMTP mock (EHLO, MAIL FROM, RCPT TO, DATA, QUIT) |

**Key features:**
- Zero new dependencies — uses `crypto/tls`, `net/smtp`, `bufio`, `net` from stdlib
- TLS support for IMAP (port 993) and STARTTLS for SMTP (port 587)
- IMAP SEARCH UNSEEN → FETCH → STORE +FLAGS (\Seen) cycle
- Email address extraction from "Display Name <addr>" format
- Case-insensitive allowed sender matching
- Dependency injection (DialFunc, SMTPSendFunc) for testability
- 25 email-specific tests with mock IMAP/SMTP servers

---

## Phase 5: Generative UI ⏳ IN PROGRESS

> Full spec: [SPEC_DYNAMIC_UI.md](./SPEC_DYNAMIC_UI.md)
> Подход: **Level 3 — Fully Generated UI** (LLM генерирует HTML/CSS/JS и ANSI с нуля)

**Goal:** LLM-generated, device-adaptive UI — CLI ANSI art, web HTML apps, tablet kiosk

### Phase 5A: UIGenerator + CLI ANSI Rendering ✅ COMPLETE

| Component | Package | Status | Tests |
|-----------|---------|--------|-------|
| GeneratedUI types | `internal/genui/types.go` | ✅ | ✅ |
| UIGenerator (LLM → UI code) | `internal/genui/types.go` | ✅ | ✅ |
| ANSI system prompt | `internal/genui/prompt_ansi.go` | ✅ | ✅ |
| ANSI sanitizer | `internal/genui/sanitize.go` | ✅ | ✅ |
| Self-Healing (retry loop) | `internal/genui/types.go` | ✅ | ✅ |
| ThoughtLog builder | `internal/genui/thoughtlog.go` | ✅ | ✅ |
| CLI renderer | `internal/genui/cli.go` | ✅ | ✅ |
| UI Reflection | `internal/genui/reflection.go` | ✅ | ✅ |
| Pipeline StageLogs | `internal/pipeline/pipeline.go` | ✅ | ✅ |
| Pipeline integration | `cmd/overhuman/main.go` | ✅ | ✅ |

**98 tests** in `internal/genui/` + 4 integration tests in `cmd/overhuman/`

### Phase 5B: HTML Generation + WebSocket + Sandbox + Web Runtime ✅ COMPLETE

| Component | Package | Status | Tests |
|-----------|---------|--------|-------|
| HTML system prompt | `internal/genui/prompt_html.go` | ✅ | ✅ |
| React system prompt | `internal/genui/prompt_react.go` | ✅ | ✅ |
| HTML sanitizer + CSP | `internal/genui/sanitize_html.go` | ✅ | ✅ |
| Sandbox wrapper (iframe) | `internal/genui/sandbox.go` | ✅ | ✅ |
| Canvas layout | `internal/genui/canvas.go` | ✅ | ✅ |
| WebSocket server (RFC 6455, stdlib) | `internal/genui/ws.go` | ✅ | ✅ |
| WS protocol (10 message types) | `internal/genui/ws_protocol.go` | ✅ | ✅ |
| Streaming generation | `internal/genui/stream.go` | ✅ | ✅ |
| HTTP REST API | `internal/genui/api.go` | ✅ | ✅ |
| Emergency Stop (WS cancel) | `internal/genui/ws_protocol.go` | ✅ | ✅ |
| Daemon WS integration | `cmd/overhuman/main.go` | ✅ | ✅ |

**93 new tests** (191 total in `internal/genui/`), zero external dependencies (RFC 6455 WebSocket in pure stdlib)

### Phase 5C: Tablet Kiosk App ✅ COMPLETE

| Component | Package | Status | Tests |
|-----------|---------|--------|-------|
| Kiosk handler (Go HTTP server) | `internal/genui/kiosk.go` | ✅ | ✅ |
| Kiosk SPA (947-line embedded HTML/JS) | `internal/genui/kiosk_html.go` | ✅ | ✅ |
| WS client + auto-reconnect | kiosk_html.go (JS) | ✅ | ✅ |
| Sandbox iframe (CSP, allow-scripts) | kiosk_html.go (JS) | ✅ | ✅ |
| Action bridge (postMessage → WS) | kiosk_html.go (JS) | ✅ | ✅ |
| Canvas layout (sidebar + responsive) | kiosk_html.go (CSS) | ✅ | ✅ |
| Offline cache (localStorage) | kiosk_html.go (JS) | ✅ | ✅ |
| UI Feedback collection | kiosk_html.go (JS) | ✅ | ✅ |
| Emergency stop button | kiosk_html.go (JS) | ✅ | ✅ |
| Touch optimizations (44px targets) | kiosk_html.go (CSS) | ✅ | ✅ |
| Daemon kiosk integration | `cmd/overhuman/main.go` | ✅ | ✅ |

**33 new tests** in `kiosk_test.go`, daemon serves kiosk on port API+2 (default: 9092)

### Phase 5D: UI Evolution — Self-Improvement

| Component | Package | Status | Tests |
|-----------|---------|--------|-------|
| UI Memory (patterns by fingerprint) | `internal/genui/memory.go` | ⏳ | ⏳ |
| Hint Builder (history → prompts) | `internal/genui/hints.go` | ⏳ | ⏳ |
| A/B Testing (2 UI variants) | `internal/genui/ab.go` | ⏳ | ⏳ |
| Style Evolution | `internal/genui/style.go` | ⏳ | ⏳ |

---

## Statistics

| Metric | Value |
|--------|-------|
| Total Go files | ~132 |
| Total lines of Go | ~37,000 |
| Total tests | 826 |
| Packages with tests | 20/20 |
| External dependencies | 2 (uuid, sqlite) |
