import type { Task, TaskStatus } from "../domain/models";

const transitions: Record<TaskStatus, TaskStatus[]> = {
  backlog: ["planning"],
  planning: ["waiting_approval", "blocked", "failed"],
  waiting_approval: ["planning", "ready", "blocked"],
  ready: ["running", "planning", "blocked"],
  running: ["in_review", "blocked", "failed"],
  in_review: ["ready", "blocked", "done", "failed"],
  blocked: ["planning", "ready", "running", "failed"],
  failed: ["planning", "ready"],
  done: [],
};

export type TransitionResult = { ok: true; task: Task } | { ok: false; reason: string };

export function canTransition(task: Task, to: TaskStatus): string | null {
  if (!transitions[task.status].includes(to)) {
    return `Invalid transition ${task.status} -> ${to}`;
  }
  if (to === "ready" && task.status === "waiting_approval" && !task.reviewApproved) {
    return "Plan approval is required before the task can become ready.";
  }
  if (to === "running" && (!task.worktreePath || !task.branchName)) {
    return "An isolated worktree and branch are required before agent execution.";
  }
  if (to === "done" && !task.reviewApproved) {
    return "Review approval is required before merge approval.";
  }
  if (to === "done" && !task.testsPassed) {
    return "Passing tests are required before merge approval.";
  }
  return null;
}

export function transitionTask(task: Task, to: TaskStatus): TransitionResult {
  const reason = canTransition(task, to);
  if (reason) {
    return { ok: false, reason };
  }
  return { ok: true, task: { ...task, status: to, updatedAt: "now" } };
}
