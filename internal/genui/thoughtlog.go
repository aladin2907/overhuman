package genui

import (
	"fmt"
	"strings"
)

// BuildThoughtLog creates a ThoughtLog from pipeline stage data.
func BuildThoughtLog(stages []ThoughtStage) *ThoughtLog {
	if len(stages) == 0 {
		return &ThoughtLog{}
	}

	var totalMs int64
	for _, s := range stages {
		totalMs += s.DurMs
	}

	return &ThoughtLog{
		Stages:  stages,
		TotalMs: totalMs,
	}
}

// FormatThoughtLogANSI renders a thought log as collapsible ANSI text.
func FormatThoughtLogANSI(t *ThoughtLog) string {
	if t == nil || len(t.Stages) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\033[90m▸ How the agent solved this (%dms)\033[0m\n", t.TotalMs))
	for i, s := range t.Stages {
		prefix := "├─"
		if i == len(t.Stages)-1 {
			prefix = "└─"
		}
		b.WriteString(fmt.Sprintf("\033[90m  %s Stage %d (%s): %s [%dms]\033[0m\n",
			prefix, s.Number, s.Name, s.Summary, s.DurMs))
	}

	return b.String()
}
