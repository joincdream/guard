package core_test

import (
	"testing"

	"guard/internal/core"
)

func phase1Policy() *core.Policy {
	return &core.Policy{
		Version: "1.0",
		Rules: []core.Rule{
			{
				Name: "dev-deny-test-read",
				Mode: core.ModeDev,
				Type: "read",
				Patterns: []string{
					"*_test.go",
					"*.test.ts",
					"*.spec.ts",
					"**/tests/**",
					"**/testutil/**",
				},
				Message: "DEV: test file read denied",
			},
			{
				Name: "plan-deny-src-write",
				Mode: core.ModePlan,
				Type: "write",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
					"src/web/**",
				},
				Message: "PLAN: source write denied",
			},
			{
				Name: "test-deny-prod-read",
				Mode: core.ModeTest,
				Type: "read",
				Patterns: []string{
					"src/internal/**",
					"src/cmd/**",
					"internal/**",
				},
				ExcludePatterns: []string{
					"*_test.go",
					"**/testutil/**",
					"doc/**",
				},
				Message: "TEST: production read denied",
			},
		},
	}
}

func assertDecision(t *testing.T, got core.Decision, wantAllowed bool, wantRule, wantReason string) {
	t.Helper()
	if got.Allowed != wantAllowed {
		t.Fatalf("Allowed = %v, want %v (RuleName=%q, Reason=%q)", got.Allowed, wantAllowed, got.RuleName, got.Reason)
	}
	if wantAllowed {
		if got.RuleName != "" || got.Reason != "" {
			t.Errorf("expected empty RuleName and Reason on allow, got RuleName=%q Reason=%q", got.RuleName, got.Reason)
		}
		return
	}
	if got.RuleName != wantRule {
		t.Errorf("RuleName = %q, want %q", got.RuleName, wantRule)
	}
	if got.Reason != wantReason {
		t.Errorf("Reason = %q, want %q", got.Reason, wantReason)
	}
}

