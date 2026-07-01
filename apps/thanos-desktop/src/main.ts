import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import thanosLogo from "../src-tauri/imgs/logo/thanos-logo.png";
import "./styles.css";

type CliResult = {
  command: string;
  stdout: string;
  stderr: string;
  code: number | null;
};

type WorkspaceInfo = {
  path: string;
  initialized: boolean;
  init_output?: CliResult | null;
};

type TaskStatus = "backlog" | "plan" | "execute" | "verify" | "done";

type Task = {
  id: string;
  title: string;
  description?: string;
  status: TaskStatus | string;
  priority?: string;
  parent_task_id?: string;
  subtasks?: string[];
  assigned_agent?: string;
  branch_name?: string;
  worktree_path?: string;
  updated_at?: string;
  plan_path?: string;
  log_path?: string;
  review_path?: string;
  test_result_path?: string;
  review_approved?: boolean;
  tests_passed?: boolean;
};

type ActivityEvent = {
  time: string;
  title: string;
  detail: string;
  tone: "ok" | "info" | "warn" | "fail";
};

type AgentCandidate = {
  name: string;
  command: string;
  installed: boolean;
  path: string | null;
  default_args: string[];
  role: string;
  allowed_steps: string[];
};

type AgentProfile = {
  name: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  role: string;
  allowed_steps: string[];
};

type SkillInfo = {
  name: string;
  source: string;
  roles: string[];
};

type ProjectConfig = {
  name: string;
  language: string;
  framework: string;
  default_runner: string;
  skills: SkillInfo[];
  mcp: string[];
};

const columns: Array<{ id: TaskStatus; title: string; hint: string }> = [
  { id: "backlog", title: "Backlog", hint: "" },
  { id: "plan", title: "Plan", hint: "Review" },
  { id: "execute", title: "Execute", hint: "" },
  { id: "verify", title: "Verify", hint: "Review" },
  { id: "done", title: "Done", hint: "" },
];

const ROLE_MATRIX: Array<{ key: string; label: string; steps: string[]; hint: string }> = [
  { key: "planner", label: "Planner", steps: ["plan"], hint: "Plan step" },
  { key: "coder", label: "Developer", steps: ["execute"], hint: "Execute step" },
  { key: "reviewer", label: "Reviewer", steps: ["verify"], hint: "Verify · review" },
  { key: "tester", label: "Tester", steps: ["verify"], hint: "Verify · tests" },
];

let term: Terminal | null = null;
let fitAddon: FitAddon | null = null;
let terminalRunning = false;

const state: {
  tasks: Task[];
  selectedTaskID: string;
  activeBottomTab: "activity" | "timeline" | "terminal";
  activeInspectorTab: "overview" | "plan" | "files" | "logs" | "reviews" | "tests";
  activity: ActivityEvent[];
  terminal: string[];
  agents: AgentCandidate[];
  profiles: AgentProfile[];
  skills: SkillInfo[];
  project: ProjectConfig | null;
  branch: string;
  selectedAgent: string;
  busy: boolean;
  modal: null | "task" | "skill";
  modalError: string;
  sidebarCollapsed: boolean;
  inspectorOpen: boolean;
  inspectorHidden: boolean;
} = {
  tasks: [],
  selectedTaskID: "",
  activeBottomTab: "activity",
  activeInspectorTab: "overview",
  activity: [{ time: "10:42", title: "Desktop ready", detail: "Connect a workspace to load CLI task state.", tone: "info" }],
  terminal: ["Waiting for CLI command output."],
  agents: [],
  profiles: [],
  skills: [],
  project: null,
  branch: "main",
  selectedAgent: localStorage.getItem("thanos.agent") ?? "custom",
  busy: false,
  modal: null,
  modalError: "",
  // Below the drawer breakpoint the sidebar starts closed so it doesn't cover the board.
  sidebarCollapsed: typeof window !== "undefined" && window.innerWidth <= 1024,
  inspectorOpen: false,
  inspectorHidden: false,
};

const DRAWER_BREAKPOINT = 1024;
const isNarrow = () => window.innerWidth <= DRAWER_BREAKPOINT;

const app = document.querySelector<HTMLDivElement>("#app");

if (!app) {
  throw new Error("app root missing");
}

app.innerHTML = renderAppShell();

const workspaceInput = byId<HTMLInputElement>("workspace");
const binaryInput = byId<HTMLInputElement>("binary");
const taskInput = byId<HTMLInputElement>("task-id");

restoreSettings();
wireEvents();
initTerminalEvents();
detectAgents();
renderAll();

function renderAppShell() {
  return `
    <div class="noise"></div>
    <div class="desktop-shell">
      <div id="scrim" class="scrim" aria-hidden="true"></div>
      ${renderSidebar()}
      <main class="workspace">
        ${renderTopbar()}
        <section class="workspace-grid">
          <section class="board-region">
            <div id="board" class="workflow-board"></div>
            <div class="bottom-panel">
              <div id="bottom-tabs" class="bottom-tabs"></div>
              <div id="bottom-content" class="bottom-content"></div>
              <div id="terminal-pane" class="terminal-pane" hidden>
                <div class="terminal-toolbar">
                  <span class="terminal-dot" id="terminal-dot"></span>
                  <span id="terminal-status">idle</span>
                  <span class="toolbar-spacer"></span>
                  <button id="terminal-stop" class="soft-button">Stop</button>
                  <button id="terminal-clear" class="soft-button">Clear</button>
                </div>
                <div id="terminal-host" class="terminal-host"></div>
              </div>
            </div>
          </section>
          <aside id="inspector" class="inspector"></aside>
        </section>
      </main>
      <div id="modal-root"></div>
      <div class="hidden-fields" hidden aria-hidden="true">
        <input id="workspace" />
        <input id="binary" value="thanos" />
      </div>
    </div>
  `;
}

