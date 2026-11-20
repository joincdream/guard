package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"guard/internal/hub/session"
	"guard/internal/i18n"
)

func formatEventsFull(sess *session.Session, maxWidth int) string {
	if sess == nil {
		return emptyHintStyle.Render(i18n.T("tui.modal.no_session"))
	}
	if len(sess.Events) == 0 {
		return emptyHintStyle.Render(i18n.T("tui.modal.no_events"))
	}

	var lines []string
	// Display newest events first
	for i := len(sess.Events) - 1; i >= 0; i-- {
		ev := sess.Events[i]
		ts := ev.Timestamp.Format("15:04:05")
		badge := allowBadgeStyle.Render("[ALLOW]")
		if strings.EqualFold(ev.Decision, "deny") {
			badge = denyBadgeStyle.Render("[DENY ]")
		}
		tool := ev.Tool
		if tool == "" {
			tool = "-"
		}
		line := fmt.Sprintf("• %s %s %-11s -> %s", ts, badge, tool, ev.Target)
		lines = append(lines, line)
		if strings.EqualFold(ev.Decision, "deny") && strings.TrimSpace(ev.Reason) != "" {
			reasonText := ev.Reason
			if maxWidth > 20 {
				reasonText = wrapPrefix(reasonText, maxWidth-16)
			}
			lines = append(lines, reasonStyle.Render(i18n.T("tui.events.reason_prefix")+reasonText))
		}
	}
	return strings.Join(lines, "\n")
}

func renderModal(m Model) string {
	sess := m.selectedSession()
	title := i18n.T("tui.modal.title_default")
	if sess != nil {
		title = i18n.T("tui.modal.title",
			session.ShortID(sess.ID), sess.Workspace, len(sess.Events), session.MaxEvents)
	}

	scrollPct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
	header := panelTitleStyle.Render("📋 "+title) + "  " + headerMetaStyle.Render(i18n.T("tui.modal.scroll", scrollPct))
	footer := footerKeyStyle.Render("[↑/↓, j/k, PgUp/PgDn]") + footerDescStyle.Render(i18n.T("tui.modal.footer_scroll")) +
		footerKeyStyle.Render("[Space / Esc]") + footerDescStyle.Render(i18n.T("tui.modal.footer_close")) +
		footerKeyStyle.Render("[Q]") + footerDescStyle.Render(i18n.T("tui.modal.footer_quit"))

	boxWidth := m.width - 2
	if boxWidth < 40 {
		boxWidth = 40
	}

	body := lipgloss.JoinVertical(lipgloss.Left, header, "", m.viewport.View(), "", footer)
	return modalBorderStyle.Width(boxWidth).Render(body)
}

func renderHelp(m Model) string {
	var b strings.Builder
	b.WriteString(panelTitleStyle.Render(i18n.T("tui.help.title")))
	b.WriteString("\n\n")

	b.WriteString(tableHeaderStyle.Render(i18n.T("tui.help.keys_header")))
	b.WriteString("\n")
	keys := [][2]string{
		{"↑ / k, ↓ / j", i18n.T("tui.help.key.move")},
		{"D", i18n.T("tui.help.key.dev")},
		{"T", i18n.T("tui.help.key.test")},
		{"P", i18n.T("tui.help.key.plan")},
		{"A", i18n.T("tui.help.key.all")},
		{"C", i18n.T("tui.help.key.reset")},
		{"Space", i18n.T("tui.help.key.space")},
		{"r", i18n.T("tui.help.key.refresh")},
		{"? / Esc", i18n.T("tui.help.key.help")},
		{"q / Ctrl+C", i18n.T("tui.help.key.quit")},
	}
	for _, kv := range keys {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			footerKeyStyle.Width(14).Render(kv[0]),
			footerDescStyle.Render(kv[1]),
		))
	}

	b.WriteString("\n")
	b.WriteString(tableHeaderStyle.Render(i18n.T("tui.help.modes_header")))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n", renderModeBadge("DEV"), i18n.T("tui.help.mode.dev")))
	b.WriteString(fmt.Sprintf("  %s  %s\n", renderModeBadge("TEST"), i18n.T("tui.help.mode.test")))
	b.WriteString(fmt.Sprintf("  %s  %s\n", renderModeBadge("PLAN"), i18n.T("tui.help.mode.plan")))
	b.WriteString(fmt.Sprintf("  %s  %s\n", renderModeBadge("ALL"), i18n.T("tui.help.mode.all")))

	b.WriteString("\n")
	b.WriteString(footerKeyStyle.Render("[? / Esc]") + footerDescStyle.Render(i18n.T("tui.modal.footer_close")) +
		footerKeyStyle.Render("[Q]") + footerDescStyle.Render(i18n.T("tui.modal.footer_quit")))

	boxWidth := m.width - 4
	if boxWidth < 50 {
		boxWidth = 50
	}
	if boxWidth > 88 {
		boxWidth = 88
	}
	return modalBorderStyle.Width(boxWidth).Render(b.String())
}

func wrapPrefix(s string, maxLen int) string {
	if maxLen <= 10 || len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	var out []string
	for len(runes) > maxLen {
		out = append(out, string(runes[:maxLen]))
		runes = runes[maxLen:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return strings.Join(out, "\n                     ")
}
