package setup_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"guard/internal/core"
	"guard/internal/hub/policy"
	"guard/internal/hub/session"
	"guard/internal/hub/setup"
)

func isolateSetupEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GUARD_POLICY_PATH", "")
	t.Setenv("AGENT_MODE", "")
	t.Setenv("AGENT_ROLE", "")
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("GUARD_LANG", "en")
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return b
}

func findCheck(t *testing.T, report setup.DoctorReport, code string) setup.CheckResult {
	t.Helper()
	for _, c := range report.Checks {
		if c.Code == code {
			return c
		}
	}
	t.Fatalf("check %s not found in DoctorReport: %+v", code, report.Checks)
	return setup.CheckResult{}
}

func okLookPath(file string) (string, error) {
	return "/usr/local/bin/" + file, nil
}

func missingLookPath(file string) (string, error) {
	return "", exec.ErrNotFound
}

// =============================================================================
// Stage 1: Setup 핵심 유틸리티 (TC-INIT-001 ~ TC-INIT-016)
// =============================================================================

// TC-INIT-001: 단일 마커 파일 스택 감지
func TestSetup_DetectStack_SingleMarkers(t *testing.T) {
	cases := []struct {
		name       string
		markerFile string
		wantPreset string
		wantMarker string
	}{
		{name: "go", markerFile: "go.mod", wantPreset: setup.PresetGo, wantMarker: "go.mod"},
		{name: "node", markerFile: "package.json", wantPreset: setup.PresetNode, wantMarker: "package.json"},
		{name: "python_pyproject", markerFile: "pyproject.toml", wantPreset: setup.PresetPython, wantMarker: "pyproject.toml"},
		{name: "python_requirements", markerFile: "requirements.txt", wantPreset: setup.PresetPython, wantMarker: "requirements.txt"},
		{name: "generic_empty", markerFile: "", wantPreset: setup.PresetGeneric, wantMarker: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.markerFile != "" {
				mustWriteFile(t, filepath.Join(dir, tc.markerFile), "marker\n")
			}
			gotPreset, gotMarker := setup.DetectStack(dir)
			if gotPreset != tc.wantPreset || gotMarker != tc.wantMarker {
				t.Errorf("DetectStack() = (%q, %q), want (%q, %q)", gotPreset, gotMarker, tc.wantPreset, tc.wantMarker)
			}
		})
	}
}

// TC-INIT-002: 다중 마커 공존 시 우선순위 (go.mod > package.json > pyproject.toml > requirements.txt)
func TestSetup_DetectStack_PriorityOrder(t *testing.T) {
	cases := []struct {
		name       string
		markers    []string
		wantPreset string
		wantMarker string
	}{
		{
			name:       "go_beats_node_and_python",
			markers:    []string{"go.mod", "package.json", "pyproject.toml", "requirements.txt"},
			wantPreset: setup.PresetGo,
			wantMarker: "go.mod",
		},
		{
			name:       "node_beats_python",
			markers:    []string{"package.json", "pyproject.toml", "requirements.txt"},
			wantPreset: setup.PresetNode,
			wantMarker: "package.json",
		},
		{
			name:       "pyproject_beats_requirements",
			markers:    []string{"pyproject.toml", "requirements.txt"},
			wantPreset: setup.PresetPython,
			wantMarker: "pyproject.toml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, m := range tc.markers {
				mustWriteFile(t, filepath.Join(dir, m), "")
			}
			gotPreset, gotMarker := setup.DetectStack(dir)
			if gotPreset != tc.wantPreset || gotMarker != tc.wantMarker {
				t.Errorf("DetectStack() = (%q, %q), want (%q, %q)", gotPreset, gotMarker, tc.wantPreset, tc.wantMarker)
			}
		})
	}
}

// TC-INIT-003: ValidatePreset 허용/거부 및 대소문자
func TestSetup_ValidatePreset(t *testing.T) {
	for _, valid := range []string{"", "auto", "AUTO", "go", "GO", "Node", "python", "PYTHON", "generic", "Generic"} {
		if err := setup.ValidatePreset(valid); err != nil {
			t.Errorf("ValidatePreset(%q) = %v, want nil", valid, err)
		}
	}

	for _, invalid := range []string{"rust", "java", "cpp", "unknown"} {
		err := setup.ValidatePreset(invalid)
		if !errors.Is(err, setup.ErrInvalidPreset) {
			t.Errorf("ValidatePreset(%q) = %v, want errors.Is(_, ErrInvalidPreset)", invalid, err)
		}
	}
}

// TC-INIT-004 & TC-INIT-005: 4종 프리셋 정책 YAML 생성/파싱 유효성 및 미지원 프리셋 에러
func TestSetup_RenderPolicyYAML(t *testing.T) {
	for _, preset := range []string{setup.PresetGo, setup.PresetNode, setup.PresetPython, setup.PresetGeneric} {
		t.Run("valid_"+preset, func(t *testing.T) {
			yamlContent, err := setup.RenderPolicyYAML(preset)
			if err != nil {
				t.Fatalf("RenderPolicyYAML(%q) error = %v", preset, err)
			}
			if strings.TrimSpace(yamlContent) == "" {
				t.Fatalf("RenderPolicyYAML(%q) returned empty string", preset)
			}
			p := filepath.Join(t.TempDir(), "guard-policy.yaml")
			mustWriteFile(t, p, yamlContent)

			pol, err := policy.LoadFile(p)
			if err != nil {
				t.Fatalf("policy.LoadFile(%q) failed: %v\nYAML:\n%s", preset, err, yamlContent)
			}
			if pol == nil || len(pol.Rules) == 0 {
				t.Errorf("preset %q policy has 0 rules, want >= 1", preset)
			}
		})
	}

	for _, invalid := range []string{"rust", "auto", ""} {
		t.Run("invalid_"+invalid, func(t *testing.T) {
			_, err := setup.RenderPolicyYAML(invalid)
			if !errors.Is(err, setup.ErrInvalidPreset) {
				t.Errorf("RenderPolicyYAML(%q) err = %v, want ErrInvalidPreset", invalid, err)
			}
		})
	}
}

