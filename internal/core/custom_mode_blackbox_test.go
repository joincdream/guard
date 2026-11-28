package core_test

import (
	"reflect"
	"testing"

	"guard/internal/core"
)

func TestContract01_ExtractModes(t *testing.T) {
	t.Run("nil policy returns ALL only", func(t *testing.T) {
		got := core.ExtractModes(nil)
		want := []string{core.ModeAll}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractModes(nil) = %v, want %v", got, want)
		}
	})

	t.Run("empty rules returns ALL only", func(t *testing.T) {
		got := core.ExtractModes(&core.Policy{Version: "1.0"})
		want := []string{core.ModeAll}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractModes(empty) = %v, want %v", got, want)
		}
	})

	t.Run("normalizes deduplicates skips global and appends ALL once", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{Name: "global", Mode: ""},
				{Name: "global-ws", Mode: "   "},
				{Name: "r1", Mode: " dev "},
				{Name: "r2", Mode: "review"},
				{Name: "r3", Mode: "DEV"},
				{Name: "r4", Mode: " Security\t"},
				{Name: "r5", Mode: "REVIEW"},
			},
		}
		got := core.ExtractModes(pol)
		want := []string{"DEV", "REVIEW", "SECURITY", "ALL"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractModes = %v, want %v", got, want)
		}
	})

	t.Run("does not duplicate ALL if already in rules", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{Name: "r1", Mode: "all"},
				{Name: "r2", Mode: "review"},
				{Name: "r3", Mode: "ALL"},
			},
		}
		got := core.ExtractModes(pol)
		want := []string{"ALL", "REVIEW"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractModes = %v, want %v", got, want)
		}
	})
}

func TestContract01_HasMode(t *testing.T) {
	pol := &core.Policy{
		Version: "1.0",
		Rules: []core.Rule{
			{Name: "global", Mode: ""},
			{Name: "r1", Mode: "dev"},
			{Name: "r2", Mode: "REVIEW"},
		},
	}

	tests := []struct {
		name   string
		policy *core.Policy
		mode   string
		want   bool
	}{
		{name: "empty mode", policy: pol, mode: "", want: false},
		{name: "whitespace mode", policy: pol, mode: "  \t ", want: false},
		{name: "nil policy bypass ALL", policy: nil, mode: "all", want: true},
		{name: "nil policy bypass QA", policy: nil, mode: " qa ", want: true},
		{name: "nil policy non-bypass", policy: nil, mode: "review", want: false},
		{name: "defined builtin mode", policy: pol, mode: " DEV ", want: true},
		{name: "defined custom mode", policy: pol, mode: "review", want: true},
		{name: "undefined mode", policy: pol, mode: "security", want: false},
		{name: "bypass ALL with policy", policy: pol, mode: "ALL", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := core.HasMode(tc.policy, tc.mode); got != tc.want {
				t.Errorf("HasMode(%v, %q) = %v, want %v", tc.policy != nil, tc.mode, got, tc.want)
			}
		})
	}
}

func TestContract01_CustomModeEvaluateRequest(t *testing.T) {
	pol := &core.Policy{
		Version: "1.0",
		Rules: []core.Rule{
			{
				Name:     "review-deny-write",
				Mode:     "REVIEW",
				Type:     "write",
				Patterns: []string{"internal/**"},
				Message:  "REVIEW: write denied",
			},
			{
				Name:     "dev-deny-test-read",
				Mode:     "DEV",
				Type:     "read",
				Patterns: []string{"*_test.go"},
				Message:  "DEV: test read denied",
			},
		},
	}

	// REVIEW mode blocks internal/** write
	dec := core.EvaluateRequest(pol, core.EvalRequest{
		Mode:       "review",
		ToolName:   "write_file",
		TargetPath: "internal/core/evaluator.go",
	})
	assertDecision(t, dec, false, "review-deny-write", "REVIEW: write denied")

	// REVIEW mode allows reading *_test.go (DEV rule does not apply)
	dec = core.EvaluateRequest(pol, core.EvalRequest{
		Mode:       "REVIEW",
		ToolName:   "read_file",
		TargetPath: "internal/core/evaluator_test.go",
	})
	assertDecision(t, dec, true, "", "")

	// REVIEW mode allows reading internal/**
	dec = core.EvaluateRequest(pol, core.EvalRequest{
		Mode:       "REVIEW",
		ToolName:   "read_file",
		TargetPath: "internal/core/evaluator.go",
	})
	assertDecision(t, dec, true, "", "")
}

func TestContract01_EdgeCases_CoreModes(t *testing.T) {
	t.Run("ExtractModes with only global rules returns ALL", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{Name: "g1", Mode: ""},
				{Name: "g2", Mode: "   \t "},
			},
		}
		got := core.ExtractModes(pol)
		want := []string{core.ModeAll}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExtractModes = %v, want %v", got, want)
		}
	})

	t.Run("HasMode on non-nil empty policy returns false for non-bypass", func(t *testing.T) {
		pol := &core.Policy{Version: "1.0", Rules: []core.Rule{}}
		if core.HasMode(pol, "DEV") {
			t.Errorf("HasMode(empty, DEV) = true, want false")
		}
		if core.HasMode(pol, "REVIEW") {
			t.Errorf("HasMode(empty, REVIEW) = true, want false")
		}
		if !core.HasMode(pol, "ALL") || !core.HasMode(pol, "QA") {
			t.Errorf("HasMode(empty, ALL/QA) expected true")
		}
	})

	t.Run("Custom mode with ExcludePatterns, command rule, and global rule", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{
					Name:     "global-deny-env",
					Mode:     "",
					Type:     "read",
					Patterns: []string{".env*"},
					Message:  "GLOBAL: env read denied",
				},
				{
					Name:            "review-deny-write-except-doc",
					Mode:            "REVIEW",
					Type:            "write",
					Patterns:        []string{"**/*"},
					ExcludePatterns: []string{"doc/**"},
					Message:         "REVIEW: write denied outside doc",
				},
				{
					Name:     "review-deny-rm",
					Mode:     "REVIEW",
					Type:     "command",
					Patterns: []string{"rm *"},
					Message:  "REVIEW: rm command denied",
				},
			},
		}

		// Global rule applies in custom mode with whitespace/mixed case
		dec := core.EvaluateRequest(pol, core.EvalRequest{
			Mode:       "  rEvIeW ",
			ToolName:   "read_file",
			TargetPath: ".env.local",
		})
		assertDecision(t, dec, false, "global-deny-env", "GLOBAL: env read denied")

		// ExcludePatterns allows doc/notes.md write in REVIEW
		dec = core.EvaluateRequest(pol, core.EvalRequest{
			Mode:       "REVIEW",
			ToolName:   "write_file",
			TargetPath: "doc/notes.md",
		})
		assertDecision(t, dec, true, "", "")

		// Command rule blocks rm in REVIEW
		dec = core.EvaluateRequest(pol, core.EvalRequest{
			Mode:        "review",
			ToolName:    "run_command",
			CommandLine: "rm -rf tmp/",
		})
		assertDecision(t, dec, false, "review-deny-rm", "REVIEW: rm command denied")
	})
}
