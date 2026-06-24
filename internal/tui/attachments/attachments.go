package attachments

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// Model holds the attachments currently staged for the active run and renders
// them as a chip strip.
type Model struct {
	items []Attachment
}

// New returns an empty attachments model.
func New() Model { return Model{} }

// Add stages an attachment.
func (m *Model) Add(a Attachment) { m.items = append(m.items, a) }

// RemoveLast drops the most recently added attachment.
func (m *Model) RemoveLast() {
	if len(m.items) > 0 {
		m.items = m.items[:len(m.items)-1]
	}
}

// Clear removes all staged attachments.
func (m *Model) Clear() { m.items = nil }

// Items returns the staged attachments.
func (m *Model) Items() []Attachment { return m.items }

// Len returns the number of staged attachments.
func (m *Model) Len() int { return len(m.items) }

// View renders the chip strip, or an empty string when nothing is staged.
func (m *Model) View(width int) string {
	if len(m.items) == 0 {
		return ""
	}
	chip := lipgloss.NewStyle().
		Foreground(styles.Text).
		Background(styles.Subtle).
		Padding(0, 1)
	var chips []string
	for _, a := range m.items {
		icon := "📎"
		if a.IsImage {
			icon = "🖼"
		}
		chips = append(chips, chip.Render(icon+" "+util.Truncate(a.Name, 22)))
	}
	row := strings.Join(chips, " ")
	return lipgloss.NewStyle().Width(width).Render(row)
}