// TC-INIT-006 ~ TC-INIT-011: MergeWorkspaceHook 생성, Safe Merge, guard 부분키, 멱등성, 깨진 JSON, DryRun
func TestSetup_MergeWorkspaceHook(t *testing.T) {
	t.Run("TC-INIT-006_create_new_and_atomic", func(t *testing.T) {
		ws := t.TempDir()
		res, err := setup.MergeWorkspaceHook(ws, false)
		if err != nil {
			t.Fatalf("MergeWorkspaceHook: %v", err)
		}
		if res.Status != setup.StatusCreated {
			t.Errorf("Status = %q, want %q", res.Status, setup.StatusCreated)
		}
		if len(res.PreservedKeys) != 0 {
			t.Errorf("PreservedKeys = %v, want empty", res.PreservedKeys)
		}

		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		raw := mustReadBytes(t, hookPath)
		if !strings.Contains(string(raw), "\n  \"") {
			t.Errorf("expected 2-space indented JSON, got:\n%s", string(raw))
		}
		if !strings.Contains(string(raw), "guard hook") {
			t.Errorf("expected 'guard hook' command in hooks.json, got:\n%s", string(raw))
		}

		// .tmp 파일 잔존 여부 확인
		entries, err := os.ReadDir(filepath.Join(ws, ".agents"))
		if err != nil {
			t.Fatalf("ReadDir(.agents): %v", err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tmp") {
				t.Errorf("leftover .tmp file found: %s", e.Name())
			}
		}
	})

	t.Run("TC-INIT-007_safe_merge_preserves_other_keys", func(t *testing.T) {
		ws := t.TempDir()
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		existing := `{
  "zeta-tool": {
    "enabled": true,
    "retries": 3
  },
  "obsidian-chatlog": {
    "PostToolUse": [
      {
        "command": "chatlog append",
        "args": ["--vault", "/notes"]
      }
    ]
  }
}`
		mustWriteFile(t, hookPath, existing)

		res, err := setup.MergeWorkspaceHook(ws, false)
		if err != nil {
			t.Fatalf("MergeWorkspaceHook: %v", err)
		}
		if res.Status != setup.StatusMerged {
			t.Errorf("Status = %q, want %q", res.Status, setup.StatusMerged)
		}
		wantPreserved := []string{"obsidian-chatlog", "zeta-tool"}
		if !reflect.DeepEqual(res.PreservedKeys, wantPreserved) {
			t.Errorf("PreservedKeys = %v, want %v", res.PreservedKeys, wantPreserved)
		}

		var got map[string]any
		if err := json.Unmarshal(mustReadBytes(t, hookPath), &got); err != nil {
			t.Fatalf("merged hooks.json invalid JSON: %v", err)
		}
		var orig map[string]any
		_ = json.Unmarshal([]byte(existing), &orig)

		for _, k := range wantPreserved {
			if !reflect.DeepEqual(got[k], orig[k]) {
				t.Errorf("preserved key %q modified:\ngot:  %#v\nwant: %#v", k, got[k], orig[k])
			}
		}
		if _, ok := got["guard"]; !ok {
			t.Errorf("expected 'guard' key after merge")
		}
	})

	t.Run("TC-INIT-008_guard_key_exists_without_guard_hook", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{name: "empty_guard_obj", body: `{"guard": {}}`},
			{name: "other_command_in_PreToolUse", body: `{"guard": {"PreToolUse": [{"command": "other-tool"}]}}`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ws := t.TempDir()
				hookPath := filepath.Join(ws, ".agents", "hooks.json")
				mustWriteFile(t, hookPath, tc.body)

				res, err := setup.MergeWorkspaceHook(ws, false)
				if err != nil {
					t.Fatalf("MergeWorkspaceHook: %v", err)
				}
				if res.Status != setup.StatusMerged {
					t.Errorf("Status = %q, want %q", res.Status, setup.StatusMerged)
				}
				exists, valid, hasGuard, err := setup.InspectWorkspaceHook(ws)
				if err != nil || !exists || !valid || !hasGuard {
					t.Errorf("InspectWorkspaceHook after merge = (%v, %v, %v, %v), want (true, true, true, nil)", exists, valid, hasGuard, err)
				}
			})
		}
	})

	t.Run("TC-INIT-009_idempotent_second_call", func(t *testing.T) {
		ws := t.TempDir()
		if _, err := setup.MergeWorkspaceHook(ws, false); err != nil {
			t.Fatalf("first MergeWorkspaceHook: %v", err)
		}
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		firstBytes := mustReadBytes(t, hookPath)

		res2, err := setup.MergeWorkspaceHook(ws, false)
		if err != nil {
			t.Fatalf("second MergeWorkspaceHook: %v", err)
		}
		if res2.Status != setup.StatusSkipped {
			t.Errorf("second Status = %q, want %q", res2.Status, setup.StatusSkipped)
		}
		secondBytes := mustReadBytes(t, hookPath)
		if !bytes.Equal(firstBytes, secondBytes) {
			t.Errorf("hooks.json bytes changed on idempotent second call")
		}
	})

	t.Run("TC-INIT-010_corrupted_json_protected", func(t *testing.T) {
		corruptCases := []struct {
			name string
			body string
		}{
			{name: "broken_syntax", body: "{invalid json"},
			{name: "empty_file", body: ""},
			{name: "json_array", body: `["not", "an", "object"]`},
		}
		for _, tc := range corruptCases {
			t.Run(tc.name, func(t *testing.T) {
				ws := t.TempDir()
				hookPath := filepath.Join(ws, ".agents", "hooks.json")
				mustWriteFile(t, hookPath, tc.body)

				_, err := setup.MergeWorkspaceHook(ws, false)
				if !errors.Is(err, setup.ErrInvalidHooksJSON) {
					t.Errorf("MergeWorkspaceHook(%s) err = %v, want ErrInvalidHooksJSON", tc.name, err)
				}
				after := mustReadBytes(t, hookPath)
				if string(after) != tc.body {
					t.Errorf("corrupted hooks.json was modified! got %q, want %q", string(after), tc.body)
				}
			})
		}
	})

	t.Run("TC-INIT-011_dry_run_no_changes", func(t *testing.T) {
		// 1. 미존재 디렉토리
		ws1 := t.TempDir()
		res1, err := setup.MergeWorkspaceHook(ws1, true)
		if err != nil {
			t.Fatalf("dryRun empty: %v", err)
		}
		if res1.Status != setup.StatusPlanned {
			t.Errorf("dryRun empty Status = %q, want %q", res1.Status, setup.StatusPlanned)
		}
		if _, err := os.Stat(filepath.Join(ws1, ".agents")); !os.IsNotExist(err) {
			t.Errorf("dryRun created .agents directory unexpectedly")
		}

		// 2. 타 키만 존재
		ws2 := t.TempDir()
		hookPath2 := filepath.Join(ws2, ".agents", "hooks.json")
		orig2 := `{"obsidian-chatlog": {}}`
		mustWriteFile(t, hookPath2, orig2)
		res2, err := setup.MergeWorkspaceHook(ws2, true)
		if err != nil {
			t.Fatalf("dryRun merge: %v", err)
		}
		if res2.Status != setup.StatusPlanned {
			t.Errorf("dryRun merge Status = %q, want %q", res2.Status, setup.StatusPlanned)
		}
		if string(mustReadBytes(t, hookPath2)) != orig2 {
			t.Errorf("dryRun modified existing hooks.json")
		}

		// 3. 이미 등록됨
		ws3 := t.TempDir()
		if _, err := setup.MergeWorkspaceHook(ws3, false); err != nil {
			t.Fatalf("seed ws3: %v", err)
		}
		res3, err := setup.MergeWorkspaceHook(ws3, true)
		if err != nil {
			t.Fatalf("dryRun skipped: %v", err)
		}
		if res3.Status != setup.StatusSkipped {
			t.Errorf("dryRun already registered Status = %q, want %q", res3.Status, setup.StatusSkipped)
		}
	})
}

