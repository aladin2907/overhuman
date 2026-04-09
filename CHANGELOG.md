# Changelog

All notable changes to Overhuman are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] — 2026-04-09

### Added
- **Channel adapter integration.** Telegram, Slack, Discord and Email adapters
  are now wired into the daemon via a unified `SenseRegistry` with env-based
  configuration. Responses are routed back to the originating channel, and a
  primary channel can be designated for notifications.
- **Fast-path rendering** (`internal/genui/fastpath.go`). A declarative
  renderer for common content types — tables, code blocks, lists, key-value
  pairs, JSON, errors and short text — that bypasses the LLM entirely when
  the classifier recognises the shape of the pipeline output. Saves tokens
  and latency for structured results.
- `Sense.Send(ctx, target, message)` method on the `Sense` interface, enabling
  bidirectional routing for all channel adapters.
- `SenseRegistry.GetBySourceType` / `SetPrimary` / `GetPrimary` for source
  routing and primary notification targets.
- Kiosk UI screenshot in `docs/assets/` and README hero image.
- `funding.json` with schema-compliant structure (monthly & goodwill plans).

### Changed
- **README redesigned** with a visual-first layout: screenshot hero, Mermaid
  pipeline diagram, 2-column feature cards, ASCII ecosystem chart.
- Telegram / Slack / Discord adapters extended to implement the new `Send`
  method and integrate with `SenseRegistry`.
- Debranding pass across documentation and README — removed specific product
  and company names in favour of neutral phrasing.
- Version constant bumped to `0.2.0` in `cmd/overhuman/main.go`.

### Fixed
- None — this release is additive.

[0.2.0]: https://github.com/aladin2907/overhuman/compare/v0.1.0...v0.2.0

---

## [0.1.0] — 2026-03-19

Initial public release.

### Added
- **Phases 1–4 complete** (Minimal Living Agent → Maturity): Soul, Pipeline
  (10 stages), Evolution, Version Control, Budget, Reflection (micro / meso /
  macro / mega), SKB, Docker sandbox, Observability.
- **Phase 5 — Generative UI** (A/B/C/D): ANSI CLI rendering, HTML + WebSocket
  web runtime (RFC 6455 pure stdlib), Tablet Kiosk SPA, UI evolution (memory,
  hints, A/B testing, style learning).
- **Phase 6 — Deployment**: PID management, launchd/systemd service install,
  auto-update with SHA256 verification and rollback, file watcher sense, log
  tee, CLI commands (`install`, `uninstall`, `stop`, `update`, `logs`).
- **Fractal Agents**: Registry, Factory, SubagentManager (sync / async /
  fan-out / best-of-N delegation).
- **Security architecture**: Input sanitizer with prompt-injection detection,
  rate limiter, append-only audit logger, AES-256-GCM credential encryption,
  secret masking, skill validator with SHA-256 signatures, policy enforcer.
- **Starter Skills (20)**: 11 full implementations (code exec, git, testing,
  web search, file ops, data analysis, knowledge search, API integration,
  scheduler, audit, credentials) + 9 stubs pending external services.
- **MCP protocol** (JSON-RPC 2.0) client, server registry, skill bridge.
- **Real IMAP/SMTP email** adapter using only Go stdlib.
- **GitHub infrastructure**: CI, issue templates, contributing guide,
  GoReleaser config.

### Statistics
- ~41,800 lines of Go across 21 packages
- 946 tests
- 2 external dependencies (`uuid`, `sqlite`)

[0.1.0]: https://github.com/aladin2907/overhuman/releases/tag/v0.1.0
