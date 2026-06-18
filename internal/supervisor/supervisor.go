// Package supervisor runs an agent command under a policy and enforces it live.
// It installs PATH shims so guarded commands are inspected before they run,
// watches the workspace for file changes, scans manifests for new dependencies,
// meters wall-clock time and reported spend, and hard-stops the agent — killing
// its whole process group — the instant any limit is crossed.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/agenticraptor/agent-leash/internal/budget"
	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/manifest"
	"github.com/agenticraptor/agent-leash/internal/policy"
	"github.com/agenticraptor/agent-leash/internal/sandbox"
	"github.com/agenticraptor/agent-leash/internal/session"
	"github.com/agenticraptor/agent-leash/internal/shim"
	"github.com/agenticraptor/agent-leash/internal/workspace"
)

// Options configures a supervised run.
type Options struct {
	Command    []string      // the agent command and its arguments (required)
	Policy     policy.Config // the policy to enforce
	PolicyPath string        // resolved policy file path, "" when using defaults
	Workspace  string        // absolute workspace root the agent is confined to
	Harden     bool          // request OS-level hardening (Linux network namespace)
	Now        func() time.Time

	// Logf logs informational messages (sandbox notes, reduced-mode warnings).
	Logf func(format string, args ...any)
	// OnStop is called once, when a limit is crossed, before the kill.
	OnStop func(StopCause)
	// OnStatus is called on each metering tick with the current usage.
	OnStatus func(budget.Snapshot)
}

// StopCause describes why a session was stopped.
type StopCause struct {
	Kind   string // "budget" or "command"
	Reason string // human-readable explanation
	Detail string // limit kind, or the offending command line
}

// Result reports the outcome of a supervised run.
type Result struct {
	ExitCode int
	Stopped  bool
	Cause    StopCause
	Usage    budget.Snapshot
	Changed  []string
	NewDeps  []string
}

// pollInterval is how often the event log and metering tick run.
const pollInterval = 200 * time.Millisecond

// StoppedExitCode is the process exit code agent-leash reports when it stops a
// session itself (as opposed to the agent exiting on its own).
const StoppedExitCode = 113

// Run launches the agent under the policy and blocks until it exits or is
// stopped. It returns the result; an error is returned only for setup failures
// (a bad command, an unstartable child), not for policy stops.
func Run(ctx context.Context, opt Options) (Result, error) {
	if len(opt.Command) == 0 {
		return Result{}, errors.New("no command to run")
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	logf := opt.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	sessDir, err := os.MkdirTemp("", "agent-leash-")
	if err != nil {
		return Result{}, fmt.Errorf("create session dir: %w", err)
	}
	defer os.RemoveAll(sessDir)
	shimDir := filepath.Join(sessDir, "bin")

	binPath, err := os.Executable()
	if err != nil {
		return Result{}, fmt.Errorf("locate agent-leash binary: %w", err)
	}

	g := guard.New(opt.Policy, opt.Workspace)
	shimOK := true
	if err := shim.Install(binPath, shimDir, installableGuards(g.GuardSet())); err != nil {
		shimOK = false
		logf("command interception reduced: could not install shims: %v", err)
	}

	baseDeps, _, _ := manifest.Scan(opt.Workspace)
	tracker := workspace.NewTracker(opt.Workspace)
	meter := budget.New(opt.Policy.Limits, opt.Now)

	var (
		stopOnce sync.Once
		cause    StopCause
		child    *exec.Cmd
	)
	triggerStop := func(c StopCause) {
		stopOnce.Do(func() {
			cause = c
			if opt.OnStop != nil {
				opt.OnStop(c)
			}
			killGroup(child, false)
			grace := opt.Policy.OnViolation.KillGrace.Duration
			if grace <= 0 {
				grace = 3 * time.Second
			}
			go func() { time.Sleep(grace); killGroup(child, true) }()
		})
	}
	enforceBudget := func() {
		if v := meter.Check(); v != nil {
			triggerStop(StopCause{Kind: "budget", Reason: v.Reason, Detail: string(v.Kind)})
		}
	}

	watcher, werr := workspace.NewWatcher(tracker, func() {
		meter.SetFilesChanged(tracker.Count())
		enforceBudget()
	})
	if werr != nil {
		logf("file watching unavailable: %v", werr)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Build the child command, applying OS hardening if requested.
	plan := sandbox.Wrap(opt.Command, sandbox.Options{
		Harden:         opt.Harden,
		NetworkAllowed: opt.Policy.Network.Allowed,
	})
	if plan.Note != "" {
		logf("%s", plan.Note)
	}
	child = exec.CommandContext(runCtx, plan.Argv[0], plan.Argv[1:]...) //nolint:gosec // launching the user's own agent command
	child.Env = childEnv(opt, shimDir, sessDir, binPath, shimOK)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	child.Dir = opt.Workspace
	setProcessGroup(child)
	// On context cancellation (e.g. Ctrl-C), tear down the whole process group
	// gracefully rather than killing only the direct child and orphaning its
	// descendants; escalate after a short delay if it ignores the signal.
	child.Cancel = func() error { killGroup(child, false); return nil }
	child.WaitDelay = 5 * time.Second

	if err := child.Start(); err != nil {
		return Result{}, fmt.Errorf("start agent: %w", err)
	}

	// All goroutines that may set `cause` are joined before it is read, so the
	// final outcome is deterministic and race-free.
	var wg sync.WaitGroup
	if watcher != nil {
		wg.Add(1)
		go func() { defer wg.Done(); _ = watcher.Start(runCtx) }()
	}
	wg.Add(2)
	go func() { defer wg.Done(); tailEvents(runCtx, sessDir, meter, opt.Policy, triggerStop, enforceBudget) }()
	go func() { defer wg.Done(); meterLoop(runCtx, opt, meter, tracker, baseDeps, enforceBudget) }()

	waitErr := child.Wait()
	cancel()
	wg.Wait() // lets the event tailer process any final denial before we report

	// Final accounting after the agent has exited.
	curDeps, _, _ := manifest.Scan(opt.Workspace)
	added := manifest.Added(baseDeps, curDeps)
	meter.SetNewDeps(len(added))
	meter.SetFilesChanged(tracker.Count())

	res := Result{
		ExitCode: exitCode(waitErr),
		Stopped:  cause.Kind != "",
		Cause:    cause,
		Usage:    meter.Snapshot(),
		Changed:  tracker.Changed(),
		NewDeps:  added,
	}
	switch {
	case res.Stopped:
		res.ExitCode = StoppedExitCode // we tore the session down
	case res.ExitCode < 0:
		res.ExitCode = 1 // killed by a signal we did not initiate
	}
	return res, nil
}

// meterLoop periodically rescans dependencies, checks time/cost budgets, and
// reports status.
func meterLoop(ctx context.Context, opt Options, meter *budget.Meter, tracker *workspace.Tracker, base manifest.Set, enforce func()) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	var lastDepScan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			// Rescanning manifests is cheaper than walking the whole tree, but
			// still throttled to about once a second.
			if now.Sub(lastDepScan) >= time.Second {
				if cur, _, err := manifest.Scan(opt.Workspace); err == nil {
					meter.SetNewDeps(len(manifest.Added(base, cur)))
				}
				lastDepScan = now
			}
			meter.SetFilesChanged(tracker.Count())
			enforce()
			if opt.OnStatus != nil {
				opt.OnStatus(meter.Snapshot())
			}
		}
	}
}

