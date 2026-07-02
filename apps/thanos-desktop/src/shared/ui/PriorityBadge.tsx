import { Flag } from "lucide-react";
import type { Priority } from "../../domain/models";

const tone: Record<Priority, string> = {
  P0: "bg-red-danger/15 text-red-danger",
  P1: "bg-orange-review/15 text-orange-review",
  P2: "bg-yellow-warning/15 text-yellow-warning",
  P3: "bg-green-success/15 text-green-success",
};

export function PriorityBadge({ priority }: { priority: Priority }) {
  return (
    <span className={`inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold ${tone[priority]}`}>
      <Flag size={14} />
      {priority}
    </span>
  );
}
