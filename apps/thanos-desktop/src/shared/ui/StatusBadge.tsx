import type { TaskStatus } from "../../domain/models";

const tone: Record<TaskStatus, string> = {
  backlog: "border-slate-700 bg-slate-900 text-slate-300",
  planning: "border-blue-info/30 bg-blue-info/10 text-blue-info",
  waiting_approval: "border-yellow-warning/30 bg-yellow-warning/10 text-yellow-warning",
  ready: "border-blue-info/30 bg-blue-info/10 text-blue-info",
  running: "border-green-success/30 bg-green-success/10 text-green-success",
  in_review: "border-orange-review/30 bg-orange-review/10 text-orange-review",
  blocked: "border-red-danger/30 bg-red-danger/10 text-red-danger",
  done: "border-green-success/30 bg-green-success/10 text-green-success",
  failed: "border-red-danger/30 bg-red-danger/10 text-red-danger",
};

export function StatusBadge({ status }: { status: TaskStatus }) {
  return <span className={`rounded-lg border px-2 py-1 text-xs font-medium ${tone[status]}`}>{status.replace("_", " ")}</span>;
}