function renderSidebar() {
  const nav = [
    ["kanban", "Kanban"],
    ["worktrees", "Worktrees"],
  ];
  return `
    <aside class="sidebar" aria-label="Thanos navigation">
      <div class="brand">
        <img class="brand-logo" src="${thanosLogo}" alt="Thanos — AI-powered development workflow" />
        <button id="sidebar-collapse" class="sidebar-toggle icon-button" title="Collapse sidebar">${icon("panel")}</button>
      </div>
      <nav class="nav-list">
        ${nav.map(([key, item]) => `<button class="nav-item ${item === "Kanban" ? "active" : ""}" data-command="/${key}">${icon(key)}<span>${item}</span></button>`).join("")}
      </nav>
      <div class="sidebar-scroll">
        <section class="sidebar-section workspace-card">
          <div class="section-head">
            <span class="sidebar-heading">Workspace</span>
            <button id="browse-workspace" class="ghost-button" title="Choose a project folder">${icon("folder")}<span>Browse</span></button>
          </div>
          <div class="connection-row">
            <span class="pulse"></span>
            <div><strong id="workspace-name">Not connected</strong><small id="branch-name">main</small></div>
          </div>
        </section>
        <section class="sidebar-section">
          <div class="sidebar-heading">Agent profiles</div>
          <div id="agent-list" class="agent-list"></div>
        </section>
        <section class="sidebar-section">
          <div class="section-head">
            <span class="sidebar-heading">Skills</span>
            <button id="add-skill" class="ghost-button" title="Add a skill to this project">${icon("plus")}<span>Add</span></button>
          </div>
          <div id="skill-list" class="skill-list"></div>
        </section>
      </div>
      <section class="sidebar-footer">
        <div class="profile-card">
          <span class="avatar profile-avatar">${icon("bot")}</span>
          <div>
            <strong id="profile-name">Local workspace</strong>
            <small id="profile-plan">Thanos CLI ${icon("star")}</small>
          </div>
        </div>
      </section>
    </aside>
  `;
}

function renderTopbar() {
  return `
    <header class="topbar">
      <div class="title-group">
        <button id="menu-button" class="icon-button" title="Toggle navigation">${icon("menu")}</button>
        <div>
          <h1>Kanban</h1>
          <p>Visualize & manage your AI workflow</p>
        </div>
      </div>
      <div class="topbar-actions">
        <label class="search-box">
          ${icon("search")}
          <input id="task-id" placeholder="Search tasks..." />
        </label>
        <button class="soft-button filter-button">${icon("sliders")}<span>All Status</span>${icon("chevron")}</button>
        <button class="soft-button filter-button">${icon("sort")}<span>Newest</span>${icon("chevron")}</button>
        <button id="new-task-toggle" class="primary-button">${icon("plus")}<span>New Task</span></button>
        <button id="refresh" class="icon-button" title="Refresh">${icon("refresh")}</button>
        <button id="inspector-toggle" class="icon-button" title="Show or hide the details panel">${icon("panel")}</button>
      </div>
    </header>
  `;
}

function renderModal() {
  if (!state.modal) {
    return "";
  }
  const connected = Boolean(workspaceInput.value.trim());
  const body = state.modal === "task" ? renderTaskForm(connected) : renderSkillForm(connected);
  const title = state.modal === "task" ? "New task" : "Add skill";
  const subtitle =
    state.modal === "task"
      ? "Create a task from the Backlog. Thanos runs it through Plan, Execute, and Verify."
      : "Register a skill so agents can use it during this project's workflow.";
  return `
    <div class="modal-overlay" data-modal-backdrop>
      <div class="modal-card" role="dialog" aria-modal="true" aria-label="${title}">
        <header class="modal-head">
          <div>
            <h2>${title}</h2>
            <p>${subtitle}</p>
          </div>
          <button class="icon-button" data-modal-close title="Close">${icon("close")}</button>
        </header>
        ${state.modalError ? `<div class="modal-error">${icon("dot")}<span>${escapeHtml(state.modalError)}</span></div>` : ""}
        ${body}
      </div>
    </div>
  `;
}

function renderTaskForm(connected: boolean) {
  if (!connected) {
    return renderModalDisconnected();
  }
  return `
    <form id="modal-form" class="modal-form">
      <label class="field">
        <span>Title</span>
        <input name="title" required autofocus placeholder="e.g. Add retry to the sync worker" />
      </label>
      <label class="field">
        <span>Description <em>optional · Markdown or plain text</em></span>
        <textarea name="description" class="mono" rows="6" placeholder="## Context&#10;Explain the task...&#10;&#10;## Acceptance criteria&#10;- [ ] ...&#10;- [ ] ..."></textarea>
        <em class="helper">Supports Markdown (headings, lists, code). Saved to the task description.</em>
      </label>
      <label class="field">
        <span>Priority</span>
        <select name="priority">
          <option value="high">High</option>
          <option value="medium" selected>Medium</option>
          <option value="low">Low</option>
        </select>
      </label>
      <footer class="modal-actions">
        <button type="button" class="soft-button" data-modal-close>Cancel</button>
        <button type="submit" class="primary-button">${icon("plus")}<span>Create task</span></button>
      </footer>
    </form>
  `;
}

function renderSkillForm(connected: boolean) {
  if (!connected) {
    return renderModalDisconnected();
  }
  return `
    <form id="modal-form" class="modal-form">
      <label class="field">
        <span>Source</span>
        <input name="source" required autofocus placeholder="owner/repo or a skills registry source" />
        <em class="helper">Passed to <code>thanos skill add &lt;source&gt;</code>.</em>
      </label>
      <div class="field-grid">
        <label class="field">
          <span>Skill name <em>optional</em></span>
          <input name="skill" placeholder="specific skill to install" />
        </label>
        <label class="field">
          <span>Roles <em>optional</em></span>
          <input name="roles" placeholder="planner, coder, reviewer" />
        </label>
      </div>
      <footer class="modal-actions">
        <button type="button" class="soft-button" data-modal-close>Cancel</button>
        <button type="submit" class="primary-button">${icon("plus")}<span>Add skill</span></button>
      </footer>
    </form>
  `;
}

