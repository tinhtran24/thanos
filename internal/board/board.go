package board

import "github.com/tinhtran/thanos/internal/workbench"

type Column = workbench.BoardColumn

func Columns(tasks []workbench.Task) []Column {
	return workbench.GroupTasks(tasks)
}
