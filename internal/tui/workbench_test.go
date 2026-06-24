package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	view := ui.View()
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
	ui.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ui.focus != focusChat {
		t.Fatalf("after first tab focus = %d, want chat", ui.focus)
	}
	ui.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ui.focus != focusInput {
		t.Fatalf("after second tab focus = %d, want input", ui.focus)
	}
	ui.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ui.focus != focusSessions {
		t.Fatalf("after third tab focus = %d, want sessions", ui.focus)
	}
}

func TestNewCommandCreatesSession(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	// Focus the command box and submit "/new Second feature".
	ui.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if ui.focus != focusInput {
		t.Fatalf("pressing n should focus input, got %d", ui.focus)
	}
	ui.input.SetValue("/new Second feature")
	model, _ := ui.submitCommand()
	ui = model.(*modelUI)
	// createFeature runs as a command; execute it and feed the result back.
	cmd := ui.createFeature("Second feature")
	msg := cmd()
	ui.Update(msg)
	// reload runs as a command too; run it synchronously.
	reload := ui.reload()
	ui.Update(reload())

	var found bool
	for _, f := range ui.features {
		if f.Title == "Second feature" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 'Second feature' session, got %+v", ui.features)
	}
}

func TestMouseClickSelectsChatBubbleAndReleaseCopies(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	// Inject some chat content and render once so the chat rectangle is known.
	ui.chat.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder, Round: 1})
	ui.chat.Append("hello from the coder\n")
	_ = ui.View()

	// Press inside the chat viewport: selects a bubble and starts a drag.
	_, _ = ui.Update(tea.MouseMsg{
		X: ui.chatX0, Y: ui.chatY0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	if !ui.dragging {
		t.Fatal("press in chat should start a drag")
	}
	if ui.focus != focusChat || !ui.chat.HasSelection() {
		t.Fatalf("press should focus chat and select a bubble (focus=%d, sel=%v)", ui.focus, ui.chat.HasSelection())
	}

	// Release: should emit a copy command and end the drag.
	_, cmd := ui.Update(tea.MouseMsg{
		X: ui.chatX0, Y: ui.chatY0, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease,
	})
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

func TestMouseSelectModeTogglesMouse(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	_, cmd := ui.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !ui.selectMode {
		t.Fatal("ctrl+s should enable select mode")
	}
	if cmd == nil {
		t.Fatal("ctrl+s should emit a DisableMouse command")
	}
	// Esc resumes.
	_, cmd = ui.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ui.selectMode {
		t.Fatal("esc should disable select mode")
	}
	if cmd == nil {
		t.Fatal("esc should emit an EnableMouse command")
	}
}