// TC-INIT-012: InspectWorkspaceHook 4상태 비파괴 판별
func TestSetup_InspectWorkspaceHook(t *testing.T) {
	t.Run("not_exists", func(t *testing.T) {
		exists, valid, hasGuard, err := setup.InspectWorkspaceHook(t.TempDir())
		if err != nil || exists || valid || hasGuard {
			t.Errorf("got (%v, %v, %v, %v), want (false, false, false, nil)", exists, valid, hasGuard, err)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		ws := t.TempDir()
		mustWriteFile(t, filepath.Join(ws, ".agents", "hooks.json"), "{broken")
		exists, valid, hasGuard, _ := setup.InspectWorkspaceHook(ws)
		if !exists || valid || hasGuard {
			t.Errorf("got (%v, %v, %v), want (true, false, false)", exists, valid, hasGuard)
		}
	})

	t.Run("valid_without_guard", func(t *testing.T) {
		ws := t.TempDir()
		mustWriteFile(t, filepath.Join(ws, ".agents", "hooks.json"), `{"other":{}}`)
		exists, valid, hasGuard, err := setup.InspectWorkspaceHook(ws)
		if err != nil || !exists || !valid || hasGuard {
			t.Errorf("got (%v, %v, %v, %v), want (true, true, false, nil)", exists, valid, hasGuard, err)
		}
	})

	t.Run("valid_with_guard", func(t *testing.T) {
		ws := t.TempDir()
		if _, err := setup.MergeWorkspaceHook(ws, false); err != nil {
			t.Fatalf("MergeWorkspaceHook: %v", err)
		}
		exists, valid, hasGuard, err := setup.InspectWorkspaceHook(ws)
		if err != nil || !exists || !valid || !hasGuard {
			t.Errorf("got (%v, %v, %v, %v), want (true, true, true, nil)", exists, valid, hasGuard, err)
		}
	})
}

// TC-INIT-013 ~ TC-INIT-016: WorkflowRuleContent, HasAlwaysOnFrontmatter, InjectAlwaysOnFrontmatter, EnsureWorkflowRule
func TestSetup_WorkflowRuleAndFrontmatter(t *testing.T) {
	t.Run("TC-INIT-013_WorkflowRuleContent_required_elements", func(t *testing.T) {
		content := setup.WorkflowRuleContent()
		if !setup.HasAlwaysOnFrontmatter(content) {
			t.Errorf("WorkflowRuleContent() missing trigger: always_on frontmatter")
		}
		if !strings.Contains(content, setup.WorkflowManagedMarker) {
			t.Errorf("WorkflowRuleContent() missing marker %q", setup.WorkflowManagedMarker)
		}
		for _, kw := range []string{"guard handoff", "inbox", "ack", "send", "approve"} {
			if !strings.Contains(content, kw) {
				t.Errorf("WorkflowRuleContent() missing keyword %q", kw)
			}
		}
	})

	t.Run("TC-INIT-014_HasAlwaysOnFrontmatter_boundaries", func(t *testing.T) {
		cases := []struct {
			name    string
			content string
			want    bool
		}{
			{name: "standard", content: "---\ntrigger: always_on\n---\n\n# Body\n", want: true},
			{name: "with_other_keys", content: "---\nname: guard\ntrigger: always_on\ndescription: x\n---\n# Body", want: true},
			{name: "crlf_frontmatter", content: "---\r\ntrigger: always_on\r\n---\r\n# Body\r\n", want: true},
			{name: "no_frontmatter", content: "# Just Markdown\nhello\n", want: false},
			{name: "only_in_body_false_positive_check", content: "# Body\ntrigger: always_on\n", want: false},
			{name: "unclosed_frontmatter", content: "---\ntrigger: always_on\n# Body without closing", want: false},
			{name: "different_trigger_value", content: "---\ntrigger: manual\n---\n# Body", want: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := setup.HasAlwaysOnFrontmatter(tc.content); got != tc.want {
					t.Errorf("HasAlwaysOnFrontmatter(%s) = %v, want %v", tc.name, got, tc.want)
				}
			})
		}
	})

	t.Run("TC-INIT-015_InjectAlwaysOnFrontmatter_3_branches", func(t *testing.T) {
		// 1. 이미 포함 -> 원본 그대로
		already := "---\ntrigger: always_on\n---\n\n# Custom Rule\nKeep me.\n"
		if got := setup.InjectAlwaysOnFrontmatter(already); got != already {
			t.Errorf("already-present content modified:\ngot:  %q\nwant: %q", got, already)
		}

		// 2. frontmatter 없음 -> 상단 추가 + 본문 보존
		noFM := "# User Custom Rule\n- Do not delete this line.\n"
		gotNoFM := setup.InjectAlwaysOnFrontmatter(noFM)
		if !setup.HasAlwaysOnFrontmatter(gotNoFM) {
			t.Errorf("InjectAlwaysOnFrontmatter(noFM) did not produce always_on frontmatter:\n%s", gotNoFM)
		}
		if !strings.Contains(gotNoFM, noFM) {
			t.Errorf("InjectAlwaysOnFrontmatter(noFM) lost original body:\n%s", gotNoFM)
		}

		// 3. 기존 frontmatter에 trigger만 없음 -> 기존 frontmatter 내부에 삽입 + 기존 키 및 본문 보존
		existingFM := "---\ndescription: user rule\n---\n\n# My Custom Body\nLine 2\n"
		gotExisting := setup.InjectAlwaysOnFrontmatter(existingFM)
		if !setup.HasAlwaysOnFrontmatter(gotExisting) {
			t.Errorf("InjectAlwaysOnFrontmatter(existingFM) missing always_on:\n%s", gotExisting)
		}
		if !strings.Contains(gotExisting, "description: user rule") {
			t.Errorf("InjectAlwaysOnFrontmatter(existingFM) lost existing frontmatter key:\n%s", gotExisting)
		}
		if !strings.Contains(gotExisting, "# My Custom Body\nLine 2\n") {
			t.Errorf("InjectAlwaysOnFrontmatter(existingFM) lost body:\n%s", gotExisting)
		}
	})

	t.Run("TC-INIT-016_EnsureWorkflowRule", func(t *testing.T) {
		ws := t.TempDir()
		rulePath, created, err := setup.EnsureWorkflowRule(ws)
		if err != nil {
			t.Fatalf("EnsureWorkflowRule(new): %v", err)
		}
		if !created {
			t.Errorf("created = false, want true")
		}
		if !setup.HasAlwaysOnFrontmatter(string(mustReadBytes(t, rulePath))) {
			t.Errorf("created workflow rule missing trigger: always_on")
		}

		// 기존 파일 커스텀 내용으로 교체 후 재호출 -> 보존 확인
		custom := "# Custom user rule\n"
		mustWriteFile(t, rulePath, custom)
		_, created2, err := setup.EnsureWorkflowRule(ws)
		if err != nil {
			t.Fatalf("EnsureWorkflowRule(existing): %v", err)
		}
		if created2 {
			t.Errorf("created2 = true, want false")
		}
		if string(mustReadBytes(t, rulePath)) != custom {
			t.Errorf("EnsureWorkflowRule overwrote existing file")
		}
	})
}

