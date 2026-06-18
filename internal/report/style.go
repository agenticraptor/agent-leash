package report

import "github.com/charmbracelet/lipgloss"

// Color palette. lipgloss degrades gracefully on terminals without truecolor
// and honors NO_COLOR.
var (
	colDanger = lipgloss.Color("#ff5f5f")
	colOK     = lipgloss.Color("#5fd75f")
	colWarn   = lipgloss.Color("#ffd75f")
	colDim    = lipgloss.Color("#8a8a8a")
	colAccent = lipgloss.Color("#5fafff")
)

var (
	styleDanger = lipgloss.NewStyle().Foreground(colDanger).Bold(true)
	styleOK     = lipgloss.NewStyle().Foreground(colOK).Bold(true)
	styleWarn   = lipgloss.NewStyle().Foreground(colWarn)
	styleDim    = lipgloss.NewStyle().Foreground(colDim)
	styleAccent = lipgloss.NewStyle().Foreground(colAccent)
	styleLabel  = lipgloss.NewStyle().Foreground(colDim)

	stopBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colDanger).
		Padding(0, 2)

	reportBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim).
			Padding(0, 2)
)
