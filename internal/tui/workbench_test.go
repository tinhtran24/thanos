package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/tui/attachments"
	"github.com/tinhtran/thanos/internal/tui/dialog"
	"github.com/tinhtran/thanos/internal/tui/util"
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

func TestCrushResponsiveChatLayout(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})

	_ = ui.View()
	if ui.compactLayout() {
		t.Fatal("140x40 should use the wide chat layout")
	}
	if ui.sidebarW != wideSidebarWidth {
		t.Fatalf("wide sidebar width = %d, want %d", ui.sidebarW, wideSidebarWidth)
	}
	if ui.chatW != 140-2-wideSidebarWidth-paneGap {
		t.Fatalf("wide chat width = %d", ui.chatW)
	}

	ui.Update(tea.WindowSizeMsg{Width: compactModeWidthBreakpoint - 1, Height: 40})
	view := ui.View().Content
	if !ui.compactLayout() {
		t.Fatal("narrow terminal should use the compact chat layout")
	}
	if ui.sidebarW != 0 {
		t.Fatalf("compact sidebar width = %d, want 0", ui.sidebarW)
	}
	if ui.chatW != compactModeWidthBreakpoint-3 {
		t.Fatalf("compact chat width = %d", ui.chatW)
	}
	if !strings.HasPrefix(util.StripANSI(view), "THANOS") {
		t.Fatalf("compact layout should start with the one-line header:\n%s", view)
	}

	ui.Update(tea.WindowSizeMsg{Width: 140, Height: compactModeHeightBreakpoint - 1})
	_ = ui.View()
	if !ui.compactLayout() {
		t.Fatal("short terminal should use the compact chat layout")
	}
}

func TestRenderedChatLayoutFitsTerminalWidth(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})

	for _, size := range []tea.WindowSizeMsg{
		{Width: 140, Height: 40},
		{Width: 100, Height: 40},
		{Width: 140, Height: 25},
	} {
		ui.Update(size)
		view := ui.View().Content
		for row, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.Width {
				t.Fatalf("%dx%d row %d width = %d, want <= %d:\n%s", size.Width, size.Height, row, got, size.Width, line)
			}
		}
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
	ui.chat.OnEvent(model.Event{Type: "role-start", Role: model.RoleCoder})
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

func focusComposer(ui *modelUI) {
	ui.focus = focusInput
	ui.chat.Blur()
	_ = ui.input.Focus()
}

func TestFocusedPasteInsertNoSubmit(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	focusComposer(ui)
	ui.input.SetValue("prefixsuffix")
	for range len("suffix") {
		ui.input.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	}

	beforeFeatures := len(ui.features)
	ui.Update(tea.PasteMsg{Content: " one\r\ntwo "})

	if got, want := ui.input.Value(), "prefix one\ntwo suffix"; got != want {
		t.Fatalf("composer value = %q, want %q", got, want)
	}
	if ui.running || ui.create.active || len(ui.features) != beforeFeatures {
		t.Fatalf("paste changed workflow state: running=%v create=%v features=%d", ui.running, ui.create.active, len(ui.features))
	}
}

func TestPasteAttachmentCandidates(t *testing.T) {
	tests := []struct {
		name string
		path func(*testing.T) string
	}{
		{
			name: "absolute",
			path: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "absolute file.txt")
				if err := os.WriteFile(path, []byte("absolute"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
		},
		{
			name: "dot slash",
			path: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "relative file.txt")
				if err := os.WriteFile(path, []byte("relative"), 0o644); err != nil {
					t.Fatal(err)
				}
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				rel, err := filepath.Rel(cwd, path)
				if err != nil {
					t.Fatal(err)
				}
				return "." + string(filepath.Separator) + rel
			},
		},
		{
			name: "home",
			path: func(t *testing.T) string {
				home := t.TempDir()
				t.Setenv("HOME", home)
				path := filepath.Join(home, "home file.txt")
				if err := os.WriteFile(path, []byte("home"), 0o644); err != nil {
					t.Fatal(err)
				}
				return "~/home file.txt"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
			focusComposer(ui)
			ui.Update(tea.PasteMsg{Content: "  " + tt.path(t) + "  "})
			if got := ui.attach.Len(); got != 1 {
				t.Fatalf("attachments = %d, want 1", got)
			}
			if got := ui.input.Value(); got != "" {
				t.Fatalf("staged path inserted into composer: %q", got)
			}
		})
	}
}

func TestPastePathFallbacksToComposerText(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tests := []string{
		"relative.txt",
		"../existing.txt",
		`"` + existing + `"`,
		"'" + existing + "'",
		filepath.Join(t.TempDir(), "missing.txt"),
		dir,
		existing + "\nextra",
	}
	for _, pasted := range tests {
		t.Run(strings.ReplaceAll(pasted, string(filepath.Separator), "_"), func(t *testing.T) {
			ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
			focusComposer(ui)
			ui.Update(tea.PasteMsg{Content: pasted})
			if got := ui.attach.Len(); got != 0 {
				t.Fatalf("attachments = %d, want 0 for %q", got, pasted)
			}
			if got := ui.input.Value(); got != pasted {
				t.Fatalf("composer value = %q, want fallback %q", got, pasted)
			}
		})
	}
}

