package senses

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileWatcherSense(t *testing.T) {
	cfg := FileWatcherConfig{
		WatchDir:     "/tmp/test",
		PollInterval: 100 * time.Millisecond,
		Extensions:   []string{".txt"},
		Recursive:    true,
	}
	fw := NewFileWatcherSense(cfg)
	if fw == nil {
		t.Fatal("NewFileWatcherSense returned nil")
	}
	if fw.cfg.WatchDir != cfg.WatchDir {
		t.Errorf("WatchDir = %q, want %q", fw.cfg.WatchDir, cfg.WatchDir)
	}
	if fw.cfg.PollInterval != cfg.PollInterval {
		t.Errorf("PollInterval = %v, want %v", fw.cfg.PollInterval, cfg.PollInterval)
	}
	if len(fw.cfg.Extensions) != 1 || fw.cfg.Extensions[0] != ".txt" {
		t.Errorf("Extensions = %v, want [.txt]", fw.cfg.Extensions)
	}
	if !fw.cfg.Recursive {
		t.Error("Recursive = false, want true")
	}
	if fw.known == nil {
		t.Error("known map not initialized")
	}
}

func TestFileWatcherSense_Name(t *testing.T) {
	fw := NewFileWatcherSense(FileWatcherConfig{WatchDir: "/tmp"})
	if got := fw.Name(); got != "FileWatcher" {
		t.Errorf("Name() = %q, want %q", got, "FileWatcher")
	}
}

func TestFileWatcherSense_Send(t *testing.T) {
	fw := NewFileWatcherSense(FileWatcherConfig{WatchDir: "/tmp"})
	err := fw.Send(context.Background(), "dest", "message")
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}
}

// startFileWatcher is a helper that starts the file watcher in a goroutine
// and returns the output channel and a cleanup function.
func startFileWatcher(t *testing.T, cfg FileWatcherConfig) (chan *UnifiedInput, context.CancelFunc) {
	t.Helper()

	fw := NewFileWatcherSense(cfg)
	out := make(chan *UnifiedInput, 20)
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	go func() {
		// The first scan (seed) happens synchronously inside Start,
		// so once Start proceeds to the ticker loop we know the seed is done.
		// We detect this by a small delay.
		time.Sleep(cfg.PollInterval / 2)
		close(started)
	}()

	go func() {
		_ = fw.Start(ctx, out)
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("file watcher did not start in time")
	}

	t.Cleanup(func() {
		cancel()
		_ = fw.Stop()
	})

	return out, cancel
}

