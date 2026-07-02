use portable_pty::{native_pty_system, ChildKiller, CommandBuilder, MasterPty, PtySize};
use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::{
    collections::BTreeMap,
    fs,
    io::{Read, Write},
    path::{Path, PathBuf},
    process::Command,
    sync::Mutex,
    time::{SystemTime, UNIX_EPOCH},
};
use tauri::{AppHandle, Emitter, Manager, State};

/// One active PTY-backed terminal session. Only a single flow runs in the
/// embedded terminal at a time; spawning a new one replaces the previous.
#[derive(Default)]
struct TerminalSession {
    active_id: Mutex<Option<String>>,
    writer: Mutex<Option<Box<dyn Write + Send>>>,
    master: Mutex<Option<Box<dyn MasterPty + Send>>>,
    killer: Mutex<Option<Box<dyn ChildKiller + Send + Sync>>>,
}

#[derive(Serialize)]
struct CliResult {
    command: String,
    stdout: String,
    stderr: String,
    code: Option<i32>,
}

#[derive(Serialize)]
struct WorkspaceInfo {
    path: String,
    initialized: bool,
    init_output: Option<CliResult>,
}

#[derive(Serialize)]
struct AgentCandidate {
    name: String,
    command: String,
    installed: bool,
    path: Option<String>,
    default_args: Vec<String>,
    role: String,
    allowed_steps: Vec<String>,
}

#[derive(Deserialize, Serialize, Clone)]
struct AgentsConfig {
    agents: Vec<AgentProfile>,
}

#[derive(Serialize, Default)]
struct ProjectConfig {
    name: String,
    language: String,
    framework: String,
    default_runner: String,
    skills: Vec<SkillInfo>,
    mcp: Vec<String>,
}

#[derive(Serialize)]
struct SkillInfo {
    name: String,
    source: String,
    roles: Vec<String>,
}

#[derive(Deserialize, Serialize, Clone)]
struct AgentProfile {
    name: String,
    command: String,
    #[serde(default)]
    args: Vec<String>,
    #[serde(default)]
    env: BTreeMap<String, String>,
    role: String,
    allowed_steps: Vec<String>,
}

#[derive(Deserialize)]
struct PrepareWorktreeRequest {
    workspace: String,
    task_id: String,
    branch_name: String,
    base_ref: Option<String>,
}

#[derive(Serialize)]
struct WorktreeInfo {
    task_id: String,
    branch_name: String,
    worktree_path: String,
    created: bool,
}

#[derive(Deserialize)]
struct StartAgentSessionRequest {
    workspace: String,
    task_id: String,
    agent_type: String,
    provider: String,
    command: String,
    #[serde(default)]
    args: Vec<String>,
    worktree_path: String,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
struct PlanStep {
    id: String,
    title: String,
    description: String,
    status: String,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
struct ExecutionPlanInfo {
    id: String,
    task_id: String,
    summary: String,
    steps: Vec<PlanStep>,
    risks: Vec<String>,
    files_to_touch: Vec<String>,
    test_strategy: Vec<String>,
    approval_status: String,
}

#[derive(Deserialize)]
struct SaveExecutionPlanRequest {
    workspace: String,
    plan: ExecutionPlanInfo,
}

#[derive(Deserialize)]
struct TaskPlanRequest {
    workspace: String,
    task_id: String,
}

#[derive(Serialize, Clone)]
struct WorkflowEvent {
    task_id: String,
    event: String,
    stage: String,
    status: String,
    artifact: String,
    created_at: u64,
}

#[derive(Deserialize)]
struct TaskWorktreeRequest {
    workspace: String,
    task_id: String,
    worktree_path: String,
}

#[derive(Serialize, Deserialize, Clone)]
struct ChangedFileInfo {
    path: String,
    status: String,
}

#[derive(Serialize, Deserialize, Clone)]
struct GitDiffInfo {
    task_id: String,
    summary: String,
    changed_files: Vec<ChangedFileInfo>,
    patch: String,
}

#[derive(Deserialize)]
struct RunTestsRequest {
    workspace: String,
    task_id: String,
    worktree_path: String,
    command: String,
}

#[derive(Serialize, Deserialize, Clone)]
struct TestRunInfo {
    task_id: String,
    command: String,
    status: String,
    stdout: String,
    stderr: String,
    code: Option<i32>,
}

#[derive(Deserialize)]
struct SaveReviewRequest {
    workspace: String,
    review: ReviewInfo,
}

#[derive(Deserialize)]
struct ApproveReviewRequest {
    workspace: String,
    task_id: String,
}

#[derive(Serialize, Deserialize, Clone)]
struct ReviewInfo {
    id: String,
    task_id: String,
    diff_summary: String,
    changed_files: Vec<String>,
    test_results: Vec<TestRunInfo>,
    reviewer_notes: String,
    status: String,
}

#[derive(Deserialize)]
struct MemoryWriteRequest {
    workspace: String,
    node: MemoryNodeInfo,
}

#[derive(Deserialize)]
struct MemorySearchRequest {
    workspace: String,
    query: String,
}

#[derive(Serialize, Deserialize, Clone)]
struct MemoryNodeInfo {
    id: String,
    project_id: String,
    node_type: String,
    title: String,
    content: String,
    links: Vec<String>,
    created_at: u64,
}

#[derive(Serialize, Deserialize, Clone)]
struct AgentSessionInfo {
    id: String,
    task_id: String,
    agent_type: String,
    provider: String,
    command: String,
    args: Vec<String>,
    status: String,
    pty_session_id: String,
    conversation_log_path: String,
    worktree_path: String,
    started_at: u64,
    ended_at: Option<u64>,
}

#[derive(Serialize, Clone)]
struct WorkbenchProjectInfo {
    id: String,
    name: String,
    root_path: String,
    repos: Vec<String>,
    settings: BTreeMap<String, String>,
}

#[derive(Serialize, Clone)]
struct WorkbenchFeatureInfo {
    id: String,
    project_id: String,
    title: String,
    description: String,
    status: String,
    plan_graph_id: Option<String>,
    created_at: String,
}

#[derive(Serialize, Clone)]
struct WorkbenchTaskInfo {
    id: String,
    feature_id: String,
    parent_task_id: Option<String>,
    title: String,
    description: String,
    status: String,
    priority: String,
    assigned_agent: String,
    executor_profile: String,
    worktree_path: String,
    branch_name: String,
    review_approved: bool,
    tests_passed: bool,
    updated_at: String,
    tags: Vec<String>,
    progress: u8,
}

#[derive(Serialize, Clone)]
struct WorkbenchSnapshot {
    project: WorkbenchProjectInfo,
    features: Vec<WorkbenchFeatureInfo>,
    tasks: Vec<WorkbenchTaskInfo>,
    plans: Vec<ExecutionPlanInfo>,
    sessions: Vec<AgentSessionInfo>,
    reviews: Vec<ReviewInfo>,
    memory_nodes: Vec<MemoryNodeInfo>,
}

#[derive(Serialize, Clone)]
struct AgentOutputEvent {
    session_id: String,
    task_id: String,
    data: String,
}

#[derive(Serialize, Clone)]
struct AgentExitEvent {
    session_id: String,
    task_id: String,
    code: i32,
}

#[tauri::command]
fn run_thanos(workspace: String, binary: String, args: Vec<String>) -> Result<CliResult, String> {
    let workspace_path = validate_workspace(&workspace)?;
    if args.is_empty() {
        return Err("at least one CLI argument is required".to_string());
    }

    let binary = normalize_binary(binary);

    let output = Command::new(&binary)
        .args(&args)
        .current_dir(&workspace_path)
        .output()
        .map_err(|err| format!("failed to run {binary}: {err}"))?;

    Ok(CliResult {
        command: format!("{} {}", binary, shell_join(&args)),
        stdout: String::from_utf8_lossy(&output.stdout).to_string(),
        stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        code: output.status.code(),
    })
}

#[tauri::command]
fn select_workspace_folder() -> Result<Option<String>, String> {
    pick_folder()
}

#[tauri::command]
fn current_workspace_folder() -> Result<String, String> {
    std::env::current_dir()
        .map(|path| path.display().to_string())
        .map_err(|err| format!("failed to read current directory: {err}"))
}

#[tauri::command]
fn ensure_workspace(workspace: String, binary: String) -> Result<WorkspaceInfo, String> {
    let workspace_path = validate_directory(&workspace)?;
    if is_initialized_workspace(&workspace_path) {
        return Ok(WorkspaceInfo {
            path: workspace_path.display().to_string(),
            initialized: true,
            init_output: None,
        });
    }

    let binary = normalize_binary(binary);
    let args = vec!["init".to_string()];
    let output = Command::new(&binary)
        .args(&args)
        .current_dir(&workspace_path)
        .output()
        .map_err(|err| format!("failed to initialize workspace with {binary}: {err}"))?;
    let result = CliResult {
        command: format!("{} {}", binary, shell_join(&args)),
        stdout: String::from_utf8_lossy(&output.stdout).to_string(),
        stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        code: output.status.code(),
    };
    if !output.status.success() {
        return Err(format!(
            "failed to initialize Thanos workspace in {}: {}",
            workspace_path.display(),
            result.stderr.trim()
        ));
    }
    Ok(WorkspaceInfo {
        path: workspace_path.display().to_string(),
        initialized: false,
        init_output: Some(result),
    })
}

#[tauri::command]
fn detect_agent_clis() -> Vec<AgentCandidate> {
    known_agents()
        .into_iter()
        .map(|profile| {
            let path = find_on_path(&profile.command);
            AgentCandidate {
                name: profile.name,
                command: profile.command,
                installed: path.is_some(),
                path: path.map(|value| value.display().to_string()),
                default_args: profile.args,
                role: profile.role,
                allowed_steps: profile.allowed_steps,
            }
        })
        .collect()
}

#[tauri::command]
fn read_agent_profiles(workspace: String) -> Result<AgentsConfig, String> {
    let workspace_path = validate_workspace(&workspace)?;
    let path = workspace_path.join(".thanos").join("agents.yaml");
    let data = fs::read_to_string(&path)
        .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    parse_agents_yaml(&data)
}

#[tauri::command]
fn read_project_config(workspace: String) -> Result<ProjectConfig, String> {
    let workspace_path = validate_workspace(&workspace)?;
    let path = workspace_path.join(".thanos").join("settings.json");
    let data = fs::read_to_string(&path)
        .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    let value: serde_json::Value = serde_json::from_str(&data)
        .map_err(|err| format!("failed to parse {}: {err}", path.display()))?;

    let project = value.get("project");
    let string_field = |parent: Option<&serde_json::Value>, key: &str| -> String {
        parent
            .and_then(|node| node.get(key))
            .and_then(|node| node.as_str())
            .unwrap_or_default()
            .to_string()
    };

    let mut skills = Vec::new();
    if let Some(items) = value.get("skills").and_then(|node| node.as_array()) {
        for item in items {
            let roles = item
                .get("roles")
                .and_then(|node| node.as_array())
                .map(|list| {
                    list.iter()
                        .filter_map(|role| role.as_str().map(|value| value.to_string()))
                        .collect::<Vec<_>>()
                })
                .unwrap_or_default();
            skills.push(SkillInfo {
                name: string_field(Some(item), "name"),
                source: string_field(Some(item), "source"),
                roles,
            });
        }
    }

    let mut mcp = Vec::new();
    if let Some(map) = value.get("mcp").and_then(|node| node.as_object()) {
        mcp = map.keys().cloned().collect();
        mcp.sort();
    }

    Ok(ProjectConfig {
        name: string_field(project, "name"),
        language: string_field(project, "language"),
        framework: string_field(project, "framework"),
        default_runner: string_field(Some(&value), "default_runner"),
        skills,
        mcp,
    })
}

#[tauri::command]
fn write_agent_profile(workspace: String, profile: AgentProfile) -> Result<AgentsConfig, String> {
    let workspace_path = validate_workspace(&workspace)?;
    let path = workspace_path.join(".thanos").join("agents.yaml");
    let data = fs::read_to_string(&path).unwrap_or_else(|_| "agents: []\n".to_string());
    let mut config = parse_agents_yaml(&data)?;
    if let Some(existing) = config
        .agents
        .iter_mut()
        .find(|agent| agent.name == profile.name)
    {
        *existing = profile;
    } else {
        config.agents.push(profile);
    }
    fs::write(&path, render_agents_yaml(&config))
        .map_err(|err| format!("failed to write {}: {err}", path.display()))?;
    Ok(config)
}

#[tauri::command]
fn save_execution_plan(
    app: AppHandle,
    request: SaveExecutionPlanRequest,
) -> Result<ExecutionPlanInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let plan = normalize_plan(request.plan)?;
    write_execution_plan_files(&workspace_path, &plan)?;
    emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id: plan.task_id.clone(),
            event: "stage.completed".to_string(),
            stage: "plan".to_string(),
            status: "waiting_approval".to_string(),
            artifact: plan_artifact(&plan.task_id),
            created_at: now_epoch(),
        },
    )?;
    Ok(plan)
}

