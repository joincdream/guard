package core

import (
	"strings"
)

const (
	ModeDev  = "DEV"
	ModeTest = "TEST"
	ModePlan = "PLAN"
	ModeAll  = "ALL"
	ModeQA   = "QA"
)

// NormalizeMode trims whitespace and converts a guardrail mode string to uppercase.
func NormalizeMode(mode string) string {
	return strings.ToUpper(strings.TrimSpace(mode))
}

// IsBypassMode reports whether the given mode disables all guardrail checks.
func IsBypassMode(mode string) bool {
	m := NormalizeMode(mode)
	return m == ModeAll || m == ModeQA
}

// Rule defines a single guardrail policy rule.
type Rule struct {
	Name            string   `yaml:"name"`
	Mode            string   `yaml:"mode"` // DEV, PLAN, TEST, or empty for all modes
	Type            string   `yaml:"type"` // read, write
	Patterns        []string `yaml:"patterns"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	Message         string   `yaml:"message"`
}

// Policy defines the root policy ruleset.
type Policy struct {
	Version string `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

// EvalRequest encapsulates all inputs required to evaluate a single tool invocation.
type EvalRequest struct {
	Mode        string
	ToolName    string
	TargetPath  string
	CommandLine string
}

// Decision represents the outcome of evaluating a tool invocation against a Policy.
type Decision struct {
	Allowed  bool
	RuleName string
	Reason   string
}

// Evaluate checks a tool invocation against the given mode and policy.
// It is a compatibility wrapper around EvaluateRequest.
func Evaluate(mode string, policy *Policy, toolName string, targetPath string, cmdLine string) Decision {
	return EvaluateRequest(policy, EvalRequest{
		Mode:        mode,
		ToolName:    toolName,
		TargetPath:  targetPath,
		CommandLine: cmdLine,
	})
}

// EvaluateRequest checks a tool invocation request against the given policy.
// It is a pure domain function with no filesystem or UI dependencies.
func EvaluateRequest(policy *Policy, req EvalRequest) Decision {
	// Trust boundary check: block agent self-approval commands regardless of mode or policy.
	if strings.TrimSpace(req.CommandLine) != "" {
		if ruleName, reason := evaluateProtectedGuardCommand(req.CommandLine); reason != "" {
			return Decision{
				Allowed:  false,
				RuleName: ruleName,
				Reason:   reason,
			}
		}
	}

	normalizedMode := NormalizeMode(req.Mode)

	// Mode ALL or QA bypasses all checks.
	if IsBypassMode(normalizedMode) {
		return Decision{Allowed: true}
	}

	if policy == nil || len(policy.Rules) == 0 {
		return Decision{Allowed: true}
	}

	// Check 1: Tool-level file path inspection.
	switch req.ToolName {
	case "read_file":
		if ruleName, reason := evaluatePathRules(policy, normalizedMode, "read", req.TargetPath); reason != "" {
			return Decision{
				Allowed:  false,
				RuleName: ruleName,
				Reason:   reason,
			}
		}
	case "write_file", "edit_file":
		if ruleName, reason := evaluatePathRules(policy, normalizedMode, "write", req.TargetPath); reason != "" {
			return Decision{
				Allowed:  false,
				RuleName: ruleName,
				Reason:   reason,
			}
		}
	}

	// Check 2: Shell command-line inspection.
	if strings.TrimSpace(req.CommandLine) != "" {
		if ruleName, reason := evaluateCommandRules(policy, normalizedMode, req.CommandLine); reason != "" {
			return Decision{
				Allowed:  false,
				RuleName: ruleName,
				Reason:   reason,
			}
		}
	}

	return Decision{Allowed: true}
}
