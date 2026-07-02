import { create } from "zustand";
import type {
    AgentSession,
    BottomTab,
    ExecutionPlan,
    Feature,
    GitDiff,
    InspectorTab,
    MemoryNode,
    Project,
    Review,
    Task,
    TaskStatus,
    TestRun,
} from "../domain/models";
import type { WorkbenchSnapshot } from "../services/nativeBackend";
import { transitionTask } from "./taskMachine";

type WorkbenchState = {
    project: Project;
    features: Feature[];
    tasks: Task[];
    plans: ExecutionPlan[];
    reviews: Review[];
    memoryNodes: MemoryNode[];
    sessions: AgentSession[];
    diffs: Record<string, GitDiff>;
    testRuns: Record<string, TestRun>;
    selectedTaskId: string;
    boardFilter: string;
    inspectorTab: InspectorTab;
    bottomTab: BottomTab;
    hydrate(snapshot: WorkbenchSnapshot): void;
    selectTask(taskId: string): void;
    setBoardFilter(value: string): void;
    setInspectorTab(tab: InspectorTab): void;
    setBottomTab(tab: BottomTab): void;
    createTask(title: string): void;
    moveTask(taskId: string, status: TaskStatus): void;
    approvePlan(taskId: string): void;
    persistPlan(plan: ExecutionPlan): void;
    requestChanges(taskId: string): void;
    runTests(taskId: string): void;
    approveMerge(taskId: string): void;
    persistDiff(diff: GitDiff): void;
    persistTestRun(test: TestRun): void;
    persistReview(review: Review): void;
    persistMemory(nodes: MemoryNode[]): void;
    upsertSession(session: AgentSession): void;
    appendSessionOutput(
        taskId: string,
        sessionId: string,
        output: string,
    ): void;
};

const emptyProject: Project = {
    id: "local",
    name: "Thanos",
    rootPath: "",
    repos: [],
    settings: {},
};

