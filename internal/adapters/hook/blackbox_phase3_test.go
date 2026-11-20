package hook_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"guard/internal/adapters/hook"
	"guard/internal/core"
	"guard/internal/hub/session"
)

func isolateHookEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("AGENT_MODE", "")
	t.Setenv("AGENT_ROLE", "")
	t.Setenv("GUARD_POLICY_PATH", "")
	t.Setenv("GUARD_DEBUG", "")
	t.Setenv("SESSION_ID", "")
	t.Setenv("CONVERSATION_ID", "")
}

func decodeOutput(t *testing.T, buf *bytes.Buffer) hook.Output {
	t.Helper()
	var out hook.Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to unmarshal stdout JSON: %v (raw: %q)", err, buf.String())
	}
	return out
}

// TC-P3-001: Fail-open 계약 (빈 입력 / 공백 / 깨진 JSON / null)
func TestPhase3_Hook_FailOpenOnInvalidInput(t *testing.T) {
	isolateHookEnv(t)

	inputs := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "whitespace", body: "   \n\t  "},
		{name: "malformed_json", body: "{invalid"},
		{name: "null_json", body: "null"},
	}

	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			mgr := session.NewManager(t.TempDir())
			var stdout, stderr bytes.Buffer
			h := hook.NewHandler(mgr, t.TempDir(), &stdout, &stderr)

			if err := h.Handle(strings.NewReader(tc.body)); err != nil {
				t.Fatalf("Handle(%q) returned error %v, want nil (fail-open)", tc.body, err)
			}
			out := decodeOutput(t, &stdout)
			if out.Decision != "allow" || out.Reason != "" {
				t.Errorf("expected {Decision:allow Reason:\"\"}, got %+v", out)
			}
			if strings.TrimSpace(stderr.String()) != "" {
				t.Errorf("expected empty stderr on fail-open allow, got %q", stderr.String())
			}
		})
	}
}

// TC-P3-002: 다중 입력 JSON 스키마 호환성
func TestPhase3_Hook_MultiSchemaInputCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantTarget string
	}{
		{
			name:       "toolCall_with_sessionId",
			payload:    `{"sessionId":"sess-dev","toolCall":{"name":"read_file","args":{"Path":"pkg/auth_test.go"}}}`,
			wantTarget: "pkg/auth_test.go",
		},
		{
			name:       "tool_name_with_session_id",
			payload:    `{"session_id":"sess-dev","tool_name":"read_file","tool_args":{"path":"pkg/user_test.go"}}`,
			wantTarget: "pkg/user_test.go",
		},
		{
			name:       "top_level_Path_with_conversationId",
			payload:    `{"conversationId":"sess-dev","tool_name":"read_file","Path":"pkg/order_test.go"}`,
			wantTarget: "pkg/order_test.go",
		},
		{
			name:       "top_level_path_lower",
			payload:    `{"sessionId":"sess-dev","tool_name":"read_file","path":"pkg/pay_test.go"}`,
			wantTarget: "pkg/pay_test.go",
		},
		{
			name:       "top_level_CommandLine",
			payload:    `{"sessionId":"sess-dev","tool_name":"run_command","CommandLine":"cat pkg/cmd_test.go"}`,
			wantTarget: "cat pkg/cmd_test.go",
		},
		{
			name:       "top_level_command_line_lower",
			payload:    `{"sessionId":"sess-dev","tool_name":"run_command","command_line":"head pkg/cmd2_test.go"}`,
			wantTarget: "head pkg/cmd2_test.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolateHookEnv(t)

			mgr := session.NewManager(t.TempDir())
			ws := t.TempDir()
			if err := mgr.Create(&session.Session{
				ID:        "sess-dev",
				Mode:      core.ModeDev,
				PID:       os.Getpid(),
				Workspace: ws,
			}); err != nil {
				t.Fatalf("Create session: %v", err)
			}

			var stdout, stderr bytes.Buffer
			h := hook.NewHandler(mgr, ws, &stdout, &stderr)
			if err := h.Handle(strings.NewReader(tc.payload)); err != nil {
				t.Fatalf("Handle: %v", err)
			}

			out := decodeOutput(t, &stdout)
			if out.Decision != "deny" || strings.TrimSpace(out.Reason) == "" {
				t.Fatalf("expected deny with reason, got %+v", out)
			}

			sess, err := mgr.Get("sess-dev")
			if err != nil {
				t.Fatalf("Get session: %v", err)
			}
			if sess.ViolationCount != 1 {
				t.Errorf("ViolationCount = %d, want 1", sess.ViolationCount)
			}
			if len(sess.Events) != 1 || sess.Events[0].Decision != "deny" || sess.Events[0].Target != tc.wantTarget {
				t.Errorf("recorded Events = %+v, want single deny event with Target=%q", sess.Events, tc.wantTarget)
			}
		})
	}
}