// =============================================================================
// Stage 2: RunInit 오케스트레이션 (TC-INIT-017 ~ TC-INIT-025)
// =============================================================================

// TC-INIT-017: 빈 Go 프로젝트 기본 초기화
func TestSetup_RunInit_DefaultGoWorkspace(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module example.com/app\n")

	res, err := setup.RunInit(setup.InitOptions{WorkspaceRoot: ws})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if res.DetectedPreset != setup.PresetGo || res.DetectedMarker != "go.mod" {
		t.Errorf("detected = (%q, %q), want (%q, %q)", res.DetectedPreset, res.DetectedMarker, setup.PresetGo, "go.mod")
	}
	if res.DryRun {
		t.Errorf("DryRun = true, want false")
	}

	// 4개 파일 생성 검증
	for _, rel := range []string{
		".agents/hooks.json",
		"guard-policy.yaml",
		".agents/rules/guard-workflow.md",
		".agents/current_mode",
	} {
		if _, err := os.Stat(filepath.Join(ws, rel)); err != nil {
			t.Errorf("expected %s to be created: %v", rel, err)
		}
	}

	mode := strings.TrimSpace(string(mustReadBytes(t, filepath.Join(ws, ".agents", "current_mode"))))
	if mode != core.ModeDev {
		t.Errorf("current_mode = %q, want %q", mode, core.ModeDev)
	}
}

// TC-INIT-018: 명시적 프리셋 및 모드 정규화
func TestSetup_RunInit_ExplicitPresetAndModeNormalization(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module example.com/app\n")

	res, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: ws,
		Preset:        "python",
		Mode:          "test",
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if res.DetectedPreset != setup.PresetPython {
		t.Errorf("DetectedPreset = %q, want %q", res.DetectedPreset, setup.PresetPython)
	}
	mode := strings.TrimSpace(string(mustReadBytes(t, filepath.Join(ws, ".agents", "current_mode"))))
	if mode != core.ModeTest {
		t.Errorf("current_mode = %q, want %q", mode, core.ModeTest)
	}
}

