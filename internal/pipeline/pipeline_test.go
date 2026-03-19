package pipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
	"github.com/overhuman/overhuman/internal/senses"
	"github.com/overhuman/overhuman/internal/soul"
)

// mockLLMServer creates a test server that returns a constant response.
func mockLLMServer(t *testing.T) *httptest.Server {
	t.Helper()
	callCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		// Return Claude-formatted response.
		type contentBlock struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		}
		type response struct {
			ID      string         `json:"id"`
			Type    string         `json:"type"`
			Role    string         `json:"role"`
			Model   string         `json:"model"`
			Content []contentBlock `json:"content"`
			StopReason string     `json:"stop_reason"`
			Usage   struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		resp := response{
			ID:    "msg_test",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-20250514",
			Content: []contentBlock{
				{Type: "text", Text: "SCORE: 0.85\nNOTES: Task completed successfully."},
			},
			StopReason: "end_turn",
		}
		resp.Usage.InputTokens = 50
		resp.Usage.OutputTokens = 30

		json.NewEncoder(w).Encode(resp)
	}))
}

func setupDeps(t *testing.T, srvURL string) Dependencies {
	t.Helper()

	// Soul.
	soulDir, err := os.MkdirTemp("", "pipeline-soul-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(soulDir) })

	s := soul.New(soulDir, "TestAgent", "general")
	s.Initialize()

	// LLM.
	llm := brain.NewClaudeProvider("test-key", brain.WithClaudeBaseURL(srvURL))

	// Memory.
	dbPath := soulDir + "/test.db"
	ltm, err := memory.NewLongTermMemory(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ltm.Close() })

	pt, err := memory.NewPatternTracker(ltm.DB())
	if err != nil {
		t.Fatal(err)
	}

	return Dependencies{
		Soul:          s,
		LLM:           llm,
		Router:        brain.NewModelRouter(),
		Context:       brain.NewContextAssembler(),
		ShortTerm:     memory.NewShortTermMemory(50),
		LongTerm:      ltm,
		Patterns:      pt,
		AutoThreshold: 3,
	}
}

func TestPipeline_FullRun(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	input := senses.UnifiedInput{
		InputID:    "input_1",
		SourceType: senses.SourceText,
		Payload:    "Summarize the latest AI research papers",
		Priority:   senses.PriorityNormal,
		SourceMeta: senses.SourceMeta{
			Sender:    "user_1",
			Timestamp: time.Now(),
		},
	}

	result, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
	if result.TaskID == "" {
		t.Error("task ID should not be empty")
	}
	if result.Result == "" {
		t.Error("result should not be empty")
	}
	if result.CostUSD <= 0 {
		t.Error("cost should be > 0")
	}
	if result.ElapsedMs < 0 {
		t.Error("elapsed should be >= 0")
	}
	if result.Fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}
}

func TestPipeline_MemoryUpdated(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	input := senses.UnifiedInput{
		InputID:    "input_mem",
		SourceType: senses.SourceText,
		Payload:    "Test memory update",
	}

	_, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	// Short-term should have entries.
	stm := deps.ShortTerm.GetAll()
	if len(stm) < 2 {
		t.Errorf("expected at least 2 short-term entries, got %d", len(stm))
	}

	// Long-term should have entries.
	ltm, err := deps.LongTerm.GetAll(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ltm) < 1 {
		t.Error("expected at least 1 long-term entry")
	}
}

func TestPipeline_PatternTracking(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	deps.AutoThreshold = 3
	p := New(deps)

	// Run same task 3 times.
	for i := 0; i < 3; i++ {
		input := senses.UnifiedInput{
			InputID:    "input_pattern",
			SourceType: senses.SourceText,
			Payload:    "Generate unit tests for module X",
		}
		result, err := p.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}

		// On 3rd run, automation should be triggered.
		if i == 2 && !result.AutomationTriggered {
			t.Error("expected automation to be triggered on 3rd run")
		}
	}
}

func TestPipeline_Heartbeat(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	input := *senses.NewHeartbeat()

	result, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Error("heartbeat should succeed")
	}
}

// --- TaskSpec Tests ---

func TestNewTaskSpec(t *testing.T) {
	ts := NewTaskSpec("task_1", "Do something")

	if ts.ID != "task_1" {
		t.Errorf("ID = %q", ts.ID)
	}
	if ts.Goal != "Do something" {
		t.Errorf("Goal = %q", ts.Goal)
	}
	if ts.Status != TaskStatusDraft {
		t.Errorf("Status = %q, want draft", ts.Status)
	}
	if ts.Version != 1 {
		t.Errorf("Version = %d, want 1", ts.Version)
	}
}

func TestTaskSpec_Advance(t *testing.T) {
	ts := NewTaskSpec("task_1", "Test")

	ts.Advance(TaskStatusClarified)
	if ts.Version != 2 {
		t.Errorf("Version = %d, want 2", ts.Version)
	}
	if ts.Status != TaskStatusClarified {
		t.Errorf("Status = %q", ts.Status)
	}

	ts.Advance(TaskStatusPlanned)
	if ts.Version != 3 {
		t.Errorf("Version = %d, want 3", ts.Version)
	}
}

