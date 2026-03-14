// File: src/result_store.rs
// -------------------------
use chrono::Utc;
use serde::Serialize;
use std::cmp::Reverse;
use std::fs;
use std::fs::read_dir;
use std::io::Read;
use std::path::Path;

#[derive(Debug, Serialize)]
pub struct CommandResult {
    pub task: String,
    pub stdout: String,
    pub stderr: String,
    pub success: bool,
    pub return_code: i32,
}

pub async fn persist(
    results: Vec<CommandResult>,
    status_dir: &str,
    node_id: Option<&str>,
) -> anyhow::Result<()> {
    let json = serde_json::to_string_pretty(&results)?;
    let ts = Utc::now().format("%Y%m%d%H%M%S");
    let prefix = node_id
        .map(|value| format!("status-{}-", value))
        .unwrap_or_else(|| "status-".to_string());
    let path = Path::new(status_dir).join(format!("{}{}.json", prefix, ts));
    fs::create_dir_all(Path::new(status_dir))?;
    fs::write(path, json)?;
    Ok(())
}

pub async fn print_latest(status_dir: &str) -> anyhow::Result<()> {
    let path = Path::new(status_dir);
    let mut entries: Vec<_> = read_dir(path)?
        .filter_map(|e| e.ok())
        .filter(|e| e.file_name().to_string_lossy().starts_with("status-"))
        .collect();

    entries.sort_by_key(|e| Reverse(e.file_name().to_string_lossy().into_owned()));

    if let Some(latest) = entries.first() {
        let mut file = fs::File::open(latest.path())?;
        let mut contents = String::new();
        file.read_to_string(&mut contents)?;
        println!("{}", contents);
    } else {
        println!("No status files found.");
    }

    Ok(())
}
