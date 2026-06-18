package shim

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSafeName(t *testing.T) {
	safe := []string{"rm", "git", "pip3", "go", "node-gyp", "a.out", "g++"}
	for _, n := range safe {
		if !SafeName(n) {
			t.Errorf("SafeName(%q) = false, want true", n)
		}
	}
	unsafe := []string{"", ".", "..", "../evil", "a/b", `a\b`, "rm -rf", "a;b", "$(x)", "a\x00b", "../../etc/cron.d/x"}
	for _, n := range unsafe {
		if SafeName(n) {
			t.Errorf("SafeName(%q) = true, want false", n)
		}
	}
}

func TestInstallSkipsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	// A hostile policy name must not cause a write outside dir.
	if err := Install("/opt/agent-leash", dir, []string{"rm", "../../pwned"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the safe shim to be written, got %d", len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "pwned")); err == nil {
		t.Fatal("traversal name escaped the shim directory")
	}
}

func TestInstallCreatesShims(t *testing.T) {
	dir := t.TempDir()
	names := []string{"rm", "git", "curl"}
	if err := Install("/opt/agent-leash", dir, names); err != nil {
		t.Fatalf("install: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Fatalf("installed %d shims, want %d", len(entries), len(names))
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
			t.Errorf("shim %q is not executable", e.Name())
		}
	}
}

func TestLookPathExcludingSkipsShimDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATHEXT executable semantics differ on Windows")
	}
	shimDir := t.TempDir()
	realDir := t.TempDir()
	writeExe(t, filepath.Join(shimDir, "demo"))
	realExe := filepath.Join(realDir, "demo")
	writeExe(t, realExe)

	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+realDir)
	got, err := LookPathExcluding("demo", shimDir)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != realExe {
		t.Errorf("resolved %q, want the real binary %q", got, realExe)
	}
	// PATH must be restored after the call.
	if os.Getenv("PATH") != shimDir+string(os.PathListSeparator)+realDir {
		t.Error("PATH was not restored after LookPathExcluding")
	}
}

func TestLookPathExcludingAbsolutePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec bit semantics differ on Windows")
	}
	exe := filepath.Join(t.TempDir(), "tool")
	writeExe(t, exe)
	got, err := LookPathExcluding(exe, "")
	if err != nil || got != exe {
		t.Errorf("absolute path lookup = %q, %v; want %q", got, err, exe)
	}
}

func writeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
