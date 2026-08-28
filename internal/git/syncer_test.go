package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/naueramant/munin/internal/config"
)

func TestGitSyncerLocal(t *testing.T) {
	// Create a source repo
	srcDir := t.TempDir()
	repo, err := gogit.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	// Create subdir with screen.yaml
	screenDir := filepath.Join(srcDir, "my-screen")
	if err := os.MkdirAll(screenDir, 0755); err != nil {
		t.Fatal(err)
	}
	screenFile := filepath.Join(screenDir, "screen.yaml")
	if err := os.WriteFile(screenFile, []byte("syntax: v1\ntabs:\n  - url: https://xkcd.com\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Add("my-screen/screen.yaml"); err != nil {
		t.Fatal(err)
	}
	_, err = w.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Target clone directory
	targetDir := filepath.Join(t.TempDir(), "checkout")

	cfg := config.GitConfig{
		Repo:      srcDir,
		Branch:    "master",
		Subdir:    "my-screen",
		TargetDir: targetDir,
	}

	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Fatalf("failed to create syncer: %v", err)
	}

	// Initial sync
	changed, subdirPath, err := syncer.Sync()
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true on initial clone")
	}
	if subdirPath != filepath.Join(targetDir, "my-screen") {
		t.Errorf("unexpected subdir path: %s", subdirPath)
	}

	// Verify screen.yaml was cloned
	clonedScreen := filepath.Join(subdirPath, "screen.yaml")
	if _, err := os.Stat(clonedScreen); err != nil {
		t.Fatalf("cloned screen.yaml does not exist: %v", err)
	}

	// Second sync without modifications
	changed, _, err = syncer.Sync()
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if changed {
		t.Errorf("expected changed=false when repository has no new commits")
	}
}

func TestGitSyncerFullCheckoutWithSharedFiles(t *testing.T) {
	srcDir := t.TempDir()
	repo, err := gogit.PlainInit(srcDir, false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple directories and files in source repo
	lobbyDir := filepath.Join(srcDir, "screens", "lobby")
	meetingDir := filepath.Join(srcDir, "screens", "meeting")
	sharedDir := filepath.Join(srcDir, "shared")

	for _, dir := range []string{lobbyDir, meetingDir, sharedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(lobbyDir, "screen.yaml"), []byte("syntax: v1\nname: lobby\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meetingDir, "screen.yaml"), []byte("syntax: v1\nname: meeting\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "logo.png"), []byte("image v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Fleet"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"screens/lobby/screen.yaml",
		"screens/meeting/screen.yaml",
		"shared/logo.png",
		"README.md",
	} {
		if _, err := w.Add(rel); err != nil {
			t.Fatal(err)
		}
	}

	_, err = w.Commit("initial commit with multiple screens and shared files", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(t.TempDir(), "checkout")
	cfg := config.GitConfig{
		Repo:      srcDir,
		Branch:    "master",
		Subdir:    "screens/lobby",
		TargetDir: targetDir,
	}

	syncer, err := NewSyncer(cfg)
	if err != nil {
		t.Fatalf("failed to create syncer: %v", err)
	}

	// 1. Initial Sync should check out the whole repo so shared files are available
	changed, subdirPath, err := syncer.Sync()
	if err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true on initial clone")
	}
	expectedSubdir := filepath.Join(targetDir, "screens", "lobby")
	if subdirPath != expectedSubdir {
		t.Errorf("expected subdir %s, got %s", expectedSubdir, subdirPath)
	}

	// Verify screens/lobby/screen.yaml exists
	if _, err := os.Stat(filepath.Join(targetDir, "screens", "lobby", "screen.yaml")); err != nil {
		t.Errorf("screens/lobby/screen.yaml was not checked out: %v", err)
	}

	// Verify shared files and other directories are also checked out
	if _, err := os.Stat(filepath.Join(targetDir, "shared", "logo.png")); err != nil {
		t.Errorf("shared/logo.png should be checked out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); err != nil {
		t.Errorf("README.md should be checked out: %v", err)
	}

	// 2. Modify shared/logo.png and screens/lobby/screen.yaml in source repo
	if err := os.WriteFile(filepath.Join(lobbyDir, "screen.yaml"), []byte("syntax: v1\nname: lobby v2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "logo.png"), []byte("image v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("screens/lobby/screen.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("shared/logo.png"); err != nil {
		t.Fatal(err)
	}
	_, err = w.Commit("update lobby and shared logo", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Second sync: pull changes and assert whole repo is updated
	changed, _, err = syncer.Sync()
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true on update commit")
	}

	updatedBytes, err := os.ReadFile(filepath.Join(targetDir, "screens", "lobby", "screen.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated screen.yaml: %v", err)
	}
	if string(updatedBytes) != "syntax: v1\nname: lobby v2\n" {
		t.Errorf("unexpected content: %s", string(updatedBytes))
	}

	updatedShared, err := os.ReadFile(filepath.Join(targetDir, "shared", "logo.png"))
	if err != nil {
		t.Fatalf("failed to read updated shared/logo.png: %v", err)
	}
	if string(updatedShared) != "image v2" {
		t.Errorf("unexpected content: %s", string(updatedShared))
	}
}

