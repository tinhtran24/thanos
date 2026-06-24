package chat

import (
	"fmt"
	"strings"

	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/styles"
	"github.com/tinhtran/thanos/internal/tui/util"
)

type workflowStep struct {
	phase       model.Phase
	label       string
	description string
}

var workflowSteps = []workflowStep{
	{model.PhasePlan, "Planning", "Analyze the ticket and generate execution tasks"},
	{model.PhaseCode, "Coding", "Implement the current execution chunk"},
	{model.PhaseTest, "EC tests", "Verify the current chunk against its acceptance cases"},
	{model.PhaseOverview, "Overview", "Summarize changes, evidence, and open issues"},
	{model.PhasePending, "Human review", "Waiting for final human approval"},
	{model.PhaseDone, "Done", "Workflow completed"},
}

// RenderWorkflow draws a persistent Claude Code-style multiline workflow panel.
// It derives completed, active, pending, rejected, and blocked states from the
// persisted workflow state.
func RenderWorkflow(current model.State, width int) string {
	active := workflowIndex(current)
	var lines []string
	for index, step := range workflowSteps {
		icon, style := "○", styles.MutedS
		description := step.description

		switch {
		case current.Phase == model.PhaseDone && index <= active:
			icon, style = "✓", styles.SuccessS
		case current.Phase == model.PhaseAmend && step.phase == model.PhaseTest:
			icon, style = "✕", styles.DangerS
			description = reasonOr(current.Reason, "Rejected; returning the EC to coding")
		case current.Phase == model.PhaseAmend && step.phase == model.PhaseCode:
			icon, style = "◆", styles.WarningS.Bold(true)
			description = fmt.Sprintf("Amending execution chunk, round %d", current.Round)
		case (current.Phase == model.PhaseBlocked || current.Phase == model.PhaseAttention) && index == active:
			icon, style = "■", styles.DangerS.Bold(true)
			description = reasonOr(current.Reason, "Workflow blocked")
		case index < active:
			icon, style = "✓", styles.SuccessS
		case index == active:
			icon, style = "◆", styles.AccentS.Bold(true)
		}

		lines = append(lines, style.Render(icon+" "+step.label))
		if width >= 28 {
			lines = append(lines, styles.MutedS.Render("  "+util.Truncate(description, width-2)))
		}
	}
	return strings.Join(lines, "\n")
}

// RenderFlow is retained for callers that only have a phase.
func RenderFlow(current model.Phase, width int) string {
	return RenderWorkflow(model.State{Phase: current}, width)
}

func workflowIndex(current model.State) int {
	phase := current.Phase
	if phase == model.PhaseInit {
		return -1
	}
	if phase == model.PhaseAmend {
		return 1
	}
	if phase == model.PhaseBlocked || phase == model.PhaseAttention {
		switch current.Role {
		case model.RolePlanner:
			return 0
		case model.RoleTester:
			return 2
		case model.RoleAcceptor:
			return 3
		default:
			return 1
		}
	}
	// Map legacy sessions onto the nearest simplified step.
	switch phase {
	case model.PhaseDesign, model.PhaseDesignReview, model.PhaseReview:
		phase = model.PhaseCode
	case model.PhaseDeepReview, model.PhaseAccept:
		phase = model.PhaseOverview
	}
	for index, step := range workflowSteps {
		if step.phase == phase {
			return index
		}
	}
	return -1
}

func reasonOr(reason, fallback string) string {
	if strings.TrimSpace(reason) != "" {
		return reason
	}
	return fallback
}
