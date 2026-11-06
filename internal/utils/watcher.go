package utils

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/radovskyb/watcher"
)

// Watch monitors a file for changes until the context is cancelled.
// It watches the file's parent directory to handle in-place writes,
// atomic renames (e.g. text editors, git sync), and file creation.
func Watch(ctx context.Context, file string, changedFunc func()) {
	NewPathWatcher(ctx, changedFunc).Set([]string{file})
}

// PathWatcher monitors a changing set of files and directories until the
// context is cancelled. Files are watched through their parent directory (see
// Watch); for a directory, any change to its direct entries counts.
type PathWatcher struct {
	w *watcher.Watcher

	mu      sync.Mutex
	files   map[string]bool // watched files
	dirs    map[string]bool // watched directories whose every entry counts
	watched map[string]bool // directories added to the underlying watcher
}

// NewPathWatcher starts a watcher with an empty set of paths. Use Set to
// choose what to watch; changedFunc is called (debounced) on every change.
func NewPathWatcher(ctx context.Context, changedFunc func()) *PathWatcher {
	pw := &PathWatcher{
		w:       watcher.New(),
		files:   map[string]bool{},
		dirs:    map[string]bool{},
		watched: map[string]bool{},
	}
	pw.w.FilterOps(watcher.Write, watcher.Remove, watcher.Rename, watcher.Move, watcher.Create)

	go func() {
		<-ctx.Done()
		pw.w.Close()
	}()

	go func() {
		var mu sync.Mutex
		var debounceTimer *time.Timer

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-pw.w.Event:
				if pw.isTarget(e.Path) || (e.OldPath != "" && pw.isTarget(e.OldPath)) {
					mu.Lock()
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
						changedFunc()
					})
					mu.Unlock()
				}
			case err := <-pw.w.Error:
				slog.Debug("Watcher reported error", "error", err)
			case <-pw.w.Closed:
				return
			}
		}
	}()

	go func() {
		if err := pw.w.Start(time.Millisecond * 200); err != nil && err != watcher.ErrWatchedFileDeleted {
			slog.Debug("Watcher stopped", "error", err)
		}
	}()

	return pw
}

// Set replaces the watched paths. Paths that are existing directories are
// watched as directories; everything else (including files that do not exist
// yet) is watched as a file.
func (pw *PathWatcher) Set(paths []string) {
	files := map[string]bool{}
	dirs := map[string]bool{}
	needed := map[string]bool{}

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs[abs] = true
			needed[abs] = true
		} else {
			files[abs] = true
			needed[filepath.Dir(abs)] = true
		}
	}

	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.files = files
	pw.dirs = dirs

	for dir := range pw.watched {
		if !needed[dir] {
			if err := pw.w.Remove(dir); err != nil {
				slog.Debug("Watcher failed to remove directory", "dir", dir, "error", err)
			}
			delete(pw.watched, dir)
		}
	}

	for dir := range needed {
		if pw.watched[dir] {
			continue
		}
		// Watch the parent directory so we catch creates, renames, and atomic saves
		if err := pw.w.Add(dir); err != nil {
			slog.Warn("Watcher failed to add directory", "dir", dir, "error", err)
			continue
		}
		pw.watched[dir] = true
	}
}

func (pw *PathWatcher) isTarget(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	pw.mu.Lock()
	defer pw.mu.Unlock()

	return pw.files[abs] || pw.dirs[abs] || pw.dirs[filepath.Dir(abs)]
}