// TC-INIT-019: 잘못된 Preset / Mode 입력 시 Fail-Fast (파일 미생성)
func TestSetup_RunInit_InvalidOptionsFailFast(t *testing.T) {
	isolateSetupEnv(t)
	cases := []struct {
		name    string
		opts    setup.InitOptions
		wantErr error
	}{
		{name: "invalid_preset", opts: setup.InitOptions{Preset: "rust"}, wantErr: setup.ErrInvalidPreset},
		{name: "invalid_mode", opts: setup.InitOptions{Mode: "ADMIN"}, wantErr: setup.ErrInvalidMode},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			tc.opts.WorkspaceRoot = ws
			_, err := setup.RunInit(tc.opts)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("RunInit err = %v, want %v", err, tc.wantErr)
			}
			entries, _ := os.ReadDir(ws)
			if len(entries) != 0 {
				t.Errorf("expected 0 files created on Fail-Fast, got %d", len(entries))
			}
		})
	}
}

// TC-INIT-020: 2회 연속 실행 멱등성
func TestSetup_RunInit_Idempotent(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module x\n")

	if _, err := setup.RunInit(setup.InitOptions{WorkspaceRoot: ws}); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}

	targets := []string{
		".agents/hooks.json",
		"guard-policy.yaml",
		".agents/rules/guard-workflow.md",
		".agents/current_mode",
	}
	firstSnap := make(map[string][]byte, len(targets))
	for _, rel := range targets {
		firstSnap[rel] = mustReadBytes(t, filepath.Join(ws, rel))
	}

	res2, err := setup.RunInit(setup.InitOptions{WorkspaceRoot: ws})
	if err != nil {
		t.Fatalf("second RunInit: %v", err)
	}
	if len(res2.Steps) == 0 {
		t.Fatalf("expected non-empty Steps on second RunInit")
	}
	for _, st := range res2.Steps {
		if st.Status != setup.StatusSkipped {
			t.Errorf("second RunInit step %q status = %q, want %q", st.Target, st.Status, setup.StatusSkipped)
		}
	}
	for _, rel := range targets {
		after := mustReadBytes(t, filepath.Join(ws, rel))
		if !bytes.Equal(firstSnap[rel], after) {
			t.Errorf("file %s modified on idempotent second RunInit", rel)
		}
	}
}

// TC-INIT-021: --dry-run 완전 무변경
func TestSetup_RunInit_DryRunNoMutation(t *testing.T) {
	isolateSetupEnv(t)

	// 빈 디렉토리
	ws := t.TempDir()
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module x\n")

	res, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: ws,
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("RunInit(DryRun): %v", err)
	}
	if !res.DryRun {
		t.Errorf("result.DryRun = false, want true")
	}
	for _, st := range res.Steps {
		if st.Status != setup.StatusPlanned {
			t.Errorf("dryRun step %q status = %q, want %q", st.Target, st.Status, setup.StatusPlanned)
		}
	}
	entries, _ := os.ReadDir(ws)
	if len(entries) != 1 || entries[0].Name() != "go.mod" {
		t.Errorf("DryRun mutated workspace! entries = %v", entries)
	}
}

// TC-INIT-022: --force .bak 백업 무손실 재생성
func TestSetup_RunInit_ForceBackup(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module x\n")

	customPolicy := "version: \"1\"\n# custom policy\n"
	customRule := "# custom workflow rule\n"
	policyPath := filepath.Join(ws, "guard-policy.yaml")
	rulePath := filepath.Join(ws, ".agents", "rules", "guard-workflow.md")
	mustWriteFile(t, policyPath, customPolicy)
	mustWriteFile(t, rulePath, customRule)

	res, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: ws,
		Force:         true,
	})
	if err != nil {
		t.Fatalf("RunInit(Force): %v", err)
	}

	backedUp := map[string]string{}
	for _, st := range res.Steps {
		if st.Status == setup.StatusBackedUp {
			backedUp[st.Target] = st.BackupPath
		}
	}
	if _, ok := backedUp["guard-policy.yaml"]; !ok {
		t.Errorf("expected guard-policy.yaml to be StatusBackedUp, got steps: %+v", res.Steps)
	}
	if _, ok := backedUp[".agents/rules/guard-workflow.md"]; !ok {
		t.Errorf("expected .agents/rules/guard-workflow.md to be StatusBackedUp, got steps: %+v", res.Steps)
	}

	if got := string(mustReadBytes(t, policyPath+".bak")); got != customPolicy {
		t.Errorf("policy .bak mismatch: got %q, want %q", got, customPolicy)
	}
	if got := string(mustReadBytes(t, rulePath+".bak")); got != customRule {
		t.Errorf("rule .bak mismatch: got %q, want %q", got, customRule)
	}
	if string(mustReadBytes(t, policyPath)) == customPolicy {
		t.Errorf("guard-policy.yaml was not regenerated after --force")
	}
	if !setup.HasAlwaysOnFrontmatter(string(mustReadBytes(t, rulePath))) {
		t.Errorf("regenerated guard-workflow.md missing trigger: always_on")
	}
}

