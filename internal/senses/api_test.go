package senses

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestAPISense_Name(t *testing.T) {
	s := NewAPISense(":0")
	if s.Name() != "API" {
		t.Errorf("Name = %q, want API", s.Name())
	}
}

func TestAPISense_ImplementsSense(t *testing.T) {
	var _ Sense = (*APISense)(nil)
}

// startAPI is a test helper that starts the API sense on a random port.
func startAPI(t *testing.T) (*APISense, chan *UnifiedInput, context.CancelFunc) {
	t.Helper()

	api := NewAPISense("127.0.0.1:0")
	out := make(chan *UnifiedInput, 10)
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	go func() {
		// Wait briefly for listener to be set, then signal.
		for {
			if api.Addr() != "127.0.0.1:0" {
				close(started)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	go func() {
		api.Start(ctx, out)
	}()

	// Wait for server to start.
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("API server did not start in time")
	}

	t.Cleanup(func() {
		cancel()
		api.Stop()
	})

	return api, out, cancel
}

func TestAPISense_Health(t *testing.T) {
	api, _, _ := startAPI(t)

	resp, err := http.Get("http://" + api.Addr() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var body apiHealthResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Status != "ok" {
		t.Errorf("status = %q", body.Status)
	}
	if body.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestAPISense_PostInput(t *testing.T) {
	api, out, _ := startAPI(t)

	payload := `{"payload":"test task","priority":"HIGH","sender":"tester"}`
	resp, err := http.Post(
		"http://"+api.Addr()+"/input",
		"application/json",
		bytes.NewBufferString(payload),
	)
	if err != nil {
		t.Fatalf("POST /input: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	var body apiResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.InputID == "" {
		t.Error("input_id should not be empty")
	}
	if body.Status != "accepted" {
		t.Errorf("status = %q", body.Status)
	}

	// Verify the input arrived in the channel.
	select {
	case input := <-out:
		if input.Payload != "test task" {
			t.Errorf("payload = %q", input.Payload)
		}
		if input.SourceType != SourceAPI {
			t.Errorf("source type = %q", input.SourceType)
		}
		if input.Priority != PriorityHigh {
			t.Errorf("priority = %d, want HIGH", input.Priority)
		}
		if input.SourceMeta.Sender != "tester" {
			t.Errorf("sender = %q", input.SourceMeta.Sender)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for input")
	}
}

func TestAPISense_PostInputEmptyPayload(t *testing.T) {
	api, _, _ := startAPI(t)

	payload := `{"payload":""}`
	resp, err := http.Post(
		"http://"+api.Addr()+"/input",
		"application/json",
		bytes.NewBufferString(payload),
	)
	if err != nil {
		t.Fatalf("POST /input: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPISense_PostInputInvalidJSON(t *testing.T) {
	api, _, _ := startAPI(t)

	resp, err := http.Post(
		"http://"+api.Addr()+"/input",
		"application/json",
		bytes.NewBufferString("not json"),
	)
	if err != nil {
		t.Fatalf("POST /input: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPISense_DefaultPriority(t *testing.T) {
	api, out, _ := startAPI(t)

	payload := `{"payload":"normal task"}`
	resp, err := http.Post(
		"http://"+api.Addr()+"/input",
		"application/json",
		bytes.NewBufferString(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case input := <-out:
		if input.Priority != PriorityNormal {
			t.Errorf("default priority = %d, want NORMAL", input.Priority)
		}
		if input.SourceMeta.Sender != "api_user" {
			t.Errorf("default sender = %q, want api_user", input.SourceMeta.Sender)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestAPISense_SendToCorrelationID(t *testing.T) {
	api := NewAPISense(":0")

	// Simulate a pending sync request.
	ch := make(chan string, 1)
	api.responsesMu.Lock()
	api.responses["corr_123"] = ch
	api.responsesMu.Unlock()

	err := api.Send(context.Background(), "corr_123", "response text")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case msg := <-ch:
		if msg != "response text" {
			t.Errorf("msg = %q", msg)
		}
	default:
		t.Error("expected message in channel")
	}
}

func TestAPISense_SendNoPendingRequest(t *testing.T) {
	api := NewAPISense(":0")

	// Send to non-existent target should not error (async mode).
	err := api.Send(context.Background(), "nonexistent", "msg")
	if err != nil {
		t.Fatalf("Send should not error for async: %v", err)
	}
}

func TestAPISense_Stop(t *testing.T) {
	api := NewAPISense(":0")
	if err := api.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