func TestPipeline_LLMError(t *testing.T) {
	// Server that always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"type":"server_error","message":"internal error"}}`))
	}))
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	input := senses.UnifiedInput{
		InputID:    "input_err",
		SourceType: senses.SourceText,
		Payload:    "This should fail",
	}

	result, err := p.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Error("should not be success")
	}
	if result.TaskID == "" {
		t.Error("task ID should still be set")
	}
}

// ---------------------------------------------------------------------------
// StageEvent / OnStageProgress / emitStage
// ---------------------------------------------------------------------------

func TestStageEvent_Fields(t *testing.T) {
	evt := StageEvent{
		TaskID:  "task_1",
		Stage:   5,
		Name:    "execute",
		Status:  "completed",
		Summary: "done",
		DurMs:   1234,
	}
	if evt.TaskID != "task_1" {
		t.Errorf("TaskID = %q", evt.TaskID)
	}
	if evt.Stage != 5 {
		t.Errorf("Stage = %d", evt.Stage)
	}
	if evt.Name != "execute" {
		t.Errorf("Name = %q", evt.Name)
	}
	if evt.Status != "completed" {
		t.Errorf("Status = %q", evt.Status)
	}
	if evt.Summary != "done" {
		t.Errorf("Summary = %q", evt.Summary)
	}
	if evt.DurMs != 1234 {
		t.Errorf("DurMs = %d", evt.DurMs)
	}
}

func TestOnStageProgress_Callback(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	var events []StageEvent
	p.OnStageProgress(func(evt StageEvent) {
		events = append(events, evt)
	})

	input := senses.UnifiedInput{
		InputID:    "input_stage",
		SourceType: senses.SourceText,
		Payload:    "Test stage callbacks",
	}

	_, _ = p.Run(context.Background(), input)

	// Should receive at least start+complete for each stage (up to 20 events for 10 stages).
	if len(events) == 0 {
		t.Fatal("no stage events received")
	}

	// Verify first event is stage 1 started.
	if events[0].Stage != 1 {
		t.Errorf("first event stage = %d, want 1", events[0].Stage)
	}
	if events[0].Status != "started" {
		t.Errorf("first event status = %q, want started", events[0].Status)
	}
	if events[0].Name != "intake" {
		t.Errorf("first event name = %q, want intake", events[0].Name)
	}

	// Verify task IDs are all non-empty and consistent.
	taskID := events[0].TaskID
	if taskID == "" {
		t.Error("task ID should not be empty")
	}
	for i, evt := range events {
		if evt.TaskID != taskID {
			t.Errorf("event %d: TaskID = %q, want %q", i, evt.TaskID, taskID)
		}
	}

	// Verify we got both "started" and "completed" statuses.
	hasStarted, hasCompleted := false, false
	for _, evt := range events {
		if evt.Status == "started" {
			hasStarted = true
		}
		if evt.Status == "completed" {
			hasCompleted = true
		}
	}
	if !hasStarted {
		t.Error("no 'started' events found")
	}
	if !hasCompleted {
		t.Error("no 'completed' events found")
	}
}

func TestOnStageProgress_NilCallbackSafe(t *testing.T) {
	srv := mockLLMServer(t)
	defer srv.Close()

	deps := setupDeps(t, srv.URL)
	p := New(deps)

	// Don't register any callback — should not panic.
	input := senses.UnifiedInput{
		InputID:    "input_nil_cb",
		SourceType: senses.SourceText,
		Payload:    "Test nil callback",
	}

	_, err := p.Run(context.Background(), input)
	// Should complete without panic (error from LLM is fine).
	_ = err
}

func TestEmitStage_WithCallback(t *testing.T) {
	p := New(Dependencies{AutoThreshold: 3})

	var received StageEvent
	p.OnStageProgress(func(evt StageEvent) {
		received = evt
	})

	p.emitStage("t1", 3, "plan", "started", "planning phase", 100)

	if received.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", received.TaskID)
	}
	if received.Stage != 3 {
		t.Errorf("Stage = %d, want 3", received.Stage)
	}
	if received.Name != "plan" {
		t.Errorf("Name = %q, want plan", received.Name)
	}
	if received.Status != "started" {
		t.Errorf("Status = %q, want started", received.Status)
	}
	if received.Summary != "planning phase" {
		t.Errorf("Summary = %q, want 'planning phase'", received.Summary)
	}
	if received.DurMs != 100 {
		t.Errorf("DurMs = %d, want 100", received.DurMs)
	}
}

func TestEmitStage_WithoutCallback(t *testing.T) {
	p := New(Dependencies{AutoThreshold: 3})
	// No callback — should not panic.
	p.emitStage("t1", 1, "intake", "started", "", 0)
}
