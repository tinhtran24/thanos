import { Bot } from "lucide-react";

export function AgentAvatar({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-text-muted">
      <span className="grid h-5 w-5 place-items-center rounded-full border border-slate-700 bg-slate-800 text-text-main">
        <Bot size={14} />
      </span>
      {label}
    </span>
  );
}
