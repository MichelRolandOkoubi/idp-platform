use anyhow::Result;
use clap::{Parser, Subcommand};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod api_client;
mod commands;
mod config;
mod error;
mod output;

use commands::{app, auth, cost, deploy, env, logs, status};
use output::OutputFormat;

#[derive(Parser)]
#[command(
    name = "idpctl",
    version,
    author= "Michel okoubi",
    about = "Internal Developer Platform CLI — deploy apps in seconds",
    long_about = None,
    propagate_version = true,
)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Commands,

    /// IDP API URL (overrides config)
    #[arg(long, global = true, env = "IDP_API_URL")]
    pub api_url: Option<String>,

    /// Kubernetes namespace
    #[arg(short = 'n', long, global = true, default_value = "default", env = "IDP_NAMESPACE")]
    pub namespace: String,

    /// Output format
    #[arg(short, long, global = true, default_value = "text", value_enum)]
    pub output: OutputFormat,

    /// Enable verbose logging
    #[arg(short, long, global = true)]
    pub verbose: bool,
}

#[derive(Subcommand)]
pub enum Commands {
    /// Deploy an application
    #[command(alias = "d")]
    Deploy(deploy::DeployArgs),

    /// Manage applications
    #[command(alias = "a")]
    App(app::AppArgs),

    /// Stream application logs
    #[command(alias = "l")]
    Logs(logs::LogsArgs),

    /// Cost estimation and history
    #[command(alias = "c")]
    Cost(cost::CostArgs),

    /// Authenticate with the IDP
    Auth(auth::AuthArgs),

    /// Manage environment variables
    Env(env::EnvArgs),

    /// Show platform status
    Status(status::StatusArgs),

    /// Generate shell completions
    Completions {
        #[arg(value_enum)]
        shell: clap_complete::Shell,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Init tracing
    let level = if cli.verbose { "debug" } else { "warn" };
    tracing_subscriber::registry()
        .with(tracing_subscriber::EnvFilter::new(
            std::env::var("RUST_LOG").unwrap_or_else(|_| level.to_string()),
        ))
        .with(tracing_subscriber::fmt::layer())
        .init();

    // Load config
    let cfg = config::Config::load()?;
    let api_url = cli
        .api_url
        .or(cfg.api_url.clone())
        .unwrap_or_else(|| "http://localhost:8080".to_string());

    let client = api_client::ApiClient::new(api_url, cfg.token.clone())?;

    match cli.command {
        Commands::Deploy(args) => {
            commands::deploy::run(args, client, &cli.namespace, &cli.output).await
        }
        Commands::App(args) => {
            commands::app::run(args, client, &cli.namespace, &cli.output).await
        }
        Commands::Logs(args) => commands::logs::run(args, client, &cli.namespace).await,
        Commands::Cost(args) => {
            commands::cost::run(args, client, &cli.namespace, &cli.output).await
        }
        Commands::Auth(args) => commands::auth::run(args, cfg).await,
        Commands::Env(args) => {
            commands::env::run(args, client, &cli.namespace, &cli.output).await
        }
        Commands::Status(args) => {
            commands::status::run(args, client, &cli.output).await
        }
        Commands::Completions { shell } => {
            use clap::CommandFactory;
            clap_complete::generate(
                shell,
                &mut Cli::command(),
                "idpctl",
                &mut std::io::stdout(),
            );
            Ok(())
        }
    }
}