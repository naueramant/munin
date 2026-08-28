package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/naueramant/munin/internal/config"
	"golang.org/x/crypto/ssh"
)

// Syncer handles cloning and synchronizing a remote Git repository.
type Syncer struct {
	cfg        config.GitConfig
	auth       transport.AuthMethod
	lastCommit plumbing.Hash
}

// NewSyncer initializes a GitSyncer with credentials and configuration.
func NewSyncer(cfg config.GitConfig) (*Syncer, error) {
	s := &Syncer{
		cfg: cfg,
	}

	if cfg.DeployKey != "" {
		keyPath := cfg.DeployKey
		if _, err := os.Stat(keyPath); err != nil {
			return nil, fmt.Errorf("deploy key not found at %s: %w", keyPath, err)
		}

		publicKeys, err := gitssh.NewPublicKeysFromFile("git", keyPath, "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse deploy key from %s: %w", keyPath, err)
		}

		// Disable strict host checking for headless/automated Raspberry Pi setup
		publicKeys.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		s.auth = publicKeys
	}

	return s, nil
}

// Sync ensures the repository is cloned and pulls the latest changes.
// Returns whether the repository state changed, the path to the configured subdir, and any error.
func (s *Syncer) Sync() (bool, string, error) {
	subdirPath := filepath.Join(s.cfg.TargetDir, s.cfg.Subdir)

	gitDir := filepath.Join(s.cfg.TargetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Repo does not exist, clone it
		slog.Info("Cloning repository", "repo", s.cfg.Repo, "branch", s.cfg.Branch, "target", s.cfg.TargetDir)
		if err := os.MkdirAll(s.cfg.TargetDir, 0755); err != nil {
			return false, "", fmt.Errorf("failed to create target directory: %w", err)
		}

		repo, err := git.PlainClone(s.cfg.TargetDir, false, &git.CloneOptions{
			URL:           s.cfg.Repo,
			Auth:          s.auth,
			ReferenceName: plumbing.NewBranchReferenceName(s.cfg.Branch),
			SingleBranch:  true,
			Depth:         1,
			Progress:      nil,
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to clone repository: %w", err)
		}

		head, err := repo.Head()
		if err == nil {
			s.lastCommit = head.Hash()
			slog.Info("Cloned commit", "commit", head.Hash().String()[:8])
		}

		if err := s.verifySubdir(subdirPath); err != nil {
			return true, subdirPath, err
		}

		return true, subdirPath, nil
	}

	// Repo already exists, open it
	repo, err := git.PlainOpen(s.cfg.TargetDir)
	if err != nil {
		return false, "", fmt.Errorf("failed to open local repository at %s: %w", s.cfg.TargetDir, err)
	}

	// Fetch remote
	remoteBranchRef := plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", s.cfg.Branch))
	err = repo.Fetch(&git.FetchOptions{
		Auth:     s.auth,
		Force:    true,
		Progress: nil,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		slog.Warn("Git fetch failed, falling back to local cached repository", "error", err)
		if s.verifySubdir(subdirPath) == nil {
			return false, subdirPath, nil
		}
		return false, "", fmt.Errorf("git fetch failed and local cache is invalid: %w", err)
	}

	// Look up remote commit
	remoteRef, err := repo.Reference(remoteBranchRef, true)
	if err != nil {
		head, err := repo.Head()
		if err != nil {
			return false, "", fmt.Errorf("failed to get repository HEAD: %w", err)
		}
		remoteRef = head
	}

	w, err := repo.Worktree()
	if err != nil {
		return false, "", fmt.Errorf("failed to get worktree: %w", err)
	}

	changed := remoteRef.Hash() != s.lastCommit
	if changed {
		slog.Info("New Git commit pulled", "commit", remoteRef.Hash().String()[:8], "previous", safeShortHash(s.lastCommit))
		err = w.Reset(&git.ResetOptions{
			Commit: remoteRef.Hash(),
			Mode:   git.HardReset,
		})
		if err != nil {
			return false, "", fmt.Errorf("failed to reset worktree to %s: %w", remoteRef.Hash(), err)
		}
		s.lastCommit = remoteRef.Hash()
	} else {
		slog.Debug("Git repository is up to date", "commit", safeShortHash(s.lastCommit))
	}

	if err := s.verifySubdir(subdirPath); err != nil {
		return changed, subdirPath, err
	}

	return changed, subdirPath, nil
}

func (s *Syncer) verifySubdir(subdirPath string) error {
	info, err := os.Stat(subdirPath)
	if err != nil {
		return fmt.Errorf("configured subdir %s does not exist in repo", s.cfg.Subdir)
	}
	if !info.IsDir() {
		return fmt.Errorf("configured subdir %s is not a directory", s.cfg.Subdir)
	}

	cand1 := filepath.Join(subdirPath, "screen.yaml")
	cand2 := filepath.Join(subdirPath, "screen.yml")
	if _, err := os.Stat(cand1); err != nil {
		if _, err := os.Stat(cand2); err != nil {
			return fmt.Errorf("no screen.yaml found in %s", subdirPath)
		}
	}

	return nil
}

func safeShortHash(h plumbing.Hash) string {
	s := h.String()
	if len(s) >= 8 && !strings.HasPrefix(s, "0000000") {
		return s[:8]
	}
	return "none"
}
