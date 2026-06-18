# Security model & threat model

`agent-leash` is **defense-in-depth against an agent that goes off the rails** —
a careless or looping AI agent, not a malicious human adversary with local code
execution. This page is deliberately blunt about what it does and does not
guarantee, so you can decide how much to lean on it.

## What it defends against well

- **Destructive commands.** `rm -rf /`, `git push --force`, `git reset --hard`,
  `mkfs`, `dd of=/dev/…`, `chmod -R 777`, `curl … | sh`, fork bombs, `sudo`, and
  your own deny patterns — blocked before they execute.
- **Dependency bloat.** A cap on net-new dependencies across manifests.
- **Runaway file churn.** A cap on how many files a session may change, with
  build output and caches ignored.
- **Blown budgets.** Hard wall-clock and (reported) spend limits.
- **Secret access & workspace escape.** Reads or writes to `~/.ssh`, `~/.aws`,
  and other protected paths, or writes outside the project, are denied.
- **The network**, when disabled — at the command level everywhere, and at the
  kernel level on Linux with `--harden`.

## How enforcement works

| Mechanism | Used by | Strength |
|-----------|---------|----------|
| Command classifier | `run` shims, `hook`, `check` | Pure, deterministic, unit-tested. |
| PATH shims | `run` | Intercept commands invoked through `PATH` (how agents normally run shell commands). |
| PreToolUse hook | `hook` | Blocks a tool call **before** it executes — the strongest pre-action guarantee. |
| Filesystem watcher | `run` | Counts changes live via the OS; kills on the limit. |
| Process-group kill | `run` | `SIGTERM` → `SIGKILL` to the agent's entire group. |
| Network namespace | `run --harden` (Linux) | Kernel-level network isolation via `unshare`. |

## Known limits — please read

- **PATH shims are not a kernel jail.** Under `run`, a child that invokes a
  binary by **absolute path** (`/bin/rm`), or that issues raw syscalls, can
  sidestep the shims. The shims cover the common case (commands run via the
  shell's `PATH`); they are not a containment boundary against deliberate
  evasion. For pre-action certainty, use the `hook`; for network, use `--harden`.
- **The shell isn't fully parsed.** The classifier splits on shell operators and
  inspects each command with conservative tokenizing. It is tuned to catch real
  agent behavior, not to defeat an adversary crafting deliberately obfuscated
  command lines (heavy `eval`, base64-piped payloads, exotic quoting).
- **`hook` sees one call at a time.** It cannot enforce session-wide budgets
  (files/deps/time) by itself — pair it with `run` for those.
- **Spend is best-effort.** `max_cost_usd` is enforced only when usage is
  reported to the meter. Time, files, deps, commands, and network are enforced
  directly.
- **`--harden` is Linux-only** and uses unprivileged user namespaces, which some
  distributions disable. Where unavailable, network is still blocked at the
  command level and `agent-leash` says so.

The honest summary: `agent-leash` dramatically reduces the blast radius of a
misbehaving agent and stops the mistakes agents actually make. It is one layer —
run your agents with the OS-level permissions you're comfortable with, too.

## Reporting a vulnerability

Please report security issues privately — see [SECURITY.md](../SECURITY.md).
