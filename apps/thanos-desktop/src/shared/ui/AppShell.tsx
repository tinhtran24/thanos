import { Bell, Bot, Boxes, Brain, CircleDot, Cpu, FolderKanban, GitBranch, Kanban, LayoutDashboard, Plus, Search, Settings } from "lucide-react";
import thanosLogo from "../../../src-tauri/imgs/logo/thanos-logo.png";
import type { Project } from "../../domain/models";
import { SidebarItem } from "./SidebarItem";

export function AppShell({ project, children }: { project: Project; children: React.ReactNode }) {
  return (
    <div className="grid h-screen grid-cols-[15rem_minmax(0,1fr)] overflow-hidden bg-bg-app text-text-main">
      <aside className="flex min-h-0 flex-col gap-4 border-r border-slate-800 bg-bg-sidebar p-3">
        <img src={thanosLogo} alt="Thanos" className="h-9 w-32 object-contain object-left" />
        <nav className="grid gap-1">
          <SidebarItem icon={LayoutDashboard} label="Workbench" active />
          <SidebarItem icon={Kanban} label="Board" />
          <SidebarItem icon={FolderKanban} label="Projects" />
          <SidebarItem icon={Brain} label="Memory" />
          <SidebarItem icon={Bot} label="Agents" />
          <SidebarItem icon={Cpu} label="Executors" />
          <SidebarItem icon={Settings} label="Settings" />
        </nav>
        <div className="mt-auto rounded-xl border border-slate-800 bg-slate-900/80 p-3 shadow-lg shadow-black/20">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Bot size={18} />
            Thanos Dev
          </div>
          <p className="mt-1 text-xs text-text-muted">Local-first workspace</p>
        </div>
      </aside>
      <main className="grid min-h-0 grid-rows-[3.75rem_minmax(0,1fr)]">
        <header className="flex items-center justify-between gap-3 border-b border-slate-800 bg-slate-950/70 px-4 backdrop-blur">
          <div className="flex items-center gap-2">
            <button className="inline-flex items-center gap-2 rounded-lg border border-slate-800 bg-slate-900/80 px-3 py-2 text-sm">
              <Boxes size={16} />
              {project.name}
            </button>
            <button className="inline-flex items-center gap-2 rounded-lg border border-slate-800 bg-slate-900/80 px-3 py-2 text-sm text-blue-info">
              <GitBranch size={16} />
              main + worktrees
            </button>
          </div>
          <div className="flex items-center gap-2 text-sm text-text-muted">
            <CircleDot size={14} className="text-green-success" />
            4 Agents Online
          </div>
          <div className="flex items-center gap-2">
            <button className="inline-flex items-center gap-2 rounded-lg border border-slate-800 bg-slate-900/80 px-3 py-2 text-sm text-text-muted">
              <Search size={16} />
              Search
            </button>
            <button className="rounded-lg border border-slate-800 bg-slate-900/80 p-2 text-text-muted">
              <Bell size={16} />
            </button>
            <button className="inline-flex items-center gap-2 rounded-lg bg-purple-primary px-3 py-2 text-sm font-medium text-white hover:bg-purple-hover">
              <Plus size={16} />
              New Task
            </button>
          </div>
        </header>
        {children}
      </main>
    </div>
  );
}
