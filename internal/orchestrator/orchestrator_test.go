package orchestrator

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/workspace"
)

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, root string, _ model.Runner, prompt string, _, _ io.Writer) error {
	id := "F001-test"
	base := filepath.Join(root, ".thanos", id)
	switch {
	case contains(prompt, "You are the Designer"):
		os.WriteFile(filepath.Join(base, "task-brief.md"), []byte("brief"), 0o644)
		os.WriteFile(filepath.Join(base, "acceptance-criteria.md"), []byte("AC-1"), 0o644)
		os.WriteFile(filepath.Join(base, "test-strategy.yaml"), []byte("verify_commands:\n  - go test ./..."), 0o644)
	case contains(prompt, "Design Reviewer"):
		os.WriteFile(filepath.Join(base, "design-review-report.md"), []byte("## Verdict\nPASS"), 0o644)
	case contains(prompt, "You are the Coder"):
		os.MkdirAll(filepath.Join(base, "rounds", "round-1"), 0o755)
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "coder-report.md"), []byte("done"), 0o644)
	case contains(prompt, "You are the Reviewer"):
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "review-report.md"), []byte("## Verdict\nPASS"), 0o644)
	case contains(prompt, "You are the Tester"):
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "test-report.md"), []byte("## Verdict\nPASS"), 0o644)
	case contains(prompt, "You are the Deep Reviewer"):
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "deep-review-report.md"), []byte("## Verdict\nPASS"), 0o644)
	case contains(prompt, "You are the Acceptor"):
		os.WriteFile(filepath.Join(base, "final-report.md"), []byte("# Final Report"), 0o644)
		os.WriteFile(filepath.Join(base, "retro-learnings.json"), []byte(`{"learnings":[]}`), 0o644)
	}
	return nil
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func TestRunToPendingReview(t *testing.T) {
	root := t.TempDir()
	ws := workspace.Open(root)
	config := model.Config{
		Project:       model.Project{Name: "test"},
		DefaultRunner: "fake",
		MaxRounds:     2,
		Runners:       map[string]model.Runner{"fake": {Command: "fake"}},
	}
	if err := ws.Init(config); err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveFeature(model.Feature{ID: "F001-test", Title: "Test", Status: "todo"}); err != nil {
		t.Fatal(err)
	}
	orch := Orchestrator{Workspace: ws, Runner: fakeRunner{}, Stdout: io.Discard, Stderr: io.Discard}
	if err := orch.Run(context.Background(), "F001", ""); err != nil {
		t.Fatal(err)
	}
	state, err := ws.ReadState("F001-test")
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != model.PhasePending {
		t.Fatalf("phase = %s", state.Phase)
	}
}