func TestPasteAttachmentCopyFailureFallsBackToText(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	focusComposer(ui)
	source := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(ui.ws.RuntimeDir("F001-x"), "attachments")
	if err := os.MkdirAll(filepath.Dir(blocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	ui.Update(tea.PasteMsg{Content: source})
	if ui.attach.Len() != 0 || ui.input.Value() != source {
		t.Fatalf("copy failure did not fall back: attachments=%d value=%q", ui.attach.Len(), ui.input.Value())
	}
}

func TestPasteFocusOrModalIgnored(t *testing.T) {
	modalCases := []struct {
		name  string
		apply func(*modelUI)
	}{
		{"no input focus", func(ui *modelUI) { ui.focus = focusSessions }},
		{"help", func(ui *modelUI) { focusComposer(ui); ui.showHelp = true }},
		{"picker", func(ui *modelUI) {
			focusComposer(ui)
			picker := dialog.NewFeaturePicker(nil, 80, 24)
			ui.picker = &picker
		}},
		{"confirm", func(ui *modelUI) {
			focusComposer(ui)
			confirm := dialog.NewConfirm("title", "message", []string{"yes"})
			ui.confirm = &confirm
		}},
		{"clarify", func(ui *modelUI) {
			focusComposer(ui)
			clarify := dialog.NewClarify("question", []string{"answer"})
			ui.clarify = &clarify
		}},
	}
	for _, tt := range modalCases {
		t.Run(tt.name, func(t *testing.T) {
			ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
			ui.input.SetValue("keep")
			tt.apply(ui)
			ui.Update(tea.PasteMsg{Content: " pasted"})
			if got := ui.input.Value(); got != "keep" {
				t.Fatalf("composer mutated to %q", got)
			}
		})
	}
}

func TestPasteMultilineFormRetainsLineBreaks(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	ui.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	ui.input.SetValue("Multiline feature")
	ui.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	ui.Update(tea.PasteMsg{Content: "first line\r\nsecond line"})
	if got := ui.input.Value(); got != "first line\nsecond line" {
		t.Fatalf("form composer value = %q", got)
	}
	ui.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := ui.create.desc; got != "first line\nsecond line" {
		t.Fatalf("stored description = %q", got)
	}
}

func TestLongPasteContentLimitShowsVisibleError(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	ui.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	focusComposer(ui)
	ui.input.SetValue("keep")
	ui.Update(tea.PasteMsg{Content: strings.Repeat("x\n", 10000)})
	if ui.err == nil {
		t.Fatal("expected visible UI error for rejected paste")
	}
	if got := ui.input.Value(); got != "keep" {
		t.Fatalf("rejected paste mutated composer to %q", got)
	}
	if !strings.Contains(ui.renderFooter(), "10,000-row") {
		t.Fatalf("footer did not surface paste error: %q", ui.renderFooter())
	}
}

func TestComposerHeightAndFrameBounds(t *testing.T) {
	for _, height := range []int{40, 14, 8, 7, 6} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
			ui.Update(tea.WindowSizeMsg{Width: 100, Height: height})
			focusComposer(ui)
			ui.input.SetValue(strings.Join([]string{"1", "2", "3", "4", "5", "6", "7", "8"}, "\n"))
			wantCap := min(6, max(1, height/3))
			if got := ui.input.Height(); got != wantCap {
				t.Fatalf("composer height = %d, want %d", got, wantCap)
			}
			if got := lipgloss.Height(ui.View().Content); got > height {
				t.Fatalf("rendered height = %d, terminal height = %d", got, height)
			}
		})
	}
}

func TestComposerLayoutCountsAttachmentAndCompletionRows(t *testing.T) {
	ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
	focusComposer(ui)
	attachment, err := attachments.SaveBytes(ui.ws, "F001-x", "note.txt", []byte("note"))
	if err != nil {
		t.Fatal(err)
	}
	ui.attach.Add(attachment)
	ui.input.SetValue("/r")
	view := ui.View()
	cursor := view.Cursor
	if cursor == nil {
		t.Fatal("expected focused composer cursor")
	}
	if ui.inputYOffset < 2 {
		t.Fatalf("inputYOffset = %d, expected attachment and completion rows", ui.inputYOffset)
	}
	if cursor.Y != ui.inputY0 {
		t.Fatalf("cursor Y = %d, textarea origin = %d", cursor.Y, ui.inputY0)
	}
}

func TestCursorMultilineAndWrapCoordinates(t *testing.T) {
	tests := []struct {
		name  string
		width int
		value string
	}{
		{"hard newline", 80, "first\nsecond"},
		{"soft wrap", 24, strings.Repeat("word ", 20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := newTestModel(t, model.Feature{ID: "F001-x", Title: "X", Status: "todo"})
			ui.Update(tea.WindowSizeMsg{Width: tt.width, Height: 24})
			focusComposer(ui)
			ui.input.SetValue(tt.value)
			relative := ui.input.Cursor()
			if relative == nil || relative.Y < 1 {
				t.Fatalf("textarea cursor = %+v, want a later visual row", relative)
			}
			view := ui.View()
			if view.Cursor == nil {
				t.Fatal("expected terminal cursor")
			}
			if got, want := view.Cursor.Y, ui.inputY0+relative.Y; got != want {
				t.Fatalf("terminal cursor Y = %d, want %d", got, want)
			}
		})
	}
}
