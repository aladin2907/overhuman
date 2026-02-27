# Overhuman: Концептуальная спецификация

> Статус: ИССЛЕДОВАНИЕ И ПРОЕКТИРОВАНИЕ. Код не пишем пока не утвердим концепцию.

---

## 1. ПРОБЛЕМА: Почему нужен новый продукт

### Что есть на рынке сейчас (февраль 2026)

**Кодовые помощники** (Claude Code, Codex, Cursor):
- Отлично пишут код, но ТОЛЬКО код
- Нет памяти между сессиями
- Нет проактивности — работают только когда спросишь
- Нет самообучения — каждая сессия с нуля
- Rate limits (Claude Pro ~45 сообщений / 5 часов)

**Агентные фреймворки** (AutoGPT, CrewAI, LangGraph):
- AutoGPT: бесконечные циклы, $50+ на провальные задачи, не production-ready
- CrewAI: сложно, loops на десятки минут, болезненный дебаг
- LangGraph: надёжный, но низкоуровневый — это SDK, не продукт
- Все: нет настоящего самообучения, нет перехода LLM→Code

**OpenClaw** (191k stars, viral hit 2025):
- Лучший из существующих: multi-channel, soul, heartbeat, skills, local-first
- НО: дыры в безопасности (CVE-2026-25253 RCE, prompt injection, supply chain)
- НО: нет настоящего self-improvement (skills пишутся вручную)
- НО: entangled roles — один LLM планирует, исполняет И редактирует себя
- НО: сломанный cron scheduler, потеря API ключей после рестарта
- НО: flat memory — нет knowledge graph, нет semantic search
- НО: нет fitness-метрик, нет A/B, нет автоматической эволюции

### Что люди хотят (Reddit, X/Twitter, HackerNews)

1. **Always-on assistant**, не чат-бот — daemon который работает в фоне
2. **Multi-channel** — один мозг за всеми каналами (email, Telegram, Slack, web...)
3. **Память между сессиями** — чтобы не повторяться
4. **Помогал во ВСЁМ**, не только в коде — маркетинг, планирование, анализ, коммуникации
5. **Учился** — чтобы на 10-й раз делал задачу быстрее и дешевле чем на 1-й
6. **Безопасность** — 62% компаний называют security главным барьером для agent adoption
7. **Production-ready** — только 14% организаций имеют agent solutions готовые к deploy

### Главный инсайт

**Никто не делает полный цикл: обучение через повторение → автоматическая генерация кода → замена LLM-вызовов на код → экономия.**

OpenClaw умеет создавать skills, но вручную. AutoGPT умеет исполнять, но не учится. Claude Code пишет код, но не знает контекст жизни пользователя.

---

## 2. РЕШЕНИЕ: Что такое Overhuman

### Одно предложение
**Самоэволюционирующий ассистент, который помогает человеку во всём, учится через повторение и заменяет дорогие LLM-вызовы автоматически сгенерированным кодом.**

### Три стадии эволюции AI-ассистентов

| Стадия | Примеры | Характеристика |
|--------|---------|---------------|
| 1. Reactive Chatbot | ChatGPT, Claude chat | Отвечает на вопросы. Нет памяти, нет инструментов |
| 2. Tool-Using Agent | OpenClaw, Claude Code | Есть инструменты, есть память, но stateless между сессиями |
| **3. Self-Evolving Organism** | **Overhuman** | Идентичность, обучение, автоматизация, мета-рефлексия |

### Flywheel Effect (маховик)

```
Задачи → LLM выполняет → Рефлексия → Паттерны → Код-навыки
  ↑                                                    ↓
  └──── Дешевле, быстрее, надёжнее ◀──── Код заменяет LLM
```

Каждый цикл делает систему:
- **Дешевле** (code-skill вместо LLM-вызова)
- **Быстрее** (код мс, LLM секунды)
- **Надёжнее** (код детерминирован, LLM — нет)
- **Умнее** (рефлексия улучшает стратегии)

---

## 3. КЛЮЧЕВЫЕ ОТЛИЧИЯ

### Overhuman vs Всё остальное

