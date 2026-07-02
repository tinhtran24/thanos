import type { LucideIcon } from "lucide-react";

export function SidebarItem({ icon: Icon, label, active = false }: { icon: LucideIcon; label: string; active?: boolean }) {
  return (
    <button
      className={`flex h-9 w-full items-center gap-3 rounded-lg px-3 text-sm transition ${
        active ? "bg-purple-primary text-white" : "text-text-muted hover:bg-slate-900 hover:text-text-main"
      }`}
    >
      <Icon size={18} />
      <span>{label}</span>
    </button>
  );
}