function renderModalDisconnected() {
  return `
    <div class="modal-disconnected">
      <p>Connect a workspace first — use <strong>Browse</strong> in the sidebar to choose your project folder.</p>
      <footer class="modal-actions">
        <button type="button" class="soft-button" data-modal-close>Close</button>
        <button type="button" class="primary-button" id="modal-browse">${icon("folder")}<span>Browse</span></button>
      </footer>
    </div>
  `;
}


function renderAll() {
  byId("board").innerHTML = renderBoard();
  byId("inspector").innerHTML = renderInspector();
  byId("bottom-tabs").innerHTML = renderBottomTabs();
  const showTerminal = state.activeBottomTab === "terminal";
  byId("terminal-pane").toggleAttribute("hidden", !showTerminal);
  byId("bottom-content").toggleAttribute("hidden", showTerminal);
  if (showTerminal) {
    fitTerminal();
  } else {
    byId("bottom-content").innerHTML = renderBottomContent();
  }
  byId("agent-list").innerHTML = renderAgentProfiles();
  byId("skill-list").innerHTML = renderSkills();
  byId("modal-root").innerHTML = renderModal();
  const shell = document.querySelector(".desktop-shell");
  shell?.classList.toggle("sidebar-collapsed", state.sidebarCollapsed);
  shell?.classList.toggle("inspector-open", state.inspectorOpen && Boolean(selectedTask()));
  shell?.classList.toggle("inspector-hidden", state.inspectorHidden);
  document.body.toggleAttribute("aria-busy", state.busy);
  const workspace = workspaceInput.value.trim();
  const projectName = state.project?.name || (workspace ? basename(workspace) : "Not connected");
  byId("workspace-name").textContent = projectName;
  byId("branch-name").textContent = state.branch || "main";
  byId("profile-name").textContent = workspace ? basename(workspace) : "Local workspace";
  const stack = [state.project?.language, state.project?.framework].filter(Boolean).join(" · ");
  byId("profile-plan").innerHTML = `${escapeHtml(stack || "Thanos CLI")} ${icon("star")}`;
}

function renderSkills() {
  if (!state.skills.length) {
    return `<div class="skill-empty">No skills registered yet. Use the <strong>Add</strong> button to install one for this project.</div>`;
  }
  return state.skills
    .map((skill) => {
      const scope = skill.roles.length ? skill.roles.join(", ") : "all roles";
      return `
        <div class="skill-row" title="${escapeHtml(skill.source || skill.name)}">
          <span class="skill-icon">${icon("skill")}</span>
          <div>
            <strong>${escapeHtml(skill.name)}</strong>
            <small>${escapeHtml(scope)}</small>
          </div>
        </div>
      `;
    })
    .join("");
}

function agentCandidates(): AgentCandidate[] {
  return state.agents.length ? state.agents : fallbackAgents();
}

function agentNameForCommand(command: string): string {
  if (!command) {
    return "custom";
  }
  const match = agentCandidates().find((candidate) => candidate.command === command);
  return match ? match.name : command;
}

function roleSelectedAgent(roleKey: string): string {
  // Explicit assignment: a profile named after the role (matrix-written) or one
  // whose `role:` field matches (hand-edited .thanos/agents.yaml).
  const explicit = state.profiles.find(
    (entry) => entry.name === roleKey || (entry.role || "").toLowerCase() === roleKey,
  );
  if (explicit) {
    return agentNameForCommand(explicit.command);
  }
  // Otherwise reflect the workspace default runner from settings.json, so the
  // matrix shows the agent that will actually run the step rather than "Not set".
  const fallback = state.project?.default_runner;
  if (fallback && agentCandidates().some((candidate) => candidate.name === fallback)) {
    return fallback;
  }
  return "";
}

function renderAgentProfiles() {
  const candidates = agentCandidates();
  return ROLE_MATRIX.map((role) => {
    const selected = roleSelectedAgent(role.key);
    const options = [`<option value="" ${selected ? "" : "selected"}>Not set</option>`]
      .concat(
        candidates.map((candidate) => {
          const label = candidate.installed || candidate.name === "custom" ? candidate.name : `${candidate.name} (not installed)`;
          return `<option value="${candidate.name}" ${candidate.name === selected ? "selected" : ""}>${escapeHtml(label)}</option>`;
        }),
      )
      .join("");
    return `
      <div class="role-row">
        <span class="role-badge ${role.key}">${escapeHtml(role.label.slice(0, 1))}</span>
        <div class="role-meta">
          <strong>${role.label}</strong>
          <small>${role.hint}</small>
        </div>
        <select class="role-select" data-role-agent="${role.key}" title="Agent for the ${role.label} role">${options}</select>
      </div>
    `;
  }).join("");
}

async function configureRoleAgent(roleKey: string, agentName: string) {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    pushActivity("Workspace required", "Connect a workspace before assigning role agents.", "warn");
    renderAll();
    return;
  }
  const role = ROLE_MATRIX.find((entry) => entry.key === roleKey);
  if (!role || !agentName) {
    return;
  }
  const candidate = agentCandidates().find((entry) => entry.name === agentName);
  if (!candidate) {
    return;
  }
  const profile: AgentProfile = {
    name: roleKey,
    command: candidate.command,
    args: candidate.default_args,
    env: {},
    role: roleKey,
    allowed_steps: role.steps,
  };
  try {
    const config = await invoke<{ agents: AgentProfile[] }>("write_agent_profile", { workspace, profile });
    state.profiles = config.agents;
    pushActivity("Role agent set", `${role.label} → ${agentName}`, "ok");
  } catch (error) {
    pushActivity("Role agent save failed", String(error), "fail");
  } finally {
    renderAll();
  }
}