func TestFileWatcherSense_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	out, _ := startFileWatcher(t, cfg)

	// Create a new file after watcher has started.
	fpath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(fpath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input == nil {
			t.Fatal("received nil input")
		}
		if input.SourceMeta.Path != fpath {
			t.Errorf("Path = %q, want %q", input.SourceMeta.Path, fpath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for new file event")
	}
}

func TestFileWatcherSense_DetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()

	// Create the file before starting the watcher so it's seeded.
	fpath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(fpath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	out, _ := startFileWatcher(t, cfg)

	// Modify the file after the seed scan.
	// Ensure the mod time changes (some filesystems have 1s granularity).
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(fpath, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input == nil {
			t.Fatal("received nil input")
		}
		if input.Payload != "modified content" {
			t.Errorf("Payload = %q, want %q", input.Payload, "modified content")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for modified file event")
	}
}

func TestFileWatcherSense_IgnoresExistingOnSeed(t *testing.T) {
	dir := t.TempDir()

	// Create files before starting.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	out, _ := startFileWatcher(t, cfg)

	// Wait for a few poll cycles — no events should be emitted for existing files.
	select {
	case input := <-out:
		t.Fatalf("unexpected event for existing file: %v", input.SourceMeta.Path)
	case <-time.After(300 * time.Millisecond):
		// Good — no events.
	}
}

func TestFileWatcherSense_ExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
		Extensions:   []string{".md"},
	}

	out, _ := startFileWatcher(t, cfg)

	// Create a .txt file — should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a .md file — should be detected.
	mdPath := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(mdPath, []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input.SourceMeta.Path != mdPath {
			t.Errorf("Path = %q, want %q", input.SourceMeta.Path, mdPath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for .md file event")
	}

	// Verify no second event for the .txt file.
	select {
	case input := <-out:
		if input.SourceMeta.Path == filepath.Join(dir, "ignored.txt") {
			t.Fatal("received event for filtered-out .txt file")
		}
		// Another .md event is fine — but a .txt event is not.
	case <-time.After(200 * time.Millisecond):
		// Good — no extra events.
	}
}

func TestFileWatcherSense_StopCancels(t *testing.T) {
	dir := t.TempDir()
	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	fw := NewFileWatcherSense(cfg)
	out := make(chan *UnifiedInput, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = fw.Start(ctx, out)
		close(done)
	}()

	// Let it start and run at least one poll cycle.
	time.Sleep(100 * time.Millisecond)

	// Stop should cause Start to return.
	if err := fw.Stop(); err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}

	select {
	case <-done:
		// Good — Start returned after Stop.
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
}

func TestFileWatcherSense_SourceTypeFile(t *testing.T) {
	dir := t.TempDir()
	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	out, _ := startFileWatcher(t, cfg)

	fpath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fpath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input.SourceType != SourceFile {
			t.Errorf("SourceType = %q, want %q", input.SourceType, SourceFile)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestFileWatcherSense_PayloadContainsContent(t *testing.T) {
	dir := t.TempDir()
	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
	}

	out, _ := startFileWatcher(t, cfg)

	content := "the quick brown fox jumps over the lazy dog"
	fpath := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input.Payload != content {
			t.Errorf("Payload = %q, want %q", input.Payload, content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestFileWatcherSense_RecursiveMode(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
		Recursive:    true,
	}

	out, _ := startFileWatcher(t, cfg)

	fpath := filepath.Join(subdir, "nested.txt")
	if err := os.WriteFile(fpath, []byte("deep file"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case input := <-out:
		if input.SourceMeta.Path != fpath {
			t.Errorf("Path = %q, want %q", input.SourceMeta.Path, fpath)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recursive file event")
	}
}

func TestFileWatcherSense_NonRecursiveMode(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := FileWatcherConfig{
		WatchDir:     dir,
		PollInterval: 50 * time.Millisecond,
		Recursive:    false,
	}

	out, _ := startFileWatcher(t, cfg)

	// Create a file in the subdirectory — should be ignored.
	if err := os.WriteFile(filepath.Join(subdir, "hidden.txt"), []byte("hidden"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file in the root dir — should be detected.
	rootFile := filepath.Join(dir, "visible.txt")
	if err := os.WriteFile(rootFile, []byte("visible"), 0644); err != nil {
		t.Fatal(err)
	}

	// We should receive exactly one event for the root-level file.
	select {
	case input := <-out:
		if input.SourceMeta.Path != rootFile {
			t.Errorf("Path = %q, want %q", input.SourceMeta.Path, rootFile)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for root file event")
	}

	// No event for subdirectory file.
	select {
	case input := <-out:
		if input.SourceMeta.Path == filepath.Join(subdir, "hidden.txt") {
			t.Fatal("received event for file in subdirectory in non-recursive mode")
		}
	case <-time.After(200 * time.Millisecond):
		// Good — no event for the subdirectory file.
	}
}

func TestFileWatcherSense_DefaultPollInterval(t *testing.T) {
	cfg := FileWatcherConfig{
		WatchDir: "/tmp",
		// PollInterval not set — should default to 5s.
	}
	fw := NewFileWatcherSense(cfg)
	if fw.cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want %v", fw.cfg.PollInterval, 5*time.Second)
	}
}
