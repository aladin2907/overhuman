package senses

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestWebhookSense_Name(t *testing.T) {
	w := NewWebhookSense(":0", "/webhook")
	if w.Name() != "Webhook" {
		t.Errorf("Name = %q", w.Name())
	}
}

func TestWebhookSense_ImplementsSense(t *testing.T) {
	var _ Sense = (*WebhookSense)(nil)
}

func startWebhook(t *testing.T, path string) (*WebhookSense, chan *UnifiedInput, context.CancelFunc) {
	t.Helper()

	wh := NewWebhookSense("127.0.0.1:0", path)
	out := make(chan *UnifiedInput, 10)
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	go func() {
		for {
			if wh.Addr() != "127.0.0.1:0" {
				close(started)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	go func() {
		wh.Start(ctx, out)
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("webhook server did not start")
	}

	t.Cleanup(func() {
		cancel()
		wh.Stop()
	})

	return wh, out, cancel
}

func TestWebhookSense_PostWebhook(t *testing.T) {
	wh, out, _ := startWebhook(t, "/webhook")

	payload := `{"event":"push","repo":"overhuman"}`
	req, _ := http.NewRequest("POST", "http://"+wh.Addr()+"/webhook", bytes.NewBufferString(payload))
	req.Header.Set("X-Webhook-Source", "github")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["input_id"] == "" {
		t.Error("input_id should not be empty")
	}

	select {
	case input := <-out:
		if input.Payload != payload {
			t.Errorf("payload = %q", input.Payload)
		}
		if input.SourceType != SourceWebhook {
			t.Errorf("source type = %q", input.SourceType)
		}
		if input.SourceMeta.Sender != "github" {
			t.Errorf("sender = %q", input.SourceMeta.Sender)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for input")
	}
}

func TestWebhookSense_PriorityHeader(t *testing.T) {
	wh, out, _ := startWebhook(t, "/hook")

	req, _ := http.NewRequest("POST", "http://"+wh.Addr()+"/hook", bytes.NewBufferString(`{"test":true}`))
	req.Header.Set("X-Priority", "CRITICAL")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case input := <-out:
		if input.Priority != PriorityCritical {
			t.Errorf("priority = %d, want CRITICAL", input.Priority)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestWebhookSense_DefaultPath(t *testing.T) {
	wh := NewWebhookSense(":0", "")
	if wh.path != "/webhook" {
		t.Errorf("default path = %q, want /webhook", wh.path)
	}
}

func TestWebhookSense_GetEndpoint(t *testing.T) {
	wh, _, _ := startWebhook(t, "/webhook")

	resp, err := http.Get("http://" + wh.Addr() + "/webhook")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestWebhookSense_Send(t *testing.T) {
	wh := NewWebhookSense(":0", "/webhook")
	// Send is a no-op for webhooks.
	err := wh.Send(context.Background(), "target", "msg")
	if err != nil {
		t.Errorf("Send should not error: %v", err)
	}
}

func TestWebhookSense_FallbackSender(t *testing.T) {
	wh, out, _ := startWebhook(t, "/webhook")

	// POST without X-Webhook-Source header.
	resp, err := http.Post("http://"+wh.Addr()+"/webhook", "text/plain", bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case input := <-out:
		if input.SourceMeta.Sender == "" {
			t.Error("sender should fallback to remote address")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}
