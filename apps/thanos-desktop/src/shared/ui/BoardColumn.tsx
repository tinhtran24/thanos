import type { LucideIcon } from "lucide-react";
import type { Task, TaskStatus } from "../../domain/models";
import { EmptyState } from "./EmptyState";
import { TaskCard } from "./TaskCard";

export function BoardColumn({
  id,
  title,
  icon: Icon,
  tasks,
  selectedTaskId,
  onSelectTask,
  renderSortableTask,
}: {
  id: TaskStatus;
  title: string;
  icon: LucideIcon;
  tasks: Task[];
  selectedTaskId: string;
  onSelectTask: (taskId: string) => void;
  renderSortableTask: (task: Task, active: boolean, onSelectTask: (taskId: string) => void) => React.ReactNode;
}) {
  return (
    <section className="flex min-h-0 w-64 shrink-0 flex-col rounded-xl border border-slate-800 bg-slate-900/80">
      <header className="flex items-center justify-between border-b border-slate-800 p-3">
        <span className="inline-flex items-center gap-2 text-sm font-medium">
          <Icon size={16} />
          {title}
        </span>
        <span className="rounded-lg bg-slate-800 px-2 py-1 text-xs text-text-muted">{tasks.length}</span>
      </header>
      <div className="grid min-h-0 flex-1 content-start gap-3 overflow-y-auto p-3" data-column-id={id}>
        {tasks.length === 0 ? <EmptyState label="No tasks" /> : tasks.map((task) => renderSortableTask(task, selectedTaskId === task.id, onSelectTask))}
      </div>
    </section>
  );
}
