package utils

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/radovskyb/watcher"
)

// Watch monitors a file for changes until the context is cancelled.
// It watches the file's parent directory to handle in-place writes,
// atomic renames (e.g. text editors, git sync), and file creation.
func Watch(ctx context.Context, file string, changedFunc func()) {
	absFile, err := filepath.Abs(file)
	if err != nil {
		absFile = file
	}
	dir := filepath.Dir(absFile)
	targetBase := filepath.Base(absFile)

	w := watcher.New()
	w.FilterOps(watcher.Write, watcher.Remove, watcher.Rename, watcher.Move, watcher.Create)

	// Watch the parent directory so we catch creates, renames, and atomic saves
	if err := w.Add(dir); err != nil {
		slog.Warn("Watcher failed to add directory, falling back to file", "dir", dir, "error", err)
		if err := w.Add(absFile); err != nil {
			slog.Warn("Watcher failed to add file", "file", absFile, "error", err)
		}
	}

	go func() {
		<-ctx.Done()
		w.Close()
	}()

	go func() {
		var mu sync.Mutex
		var debounceTimer *time.Timer

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-w.Event:
				eventTarget, _ := filepath.Abs(e.Path)
				isTarget := eventTarget == absFile ||
					filepath.Base(e.Path) == targetBase ||
					(e.OldPath != "" && filepath.Base(e.OldPath) == targetBase)

				if isTarget {
					mu.Lock()
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
						changedFunc()
					})
					mu.Unlock()
				}
			case err := <-w.Error:
				slog.Debug("Watcher reported error", "error", err)
			case <-w.Closed:
				return
			}
		}
	}()

	go func() {
		if err := w.Start(time.Millisecond * 200); err != nil && err != watcher.ErrWatchedFileDeleted {
			slog.Debug("Watcher stopped", "error", err)
		}
	}()
}