function renderBoard() {
  return columns
    .map((column) => {
      const tasks = state.tasks.filter((task) => normalizeStatus(task.status) === column.id);
      return `
        <section class="workflow-column" aria-label="${column.title}">
          <header class="column-header">
            <div><span class="status-dot ${column.id}"></span><strong>${column.title}</strong>${column.hint ? `<small>${column.hint}</small>` : ""}</div>
            <span class="count">${tasks.length}</span>
            <button class="column-menu" title="Column actions">${icon("more")}</button>
          </header>
          <div class="column-stack">
            ${tasks.length ? tasks.map(renderTaskCard).join("") : `<div class="empty-card">No ${column.title.toLowerCase()} tasks from CLI JSON.</div>`}
            <button class="add-task-button" data-add-task>${icon("plus")}<span>Add task</span></button>
          </div>
        </section>
      `;
    })
    .join("");
}

function renderTaskCard(task: Task) {
  const selected = selectedTask()?.id === task.id ? "selected" : "";
  const progress = taskProgress(task);
  return `
    <article class="task-card ${selected}" data-task-id="${task.id}" tabindex="0">
      <div class="task-meta"><span>${task.id}</span><span>${task.tests_passed ? icon("check") : icon("brackets")}</span></div>
      <h2>${escapeHtml(task.title)}</h2>
      <div class="label-row">
        <span class="tag ${normalizeStatus(task.status)}">${escapeHtml(task.priority || statusLabel(task))}</span>
        ${task.review_approved ? `<span class="tag ok">review</span>` : ""}
        ${task.tests_passed ? `<span class="tag ok">tests</span>` : ""}
      </div>
      ${normalizeStatus(task.status) === "done" ? "" : `<div class="progress-row"><span style="--progress:${progress}%"></span><strong>${progress}%</strong></div>`}
      <div class="card-footer">
        <span class="avatar">${ownerInitial(task)}</span>
        <span class="card-stats">${icon("message")} ${commentCount(task)} ${icon("file")} ${fileCount(task)}</span>
        <span>${formatUpdated(task.updated_at)}</span>
      </div>
    </article>
  `;
}

function renderInspector() {
  const task = selectedTask();
  if (!task) {
    return `
      <div class="inspector-empty">
        <div class="brand-mark">T</div>
        <h2>Select a task</h2>
        <p>Load CLI JSON with Refresh, then choose a task to inspect plan, logs, reviews, tests, and available guarded actions.</p>
      </div>
    `;
  }
  const tabs = ["overview", "plan", "files", "logs", "reviews", "tests"] as const;
  return `
    <header class="inspector-header">
      <div class="inspector-toolbar">
        <div class="brand-mark small">${icon("bot")}</div>
        <span class="task-kicker">${task.id}</span>
        <span class="toolbar-spacer"></span>
        <button class="icon-button" title="More">${icon("more")}</button>
        <button class="icon-button" data-inspector-close title="Close">${icon("close")}</button>
      </div>
      <h2>${escapeHtml(task.title)}</h2>
      <div class="state-line"><span class="pill ${normalizeStatus(task.status)}">${icon("dot")} ${titleCase(normalizeStatus(task.status))}</span><span>${task.review_approved ? "review approved" : "waiting_for_review"}</span></div>
    </header>
    <div class="tab-row">
      ${tabs.map((tab) => `<button class="${state.activeInspectorTab === tab ? "active" : ""}" data-inspector-tab="${tab}">${tab}</button>`).join("")}
    </div>
    <section class="inspector-body">
      ${renderInspectorTab(task)}
    </section>
    <section class="inspector-chat">
      <div class="chat-tabs"><button class="active">Chat</button><button>Comments</button><button>History</button></div>
      <div class="message-list">
        <div class="message agent"><span class="brand-mark tiny">${icon("bot")}</span><p>Plan is ready for review. Please review the plan and approve to continue to Execute step.</p></div>
        <div class="message user"><p>Looks good. Please proceed.</p></div>
        <div class="message agent"><span class="brand-mark tiny">${icon("bot")}</span><p>Plan approved. Moving to Execute step.</p></div>
      </div>
    </section>
    <section class="action-stack">
      <button class="primary-button" data-run-task="${task.id}">${icon("send")}<span>Run ${runStepLabel(task)}</span></button>
      <button class="soft-button" data-action="/approve">Approve</button>
      <button class="soft-button" data-action="/reject">Request changes</button>
      <button class="danger-button" data-action="/plan">Reopen plan</button>
    </section>
  `;
}

function renderInspectorTab(task: Task) {
  const fields = [
    ["Feature", task.parent_task_id || "Standalone task"],
    ["Branch", task.branch_name || "not assigned"],
    ["Worktree", task.worktree_path || "not created"],
    ["Plan", task.plan_path || "not generated"],
    ["Execution log", task.log_path || "not generated"],
    ["Review", task.review_path || "not generated"],
    ["Tests", task.test_result_path || "not generated"],
  ];
  if (state.activeInspectorTab === "overview") {
    return `
      <div class="summary-card">
        <h3>Current step</h3>
        <p>${stepCopy(normalizeStatus(task.status))}</p>
      </div>
      <div class="field-list">
        ${fields.map(([label, value]) => `<div><span>${label}</span><strong>${escapeHtml(value)}</strong></div>`).join("")}
      </div>
    `;
  }
  if (state.activeInspectorTab === "plan") {
    return renderArtifactPanel("Plan summary", task.plan_path, [
      ["Requirement summary", "Loaded from .thanos/plans when the CLI exposes artifact read output."],
      ["Acceptance criteria", "Verify against the saved plan only."],
      ["Checklist", "Execute consumes this artifact without re-reading the ticket."],
    ]);
  }
  if (state.activeInspectorTab === "files") {
    return renderArtifactPanel("Affected files", task.log_path, [
      ["Changed files", "Recorded in the execution summary."],
      ["Current worktree", task.worktree_path || "not created"],
    ]);
  }
  if (state.activeInspectorTab === "logs") {
    return renderArtifactPanel("Execution logs", task.log_path, [["Command", state.terminal[state.terminal.length - 1] || "No command output yet."]]);
  }
  if (state.activeInspectorTab === "reviews") {
    return renderArtifactPanel("Review gate", task.review_path, [
      ["Review approved", String(Boolean(task.review_approved))],
      ["Human approval", "Required before done."],
    ]);
  }
  return renderArtifactPanel("Test state", task.test_result_path, [
    ["Tests passed", String(Boolean(task.tests_passed))],
    ["Gate", "Done requires passing tests and approved review."],
  ]);
}

