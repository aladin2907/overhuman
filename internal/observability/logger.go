// Package observability provides structured logging and metrics collection.
//
// Logger wraps log/slog with agent-specific context fields.
// Metrics collects run statistics, quality scores, costs, and skill fitness.
package observability

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

// Logger wraps slog with persistent agent context.
type Logger struct {
	mu     sync.RWMutex
	inner  *slog.Logger
	agent  string
	fields []slog.Attr
}

// NewLogger creates a structured logger for a given agent.
// Output defaults to os.Stderr if w is nil.
func NewLogger(agentName string, w io.Writer) *Logger {
	if w == nil {
		w = os.Stderr
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &Logger{
		inner: slog.New(handler),
		agent: agentName,
	}
}

// NewLoggerWithHandler creates a logger with a custom slog handler.
func NewLoggerWithHandler(agentName string, h slog.Handler) *Logger {
	return &Logger{
		inner: slog.New(h),
		agent: agentName,
	}
}

// With returns a new Logger with additional persistent fields.
func (l *Logger) With(key string, value any) *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return &Logger{
		inner:  l.inner.With(slog.Any(key, value)),
		agent:  l.agent,
		fields: append(l.fields, slog.Any(key, value)),
	}
}

// attrs prepends agent name to the arguments.
func (l *Logger) attrs(msg string, args []any) (string, []any) {
	return msg, append([]any{slog.String("agent", l.agent)}, args...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	msg, args = l.attrs(msg, args)
	l.inner.Debug(msg, args...)
}

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) {
	msg, args = l.attrs(msg, args)
	l.inner.Info(msg, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	msg, args = l.attrs(msg, args)
	l.inner.Warn(msg, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	msg, args = l.attrs(msg, args)
	l.inner.Error(msg, args...)
}

// Pipeline logs a pipeline stage event.
func (l *Logger) Pipeline(stage int, total int, msg string, args ...any) {
	allArgs := append([]any{
		slog.String("agent", l.agent),
		slog.Int("stage", stage),
		slog.Int("total_stages", total),
	}, args...)
	l.inner.Info(msg, allArgs...)
}

// SkillEvent logs a skill-related event.
func (l *Logger) SkillEvent(event, skillID string, args ...any) {
	allArgs := append([]any{
		slog.String("agent", l.agent),
		slog.String("event", event),
		slog.String("skill_id", skillID),
	}, args...)
	l.inner.Info("skill", allArgs...)
}

// ReflectionEvent logs a reflection cycle event.
func (l *Logger) ReflectionEvent(level string, quality float64, args ...any) {
	allArgs := append([]any{
		slog.String("agent", l.agent),
		slog.String("reflection_level", level),
		slog.Float64("quality", quality),
	}, args...)
	l.inner.Info("reflection", allArgs...)
}

// AgentName returns the agent name associated with this logger.
func (l *Logger) AgentName() string {
	return l.agent
}
