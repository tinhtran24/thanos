package taskworkflow

import (
	"fmt"
	"time"

	"github.com/tinhtran/thanos/internal/model"
)

var transitions = map[model.TaskStatus][]model.TaskStatus{
	model.TaskBacklog:  {model.TaskAnalysis, model.TaskPlan},
	model.TaskAnalysis: {model.TaskPlan},
	model.TaskPlan:     {model.TaskDev, model.TaskBacklog},
	model.TaskDev:      {model.TaskReview},
	model.TaskReview:   {model.TaskDev, model.TaskTest, model.TaskPlan},
	model.TaskTest:     {model.TaskReview, model.TaskDone, model.TaskDev},
	model.TaskDone:     {model.TaskBacklog},
}

func CanTransition(task model.Task, to model.TaskStatus) error {
	if to == "" {
		return fmt.Errorf("task %s transition target is empty", task.ID)
	}
	allowed := false
	for _, candidate := range transitions[task.Status] {
		if candidate == to {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("invalid task transition %s -> %s", task.Status, to)
	}
	if to == model.TaskDev && task.PlanPath == "" {
		return fmt.Errorf("task %s cannot enter Dev without a plan", task.ID)
	}
	if to == model.TaskDone {
		if !task.ReviewApproved {
			return fmt.Errorf("task %s cannot be Done before review is approved", task.ID)
		}
		if !task.TestsPassed {
			return fmt.Errorf("task %s cannot be Done before tests pass", task.ID)
		}
	}
	return nil
}

func Transition(task model.Task, to model.TaskStatus) (model.Task, error) {
	if err := CanTransition(task, to); err != nil {
		return task, err
	}
	task.Status = to
	task.UpdatedAt = time.Now().UTC()
	return task, nil
}
