<div align="center">

# 🐕 agent-leash

### A hard budget + kill-switch for your AI agent's *actions* — not just its tokens.

Token trackers tell you what an agent did **after** it happened. `agent-leash`
stops it **during**: set a limit on how many files it can change, how many
dependencies it can add, how long it runs, how much it spends, whether it
touches the network, and which commands it can *never* run — and it hard-stops
the agent the instant a line is crossed, with a readable reason.

Works with **any** agent — Claude Code, Cursor, Aider, OpenAI Codex, your own
script — as a supervisor (`run`) or a per-tool-call hook (`hook`).

[![CI](https://github.com/agenticraptor/agent-leash/actions/workflows/ci.yml/badge.svg)](https://github.com/agenticraptor/agent-leash/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/agenticraptor/agent-leash?sort=semver)](https://github.com/agenticraptor/agent-leash/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/agenticraptor/agent-leash.svg)](https://pkg.go.dev/github.com/agenticraptor/agent-leash)
[![Go Report Card](https://goreportcard.com/badge/github.com/agenticraptor/agent-leash)](https://goreportcard.com/report/github.com/agenticraptor/agent-leash)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

> **Try it in one line — no install, no signup, no config:**
>
> ```bash
> go run github.com/agenticraptor/agent-leash/cmd/agent-leash@latest run -- echo "hello from a leashed shell"
> ```

<!--
  📸 Replace this block with a 15–20s screen capture: run an agent under
  `agent-leash run`, have it try `git push --force` (or blow the file budget),
  and watch the red STOP banner appear and the session end. The hero GIF is the
  single biggest driver of stars — record it once, drop it at docs/demo.gif,
  then uncomment:

  <p align="center"><img src="docs/demo.gif" alt="agent-leash demo" width="760"></p>
-->

```text
🐕 agent-leash — leashing this session
   workspace /home/you/code/checkout-service
   policy    .agent-leash.toml
   limits    files≤25 · deps≤3 · cmds≤∞ · time≤20:00 · spend≤$5.00
   guards    network off · on violation: stop

[agent] refactoring the payment module…

╭──────────────────────────────────────────────────╮
│  ⛔  agent-leash stopped the session              │
│  matches a denied pattern (git push --force*)     │
│  $ git push --force origin main                   │
╰──────────────────────────────────────────────────╯

╭──────────────────────────────────────────────────────────────╮
│  agent-leash · session report                                │
│  status          STOPPED — git push --force was blocked      │
│  files changed   8 / 25                                      │
│  new deps        1 / 3                                       │
│  commands        14                                         │
│  time            4:12 / 20:00                               │
│  exit code       113                                        │
╰──────────────────────────────────────────────────────────────╯
```

## The runaway-agent problem

Agentic coding is incredible right up until the moment it isn't. You step away
for coffee and come back to an agent that "fixed" the failing test by
`git reset --hard`-ing your morning's work, pulled in a 40 MB dependency for a
three-line job, rewrote forty files you didn't ask it to touch, or burned
through your budget in a loop.

Cost trackers (CodeBurn, ccost, …) are great, but they're a **receipt** — they
tell you what happened once it's already done. What's missing is a **leash**:
something that watches what the agent *does*, in real time, and yanks it back
the moment it crosses a line you set.

That's `agent-leash`. You declare the boundaries of a session; it enforces them
live and hard-stops the agent — killing its whole process group — the instant a
limit is hit.

## Why you'll like it

- **It stops things _during_, not after.** A budget meter + a kill-switch, not a
  report. The dangerous command never runs; the runaway session is torn down in
  a fraction of a second, with a reason you can read.
- **It guards _actions_, not tokens.** Max files changed, max new dependencies,
  max wall-clock time, max spend, no network, and a deny-list of commands an
  agent should never run (`rm -rf /`, `git push --force`, `curl … | sh`, reading
  `~/.ssh`, …). 28 dangerous patterns are guarded out of the box.
- **It works with _any_ agent.** `agent-leash run -- <anything>` wraps Claude
  Code, Cursor, Aider, Codex, or your own script. No SDK, no framework lock-in.
- **It plugs into Claude Code natively.** `agent-leash hook` enforces the same
  policy at the PreToolUse layer, blocking a tool call *before* it executes — and
  it speaks a generic JSON contract any platform can use.
- **One file to tune.** Drop an `.agent-leash.toml` in your repo to tighten the
  defaults; commit it so your whole team shares the same guardrails.
- **One static binary. No telemetry, no account, no cloud.** It runs locally and
  makes no network connections of its own.

## Install

### `go install`

```bash
go install github.com/agenticraptor/agent-leash/cmd/agent-leash@latest
```

### Pre-built binaries

Grab a binary for your OS/arch from the
[**Releases**](https://github.com/agenticraptor/agent-leash/releases) page.

### Homebrew (macOS / Linux)

```bash
brew install agenticraptor/tap/agent-leash
```

> Available once the Homebrew tap is published — see the note in
> [`.goreleaser.yaml`](.goreleaser.yaml) to enable it.

### From source

```bash
git clone https://github.com/agenticraptor/agent-leash
cd agent-leash
make install
```

## Quickstart

```bash
# 1. Leash any agent. A protective default policy applies with zero config.
agent-leash run -- claude

# 2. Tighten the leash for one run, no config file needed.
agent-leash run --no-network --max-files 10 --max-duration 10m -- aider

# 3. Write a policy you can commit and share.
agent-leash init        # creates .agent-leash.toml
agent-leash run -- ./my-agent --task "upgrade deps"

# 4. Test what the policy would do — without running anything.
agent-leash check -- git push --force      # ⛔ deny  (exit 1)
agent-leash check -- go test ./...         # ✓ allow (exit 0)

# 5. See what agent-leash can enforce on your machine.
agent-leash doctor
```

## Two ways to leash an agent

`agent-leash` enforces the **same policy** through two complementary mechanisms.
Use either or both.

### `run` — supervise any agent, live

```bash
agent-leash run -- <your agent command>
```

`run` launches your agent as a child process and watches the session as it
happens:

- a **filesystem watcher** counts the files the agent changes,
- a **manifest scanner** counts new dependencies across `package.json`,
  `go.mod`, `requirements.txt`, `pyproject.toml`, `Cargo.toml`, `Gemfile`, and
  `composer.json`,
- a **meter** tracks wall-clock time and reported spend,
- **PATH shims** intercept guarded commands (`rm`, `git`, `npm`, `curl`, …) and
  evaluate them *before* they run.

Cross a limit and the whole process group is hard-stopped with a readable
banner. Works with any agent or CLI on macOS, Linux, and Windows.

### `hook` — enforce per tool-call (Claude Code & friends)

```bash
agent-leash hook
```

`hook` reads a "tool is about to be used" event as JSON on stdin and returns an
allow/deny decision — blocking the action *before* the agent takes it, with zero
leakage. It understands [Claude Code](docs/integrations.md)'s `PreToolUse`
payload natively and a generic `{tool, input}` schema for everything else.

Wire it into Claude Code (`.claude/settings.json`):

```json
{
  "hooks": {
    "PreToolUse": [
      { "hooks": [{ "type": "command", "command": "agent-leash hook" }] }
    ]
  }
}
```

Now every `Bash`, `Write`, `Edit`, and `WebFetch` the agent attempts is checked
against your `.agent-leash.toml` first. See [docs/integrations.md](docs/integrations.md)
for Cursor, Aider, Codex, and custom agents.

## The policy file

`agent-leash` runs with a protective default policy and needs no configuration.
To customize, run `agent-leash init` and edit `.agent-leash.toml`:

```toml
[limits]
max_files_changed = 25     # distinct files the agent may create or modify
max_new_deps      = 3       # dependencies it may add across all manifests
max_commands      = 0       # guarded commands it may run (0 = unlimited)
max_duration      = "20m"   # wall-clock budget
max_cost_usd      = 5.0     # spend budget (enforced when usage is reported)

[network]
allowed = false             # deny curl/wget/npm-install/git-pull/… this session

[filesystem]
workspace     = "."         # the only tree the agent may modify
allow_outside = false       # block writes outside the workspace
protect = ["~/.ssh", "~/.aws", "~/.npmrc", "~/.netrc"]  # always off-limits

[commands]
deny = ["rm -rf /*", "git push --force*", "* | sh"]     # extends the built-ins
# allow = ["git", "go", "npm"]   # uncomment for strict allowlist mode

[on_violation]
action = "stop"             # stop (kill), warn (log & continue), or ask
```

Full reference: [docs/policy.md](docs/policy.md).

## How it works

```
                 agent-leash run -- <agent>
                          │
        ┌─────────────────┼───────────────────────────┐
        │                 │                            │
   PATH shims        file watcher                 budget meter
   (rm, git, npm,    (counts changed              (time, spend,
    curl, sudo…)      files, ignores               commands)
        │              build dirs)                     │
        ▼                 │                            │
   guard-exec ──┐         ▼                            ▼
   evaluate the │     manifest scan ───────────►  limit crossed?
   command vs   │     (new deps)                       │
   the policy   │                                      │ yes
        │       └──────────────► session log ──────────┤
        ▼                                               ▼
   allow → exec real binary                  ⛔ hard-stop the process group
   deny  → block + record                       (SIGTERM → SIGKILL) + report
```

The command classifier, budget meters, manifest parsers, and policy loader are
all pure and unit-tested; the system-specific pieces (process-group control,
filesystem watching, PATH shims, the optional sandbox) are kept thin. See
[docs/security.md](docs/security.md) for the threat model.

## What it is — and isn't (read this)

`agent-leash` is **defense-in-depth against an agent that goes off the rails**,
not a sandbox that contains a determined attacker.

- ✅ It reliably catches the things agents actually do wrong: destructive
  commands, dependency bloat, runaway file churn, blown time/spend budgets, and
  reads/writes of secrets or paths outside your project.
- ✅ `run` intercepts commands the agent executes through the shell's `PATH`
  (the overwhelming majority) and `hook` blocks tool calls before they happen.
- ⚠️ `run`'s command interception is not a kernel jail: a process that calls a
  binary by absolute path, or makes raw syscalls, can bypass the PATH shims. For
  a real network kill on Linux, add `--harden` (drops the agent into a private
  network namespace). For the strongest pre-action guarantee, use the `hook`.
- ⚠️ Spend (`max_cost_usd`) is enforced when usage is reported to agent-leash;
  files, deps, commands, time, and network are enforced directly.

We'd rather tell you exactly where the edges are than oversell a number. Details
in [docs/security.md](docs/security.md).

## Privacy

`agent-leash` runs entirely on your machine and **makes no network connections**
— no telemetry, no update checks, nothing. It reads your policy, watches your
workspace, and supervises the command you ask it to run. The only thing it
changes about your system is stopping the agent you pointed it at.

## Contributing

Contributions are very welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). Good
first issues include new deny-pattern presets, more manifest ecosystems,
adapters for additional agent platforms in the `hook`, and richer Windows
process-group handling. Please also read our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE) © agent-leash contributors.
