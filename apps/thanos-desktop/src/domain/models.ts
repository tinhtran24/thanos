export type TaskStatus =
    | "backlog"
    | "planning"
    | "waiting_approval"
    | "ready"
    | "running"
    | "in_review"
    | "blocked"
    | "done"
    | "failed";

export type Priority = "P0" | "P1" | "P2" | "P3";
export type InspectorTab = "overview" | "plan" | "files" | "changes" | "terminal" | "browser" | "tests" | "timeline" | "chat" | "memory";
export type BottomTab = "chat" | "terminal" | "timeline" | "logs";

export type Project = {
    id: string;
    name: string;
    rootPath: string;
    repos: string[];
    settings: Record<string, string>;
};

export type Feature = {
    id: string;
    projectId: string;
    title: string;
    description: string;
    status: "backlog" | "active" | "done";
    planGraphId?: string;
    createdAt: string;
};

export type Task = {
    id: string;
    featureId: string;
    parentTaskId?: string;
    title: string;
    description: string;
    status: TaskStatus;
    priority: Priority;
    assignedAgent: string;
    executorProfile: string;
    worktreePath: string;
    branchName: string;
    reviewApproved: boolean;
    testsPassed: boolean;
    updatedAt: string;
    tags: string[];
    progress: number;
};

export type ExecutionPlan = {
    id: string;
    taskId: string;
    summary: string;
    steps: Array<{
        id: string;
        title: string;
        description: string;
        status: string;
    }>;
    risks: string[];
    filesToTouch: string[];
    testStrategy: string[];
    approvalStatus:
        "draft" | "pending" | "approved" | "rejected" | "changes_requested";
};

export type AgentSession = {
    id: string;
    taskId: string;
    agentType: "planner" | "coder" | "reviewer" | "tester" | "utility";
    provider: string;
    command: string;
    status: "idle" | "starting" | "running" | "stopping" | "stopped" | "failed";
    ptySessionId?: string;
    conversationLogPath?: string;
    output: string[];
};

export type Review = {
    id: string;
    taskId: string;
    diffSummary: string;
    changedFiles: string[];
    testResults: Array<{ command: string; status: string; output: string }>;
    reviewerNotes: string;
    status: "pending" | "approved" | "rejected" | "changes_requested";
};

export type GitDiff = {
    taskId: string;
    summary: string;
    changedFiles: Array<{ path: string; status: string }>;
    patch: string;
};

export type TestRun = {
    taskId: string;
    command: string;
    status: "passed" | "failed";
    stdout: string;
    stderr: string;
    code: number | null;
};

export type MemoryNode = {
    id: string;
    projectId: string;
    type:
        | "feature"
        | "decision"
        | "architecture"
        | "file"
        | "task"
        | "bug"
        | "convention";
    title: string;
    content: string;
    links: string[];
    createdAt: string;
};

export type WorkbenchEvent =
    | { type: "onCreateTask"; title: string }
    | { type: "onSelectTask"; taskId: string }
    | { type: "onApprovePlan"; taskId: string }
    | { type: "onRequestChanges"; taskId: string; notes?: string }
    | { type: "onStartAgent"; taskId: string }
    | { type: "onStopAgent"; taskId: string }
    | { type: "onRunTests"; taskId: string }
    | { type: "onApproveMerge"; taskId: string };

export type FlowEvents = {
    onCreateTask(title: string): void;
    onSelectTask(taskId: string): void;
    onApprovePlan(taskId: string): void;
    onRequestChanges(taskId: string, notes?: string): void;
    onStartAgent(taskId: string): void;
    onStopAgent(taskId: string): void;
    onRunTests(taskId: string): void;
    onApproveMerge(taskId: string): void;
};