function renderArtifactPanel(title: string, path: string | undefined, rows: string[][]) {
  return `
    <div class="summary-card">
      <h3>${title}</h3>
      <p>${escapeHtml(path || "Artifact not generated yet.")}</p>
    </div>
    <div class="checklist">
      ${rows.map(([label, value]) => `<div><span></span><strong>${escapeHtml(label)}</strong><small>${escapeHtml(value)}</small></div>`).join("")}
    </div>
  `;
}




function renderBottomTabs() {
  type BottomTab = "activity" | "timeline" | "terminal";

  const bottomTabs: { id: BottomTab; label: string }[] = [
    { id: "activity", label: "Activity" },
    { id: "timeline", label: "Timeline" },
    { id: "terminal", label: "Terminal" },
  ];

  return bottomTabs
      .map(
          ({ id, label }) => `
        <button
          class="bottom-tab ${state.activeBottomTab === id ? "active" : ""}"
          data-bottom-tab="${id}"
        >
          ${label}
        </button>
      `
      )
      .join("");
}

function renderBottomContent() {
  if (state.activeBottomTab === "timeline") {
    return renderTimeline();
  }
  return renderActivity();
}

function renderActivity() {
  return `
    <div class="activity-list">
      ${state.activity.map((event) => `<div class="activity-row ${event.tone}"><span></span><div><strong>${escapeHtml(event.title)}</strong><small>${escapeHtml(event.detail)} - ${event.time}</small></div></div>`).join("")}
    </div>
    <div class="metrics-card">
      <div><span>Total tasks</span><strong>${state.tasks.length}</strong></div>
      <div><span>In progress</span><strong>${state.tasks.filter((task) => !["backlog", "done"].includes(normalizeStatus(task.status))).length}</strong></div>
      <div><span>Done</span><strong>${state.tasks.filter((task) => normalizeStatus(task.status) === "done").length}</strong></div>
    </div>
  `;
}

function icon(name: string) {
  const icons: Record<string, string> = {
    dashboard: "⌂",
    kanban: "▦",
    tasks: "✓",
    features: "⚙",
    plans: "▣",
    reviews: "✉",
    tests: "⌘",
    worktrees: "⌬",
    settings: "⚙",
    bot: "◕",
    panel: "◫",
    menu: "☰",
    search: "⌕",
    sliders: "≡",
    sort: "↕",
    plus: "+",
    refresh: "↻",
    chevron: "⌄",
    more: "⋯",
    brackets: "{}",
    check: "✓",
    message: "◌",
    file: "▱",
    send: "➤",
    close: "×",
    dot: "●",
    folder: "🗀",
    skill: "✦",
    star: "★",
  };
  return `<span class="ui-icon" aria-hidden="true">${icons[name] || "•"}</span>`;
}

function titleCase(value: string) {
  return value.slice(0, 1).toUpperCase() + value.slice(1);
}

function statusLabel(task: Task) {
  const status = normalizeStatus(task.status);
  if (status === "plan" || status === "verify") return "review";
  if (status === "execute") return "backend";
  if (status === "done") return "done";
  return "feature";
}

function commentCount(task: Task) {
  return task.review_path ? 2 : task.review_approved ? 1 : 0;
}

function fileCount(task: Task) {
  return [task.plan_path, task.log_path, task.review_path, task.test_result_path].filter(Boolean).length;
}

function renderTimeline() {
  return `
    <div class="timeline">
      ${state.activity.map((event) => `<div><time>${event.time}</time><strong>${escapeHtml(event.title)}</strong><span>${escapeHtml(event.detail)}</span></div>`).join("")}
    </div>
  `;
}

// ---- Embedded PTY terminal (xterm.js) ----

function ensureTerminal() {
  if (term) {
    return;
  }
  term = new Terminal({
    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", monospace',
    fontSize: 12,
    cursorBlink: true,
    scrollback: 5000,
    theme: {
      background: "#0a1220",
      foreground: "#cbd6ea",
      cursor: "#3d72ff",
      selectionBackground: "rgba(61,114,255,0.32)",
    },
  });
  fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.open(byId("terminal-host"));
  term.onData((data) => {
    void invoke("write_terminal", { data }).catch(() => {});
  });
  term.writeln("\x1b[38;5;244mThanos terminal ready. Run a task step to watch the agent work here.\x1b[0m");
}

function fitTerminal() {
  ensureTerminal();
  requestAnimationFrame(() => {
    try {
      fitAddon?.fit();
      if (term && terminalRunning) {
        void invoke("resize_terminal", { rows: term.rows, cols: term.cols }).catch(() => {});
      }
    } catch {
      // terminal-host may not be laid out yet; ignore.
    }
  });
}

function setTerminalStatus(status: "idle" | "running") {
  terminalRunning = status === "running";
  const label = byId("terminal-status");
  label.textContent = status;
  byId("terminal-dot").classList.toggle("live", terminalRunning);
}