#[tauri::command]
fn read_execution_plan(request: TaskPlanRequest) -> Result<ExecutionPlanInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    read_execution_plan_file(&workspace_path, &task_id)
}

#[tauri::command]
fn approve_execution_plan(
    app: AppHandle,
    request: TaskPlanRequest,
) -> Result<ExecutionPlanInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    let mut plan = read_execution_plan_file(&workspace_path, &task_id)?;
    plan.approval_status = "approved".to_string();
    write_execution_plan_files(&workspace_path, &plan)?;
    emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id,
            event: "plan.approved".to_string(),
            stage: "plan".to_string(),
            status: "ready".to_string(),
            artifact: plan_artifact(&plan.task_id),
            created_at: now_epoch(),
        },
    )?;
    Ok(plan)
}

#[tauri::command]
fn collect_git_diff(request: TaskWorktreeRequest) -> Result<GitDiffInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    let worktree_path = resolve_child_path(&workspace_path, &request.worktree_path)?;
    if !worktree_path.exists() {
        return Err(format!(
            "worktree does not exist: {}",
            worktree_path.display()
        ));
    }
    let names = run_git_output(&worktree_path, &["diff", "--name-status"])?;
    let summary = run_git_output(&worktree_path, &["diff", "--stat"])?;
    let patch = run_git_output(&worktree_path, &["diff"])?;
    let changed_files = names
        .lines()
        .filter_map(|line| {
            let mut parts = line.split_whitespace();
            Some(ChangedFileInfo {
                status: parts.next()?.to_string(),
                path: parts.next()?.to_string(),
            })
        })
        .collect();
    Ok(GitDiffInfo {
        task_id,
        summary,
        changed_files,
        patch,
    })
}

#[tauri::command]
fn run_task_tests(app: AppHandle, request: RunTestsRequest) -> Result<TestRunInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    let worktree_path = resolve_child_path(&workspace_path, &request.worktree_path)?;
    if request.command.trim().is_empty() {
        return Err("test command is required".to_string());
    }
    let output = Command::new("sh")
        .args(["-lc", request.command.as_str()])
        .current_dir(&worktree_path)
        .output()
        .map_err(|err| format!("failed to run tests: {err}"))?;
    let info = TestRunInfo {
        task_id: task_id.clone(),
        command: request.command,
        status: if output.status.success() {
            "passed"
        } else {
            "failed"
        }
        .to_string(),
        stdout: String::from_utf8_lossy(&output.stdout).to_string(),
        stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        code: output.status.code(),
    };
    write_test_summary(&workspace_path, &info)?;
    emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id,
            event: "tests.completed".to_string(),
            stage: "verify".to_string(),
            status: info.status.clone(),
            artifact: format!(".thanos/tests/{}.json", info.task_id),
            created_at: now_epoch(),
        },
    )?;
    Ok(info)
}

#[tauri::command]
fn save_review(app: AppHandle, request: SaveReviewRequest) -> Result<ReviewInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let review = normalize_review(request.review)?;
    write_review_files(&workspace_path, &review)?;
    emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id: review.task_id.clone(),
            event: "review.saved".to_string(),
            stage: "review".to_string(),
            status: review.status.clone(),
            artifact: format!(".thanos/reviews/{}.json", review.task_id),
            created_at: now_epoch(),
        },
    )?;
    Ok(review)
}

#[tauri::command]
fn approve_review(app: AppHandle, request: ApproveReviewRequest) -> Result<ReviewInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    let mut review = read_review_file(&workspace_path, &task_id)?;
    let test = read_test_summary(&workspace_path, &task_id)?;
    if test.status != "passed" {
        return Err(format!(
            "task {task_id} cannot be approved before tests pass"
        ));
    }
    review.status = "approved".to_string();
    write_review_files(&workspace_path, &review)?;
    write_memory_from_review(&workspace_path, &review)?;
    emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id,
            event: "review.approved".to_string(),
            stage: "review".to_string(),
            status: "approved".to_string(),
            artifact: format!(".thanos/reviews/{}.json", review.task_id),
            created_at: now_epoch(),
        },
    )?;
    Ok(review)
}

#[tauri::command]
fn write_memory_node(request: MemoryWriteRequest) -> Result<MemoryNodeInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let conn = open_memory_db(&workspace_path)?;
    insert_memory_node(&conn, &request.node)?;
    Ok(request.node)
}

