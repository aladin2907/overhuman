package senses

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestCLISense_Name(t *testing.T) {
	s := NewCLISense(nil, nil)
	if s.Name() != "CLI" {
		t.Errorf("Name = %q, want CLI", s.Name())
	}
}

func TestCLISense_ReadLines(t *testing.T) {
	input := "hello\nworld\n"
	reader := strings.NewReader(input)
	writer := &bytes.Buffer{}

	cli := NewCLISense(reader, writer)
	out := make(chan *UnifiedInput, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := cli.Start(ctx, out)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Should have received 2 inputs.
	if len(out) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(out))
	}

	msg1 := <-out
	if msg1.Payload != "hello" {
		t.Errorf("first payload = %q", msg1.Payload)
	}
	if msg1.SourceType != SourceText {
		t.Errorf("source type = %q", msg1.SourceType)
	}
	if msg1.SourceMeta.Channel != "cli" {
		t.Errorf("channel = %q", msg1.SourceMeta.Channel)
	}

	msg2 := <-out
	if msg2.Payload != "world" {
		t.Errorf("second payload = %q", msg2.Payload)
	}
}

func TestCLISense_SkipsEmptyLines(t *testing.T) {
	input := "hello\n\n\n  \nworld\n"
	reader := strings.NewReader(input)
	writer := &bytes.Buffer{}

	cli := NewCLISense(reader, writer)
	out := make(chan *UnifiedInput, 10)

	cli.Start(context.Background(), out)

	if len(out) != 2 {
		t.Fatalf("expected 2 inputs (skip empty lines), got %d", len(out))
	}
}

func TestCLISense_QuitCommand(t *testing.T) {
	input := "hello\n/quit\nshould not appear\n"
	reader := strings.NewReader(input)
	writer := &bytes.Buffer{}

	cli := NewCLISense(reader, writer)
	out := make(chan *UnifiedInput, 10)

	err := cli.Start(context.Background(), out)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Only "hello" should be received, /quit stops before "should not appear".
	if len(out) != 1 {
		t.Fatalf("expected 1 input (quit stops reading), got %d", len(out))
	}
}

func TestCLISense_ExitCommand(t *testing.T) {
	input := "/exit\n"
	reader := strings.NewReader(input)
	writer := &bytes.Buffer{}

	cli := NewCLISense(reader, writer)
	out := make(chan *UnifiedInput, 10)

	err := cli.Start(context.Background(), out)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(out) != 0 {
		t.Fatalf("expected 0 inputs after /exit, got %d", len(out))
	}
}

func TestCLISense_Send(t *testing.T) {
	writer := &bytes.Buffer{}
	cli := NewCLISense(nil, writer)

	err := cli.Send(context.Background(), "any_target", "Hello, user!")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !strings.Contains(writer.String(), "Hello, user!") {
		t.Errorf("writer should contain the message, got %q", writer.String())
	}
}

func TestCLISense_SendAfterStop(t *testing.T) {
	writer := &bytes.Buffer{}
	cli := NewCLISense(nil, writer)

	cli.Stop()

	err := cli.Send(context.Background(), "target", "msg")
	if err == nil {
		t.Error("expected error when sending after stop")
	}
}

func TestCLISense_Stop(t *testing.T) {
	cli := NewCLISense(nil, nil)
	if err := cli.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestCLISense_ImplementsSense(t *testing.T) {
	var _ Sense = (*CLISense)(nil)
}
