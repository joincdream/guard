package tui_test

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/adapters/tui"
	"guard/internal/core"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

func sendKey(m tui.Model, msg tea.KeyMsg) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// TC-P4-005: 빈 세션 목록에서 모든 조작 키 입력 시 패닉 방어
func TestPhase4_TUI_EmptySessionListHandlesAllKeysSafely(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	mgr := session.NewManager(t.TempDir())
	m := tui.NewModel(mgr)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.Model)

	keys := []tea.KeyMsg{
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		runeKey('k'),
		runeKey('j'),
		runeKey('d'),
		runeKey('D'),
		runeKey('t'),
		runeKey('T'),
		runeKey('p'),
		runeKey('P'),
		runeKey('a'),
		runeKey('A'),
		runeKey('c'),
		runeKey('C'),
		{Type: tea.KeySpace},
		{Type: tea.KeyEsc},
		runeKey('?'),
		{Type: tea.KeyEsc},
		runeKey('r'),
		runeKey('R'),
	}

	for _, k := range keys {
		m, _ = sendKey(m, k)
		if view := m.View(); strings.TrimSpace(view) == "" {
			t.Errorf("View() returned empty string after key %v on empty list", k)
		}
	}
}

// TC-P4-006: 커서 이동 및 모드 전환(대/소문자) / 차단 리셋 영속 반영
func TestPhase4_TUI_CursorNavigationModeSwitchAndResetPersist(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("en")

	mgr := session.NewManager(t.TempDir())
	now := time.Now()

	// Create 2 active sessions with deterministic UpdatedAt order (sess-first is newer -> index 0, sess-second -> index 1)
	if err := mgr.Save(&session.Session{
		ID:             "sess-first",
		Mode:           core.ModeDev,
		PID:            os.Getpid(),
		Workspace:      "/ws/first",
		CreatedAt:      now,
		UpdatedAt:      now,
		ViolationCount: 3,
		Events:         []session.Event{},
	}); err != nil {
		t.Fatalf("Save sess-first: %v", err)
	}
	if err := mgr.Save(&session.Session{
		ID:             "sess-second",
		Mode:           core.ModeDev,
		PID:            os.Getpid(),
		Workspace:      "/ws/second",
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now.Add(-time.Minute),
		ViolationCount: 5,
		Events:         []session.Event{},
	}); err != nil {
		t.Fatalf("Save sess-second: %v", err)
	}

	m := tui.NewModel(mgr)

	// Move cursor down to index 1 (sess-second), and test lower bound clamping with extra down presses
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = sendKey(m, runeKey('j')) // clamp at last item

	// Switch sess-second mode with uppercase keys T -> P -> A -> D
	for _, tc := range []struct {
		key  rune
		want string
	}{
		{'T', core.ModeTest},
		{'P', core.ModePlan},
		{'A', core.ModeAll},
		{'D', core.ModeDev},
	} {
		m, _ = sendKey(m, runeKey(tc.key))
		s2, err := mgr.Get("sess-second")
		if err != nil {
			t.Fatalf("Get(sess-second): %v", err)
		}
		if s2.Mode != tc.want {
			t.Errorf("after key %q: sess-second.Mode = %q, want %q", tc.key, s2.Mode, tc.want)
		}
	}

	// Reset violations on sess-second with 'C'
	m, _ = sendKey(m, runeKey('C'))
	s2, err := mgr.Get("sess-second")
	if err != nil {
		t.Fatalf("Get(sess-second): %v", err)
	}
	if s2.ViolationCount != 0 {
		t.Errorf("sess-second.ViolationCount = %d after 'C', want 0", s2.ViolationCount)
	}

	// Verify sess-first was NOT mutated
	s1, err := mgr.Get("sess-first")
	if err != nil {
		t.Fatalf("Get(sess-first): %v", err)
	}
	if s1.ViolationCount != 3 || s1.Mode != core.ModeDev {
		t.Errorf("sess-first unexpectedly modified: Mode=%q ViolationCount=%d", s1.Mode, s1.ViolationCount)
	}
}