// TC-P3-003: 세션 해석 우선순위 & 미존재 GUARD_SESSION_ID 우회 (DEF-001 회귀 방지)
func TestPhase3_Hook_NonexistentOuterEnvSessionFallsThroughToPayloadSession(t *testing.T) {
	isolateHookEnv(t)

	// Simulate outer guard TEST launcher env leaking into inner process
	t.Setenv("GUARD_SESSION_ID", "nonexistent-outer-session")
	t.Setenv("AGENT_MODE", core.ModeTest)

	mgr := session.NewManager(t.TempDir())
	ws := t.TempDir()
	if err := mgr.Create(&session.Session{
		ID:        "real-dev-sess",
		Mode:      core.ModeDev,
		PID:       os.Getpid(),
		Workspace: ws,
	}); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	var stdout, stderr bytes.Buffer
	h := hook.NewHandler(mgr, ws, &stdout, &stderr)

	// In TEST mode reading *_test.go is allowed, but in DEV mode it MUST be denied.
	payload := `{"sessionId":"real-dev-sess","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeOutput(t, &stdout)
	if out.Decision != "deny" || strings.TrimSpace(out.Reason) == "" {
		t.Fatalf("expected DEV session to deny *_test.go read (not overridden by outer TEST env), got %+v", out)
	}

	sess, err := mgr.Get("real-dev-sess")
	if err != nil {
		t.Fatalf("Get(real-dev-sess): %v", err)
	}
	if sess.ViolationCount != 1 || len(sess.Events) != 1 {
		t.Errorf("expected event recorded on real-dev-sess, got ViolationCount=%d Events=%+v", sess.ViolationCount, sess.Events)
	}
}

// TC-P3-004: 미등록 세션 폴백 & PID: 0 비생성 계약
func TestPhase3_Hook_UntrackedSessionDoesNotCreateUnboundFile(t *testing.T) {
	isolateHookEnv(t)
	t.Setenv("AGENT_MODE", core.ModeDev)

	sessDir := t.TempDir()
	mgr := session.NewManager(sessDir)
	ws := t.TempDir()

	var stdout, stderr bytes.Buffer
	h := hook.NewHandler(mgr, ws, &stdout, &stderr)

	payload := `{"sessionId":"untracked-session-xyz","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeOutput(t, &stdout)
	if out.Decision != "deny" || strings.TrimSpace(out.Reason) == "" {
		t.Fatalf("expected fallback DEV mode to deny *_test.go read, got %+v", out)
	}

	// Must NOT create any PID: 0 session file in sessionMgr
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 sessions in sessionMgr (no auto-created PID:0 session), got %d: %+v", len(list), list)
	}
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty session dir, found files: %v", entries)
	}
}