// tailEvents follows the session command log, counting commands and reacting to
// denials that require stopping the session.
func tailEvents(ctx context.Context, dir string, meter *budget.Meter, pol policy.Config, stop func(StopCause), enforce func()) {
	var off int64
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	drain := func() {
		evs, newOff, _ := session.ReadFrom(dir, off)
		off = newOff
		for _, e := range evs {
			if e.Cost > 0 {
				meter.AddCost(e.Cost)
			}
			if e.Type == session.TypeCommand {
				meter.AddCommand()
				if !e.Allow && e.Action == policy.ActionStop {
					stop(StopCause{Kind: "command", Reason: e.Reason, Detail: e.Command})
				}
			}
			enforce()
		}
	}
	for {
		select {
		case <-ctx.Done():
			drain() // catch any final events
			return
		case <-t.C:
			drain()
		}
	}
}

// childEnv builds the environment for the agent: the shim directory is
// prepended to PATH and the AGENT_LEASH_* variables tell the shims where to
// record events and which policy to enforce.
func childEnv(opt Options, shimDir, sessDir, binPath string, shimOK bool) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+6)
	for _, kv := range env {
		if shimOK && len(kv) >= 5 && kv[:5] == "PATH=" {
			continue // replaced below
		}
		out = append(out, kv)
	}
	if shimOK {
		path := shimDir + string(os.PathListSeparator) + os.Getenv("PATH")
		out = append(out, "PATH="+path)
		out = append(out, shim.EnvShimDir+"="+shimDir)
	}
	out = append(out,
		shim.EnvBin+"="+binPath,
		shim.EnvSession+"="+sessDir,
		shim.EnvWorkspace+"="+opt.Workspace,
		shim.EnvActive+"=1",
	)
	if opt.PolicyPath != "" {
		out = append(out, policy.EnvVar+"="+opt.PolicyPath)
	}
	return out
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return 1
}

// installableGuards keeps only the guarded names that are both safe to write as
// a shim filename and actually present on PATH, so agent-leash never shadows a
// command the user does not have or writes a shim from a hostile policy name.
func installableGuards(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if !shim.SafeName(name) {
			continue
		}
		if _, err := exec.LookPath(name); err != nil {
			continue
		}
		out = append(out, name)
	}
	return out
}
