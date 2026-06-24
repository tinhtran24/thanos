package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/workspace"
)

func newTestModel(t *testing.T, feature model.Feature) *modelUI {
	t.Helper()
	ws := workspace.Open(t.TempDir())
	if err := ws.Init(model.Config{
		Project:       model.Project{Name: "example"},
		DefaultRunner: "codex",
		Runners:       map[string]model.Runner{"codex": {Command: "codex"}, "claude": {Command: "claude"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	ui, err := newModel(context.Background(), ws, "test")
	if err != nil {
		t.Fatal(err)
	}
	ui.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return ui
}

func TestViewOmitsBriefAndAcceptance(t *testing.T) {
	feature := model.Feature{
		ID: "F001-login", Title: "Add login", Status: "todo", Priority: "high",
		Description: "SECRET_BRIEF_TEXT", Acceptance: []string{"ACCEPT_CRITERION_ONE"},
	}
	ui := newTestModel(t, feature)
	view := ui.View().Content
	// The old detail view rendered the feature Description under a "BRIEF"
	// section and the acceptance list under an "ACCEPTANCE" section; both are
	// gone now. (Upper-cased section titles are checked so the temp-dir path in
	// the header — which contains the test name — doesn't cause false matches.)
	for _, banned := range []string{"SECRET_BRIEF_TEXT", "ACCEPT_CRITERION_ONE", "BRIEF", "ACCEPTANCE"} {
		if strings.Contains(view, banned) {
			t.Fatalf("view should not contain %q:\n%s", banned, view)
		}
	}
	if !strings.Contains(view, "Add login") {
		t.Fatalf("view missing feature title:\n%s", view)
	}
}

func TestTabCyclesFocusToChatAndInput(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	if ui.focus != focusSessions {
		t.Fatalf("initial focus = %d, want sessions", ui.focus)
	}
	ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if ui.focus != focusChat {
		t.Fatalf("after first tab focus = %d, want chat", ui.focus)
	}
	ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if ui.focus != focusInput {
		t.Fatalf("after second tab focus = %d, want input", ui.focus)
	}
	ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if ui.focus != focusSessions {
		t.Fatalf("after third tab focus = %d, want sessions", ui.focus)
	}
}

func TestNewSessionFlowCollectsDescriptionAndAcceptance(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})

	// n opens the guided flow, focused on the command box at the title step.
	ui.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if !ui.create.active || ui.focus != focusInput {
		t.Fatalf("n should start the create flow focused on input (active=%v focus=%d)", ui.create.active, ui.focus)
	}
	if ui.create.step != stepTitle {
		t.Fatalf("flow should begin at the title step, got %d", ui.create.step)
	}

	// Title → description → acceptance, each saved on enter.
	ui.input.SetValue("Add OAuth login")
	ui.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if ui.create.step != stepDesc {
		t.Fatalf("after title, step = %d, want description", ui.create.step)
	}
	ui.input.SetValue("Support Google and GitHub OAuth")
	ui.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if ui.create.step != stepAccept {
		t.Fatalf("after description, step = %d, want acceptance", ui.create.step)
	}
	ui.input.SetValue("google works; github works")
	_, cmd := ui.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if ui.create.active {
		t.Fatal("flow should finish after the acceptance step")
	}
	if cmd == nil {
		t.Fatal("finishing the flow should create the session")
	}
	ui.Update(cmd()) // run the create command (writes the feature to disk)

	// The created feature carries the description and acceptance from the flow.
	feats, err := ui.ws.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, f := range feats {
		if f.Title == "Add OAuth login" {
			id = f.ID
		}
	}
	if id == "" {
		t.Fatalf("created feature not found among %+v", feats)
	}
	full, err := ui.ws.LoadFeature(id)
	if err != nil {
		t.Fatal(err)
	}
	if full.Description != "Support Google and GitHub OAuth" {
		t.Fatalf("description = %q", full.Description)
	}
	if len(full.Acceptance) != 2 || full.Acceptance[0] != "google works" || full.Acceptance[1] != "github works" {
		t.Fatalf("acceptance = %v", full.Acceptance)
	}
}

func TestMouseClickSelectsChatBubbleAndReleaseCopies(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	// Inject some chat content and render once so the chat rectangle is known.
	ui.chat.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder, Round: 1})
	ui.chat.Append("hello from the coder\n")
	_ = ui.View()

	// Press inside the chat viewport: selects a bubble and starts a drag.
	_, _ = ui.Update(tea.MouseClickMsg{X: ui.chatX0, Y: ui.chatY0, Button: tea.MouseLeft})
	if !ui.dragging {
		t.Fatal("press in chat should start a drag")
	}
	if ui.focus != focusChat || !ui.chat.HasSelection() {
		t.Fatalf("press should focus chat and select a bubble (focus=%d, sel=%v)", ui.focus, ui.chat.HasSelection())
	}

	// Release: should emit a copy command and end the drag.
	_, cmd := ui.Update(tea.MouseReleaseMsg{X: ui.chatX0, Y: ui.chatY0, Button: tea.MouseLeft})
	if ui.dragging {
		t.Fatal("release should end the drag")
	}
	if cmd == nil {
		t.Fatal("release with a selection should emit a copy command")
	}
}

