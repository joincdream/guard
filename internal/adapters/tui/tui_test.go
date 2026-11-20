package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/hub/session"
)

func TestModel_ModeSwitchResetAndModals(t *testing.T) {
	dir := t.TempDir()
	mgr := session.NewManager(dir)

	if err := mgr.Create(&session.Session{
		ID:        "sess-tui-1",
		Mode:      "DEV",
		PID:       os.Getpid(),
		Workspace: "/home/user/project/guard",
	}); err != nil {
		t.Fatalf("Create session: %v", err)
	}
	_, _ = mgr.AppendEvent("sess-tui-1", session.Event{
		Timestamp: time.Now(),
		Decision:  "deny",
		Tool:      "read_file",
		Target:    "internal/core/evaluator_test.go",
		Reason:    "[POLICY VIOLATION] DEV blocked",
	})

	m := NewModel(mgr)

	// Verify initial main View renders session and event
	view := m.View()
	if !strings.Contains(view, "GUARD CONTROL TOWER") || !strings.Contains(view, "sess-tui") || !strings.Contains(view, "evaluator_test.go") {
		t.Fatalf("unexpected initial view:\n%s", view)
	}

	// Press 't' to switch mode to TEST
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	sess, err := mgr.Get("sess-tui-1")
	if err != nil || sess.Mode != "TEST" {
		t.Fatalf("expected session mode TEST after pressing 't', got %+v (err=%v)", sess, err)
	}

	// Press 'p' -> PLAN, 'a' -> ALL, 'd' -> DEV
	for _, tc := range []struct {
		key  rune
		want string
	}{
		{'p', "PLAN"},
		{'a', "ALL"},
		{'d', "DEV"},
	} {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		m = u.(Model)
		s, _ := mgr.Get("sess-tui-1")
		if s.Mode != tc.want {
			t.Errorf("after pressing %q expected mode %q, got %q", tc.key, tc.want, s.Mode)
		}
	}

	// Press 'c' to reset violation count
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	sess, _ = mgr.Get("sess-tui-1")
	if sess.ViolationCount != 0 {
		t.Errorf("expected ViolationCount 0 after pressing 'c', got %d", sess.ViolationCount)
	}

	// Press Space to open full event log modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if !m.showModal {
		t.Fatalf("expected showModal=true after Space")
	}
	modalView := m.View()
	if !strings.Contains(modalView, "전체 이벤트 로그") {
		t.Errorf("expected modal title in view, got:\n%s", modalView)
	}

	// Press Esc to close modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showModal {
		t.Fatalf("expected showModal=false after Esc")
	}

	// Press '?' to open help overlay
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.showHelp {
		t.Fatalf("expected showHelp=true after '?'")
	}
	if !strings.Contains(m.View(), "도움말") {
		t.Errorf("expected help overlay view")
	}
}
