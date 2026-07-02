package workbench

import (
	"testing"
	"time"
)

func TestGroupTasksPreservesPhaseOneBoardOrder(t *testing.T) {
	now := time.Now()
	columns := GroupTasks([]Task{
		{ID: "T2", Status: TaskRunning, Priority: PriorityP2, UpdatedAt: now},
		{ID: "T1", Status: TaskBacklog, Priority: PriorityP0, UpdatedAt: now},
	})

	if got, want := columns[0].Status, TaskBacklog; got != want {
		t.Fatalf("first column = %s, want %s", got, want)
	}
	if got, want := columns[4].Status, TaskRunning; got != want {
		t.Fatalf("running column = %s, want %s", got, want)
	}
	if len(columns[0].Tasks) != 1 || columns[0].Tasks[0].ID != "T1" {
		t.Fatalf("backlog tasks not grouped: %#v", columns[0].Tasks)
	}
}
