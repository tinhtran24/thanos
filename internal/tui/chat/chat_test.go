package chat

import (
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/util"
)

func TestRenderWorkflowStates(t *testing.T) {
	tests := []struct {
		name  string
		state model.State
		want  []string
	}{
		{
			name:  "coding active",
			state: model.State{Phase: model.PhaseCode, Round: 1},
			want:  []string{"✓ Planning", "◆ Coding", "○ EC tests", "○ Overview"},
		},
		{
			name:  "test rejected",
			state: model.State{Phase: model.PhaseAmend, Round: 2},
			want:  []string{"◆ Coding", "✕ EC tests", "Rejected; returning the EC to coding"},
		},
		{
			name:  "tester blocked",
			state: model.State{Phase: model.PhaseBlocked, Role: model.RoleTester, Reason: "test environment unavailable"},
			want:  []string{"✓ Planning", "✓ Coding", "■ EC tests", "test environment unavailable"},
		},
		{
			name:  "done completed",
			state: model.State{Phase: model.PhaseDone},
			want:  []string{"✓ Planning", "✓ Overview", "✓ Human review", "✓ Done"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.StripANSI(RenderWorkflow(tt.state, 80))
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("workflow missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestOnEventSegmentsRolesIntoBubbles(t *testing.T) {
	m := New()
	m.SetSize(60, 20)

	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleDesigner, Phase: model.PhaseDesign, Round: 1})
	m.Append("drafting the design\n")
	m.OnEvent(model.Event{Type: "role-end", Role: model.RoleDesigner, Data: map[string]any{"success": true}})
	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder, Phase: model.PhaseCode, Round: 1})
	m.Append("writing code\n")

	if m.Count() != 2 {
		t.Fatalf("message count = %d, want 2", m.Count())
	}
	// The coder bubble is still open; designer was closed by role-end.
	if got := m.LastBody(); !strings.Contains(got, "writing code") {
		t.Fatalf("open bubble body = %q", got)
	}
	view := m.View()
	for _, want := range []string{"designer", "coder", "drafting the design", "writing code"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestSelectedTextCopiesBubblePlaintext(t *testing.T) {
	m := New()
	m.SetSize(60, 20)
	m.Focus()
	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder, Round: 2})
	m.Append("line one\nline two\n")

	m.SelectMove(-1) // select last (only) bubble
	if !m.HasSelection() {
		t.Fatal("expected a selection")
	}
	text := m.SelectedText()
	if !strings.Contains(text, "coder") || !strings.Contains(text, "line one") || !strings.Contains(text, "line two") {
		t.Fatalf("selected text = %q", text)
	}
}

func TestSelectAtViewportRowSelectsTopBubble(t *testing.T) {
	m := New()
	m.SetSize(60, 20)
	m.Focus()
	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleDesigner, Round: 1})
	m.Append("first bubble body\n")
	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder, Round: 1})
	m.Append("second bubble body\n")

	// Row 0 is the top visible line, which belongs to the first bubble.
	if !m.SelectAtViewportRow(0) {
		t.Fatal("expected row 0 to select a bubble")
	}
	if text := m.SelectedText(); !strings.Contains(text, "first bubble body") {
		t.Fatalf("row 0 selected %q, want the first bubble", text)
	}

	// Extending downward should grow the range to include the second bubble.
	m.ExtendAtViewportRow(m.vp.Height() - 1)
	if text := m.SelectedText(); !strings.Contains(text, "second bubble body") {
		t.Fatalf("extended selection = %q, want both bubbles", text)
	}
}

func TestRoleEndErrorSurfacesErrorBubble(t *testing.T) {
	m := New()
	m.SetSize(60, 20)
	m.OnEvent(model.Event{Type: "role-start", Role: model.RoleTester, Round: 1})
	m.OnEvent(model.Event{Type: "role-end", Data: map[string]any{"success": false, "error": "tests failed"}})
	if !strings.Contains(m.View(), "tests failed") {
		t.Fatalf("expected error bubble in view:\n%s", m.View())
	}
}