#[tauri::command]
fn search_memory(request: MemorySearchRequest) -> Result<Vec<MemoryNodeInfo>, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let conn = open_memory_db(&workspace_path)?;
    let query = request.query.trim();
    if query.is_empty() {
        return Ok(Vec::new());
    }
    let mut stmt = conn
        .prepare(
            "SELECT m.id, m.project_id, m.node_type, m.title, m.content, m.links_json, m.created_at
             FROM memory_nodes_fts f
             JOIN memory_nodes m ON m.rowid = f.rowid
             WHERE memory_nodes_fts MATCH ?
             ORDER BY rank
             LIMIT 20",
        )
        .map_err(|err| err.to_string())?;
    let rows = stmt
        .query_map(params![query], row_to_memory_node)
        .map_err(|err| err.to_string())?;
    let mut out = Vec::new();
    for row in rows {
        out.push(row.map_err(|err| err.to_string())?);
    }
    Ok(out)
}

#[tauri::command]
fn load_workbench_state(workspace: String) -> Result<WorkbenchSnapshot, String> {
    let workspace_path = validate_workspace(&workspace)?;
    WorkbenchRepository::new(workspace_path).load()
}

#[tauri::command]
fn prepare_task_worktree(request: PrepareWorktreeRequest) -> Result<WorktreeInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    let task_id = sanitize_id(&request.task_id)?;
    let branch_name = validate_branch(&request.branch_name)?;
    let worktrees_dir = workspace_path.join(".thanos").join("worktrees");
    fs::create_dir_all(&worktrees_dir)
        .map_err(|err| format!("failed to create {}: {err}", worktrees_dir.display()))?;
    let worktree_path = worktrees_dir.join(&task_id);

    if worktree_path.exists() {
        if !worktree_path.join(".git").exists() {
            return Err(format!(
                "worktree path exists but is not a git worktree: {}",
                worktree_path.display()
            ));
        }
        return Ok(WorktreeInfo {
            task_id,
            branch_name,
            worktree_path: worktree_path.display().to_string(),
            created: false,
        });
    }

    run_git(&workspace_path, &["rev-parse", "--is-inside-work-tree"])?;
    let branch_ref = format!("refs/heads/{branch_name}");
    let branch_exists = Command::new("git")
        .args(["show-ref", "--verify", "--quiet", &branch_ref])
        .current_dir(&workspace_path)
        .status()
        .map_err(|err| format!("failed to inspect branch {branch_name}: {err}"))?
        .success();
    if branch_exists {
        run_git(
            &workspace_path,
            &["worktree", "add", path_str(&worktree_path)?, &branch_name],
        )?;
    } else {
        let base = request.base_ref.as_deref().unwrap_or("HEAD");
        run_git(
            &workspace_path,
            &[
                "worktree",
                "add",
                "-b",
                &branch_name,
                path_str(&worktree_path)?,
                base,
            ],
        )?;
    }
    Ok(WorktreeInfo {
        task_id,
        branch_name,
        worktree_path: worktree_path.display().to_string(),
        created: true,
    })
}

#[tauri::command]
fn start_agent_session(
    app: AppHandle,
    session: State<'_, TerminalSession>,
    request: StartAgentSessionRequest,
) -> Result<AgentSessionInfo, String> {
    let workspace_path = validate_workspace(&request.workspace)?;
    if request.command.trim().is_empty() {
        return Err("agent command is required".to_string());
    }
    let session_cwd = if request.agent_type == "coder" {
        let worktree_path = resolve_child_path(&workspace_path, &request.worktree_path)?;
        if !worktree_path.exists() || !worktree_path.join(".git").exists() {
            return Err(format!(
                "coder session requires an existing task worktree: {}",
                worktree_path.display()
            ));
        }
        worktree_path
    } else {
        workspace_path.clone()
    };
    if request.agent_type == "coder" {
        let plan = read_execution_plan_file(&workspace_path, &request.task_id)?;
        if plan.approval_status != "approved" {
            return Err(format!(
                "task {} cannot start coder before plan approval",
                request.task_id
            ));
        }
    }

    stop_active_session(&session);

    let session_id = format!(
        "{}-{}-{}",
        sanitize_id(&request.task_id)?,
        sanitize_id(&request.agent_type)?,
        now_epoch()
    );
    let logs_dir = workspace_path.join(".thanos").join("logs").join("sessions");
    fs::create_dir_all(&logs_dir)
        .map_err(|err| format!("failed to create {}: {err}", logs_dir.display()))?;
    let log_path = logs_dir.join(format!("{session_id}.log"));
    let metadata_path = logs_dir.join(format!("{session_id}.json"));

    let mut info = AgentSessionInfo {
        id: session_id.clone(),
        task_id: request.task_id.clone(),
        agent_type: request.agent_type.clone(),
        provider: request.provider.clone(),
        command: request.command.clone(),
        args: request.args.clone(),
        status: "running".to_string(),
        pty_session_id: session_id.clone(),
        conversation_log_path: log_path.display().to_string(),
        worktree_path: session_cwd.display().to_string(),
        started_at: now_epoch(),
        ended_at: None,
    };
    write_session_metadata(&metadata_path, &info)?;

    let pair = native_pty_system()
        .openpty(PtySize {
            rows: 24,
            cols: 100,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|err| format!("failed to open pty: {err}"))?;
    let mut cmd = CommandBuilder::new(&request.command);
    cmd.args(&request.args);
    cmd.cwd(&session_cwd);
    cmd.env("THANOS_PROJECT_ROOT", workspace_path.display().to_string());
    cmd.env("THANOS_TASK_ID", &request.task_id);

    let mut child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|err| format!("failed to run {}: {err}", request.command))?;
    drop(pair.slave);

    let killer = child.clone_killer();
    let writer = pair
        .master
        .take_writer()
        .map_err(|err| format!("failed to open pty writer: {err}"))?;
    let mut reader = pair
        .master
        .try_clone_reader()
        .map_err(|err| format!("failed to open pty reader: {err}"))?;

    *session.active_id.lock().unwrap() = Some(session_id.clone());
    *session.writer.lock().unwrap() = Some(writer);
    *session.killer.lock().unwrap() = Some(killer);
    *session.master.lock().unwrap() = Some(pair.master);

    let _ = append_log(
        &log_path,
        &format!("$ {} {}\n", request.command, shell_join(&request.args)),
    );
    let _ = app.emit("agent-session-started", info.clone());
    let _ = emit_workflow_event(
        &app,
        &workspace_path,
        WorkflowEvent {
            task_id: request.task_id.clone(),
            event: "agent.started".to_string(),
            stage: request.agent_type.clone(),
            status: "running".to_string(),
            artifact: log_path.display().to_string(),
            created_at: now_epoch(),
        },
    );

    let task_id = request.task_id.clone();
    let metadata_path_for_thread = metadata_path.clone();
    std::thread::spawn(move || {
        let mut buffer = [0u8; 4096];
        loop {
            match reader.read(&mut buffer) {
                Ok(0) => break,
                Ok(count) => {
                    let chunk = String::from_utf8_lossy(&buffer[..count]).to_string();
                    let _ = append_log(&log_path, &chunk);
                    let _ = app.emit(
                        "agent-session-output",
                        AgentOutputEvent {
                            session_id: session_id.clone(),
                            task_id: task_id.clone(),
                            data: chunk,
                        },
                    );
                }
                Err(_) => break,
            }
        }
        let code = child
            .wait()
            .map(|status| status.exit_code() as i32)
            .unwrap_or(-1);
        info.status = "stopped".to_string();
        info.ended_at = Some(now_epoch());
        let _ = write_session_metadata(&metadata_path_for_thread, &info);
        let _ = app.emit(
            "agent-session-exit",
            AgentExitEvent {
                session_id,
                task_id,
                code,
            },
        );
    });

    Ok(read_session_metadata(&metadata_path)?)
}

#[tauri::command]
fn stop_agent_session(session: State<'_, TerminalSession>) -> Result<(), String> {
    stop_active_session(&session);
    Ok(())
}

#[tauri::command]
fn resume_agent_session(workspace: String, session_id: String) -> Result<AgentSessionInfo, String> {
    let workspace_path = validate_workspace(&workspace)?;
    let id = sanitize_id(&session_id)?;
    let path = workspace_path
        .join(".thanos")
        .join("logs")
        .join("sessions")
        .join(format!("{id}.json"));
    read_session_metadata(&path)
}