| Возможность | OpenClaw | Claude Code | AutoGPT | CrewAI | LangGraph | **Overhuman** |
|------------|----------|-------------|---------|--------|-----------|--------------|
| Multi-channel input | 13+ каналов | Terminal only | Terminal | API | API | **Multi-channel + Timer** |
| Persistent memory | File-based | Нет | Vector DB (нестабильно) | Нет | Нет | **Short+Long+Patterns+SKB** |
| Self-improvement | Skill creation (ручное) | Нет | Нет | Нет | Нет | **4 петли рефлексии** |
| LLM→Code transition | Нет | Нет | Нет | Нет | Нет | **Автоматический** |
| Evolutionary selection | Нет | Нет | Нет | Нет | Нет | **Fitness + A/B** |
| Identity/Soul | SOUL.md (статичный) | Нет | Нет | Нет | Нет | **Живой документ** |
| Proactive goals | HEARTBEAT (cron list) | Нет | Task loop | Нет | Нет | **GoalEngine** |
| Budget control | Нет | Нет | Нет | Нет | Нет | **Per-task + global** |
| Version control + rollback | Нет | Нет | Нет | Нет | Нет | **Полный + auto-rollback** |
| Security architecture | Уязвим (RCE, SSRF) | Sandbox | Уязвим | Basic | Basic | **Security-first design** |
| Fractal agents | Нет | Нет | Нет | Multi-agent crews | Graph nodes | **Иерархия со своим soul** |

### 5 ключевых инноваций Overhuman

1. **LLM→Code Flywheel**: автоматическое превращение повторяющихся LLM-задач в код
2. **4-уровневая рефлексия**: микро (шаг) → мезо (задача) → макро (стратегия) → мега (процесс рефлексии)
3. **Живой Soul**: не статичный файл, а эволюционирующий документ с версионированием
4. **Дарвиновский отбор**: skills конкурируют, слабые выбраковываются, сильные размножаются
5. **Проактивное целеполагание**: агент сам ставит цели на основе рефлексии

---

## 4. ЧТО БЕРЁМ ОТ OPENCLAW (лучшие идеи)

| Идея OpenClaw | Что берём | Как улучшаем |
|---------------|-----------|-------------|
| Gateway (центральный хаб) | Архитектурный паттерн | Добавляем Budget Engine и Security Layer |
| Soul файл | Концепция идентичности | Делаем живым: версионирование, эволюция, неизменяемые якоря |
| Heartbeat (cron) | Проактивное поведение | GoalEngine вместо плоского cron-списка |
| Markdown Skills | Формат навыков | + Code-skills + Hybrid + автогенерация + fitness |
| Multi-channel | 13+ каналов | Архитектура InputAdapters — любой канал через адаптер |
| Local-first | Всё хранится локально | SQLite + файлы, git-versionable |
| File-based memory | Прозрачность, инспектируемость | + Long-term summarization + semantic search |
| Lane Queue | Последовательная обработка | + Параллелизм через asyncio DAG |
| Session keys | workspace:channel:userId | Берём как есть |

### Чего НЕ берём

- Node.js runtime (Python лучше для AI/ML ecosystem)
- `exec()` без sandbox (security-first)
- Entangled roles (разделяем планирование, исполнение, самоизменение)
- Плоскую память без semantic search
- Отсутствие метрик качества

---

## 5. АРХИТЕКТУРА: 12 систем

### Концептуальная модель: Агент = живой организм

```
                    ┌─────────────────────────────────────────┐
                    │              SOUL (ДНК)                  │
                    │  принципы | стратегии | состояние | цели │
                    └──────────────────┬──────────────────────┘
                                       │
          ┌────────────────────────────┼────────────────────────────┐
          │                            │                            │
    ┌─────▼─────┐              ┌──────▼──────┐             ┌──────▼──────┐
    │ ВОСПРИЯТИЕ │              │    МОЗГ     │             │    РУКИ     │
    │  (Input)   │──сигналы──▶│   (LLM)     │──решения──▶│(Instruments)│
    │            │              │             │             │             │
    │ - каналы   │              │ - думает    │             │ - skills    │
    │ - таймер   │              │ - планирует │             │ - код       │
    │ - webhook  │              │ - оценивает │             │ - контейнеры│
    │ - API      │              │             │             │ - субагенты │
    └────────────┘              └──────┬──────┘             └─────────────┘
                                       │
          ┌────────────────────────────┼────────────────────────────┐
          │                            │                            │
    ┌─────▼─────┐              ┌──────▼──────┐             ┌──────▼──────┐
    │   ПАМЯТЬ   │              │  РЕФЛЕКСИЯ  │             │  ЭВОЛЮЦИЯ   │
    │            │              │             │             │             │
    │ - кратко   │              │ - микро     │             │ - fitness   │
    │ - долго    │              │ - мезо      │             │ - A/B       │
    │ - паттерны │              │ - макро     │             │ - отбор     │
    │ - SKB      │              │ - мега      │             │ - откат     │
    └────────────┘              └─────────────┘             └─────────────┘
```