// TC-P3-005: 차단 시 stdout JSON + stderr 1줄 사유 출력 계약
func TestPhase3_Hook_DenyWritesReasonToStdoutAndStderr(t *testing.T) {
	isolateHookEnv(t)
	t.Setenv("AGENT_MODE", core.ModeDev)

	mgr := session.NewManager(t.TempDir())
	ws := t.TempDir()
	var stdout, stderr bytes.Buffer
	h := hook.NewHandler(mgr, ws, &stdout, &stderr)

	payload := `{"tool_name":"read_file","tool_args":{"Path":"pkg/secret_test.go"}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	out := decodeOutput(t, &stdout)
	if out.Decision != "deny" || strings.TrimSpace(out.Reason) == "" {
		t.Fatalf("expected deny with non-empty reason, got %+v", out)
	}

	stderrTrimmed := strings.TrimSpace(stderr.String())
	if stderrTrimmed == "" {
		t.Fatalf("expected denial reason written to stderr, got empty")
	}
	if !strings.Contains(stderrTrimmed, out.Reason) {
		t.Errorf("stderr %q does not contain output reason %q", stderrTrimmed, out.Reason)
	}
	if strings.Contains(stderrTrimmed, "\n") {
		t.Errorf("expected single-line reason on stderr (with GUARD_DEBUG unset), got multiple lines: %q", stderrTrimmed)
	}
}

// TC-P3-006: 진단 로그(GUARD_DEBUG) 토글
func TestPhase3_Hook_DebugLogToggle(t *testing.T) {
	t.Run("GUARD_DEBUG_unset_produces_no_stderr_on_allow", func(t *testing.T) {
		isolateHookEnv(t)
		t.Setenv("AGENT_MODE", core.ModeAll)

		mgr := session.NewManager(t.TempDir())
		var stdout, stderr bytes.Buffer
		h := hook.NewHandler(mgr, t.TempDir(), &stdout, &stderr)

		payload := `{"tool_name":"read_file","tool_args":{"Path":"pkg/main.go"}}`
		if err := h.Handle(strings.NewReader(payload)); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		out := decodeOutput(t, &stdout)
		if out.Decision != "allow" {
			t.Fatalf("expected allow, got %+v", out)
		}
		if stderr.Len() != 0 {
			t.Errorf("expected empty stderr when GUARD_DEBUG is unset, got %q", stderr.String())
		}
	})

	t.Run("GUARD_DEBUG_1_writes_debug_prefix_to_stderr", func(t *testing.T) {
		isolateHookEnv(t)
		t.Setenv("AGENT_MODE", core.ModeAll)
		t.Setenv("GUARD_DEBUG", "1")

		mgr := session.NewManager(t.TempDir())
		var stdout, stderr bytes.Buffer
		h := hook.NewHandler(mgr, t.TempDir(), &stdout, &stderr)

		payload := `{"tool_name":"read_file","tool_args":{"Path":"pkg/main.go"}}`
		if err := h.Handle(strings.NewReader(payload)); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		out := decodeOutput(t, &stdout)
		if out.Decision != "allow" {
			t.Fatalf("expected allow, got %+v", out)
		}
		if !strings.Contains(stderr.String(), "[guard:debug]") {
			t.Errorf("expected [guard:debug] in stderr when GUARD_DEBUG=1, got %q", stderr.String())
		}
	})
}

// Sanity check: workspace .agents/current_mode fallback when no session & no env
func TestPhase3_Hook_WorkspaceCurrentModeFallback(t *testing.T) {
	isolateHookEnv(t)

	ws := t.TempDir()
	agentsDir := filepath.Join(ws, ".agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "current_mode"), []byte("ALL\n"), 0o644); err != nil {
		t.Fatalf("WriteFile current_mode: %v", err)
	}

	mgr := session.NewManager(t.TempDir())
	var stdout, stderr bytes.Buffer
	h := hook.NewHandler(mgr, ws, &stdout, &stderr)

	payload := `{"tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	out := decodeOutput(t, &stdout)
	if out.Decision != "allow" {
		t.Errorf("expected allow via .agents/current_mode=ALL fallback, got %+v", out)
	}
}