/// Spawn a command in a PTY and stream its output to the frontend via
/// `terminal-output` events, emitting `terminal-exit` with the exit code when
/// it finishes. Replaces any previously running session.
#[tauri::command]
fn spawn_terminal(
    app: AppHandle,
    session: State<'_, TerminalSession>,
    workspace: String,
    binary: String,
    args: Vec<String>,
) -> Result<(), String> {
    let workspace_path = validate_workspace(&workspace)?;
    let binary = normalize_binary(binary);

    if let Some(mut killer) = session.killer.lock().unwrap().take() {
        let _ = killer.kill();
    }

    let pair = native_pty_system()
        .openpty(PtySize {
            rows: 24,
            cols: 100,
            pixel_width: 0,
            pixel_height: 0,
        })
        .map_err(|err| format!("failed to open pty: {err}"))?;

    let mut cmd = CommandBuilder::new(&binary);
    cmd.args(&args);
    cmd.cwd(&workspace_path);
    cmd.env("THANOS_PROJECT_ROOT", workspace_path.display().to_string());

    let mut child = pair
        .slave
        .spawn_command(cmd)
        .map_err(|err| format!("failed to run {binary}: {err}"))?;
    drop(pair.slave);

    let killer = child.clone_killer();
    let writer = pair
        .master
        .take_writer()
        .map_err(|err| format!("failed to open pty writer: {err}"))?;
    let mut reader = pair
        .master
        .try_clone_reader()
        .map_err(|err| format!("failed to open pty reader: {err}"))?;

    *session.writer.lock().unwrap() = Some(writer);
    *session.killer.lock().unwrap() = Some(killer);
    *session.master.lock().unwrap() = Some(pair.master);

    let command_line = format!("{} {}", binary, shell_join(&args));
    let _ = app.emit(
        "terminal-output",
        format!("\x1b[38;5;244m$ {command_line}\x1b[0m\r\n"),
    );

    std::thread::spawn(move || {
        let mut buffer = [0u8; 4096];
        loop {
            match reader.read(&mut buffer) {
                Ok(0) => break,
                Ok(count) => {
                    let chunk = String::from_utf8_lossy(&buffer[..count]).to_string();
                    if app.emit("terminal-output", chunk).is_err() {
                        break;
                    }
                }
                Err(_) => break,
            }
        }
        let code = child
            .wait()
            .map(|status| status.exit_code() as i32)
            .unwrap_or(-1);
        let _ = app.emit("terminal-exit", code);
    });

    Ok(())
}

#[tauri::command]
fn write_terminal(session: State<'_, TerminalSession>, data: String) -> Result<(), String> {
    let mut guard = session.writer.lock().unwrap();
    if let Some(writer) = guard.as_mut() {
        writer
            .write_all(data.as_bytes())
            .map_err(|err| err.to_string())?;
        writer.flush().map_err(|err| err.to_string())?;
    }
    Ok(())
}

#[tauri::command]
fn resize_terminal(
    session: State<'_, TerminalSession>,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    if let Some(master) = session.master.lock().unwrap().as_ref() {
        master
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|err| err.to_string())?;
    }
    Ok(())
}

#[tauri::command]
fn kill_terminal(session: State<'_, TerminalSession>) -> Result<(), String> {
    if let Some(mut killer) = session.killer.lock().unwrap().take() {
        let _ = killer.kill();
    }
    Ok(())
}

fn stop_active_session(session: &State<'_, TerminalSession>) {
    if let Some(mut killer) = session.killer.lock().unwrap().take() {
        let _ = killer.kill();
    }
    *session.writer.lock().unwrap() = None;
    *session.master.lock().unwrap() = None;
    *session.active_id.lock().unwrap() = None;
}

fn validate_workspace(value: &str) -> Result<PathBuf, String> {
    let path = validate_directory(value)?;
    if !is_initialized_workspace(&path) {
        return Err(format!(
            "workspace is not initialized; missing {}",
            path.join(".thanos").join("settings.json").display()
        ));
    }
    Ok(path)
}

fn validate_directory(value: &str) -> Result<PathBuf, String> {
    let path = PathBuf::from(value.trim());
    if !path.exists() {
        return Err(format!("workspace does not exist: {}", path.display()));
    }
    if !path.is_dir() {
        return Err(format!("workspace is not a directory: {}", path.display()));
    }
    Ok(path)
}

fn resolve_child_path(root: &Path, value: &str) -> Result<PathBuf, String> {
    let raw = PathBuf::from(value.trim());
    let path = if raw.is_absolute() {
        raw
    } else {
        root.join(raw)
    };
    let canonical_root = root
        .canonicalize()
        .map_err(|err| format!("failed to resolve {}: {err}", root.display()))?;
    let parent = path.parent().unwrap_or(root);
    let canonical_parent = parent
        .canonicalize()
        .map_err(|err| format!("failed to resolve {}: {err}", parent.display()))?;
    if !canonical_parent.starts_with(&canonical_root) {
        return Err(format!("path escapes workspace: {}", path.display()));
    }
    Ok(path)
}

fn sanitize_id(value: &str) -> Result<String, String> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return Err("id is required".to_string());
    }
    if !trimmed
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '-' | '_' | '.'))
    {
        return Err(format!("id contains unsupported characters: {trimmed}"));
    }
    Ok(trimmed.to_string())
}

fn validate_branch(value: &str) -> Result<String, String> {
    let branch = value.trim();
    if branch.is_empty() {
        return Err("branch name is required".to_string());
    }
    if matches!(branch, "main" | "master" | "trunk") {
        return Err(format!("refusing to use protected branch {branch}"));
    }
    if branch.contains("..")
        || branch.starts_with('/')
        || branch.ends_with('/')
        || branch.contains(' ')
    {
        return Err(format!("invalid branch name: {branch}"));
    }
    Ok(branch.to_string())
}

