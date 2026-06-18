# Integrations

`agent-leash` enforces one policy through two mechanisms, so it fits almost any
agent:

- **`run`** wraps the agent as a child process — works with *anything* that runs
  in a terminal, no integration required.
- **`hook`** evaluates one tool call at a time — for platforms that can call a
  command before they act.

## Anything with a CLI (universal)

If your agent runs in a terminal, just put it behind `run`:

```bash
agent-leash run -- <your agent command and args>
```

Examples:

```bash
agent-leash run -- claude
agent-leash run -- cursor-agent
agent-leash run --no-network -- aider --model gpt-4o
agent-leash run --max-files 15 -- ./my_agent.py --task "fix the flaky test"
```

No SDK, no plugin — the agent runs normally and `agent-leash` supervises the
whole process group.

## Claude Code

Claude Code can call a command before every tool use, which `agent-leash hook`
implements directly.

Add to `.claude/settings.json` (project) or `~/.claude/settings.json` (global):

```json
{
  "hooks": {
    "PreToolUse": [
      { "hooks": [{ "type": "command", "command": "agent-leash hook" }] }
    ]
  }
}
```

`agent-leash hook` reads Claude Code's `PreToolUse` payload on stdin and replies
with the permission-decision protocol:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "agent-leash: matches a denied pattern (git push --force*)"
  }
}
```

- `Bash` tool calls are checked as commands.
- `Write` / `Edit` / `MultiEdit` are checked for workspace escape and protected
  paths.
- `Read` is checked against protected paths.
- `WebFetch` / `WebSearch` are denied when the policy disables the network.

When `[on_violation] action = "ask"`, a denial becomes `permissionDecision:
"ask"`, prompting you instead of blocking outright. With `action = "warn"`, it
is allowed and logged.

> **Belt and suspenders:** run Claude Code *inside* `agent-leash run` **and**
> register the hook. The hook blocks individual tool calls pre-emptively; the
> supervisor enforces the session-wide budgets (files, deps, time, spend) and is
> the backstop if a hook is ever bypassed.

## Cursor, Aider, Codex, and custom agents

Any platform that can shell out on a tool call can use the **generic** format:

```bash
echo '{"tool":"run_command","input":{"command":"rm -rf /"}}' \
  | agent-leash hook --format generic
```

```json
{
  "allow": false,
  "decision": "deny",
  "reason": "agent-leash: matches a denied pattern (rm -rf /*)",
  "category": "denied-command",
  "rule": "rm -rf /*"
}
```

The process exits non-zero on a deny, so hook systems that key on exit status
block automatically. The parser accepts either field naming:

| Field | Claude Code | Generic |
|-------|-------------|---------|
| tool name | `tool_name` | `tool` |
| arguments | `tool_input` | `input` |
| working dir | `cwd` | `cwd` |

Recognized tool families (case-insensitive): commands (`bash`, `shell`, `run`,
`execute`, …), writes (`write`, `edit`, `multiedit`, `apply_patch`, …), reads
(`read`, `cat`, `view`, …), and network (`webfetch`, `websearch`, `fetch`, …).
Unknown tools are allowed unless they carry a `command` field.

## Reporting spend (so `max_cost_usd` works)

`agent-leash` can't see your model bill, so spend is enforced only when it's
reported to the running session. Two ways:

```bash
# From your agent or a wrapper, whenever you have an updated cost:
agent-leash report --cost 0.12
```

`report` is a no-op when not inside `agent-leash run`, so it's safe to call
unconditionally. Alternatively, if a hook payload carries a cost field
(`total_cost_usd`, `cost_usd`, `cost`, or `usd`), `agent-leash hook` meters it
automatically — wire `hook` into a `PostToolUse`/`Stop` event that includes cost.

## CI / pre-commit gate

`agent-leash check` evaluates a command without running anything and exits `1`
on a deny, so it doubles as a guard in scripts:

```bash
agent-leash check -- "$PROPOSED_COMMAND" || {
  echo "blocked by policy"; exit 1;
}
```
