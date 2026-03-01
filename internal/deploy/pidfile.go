package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const pidFileName = "overhuman.pid"

// PIDFile manages the daemon's PID file.
type PIDFile struct {
	path string
}

// NewPIDFile creates a PID file manager for the given data directory.
func NewPIDFile(dataDir string) *PIDFile {
	return &PIDFile{path: filepath.Join(dataDir, pidFileName)}
}

// Path returns the full path to the PID file.
func (p *PIDFile) Path() string {
	return p.path
}

// Write creates/overwrites the PID file with the current process ID.
func (p *PIDFile) Write() error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	data := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(p.path, data, 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// Read returns the PID stored in the PID file, or 0 if not found/invalid.
func (p *PIDFile) Read() (int, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid: %w", err)
	}
	return pid, nil
}

// Remove deletes the PID file.
func (p *PIDFile) Remove() error {
	if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}

// IsRunning checks if a daemon process is currently running.
// Returns the PID if running, 0 if not.
func (p *PIDFile) IsRunning() (int, bool) {
	pid, err := p.Read()
	if err != nil || pid == 0 {
		return 0, false
	}
	if !processExists(pid) {
		// Stale PID file — process died without cleanup.
		p.Remove()
		return 0, false
	}
	return pid, true
}

// Guard ensures no other instance is running. Returns an error if one is.
func (p *PIDFile) Guard() error {
	pid, running := p.IsRunning()
	if running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}
	return p.Write()
}

// processExists checks if a process with the given PID is alive.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Signal 0 checks existence.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// StopDaemon sends SIGTERM to the running daemon process.
func StopDaemon(dataDir string) error {
	pf := NewPIDFile(dataDir)
	pid, running := pf.IsRunning()
	if !running {
		return fmt.Errorf("daemon is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to %d: %w", pid, err)
	}

	return nil
}
