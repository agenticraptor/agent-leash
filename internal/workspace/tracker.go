// Package workspace tracks which files an agent changes during a session and
// confines that accounting to the project tree. The Tracker is pure and
// concurrency-safe; the Watcher feeds it from the operating system via fsnotify.
package workspace

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ignoredDirs are directories whose contents never count as agent changes
// (build output, dependency caches, version-control metadata).
var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, ".next": true, ".cache": true,
	".venv": true, "venv": true, "__pycache__": true, ".tox": true,
	".idea": true, ".vscode": true, ".agent-leash": true,
}

// ignoredFiles are individual files that should not count as changes.
var ignoredFiles = map[string]bool{
	".DS_Store": true, ".agent-leash.toml": true, "coverage.out": true,
}

// Tracker records the distinct set of files changed under a workspace root,
// applying ignore rules so that build artifacts and caches do not inflate the
// count. It is safe for concurrent use.
type Tracker struct {
	root string

	mu      sync.Mutex
	changed map[string]struct{}
}

// NewTracker returns a Tracker rooted at the given absolute directory.
func NewTracker(root string) *Tracker {
	return &Tracker{root: filepath.Clean(root), changed: make(map[string]struct{})}
}

// Observe records that path changed and reports whether it was newly counted
// (false if it was ignored or already seen). path may be absolute or relative
// to the workspace root.
func (t *Tracker) Observe(path string) bool {
	clean := t.abs(path)
	if t.Ignored(clean) {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.changed[clean]; ok {
		return false
	}
	t.changed[clean] = struct{}{}
	return true
}

// Count returns the number of distinct files changed.
func (t *Tracker) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.changed)
}

// Changed returns the sorted list of changed file paths.
func (t *Tracker) Changed() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, 0, len(t.changed))
	for p := range t.changed {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Ignored reports whether a path should be excluded from the change count.
func (t *Tracker) Ignored(path string) bool {
	clean := t.abs(path)
	rel, err := filepath.Rel(t.root, clean)
	if err != nil || strings.HasPrefix(rel, "..") {
		// Outside the workspace: not our concern, so treat as ignored here.
		return true
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if ignoredDirs[part] {
			return true
		}
	}
	base := filepath.Base(clean)
	if ignoredFiles[base] {
		return true
	}
	return isTempFile(base)
}

func (t *Tracker) abs(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.root, path)
	}
	return filepath.Clean(path)
}

// isTempFile matches editor/formatter scratch files that should not count.
func isTempFile(base string) bool {
	switch {
	case strings.HasSuffix(base, ".swp"), strings.HasSuffix(base, ".tmp"), strings.HasSuffix(base, "~"):
		return true
	}
	// gofmt/goimports temp files look like "types.go.123456789".
	if i := strings.Index(base, ".go."); i >= 0 {
		suffix := base[i+len(".go."):]
		return suffix != "" && allDigits(suffix)
	}
	return false
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
