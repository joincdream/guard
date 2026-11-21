package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"guard/internal/hub/session"
	"guard/internal/i18n"
)

// View renders the complete Guard TUI Control Tower screen.
func (m Model) View() string {
	if m.showHelp {
		return renderHelp(m)
	}
	if m.showModal {
		return renderModal(m)
	}
	if m.handoffMode == handoffViewRefViewer {
		return renderHandoffRefViewer(m)
	}
	if m.handoffMode == handoffViewSummary && m.activeHandoff != nil {
		return renderHandoffSummaryModal(m)
	}

	w := m.width
	if w < 60 {
		w = 60
	}
	h := m.height
	if h < 18 {
		h = 18
	}

	header := renderHeader(m, w)
	footer := renderFooter(m)

	// Allocate remaining vertical space between session table (max 40%) and event log panel.
	availHeight := h - 4 // header (1) + footer (2) + spacing (1)
	if availHeight < 10 {
		availHeight = 10
	}
	sessionPanelHeight := (availHeight * 40) / 100
	if sessionPanelHeight < 5 {
		sessionPanelHeight = 5
	}
	if maxNeeded := len(m.sessions) + 4; maxNeeded < sessionPanelHeight && maxNeeded >= 5 {
		sessionPanelHeight = maxNeeded
	}
	eventPanelHeight := availHeight - sessionPanelHeight
	if eventPanelHeight < 5 {
		eventPanelHeight = 5
	}

	sessionsPanel := renderSessionsPanel(m, w, sessionPanelHeight)
	var bottomPanel string
	if m.showHandoffQueue {
		bottomPanel = renderHandoffQueuePanel(m, w, eventPanelHeight)
	} else {
		bottomPanel = renderEventsPanel(m, w, eventPanelHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, sessionsPanel, bottomPanel, footer)
}

func renderHeader(m Model, width int) string {
	totalViolations := 0
	for _, s := range m.sessions {
		totalViolations += s.ViolationCount
	}

	left := titleStyle.Render("🛡️  GUARD CONTROL TOWER") + "   " +
		headerMetaStyle.Render(i18n.T("tui.header.meta",
			len(m.sessions), totalViolations, m.now.Format("15:04:05")))

	right := headerHintStyle.Render(i18n.T("tui.header.hint"))

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderSessionsPanel(m Model, width int, panelHeight int) string {
	innerWidth := width - 4
	if innerWidth < 50 {
		innerWidth = 50
	}

	var lines []string
	lines = append(lines, panelTitleStyle.Render(i18n.T("tui.sessions.title", len(m.sessions))))

	if len(m.sessions) == 0 {
		lines = append(lines, "")
		lines = append(lines, emptyHintStyle.Render(i18n.T("tui.sessions.empty")))
	} else {
		wsColWidth := innerWidth - 42
		if wsColWidth < 16 {
			wsColWidth = 16
		}

		hdr := fmt.Sprintf("  %s %s %s %s %s",
			padRight(i18n.T("tui.sessions.col.id"), 9),
			padRight(i18n.T("tui.sessions.col.mode"), 6),
			padRight(i18n.T("tui.sessions.col.workspace"), wsColWidth),
			padRight(i18n.T("tui.sessions.col.last_active"), 12),
			i18n.T("tui.sessions.col.deny"),
		)
		lines = append(lines, tableHeaderStyle.Render(hdr))

		maxRows := panelHeight - 4
		if maxRows < 1 {
			maxRows = 1
		}

		startIdx := 0
		if m.cursor >= maxRows {
			startIdx = m.cursor - maxRows + 1
		}
		endIdx := startIdx + maxRows
		if endIdx > len(m.sessions) {
			endIdx = len(m.sessions)
		}

		for i := startIdx; i < endIdx; i++ {
			s := m.sessions[i]
			cursorChar := " "
			if i == m.cursor {
				cursorChar = cursorMarkStyle.Render("●")
			}

			denyStr := i18n.T("tui.sessions.deny_count", s.ViolationCount)
			if s.ViolationCount > 0 {
				denyStr = denyCountStyle.Render(denyStr)
			} else {
				denyStr = zeroCountStyle.Render(denyStr)
			}

			wsText := shortenWorkspace(s.Workspace, wsColWidth)
			row := fmt.Sprintf("%s %s %s %s %s %s",
				cursorChar,
				padRight(session.ShortID(s.ID), 9),
				padRight(renderModeBadge(s.Mode), 6),
				padRight(wsText, wsColWidth),
				padRight(i18n.RelativeTime(s.UpdatedAt), 12),
				denyStr,
			)
			if i == m.cursor {
				row = selectedRowStyle.Width(innerWidth).Render(row)
			}
			lines = append(lines, row)
		}
	}

	content := strings.Join(lines, "\n")
	return panelBorderStyle.Width(width - 2).Height(panelHeight - 2).Render(content)
}

func renderEventsPanel(m Model, width int, panelHeight int) string {
	innerWidth := width - 4
	if innerWidth < 50 {
		innerWidth = 50
	}

	sess := m.selectedSession()
	var lines []string

	if sess == nil {
		lines = append(lines, panelTitleStyle.Render(i18n.T("tui.events.title_empty")))
		lines = append(lines, "")
		lines = append(lines, emptyHintStyle.Render(i18n.T("tui.events.no_session")))
	} else {
		title := i18n.T("tui.events.title",
			session.ShortID(sess.ID),
			shortenWorkspace(sess.Workspace, innerWidth-48),
			len(sess.Events),
			session.MaxEvents,
		)
		lines = append(lines, panelTitleStyle.Render(title))

		if len(sess.Events) == 0 {
			lines = append(lines, "")
			lines = append(lines, emptyHintStyle.Render(i18n.T("tui.events.empty")))
		} else {
			maxLines := panelHeight - 3
			if maxLines < 2 {
				maxLines = 2
			}
			usedLines := 0
			for i := len(sess.Events) - 1; i >= 0 && usedLines < maxLines; i-- {
				ev := sess.Events[i]
				ts := ev.Timestamp.Format("15:04:05")
				badge := allowBadgeStyle.Render("[ALLOW]")
				isDeny := strings.EqualFold(ev.Decision, "deny")
				if isDeny {
					badge = denyBadgeStyle.Render("[DENY ]")
				}
				tool := ev.Tool
				if tool == "" {
					tool = "-"
				}
				target := truncateText(ev.Target, innerWidth-36)
				lines = append(lines, fmt.Sprintf("• %s %s %-11s -> %s", ts, badge, tool, target))
				usedLines++

				if isDeny && strings.TrimSpace(ev.Reason) != "" && usedLines < maxLines {
					reason := truncateText(ev.Reason, innerWidth-20)
					lines = append(lines, reasonStyle.Render(i18n.T("tui.events.reason_prefix")+reason))
					usedLines++
				}
			}
		}
	}

	content := strings.Join(lines, "\n")
	return panelBorderStyle.Width(width - 2).Height(panelHeight - 2).Render(content)
}

func renderFooter(m Model) string {
	toastLine := ""
	if m.toast.text != "" {
		if m.toast.isErr {
			toastLine = "  " + toastErrStyle.Render(m.toast.text)
		} else {
			toastLine = "  " + toastStyle.Render(m.toast.text)
		}
	}

	bar := "  " +
		footerKeyStyle.Render("[↑/↓]") + footerDescStyle.Render(i18n.T("tui.footer.select")) +
		footerKeyStyle.Render("[D]") + footerDescStyle.Render(" DEV │ ") +
		footerKeyStyle.Render("[T]") + footerDescStyle.Render(" TEST │ ") +
		footerKeyStyle.Render("[P]") + footerDescStyle.Render(" PLAN │ ") +
		footerKeyStyle.Render("[A]") + footerDescStyle.Render(" ALL │ ") +
		footerKeyStyle.Render("[C]") + footerDescStyle.Render(i18n.T("tui.footer.reset")) +
		footerKeyStyle.Render("[Space]") + footerDescStyle.Render(i18n.T("tui.footer.full_log")) +
		footerKeyStyle.Render("[H]") + footerDescStyle.Render(i18n.T("tui.footer.handoff")) +
		footerKeyStyle.Render("[r]") + footerDescStyle.Render(i18n.T("tui.footer.refresh")) +
		footerKeyStyle.Render("[Q]") + footerDescStyle.Render(i18n.T("tui.footer.quit"))

	return toastLine + "\n" + bar
}

func shortenWorkspace(ws string, maxLen int) string {
	if maxLen < 8 {
		maxLen = 8
	}
	if lipgloss.Width(ws) <= maxLen {
		return ws
	}
	runes := []rune(ws)
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := "..." + string(runes[i:])
		if lipgloss.Width(candidate) > maxLen {
			if i+1 < len(runes) {
				return "..." + string(runes[i+1:])
			}
			return candidate
		}
	}
	return ws
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if maxLen < 8 {
		maxLen = 8
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
