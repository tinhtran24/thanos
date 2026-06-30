(function polyfill() {
  const relList = document.createElement("link").relList;
  if (relList && relList.supports && relList.supports("modulepreload")) {
    return;
  }
  for (const link of document.querySelectorAll('link[rel="modulepreload"]')) {
    processPreload(link);
  }
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type !== "childList") {
        continue;
      }
      for (const node of mutation.addedNodes) {
        if (node.tagName === "LINK" && node.rel === "modulepreload")
          processPreload(node);
      }
    }
  }).observe(document, { childList: true, subtree: true });
  function getFetchOpts(link) {
    const fetchOpts = {};
    if (link.integrity) fetchOpts.integrity = link.integrity;
    if (link.referrerPolicy) fetchOpts.referrerPolicy = link.referrerPolicy;
    if (link.crossOrigin === "use-credentials")
      fetchOpts.credentials = "include";
    else if (link.crossOrigin === "anonymous") fetchOpts.credentials = "omit";
    else fetchOpts.credentials = "same-origin";
    return fetchOpts;
  }
  function processPreload(link) {
    if (link.ep)
      return;
    link.ep = true;
    const fetchOpts = getFetchOpts(link);
    fetch(link.href, fetchOpts);
  }
})();
typeof SuppressedError === "function" ? SuppressedError : function(error, suppressed, message) {
  var e = new Error(message);
  return e.name = "SuppressedError", e.error = error, e.suppressed = suppressed, e;
};
async function invoke(cmd, args = {}, options) {
  return window.__TAURI_INTERNALS__.invoke(cmd, args, options);
}
const columns = [
  { id: "backlog", title: "Backlog", hint: "" },
  { id: "plan", title: "Plan", hint: "Review" },
  { id: "execute", title: "Execute", hint: "" },
  { id: "verify", title: "Verify", hint: "Review" },
  { id: "done", title: "Done", hint: "" }
];
const state = {
  tasks: [],
  selectedTaskID: "",
  activeBottomTab: "activity",
  activeInspectorTab: "overview",
  activity: [{ time: "10:42", title: "Desktop ready", detail: "Connect a workspace to load CLI task state.", tone: "info" }],
  terminal: ["Waiting for CLI command output."],
  agents: [],
  profiles: [],
  selectedAgent: localStorage.getItem("thanos.agent") ?? "custom",
  busy: false,
  paletteOpen: false
};
const app = document.querySelector("#app");
if (!app) {
  throw new Error("app root missing");
}
app.innerHTML = renderAppShell();
const workspaceInput = byId("workspace");
const binaryInput = byId("binary");
const taskInput = byId("task-id");
const chatInput = byId("chat-input");
const commandInput = byId("palette-input");
restoreSettings();
wireEvents();
detectAgents();
renderAll();
function renderAppShell() {
  return `
    <div class="noise"></div>
    <div class="desktop-shell">
      ${renderSidebar()}
      <main class="workspace">
        ${renderTopbar()}
        <section class="workspace-grid">
          <section class="board-region">
            <div id="board" class="workflow-board"></div>
            <div id="bottom-panel" class="bottom-panel"></div>
          </section>
          <aside id="inspector" class="inspector"></aside>
        </section>
        ${renderChatBar()}
      </main>
      ${renderCommandPalette()}
    </div>
  `;
}
function renderSidebar() {
  const nav = [
    ["dashboard", "Dashboard"],
    ["kanban", "Kanban"],
    ["tasks", "Tasks"],
    ["features", "Features"],
    ["plans", "Plans"],
    ["reviews", "Reviews"],
    ["tests", "Tests"],
    ["worktrees", "Worktrees"],
    ["settings", "Settings"]
  ];
  return `
    <aside class="sidebar" aria-label="Thanos navigation">
      <div class="brand">
        <div class="brand-mark">${icon("bot")}</div>
        <div>
          <div class="brand-title">Thanos</div>
          <div class="brand-subtitle">AI Workflow</div>
        </div>
        <button class="sidebar-toggle icon-button" title="Collapse sidebar">${icon("panel")}</button>
      </div>
      <nav class="nav-list">
        ${nav.map(([key, item]) => `<button class="nav-item ${item === "Kanban" ? "active" : ""}" data-command="/${key}">${icon(key)}<span>${item}</span></button>`).join("")}
      </nav>
      <section class="sidebar-section workspace-card">
        <div class="sidebar-heading">Workspace</div>
        <div class="connection-row">
          <span class="pulse"></span>
          <div><strong id="workspace-name">thanos-cli</strong><small id="branch-name">main</small></div>
          <span class="chevron">${icon("chevron")}</span>
        </div>
      </section>
      <section class="sidebar-section">
        <div class="sidebar-heading">Agent profiles</div>
        <div id="agent-list" class="agent-list"></div>
      </section>
      <section class="sidebar-footer">
        <label>
          Workspace
          <input id="workspace" placeholder="/path/to/project" />
        </label>
        <div class="workspace-actions">
          <button id="browse-workspace" class="soft-button">Browse</button>
          <button id="current-workspace" class="soft-button">Current</button>
          <button id="connect-workspace" class="soft-button">Connect</button>
          <button id="detect-agents" class="soft-button">Detect CLIs</button>
        </div>
        <label>
          CLI binary
          <input id="binary" value="thanos" />
        </label>
        <label>
          Task agent
          <select id="agent-select"></select>
        </label>
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
          <kbd>⌘K</kbd>
        </label>
        <button class="soft-button filter-button">${icon("sliders")}<span>All Status</span>${icon("chevron")}</button>
        <button class="soft-button filter-button">${icon("sort")}<span>Newest</span>${icon("chevron")}</button>
        <button id="new-task-toggle" class="primary-button">${icon("plus")}<span>New Task</span></button>
        <button id="refresh" class="icon-button" title="Refresh">${icon("refresh")}</button>
      </div>
    </header>
  `;
}
function renderChatBar() {
  return `
    <footer class="chatbar">
      <div class="shortcut-strip">
        <button data-chat="/status">/status</button>
        <button data-chat="/plan">/plan</button>
        <button data-chat="/execute">/execute</button>
        <button data-chat="/verify">/verify</button>
        <button data-chat="/approve">/approve</button>
        <button data-chat="/reject">/reject</button>
        <button data-chat="/help">/help</button>
      </div>
      <form id="chat-form" class="chat-input-wrap">
        <span>/</span>
        <input id="chat-input" placeholder="Type message or /command..." />
        <button class="send-button" title="Send command">${icon("send")}</button>
      </form>
    </footer>
  `;
}
function renderCommandPalette() {
  return `
    <div id="command-palette" class="palette" hidden>
      <div class="palette-card">
        <div class="palette-title">Command palette</div>
        <input id="palette-input" placeholder="Run task, open plan, approve step..." />
        <div class="palette-list">
          ${["/new", "/status", "/plan", "/execute", "/verify", "/approve", "/reject", "/help"].map((command) => `<button data-palette="${command}">${command}</button>`).join("")}
        </div>
      </div>
    </div>
  `;
}
function renderAll() {
  byId("board").innerHTML = renderBoard();
  byId("inspector").innerHTML = renderInspector();
  byId("bottom-panel").innerHTML = renderBottomPanel();
  byId("agent-list").innerHTML = renderAgentProfiles();
  renderAgentSelect();
  byId("command-palette").toggleAttribute("hidden", !state.paletteOpen);
  document.body.toggleAttribute("aria-busy", state.busy);
  const workspace = workspaceInput.value.trim();
  byId("workspace-name").textContent = workspace ? basename(workspace) : "Not connected";
}
function renderAgentProfiles() {
  const agents = state.agents.length ? state.agents : fallbackAgents();
  return agents.map((agent) => {
    const configured = state.profiles.some((profile) => profile.name === agent.name);
    const active = state.selectedAgent === agent.name ? "active" : "";
    return `
        <div class="agent-row ${active} ${agent.installed ? "installed" : "missing"}">
          <span>${agent.name.slice(0, 1).toUpperCase()}</span>
          <div>
            <strong>${escapeHtml(agent.name)}</strong>
            <small>${agent.installed ? escapeHtml(agent.path || agent.command) : "not installed on PATH"}</small>
          </div>
          <button data-agent-use="${agent.name}" ${agent.installed || agent.name === "custom" ? "" : "disabled"}>${configured ? "Use" : "Add"}</button>
        </div>
      `;
  }).join("");
}
function renderAgentSelect() {
  const select = byId("agent-select");
  const agents = state.agents.length ? state.agents : fallbackAgents();
  const names = /* @__PURE__ */ new Set([...agents.map((agent) => agent.name), ...state.profiles.map((profile) => profile.name), "custom"]);
  select.innerHTML = Array.from(names).map((name) => `<option value="${name}" ${name === state.selectedAgent ? "selected" : ""}>${name}</option>`).join("");
}
function renderBoard() {
  return columns.map((column) => {
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
            <button class="add-task-button" data-chat="/new ">${icon("plus")}<span>Add task</span></button>
          </div>
        </section>
      `;
  }).join("");
}
function renderTaskCard(task) {
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
  const tabs = ["overview", "plan", "files", "logs", "reviews", "tests"];
  return `
    <header class="inspector-header">
      <div class="inspector-toolbar">
        <div class="brand-mark small">${icon("bot")}</div>
        <span class="task-kicker">${task.id}</span>
        <span class="toolbar-spacer"></span>
        <button class="icon-button" title="More">${icon("more")}</button>
        <button class="icon-button" title="Close">${icon("close")}</button>
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
      <button class="primary-button" data-action="/approve">Approve</button>
      <button class="soft-button" data-action="/reject">Request changes</button>
      <button class="soft-button" data-action="/execute">Run again</button>
      <button class="danger-button" data-action="/plan">Reopen plan</button>
    </section>
  `;
}
function renderInspectorTab(task) {
  const fields = [
    ["Feature", task.parent_task_id || "Standalone task"],
    ["Branch", task.branch_name || "not assigned"],
    ["Worktree", task.worktree_path || "not created"],
    ["Plan", task.plan_path || "not generated"],
    ["Execution log", task.log_path || "not generated"],
    ["Review", task.review_path || "not generated"],
    ["Tests", task.test_result_path || "not generated"]
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
      ["Checklist", "Execute consumes this artifact without re-reading the ticket."]
    ]);
  }
  if (state.activeInspectorTab === "files") {
    return renderArtifactPanel("Affected files", task.log_path, [
      ["Changed files", "Recorded in the execution summary."],
      ["Current worktree", task.worktree_path || "not created"]
    ]);
  }
  if (state.activeInspectorTab === "logs") {
    return renderArtifactPanel("Execution logs", task.log_path, [["Command", state.terminal[state.terminal.length - 1] || "No command output yet."]]);
  }
  if (state.activeInspectorTab === "reviews") {
    return renderArtifactPanel("Review gate", task.review_path, [
      ["Review approved", String(Boolean(task.review_approved))],
      ["Human approval", "Required before done."]
    ]);
  }
  return renderArtifactPanel("Test state", task.test_result_path, [
    ["Tests passed", String(Boolean(task.tests_passed))],
    ["Gate", "Done requires passing tests and approved review."]
  ]);
}
function renderArtifactPanel(title, path, rows) {
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
function renderBottomPanel() {
  const tabs = ["activity", "timeline", "terminal"];
  return `
    <div class="bottom-tabs">
      ${tabs.map((tab) => `<button class="${state.activeBottomTab === tab ? "active" : ""}" data-bottom-tab="${tab}">${tab}</button>`).join("")}
    </div>
    <div class="bottom-content">
      ${state.activeBottomTab === "activity" ? renderActivity() : ""}
      ${state.activeBottomTab === "timeline" ? renderTimeline() : ""}
      ${state.activeBottomTab === "terminal" ? renderTerminal() : ""}
    </div>
  `;
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
function icon(name) {
  const icons = {
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
    dot: "●"
  };
  return `<span class="ui-icon" aria-hidden="true">${icons[name] || "•"}</span>`;
}
function titleCase(value) {
  return value.slice(0, 1).toUpperCase() + value.slice(1);
}
function statusLabel(task) {
  const status = normalizeStatus(task.status);
  if (status === "plan" || status === "verify") return "review";
  if (status === "execute") return "backend";
  if (status === "done") return "done";
  return "feature";
}
function commentCount(task) {
  return task.review_path ? 2 : task.review_approved ? 1 : 0;
}
function fileCount(task) {
  return [task.plan_path, task.log_path, task.review_path, task.test_result_path].filter(Boolean).length;
}
function renderTimeline() {
  return `
    <div class="timeline">
      ${state.activity.map((event) => `<div><time>${event.time}</time><strong>${escapeHtml(event.title)}</strong><span>${escapeHtml(event.detail)}</span></div>`).join("")}
    </div>
  `;
}
function renderTerminal() {
  return `<pre class="terminal">${escapeHtml(state.terminal.join("\n\n"))}</pre>`;
}
function wireEvents() {
  byId("refresh").addEventListener("click", refreshBoard);
  byId("new-task-toggle").addEventListener("click", () => {
    chatInput.value = "/new ";
    chatInput.focus();
  });
  byId("chat-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const value = chatInput.value.trim();
    if (!value) {
      return;
    }
    chatInput.value = "";
    if (value.startsWith("/")) {
      runSlashCommand(value);
    } else {
      pushActivity("Question noted", value, "info");
    }
  });
  document.addEventListener("click", (event) => {
    const target = event.target;
    const taskCard = target.closest("[data-task-id]");
    if (taskCard) {
      state.selectedTaskID = taskCard.dataset.taskId || "";
      taskInput.value = state.selectedTaskID;
      renderAll();
      return;
    }
    const bottomTab = target.closest("[data-bottom-tab]");
    if (bottomTab) {
      state.activeBottomTab = bottomTab.dataset.bottomTab;
      renderAll();
      return;
    }
    const inspectorTab = target.closest("[data-inspector-tab]");
    if (inspectorTab) {
      state.activeInspectorTab = inspectorTab.dataset.inspectorTab;
      renderAll();
      return;
    }
    const chatCommand = target.closest("[data-chat]");
    if (chatCommand) {
      chatInput.value = chatCommand.dataset.chat || "";
      chatInput.focus();
      return;
    }
    const useAgent = target.closest("[data-agent-use]");
    if (useAgent) {
      configureAgent(useAgent.dataset.agentUse || "custom");
      return;
    }
    const action = target.closest("[data-action]");
    if (action) {
      runSlashCommand(action.dataset.action || "/status");
      return;
    }
    const palette = target.closest("[data-palette]");
    if (palette) {
      state.paletteOpen = false;
      runSlashCommand(palette.dataset.palette || "/status");
    }
  });
  byId("connect-workspace").addEventListener("click", connectWorkspace);
  byId("browse-workspace").addEventListener("click", browseWorkspace);
  byId("current-workspace").addEventListener("click", useCurrentWorkspace);
  byId("detect-agents").addEventListener("click", detectAgents);
  byId("agent-select").addEventListener("change", (event) => {
    const select = event.target;
    state.selectedAgent = select.value;
    localStorage.setItem("thanos.agent", state.selectedAgent);
    pushActivity("Agent selected", `${state.selectedAgent} will be used for new tasks.`, "info");
    renderAll();
  });
  document.addEventListener("keydown", (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      state.paletteOpen = !state.paletteOpen;
      renderAll();
      if (state.paletteOpen) {
        commandInput.focus();
      }
      return;
    }
    if (event.key === "/") {
      const active = document.activeElement?.tagName.toLowerCase();
      if (active !== "input" && active !== "textarea") {
        event.preventDefault();
        chatInput.focus();
      }
    }
    if (event.key === "Escape" && state.paletteOpen) {
      state.paletteOpen = false;
      renderAll();
    }
  });
}
async function refreshBoard() {
  await runCli(["task", "list", "--json"], "Task list refreshed", (result) => {
    const parsed = JSON.parse(result.stdout || "[]");
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
  await refreshBoard();
}
async function browseWorkspace() {
  try {
    const selected = await invoke("select_workspace_folder");
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
async function useCurrentWorkspace() {
  try {
    workspaceInput.value = await invoke("current_workspace_folder");
    await connectWorkspace();
  } catch (error) {
    pushActivity("Current folder unavailable", String(error), "fail");
    renderAll();
  }
}
async function ensureSelectedWorkspace() {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  state.busy = true;
  renderAll();
  try {
    const info = await invoke("ensure_workspace", { workspace, binary });
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
    state.agents = await invoke("detect_agent_clis");
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
    const config = await invoke("read_agent_profiles", { workspace });
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
async function configureAgent(name) {
  const workspace = workspaceInput.value.trim();
  if (!workspace) {
    pushActivity("Workspace required", "Set the workspace path before choosing an agent.", "warn");
    renderAll();
    return;
  }
  const candidate = (state.agents.length ? state.agents : fallbackAgents()).find((agent) => agent.name === name);
  if (!candidate) {
    return;
  }
  const profile = {
    name: candidate.name,
    command: candidate.command,
    args: candidate.default_args,
    env: {},
    role: candidate.role,
    allowed_steps: candidate.allowed_steps
  };
  try {
    const config = await invoke("write_agent_profile", { workspace, profile });
    state.profiles = config.agents;
    state.selectedAgent = name;
    localStorage.setItem("thanos.agent", name);
    pushActivity("Agent profile saved", `${name} now points to ${candidate.path || candidate.command || "custom command"}.`, "ok");
  } catch (error) {
    pushActivity("Agent save failed", String(error), "fail");
  } finally {
    renderAll();
  }
}
async function runSlashCommand(value) {
  const [command, ...rest] = value.trim().split(/\s+/);
  const selected = taskInput.value.trim() || state.selectedTaskID;
  const newTaskTitle = rest.join(" ").trim();
  const map = {
    "/status": ["task", "list", "--json"],
    "/task": ["task", "show", rest[0] || selected, "--json"],
    "/new": newTaskTitle ? ["task", "create", newTaskTitle, "--agent", state.selectedAgent] : [""],
    "/plan": ["task", "plan", selected],
    "/execute": ["task", "execute", selected],
    "/verify": ["task", "verify", selected],
    "/approve": ["task", "verify", selected, "approve"],
    "/reject": ["task", "verify", selected, "request-changes"],
    "/help": ["help"]
  };
  const args = command === "/run" ? ["ask", rest.join(" ")] : map[command];
  if (!args || args.some((part) => part === "")) {
    pushActivity("Command needs a task", `${command} requires a selected task.`, "warn");
    renderAll();
    return;
  }
  await runCli(args, `Command ${command} finished`, (result) => {
    if (command === "/status") {
      state.tasks = JSON.parse(result.stdout || "[]");
    }
    if (command === "/task") {
      const task = JSON.parse(result.stdout || "{}");
      state.tasks = upsertTask(state.tasks, task);
      state.selectedTaskID = task.id;
    }
  });
  if (command !== "/help" && command !== "/task") {
    await refreshBoard();
  }
}
async function runCli(args, successTitle, consume) {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  if (!workspace) {
    pushActivity("Workspace required", "Set a local Thanos workspace before running commands.", "warn");
    renderAll();
    return;
  }
  saveSettings();
  state.busy = true;
  state.activeBottomTab = "terminal";
  renderAll();
  try {
    const result = await invoke("run_thanos", { workspace, binary, args });
    state.terminal = [result.command, result.stdout || "(no stdout)", result.stderr || "(no stderr)"];
    if (result.code === 0) {
      consume?.(result);
      pushActivity(successTitle, result.command, "ok");
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
}
function byId(id) {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`${id} missing`);
  }
  return element;
}
function selectedTask() {
  return state.tasks.find((task) => task.id === state.selectedTaskID) || state.tasks[0];
}
function normalizeStatus(status) {
  const lower = status.toLowerCase();
  if (lower === "plan" || lower === "analysis") return "plan";
  if (lower === "execute" || lower === "dev") return "execute";
  if (lower === "verify" || lower === "review" || lower === "test") return "verify";
  if (lower === "done") return "done";
  return "backlog";
}
function taskProgress(task) {
  const values = { backlog: 8, plan: 28, execute: 58, verify: 82, done: 100 };
  return values[normalizeStatus(task.status)];
}
function stepCopy(status) {
  return {
    backlog: "Waiting for a plan. No implementation should start yet.",
    plan: "Plan once, ask for approval, and save reusable artifacts.",
    execute: "Run the agent against the saved plan only.",
    verify: "Review output, run tests, and wait for human approval.",
    done: "Review and tests passed. Workflow is closed."
  }[status];
}
function ownerInitial(task) {
  return (task.assigned_agent || "T").slice(0, 2).toUpperCase();
}
function formatUpdated(value) {
  if (!value) return "not synced";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(void 0, { month: "short", day: "numeric" });
}
function pushActivity(title, detail, tone) {
  state.activity.unshift({
    time: (/* @__PURE__ */ new Date()).toLocaleTimeString(void 0, { hour: "2-digit", minute: "2-digit" }),
    title,
    detail,
    tone
  });
  state.activity = state.activity.slice(0, 12);
}
function upsertTask(tasks, task) {
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
function fallbackAgents() {
  return [
    { name: "codex", command: "codex", installed: false, path: null, default_args: ["exec", "--full-auto", "-"], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "claude", command: "claude", installed: false, path: null, default_args: ["--print", "--dangerously-skip-permissions"], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "gemini", command: "gemini", installed: false, path: null, default_args: [], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "opencode", command: "opencode", installed: false, path: null, default_args: [], role: "implementation", allowed_steps: ["plan", "execute"] },
    { name: "custom", command: "", installed: true, path: null, default_args: [], role: "custom", allowed_steps: ["plan", "execute", "verify"] }
  ];
}
function installedAgentSummary() {
  const installed = state.agents.filter((agent) => agent.installed).map((agent) => agent.name);
  return installed.length ? `Installed: ${installed.join(", ")}` : "No known agent CLIs found on PATH.";
}
function basename(path) {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] || path;
}
function escapeHtml(value) {
  return value.replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;" })[char] || char);
}
