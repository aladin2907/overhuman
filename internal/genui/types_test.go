package genui

import (
	"encoding/json"
	"testing"
)

func TestGeneratedUI_MarshalJSON(t *testing.T) {
	ui := GeneratedUI{
		TaskID: "task-42",
		Format: FormatANSI,
		Code:   "\033[1mHello\033[0m",
		Actions: []GeneratedAction{
			{ID: "act-1", Label: "Apply Fix", Callback: "cb-apply"},
		},
		Meta: UIMeta{
			Title:   "Code Review",
			Summary: "All checks passed",
		},
	}

	data, err := json.Marshal(ui)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map error: %v", err)
	}

	if raw["task_id"] != "task-42" {
		t.Errorf("expected task_id=task-42, got %v", raw["task_id"])
	}
	if raw["format"] != "ansi" {
		t.Errorf("expected format=ansi, got %v", raw["format"])
	}

	actions, ok := raw["actions"].([]interface{})
	if !ok || len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", raw["actions"])
	}
}

func TestGeneratedUI_UnmarshalJSON(t *testing.T) {
	input := `{
		"task_id": "task-99",
		"format": "html",
		"code": "<div>Result</div>",
		"actions": [
			{"id": "a1", "label": "Deploy", "callback": "cb-deploy"},
			{"id": "a2", "label": "Rollback", "callback": "cb-rollback"}
		],
		"meta": {
			"title": "Deploy Result",
			"summary": "Completed in 320ms",
			"streaming": true
		}
	}`

	var ui GeneratedUI
	if err := json.Unmarshal([]byte(input), &ui); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if ui.TaskID != "task-99" {
		t.Errorf("expected task-99, got %s", ui.TaskID)
	}
	if ui.Format != FormatHTML {
		t.Errorf("expected html, got %s", ui.Format)
	}
	if ui.Code != "<div>Result</div>" {
		t.Errorf("unexpected code: %s", ui.Code)
	}
	if len(ui.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(ui.Actions))
	}
	if ui.Actions[0].Label != "Deploy" {
		t.Errorf("expected Deploy, got %s", ui.Actions[0].Label)
	}
	if ui.Actions[1].Callback != "cb-rollback" {
		t.Errorf("expected cb-rollback, got %s", ui.Actions[1].Callback)
	}
	if ui.Meta.Title != "Deploy Result" {
		t.Errorf("expected 'Deploy Result', got %s", ui.Meta.Title)
	}
	if !ui.Meta.Streaming {
		t.Error("expected streaming=true")
	}
}

func TestDeviceCapabilities_CLI(t *testing.T) {
	caps := CLICapabilities()

	if caps.Format != FormatANSI {
		t.Errorf("expected format %s, got %s", FormatANSI, caps.Format)
	}
	if caps.Width != 80 {
		t.Errorf("expected width 80, got %d", caps.Width)
	}
	if caps.Height != 24 {
		t.Errorf("expected height 24, got %d", caps.Height)
	}
	if caps.ColorDepth != 256 {
		t.Errorf("expected color depth 256, got %d", caps.ColorDepth)
	}
	if !caps.Interactive {
		t.Error("expected interactive=true")
	}
	if caps.JavaScript {
		t.Error("expected javascript=false for CLI")
	}
	if caps.SVG {
		t.Error("expected svg=false for CLI")
	}
	if caps.Animation {
		t.Error("expected animation=false for CLI")
	}
	if caps.TouchScreen {
		t.Error("expected touch_screen=false for CLI")
	}
}

func TestDeviceCapabilities_Web(t *testing.T) {
	caps := WebCapabilities(1280, 800)

	if caps.Format != FormatHTML {
		t.Errorf("expected format %s, got %s", FormatHTML, caps.Format)
	}
	if caps.Width != 1280 {
		t.Errorf("expected width 1280, got %d", caps.Width)
	}
	if caps.Height != 800 {
		t.Errorf("expected height 800, got %d", caps.Height)
	}
	if caps.ColorDepth != 16777216 {
		t.Errorf("expected 24-bit color (16777216), got %d", caps.ColorDepth)
	}
	if !caps.Interactive {
		t.Error("expected interactive=true")
	}
	if !caps.JavaScript {
		t.Error("expected javascript=true for web")
	}
	if !caps.SVG {
		t.Error("expected svg=true for web")
	}
	if !caps.Animation {
		t.Error("expected animation=true for web")
	}
	if caps.TouchScreen {
		t.Error("expected touch_screen=false for web")
	}
}

func TestDeviceCapabilities_Tablet(t *testing.T) {
	caps := TabletCapabilities(1024, 768)

	if caps.Format != FormatHTML {
		t.Errorf("expected format %s, got %s", FormatHTML, caps.Format)
	}
	if caps.Width != 1024 {
		t.Errorf("expected width 1024, got %d", caps.Width)
	}
	if caps.Height != 768 {
		t.Errorf("expected height 768, got %d", caps.Height)
	}
	if !caps.TouchScreen {
		t.Error("expected touch_screen=true for tablet")
	}
	if !caps.JavaScript {
		t.Error("expected javascript=true for tablet")
	}
	if !caps.SVG {
		t.Error("expected svg=true for tablet")
	}
	if !caps.Animation {
		t.Error("expected animation=true for tablet")
	}
}

func TestUIFormat_Constants(t *testing.T) {
	tests := []struct {
		format UIFormat
		want   string
	}{
		{FormatANSI, "ansi"},
		{FormatHTML, "html"},
		{FormatMarkdown, "markdown"},
	}

	for _, tt := range tests {
		if string(tt.format) != tt.want {
			t.Errorf("format constant %v: expected %q, got %q", tt.format, tt.want, string(tt.format))
		}
	}

	// Verify they are all distinct
	seen := map[UIFormat]bool{}
	for _, tt := range tests {
		if seen[tt.format] {
			t.Errorf("duplicate format constant: %s", tt.format)
		}
		seen[tt.format] = true
	}
}

func TestUIReflection_MarshalJSON(t *testing.T) {
	r := UIReflection{
		TaskID:       "task-reflect",
		UIFormat:     FormatHTML,
		ActionsShown: []string{"apply", "dismiss"},
		ActionsUsed:  []string{"apply"},
		TimeToAction: 2300,
		Scrolled:     true,
		Dismissed:    false,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Round-trip: unmarshal back and verify
	var got UIReflection
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got.TaskID != r.TaskID {
		t.Errorf("TaskID: expected %s, got %s", r.TaskID, got.TaskID)
	}
	if got.UIFormat != r.UIFormat {
		t.Errorf("UIFormat: expected %s, got %s", r.UIFormat, got.UIFormat)
	}
	if len(got.ActionsShown) != 2 {
		t.Errorf("ActionsShown: expected 2, got %d", len(got.ActionsShown))
	}
	if len(got.ActionsUsed) != 1 || got.ActionsUsed[0] != "apply" {
		t.Errorf("ActionsUsed: expected [apply], got %v", got.ActionsUsed)
	}
	if got.TimeToAction != 2300 {
		t.Errorf("TimeToAction: expected 2300, got %d", got.TimeToAction)
	}
	if !got.Scrolled {
		t.Error("Scrolled: expected true")
	}
	if got.Dismissed {
		t.Error("Dismissed: expected false")
	}
}
