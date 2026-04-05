package genui

import (
	"encoding/json"
	"fmt"
	"bytes"
	"html"
	"strings"
	"unicode/utf8"
)

// ContentType classifies pipeline result content for fast-path rendering.
type ContentType string

const (
	ContentTable    ContentType = "table"
	ContentCode     ContentType = "code"
	ContentList     ContentType = "list"
	ContentKeyValue ContentType = "key_value"
	ContentJSON     ContentType = "json"
	ContentError    ContentType = "error"
	ContentShort    ContentType = "short"
	ContentUnknown  ContentType = "unknown"
)

// FastPathResult holds the outcome of a fast-path rendering attempt.
type FastPathResult struct {
	Matched bool
	Type    ContentType
	Code    string
}

// TryFastPath attempts to render content without an LLM call.
// Returns Matched=true if the content was rendered declaratively.
// FormatReact and FormatMarkdown always fall through to LLM.
func TryFastPath(content string, format UIFormat) FastPathResult {
	if format == FormatReact || format == FormatMarkdown {
		return FastPathResult{Matched: false}
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return FastPathResult{Matched: true, Type: ContentShort, Code: renderEmpty(format)}
	}

	ct := detectContentType(trimmed)
	if ct == ContentUnknown {
		return FastPathResult{Matched: false}
	}

	code := renderContent(trimmed, ct, format)
	return FastPathResult{Matched: true, Type: ct, Code: code}
}

// ── Detection ────────────────────────────────────────────────────────

// detectContentType classifies content. Priority: JSON > Error > Table > Code > List > KV > Short.
func detectContentType(content string) ContentType {
	if detectJSON(content) {
		return ContentJSON
	}
	if detectError(content) {
		return ContentError
	}
	if detectTable(content) {
		return ContentTable
	}
	if detectCode(content) {
		return ContentCode
	}
	if detectList(content) {
		return ContentList
	}
	if detectKeyValue(content) {
		return ContentKeyValue
	}
	if len(content) < 200 {
		return ContentShort
	}
	return ContentUnknown
}

func detectJSON(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < 2 {
		return false
	}
	if (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
		return true
	}
	return false
}

func detectError(content string) bool {
	lower := strings.ToLower(content)
	strongPatterns := []string{
		"error:", "error -", "failed:", "failed to ",
		"exception:", "panic:", "fatal:", "fatal error",
		"err:", "stderr:", "traceback", "segfault",
	}
	for _, p := range strongPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// Stack trace patterns.
	if strings.Contains(content, ".go:") || strings.Contains(content, ".py:") ||
		strings.Contains(content, ".js:") || strings.Contains(lower, "at line ") ||
		strings.Contains(content, "goroutine ") {
		return true
	}
	return false
}

func detectTable(content string) bool {
	lines := splitLines(content)
	pipeRows := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isSeparatorLine(line) {
			continue
		}
		cells := parsePipeLine(line)
		if len(cells) >= 2 {
			pipeRows++
		}
	}
	if pipeRows >= 2 {
		return true
	}
	// Tab-separated: >= 2 lines with consistent tab count.
	tabRows := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Count(line, "\t") >= 1 {
			tabRows++
		}
	}
	return tabRows >= 2
}

var codeKeywords = []string{
	"func ", "function ", "class ", "def ", "return ",
	"import ", "package ", "#include", "var ", "const ",
	"let ", "pub fn ", "async ", "await ", "struct ",
	"enum ", "interface ", ":=", "=>", "->",
}

func detectCode(content string) bool {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		return true
	}
	lines := splitLines(trimmed)
	if len(lines) < 3 {
		return false
	}
	indented := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			indented++
		}
	}
	if float64(indented)/float64(len(lines)) < 0.4 {
		return false
	}
	kwMatches := 0
	for _, kw := range codeKeywords {
		if strings.Contains(content, kw) {
			kwMatches++
		}
	}
	return kwMatches >= 3
}

func detectList(content string) bool {
	lines := splitLines(content)
	matches := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
			matches++
			continue
		}
		if parseNumberedPrefix(trimmed) != "" {
			matches++
		}
	}
	return matches >= 2
}

