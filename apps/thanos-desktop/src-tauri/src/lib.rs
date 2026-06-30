use serde::Serialize;
use std::{path::PathBuf, process::Command};

#[derive(Serialize)]
struct CliResult {
    command: String,
    stdout: String,
    stderr: String,
    code: Option<i32>,
}

#[tauri::command]
fn run_thanos(workspace: String, binary: String, args: Vec<String>) -> Result<CliResult, String> {
    let workspace_path = validate_workspace(&workspace)?;
    if args.is_empty() {
        return Err("at least one CLI argument is required".to_string());
    }

    let binary = if binary.trim().is_empty() {
        "thanos".to_string()
    } else {
        binary
    };

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

fn validate_workspace(value: &str) -> Result<PathBuf, String> {
    let path = PathBuf::from(value.trim());
    if !path.exists() {
        return Err(format!("workspace does not exist: {}", path.display()));
    }
    if !path.is_dir() {
        return Err(format!("workspace is not a directory: {}", path.display()));
    }
    let thanos_dir = path.join(".thanos");
    if !thanos_dir.is_dir() {
        return Err(format!(
            "workspace is not initialized; missing {}",
            thanos_dir.display()
        ));
    }
    Ok(path)
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

pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![run_thanos])
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
    fn validate_workspace_requires_thanos_dir() {
        let temp = std::env::temp_dir().join("thanos-desktop-missing-workspace");
        let _ = std::fs::remove_dir_all(&temp);
        std::fs::create_dir_all(&temp).unwrap();
        let error = validate_workspace(std::path::Path::new(&temp).to_str().unwrap()).unwrap_err();
        assert!(error.contains(".thanos"));
        let _ = std::fs::remove_dir_all(&temp);
    }
}
