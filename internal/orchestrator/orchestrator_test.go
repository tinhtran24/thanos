package orchestrator

import (
	"context"
	"fmt"
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
	case contains(prompt, "You are the Coder"):
		os.MkdirAll(filepath.Join(base, "rounds", "round-1"), 0o755)
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "coder-report.md"), []byte("done"), 0o644)
	case contains(prompt, "You are the Tester"):
		os.WriteFile(filepath.Join(base, "rounds", "round-1", "test-report.md"), []byte("## Verdict\nPASS"), 0o644)
	case contains(prompt, "You are the Overview agent"):
		os.WriteFile(filepath.Join(base, "final-report.md"), []byte("# Final Report"), 0o644)
		os.WriteFile(filepath.Join(base, "retro-learnings.json"), []byte(`{"learnings":[]}`), 0o644)
		os.WriteFile(filepath.Join(base, "feature-memory.json"), []byte(`{"business_rules":[],"architectural_decisions":[],"affected_paths":[]}`), 0o644)
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

type rejectOnceRunner struct{ id string }

func (r rejectOnceRunner) Run(_ context.Context, root string, _ model.Runner, prompt string, _, _ io.Writer) error {
	ws := workspace.Open(root)
	st, _ := ws.ReadState(r.id)
	base := filepath.Join(root, ".thanos", r.id)
	write := func(name, content string) {
		path := filepath.Join(base, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}
	round := fmt.Sprintf("rounds/round-%d", st.Round)
	switch {
	case contains(prompt, "You are the Coder"):
		write(round+"/coder-report.md", "done")
	case contains(prompt, "You are the Tester"):
		verdict := "FAIL"
		if st.Round == 2 {
			verdict = "PASS"
		}
		write(round+"/test-report.md", "## Verdict\n"+verdict)
	case contains(prompt, "You are the Overview agent"):
		write("final-report.md", "# Final")
		write("retro-learnings.json", `{"learnings":[]}`)
		write("feature-memory.json", `{"business_rules":[],"architectural_decisions":[],"affected_paths":[]}`)
	}
	return nil
}

func TestFailedECTestReturnsToCoding(t *testing.T) {
	root := t.TempDir()
	ws := workspace.Open(root)
	if err := ws.Init(model.Config{DefaultRunner: "fake", MaxRounds: 2, Runners: map[string]model.Runner{"fake": {Command: "fake"}}}); err != nil {
		t.Fatal(err)
	}
	feature := model.Feature{ID: "F001-retry", Title: "Retry", Status: "todo"}
	if err := ws.SaveFeature(feature); err != nil {
		t.Fatal(err)
	}
	orch := Orchestrator{Workspace: ws, Runner: rejectOnceRunner{id: feature.ID}, Stdout: io.Discard, Stderr: io.Discard}
	if err := orch.Run(context.Background(), feature.ID, ""); err != nil {
		t.Fatal(err)
	}
	st, _ := ws.ReadState(feature.ID)
	if st.Phase != model.PhasePending || st.Round != 2 {
		t.Fatalf("state = %+v", st)
	}
	for _, name := range []string{
		"rounds/round-1/test-report.md",
		"rounds/round-2/coder-report.md",
		"rounds/round-2/test-report.md",
	} {
		if !ws.ArtifactExists(feature.ID, name) {
			t.Fatalf("missing retry artifact %s", name)
		}
	}
}

// multiECRunner writes EC-scoped artifacts, reading state.json to learn which
// chunk/round it is on. The planner emits a 2-chunk plan.
type multiECRunner struct{ id string }

func (r multiECRunner) Run(_ context.Context, root string, _ model.Runner, prompt string, _, _ io.Writer) error {
	ws := workspace.Open(root)
	st, _ := ws.ReadState(r.id)
	ecdir := ""
	if st.ECTotal > 1 && st.ECIndex >= 1 {
		ecdir = fmt.Sprintf("ec-%d", st.ECIndex)
	}
	write := func(name, content string) {
		p := filepath.Join(root, ".thanos", r.id, ecdir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	round := fmt.Sprintf("rounds/round-%d", st.Round)
	switch {
	case contains(prompt, "You are the Planner"):
		os.WriteFile(filepath.Join(root, ".thanos", r.id, "execution-plan.yaml"),
			[]byte("chunks:\n  - index: 1\n    id: ec-1-first\n    title: First\n    status: todo\n  - index: 2\n    id: ec-2-second\n    title: Second\n    status: todo\n"), 0o644)
	case contains(prompt, "You are the Coder"):
		write(round+"/coder-report.md", "done")
	case contains(prompt, "You are the Tester"):
		write(round+"/test-report.md", "## Verdict\nPASS")
	case contains(prompt, "You are the Overview agent"):
		base := filepath.Join(root, ".thanos", r.id)
		os.WriteFile(filepath.Join(base, "final-report.md"), []byte("# Final"), 0o644)
		os.WriteFile(filepath.Join(base, "retro-learnings.json"), []byte(`{"learnings":[]}`), 0o644)
		os.WriteFile(filepath.Join(base, "feature-memory.json"), []byte(`{"business_rules":[],"architectural_decisions":[],"affected_paths":[]}`), 0o644)
	}
	return nil
}

func TestRunCyclesEachChunkInOrder(t *testing.T) {
	root := t.TempDir()
	ws := workspace.Open(root)
	if err := ws.Init(model.Config{DefaultRunner: "fake", MaxRounds: 2, Runners: map[string]model.Runner{"fake": {Command: "fake"}}}); err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveFeature(model.Feature{ID: "F001-multi", Title: "Multi", Status: "todo"}); err != nil {
		t.Fatal(err)
	}
	orch := Orchestrator{Workspace: ws, Runner: multiECRunner{id: "F001-multi"}, Stdout: io.Discard, Stderr: io.Discard}
	if err := orch.Run(context.Background(), "F001-multi", ""); err != nil {
		t.Fatal(err)
	}
	st, _ := ws.ReadState("F001-multi")
	if st.Phase != model.PhasePending {
		t.Fatalf("phase = %s, want pending-review", st.Phase)
	}
	if st.ECTotal != 2 {
		t.Fatalf("ec total = %d, want 2", st.ECTotal)
	}
	// Both chunks ran their coding and EC-test cycle and are done.
	for _, ec := range []string{"ec-1", "ec-2"} {
		for _, f := range []string{"rounds/round-1/coder-report.md", "rounds/round-1/test-report.md"} {
			if !ws.ArtifactExists("F001-multi", filepath.Join(ec, f)) {
				t.Fatalf("missing %s/%s — chunk did not complete its cycle", ec, f)
			}
		}
	}
	plan, _ := ws.ReadPlan("F001-multi")
	for _, c := range plan.Chunks {
		if c.Status != "done" {
			t.Fatalf("chunk %s status = %q, want done", c.ID, c.Status)
		}
	}
}

// clarifyRunner has the coder ask a question (write clarify.json) until an
// answer exists, then proceed normally for the rest of the cycle.
type clarifyRunner struct{ id string }

func (r clarifyRunner) Run(_ context.Context, root string, _ model.Runner, prompt string, _, _ io.Writer) error {
	base := filepath.Join(root, ".thanos", r.id)
	w := func(name, content string) {
		os.MkdirAll(filepath.Dir(filepath.Join(base, name)), 0o755)
		os.WriteFile(filepath.Join(base, name), []byte(content), 0o644)
	}
	switch {
	case contains(prompt, "You are the Coder"):
		if _, err := os.Stat(filepath.Join(base, "clarify-answer.md")); err != nil {
			w("clarify.json", `{"question":"Which DB?","options":["postgres","sqlite"]}`)
			return nil // punt — ask first
		}
		w("rounds/round-1/coder-report.md", "done")
	case contains(prompt, "You are the Tester"):
		w("rounds/round-1/test-report.md", "## Verdict\nPASS")
	case contains(prompt, "You are the Overview agent"):
		w("final-report.md", "# Final")
		w("retro-learnings.json", `{"learnings":[]}`)
		w("feature-memory.json", `{"business_rules":[],"architectural_decisions":[],"affected_paths":[]}`)
	}
	return nil
}

func TestClarifyPauseAndResume(t *testing.T) {
	root := t.TempDir()
	ws := workspace.Open(root)
	if err := ws.Init(model.Config{DefaultRunner: "fake", MaxRounds: 2, Runners: map[string]model.Runner{"fake": {Command: "fake"}}}); err != nil {
		t.Fatal(err)
	}
	ws.SaveFeature(model.Feature{ID: "F001-clar", Title: "Clar", Status: "todo"})
	orch := Orchestrator{Workspace: ws, Runner: clarifyRunner{id: "F001-clar"}, Stdout: io.Discard, Stderr: io.Discard}

	// First run pauses for clarification (clean stop, no error).
	if err := orch.Run(context.Background(), "F001-clar", ""); err != nil {
		t.Fatalf("run should pause cleanly, got %v", err)
	}
	st, _ := ws.ReadState("F001-clar")
	if st.Active || st.Reason != "needs clarification" {
		t.Fatalf("expected paused-for-clarification state, got %+v", st)
	}
	if !ws.ArtifactExists("F001-clar", "clarify.json") {
		t.Fatal("expected clarify.json to exist")
	}

	// Answer it and resume → run completes to pending-review.
	ws.WriteArtifact("F001-clar", "clarify-answer.md", "postgres")
	st.Active = true
	st.Reason = ""
	ws.WriteState(st)
	if err := orch.Run(context.Background(), "F001-clar", ""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	st, _ = ws.ReadState("F001-clar")
	if st.Phase != model.PhasePending {
		t.Fatalf("after resume phase = %s, want pending-review", st.Phase)
	}
}
