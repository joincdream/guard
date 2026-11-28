package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"guard/internal/hub/handoff"
	"guard/internal/hub/session"
	"guard/internal/i18n"
)

const maxRefViewerBytes = 256 * 1024 // 256KB safety limit to prevent TUI freeze

func loadReferenceContent(workspace, refPath string) (string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(refPath), "@")
	if trimmed == "" {
		return "", fmt.Errorf("empty reference path")
	}

	resolved := trimmed
	if !filepath.IsAbs(resolved) && strings.TrimSpace(workspace) != "" {
		resolved = filepath.Join(workspace, resolved)
	}

	f, err := os.Open(resolved)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(f, maxRefViewerBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}

	truncated := len(data) > maxRefViewerBytes
	if truncated {
		data = data[:maxRefViewerBytes]
	}

	content := string(data)
	if truncated {
		content += "\n\n" + i18n.T("tui.handoff.ref_truncated")
	}
	return content, nil
}

func renderHandoffSummaryModal(m Model) string {
	item := m.activeHandoff
	if item == nil {
		return ""
	}

	route := fmt.Sprintf("%s -> %s", formatTUINode(item.From), formatTUINode(item.To))
	header := fmt.Sprintf("🔔 %s %s  %s",
		renderHandoffTypeBadge(item.Type),
		panelTitleStyle.Render(route),
		headerMetaStyle.Render("(#"+item.ID+")"),
	)

	boxWidth := m.width - 4
	if boxWidth < 56 {
		boxWidth = 56
	}
	if boxWidth > 96 {
		boxWidth = 96
	}
	innerWidth := boxWidth - 6

	var bodyLines []string
	bodyLines = append(bodyLines, header, "")

	summaryLines := wrapLines(item.Summary, innerWidth-12, "    ")
	if len(summaryLines) > 0 {
		bodyLines = append(bodyLines, fmt.Sprintf("• %s: %s",
			tableHeaderStyle.Render(i18n.T("tui.handoff.field.summary")),
			summaryLines[0],
		))
		for _, extra := range summaryLines[1:] {
			bodyLines = append(bodyLines, extra)
		}
	}

	if payload := strings.TrimSpace(item.Payload); payload != "" {
		bodyLines = append(bodyLines, fmt.Sprintf("• %s:",
			tableHeaderStyle.Render(i18n.T("tui.handoff.field.payload")),
		))
		wrapped := wrapLines(payload, innerWidth-4, "    ")
		const maxPayloadLines = 12
		if len(wrapped) > maxPayloadLines {
			for _, ln := range wrapped[:maxPayloadLines] {
				bodyLines = append(bodyLines, "    "+ln)
			}
			bodyLines = append(bodyLines, headerMetaStyle.Render(fmt.Sprintf("    ... (+%d lines)", len(wrapped)-maxPayloadLines)))
		} else {
			for _, ln := range wrapped {
				bodyLines = append(bodyLines, "    "+ln)
			}
		}
	}

	if len(item.References) > 0 {
		bodyLines = append(bodyLines, fmt.Sprintf("• %s: %s",
			tableHeaderStyle.Render(i18n.T("tui.handoff.field.refs")),
			strings.Join(item.References, ", "),
		))
	}

	if len(item.Targets) > 0 {
		lockNote := ""
		if handoff.ShouldApplyScopeLock(*item, item.To.Mode) {
			lockNote = " " + headerMetaStyle.Render(i18n.T("tui.handoff.scope_lock_hint"))
		}
		bodyLines = append(bodyLines, fmt.Sprintf("• %s: %s%s",
			tableHeaderStyle.Render(i18n.T("tui.handoff.field.targets")),
			strings.Join(item.Targets, ", "),
			lockNote,
		))
	}

	if item.ParentID != "" {
		bodyLines = append(bodyLines, fmt.Sprintf("• %s: %s",
			tableHeaderStyle.Render(i18n.T("tui.handoff.field.parent")),
			item.ParentID,
		))
	}

	footer := footerKeyStyle.Render("[Enter]") + footerDescStyle.Render(i18n.T("tui.handoff.key.approve")) +
		footerKeyStyle.Render("[V]") + footerDescStyle.Render(i18n.T("tui.handoff.key.view_ref")) +
		footerKeyStyle.Render("[X/C]") + footerDescStyle.Render(i18n.T("tui.handoff.key.reject")) +
		footerKeyStyle.Render("[Esc]") + footerDescStyle.Render(i18n.T("tui.handoff.key.dismiss"))

	bodyLines = append(bodyLines, "", footer)

	return handoffModalBorderStyle.Width(boxWidth).Render(strings.Join(bodyLines, "\n"))
}

