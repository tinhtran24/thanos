package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/tui/styles"
)

// HelpEntry is one key/description row in the help overlay.
type HelpEntry struct {
	Keys string
	Desc string
}

// RenderHelp draws the keybinding help inside a bordered box.
func RenderHelp(entries []HelpEntry) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
	descStyle := styles.MutedS

	width := 0
	for _, e := range entries {
		if len(e.Keys) > width {
			width = len(e.Keys)
		}
	}
	var b strings.Builder
	b.WriteString(styles.Title.Render("Keyboard shortcuts"))
	b.WriteString("\n\n")
	for _, e := range entries {
		pad := strings.Repeat(" ", width-len(e.Keys)+2)
		b.WriteString(keyStyle.Render(e.Keys) + pad + descStyle.Render(e.Desc) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.MutedS.Render("esc / ? to close"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Accent).
		Padding(1, 2)
	return box.Render(b.String())
}