### 12 подсистем (кратко)

| # | Система | Назначение | Аналог в OpenClaw |
|---|---------|-----------|-------------------|
| 1 | **Soul** | Идентичность, принципы, стратегии, эволюция | SOUL.md (статичный) |
| 2 | **Signal Intake** | Приём и нормализация входных сигналов | Channel plugins |
| 3 | **Brain (LLM)** | Думает и решает. НЕ исполняет | LLM provider |
| 4 | **Instruments** | Skills, код, контейнеры, субагенты | Skills + tools |
| 5 | **Memory** | Short/long term, patterns, run history | Conversation logs |
| 6 | **Reflection** | 4 вложенных петли обратной связи | Нет |
| 7 | **GoalEngine** | Проактивные цели из рефлексии | HEARTBEAT.md |
| 8 | **Evolution** | Fitness, A/B, deprecation, автогенерация | Нет |
| 9 | **Version Control** | Версии + observation window + auto-rollback | Нет |
| 10 | **SKB** | Межагентный обмен опытом | Нет |
| 11 | **Budget** | Cost control, model routing, лимиты | Нет |
| 12 | **Execution Engine** | 10-стадийный пайплайн | Pipeline (неформализованный) |

### Два режима жизни

**Реактивный:** сигнал → пайплайн → результат → рефлексия
**Проактивный:** таймер → soul/цели → саморазвитие → обновление soul

### Рефлексия — 4 вложенных петли

| Петля | Когда | Вопрос | Что меняет |
|-------|-------|--------|------------|
| Микро | каждый шаг pipeline | "Этот шаг удался?" | Следующий шаг |
| Мезо | после каждого run | "Задача решена качественно?" | Память, навыки, паттерны |
| Макро | каждые N runs / таймер | "Мои стратегии адекватны?" | Soul, пороги, метрики, цели |
| Мега | редко / по триггеру | "Мои петли рефлексии работают?" | Сам процесс развития |

### Pipeline (10 стадий, из ТЗ + расширения)

1. **Intake** — UnifiedInput → TaskSpec v0
2. **Clarification** — уточняющие вопросы → TaskSpec v1
3. **Planning** — декомпозиция в DAG подзадач
4. **Agent Selection** — подбор/создание субагентов
5. **Execution** — параллельное выполнение, сбор результатов
6. **Review** — обязательная проверка качества
7. **Memory Update** — short/long term, run_history, метрики
8. **Pattern Tracking** — fingerprint, счётчики, примеры
9. **Reflection** — мезо-петля + триггер макро/мега
10. **Goal Update** — обновление GoalEngine

---

## 6. РАСШИРЕНИЯ ТЗ (что добавляем к базовому черновику)

Базовое ТЗ покрывает 9-стадийный pipeline + паттерны + автоматизацию. На основе исследования добавляем:

### 6.1 Soul (идентичность)
- ТЗ не содержит понятия идентичности агента
- Берём от OpenClaw: файл-DNA определяющий "кто я"
- Улучшаем: версионирование, неизменяемые принципы ("якоря"), история эволюции

### 6.2 Проактивное поведение (Timer + GoalEngine)
- ТЗ описывает только реактивный режим (сигнал → обработка)
- Добавляем: heartbeat-таймер (30 мин) для саморазвития без входящих задач
- GoalEngine: агент сам ставит цели (из рефлексии, паттернов, эволюции)

### 6.3 Многоуровневая рефлексия
- ТЗ описывает самоулучшение как один шаг после run
- Расширяем до 4 петель: микро/мезо/макро/мега
- Макро-петля: мета-рефлексия ("правильно ли я оцениваю себя?")
- Мега-петля: рефлексия над рефлексией ("правильно ли устроен мой процесс улучшения?")

### 6.4 Эволюционный отбор
- ТЗ описывает один code-skill на паттерн
- Расширяем: несколько конкурирующих skills → fitness-метрика → A/B → лучший побеждает
- Deprecation: skill с плохим fitness деактивируется
- Автогенерация: плохой fitness → GoalEngine → улучшенная версия

### 6.5 Версионирование + откат
- ТЗ не содержит механизма отката
- Добавляем: всё мутабельное версионируется (soul, skills, policies)
- Observation window: 5 runs после изменения → сравнение метрик
- Авто-откат при деградации

