package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// Confirm is a generic modal whose presence prevents the composer from owning
// paste input.
type Confirm struct {
	title   string
	message string
	options []string
	cursor  int
	done    bool
	chosen  int // selected option index, -1 when dismissed
}

// NewConfirm builds a confirmation dialog. options[0] should be the affirmative
// action; a trailing "Stop"/"Cancel" entry is conventional.
func NewConfirm(title, message string, options []string) Confirm {
	return Confirm{title: title, message: message, options: options, chosen: -1}
}

// Update handles navigation/selection keys.
func (c *Confirm) Update(msg tea.Msg) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch key.String() {
	case "up", "k":
		if c.cursor > 0 {
			c.cursor--
		}
	case "down", "j":
		if c.cursor < len(c.options)-1 {
			c.cursor++
		}
	case "esc", "q":
		c.done, c.chosen = true, -1
	case "enter", "space":
		c.done, c.chosen = true, c.cursor
	}
}

func (c *Confirm) Done() bool { return c.done }

// Chosen returns the selected option index, or -1 when the dialog was dismissed.
func (c *Confirm) Chosen() int { return c.chosen }

// Affirmed reports whether the user picked the first (affirmative) option.
func (c *Confirm) Affirmed() bool { return c.done && c.chosen == 0 }

// View renders the dialog inside a bordered box.
func (c *Confirm) View() string {
	const width = 56
	var b strings.Builder
	b.WriteString(styles.AccentS.Bold(true).Render(c.title))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.Text).Render(util.Wrap(c.message, width)))
	b.WriteString("\n\n")
	for i, opt := range c.options {
		cursor := "  "
		style := styles.MutedS
		if i == c.cursor {
			cursor = "› "
			style = styles.AccentS.Bold(true)
		}
		b.WriteString(style.Render(cursor + util.Truncate(opt, width-2)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.MutedS.Render("↑↓ choose · enter confirm · esc dismiss"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Accent).
		Padding(1, 2).
		Width(width + 4)
	return box.Render(b.String())
}
