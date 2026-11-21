package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
)

// Start launches the Guard TUI Control Tower in alternate screen mode.
func Start(sessMgr *session.Manager) error {
	return StartWithHandoff(sessMgr, nil, "")
}

// StartWithHandoff launches the Guard TUI Control Tower with explicit handoff manager and workspace root.
func StartWithHandoff(sessMgr *session.Manager, handoffMgr *handoff.Manager, workspaceRoot string) error {
	p := tea.NewProgram(NewModelWithHandoff(sessMgr, handoffMgr, workspaceRoot), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