async function runInTerminal(args: string[]) {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  if (!workspace) {
    pushActivity("Workspace required", "Connect a workspace before running a step.", "warn");
    renderAll();
    return;
  }
  saveSettings();
  state.activeBottomTab = "terminal";
  renderAll();
  ensureTerminal();
  fitTerminal();
  setTerminalStatus("running");
  term?.writeln("");
  try {
    await invoke("spawn_terminal", { workspace, binary, args });
  } catch (error) {
    term?.writeln(`\x1b[31m${String(error)}\x1b[0m`);
    setTerminalStatus("idle");
  }
}

function initTerminalEvents() {
  try {
    void listen<string>("terminal-output", (event) => {
      ensureTerminal();
      term?.write(event.payload);
    });
    void listen<number>("terminal-exit", (event) => {
      ensureTerminal();
      term?.write(`\r\n\x1b[38;5;244m[process exited with code ${event.payload}]\x1b[0m\r\n`);
      setTerminalStatus("idle");
      void refreshBoard();
    });
  } catch {
    // Event bridge is only available inside the Tauri runtime.
  }
  window.addEventListener("resize", () => {
    if (state.activeBottomTab === "terminal") {
      fitTerminal();
    }
  });
}

function runStepLabel(task: Task): string {
  const step = normalizeStatus(task.status);
  if (step === "backlog" || step === "plan") {
    return "plan";
  }
  if (step === "verify") {
    return "verify";
  }
  return "execute";
}

function taskRunArgs(task: Task): string[] {
  return ["task", runStepLabel(task), task.id];
}

function wireEvents() {
  byId("refresh").addEventListener("click", refreshBoard);
  byId("new-task-toggle").addEventListener("click", () => openModal("task"));
  byId("browse-workspace").addEventListener("click", browseWorkspace);
  byId("add-skill").addEventListener("click", () => openModal("skill"));
  byId("menu-button").addEventListener("click", toggleSidebar);
  byId("sidebar-collapse").addEventListener("click", toggleSidebar);
  byId("inspector-toggle").addEventListener("click", toggleInspector);
  byId("scrim").addEventListener("click", closeDrawers);
  byId("terminal-stop").addEventListener("click", () => {
    void invoke("kill_terminal").catch(() => {});
    setTerminalStatus("idle");
  });
  byId("terminal-clear").addEventListener("click", () => term?.clear());

  document.addEventListener("click", (event) => {
    const target = event.target as HTMLElement;
    // Close on an explicit close/cancel button, or a direct click on the backdrop
    // itself — never when interacting with fields inside the modal card.
    if (target.closest("[data-modal-close]") || target.hasAttribute("data-modal-backdrop")) {
      closeModal();
      return;
    }
    if (target.closest("#modal-browse")) {
      browseWorkspace();
      return;
    }
    if (target.closest("[data-inspector-close]")) {
      state.inspectorOpen = false;
      state.inspectorHidden = true;
      renderAll();
      return;
    }
    const taskCard = target.closest<HTMLElement>("[data-task-id]");
    if (taskCard) {
      state.selectedTaskID = taskCard.dataset.taskId || "";
      taskInput.value = state.selectedTaskID;
      // Reveal the details panel (desktop) / drawer (mobile) for the clicked task.
      state.inspectorOpen = true;
      state.inspectorHidden = false;
      renderAll();
      return;
    }
    if (target.closest("[data-add-task]")) {
      openModal("task");
      return;
    }
    const bottomTab = target.closest<HTMLButtonElement>("[data-bottom-tab]");
    if (bottomTab) {
      state.activeBottomTab = bottomTab.dataset.bottomTab as typeof state.activeBottomTab;
      renderAll();
      return;
    }
    const inspectorTab = target.closest<HTMLButtonElement>("[data-inspector-tab]");
    if (inspectorTab) {
      state.activeInspectorTab = inspectorTab.dataset.inspectorTab as typeof state.activeInspectorTab;
      renderAll();
      return;
    }
    const runTask = target.closest<HTMLElement>("[data-run-task]");
    if (runTask) {
      const task = state.tasks.find((entry) => entry.id === runTask.dataset.runTask) || selectedTask();
      if (task) {
        void runInTerminal(taskRunArgs(task));
      }
      return;
    }
    const action = target.closest<HTMLButtonElement>("[data-action]");
    if (action) {
      runSlashCommand(action.dataset.action || "/status");
      return;
    }
  });

  document.addEventListener("change", (event) => {
    const roleSelect = (event.target as HTMLElement).closest<HTMLSelectElement>("[data-role-agent]");
    if (roleSelect) {
      configureRoleAgent(roleSelect.dataset.roleAgent || "", roleSelect.value);
    }
  });

  document.addEventListener("submit", (event) => {
    const form = (event.target as HTMLElement).closest<HTMLFormElement>("#modal-form");
    if (!form) {
      return;
    }
    event.preventDefault();
    const data = new FormData(form);
    if (state.modal === "task") {
      submitTask(data);
    } else if (state.modal === "skill") {
      submitSkill(data);
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return;
    }
    if (state.modal) {
      closeModal();
    } else if (isNarrow() && (!state.sidebarCollapsed || state.inspectorOpen)) {
      closeDrawers();
    }
  });

  let lastNarrow = isNarrow();
  window.addEventListener("resize", () => {
    const narrow = isNarrow();
    if (narrow === lastNarrow) {
      return;
    }
    lastNarrow = narrow;
    // Collapse the drawer when entering narrow mode; restore it on desktop.
    state.sidebarCollapsed = narrow;
    state.inspectorOpen = false;
    renderAll();
  });
}

function toggleSidebar() {
  state.sidebarCollapsed = !state.sidebarCollapsed;
  // Opening the sidebar drawer on a narrow screen should hide the inspector drawer.
  if (!state.sidebarCollapsed && isNarrow()) {
    state.inspectorOpen = false;
  }
  renderAll();
}

function toggleInspector() {
  if (isNarrow()) {
    // On small screens the inspector is a drawer keyed off inspectorOpen.
    state.inspectorOpen = !state.inspectorOpen;
  } else {
    state.inspectorHidden = !state.inspectorHidden;
  }
  renderAll();
}

