package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default policy should validate: %v", err)
	}
	if cfg.OnViolation.Action != ActionStop {
		t.Errorf("default action = %q, want stop", cfg.OnViolation.Action)
	}
	if cfg.Limits.MaxFilesChanged == 0 {
		t.Error("default should set a file-change limit")
	}
	if !cfg.Network.Allowed {
		t.Error("default network should be allowed so installs work")
	}
}

func TestValidateRejectsUnknownAction(t *testing.T) {
	cfg := Default()
	cfg.OnViolation.Action = "explode"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestValidateNormalizesEmptyAction(t *testing.T) {
	cfg := Default()
	cfg.OnViolation.Action = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OnViolation.Action != ActionStop {
		t.Errorf("empty action should normalize to stop, got %q", cfg.OnViolation.Action)
	}
}

func TestValidateRejectsBadDenyPattern(t *testing.T) {
	cfg := Default()
	cfg.Commands.Deny = []string{"["} // malformed glob
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for malformed deny pattern")
	}
}

func TestValidateRejectsUnsafeGuardName(t *testing.T) {
	cfg := Default()
	cfg.Commands.Guard = []string{"rm", "../../etc/evil"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for a guard name containing a path")
	}
	// A normal guard list still validates.
	cfg.Commands.Guard = []string{"rm", "git", "npm"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("plain guard names should validate: %v", err)
	}
}

func TestDurationRoundTrip(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("1h30m")); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Duration != 90*time.Minute {
		t.Errorf("got %v, want 90m", d.Duration)
	}
	text, err := d.MarshalText()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(text) != "1h30m0s" {
		t.Errorf("marshal = %q", string(text))
	}
}

func TestDurationZero(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("0")); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.Duration != 0 || d.String() != "0" {
		t.Errorf("zero duration mishandled: %v / %q", d.Duration, d.String())
	}
}

func TestLoadOverlaysDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigName)
	body := `
[limits]
max_files_changed = 7
max_duration = "5m"

[network]
allowed = false
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Limits.MaxFilesChanged != 7 {
		t.Errorf("max_files_changed = %d, want 7", cfg.Limits.MaxFilesChanged)
	}
	if cfg.Limits.MaxDuration.Duration != 5*time.Minute {
		t.Errorf("max_duration = %v, want 5m", cfg.Limits.MaxDuration.Duration)
	}
	if cfg.Network.Allowed {
		t.Error("network.allowed should be false from file")
	}
	// Untouched fields keep their defaults.
	if cfg.Limits.MaxNewDeps != Default().Limits.MaxNewDeps {
		t.Error("unspecified field should keep default")
	}
	if len(cfg.Commands.Deny) == 0 {
		t.Error("deny list should retain built-in defaults when not overridden")
	}
}

func TestResolvePrefersExplicit(t *testing.T) {
	if p, ok := Resolve("/tmp/x.toml", "."); !ok || p != "/tmp/x.toml" {
		t.Errorf("explicit not preferred: %q %v", p, ok)
	}
}

func TestResolveFindsProjectLocalUpTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigName), []byte("[limits]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvVar, "") // ensure env does not interfere
	got, ok := Resolve("", deep)
	if !ok {
		t.Fatal("expected to find project-local policy up the tree")
	}
	if got != filepath.Join(root, ConfigName) {
		t.Errorf("resolved %q, want %q", got, filepath.Join(root, ConfigName))
	}
}

func TestInitWritesAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if path != filepath.Join(dir, ConfigName) {
		t.Errorf("path = %q", path)
	}
	// The starter must itself be loadable and valid.
	if _, err := Load(path); err != nil {
		t.Fatalf("starter policy should load: %v", err)
	}
	if _, err := Init(dir); err == nil {
		t.Error("second init should refuse to overwrite")
	}
}

func TestExpandPathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	got := ExpandPath("~/.ssh")
	if got != filepath.Join(home, ".ssh") {
		t.Errorf("ExpandPath(~/.ssh) = %q", got)
	}
}

func TestWorkspaceRootResolvesRelative(t *testing.T) {
	cfg := Default()
	cfg.Filesystem.Workspace = "sub"
	got, err := cfg.WorkspaceRoot("/base")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean("/base/sub") {
		t.Errorf("WorkspaceRoot = %q", got)
	}
}
