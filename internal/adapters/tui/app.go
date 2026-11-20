package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"guard/internal/hub/session"
)

// Start launches the Guard TUI Control Tower in alternate screen mode.
func Start(sessMgr *session.Manager) error {
	p := tea.NewProgram(NewModel(sessMgr), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
