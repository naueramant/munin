package utils

import (
	"context"
	"log/slog"
	"time"

	"github.com/radovskyb/watcher"
)

// Watch monitors a file for changes until the context is cancelled.
func Watch(ctx context.Context, file string, changedFunc func()) {
	w := watcher.New()
	w.FilterOps(watcher.Write, watcher.Remove, watcher.Rename, watcher.Move, watcher.Create)

	if err := w.Add(file); err != nil {
		slog.Warn("Watcher failed to add file", "file", file, "error", err)
	}

	go func() {
		<-ctx.Done()
		w.Close()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.Event:
				changedFunc()
			case err := <-w.Error:
				slog.Debug("Watcher reported error", "error", err)
			case <-w.Closed:
				return
			}
		}
	}()

	go func() {
		if err := w.Start(time.Millisecond * 300); err != nil && err != watcher.ErrWatchedFileDeleted {
			slog.Debug("Watcher stopped", "error", err)
		}
	}()
}
