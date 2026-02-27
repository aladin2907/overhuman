# Overhuman — Architecture

## Core Concept: Agent = Living Organism

```
                    ┌─────────────────────────────────────────┐
                    │              SOUL (DNA)                  │
                    │  principles | strategies | state | goals │
                    └──────────────────┬──────────────────────┘
                                       │
          ┌────────────────────────────┼────────────────────────────┐
          │                            │                            │
    ┌─────▼─────┐              ┌──────▼──────┐             ┌──────▼──────┐
    │  SENSES   │              │    BRAIN    │             │ INSTRUMENTS │
    │  (Input)  │──signals───▶│   (LLM)    │──decisions─▶│  (Skills)   │
    │           │              │            │             │             │
    │ - CLI     │              │ - thinks   │             │ - LLM skill │
    │ - API     │              │ - plans    │             │ - code skill│
    │ - webhook │              │ - reviews  │             │ - hybrid    │
    │ - timer   │              │            │             │ - subagents │
    └───────────┘              └──────┬─────┘             └─────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
    ┌─────▼─────┐             ┌──────▼──────┐            ┌──────▼──────┐
    │  MEMORY   │             │ REFLECTION  │            │  EVOLUTION  │
    │           │             │             │            │             │
    │ - short   │             │ - meso      │            │ - fitness   │
    │ - long    │             │ - macro     │            │ - A/B test  │
    │ - pattern │             │ - (micro)   │            │ - deprecate │
    │ - (SKB)   │             │ - (mega)    │            │ - rollback  │
    └───────────┘             └─────────────┘            └─────────────┘
```

## 10-Stage Pipeline

```
Signal ──▶ [1.Intake] ──▶ [2.Clarify] ──▶ [3.Plan] ──▶ [4.Select Agent]
                                                              │
          [10.Goals] ◀── [9.Reflect] ◀── [8.Patterns] ◀── [7.Memory]
                                                              │
                                                        [6.Review] ◀── [5.Execute]
```

Each stage transforms or enriches the `TaskSpec`:
1. **Intake**: `UnifiedInput` → `TaskSpec` (draft)
2. **Clarification**: LLM refines goal, constraints, expected output
3. **Planning**: Decompose into subtask DAG
4. **Agent Selection**: Assign skills or "self" to each subtask
5. **Execution**: DAG executor runs subtasks in parallel (goroutines)
6. **Review**: LLM evaluates quality (0.0-1.0)
7. **Memory Update**: Store in short-term + long-term
8. **Pattern Tracking**: Fingerprint → count → automation trigger
9. **Reflection**: Meso per-run + macro per-N-runs
10. **Goal Update**: Create goals from patterns, quality signals

Post-pipeline hooks (Phase 3):
- **Evolution**: Evaluate A/B tests, deprecate weak skills
- **Version Control**: Observe metrics, auto-rollback on degradation

## Package Dependencies

```
cmd/overhuman/main.go
    └── pipeline.Pipeline
            ├── soul.Soul
            ├── brain.LLMProvider (Claude | OpenAI)
            ├── brain.ModelRouter
            ├── brain.ContextAssembler
            ├── memory.ShortTermMemory
            ├── memory.LongTermMemory
            ├── memory.PatternTracker
            ├── senses.Sense (CLI | API | Webhook)
            ├── instruments.SkillRegistry       [Phase 2]
            ├── instruments.Generator           [Phase 2]
            ├── goals.Engine                    [Phase 2]
            ├── budget.Tracker                  [Phase 2]
            ├── evolution.Engine                [Phase 3]
            ├── reflection.Engine               [Phase 3]
            └── versioning.Controller           [Phase 3]
```

## Data Flow

### Reactive mode
```
User input → Sense adapter → UnifiedInput → Pipeline.Run() → RunResult → Sense response
```

### Proactive mode
```
Timer tick → Heartbeat UnifiedInput → Pipeline.Run() → Self-improvement / Goal execution
```

### LLM→Code Flywheel
```
Repeated task → PatternTracker (count >= K) → GoalEngine → Generator → CodeSkill
                                                                          │
Next occurrence of same pattern → SkillRegistry.FindActive() → CodeSkill.Execute()
                                                                (free, fast, deterministic)
```

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Language | Go | Daemon-first, goroutines, single binary, <10MB RAM |
| Storage | SQLite + files | Self-contained, human-readable, git-versionable |
| LLM SDK | Direct HTTP | No heavy SDK deps, only 2 external packages total |
| Concurrency | sync.RWMutex + goroutines | Simple, Go-native, no external coordination |
| Phase deps | Nil-safe optionals | Pipeline works with Phase 1 only, Phase 2/3 features activate when deps provided |
| Testing | httptest mock servers | Claude API format responses, no real API calls in tests |
| Code-skills | Multi-language (Python/JS/Bash/Go) | Agent chooses best language per task |
| Skill format | MCP servers (planned) | Industry standard, compatible with Claude/ChatGPT/Cursor |

## Thread Safety Model

All shared state is protected by `sync.RWMutex`:
- `SkillRegistry.mu` — skill map access
- `evolution.Engine.mu` — weights, tests, thresholds
- `versioning.Controller.mu` — change tracking
- `memory.ShortTermMemory.mu` — ring buffer
- `goals.Engine.mu` — goal list

**Critical pattern**: Never call a public locking method from within a locked method. Use lock-free inner functions (e.g., `computeFitness()` vs `ComputeFitness()`).