function closeDrawers() {
  if (isNarrow()) {
    state.sidebarCollapsed = true;
  }
  state.inspectorOpen = false;
  renderAll();
}

function openModal(kind: "task" | "skill") {
  state.modal = kind;
  state.modalError = "";
  renderAll();
  const first = document.querySelector<HTMLElement>("#modal-form [autofocus]");
  first?.focus();
}

function closeModal() {
  state.modal = null;
  state.modalError = "";
  renderAll();
}

async function refreshBoard() {
  await runCli(["task", "list", "--json"], "Task list refreshed", (result) => {
    const parsed = JSON.parse(result.stdout || "[]") as Task[];
    state.tasks = parsed;
    if (!state.selectedTaskID && parsed[0]) {
      state.selectedTaskID = parsed[0].id;
      taskInput.value = parsed[0].id;
    }
  });
}

async function connectWorkspace() {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    pushActivity("Workspace required", "Choose a folder or paste a local project path.", "warn");
    renderAll();
    return;
  }
  saveSettings();
  try {
    await ensureSelectedWorkspace();
  } catch {
    return;
  }
  await readAgentProfiles();
  await readProjectConfig();
  await readBranch();
  await refreshBoard();
}

async function readProjectConfig() {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    return;
  }
  try {
    const config = await invoke<ProjectConfig>("read_project_config", { workspace });
    state.project = config;
    state.skills = config.skills;
    pushActivity("Project loaded", `${config.name || basename(workspace)} · ${config.skills.length} skills`, "ok");
  } catch (error) {
    state.project = null;
    state.skills = [];
    pushActivity("Project config unavailable", String(error), "warn");
  } finally {
    renderAll();
  }
}

async function readBranch() {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    return;
  }
  try {
    const result = await invoke<CliResult>("run_thanos", { workspace, binary: "git", args: ["branch", "--show-current"] });
    const branch = (result.stdout || "").trim();
    if (branch) {
      state.branch = branch;
    }
  } catch {
    // branch is best-effort; keep the previous value.
  } finally {
    renderAll();
  }
}

async function browseWorkspace() {
  try {
    const selected = await invoke<string | null>("select_workspace_folder");
    if (!selected) {
      return;
    }
    workspaceInput.value = selected;
    await connectWorkspace();
  } catch (error) {
    pushActivity("Folder selection failed", String(error), "fail");
    renderAll();
  }
}

async function ensureSelectedWorkspace() {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  state.busy = true;
  renderAll();
  try {
    const info = await invoke<WorkspaceInfo>("ensure_workspace", { workspace, binary });
    workspaceInput.value = info.path;
    saveSettings();
    if (info.initialized) {
      pushActivity("Workspace connected", `${basename(info.path)} is ready.`, "ok");
      return;
    }
    state.terminal = [info.init_output?.command || `${binary} init`, info.init_output?.stdout || "(no stdout)", info.init_output?.stderr || "(no stderr)"];
    pushActivity("Workspace initialized", `Created .thanos in ${basename(info.path)}.`, "ok");
  } catch (error) {
    pushActivity("Workspace setup failed", String(error), "fail");
    throw error;
  } finally {
    state.busy = false;
    renderAll();
  }
}

async function detectAgents() {
  try {
    state.agents = await invoke<AgentCandidate[]>("detect_agent_clis");
    pushActivity("Agent CLIs detected", installedAgentSummary(), "info");
  } catch (error) {
    pushActivity("Agent detection failed", String(error), "fail");
  } finally {
    renderAll();
  }
}

async function readAgentProfiles() {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    pushActivity("Workspace required", "Set the workspace path before loading agent profiles.", "warn");
    return;
  }
  try {
    const config = await invoke<{ agents: AgentProfile[] }>("read_agent_profiles", { workspace });
    state.profiles = config.agents;
    if (!state.profiles.some((profile) => profile.name === state.selectedAgent) && state.profiles[0]) {
      state.selectedAgent = state.profiles[0].name;
      localStorage.setItem("thanos.agent", state.selectedAgent);
    }
    pushActivity("Agent profiles loaded", `.thanos/agents.yaml has ${state.profiles.length} profiles.`, "ok");
  } catch (error) {
    pushActivity("Agent profiles unavailable", String(error), "warn");
  }
}

async function runSlashCommand(value: string) {
  const [command, ...rest] = value.trim().split(/\s+/);
  const selected = taskInput.value.trim() || state.selectedTaskID;
  const newTaskTitle = rest.join(" ").trim();
  const map: Record<string, string[]> = {
    "/status": ["task", "list", "--json"],
    "/task": ["task", "show", rest[0] || selected, "--json"],
    "/new": newTaskTitle ? ["task", "create", newTaskTitle, "--agent", state.selectedAgent] : [""],
    "/plan": ["task", "plan", selected],
    "/execute": ["task", "execute", selected],
    "/verify": ["task", "verify", selected],
    "/approve": ["task", "verify", selected, "approve"],
    "/reject": ["task", "verify", selected, "request-changes"],
    "/help": ["help"],
  };
  if (command === "/skill") {
    if (!rest.length) {
      pushActivity("Skill command needs args", "Try /skill add <source> or /skill find <query>.", "warn");
      renderAll();
      return;
    }
    await runCli(["skill", ...rest], "Skill command finished");
    await readProjectConfig();
    return;
  }
  const args = command === "/run" ? ["ask", rest.join(" ")] : map[command];
  if (!args || args.some((part) => part === "")) {
    pushActivity("Command needs a task", `${command} requires a selected task.`, "warn");
    renderAll();
    return;
  }
  await runCli(args, `Command ${command} finished`, (result) => {
    if (command === "/status") {
      state.tasks = JSON.parse(result.stdout || "[]") as Task[];
    }
    if (command === "/task") {
      const task = JSON.parse(result.stdout || "{}") as Task;
      state.tasks = upsertTask(state.tasks, task);
      state.selectedTaskID = task.id;
    }
  });
  if (command !== "/help" && command !== "/task") {
    await refreshBoard();
  }
}

