import { Check, Clock3, FlaskConical, FolderTree, GitCompare, Globe, LayoutPanelTop, MessageSquare, Play, RotateCcw, ScrollText, Square, Terminal } from "lucide-react";
import type { BottomTab, InspectorTab } from "../domain/models";
import { EmptyState } from "../shared/ui/EmptyState";
import { FlowTabs } from "../shared/ui/FlowTabs";
import { StatusBadge } from "../shared/ui/StatusBadge";
import { selectedTask, planFor, reviewFor, sessionFor, useWorkbenchStore } from "../state/workbenchStore";
import { useAgentSessionFlow, XtermPanel } from "./AgentSessionFlow";

const detailTabs: Array<{ id: InspectorTab; label: string; icon: typeof LayoutPanelTop }> = [
  { id: "overview", label: "Overview", icon: LayoutPanelTop },
  { id: "plan", label: "Plan", icon: ScrollText },
  { id: "files", label: "Files", icon: FolderTree },
  { id: "changes", label: "Changes", icon: GitCompare },
  { id: "terminal", label: "Terminal", icon: Terminal },
  { id: "browser", label: "Browser", icon: Globe },
  { id: "tests", label: "Tests", icon: FlaskConical },
  { id: "timeline", label: "Timeline", icon: Clock3 },
  { id: "chat", label: "Chat", icon: MessageSquare },
];

const bottomTabs: Array<{ id: BottomTab; label: string; icon: typeof MessageSquare }> = [
  { id: "chat", label: "Chat", icon: MessageSquare },
  { id: "terminal", label: "Terminal", icon: Terminal },
  { id: "timeline", label: "Timeline", icon: Clock3 },
  { id: "logs", label: "Logs", icon: ScrollText },
];

export function TaskWorkbenchFlow() {
  return <TaskWorkbenchMain />;
}

export function TaskWorkbenchMain() {
  const state = useWorkbenchStore();
  const task = selectedTask(state);
  const agentFlow = useAgentSessionFlow();
  if (!task) {
    return (
      <section className="grid h-full min-h-0 place-items-center border-t border-slate-800 bg-bg-app p-4">
        <EmptyState label="No task selected" />
      </section>
    );
  }
  const plan = planFor(task, state);
  const review = reviewFor(task, state);
  const session = sessionFor(task, state);
  const approvePlan = async () => {
    const saved = await agentFlow.savePlan(plan);
    if (saved) state.persistPlan(saved);
    const approved = await agentFlow.approvePlan(task.id);
    if (approved) state.persistPlan(approved);
    state.approvePlan(task.id);
  };
  const collectDiff = async () => {
    const diff = await agentFlow.collectDiff(task);
    if (diff) state.persistDiff(diff);
  };
  const runTests = async () => {
    const test = await agentFlow.runTests(task);
    if (test) state.persistTestRun(test);
    else state.runTests(task.id);
  };
  const approveReview = async () => {
    const diff = state.diffs[task.id];
    const test = state.testRuns[task.id];
    const draft = {
      id: `review-${task.id}`,
      taskId: task.id,
      diffSummary: diff?.summary || review.diffSummary,
      changedFiles: diff?.changedFiles.map((file) => file.path) || review.changedFiles,
      testResults: test ? [{ command: test.command, status: test.status, output: test.stdout || test.stderr }] : review.testResults,
      reviewerNotes: "User approved review from workbench.",
      status: "pending" as const,
    };
    const saved = await agentFlow.saveReview(draft);
    if (saved) state.persistReview(saved);
    const approved = await agentFlow.approveReview(task.id);
    if (approved) state.persistReview(approved);
    const memory = await agentFlow.searchMemory(task.title.split(" ")[0] || task.id);
    if (memory.length) state.persistMemory(memory);
  };

  return (
    <section className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] border-t border-slate-800 bg-bg-app">
      <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-slate-800 bg-slate-950/80 p-4 backdrop-blur">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold">{task.title}</h2>
            <StatusBadge status={task.status} />
          </div>
          <p className="mt-1 text-sm text-text-muted">{task.id} · {task.description}</p>
        </div>
        <div className="flex items-center gap-2">
          <Action icon={Play} label="Planner" onClick={() => agentFlow.start(task, "planner")} />
          <Action icon={Check} label="Approve" onClick={approvePlan} primary />
          <Action icon={RotateCcw} label="Changes" onClick={() => state.requestChanges(task.id)} />
          <Action icon={GitCompare} label="Diff" onClick={collectDiff} />
          <Action icon={Play} label="Run Tests" onClick={runTests} />
          <Action icon={Check} label="Review" onClick={approveReview} />
          <Action icon={Play} label="Coder" onClick={() => agentFlow.start(task, "coder")} />
          <Action icon={Square} label="Stop" onClick={() => agentFlow.stop()} danger />
        </div>
      </header>
      <div className="min-h-0 overflow-y-auto p-4">
        <div className="grid grid-cols-3 gap-3">
          <Panel title="Execution Plan" meta={plan.approvalStatus}>
            <ol className="grid gap-2 text-sm">
              {plan.steps.map((step) => <li key={step.id}><span className="font-medium">{step.title}</span><p className="text-xs text-text-muted">{step.description}</p></li>)}
            </ol>
          </Panel>
          <Panel title="Files" meta="planned">
            <ul className="grid gap-2 text-sm text-text-muted">{plan.filesToTouch.map((file) => <li key={file}>{file}</li>)}</ul>
          </Panel>
          <Panel title="Git Changes" meta="isolated">
            <pre className="max-h-40 overflow-auto whitespace-pre-wrap text-xs text-text-muted">{state.diffs[task.id]?.summary || review.diffSummary || "Collect diff after coder changes."}</pre>
            <ul className="mt-3 grid gap-2 text-sm">{(state.diffs[task.id]?.changedFiles.map((file) => file.path) || review.changedFiles).map((file) => <li key={file}><span className="text-green-success">•</span> {file}</li>)}</ul>
          </Panel>
          <Panel title="Terminal" meta={session.status}>
            <XtermPanel output={session.output} />
          </Panel>
          <Panel title="Browser" meta="preview">
            <div className="rounded-xl bg-slate-950/70 p-4 text-sm"><strong>Shopping Cart</strong><p className="mt-2 text-text-muted">Preview attaches after runtime starts.</p></div>
          </Panel>
          <Panel title="Tests" meta={task.testsPassed ? "passed" : "pending"}>
            <p className="text-sm text-text-muted">{state.testRuns[task.id]?.status || "Run tests from the workbench when implementation is ready."}</p>
            <pre className="mt-2 max-h-32 overflow-auto whitespace-pre-wrap text-xs text-text-muted">{state.testRuns[task.id]?.stdout || state.testRuns[task.id]?.stderr || ""}</pre>
          </Panel>
        </div>
      </div>
    </section>
  );
}

