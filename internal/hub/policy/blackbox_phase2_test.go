package policy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guard/internal/core"
	"guard/internal/hub/policy"
)

func isolatePolicyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("AGENT_MODE", "")
	t.Setenv("AGENT_ROLE", "")
	t.Setenv(policy.EnvPolicyPath, "")
	t.Setenv("GUARD_DEBUG", "")
}

func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func validYAML(ruleName string) string {
	return "version: \"1.0\"\nrules:\n  - name: " + ruleName + "\n    mode: DEV\n    type: read\n    patterns:\n      - \"*_test.go\"\n    message: \"blocked by " + ruleName + "\"\n"
}

// TC-P2-001: 5단계 우선순위 정합성 (EnvPath -> CWD -> WorkspaceRoot/.agents -> HomeDir/.config/guard -> builtin)
func TestPhase2_Policy_PriorityOrder(t *testing.T) {
	isolatePolicyEnv(t)

	root := t.TempDir()
	envPath := filepath.Join(root, "env", "custom-policy.yaml")
	cwdDir := filepath.Join(root, "cwd")
	cwdPath := filepath.Join(cwdDir, policy.PolicyFileName)
	wsDir := filepath.Join(root, "ws")
	wsPath := filepath.Join(wsDir, ".agents", policy.PolicyFileName)
	homeDir := filepath.Join(root, "home")
	homePath := filepath.Join(homeDir, ".config", "guard", policy.PolicyFileName)

	writeYAML(t, envPath, validYAML("rule-env"))
	writeYAML(t, cwdPath, validYAML("rule-cwd"))
	writeYAML(t, wsPath, validYAML("rule-ws"))
	writeYAML(t, homePath, validYAML("rule-home"))

	opts := policy.LoadOptions{
		EnvPath:       envPath,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	}

	steps := []struct {
		name       string
		removeFile string
		wantSource string
		wantRule   string
	}{
		{name: "1_env_path", wantSource: envPath, wantRule: "rule-env"},
		{name: "2_cwd_policy", removeFile: envPath, wantSource: cwdPath, wantRule: "rule-cwd"},
		{name: "3_workspace_agents_policy", removeFile: cwdPath, wantSource: wsPath, wantRule: "rule-ws"},
		{name: "4_home_config_policy", removeFile: wsPath, wantSource: homePath, wantRule: "rule-home"},
		{name: "5_builtin_fallback", removeFile: homePath, wantSource: policy.BuiltinSource, wantRule: ""},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			if st.removeFile != "" {
				if err := os.Remove(st.removeFile); err != nil {
					t.Fatalf("Remove(%s): %v", st.removeFile, err)
				}
			}
			res := policy.LoadDetailedWithOptions(opts)
			if res.Policy == nil {
				t.Fatalf("expected non-nil Policy")
			}
			if res.Source != st.wantSource {
				t.Errorf("Source = %q, want %q", res.Source, st.wantSource)
			}
			if len(res.Warnings) != 0 {
				t.Errorf("expected 0 warnings, got %v", res.Warnings)
			}
			if st.wantRule != "" {
				if len(res.Policy.Rules) != 1 || res.Policy.Rules[0].Name != st.wantRule {
					t.Errorf("Rules = %+v, want single rule %q", res.Policy.Rules, st.wantRule)
				}
			} else if len(res.Policy.Rules) == 0 {
				t.Errorf("expected builtin fallback policy to contain rules")
			}

			// Verify LoadWithOptions wrapper parity
			polWrap, srcWrap := policy.LoadWithOptions(opts)
			if srcWrap != res.Source || polWrap == nil || len(polWrap.Rules) != len(res.Policy.Rules) {
				t.Errorf("LoadWithOptions wrapper mismatch: got (%v, %q), want source %q", polWrap, srcWrap, res.Source)
			}
		})
	}
}

