package deploy

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewPIDFile(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	want := filepath.Join(dir, pidFileName)
	if pf.Path() != want {
		t.Fatalf("path = %q, want %q", pf.Path(), want)
	}
}

func TestPIDFile_Write_Read(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	if err := pf.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestPIDFile_Read_NotExist(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("Read: unexpected error: %v", err)
	}
	if pid != 0 {
		t.Fatalf("pid = %d, want 0 for non-existent file", pid)
	}
}

func TestPIDFile_Read_Invalid(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	// Write non-numeric content.
	if err := os.WriteFile(pf.Path(), []byte("not-a-number"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := pf.Read()
	if err == nil {
		t.Fatal("Read: expected error for invalid PID content, got nil")
	}
}

func TestPIDFile_Remove(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	if err := pf.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := pf.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(pf.Path()); !os.IsNotExist(err) {
		t.Fatalf("PID file still exists after Remove")
	}
}

func TestPIDFile_Remove_NotExist(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	// Should not return an error if file doesn't exist.
	if err := pf.Remove(); err != nil {
		t.Fatalf("Remove non-existent: unexpected error: %v", err)
	}
}

func TestPIDFile_IsRunning_CurrentProcess(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	if err := pf.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	pid, running := pf.IsRunning()
	if !running {
		t.Fatal("IsRunning: expected true for current process")
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestPIDFile_IsRunning_NoFile(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	pid, running := pf.IsRunning()
	if running {
		t.Fatal("IsRunning: expected false when no PID file")
	}
	if pid != 0 {
		t.Fatalf("pid = %d, want 0", pid)
	}
}

func TestPIDFile_IsRunning_StalePID(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	// Write a PID that almost certainly doesn't exist.
	stalePID := 99999999
	if err := os.WriteFile(pf.Path(), []byte(strconv.Itoa(stalePID)), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pid, running := pf.IsRunning()
	if running {
		t.Fatal("IsRunning: expected false for stale PID")
	}
	if pid != 0 {
		t.Fatalf("pid = %d, want 0", pid)
	}

	// Stale PID file should be cleaned up.
	if _, err := os.Stat(pf.Path()); !os.IsNotExist(err) {
		t.Fatal("stale PID file was not cleaned up")
	}
}

func TestPIDFile_Guard_FirstInstance(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	if err := pf.Guard(); err != nil {
		t.Fatalf("Guard: %v", err)
	}

	// Verify PID was written.
	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("Read after Guard: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestPIDFile_Guard_AlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	pf := NewPIDFile(dir)

	// Write current process PID (simulates a running instance).
	if err := pf.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	err := pf.Guard()
	if err == nil {
		t.Fatal("Guard: expected error when process is already running")
	}
}

func TestProcessExists_CurrentPID(t *testing.T) {
	if !processExists(os.Getpid()) {
		t.Fatal("processExists: expected true for current PID")
	}
}

func TestProcessExists_InvalidPID(t *testing.T) {
	if processExists(99999999) {
		t.Fatal("processExists: expected false for PID 99999999")
	}
}