// TC-INIT-023: 선택 스킵 옵션 (NoHooks, NoPolicy, NoRules)
func TestSetup_RunInit_SkipFlags(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()

	_, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: ws,
		NoHooks:       true,
		NoPolicy:      true,
		NoRules:       true,
	})
	if err != nil {
		t.Fatalf("RunInit(all skip): %v", err)
	}
	for _, rel := range []string{".agents/hooks.json", "guard-policy.yaml", ".agents/rules/guard-workflow.md"} {
		if _, err := os.Stat(filepath.Join(ws, rel)); !os.IsNotExist(err) {
			t.Errorf("skipped target %s was created unexpectedly", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(ws, ".agents", "current_mode")); err != nil {
		t.Errorf("expected .agents/current_mode to still be created: %v", err)
	}
}

// TC-INIT-024: 깨진 hooks.json 존재 시 RunInit 에러 및 원본 무변경
func TestSetup_RunInit_CorruptedHooksJSON(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	hookPath := filepath.Join(ws, ".agents", "hooks.json")
	mustWriteFile(t, hookPath, "{corrupted")

	_, err := setup.RunInit(setup.InitOptions{WorkspaceRoot: ws})
	if !errors.Is(err, setup.ErrInvalidHooksJSON) {
		t.Errorf("RunInit err = %v, want ErrInvalidHooksJSON", err)
	}
	if string(mustReadBytes(t, hookPath)) != "{corrupted" {
		t.Errorf("corrupted hooks.json modified by RunInit")
	}
}

// TC-INIT-025: 기존 .agents/current_mode 보존
func TestSetup_RunInit_PreservesExistingCurrentMode(t *testing.T) {
	isolateSetupEnv(t)
	ws := t.TempDir()
	modePath := filepath.Join(ws, ".agents", "current_mode")
	mustWriteFile(t, modePath, "PLAN\n")

	res, err := setup.RunInit(setup.InitOptions{
		WorkspaceRoot: ws,
		Mode:          "DEV",
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if got := strings.TrimSpace(string(mustReadBytes(t, modePath))); got != "PLAN" {
		t.Errorf("current_mode overwritten: got %q, want PLAN", got)
	}
	foundSkipped := false
	for _, st := range res.Steps {
		if st.Target == ".agents/current_mode" && st.Status == setup.StatusSkipped {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Errorf("expected .agents/current_mode step StatusSkipped, got %+v", res.Steps)
	}
}

// =============================================================================
// Stage 3: RunDoctor 진단 및 자동 복구 (TC-DOC-001 ~ TC-DOC-012)
// =============================================================================

func newHealthyWorkspace(t *testing.T) (string, *session.Manager) {
	t.Helper()
	ws := t.TempDir()
	// canonical path로 정규화 (macOS /var -> /private/var 등 대응)
	if resolved, err := filepath.EvalSymlinks(ws); err == nil {
		ws = resolved
	}
	mustWriteFile(t, filepath.Join(ws, "go.mod"), "module example.com/healthy\n")
	if _, err := setup.RunInit(setup.InitOptions{WorkspaceRoot: ws}); err != nil {
		t.Fatalf("RunInit healthy ws: %v", err)
	}
	sessDir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll sessions: %v", err)
	}
	return ws, session.NewManager(sessDir)
}

// TC-DOC-001: 완전히 건강한 워크스페이스 진단
func TestSetup_RunDoctor_HealthyWorkspace(t *testing.T) {
	isolateSetupEnv(t)
	ws, sessMgr := newHealthyWorkspace(t)

	report := setup.RunDoctor(setup.DoctorOptions{
		WorkspaceRoot: ws,
		SessionMgr:    sessMgr,
		Fix:           false,
		LookPath:      okLookPath,
	})

	if len(report.Checks) != 6 {
		t.Fatalf("len(Checks) = %d, want 6", len(report.Checks))
	}
	wantCodes := []string{
		setup.CodeBinaryPath,
		setup.CodeHooksJSON,
		setup.CodePolicyYAML,
		setup.CodeWorkflowRule,
		setup.CodeSessionStore,
		setup.CodeSymlinkPath,
	}
	for i, code := range wantCodes {
		if report.Checks[i].Code != code {
			t.Errorf("Checks[%d].Code = %q, want %q", i, report.Checks[i].Code, code)
		}
		if report.Checks[i].Severity != setup.SeverityOK {
			t.Errorf("Checks[%d] (%s) Severity = %q, want OK (msg: %s)", i, code, report.Checks[i].Severity, report.Checks[i].Message)
		}
	}
	if report.ErrorCount != 0 || report.WarnCount != 0 || report.FixedCount != 0 || report.ExitCode() != 0 {
		t.Errorf("unexpected counts/exit: err=%d warn=%d fixed=%d exit=%d", report.ErrorCount, report.WarnCount, report.FixedCount, report.ExitCode())
	}
}

// TC-DOC-002: DOC-001 PATH 미발견 (복구 불가 ERROR)
func TestSetup_RunDoctor_DOC001_MissingBinary(t *testing.T) {
	isolateSetupEnv(t)
	ws, sessMgr := newHealthyWorkspace(t)

	for _, fix := range []bool{false, true} {
		report := setup.RunDoctor(setup.DoctorOptions{
			WorkspaceRoot: ws,
			SessionMgr:    sessMgr,
			Fix:           fix,
			LookPath:      missingLookPath,
		})
		c := findCheck(t, report, setup.CodeBinaryPath)
		if c.Severity != setup.SeverityError || c.Fixable || c.Fixed {
			t.Errorf("fix=%v DOC-001 = %+v, want ERROR & Fixable=false & Fixed=false", fix, c)
		}
		if report.ExitCode() != 1 {
			t.Errorf("fix=%v ExitCode() = %d, want 1", fix, report.ExitCode())
		}
	}
}

// TC-DOC-003 & TC-DOC-004: DOC-002 누락 자동 복구 vs 깨진 JSON 보호
func TestSetup_RunDoctor_DOC002_Hooks(t *testing.T) {
	isolateSetupEnv(t)

	t.Run("missing_guard_hook_auto_fixed_preserving_other_keys", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		mustWriteFile(t, hookPath, `{"obsidian-chatlog": {"enabled": true}}`)

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		c1 := findCheck(t, rep1, setup.CodeHooksJSON)
		if c1.Severity != setup.SeverityError || !c1.Fixable || rep1.ExitCode() != 1 {
			t.Errorf("before fix DOC-002 = %+v (exit=%d), want ERROR & Fixable=true", c1, rep1.ExitCode())
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		c2 := findCheck(t, rep2, setup.CodeHooksJSON)
		if c2.Severity != setup.SeverityFixed || !c2.Fixed || rep2.FixedCount < 1 || rep2.ExitCode() != 0 {
			t.Errorf("after fix DOC-002 = %+v (fixedCount=%d, exit=%d), want FIXED", c2, rep2.FixedCount, rep2.ExitCode())
		}
		if !strings.Contains(string(mustReadBytes(t, hookPath)), "obsidian-chatlog") {
			t.Errorf("DOC-002 --fix lost obsidian-chatlog key")
		}

		rep3 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if findCheck(t, rep3, setup.CodeHooksJSON).Severity != setup.SeverityOK {
			t.Errorf("re-check DOC-002 not OK after fix")
		}
	})

	t.Run("corrupted_json_not_overwritten_on_fix", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		mustWriteFile(t, hookPath, "{broken")

		rep := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		c := findCheck(t, rep, setup.CodeHooksJSON)
		if c.Severity != setup.SeverityError || c.Fixed || rep.ExitCode() != 1 {
			t.Errorf("corrupted hooks.json on --fix: %+v (exit=%d), want ERROR & Fixed=false", c, rep.ExitCode())
		}
		if string(mustReadBytes(t, hookPath)) != "{broken" {
			t.Errorf("DOC-002 --fix overwrote corrupted hooks.json!")
		}
	})
}

// TC-DOC-005 & TC-DOC-006: DOC-003 builtin fallback 자동 생성 vs 기존 정책 문법 오류 보호
func TestSetup_RunDoctor_DOC003_Policy(t *testing.T) {
	isolateSetupEnv(t)

	t.Run("missing_policy_builtin_warn_and_autofix", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		policyPath := filepath.Join(ws, "guard-policy.yaml")
		if err := os.Remove(policyPath); err != nil {
			t.Fatalf("Remove policy: %v", err)
		}

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		c1 := findCheck(t, rep1, setup.CodePolicyYAML)
		if c1.Severity != setup.SeverityWarn || !c1.Fixable || rep1.ExitCode() != 0 {
			t.Errorf("missing policy DOC-003 = %+v (exit=%d), want WARN & Fixable=true & exit=0", c1, rep1.ExitCode())
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		c2 := findCheck(t, rep2, setup.CodePolicyYAML)
		if c2.Severity != setup.SeverityFixed || !c2.Fixed {
			t.Errorf("fixed policy DOC-003 = %+v, want FIXED", c2)
		}

		rep3 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if findCheck(t, rep3, setup.CodePolicyYAML).Severity != setup.SeverityOK {
			t.Errorf("re-check DOC-003 not OK after fix")
		}
	})

	t.Run("broken_existing_policy_not_overwritten_on_fix", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		policyPath := filepath.Join(ws, "guard-policy.yaml")
		brokenYAML := "version: \"1\"\nrules:\n  - [invalid_yaml\n"
		mustWriteFile(t, policyPath, brokenYAML)

		rep := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		c := findCheck(t, rep, setup.CodePolicyYAML)
		if c.Severity != setup.SeverityError || c.Fixed || rep.ExitCode() != 1 {
			t.Errorf("broken policy on --fix: %+v (exit=%d), want ERROR & Fixed=false & exit=1", c, rep.ExitCode())
		}
		if string(mustReadBytes(t, policyPath)) != brokenYAML {
			t.Errorf("DOC-003 --fix overwrote broken guard-policy.yaml!")
		}
	})
}

// TC-DOC-007 & TC-DOC-008: DOC-004 룰 파일 미존재 생성 & trigger: always_on 누락 비파괴 복구
func TestSetup_RunDoctor_DOC004_WorkflowRule(t *testing.T) {
	isolateSetupEnv(t)

	t.Run("missing_rule_autofix", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		rulePath := filepath.Join(ws, ".agents", "rules", "guard-workflow.md")
		_ = os.Remove(rulePath)

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if c := findCheck(t, rep1, setup.CodeWorkflowRule); c.Severity != setup.SeverityWarn || !c.Fixable {
			t.Errorf("missing rule DOC-004 = %+v, want WARN & Fixable=true", c)
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		if c := findCheck(t, rep2, setup.CodeWorkflowRule); c.Severity != setup.SeverityFixed || !c.Fixed {
			t.Errorf("fixed missing rule DOC-004 = %+v, want FIXED", c)
		}
		if !setup.HasAlwaysOnFrontmatter(string(mustReadBytes(t, rulePath))) {
			t.Errorf("created rule missing trigger: always_on")
		}
	})

	t.Run("missing_frontmatter_non_destructive_fix", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		rulePath := filepath.Join(ws, ".agents", "rules", "guard-workflow.md")
		userBody := "# My Custom Team Rules\n- Always write clean Go code.\n- Never touch secrets.\n"
		mustWriteFile(t, rulePath, userBody)

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if c := findCheck(t, rep1, setup.CodeWorkflowRule); c.Severity != setup.SeverityWarn || !c.Fixable {
			t.Errorf("no-frontmatter DOC-004 = %+v, want WARN & Fixable=true", c)
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		if c := findCheck(t, rep2, setup.CodeWorkflowRule); c.Severity != setup.SeverityFixed || !c.Fixed {
			t.Errorf("fixed no-frontmatter DOC-004 = %+v, want FIXED", c)
		}

		updated := string(mustReadBytes(t, rulePath))
		if !setup.HasAlwaysOnFrontmatter(updated) {
			t.Errorf("updated rule missing trigger: always_on:\n%s", updated)
		}
		if !strings.Contains(updated, userBody) {
			t.Errorf("DOC-004 --fix lost user body!\ngot:\n%s\nwant substring:\n%s", updated, userBody)
		}

		rep3 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if findCheck(t, rep3, setup.CodeWorkflowRule).Severity != setup.SeverityOK {
			t.Errorf("re-check DOC-004 not OK after non-destructive fix")
		}
	})
}

// TC-DOC-009 & TC-DOC-010: DOC-005 세션 디렉토리 미존재, Stale PID 정리, 쓰기 권한 없음
func TestSetup_RunDoctor_DOC005_SessionStore(t *testing.T) {
	isolateSetupEnv(t)

	t.Run("missing_session_dir_autofix", func(t *testing.T) {
		ws, _ := newHealthyWorkspace(t)
		missingDir := filepath.Join(t.TempDir(), "not-yet-created-sessions")
		sessMgr := session.NewManager(missingDir)

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if c := findCheck(t, rep1, setup.CodeSessionStore); c.Severity != setup.SeverityWarn || !c.Fixable {
			t.Errorf("missing dir DOC-005 = %+v, want WARN & Fixable=true", c)
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		if c := findCheck(t, rep2, setup.CodeSessionStore); c.Severity != setup.SeverityFixed || !c.Fixed {
			t.Errorf("fixed missing dir DOC-005 = %+v, want FIXED", c)
		}
		if _, err := os.Stat(missingDir); err != nil {
			t.Errorf("expected session dir to be created: %v", err)
		}
	})

	t.Run("stale_pid_cleaned_while_alive_preserved", func(t *testing.T) {
		ws, sessMgr := newHealthyWorkspace(t)
		if err := sessMgr.Create(&session.Session{ID: "alive-sess", Mode: core.ModeDev, PID: os.Getpid(), Workspace: ws}); err != nil {
			t.Fatalf("Create alive: %v", err)
		}
		if err := sessMgr.Create(&session.Session{ID: "dead-sess", Mode: core.ModeDev, PID: 999999, Workspace: ws}); err != nil {
			t.Fatalf("Create dead: %v", err)
		}
		if err := sessMgr.Create(&session.Session{ID: "zero-pid-sess", Mode: core.ModeDev, PID: 0, Workspace: ws}); err != nil {
			t.Fatalf("Create zero pid: %v", err)
		}

		rep1 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		if c := findCheck(t, rep1, setup.CodeSessionStore); c.Severity != setup.SeverityWarn || !c.Fixable {
			t.Errorf("stale sessions DOC-005 = %+v, want WARN & Fixable=true", c)
		}

		rep2 := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: true, LookPath: okLookPath})
		if c := findCheck(t, rep2, setup.CodeSessionStore); c.Severity != setup.SeverityFixed || !c.Fixed {
			t.Errorf("fixed stale sessions DOC-005 = %+v, want FIXED", c)
		}

		if _, err := sessMgr.Get("alive-sess"); err != nil {
			t.Errorf("alive session was deleted unexpectedly: %v", err)
		}
		if _, err := sessMgr.Get("dead-sess"); err == nil {
			t.Errorf("dead session still exists after --fix")
		}
		if _, err := sessMgr.Get("zero-pid-sess"); err == nil {
			t.Errorf("zero-pid session still exists after --fix")
		}
	})

	t.Run("readonly_session_dir_error", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("skipping permission test when running as root")
		}
		ws, _ := newHealthyWorkspace(t)
		roDir := filepath.Join(t.TempDir(), "ro-sessions")
		if err := os.MkdirAll(roDir, 0o555); err != nil {
			t.Fatalf("MkdirAll roDir: %v", err)
		}
		defer os.Chmod(roDir, 0o755)

		sessMgr := session.NewManager(roDir)
		rep := setup.RunDoctor(setup.DoctorOptions{WorkspaceRoot: ws, SessionMgr: sessMgr, Fix: false, LookPath: okLookPath})
		c := findCheck(t, rep, setup.CodeSessionStore)
		if c.Severity != setup.SeverityError || rep.ExitCode() != 1 {
			t.Errorf("readonly dir DOC-005 = %+v (exit=%d), want ERROR & exit=1", c, rep.ExitCode())
		}
	})
}

