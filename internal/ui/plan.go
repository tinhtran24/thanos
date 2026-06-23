package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type State string

const (
	Pending   State = "pending"
	Running   State = "running"
	Done      State = "done"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Completed State = "completed"
	Warned    State = "warning"
)

type PlanStep struct {
	Title string
	State State
}

func Plan(steps []PlanStep) string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		icon, style := stateIcon(step.State)
		lines = append(lines, style.Render(icon)+" "+step.Title)
	}
	return strings.Join(lines, "\n")
}

func stateIcon(state State) (string, lipgloss.Style) {
	switch state {
	case Done, Succeeded, Completed:
		return "✓", SuccessStyle
	case Failed:
		return "✗", ErrorStyle
	case Running:
		return "→", WarningStyle
	case Warned:
		return "⚠", WarningStyle
	default:
		return "•", MutedStyle
	}
}
