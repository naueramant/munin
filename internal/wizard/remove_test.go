package wizard

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveFileIfExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Test non-existing file
	removed, err := RemoveFileIfExists(testFile)
	if err != nil {
		t.Fatalf("unexpected error on non-existing file: %v", err)
	}
	if removed {
		t.Errorf("expected removed=false for non-existing file")
	}

	// Create file
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test existing file
	removed, err = RemoveFileIfExists(testFile)
	if err != nil {
		t.Fatalf("unexpected error removing file: %v", err)
	}
	if !removed {
		t.Errorf("expected removed=true for existing file")
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("file still exists after removal")
	}
}

func TestRemoveDirectoryContent(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, ".munin")
	if err := os.MkdirAll(filepath.Join(subDir, "repo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "agent.yaml"), []byte("mode: local"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveDirectoryContent(subDir); err != nil {
		t.Fatalf("failed to remove directory content: %v", err)
	}

	if _, err := os.Stat(subDir); !os.IsNotExist(err) {
		t.Errorf("expected directory to be deleted")
	}
}

func TestRemovePrompt_Keep(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("n\n"))
	ans := prompt(reader, "Remove item? (y/N)", "N")
	if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
		t.Errorf("expected prompt answer to be negative, got %s", ans)
	}
}

func TestRemovePrompt_Yes(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("y\n"))
	ans := prompt(reader, "Remove item? (y/N)", "N")
	if !strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes") {
		t.Errorf("expected prompt answer to be positive, got %s", ans)
	}
}
