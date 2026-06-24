package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
)

var flowPhases = []model.Phase{
	model.PhasePlan, model.PhaseDesign, model.PhaseDesignReview, model.PhaseCode, model.PhaseReview,
	model.PhaseTest, model.PhaseDeepReview, model.PhaseAccept, model.PhasePending, model.PhaseDone,
}

// RenderFlow draws the phase progress strip for the current phase, wrapping to
// the given width.
func RenderFlow(current model.Phase, width int) string {
	currentIndex := phaseIndex(current)
	var parts []string
	for index, phase := range flowPhases {
		label := styles.PhaseLabel(phase)
		style := styles.MutedS
		icon := "○"
		if current == model.PhaseAmend && phase == model.PhaseCode {
			style = styles.WarningS.Bold(true)
			icon = "↺"
		} else if index < currentIndex {
			style = styles.SuccessS
			icon = "●"
		} else if index == currentIndex {
			style = styles.AccentS.Bold(true)
			icon = "◆"
		}
		parts = append(parts, style.Render(icon+" "+label))
	}
	var lines []string
	var line string
	for _, part := range parts {
		plainWidth := lipgloss.Width(part)
		if line != "" && lipgloss.Width(line)+3+plainWidth > width {
			lines = append(lines, line)
			line = part
		} else if line == "" {
			line = part
		} else {
			line += styles.MutedS.Render(" ─ ") + part
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func phaseIndex(current model.Phase) int {
	if current == model.PhaseInit {
		return 0
	}
	if current == model.PhaseAmend {
		current = model.PhaseCode // amend re-enters the coding step
	}
	for index, phase := range flowPhases {
		if phase == current {
			return index
		}
	}
	return 0
}