func detectKeyValue(content string) bool {
	lines := splitLines(content)
	if len(lines) < 2 {
		return false
	}
	matches := 0
	total := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		total++
		if idx := strings.Index(line, ": "); idx > 0 && idx < 40 {
			matches++
		} else if idx := strings.Index(line, " = "); idx > 0 && idx < 40 {
			matches++
		}
	}
	return total >= 2 && float64(matches)/float64(total) > 0.5
}

// ── Rendering Dispatch ───────────────────────────────────────────────

func renderContent(content string, ct ContentType, format UIFormat) string {
	switch format {
	case FormatANSI:
		return renderContentANSI(content, ct)
	case FormatHTML:
		return renderContentHTML(content, ct)
	default:
		return content
	}
}

func renderContentANSI(content string, ct ContentType) string {
	switch ct {
	case ContentShort:
		return renderShortANSI(content)
	case ContentError:
		return renderErrorANSI(content)
	case ContentCode:
		return renderCodeANSI(content)
	case ContentTable:
		return renderTableANSI(content)
	case ContentList:
		return renderListANSI(content)
	case ContentKeyValue:
		return renderKVANSI(content)
	case ContentJSON:
		return renderJSONANSI(content)
	default:
		return content + "\033[0m"
	}
}

func renderContentHTML(content string, ct ContentType) string {
	switch ct {
	case ContentShort:
		return renderShortHTML(content)
	case ContentError:
		return renderErrorHTML(content)
	case ContentCode:
		return renderCodeHTML(content)
	case ContentTable:
		return renderTableHTML(content)
	case ContentList:
		return renderListHTML(content)
	case ContentKeyValue:
		return renderKVHTML(content)
	case ContentJSON:
		return renderJSONHTML(content)
	default:
		return htmlPage("Result", "", "<p>"+html.EscapeString(content)+"</p>")
	}
}

// ── ANSI Renderers ───────────────────────────────────────────────────

func renderShortANSI(content string) string {
	var b strings.Builder
	b.WriteString("\033[1m\033[36m━━━ Result ━━━\033[0m\n\n")
	b.WriteString(content)
	b.WriteString("\n\033[90m──────────────────────────────\033[0m\n\033[0m")
	return b.String()
}

func renderErrorANSI(content string) string {
	lines := splitLines(content)
	maxW := 0
	for _, line := range lines {
		w := utf8.RuneCountInString(strings.TrimSpace(line))
		if w > maxW {
			maxW = w
		}
	}
	if maxW > 76 {
		maxW = 76
	}
	boxW := maxW + 2

	var b strings.Builder
	b.WriteString("\033[1m\033[31m✗ Error\033[0m\n")
	b.WriteString("\033[31m┌" + strings.Repeat("─", boxW) + "┐\033[0m\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			trimmed = " "
		}
		w := utf8.RuneCountInString(trimmed)
		if w > maxW {
			trimmed = string([]rune(trimmed)[:maxW])
			w = maxW
		}
		pad := boxW - 1 - w
		if pad < 0 {
			pad = 0
		}
		b.WriteString("\033[31m│\033[0m " + trimmed + strings.Repeat(" ", pad) + "\033[31m│\033[0m\n")
	}
	b.WriteString("\033[31m└" + strings.Repeat("─", boxW) + "┘\033[0m\n")
	return b.String()
}

func renderCodeANSI(content string) string {
	code, lang := stripCodeFence(content)

	var b strings.Builder
	header := "Code"
	if lang != "" {
		header = lang
	}
	b.WriteString("\033[1m\033[36m " + header + " \033[0m\n")
	for _, line := range splitLines(code) {
		b.WriteString("\033[90m│\033[0m " + line + "\n")
	}
	b.WriteString("\033[0m")
	return b.String()
}