// TC-DOC-011: DOC-006 WorkspaceRoot가 Symlink인 경우
func TestSetup_RunDoctor_DOC006_SymlinkWorkspace(t *testing.T) {
	isolateSetupEnv(t)
	realWS, sessMgr := newHealthyWorkspace(t)
	linkWS := filepath.Join(t.TempDir(), "ws-symlink")
	if err := os.Symlink(realWS, linkWS); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	for _, fix := range []bool{false, true} {
		rep := setup.RunDoctor(setup.DoctorOptions{
			WorkspaceRoot: linkWS,
			SessionMgr:    sessMgr,
			Fix:           fix,
			LookPath:      okLookPath,
		})
		c := findCheck(t, rep, setup.CodeSymlinkPath)
		if c.Severity != setup.SeverityWarn || c.Fixable || c.Fixed {
			t.Errorf("fix=%v DOC-006 = %+v, want WARN & Fixable=false & Fixed=false", fix, c)
		}
		if rep.ExitCode() != 0 {
			t.Errorf("fix=%v symlink-only warning ExitCode() = %d, want 0", fix, rep.ExitCode())
		}
	}
}

// TC-DOC-012: DoctorReport 카운터 집계 및 ExitCode() 경계
func TestSetup_DoctorReport_ExitCodeAndCounters(t *testing.T) {
	warnOnly := setup.DoctorReport{WarnCount: 3, ErrorCount: 0}
	if warnOnly.ExitCode() != 0 {
		t.Errorf("warnOnly.ExitCode() = %d, want 0", warnOnly.ExitCode())
	}
	withErr := setup.DoctorReport{WarnCount: 0, ErrorCount: 1}
	if withErr.ExitCode() != 1 {
		t.Errorf("withErr.ExitCode() = %d, want 1", withErr.ExitCode())
	}
	fixedOnly := setup.DoctorReport{FixedCount: 2, ErrorCount: 0}
	if fixedOnly.ExitCode() != 0 {
		t.Errorf("fixedOnly.ExitCode() = %d, want 0", fixedOnly.ExitCode())
	}
}
