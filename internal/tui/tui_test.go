package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	view := ui.View()
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
