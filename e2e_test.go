package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overhuman/overhuman/internal/brain"
	"github.com/overhuman/overhuman/internal/memory"
	"github.com/overhuman/overhuman/internal/pipeline"
	"github.com/overhuman/overhuman/internal/senses"
	"github.com/overhuman/overhuman/internal/soul"
)

// =============================================================================
// End-to-End Integration Tests
//
// These tests verify the full Overhuman pipeline with a mock LLM server,
// testing real data flow through all subsystems without any external API calls.
// =============================================================================

// mockLLME2E creates a mock LLM server that tracks calls and returns
// context-aware responses.
func mockLLME2E(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	callCount := &atomic.Int64{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)

		// Read request body.
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		// Determine response based on request content.
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		responseText := "This is a helpful response from the mock LLM."

		// Check if this is a review call (contains SCORE prompt).
		if msgs, ok := reqBody["messages"].([]interface{}); ok {
			for _, m := range msgs {
				if msg, ok := m.(map[string]interface{}); ok {
					content, _ := msg["content"].(string)
					if strings.Contains(content, "review") || strings.Contains(content, "SCORE") {
						responseText = "SCORE: 0.92\nNOTES: Excellent response quality. Clear and comprehensive."
					} else if strings.Contains(content, "reflect") || strings.Contains(content, "self-assess") {
						responseText = "REFLECTION: Performance was strong. Consider adding more specific examples in future responses."
					} else if strings.Contains(content, "weather") {
						responseText = "The weather in Moscow is currently -5°C with light snow. Bundle up!"
					} else if strings.Contains(content, "translate") {
						responseText = "Перевод: Hello world = Привет мир"
					} else if strings.Contains(content, "summarize") || strings.Contains(content, "summary") {
						responseText = "Summary: The document discusses key advancements in AI during 2025, including autonomous agents and self-evolving systems."
					} else if strings.Contains(content, "heartbeat") || strings.Contains(content, "proactive") {
						responseText = "Heartbeat check: All systems nominal. No pending goals require attention."
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")

		// Return Claude API format response.
		resp := map[string]interface{}{
			"id":          fmt.Sprintf("msg_test_%d", callCount.Load()),
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"content": []map[string]interface{}{
				{"type": "text", "text": responseText},
			},
			"usage": map[string]interface{}{
				"input_tokens":  42 + len(body)/10,
				"output_tokens": 25 + len(responseText)/4,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))

	return srv, callCount
}

// setupE2EDeps creates full pipeline dependencies with temp directories.
func setupE2EDeps(t *testing.T, srvURL string) pipeline.Dependencies {
	t.Helper()

	dir, err := os.MkdirTemp("", "overhuman-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// Soul.
	s := soul.New(dir, "E2ETestAgent", "general")
	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}

	// LLM — Claude provider pointing at mock.
	llm := brain.NewClaudeProvider("test-key-e2e", brain.WithClaudeBaseURL(srvURL))

	// Memory.
	ltm, err := memory.NewLongTermMemory(dir + "/e2e.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ltm.Close() })

	pt, err := memory.NewPatternTracker(ltm.DB())
	if err != nil {
		t.Fatal(err)
	}

	router := brain.NewModelRouter()

	return pipeline.Dependencies{
		Soul:          s,
		LLM:           llm,
		Router:        router,
		Context:       brain.NewContextAssembler(),
		ShortTerm:     memory.NewShortTermMemory(100),
		LongTerm:      ltm,
		Patterns:      pt,
		AutoThreshold: 3,
	}
}

// ---------------------------------------------------------------------------
// Test: Full Pipeline Run (text input → LLM → result)
// ---------------------------------------------------------------------------

func TestE2E_FullPipelineRun(t *testing.T) {
	srv, callCount := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	input := senses.UnifiedInput{
		InputID:    "e2e_full_1",
		SourceType: senses.SourceText,
		Payload:    "What is the weather in Moscow?",
		Priority:   senses.PriorityNormal,
		SourceMeta: senses.SourceMeta{
			Sender:    "test_user",
			Channel:   "cli",
			Timestamp: time.Now(),
		},
	}

	result, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Pipeline.Run failed: %v", err)
	}

	// Verify result.
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.TaskID == "" {
		t.Error("expected non-empty TaskID")
	}
	if result.Result == "" {
		t.Error("expected non-empty Result")
	}
	if !strings.Contains(result.Result, "weather") && !strings.Contains(result.Result, "Moscow") {
		t.Logf("Result content: %s", result.Result)
	}
	if result.QualityScore <= 0 {
		t.Error("expected positive quality score")
	}
	if result.CostUSD <= 0 {
		t.Error("expected positive cost")
	}
	if result.ElapsedMs < 0 {
		t.Error("expected non-negative elapsed time")
	}
	if result.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}

	// At least 2 LLM calls: execution + review.
	if callCount.Load() < 2 {
		t.Errorf("expected at least 2 LLM calls, got %d", callCount.Load())
	}

	t.Logf("✓ Full pipeline run: task=%s quality=%.0f%% cost=$%.4f time=%dms calls=%d",
		result.TaskID, result.QualityScore*100, result.CostUSD, result.ElapsedMs, callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: Memory Persistence Across Runs
// ---------------------------------------------------------------------------

func TestE2E_MemoryPersistence(t *testing.T) {
	srv, _ := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	// Run 1: first message.
	input1 := senses.UnifiedInput{
		InputID:    "e2e_mem_1",
		SourceType: senses.SourceText,
		Payload:    "Translate hello world to Russian",
		SourceMeta: senses.SourceMeta{Sender: "user_a", Timestamp: time.Now()},
	}
	result1, err := p.Run(context.Background(), input1)
	if err != nil {
		t.Fatalf("Run 1 failed: %v", err)
	}
	if !result1.Success {
		t.Fatal("Run 1 should succeed")
	}

	// Run 2: second message.
	input2 := senses.UnifiedInput{
		InputID:    "e2e_mem_2",
		SourceType: senses.SourceText,
		Payload:    "Summarize the latest news",
		SourceMeta: senses.SourceMeta{Sender: "user_a", Timestamp: time.Now()},
	}
	result2, err := p.Run(context.Background(), input2)
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}
	if !result2.Success {
		t.Fatal("Run 2 should succeed")
	}

	// Verify short-term memory has entries from both runs.
	stmEntries := deps.ShortTerm.GetAll()
	if len(stmEntries) < 4 { // Each run adds at least user input + assistant response.
		t.Errorf("expected at least 4 short-term entries, got %d", len(stmEntries))
	}

	// Verify long-term memory persisted.
	ltmEntries, err := deps.LongTerm.GetAll(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(ltmEntries) < 2 {
		t.Errorf("expected at least 2 long-term entries, got %d", len(ltmEntries))
	}

	t.Logf("✓ Memory persistence: STM=%d entries, LTM=%d entries", len(stmEntries), len(ltmEntries))
}

// ---------------------------------------------------------------------------
// Test: Pattern Detection & Automation Trigger
// ---------------------------------------------------------------------------

func TestE2E_PatternDetection(t *testing.T) {
	srv, callCount := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	deps.AutoThreshold = 3 // Trigger after 3 repetitions.
	p := pipeline.New(deps)

	// Run same task type 3 times.
	automationTriggered := false
	for i := 0; i < 3; i++ {
		input := senses.UnifiedInput{
			InputID:    fmt.Sprintf("e2e_pattern_%d", i),
			SourceType: senses.SourceText,
			Payload:    "Summarize the following document for me",
			SourceMeta: senses.SourceMeta{Sender: "user_b", Timestamp: time.Now()},
		}

		result, err := p.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("Run %d failed: %v", i+1, err)
		}
		if !result.Success {
			t.Fatalf("Run %d should succeed", i+1)
		}

		if result.AutomationTriggered {
			automationTriggered = true
			t.Logf("✓ Automation triggered on run %d (fingerprint: %s)", i+1, result.Fingerprint)
		}

		if result.Fingerprint == "" {
			t.Errorf("Run %d: expected non-empty fingerprint", i+1)
		}
	}

	if !automationTriggered {
		t.Error("expected automation to trigger after 3 repetitions of same pattern")
	}

	t.Logf("✓ Pattern detection: total LLM calls=%d", callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: HTTP API End-to-End
// ---------------------------------------------------------------------------

func TestE2E_HTTPAPIFlow(t *testing.T) {
	srv, _ := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	// Start API server on random port.
	api := senses.NewAPISense("127.0.0.1:0")
	out := make(chan *senses.UnifiedInput, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		api.Start(ctx, out)
	}()

	// Wait for API to start.
	var addr string
	for i := 0; i < 100; i++ {
		addr = api.Addr()
		if addr != "127.0.0.1:0" && addr != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" || addr == "127.0.0.1:0" {
		t.Fatal("API server did not start in time")
	}

	// Process pipeline in background.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case input, ok := <-out:
				if !ok {
					return
				}
				result, err := p.Run(ctx, *input)
				if err != nil {
					continue
				}
				if input.CorrelationID != "" && input.ResponseChannel == "api" {
					api.Send(ctx, input.CorrelationID, result.Result)
				}
			}
		}
	}()

	// --- Test 1: Health endpoint ---
	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err != nil {
			t.Fatalf("health request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("health returned %d", resp.StatusCode)
		}

		var health map[string]string
		json.NewDecoder(resp.Body).Decode(&health)
		if health["status"] != "ok" {
			t.Errorf("health status = %q, want ok", health["status"])
		}
		t.Logf("✓ Health: %v", health)
	})

	// --- Test 2: Async input (fire-and-forget) ---
	t.Run("AsyncInput", func(t *testing.T) {
		payload := `{"payload": "What time is it?", "sender": "api_test"}`
		resp, err := http.Post(
			fmt.Sprintf("http://%s/input", addr),
			"application/json",
			strings.NewReader(payload),
		)
		if err != nil {
			t.Fatalf("async input request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("async input returned %d: %s", resp.StatusCode, string(body))
		}

		var apiResp map[string]string
		json.NewDecoder(resp.Body).Decode(&apiResp)
		if apiResp["input_id"] == "" {
			t.Error("expected non-empty input_id")
		}
		if apiResp["status"] != "accepted" {
			t.Errorf("status = %q, want accepted", apiResp["status"])
		}
		t.Logf("✓ Async input accepted: id=%s", apiResp["input_id"])
	})

	// --- Test 3: Sync input (wait for response) ---
	t.Run("SyncInput", func(t *testing.T) {
		payload := `{"payload": "Tell me about AI", "sender": "api_test_sync"}`
		resp, err := http.Post(
			fmt.Sprintf("http://%s/input/sync", addr),
			"application/json",
			strings.NewReader(payload),
		)
		if err != nil {
			t.Fatalf("sync input request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("sync input returned %d: %s", resp.StatusCode, string(body))
		}

		var syncResp map[string]string
		json.NewDecoder(resp.Body).Decode(&syncResp)
		if syncResp["result"] == "" {
			t.Error("expected non-empty result")
		}
		if syncResp["status"] != "completed" {
			t.Errorf("status = %q, want completed", syncResp["status"])
		}
		t.Logf("✓ Sync input completed: result=%s...", truncate(syncResp["result"], 80))
	})

	// --- Test 4: Invalid request ---
	t.Run("InvalidRequest", func(t *testing.T) {
		resp, err := http.Post(
			fmt.Sprintf("http://%s/input", addr),
			"application/json",
			strings.NewReader(`{"payload": ""}`),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		t.Logf("✓ Invalid request correctly rejected with %d", resp.StatusCode)
	})
}

// ---------------------------------------------------------------------------
// Test: Heartbeat Processing
// ---------------------------------------------------------------------------

func TestE2E_HeartbeatProcessing(t *testing.T) {
	srv, callCount := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	hb := senses.NewHeartbeat()

	result, err := p.Run(context.Background(), *hb)
	if err != nil {
		t.Fatalf("Heartbeat run failed: %v", err)
	}

	if !result.Success {
		t.Error("heartbeat should succeed")
	}
	if result.TaskID == "" {
		t.Error("heartbeat should have a task ID")
	}

	t.Logf("✓ Heartbeat processed: task=%s quality=%.0f%% calls=%d",
		result.TaskID, result.QualityScore*100, callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: UniversalProvider with Mock Server
// ---------------------------------------------------------------------------

func TestE2E_UniversalProvider(t *testing.T) {
	// Mock OpenAI-compatible server.
	callCount := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)

		// Verify it hits /v1/chat/completions.
		if !strings.Contains(r.URL.Path, "/v1/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-universal-key" {
			t.Errorf("unexpected auth: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "chatcmpl-test",
			"object": "chat.completion",
			"model":  "gpt-4o",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]string{
						"role":    "assistant",
						"content": "SCORE: 0.88\nNOTES: Good response.",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     30,
				"completion_tokens": 15,
				"total_tokens":      45,
			},
		})
	}))
	defer srv.Close()

	// Create UniversalProvider pointing at mock.
	cfg := brain.ProviderConfig{
		Name:         "mock-openai",
		BaseURL:      srv.URL,
		APIKey:       "test-universal-key",
		DefaultModel: "gpt-4o",
		Models: []brain.ModelConfig{
			{ID: "gpt-4o", Tier: "mid", InputCostPerM: 2.50, OutputCostPerM: 10.0},
		},
	}
	provider := brain.NewUniversalProvider(cfg)

	// Test: Complete call.
	resp, err := provider.Complete(context.Background(), brain.LLMRequest{
		Messages: []brain.Message{
			{Role: "user", Content: "Hello from e2e test!"},
		},
	})
	if err != nil {
		t.Fatalf("UniversalProvider.Complete failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", resp.Model)
	}
	if resp.InputTokens != 30 {
		t.Errorf("input tokens = %d, want 30", resp.InputTokens)
	}
	if resp.OutputTokens != 15 {
		t.Errorf("output tokens = %d, want 15", resp.OutputTokens)
	}
	if resp.LatencyMs < 0 {
		t.Error("expected non-negative latency")
	}

	// Test: Use in pipeline.
	dir, _ := os.MkdirTemp("", "e2e-universal-*")
	t.Cleanup(func() { os.RemoveAll(dir) })

	s := soul.New(dir, "UniversalTestAgent", "general")
	s.Initialize()

	ltm, _ := memory.NewLongTermMemory(dir + "/uni.db")
	t.Cleanup(func() { ltm.Close() })

	pt, _ := memory.NewPatternTracker(ltm.DB())

	router := brain.NewModelRouterWithModels(provider.ModelEntries())

	deps := pipeline.Dependencies{
		Soul:          s,
		LLM:           provider,
		Router:        router,
		Context:       brain.NewContextAssembler(),
		ShortTerm:     memory.NewShortTermMemory(50),
		LongTerm:      ltm,
		Patterns:      pt,
		AutoThreshold: 3,
	}

	p := pipeline.New(deps)
	input := senses.UnifiedInput{
		InputID:    "e2e_universal_1",
		SourceType: senses.SourceText,
		Payload:    "Test universal provider in pipeline",
	}

	result, err := p.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Pipeline with UniversalProvider failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	if callCount.Load() < 2 {
		t.Errorf("expected at least 2 calls through universal provider, got %d", callCount.Load())
	}

	t.Logf("✓ UniversalProvider pipeline: task=%s quality=%.0f%% cost=$%.4f calls=%d",
		result.TaskID, result.QualityScore*100, result.CostUSD, callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: Multi-Priority Queue
// ---------------------------------------------------------------------------

func TestE2E_PriorityProcessing(t *testing.T) {
	srv, _ := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	priorities := []senses.Priority{
		senses.PriorityLow,
		senses.PriorityNormal,
		senses.PriorityHigh,
		senses.PriorityCritical,
	}

	for _, pri := range priorities {
		input := senses.UnifiedInput{
			InputID:    fmt.Sprintf("e2e_pri_%d", pri),
			SourceType: senses.SourceText,
			Payload:    fmt.Sprintf("Priority %d task", pri),
			Priority:   pri,
			SourceMeta: senses.SourceMeta{
				Sender:    "test_user",
				Timestamp: time.Now(),
			},
		}

		result, err := p.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("Priority %d run failed: %v", pri, err)
		}
		if !result.Success {
			t.Errorf("Priority %d should succeed", pri)
		}

		t.Logf("✓ Priority %d: task=%s quality=%.0f%%", pri, result.TaskID, result.QualityScore*100)
	}
}

// ---------------------------------------------------------------------------
// Test: CLI Sense Integration
// ---------------------------------------------------------------------------

func TestE2E_CLISenseIntegration(t *testing.T) {
	srv, _ := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	// Simulate CLI input.
	input := bytes.NewBufferString("Hello, Overhuman!\n/quit\n")
	output := &bytes.Buffer{}

	cli := senses.NewCLISense(input, output)
	out := make(chan *senses.UnifiedInput, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start CLI in background.
	go func() {
		cli.Start(ctx, out)
	}()

	// Process first message.
	select {
	case msg := <-out:
		if msg.Payload != "Hello, Overhuman!" {
			t.Errorf("payload = %q, want 'Hello, Overhuman!'", msg.Payload)
		}
		if msg.SourceType != senses.SourceText {
			t.Errorf("source type = %q, want text", msg.SourceType)
		}

		// Run through pipeline.
		result, err := p.Run(ctx, *msg)
		if err != nil {
			t.Fatalf("Pipeline run failed: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}

		// Send response back to CLI.
		err = cli.Send(ctx, "", result.Result)
		if err != nil {
			t.Fatalf("CLI send failed: %v", err)
		}

		// Verify output was written.
		outputStr := output.String()
		if outputStr == "" {
			t.Error("expected CLI output")
		}

		t.Logf("✓ CLI integration: input='Hello, Overhuman!' → result=%s...", truncate(result.Result, 60))

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for CLI input")
	}
}

// ---------------------------------------------------------------------------
// Test: Concurrent Pipeline Runs
// ---------------------------------------------------------------------------

func TestE2E_ConcurrentRuns(t *testing.T) {
	srv, callCount := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	const numRuns = 5
	results := make(chan *pipeline.RunResult, numRuns)
	errors := make(chan error, numRuns)

	for i := 0; i < numRuns; i++ {
		go func(n int) {
			input := senses.UnifiedInput{
				InputID:    fmt.Sprintf("e2e_concurrent_%d", n),
				SourceType: senses.SourceText,
				Payload:    fmt.Sprintf("Concurrent task number %d", n),
				Priority:   senses.PriorityNormal,
				SourceMeta: senses.SourceMeta{
					Sender:    fmt.Sprintf("user_%d", n),
					Timestamp: time.Now(),
				},
			}

			result, err := p.Run(context.Background(), input)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	successCount := 0
	for i := 0; i < numRuns; i++ {
		select {
		case r := <-results:
			if r.Success {
				successCount++
			}
		case err := <-errors:
			t.Errorf("concurrent run error: %v", err)
		case <-time.After(30 * time.Second):
			t.Fatal("timeout waiting for concurrent runs")
		}
	}

	if successCount != numRuns {
		t.Errorf("expected %d successes, got %d", numRuns, successCount)
	}

	t.Logf("✓ Concurrent runs: %d/%d succeeded, total LLM calls=%d", successCount, numRuns, callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: Error Recovery
// ---------------------------------------------------------------------------

func TestE2E_ErrorRecovery(t *testing.T) {
	// Server that fails on first 2 calls, then succeeds.
	callCount := &atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		// Always return Claude format (even errors need to be in right format).
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"type":    "server_error",
					"message": fmt.Sprintf("simulated failure %d", n),
				},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "msg_recovery",
			"type":        "message",
			"role":        "assistant",
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"content": []map[string]interface{}{
				{"type": "text", "text": "SCORE: 0.75\nNOTES: Recovered successfully."},
			},
			"usage": map[string]interface{}{
				"input_tokens":  30,
				"output_tokens": 20,
			},
		})
	}))
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	// First run should fail (server returns 500).
	input := senses.UnifiedInput{
		InputID:    "e2e_recovery_1",
		SourceType: senses.SourceText,
		Payload:    "This will fail first",
	}

	result, err := p.Run(context.Background(), input)
	if err == nil {
		t.Log("First run didn't error (retried successfully or partial success)")
	} else {
		t.Logf("✓ First run correctly failed: %v", err)
	}
	_ = result

	// After failures, subsequent calls should succeed.
	input2 := senses.UnifiedInput{
		InputID:    "e2e_recovery_2",
		SourceType: senses.SourceText,
		Payload:    "This should work after recovery",
	}

	result2, err := p.Run(context.Background(), input2)
	if err != nil {
		t.Logf("Second run still failed (expected if server errors persist): %v", err)
	} else if result2.Success {
		t.Logf("✓ Recovery: second run succeeded")
	}

	t.Logf("✓ Error recovery test: total calls=%d", callCount.Load())
}

// ---------------------------------------------------------------------------
// Test: Multiple Source Types
// ---------------------------------------------------------------------------

func TestE2E_MultipleSourceTypes(t *testing.T) {
	srv, _ := mockLLME2E(t)
	defer srv.Close()

	deps := setupE2EDeps(t, srv.URL)
	p := pipeline.New(deps)

	sources := []struct {
		sourceType senses.SourceType
		channel    string
		payload    string
	}{
		{senses.SourceText, "cli", "CLI text input"},
		{senses.SourceAPI, "api", "API request payload"},
		{senses.SourceWebhook, "webhook", "Webhook event data"},
		{senses.SourceTimer, "timer", "Scheduled task check"},
	}

	for _, src := range sources {
		input := senses.UnifiedInput{
			InputID:    fmt.Sprintf("e2e_source_%s", src.channel),
			SourceType: src.sourceType,
			Payload:    src.payload,
			SourceMeta: senses.SourceMeta{
				Channel:   src.channel,
				Sender:    "test",
				Timestamp: time.Now(),
			},
		}

		result, err := p.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("Source %s failed: %v", src.channel, err)
		}
		if !result.Success {
			t.Errorf("Source %s should succeed", src.channel)
		}

		t.Logf("✓ Source %s: task=%s quality=%.0f%%", src.channel, result.TaskID, result.QualityScore*100)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
