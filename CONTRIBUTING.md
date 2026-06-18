# Contributing to agent-leash

Thanks for your interest in contributing! `agent-leash` aims to be a small,
fast, dependency-light tool with an honest security story — contributions that
keep it that way are especially appreciated.

## Getting started

```bash
git clone https://github.com/agenticraptor/agent-leash
cd agent-leash
go mod tidy        # fetch dependencies & populate go.sum
make build         # build into ./bin/agent-leash
make test          # run the unit tests
make run ARGS="doctor"
```

Requirements:

- Go 1.22 or newer
- (optional) [`golangci-lint`](https://golangci-lint.run/) for `make lint`
- (optional) [`goreleaser`](https://goreleaser.com/) for `make snapshot`

## Development workflow

1. Fork the repo and create a feature branch from `main`.
2. Make your change, with tests where it makes sense.
3. Run the full check suite locally:
   ```bash
   make fmt vet test
   ```
4. Open a pull request. Fill in the PR template and link any related issue.

CI runs `gofmt`, `go vet`, `golangci-lint`, and the test suite on Linux, macOS,
and Windows. All checks must pass before review.

## Architecture at a glance

The pure logic is isolated from the system calls so it stays easy to test:

- `internal/policy` — load and validate the TOML policy; built-in defaults.
- `internal/guard` — the command classifier. Pure and heavily unit-tested: deny
  patterns, network detection, protected paths, workspace escape.
- `internal/budget` — the meters (files, deps, commands, time, cost) and the
  first-violation check.
- `internal/manifest` — dependency counting and diffing across ecosystems.
- `internal/workspace` — the change tracker (pure) and the fsnotify watcher.
- `internal/shim` — PATH-shim generation and the `guard-exec` body.
- `internal/sandbox` — optional OS hardening (Linux network namespace).
- `internal/supervisor` — orchestration: child process group, watcher, event
  tailer, and the kill-switch.
- `internal/hook` — the tool-call adapter (Claude Code + generic) and responses.
- `internal/report` — the banners and the session report (lipgloss).
- `internal/cli` — the cobra command wiring.

Prefer putting new behavior in a pure function in `guard`, `budget`, `manifest`,
or `policy` (easy to test) and keeping the system calls thin.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/). This keeps
the generated changelog readable and drives semantic-version bumps.

```
feat: add a deny preset for cloud-CLI destructive commands
fix: count files created in a brand-new subdirectory
docs: clarify the PATH-shim limitation
test: cover the chained-command network path
chore: bump fsnotify to v1.8.0
```

## Coding guidelines

- **Keep dependencies minimal.** If a new dependency is truly needed, call it out
  in the PR description.
- **Be honest about enforcement.** Anything that affects what is or isn't
  blocked must be reflected accurately in `docs/security.md`. We never oversell a
  guarantee.
- **Degrade gracefully.** If a capability is unavailable (no fsnotify, no
  `unshare`, can't write shims), reduce scope and tell the user — never crash a
  session.
- **Killing is serious.** Anything that terminates a process must be explicit and
  target the agent's group, never `agent-leash` itself.
- **Format with `gofmt -s`** and keep `go vet` clean.

## Good first issues

- New deny-pattern presets (cloud CLIs, Kubernetes, database drops).
- Additional manifest ecosystems (e.g. `pubspec.yaml`, `mix.exs`).
- More `hook` tool-name aliases for other agent platforms.
- Richer Windows process-group handling.

## Reporting bugs & requesting features

Use the [issue templates](https://github.com/agenticraptor/agent-leash/issues/new/choose).
For anything security-related, please follow [SECURITY.md](SECURITY.md) instead
of opening a public issue.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).
