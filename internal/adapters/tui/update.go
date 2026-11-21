package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/core"
	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

// Update processes keyboard interactions, window resizes, and 0.5-second polling ticks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncViewportSize()
		if m.showModal {
			m.viewport.SetContent(formatEventsFull(m.selectedSession(), m.viewport.Width))
		}
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		m.reloadSessions()
		m.reloadHandoffs()
		m.toast.clearIfExpired(m.now)
		if m.showModal {
			m.viewport.SetContent(formatEventsFull(m.selectedSession(), m.viewport.Width))
		}
		return m, tickCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Always allow quitting
	if key == "ctrl+c" || key == "q" || key == "Q" {
		return m, tea.Quit
	}

	// Help overlay toggle
	if key == "?" {
		m.showHelp = !m.showHelp
		if m.showHelp {
			m.showModal = false
		}
		return m, nil
	}

	// When help overlay is open, Esc closes it; ignore other keys
	if m.showHelp {
		if key == "esc" {
			m.showHelp = false
		}
		return m, nil
	}

	// Handoff reference viewer overlay takes precedence while active
	if m.handoffMode == handoffViewRefViewer {
		if key == "esc" || key == "v" || key == "V" {
			m.handoffMode = handoffViewSummary
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	// Handoff summary approval modal
	if m.handoffMode == handoffViewSummary && m.activeHandoff != nil {
		switch key {
		case "enter":
			m.approveActiveHandoff()
			return m, nil
		case "v", "V":
			m.openActiveHandoffRef()
			return m, nil
		case "esc":
			m.rejectActiveHandoff()
			return m, nil
		}
		return m, nil
	}

	// Escape closes event detail modal
	if key == "esc" {
		m.showModal = false
		return m, nil
	}

	// Space toggles the full event log viewport modal
	if key == " " {
		if m.showModal {
			m.showModal = false
			return m, nil
		}
		if sess := m.selectedSession(); sess != nil {
			m.syncViewportSize()
			m.viewport.SetContent(formatEventsFull(sess, m.viewport.Width))
			m.viewport.GotoTop()
			m.showModal = true
		} else {
			m.setToast(i18n.T("tui.toast.no_selected_session"), true)
		}
		return m, nil
	}

	// Delegate navigation keys to viewport while modal is open
	if m.showModal {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	switch key {
	case "up", "k":
		if len(m.sessions) > 0 && m.cursor > 0 {
			m.cursor--
			m.selectedID = m.sessions[m.cursor].ID
		}

	case "down", "j":
		if len(m.sessions) > 0 && m.cursor < len(m.sessions)-1 {
			m.cursor++
			m.selectedID = m.sessions[m.cursor].ID
		}

	case "d", "D":
		m.switchSelectedMode(core.ModeDev)

	case "t", "T":
		m.switchSelectedMode(core.ModeTest)

	case "p", "P":
		m.switchSelectedMode(core.ModePlan)

	case "a", "A":
		m.switchSelectedMode(core.ModeAll)

	case "c", "C":
		m.resetSelectedViolations()

	case "h", "H":
		m.showHandoffQueue = !m.showHandoffQueue

	case "r", "R":
		m.reloadSessions()
		m.reloadHandoffs()
		m.setToast(i18n.T("tui.toast.refreshed"), false)
	}

	return m, nil
}

func (m *Model) approveActiveHandoff() {
	if m.activeHandoff == nil || m.handoffMgr == nil {
		return
	}
	id := m.activeHandoff.ID
	res, err := handoff.ApproveAndRoute(m.handoffMgr, m.sessMgr, m.workspaceRoot, id)
	if err != nil {
		m.setToast(i18n.T("tui.handoff.toast.approve_failed", id, err), true)
		return
	}
	m.handoffMode = handoffViewNone
	m.activeHandoff = nil
	m.reloadSessions()
	m.reloadHandoffs()

	if res.Buffered {
		m.setToast(i18n.T("tui.handoff.toast.approved_buffered", id, res.Item.To.Mode), false)
		return
	}
	m.setToast(i18n.T("tui.handoff.toast.approved_delivered", id, session.ShortID(res.SessionID)), false)
}

func (m *Model) rejectActiveHandoff() {
	if m.activeHandoff == nil || m.handoffMgr == nil {
		return
	}
	id := m.activeHandoff.ID
	if _, err := m.handoffMgr.UpdateStatus(id, handoff.StatusRejected, "rejected via TUI"); err != nil {
		m.setToast(i18n.T("tui.handoff.toast.reject_failed", id, err), true)
		return
	}
	m.handoffMode = handoffViewNone
	m.activeHandoff = nil
	m.reloadHandoffs()
	m.setToast(i18n.T("tui.handoff.toast.rejected", id), false)
}

func (m *Model) openActiveHandoffRef() {
	if m.activeHandoff == nil {
		return
	}
	if len(m.activeHandoff.References) == 0 {
		m.setToast(i18n.T("tui.handoff.toast.no_refs"), true)
		return
	}
	ref := m.activeHandoff.References[0]
	content, err := loadReferenceContent(m.workspaceRoot, ref)
	if err != nil {
		m.setToast(i18n.T("tui.handoff.toast.ref_open_failed", ref, err), true)
		return
	}
	m.syncViewportSize()
	m.refViewerTitle = ref
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.handoffMode = handoffViewRefViewer
}

func (m *Model) switchSelectedMode(mode string) {
	sess := m.selectedSession()
	if sess == nil {
		m.setToast(i18n.T("tui.toast.no_session_for_mode"), true)
		return
	}
	if _, err := m.sessMgr.UpdateMode(sess.ID, mode); err != nil {
		m.setToast(i18n.T("tui.toast.mode_change_failed", session.ShortID(sess.ID), err), true)
		return
	}
	m.reloadSessions()
	m.setToast(i18n.T("tui.toast.mode_changed", session.ShortID(sess.ID), mode), false)
}

func (m *Model) resetSelectedViolations() {
	sess := m.selectedSession()
	if sess == nil {
		m.setToast(i18n.T("tui.toast.no_session_for_reset"), true)
		return
	}
	if _, err := m.sessMgr.ResetViolations(sess.ID); err != nil {
		m.setToast(i18n.T("tui.toast.reset_failed", err), true)
		return
	}
	m.reloadSessions()
	m.setToast(i18n.T("tui.toast.reset_success", session.ShortID(sess.ID)), false)
}
