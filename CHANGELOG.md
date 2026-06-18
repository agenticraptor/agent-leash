# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-06-16

### Added

- Initial release. 🎉
- `agent-leash run -- <command>`: supervise any agent or CLI live — installs
  PATH shims to intercept guarded commands, watches the workspace for file
  changes, scans manifests for new dependencies, meters wall-clock time and
  reported spend, and hard-stops the agent's whole process group the instant a
  limit is crossed.
- `agent-leash hook`: enforce the same policy per tool-call. Speaks Claude Code's
  `PreToolUse` permission-decision protocol and a generic `{tool, input}` schema
  for any other platform (Cursor, Aider, Codex, custom agents).
- `agent-leash check`: evaluate a command or file action against the policy
  without running anything (exit 0 allow / 1 deny).
- `agent-leash report --cost <usd>`: report spend to the supervising session so
  `max_cost_usd` is enforced; `hook` also meters a cost field if the payload
  carries one.
- `agent-leash init` / `doctor` / `version`.
- Command classifier covering destructive commands, network use (with off-line
  enforcement), protected-path access, and workspace escape — with a built-in
  deny list and optional allowlist mode.
- Dependency counting across npm, Go, pip (`requirements.txt` + `pyproject.toml`),
  Cargo, RubyGems, and Composer manifests.
- TOML policy with project-local (`.agent-leash.toml`) and XDG discovery; runs
  with protective defaults and zero configuration.
- Optional Linux OS hardening (`run --harden`) via an unprivileged network
  namespace, with an honest fallback elsewhere.
- Single static binary; no telemetry, no network, no account. macOS, Linux, and
  Windows (amd64 + arm64).

[Unreleased]: https://github.com/agenticraptor/agent-leash/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/agenticraptor/agent-leash/releases/tag/v0.1.0