func wrapLines(text string, maxLen int, contIndent string) []string {
	if maxLen < 20 {
		maxLen = 20
	}
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string
	for lineIdx, raw := range rawLines {
		runes := []rune(raw)
		if len(runes) == 0 {
			if lineIdx > 0 {
				out = append(out, "")
			}
			continue
		}
		firstChunk := true
		for len(runes) > maxLen {
			prefix := ""
			if !firstChunk || lineIdx > 0 {
				prefix = contIndent
			}
			out = append(out, prefix+string(runes[:maxLen]))
			runes = runes[maxLen:]
			firstChunk = false
		}
		prefix := ""
		if !firstChunk {
			prefix = contIndent
		}
		out = append(out, prefix+string(runes))
	}
	return out
}

func renderHandoffRefViewer(m Model) string {
	scrollPct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
	refBadge := ""
	hasMultiRefs := m.activeHandoff != nil && len(m.activeHandoff.References) > 1
	if hasMultiRefs {
		refBadge = fmt.Sprintf("[%d/%d] ", m.refIndex+1, len(m.activeHandoff.References))
	}
	header := panelTitleStyle.Render("📄 "+refBadge+m.refViewerTitle) + "  " +
		headerMetaStyle.Render(i18n.T("tui.modal.scroll", scrollPct))
	switchHint := ""
	if hasMultiRefs {
		switchHint = footerKeyStyle.Render("[Tab / ←/→]") + footerDescStyle.Render(i18n.T("tui.handoff.key.switch_ref"))
	}
	footer := footerKeyStyle.Render("[↑/↓, j/k, PgUp/PgDn]") + footerDescStyle.Render(i18n.T("tui.modal.footer_scroll")) +
		switchHint +
		footerKeyStyle.Render("[Esc / V]") + footerDescStyle.Render(i18n.T("tui.handoff.key.back")) +
		footerKeyStyle.Render("[Q]") + footerDescStyle.Render(i18n.T("tui.modal.footer_quit"))

	boxWidth := m.width - 2
	if boxWidth < 40 {
		boxWidth = 40
	}

	body := lipgloss.JoinVertical(lipgloss.Left, header, "", m.viewport.View(), "", footer)
	return modalBorderStyle.Width(boxWidth).Render(body)
}

func renderHandoffQueuePanel(m Model, width int, panelHeight int) string {
	innerWidth := width - 4
	if innerWidth < 50 {
		innerWidth = 50
	}

	var lines []string
	lines = append(lines, panelTitleStyle.Render(i18n.T("tui.handoff.queue_title", len(m.handoffs))))

	if len(m.handoffs) == 0 {
		lines = append(lines, "")
		lines = append(lines, emptyHintStyle.Render(i18n.T("tui.handoff.queue_empty")))
	} else {
		maxRows := panelHeight - 3
		if maxRows < 2 {
			maxRows = 2
		}

		// Show newest tickets first.
		shown := 0
		for i := len(m.handoffs) - 1; i >= 0 && shown < maxRows; i-- {
			it := m.handoffs[i]
			route := fmt.Sprintf("%s->%s", formatTUINode(it.From), formatTUINode(it.To))
			summaryWidth := innerWidth - 46
			if summaryWidth < 12 {
				summaryWidth = 12
			}
			summary := truncateText(it.Summary, summaryWidth)
			parentTag := ""
			if it.ParentID != "" {
				parentTag = " ↩" + session.ShortID(it.ParentID)
			}
			line := fmt.Sprintf("• %s %s %-14s %-9s %s%s",
				padRight(it.ID, 18),
				renderHandoffTypeBadge(it.Type),
				truncateText(route, 14),
				string(it.Status),
				summary,
				headerMetaStyle.Render(parentTag),
			)
			lines = append(lines, line)
			shown++
		}
	}

	content := strings.Join(lines, "\n")
	return panelBorderStyle.Width(width - 2).Height(panelHeight - 2).Render(content)
}

func formatTUINode(n handoff.HandoffNode) string {
	mode := strings.ToUpper(strings.TrimSpace(n.Mode))
	sid := strings.TrimSpace(n.SessionID)
	switch {
	case mode != "" && sid != "":
		return fmt.Sprintf("%s(%s)", mode, session.ShortID(sid))
	case mode != "":
		return mode
	case sid != "":
		return session.ShortID(sid)
	default:
		return "-"
	}
}
