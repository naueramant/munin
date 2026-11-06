package utils

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatch_Modify(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "screen.yaml")

	if err := os.WriteFile(filePath, []byte("syntax: v1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changeCount int32
	Watch(ctx, filePath, func() {
		atomic.AddInt32(&changeCount, 1)
	})

	time.Sleep(300 * time.Millisecond)

	// Modify file
	if err := os.WriteFile(filePath, []byte("syntax: v1\ntabs: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(600 * time.Millisecond)

	if atomic.LoadInt32(&changeCount) == 0 {
		t.Errorf("expected at least 1 change event, got 0")
	}
}

func TestWatch_AtomicRename(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "screen.yaml")

	if err := os.WriteFile(filePath, []byte("syntax: v1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changeCount int32
	Watch(ctx, filePath, func() {
		atomic.AddInt32(&changeCount, 1)
	})

	time.Sleep(300 * time.Millisecond)

	// Simulate atomic write: write to temp and rename over target
	tempPath := filepath.Join(tmpDir, "screen.yaml.tmp")
	if err := os.WriteFile(tempPath, []byte("syntax: v1\ntabs:\n  - url: http://example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		t.Fatal(err)
	}

	time.Sleep(600 * time.Millisecond)

	if atomic.LoadInt32(&changeCount) == 0 {
		t.Errorf("expected at least 1 change event on atomic rename, got 0")
	}

	// Now modify it AGAIN to verify the watcher survives atomic rename
	if err := os.WriteFile(tempPath, []byte("syntax: v1\ntabs:\n  - url: http://google.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		t.Fatal(err)
	}

	time.Sleep(600 * time.Millisecond)

	if atomic.LoadInt32(&changeCount) < 2 {
		t.Errorf("expected at least 2 change events on subsequent atomic rename, got %d", atomic.LoadInt32(&changeCount))
	}
}

func TestWatch_FileCreatedLater(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "screen.yaml")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changeCount int32
	Watch(ctx, filePath, func() {
		atomic.AddInt32(&changeCount, 1)
	})

	time.Sleep(300 * time.Millisecond)

	// Now create the file
	if err := os.WriteFile(filePath, []byte("syntax: v1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(600 * time.Millisecond)

	if atomic.LoadInt32(&changeCount) == 0 {
		t.Errorf("expected change event when file is created later, got 0")
	}
}

func TestPathWatcher_IncludedFileAndDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	screen := filepath.Join(tmpDir, "screens", "lobby", "screen.yaml")
	shared := filepath.Join(tmpDir, "shared", "power.yaml")
	schedules := filepath.Join(tmpDir, "shared", "schedules")
	for _, dir := range []string{filepath.Dir(screen), schedules} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{screen, shared} {
		if err := os.WriteFile(f, []byte("syntax: v1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changeCount int32
	pw := NewPathWatcher(ctx, func() {
		atomic.AddInt32(&changeCount, 1)
	})
	pw.Set([]string{screen, shared, schedules})

	time.Sleep(300 * time.Millisecond)

	// Unrelated file next to the shared include must not trigger
	if err := os.WriteFile(filepath.Join(tmpDir, "shared", "other.yaml"), []byte("x: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if got := atomic.LoadInt32(&changeCount); got != 0 {
		t.Fatalf("expected no change events for unrelated file, got %d", got)
	}

	// Modifying the included file triggers
	if err := os.WriteFile(shared, []byte("screen_on: \"07:00\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if got := atomic.LoadInt32(&changeCount); got == 0 {
		t.Fatalf("expected change event for included file, got 0")
	}

	// Adding a file to an included directory triggers
	before := atomic.LoadInt32(&changeCount)
	if err := os.WriteFile(filepath.Join(schedules, "lunch.yaml"), []byte("[]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if got := atomic.LoadInt32(&changeCount); got <= before {
		t.Fatalf("expected change event for new file in included directory")
	}

	// After dropping the include from the set, changes are ignored
	pw.Set([]string{screen})
	before = atomic.LoadInt32(&changeCount)
	if err := os.WriteFile(shared, []byte("screen_on: \"08:00\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if got := atomic.LoadInt32(&changeCount); got != before {
		t.Fatalf("expected no change events after include removed, got %d new", got-before)
	}
}
