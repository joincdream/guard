package launcher_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guard/internal/adapters/launcher"
	"guard/internal/core"
	"guard/internal/hub/session"
)

func isolateLauncherEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("AGENT_MODE", "")
	t.Setenv("AGENT_ROLE", "")
	t.Setenv("GUARD_POLICY_PATH", "")
	t.Setenv("GUARD_DEBUG", "")
}

// TC-P3-007: 빈 cmdArgs 실행 방어
func TestPhase3_Launcher_EmptyCmdArgsReturnsOne(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "nil_args", args: nil},
		{name: "empty_slice", args: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateLauncherEnv(t)

			sessDir := t.TempDir()
			mgr := session.NewManager(sessDir)
			var stdout, stderr bytes.Buffer
			l := launcher.New(mgr, &stdout, &stderr)

			code := l.Run(core.ModeDev, t.TempDir(), tc.args)
			if code != 1 {
				t.Errorf("Run() exit code = %d, want 1", code)
			}
			if strings.TrimSpace(stderr.String()) == "" {
				t.Errorf("expected error message on stderr for empty cmdArgs")
			}
			list, err := mgr.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 0 {
				t.Errorf("expected 0 sessions created on empty cmdArgs, got %d", len(list))
			}
		})
	}
}

// TC-P3-008: 자식 프로세스 환경변수 주입 · 실제 자식 PID 등록 · 종료 시 정리
func TestPhase3_Launcher_ChildEnvPIDRegistrationAndCleanup(t *testing.T) {
	isolateLauncherEnv(t)

	sessDir := t.TempDir()
	ws := t.TempDir()
	mgr := session.NewManager(sessDir)

	// Record runtime observations from inside the child process to a temp file
	probeFile := filepath.Join(t.TempDir(), "probe.txt")
	script := `
echo "AGENT_MODE=$AGENT_MODE" > "$PROBE_FILE"
echo "GUARD_SESSION_ID=$GUARD_SESSION_ID" >> "$PROBE_FILE"
echo "CHILD_PID=$$" >> "$PROBE_FILE"
if [ -f "$SESS_DIR/$GUARD_SESSION_ID.json" ]; then
  echo "SESS_FILE_EXISTS=true" >> "$PROBE_FILE"
  cat "$SESS_DIR/$GUARD_SESSION_ID.json" > "$PROBE_FILE.sess.json"
else
  echo "SESS_FILE_EXISTS=false" >> "$PROBE_FILE"
fi
`
	t.Setenv("PROBE_FILE", probeFile)
	t.Setenv("SESS_DIR", sessDir)

	var stdout, stderr bytes.Buffer
	l := launcher.New(mgr, &stdout, &stderr)

	code := l.Run(core.ModePlan, ws, []string{"sh", "-c", script})
	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}

	probeData, err := os.ReadFile(probeFile)
	if err != nil {
		t.Fatalf("ReadFile(probe): %v", err)
	}
	probeText := string(probeData)

	if !strings.Contains(probeText, "AGENT_MODE=PLAN") {
		t.Errorf("child did not receive AGENT_MODE=PLAN; probe:\n%s", probeText)
	}
	if !strings.Contains(probeText, "SESS_FILE_EXISTS=true") {
		t.Errorf("session file was not created during child execution; probe:\n%s", probeText)
	}

	// Verify the recorded session JSON during child execution had PID > 0 and Mode == PLAN
	sessBytes, err := os.ReadFile(probeFile + ".sess.json")
	if err != nil {
		t.Fatalf("ReadFile(sess.json snapshot): %v", err)
	}
	var snap session.Session
	if err := json.Unmarshal(sessBytes, &snap); err != nil {
		t.Fatalf("Unmarshal snapshot: %v", err)
	}
	if snap.PID <= 0 {
		t.Errorf("expected recorded child PID > 0, got %d", snap.PID)
	}
	if snap.Mode != core.ModePlan {
		t.Errorf("expected recorded session Mode=%q, got %q", core.ModePlan, snap.Mode)
	}

	// After Run() returns, session must be cleaned up via defer Delete
	remaining, err := mgr.List()
	if err != nil {
		t.Fatalf("List after Run: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 remaining sessions after child exit (defer Delete), got %d: %+v", len(remaining), remaining)
	}
}

// TC-P3-009: 자식 프로세스 비정상 종료 코드 전파 및 정리
func TestPhase3_Launcher_PropagatesNonZeroExitCodeAndCleansUp(t *testing.T) {
	isolateLauncherEnv(t)

	sessDir := t.TempDir()
	mgr := session.NewManager(sessDir)
	var stdout, stderr bytes.Buffer
	l := launcher.New(mgr, &stdout, &stderr)

	code := l.Run(core.ModeDev, t.TempDir(), []string{"sh", "-c", "exit 42"})
	if code != 42 {
		t.Errorf("Run() exit code = %d, want 42", code)
	}

	remaining, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected session cleanup even on non-zero exit, got %d remaining", len(remaining))
	}
}

// TC-P3-010: 비-TTY(bytes.Buffer) stdout에 ANSI OSC 타이틀 시퀀스 미혼입
func TestPhase3_Launcher_NonTTYStdoutHasNoANSIOscTitle(t *testing.T) {
	isolateLauncherEnv(t)

	mgr := session.NewManager(t.TempDir())
	var stdout, stderr bytes.Buffer
	l := launcher.New(mgr, &stdout, &stderr)

	code := l.Run(core.ModeTest, t.TempDir(), []string{"sh", "-c", "true"})
	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}

	if strings.Contains(stdout.String(), "\033]0;") {
		t.Errorf("non-TTY stdout must not contain ANSI OSC title sequence \\033]0;, got %q", stdout.String())
	}
}

