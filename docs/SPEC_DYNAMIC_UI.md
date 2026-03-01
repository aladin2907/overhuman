# Overhuman: Generative UI — Спецификация

> Статус: ПРОЕКТИРОВАНИЕ. Spec → Test → Code.
> Зависимость: Phase 1-4 complete, CLI работает.
> Подход: **Level 3 — Fully Generated UI** (не шаблоны, не каталог компонентов)

---

## 1. ПРОБЛЕМА

### Текущее состояние

Overhuman умеет **думать** (10-stage pipeline), **запоминать** (memory), **учиться** (reflection + evolution), но имеет только **один рот** — CLI (stdin/stdout). Весь вывод — плоский текст.

```
Pipeline.Run() → RunResult{Result: string} → CLISense.Send() → fmt.Fprintf(stdout)
```

### Что делают конкуренты (feb 2026)

| Продукт | Подход | Уровень |
|---------|--------|---------|
| Vercel AI SDK | Разработчик пишет компоненты, AI выбирает | Level 1 — Controlled |
| A2UI (Google) | AI генерит JSON, клиент рендерит из каталога | Level 2 — Declarative |
| DivKit (Яндекс) | JSON → предзаданные нативные виджеты | Level 2 — Declarative |
| **Gemini Dynamic View** | LLM пишет HTML/CSS/JS с нуля на каждый промпт | **Level 3 — Fully Generated** |
| **Anthropic Artifacts** | Claude генерит React/HTML приложения в sandbox | **Level 3 — Fully Generated** |
| **MCP Apps** | Интерактивный UI через sandboxed iframe | **Level 3 — Fully Generated** |

**Никто не делает Level 3 в Go daemon + мультиустройства (CLI + tablet kiosk + web).**

### Главный инсайт

Level 1-2 — это старый подход: разработчик заранее решает ЧТО можно показать (каталог). AI только выбирает из готового. Это ограничивает агента — он не может создать UI, которого нет в каталоге.

Level 3 — это настоящая генерация: **LLM сам решает как визуализировать результат**. Никаких предзаданных компонентов. Агент может создать дашборд, игру, форму, визуализацию — что угодно.

---

## 2. РЕШЕНИЕ: Fully Generated UI

### Одно предложение

**LLM генерирует полный UI-код (HTML/CSS/JS) на лету для каждого ответа, а клиентское устройство рендерит его в изолированной песочнице.**

### Ключевое отличие от Level 2

| | Level 2 (прошлая спека) | Level 3 (новая спека) |
|-|------------------------|----------------------|
| Кто решает UI | Разработчик (каталог из 16 компонентов) | **LLM (генерирует с нуля)** |
| Формат | JSON с типами компонентов | **HTML + CSS + JS** (или Markdown+ANSI для CLI) |
| Гибкость | Только то что в каталоге | **Бесконечная** — агент может создать что угодно |
| Безопасность | Высокая (статический каталог) | Нужна песочница (iframe sandbox) |
| Стриминг | Partial JSON updates | **Прогрессивный HTML рендеринг** |
| Обучение | Нет | **UI улучшается через рефлексию** |

### Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│  Слой 1: PIPELINE + UI GENERATION (Go daemon)                   │
│                                                                  │
│  Pipeline.Run()                                                  │
│      ↓                                                           │
│  RunResult{Result: string, ...}                                  │
│      ↓                                                           │
│  UIGenerator.Generate(result, deviceCaps) → GeneratedUI          │
│      │                                                           │
│      ├─ CLI device  → LLM генерит Markdown + ANSI               │
│      ├─ Web device  → LLM генерит HTML + CSS + JS               │
│      └─ Tablet      → LLM генерит HTML + CSS + JS               │
│                                                                  │
│  GeneratedUI содержит полный код для рендеринга                  │
└────────────────────┬────────────────────────────────────────────┘
                     │ WebSocket / stdout / HTTP
┌────────────────────▼────────────────────────────────────────────┐
│  Слой 2: TRANSPORT                                              │
│  CLI: stdout (ANSI text stream)                                  │
│  Web/Tablet: WebSocket → { html, css, js, actions }             │
└────────────────────┬────────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────────┐
│  Слой 3: SANDBOX RENDERER (устройство)                          │
│  CLI: Terminal emulator (ANSI, встроенный)                       │
│  Web: Sandboxed iframe (CSP restricted)                          │
│  Tablet: WebView в kiosk mode (sandboxed)                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. UIGenerator — Сердце системы

### Принцип работы

UIGenerator — это **отдельный LLM-вызов** после pipeline. Pipeline думает и решает задачу. UIGenerator берёт результат и генерирует визуальное представление.

```go
// UIGenerator генерирует UI-код из результата pipeline.
type UIGenerator struct {
    llm    brain.LLMProvider
    router *brain.ModelRouter
    ctx    *brain.ContextAssembler
}

// GeneratedUI — полностью сгенерированный UI для одного ответа.
type GeneratedUI struct {
    TaskID    string            `json:"task_id"`
    Format    UIFormat          `json:"format"`       // "ansi", "html", "markdown"
    Code      string            `json:"code"`         // полный код UI
    Actions   []GeneratedAction `json:"actions,omitempty"`
    Meta      UIMeta            `json:"meta,omitempty"`
}

type UIFormat string

const (
    FormatANSI     UIFormat = "ansi"      // CLI: ANSI escape + box drawing
    FormatHTML     UIFormat = "html"      // Web/Tablet: HTML + CSS + JS
    FormatMarkdown UIFormat = "markdown"  // Fallback: plain markdown
)

// GeneratedAction — интерактивное действие, вшитое в сгенерированный UI.
type GeneratedAction struct {
    ID       string `json:"id"`
    Label    string `json:"label"`
    Callback string `json:"callback"` // callback ID для daemon
}

type UIMeta struct {
    Title     string `json:"title,omitempty"`
    Streaming bool   `json:"streaming,omitempty"`
}
```

### Generate flow

```go
func (g *UIGenerator) Generate(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities) (*GeneratedUI, error) {
    // 1. Выбираем формат по устройству
    format := g.selectFormat(caps)

    // 2. Собираем промпт для LLM
    prompt := g.buildPrompt(result, format, caps)

    // 3. LLM генерирует UI-код с нуля
    model := g.router.Select("simple", 100.0)  // дешёвая модель для UI
    resp, err := g.llm.Complete(ctx, brain.LLMRequest{
        Messages: prompt,
        Model:    model,
    })

    // 4. Парсим и валидируем сгенерированный код
    ui := g.parseResponse(resp.Content, format)

    // 5. Security: санитизация (XSS, script injection)
    ui.Code = g.sanitize(ui.Code, format)

    return ui, nil
}
```

