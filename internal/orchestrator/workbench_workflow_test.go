package orchestrator

import (
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/workbench"
)

func TestWorkbenchTaskWorkflowRequiresPlanApprovalBeforeReady(t *testing.T) {
	task := workbench.Task{ID: "T-1", Status: workbench.TaskWaitingApproval}
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskReady, "approve plan"); err == nil || !strings.Contains(err.Error(), "plan approval") {
		t.Fatalf("expected plan approval gate, got %v", err)
	}

	task.ReviewApproved = true
	next, transition, err := MoveWorkbenchTask(task, workbench.TaskReady, "approve plan")
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != workbench.TaskReady || transition.From != workbench.TaskWaitingApproval || transition.To != workbench.TaskReady {
		t.Fatalf("unexpected transition: %#v %#v", next, transition)
	}
}

func TestWorkbenchTaskWorkflowRequiresWorktreeBeforeRunning(t *testing.T) {
	task := workbench.Task{ID: "T-1", Status: workbench.TaskReady, ReviewApproved: true}
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskRunning, "start coder"); err == nil || !strings.Contains(err.Error(), "worktree") {
		t.Fatalf("expected worktree gate, got %v", err)
	}

	task.WorktreePath = ".thanos/worktrees/T-1"
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskRunning, "start coder"); err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("expected branch gate, got %v", err)
	}
}

func TestWorkbenchTaskWorkflowDoneRequiresReviewAndTests(t *testing.T) {
	task := workbench.Task{ID: "T-1", Status: workbench.TaskInReview}
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskDone, "merge"); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected review gate, got %v", err)
	}

	task.ReviewApproved = true
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskDone, "merge"); err == nil || !strings.Contains(err.Error(), "tests") {
		t.Fatalf("expected tests gate, got %v", err)
	}

	task.TestsPassed = true
	if _, _, err := MoveWorkbenchTask(task, workbench.TaskDone, "merge"); err != nil {
		t.Fatal(err)
	}
}
