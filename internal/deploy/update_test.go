package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// githubRelease is a minimal GitHub release response for test mocking.
type githubRelease struct {
	TagName     string         `json:"tag_name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []githubAsset  `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func assetName() string {
	return fmt.Sprintf("overhuman-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func newReleaseServer(t *testing.T, release githubRelease, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Fatalf("encode release: %v", err)
			}
		}
	}))
}

func TestCheckUpdate_NewVersion(t *testing.T) {
	srv := newReleaseServer(t, githubRelease{
		TagName:     "v2.0.0",
		Body:        "new features",
		PublishedAt: "2026-01-01T00:00:00Z",
		Assets: []githubAsset{
			{Name: assetName(), BrowserDownloadURL: "https://example.com/binary"},
		},
	}, http.StatusOK)
	defer srv.Close()

	info, err := CheckUpdate(UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil UpdateInfo for newer version")
	}
	if info.Version != "2.0.0" {
		t.Fatalf("version = %q, want %q", info.Version, "2.0.0")
	}
}

func TestCheckUpdate_SameVersion(t *testing.T) {
	srv := newReleaseServer(t, githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: assetName(), BrowserDownloadURL: "https://example.com/binary"},
		},
	}, http.StatusOK)
	defer srv.Close()

	info, err := CheckUpdate(UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil for same version, got %+v", info)
	}
}

func TestCheckUpdate_OlderVersion(t *testing.T) {
	srv := newReleaseServer(t, githubRelease{
		TagName: "v0.9.0",
		Assets: []githubAsset{
			{Name: assetName(), BrowserDownloadURL: "https://example.com/binary"},
		},
	}, http.StatusOK)
	defer srv.Close()

	info, err := CheckUpdate(UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateURL:      srv.URL,
	})
	if err != nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil for older version, got %+v", info)
	}
}

func TestCheckUpdate_NoAsset(t *testing.T) {
	srv := newReleaseServer(t, githubRelease{
		TagName: "v2.0.0",
		Assets:  []githubAsset{}, // no matching asset
	}, http.StatusOK)
	defer srv.Close()

	_, err := CheckUpdate(UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateURL:      srv.URL,
	})
	if err == nil {
		t.Fatal("expected error when no matching asset")
	}
}

func TestCheckUpdate_ServerError(t *testing.T) {
	srv := newReleaseServer(t, githubRelease{}, http.StatusInternalServerError)
	defer srv.Close()

	_, err := CheckUpdate(UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateURL:      srv.URL,
	})
	if err == nil {
		t.Fatal("expected error on server 500")
	}
}

func TestApplyUpdate_Success(t *testing.T) {
	dir := t.TempDir()

	// Create a fake "current binary".
	binaryPath := filepath.Join(dir, "overhuman")
	if err := os.WriteFile(binaryPath, []byte("old-binary-content"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	// Serve a fake "new binary".
	newContent := []byte("new-binary-content")
	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(newContent)
	}))
	defer downloadSrv.Close()

	cfg := UpdateConfig{
		CurrentVersion: "1.0.0",
		DataDir:        dir,
		BinaryPath:     binaryPath,
	}
	info := &UpdateInfo{
		Version:     "2.0.0",
		DownloadURL: downloadSrv.URL,
	}

	result, err := ApplyUpdate(cfg, info)
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated = true")
	}
	if result.BackupPath == "" {
		t.Fatal("expected non-empty BackupPath")
	}

	// Verify the new binary is in place.
	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(got) != "new-binary-content" {
		t.Fatalf("binary content = %q, want %q", string(got), "new-binary-content")
	}
}

func TestApplyUpdate_ChecksumMismatch(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "overhuman")
	if err := os.WriteFile(binaryPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new-binary-content"))
	}))
	defer downloadSrv.Close()

	cfg := UpdateConfig{
		CurrentVersion: "1.0.0",
		DataDir:        dir,
		BinaryPath:     binaryPath,
	}
	info := &UpdateInfo{
		Version:     "2.0.0",
		DownloadURL: downloadSrv.URL,
		SHA256:      "0000000000000000000000000000000000000000000000000000000000000000",
	}

	_, err := ApplyUpdate(cfg, info)
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
}

func TestApplyUpdate_ChecksumValid(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "overhuman")
	if err := os.WriteFile(binaryPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	newContent := []byte("new-binary-verified")
	h := sha256.Sum256(newContent)
	expectedHash := hex.EncodeToString(h[:])

	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(newContent)
	}))
	defer downloadSrv.Close()

	cfg := UpdateConfig{
		CurrentVersion: "1.0.0",
		DataDir:        dir,
		BinaryPath:     binaryPath,
	}
	info := &UpdateInfo{
		Version:     "2.0.0",
		DownloadURL: downloadSrv.URL,
		SHA256:      expectedHash,
	}

	result, err := ApplyUpdate(cfg, info)
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated = true")
	}
}

func TestRollback_Success(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "overhuman")
	if err := os.WriteFile(binaryPath, []byte("broken-binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	backupPath := filepath.Join(dir, "overhuman-backup")
	if err := os.WriteFile(backupPath, []byte("good-binary"), 0o755); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	cfg := UpdateConfig{BinaryPath: binaryPath}
	if err := Rollback(cfg, backupPath); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != "good-binary" {
		t.Fatalf("binary = %q, want %q", string(got), "good-binary")
	}
}

func TestRollback_NoBackup(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "overhuman")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	cfg := UpdateConfig{BinaryPath: binaryPath}
	err := Rollback(cfg, filepath.Join(dir, "nonexistent-backup"))
	if err == nil {
		t.Fatal("expected error when backup doesn't exist")
	}
}

func TestListBackups_Empty(t *testing.T) {
	dir := t.TempDir()
	// No backups directory at all.
	backups, err := ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if backups != nil {
		t.Fatalf("expected nil for empty dir, got %v", backups)
	}
}

func TestListBackups_WithBackups(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	names := []string{"overhuman-1.0.0", "overhuman-1.1.0"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte("binary"), 0o644); err != nil {
			t.Fatalf("write backup %s: %v", name, err)
		}
	}

	backups, err := ListBackups(dir)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("len(backups) = %d, want 2", len(backups))
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}

	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])
	if got != want {
		t.Fatalf("hash = %q, want %q", got, want)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	content := []byte("copy me please")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("dst content = %q, want %q", string(got), string(content))
	}
}

func TestSplitFields(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"abc def ghi", 3},
		{"  one  two  ", 2},
		{"single", 1},
		{"", 0},
		{"tab\there", 2},
		{"newline\nhere", 2},
	}

	for _, tt := range tests {
		fields := splitFields(tt.input)
		if len(fields) != tt.want {
			t.Errorf("splitFields(%q) = %d fields %v, want %d", tt.input, len(fields), fields, tt.want)
		}
	}
}
