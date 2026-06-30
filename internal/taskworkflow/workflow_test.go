package taskworkflow

import (
	"strings"
	"testing"

	"github.com/tinhtran/thanos/internal/model"
)

func TestTaskTransitionsFollowReviewFirstWorkflow(t *testing.T) {
	task := model.Task{ID: "T001-demo", Status: model.TaskBacklog, PlanPath: ".thanos/plans/T001-demo.md"}
	for _, status := range []model.TaskStatus{
		model.TaskAnalysis,
		model.TaskPlan,
		model.TaskDev,
		model.TaskReview,
		model.TaskTest,
	} {
		next, err := Transition(task, status)
		if err != nil {
			t.Fatalf("%s -> %s: %v", task.Status, status, err)
		}
		task = next
	}
	task.ReviewApproved = true
	task.TestsPassed = true
	if _, err := Transition(task, model.TaskDone); err != nil {
		t.Fatal(err)
	}
}

func TestTaskDoneRequiresReviewAndTests(t *testing.T) {
	task := model.Task{ID: "T001-demo", Status: model.TaskTest}
	if _, err := Transition(task, model.TaskDone); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected review gate error, got %v", err)
	}
	task.ReviewApproved = true
	if _, err := Transition(task, model.TaskDone); err == nil || !strings.Contains(err.Error(), "tests") {
		t.Fatalf("expected test gate error, got %v", err)
	}
}

func TestTaskDevRequiresPlan(t *testing.T) {
	task := model.Task{ID: "T001-demo", Status: model.TaskPlan}
	if _, err := Transition(task, model.TaskDev); err == nil || !strings.Contains(err.Error(), "plan") {
		t.Fatalf("expected plan gate error, got %v", err)
	}
}

func TestTaskCannotSkipFromBacklogToDev(t *testing.T) {
	task := model.Task{ID: "T001-demo", Status: model.TaskBacklog, PlanPath: ".thanos/plans/T001-demo.md"}
	if _, err := Transition(task, model.TaskDev); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid transition error, got %v", err)
	}
}
