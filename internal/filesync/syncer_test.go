package filesync

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// writeTree creates files (relative path -> content) under dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// listTree returns all files under dir as slash-separated relative paths.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func TestSyncFilesPatterns(t *testing.T) {
	cases := []struct {
		name string
		src  string
		dest string
		want []string
	}{
		{"flat glob", "../../shared/*.html", "out/", []string{"out/beer.html", "out/lunch.html"}},
		{"recursive glob keeps subdirs", "../../shared/assets/**/*.svg", "out", []string{"out/icons/a.svg", "out/logo.svg"}},
		{"directory is copied recursively", "../../shared/assets", "out", []string{"out/icons/a.svg", "out/icons/b.png", "out/logo.svg"}},
		{"hidden files only when named", "../../shared/.*", "out", []string{"out/.env"}},
		{"single file into dest directory", "../../shared/lunch.html", "out/", []string{"out/lunch.html"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, map[string]string{
				"shared/lunch.html":          "lunch",
				"shared/beer.html":           "beer",
				"shared/notes.txt":           "notes",
				"shared/.env":                "secret",
				"shared/.hidden/x.html":      "hidden",
				"shared/assets/logo.svg":     "logo",
				"shared/assets/icons/a.svg":  "a",
				"shared/assets/icons/b.png":  "b",
				"shared/assets/.cache/c.svg": "c",
				"shared/assets/icons/.d.svg": "d",
				"screens/lobby/screen.yaml":  "syntax: v1",
			})
			dest := t.TempDir()

			mappings := []config.FileMapping{{Src: tc.src, Dest: filepath.Join(dest, tc.dest) + suffixSlash(tc.dest)}}
			if err := SyncFiles(filepath.Join(root, "screens", "lobby"), mappings); err != nil {
				t.Fatalf("SyncFiles failed: %v", err)
			}

			if got := listTree(t, dest); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("unexpected files:\n got  %v\n want %v", got, tc.want)
			}
		})
	}
}

func suffixSlash(dest string) string {
	if strings.HasSuffix(dest, "/") {
		return "/"
	}
	return ""
}

func TestSyncFilesNoMatchDoesNotStopOthers(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"lunch.html": "lunch"})
	dest := t.TempDir()

	mappings := []config.FileMapping{
		{Src: "*.missing", Dest: dest},
		{Src: "lunch.html", Dest: filepath.Join(dest, "lunch.html")},
	}

	err := SyncFiles(root, mappings)
	if err == nil || !strings.Contains(err.Error(), `pattern "*.missing" matched no files`) {
		t.Errorf("expected no-match error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "lunch.html")); statErr != nil {
		t.Errorf("expected later mapping to still sync: %v", statErr)
	}
}

func TestSyncFilesGlobAppliesMode(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"scripts/a.sh": "a", "scripts/b.sh": "b"})
	dest := t.TempDir()

	if err := SyncFiles(root, []config.FileMapping{{Src: "scripts/*.sh", Dest: dest, Mode: "0755"}}); err != nil {
		t.Fatalf("SyncFiles failed: %v", err)
	}
	for _, name := range []string{"a.sh", "b.sh"} {
		info, err := os.Stat(filepath.Join(dest, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("expected %s to have mode 0755, got %o", name, info.Mode().Perm())
		}
	}
}
