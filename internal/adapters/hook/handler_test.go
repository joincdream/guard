package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"guard/internal/hub/session"
)

func TestHandler_DenyAndRecordEvent(t *testing.T) {
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("AGENT_MODE", "")

	sessDir := t.TempDir()
	mgr := session.NewManager(sessDir)

	if err := mgr.Create(&session.Session{
		ID:        "sess-hook-1",
		Mode:      "DEV",
		PID:       0,
		Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	var stdout, stderr bytes.Buffer
	h := NewHandler(mgr, t.TempDir(), &stdout, &stderr)

	payload := `{"sessionId":"sess-hook-1","tool_name":"read_file","tool_args":{"Path":"pkg/auth_test.go"}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal stdout: %v (raw: %q)", err, stdout.String())
	}
	if out.Decision != "deny" || out.Reason == "" {
		t.Fatalf("expected deny with reason, got %+v", out)
	}
	if strings.TrimSpace(stderr.String()) == "" {
		t.Errorf("expected violation reason written to stderr")
	}

	// Verify session hub recorded the event and incremented ViolationCount
	sess, err := mgr.Get("sess-hook-1")
	if err != nil {
		t.Fatalf("Get session: %v", err)
	}
	if sess.ViolationCount != 1 {
		t.Errorf("expected ViolationCount 1, got %d", sess.ViolationCount)
	}
	if len(sess.Events) != 1 || sess.Events[0].Decision != "deny" || sess.Events[0].Target != "pkg/auth_test.go" {
		t.Errorf("unexpected recorded events: %+v", sess.Events)
	}
}

func TestHandler_SessionModeOverridesWorkspace(t *testing.T) {
	t.Setenv("GUARD_SESSION_ID", "")
	t.Setenv("AGENT_MODE", "")

	sessDir := t.TempDir()
	mgr := session.NewManager(sessDir)

	// Switch session mode to ALL -> test file read should be allowed
	if err := mgr.Create(&session.Session{
		ID:        "sess-hook-2",
		Mode:      "ALL",
		PID:       0,
		Workspace: t.TempDir(),
	}); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	var stdout, stderr bytes.Buffer
	h := NewHandler(mgr, t.TempDir(), &stdout, &stderr)

	payload := `{"sessionId":"sess-hook-2","toolCall":{"name":"run_command","args":{"CommandLine":"cat pkg/auth_test.go"}}}`
	if err := h.Handle(strings.NewReader(payload)); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if out.Decision != "allow" {
		t.Fatalf("expected allow in ALL mode, got %+v", out)
	}
}
