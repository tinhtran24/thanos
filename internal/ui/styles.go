package ui

import "github.com/charmbracelet/lipgloss"

const (
	PrimaryColor = "#7C3AED"
	SuccessColor = "#22C55E"
	WarningColor = "#F59E0B"
	ErrorColor   = "#EF4444"
	InfoColor    = "#38BDF8"
	MutedColor   = "#94A3B8"
)

var (
	HeadingStyle = lipgloss.NewStyle().
			Bold(true)

	PrimaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(PrimaryColor))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SuccessColor))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(WarningColor))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ErrorColor))

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(InfoColor))

	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(MutedColor))

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(MutedColor)).
			Padding(0, 1)

	SuccessPanelStyle = PanelStyle.Copy().
				BorderForeground(lipgloss.Color(SuccessColor))

	ErrorPanelStyle = PanelStyle.Copy().
			BorderForeground(lipgloss.Color(ErrorColor))

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(MutedColor))

	TableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)

	DividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(MutedColor))
)