### 6.6 Межагентный опыт (SKB)
- ТЗ описывает фрактальную структуру, но без передачи опыта
- Добавляем: Shared Knowledge Base для пропагации паттернов, инсайтов, навыков
- Вверх (инсайт → parent), вниз (best practice → children), горизонтально

### 6.7 Budget Engine
- ТЗ не учитывает стоимость LLM-вызовов
- Добавляем: budget per task, global daily/monthly limits
- Model routing: простые → дешёвая модель, сложные → мощная
- Авто-переключение при приближении к лимиту

### 6.8 Security Architecture
- ТЗ не описывает безопасность
- Из исследования: OpenClaw взломали (CVE-2026-25253), supply chain attacks через skills
- Архитектурно предусматриваем: sandbox для code-skills, разделение ролей LLM, валидация skills

---

## 7. ТЕХНОЛОГИЧЕСКИЕ РЕШЕНИЯ

### Исследование языков (данные feb 2026)

| Метрика | Python (asyncio) | Go | Rust | TypeScript (OpenClaw) |
|---------|-----------------|-----|------|----------------------|
| **Memory usage** | ~200-500MB | **<10MB** | **7.8MB** | 1.52GB |
| **Startup time** | ~1-3s | **<1s** | **<10ms** | 5.98s |
| **Binary size** | N/A (interpreter) | Single binary | **3.4MB** | 28MB+ |
| **Concurrency** | asyncio (I/O-bound OK) | **goroutines** | **Tokio async** | Event loop |
| **AI/ML ecosystem** | **300k+ пакетов** | Растёт (Google ADK) | Растёт (Rig, ADK-Rust) | Хороший |
| **LLM SDK** | **anthropic, openai native** | Community SDKs | Community SDKs | Хороший |
| **Dev speed** | **Быстрый** | Быстрый | Медленный | Быстрый |
| **Scale ceiling** | ~5000 RPS (GIL) | **100k+ RPS** | **100k+ RPS** | ~10k RPS |
| **Deployment** | venv/docker | Single binary | Single binary | Node.js 22+ |

### Ключевые факты из исследования

**Python проблемы на масштабе:**
- GIL создаёт "saturation cliff" при 512+ потоках
- При 5000+ RPS — latency spikes
- Каждый процесс = 20-30MB overhead
- НО: для I/O-bound задач (LLM API calls) asyncio работает отлично

**Go преимущества для daemon:**
- Goroutines стартуют с KB стека (не MB)
- Microsoft снизил agent-to-agent latency на 42% заменив Python на Go
- Google ADK for Go — production-ready
- Single binary = простой deploy

**Rust — максимальная производительность:**
- ZeroClaw: 194x меньше памяти чем OpenClaw
- Memory safety без garbage collector
- Но: медленная разработка, steep learning curve

**Критический нюанс:** Для AI-агента 99% времени — это ожидание ответа от LLM API (1-30 секунд). Производительность языка в этом случае почти не имеет значения. Что имеет значение:
1. Скорость разработки (Python > Go > Rust)
2. Экосистема AI/ML (Python >> Go > Rust)
3. Надёжность на продакшене (Rust > Go > Python)
4. Простота деплоя (Go > Rust > Python)

### РЕШЕНИЕ: Go

**Обоснование:**
- **Daemon-first**: Go создан для long-running сервисов (goroutines, built-in concurrency)
- **Multi-channel**: goroutines идеальны для параллельной обработки множества каналов
- **Single binary**: один файл = простейший deploy, нет dependency hell
- **Масштабируемость**: не упрёмся в потолок языка при росте пользователей
- **Google ADK**: production-ready AI agent framework от Google уже на Go
- **Надёжность**: strict typing ловит ошибки на компиляции, не в runtime
- **Ресурсы**: <10MB RAM vs 200-500MB Python — можно запускать даже на Raspberry Pi
- **Community**: Go растёт как язык для AI tooling (Eino, ADK, PicoClaw)

**LLM SDK на Go:**
- anthropic-go (community, но стабильный)
- openai-go (official от OpenAI)
- langchaingo (LangChain порт)
- Google ADK (native)

**Риски Go:**
- Меньше AI-библиотек чем у Python (митигация: LLM SDK есть, остальное — HTTP API)
- Нет Pydantic (митигация: Go struct tags + validation libraries)
- Генерация кода (code-skills) — agent будет генерировать Python/JS код, но сам engine на Go

### Хранение: SQLite + файлы

