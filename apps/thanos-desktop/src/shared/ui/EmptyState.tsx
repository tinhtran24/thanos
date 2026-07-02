import { Inbox } from "lucide-react";

export function EmptyState({ label }: { label: string }) {
  return (
    <div className="grid place-items-center rounded-xl border border-dashed border-slate-800 bg-slate-950/40 p-4 text-center text-sm text-text-muted">
      <Inbox size={18} className="mb-2" />
      {label}
    </div>
  );
}
