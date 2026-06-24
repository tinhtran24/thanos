// Package sidebar renders the right column of the workbench: the THANOS logo, a
// clickable Feature→EC tree, and the active model/MCP info. It mirrors crush's
// right-hand session sidebar, adapted to Thanos's feature/execution-chunk model.
package sidebar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

// Logo renders the brand block at the top of the sidebar.
func Logo(version string) string {
	brand := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true).Render("◆ THANOS")
	sub := styles.MutedS.Render("deterministic agents · v" + version)
	return brand + "\n" + sub
}

// Row is one rendered tree row mapped back to a feature/EC for hit-testing.
type Row struct {
	FeatureIndex int
	ECIndex      int // -1 for a feature row, else the chunk's display index
}

// TreeData carries everything the tree needs.
type TreeData struct {
	Features  []model.Feature
	Phase     map[string]model.Phase // feature ID → current phase
	Started   map[string]bool
	Plans     map[string]model.ExecutionPlan
	Cursor    int // selected feature index
	ECCursor  int // selected EC (-1 = feature row)
	RunningID string
}

// RenderTree draws the feature list with the selected feature expanded into its
// execution chunks. It returns the rendered string and the row→target map (one
// entry per visible row) for mouse hit-testing.
func RenderTree(d TreeData, width int) (string, []Row) {
	var b strings.Builder
	var rows []Row
	b.WriteString(styles.SectionTitle("Features"))
	b.WriteString("\n")
	rows = append(rows, Row{FeatureIndex: -1, ECIndex: -1}) // section title line

	if len(d.Features) == 0 {
		b.WriteString("\n")
		b.WriteString(styles.MutedS.Render("No features yet.\nPress n or /new to create one."))
		return b.String(), rows
	}

	for fi, f := range d.Features {
		selected := fi == d.Cursor
		glyph := "  "
		style := lipgloss.NewStyle().Foreground(styles.Text)
		if selected {
			glyph = "› "
			style = styles.AccentS.Bold(true)
		}
		if f.ID == d.RunningID {
			glyph = "▶ "
			style = styles.WarningS.Bold(true)
		}
		phase := "not started"
		if d.Started[f.ID] {
			phase = styles.PhaseLabel(d.Phase[f.ID])
		}
		b.WriteString(style.Render(glyph + util.Truncate(f.Title, width-2)))
		b.WriteString("\n")
		rows = append(rows, Row{FeatureIndex: fi, ECIndex: -1})
		b.WriteString(styles.MutedS.Render(util.Truncate("  "+f.ID+" · "+phase, width)))
		b.WriteString("\n")
		rows = append(rows, Row{FeatureIndex: fi, ECIndex: -1})

		// Expand the selected feature into its execution chunks.
		if selected {
			chunks := d.Plans[f.ID].ActiveChunks()
			if len(chunks) == 0 {
				b.WriteString(styles.MutedS.Render("    (no execution plan yet)"))
				b.WriteString("\n")
				rows = append(rows, Row{FeatureIndex: fi, ECIndex: -1})
			}
			for ci, c := range chunks {
				ecSelected := d.ECCursor == ci
				mark := chunkGlyph(c.Status)
				line := "    " + mark + " EC-" + itoa(c.Index) + " " + util.Truncate(c.Title, util.Max(6, width-12))
				ls := styles.MutedS
				if ecSelected {
					ls = styles.AccentS.Bold(true)
					line = "  ▸ " + mark + " EC-" + itoa(c.Index) + " " + util.Truncate(c.Title, util.Max(6, width-12))
				}
				b.WriteString(ls.Render(line))
				b.WriteString("\n")
				rows = append(rows, Row{FeatureIndex: fi, ECIndex: ci})
			}
		}
		b.WriteString("\n")
		rows = append(rows, Row{FeatureIndex: fi, ECIndex: -1}) // spacer line
	}
	return b.String(), rows
}

func chunkGlyph(status string) string {
	switch status {
	case "done":
		return styles.SuccessS.Render("●")
	case "active":
		return styles.WarningS.Render("◆")
	case "removed":
		return styles.MutedS.Render("✗")
	default:
		return styles.MutedS.Render("○")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
