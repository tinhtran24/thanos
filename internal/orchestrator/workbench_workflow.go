package orchestrator

import (
	"fmt"
	"time"

	"github.com/tinhtran/thanos/internal/workbench"
)

type WorkbenchTransition struct {
	From      workbench.TaskStatus `json:"from"`
	To        workbench.TaskStatus `json:"to"`
	TaskID    string               `json:"task_id"`
	Reason    string               `json:"reason,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
}

var workbenchTransitions = map[workbench.TaskStatus][]workbench.TaskStatus{
	workbench.TaskBacklog:         {workbench.TaskPlanning},
	workbench.TaskPlanning:        {workbench.TaskWaitingApproval, workbench.TaskBlocked, workbench.TaskFailed},
	workbench.TaskWaitingApproval: {workbench.TaskPlanning, workbench.TaskReady, workbench.TaskBlocked},
	workbench.TaskReady:           {workbench.TaskRunning, workbench.TaskPlanning, workbench.TaskBlocked},
	workbench.TaskRunning:         {workbench.TaskInReview, workbench.TaskBlocked, workbench.TaskFailed},
	workbench.TaskInReview:        {workbench.TaskReady, workbench.TaskBlocked, workbench.TaskDone, workbench.TaskFailed},
	workbench.TaskBlocked:         {workbench.TaskPlanning, workbench.TaskReady, workbench.TaskRunning, workbench.TaskFailed},
	workbench.TaskFailed:          {workbench.TaskPlanning, workbench.TaskReady},
	workbench.TaskDone:            {},
}

func CanMoveWorkbenchTask(task workbench.Task, to workbench.TaskStatus) error {
	if task.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if to == "" {
		return fmt.Errorf("task %s transition target is empty", task.ID)
	}
	allowed := false
	for _, candidate := range workbenchTransitions[task.Status] {
		if candidate == to {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("invalid task transition %s -> %s", task.Status, to)
	}
	switch to {
	case workbench.TaskReady:
		if task.Status == workbench.TaskWaitingApproval && !task.ReviewApproved {
			return fmt.Errorf("task %s cannot become ready before plan approval", task.ID)
		}
	case workbench.TaskRunning:
		if task.WorktreePath == "" {
			return fmt.Errorf("task %s cannot run without an isolated worktree", task.ID)
		}
		if task.BranchName == "" {
			return fmt.Errorf("task %s cannot run without an isolated branch", task.ID)
		}
	case workbench.TaskDone:
		if !task.ReviewApproved {
			return fmt.Errorf("task %s cannot be done before review approval", task.ID)
		}
		if !task.TestsPassed {
			return fmt.Errorf("task %s cannot be done before tests pass", task.ID)
		}
	}
	return nil
}

func MoveWorkbenchTask(task workbench.Task, to workbench.TaskStatus, reason string) (workbench.Task, WorkbenchTransition, error) {
	if err := CanMoveWorkbenchTask(task, to); err != nil {
		return task, WorkbenchTransition{}, err
	}
	now := time.Now().UTC()
	transition := WorkbenchTransition{
		From:      task.Status,
		To:        to,
		TaskID:    task.ID,
		Reason:    reason,
		CreatedAt: now,
	}
	task.Status = to
	task.UpdatedAt = now
	return task, transition, nil
}
