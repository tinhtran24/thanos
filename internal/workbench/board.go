package workbench

import "sort"

type BoardColumn struct {
	Status TaskStatus `json:"status"`
	Title  string     `json:"title"`
	Tasks  []Task     `json:"tasks"`
}

var BoardOrder = []TaskStatus{
	TaskBacklog,
	TaskPlanning,
	TaskWaitingApproval,
	TaskReady,
	TaskRunning,
	TaskInReview,
	TaskBlocked,
	TaskDone,
	TaskFailed,
}

func GroupTasks(tasks []Task) []BoardColumn {
	byStatus := make(map[TaskStatus][]Task, len(BoardOrder))
	for _, task := range tasks {
		byStatus[task.Status] = append(byStatus[task.Status], task)
	}
	columns := make([]BoardColumn, 0, len(BoardOrder))
	for _, status := range BoardOrder {
		items := append([]Task(nil), byStatus[status]...)
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Priority != items[j].Priority {
				return items[i].Priority < items[j].Priority
			}
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		})
		columns = append(columns, BoardColumn{Status: status, Title: titleForStatus(status), Tasks: items})
	}
	return columns
}

func titleForStatus(status TaskStatus) string {
	switch status {
	case TaskBacklog:
		return "Backlog"
	case TaskPlanning:
		return "Planning"
	case TaskWaitingApproval:
		return "Waiting Approval"
	case TaskReady:
		return "Ready"
	case TaskRunning:
		return "Running"
	case TaskInReview:
		return "In Review"
	case TaskBlocked:
		return "Blocked"
	case TaskDone:
		return "Done"
	case TaskFailed:
		return "Failed"
	default:
		return string(status)
	}
}
