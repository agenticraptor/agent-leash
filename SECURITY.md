# Security Policy

## Supported versions

The latest released minor version receives security fixes. Please upgrade to the
most recent release before reporting an issue.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Instead, use GitHub's private vulnerability reporting on this repository:
[**Report a vulnerability**](https://github.com/agenticraptor/agent-leash/security/advisories/new)
(GitHub → the repo's **Security** tab → **Report a vulnerability**).

Please include:

- A description of the issue and its impact.
- Steps to reproduce (a minimal proof of concept is ideal).
- Affected version(s) and platform.

We aim to acknowledge reports within **72 hours** and to provide a remediation
timeline after triage. We will credit reporters in the release notes unless you
prefer to remain anonymous.

## Scope & what agent-leash is

`agent-leash` is **defense-in-depth against a misbehaving AI agent**, not a
sandbox that contains a determined local attacker. Before reporting, please read
[docs/security.md](docs/security.md), which documents the threat model and the
known limits in detail. In particular:

- The PATH shims used by `run` intercept commands invoked through `PATH`; a
  process that calls a binary by absolute path or issues raw syscalls can bypass
  them. This is a documented limitation, not a vulnerability.
- The command classifier is tuned to catch the mistakes agents actually make, not
  to defeat deliberately obfuscated command lines.

Reports that strengthen enforcement against **realistic agent behavior**, that
find a way the tool fails to enforce a limit it claims to enforce, or that affect
`agent-leash` itself (for example, a shim resolving to itself, or a policy parse
that crashes) are very much in scope and welcome.

## Data handling notes

- **No network, no telemetry.** `agent-leash` never makes outbound network
  connections. It reads your policy, watches your workspace, and supervises the
  command you give it.
- **It terminates processes.** `run` sends real signals (`SIGTERM` → `SIGKILL`,
  `taskkill /T` on Windows) to the agent's process group when a limit is crossed.
  It never targets `agent-leash` itself.
