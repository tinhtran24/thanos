use serde::{Deserialize, Serialize};
use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
    process::Command,
};

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
    }
    None
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
        .invoke_handler(tauri::generate_handler![
            run_thanos,
            select_workspace_folder,
            current_workspace_folder,
            ensure_workspace,
            detect_agent_clis,
            read_agent_profiles,
            write_agent_profile
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
