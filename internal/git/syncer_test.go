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
