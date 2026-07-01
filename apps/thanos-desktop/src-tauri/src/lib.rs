use portable_pty::{native_pty_system, ChildKiller, CommandBuilder, MasterPty, PtySize};
use serde::{Deserialize, Serialize};
use std::{
    collections::BTreeMap,
    fs,
    io::{Read, Write},
    path::{Path, PathBuf},
    process::Command,
    sync::Mutex,
};
use tauri::{AppHandle, Emitter, Manager, State};

/// One active PTY-backed terminal session. Only a single flow runs in the
/// embedded terminal at a time; spawning a new one replaces the previous.
#[derive(Default)]
struct TerminalSession {
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
    let data = fs::read_to_string(&path).map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    parse_agents_yaml(&data)
}

#[tauri::command]
fn read_project_config(workspace: String) -> Result<ProjectConfig, String> {
    let workspace_path = validate_workspace(&workspace)?;
    let path = workspace_path.join(".thanos").join("settings.json");
    let data = fs::read_to_string(&path).map_err(|err| format!("failed to read {}: {err}", path.display()))?;
    let value: serde_json::Value =
        serde_json::from_str(&data).map_err(|err| format!("failed to parse {}: {err}", path.display()))?;

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
    if let Some(existing) = config.agents.iter_mut().find(|agent| agent.name == profile.name) {
        *existing = profile;
    } else {
        config.agents.push(profile);
    }
    fs::write(&path, render_agents_yaml(&config)).map_err(|err| format!("failed to write {}: {err}", path.display()))?;
    Ok(config)
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
    let _ = app.emit("terminal-output", format!("\x1b[38;5;244m$ {command_line}\x1b[0m\r\n"));

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
        let code = child.wait().map(|status| status.exit_code() as i32).unwrap_or(-1);
        let _ = app.emit("terminal-exit", code);
    });

    Ok(())
}

#[tauri::command]
fn write_terminal(session: State<'_, TerminalSession>, data: String) -> Result<(), String> {
    let mut guard = session.writer.lock().unwrap();
    if let Some(writer) = guard.as_mut() {
        writer.write_all(data.as_bytes()).map_err(|err| err.to_string())?;
        writer.flush().map_err(|err| err.to_string())?;
    }
    Ok(())
}

#[tauri::command]
fn resize_terminal(session: State<'_, TerminalSession>, rows: u16, cols: u16) -> Result<(), String> {
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
            args: vec!["exec".to_string(), "--full-auto".to_string(), "-".to_string()],
            env: BTreeMap::new(),
            role: "implementation".to_string(),
            allowed_steps: vec!["plan".to_string(), "execute".to_string()],
        },
        AgentProfile {
            name: "claude".to_string(),
            command: "claude".to_string(),
            args: vec!["--print".to_string(), "--dangerously-skip-permissions".to_string()],
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
            allowed_steps: vec!["plan".to_string(), "execute".to_string(), "verify".to_string()],
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
                profile.env.insert(key.trim().to_string(), unquote(value.trim()));
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
        out.push_str(&format!("    args: [{}]\n", profile.args.iter().map(|item| yaml_quote(item)).collect::<Vec<_>>().join(", ")));
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
}
