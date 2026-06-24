// Package styles holds the shared lipgloss palette and component styles for the
// interactive Thanos TUI. It mirrors crush's internal/ui/styles: a single place
// for colors so the rest of the UI never hard-codes hex values.
package styles

import "github.com/charmbracelet/lipgloss"

// Core palette. These are the interactive-TUI colors (distinct from the plain
// CLI output palette in internal/ui).
var (
	Accent    = lipgloss.Color("#B48EFA")
	AccentDim = lipgloss.Color("#6E56A8")
	Text      = lipgloss.Color("#E8E3F0")
	Muted     = lipgloss.Color("#8B8498")
	Subtle    = lipgloss.Color("#2B2731")
	Success   = lipgloss.Color("#7FC8A9")
	Warning   = lipgloss.Color("#E7B86B")
	Danger    = lipgloss.Color("#E07A8D")
	Info      = lipgloss.Color("#6FB3D9")
)

// Frequently used text styles.
var (
	Title    = lipgloss.NewStyle().Foreground(Text).Bold(true)
	MutedS   = lipgloss.NewStyle().Foreground(Muted)
	AccentS  = lipgloss.NewStyle().Foreground(Accent)
	SuccessS = lipgloss.NewStyle().Foreground(Success)
	WarningS = lipgloss.NewStyle().Foreground(Warning)
	DangerS  = lipgloss.NewStyle().Foreground(Danger)
)

// SectionTitle renders an upper-cased, dimmed section heading.
func SectionTitle(value string) string {
	return lipgloss.NewStyle().Foreground(AccentDim).Bold(true).Render(upper(value))
}

// Panel returns a rounded-border box style sized to the given dimensions.
func Panel(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(max(1, width-2)).
		Height(max(1, height-2)).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Subtle)
}

// FocusedPanel is like Panel but with an accent border, used for the pane that
// currently has keyboard focus.
func FocusedPanel(width, height int) lipgloss.Style {
	return Panel(width, height).BorderForeground(Accent)
}

// StatusLine renders a "● label" / "○ label" capability indicator.
func StatusLine(ok bool, label string) string {
	color, icon := Muted, "○"
	if ok {
		color, icon = Success, "●"
	}
	return "\n" + lipgloss.NewStyle().Foreground(color).Render(icon+" "+label)
}

func upper(value string) string {
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
