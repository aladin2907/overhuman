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
