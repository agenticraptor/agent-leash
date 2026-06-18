package hook

import "encoding/json"

// Decision values used in responses.
const (
	decisionAllow = "allow"
	decisionDeny  = "deny"
	decisionAsk   = "ask"
)

// resolveDecision maps a guard Result plus the policy's on-violation action into
// a final decision string. "warn" downgrades a denial to allow (it is logged
// elsewhere); "ask" turns it into a prompt; "stop" denies outright.
func resolveDecision(res Result, action string) (decision, reason string) {
	if res.Allow {
		return decisionAllow, res.Reason
	}
	switch action {
	case "warn":
		return decisionAllow, "agent-leash (warn): " + res.Reason
	case "ask":
		return decisionAsk, "agent-leash: " + res.Reason
	default:
		return decisionDeny, "agent-leash: " + res.Reason
	}
}

type claudeOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// ClaudeResponse renders a Claude Code PreToolUse decision as JSON on stdout
// with a zero exit code (Claude Code reads the decision from the JSON body).
func ClaudeResponse(res Result, action string) ([]byte, int) {
	decision, reason := resolveDecision(res, action)
	var out claudeOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = decision
	out.HookSpecificOutput.PermissionDecisionReason = reason
	b, _ := json.MarshalIndent(out, "", "  ")
	return b, 0
}

type genericOutput struct {
	Allow    bool   `json:"allow"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Category string `json:"category,omitempty"`
	Rule     string `json:"rule,omitempty"`
}

// GenericResponse renders a platform-neutral decision as JSON, with a non-zero
// exit code on deny so shell-based hooks that key on the exit status also block.
func GenericResponse(res Result, action string) ([]byte, int) {
	decision, reason := resolveDecision(res, action)
	out := genericOutput{
		Allow:    decision != decisionDeny,
		Decision: decision,
		Reason:   reason,
		Category: res.Category,
		Rule:     res.Rule,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	code := 0
	if decision == decisionDeny {
		code = 1
	}
	return b, code
}