async function submitTask(data: FormData) {
  const title = String(data.get("title") || "").trim();
  if (!title) {
    state.modalError = "Give the task a title.";
    renderAll();
    return;
  }
  const description = String(data.get("description") || "").trim();
  const priority = String(data.get("priority") || "medium");
  // Agent is no longer chosen per task; the Agent Profiles role matrix drives execution.
  const args = ["task", "create", title, "--priority", priority];
  if (description) {
    args.push("--description", description);
  }
  const ok = await runCli(args, `Created task ${title}`);
  if (ok) {
    closeModal();
    await refreshBoard();
  } else {
    state.modalError = "Could not create the task. See the terminal panel for details.";
    renderAll();
  }
}

async function submitSkill(data: FormData) {
  const source = String(data.get("source") || "").trim();
  if (!source) {
    state.modalError = "Enter a skill source (e.g. owner/repo).";
    renderAll();
    return;
  }
  const skill = String(data.get("skill") || "").trim();
  const roles = String(data.get("roles") || "").trim();
  const args = ["skill", "add", source];
  if (skill) {
    args.push("--skill", skill);
  }
  if (roles) {
    args.push("--roles", roles.split(/[,\s]+/).filter(Boolean).join(","));
  }
  const ok = await runCli(args, `Added skill from ${source}`);
  if (ok) {
    closeModal();
    await readProjectConfig();
  } else {
    state.modalError = "Could not add the skill. See the terminal panel for details.";
    renderAll();
  }
}

async function runCli(args: string[], successTitle: string, consume?: (result: CliResult) => void): Promise<boolean> {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  if (!workspace) {
    pushActivity("Workspace required", "Set a local Thanos workspace before running commands.", "warn");
    renderAll();
    return false;
  }
  saveSettings();
  state.busy = true;
  state.activeBottomTab = "terminal";
  renderAll();
  let ok = false;
  try {
    const result = await invoke<CliResult>("run_thanos", { workspace, binary, args });
    state.terminal = [result.command, result.stdout || "(no stdout)", result.stderr || "(no stderr)"];
    if (result.code === 0) {
      consume?.(result);
      pushActivity(successTitle, result.command, "ok");
      ok = true;
    } else {
      pushActivity("Command failed", result.stderr || result.command, "fail");
    }
  } catch (error) {
    state.terminal = [String(error)];
    pushActivity("CLI connection failed", String(error), "fail");
  } finally {
    state.busy = false;
    renderAll();
  }
  return ok;
}

function byId<T extends HTMLElement = HTMLElement>(id: string): T {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`${id} missing`);
  }
  return element as T;
}

function selectedTask() {
  return state.tasks.find((task) => task.id === state.selectedTaskID) || state.tasks[0];
}

function normalizeStatus(status: string): TaskStatus {
  const lower = status.toLowerCase();
  if (lower === "plan" || lower === "analysis") return "plan";
  if (lower === "execute" || lower === "dev") return "execute";
  if (lower === "verify" || lower === "review" || lower === "test") return "verify";
  if (lower === "done") return "done";
  return "backlog";
}

function taskProgress(task: Task) {
  const values: Record<TaskStatus, number> = { backlog: 8, plan: 28, execute: 58, verify: 82, done: 100 };
  return values[normalizeStatus(task.status)];
}

function stepCopy(status: TaskStatus) {
  return {
    backlog: "Waiting for a plan. No implementation should start yet.",
    plan: "Plan once, ask for approval, and save reusable artifacts.",
    execute: "Run the agent against the saved plan only.",
    verify: "Review output, run tests, and wait for human approval.",
    done: "Review and tests passed. Workflow is closed.",
  }[status];
}

function ownerInitial(task: Task) {
  return (task.assigned_agent || "T").slice(0, 2).toUpperCase();
}

function formatUpdated(value?: string) {
  if (!value) return "not synced";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function pushActivity(title: string, detail: string, tone: ActivityEvent["tone"]) {
  state.activity.unshift({
    time: new Date().toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" }),
    title,
    detail,
    tone,
  });
  state.activity = state.activity.slice(0, 12);
}

function upsertTask(tasks: Task[], task: Task) {
  const index = tasks.findIndex((item) => item.id === task.id);
  if (index < 0) return [task, ...tasks];
  const next = [...tasks];
  next[index] = task;
  return next;
}

function restoreSettings() {
  workspaceInput.value = localStorage.getItem("thanos.workspace") ?? "";
  binaryInput.value = localStorage.getItem("thanos.binary") ?? "thanos";
}

function saveSettings() {
  localStorage.setItem("thanos.workspace", workspaceInput.value.trim());
  localStorage.setItem("thanos.binary", binaryInput.value.trim() || "thanos");
}

function fallbackAgents(): AgentCandidate[] {
  return [
    { name: "codex", command: "codex", installed: false, path: null, default_args: ["exec", "--full-auto", "-"], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "claude", command: "claude", installed: false, path: null, default_args: ["--print", "--dangerously-skip-permissions"], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "gemini", command: "gemini", installed: false, path: null, default_args: [], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "deepseek", command: "deepseek", installed: false, path: null, default_args: [], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "opencode", command: "opencode", installed: false, path: null, default_args: [], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "custom", command: "", installed: true, path: null, default_args: [], role: "custom", allowed_steps: ["plan", "execute", "verify"] },
  ];
}

function installedAgentSummary() {
  const installed = state.agents.filter((agent) => agent.installed).map((agent) => agent.name);
  return installed.length ? `Installed: ${installed.join(", ")}` : "No known agent CLIs found on PATH.";
}

function basename(path: string) {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] || path;
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char] || char);
}
