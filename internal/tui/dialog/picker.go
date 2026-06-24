// Package dialog provides modal overlays for the TUI: a fuzzy feature picker and
// a keybinding help panel. It mirrors crush's internal/ui/dialog.
package dialog

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/tui/styles"
)

// featureItem adapts a feature to bubbles/list.DefaultItem.
type featureItem struct {
	id    string
	title string
	phase string
}

func (i featureItem) Title() string       { return i.title }
func (i featureItem) Description() string { return i.id + " · " + i.phase }
func (i featureItem) FilterValue() string { return i.id + " " + i.title }

// FeaturePicker is a fuzzy-filterable modal listing all features.
type FeaturePicker struct {
	list     list.Model
	chosenID string
	done     bool
	cancel   bool
}

// FeatureRow is the data needed to list one feature.
type FeatureRow struct {
	ID    string
	Title string
	Phase string
}

// NewFeaturePicker builds a picker over the given features.
func NewFeaturePicker(rows []FeatureRow, width, height int) FeaturePicker {
	items := make([]list.Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, featureItem{id: r.ID, title: r.Title, phase: r.Phase})
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(styles.Accent).BorderForeground(styles.Accent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(styles.AccentDim).BorderForeground(styles.Accent)

	w, h := boxSize(width, height)
	l := list.New(items, delegate, w, h-2)
	l.Title = "Jump to session"
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = l.Styles.Title.Background(styles.AccentDim).Foreground(styles.Text)
	return FeaturePicker{list: l}
}

// SetSize resizes the picker.
func (p *FeaturePicker) SetSize(width, height int) {
	w, h := boxSize(width, height)
	p.list.SetSize(w, h-2)
}

// Update handles keys. It reports completion via Done/Chosen/Cancelled.
func (p *FeaturePicker) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		// Only intercept enter/esc when not actively typing a filter.
		filtering := p.list.FilterState() == list.Filtering
		switch key.String() {
		case "esc":
			if filtering {
				break // let the list clear its filter first
			}
			p.done = true
			p.cancel = true
			return nil
		case "enter":
			if filtering {
				break // first enter confirms the filter
			}
			if item, ok := p.list.SelectedItem().(featureItem); ok {
				p.chosenID = item.id
				p.done = true
			}
			return nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

// Done reports whether the picker has finished.
func (p *FeaturePicker) Done() bool { return p.done }

// Cancelled reports whether the user dismissed the picker.
func (p *FeaturePicker) Cancelled() bool { return p.cancel }

// Chosen returns the selected feature ID (empty if cancelled).
func (p *FeaturePicker) Chosen() string { return p.chosenID }

// View renders the picker inside a bordered box.
func (p *FeaturePicker) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Accent).
		Padding(0, 1)
	return box.Render(p.list.View())
}

func boxSize(width, height int) (int, int) {
	w := width * 6 / 10
	if w < 40 {
		w = min(40, width-4)
	}
	if w > 80 {
		w = 80
	}
	h := height * 6 / 10
	if h < 10 {
		h = min(10, height-4)
	}
	if h > 24 {
		h = 24
	}
	return w, h
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
