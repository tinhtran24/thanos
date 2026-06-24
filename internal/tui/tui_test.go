package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/workspace"
)

func TestViewRendersSessionFlowAndCapabilities(t *testing.T) {
	ws := workspace.Open(t.TempDir())
	config := model.Config{
		Project:       model.Project{Name: "example"},
		DefaultRunner: "codex",
		Runners: map[string]model.Runner{
			"codex": {Command: "codex"},
		},
		LSP: map[string]model.LSP{
			"go": {Command: "gopls"},
		},
		MCP: map[string]model.MCP{
			"github": {Type: "http", URL: "https://example.test/mcp"},
		},
	}
	if err := ws.Init(config); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-session-ui", Title: "Session UI", Status: "todo", Priority: "high"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := ws.WriteState(model.State{
		FeatureID: feature.ID, Phase: model.PhaseCode, Role: model.RoleCoder,
		Round: 1, MaxRounds: 3, Active: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	ui, err := newModel(context.Background(), ws, "test")
	if err != nil {
		t.Fatal(err)
	}
	ui.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	view := ui.View().Content
	for _, want := range []string{"THANOS", "Session UI", "CODE INTELLIGENCE", "LSP · go", "MCP · github"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view does not contain %q:\n%s", want, view)
		}
	}
}

func TestCycleRunnerPersistsSessionSelection(t *testing.T) {
	ws := workspace.Open(t.TempDir())
	config := model.Config{
		DefaultRunner: "codex",
		Runners: map[string]model.Runner{
			"claude": {Command: "claude"},
			"codex":  {Command: "codex"},
		},
	}
	if err := ws.Init(config); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-switch", Title: "Switch runner", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}

	ui, err := newModel(context.Background(), ws, "test")
	if err != nil {
		t.Fatal(err)
	}
	ui.cycleRunner()
	got, err := ws.LoadFeature(feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Runner != "claude" {
		t.Fatalf("runner = %q, want claude", got.Runner)
	}
}

func TestArrowKeysMoveSessionSelection(t *testing.T) {
	ws := workspace.Open(t.TempDir())
	if err := ws.Init(model.Config{
		DefaultRunner: "codex",
		Runners: map[string]model.Runner{
			"codex": {Command: "codex"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, feature := range []model.Feature{
		{ID: "F001-first", Title: "First", Status: "todo"},
		{ID: "F002-second", Title: "Second", Status: "todo"},
	} {
		if err := ws.SaveFeature(feature); err != nil {
			t.Fatal(err)
		}
	}
	ui, err := newModel(context.Background(), ws, "test")
	if err != nil {
		t.Fatal(err)
	}

	ui.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ui.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", ui.cursor)
	}
	ui.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ui.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", ui.cursor)
	}
	ui.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if ui.cursor != 1 {
		t.Fatalf("cursor = %d after numeric selection, want 1", ui.cursor)
	}
	// Clicking a feature row in the right sidebar selects it. Render once so the
	// tree row geometry is known, then click the first feature-0 row.
	ui.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	_ = ui.View()
	row0 := -1
	for i, r := range ui.treeRows {
		if r.FeatureIndex == 0 {
			row0 = i
			break
		}
	}
	if row0 < 0 {
		t.Fatal("no feature-0 row in the sidebar tree")
	}
	ui.Update(tea.MouseClickMsg{X: ui.sidebarX0, Y: ui.treeY0 + row0, Button: tea.MouseLeft})
	if ui.cursor != 0 {
		t.Fatalf("cursor = %d after sidebar click, want 0", ui.cursor)
	}
}

func TestActivityWriterStreamsLiveAgentOutput(t *testing.T) {
	updates := make(chan string, 1)
	writer := &activityWriter{target: updates}
	if _, err := writer.Write([]byte("\x1b[31mchecking files\x1b[0m\n")); err != nil {
		t.Fatal(err)
	}
	update := <-updates

	ui := &modelUI{
		running:  true,
		activity: make(chan string),
		width:    100,
		height:   30,
	}
	ui.Update(activityMsg(update))
	if ui.lastOutput != "checking files\n" {
		t.Fatalf("activity = %q", ui.lastOutput)
	}
}