// TC-P1-001: 모드 정규화 기본 동작
func TestPhase1_NormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercase dev", in: "dev", want: core.ModeDev},
		{name: "padded test", in: " TEST ", want: core.ModeTest},
		{name: "tab and newline plan", in: "\tPlan\n", want: core.ModePlan},
		{name: "mixed all", in: " aLl ", want: core.ModeAll},
		{name: "lowercase qa", in: "qa", want: core.ModeQA},
		{name: "empty string", in: "", want: ""},
		{name: "whitespace only", in: "   \t\n ", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := core.NormalizeMode(tc.in); got != tc.want {
				t.Errorf("NormalizeMode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TC-P1-002: Bypass 모드 판정
func TestPhase1_IsBypassMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{mode: core.ModeAll, want: true},
		{mode: " all ", want: true},
		{mode: "\tAll\n", want: true},
		{mode: core.ModeQA, want: true},
		{mode: "qa", want: true},
		{mode: " Qa ", want: true},
		{mode: core.ModeDev, want: false},
		{mode: core.ModeTest, want: false},
		{mode: core.ModePlan, want: false},
		{mode: "", want: false},
		{mode: "   ", want: false},
		{mode: "ALL_OTHER", want: false},
		{mode: "QA2", want: false},
	}

	for _, tc := range tests {
		t.Run("mode_"+tc.mode, func(t *testing.T) {
			if got := core.IsBypassMode(tc.mode); got != tc.want {
				t.Errorf("IsBypassMode(%q) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

// TC-P1-003, TC-P1-004, TC-P1-005: nil/빈 정책, Bypass 모드, Evaluate 래퍼 동등성
func TestPhase1_BypassEmptyAndWrapperParity(t *testing.T) {
	pol := phase1Policy()

	t.Run("TC-P1-003_nil_policy", func(t *testing.T) {
		got := core.EvaluateRequest(nil, core.EvalRequest{
			Mode:        core.ModeDev,
			ToolName:    "read_file",
			TargetPath:  "pkg/a_test.go",
			CommandLine: "cat pkg/a_test.go",
		})
		assertDecision(t, got, true, "", "")
	})

	t.Run("TC-P1-003_empty_rules_nil_slice", func(t *testing.T) {
		got := core.EvaluateRequest(&core.Policy{Version: "1.0", Rules: nil}, core.EvalRequest{
			Mode:       core.ModeDev,
			ToolName:   "read_file",
			TargetPath: "pkg/a_test.go",
		})
		assertDecision(t, got, true, "", "")
	})

	t.Run("TC-P1-003_empty_rules_zero_len", func(t *testing.T) {
		got := core.EvaluateRequest(&core.Policy{Version: "1.0", Rules: []core.Rule{}}, core.EvalRequest{
			Mode:        core.ModePlan,
			ToolName:    "write_file",
			TargetPath:  "src/internal/main.go",
			CommandLine: "echo x > src/internal/main.go",
		})
		assertDecision(t, got, true, "", "")
	})

	t.Run("TC-P1-004_bypass_modes_override_all_rules", func(t *testing.T) {
		bypassModes := []string{" all ", "qa", core.ModeAll, core.ModeQA}
		reqs := []core.EvalRequest{
			{ToolName: "read_file", TargetPath: "pkg/a_test.go"},
			{ToolName: "write_file", TargetPath: "src/internal/a.go"},
			{ToolName: "edit_file", TargetPath: "src/cmd/main.go"},
			{ToolName: "run_command", CommandLine: "cat pkg/a_test.go && echo x > src/internal/a.go"},
		}
		for _, mode := range bypassModes {
			for _, r := range reqs {
				r.Mode = mode
				got := core.EvaluateRequest(pol, r)
				assertDecision(t, got, true, "", "")
			}
		}
	})

	t.Run("TC-P1-005_evaluate_wrapper_parity", func(t *testing.T) {
		reqs := []core.EvalRequest{
			{Mode: core.ModeDev, ToolName: "read_file", TargetPath: "pkg/a_test.go"},
			{Mode: core.ModeDev, ToolName: "read_file", TargetPath: "pkg/a.go"},
			{Mode: core.ModePlan, ToolName: "edit_file", TargetPath: "src/cmd/app.go"},
			{Mode: core.ModeDev, ToolName: "run_command", CommandLine: "git status && cat pkg/a_test.go"},
			{Mode: core.ModeTest, ToolName: "run_command", CommandLine: "cat src/internal/service.go"},
		}
		for _, r := range reqs {
			gotReq := core.EvaluateRequest(pol, r)
			gotWrap := core.Evaluate(r.Mode, pol, r.ToolName, r.TargetPath, r.CommandLine)
			if gotReq != gotWrap {
				t.Errorf("parity mismatch for %+v: EvaluateRequest=%+v vs Evaluate=%+v", r, gotReq, gotWrap)
			}
		}
	})
}

// TC-P1-006, TC-P1-007, TC-P1-008, TC-P1-012: MatchPattern 정교성 및 경계
func TestPhase1_MatchPattern_BoundariesAndEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// TC-P1-006: 접미사/확장자 매칭
		{name: "suffix_go_test_match", path: "pkg/a_test.go", pattern: "*_test.go", want: true},
		{name: "suffix_go_prod_mismatch", path: "pkg/a.go", pattern: "*_test.go", want: false},
		{name: "standard_glob_config_match", path: "config/app.json", pattern: "config/*.json", want: true},
		{name: "standard_glob_config_mismatch", path: "config/app.yaml", pattern: "config/*.json", want: false},

		// TC-P1-007: 디렉토리 경계 오탐 방지
		{name: "boundary_dir_prefix_no_false_positive", path: "src/internal_extra/a.go", pattern: "src/internal/**", want: false},
		{name: "boundary_double_star_prefix_no_false_positive", path: "mytests/a.go", pattern: "**/tests/**", want: false},
		{name: "boundary_double_star_suffix_no_false_positive", path: "src/tests_backup/a.go", pattern: "**/tests/**", want: false},

		// TC-P1-008: 다중 디렉토리 ** 루트/중첩/절대/상대(./) 경로
		{name: "double_star_root_level", path: "tests/a.go", pattern: "**/tests/**", want: true},
		{name: "double_star_nested", path: "a/b/tests/c.go", pattern: "**/tests/**", want: true},
		{name: "double_star_absolute", path: "/abs/path/tests/c.go", pattern: "**/tests/**", want: true},
		{name: "double_star_dot_slash", path: "./tests/a.go", pattern: "**/tests/**", want: true},
		{name: "dir_prefix_dot_slash", path: "./src/internal/a.go", pattern: "src/internal/**", want: true},

		// TC-P1-012: 빈 경로/빈 패턴 패닉 방어
		{name: "empty_path", path: "", pattern: "*.go", want: false},
		{name: "empty_pattern", path: "a.go", pattern: "", want: false},
		{name: "both_empty", path: "", pattern: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := core.MatchPattern(tc.path, tc.pattern); got != tc.want {
				t.Errorf("MatchPattern(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
			}
		})
	}
}

// TC-P1-009, TC-P1-010, TC-P1-011: ExcludePatterns, 규칙 선언 순서, 글로벌 규칙
func TestPhase1_RulePrecedenceExcludeAndGlobal(t *testing.T) {
	t.Run("TC-P1-009_exclude_patterns_exemption", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{
					Name:            "test-block-all-except-allowed",
					Mode:            core.ModeTest,
					Type:            "read",
					Patterns:        []string{"**/*"},
					ExcludePatterns: []string{"*_test.go", "doc/**"},
					Message:         "TEST: only doc and test files allowed",
				},
			},
		}

		assertDecision(t, core.EvaluateRequest(pol, core.EvalRequest{
			Mode: core.ModeTest, ToolName: "read_file", TargetPath: "doc/spec.md",
		}), true, "", "")

		assertDecision(t, core.EvaluateRequest(pol, core.EvalRequest{
			Mode: core.ModeTest, ToolName: "read_file", TargetPath: "internal/core/a_test.go",
		}), true, "", "")

		assertDecision(t, core.EvaluateRequest(pol, core.EvalRequest{
			Mode: core.ModeTest, ToolName: "read_file", TargetPath: "internal/a.go",
		}), false, "test-block-all-except-allowed", "TEST: only doc and test files allowed")
	})

	t.Run("TC-P1-010_first_matching_rule_wins", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{
					Name:     "rule-first",
					Mode:     core.ModeDev,
					Type:     "read",
					Patterns: []string{"*_test.go"},
					Message:  "first rule message",
				},
				{
					Name:     "rule-second",
					Mode:     core.ModeDev,
					Type:     "read",
					Patterns: []string{"pkg/**"},
					Message:  "second rule message",
				},
			},
		}

		got := core.EvaluateRequest(pol, core.EvalRequest{
			Mode:       core.ModeDev,
			ToolName:   "read_file",
			TargetPath: "pkg/service_test.go",
		})
		assertDecision(t, got, false, "rule-first", "first rule message")
	})

	t.Run("TC-P1-011_global_rule_empty_mode", func(t *testing.T) {
		pol := &core.Policy{
			Version: "1.0",
			Rules: []core.Rule{
				{
					Name:     "global-secret-read",
					Mode:     "",
					Type:     "read",
					Patterns: []string{"*.secret"},
					Message:  "secret read forbidden in all non-bypass modes",
				},
			},
		}

		for _, m := range []string{core.ModeDev, core.ModeTest, core.ModePlan} {
			got := core.EvaluateRequest(pol, core.EvalRequest{
				Mode: m, ToolName: "read_file", TargetPath: "keys/prod.secret",
			})
			assertDecision(t, got, false, "global-secret-read", "secret read forbidden in all non-bypass modes")
		}
		for _, m := range []string{core.ModeAll, core.ModeQA} {
			got := core.EvaluateRequest(pol, core.EvalRequest{
				Mode: m, ToolName: "read_file", TargetPath: "keys/prod.secret",
			})
			assertDecision(t, got, true, "", "")
		}
	})
}