func TestCLICommandStartsSubprocessAndOpensBubble(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	before := ui.chat.Count()
	ui.input.SetValue("/status")
	m, cmd := ui.submitCommand() // returns a batch incl. the subprocess; do NOT run it
	ui = m.(*modelUI)
	if !ui.running {
		t.Fatal("/status should mark the model running")
	}
	if ui.chat.Count() <= before {
		t.Fatal("/status should open a command bubble in the chat")
	}
	if cmd == nil {
		t.Fatal("/status should return a command")
	}
}

func TestTransitionCommandRequiresArg(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	ui.input.SetValue("/transition")
	ui.submitCommand()
	if ui.err == nil {
		t.Fatal("/transition with no phase should error")
	}
	if ui.running {
		t.Fatal("/transition with no phase should not start a process")
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	ui.input.SetValue("/bogus")
	ui.submitCommand()
	if ui.err == nil {
		t.Fatal("unknown command should set an error")
	}
}

func TestInputCursorSurfacedWhenFocused(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	// With the sessions pane focused, no terminal cursor is shown.
	if cur := ui.View().Cursor; cur != nil {
		t.Fatalf("cursor should be hidden when sessions focused, got %+v", cur)
	}
	// Pressing n focuses the command box; the real cursor should appear on the
	// input row (crush's pattern: View.Cursor positioned to the input).
	ui.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if ui.focus != focusInput {
		t.Fatalf("n should focus the input, got focus=%d", ui.focus)
	}
	v := ui.View()
	if v.Cursor == nil {
		t.Fatal("cursor should be visible when the command box is focused")
	}
	if v.Cursor.Y != ui.inputY0 {
		t.Fatalf("cursor Y = %d, want input row %d", v.Cursor.Y, ui.inputY0)
	}
	if v.Cursor.X < 2 {
		t.Fatalf("cursor X = %d, want >= prompt width (2)", v.Cursor.X)
	}
}

func TestMouseSelectModeTogglesMouseMode(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	// Default: the app captures the mouse (cell motion) to handle clicks/drags.
	if ui.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("default MouseMode = %v, want CellMotion", ui.View().MouseMode)
	}
	// ctrl+s releases mouse capture so the terminal does native text selection.
	ui.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !ui.selectMode {
		t.Fatal("ctrl+s should enable select mode")
	}
	if ui.View().MouseMode != tea.MouseModeNone {
		t.Fatalf("select-mode MouseMode = %v, want None", ui.View().MouseMode)
	}
	// Esc resumes app mouse handling.
	ui.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if ui.selectMode {
		t.Fatal("esc should disable select mode")
	}
	if ui.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("after esc MouseMode = %v, want CellMotion", ui.View().MouseMode)
	}
}
