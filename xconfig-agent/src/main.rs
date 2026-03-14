// File: src/main.rs

mod config;
mod executor;
mod models;
mod result_store;
mod scheduler;

use crate::config::{load_agent_config, AgentConfig};
use crate::executor::run as run_playbook;
use clap::{Parser, Subcommand};
use std::path::{Path, PathBuf};
use tokio::fs;

#[derive(Parser, Debug)]
#[command(name = "xconfig-agent", version)]
#[command(about = "Xconfig Agent - lightweight local playbook runner")]
struct Cli {
    #[arg(long, value_name = "FILE", default_value = "/etc/xconfig-agent.conf")]
    config: PathBuf,
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand, Debug)]
enum Commands {
    /// Run once using playbook(s) from Git repo
    Oneshot,

    /// Run as daemon with interval from config file
    Daemon,

    /// Run full playbook from local file (array of plays)
    Playbook {
        #[arg(short, long)]
        file: PathBuf,
    },

    /// Print latest execution result from local store
    Status,

    /// Show version info
    Version,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let Cli { config, command } = Cli::parse();

    // 加载配置文件（共享）
    let agent_config: AgentConfig = load_agent_config(&config).await.unwrap_or_else(|e| {
        eprintln!("⚠️ Failed to load config: {}", e);
        std::process::exit(1);
    });

    match command {
        Commands::Oneshot => {
            let repo_dir = agent_config.repo_dir();
            let branch = agent_config.branch.as_deref().unwrap_or("main");

            config::init_or_update_repo(&agent_config.repo, branch, repo_dir)?;

            let workdir_prefix = agent_config
                .workdir
                .as_deref()
                .map(Path::new)
                .map(|p| {
                    if p.is_absolute() {
                        p.to_path_buf()
                    } else {
                        Path::new(repo_dir).join(p)
                    }
                })
                .unwrap_or_else(|| Path::new(repo_dir).to_path_buf());

            for path in &agent_config.playbook {
                let playbook_path = workdir_prefix.join(path);
                let content = fs::read_to_string(&playbook_path).await?;
                let parsed: Vec<models::Play> = serde_yaml::from_str(&content)?;
                let results = run_playbook(parsed).await?;
                result_store::persist(results, agent_config.status_dir(), agent_config.node_id())
                    .await?;
            }
        }

        Commands::Daemon => {
            scheduler::run_schedule(&agent_config).await?;
        }

        Commands::Playbook { file } => {
            let content = fs::read_to_string(file).await?;
            let parsed: Vec<models::Play> = serde_yaml::from_str(&content)?;
            let results = run_playbook(parsed).await?;
            result_store::persist(results, agent_config.status_dir(), agent_config.node_id())
                .await?;
        }

        Commands::Status => {
            result_store::print_latest(agent_config.status_dir()).await?;
        }

        Commands::Version => {
            println!("xconfig-agent version {}", env!("CARGO_PKG_VERSION"));
        }
    }

    Ok(())
}
