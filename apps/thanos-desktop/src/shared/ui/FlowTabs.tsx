import type { LucideIcon } from "lucide-react";

export function FlowTabs<T extends string>({
  tabs,
  active,
  onChange,
}: {
  tabs: Array<{ id: T; label: string; icon: LucideIcon }>;
  active: T;
  onChange: (id: T) => void;
}) {
  return (
    <div className="flex items-center gap-1 border-b border-slate-800">
      {tabs.map(({ id, label, icon: Icon }) => (
        <button
          key={id}
          onClick={() => onChange(id)}
          className={`inline-flex items-center gap-2 border-b px-3 py-2 text-sm ${
            active === id ? "border-purple-primary text-text-main" : "border-transparent text-text-muted hover:text-text-main"
          }`}
        >
          <Icon size={16} />
          {label}
        </button>
      ))}
    </div>
  );
}
