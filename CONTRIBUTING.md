# Contributing to Overhuman

Thanks for your interest in contributing!

## Getting Started

```bash
git clone https://github.com/aladin2907/overhuman.git
cd overhuman
go build ./...
go test ./...
```

## Development

- **Language**: Go 1.25+
- **Dependencies**: 3 total (`google/uuid`, `modernc.org/sqlite`, `golang.org/x/term`)
- **Tests**: `go test ./...` — all tests use a mock LLM server, no API keys needed
- **Race check**: `go test -race ./...`

## Project Structure

All packages live under `internal/`. The entry point is `cmd/overhuman/main.go`.

Key packages:
- `pipeline/` — 10-stage task orchestrator
- `genui/` — generative UI (ANSI + HTML + Kiosk)
- `brain/` — LLM integration and model routing
- `senses/` — input channel adapters
- `memory/` — short-term + long-term storage

## Pull Requests

1. Fork the repo and create a branch from `main`
2. Write tests for new functionality
3. Run `go test ./...` and `go vet ./...` before submitting
4. Keep PRs focused — one feature or fix per PR
5. Follow existing code patterns and conventions

## Code Style

- Standard Go formatting (`gofmt`)
- No external linters required
- Nil-safe checks for optional dependencies (`if p.deps.Xxx != nil`)
- Lock-free inner functions for methods called under lock
- Tests use `httptest.Server` with mock Claude API responses

## Reporting Bugs

Use [GitHub Issues](https://github.com/aladin2907/overhuman/issues) with the bug report template.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
