package policy

import (
	"guard/internal/core"
	"guard/internal/i18n"
)

// Policy is an alias for core.Policy to share a single domain model without conversion overhead.
type Policy = core.Policy

// Rule is an alias for core.Rule.
type Rule = core.Rule

// DefaultPolicy returns the built-in fallback guardrail policy used when no YAML file is found or readable.
func DefaultPolicy() *Policy {
	return &Policy{
		Version: "1.0",
		Rules: []Rule{
			{
				Name: "guard-test-files-read",
				Mode: "DEV",
				Type: "read",
				Patterns: []string{
					"*_test.go",
					"*.test.ts",
					"*.test.js",
					"*.spec.ts",
					"*.spec.js",
					"**/testutil/**",
					"**/tests/**",
					"**/000/**",
				},
				Message: i18n.T("policy.builtin.dev_read"),
			},
			{
				Name: "guard-source-code-write",
				Mode: "PLAN",
				Type: "write",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
					"src/web/**",
					"src/docker/**",
				},
				Message: i18n.T("policy.builtin.plan_write"),
			},
			{
				Name: "guard-production-code-read",
				Mode: "TEST",
				Type: "read",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
				},
				ExcludePatterns: []string{
					"*_test.go",
					"*.test.ts",
					"*.test.js",
					"*.spec.ts",
					"*.spec.js",
					"**/testutil/**",
					"**/contract/**",
				},
				Message: i18n.T("policy.builtin.test_read"),
			},
		},
	}
}
