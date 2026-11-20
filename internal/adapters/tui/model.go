package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/hub/session"
	"guard/internal/i18n"
)

const pollInterval = 500 * time.Millisecond

type toastState struct {
	text  string
	isErr bool
	until time.Time
}

func (t *toastState) set(msg string, isErr bool) {
	t.text = msg
	t.isErr = isErr
	t.until = time.Now().Add(2500 * time.Millisecond)
}

func (t *toastState) clearIfExpired(now time.Time) {
	if t.text != "" && !t.until.IsZero() && now.After(t.until) {
		t.text = ""
	}
}

// Model holds all state for the Guard TUI Control Tower.
type Model struct {
	sessMgr    *session.Manager
	sessions   []*session.Session
	cursor     int
	selectedID string

	width  int
	height int
	now    time.Time

	toast toastState

	showModal bool
	showHelp  bool
	viewport  viewport.Model
}

type tickMsg time.Time

// NewModel initializes the TUI state model and performs an initial session load.
func NewModel(sessMgr *session.Manager) Model {
	if sessMgr == nil {
		sessMgr = session.NewManager("")
	}
	vp := viewport.New(80, 20)
	m := Model{
		sessMgr:  sessMgr,
		width:    100,
		height:   30,
		now:      time.Now(),
		viewport: vp,
	}
	m.reloadSessions()
	return m
}

// Init starts the 0.5-second polling ticker.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) selectedSession() *session.Session {
	if len(m.sessions) == 0 || m.cursor < 0 || m.cursor >= len(m.sessions) {
		return nil
	}
	return m.sessions[m.cursor]
}

func (m *Model) reloadSessions() {
	list, _, err := m.sessMgr.CleanAndList()
	if err != nil {
		m.setToast(i18n.T("tui.toast.load_failed", err.Error()), true)
		return
	}
	m.sessions = list

	if len(m.sessions) == 0 {
		m.cursor = 0
		m.selectedID = ""
		return
	}

	// Preserve cursor focus on the same session ID across refreshes when possible.
	if m.selectedID != "" {
		for i, s := range m.sessions {
			if s.ID == m.selectedID {
				m.cursor = i
				return
			}
		}
	}

	if m.cursor >= len(m.sessions) {
		m.cursor = len(m.sessions) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.selectedID = m.sessions[m.cursor].ID
}

func (m *Model) setToast(msg string, isErr bool) {
	m.toast.set(msg, isErr)
}

func (m *Model) syncViewportSize() {
	w := m.width - 6
	if w < 30 {
		w = 30
	}
	h := m.height - 8
	if h < 8 {
		h = 8
	}
	m.viewport.Width = w
	m.viewport.Height = h
}
