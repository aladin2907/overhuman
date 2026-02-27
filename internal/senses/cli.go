package senses

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// CLISense implements the Sense interface for interactive CLI (stdin/stdout).
// It reads lines from an io.Reader (typically os.Stdin) and sends responses
// to an io.Writer (typically os.Stdout).
type CLISense struct {
	reader io.Reader
	writer io.Writer

	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
}

// NewCLISense creates a CLI sense adapter.
// reader and writer are typically os.Stdin and os.Stdout.
func NewCLISense(reader io.Reader, writer io.Writer) *CLISense {
	return &CLISense{
		reader: reader,
		writer: writer,
	}
}

// Name returns the sense name.
func (c *CLISense) Name() string { return "CLI" }

// Start reads lines from the reader and emits UnifiedInput messages.
// It blocks until ctx is cancelled or the reader returns EOF.
func (c *CLISense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	scanner := bufio.NewScanner(c.reader)

	// Read lines in a goroutine so we can respect context cancellation.
	lines := make(chan string)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-lines:
			if !ok {
				return nil // EOF
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Special commands.
			if line == "/quit" || line == "/exit" {
				return nil
			}

			input := &UnifiedInput{
				InputID:    newUUID(),
				SourceType: SourceText,
				SourceMeta: SourceMeta{
					Channel: "cli",
					Sender:  "local_user",
				},
				Payload:  line,
				Priority: PriorityNormal,
			}

			select {
			case out <- input:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// Send writes a response message to the writer.
func (c *CLISense) Send(ctx context.Context, target string, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return fmt.Errorf("cli sense is stopped")
	}

	_, err := fmt.Fprintf(c.writer, "\n%s\n\n", message)
	return err
}

// Stop gracefully stops the CLI sense.
func (c *CLISense) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopped = true
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
