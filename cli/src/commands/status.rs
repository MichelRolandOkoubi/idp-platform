use anyhow::Result;
use clap::Args;
use colored::Colorize;

use crate::api_client::ApiClient;
use crate::output::{print_json, OutputFormat};

#[derive(Args, Debug)]
pub struct StatusArgs {
    /// Check specific component
    #[arg(long)]
    pub component: Option<String>,
}

pub async fn run(args: StatusArgs, client: ApiClient, output: &OutputFormat) -> Result<()> {
    let status = client.platform_status().await?;

    match output {
        OutputFormat::Json => print_json(&status),
        _ => {
            println!("{}", "🚀 IDP Platform Status".cyan().bold());
            println!();

            let check = |healthy: bool, name: &str, latency: u64, msg: Option<&str>| {
                let icon = if healthy {
                    "✓".green().bold()
                } else {
                    "✗".red().bold()
                };
                let status_str = if healthy {
                    "Healthy".green().to_string()
                } else {
                    "Unhealthy".red().to_string()
                };
                println!(
                    "  {} {:<20} {} ({}ms){}",
                    icon,
                    name,
                    status_str,
                    latency,
                    msg.map(|m| format!(" — {m}")).unwrap_or_default()
                );
            };

            check(
                status.control_plane.healthy,
                "Control Plane",
                status.control_plane.latency_ms,
                status.control_plane.message.as_deref(),
            );
            check(
                status.ml_engine.healthy,
                "ML Cost Engine",
                status.ml_engine.latency_ms,
                status.ml_engine.message.as_deref(),
            );
            check(
                status.kubernetes.healthy,
                "Kubernetes",
                status.kubernetes.latency_ms,
                status.kubernetes.message.as_deref(),
            );
            check(
                status.database.healthy,
                "Database",
                status.database.latency_ms,
                status.database.message.as_deref(),
            );

            println!();
            println!(
                "  {} {} apps across {} namespaces",
                "Workloads:".dimmed(),
                status.total_apps,
                status.total_namespaces
            );
        }
    }

    Ok(())
}