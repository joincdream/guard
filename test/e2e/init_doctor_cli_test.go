package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guard/internal/hub/setup"
)

func withBinInPath(bin string) []string {
	return []string{
		"PATH=" + filepath.Dir(bin) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GUARD_LANG=en",
	}
}

// TC-E2E-001: guard help / --help에 guard init 및 guard doctor 포함
func TestInitDoctor_E2E_HelpListsInitAndDoctor(t *testing.T) {
	env := newIsolatedCLIEnv(t)
	for _, arg := range []string{"help", "--help"} {
		code, stdout, stderr := env.run("", []string{"GUARD_LANG=en"}, arg)
		if code != 0 {
			t.Errorf("guard %s exit = %d, want 0 (stderr=%s)", arg, code, stderr)
		}
		if !strings.Contains(stdout, "init") || !strings.Contains(stdout, "doctor") {
			t.Errorf("guard %s output missing init/doctor:\n%s", arg, stdout)
		}
	}
}

// TC-E2E-002: guard init --dry-run -> guard init -> 2회차 멱등성
func TestInitDoctor_E2E_InitDryRunAndIdempotency(t *testing.T) {
	env := newIsolatedCLIEnv(t)
	if err := os.WriteFile(filepath.Join(env.ws, "go.mod"), []byte("module e2e.test/app\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// 1. --dry-run
	code, _, stderr := env.run("", withBinInPath(env.bin), "init", "--dry-run")
	if code != 0 {
		t.Fatalf("guard init --dry-run exit = %d, want 0 (stderr=%s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(env.ws, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("guard init --dry-run created .agents unexpectedly")
	}

	// 2. 1회차 init
	code, _, stderr = env.run("", withBinInPath(env.bin), "init")
	if code != 0 {
		t.Fatalf("guard init (1st) exit = %d, want 0 (stderr=%s)", code, stderr)
	}
	targets := []string{
		".agents/hooks.json",
		"guard-policy.yaml",
		".agents/rules/guard-workflow.md",
		".agents/current_mode",
	}
	snap1 := make(map[string][]byte, len(targets))
	for _, rel := range targets {
		b, err := os.ReadFile(filepath.Join(env.ws, rel))
		if err != nil {
			t.Fatalf("expected %s after init: %v", rel, err)
		}
		snap1[rel] = b
	}

	// 3. 2회차 init (멱등성)
	code, _, stderr = env.run("", withBinInPath(env.bin), "init")
	if code != 0 {
		t.Fatalf("guard init (2nd) exit = %d, want 0 (stderr=%s)", code, stderr)
	}
	for _, rel := range targets {
		b, err := os.ReadFile(filepath.Join(env.ws, rel))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", rel, err)
		}
		if !bytes.Equal(snap1[rel], b) {
			t.Errorf("%s bytes changed on second guard init", rel)
		}
	}
}

// TC-E2E-003: guard init --force 및 --no-* CLI 옵션
func TestInitDoctor_E2E_InitForceAndSkipFlags(t *testing.T) {
	t.Run("force_backup", func(t *testing.T) {
		env := newIsolatedCLIEnv(t)
		policyPath := filepath.Join(env.ws, "guard-policy.yaml")
		const custom = "version: \"1\"\n# my custom\n"
		if err := os.WriteFile(policyPath, []byte(custom), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		code, _, stderr := env.run("", withBinInPath(env.bin), "init", "--force")
		if code != 0 {
			t.Fatalf("guard init --force exit = %d, want 0 (stderr=%s)", code, stderr)
		}
		bak, err := os.ReadFile(policyPath + ".bak")
		if err != nil {
			t.Fatalf("expected .bak file: %v", err)
		}
		if string(bak) != custom {
			t.Errorf(".bak content = %q, want %q", string(bak), custom)
		}
	})

	t.Run("no_hooks_and_no_rules", func(t *testing.T) {
		env := newIsolatedCLIEnv(t)
		code, _, stderr := env.run("", withBinInPath(env.bin), "init", "--no-hooks", "--no-rules")
		if code != 0 {
			t.Fatalf("guard init --no-hooks --no-rules exit = %d, want 0 (stderr=%s)", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(env.ws, ".agents", "hooks.json")); !os.IsNotExist(err) {
			t.Errorf("hooks.json created despite --no-hooks")
		}
		if _, err := os.Stat(filepath.Join(env.ws, ".agents", "rules", "guard-workflow.md")); !os.IsNotExist(err) {
			t.Errorf("guard-workflow.md created despite --no-rules")
		}
		if _, err := os.Stat(filepath.Join(env.ws, "guard-policy.yaml")); err != nil {
			t.Errorf("expected guard-policy.yaml to be created: %v", err)
		}
	})
}

// TC-E2E-004: guard init 잘못된 인자 Fail-Fast
func TestInitDoctor_E2E_InitInvalidArgsFailFast(t *testing.T) {
	cases := [][]string{
		{"init", "--unknown-flag"},
		{"init", "unexpected-arg"},
		{"init", "--preset", "rust"},
		{"init", "--mode", "BOGUS"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			env := newIsolatedCLIEnv(t)
			code, _, stderr := env.run("", withBinInPath(env.bin), args...)
			if code != 1 {
				t.Errorf("guard %v exit = %d, want 1", args, code)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("guard %v expected non-empty stderr", args)
			}
			entries, _ := os.ReadDir(env.ws)
			if len(entries) != 0 {
				t.Errorf("guard %v mutated workspace on error: %v", args, entries)
			}
		})
	}
}

// TC-E2E-005 & TC-E2E-006: guard doctor 오류 -> --fix -> 정상 및 --json 순수 JSON 계약
func TestInitDoctor_E2E_DoctorFixAndJSONOutput(t *testing.T) {
	env := newIsolatedCLIEnv(t)
	if err := os.WriteFile(filepath.Join(env.ws, "go.mod"), []byte("module e2e.test/doc\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	// 1. 미초기화 워크스페이스 -> Exit 1 + --json 순수 유효 JSON
	code, stdout, stderr := env.run("", withBinInPath(env.bin), "doctor", "--json")
	if code != 1 {
		t.Fatalf("guard doctor --json (uninitialized) exit = %d, want 1 (stdout=%s, stderr=%s)", code, stdout, stderr)
	}
	var rep1 setup.DoctorReport
	if err := json.Unmarshal([]byte(stdout), &rep1); err != nil {
		t.Fatalf("guard doctor --json stdout is not pure valid JSON: %v\nstdout:\n%s", err, stdout)
	}
	if len(rep1.Checks) != 6 || rep1.ErrorCount == 0 {
		t.Errorf("unexpected uninitialized DoctorReport: checks=%d errors=%d", len(rep1.Checks), rep1.ErrorCount)
	}

	// 2. --fix -> 복구 후 Exit 0
	code, stdout, stderr = env.run("", withBinInPath(env.bin), "doctor", "--fix", "--json")
	if code != 0 {
		t.Fatalf("guard doctor --fix --json exit = %d, want 0 (stdout=%s, stderr=%s)", code, stdout, stderr)
	}
	var rep2 setup.DoctorReport
	if err := json.Unmarshal([]byte(stdout), &rep2); err != nil {
		t.Fatalf("guard doctor --fix --json stdout invalid JSON: %v\nstdout:\n%s", err, stdout)
	}
	if rep2.ErrorCount != 0 || rep2.FixedCount == 0 {
		t.Errorf("expected ErrorCount==0 & FixedCount>0 after --fix, got err=%d fixed=%d", rep2.ErrorCount, rep2.FixedCount)
	}

	// 3. 재진단 -> Exit 0, 전체 OK
	code, stdout, stderr = env.run("", withBinInPath(env.bin), "doctor", "--json")
	if code != 0 {
		t.Fatalf("guard doctor --json (after fix) exit = %d, want 0 (stdout=%s, stderr=%s)", code, stdout, stderr)
	}
	var rep3 setup.DoctorReport
	if err := json.Unmarshal([]byte(stdout), &rep3); err != nil {
		t.Fatalf("guard doctor --json (after fix) invalid JSON: %v", err)
	}
	if rep3.ErrorCount != 0 || rep3.WarnCount != 0 {
		t.Errorf("expected 0 errors & 0 warns after fix, got %+v", rep3)
	}
}

// TC-E2E-007: guard doctor 잘못된 인자 Fail-Fast
func TestInitDoctor_E2E_DoctorInvalidArgsFailFast(t *testing.T) {
	cases := [][]string{
		{"doctor", "--unknown-flag"},
		{"doctor", "unexpected-arg"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			env := newIsolatedCLIEnv(t)
			code, _, stderr := env.run("", withBinInPath(env.bin), args...)
			if code != 1 {
				t.Errorf("guard %v exit = %d, want 1", args, code)
			}
			if strings.TrimSpace(stderr) == "" {
				t.Errorf("guard %v expected non-empty stderr", args)
			}
		})
	}
}

// TC-E2E-008: Symlink WORKSPACE_ROOT 정규화 (guard status 및 guard handoff)
func TestInitDoctor_E2E_SymlinkWorkspaceNormalization(t *testing.T) {
	env := newIsolatedCLIEnv(t)
	realWS := env.ws
	if resolved, err := filepath.EvalSymlinks(realWS); err == nil {
		realWS = resolved
	}
	linkWS := filepath.Join(t.TempDir(), "ws-link")
	if err := os.Symlink(realWS, linkWS); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// Symlink 경로로 WORKSPACE_ROOT를 전달해 guard status 실행 -> canonical path 출력 확인
	symlinkEnv := append(withBinInPath(env.bin), "WORKSPACE_ROOT="+linkWS)
	code, stdout, stderr := env.run("", symlinkEnv, "status")
	if code != 0 {
		t.Fatalf("guard status via symlink exit = %d, want 0 (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, realWS) {
		t.Errorf("guard status via symlink did not normalize workspace to canonical path %q:\n%s", realWS, stdout)
	}

	// Symlink 경로로 handoff send 후 실제 경로에서 inbox 조회 -> 동일 워크스페이스로 인식되어야 함
	code, _, stderr = env.run("", append(symlinkEnv, "AGENT_MODE=DEV"), "handoff", "send", "--to", "test", "-m", "symlink handoff check")
	if code != 0 {
		t.Fatalf("guard handoff send via symlink exit = %d, want 0 (stderr=%s)", code, stderr)
	}

	realEnv := append(withBinInPath(env.bin), "WORKSPACE_ROOT="+realWS, "AGENT_MODE=TEST")
	code, stdout, stderr = env.run("", realEnv, "handoff", "inbox", "--all", "--json")
	if code != 0 {
		t.Fatalf("guard handoff inbox via real path exit = %d, want 0 (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "symlink handoff check") {
		t.Errorf("handoff sent from symlink not visible from canonical workspace:\n%s", stdout)
	}
}
