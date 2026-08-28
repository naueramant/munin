package filesync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/naueramant/munin/internal/config"
)

func TestSyncFiles(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "test.sh")
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	targetFile := filepath.Join(destDir, "nested", "test.sh")

	mappings := []config.FileMapping{
		{
			Src:  "test.sh",
			Dest: targetFile,
			Mode: "0755",
		},
	}

	// 1. Initial sync
	if err := SyncFiles(srcDir, mappings); err != nil {
		t.Fatalf("SyncFiles failed: %v", err)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(content) != "#!/bin/sh\necho hello" {
		t.Errorf("unexpected content: %s", string(content))
	}

	info, err := os.Stat(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755 mode, got %o", info.Mode().Perm())
	}

	// 2. Re-sync without changes (hash match test)
	if err := SyncFiles(srcDir, mappings); err != nil {
		t.Fatalf("second SyncFiles failed: %v", err)
	}

	// 3. Update source file
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh\necho updated"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := SyncFiles(srcDir, mappings); err != nil {
		t.Fatalf("third SyncFiles failed: %v", err)
	}

	contentUpdated, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if string(contentUpdated) != "#!/bin/sh\necho updated" {
		t.Errorf("unexpected updated content: %s", string(contentUpdated))
	}
}
