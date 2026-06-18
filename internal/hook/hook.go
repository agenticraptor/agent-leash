// Package hook adapts an agent platform's "about to use a tool" event into an
// allow/deny decision from agent-leash's guard. It is the precise counterpart to
// the PATH-shim interception in `run`: instead of catching a command as it
// executes, it inspects the tool call before the agent runs it.
//
// It understands Claude Code's PreToolUse payload natively and also accepts a
// minimal generic schema, so any platform that can shell out on a tool call —
// Cursor, OpenAI Codex, Aider, a custom agent — can enforce the same policy.
package hook

import (
	"encoding/json"
	"strings"

	"github.com/agenticraptor/agent-leash/internal/guard"
)

// Request is a normalized "about to use a tool" event.
type Request struct {
	Tool  string         // the tool name, e.g. "Bash", "Write", "WebFetch"
	Input map[string]any // the tool's arguments
	CWD   string         // the agent's working directory, if provided
}

// Result is the decision for a tool call.
type Result struct {
	Allow    bool
	Reason   string
	Category string
	Rule     string
	Command  string // the command or path that was evaluated, for logging
}

// Parse reads a hook payload, accepting either Claude Code's field names
// (tool_name / tool_input) or a generic schema (tool / input).
func Parse(data []byte) (Request, error) {
	var raw struct {
		ToolName  string         `json:"tool_name"`
		Tool      string         `json:"tool"`
		ToolInput map[string]any `json:"tool_input"`
		Input     map[string]any `json:"input"`
		CWD       string         `json:"cwd"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Request{}, err
	}
	req := Request{
		Tool:  firstNonEmpty(raw.ToolName, raw.Tool),
		Input: raw.ToolInput,
		CWD:   raw.CWD,
	}
	if req.Input == nil {
		req.Input = raw.Input
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	return req, nil
}

// Evaluate applies the guard to a parsed tool call. networkAllowed gates network
// tools (WebFetch/WebSearch) that have no shell command to inspect.
func Evaluate(g *guard.Guard, req Request, networkAllowed bool) Result {
	switch classify(req.Tool) {
	case toolCommand:
		cmd := stringField(req.Input, "command", "cmd", "script")
		return fromDecision(g.Check(cmd), cmd)
	case toolWrite:
		path := stringField(req.Input, "file_path", "path", "filename", "notebook_path")
		return fromDecision(g.CheckFileWrite(path), path)
	case toolRead:
		path := stringField(req.Input, "file_path", "path", "filename")
		return fromDecision(g.CheckFileRead(path), path)
	case toolNetwork:
		target := stringField(req.Input, "url", "query")
		if !networkAllowed {
			return Result{Allow: false, Category: string(guard.CatNetwork),
				Reason: "network access is disabled by policy", Rule: req.Tool, Command: target}
		}
		return Result{Allow: true, Command: target}
	default:
		// Unknown tool: inspect a command-like field if present, else allow.
		if cmd := stringField(req.Input, "command", "cmd", "script"); cmd != "" {
			return fromDecision(g.Check(cmd), cmd)
		}
		return Result{Allow: true}
	}
}

type toolKind int

const (
	toolUnknown toolKind = iota
	toolCommand
	toolWrite
	toolRead
	toolNetwork
)

func classify(tool string) toolKind {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "bash", "shell", "sh", "run", "run_command", "runcommand", "execute", "exec", "command", "terminal":
		return toolCommand
	case "write", "edit", "multiedit", "create", "write_file", "writefile", "edit_file", "editfile",
		"create_file", "str_replace_editor", "apply_patch", "notebookedit":
		return toolWrite
	case "read", "read_file", "readfile", "cat", "view", "open":
		return toolRead
	case "webfetch", "websearch", "fetch", "browse", "http_request", "web_search", "web_fetch":
		return toolNetwork
	default:
		return toolUnknown
	}
}

func fromDecision(d guard.Decision, command string) Result {
	return Result{
		Allow:    d.Allow,
		Reason:   d.Reason,
		Category: string(d.Category),
		Rule:     d.Rule,
		Command:  command,
	}
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
