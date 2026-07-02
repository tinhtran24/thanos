import { Bot } from "lucide-react";
import type { DraggableAttributes } from "@dnd-kit/core";
import type { SyntheticListenerMap } from "@dnd-kit/core/dist/hooks/utilities";
import type { Task } from "../../domain/models";
import { AgentAvatar } from "./AgentAvatar";
import { PriorityBadge } from "./PriorityBadge";

export function TaskCard({ task, active, dragAttributes, dragListeners, setNodeRef, style }: {
  task: Task;
  active: boolean;
  dragAttributes?: DraggableAttributes;
  dragListeners?: SyntheticListenerMap;
  setNodeRef?: (node: HTMLElement | null) => void;
  style?: React.CSSProperties;
}) {
  return (
    <article
      ref={setNodeRef}
      style={style}
      {...dragAttributes}
      {...dragListeners}
      className={`cursor-grab rounded-xl border bg-bg-card p-3 shadow-lg shadow-black/20 ${active ? "border-purple-primary" : "border-slate-800"}`}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs text-text-muted">{task.id}</span>
        <PriorityBadge priority={task.priority} />
      </div>
      <h3 className="mt-2 text-sm font-semibold text-text-main">{task.title}</h3>
      <p className="mt-1 line-clamp-2 text-xs text-text-muted">{task.description}</p>
      <div className="mt-3 flex flex-wrap gap-1">
        {task.tags.map((tag) => (
          <span key={tag} className="rounded-lg bg-slate-800 px-2 py-1 text-xs text-slate-300">
            {tag}
          </span>
        ))}
      </div>
      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-800">
        <div className="h-full rounded-full bg-green-success" style={{ width: `${task.progress}%` }} />
      </div>
      <div className="mt-3 flex items-center justify-between gap-2">
        <AgentAvatar label={task.assignedAgent} />
        <Bot size={14} className="text-text-muted" />
      </div>
    </article>
  );
}
