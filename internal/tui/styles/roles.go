package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/tinhtran/thanos/internal/model"
)

// RoleStyle pairs a color and a short avatar/badge glyph for a phase role, so the
// chat log can render each agent role as a distinct "speaker".
type RoleStyle struct {
	Color  lipgloss.Color
	Avatar string
	Label  string
}

var roleStyles = map[model.Role]RoleStyle{
	model.RoleDesigner:       {Accent, "◆", "designer"},
	model.RoleDesignReviewer: {Info, "◇", "design-reviewer"},
	model.RoleCoder:          {Success, "▸", "coder"},
	model.RoleReviewer:       {Warning, "◈", "reviewer"},
	model.RoleTester:         {Info, "✓", "tester"},
	model.RoleDeepReviewer:   {Accent, "❖", "deep-reviewer"},
	model.RoleAcceptor:       {Success, "★", "acceptor"},
	model.RoleMiniCoder:      {Success, "▹", "mini-coder"},
	model.RoleReVerifier:     {Warning, "↻", "re-verifier"},
	model.RoleSynthesizer:    {Muted, "⊕", "synthesizer"},
	model.RoleGate:           {Muted, "▣", "gate"},
}

// Role returns the style for a role, falling back to a neutral system style.
func Role(role model.Role) RoleStyle {
	if rs, ok := roleStyles[role]; ok {
		return rs
	}
	label := string(role)
	if label == "" {
		label = "agent"
	}
	return RoleStyle{Color: Muted, Avatar: "•", Label: label}
}

// PhaseLabel returns a short human label for a workflow phase.
func PhaseLabel(phase model.Phase) string {
	switch phase {
	case model.PhaseDesign:
		return "design"
	case model.PhaseDesignReview:
		return "design review"
	case model.PhaseCode:
		return "code"
	case model.PhaseReview:
		return "review"
	case model.PhaseTest:
		return "test"
	case model.PhaseDeepReview:
		return "deep review"
	case model.PhaseAmend:
		return "amend"
	case model.PhaseAccept:
		return "accept"
	case model.PhasePending:
		return "human review"
	case model.PhaseDone:
		return "done"
	case model.PhaseBlocked:
		return "blocked"
	case model.PhaseAttention:
		return "attention"
	default:
		return "not started"
	}
}