// TC-P2-002: 상위 YAML 문법 오류 -> 하위 폴백 + Warnings에 오류 파일 경로 포함
func TestPhase2_Policy_CorruptedHigherPriorityFallsBackWithWarning(t *testing.T) {
	isolatePolicyEnv(t)

	root := t.TempDir()
	envPath := filepath.Join(root, "broken.yaml")
	cwdDir := filepath.Join(root, "cwd")
	cwdPath := filepath.Join(cwdDir, policy.PolicyFileName)

	writeYAML(t, envPath, "version: [\n  unclosed: yaml: :::")
	writeYAML(t, cwdPath, validYAML("rule-cwd-valid"))

	res := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       envPath,
		CWD:           cwdDir,
		WorkspaceRoot: filepath.Join(root, "ws"),
		HomeDir:       filepath.Join(root, "home"),
	})

	if res.Source != cwdPath {
		t.Fatalf("Source = %q, want %q", res.Source, cwdPath)
	}
	if res.Policy == nil || len(res.Policy.Rules) != 1 || res.Policy.Rules[0].Name != "rule-cwd-valid" {
		t.Fatalf("unexpected Policy: %+v", res.Policy)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(res.Warnings), res.Warnings)
	}
	if !strings.Contains(res.Warnings[0], envPath) {
		t.Errorf("expected warning to reference broken path %q, got %q", envPath, res.Warnings[0])
	}
}

// TC-P2-003: 상위 YAML이 유효하나 rules: [] (0건) -> 경고 후 하위 폴백
func TestPhase2_Policy_EmptyRulesFallsBackWithWarning(t *testing.T) {
	isolatePolicyEnv(t)

	root := t.TempDir()
	envPath := filepath.Join(root, "empty-rules.yaml")
	cwdDir := filepath.Join(root, "cwd")
	cwdPath := filepath.Join(cwdDir, policy.PolicyFileName)

	writeYAML(t, envPath, "version: \"1.0\"\nrules: []\n")
	writeYAML(t, cwdPath, validYAML("rule-fallback"))

	res := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       envPath,
		CWD:           cwdDir,
		WorkspaceRoot: filepath.Join(root, "ws"),
		HomeDir:       filepath.Join(root, "home"),
	})

	if res.Source != cwdPath {
		t.Fatalf("Source = %q, want %q (empty-rules policy must not be accepted as active)", res.Source, cwdPath)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning for empty-rules policy, got %d: %v", len(res.Warnings), res.Warnings)
	}
	if !strings.Contains(res.Warnings[0], envPath) {
		t.Errorf("expected warning to contain %q, got %q", envPath, res.Warnings[0])
	}
}

// TC-P2-004: 1~4순위 후보 모두 손상 -> builtin 폴백 + 경고 4건 누적
func TestPhase2_Policy_AllCandidatesCorruptedAccumulatesWarnings(t *testing.T) {
	isolatePolicyEnv(t)

	root := t.TempDir()
	envPath := filepath.Join(root, "env.yaml")
	cwdDir := filepath.Join(root, "cwd")
	wsDir := filepath.Join(root, "ws")
	homeDir := filepath.Join(root, "home")

	corrupted := []string{
		envPath,
		filepath.Join(cwdDir, policy.PolicyFileName),
		filepath.Join(wsDir, ".agents", policy.PolicyFileName),
		filepath.Join(homeDir, ".config", "guard", policy.PolicyFileName),
	}
	for _, p := range corrupted {
		writeYAML(t, p, "rules: [broken: {")
	}

	res := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       envPath,
		CWD:           cwdDir,
		WorkspaceRoot: wsDir,
		HomeDir:       homeDir,
	})

	if res.Source != policy.BuiltinSource {
		t.Fatalf("Source = %q, want %q", res.Source, policy.BuiltinSource)
	}
	if res.Policy == nil || len(res.Policy.Rules) == 0 {
		t.Fatalf("expected non-empty DefaultPolicy on total fallback")
	}
	if len(res.Warnings) != 4 {
		t.Fatalf("expected 4 accumulated warnings, got %d: %v", len(res.Warnings), res.Warnings)
	}
	for i, p := range corrupted {
		if !strings.Contains(res.Warnings[i], p) {
			t.Errorf("Warnings[%d] = %q, expected to mention %q", i, res.Warnings[i], p)
		}
	}
}

