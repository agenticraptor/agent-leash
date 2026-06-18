package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePackageJSON(t *testing.T) {
	data := []byte(`{
		"name": "demo",
		"dependencies": {"react": "^18.0.0", "left-pad": "1.0.0"},
		"devDependencies": {"vitest": "^1.0.0"}
	}`)
	got := setOf(parsePackageJSON(data))
	want := setOf([]string{"react", "left-pad", "vitest"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("package.json deps = %v, want %v", got, want)
	}
}

func TestParseGoMod(t *testing.T) {
	data := []byte(`module example.com/m

go 1.22

require github.com/spf13/cobra v1.8.1

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/fsnotify/fsnotify v1.7.0 // indirect
)
`)
	got := setOf(parseGoMod(data))
	want := setOf([]string{
		"github.com/spf13/cobra",
		"github.com/BurntSushi/toml",
		"github.com/fsnotify/fsnotify",
	})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("go.mod deps = %v, want %v", got, want)
	}
}

func TestParseRequirements(t *testing.T) {
	data := []byte(`# comment
requests==2.31.0
Flask>=2.0
pytest[testing]; python_version < "3.9"
-r other.txt
-e .
`)
	got := setOf(parseRequirements(data))
	want := setOf([]string{"requests", "flask", "pytest"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requirements deps = %v, want %v", got, want)
	}
}

func TestParsePyproject(t *testing.T) {
	data := []byte(`
[project]
dependencies = ["httpx>=0.27", "rich"]

[project.optional-dependencies]
dev = ["pytest", "mypy"]

[tool.poetry.dependencies]
python = "^3.11"
click = "^8.1"
`)
	got := setOf(parsePyproject(data))
	want := setOf([]string{"httpx", "rich", "pytest", "mypy", "click"})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pyproject deps = %v, want %v", got, want)
	}
}

func TestParseCargoAndGemfile(t *testing.T) {
	cargo := []byte(`
[dependencies]
serde = "1.0"
tokio = { version = "1", features = ["full"] }

[dev-dependencies]
criterion = "0.5"
`)
	if got, want := setOf(parseCargo(cargo)), setOf([]string{"serde", "tokio", "criterion"}); !reflect.DeepEqual(got, want) {
		t.Errorf("cargo deps = %v, want %v", got, want)
	}

	gem := []byte(`source "https://rubygems.org"
gem "rails", "~> 7.0"
gem 'pg'
# gem "commented_out"
`)
	if got, want := setOf(parseGemfile(gem)), setOf([]string{"rails", "pg"}); !reflect.DeepEqual(got, want) {
		t.Errorf("gemfile deps = %v, want %v", got, want)
	}
}

func TestScanAddedAndIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("package.json", `{"dependencies":{"react":"18"}}`)
	// A manifest inside node_modules must be ignored.
	write("node_modules/dep/package.json", `{"dependencies":{"should-not-count":"1"}}`)

	base, found, err := Scan(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 manifest found, got %d (%v)", len(found), found)
	}
	if _, bad := base["npm:should-not-count"]; bad {
		t.Error("dependency inside node_modules should be ignored")
	}
	if Count(base) != 1 {
		t.Errorf("base count = %d, want 1", Count(base))
	}

	// The agent adds two dependencies.
	write("package.json", `{"dependencies":{"react":"18","left-pad":"1","lodash":"4"}}`)
	cur, _, err := Scan(root)
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	added := Added(base, cur)
	want := []string{"npm:left-pad", "npm:lodash"}
	if !reflect.DeepEqual(added, want) {
		t.Errorf("Added = %v, want %v", added, want)
	}
}

func setOf(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}
