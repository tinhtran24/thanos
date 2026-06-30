import { invoke } from "@tauri-apps/api/core";
import "./styles.css";

type CliResult = {
  command: string;
  stdout: string;
  stderr: string;
  code: number | null;
};

type TaskCommand = {
  label: string;
  args: string[];
  requiresTask?: boolean;
  variant?: "primary" | "danger";
};

const taskCommands: TaskCommand[] = [
  { label: "Board", args: ["board"] },
  { label: "Plan", args: ["task", "plan"], requiresTask: true },
  { label: "Run Dev", args: ["task", "run"], requiresTask: true, variant: "primary" },
  { label: "Review", args: ["task", "review"], requiresTask: true },
  { label: "Approve", args: ["task", "review"], requiresTask: true, variant: "primary" },
  { label: "Test", args: ["task", "test"], requiresTask: true },
  { label: "Done", args: ["task", "done"], requiresTask: true, variant: "primary" },
  { label: "Reopen", args: ["task", "reopen"], requiresTask: true, variant: "danger" }
];

const app = document.querySelector<HTMLDivElement>("#app");

if (!app) {
  throw new Error("app root missing");
}

app.innerHTML = `
  <main class="shell">
    <header class="topbar">
      <div>
        <h1>Thanos Desktop UI</h1>
        <p>Review-first desktop controls for the local Thanos CLI workflow.</p>
      </div>
      <button id="refresh" class="icon-button" title="Refresh board">Refresh</button>
    </header>

    <section class="workspace-bar">
      <label>
        Workspace
        <input id="workspace" value="" placeholder="/path/to/local/project" />
      </label>
      <label>
        Thanos binary
        <input id="binary" value="thanos" />
      </label>
      <label>
        Task ID
        <input id="task-id" placeholder="T001-example" />
      </label>
    </section>

    <section class="command-bar" id="commands"></section>

    <section class="create-task">
      <label>
        New task title
        <input id="new-title" placeholder="Implement review gate" />
      </label>
      <label>
        Description
        <textarea id="new-description" rows="3" placeholder="What should change?"></textarea>
      </label>
      <label>
        Agent
        <input id="new-agent" placeholder="custom" />
      </label>
      <button id="create-task" class="primary">Create Task</button>
    </section>

    <section class="output-grid">
      <article>
        <div class="section-title">Command</div>
        <pre id="command-output"></pre>
      </article>
      <article>
        <div class="section-title">Output</div>
        <pre id="stdout"></pre>
      </article>
      <article>
        <div class="section-title">Errors</div>
        <pre id="stderr"></pre>
      </article>
    </section>
  </main>
`;

const workspaceInput = byId<HTMLInputElement>("workspace");
const binaryInput = byId<HTMLInputElement>("binary");
const taskInput = byId<HTMLInputElement>("task-id");
const commandOutput = byId<HTMLPreElement>("command-output");
const stdoutOutput = byId<HTMLPreElement>("stdout");
const stderrOutput = byId<HTMLPreElement>("stderr");
const commandsRoot = byId<HTMLElement>("commands");

restoreSettings();
renderCommands();
bindCreateTask();
byId<HTMLButtonElement>("refresh").addEventListener("click", () => run(["board"]));

function byId<T extends HTMLElement>(id: string): T {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`${id} missing`);
  }
  return element as T;
}

function renderCommands() {
  commandsRoot.innerHTML = "";
  for (const command of taskCommands) {
    const button = document.createElement("button");
    button.textContent = command.label;
    button.className = command.variant ?? "";
    button.addEventListener("click", () => {
      const args = [...command.args];
      if (command.requiresTask) {
        const taskID = taskInput.value.trim();
        if (!taskID) {
          showError("Task ID is required for this command.");
          return;
        }
        args.push(taskID);
        if (command.label === "Approve") {
          args.push("approve");
        }
      }
      run(args);
    });
    commandsRoot.appendChild(button);
  }
}

function bindCreateTask() {
  byId<HTMLButtonElement>("create-task").addEventListener("click", () => {
    const title = byId<HTMLInputElement>("new-title").value.trim();
    const description = byId<HTMLTextAreaElement>("new-description").value.trim();
    const agent = byId<HTMLInputElement>("new-agent").value.trim();
    if (!title) {
      showError("New task title is required.");
      return;
    }
    const args = ["task", "create", title];
    if (description) {
      args.push("--description", description);
    }
    if (agent) {
      args.push("--agent", agent);
    }
    run(args);
  });
}

async function run(args: string[]) {
  const workspace = workspaceInput.value.trim();
  const binary = binaryInput.value.trim() || "thanos";
  if (!workspace) {
    showError("Workspace path is required.");
    return;
  }
  saveSettings();
  setBusy(true);
  try {
    const result = await invoke<CliResult>("run_thanos", { workspace, binary, args });
    commandOutput.textContent = result.command;
    stdoutOutput.textContent = result.stdout || "(no stdout)";
    stderrOutput.textContent = result.stderr || (result.code === 0 ? "(no stderr)" : "");
  } catch (error) {
    showError(String(error));
  } finally {
    setBusy(false);
  }
}

function showError(message: string) {
  commandOutput.textContent = "";
  stdoutOutput.textContent = "";
  stderrOutput.textContent = message;
}

function setBusy(busy: boolean) {
  document.body.toggleAttribute("aria-busy", busy);
  for (const button of document.querySelectorAll<HTMLButtonElement>("button")) {
    button.disabled = busy;
  }
}

function restoreSettings() {
  workspaceInput.value = localStorage.getItem("thanos.workspace") ?? "";
  binaryInput.value = localStorage.getItem("thanos.binary") ?? "thanos";
}

function saveSettings() {
  localStorage.setItem("thanos.workspace", workspaceInput.value.trim());
  localStorage.setItem("thanos.binary", binaryInput.value.trim() || "thanos");
}