// TC-P2-005: 후보 파일이 전혀 없는 경우 -> builtin 폴백 + 경고 0건
func TestPhase2_Policy_NoCandidateFilesProducesNoWarnings(t *testing.T) {
	isolatePolicyEnv(t)

	res := policy.LoadDetailedWithOptions(policy.LoadOptions{
		EnvPath:       filepath.Join(t.TempDir(), "missing.yaml"),
		CWD:           t.TempDir(),
		WorkspaceRoot: t.TempDir(),
		HomeDir:       t.TempDir(),
	})

	if res.Source != policy.BuiltinSource {
		t.Fatalf("Source = %q, want %q", res.Source, policy.BuiltinSource)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected 0 warnings when candidate files do not exist, got %v", res.Warnings)
	}
	if res.Policy == nil || len(res.Policy.Rules) == 0 {
		t.Fatalf("expected non-empty builtin policy")
	}
}

// TC-P2-006: LoadFile 디렉토리 경로 / 미존재 파일 에러 반환
func TestPhase2_Policy_LoadFileErrors(t *testing.T) {
	if _, err := policy.LoadFile(t.TempDir()); err == nil {
		t.Errorf("expected error when LoadFile is given a directory path")
	}
	if _, err := policy.LoadFile(filepath.Join(t.TempDir(), "does-not-exist.yaml")); err == nil {
		t.Errorf("expected error when LoadFile is given a nonexistent path")
	}
}

// TC-P2-007: ResolveWorkspaceMode 4단계 폴백 (AGENT_MODE -> AGENT_ROLE -> .agents/current_mode -> DEV)
func TestPhase2_Policy_ResolveWorkspaceMode(t *testing.T) {
	t.Run("AGENT_MODE_highest_priority", func(t *testing.T) {
		isolatePolicyEnv(t)
		ws := t.TempDir()
		writeYAML(t, filepath.Join(ws, ".agents", "current_mode"), "PLAN\n")
		t.Setenv("AGENT_ROLE", "TEST")
		t.Setenv("AGENT_MODE", " all ")

		if got := policy.ResolveWorkspaceMode(ws); got != core.ModeAll {
			t.Errorf("ResolveWorkspaceMode = %q, want %q", got, core.ModeAll)
		}
	})

	t.Run("AGENT_ROLE_second_priority", func(t *testing.T) {
		isolatePolicyEnv(t)
		ws := t.TempDir()
		writeYAML(t, filepath.Join(ws, ".agents", "current_mode"), "PLAN\n")
		t.Setenv("AGENT_ROLE", " test ")

		if got := policy.ResolveWorkspaceMode(ws); got != core.ModeTest {
			t.Errorf("ResolveWorkspaceMode = %q, want %q", got, core.ModeTest)
		}
	})

	t.Run("current_mode_file_third_priority", func(t *testing.T) {
		isolatePolicyEnv(t)
		ws := t.TempDir()
		writeYAML(t, filepath.Join(ws, ".agents", "current_mode"), "  plan \n")

		if got := policy.ResolveWorkspaceMode(ws); got != core.ModePlan {
			t.Errorf("ResolveWorkspaceMode = %q, want %q", got, core.ModePlan)
		}
	})

	t.Run("empty_current_mode_falls_back_to_DEV", func(t *testing.T) {
		isolatePolicyEnv(t)
		ws := t.TempDir()
		writeYAML(t, filepath.Join(ws, ".agents", "current_mode"), "   \n\t ")

		if got := policy.ResolveWorkspaceMode(ws); got != core.ModeDev {
			t.Errorf("ResolveWorkspaceMode = %q, want %q", got, core.ModeDev)
		}
	})

	t.Run("no_env_no_file_defaults_to_DEV", func(t *testing.T) {
		isolatePolicyEnv(t)
		ws := t.TempDir()

		if got := policy.ResolveWorkspaceMode(ws); got != core.ModeDev {
			t.Errorf("ResolveWorkspaceMode = %q, want %q", got, core.ModeDev)
		}
	})
}
