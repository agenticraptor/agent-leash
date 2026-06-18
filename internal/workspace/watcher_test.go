package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsChanges(t *testing.T) {
	root := t.TempDir()
	tr := NewTracker(root)
	w, err := NewWatcher(tr, nil)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Start(ctx) }()
	time.Sleep(100 * time.Millisecond) // let the initial watches register

	// Two real files, plus one in an ignored dir and a new subdirectory file.
	mustWrite(t, filepath.Join(root, "a.go"), "package main")
	mustWrite(t, filepath.Join(root, "b.txt"), "hello")
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustWrite(t, filepath.Join(root, "node_modules", "ignored.js"), "x")
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "sub", "c.go"), "package sub")

	if !waitFor(func() bool { return tr.Count() >= 3 }, 3*time.Second) {
		t.Fatalf("expected at least 3 changed files, got %d: %v", tr.Count(), tr.Changed())
	}
	for _, p := range tr.Changed() {
		if filepath.Base(p) == "ignored.js" {
			t.Errorf("file inside node_modules should not be counted: %s", p)
		}
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
