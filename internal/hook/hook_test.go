package hook

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/policy"
)

func testGuard(t *testing.T, ws string, mutate func(*policy.Config)) *guard.Guard {
	t.Helper()
	cfg := policy.Default()
	if mutate != nil {
		mutate(&cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return guard.New(cfg, ws)
}

func TestParseClaudeSchema(t *testing.T) {
	data := []byte(`{
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf /"},
		"cwd": "/work"
	}`)
	req, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if req.Tool != "Bash" || req.CWD != "/work" {
		t.Errorf("unexpected request: %+v", req)
	}
	if got := req.Input["command"]; got != "rm -rf /" {
		t.Errorf("command = %v", got)
	}
}

func TestParseGenericSchema(t *testing.T) {
	data := []byte(`{"tool":"write_file","input":{"path":"/etc/passwd"}}`)
	req, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if req.Tool != "write_file" || req.Input["path"] != "/etc/passwd" {
		t.Errorf("unexpected request: %+v", req)
	}
}

func TestEvaluateBashDeny(t *testing.T) {
	g := testGuard(t, "/ws", nil)
	res := Evaluate(g, Request{Tool: "Bash", Input: map[string]any{"command": "rm -rf /"}}, true)
	if res.Allow {
		t.Fatal("rm -rf / should be denied")
	}
	if res.Category != string(guard.CatDenyRule) {
		t.Errorf("category = %s", res.Category)
	}
}

func TestEvaluateBashAllow(t *testing.T) {
	g := testGuard(t, "/ws", nil)
	res := Evaluate(g, Request{Tool: "Bash", Input: map[string]any{"command": "go test ./..."}}, true)
	if !res.Allow {
		t.Errorf("go test should be allowed: %s", res.Reason)
	}
}

func TestEvaluateWriteEscape(t *testing.T) {
	ws := t.TempDir()
	g := testGuard(t, ws, nil)
	res := Evaluate(g, Request{Tool: "Write", Input: map[string]any{"file_path": "/etc/hosts"}}, true)
	if res.Allow || res.Category != string(guard.CatEscape) {
		t.Errorf("write outside workspace should be denied: %+v", res)
	}
	in := Evaluate(g, Request{Tool: "Write", Input: map[string]any{"file_path": filepath.Join(ws, "main.go")}}, true)
	if !in.Allow {
		t.Errorf("write inside workspace should be allowed: %s", in.Reason)
	}
}

func TestEvaluateReadProtected(t *testing.T) {
	ws := t.TempDir()
	secret := t.TempDir()
	g := testGuard(t, ws, func(c *policy.Config) { c.Filesystem.Protect = []string{secret} })
	res := Evaluate(g, Request{Tool: "Read", Input: map[string]any{"file_path": filepath.Join(secret, "id_rsa")}}, true)
	if res.Allow || res.Category != string(guard.CatProtected) {
		t.Errorf("reading a protected path should be denied: %+v", res)
	}
	// Reading outside the workspace but not protected is allowed for Read.
	ok := Evaluate(g, Request{Tool: "Read", Input: map[string]any{"file_path": "/usr/include/stdio.h"}}, true)
	if !ok.Allow {
		t.Errorf("ordinary read outside workspace should be allowed: %s", ok.Reason)
	}
}

func TestEvaluateNetworkTool(t *testing.T) {
	g := testGuard(t, "/ws", nil)
	deny := Evaluate(g, Request{Tool: "WebFetch", Input: map[string]any{"url": "https://x"}}, false)
	if deny.Allow || deny.Category != string(guard.CatNetwork) {
		t.Errorf("WebFetch with network off should be denied: %+v", deny)
	}
	allow := Evaluate(g, Request{Tool: "WebFetch", Input: map[string]any{"url": "https://x"}}, true)
	if !allow.Allow {
		t.Error("WebFetch with network on should be allowed")
	}
}

func TestClaudeResponseDecisions(t *testing.T) {
	deny := Result{Allow: false, Reason: "blocked"}
	body, code := ClaudeResponse(deny, policy.ActionStop)
	if code != 0 {
		t.Errorf("claude response exit code = %d, want 0", code)
	}
	var out claudeOutput
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("decision = %s, want deny", out.HookSpecificOutput.PermissionDecision)
	}
	if !strings.Contains(out.HookSpecificOutput.PermissionDecisionReason, "blocked") {
		t.Error("reason should be carried through")
	}

	// "ask" action turns a denial into a prompt; "warn" lets it through.
	if b, _ := ClaudeResponse(deny, policy.ActionAsk); !strings.Contains(string(b), `"ask"`) {
		t.Error("ask action should yield permissionDecision ask")
	}
	if b, _ := ClaudeResponse(deny, policy.ActionWarn); !strings.Contains(string(b), `"allow"`) {
		t.Error("warn action should yield permissionDecision allow")
	}
}

func TestGenericResponseExitCodes(t *testing.T) {
	if _, code := GenericResponse(Result{Allow: true}, policy.ActionStop); code != 0 {
		t.Errorf("allow exit code = %d, want 0", code)
	}
	if _, code := GenericResponse(Result{Allow: false, Reason: "x"}, policy.ActionStop); code != 1 {
		t.Errorf("deny exit code = %d, want 1", code)
	}
	if _, code := GenericResponse(Result{Allow: false, Reason: "x"}, policy.ActionWarn); code != 0 {
		t.Errorf("warn exit code = %d, want 0", code)
	}
}
