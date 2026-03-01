package senses

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// FileWatcherConfig — configuration for the file-watching sense.
// ---------------------------------------------------------------------------

// FileWatcherConfig holds configuration for the FileWatcherSense.
type FileWatcherConfig struct {
	// WatchDir is the root directory to watch for file changes.
	WatchDir string

	// PollInterval is how often the directory is scanned. Default: 5s.
	PollInterval time.Duration

	// Extensions filters which file extensions to watch (e.g. [".txt", ".md"]).
	// An empty slice means all files are watched.
	Extensions []string

	// Recursive controls whether subdirectories are scanned.
	Recursive bool
}

// ---------------------------------------------------------------------------
// FileWatcherSense — polls a directory for new/modified files.
// ---------------------------------------------------------------------------

// FileWatcherSense implements the Sense interface by polling a directory for
// new or modified files and emitting UnifiedInput messages with file content.
// It uses Go stdlib only (no fsnotify) — a simple polling approach.
type FileWatcherSense struct {
	cfg FileWatcherConfig

	mu      sync.Mutex
	out     chan<- *UnifiedInput
	cancel  context.CancelFunc
	stopped bool

	// known tracks filepath → last modification time.
	known map[string]time.Time
}

// NewFileWatcherSense creates a new FileWatcherSense with the given config.
// If PollInterval is zero, it defaults to 5 seconds.
func NewFileWatcherSense(cfg FileWatcherConfig) *FileWatcherSense {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	return &FileWatcherSense{
		cfg:   cfg,
		known: make(map[string]time.Time),
	}
}

// Name returns the sense name.
func (fw *FileWatcherSense) Name() string { return "FileWatcher" }

// Start begins polling the configured directory for file changes.
// On the first scan it records all existing files without emitting events
// (to avoid flooding the pipeline). Subsequent scans emit events for new
// or modified files. Start blocks until ctx is cancelled.
func (fw *FileWatcherSense) Start(ctx context.Context, out chan<- *UnifiedInput) error {
	fw.mu.Lock()
	fw.out = out
	fw.stopped = false
	ctx, fw.cancel = context.WithCancel(ctx)
	fw.mu.Unlock()

	// Initial scan — seed the known map, don't emit events.
	if err := fw.scan(ctx, true); err != nil {
		return fmt.Errorf("filewatcher: initial scan: %w", err)
	}

	ticker := time.NewTicker(fw.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = fw.scan(ctx, false)
		}
	}
}

// Send is a no-op — the file watcher is an input-only sense.
func (fw *FileWatcherSense) Send(_ context.Context, _ string, _ string) error {
	return nil
}

// Stop gracefully stops the polling loop.
func (fw *FileWatcherSense) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.stopped = true
	if fw.cancel != nil {
		fw.cancel()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scan walks the watch directory, detects new/modified files, and optionally
// emits UnifiedInput events. When seed is true the known map is populated
// but no events are sent.
func (fw *FileWatcherSense) scan(ctx context.Context, seed bool) error {
	current := make(map[string]time.Time)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't stat
		}
		if info.IsDir() {
			if !fw.cfg.Recursive && path != fw.cfg.WatchDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !fw.matchesExtension(path) {
			return nil
		}
		current[path] = info.ModTime()
		return nil
	}

	if err := filepath.Walk(fw.cfg.WatchDir, walkFn); err != nil {
		return fmt.Errorf("walk %s: %w", fw.cfg.WatchDir, err)
	}

	if !seed {
		for path, modTime := range current {
			prev, exists := fw.known[path]
			if !exists || modTime.After(prev) {
				fw.emit(ctx, path, modTime)
			}
		}
	}

	fw.mu.Lock()
	fw.known = current
	fw.mu.Unlock()

	return nil
}

// matchesExtension returns true if the file at path has one of the
// configured extensions, or if no extension filter is set.
func (fw *FileWatcherSense) matchesExtension(path string) bool {
	if len(fw.cfg.Extensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range fw.cfg.Extensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	return false
}

// emit reads the file content and sends a UnifiedInput to the output channel.
func (fw *FileWatcherSense) emit(ctx context.Context, path string, modTime time.Time) {
	content, err := os.ReadFile(path)
	if err != nil {
		return // file may have been deleted between scan and read
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	input := NewUnifiedInput(SourceFile, string(content))
	input.SourceMeta.Channel = "filewatcher"
	input.SourceMeta.Path = path
	input.SourceMeta.Timestamp = modTime
	input.SourceMeta.Extra = map[string]string{
		"filename": filepath.Base(path),
		"size":     fmt.Sprintf("%d", info.Size()),
	}

	select {
	case <-ctx.Done():
	case fw.out <- input:
	}
}