- SQLite: метрики, run_history, patterns — структурированные данные
- Файлы: soul, skills, артефакты — человекочитаемые, git-versionable
- Нет внешних зависимостей (self-contained)
- FTS5 для текстового поиска на MVP

### Каналы (Senses / Органы чувств)

Каналы = динамические "органы чувств" агента. Архитектура адаптеров позволяет подключать новые без изменения ядра.

**MVP каналы (копируем подход OpenClaw):**
| Канал | Протокол | Библиотека Go |
|-------|----------|---------------|
| CLI (текст) | stdin/stdout | Built-in |
| HTTP API | REST/WebSocket | net/http, gorilla/websocket |
| Telegram | Bot API | telebot, gotgbot |
| Slack | Events API + Socket Mode | slack-go |
| Discord | Gateway WebSocket | discordgo |
| Email | IMAP/SMTP | go-imap, go-smtp |
| Webhook | HTTP POST | net/http |
| Timer/Heartbeat | Internal cron | robfig/cron |
| File watcher | FS events | fsnotify |

**Архитектурный паттерн:**
```
Channel Adapter → normalize → UnifiedInput → Pipeline
```
Каждый адаптер реализует один interface: `Sense` (Listen + Send)

---

## 8. ФАЗЫ РЕАЛИЗАЦИИ (Spec → Test → Code)

### Подход

Для каждого компонента:
1. **Spec** — что именно делает, acceptance criteria
2. **Test** — тесты которые должны пройти
3. **Code** — реализация на Go чтобы тесты прошли

### Фаза 1: Минимальный живой агент
**Цель:** Принимает текст → выполняет через LLM → сохраняет → учится

| Компонент | Что делает |
|-----------|-----------|
| Soul | Файл + версионирование + неизменяемые якоря |
| Agent model | Все поля из ТЗ п.7 (Go structs) |
| UnifiedInput | Нормализованный формат входа |
| Senses (каналы) | CLI + HTTP API + Timer |
| LLMProvider | Абстракция + Claude + OpenAI |
| Pipeline | 10 стадий (последовательно) |
| Memory | Short-term (ring buffer) + Long-term (SQLite + FTS5) |
| Pattern Tracker | Fingerprinting + счётчики |
| Meso-reflection | Самоулучшение после каждого run |

### Фаза 2: Каналы + Автоматизация
**Цель:** Multi-channel input + повторяющиеся задачи → code-skill

| Компонент | Что делает |
|-----------|-----------|
| Telegram Sense | Адаптер Telegram Bot API |
| Slack Sense | Адаптер Slack Events API |
| Discord Sense | Адаптер Discord Gateway |
| Email Sense | IMAP/SMTP адаптер |
| Webhook Sense | HTTP POST адаптер |
| Skill System | LLM-skill, Code-skill, Hybrid-skill |
| Code Generator | spec → code → tests → register |
| GoalEngine | Проактивные цели из рефлексии |
| Heartbeat | Проактивное саморазвитие по таймеру |

### Фаза 3: Эволюция и масштабирование
**Цель:** Конкурентный отбор навыков, бюджет, параллелизм

| Компонент | Что делает |
|-----------|-----------|
| Evolution Engine | Fitness + A/B + deprecation |
| Version Control | Версии + observation window + auto-rollback |
| Budget Engine | Per-task + global limits + model routing |
| Macro-reflection | Мета-рефлексия (стратегии адекватны?) |
| DAG Executor | Параллельное выполнение подзадач (goroutines) |

### Фаза 4: Зрелость
**Цель:** Полная автономия, межагентный опыт, безопасность

| Компонент | Что делает |
|-----------|-----------|
| SKB | Shared Knowledge Base (межагентный опыт) |
| Mega-reflection | Рефлексия над рефлексией |
| Micro-reflection | Корректировка каждого шага pipeline |
| Experimentation | A/B на стратегиях с гипотезами |
| Sandbox | Docker/Wasm изоляция для code-skills |
| Observability | Метрики, логи, трейсинг |

---

## 9. СТРУКТУРА ПРОЕКТА (Go)

