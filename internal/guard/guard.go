// Package guard decides whether a single command (or a chained command line) is
// allowed to run under a policy. It is the pure, deterministic core behind both
// the PATH-shim interception used by `agent-leash run` and the tool-call hook
// used by `agent-leash hook`. It performs no I/O and executes nothing — given a
// command and a policy it returns an allow/deny Decision with a readable reason.
package guard

import (
	"path/filepath"
	"strings"

	"github.com/agenticraptor/agent-leash/internal/policy"
)

// Category labels why a command was denied, for machine-readable consumers.
type Category string

// Decision categories.
const (
	CatAllowed   Category = "allowed"
	CatDenyRule  Category = "denied-command"
	CatNetwork   Category = "network-blocked"
	CatAllowlist Category = "not-in-allowlist"
	CatProtected Category = "protected-path"
	CatEscape    Category = "workspace-escape"
)

// Decision is the result of evaluating a command against a policy.
type Decision struct {
	Allow    bool
	Category Category
	Reason   string // human-readable explanation
	Rule     string // the specific pattern or path that matched, if any
	Command  string // the offending command segment (normalized)
}

func allowed() Decision { return Decision{Allow: true, Category: CatAllowed} }

// Guard evaluates commands against a fixed policy.
type Guard struct {
	deny         []string
	allow        map[string]bool
	allowMode    bool
	networkOK    bool
	allowOutside bool
	workspace    string
	protected    []string
	guardSet     map[string]bool
}

// New builds a Guard from a policy and the absolute workspace root the agent is
// confined to.
func New(cfg policy.Config, workspaceRoot string) *Guard {
	g := &Guard{
		deny:         append(policy.DefaultDeny(), cfg.Commands.Deny...),
		allow:        toSet(cfg.Commands.Allow),
		allowMode:    len(cfg.Commands.Allow) > 0,
		networkOK:    cfg.Network.Allowed,
		allowOutside: cfg.Filesystem.AllowOutside,
		workspace:    filepath.Clean(workspaceRoot),
		protected:    cfg.ProtectedPaths(),
		guardSet:     toSet(orDefault(cfg.Commands.Guard, policy.DefaultGuard())),
	}
	// The built-in deny list already lives in the policy default; when a file
	// overrides cfg.Commands.Deny, re-add the built-ins so they cannot be
	// silently dropped. De-duplicate to keep matching cheap.
	g.deny = dedupe(g.deny)
	return g
}

// Guarded reports whether bin (a binary name) is one we install a shim for.
func (g *Guard) Guarded(bin string) bool {
	return g.guardSet[filepath.Base(bin)]
}

// GuardSet returns the sorted set of guarded binary names.
func (g *Guard) GuardSet() []string { return keys(g.guardSet) }

// Check evaluates a full command line, which may chain several commands with
// ;, &&, ||, or |. The first denial found wins; otherwise the command is
// allowed.
func (g *Guard) Check(line string) Decision {
	return g.evaluate(line, splitSegments(line))
}

// CheckArgv evaluates a command already split into arguments (for example the
// argv a shim intercepts), without re-parsing shell syntax.
func (g *Guard) CheckArgv(argv []string) Decision {
	line := strings.Join(argv, " ")
	return g.evaluate(line, []segment{{Args: argv}})
}

// CheckFileWrite evaluates a direct file modification (used by editor-style tool
// hooks) against the protected-path and workspace-escape rules.
func (g *Guard) CheckFileWrite(path string) Decision {
	abs := g.resolve(path)
	if rule, ok := g.underProtected(abs); ok {
		return Decision{Category: CatProtected, Command: path,
			Reason: "writes to a protected path (" + rule + ")", Rule: rule}
	}
	if !g.allowOutside && !g.underWorkspace(abs) {
		return Decision{Category: CatEscape, Command: path,
			Reason: "writes outside the workspace (" + g.workspace + ")", Rule: abs}
	}
	return allowed()
}