// TC-P1-013, TC-P1-014: 파일 도구-타입 매핑 정확성 및 미지원 도구
func TestPhase1_FileToolsMappingAndUnknownTool(t *testing.T) {
	pol := phase1Policy()

	tests := []struct {
		name        string
		req         core.EvalRequest
		wantAllowed bool
		wantRule    string
		wantReason  string
	}{
		{
			name:        "read_rule_blocks_read_file",
			req:         core.EvalRequest{Mode: core.ModeDev, ToolName: "read_file", TargetPath: "pkg/a_test.go"},
			wantAllowed: false,
			wantRule:    "dev-deny-test-read",
			wantReason:  "DEV: test file read denied",
		},
		{
			name:        "read_rule_does_not_block_write_file",
			req:         core.EvalRequest{Mode: core.ModeDev, ToolName: "write_file", TargetPath: "pkg/a_test.go"},
			wantAllowed: true,
		},
		{
			name:        "read_rule_does_not_block_edit_file",
			req:         core.EvalRequest{Mode: core.ModeDev, ToolName: "edit_file", TargetPath: "pkg/a_test.go"},
			wantAllowed: true,
		},
		{
			name:        "write_rule_blocks_write_file",
			req:         core.EvalRequest{Mode: core.ModePlan, ToolName: "write_file", TargetPath: "src/internal/a.go"},
			wantAllowed: false,
			wantRule:    "plan-deny-src-write",
			wantReason:  "PLAN: source write denied",
		},
		{
			name:        "write_rule_blocks_edit_file",
			req:         core.EvalRequest{Mode: core.ModePlan, ToolName: "edit_file", TargetPath: "src/cmd/main.go"},
			wantAllowed: false,
			wantRule:    "plan-deny-src-write",
			wantReason:  "PLAN: source write denied",
		},
		{
			name:        "write_rule_does_not_block_read_file",
			req:         core.EvalRequest{Mode: core.ModePlan, ToolName: "read_file", TargetPath: "src/internal/a.go"},
			wantAllowed: true,
		},
		{
			name:        "TC-P1-014_unknown_tool_allowed",
			req:         core.EvalRequest{Mode: core.ModeDev, ToolName: "unknown_tool", TargetPath: "pkg/a_test.go"},
			wantAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := core.EvaluateRequest(pol, tc.req)
			assertDecision(t, got, tc.wantAllowed, tc.wantRule, tc.wantReason)
		})
	}
}