export const useWorkbenchStore = create<WorkbenchState>((set, get) => ({
    project: emptyProject,
    features: [],
    tasks: [],
    plans: [],
    reviews: [],
    memoryNodes: [],
    sessions: [],
    diffs: {},
    testRuns: {},
    selectedTaskId: "",
    boardFilter: "",
    inspectorTab: "plan",
    bottomTab: "chat",
    hydrate: (snapshot) =>
        set((state) => {
            const selectedExists = snapshot.tasks.some(
                (task) => task.id === state.selectedTaskId,
            );
            return {
                project: snapshot.project,
                features: snapshot.features,
                tasks: snapshot.tasks,
                plans: snapshot.plans,
                reviews: snapshot.reviews,
                memoryNodes: snapshot.memoryNodes,
                sessions: snapshot.sessions,
                selectedTaskId: selectedExists
                    ? state.selectedTaskId
                    : snapshot.tasks[0]?.id ?? "",
            };
        }),
    selectTask: (taskId) => set({ selectedTaskId: taskId }),
    setBoardFilter: (boardFilter) => set({ boardFilter }),
    setInspectorTab: (inspectorTab) => set({ inspectorTab }),
    setBottomTab: (bottomTab) => set({ bottomTab }),
    createTask: (title) =>
        set((state) => ({
            tasks: [
                ...state.tasks,
                {
                    id: `T-${100 + state.tasks.length}`,
                    featureId: "F-100",
                    title,
                    description: "New task created from board flow.",
                    status: "backlog",
                    priority: "P2",
                    assignedAgent: "Unassigned",
                    executorProfile: "codex-local",
                    worktreePath: "",
                    branchName: "",
                    reviewApproved: false,
                    testsPassed: false,
                    updatedAt: "now",
                    tags: ["new"],
                    progress: 0,
                },
            ],
        })),
    moveTask: (taskId, status) =>
        set((state) => ({
            tasks: state.tasks.map((task) => {
                if (task.id !== taskId) return task;
                const result = transitionTask(task, status);
                return result.ok ? result.task : task;
            }),
        })),
    approvePlan: (taskId) =>
        set((state) => ({
            plans: state.plans.map((plan) =>
                plan.taskId === taskId
                    ? { ...plan, approvalStatus: "approved" }
                    : plan,
            ),
            tasks: state.tasks.map((task) => {
                if (task.id !== taskId) return task;
                const approved = { ...task, reviewApproved: true };
                const result = transitionTask(approved, "ready");
                return result.ok ? result.task : approved;
            }),
        })),
    persistPlan: (plan) =>
        set((state) => ({
            plans: state.plans.some((item) => item.taskId === plan.taskId)
                ? state.plans.map((item) =>
                      item.taskId === plan.taskId ? plan : item,
                  )
                : [...state.plans, plan],
        })),
    requestChanges: (taskId) =>
        set((state) => ({
            tasks: state.tasks.map((task) => {
                if (task.id !== taskId) return task;
                const result = transitionTask(task, "planning");
                return result.ok ? result.task : task;
            }),
        })),
    runTests: (taskId) =>
        set((state) => ({
            tasks: state.tasks.map((task) =>
                task.id === taskId
                    ? { ...task, testsPassed: true, updatedAt: "now" }
                    : task,
            ),
        })),
    approveMerge: (taskId) =>
        set((state) => ({
            tasks: state.tasks.map((task) => {
                if (task.id !== taskId) return task;
                const result = transitionTask(task, "done");
                return result.ok ? result.task : task;
            }),
        })),
    persistDiff: (diff) =>
        set((state) => ({ diffs: { ...state.diffs, [diff.taskId]: diff } })),
    persistTestRun: (test) =>
        set((state) => ({
            testRuns: { ...state.testRuns, [test.taskId]: test },
            tasks: state.tasks.map((task) =>
                task.id === test.taskId
                    ? { ...task, testsPassed: test.status === "passed" }
                    : task,
            ),
        })),
    persistReview: (review) =>
        set((state) => ({
            reviews: state.reviews.some((item) => item.taskId === review.taskId)
                ? state.reviews.map((item) =>
                      item.taskId === review.taskId ? review : item,
                  )
                : [...state.reviews, review],
            tasks: state.tasks.map((task) =>
                task.id === review.taskId
                    ? { ...task, reviewApproved: review.status === "approved" }
                    : task,
            ),
        })),
    persistMemory: (nodes) => set({ memoryNodes: nodes }),
    upsertSession: (session) =>
        set((state) => {
            const index = state.sessions.findIndex(
                (item) =>
                    item.id === session.id || item.taskId === session.taskId,
            );
            if (index < 0) return { sessions: [...state.sessions, session] };
            const next = [...state.sessions];
            next[index] = session;
            return { sessions: next };
        }),
    appendSessionOutput: (taskId, sessionId, output) => {
        const existing = get().sessions.find(
            (session) => session.id === sessionId || session.taskId === taskId,
        );
        if (!existing) return;
        get().upsertSession({
            ...existing,
            id: sessionId,
            output: [...existing.output, output],
        });
    },
}));

export function selectedTask(state: WorkbenchState): Task | null {
    return (
        state.tasks.find((task) => task.id === state.selectedTaskId) ??
        state.tasks[0] ??
        null
    );
}

export function planFor(task: Task, state: WorkbenchState) {
    return (
        state.plans.find((plan) => plan.taskId === task.id) ?? {
            id: `plan-${task.id}`,
            taskId: task.id,
            summary: "No execution plan has been saved for this task.",
            steps: [],
            risks: [],
            filesToTouch: [],
            testStrategy: [],
            approvalStatus: "draft" as const,
        }
    );
}

export function reviewFor(task: Task, state: WorkbenchState) {
    return (
        state.reviews.find((review) => review.taskId === task.id) ?? {
            id: `review-${task.id}`,
            taskId: task.id,
            diffSummary: "",
            changedFiles: [],
            testResults: [],
            reviewerNotes: "",
            status: "pending" as const,
        }
    );
}

export function sessionFor(task: Task, state: WorkbenchState): AgentSession {
    return (
        state.sessions.find((session) => session.taskId === task.id) ?? {
            id: `session-${task.id}`,
            taskId: task.id,
            agentType: "coder",
            provider: "codex",
            command: "codex",
            status: "idle",
            output: [],
        }
    );
}