// CheckFileRead evaluates a direct file read (used by editor-style tool hooks)
// against the protected-path rule only. Reading outside the workspace is
// permitted; reading a protected secret is not.
func (g *Guard) CheckFileRead(path string) Decision {
	abs := g.resolve(path)
	if rule, ok := g.underProtected(abs); ok {
		return Decision{Category: CatProtected, Command: path,
			Reason: "reads a protected path (" + rule + ")", Rule: rule}
	}
	return allowed()
}

func (g *Guard) evaluate(line string, segs []segment) Decision {
	norm := normalize(line)
	if norm == "" {
		return allowed()
	}
	// Whole-line deny patterns catch cross-command shapes like "curl … | sh".
	if rule, ok := g.matchDeny(norm); ok {
		return Decision{Category: CatDenyRule, Command: norm,
			Reason: "matches a denied pattern (" + rule + ")", Rule: rule}
	}
	for _, seg := range segs {
		if d := g.checkSegment(seg); !d.Allow {
			return d
		}
	}
	return allowed()
}

func (g *Guard) checkSegment(seg segment) Decision {
	segNorm := normalize(strings.Join(seg.Args, " "))
	if segNorm == "" && len(seg.Redirects) == 0 {
		return allowed()
	}
	if rule, ok := g.matchDeny(segNorm); ok {
		return Decision{Category: CatDenyRule, Command: segNorm,
			Reason: "matches a denied pattern (" + rule + ")", Rule: rule}
	}

	args := stripEnvAndWrappers(seg.Args)
	if len(args) == 0 {
		return g.checkProtectedTokens(seg, segNorm) // e.g. a bare redirect
	}
	cmd := filepath.Base(args[0])

	// Network.
	if !g.networkOK && isNetworkUse(cmd, args[1:]) {
		return Decision{Category: CatNetwork, Command: segNorm,
			Reason: "network access is disabled by policy", Rule: cmd}
	}
	// Allowlist mode.
	if g.allowMode && !g.allow[cmd] {
		return Decision{Category: CatAllowlist, Command: segNorm,
			Reason: "command is not in the allowlist", Rule: cmd}
	}
	// Protected paths (any token that resolves under a protected path).
	if d := g.checkProtectedTokens(seg, segNorm); !d.Allow {
		return d
	}
	// Workspace escape for mutating commands and redirect targets.
	for _, p := range g.writeTargets(cmd, args[1:], seg.Redirects) {
		abs := g.resolve(p)
		if !g.allowOutside && !g.underWorkspace(abs) {
			return Decision{Category: CatEscape, Command: segNorm,
				Reason: "writes outside the workspace (" + g.workspace + ")", Rule: p}
		}
	}
	return allowed()
}

// checkProtectedTokens denies a segment if any path-like token resolves to a
// protected location, regardless of the command (reads of secrets count too).
func (g *Guard) checkProtectedTokens(seg segment, segNorm string) Decision {
	toks := append([]string{}, seg.Args...)
	toks = append(toks, seg.Redirects...)
	for _, t := range toks {
		if !looksLikePath(t) {
			continue
		}
		abs := g.resolve(t)
		if rule, ok := g.underProtected(abs); ok {
			return Decision{Category: CatProtected, Command: segNorm,
				Reason: "touches a protected path (" + rule + ")", Rule: rule}
		}
	}
	return allowed()
}

func (g *Guard) matchDeny(s string) (string, bool) {
	for _, pat := range g.deny {
		if globMatch(pat, s) {
			return pat, true
		}
	}
	return "", false
}

func (g *Guard) resolve(p string) string {
	e := policy.ExpandPath(p)
	if !filepath.IsAbs(e) {
		e = filepath.Join(g.workspace, e)
	}
	return filepath.Clean(e)
}

func (g *Guard) underWorkspace(abs string) bool { return underDir(abs, g.workspace) }

func (g *Guard) underProtected(abs string) (string, bool) {
	for _, p := range g.protected {
		if underDir(abs, p) {
			return p, true
		}
	}
	return "", false
}

func underDir(path, dir string) bool {
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, ensureTrailingSep(dir))
}

func ensureTrailingSep(p string) string {
	if strings.HasSuffix(p, string(filepath.Separator)) {
		return p
	}
	return p + string(filepath.Separator)
}
