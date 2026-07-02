import { DndContext, type DragEndEvent, PointerSensor, useDroppable, useSensor, useSensors } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { CheckCircle2, GitPullRequest, Inbox, ListTodo, PlayCircle, ShieldCheck } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Task, TaskStatus } from "../domain/models";
import { BoardColumn } from "../shared/ui/BoardColumn";
import { TaskCard } from "../shared/ui/TaskCard";
import { useWorkbenchStore } from "../state/workbenchStore";

const columns: Array<{ id: TaskStatus; title: string; icon: LucideIcon }> = [
  { id: "backlog", title: "Backlog", icon: Inbox },
  { id: "planning", title: "Planning", icon: ListTodo },
  { id: "waiting_approval", title: "Waiting Approval", icon: ShieldCheck },
  { id: "running", title: "In Progress", icon: PlayCircle },
  { id: "in_review", title: "In Review", icon: GitPullRequest },
  { id: "done", title: "Done", icon: CheckCircle2 },
];

export function BoardFlow() {
  const tasks = useWorkbenchStore((state) => state.tasks);
  const selectedTaskId = useWorkbenchStore((state) => state.selectedTaskId);
  const filter = useWorkbenchStore((state) => state.boardFilter);
  const setFilter = useWorkbenchStore((state) => state.setBoardFilter);
  const selectTask = useWorkbenchStore((state) => state.selectTask);
  const moveTask = useWorkbenchStore((state) => state.moveTask);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));
  const visible = filter.trim()
    ? tasks.filter((task) => `${task.id} ${task.title} ${task.description} ${task.tags.join(" ")}`.toLowerCase().includes(filter.toLowerCase()))
    : tasks;

  function onDragEnd(event: DragEndEvent) {
    const taskId = String(event.active.id);
    const status = event.over?.id ? String(event.over.id) as TaskStatus : null;
    if (status) moveTask(taskId, status);
  }

  return (
    <section className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] gap-3 p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold">Board</h1>
          <p className="text-sm text-text-muted">Approval-gated agent tasks across isolated worktrees</p>
        </div>
        <input
          value={filter}
          onChange={(event) => setFilter(event.target.value)}
          className="h-9 w-64 rounded-lg border border-slate-800 bg-slate-900/80 px-3 text-sm text-text-main placeholder:text-text-muted"
          placeholder="Search tasks..."
        />
      </div>
      <DndContext sensors={sensors} onDragEnd={onDragEnd}>
        <div className="flex min-h-0 gap-3 overflow-x-auto pb-2">
          {columns.map((column) => {
            const columnTasks = visible.filter((task) => task.status === column.id);
            return (
              <DroppableColumn key={column.id} id={column.id}>
                <SortableContext items={columnTasks.map((task) => task.id)} strategy={verticalListSortingStrategy}>
                  <BoardColumn
                    {...column}
                    tasks={columnTasks}
                    selectedTaskId={selectedTaskId}
                    onSelectTask={selectTask}
                    renderSortableTask={(task, active, onSelect) => <SortableTask key={task.id} task={task} active={active} onSelect={onSelect} />}
                  />
                </SortableContext>
              </DroppableColumn>
            );
          })}
        </div>
      </DndContext>
    </section>
  );
}

function DroppableColumn({ id, children }: { id: TaskStatus; children: React.ReactNode }) {
  const { setNodeRef } = useDroppable({ id });
  return <div ref={setNodeRef}>{children}</div>;
}

function SortableTask({ task, active, onSelect }: { task: Task; active: boolean; onSelect: (taskId: string) => void }) {
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id: task.id });
  return (
    <button className="text-left" onClick={() => onSelect(task.id)}>
      <TaskCard
        task={task}
        active={active}
        setNodeRef={setNodeRef}
        dragAttributes={attributes}
        dragListeners={listeners}
        style={{ transform: CSS.Transform.toString(transform), transition }}
      />
    </button>
  );
}
