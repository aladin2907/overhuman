package soul

import (
	"os"
	"strings"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "soul-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestInitialize(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")

	if err := s.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Soul file should exist.
	content, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "# Soul: TestAgent") {
		t.Error("soul should contain agent name")
	}
	if !strings.Contains(content, "ANCHOR:START") {
		t.Error("soul should contain anchor markers")
	}
	if !strings.Contains(content, "general") {
		t.Error("soul should contain specialization")
	}

	// Version 1 should exist.
	versions, err := s.ListVersions()
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("expected [1], got %v", versions)
	}
}

func TestInitializeAlreadyExists(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")

	if err := s.Initialize(); err != nil {
		t.Fatal(err)
	}

	// Second init should fail.
	if err := s.Initialize(); err == nil {
		t.Error("expected error on second Initialize")
	}
}

func TestUpdate(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	original, _ := s.Read()

	// Modify strategies section but keep anchors intact.
	newContent := strings.Replace(original,
		"- Default task decomposition: break into 3-7 subtasks",
		"- Default task decomposition: break into 2-5 subtasks",
		1)

	version, err := s.Update(newContent, "Adjusted decomposition strategy")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}

	// Read should return updated content.
	content, _ := s.Read()
	if !strings.Contains(content, "2-5 subtasks") {
		t.Error("content should reflect the update")
	}

	// Should have 2 versions now.
	versions, _ := s.ListVersions()
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestUpdateRejectsAnchorModification(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	original, _ := s.Read()

	// Try to modify an anchor.
	tampered := strings.Replace(original,
		"- I always act in the best interest of the user",
		"- I always act in my own interest",
		1)

	_, err := s.Update(tampered, "Evil mutation")
	if err == nil {
		t.Fatal("expected error when modifying anchors")
	}
	if !strings.Contains(err.Error(), "anchor violation") {
		t.Errorf("expected anchor violation error, got: %v", err)
	}

	// Content should remain unchanged.
	content, _ := s.Read()
	if strings.Contains(content, "my own interest") {
		t.Error("anchors should not have been modified")
	}
}

func TestUpdateRejectsRemovedAnchors(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	// Try to remove anchor section entirely.
	noAnchors := "# Soul: TestAgent\n\n## Strategies\n\n- Just vibes\n"

	_, err := s.Update(noAnchors, "Remove anchors")
	if err == nil {
		t.Fatal("expected error when removing anchors")
	}
}

func TestRollback(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	v1Content, _ := s.Read()

	// Make an update.
	newContent := strings.Replace(v1Content,
		"Run count: 0",
		"Run count: 10",
		1)
	s.Update(newContent, "Updated run count")

	// Verify update took effect.
	current, _ := s.Read()
	if !strings.Contains(current, "Run count: 10") {
		t.Fatal("update should have taken effect")
	}

	// Rollback to version 1.
	restored, err := s.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if !strings.Contains(restored, "Run count: 0") {
		t.Error("rollback should restore original content")
	}

	// Current soul should be v1 content.
	current, _ = s.Read()
	if !strings.Contains(current, "Run count: 0") {
		t.Error("soul should reflect rollback")
	}

	// Should now have 3 versions: original, update, rollback.
	versions, _ := s.ListVersions()
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}
}

func TestRollbackInvalidVersion(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	_, err := s.Rollback(999)
	if err == nil {
		t.Error("expected error for invalid version")
	}
}

func TestReadVersion(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	v1, _ := s.ReadVersion(1)
	current, _ := s.Read()

	if v1 != current {
		t.Error("v1 content should match current soul")
	}
}

func TestReadVersionMeta(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	meta, err := s.ReadVersionMeta(1)
	if err != nil {
		t.Fatalf("ReadVersionMeta: %v", err)
	}
	if meta.Version != 1 {
		t.Errorf("expected version 1, got %d", meta.Version)
	}
	if meta.Reason != "Initial soul creation" {
		t.Errorf("unexpected reason: %s", meta.Reason)
	}
	if meta.Checksum == "" {
		t.Error("checksum should not be empty")
	}
	if meta.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestExtractAnchors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "normal anchors",
			content: "before\n<!-- ANCHOR:START -->\n- rule 1\n- rule 2\n<!-- ANCHOR:END -->\nafter",
			want:    "- rule 1\n- rule 2",
		},
		{
			name:    "no anchors",
			content: "just some text",
			want:    "",
		},
		{
			name:    "missing end",
			content: "<!-- ANCHOR:START -->\n- rule",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAnchors(tt.content)
			if got != tt.want {
				t.Errorf("extractAnchors() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLatestVersion(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	v, _ := s.LatestVersion()
	if v != 1 {
		t.Errorf("expected 1, got %d", v)
	}

	content, _ := s.Read()
	s.Update(strings.Replace(content, "Run count: 0", "Run count: 1", 1), "bump")

	v, _ = s.LatestVersion()
	if v != 2 {
		t.Errorf("expected 2, got %d", v)
	}
}

func TestConcurrentReads(t *testing.T) {
	dir := tempDir(t)
	s := New(dir, "TestAgent", "general")
	s.Initialize()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := s.Read()
			done <- (err == nil)
		}()
	}

	for i := 0; i < 10; i++ {
		if ok := <-done; !ok {
			t.Error("concurrent read failed")
		}
	}
}
