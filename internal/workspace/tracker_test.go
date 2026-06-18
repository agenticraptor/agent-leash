package workspace

import (
	"path/filepath"
	"testing"
)

func TestObserveDedupAndCount(t *testing.T) {
	root := t.TempDir()
	tr := NewTracker(root)

	if !tr.Observe(filepath.Join(root, "a.go")) {
		t.Error("first observe should count")
	}
	if tr.Observe(filepath.Join(root, "a.go")) {
		t.Error("second observe of same file should not count again")
	}
	if !tr.Observe("b.go") { // relative path resolves under root
		t.Error("relative path should count")
	}
	if tr.Count() != 2 {
		t.Errorf("count = %d, want 2", tr.Count())
	}
	want := []string{filepath.Join(root, "a.go"), filepath.Join(root, "b.go")}
	if got := tr.Changed(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("changed = %v, want %v", got, want)
	}
}

func TestIgnoredDirsAndFiles(t *testing.T) {
	root := t.TempDir()
	tr := NewTracker(root)
	cases := []struct {
		path    string
		ignored bool
	}{
		{"src/main.go", false},
		{"node_modules/react/index.js", true},
		{".git/HEAD", true},
		{"dist/bundle.js", true},
		{"a/b/__pycache__/x.pyc", true},
		{".DS_Store", true},
		{".agent-leash.toml", true},
		{"src/types.go.123456789", true},
		{"notes.txt.swp", true},
		{"src/types.go", false},
	}
	for _, c := range cases {
		if got := tr.Ignored(filepath.Join(root, c.path)); got != c.ignored {
			t.Errorf("Ignored(%q) = %v, want %v", c.path, got, c.ignored)
		}
	}
}

func TestObserveSkipsIgnored(t *testing.T) {
	root := t.TempDir()
	tr := NewTracker(root)
	if tr.Observe(filepath.Join(root, "node_modules", "x", "y.js")) {
		t.Error("files in node_modules should not count")
	}
	if tr.Count() != 0 {
		t.Errorf("count = %d, want 0", tr.Count())
	}
}

func TestOutsideWorkspaceIgnored(t *testing.T) {
	root := t.TempDir()
	tr := NewTracker(root)
	if !tr.Ignored("/etc/passwd") {
		t.Error("paths outside the workspace should be treated as ignored by the tracker")
	}
}