### System Prompt для LLM (UI Generation)

Ключевая часть — системный промпт, который превращает LLM в UI-генератор:

```go
const uiSystemPromptHTML = `You are a UI generator for the Overhuman AI assistant.
Your job: take a task result and generate a COMPLETE, SELF-CONTAINED HTML page
that beautifully visualizes it.

RULES:
- Generate a SINGLE HTML document with inline <style> and <script>
- Use modern CSS (flexbox, grid, custom properties, animations)
- Dark theme by default (bg: #1a1a2e, text: #e0e0e0, accent: #00d4aa)
- Responsive: works on any screen size
- NO external dependencies (no CDN, no imports)
- NO fetch/XMLHttpRequest (sandboxed — no network)
- For charts: use SVG or Canvas (no Chart.js)
- For tables: sortable, striped, with hover
- For code: syntax-highlighted with monospace font
- For errors: red accent panel with icon
- Include subtle animations (fade-in, slide-up)
- If the result contains structured data — visualize it (chart, table, cards)
- If the result is plain text — beautiful typography with good spacing
- If the result is code — syntax highlighting with copy button
- If the result is a list — card grid or timeline depending on context

For interactive actions, emit buttons with:
  onclick="window.parent.postMessage({action: 'CALLBACK_ID', data: {}}, '*')"

RESPOND WITH ONLY THE HTML CODE. No explanations, no markdown fences.`

const uiSystemPromptANSI = `You are a terminal UI generator for the Overhuman AI assistant.
Your job: take a task result and generate beautiful ANSI terminal output.

RULES:
- Use ANSI escape codes for colors and formatting
- \033[1m for bold, \033[3m for italic, \033[0m for reset
- Colors: \033[36m cyan, \033[32m green, \033[31m red, \033[33m yellow, \033[90m grey
- Use box drawing characters: ┌ ┐ └ ┘ │ ─ ├ ┤ ┬ ┴ ┼
- Tables: aligned columns with box drawing borders
- Code: grey background simulation with │ left border
- Progress: [████████░░░░] 67%
- Lists: • or numbered with indent
- Headers: bold + cyan + underline
- Dividers: ─────────────────
- Key-value: right-aligned keys, left-aligned values
- Max width: 100 columns (wrap gracefully)
- Tree: ├── and └── with proper indentation

For structured data — use the most appropriate visualization.
For plain text — clean typography with section headers.
For code — monospace with language hint and │ border.

RESPOND WITH ONLY THE ANSI TEXT. No explanations, no markdown fences.`
```

### DeviceCapabilities — что умеет устройство

```go
// DeviceCapabilities описывает возможности устройства-рендерера.
type DeviceCapabilities struct {
    Format       UIFormat  `json:"format"`         // "html", "ansi", "markdown"
    Width        int       `json:"width"`          // ширина в символах (CLI) или пикселях (web)
    Height       int       `json:"height"`         // высота
    ColorDepth   int       `json:"color_depth"`    // 1 (monochrome), 8, 256, 16M
    Interactive  bool      `json:"interactive"`    // кнопки, формы
    JavaScript   bool      `json:"javascript"`     // JS execution (web only)
    SVG          bool      `json:"svg"`            // SVG rendering
    Animation    bool      `json:"animation"`      // CSS animation support
    TouchScreen  bool      `json:"touch_screen"`   // tablet
}

// CLICapabilities возвращает возможности терминала.
func CLICapabilities() DeviceCapabilities {
    return DeviceCapabilities{
        Format:      FormatANSI,
        Width:       getTermWidth(),  // os.Getenv("COLUMNS") или 80
        Height:      getTermHeight(),
        ColorDepth:  256,
        Interactive: true,            // stdin readline
        JavaScript:  false,
        SVG:         false,
        Animation:   false,
        TouchScreen: false,
    }
}

// WebCapabilities возвращает возможности веб-клиента.
func WebCapabilities(w, h int) DeviceCapabilities {
    return DeviceCapabilities{
        Format:      FormatHTML,
        Width:       w,
        Height:      h,
        ColorDepth:  16777216,  // 24-bit
        Interactive: true,
        JavaScript:  true,
        SVG:         true,
        Animation:   true,
        TouchScreen: false,
    }
}

// TabletCapabilities возвращает возможности планшета-киоска.
func TabletCapabilities(w, h int) DeviceCapabilities {
    return DeviceCapabilities{
        Format:      FormatHTML,
        Width:       w,
        Height:      h,
        ColorDepth:  16777216,
        Interactive: true,
        JavaScript:  true,
        SVG:         true,
        Animation:   true,
        TouchScreen: true,
    }
}
```

---

## 4. БЕЗОПАСНОСТЬ: Sandboxed Rendering

### Проблема

LLM генерирует произвольный код. Без песочницы это XSS, data exfiltration, и прочие атаки.

### Решение: CSP + Sandbox

**Для HTML (web/tablet):**

```html
<iframe
  sandbox="allow-scripts"
  srcdoc="GENERATED_HTML_HERE"
  style="width:100%;height:100%;border:none;"
  csp="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:;"
></iframe>
```

Запрещено:
- Сеть (fetch, XMLHttpRequest, WebSocket, EventSource) — `default-src 'none'`
- Навигация (top-level redirect, window.open)
- Формы (form submission to external)
- Доступ к parent DOM (sandbox без allow-same-origin)

Разрешено:
- Inline CSS и JS (для самодостаточного UI)
- SVG и Canvas рисование
- `postMessage` для action callbacks к parent

**Для ANSI (CLI):**
- Нет execution — просто текст с escape codes
- Санитизация: убираем опасные escape sequences (cursor repositioning, screen clear и т.д.)
- Whitelist: только цвета, bold, italic, underline, reset

```go
// sanitizeANSI убирает опасные ANSI escape sequences.
func sanitizeANSI(code string) string {
    // Разрешаем только: SGR (Select Graphic Rendition) — \033[...m
    // Блокируем: cursor movement, screen clear, scrolling, etc.
}
```

### Callback Security

Actions из сгенерированного UI проходят через `postMessage`:

```
iframe → postMessage({action: "apply_fix"}) → parent window → WebSocket → daemon
```

- Parent проверяет origin
- Daemon проверяет callback ID (белый список из GeneratedUI.Actions)
- Неизвестные callback ID отклоняются

---

## 5. UI РЕФЛЕКСИЯ — Самоулучшение UI

### Ключевая инновация Overhuman

Никто из конкурентов не делает рефлексию над сгенерированным UI. Gemini Dynamic View генерирует и забывает. Artifacts генерирует и забывает. **Overhuman запоминает и учится.**

### Как это работает

```
1. LLM генерирует UI → пользователь видит результат
2. Пользователь взаимодействует (или не взаимодействует)
3. UI Reflection: "Пользователь кликнул 'показать график'?
                   Значит для таких данных нужен график сразу."
4. Паттерн сохраняется: {data_type: "timeseries"} → {ui_hint: "include chart"}
5. Следующий раз: UIGenerator получает hint и генерирует UI с графиком сразу
```

```go
// UIReflection анализирует взаимодействие пользователя с UI.
type UIReflection struct {
    TaskID        string   `json:"task_id"`
    UIFormat      UIFormat `json:"format"`
    ActionsShown  []string `json:"actions_shown"`    // какие actions были в UI
    ActionsUsed   []string `json:"actions_used"`     // какие пользователь нажал
    TimeToAction  int64    `json:"time_to_action_ms"` // сколько думал
    Scrolled      bool     `json:"scrolled"`          // прокручивал ли
    Dismissed     bool     `json:"dismissed"`          // закрыл без действий
}

// LearnFromInteraction сохраняет паттерн UI-взаимодействия.
func (g *UIGenerator) LearnFromInteraction(ctx context.Context, refl UIReflection) {
    // Сохраняем в long-term memory как UI-паттерн.
    // При следующей генерации для похожих задач — подсказываем LLM.
}
```

### UI Hints из памяти

```go
// buildPrompt добавляет hints из предыдущих взаимодействий.
func (g *UIGenerator) buildPrompt(result pipeline.RunResult, format UIFormat, caps DeviceCapabilities) []brain.Message {
    // 1. System prompt (uiSystemPromptHTML или uiSystemPromptANSI)
    // 2. Результат задачи (result.Result, result.QualityScore, etc.)
    // 3. Device capabilities
    // 4. UI HINTS из памяти:
    //    "For similar tasks (fingerprint: summarize_csv), users preferred
    //     chart + table layout. Include a bar chart for numeric columns."
}
```

---

## 6. SELF-HEALING UI — Автоисцеление

### Проблема

LLM галлюцинирует. Сгенерированный код может содержать ошибки: невалидный HTML, битый JS, неработающий CSS. Юзер не должен видеть ошибки — агент должен сам починить.

### Решение: Error → Re-generate loop

```
LLM генерирует UI
        ↓
    Validate (parse HTML, lint ANSI)
        ↓
    Ошибка? ─── Нет ──→ Рендерим
        │
        Да
        ↓
    Отправляем LLM: "Ошибка: {error}. Исправь."
        ↓
    LLM регенерит (retry, max 2 попытки)
        ↓
    Всё ещё ошибка? → Fallback: plain text
```

```go
// generateWithRetry генерирует UI с автоисправлением ошибок.
func (g *UIGenerator) generateWithRetry(ctx context.Context, prompt []brain.Message, format UIFormat, maxRetries int) (string, error) {
    var lastErr string

    for attempt := 0; attempt <= maxRetries; attempt++ {
        messages := prompt
        if lastErr != "" {
            messages = append(messages, brain.Message{
                Role:    "user",
                Content: fmt.Sprintf("The previous UI code had an error:\n%s\n\nFix it and regenerate.", lastErr),
            })
        }

        resp, err := g.llm.Complete(ctx, brain.LLMRequest{
            Messages: messages,
            Model:    g.router.Select("simple", 100.0),
        })
        if err != nil {
            return "", err
        }

        // Validate
        if validationErr := g.validate(resp.Content, format); validationErr != nil {
            lastErr = validationErr.Error()
            continue
        }

        return resp.Content, nil
    }

    return "", fmt.Errorf("UI generation failed after %d attempts: %s", maxRetries+1, lastErr)
}
```

**Валидация по формату:**
- **ANSI**: проверяем что все `\033[` sequences закрыты, нет незакрытых цветов
- **HTML**: проверяем парсинг через `html.Parse()`, нет unclosed tags
- **React** (Phase 5B web): Sandpack/react-runner ловит `LiveError`, отдаём LLM

---

## 7. PROGRESSIVE DISCLOSURE + THOUGHT LOGS

### Progressive Disclosure — не перегружаем мозг

Агент не вываливает всё сразу. Показываем TL;DR, а детали — по запросу:

```
┌──────────────────────────────────────────────┐
│  ✓ Анализ завершён: 3 ошибки в 1,247 строках │  ← TL;DR (всегда виден)
│                                               │
│  [▸ Показать детали]                          │  ← drill-down
│  [▸ Цепочка мыслей агента]                    │  ← thought log
│                                               │
│  [1] Применить фикс  [2] Показать diff        │  ← actions
└──────────────────────────────────────────────┘
```

Для **CLI**: `[▸ Показать детали]` → пользователь нажимает `d` → разворачивается.
Для **Web/Tablet**: collapsible sections в HTML.

### Thought Logs — прозрачность работы агента

LLM генерирует в UI раскрываемый блок "Цепочка мыслей":

```
▸ Как агент решал задачу (18.2s, $0.007)
  ├─ Stage 1: Intake → "Анализ кода в main.go"
  ├─ Stage 2: Clarify → "Ищем баги и стилистические ошибки"
  ├─ Stage 3: Plan → "1) lint 2) analyze 3) suggest fixes"
  ├─ Stage 5: Execute → вызвал Code Execution skill
  ├─ Stage 6: Review → качество 80%
  └─ Stage 9: Reflect → "В следующий раз проверять тесты тоже"
```

Данные для Thought Logs берём из pipeline stages (они уже логируются):

```go
// ThoughtLog — запись о работе pipeline для отображения в UI.
type ThoughtLog struct {
    Stages []ThoughtStage `json:"stages"`
}

type ThoughtStage struct {
    Number  int    `json:"number"`
    Name    string `json:"name"`
    Summary string `json:"summary"`
    DurMs   int64  `json:"duration_ms"`
}
```

### Emergency Stop — аварийная остановка

Во всех UI (CLI, web, tablet) обязательна кнопка/команда остановки:

- **CLI**: `Ctrl+C` → прерывает текущий pipeline run + UI generation
- **Web/Tablet**: красная кнопка "⏹ Stop" → WebSocket `{type: "cancel"}` → daemon `cancel()`
- Агент корректно завершает текущую работу, показывает partial result

---

## 8. CANVAS ARCHITECTURE — Холст, не чат

### Проблема обычного чата

В чате UI линеен: вопрос → ответ → вопрос → ответ. Для агента это слабо:
- Большая таблица не помещается в узкую колонку чата
- Графики нечитабельны в маленьком блоке
- Нет места для sidebar с фоновыми задачами

### Canvas-first подход

Для web/tablet клиентов используем Canvas layout:

```
┌──────────────────────────────────────────────────────────┐
│ ┌──────────────┐  ┌────────────────────────────────────┐ │
│ │              │  │                                    │ │
│ │  SIDEBAR     │  │         CANVAS (main area)         │ │
│ │              │  │                                    │ │
│ │  • Tasks     │  │  [Generated UI fills this space]   │ │
│ │  • Status    │  │                                    │ │
│ │  • Health    │  │  Graphs, tables, code, forms...    │ │
│ │              │  │                                    │ │
│ │              │  │                                    │ │
│ ├──────────────┤  ├────────────────────────────────────┤ │
│ │              │  │                                    │ │
│ │  CHAT INPUT  │  │  [Thought Logs — collapsible]      │ │
│ │  ───────── ▶ │  │                                    │ │
│ └──────────────┘  └────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

**Dynamic Expand**: если данных много → Canvas занимает 100% экрана, sidebar сворачивается в иконки.

Для **CLI**: Canvas = полная ширина терминала. Sidebar = нет (CLI линеен).

---

## 9. WEB RUNTIME: Sandpack / In-Browser Compilation

### Почему не просто HTML в iframe

Простой HTML ограничен: нет компонентной модели, нет state management, нет библиотек для графиков. Для web-клиента мы идём дальше:

### Подход: LLM генерирует React + Tailwind → компилируется в браузере

```
LLM генерирует React-компонент + Tailwind CSS
        ↓
    Стримится по WebSocket
        ↓
    Sandpack / react-runner компилирует JSX→JS в браузере
        ↓
    Рендерится в sandbox iframe
        ↓
    Если LiveError → Self-Healing (отправляем ошибку LLM)
```

**System prompt для React UI (web/tablet):**

```
You are a UI generator. Write a SINGLE React component using:
- React hooks (useState, useEffect)
- Tailwind CSS for styling (dark theme: bg-gray-900 text-gray-100)
- Recharts for charts (import { BarChart, LineChart } from 'recharts')
- Lucide icons (import { Check, X, AlertCircle } from 'lucide-react')

RULES:
- Export default function Component()
- NO fetch/axios — data is passed as props
- For actions: call props.onAction('callback_id', data)
- Responsive: works on mobile and desktop
- Animations: use Tailwind transitions
- Include error boundaries

RESPOND WITH ONLY THE JSX CODE. No explanations.
```

**Доступные библиотеки в песочнице (whitelist):**
- `react`, `react-dom` — core
- `recharts` — графики
- `lucide-react` — иконки
- `tailwindcss` — стили
- `date-fns` — даты

Всё остальное заблокировано. LLM не может импортировать произвольные пакеты.

### Fallback chain

```
React + Tailwind (web) → если ошибка → HTML + inline CSS → если ошибка → Markdown
```

Для **CLI** и **tablet без JS runtime**: всегда ANSI / чистый HTML.

---

## 10. СТРИМИНГ UI

### Прогрессивная генерация

LLM генерирует UI токен за токеном. Мы можем стримить HTML по мере генерации:

```
Token 1-50:   <html><head><style>...
Token 50-100: ...body styles...</style></head><body>
Token 100+:   <div class="result">... content ...
```

**Для CLI:** стримим ANSI текст построчно — пользователь видит UI по мере генерации.

**Для Web/Tablet:** два подхода:
1. **Progressively render HTML** — обновляем srcdoc iframe по мере получения токенов
2. **Show skeleton → replace** — показываем skeleton loader, заменяем финальным UI

```go
// StreamGenerate стримит UI по мере генерации.
func (g *UIGenerator) StreamGenerate(ctx context.Context, result pipeline.RunResult, caps DeviceCapabilities) (<-chan UIChunk, error) {
    chunks := make(chan UIChunk, 100)

    go func() {
        defer close(chunks)

        // LLM streaming response
        stream, err := g.llm.StreamComplete(ctx, brain.LLMRequest{...})
        if err != nil {
            chunks <- UIChunk{Error: err}
            return
        }

        for token := range stream {
            chunks <- UIChunk{
                Content: token,
                Done:    false,
            }
        }
        chunks <- UIChunk{Done: true}
    }()

    return chunks, nil
}

type UIChunk struct {
    Content string `json:"content,omitempty"`
    Done    bool   `json:"done"`
    Error   error  `json:"-"`
}
```

---

## 11. TRANSPORT — Протоколы доставки

### 11.1 CLI Transport

```
UIGenerator → ANSI text → stdout (line by line)
```

- Стримим ANSI текст прямо в терминал
- Пользователь видит UI появляющийся построчно
- Actions: `[1] Apply fix  [2] Show diff` → readline input

### 11.2 WebSocket Transport (web/tablet)

```
UIGenerator → GeneratedUI → WebSocket frame → Client iframe
```

**Протокол:**

```go
// Сервер → Клиент:
type WSMessage struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// type = "ui_full"     → {html: "...", actions: [...]}   полный UI
// type = "ui_stream"   → {chunk: "...", done: false}     стриминг
// type = "action_result" → {action_id, success, data}    ответ на action
// type = "error"       → {code, message}
// type = "pong"        → {}

// Клиент → Сервер:
// type = "action"  → {action_id: "apply_fix", data: {...}}
// type = "input"   → {text: "user query"}
// type = "ping"    → {}
// type = "ui_feedback" → {task_id, scrolled, time_to_action, actions_used}
```

### 11.3 HTTP API

```
POST /api/task  → pipeline + UI generation → {result, ui: {format, code, actions}}
GET  /api/task/{id}/ui  → последний GeneratedUI
GET  /api/task/{id}/stream  → SSE стриминг
```

---

## 12. ИНТЕГРАЦИЯ С PIPELINE

### Текущий flow

```go
result, err := p.Run(ctx, *input)
cli.Send(ctx, "", fmt.Sprintf("[task: %s]\n%s", result.TaskID, result.Result))
```

### Новый flow

```go
result, err := p.Run(ctx, *input)

// UI генерация — отдельный шаг после pipeline
ui, err := uiGen.Generate(ctx, result, deviceCaps)
if err != nil {
    // Fallback: plain text как раньше
    renderer.RenderPlainText(ctx, result.Result)
} else {
    renderer.RenderUI(ctx, ui)
}

// Обработка действий (если интерактивный UI)
if ui != nil && len(ui.Actions) > 0 {
    action := renderer.WaitForAction(ctx)
    if action != nil {
        p.HandleCallback(ctx, action)
    }
}
```

### Pipeline НЕ меняется

Pipeline остаётся 10-stage. UIGenerator — это **пост-процессинг** после pipeline, как и рефлексия. Pipeline генерирует `RunResult`, UIGenerator визуализирует его.

---

## 13. ФАЗЫ РЕАЛИЗАЦИИ

### Phase 5A: UIGenerator + CLI ANSI Rendering

**Цель:** LLM генерирует красивый ANSI-вывод для терминала вместо плоского текста

| Компонент | Пакет | Что делает |
|-----------|-------|-----------|
| GeneratedUI types | `internal/genui/types.go` | Все структуры (GeneratedUI, DeviceCapabilities, UIChunk, ThoughtLog, etc.) |
| UIGenerator | `internal/genui/generator.go` | LLM-вызов с system prompt для генерации UI |
| ANSI system prompt | `internal/genui/prompt_ansi.go` | System prompt для терминального UI |
| ANSI sanitizer | `internal/genui/sanitize.go` | Фильтрация опасных ANSI sequences |
| Self-Healing | `internal/genui/selfheal.go` | Validate → retry (max 2) → fallback |
| ThoughtLog builder | `internal/genui/thoughtlog.go` | Pipeline stages → ThoughtLog для UI |
| CLI renderer | `internal/genui/cli.go` | Вывод GeneratedUI в терминал (Progressive Disclosure) |
| CLI action handler | `internal/genui/cli_actions.go` | Обработка `[1] Label` input + Emergency Stop |
| UI Reflection | `internal/genui/reflection.go` | Запись UI-паттернов взаимодействия |
| Pipeline integration | `cmd/overhuman/main.go` | Подключение UIGenerator после pipeline |

**Acceptance criteria:**
1. LLM генерирует ANSI UI для каждого ответа (таблицы, код, заголовки)
2. ANSI sanitizer блокирует опасные sequences
3. Actions работают через numbered input
4. Fallback: если UI generation fails → plain text как раньше
5. Self-Healing: невалидный код → retry с ошибкой → max 2 попытки → fallback
6. ThoughtLog: pipeline stages отображаются в UI как раскрываемый блок
7. Progressive Disclosure: TL;DR + `[d] Details` в CLI
8. Emergency Stop: `Ctrl+C` прерывает pipeline + UI generation
9. UI reflection записывает паттерны взаимодействия
10. `go test ./internal/genui/...` — все тесты проходят
11. Нет новых зависимостей

### Phase 5B: HTML Generation + WebSocket + Sandbox

**Цель:** LLM генерирует полные HTML-приложения, отдаются по WebSocket в sandboxed iframe

| Компонент | Пакет | Что делает |
|-----------|-------|-----------|
| HTML system prompt | `internal/genui/prompt_html.go` | System prompt для HTML/CSS/JS генерации |
| React system prompt | `internal/genui/prompt_react.go` | System prompt для React+Tailwind (Sandpack) |
| HTML sanitizer | `internal/genui/sanitize_html.go` | CSP header, dangerous tag removal |
| Canvas layout | `internal/genui/canvas.go` | Sidebar + Canvas + Dynamic Expand |
| WebSocket server | `internal/genui/ws.go` | WS endpoint `/ws` |
| WS protocol + Cancel | `internal/genui/ws_protocol.go` | Message types, action routing, emergency stop |
| Streaming | `internal/genui/stream.go` | StreamGenerate, progressive rendering |
| HTTP API | `internal/genui/api.go` | REST endpoints |
| Sandbox wrapper | `internal/genui/sandbox.go` | iframe + CSP generation |

**Acceptance criteria:**
1. LLM генерирует полные HTML страницы с inline CSS/JS
2. React+Tailwind: Sandpack/react-runner компилирует JSX в браузере
3. HTML проходит через sanitizer (нет fetch, нет navigation)
4. Canvas layout: sidebar + main area, dynamic expand для больших данных
5. Sandbox iframe не может exfiltrate данные
6. WebSocket стримит UI по мере генерации
7. Action callbacks из iframe → daemon → pipeline
8. Emergency Stop: WS cancel → daemon cancel → partial result
9. Reconnect: клиент получает последний UI
10. Fallback chain: React → HTML → Markdown

### Phase 5C: Tablet Kiosk App

**Цель:** Планшет в kiosk mode показывает сгенерированный HTML UI

| Компонент | Что делает |
|-----------|-----------|
| Flutter/Kotlin app shell | Fullscreen WebView, kiosk mode, no navigation |
| WS client + reconnect | Подключение к daemon |
| WebView sandbox | Загрузка HTML в sandboxed WebView |
| Action bridge | postMessage → WS → daemon |
| Offline cache | Последний UI, retry на reconnect |
| Theme adaptation | Инжектируем CSS variables для dark/light |

### Phase 5D: UI Evolution — Самоулучшение

**Цель:** UI учится на взаимодействиях пользователя

| Компонент | Пакет | Что делает |
|-----------|-------|-----------|
| UI Memory | `internal/genui/memory.go` | Хранение UI-паттернов по fingerprint задачи |
| Hint Builder | `internal/genui/hints.go` | Построение hints из истории взаимодействий |
| A/B Testing | `internal/genui/ab.go` | Генерация 2 вариантов UI, выбор лучшего |
| Style Evolution | `internal/genui/style.go` | Адаптация стиля под предпочтения пользователя |

---

## 14. СПРАВКА: State of the Art (feb 2026)

### Академические работы

| Paper | Авторы | Ключевой результат |
|-------|--------|-------------------|
| "Generative UI: LLMs are Effective UI Generators" | Leviathan, Valevski (Google, Nov 2025) | 44% на уровне человека-эксперта, dataset PAGEN |
| "Generative Interfaces for Language Models" | Chen et al. (Stanford SALT-NLP, Aug 2025) | +72% предпочтения vs текстовый чат, FSM для взаимодействий |
| "GenerativeGUI: Dynamic GUI Generation Leveraging LLMs" | CHI 2025 | LLM генерит HTML на каждый turn, интерактивные виджеты |
| "Generative and Malleable User Interfaces" | Cao et al. (CHI 2025) | Модификация UI через natural language + direct manipulation |

### Протоколы и стандарты

| Протокол | Автор | Назначение | Stars |
|----------|-------|-----------|-------|
| MCP Apps (SEP-1865) | Anthropic + OpenAI | Интерактивный UI через sandboxed iframe в MCP | — |
| A2UI v0.9 | Google | Декларативный JSON (Level 2) | 7.7k |
| AG-UI | CopilotKit | Транспортный слой agent↔frontend | 11.5k |
| Open-JSON-UI | OpenAI | JSON UI schema | — |

### Фреймворки

| Фреймворк | Stars | Level | Язык |
|-----------|-------|-------|------|
| CopilotKit | ~29k | 1-3 (все уровни) | React/Angular |
| Tambo | ~10.8k | 1 (controlled) | React |
| Thesys C1 + Crayon | — | 3 (fully generated) | React + proprietary API |
| Vercel AI SDK | ~30k+ | 1 (tool→component) | React/Next.js |
| OpenUI (W&B) | ~21.6k | 3 (dev tool) | Python + browser |
| E2B Fragments | ~5.5k | 3 (sandbox) | Next.js |

### Go Terminal UI

| Библиотека | Stars | Назначение |
|-----------|-------|-----------|
| Bubble Tea (Charm) | ~30k | TUI framework (Elm Architecture) |
| tview | ~13k | Rich terminal widgets |
| termui | ~13k | Terminal dashboards |

**Ни одна Go-библиотека не делает AI-generated terminal UI.** Overhuman будет первым.

---

## 15. СТРУКТУРА ПРОЕКТА (новые файлы)

```
overhuman/
├── internal/
│   └── genui/                       # NEW — Generative UI subsystem
│       ├── types.go                 # GeneratedUI, DeviceCapabilities, UIChunk
│       ├── generator.go             # UIGenerator: LLM → UI code
│       ├── prompt_ansi.go           # System prompt для ANSI terminal UI
│       ├── prompt_html.go           # System prompt для HTML/CSS/JS  [Phase 5B]
│       ├── sanitize.go              # ANSI sanitizer (whitelist safe sequences)
│       ├── sanitize_html.go         # HTML/CSP sanitizer              [Phase 5B]
│       ├── sandbox.go               # Iframe sandbox wrapper           [Phase 5B]
│       ├── selfheal.go              # Self-Healing: validate → retry → fallback
│       ├── thoughtlog.go            # ThoughtLog builder from pipeline stages
│       ├── cli.go                   # CLI renderer (output to terminal)
│       ├── cli_actions.go           # CLI action handler
│       ├── canvas.go                # Canvas layout (sidebar+main)     [Phase 5B]
│       ├── ws.go                    # WebSocket server                 [Phase 5B]
│       ├── ws_protocol.go           # WS message types + cancel        [Phase 5B]
│       ├── prompt_react.go          # System prompt для React+Tailwind [Phase 5B]
│       ├── api.go                   # HTTP REST API                    [Phase 5B]
│       ├── stream.go                # Streaming generation             [Phase 5B]
│       ├── reflection.go            # UI reflection (learn from usage)
│       ├── memory.go                # UI pattern memory                [Phase 5D]
│       ├── hints.go                 # Hint builder from history        [Phase 5D]
│       ├── ab.go                    # A/B testing for UI               [Phase 5D]
│       ├── style.go                 # Style evolution                  [Phase 5D]
│       ├── generator_test.go
│       ├── selfheal_test.go
│       ├── disclosure_test.go
│       ├── canvas_test.go
│       ├── webruntime_test.go
│       ├── sanitize_test.go
│       ├── cli_test.go
│       ├── reflection_test.go
│       └── integration_test.go
├── cmd/
│   └── overhuman/
│       └── main.go                  # Modified: UIGenerator integration
└── docs/
    └── SPEC_DYNAMIC_UI.md           # This file
```

---

## 16. ТЕСТ-ПЛАН (Spec → Test → Code)

### Phase 5A тесты

#### types_test.go

```
TestGeneratedUI_MarshalJSON            — сериализация GeneratedUI
TestGeneratedUI_UnmarshalJSON          — десериализация GeneratedUI
TestDeviceCapabilities_CLI             — CLICapabilities() возвращает correct defaults
TestDeviceCapabilities_Web             — WebCapabilities() возвращает correct defaults
TestDeviceCapabilities_Tablet          — TabletCapabilities() с touch
TestUIFormat_Constants                 — FormatANSI, FormatHTML, FormatMarkdown
```

#### generator_test.go

```
TestGenerate_ANSI_SimpleText           — простой текст → ANSI вывод с форматированием
TestGenerate_ANSI_CodeResult           — код → ANSI с border и highlighting
TestGenerate_ANSI_TableData            — табличные данные → ASCII table
TestGenerate_ANSI_ErrorResult          — ошибка → красный блок
TestGenerate_ANSI_WithActions          — результат с actions → numbered options
TestGenerate_HTML_FullPage             — результат → полная HTML страница
TestGenerate_HTML_ContainsCSS          — HTML содержит inline <style>
TestGenerate_HTML_NoExternalDeps       — HTML не содержит CDN/import/fetch
TestGenerate_HTML_WithActions          — actions → postMessage buttons
TestGenerate_FallbackOnError           — LLM error → plain text fallback
TestGenerate_RespectsCapabilities      — ANSI для CLI, HTML для web
TestGenerate_UsesHintsFromMemory       — UI hints из памяти включены в промпт
```

#### sanitize_test.go

```
TestSanitizeANSI_AllowsColors         — \033[31m разрешён
TestSanitizeANSI_AllowsBold           — \033[1m разрешён
TestSanitizeANSI_AllowsBoxDrawing     — ┌┐└┘│─ разрешены
TestSanitizeANSI_BlocksCursorMove     — \033[H блокируется
TestSanitizeANSI_BlocksScreenClear    — \033[2J блокируется
TestSanitizeANSI_BlocksScrollRegion   — \033[r блокируется
TestSanitizeANSI_PreservesText        — обычный текст без изменений
TestSanitizeHTML_RemovesFetch          — fetch() удаляется
TestSanitizeHTML_RemovesXHR            — XMLHttpRequest удаляется
TestSanitizeHTML_RemovesWebSocket      — new WebSocket удаляется
TestSanitizeHTML_AllowsInlineCSS       — <style> разрешён
TestSanitizeHTML_AllowsInlineJS        — <script> (без network) разрешён
TestSanitizeHTML_AllowsSVG             — <svg> разрешён
TestSanitizeHTML_AllowsCanvas          — <canvas> разрешён
TestSanitizeHTML_AllowsPostMessage     — postMessage разрешён
TestSanitizeHTML_BlocksFormAction      — <form action="http://"> блокируется
TestSanitizeHTML_BlocksWindowOpen      — window.open блокируется
TestSanitizeHTML_GeneratesCSP          — генерирует корректный CSP header
```

#### cli_test.go

```
TestCLI_RenderANSI                     — GeneratedUI с ANSI → stdout
TestCLI_RenderFallbackMarkdown         — FormatMarkdown → plain output
TestCLI_RenderWithActions              — actions → numbered options displayed
TestCLI_HandleAction_ValidChoice       — "1" → correct ActionResponse
TestCLI_HandleAction_InvalidChoice     — "abc" → retry prompt
TestCLI_HandleAction_NoActions         — no actions → nil
TestCLI_StreamRender                   — UIChunks → progressive output
```

#### reflection_test.go

```
TestUIReflection_RecordInteraction     — записывает взаимодействие
TestUIReflection_NoAction              — dismissed=true если нет действий
TestUIReflection_ActionUsed            — записывает использованные actions
TestUIReflection_BuildHints            — история → UI hints для промпта
TestUIReflection_SimilarTasks          — hints из задач с таким же fingerprint
```

#### selfheal_test.go (§6 Self-Healing)

```
TestSelfHeal_ValidHTML_NoRetry         — валидный HTML → 0 retry, возвращает сразу
TestSelfHeal_InvalidHTML_RetryOnce     — невалидный HTML → LLM получает ошибку → 2-й вызов успешен
TestSelfHeal_InvalidHTML_RetryTwice    — 2 ошибки → 3-й вызов успешен (max retries=2)
TestSelfHeal_AllRetrysFail_Fallback    — 3 ошибки подряд → fallback на plain text
TestSelfHeal_ValidANSI_NoRetry         — валидный ANSI → без retry
TestSelfHeal_UnclosedANSI_Retry        — незакрытые ANSI escape → retry с ошибкой в промпте
TestSelfHeal_ErrorMessageInPrompt      — ошибка валидации содержится в retry-промпте
TestSelfHeal_HTMLParseFail             — html.Parse() ошибка → retry
TestSelfHeal_ReactLiveError            — Sandpack LiveError → отправляем LLM (Phase 5B)
```

#### disclosure_test.go (§7 Progressive Disclosure + Thought Logs)

```
TestThoughtLog_FromPipelineStages      — pipeline stages → ThoughtLog с номерами, именами, длительностью
TestThoughtLog_EmptyStages             — пустой pipeline → пустой ThoughtLog
TestThoughtLog_MarshalJSON             — сериализация ThoughtLog
TestThoughtLog_IncludedInPrompt        — ThoughtLog передаётся UIGenerator в промпт
TestProgressiveDisclosure_ANSI         — CLI: TL;DR + expand marker в ANSI output
TestProgressiveDisclosure_HTML         — Web: collapsible sections в HTML
TestEmergencyStop_CLI_Cancel           — Ctrl+C → context cancel → partial result
TestEmergencyStop_WS_Cancel            — WebSocket "cancel" → pipeline cancel (Phase 5B)
```

#### canvas_test.go (§8 Canvas Architecture)

```
TestCanvas_HTMLLayout                  — HTML содержит sidebar + canvas + chat input
TestCanvas_DynamicExpand               — большие данные → canvas=100%, sidebar collapsed
TestCanvas_ResponsiveWidth             — width < 768 → single column (no sidebar)
TestCanvas_CLI_NoCanvas                — CLI device → canvas layout не применяется
TestCanvas_TabletTouch                 — touch device → larger buttons/targets
```

#### webruntime_test.go (§9 Web Runtime)

```
TestWebRuntime_ReactPrompt             — web device → system prompt содержит React/Tailwind instructions
TestWebRuntime_AllowedImports          — whitelist: react, recharts, lucide-react, tailwindcss, date-fns
TestWebRuntime_BlockedImports          — axios, node-fetch, fs → блокируются sanitizer-ом
TestWebRuntime_FallbackChain           — React error → HTML fallback → Markdown fallback
TestWebRuntime_PropsDataPassing        — данные передаются через props, не fetch
TestWebRuntime_OnActionCallback        — props.onAction() → postMessage → daemon
```

#### integration_test.go

```
TestIntegration_EndToEnd_ANSI          — RunResult → Generate → CLI render
TestIntegration_EndToEnd_HTML          — RunResult → Generate → HTML string
TestIntegration_EndToEnd_React         — RunResult → Generate → React component (Phase 5B)
TestIntegration_Fallback               — LLM error → plain text
TestIntegration_ActionCallback         — UI action → pipeline callback
TestIntegration_SelfHeal_EndToEnd      — invalid → retry → valid → render
TestIntegration_ThoughtLogVisible      — pipeline stages → thought log в UI
TestIntegration_ProgressiveDisclosure  — large result → TL;DR + expand
```

---

## 17. ВЕРИФИКАЦИЯ (Acceptance Criteria)

### Phase 5A (ANSI Terminal)

1. **UIGenerator вызывает LLM** с UI system prompt для каждого ответа
2. **LLM генерирует ANSI** с таблицами, кодом, заголовками, цветами
3. **Sanitizer** блокирует cursor movement, screen clear, scroll region
4. **Fallback**: LLM error → результат показывается как plain text (как сейчас)
5. **Actions**: numbered options в CLI, readline input
6. **UI Reflection**: записывается какие actions использовал пользователь
7. **Нет новых зависимостей**: только stdlib
8. **Все тесты**: `go test ./internal/genui/...`

### Phase 5A (Self-Healing, Disclosure, Canvas) — §6-§9

9. **Self-Healing**: невалидный UI → LLM получает ошибку → регенерит (max 2 retries)
10. **Self-Healing fallback**: 3 провала подряд → plain text (graceful degradation)
11. **Progressive Disclosure**: TL;DR показывается сразу, детали — по запросу
12. **Thought Logs**: pipeline stages отображаются как раскрываемый блок
13. **Emergency Stop**: CLI `Ctrl+C` → cancel pipeline + UI generation
14. **Canvas layout**: web/tablet — sidebar + canvas; CLI — полная ширина
15. **Dynamic Expand**: большие данные → canvas на 100%, sidebar сворачивается

### Phase 5B (HTML + WebSocket)

16. **LLM генерирует полный HTML** с inline CSS + JS
17. **Sandbox**: iframe с CSP, нет network access
18. **WebSocket**: стриминг HTML по мере генерации
19. **Action bridge**: postMessage → WS → daemon → pipeline
20. **HTML sanitizer**: нет fetch/XHR/WebSocket/window.open
21. **Web Runtime**: React + Tailwind компиляция через Sandpack/react-runner
22. **React fallback chain**: React error → HTML → Markdown
23. **Emergency Stop WS**: красная кнопка → WebSocket cancel → daemon cancel

### Phase 5C (Tablet Kiosk)

24. **WebView** рендерит сгенерированный HTML
25. **Kiosk mode**: fullscreen, no system UI
26. **WS reconnect**: auto-reconnect с retry
27. **Canvas layout**: sidebar + canvas, dynamic expand на tablet

### Phase 5D (Evolution)

28. **UI Memory**: хранит паттерны по fingerprint
29. **Hints**: подсказки из истории включаются в промпт
30. **A/B**: два варианта UI для одного ответа, выбор лучшего

---

## 18. СТОИМОСТЬ UI ГЕНЕРАЦИИ

### Расчёт

UI-генерация — это дополнительный LLM-вызов. Используем дешёвую модель:

| Модель | Input (1K tokens) | Output (1K tokens) | Типичный UI (2K out) |
|--------|-------------------|---------------------|---------------------|
| gpt-4.1-nano | $0.10/M | $0.40/M | **$0.0008** (~0.1 цент) |
| gpt-4.1-mini | $0.40/M | $1.60/M | $0.0032 |
| o4-mini | $1.10/M | $4.40/M | $0.0088 |

**~0.1 цента за красивый UI** на gpt-4.1-nano — приемлемо.

### Оптимизации

1. **Кэширование**: одинаковые fingerprint → одинаковый UI (с параметризацией данных)
2. **Skip for simple**: "4" (ответ на 2+2) → не генерируем UI, показываем как есть
3. **Threshold**: UI генерация только если `len(result) > 100` символов
4. **User preference**: `overhuman configure` → enable/disable rich UI

---

## 19. ПРИНЯТЫЕ РЕШЕНИЯ

| # | Вопрос | Решение | Обоснование |
|---|--------|---------|-------------|
| 1 | Уровень генерации | **Level 3 — Fully Generated** | Бесконечная гибкость, настоящее новшество |
| 2 | Формат для CLI | ANSI escape codes | Красивый терминальный вывод без зависимостей |
| 3 | Формат для web/tablet | Полный HTML + CSS + JS | Самодостаточная страница, рендерится в iframe |
| 4 | Безопасность | Sandbox iframe + CSP + ANSI whitelist | Защита от XSS и exfiltration |
| 5 | Модель для UI | gpt-4.1-nano (~0.1¢ за UI) | Дешёвая, быстрая, достаточная для UI |
| 6 | UI рефлексия | Запоминаем взаимодействия, hints в промпт | Уникальная фича Overhuman |
| 7 | Стриминг | Token-by-token для ANSI, progressive для HTML | Мгновенная обратная связь |
| 8 | Pipeline | НЕ меняется | UIGenerator — пост-процессинг после pipeline |
| 9 | Fallback | LLM error → plain text | Graceful degradation |
| 10 | Пакет | `internal/genui` | Отдельно от `render` (который был Level 2) |
| 11 | Первая фаза | ANSI CLI (Phase 5A) | Самый простой рендерер для проверки концепции |
| 12 | Новые зависимости Phase 5A | 0 | Только stdlib |
| 13 | Self-Healing | Retry loop (max 2) + fallback | LLM галлюцинирует — агент сам чинит |
| 14 | Progressive Disclosure | TL;DR + drill-down | Не перегружаем пользователя |
| 15 | Thought Logs | Pipeline stages в UI (collapsible) | Прозрачность: юзер видит как агент думал |
| 16 | Canvas vs Chat | Canvas-first (web/tablet) | Больше места для визуализации |
| 17 | Web Runtime | Sandpack/react-runner (Phase 5B) | React+Tailwind в браузере, не голый HTML |
| 18 | Emergency Stop | Ctrl+C (CLI), Stop button (WS) | Пользователь всегда может прервать |

---

## 20. ПОЧЕМУ ЭТО НОВШЕСТВО

| Что | Конкуренты | Overhuman |
|-----|-----------|-----------|
| Генерация UI | Gemini Dynamic View, Artifacts | **✅ Тоже Level 3** |
| Мультиустройства | Нет (только web) | **✅ CLI + Web + Tablet kiosk** |
| Self-Healing UI | Нет (галлюцинация = ошибка юзеру) | **✅ Auto-retry + fallback** |
| Thought Logs | Частично (Vercel AI SDK streaming) | **✅ Полная цепочка pipeline stages** |
| Progressive Disclosure | Нет (всё или ничего) | **✅ TL;DR + drill-down** |
| Canvas layout | Claude Artifacts (простой) | **✅ Sidebar + Canvas + Dynamic Expand** |
| UI рефлексия | Нет ни у кого | **✅ Учится на взаимодействиях** |
| UI A/B тестирование | Нет ни у кого | **✅ Автоматическое** |
| Go daemon | Нет ни у кого | **✅ Первый в Go** |
| Tablet kiosk | Нет ни у кого | **✅ Физическое устройство** |
| Стоимость | Gemini: дорого (мощная модель) | **✅ gpt-4.1-nano: 0.1¢** |
| Self-evolving | Нет | **✅ UI эволюционирует с агентом** |

**Overhuman = первый self-evolving AI agent с fully generated multi-device UI, self-healing, progressive disclosure + UI рефлексией.**
