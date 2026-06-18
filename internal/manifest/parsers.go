package manifest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// parsePackageJSON extracts dependency names from an npm package.json.
func parsePackageJSON(data []byte) []string {
	var pkg struct {
		Dependencies         map[string]json.RawMessage `json:"dependencies"`
		DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
		OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
		PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	var out []string
	for _, m := range []map[string]json.RawMessage{
		pkg.Dependencies, pkg.DevDependencies, pkg.OptionalDependencies, pkg.PeerDependencies,
	} {
		for name := range m {
			out = append(out, name)
		}
	}
	return out
}

// parseComposerJSON extracts dependency names from a PHP composer.json.
func parseComposerJSON(data []byte) []string {
	var c struct {
		Require    map[string]json.RawMessage `json:"require"`
		RequireDev map[string]json.RawMessage `json:"require-dev"`
	}
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	var out []string
	for _, m := range []map[string]json.RawMessage{c.Require, c.RequireDev} {
		for name := range m {
			out = append(out, name)
		}
	}
	return out
}

// parseGoMod extracts required module paths from a go.mod file.
func parseGoMod(data []byte) []string {
	var out []string
	inBlock := false
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case strings.HasPrefix(line, "require "):
			if f := strings.Fields(strings.TrimPrefix(line, "require ")); len(f) > 0 {
				out = append(out, f[0])
			}
		case inBlock:
			if f := strings.Fields(line); len(f) > 0 {
				out = append(out, f[0])
			}
		}
	}
	return out
}

// parseRequirements extracts package names from a pip requirements.txt.
func parseRequirements(data []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" || strings.HasPrefix(line, "-") {
			continue // blank, comment, or an option/include like -r, -e
		}
		if name := pep508Name(line); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parsePyproject extracts dependency names from a PEP 621 or Poetry pyproject.
func parsePyproject(data []byte) []string {
	var doc struct {
		Project struct {
			Dependencies         []string            `toml:"dependencies"`
			OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		} `toml:"project"`
		Tool struct {
			Poetry struct {
				Dependencies    map[string]toml.Primitive `toml:"dependencies"`
				DevDependencies map[string]toml.Primitive `toml:"dev-dependencies"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil
	}
	var out []string
	for _, dep := range doc.Project.Dependencies {
		if name := pep508Name(dep); name != "" {
			out = append(out, name)
		}
	}
	for _, group := range doc.Project.OptionalDependencies {
		for _, dep := range group {
			if name := pep508Name(dep); name != "" {
				out = append(out, name)
			}
		}
	}
	for name := range doc.Tool.Poetry.Dependencies {
		if !strings.EqualFold(name, "python") {
			out = append(out, name)
		}
	}
	for name := range doc.Tool.Poetry.DevDependencies {
		out = append(out, name)
	}
	return out
}

// parseCargo extracts dependency names from a Rust Cargo.toml.
func parseCargo(data []byte) []string {
	var doc struct {
		Dependencies      map[string]toml.Primitive `toml:"dependencies"`
		DevDependencies   map[string]toml.Primitive `toml:"dev-dependencies"`
		BuildDependencies map[string]toml.Primitive `toml:"build-dependencies"`
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil
	}
	var out []string
	for _, m := range []map[string]toml.Primitive{
		doc.Dependencies, doc.DevDependencies, doc.BuildDependencies,
	} {
		for name := range m {
			out = append(out, name)
		}
	}
	return out
}

var gemRe = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"]`)

// parseGemfile extracts gem names from a Ruby Gemfile.
func parseGemfile(data []byte) []string {
	var out []string
	for _, m := range gemRe.FindAllSubmatch(data, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// pep508Name returns the bare package name from a PEP 508 requirement string,
// e.g. "requests[security]>=2.0; python_version<'3.9'" -> "requests".
func pep508Name(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Cut at the first delimiter that ends the name.
	if i := strings.IndexAny(s, "[<>=!~; ("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(strings.ToLower(s))
}
