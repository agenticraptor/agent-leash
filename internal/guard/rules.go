package guard

import (
	"sort"
	"strings"
)

// pureNetwork holds binaries whose sole purpose is to talk to the network.
var pureNetwork = toSet([]string{
	"curl", "wget", "nc", "ncat", "netcat", "telnet",
	"ssh", "scp", "sftp", "ftp", "aria2c", "socat",
	"http", "https", "httpie",
})

// networkSubcommands maps tools with both local and network modes to the
// subcommands that reach out over the network.
var networkSubcommands = map[string]map[string]bool{
	"git":   toSet([]string{"clone", "fetch", "pull", "push", "remote", "ls-remote", "submodule"}),
	"npm":   toSet([]string{"install", "i", "add", "ci", "update", "up", "exec", "dlx", "create", "publish"}),
	"pnpm":  toSet([]string{"install", "i", "add", "update", "up", "dlx", "create", "publish"}),
	"yarn":  toSet([]string{"install", "add", "up", "upgrade", "create", "dlx", "publish"}),
	"pip":   toSet([]string{"install", "download"}),
	"pip3":  toSet([]string{"install", "download"}),
	"gem":   toSet([]string{"install", "update", "fetch", "push"}),
	"cargo": toSet([]string{"install", "add", "update", "fetch", "publish"}),
	"brew":  toSet([]string{"install", "update", "upgrade", "fetch", "tap"}),
	"go":    toSet([]string{"get", "install"}),
}

// mutating holds commands whose positional arguments are filesystem paths the
// command can destroy or overwrite.
var mutating = toSet([]string{
	"rm", "rmdir", "shred", "srm", "truncate", "touch", "mkdir",
	"mv", "cp", "ln", "install", "unlink", "rsync",
})

// wrappers are leading words that prefix the real command without changing it.
var wrappers = toSet([]string{"env", "nohup", "nice", "time", "command", "exec", "builtin", "stdbuf"})

// stripEnvAndWrappers removes leading VAR=value assignments and wrapper words so
// the real command word can be identified.
func stripEnvAndWrappers(args []string) []string {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case isAssignment(a):
			i++
		case wrappers[a]:
			i++
		default:
			return args[i:]
		}
	}
	return nil
}

func isAssignment(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	for j, r := range s[:eq] {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		if j > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

// isNetworkUse reports whether running cmd with rest reaches the network.
func isNetworkUse(cmd string, rest []string) bool {
	if pureNetwork[cmd] {
		return true
	}
	subs, ok := networkSubcommands[cmd]
	if !ok {
		return false
	}
	sub := firstWord(rest)
	if cmd == "go" && sub == "mod" {
		next := firstWord(skipFirstWord(rest))
		return next == "download" || next == "tidy"
	}
	return subs[sub]
}

// writeTargets returns the paths a command writes to: redirect targets always,
// plus positional path arguments for known mutating commands.
func (g *Guard) writeTargets(cmd string, rest, redirects []string) []string {
	out := append([]string{}, redirects...)
	switch cmd {
	case "dd":
		for _, a := range rest {
			if strings.HasPrefix(a, "of=") {
				out = append(out, strings.TrimPrefix(a, "of="))
			}
		}
	case "tee":
		out = append(out, nonFlagArgs(rest)...)
	case "chmod", "chown":
		paths := nonFlagArgs(rest)
		if len(paths) > 1 { // drop the mode/owner argument
			out = append(out, paths[1:]...)
		}
	default:
		if mutating[cmd] {
			out = append(out, nonFlagArgs(rest)...)
		}
	}
	return out
}

func nonFlagArgs(args []string) []string {
	var out []string
	flagsDone := false
	for _, a := range args {
		if a == "--" {
			flagsDone = true
			continue
		}
		if !flagsDone && strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func firstWord(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

func skipFirstWord(args []string) []string {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			return args[i+1:]
		}
	}
	return nil
}

// looksLikePath reports whether a token is plausibly a filesystem path (rather
// than a flag, subcommand, or option value).
func looksLikePath(t string) bool {
	if t == "" || strings.HasPrefix(t, "-") {
		return false
	}
	return strings.ContainsAny(t, "/\\") || strings.HasPrefix(t, "~")
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		if it != "" {
			m[it] = true
		}
	}
	return m
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func orDefault(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	var out []string
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}
