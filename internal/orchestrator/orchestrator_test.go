package orchestrator

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
	"github.com/tinhtran/thanos/internal/runner"
	"github.com/tinhtran/thanos/internal/workspace"
)

type workflowRunner struct {
	id             string
	rejectReview   bool
	rejectTesting  bool
	reviewAttempts int
	testAttempts   int
	multiEC        bool
	clarify        bool
}

func (r *workflowRunner) Run(_ context.Context, root string, _ model.Runner, prompt string, _, _ io.Writer) error {
	ws := workspace.Open(root)
	st, _ := ws.ReadState(r.id)
	ecDir := ""
	if st.ECTotal > 1 && st.ECIndex >= 1 {
		ecDir = fmt.Sprintf("ec-%d", st.ECIndex)
	}
	write := func(name, content string) {
		path := filepath.Join(root, ".thanos", r.id, ecDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			panic(err)
		}
	}
	switch {
	case contains(prompt, "You are the QA Analyst and Planner"):
		if r.multiEC {
			path := filepath.Join(root, ".thanos", r.id, "execution-plan.yaml")
			_ = os.WriteFile(path, []byte("chunks:\n  - index: 1\n    id: ec-1-first\n    title: First\n    status: todo\n  - index: 2\n    id: ec-2-second\n    title: Second\n    status: todo\n"), 0o644)
		}
	case contains(prompt, "You are the Developer"):
		if r.clarify && !ws.ArtifactExists(r.id, filepath.Join(ecDir, "clarify-answer.md")) {
			write("clarify.json", `{"question":"Which DB?","options":["postgres","sqlite"]}`)
			return nil
		}
		write("implementation-note.md", "# Implementation Note\n## Verification\n- go test: PASS")
	case contains(prompt, "You are the independent Reviewer"):
		r.reviewAttempts++
		verdict := "PASS"
		decision := "APPROVED"
		if r.rejectReview && r.reviewAttempts == 1 {
			verdict = "FAIL"
			decision = "CHANGES REQUESTED"
		}
		write("review-report.md", "# Code Review\n## Decision\n"+decision+"\n## Verdict\n"+verdict)
	case contains(prompt, "You are the Tester"):
		r.testAttempts++
		verdict := "PASS"
		if r.rejectTesting && r.testAttempts == 1 {
			verdict = "FAIL"
		}
		write("test-report.md", "# Test Report\n## ECs\n### EC-1\n- Status: Passed\n## Verdict\n"+verdict)
	case contains(prompt, "You are the Completion and Memory agent"):
		base := filepath.Join(root, ".thanos", r.id)
		_ = os.WriteFile(filepath.Join(base, "final-report.md"), []byte("# Final Report"), 0o644)
		_ = os.WriteFile(filepath.Join(base, "retro-learnings.json"), []byte(`{"learnings":[]}`), 0o644)
		_ = os.WriteFile(filepath.Join(base, "feature-memory.json"), []byte(`{"business_rules":[],"architectural_decisions":[],"affected_paths":[]}`), 0o644)
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

func newWorkflow(t *testing.T, id string, r runner.Runner) (*workspace.Workspace, Orchestrator) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.Open(root)
	if err := ws.Init(model.Config{
		Project:       model.Project{Name: "test"},
		DefaultRunner: "fake",
		Runners:       map[string]model.Runner{"fake": {Command: "fake"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ws.SaveFeature(model.Feature{ID: id, Title: "Test", Status: "todo"}); err != nil {
		t.Fatal(err)
	}
	return ws, Orchestrator{Workspace: ws, Runner: r, Stdout: io.Discard, Stderr: io.Discard}
}

func TestRunUsesPlanDevelopmentReviewTestingFlow(t *testing.T) {
	r := &workflowRunner{id: "F001-test"}
	ws, orch := newWorkflow(t, r.id, r)
	if err := os.RemoveAll(filepath.Join(ws.DotDir(), "prompts")); err != nil {
		t.Fatal(err)
	}
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.DotDir(), "prompts", r.id+"-ec0-planner.md")); err != nil {
		t.Fatalf("planner prompt was not recreated: %v", err)
	}
	st, err := ws.ReadState(r.id)
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != model.PhaseDone {
		t.Fatalf("phase = %s, want done", st.Phase)
	}
	feature, _ := ws.LoadFeature(r.id)
	if feature.Status != "done" {
		t.Fatalf("feature status = %q, want done", feature.Status)
	}
	for _, name := range []string{"implementation-note.md", "review-report.md", "test-report.md", "final-report.md", "feature-memory.json"} {
		if !ws.ArtifactExists(r.id, name) {
			t.Fatalf("missing workflow artifact %s", name)
		}
	}
}

func TestWorkItemNameUsesFeatureAndEC(t *testing.T) {
	got := workItemName(
		model.Feature{ID: "F001-password-validation", Title: "Password validation"},
		model.State{ECIndex: 1},
		&model.ExecutionChunk{Index: 1, Title: "Implement ABC"},
	)
	if got != "Feature 001-EC1 Implement ABC" {
		t.Fatalf("work item name = %q", got)
	}
}

func TestValidateExecutionPlanContract(t *testing.T) {
	feature := model.Feature{Acceptance: []string{"backend rejects short password", "frontend shows error"}}
	valid := []model.ExecutionChunk{
		{Index: 1, ID: "ec-1-backend", Acceptance: []string{"backend rejects short password"}},
		{Index: 2, ID: "ec-2-frontend", Dependencies: []string{"ec-1-backend"}, Acceptance: []string{"frontend shows error"}},
	}
	if err := validateExecutionPlan(feature, valid); err != nil {
		t.Fatal(err)
	}
	invalid := append([]model.ExecutionChunk(nil), valid...)
	invalid[1].Dependencies = []string{"missing"}
	if err := validateExecutionPlan(feature, invalid); err == nil {
		t.Fatal("expected invalid dependency error")
	}
	invalid = append([]model.ExecutionChunk(nil), valid...)
	invalid[1].Acceptance = []string{"backend rejects short password"}
	if err := validateExecutionPlan(feature, invalid); err == nil {
		t.Fatal("expected acceptance coverage error")
	}
}

func TestFailedReviewReopensDevelopmentAndStops(t *testing.T) {
	r := &workflowRunner{id: "F001-review", rejectReview: true}
	ws, orch := newWorkflow(t, r.id, r)
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ := ws.ReadState(r.id)
	if st.Phase != model.PhaseCode || st.Role != model.RoleCoder {
		t.Fatalf("state after failed review = %+v", st)
	}
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ = ws.ReadState(r.id)
	if st.Phase != model.PhaseDone {
		t.Fatalf("phase after corrected review = %s", st.Phase)
	}
}

func TestFailedTestingReopensDevelopmentAndStops(t *testing.T) {
	r := &workflowRunner{id: "F001-test-fail", rejectTesting: true}
	ws, orch := newWorkflow(t, r.id, r)
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ := ws.ReadState(r.id)
	if st.Phase != model.PhaseCode || st.Role != model.RoleCoder {
		t.Fatalf("state after failed testing = %+v", st)
	}
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ = ws.ReadState(r.id)
	if st.Phase != model.PhaseDone {
		t.Fatalf("phase after corrected testing = %s", st.Phase)
	}
}

func TestRunCyclesEachChunkThroughReviewAndTesting(t *testing.T) {
	r := &workflowRunner{id: "F001-multi", multiEC: true}
	ws, orch := newWorkflow(t, r.id, r)
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ := ws.ReadState(r.id)
	if st.Phase != model.PhaseDone || st.ECTotal != 2 {
		t.Fatalf("state = %+v", st)
	}
	for _, ec := range []string{"ec-1", "ec-2"} {
		for _, name := range []string{"implementation-note.md", "review-report.md", "test-report.md"} {
			if !ws.ArtifactExists(r.id, filepath.Join(ec, name)) {
				t.Fatalf("missing %s/%s", ec, name)
			}
		}
	}
}

func TestClarifyPauseAndResume(t *testing.T) {
	r := &workflowRunner{id: "F001-clar", clarify: true}
	ws, orch := newWorkflow(t, r.id, r)
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatalf("run should pause cleanly, got %v", err)
	}
	st, _ := ws.ReadState(r.id)
	if st.Active || st.Reason != "needs clarification" {
		t.Fatalf("paused state = %+v", st)
	}
	if err := ws.WriteArtifact(r.id, "clarify-answer.md", "postgres"); err != nil {
		t.Fatal(err)
	}
	st.Active = true
	st.Reason = ""
	if err := ws.WriteState(st); err != nil {
		t.Fatal(err)
	}
	if err := orch.Run(context.Background(), r.id, ""); err != nil {
		t.Fatal(err)
	}
	st, _ = ws.ReadState(r.id)
	if st.Phase != model.PhaseDone {
		t.Fatalf("phase after resume = %s", st.Phase)
	}
}