// TC-P3-011: 배너 정책 경고 출력
func TestPhase3_Launcher_PrintBannerIncludesPolicyWarnings(t *testing.T) {
	var buf bytes.Buffer
	launcher.PrintBanner(&buf, launcher.BannerInfo{
		Mode:           core.ModeDev,
		SessionID:      "sess-banner-1",
		Workspace:      "/tmp/ws",
		PolicySource:   "builtin",
		PolicyWarnings: []string{"/tmp/ws/guard-policy.yaml: yaml parse error"},
		CmdName:        "sh",
		HasHook:        true,
	})

	out := buf.String()
	if !strings.Contains(out, "⚠️  [Policy Warning]") {
		t.Errorf("expected banner to contain policy warning header, got:\n%s", out)
	}
	if !strings.Contains(out, "/tmp/ws/guard-policy.yaml: yaml parse error") {
		t.Errorf("expected banner to contain warning detail, got:\n%s", out)
	}
}

// TC-P3-012: EnsureAgentSkill 최초 생성 및 기존 파일 보존
func TestPhase3_Launcher_EnsureAgentSkillCreatesOnceAndPreservesCustomContent(t *testing.T) {
	homeDir := t.TempDir()

	path1, created1, err := launcher.EnsureAgentSkill(homeDir)
	if err != nil {
		t.Fatalf("first EnsureAgentSkill: %v", err)
	}
	if !created1 {
		t.Errorf("expected created=true on first call")
	}
	if path1 == "" {
		t.Fatalf("expected non-empty skillPath")
	}

	custom := "# User Customized Guard Skill\nDo not overwrite me.\n"
	if err := os.WriteFile(path1, []byte(custom), 0o644); err != nil {
		t.Fatalf("WriteFile custom skill: %v", err)
	}

	path2, created2, err := launcher.EnsureAgentSkill(homeDir)
	if err != nil {
		t.Fatalf("second EnsureAgentSkill: %v", err)
	}
	if created2 {
		t.Errorf("expected created=false on second call when file already exists")
	}
	if path2 != path1 {
		t.Errorf("skillPath changed: %q -> %q", path1, path2)
	}

	gotBytes, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(gotBytes) != custom {
		t.Errorf("existing skill content was overwritten: got %q, want %q", string(gotBytes), custom)
	}
}

// TC-P3-013: EnsureWorkspaceHook 병합 보존 & 손상 JSON 덮어쓰기 방어
func TestPhase3_Launcher_EnsureWorkspaceHookMergeAndCorruptedJSONDefense(t *testing.T) {
	t.Run("fresh_workspace_creates_hook", func(t *testing.T) {
		ws := t.TempDir()
		if launcher.HasWorkspaceHook(ws) {
			t.Fatalf("expected HasWorkspaceHook=false on empty workspace")
		}

		hookPath, updated, err := launcher.EnsureWorkspaceHook(ws)
		if err != nil {
			t.Fatalf("EnsureWorkspaceHook: %v", err)
		}
		if !updated {
			t.Errorf("expected updated=true on fresh workspace")
		}
		if hookPath != filepath.Join(ws, ".agents", "hooks.json") {
			t.Errorf("hookPath = %q, want %q", hookPath, filepath.Join(ws, ".agents", "hooks.json"))
		}
		if !launcher.HasWorkspaceHook(ws) {
			t.Errorf("expected HasWorkspaceHook=true after EnsureWorkspaceHook")
		}
	})

	t.Run("merges_guard_while_preserving_existing_keys", func(t *testing.T) {
		ws := t.TempDir()
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		existing := `{"obsidian-chatlog":{"command":"chatlog","events":["PostTurn"]}}`
		if err := os.WriteFile(hookPath, []byte(existing), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, updated, err := launcher.EnsureWorkspaceHook(ws)
		if err != nil {
			t.Fatalf("EnsureWorkspaceHook: %v", err)
		}
		if !updated {
			t.Errorf("expected updated=true when adding guard key")
		}

		raw, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("Unmarshal merged hooks.json: %v", err)
		}
		if _, ok := parsed["guard"]; !ok {
			t.Errorf("expected 'guard' key in merged hooks.json, got %v", parsed)
		}
		chatlog, ok := parsed["obsidian-chatlog"].(map[string]any)
		if !ok || chatlog["command"] != "chatlog" {
			t.Errorf("expected 'obsidian-chatlog' key to be 100%% preserved, got %v", parsed["obsidian-chatlog"])
		}
	})

	t.Run("corrupted_json_returns_error_without_overwriting", func(t *testing.T) {
		ws := t.TempDir()
		hookPath := filepath.Join(ws, ".agents", "hooks.json")
		if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		broken := `{broken json content`
		if err := os.WriteFile(hookPath, []byte(broken), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		_, updated, err := launcher.EnsureWorkspaceHook(ws)
		if err == nil {
			t.Fatalf("expected error on corrupted hooks.json, got nil")
		}
		if updated {
			t.Errorf("expected updated=false on corrupted hooks.json")
		}

		after, readErr := os.ReadFile(hookPath)
		if readErr != nil {
			t.Fatalf("ReadFile: %v", readErr)
		}
		if string(after) != broken {
			t.Errorf("corrupted hooks.json must not be overwritten: got %q, want %q", string(after), broken)
		}
	})
}
