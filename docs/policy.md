# Policy reference

`agent-leash` is governed by a single TOML policy. It works with **no policy at
all** — a protective default applies — so this file only documents how to tune
it.

## Where the policy comes from

When `agent-leash` starts it resolves the active policy in this order:

1. an explicit `--policy <path>` flag;
2. the `AGENT_LEASH_POLICY` environment variable;
3. the nearest `.agent-leash.toml`, searching from the workspace directory
   upward (so a repo-root policy applies to every subdirectory);
4. the user-level file at `${XDG_CONFIG_HOME:-~/.config}/agent-leash/policy.toml`;
5. the built-in defaults.

Create a starter file with `agent-leash init` (project-local) or
`agent-leash init --global` (user-level). Any field you omit keeps its default,
so a partial file is fine.

## Sections

### `[limits]`

Countable budgets for a session. **`0` (or an empty duration) means "no limit".**

| Key | Type | Default | Meaning |
|-----|------|---------|---------|
| `max_files_changed` | int | `50` | Distinct files the agent may create or modify (build dirs and caches are ignored). |
| `max_new_deps` | int | `5` | Dependencies added across all manifests, measured by identity (swapping one for another doesn't count). |
| `max_commands` | int | `0` | Guarded commands the agent may run. |
| `max_duration` | duration | `"30m"` | Wall-clock budget, e.g. `"90s"`, `"20m"`, `"1h30m"`. |
| `max_cost_usd` | float | `0` | Spend budget in USD; enforced when usage is reported (see [Cost](#cost)). |

### `[network]`

| Key | Type | Default | Meaning |
|-----|------|---------|---------|
| `allowed` | bool | `true` | When `false`, denies known network commands (`curl`, `wget`, `nc`, …) and network subcommands (`npm install`, `git pull`, `pip install`, `go get`, …). With `agent-leash run --harden` on Linux it also drops the agent into a private network namespace with no connectivity. |

Network is **on** by default so ordinary installs keep working. Turn it off for
untrusted or offline-only sessions.

### `[filesystem]`

| Key | Type | Default | Meaning |
|-----|------|---------|---------|
| `workspace` | string | `"."` | The only directory tree the agent may modify. |
| `allow_outside` | bool | `false` | When `false`, a write outside the workspace is a violation. |
| `protect` | []string | secrets list | Paths that are always off-limits — even for reads — regardless of the workspace. `~` and `$VAR` are expanded. |

The default `protect` list covers `~/.ssh`, `~/.aws`, `~/.config/gh`, `~/.gnupg`,
`~/.kube`, `~/.docker/config.json`, `~/.npmrc`, and `~/.netrc`.

### `[commands]`

| Key | Type | Meaning |
|-----|------|---------|
| `deny` | []string | Wildcard patterns that hard-stop the session. **Extends** the built-in deny list. |
| `allow` | []string | If non-empty, switches to **allowlist mode**: only these command words may run; everything else is denied. |
| `guard` | []string | Binary names that get PATH shims under `run`. Override only to add or remove tools from the [built-in set](#guarded-commands). |

**Pattern syntax.** Patterns are matched against each normalized command in a
(possibly chained) line, with `*` matching any run of characters — including
spaces and `/` — and `?` matching exactly one. Examples:

```toml
deny = [
  "rm -rf /*",            # any rm -rf of an absolute path
  "git push --force*",    # force-push in any form
  "* | sh",               # piping anything into a shell
  "kubectl delete *",     # your own additions
]
```

The built-in deny list always applies (it cannot be silently dropped by
overriding `deny`). See it any time with `agent-leash doctor`.

### `[on_violation]`

| Key | Type | Default | Meaning |
|-----|------|---------|---------|
| `action` | string | `"stop"` | `stop` kills the agent; `warn` logs and continues; `ask` turns a denial into a prompt (in the `hook`) or a soft block (in `run`). |
| `kill_grace` | duration | `"5s"` | How long to wait after `SIGTERM` before escalating to `SIGKILL`. |

## Guarded commands

Out of the box `run` installs shims for: `rm`, `rmdir`, `dd`, `mkfs`, `shred`,
`srm`, `chmod`, `chown`, `mv`, `curl`, `wget`, `scp`, `ssh`, `sftp`, `nc`,
`ncat`, `telnet`, `sudo`, `doas`, `git`, `npm`, `pnpm`, `yarn`, `pip`, `pip3`,
`gem`, `cargo`, and `brew`. Other binaries run normally.

## Cost

Time, files, dependencies, commands, and network are enforced directly and
always apply. **Spend (`max_cost_usd`) is enforced when spend is reported to the
running session**, because `agent-leash` can't see your model bill on its own.
There are two ways to report it:

```bash
# 1. From your agent or a wrapper script, whenever you have an updated cost:
agent-leash report --cost 0.12        # added to the session total

# 2. Automatically: if a hook payload includes a cost field
#    (total_cost_usd / cost_usd / cost / usd), `agent-leash hook` meters it.
```

Each reported value is added to the session total and checked against
`max_cost_usd` immediately. If you can't report cost from your platform, leave
`max_cost_usd = 0` and rely on `max_duration` as a practical proxy for runaway
spend.

## A worked example

A strict policy for an unattended overnight refactor:

```toml
[limits]
max_files_changed = 40
max_new_deps      = 0      # no new dependencies at all
max_duration      = "2h"

[network]
allowed = false           # offline; pair with `run --harden` on Linux

[filesystem]
workspace = "."
protect   = ["~/.ssh", "~/.aws", "~/.config", "../"]

[commands]
deny = ["git push*", "gh *"]   # no pushing or GitHub CLI

[on_violation]
action = "stop"
```