fn run_git(workspace: &Path, args: &[&str]) -> Result<(), String> {
    let output = Command::new("git")
        .args(args)
        .current_dir(workspace)
        .output()
        .map_err(|err| format!("failed to run git {}: {err}", args.join(" ")))?;
    if !output.status.success() {
        return Err(format!(
            "git {} failed: {}",
            args.join(" "),
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    Ok(())
}

fn run_git_output(workspace: &Path, args: &[&str]) -> Result<String, String> {
    let output = Command::new("git")
        .args(args)
        .current_dir(workspace)
        .output()
        .map_err(|err| format!("failed to run git {}: {err}", args.join(" ")))?;
    if !output.status.success() {
        return Err(format!(
            "git {} failed: {}",
            args.join(" "),
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

fn path_str(path: &Path) -> Result<&str, String> {
    path.to_str()
        .ok_or_else(|| format!("path is not valid utf-8: {}", path.display()))
}

fn now_epoch() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or(0)
}

fn normalize_plan(mut plan: ExecutionPlanInfo) -> Result<ExecutionPlanInfo, String> {
    plan.task_id = sanitize_id(&plan.task_id)?;
    if plan.id.trim().is_empty() {
        plan.id = format!("plan-{}", plan.task_id);
    }
    if plan.summary.trim().is_empty() {
        return Err("execution plan summary is required".to_string());
    }
    if plan.steps.is_empty() {
        return Err("execution plan requires at least one step".to_string());
    }
    if plan.approval_status.trim().is_empty() {
        plan.approval_status = "pending".to_string();
    }
    Ok(plan)
}

fn plan_artifact(task_id: &str) -> String {
    format!(".thanos/plans/{task_id}.md")
}

fn plan_json_path(workspace: &Path, task_id: &str) -> PathBuf {
    workspace
        .join(".thanos")
        .join("plans")
        .join(format!("{task_id}.json"))
}

fn plan_md_path(workspace: &Path, task_id: &str) -> PathBuf {
    workspace
        .join(".thanos")
        .join("plans")
        .join(format!("{task_id}.md"))
}

fn write_execution_plan_files(workspace: &Path, plan: &ExecutionPlanInfo) -> Result<(), String> {
    let plans_dir = workspace.join(".thanos").join("plans");
    fs::create_dir_all(&plans_dir)
        .map_err(|err| format!("failed to create {}: {err}", plans_dir.display()))?;
    let json = serde_json::to_string_pretty(plan).map_err(|err| err.to_string())?;
    fs::write(plan_json_path(workspace, &plan.task_id), json)
        .map_err(|err| format!("failed to write execution plan json: {err}"))?;
    fs::write(
        plan_md_path(workspace, &plan.task_id),
        render_plan_markdown(plan),
    )
    .map_err(|err| format!("failed to write execution plan markdown: {err}"))
}

fn read_execution_plan_file(workspace: &Path, task_id: &str) -> Result<ExecutionPlanInfo, String> {
    let path = plan_json_path(workspace, task_id);
    let data = fs::read_to_string(&path)
        .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    serde_json::from_str(&data).map_err(|err| format!("failed to parse {}: {err}", path.display()))
}

fn render_plan_markdown(plan: &ExecutionPlanInfo) -> String {
    let mut out = format!(
        "# Execution Plan: {}\n\n## Summary\n\n{}\n\n## Steps\n\n",
        plan.task_id, plan.summary
    );
    for step in &plan.steps {
        out.push_str(&format!(
            "- [{}] {} — {}\n",
            if step.status == "done" { "x" } else { " " },
            step.title,
            step.description
        ));
    }
    out.push_str("\n## Files To Touch\n\n");
    for file in &plan.files_to_touch {
        out.push_str(&format!("- `{file}`\n"));
    }
    out.push_str("\n## Risks\n\n");
    for risk in &plan.risks {
        out.push_str(&format!("- {risk}\n"));
    }
    out.push_str("\n## Test Strategy\n\n");
    for item in &plan.test_strategy {
        out.push_str(&format!("- {item}\n"));
    }
    out.push_str(&format!(
        "\n## Approval\n\napproval_status: {}\n",
        plan.approval_status
    ));
    out
}

fn emit_workflow_event(
    app: &AppHandle,
    workspace: &Path,
    event: WorkflowEvent,
) -> Result<(), String> {
    let events_dir = workspace.join(".thanos").join("events");
    fs::create_dir_all(&events_dir)
        .map_err(|err| format!("failed to create {}: {err}", events_dir.display()))?;
    let line = serde_json::to_string(&event).map_err(|err| err.to_string())? + "\n";
    append_log(&events_dir.join("workflow.jsonl"), &line)?;
    app.emit("workflow-event", event)
        .map_err(|err| err.to_string())
}

fn normalize_review(mut review: ReviewInfo) -> Result<ReviewInfo, String> {
    review.task_id = sanitize_id(&review.task_id)?;
    if review.id.trim().is_empty() {
        review.id = format!("review-{}", review.task_id);
    }
    if review.status.trim().is_empty() {
        review.status = "pending".to_string();
    }
    Ok(review)
}

fn review_json_path(workspace: &Path, task_id: &str) -> PathBuf {
    workspace
        .join(".thanos")
        .join("reviews")
        .join(format!("{task_id}.json"))
}

fn test_json_path(workspace: &Path, task_id: &str) -> PathBuf {
    workspace
        .join(".thanos")
        .join("tests")
        .join(format!("{task_id}.json"))
}

fn write_review_files(workspace: &Path, review: &ReviewInfo) -> Result<(), String> {
    let dir = workspace.join(".thanos").join("reviews");
    fs::create_dir_all(&dir).map_err(|err| format!("failed to create {}: {err}", dir.display()))?;
    fs::write(
        review_json_path(workspace, &review.task_id),
        serde_json::to_string_pretty(review).map_err(|err| err.to_string())?,
    )
    .map_err(|err| err.to_string())?;
    fs::write(
        dir.join(format!("{}.md", review.task_id)),
        render_review_markdown(review),
    )
    .map_err(|err| err.to_string())
}

fn read_review_file(workspace: &Path, task_id: &str) -> Result<ReviewInfo, String> {
    let data =
        fs::read_to_string(review_json_path(workspace, task_id)).map_err(|err| err.to_string())?;
    serde_json::from_str(&data).map_err(|err| err.to_string())
}

fn render_review_markdown(review: &ReviewInfo) -> String {
    let mut out = format!(
        "# Review: {}\n\n## Result\n\n{}\n\n## Findings\n\n{}\n\n## Changed Files\n\n",
        review.task_id, review.status, review.reviewer_notes
    );
    for file in &review.changed_files {
        out.push_str(&format!("- `{file}`\n"));
    }
    out
}

fn write_test_summary(workspace: &Path, info: &TestRunInfo) -> Result<(), String> {
    let dir = workspace.join(".thanos").join("tests");
    fs::create_dir_all(&dir).map_err(|err| format!("failed to create {}: {err}", dir.display()))?;
    fs::write(
        test_json_path(workspace, &info.task_id),
        serde_json::to_string_pretty(info).map_err(|err| err.to_string())?,
    )
    .map_err(|err| err.to_string())?;
    fs::write(
        dir.join(format!("{}.md", info.task_id)),
        render_test_markdown(info),
    )
    .map_err(|err| err.to_string())
}

fn read_test_summary(workspace: &Path, task_id: &str) -> Result<TestRunInfo, String> {
    let data =
        fs::read_to_string(test_json_path(workspace, task_id)).map_err(|err| err.to_string())?;
    serde_json::from_str(&data).map_err(|err| err.to_string())
}

fn render_test_markdown(info: &TestRunInfo) -> String {
    format!(
        "# Test Summary: {}\n\n## Result\n\n{}\n\n## Commands\n\n```text\n{}\n```\n\n## Status\n\ntests_passed: {}\n",
        info.task_id,
        info.status,
        info.command,
        info.status == "passed"
    )
}

fn open_memory_db(workspace: &Path) -> Result<Connection, String> {
    let dir = workspace.join(".thanos").join("memory");
    fs::create_dir_all(&dir).map_err(|err| format!("failed to create {}: {err}", dir.display()))?;
    let conn = Connection::open(dir.join("workbench.sqlite")).map_err(|err| err.to_string())?;
    conn.execute_batch(
        "CREATE TABLE IF NOT EXISTS memory_nodes (
            id TEXT PRIMARY KEY,
            project_id TEXT NOT NULL,
            node_type TEXT NOT NULL,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            links_json TEXT NOT NULL,
            created_at INTEGER NOT NULL
        );
        CREATE VIRTUAL TABLE IF NOT EXISTS memory_nodes_fts USING fts5(title, content, content='memory_nodes', content_rowid='rowid');
        CREATE TRIGGER IF NOT EXISTS memory_nodes_ai AFTER INSERT ON memory_nodes BEGIN
            INSERT INTO memory_nodes_fts(rowid, title, content) VALUES (new.rowid, new.title, new.content);
        END;
        CREATE TRIGGER IF NOT EXISTS memory_nodes_ad AFTER DELETE ON memory_nodes BEGIN
            INSERT INTO memory_nodes_fts(memory_nodes_fts, rowid, title, content) VALUES('delete', old.rowid, old.title, old.content);
        END;
        CREATE TRIGGER IF NOT EXISTS memory_nodes_au AFTER UPDATE ON memory_nodes BEGIN
            INSERT INTO memory_nodes_fts(memory_nodes_fts, rowid, title, content) VALUES('delete', old.rowid, old.title, old.content);
            INSERT INTO memory_nodes_fts(rowid, title, content) VALUES (new.rowid, new.title, new.content);
        END;",
    )
    .map_err(|err| err.to_string())?;
    Ok(conn)
}

fn insert_memory_node(conn: &Connection, node: &MemoryNodeInfo) -> Result<(), String> {
    let links = serde_json::to_string(&node.links).map_err(|err| err.to_string())?;
    conn.execute(
        "INSERT INTO memory_nodes (id, project_id, node_type, title, content, links_json, created_at)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)
         ON CONFLICT(id) DO UPDATE SET title=excluded.title, content=excluded.content, links_json=excluded.links_json",
        params![node.id, node.project_id, node.node_type, node.title, node.content, links, node.created_at],
    )
    .map_err(|err| err.to_string())?;
    Ok(())
}

fn row_to_memory_node(row: &rusqlite::Row<'_>) -> rusqlite::Result<MemoryNodeInfo> {
    let links_json: String = row.get(5)?;
    let links = serde_json::from_str(&links_json).unwrap_or_default();
    Ok(MemoryNodeInfo {
        id: row.get(0)?,
        project_id: row.get(1)?,
        node_type: row.get(2)?,
        title: row.get(3)?,
        content: row.get(4)?,
        links,
        created_at: row.get(6)?,
    })
}

struct WorkbenchRepository {
    workspace: PathBuf,
}

impl WorkbenchRepository {
    fn new(workspace: PathBuf) -> Self {
        Self { workspace }
    }

    fn load(&self) -> Result<WorkbenchSnapshot, String> {
        let project = self.load_project()?;
        let features = self.load_features(&project);
        let plans = self.load_plans()?;
        let reviews = self.load_reviews()?;
        let sessions = self.load_sessions()?;
        let memory_nodes = self.load_memory_nodes()?;
        let tasks = self.load_tasks(&project, &plans, &reviews)?;
        Ok(WorkbenchSnapshot {
            project,
            features,
            tasks,
            plans,
            sessions,
            reviews,
            memory_nodes,
        })
    }

    fn load_project(&self) -> Result<WorkbenchProjectInfo, String> {
        let path = self.workspace.join(".thanos").join("settings.json");
        let data = fs::read_to_string(&path)
            .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
        let value: serde_json::Value = serde_json::from_str(&data)
            .map_err(|err| format!("failed to parse {}: {err}", path.display()))?;
        let project = value.get("project").unwrap_or(&serde_json::Value::Null);
        let name = string_field(project, "name").unwrap_or_else(|| "Thanos Workspace".to_string());
        let mut settings = BTreeMap::new();
        if let Some(language) = string_field(project, "language") {
            settings.insert("language".to_string(), language);
        }
        if let Some(default_runner) = string_field(&value, "default_runner") {
            settings.insert("defaultRunner".to_string(), default_runner);
        }
        if let Some(locale) = string_field(&value, "locale") {
            settings.insert("locale".to_string(), locale);
        }
        Ok(WorkbenchProjectInfo {
            id: slug_id(&name),
            name,
            root_path: self.workspace.display().to_string(),
            repos: vec![self
                .workspace
                .file_name()
                .and_then(|name| name.to_str())
                .unwrap_or("workspace")
                .to_string()],
            settings,
        })
    }

    fn load_features(&self, project: &WorkbenchProjectInfo) -> Vec<WorkbenchFeatureInfo> {
        vec![WorkbenchFeatureInfo {
            id: "local".to_string(),
            project_id: project.id.clone(),
            title: project.name.clone(),
            description: "Local Thanos workspace".to_string(),
            status: "active".to_string(),
            plan_graph_id: Some("local-plan-graph".to_string()),
            created_at: "1970-01-01T00:00:00Z".to_string(),
        }]
    }

    fn load_tasks(
        &self,
        project: &WorkbenchProjectInfo,
        plans: &[ExecutionPlanInfo],
        reviews: &[ReviewInfo],
    ) -> Result<Vec<WorkbenchTaskInfo>, String> {
        let mut tasks = Vec::new();
        for value in read_json_values(&self.workspace.join(".thanos").join("tasks"))? {
            if let Some(task) = task_from_value(project, &value) {
                tasks.push(task);
            }
        }
        for plan in plans {
            if tasks.iter().any(|task| task.id == plan.task_id) {
                continue;
            }
            let review = reviews.iter().find(|review| review.task_id == plan.task_id);
            tasks.push(task_from_plan(project, plan, review));
        }
        tasks.sort_by(|a, b| a.id.cmp(&b.id));
        Ok(tasks)
    }

    fn load_plans(&self) -> Result<Vec<ExecutionPlanInfo>, String> {
        let mut out =
            read_json_files::<ExecutionPlanInfo>(&self.workspace.join(".thanos").join("plans"))?;
        out.sort_by(|a, b| a.task_id.cmp(&b.task_id));
        Ok(out)
    }

    fn load_reviews(&self) -> Result<Vec<ReviewInfo>, String> {
        let mut out =
            read_json_files::<ReviewInfo>(&self.workspace.join(".thanos").join("reviews"))?;
        out.sort_by(|a, b| a.task_id.cmp(&b.task_id));
        Ok(out)
    }

    fn load_sessions(&self) -> Result<Vec<AgentSessionInfo>, String> {
        let mut out = read_json_files::<AgentSessionInfo>(
            &self.workspace.join(".thanos").join("logs").join("sessions"),
        )?;
        out.sort_by(|a, b| b.started_at.cmp(&a.started_at));
        Ok(out)
    }

    fn load_memory_nodes(&self) -> Result<Vec<MemoryNodeInfo>, String> {
        let conn = open_memory_db(&self.workspace)?;
        let mut stmt = conn
            .prepare(
                "SELECT id, project_id, node_type, title, content, links_json, created_at
                 FROM memory_nodes
                 ORDER BY created_at DESC
                 LIMIT 50",
            )
            .map_err(|err| err.to_string())?;
        let rows = stmt
            .query_map([], row_to_memory_node)
            .map_err(|err| err.to_string())?;
        let mut out = Vec::new();
        for row in rows {
            out.push(row.map_err(|err| err.to_string())?);
        }
        Ok(out)
    }
}

fn read_json_files<T>(dir: &Path) -> Result<Vec<T>, String>
where
    T: for<'de> Deserialize<'de>,
{
    let mut out = Vec::new();
    for value in read_json_values(dir)? {
        let item = serde_json::from_value(value).map_err(|err| err.to_string())?;
        out.push(item);
    }
    Ok(out)
}

fn read_json_values(dir: &Path) -> Result<Vec<serde_json::Value>, String> {
    if !dir.is_dir() {
        return Ok(Vec::new());
    }
    let mut values = Vec::new();
    let mut entries = fs::read_dir(dir)
        .map_err(|err| format!("failed to read {}: {err}", dir.display()))?
        .collect::<Result<Vec<_>, _>>()
        .map_err(|err| err.to_string())?;
    entries.sort_by_key(|entry| entry.path());
    for entry in entries {
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("json") {
            continue;
        }
        let data = fs::read_to_string(&path)
            .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
        values.push(
            serde_json::from_str(&data)
                .map_err(|err| format!("failed to parse {}: {err}", path.display()))?,
        );
    }
    Ok(values)
}

fn task_from_value(
    project: &WorkbenchProjectInfo,
    value: &serde_json::Value,
) -> Option<WorkbenchTaskInfo> {
    let id = string_field(value, "id")?;
    let title = string_field(value, "title").unwrap_or_else(|| id.clone());
    let status = map_task_status(
        string_field(value, "status")
            .as_deref()
            .unwrap_or("backlog"),
    );
    let review_approved = bool_field(value, "review_approved");
    let tests_passed = bool_field(value, "tests_passed");
    Some(WorkbenchTaskInfo {
        feature_id: string_field(value, "feature_id").unwrap_or_else(|| "local".to_string()),
        parent_task_id: string_field(value, "parent_task_id"),
        description: string_field(value, "description").unwrap_or_default(),
        priority: normalize_priority(string_field(value, "priority").as_deref()),
        assigned_agent: string_field(value, "assigned_agent")
            .unwrap_or_else(|| "Unassigned".to_string()),
        executor_profile: string_field(value, "executor_profile").unwrap_or_else(|| {
            project
                .settings
                .get("defaultRunner")
                .cloned()
                .unwrap_or_else(|| "codex".to_string())
        }),
        worktree_path: string_field(value, "worktree_path").unwrap_or_default(),
        branch_name: string_field(value, "branch_name").unwrap_or_default(),
        updated_at: string_field(value, "updated_at").unwrap_or_else(|| "unknown".to_string()),
        tags: string_array_field(value, "tags"),
        progress: progress_for_status(&status, review_approved, tests_passed),
        id,
        title,
        status,
        review_approved,
        tests_passed,
    })
}

fn task_from_plan(
    project: &WorkbenchProjectInfo,
    plan: &ExecutionPlanInfo,
    review: Option<&ReviewInfo>,
) -> WorkbenchTaskInfo {
    let approved = plan.approval_status == "approved";
    let review_approved = review
        .map(|review| review.status == "approved")
        .unwrap_or(false);
    let status = if review_approved {
        "done".to_string()
    } else if approved {
        "ready".to_string()
    } else {
        "waiting_approval".to_string()
    };
    WorkbenchTaskInfo {
        id: plan.task_id.clone(),
        feature_id: "local".to_string(),
        parent_task_id: None,
        title: title_from_task_id(&plan.task_id),
        description: plan.summary.clone(),
        status: status.clone(),
        priority: "P2".to_string(),
        assigned_agent: "Planner".to_string(),
        executor_profile: project
            .settings
            .get("defaultRunner")
            .cloned()
            .unwrap_or_else(|| "codex".to_string()),
        worktree_path: format!(".thanos/worktrees/{}", plan.task_id),
        branch_name: format!("thanos/{}", plan.task_id.to_lowercase()),
        review_approved,
        tests_passed: false,
        updated_at: "from plan artifact".to_string(),
        tags: plan
            .files_to_touch
            .iter()
            .take(2)
            .map(|file| file_tag(file))
            .collect(),
        progress: progress_for_status(&status, review_approved, false),
    }
}

fn string_field(value: &serde_json::Value, key: &str) -> Option<String> {
    value
        .get(key)
        .and_then(|item| item.as_str())
        .map(|item| item.to_string())
}

fn bool_field(value: &serde_json::Value, key: &str) -> bool {
    value
        .get(key)
        .and_then(|item| item.as_bool())
        .unwrap_or(false)
}

fn string_array_field(value: &serde_json::Value, key: &str) -> Vec<String> {
    value
        .get(key)
        .and_then(|item| item.as_array())
        .map(|items| {
            items
                .iter()
                .filter_map(|item| item.as_str().map(|value| value.to_string()))
                .collect()
        })
        .unwrap_or_default()
}

fn map_task_status(status: &str) -> String {
    match status {
        "plan" => "planning",
        "execute" => "running",
        "verify" => "in_review",
        "done" => "done",
        "blocked" => "blocked",
        "failed" => "failed",
        "waiting_approval" => "waiting_approval",
        "ready" => "ready",
        "running" => "running",
        "in_review" => "in_review",
        _ => "backlog",
    }
    .to_string()
}

fn normalize_priority(value: Option<&str>) -> String {
    match value {
        Some("P0" | "P1" | "P2" | "P3") => value.unwrap().to_string(),
        Some("high") => "P1".to_string(),
        Some("low") => "P3".to_string(),
        _ => "P2".to_string(),
    }
}

fn progress_for_status(status: &str, review_approved: bool, tests_passed: bool) -> u8 {
    match status {
        "done" => 100,
        "in_review" => {
            if review_approved && tests_passed {
                95
            } else {
                80
            }
        }
        "running" => 65,
        "ready" => 45,
        "waiting_approval" => 30,
        "planning" => 20,
        _ => 0,
    }
}

fn title_from_task_id(task_id: &str) -> String {
    task_id.replace(['-', '_'], " ")
}

fn file_tag(file: &str) -> String {
    file.split('/').next().unwrap_or(file).to_string()
}

fn slug_id(value: &str) -> String {
    let mut out = String::new();
    for ch in value.to_lowercase().chars() {
        if ch.is_ascii_alphanumeric() {
            out.push(ch);
        } else if !out.ends_with('-') {
            out.push('-');
        }
    }
    out.trim_matches('-').to_string()
}

fn write_memory_from_review(workspace: &Path, review: &ReviewInfo) -> Result<(), String> {
    let conn = open_memory_db(workspace)?;
    let node = MemoryNodeInfo {
        id: format!("review-{}", review.task_id),
        project_id: "local".to_string(),
        node_type: "task".to_string(),
        title: format!("Approved review for {}", review.task_id),
        content: format!(
            "{}\nChanged files: {}",
            review.reviewer_notes,
            review.changed_files.join(", ")
        ),
        links: review.changed_files.clone(),
        created_at: now_epoch(),
    };
    insert_memory_node(&conn, &node)
}

fn append_log(path: &Path, data: &str) -> Result<(), String> {
    let mut file = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .map_err(|err| format!("failed to open {}: {err}", path.display()))?;
    file.write_all(data.as_bytes())
        .map_err(|err| format!("failed to write {}: {err}", path.display()))
}

fn write_session_metadata(path: &Path, info: &AgentSessionInfo) -> Result<(), String> {
    let data = serde_json::to_string_pretty(info).map_err(|err| err.to_string())?;
    fs::write(path, data).map_err(|err| format!("failed to write {}: {err}", path.display()))
}

fn read_session_metadata(path: &Path) -> Result<AgentSessionInfo, String> {
    let data = fs::read_to_string(path)
        .map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    serde_json::from_str(&data).map_err(|err| format!("failed to parse {}: {err}", path.display()))
}

fn is_initialized_workspace(path: &Path) -> bool {
    path.join(".thanos").join("settings.json").is_file()
}

fn normalize_binary(binary: String) -> String {
    if binary.trim().is_empty() {
        "thanos".to_string()
    } else {
        binary
    }
}

#[cfg(target_os = "macos")]
fn pick_folder() -> Result<Option<String>, String> {
    let output = Command::new("osascript")
        .args([
            "-e",
            "POSIX path of (choose folder with prompt \"Select Thanos workspace project\")",
        ])
        .output()
        .map_err(|err| format!("failed to open folder picker: {err}"))?;
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        if stderr.to_lowercase().contains("user canceled") {
            return Ok(None);
        }
        return Err(format!("folder picker failed: {}", stderr.trim()));
    }
    let path = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if path.is_empty() {
        Ok(None)
    } else {
        Ok(Some(path))
    }
}

#[cfg(target_os = "windows")]
fn pick_folder() -> Result<Option<String>, String> {
    let script = "Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.FolderBrowserDialog; if ($d.ShowDialog() -eq 'OK') { $d.SelectedPath }";
    let output = Command::new("powershell")
        .args(["-NoProfile", "-Command", script])
        .output()
        .map_err(|err| format!("failed to open folder picker: {err}"))?;
    if !output.status.success() {
        return Err(format!(
            "folder picker failed: {}",
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    let path = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if path.is_empty() {
        Ok(None)
    } else {
        Ok(Some(path))
    }
}

#[cfg(all(not(target_os = "macos"), not(target_os = "windows")))]
fn pick_folder() -> Result<Option<String>, String> {
    let picker = if find_on_path("zenity").is_some() {
        ("zenity", vec!["--file-selection", "--directory"])
    } else if find_on_path("kdialog").is_some() {
        ("kdialog", vec!["--getexistingdirectory"])
    } else {
        return Err("no supported folder picker found; install zenity or kdialog, or paste the path manually".to_string());
    };
    let output = Command::new(picker.0)
        .args(picker.1)
        .output()
        .map_err(|err| format!("failed to open folder picker: {err}"))?;
    if !output.status.success() {
        return Ok(None);
    }
    let path = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if path.is_empty() {
        Ok(None)
    } else {
        Ok(Some(path))
    }
}

fn shell_join(args: &[String]) -> String {
    args.iter()
        .map(|arg| shell_quote(arg))
        .collect::<Vec<_>>()
        .join(" ")
}

fn shell_quote(value: &str) -> String {
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || "-_./:".contains(ch))
    {
        return value.to_string();
    }
    format!("'{}'", value.replace('\'', "'\\''"))
}

fn known_agents() -> Vec<AgentProfile> {
    vec![
        AgentProfile {
            name: "codex".to_string(),
            command: "codex".to_string(),
            args: vec![
                "exec".to_string(),
                "--full-auto".to_string(),
                "-".to_string(),
            ],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "claude".to_string(),
            command: "claude".to_string(),
            args: vec![
                "--print".to_string(),
                "--dangerously-skip-permissions".to_string(),
            ],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "gemini".to_string(),
            command: "gemini".to_string(),
            args: vec![],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "deepseek".to_string(),
            command: "deepseek".to_string(),
            args: vec![],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "opencode".to_string(),
            command: "opencode".to_string(),
            args: vec![],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "custom".to_string(),
            command: "".to_string(),
            args: vec![],
            env: BTreeMap::new(),
            role: "custom".to_string(),
            allowed_steps: vec![
                "plan".to_string(),
                "execute".to_string(),
                "verify".to_string(),
            ],
        },
    ]
}

fn find_on_path(command: &str) -> Option<PathBuf> {
    if command.trim().is_empty() {
        return None;
    }
    let candidate = Path::new(command);
    if candidate.components().count() > 1 && candidate.is_file() {
        return Some(candidate.to_path_buf());
    }
    let path_var = std::env::var_os("PATH")?;
    for dir in std::env::split_paths(&path_var) {
        let full = dir.join(command);
        if full.is_file() {
            return Some(full);
        }
        for ext in executable_extensions() {
            let candidate = dir.join(format!("{command}{ext}"));
            if candidate.is_file() {
                return Some(candidate);
            }
        }
    }
    None
}

/// Executable suffixes to probe on Windows (from PATHEXT, e.g. .exe/.cmd/.bat)
/// so agents installed as `claude.exe` are detected. Empty on other platforms.
#[cfg(target_os = "windows")]
fn executable_extensions() -> Vec<String> {
    std::env::var("PATHEXT")
        .unwrap_or_else(|_| ".EXE;.CMD;.BAT;.COM".to_string())
        .split(';')
        .filter(|ext| !ext.is_empty())
        .map(|ext| ext.to_lowercase())
        .collect()
}

#[cfg(not(target_os = "windows"))]
fn executable_extensions() -> Vec<String> {
    Vec::new()
}

fn parse_agents_yaml(data: &str) -> Result<AgentsConfig, String> {
    let mut agents = Vec::new();
    let mut current: Option<AgentProfile> = None;
    let mut current_field = String::new();
    for line in data.lines() {
        let trimmed = line.trim();
        if trimmed == "agents:" || trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        if let Some(rest) = trimmed.strip_prefix("- name:") {
            if let Some(profile) = current.take() {
                agents.push(profile);
            }
            current = Some(AgentProfile {
                name: unquote(rest.trim()),
                command: String::new(),
                args: Vec::new(),
                env: BTreeMap::new(),
                role: String::new(),
                allowed_steps: Vec::new(),
            });
            current_field.clear();
            continue;
        }
        let Some(profile) = current.as_mut() else {
            continue;
        };
        if let Some(value) = trimmed.strip_prefix("command:") {
            profile.command = unquote(value.trim());
            current_field.clear();
        } else if let Some(value) = trimmed.strip_prefix("role:") {
            profile.role = unquote(value.trim());
            current_field.clear();
        } else if let Some(value) = trimmed.strip_prefix("args:") {
            profile.args = parse_inline_list(value.trim());
            current_field = "args".to_string();
        } else if let Some(value) = trimmed.strip_prefix("allowed_steps:") {
            profile.allowed_steps = parse_inline_list(value.trim());
            current_field = "allowed_steps".to_string();
        } else if trimmed == "env:" {
            current_field = "env".to_string();
        } else if let Some(value) = trimmed.strip_prefix("- ") {
            match current_field.as_str() {
                "args" => profile.args.push(unquote(value.trim())),
                "allowed_steps" => profile.allowed_steps.push(unquote(value.trim())),
                _ => {}
            }
        } else if current_field == "env" {
            if let Some((key, value)) = trimmed.split_once(':') {
                profile
                    .env
                    .insert(key.trim().to_string(), unquote(value.trim()));
            }
        }
    }
    if let Some(profile) = current.take() {
        agents.push(profile);
    }
    Ok(AgentsConfig { agents })
}

fn parse_inline_list(value: &str) -> Vec<String> {
    let value = value.trim();
    if value.is_empty() || value == "[]" {
        return Vec::new();
    }
    if !value.starts_with('[') || !value.ends_with(']') {
        return Vec::new();
    }
    value
        .trim_start_matches('[')
        .trim_end_matches(']')
        .split(',')
        .map(|item| unquote(item.trim()))
        .filter(|item| !item.is_empty())
        .collect()
}

fn render_agents_yaml(config: &AgentsConfig) -> String {
    let mut out = String::from("agents:\n");
    for profile in &config.agents {
        out.push_str(&format!("  - name: {}\n", yaml_quote(&profile.name)));
        out.push_str(&format!("    command: {}\n", yaml_quote(&profile.command)));
        out.push_str(&format!(
            "    args: [{}]\n",
            profile
                .args
                .iter()
                .map(|item| yaml_quote(item))
                .collect::<Vec<_>>()
                .join(", ")
        ));
        out.push_str("    env:\n");
        for (key, value) in &profile.env {
            out.push_str(&format!("      {}: {}\n", key, yaml_quote(value)));
        }
        out.push_str(&format!("    role: {}\n", yaml_quote(&profile.role)));
        out.push_str(&format!(
            "    allowed_steps: [{}]\n",
            profile
                .allowed_steps
                .iter()
                .map(|item| yaml_quote(item))
                .collect::<Vec<_>>()
                .join(", ")
        ));
    }
    out
}

fn yaml_quote(value: &str) -> String {
    if value
        .chars()
        .all(|ch| ch.is_ascii_alphanumeric() || "_-./".contains(ch))
    {
        return value.to_string();
    }
    format!("\"{}\"", value.replace('"', "\\\""))
}

fn unquote(value: &str) -> String {
    value
        .trim()
        .trim_matches('"')
        .trim_matches('\'')
        .to_string()
}

pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            app.manage(TerminalSession::default());
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            run_thanos,
            select_workspace_folder,
            current_workspace_folder,
            ensure_workspace,
            detect_agent_clis,
            read_agent_profiles,
            read_project_config,
            write_agent_profile,
            save_execution_plan,
            read_execution_plan,
            approve_execution_plan,
            collect_git_diff,
            run_task_tests,
            save_review,
            approve_review,
            write_memory_node,
            search_memory,
            load_workbench_state,
            prepare_task_worktree,
            start_agent_session,
            stop_agent_session,
            resume_agent_session,
            spawn_terminal,
            write_terminal,
            resize_terminal,
            kill_terminal
        ])
        .run(tauri::generate_context!())
        .expect("error while running Thanos Desktop UI");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn shell_quote_leaves_safe_values_plain() {
        assert_eq!(shell_quote("task"), "task");
        assert_eq!(shell_quote("T001-demo"), "T001-demo");
    }

    #[test]
    fn shell_quote_wraps_spaces() {
        assert_eq!(shell_quote("hello world"), "'hello world'");
    }

    #[test]
    fn validate_workspace_requires_settings_file() {
        let temp = std::env::temp_dir().join("thanos-desktop-missing-workspace");
        let _ = std::fs::remove_dir_all(&temp);
        std::fs::create_dir_all(&temp).unwrap();
        let error = validate_workspace(std::path::Path::new(&temp).to_str().unwrap()).unwrap_err();
        assert!(error.contains("settings.json"));
        let _ = std::fs::remove_dir_all(&temp);
    }

    #[test]
    fn validate_branch_rejects_main() {
        let error = validate_branch("main").unwrap_err();
        assert!(error.contains("protected"));
    }

    #[test]
    fn sanitize_id_rejects_path_segments() {
        let error = sanitize_id("../T-1").unwrap_err();
        assert!(error.contains("unsupported"));
    }

    #[test]
    fn normalize_plan_requires_steps() {
        let error = normalize_plan(ExecutionPlanInfo {
            id: "".to_string(),
            task_id: "T-1".to_string(),
            summary: "Plan".to_string(),
            steps: Vec::new(),
            risks: Vec::new(),
            files_to_touch: Vec::new(),
            test_strategy: Vec::new(),
            approval_status: "pending".to_string(),
        })
        .unwrap_err();
        assert!(error.contains("step"));
    }

    #[test]
    fn render_plan_markdown_includes_approval_status() {
        let markdown = render_plan_markdown(&ExecutionPlanInfo {
            id: "plan-T-1".to_string(),
            task_id: "T-1".to_string(),
            summary: "Plan".to_string(),
            steps: vec![PlanStep {
                id: "1".to_string(),
                title: "Do work".to_string(),
                description: "Implement scoped change".to_string(),
                status: "pending".to_string(),
            }],
            risks: vec!["Risk".to_string()],
            files_to_touch: vec!["src/main.ts".to_string()],
            test_strategy: vec!["npm run build".to_string()],
            approval_status: "approved".to_string(),
        });
        assert!(markdown.contains("approval_status: approved"));
        assert!(markdown.contains("src/main.ts"));
    }

    #[test]
    fn render_review_markdown_lists_changed_files() {
        let markdown = render_review_markdown(&ReviewInfo {
            id: "review-T-1".to_string(),
            task_id: "T-1".to_string(),
            diff_summary: "summary".to_string(),
            changed_files: vec!["src/main.ts".to_string()],
            test_results: Vec::new(),
            reviewer_notes: "approved".to_string(),
            status: "approved".to_string(),
        });
        assert!(markdown.contains("src/main.ts"));
        assert!(markdown.contains("approved"));
    }

    #[test]
    fn render_test_markdown_records_passed_status() {
        let markdown = render_test_markdown(&TestRunInfo {
            task_id: "T-1".to_string(),
            command: "npm run build".to_string(),
            status: "passed".to_string(),
            stdout: String::new(),
            stderr: String::new(),
            code: Some(0),
        });
        assert!(markdown.contains("tests_passed: true"));
    }

    #[test]
    fn memory_db_searches_written_nodes() {
        let root = std::env::temp_dir().join(format!("thanos-memory-test-{}", now_epoch()));
        let _ = std::fs::remove_dir_all(&root);
        std::fs::create_dir_all(root.join(".thanos").join("memory")).unwrap();
        let conn = open_memory_db(&root).unwrap();
        insert_memory_node(
            &conn,
            &MemoryNodeInfo {
                id: "mem-1".to_string(),
                project_id: "local".to_string(),
                node_type: "decision".to_string(),
                title: "Cart Decision".to_string(),
                content: "Persist cart state after review approval".to_string(),
                links: vec!["cart.ts".to_string()],
                created_at: now_epoch(),
            },
        )
        .unwrap();
        let mut stmt = conn
            .prepare(
                "SELECT m.id, m.project_id, m.node_type, m.title, m.content, m.links_json, m.created_at
                 FROM memory_nodes_fts f JOIN memory_nodes m ON m.rowid = f.rowid
                 WHERE memory_nodes_fts MATCH ? LIMIT 1",
            )
            .unwrap();
        let mut rows = stmt.query_map(params!["cart"], row_to_memory_node).unwrap();
        assert_eq!(rows.next().unwrap().unwrap().id, "mem-1");
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn repository_infers_task_from_plan_artifact() {
        let root = std::env::temp_dir().join(format!("thanos-repository-test-{}", now_epoch()));
        let _ = std::fs::remove_dir_all(&root);
        std::fs::create_dir_all(root.join(".thanos").join("plans")).unwrap();
        std::fs::write(
            root.join(".thanos").join("settings.json"),
            r#"{"project":{"name":"Demo","language":"go"},"default_runner":"codex"}"#,
        )
        .unwrap();
        write_execution_plan_files(
            &root,
            &ExecutionPlanInfo {
                id: "plan-T-900".to_string(),
                task_id: "T-900".to_string(),
                summary: "Build repository loader".to_string(),
                steps: vec![PlanStep {
                    id: "1".to_string(),
                    title: "Load artifacts".to_string(),
                    description: "Read plans and memory".to_string(),
                    status: "pending".to_string(),
                }],
                risks: Vec::new(),
                files_to_touch: vec!["src/main.tsx".to_string()],
                test_strategy: vec!["npm run build".to_string()],
                approval_status: "pending".to_string(),
            },
        )
        .unwrap();

        let snapshot = WorkbenchRepository::new(root.clone()).load().unwrap();
        assert_eq!(snapshot.project.name, "Demo");
        assert_eq!(snapshot.plans.len(), 1);
        assert_eq!(snapshot.tasks[0].id, "T-900");
        assert_eq!(snapshot.tasks[0].status, "waiting_approval");
        let _ = std::fs::remove_dir_all(&root);
    }
}
