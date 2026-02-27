package soul

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VersionMeta holds metadata about a soul version snapshot.
type VersionMeta struct {
	Version   int       `json:"version"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Checksum  string    `json:"checksum"`
}

// Soul manages the agent's identity document — a living Markdown file
// with immutable principles ("anchors"), evolving strategies, current state,
// evolution history, and goals. Every mutation is versioned with rollback support.
type Soul struct {
	dir            string
	agentName      string
	specialization string

	mu sync.RWMutex
}

// New creates a Soul manager for the given directory.
// It does NOT initialize the soul file — call Initialize() for that.
func New(dir, agentName, specialization string) *Soul {
	return &Soul{
		dir:            dir,
		agentName:      agentName,
		specialization: specialization,
	}
}

// soulPath returns the path to the main soul file.
func (s *Soul) soulPath() string {
	return filepath.Join(s.dir, "soul.md")
}

// versionsDir returns the path to the versions directory.
func (s *Soul) versionsDir() string {
	return filepath.Join(s.dir, "soul_versions")
}

// Initialize creates the soul directory, versions dir, and writes the default
// soul template. If soul.md already exists, it returns an error.
func (s *Soul) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.soulPath()); err == nil {
		return fmt.Errorf("soul already exists at %s", s.soulPath())
	}

	if err := os.MkdirAll(s.versionsDir(), 0o755); err != nil {
		return fmt.Errorf("create versions dir: %w", err)
	}

	content := s.defaultTemplate()
	if err := os.WriteFile(s.soulPath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write soul: %w", err)
	}

	// Save initial version (v1).
	return s.saveVersionLocked(content, "Initial soul creation")
}

// Read returns the current soul content.
func (s *Soul) Read() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.soulPath())
	if err != nil {
		return "", fmt.Errorf("read soul: %w", err)
	}
	return string(data), nil
}

// Update writes new content to the soul file and creates a new version snapshot.
// It validates that immutable anchors are preserved before allowing the update.
// Returns the new version number.
func (s *Soul) Update(newContent, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read current content to extract anchors.
	currentData, err := os.ReadFile(s.soulPath())
	if err != nil {
		return 0, fmt.Errorf("read current soul: %w", err)
	}

	// Validate anchors are preserved.
	currentAnchors := extractAnchors(string(currentData))
	newAnchors := extractAnchors(newContent)
	if err := validateAnchors(currentAnchors, newAnchors); err != nil {
		return 0, fmt.Errorf("anchor violation: %w", err)
	}

	if err := os.WriteFile(s.soulPath(), []byte(newContent), 0o644); err != nil {
		return 0, fmt.Errorf("write soul: %w", err)
	}

	if err := s.saveVersionLocked(newContent, reason); err != nil {
		return 0, fmt.Errorf("save version: %w", err)
	}

	return s.latestVersionLocked()
}

// Rollback restores the soul to a specific version.
// Returns the restored content.
func (s *Soul) Rollback(version int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := s.readVersionLocked(version)
	if err != nil {
		return "", fmt.Errorf("read version %d: %w", version, err)
	}

	if err := os.WriteFile(s.soulPath(), []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write soul: %w", err)
	}

	// Create a new version noting the rollback.
	reason := fmt.Sprintf("Rollback to version %d", version)
	if err := s.saveVersionLocked(content, reason); err != nil {
		return "", fmt.Errorf("save rollback version: %w", err)
	}

	return content, nil
}

// ListVersions returns all available version numbers in ascending order.
func (s *Soul) ListVersions() ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listVersionsLocked()
}

// ReadVersion returns the content of a specific version.
func (s *Soul) ReadVersion(version int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readVersionLocked(version)
}

// ReadVersionMeta returns metadata for a specific version.
func (s *Soul) ReadVersionMeta(version int) (*VersionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metaPath := filepath.Join(s.versionsDir(), fmt.Sprintf("v%d.meta", version))
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read version meta %d: %w", version, err)
	}

	var meta VersionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse version meta %d: %w", version, err)
	}
	return &meta, nil
}

// LatestVersion returns the latest version number.
func (s *Soul) LatestVersion() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.latestVersionLocked()
}

// --- Internal helpers (must be called with lock held) ---

func (s *Soul) saveVersionLocked(content, reason string) error {
	versions, _ := s.listVersionsLocked()
	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[len(versions)-1] + 1
	}

	// Write content snapshot.
	contentPath := filepath.Join(s.versionsDir(), fmt.Sprintf("v%d.md", nextVersion))
	if err := os.WriteFile(contentPath, []byte(content), 0o644); err != nil {
		return err
	}

	// Write metadata.
	meta := VersionMeta{
		Version:   nextVersion,
		Reason:    reason,
		Timestamp: time.Now().UTC(),
		Checksum:  checksum(content),
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metaPath := filepath.Join(s.versionsDir(), fmt.Sprintf("v%d.meta", nextVersion))
	return os.WriteFile(metaPath, metaData, 0o644)
}

func (s *Soul) readVersionLocked(version int) (string, error) {
	path := filepath.Join(s.versionsDir(), fmt.Sprintf("v%d.md", version))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Soul) listVersionsLocked() ([]int, error) {
	entries, err := os.ReadDir(s.versionsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var versions []int
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "v") && strings.HasSuffix(name, ".md") {
			numStr := strings.TrimSuffix(strings.TrimPrefix(name, "v"), ".md")
			if n, err := strconv.Atoi(numStr); err == nil {
				versions = append(versions, n)
			}
		}
	}
	sort.Ints(versions)
	return versions, nil
}

func (s *Soul) latestVersionLocked() (int, error) {
	versions, err := s.listVersionsLocked()
	if err != nil {
		return 0, err
	}
	if len(versions) == 0 {
		return 0, fmt.Errorf("no versions found")
	}
	return versions[len(versions)-1], nil
}

func (s *Soul) defaultTemplate() string {
	return fmt.Sprintf(`# Soul: %s

## Principles (IMMUTABLE ANCHORS)

<!-- ANCHOR:START -->
- I always act in the best interest of the user
- I never execute harmful or destructive actions without explicit confirmation
- I preserve data integrity and never silently discard information
- I am transparent about my capabilities and limitations
- I learn from every interaction to become more effective
<!-- ANCHOR:END -->

## Strategies (evolving)

- Default task decomposition: break into 3-7 subtasks
- Prefer code-skills over LLM-skills when pattern count >= 3
- Use the cheapest effective LLM model for each subtask
- Always validate outputs before reporting completion

## Current State

- Specialization: %s
- Skills: none (cold start)
- Strengths: general-purpose reasoning
- Weaknesses: no domain-specific experience yet
- Run count: 0

## Evolution History

| Version | Date | Change | Reason |
|---------|------|--------|--------|
| 1 | %s | Initial creation | Cold start |

## Goals

- Learn user's common task patterns
- Build first code-skills from repeated patterns
- Establish baseline quality metrics
`, s.agentName, s.specialization, time.Now().Format("2006-01-02"))
}

// extractAnchors finds content between ANCHOR:START and ANCHOR:END markers.
func extractAnchors(content string) string {
	const startMarker = "<!-- ANCHOR:START -->"
	const endMarker = "<!-- ANCHOR:END -->"

	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return ""
	}

	return strings.TrimSpace(content[startIdx+len(startMarker) : endIdx])
}

// validateAnchors ensures the new anchors match the original anchors exactly.
func validateAnchors(current, new string) error {
	if current == "" {
		// No anchors in current — allow any change (first init).
		return nil
	}
	if new == "" {
		return fmt.Errorf("new content is missing anchor section (<!-- ANCHOR:START --> ... <!-- ANCHOR:END -->)")
	}
	if current != new {
		return fmt.Errorf("immutable anchors have been modified; original:\n%s\n\nnew:\n%s", current, new)
	}
	return nil
}

func checksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}
