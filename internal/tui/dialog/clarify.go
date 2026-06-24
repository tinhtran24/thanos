package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// Clarify is a modal that presents an agent's question and its options so the
// user can disambiguate when the AI is unsure (the "popup choose" flow).
type Clarify struct {
	question string
	options  []string
	cursor   int
	done     bool
	cancel   bool
	chosen   string
}

// NewClarify builds a clarification dialog.
func NewClarify(question string, options []string) Clarify {
	return Clarify{question: question, options: options}
}

// Update handles navigation/selection keys.
func (c *Clarify) Update(msg tea.Msg) {
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
	case "esc":
		c.done, c.cancel = true, true
	case "enter", "space":
		if c.cursor >= 0 && c.cursor < len(c.options) {
			c.chosen = c.options[c.cursor]
			c.done = true
		}
	}
}

func (c *Clarify) Done() bool      { return c.done }
func (c *Clarify) Cancelled() bool { return c.cancel }
func (c *Clarify) Chosen() string  { return c.chosen }

// View renders the dialog inside a bordered box.
func (c *Clarify) View() string {
	var b strings.Builder
	b.WriteString(styles.WarningS.Bold(true).Render("Agent needs a decision"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(styles.Text).Render(util.Wrap(c.question, 56)))
	b.WriteString("\n\n")
	for i, opt := range c.options {
		cursor := "  "
		style := styles.MutedS
		if i == c.cursor {
			cursor = "› "
			style = styles.AccentS.Bold(true)
		}
		b.WriteString(style.Render(cursor + util.Truncate(opt, 54)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.MutedS.Render("↑↓ choose · enter answer · esc cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Warning).
		Padding(1, 2).
		Width(62)
	return box.Render(b.String())
}