```
overhuman/
├── cmd/
│   └── overhuman/
│       └── main.go              # Entry point (daemon + CLI commands)
├── internal/
│   ├── soul/
│   │   ├── soul.go              # Soul — живой документ (DNA)
│   │   └── version.go           # Версионирование soul + rollback
│   ├── agent/
│   │   ├── agent.go             # Agent model (все поля из ТЗ п.7)
│   │   └── registry.go          # Реестр агентов + субагентов
│   ├── pipeline/
│   │   ├── pipeline.go          # 10-стадийный pipeline
│   │   ├── taskspec.go          # TaskSpec model (versioned)
│   │   └── dag.go               # DAG executor (goroutines)
│   ├── brain/
│   │   ├── provider.go          # LLMProvider interface
│   │   ├── claude.go            # Claude implementation
│   │   ├── openai.go            # OpenAI implementation
│   │   ├── router.go            # Model routing по сложности/бюджету
│   │   └── context.go           # Context assembler (6 layers)
│   ├── senses/                  # "Органы чувств" — input channels
│   │   ├── sense.go             # Sense interface (Listen + Send)
│   │   ├── unified.go           # UnifiedInput model
│   │   ├── cli.go               # CLI (stdin/stdout)
│   │   ├── api.go               # HTTP REST + WebSocket
│   │   ├── telegram.go          # Telegram Bot API
│   │   ├── slack.go             # Slack Events + Socket Mode
│   │   ├── discord.go           # Discord Gateway
│   │   ├── email.go             # IMAP/SMTP
│   │   ├── webhook.go           # HTTP POST receiver
│   │   ├── filewatcher.go       # FS events (fsnotify)
│   │   └── heartbeat.go         # Internal timer (cron)
│   ├── mcp/                     # Model Context Protocol
│   │   ├── client.go            # MCP client (подключение к серверам)
│   │   ├── server.go            # MCP server (экспорт skills как tools)
│   │   └── registry.go          # Реестр MCP серверов (local + remote)
│   ├── instruments/
│   │   ├── skill.go             # Skill types (LLM/Code/Hybrid)
│   │   ├── registry.go          # Skill registry
│   │   ├── generator.go         # Code-skill generator (multi-language)
│   │   ├── docker.go            # Docker container manager для skills
│   │   └── subagent.go          # Subagent manager
│   ├── memory/
│   │   ├── shortterm.go         # Ring buffer (last N)
│   │   ├── longterm.go          # SQLite + embedding search
│   │   ├── patterns.go          # Fingerprinting + counters
│   │   └── skb.go               # Shared Knowledge Base
│   ├── reflection/
│   │   ├── engine.go            # Reflection orchestrator (4 loops)
│   │   ├── micro.go             # Per-step reflection
│   │   ├── meso.go              # Per-run reflection
│   │   ├── macro.go             # Meta-reflection (стратегии)
│   │   └── mega.go              # Reflection on reflection
│   ├── evolution/
│   │   ├── fitness.go           # Fitness metrics
│   │   ├── abtesting.go         # A/B testing skills
│   │   └── deprecation.go       # Skill deprecation + auto-generate
│   ├── goals/
│   │   └── engine.go            # GoalEngine (proactive goals)
│   ├── budget/
│   │   ├── tracker.go           # Cost tracking per agent/skill/task
│   │   └── limiter.go           # Daily/monthly limits + auto-downgrade
│   ├── versioning/
│   │   └── control.go           # Version control + observation window + rollback
│   └── storage/
│       ├── sqlite.go            # SQLite store (metrics, patterns, memory)
│       └── filestore.go         # File store (soul, skills, artifacts)
├── skills/                      # Стартовые MCP-серверы skills
│   ├── code-execution/          # Запуск кода в Docker
│   ├── web-search/              # Brave/Google search
│   ├── file-ops/                # Файловые операции
│   ├── git-management/          # Git операции
│   └── ...                      # (20 starter skills)
├── docker/
│   ├── Dockerfile               # Для self-deploy
│   └── skill-base/              # Базовые образы для skill-контейнеров
├── go.mod
├── go.sum
├── Makefile                     # build, test, install, release
└── README.md
```

---

## 10. ВЕРИФИКАЦИЯ (Acceptance Criteria)

**Базовые (из ТЗ):**
1. Принимает входящие сигналы разных типов → формирует TaskSpec
2. Выполняет задачу с обязательным ревью
3. Сохраняет артефакты run локально
4. Корректно ведёт паттерны и счётчики
5. После K повторений генерирует code-skill с тестами и регистрирует
6. На последующих повторениях использует code-skill с fallback на LLM
7. После каждого run выполняет самооценку и обновляет память/специализацию

**Расширенные (из исследования):**
8. Soul эволюционирует и версионируется
9. GoalEngine ставит цели проактивно
10. Fitness-метрики корректно считаются
11. Деградация skill → auto-rollback срабатывает
12. Budget лимиты соблюдаются
13. Multi-channel: одна задача может прийти из Telegram, ответ уйти в Slack
14. Single binary deploy: `./overhuman` запускает daemon