func renderTableANSI(content string) string {
	pt := parseTableContent(content)
	if pt == nil {
		return renderShortANSI(content)
	}

	numCols := len(pt.headers)
	widths := make([]int, numCols)
	for i, h := range pt.headers {
		widths[i] = utf8.RuneCountInString(strings.TrimSpace(h))
	}
	for _, row := range pt.rows {
		for i := 0; i < numCols && i < len(row); i++ {
			w := utf8.RuneCountInString(strings.TrimSpace(row[i]))
			if w > widths[i] {
				widths[i] = w
			}
		}
	}
	for i := range widths {
		widths[i] += 2 // 1-char padding each side
	}

	var b strings.Builder
	b.WriteString("\033[1m\033[36m Data Table \033[0m\n")

	// ┌─────┬─────┐
	b.WriteString("┌")
	for i, w := range widths {
		b.WriteString(strings.Repeat("─", w))
		if i < numCols-1 {
			b.WriteString("┬")
		}
	}
	b.WriteString("┐\n")

	// │ Header │
	b.WriteString("│")
	for i, h := range pt.headers {
		h = strings.TrimSpace(h)
		w := utf8.RuneCountInString(h)
		pad := widths[i] - 1 - w
		if pad < 0 {
			pad = 0
		}
		b.WriteString(" \033[1m" + h + "\033[0m" + strings.Repeat(" ", pad) + "│")
	}
	b.WriteString("\n")

	// ├─────┼─────┤
	b.WriteString("├")
	for i, w := range widths {
		b.WriteString(strings.Repeat("─", w))
		if i < numCols-1 {
			b.WriteString("┼")
		}
	}
	b.WriteString("┤\n")

	// Data rows
	for _, row := range pt.rows {
		b.WriteString("│")
		for i := 0; i < numCols; i++ {
			var cell string
			if i < len(row) {
				cell = strings.TrimSpace(row[i])
			}
			w := utf8.RuneCountInString(cell)
			pad := widths[i] - 1 - w
			if pad < 0 {
				pad = 0
			}
			b.WriteString(" " + cell + strings.Repeat(" ", pad) + "│")
		}
		b.WriteString("\n")
	}

	// └─────┴─────┘
	b.WriteString("└")
	for i, w := range widths {
		b.WriteString(strings.Repeat("─", w))
		if i < numCols-1 {
			b.WriteString("┴")
		}
	}
	b.WriteString("┘\n\033[0m")
	return b.String()
}

func renderListANSI(content string) string {
	var b strings.Builder
	b.WriteString("\033[1m\033[36m━━━ Results ━━━\033[0m\n\n")
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			b.WriteString("\n")
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "- "):
			b.WriteString("  \033[36m•\033[0m " + trimmed[2:] + "\n")
		case strings.HasPrefix(trimmed, "* "):
			b.WriteString("  \033[36m•\033[0m " + trimmed[2:] + "\n")
		case strings.HasPrefix(trimmed, "• "):
			b.WriteString("  \033[36m•\033[0m " + strings.TrimPrefix(trimmed, "• ") + "\n")
		default:
			if pfx := parseNumberedPrefix(trimmed); pfx != "" {
				b.WriteString("  \033[36m" + pfx + "\033[0m" + trimmed[len(pfx):] + "\n")
			} else {
				b.WriteString("  " + trimmed + "\n")
			}
		}
	}
	b.WriteString("\033[0m")
	return b.String()
}

func renderKVANSI(content string) string {
	pairs := parseKVContent(content)
	if len(pairs) == 0 {
		return renderShortANSI(content)
	}

	maxKeyW := 0
	for _, p := range pairs {
		w := utf8.RuneCountInString(p.key)
		if w > maxKeyW {
			maxKeyW = w
		}
	}

	var b strings.Builder
	b.WriteString("\033[1m\033[36m━━━ Details ━━━\033[0m\n\n")
	for _, p := range pairs {
		w := utf8.RuneCountInString(p.key)
		pad := maxKeyW - w
		if pad < 0 {
			pad = 0
		}
		b.WriteString("  \033[36m" + strings.Repeat(" ", pad) + p.key + "\033[0m │ " + p.value + "\n")
	}
	b.WriteString("\033[0m")
	return b.String()
}

func renderJSONANSI(content string) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(content), "", "  "); err != nil {
		return renderCodeANSI(content)
	}
	return renderCodeANSI("```json\n" + pretty.String() + "\n```")
}

// ── HTML Renderers ───────────────────────────────────────────────────

const htmlBaseCSS = `* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #1a1a2e; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; padding: 24px; line-height: 1.6; }
.container { max-width: 900px; margin: 0 auto; }
h1 { color: #00d4aa; font-size: 1.4em; margin-bottom: 16px; }
`

