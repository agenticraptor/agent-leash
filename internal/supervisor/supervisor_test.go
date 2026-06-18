package supervisor

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/agenticraptor/agent-leash/internal/policy"
)

func testPolicy(t *testing.T, mutate func(*policy.Config)) policy.Config {
	t.Helper()
	cfg := policy.Default()
	if mutate != nil {
		mutate(&cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("invalid policy: %v", err)
	}
	return cfg
}

func TestRunStopsOnFileBudget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell loop")
	}
	ws := t.TempDir()
	cfg := testPolicy(t, func(c *policy.Config) {
		c.Limits = policy.Limits{
			MaxFilesChanged: 3,
			MaxDuration:     policy.Duration{Duration: 15 * time.Second},
		}
	})

	// Create files one at a time, then idle. The watcher should count past the
	// limit and the supervisor should kill the whole group before the idle.
	script := `i=0; while [ $i -lt 30 ]; do echo x > "f$i.txt"; i=$((i+1)); sleep 0.1; done; sleep 30`

	var stopped StopCause
	opt := Options{
		Command:   []string{"sh", "-c", script},
		Policy:    cfg,
		Workspace: ws,
		OnStop:    func(c StopCause) { stopped = c },
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	res, err := Run(ctx, opt)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if time.Since(start) > 18*time.Second {
		t.Error("supervisor did not stop the agent promptly")
	}
	if !res.Stopped || res.Cause.Kind != "budget" {
		t.Fatalf("expected a budget stop, got stopped=%v cause=%+v", res.Stopped, res.Cause)
	}
	if res.ExitCode == 0 {
		t.Error("a stopped session should have a non-zero exit code")
	}
	if stopped.Kind != "budget" {
		t.Error("OnStop callback should have fired with a budget cause")
	}
	if res.Usage.FilesChanged < 3 {
		t.Errorf("expected at least 3 files counted, got %d", res.Usage.FilesChanged)
	}
}

func TestRunStopsOnCostBudget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	ws := t.TempDir()
	cfg := testPolicy(t, func(c *policy.Config) {
		c.Limits = policy.Limits{
			MaxCostUSD:  1.00,
			MaxDuration: policy.Duration{Duration: 15 * time.Second},
		}
	})
	// The agent reports spend by appending a cost event to the session log
	// (exactly what `agent-leash report --cost` does), then idles.
	script := `printf '%s\n' '{"type":"cost","cost":1.5}' >> "$AGENT_LEASH_SESSION/events.jsonl"; sleep 30`

	opt := Options{Command: []string{"sh", "-c", script}, Policy: cfg, Workspace: ws}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res, err := Run(ctx, opt)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !res.Stopped || res.Cause.Kind != "budget" || res.Cause.Detail != "cost" {
		t.Fatalf("expected a cost budget stop, got stopped=%v cause=%+v", res.Stopped, res.Cause)
	}
	if res.Usage.CostUSD < 1.5 {
		t.Errorf("expected reported cost to be metered, got %.2f", res.Usage.CostUSD)
	}
}

func TestRunCleanCompletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	ws := t.TempDir()
	cfg := testPolicy(t, nil)
	opt := Options{
		Command:   []string{"sh", "-c", "echo hello > note.txt"},
		Policy:    cfg,
		Workspace: ws,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := Run(ctx, opt)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Stopped {
		t.Errorf("clean command should not be stopped: %+v", res.Cause)
	}
	if res.ExitCode != 0 {
		t.Errorf("clean command exit code = %d, want 0", res.ExitCode)
	}
}

func TestRunNoCommand(t *testing.T) {
	_, err := Run(context.Background(), Options{Policy: policy.Default(), Workspace: t.TempDir()})
	if err == nil {
		t.Error("expected an error when no command is given")
	}
}