// TC-P4-007: 모달/도움말 토글, 수동 새로고침(r), 종료(q/Q/Ctrl+C)
func TestPhase4_TUI_ModalsRefreshAndQuitCommands(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)
	i18n.SetLang("ko")

	mgr := session.NewManager(t.TempDir())
	if err := mgr.Create(&session.Session{
		ID:        "sess-modal-1",
		Mode:      core.ModeDev,
		PID:       os.Getpid(),
		Workspace: "/ws/modal",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	m := tui.NewModel(mgr)
	baseView := m.View()

	// Space opens modal, Esc closes it
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeySpace})
	modalView := m.View()
	if modalView == baseView {
		t.Errorf("expected View() to change when event log modal is opened via Space")
	}
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if afterEsc := m.View(); afterEsc != baseView {
		t.Errorf("expected View() to return to main view after closing modal with Esc")
	}

	// '?' opens help overlay, Esc closes it
	m, _ = sendKey(m, runeKey('?'))
	helpView := m.View()
	if helpView == baseView {
		t.Errorf("expected View() to change when help overlay is opened via '?'")
	}
	m, _ = sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if afterEsc := m.View(); afterEsc != baseView {
		t.Errorf("expected View() to return to main view after closing help with Esc")
	}

	// Add a new session externally and press 'r' to refresh
	if err := mgr.Create(&session.Session{
		ID:        "sess-refreshed",
		Mode:      core.ModeTest,
		PID:       os.Getpid(),
		Workspace: "/ws/refreshed",
	}); err != nil {
		t.Fatalf("Create second session: %v", err)
	}
	m, _ = sendKey(m, runeKey('r'))
	if !strings.Contains(m.View(), session.ShortID("sess-refreshed")) {
		t.Errorf("expected refreshed View() to show newly created session %q", session.ShortID("sess-refreshed"))
	}

	// Quit keys (q, Q, Ctrl+C) must return tea.Quit command
	quitKeys := []tea.KeyMsg{
		runeKey('q'),
		runeKey('Q'),
		{Type: tea.KeyCtrlC},
	}
	for _, qk := range quitKeys {
		_, cmd := sendKey(m, qk)
		if cmd == nil {
			t.Errorf("expected non-nil Quit cmd for key %v", qk)
			continue
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg from cmd for key %v, got %T", qk, cmd())
		}
	}
}

// TC-P4-008: 좁은 터미널(width=80)에서 한글/영문 5컬럼 1줄 렌더링 불변성 및 PID/Active 미노출
func TestPhase4_TUI_NarrowTerminalSingleLineAndNoPIDOrActive(t *testing.T) {
	prevLang := i18n.Lang()
	defer i18n.SetLang(prevLang)

	for _, lang := range []string{"en", "ko"} {
		t.Run("lang_"+lang, func(t *testing.T) {
			i18n.SetLang(lang)

			mgr := session.NewManager(t.TempDir())
			if err := mgr.Create(&session.Session{
				ID:        "sess-narrow-1234",
				Mode:      core.ModePlan,
				PID:       os.Getpid(),
				Workspace: "/very/long/workspace/path/for/testing/narrow/terminal/rendering/guard",
			}); err != nil {
				t.Fatalf("Create: %v", err)
			}

			m := tui.NewModel(mgr)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = updated.(tui.Model)

			view := m.View()
			if strings.Contains(view, "PID") {
				t.Errorf("View() must not expose removed 'PID' column/text in %s:\n%s", lang, view)
			}

			shortID := session.ShortID("sess-narrow-1234")
			foundHeader := false
			foundRow := false
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "ID") && (strings.Contains(line, "Workspace") || strings.Contains(line, "워크스페이스")) {
					foundHeader = true
					if strings.Contains(line, "Status") || strings.Contains(line, "상태") || strings.Contains(line, "PID") {
						t.Errorf("table header must not contain removed PID/Status columns (lang=%s):\nline: %q", lang, line)
					}
				}
				if strings.Contains(line, shortID) && strings.Contains(line, "●") {
					foundRow = true
					if !strings.Contains(line, core.ModePlan) {
						t.Errorf("session row wrapped or missing Mode %q on same line as %q (lang=%s):\nline: %q", core.ModePlan, shortID, lang, line)
					}
					if strings.Contains(line, "Active") {
						t.Errorf("session data row must not expose removed 'Active' status text (lang=%s):\nline: %q", lang, line)
					}
				}
			}
			if !foundHeader {
				t.Errorf("session table header not found in View() (lang=%s):\n%s", lang, view)
			}
			if !foundRow {
				t.Errorf("session row with ID %q not found in View() (lang=%s):\n%s", shortID, lang, view)
			}
		})
	}
}