---

## 11. РИСКИ И МИТИГАЦИЯ (обновлено)

| Риск | Вероятность | Влияние | Митигация |
|------|------------|---------|-----------|
| Бесконтрольная мутация soul | Высокая | Критическое | Неизменяемые "якоря" в принципах |
| Стоимость рефлексии (4 петли = +LLM вызовы) | Высокая | Среднее | Мега-петлю отложить, micro — дешёвая модель |
| Холодный старт (агент бесполезен без опыта) | Высокая | Среднее | Starter pack навыков и шаблонов |
| Code-skill ломает систему | Средняя | Высокое | Sandbox + версионирование + auto-rollback |
| Supply chain через skills | Средняя | Высокое | Signed skills, sandbox execution |
| Prompt injection через каналы | Высокая | Высокое | Разделение ролей LLM, input sanitization |
| Сложность (12 подсистем) | Средняя | Среднее | Phased approach, MVP = 8 подсистем |
| Go ecosystem для AI слабее Python | Средняя | Среднее | LLM SDK есть, остальное через HTTP API |
| Нет Pydantic в Go | Низкая | Низкое | Go structs + validation tags + json marshal |

---

## 11. ВСЕ ПРИНЯТЫЕ РЕШЕНИЯ

| # | Вопрос | Решение | Обоснование |
|---|--------|---------|-------------|
| 1 | Scope | Универсальный | Помогает во всём, субагенты специализируются динамически |
| 2 | Каналы | Максимум сразу (9 каналов) | CLI + API + Telegram + Slack + Discord + Email + Webhook + File + Timer |
| 3 | Язык | **Go** | Daemon-first, goroutines, single binary, <10MB, масштабируется |
| 4 | OpenClaw | Копируем подход, НЕ интегрируемся | Свои адаптеры каналов, без зависимости |
| 5 | Хранение | SQLite + файлы | Self-contained, human-readable, git-versionable |
| 6 | Code-skills язык | Агент сам выбирает | Python/JS/Bash/Go — под задачу |
| 7 | Tool integration | MCP (Model Context Protocol) | Industry standard, 97M+ downloads, совместим с Claude/GPT/Gemini |
| 8 | Semantic search | Embedding-based (сразу в прод) | Не MVP-подход, делаем качественно |
| 9 | Sandbox | Docker | Контейнерная изоляция для skills |
| 10 | Starter skills | 20 skills в 5 категориях | Dev, Communication, Research, Files, Automation |
| 11 | Deployment | Single binary + OS service | launchd/systemd, auto-update, rollback |
| 12 | Качество | Сразу production | Не делаем "для MVP потом переделаем" |

## 12. ЗАКРЫТЫЕ ВОПРОСЫ (все решены)

| Вопрос | Решение | Обоснование |
|--------|---------|-------------|
| Code-skills язык | Агент сам выбирает | Гибкость: Python для data, JS для web, Bash для ops |
| MCP | Да, используем | Industry standard (Anthropic + OpenAI + Google + Microsoft). 97M+ SDK downloads, 10k+ серверов |
| Semantic search | Сразу embedding-based | Делаем в прод, не MVP. SQLite + vector extension или встроенный |
| Sandbox | Docker | Агент поднимает контейнеры для skills. Изоляция + воспроизводимость |
| Starter skills | 20 skills в 5 категориях | См. раздел 13 |
| Deployment | Single binary + OS service | launchd (macOS), systemd (Linux), auto-update + rollback |

---

## 13. СТАРТОВЫЕ НАВЫКИ (20 skills)

Skills реализуются как **MCP-серверы** — стандартный формат, совместимый с Claude/ChatGPT/Cursor.
Каждый skill запускается в Docker-контейнере при необходимости.

### Development & Code (5)

| # | Skill | Описание |
|---|-------|----------|
| 1 | **Code Execution** | Запуск Python/JS/Bash в Docker sandbox с timeout |
| 2 | **Git Management** | Clone, branch, commit, push, PR. Read-only по умолчанию |
| 3 | **Testing & QA** | Генерация и запуск unit/integration тестов, coverage |
| 4 | **Browser Automation** | Playwright для UI тестов, скриншоты, form filling |
| 5 | **Database Query** | SQL запросы, миграции, анализ схемы |

### Communication (4)