func htmlPage(title, extraCSS, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>%s</title>
<style>
%s%s
</style>
</head>
<body>
<div class="container">
%s
</div>
</body>
</html>`, html.EscapeString(title), htmlBaseCSS, extraCSS, body)
}

func renderShortHTML(content string) string {
	return htmlPage("Result", `
p { font-size: 1.1em; }
`, "<h1>Result</h1>\n<p>"+html.EscapeString(content)+"</p>")
}

func renderErrorHTML(content string) string {
	css := `
.error-box { background: rgba(255,59,48,0.1); border: 1px solid #ff3b30; border-radius: 8px; padding: 16px 20px; }
.error-title { color: #ff3b30; font-weight: 700; margin-bottom: 8px; }
.error-body { white-space: pre-wrap; font-family: 'SF Mono', Monaco, monospace; font-size: 0.9em; }
`
	body := `<div class="error-box">
<div class="error-title">✗ Error</div>
<div class="error-body">` + html.EscapeString(content) + `</div>
</div>`
	return htmlPage("Error", css, body)
}

func renderCodeHTML(content string) string {
	code, lang := stripCodeFence(content)
	css := `
.code-block { background: #0d1117; border: 1px solid #30363d; border-radius: 8px; padding: 16px; overflow-x: auto; }
.code-lang { color: #00d4aa; font-size: 0.85em; margin-bottom: 8px; }
pre { font-family: 'SF Mono', Monaco, 'Fira Code', monospace; font-size: 0.9em; line-height: 1.5; white-space: pre; }
`
	var langLabel string
	if lang != "" {
		langLabel = `<div class="code-lang">` + html.EscapeString(lang) + `</div>`
	}
	body := `<div class="code-block">
` + langLabel + `
<pre>` + html.EscapeString(code) + `</pre>
</div>`
	return htmlPage("Code", css, body)
}

func renderTableHTML(content string) string {
	pt := parseTableContent(content)
	if pt == nil {
		return renderShortHTML(content)
	}
	css := `
table { width: 100%; border-collapse: collapse; }
th { background: #16213e; color: #00d4aa; text-align: left; padding: 10px 14px; font-weight: 600; }
td { padding: 10px 14px; border-bottom: 1px solid #1e2a45; }
tr:nth-child(even) td { background: rgba(255,255,255,0.02); }
tr:hover td { background: rgba(0,212,170,0.08); }
`
	var b strings.Builder
	b.WriteString("<h1>Data</h1>\n<table>\n<thead><tr>")
	for _, h := range pt.headers {
		b.WriteString("<th>" + html.EscapeString(strings.TrimSpace(h)) + "</th>")
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range pt.rows {
		b.WriteString("<tr>")
		for i := 0; i < len(pt.headers); i++ {
			var cell string
			if i < len(row) {
				cell = strings.TrimSpace(row[i])
			}
			b.WriteString("<td>" + html.EscapeString(cell) + "</td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>")
	return htmlPage("Data", css, b.String())
}

func renderListHTML(content string) string {
	lines := splitLines(content)
	isOrdered := false
	if len(lines) > 0 {
		isOrdered = parseNumberedPrefix(strings.TrimSpace(lines[0])) != ""
	}
	css := `
ul, ol { padding-left: 0; list-style: none; }
li { padding: 8px 0; border-bottom: 1px solid rgba(255,255,255,0.05); position: relative; padding-left: 20px; }
li::before { color: #00d4aa; position: absolute; left: 0; }
ul li::before { content: '•'; }
`

	tag := "ul"
	if isOrdered {
		tag = "ol"
		css += `ol { counter-reset: item; }
ol li::before { counter-increment: item; content: counter(item) '.'; font-weight: 600; }
`
	}

	var b strings.Builder
	b.WriteString("<h1>Results</h1>\n<" + tag + ">\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip bullet/number prefix.
		text := trimmed
		if strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "* ") {
			text = text[2:]
		} else if strings.HasPrefix(text, "• ") {
			text = strings.TrimPrefix(text, "• ")
		} else if pfx := parseNumberedPrefix(text); pfx != "" {
			text = text[len(pfx):]
		}
		b.WriteString("<li>" + html.EscapeString(text) + "</li>\n")
	}
	b.WriteString("</" + tag + ">")
	return htmlPage("Results", css, b.String())
}

func renderKVHTML(content string) string {
	pairs := parseKVContent(content)
	if len(pairs) == 0 {
		return renderShortHTML(content)
	}
	css := `
.kv-grid { display: grid; grid-template-columns: auto 1fr; gap: 6px 16px; }
.kv-key { color: #00d4aa; font-weight: 600; text-align: right; padding: 6px 0; }
.kv-val { padding: 6px 0; border-bottom: 1px solid rgba(255,255,255,0.05); }
`
	var b strings.Builder
	b.WriteString("<h1>Details</h1>\n<div class=\"kv-grid\">\n")
	for _, p := range pairs {
		b.WriteString("<div class=\"kv-key\">" + html.EscapeString(p.key) + "</div>")
		b.WriteString("<div class=\"kv-val\">" + html.EscapeString(p.value) + "</div>\n")
	}
	b.WriteString("</div>")
	return htmlPage("Details", css, b.String())
}

func renderJSONHTML(content string) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(content), "", "  "); err != nil {
		return renderCodeHTML(content)
	}
	return renderCodeHTML("```json\n" + pretty.String() + "\n```")
}

// ── Helpers ──────────────────────────────────────────────────────────

func renderEmpty(format UIFormat) string {
	switch format {
	case FormatANSI:
		return "\033[90m(no output)\033[0m\n\033[0m"
	case FormatHTML:
		return htmlPage("Result", "", "<p style=\"color:#666\">No output</p>")
	default:
		return ""
	}
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func stripCodeFence(content string) (code string, lang string) {
	lines := splitLines(content)
	if len(lines) == 0 {
		return content, ""
	}
	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "```") {
		return content, ""
	}
	lang = strings.TrimSpace(strings.TrimPrefix(first, "```"))

	endIdx := len(lines) - 1
	for endIdx > 0 {
		if strings.TrimSpace(lines[endIdx]) == "```" {
			break
		}
		endIdx--
	}
	if endIdx > 0 {
		code = strings.Join(lines[1:endIdx], "\n")
	} else {
		code = strings.Join(lines[1:], "\n")
	}
	return code, lang
}

type parsedTable struct {
	headers []string
	rows    [][]string
}

func parseTableContent(content string) *parsedTable {
	lines := splitLines(content)

	// Try pipe-separated.
	var dataLines [][]string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isSeparatorLine(line) {
			continue
		}
		cells := parsePipeLine(line)
		if len(cells) >= 2 {
			dataLines = append(dataLines, cells)
		}
	}
	if len(dataLines) >= 2 {
		return &parsedTable{headers: dataLines[0], rows: dataLines[1:]}
	}

	// Try tab-separated.
	dataLines = nil
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) >= 2 {
			trimmed := make([]string, len(cells))
			for i, c := range cells {
				trimmed[i] = strings.TrimSpace(c)
			}
			dataLines = append(dataLines, trimmed)
		}
	}
	if len(dataLines) >= 2 {
		return &parsedTable{headers: dataLines[0], rows: dataLines[1:]}
	}
	return nil
}

func parsePipeLine(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func isSeparatorLine(line string) bool {
	clean := strings.NewReplacer("|", "", "-", "", "+", "", ":", "", " ", "").Replace(strings.TrimSpace(line))
	return clean == "" && len(strings.TrimSpace(line)) > 0
}

type kvPair struct {
	key   string
	value string
}

func parseKVContent(content string) []kvPair {
	var pairs []kvPair
	for _, line := range splitLines(content) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx > 0 && idx < 40 {
			pairs = append(pairs, kvPair{key: line[:idx], value: strings.TrimSpace(line[idx+2:])})
		} else if idx := strings.Index(line, " = "); idx > 0 && idx < 40 {
			pairs = append(pairs, kvPair{key: line[:idx], value: strings.TrimSpace(line[idx+3:])})
		}
	}
	return pairs
}

// parseNumberedPrefix returns the prefix like "1. " or "2) " from a line, or "".
func parseNumberedPrefix(line string) string {
	for i, c := range line {
		if c >= '0' && c <= '9' {
			continue
		}
		if (c == '.' || c == ')') && i > 0 && i+1 < len(line) && line[i+1] == ' ' {
			return line[:i+2]
		}
		break
	}
	return ""
}
