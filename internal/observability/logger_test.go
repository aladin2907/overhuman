package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("test-agent", &buf)
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
	if l.AgentName() != "test-agent" {
		t.Errorf("AgentName = %q", l.AgentName())
	}
}

func TestNewLogger_NilWriter(t *testing.T) {
	l := NewLogger("test", nil)
	if l == nil {
		t.Fatal("NewLogger with nil writer returned nil")
	}
	// Should not panic on log call.
	l.Info("test message")
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("myagent", &buf)
	l.Info("hello world", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "hello world") {
		t.Errorf("output missing message: %s", output)
	}
	if !strings.Contains(output, `"agent":"myagent"`) {
		t.Errorf("output missing agent: %s", output)
	}

	// Should be valid JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(output), &m); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.Debug("debug msg")

	if !strings.Contains(buf.String(), "debug msg") {
		t.Error("debug message not found")
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.Warn("warning msg")

	if !strings.Contains(buf.String(), "warning msg") {
		t.Error("warn message not found")
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.Error("error msg", "code", 500)

	output := buf.String()
	if !strings.Contains(output, "error msg") {
		t.Error("error message not found")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("expected ERROR level")
	}
}

func TestLogger_Pipeline(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.Pipeline(3, 10, "planning complete", "subtasks", 5)

	output := buf.String()
	if !strings.Contains(output, "planning complete") {
		t.Error("pipeline message not found")
	}
	if !strings.Contains(output, `"stage":3`) {
		t.Errorf("stage not found: %s", output)
	}
	if !strings.Contains(output, `"total_stages":10`) {
		t.Errorf("total_stages not found: %s", output)
	}
}

func TestLogger_SkillEvent(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.SkillEvent("executed", "sk_42", "cost", 0.003)

	output := buf.String()
	if !strings.Contains(output, `"event":"executed"`) {
		t.Errorf("event not found: %s", output)
	}
	if !strings.Contains(output, `"skill_id":"sk_42"`) {
		t.Errorf("skill_id not found: %s", output)
	}
}

func TestLogger_ReflectionEvent(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l.ReflectionEvent("meso", 0.85, "insight", "good job")

	output := buf.String()
	if !strings.Contains(output, `"reflection_level":"meso"`) {
		t.Errorf("reflection_level not found: %s", output)
	}
	if !strings.Contains(output, `"quality":0.85`) {
		t.Errorf("quality not found: %s", output)
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger("agent1", &buf)
	l2 := l.With("task_id", "t_123")

	l2.Info("with context")

	output := buf.String()
	if !strings.Contains(output, "t_123") {
		t.Errorf("With context not found: %s", output)
	}
	// Original logger should not have the context field.
	if l2.AgentName() != "agent1" {
		t.Errorf("AgentName = %q", l2.AgentName())
	}
}