| # | Skill | Описание |
|---|-------|----------|
| 6 | **Email Management** | Read/draft/send, фильтры, приоритизация (IMAP/SMTP) |
| 7 | **Calendar Integration** | Расписание, проверка слотов, приглашения |
| 8 | **Messaging** | Slack/Discord/Telegram — отправка, чтение, треды |
| 9 | **Document Collaboration** | Google Docs, Notion — чтение/редактирование |

### Research & Information (4)

| # | Skill | Описание |
|---|-------|----------|
| 10 | **Web Search** | Поиск + извлечение данных (Brave MCP server) |
| 11 | **PDF & Document Analysis** | Извлечение текста, таблиц, анализ содержимого |
| 12 | **Data Aggregation** | Сбор данных из источников, нормализация, сводки |
| 13 | **Real-time Monitoring** | Трекинг изменений сайтов, RSS, цены |

### File & Data Management (3)

| # | Skill | Описание |
|---|-------|----------|
| 14 | **File Operations** | Чтение/запись, организация, поиск по паттернам |
| 15 | **Data Analysis** | CSV/JSON обработка, графики, статистика |
| 16 | **Knowledge Base Search** | RAG по документам с semantic search |

### Automation & Security (4)

| # | Skill | Описание |
|---|-------|----------|
| 17 | **API Integration** | REST вызовы, webhook обработка, трансформация данных |
| 18 | **Scheduled Tasks** | Cron-задачи, напоминания, триггеры по расписанию |
| 19 | **Audit & Logging** | Логирование всех действий, audit trail |
| 20 | **Credential Management** | Безопасное хранение API ключей, токенов (never expose) |

---

## 14. MCP (Model Context Protocol) АРХИТЕКТУРА

### Почему MCP

- **Industry standard**: Anthropic + OpenAI + Google + Microsoft + Block стандартизировались на MCP в 2025
- **Linux Foundation**: MCP передан в Agentic AI Foundation (AAIF) под Linux Foundation
- **97M+ SDK downloads**: зрелая экосистема
- **500+ публичных серверов**: не изобретаем велосипед
- **40-60% быстрее deploy**: по данным early adopters (Block, Apollo)

### Архитектура интеграции

```
Overhuman (Go daemon)
├── MCP Client (встроенный)
│   ├── Локальные MCP-серверы (Docker)
│   │   ├── Code Execution server
│   │   ├── File System server
│   │   ├── Database server
│   │   └── ... (custom skills)
│   ├── Публичные MCP-серверы
│   │   ├── Brave Search
│   │   ├── GitHub
│   │   ├── Google Drive
│   │   └── ... (community)
│   └── Автогенерированные MCP-серверы
│       └── (code-skills регистрируются как MCP tools)
└── LLM API Client
    └── Claude/OpenAI с MCP tool definitions
```

### Ключевое решение: Skills = MCP Servers

Каждый skill (включая автогенерированные code-skills) — это MCP-сервер:
- Стандартный формат tool definition
- Совместимость с Claude, ChatGPT, Cursor
- Docker-изоляция
- Горизонтальное масштабирование
- Легко шарить между агентами (SKB)

---

## 15. DEPLOYMENT

### Single Binary + OS Service

```
$ overhuman install   # Устанавливает как OS service
$ overhuman start     # Запускает daemon
$ overhuman stop      # Останавливает
$ overhuman status    # Статус + метрики
$ overhuman update    # Проверяет и обновляет
$ overhuman cli       # Интерактивный режим
```

### Платформы

| OS | Service Manager | Config Location |
|----|----------------|----------------|
| macOS | launchd | ~/Library/LaunchAgents/com.overhuman.agent.plist |
| Linux | systemd | /etc/systemd/system/overhuman.service |
| Windows | Windows Services (через WSL2 или нативно) | %AppData%\overhuman\ |

### Auto-update

1. Агент периодически проверяет `GET /api/version`
2. Скачивает новый binary, проверяет SHA256 подпись
3. Backup текущего binary
4. Atomic swap (rename)
5. Health check → если fail → rollback к backup
6. Restart через OS service manager

### Хранение данных

```
~/.overhuman/
├── soul.md              # Soul (DNA)
├── soul_versions/       # История версий soul
├── config.yaml          # Конфигурация
├── data/
│   ├── overhuman.db     # SQLite (метрики, patterns, memory)
│   ├── runs/            # Артефакты каждого run
│   └── skills/          # Автогенерированные skills
├── mcp/
│   └── servers/         # Конфигурации MCP серверов
└── logs/
    └── overhuman.log    # Логи
```
