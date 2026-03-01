package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// DefaultUpdateURL is the GitHub releases API endpoint.
	DefaultUpdateURL = "https://api.github.com/repos/aladin2907/overhuman/releases/latest"

	updateTimeout = 60 * time.Second
)

// UpdateConfig configures the auto-updater.
type UpdateConfig struct {
	// CurrentVersion is the running binary version (e.g., "0.1.0").
	CurrentVersion string

	// UpdateURL is the API endpoint to check for updates.
	UpdateURL string

	// DataDir is the data directory for storing backups.
	DataDir string

	// BinaryPath is the path to the current running binary.
	BinaryPath string
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	SHA256       string `json:"sha256"`
	ReleaseDate  string `json:"release_date"`
	ReleaseNotes string `json:"release_notes"`
}

// UpdateResult describes what happened during an update attempt.
type UpdateResult struct {
	Updated      bool   // true if binary was replaced
	OldVersion   string
	NewVersion   string
	BackupPath   string // path to backup of old binary
	NeedsRestart bool
	Message      string
}

// CheckUpdate checks if a newer version is available.
func CheckUpdate(cfg UpdateConfig) (*UpdateInfo, error) {
	if cfg.UpdateURL == "" {
		cfg.UpdateURL = DefaultUpdateURL
	}

	client := &http.Client{Timeout: updateTimeout}
	req, err := http.NewRequest("GET", cfg.UpdateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "overhuman/"+cfg.CurrentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("update server returned %d", resp.StatusCode)
	}

	var release struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	// Strip "v" prefix from tag.
	newVersion := release.TagName
	if len(newVersion) > 0 && newVersion[0] == 'v' {
		newVersion = newVersion[1:]
	}

	// Compare versions (simple string comparison).
	if newVersion <= cfg.CurrentVersion {
		return nil, nil // no update available
	}

	// Find matching asset for this platform.
	assetName := fmt.Sprintf("overhuman-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	var checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
		}
		if a.Name == assetName+".sha256" {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	// Fetch checksum if available.
	var checksum string
	if checksumURL != "" {
		checksum, _ = fetchChecksum(client, checksumURL)
	}

	return &UpdateInfo{
		Version:      newVersion,
		DownloadURL:  downloadURL,
		SHA256:       checksum,
		ReleaseDate:  release.PublishedAt,
		ReleaseNotes: release.Body,
	}, nil
}

// ApplyUpdate downloads, verifies, and atomically swaps the binary.
func ApplyUpdate(cfg UpdateConfig, info *UpdateInfo) (*UpdateResult, error) {
	if info == nil {
		return nil, fmt.Errorf("no update info")
	}

	result := &UpdateResult{
		OldVersion: cfg.CurrentVersion,
		NewVersion: info.Version,
	}

	// 1. Download new binary to temp file.
	tmpPath := cfg.BinaryPath + ".update"
	if err := downloadFile(tmpPath, info.DownloadURL); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpPath) // cleanup on failure

	// 2. Verify SHA256 if provided.
	if info.SHA256 != "" {
		actualHash, err := fileSHA256(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("hash new binary: %w", err)
		}
		if actualHash != info.SHA256 {
			return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", info.SHA256, actualHash)
		}
	}

	// 3. Make new binary executable.
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return nil, fmt.Errorf("chmod: %w", err)
	}

	// 4. Backup current binary.
	backupDir := filepath.Join(cfg.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}
	backupPath := filepath.Join(backupDir, fmt.Sprintf("overhuman-%s", cfg.CurrentVersion))
	if err := copyFile(cfg.BinaryPath, backupPath); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}
	result.BackupPath = backupPath

	// 5. Atomic swap (rename).
	if err := os.Rename(tmpPath, cfg.BinaryPath); err != nil {
		return nil, fmt.Errorf("swap binary: %w", err)
	}

	result.Updated = true
	result.NeedsRestart = true
	result.Message = fmt.Sprintf("Updated %s → %s (backup: %s)", cfg.CurrentVersion, info.Version, backupPath)
	return result, nil
}

// Rollback restores the previous binary version from backup.
func Rollback(cfg UpdateConfig, backupPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", backupPath)
	}

	if err := copyFile(backupPath, cfg.BinaryPath); err != nil {
		return fmt.Errorf("restore from backup: %w", err)
	}

	if err := os.Chmod(cfg.BinaryPath, 0o755); err != nil {
		return fmt.Errorf("chmod restored binary: %w", err)
	}

	return nil
}

// ListBackups returns available backup versions.
func ListBackups(dataDir string) ([]string, error) {
	backupDir := filepath.Join(dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []string
	for _, e := range entries {
		if !e.IsDir() {
			backups = append(backups, e.Name())
		}
	}
	return backups, nil
}

// --- helpers ---

func downloadFile(dst, url string) error {
	client := &http.Client{Timeout: updateTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fetchChecksum(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	// Checksum file format: "hash  filename" or just "hash".
	parts := fmt.Sprintf("%s", data)
	fields := splitFields(parts)
	if len(fields) > 0 {
		return fields[0], nil
	}
	return "", fmt.Errorf("empty checksum")
}

// splitFields splits a string by whitespace.
func splitFields(s string) []string {
	var fields []string
	current := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if current != "" {
				fields = append(fields, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}
