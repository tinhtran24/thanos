// Package list renders the left "Work sessions" pane. Cursor state stays on the
// root model (so existing navigation/mouse tests keep working); this package is
// a pure renderer plus the row geometry the mouse handler needs.
package list

import (
	"strings"

	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// RowTop is the Y of the first session row inside the panel body.
const RowTop = 3

// RowHeight is the vertical span of one session row (title + meta + gap).
const RowHeight = 3

// Session is one item in the sessions pane.
type Session struct {
	Feature model.Feature
	Phase   model.Phase
	Started bool
}

// IndexAtY maps a mouse Y coordinate to a session index, or -1 if outside rows.
func IndexAtY(y, count int) int {
	index := (y - RowTop) / RowHeight
	if index < 0 || index >= count {
		return -1
	}
	return index
}

// Render draws the sessions list with the cursor and running markers.
func Render(sessions []Session, cursor int, runningID string, width int) string {
	var b strings.Builder
	b.WriteString(styles.SectionTitle("Work sessions"))
	b.WriteString("\n")
	if len(sessions) == 0 {
		b.WriteString("\n")
		b.WriteString(styles.MutedS.Render("No sessions yet.\nPress n or /new to create one."))
		return b.String()
	}
	for index, s := range sessions {
		phase := "not started"
		if s.Started {
			phase = styles.PhaseLabel(s.Phase)
		}
		cursorGlyph := "  "
		style := styles.Title.Foreground(styles.Text).Bold(false)
		if index == cursor {
			cursorGlyph = "› "
			style = styles.AccentS.Bold(true)
		}
		if s.Feature.ID == runningID {
			cursorGlyph = "▶ "
			style = styles.WarningS.Bold(true)
		}
		title := util.Truncate(s.Feature.Title, width-2)
		b.WriteString(style.Render(cursorGlyph + title))
		b.WriteString("\n")
		b.WriteString(styles.MutedS.Render(util.Truncate("  "+s.Feature.ID+" · "+phase, width)))
		if index < len(sessions)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}
