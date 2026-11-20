package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"guard/internal/core"
)

var (
	colorYellow  = lipgloss.Color("#F5C211")
	colorCyan    = lipgloss.Color("#2EC4B6")
	colorBlue    = lipgloss.Color("#4EA8DE")
	colorGray    = lipgloss.Color("#8D99AE")
	colorGreen   = lipgloss.Color("#57CC99")
	colorRed     = lipgloss.Color("#E63946")
	colorDim     = lipgloss.Color("#6C757D")
	colorBorder  = lipgloss.Color("#495057")
	colorFocusBg = lipgloss.Color("#263238")
	colorWhite   = lipgloss.Color("#F8F9FA")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	headerMetaStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	headerHintStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	panelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	modalBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBlue).
				Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorGray)

	selectedRowStyle = lipgloss.NewStyle().
				Background(colorFocusBg).
				Bold(true)

	cursorMarkStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	denyCountStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	zeroCountStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	allowBadgeStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	denyBadgeStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	reasonStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	toastStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	toastErrStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true)

	footerDescStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	emptyHintStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)
)

func renderModeBadge(mode string) string {
	upper := core.NormalizeMode(mode)
	switch upper {
	case core.ModeDev:
		return lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("[DEV ]")
	case core.ModeTest:
		return lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render("[TEST]")
	case core.ModePlan:
		return lipgloss.NewStyle().Foreground(colorBlue).Bold(true).Render("[PLAN]")
	case core.ModeAll, core.ModeQA:
		return lipgloss.NewStyle().Foreground(colorGray).Bold(true).Render("[ALL ]")
	default:
		label := upper
		if len(label) < 4 {
			label += strings.Repeat(" ", 4-len(label))
		}
		return lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render("[" + label + "]")
	}
}