export function TaskRightSidebar() {
  const state = useWorkbenchStore();
  const task = selectedTask(state);
  if (!task) {
    return (
      <aside className="min-h-0 overflow-y-auto border-l border-slate-800 bg-slate-900/80">
        <FlowTabs tabs={detailTabs} active={state.inspectorTab} onChange={state.setInspectorTab} />
        <div className="p-4">
          <EmptyState label="No task selected" />
        </div>
      </aside>
    );
  }
  return (
    <aside className="min-h-0 overflow-y-auto border-l border-slate-800 bg-slate-900/80">
      <FlowTabs tabs={detailTabs} active={state.inspectorTab} onChange={state.setInspectorTab} />
      <div className="grid gap-3 p-4 text-sm">
        <h3 className="text-base font-semibold">{task.title}</h3>
        <p className="text-text-muted">{task.description}</p>
        {state.memoryNodes.map((node) => (
          <div key={node.id} className="rounded-xl border border-slate-800 bg-bg-card p-3">
            <p className="font-medium">{node.title}</p>
            <p className="mt-1 text-xs text-text-muted">{node.type}</p>
          </div>
        ))}
      </div>
    </aside>
  );
}

export function TaskBottomPanel() {
  const state = useWorkbenchStore();
  const task = selectedTask(state);
  const session = task ? sessionFor(task, state) : null;
  return (
    <section className="min-h-0 border-t border-slate-800 bg-slate-950/70">
      <FlowTabs tabs={bottomTabs} active={state.bottomTab} onChange={state.setBottomTab} />
      <div className="h-[calc(100%-2.5rem)] min-h-0 p-3 text-sm text-text-muted">
        {state.bottomTab === "terminal" && session ? <XtermPanel output={session.output} /> : <div className="rounded-xl border border-slate-800 bg-bg-card p-3">{state.bottomTab} panel</div>}
      </div>
    </section>
  );
}

function Panel({ title, meta, children }: { title: string; meta: string; children: React.ReactNode }) {
  return (
    <article className="min-h-48 rounded-xl border border-slate-800 bg-slate-900/80 p-4 shadow-lg shadow-black/20">
      <header className="mb-3 flex items-center justify-between">
        <h3 className="text-base font-semibold">{title}</h3>
        <span className="text-xs text-text-muted">{meta}</span>
      </header>
      {children}
    </article>
  );
}

function Action({ icon: Icon, label, onClick, primary, danger }: { icon: typeof Check; label: string; onClick: () => void; primary?: boolean; danger?: boolean }) {
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm ${
        primary ? "border-purple-primary bg-purple-primary text-white hover:bg-purple-hover" : danger ? "border-red-danger/40 text-red-danger" : "border-slate-800 bg-slate-900/80 text-text-main"
      }`}
    >
      <Icon size={16} />
      {label}
    </button>
  );
}
