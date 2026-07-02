import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import type { ExecutionPlan, Task } from "../domain/models";
import { NativeBackend } from "../services/nativeBackend";
import { useWorkbenchStore } from "../state/workbenchStore";

const backend = new NativeBackend();

export function useAgentSessionFlow() {
  const upsertSession = useWorkbenchStore((state) => state.upsertSession);
  const appendSessionOutput = useWorkbenchStore((state) => state.appendSessionOutput);

  useEffect(() => {
    let active = true;
    const unsubs: Array<() => void> = [];
    backend.onAgentOutput((taskId, sessionId, data) => active && appendSessionOutput(taskId, sessionId, data)).then((unsub) => unsubs.push(unsub));
    backend.onAgentExit((taskId, sessionId, code) => active && appendSessionOutput(taskId, sessionId, `process exited with code ${code}`)).then((unsub) => unsubs.push(unsub));
    return () => {
      active = false;
      unsubs.forEach((unsub) => unsub());
    };
  }, [appendSessionOutput]);

  return {
    async start(task: Task, agentType: "planner" | "coder" = "coder") {
      if (agentType === "coder" && !task.reviewApproved) {
        upsertSession({
          id: `session-${task.id}-coder`,
          taskId: task.id,
          agentType: "coder",
          provider: "codex",
          command: "codex",
          status: "failed",
          output: ["coder blocked: approve the execution plan first"],
        });
        return;
      }
      const prepared = agentType === "coder" ? await backend.prepareWorktree(task) : null;
      const runnable = prepared ? { ...task, branchName: prepared.branchName, worktreePath: prepared.worktreePath } : task;
      const session = agentType === "planner" || runnable.worktreePath ? await backend.startAgentRole(runnable, agentType) : null;
      if (session) {
        upsertSession(session);
      } else {
        upsertSession({
          id: `session-${task.id}-${agentType}`,
          taskId: task.id,
          agentType,
          provider: agentType === "planner" ? "claude-code" : "codex",
          command: agentType === "planner" ? "claude" : "codex",
          status: "running",
          output: [`${agentType} session started`],
        });
      }
    },
    async stop() {
      await backend.stopAgent();
    },
    async savePlan(plan: ExecutionPlan) {
      return await backend.saveExecutionPlan(plan);
    },
    async approvePlan(taskId: string) {
      return await backend.approveExecutionPlan(taskId);
    },
    async collectDiff(task: Task) {
      return await backend.collectGitDiff(task);
    },
    async runTests(task: Task) {
      return await backend.runTaskTests(task);
    },
    async saveReview(review: import("../domain/models").Review) {
      return await backend.saveReview(review);
    },
    async approveReview(taskId: string) {
      return await backend.approveReview(taskId);
    },
    async searchMemory(query: string) {
      return await backend.searchMemory(query);
    },
    async writeMemory(node: import("../domain/models").MemoryNode) {
      return await backend.writeMemoryNode(node);
    },
  };
}

export function XtermPanel({ output }: { output: string[] }) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<XTerm | null>(null);

  useEffect(() => {
    if (!hostRef.current || terminalRef.current) return;
    const term = new XTerm({
      convertEol: true,
      cursorBlink: true,
      fontFamily: "Menlo, Monaco, Consolas, monospace",
      fontSize: 12,
      theme: { background: "#020617", foreground: "#E5E7EB" },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(hostRef.current);
    fit.fit();
    terminalRef.current = term;
    return () => term.dispose();
  }, []);

  useEffect(() => {
    const terminal = terminalRef.current;
    if (!terminal) return;
    terminal.clear();
    terminal.write(output.join("\r\n") || "waiting for native agent CLI session");
  }, [output]);

  return <div ref={hostRef} className="h-full min-h-0 overflow-hidden rounded-xl bg-slate-950" />;
}
