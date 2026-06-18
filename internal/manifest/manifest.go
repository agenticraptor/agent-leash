// Package manifest counts the dependencies declared across a project's package
// manifests and diffs two scans to report how many dependencies were added.
// It understands the common ecosystems an agent is likely to touch: npm
// (package.json), Go (go.mod), Python (requirements.txt, pyproject.toml), Rust
// (Cargo.toml), Ruby (Gemfile), and PHP (composer.json).
//
// "New dependencies" is measured by identity: a scan taken before the session
// is compared with the current scan, and only dependencies that did not exist
// before are counted — so churn that swaps one dependency for another is not
// mistaken for unbounded growth.
package manifest

import (
	"os"
	"path/filepath"
	"sort"
)

// Set is a set of ecosystem-qualified dependency keys, e.g. "npm:react".
type Set map[string]struct{}

// ignoredDirs are skipped while walking, both for speed and to avoid counting a
// project's own installed dependencies as declarations.
var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, ".next": true, ".cache": true,
	".venv": true, "venv": true, "__pycache__": true, ".tox": true,
	".idea": true, ".vscode": true,
}

// parser maps a manifest filename to a function that extracts dependency names
// and the ecosystem prefix to qualify them with.
type parser struct {
	eco string
	fn  func([]byte) []string
}

var parsers = map[string]parser{
	"package.json":     {"npm", parsePackageJSON},
	"composer.json":    {"composer", parseComposerJSON},
	"go.mod":           {"go", parseGoMod},
	"requirements.txt": {"pypi", parseRequirements},
	"pyproject.toml":   {"pypi", parsePyproject},
	"Cargo.toml":       {"cargo", parseCargo},
	"Gemfile":          {"gem", parseGemfile},
}

// Scan walks root and returns the set of declared dependencies along with the
// manifest files it found. Parse errors on individual files are tolerated:
// a malformed manifest contributes nothing rather than failing the scan.
func Scan(root string) (Set, []string, error) {
	set := make(Set)
	var found []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if path != root && ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		p, ok := parsers[d.Name()]
		if !ok {
			return nil
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // path comes from walking the workspace
		if readErr != nil {
			return nil
		}
		found = append(found, path)
		for _, name := range p.fn(data) {
			if name != "" {
				set[p.eco+":"+name] = struct{}{}
			}
		}
		return nil
	})
	return set, found, err
}

// Added returns the dependency keys present in cur but not in base, sorted.
func Added(base, cur Set) []string {
	var out []string
	for k := range cur {
		if _, ok := base[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Count returns the number of dependencies in the set.
func Count(s Set) int { return len(s) }