// TC-P1-015 ~ TC-P1-025: 셸 명령어 파싱 & 보안 우회 공격 (Adversarial)
func TestPhase1_ShellCommandAdversarialAndSafeCases(t *testing.T) {
	pol := phase1Policy()

	const (
		devRule    = "dev-deny-test-read"
		devMsg     = "DEV: test file read denied"
		planRule   = "plan-deny-src-write"
		planMsg    = "PLAN: source write denied"
		testRule   = "test-deny-prod-read"
		testMsg    = "TEST: production read denied"
	)

	tests := []struct {
		name        string
		mode        string
		cmd         string
		wantAllowed bool
		wantRule    string
		wantReason  string
	}{
		// TC-P1-015: 화이트리스트 앞세우기 + 제어 연산자 우회 (&&, ;, ||, |, \n)
		{name: "P1-015_and_bypass", mode: core.ModeDev, cmd: "git status && cat pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-015_semicolon_bypass", mode: core.ModeDev, cmd: "go test ./... ; head pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-015_or_bypass", mode: core.ModeDev, cmd: "go vet ./... || tail pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-015_pipe_bypass", mode: core.ModeDev, cmd: "git branch | cat pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-015_newline_bypass", mode: core.ModeDev, cmd: "git status\ncat pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},

		// TC-P1-016: 화이트리스트 명령 + 출력 리다이렉션 변형 (>, >>, 1>, 2>, &>)
		{name: "P1-016_whitelist_redirect_gt", mode: core.ModePlan, cmd: "git status > src/internal/a.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-016_whitelist_redirect_append", mode: core.ModePlan, cmd: "go build ./... >> src/cmd/a.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-016_whitelist_redirect_fd1", mode: core.ModePlan, cmd: "git status 1> src/internal/a.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-016_whitelist_redirect_fd2", mode: core.ModePlan, cmd: "go vet ./... 2> src/internal/b.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-016_whitelist_redirect_ampgt", mode: core.ModePlan, cmd: "git branch &> src/internal/c.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},

		// TC-P1-017: 안전한 화이트리스트 명령 허용
		{name: "P1-017_safe_git_status", mode: core.ModeDev, cmd: "git status", wantAllowed: true},
		{name: "P1-017_safe_git_branch", mode: core.ModeDev, cmd: "git branch -a", wantAllowed: true},
		{name: "P1-017_safe_git_commit", mode: core.ModePlan, cmd: `git commit -m "docs: update spec"`, wantAllowed: true},
		{name: "P1-017_safe_git_log_oneline", mode: core.ModeDev, cmd: "git log --oneline -n 5", wantAllowed: true},
		{name: "P1-017_safe_go_build", mode: core.ModePlan, cmd: "go build ./...", wantAllowed: true},
		{name: "P1-017_safe_go_vet", mode: core.ModePlan, cmd: "go vet ./...", wantAllowed: true},

		// TC-P1-018: Git 내용 조회 우회
		{name: "P1-018_git_show_rev_path", mode: core.ModeDev, cmd: "git show HEAD:pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-018_git_diff", mode: core.ModeDev, cmd: "git diff pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-018_git_checkout", mode: core.ModeDev, cmd: "git checkout -- pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-018_git_log_p", mode: core.ModeDev, cmd: "git log -p pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-018_git_log_patch", mode: core.ModeDev, cmd: "git log --patch pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-018_git_log_u", mode: core.ModeDev, cmd: "git log -u pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},

		// TC-P1-019: 파일 뷰어 전종 + 플래그/와일드카드
		{name: "P1-019_cat", mode: core.ModeDev, cmd: "cat pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_head_flag", mode: core.ModeDev, cmd: "head -n 20 pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_tail_wildcard", mode: core.ModeDev, cmd: "tail -f pkg/*_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_more", mode: core.ModeDev, cmd: "more pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_less", mode: core.ModeDev, cmd: "less pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_strings", mode: core.ModeDev, cmd: "strings src/testutil/fixture.bin", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_nl", mode: core.ModeDev, cmd: "nl -ba pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_tac", mode: core.ModeDev, cmd: "tac pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-019_bat", mode: core.ModeDev, cmd: "bat --plain pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},

		// TC-P1-020: 입력 리다이렉션 변형 (공백 유무)
		{name: "P1-020_stdin_redirect_spaced", mode: core.ModeDev, cmd: "wc -l < pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-020_stdin_redirect_compact", mode: core.ModeDev, cmd: "tr a A <pkg/a_test.go", wantAllowed: false, wantRule: devRule, wantReason: devMsg},

		// TC-P1-021: 쓰기 도구 (tee, tee -a, sed -i, sed --in-place)
		{name: "P1-021_tee", mode: core.ModePlan, cmd: "echo x | tee src/internal/a.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-021_tee_append", mode: core.ModePlan, cmd: "echo x | tee -a src/internal/a.go", wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-021_sed_i", mode: core.ModePlan, cmd: `sed -i 's/a/b/' src/internal/a.go`, wantAllowed: false, wantRule: planRule, wantReason: planMsg},
		{name: "P1-021_sed_in_place", mode: core.ModePlan, cmd: `sed --in-place 's/a/b/' src/internal/a.go`, wantAllowed: false, wantRule: planRule, wantReason: planMsg},

		// TC-P1-022: 인라인 스크립트 읽기·쓰기 우회
		{name: "P1-022_python_c_read", mode: core.ModeDev, cmd: `python -c "open('pkg/a_test.go').read()"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_python3_c_read", mode: core.ModeDev, cmd: `python3 -c "print(open('pkg/a_test.go').read())"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_node_e_read", mode: core.ModeDev, cmd: `node -e "fs.readFileSync('web/app.spec.ts')"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_node_eval_read", mode: core.ModeDev, cmd: `node --eval "fs.readFileSync('web/app.test.ts')"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_ruby_e_read", mode: core.ModeDev, cmd: `ruby -e "puts File.read('pkg/a_test.go')"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_perl_e_read", mode: core.ModeDev, cmd: `perl -e "open F, 'pkg/a_test.go'; print <F>"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_bash_c_read", mode: core.ModeDev, cmd: `bash -c "cat pkg/a_test.go"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_sh_c_read", mode: core.ModeDev, cmd: `sh -c "cat pkg/a_test.go"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_zsh_c_read", mode: core.ModeDev, cmd: `zsh -c "cat pkg/a_test.go"`, wantAllowed: false, wantRule: devRule, wantReason: devMsg},
		{name: "P1-022_python3_c_write_in_plan", mode: core.ModePlan, cmd: `python3 -c "open('src/internal/a.go', 'w').write('x')"`, wantAllowed: false, wantRule: planRule, wantReason: planMsg},

		// TC-P1-023: 인라인 스크립트 무해한 실행 허용
		{name: "P1-023_python3_benign", mode: core.ModeDev, cmd: `python3 -c "print(1 + 1)"`, wantAllowed: true},
		{name: "P1-023_node_benign", mode: core.ModeDev, cmd: `node -e "console.log('ok')"`, wantAllowed: true},

		// TC-P1-024: 따옴표 내부 연산자 보존
		{name: "P1-024_quoted_operators_in_commit_msg", mode: core.ModeDev, cmd: `git commit -m "fix a && b ; c | d"`, wantAllowed: true},

		// TC-P1-025: TEST 모드에서 ExcludePatterns 셸 허용 vs 프로덕션 차단
		{name: "P1-025_test_mode_cat_test_file_allowed", mode: core.ModeTest, cmd: "cat internal/core/a_test.go", wantAllowed: true},
		{name: "P1-025_test_mode_cat_doc_allowed", mode: core.ModeTest, cmd: "cat doc/spec/01_core_policy_contract.md", wantAllowed: true},
		{name: "P1-025_test_mode_cat_prod_denied", mode: core.ModeTest, cmd: "cat internal/core/eval.go", wantAllowed: false, wantRule: testRule, wantReason: testMsg},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := core.EvaluateRequest(pol, core.EvalRequest{
				Mode:        tc.mode,
				ToolName:    "run_command",
				CommandLine: tc.cmd,
			})
			assertDecision(t, got, tc.wantAllowed, tc.wantRule, tc.wantReason)
		})
	}
}
