use anyhow::Result;
use clap::Args;
use colored::Colorize;
use indicatif::{ProgressBar, ProgressStyle};
use std::collections::HashMap;
use std::time::Duration;

use crate::api_client::{ApiClient, CostEstimateRequest, DeployRequest, ResourceSpec};
use crate::output::{print_error, print_info, print_success, print_warning, OutputFormat};

#[derive(Args, Debug)]
pub struct DeployArgs {
    /// Application name
    #[arg(short, long)]
    pub name: String,

    /// Container image (e.g., nginx:latest)
    #[arg(short, long)]
    pub image: String,

    /// Number of replicas
    #[arg(short, long, default_value = "1")]
    pub replicas: i32,

    /// CPU request (e.g., 100m)
    #[arg(long, default_value = "100m")]
    pub cpu_request: String,

    /// CPU limit (e.g., 500m)
    #[arg(long, default_value = "500m")]
    pub cpu_limit: String,

    /// Memory request (e.g., 128Mi)
    #[arg(long, default_value = "128Mi")]
    pub memory_request: String,

    /// Memory limit (e.g., 512Mi)
    #[arg(long, default_value = "512Mi")]
    pub memory_limit: String,

    /// Environment variables (KEY=VALUE, repeatable)
    #[arg(short, long = "env", value_name = "KEY=VALUE")]
    pub env_vars: Vec<String>,

    /// Expose port
    #[arg(long)]
    pub port: Option<u16>,

    /// Dry run (show manifest only)
    #[arg(long)]
    pub dry_run: bool,

    /// Wait for deployment to complete
    #[arg(short, long)]
    pub wait: bool,

    /// Cloud region (for cost estimation)
    #[arg(long, default_value = "eu-west-1")]
    pub region: String,
}

pub async fn run(
    args: DeployArgs,
    client: ApiClient,
    namespace: &str,
    output: &OutputFormat,
) -> Result<()> {
    // Parse env vars
    let env_vars: HashMap<String, String> = args
        .env_vars
        .iter()
        .filter_map(|kv| {
            let mut parts = kv.splitn(2, '=');
            Some((parts.next()?.to_string(), parts.next()?.to_string()))
        })
        .collect();

    if args.dry_run {
        print_warning("DRY RUN — No changes will be applied");
    }

    // Show deploy plan
    println!();
    println!("  {} {}", "Application:".dimmed(), args.name.cyan().bold());
    println!("  {} {}", "Image:      ".dimmed(), args.image.yellow());
    println!("  {} {}", "Namespace:  ".dimmed(), namespace.yellow());
    println!("  {} {}", "Replicas:   ".dimmed(), args.replicas);
    println!(
        "  {} {} / {}",
        "CPU:        ".dimmed(),
        args.cpu_request,
        args.cpu_limit
    );
    println!(
        "  {} {} / {}",
        "Memory:     ".dimmed(),
        args.memory_request,
        args.memory_limit
    );
    if !env_vars.is_empty() {
        println!("  {} {} vars", "Env Vars:   ".dimmed(), env_vars.len());
    }
    println!();

    // Get cost estimate first
    let estimate = client
        .estimate_cost(CostEstimateRequest {
            name: args.name.clone(),
            namespace: namespace.to_string(),
            replicas: args.replicas,
            cpu_limit: args.cpu_limit.clone(),
            memory_limit: args.memory_limit.clone(),
            region: args.region.clone(),
        })
        .await;

    if let Ok(est) = &estimate {
        println!(
            "  {} ${:.2}/month (${:.2}/day) — confidence: {:.0}%",
            "Est. Cost:  ".dimmed(),
            est.monthly_estimate,
            est.daily_estimate,
            est.confidence * 100.0
        );
        for rec in &est.recommendations {
            print_warning(&format!("Tip: {rec}"));
        }
        println!();
    }

    let req = DeployRequest {
        name: args.name.clone(),
        image: args.image.clone(),
        replicas: args.replicas,
        namespace: namespace.to_string(),
        dry_run: args.dry_run,
        resources: ResourceSpec {
            cpu_request: args.cpu_request,
            cpu_limit: args.cpu_limit,
            memory_request: args.memory_request,
            memory_limit: args.memory_limit,
        },
        env_vars,
        port: args.port,
        ..Default::default()
    };

    let pb = ProgressBar::new_spinner();
    pb.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.green} {msg}")
            .unwrap(),
    );
    pb.set_message("Deploying application...");
    pb.enable_steady_tick(Duration::from_millis(80));

    let response = client.deploy(req).await?;
    pb.finish_and_clear();

    match output {
        OutputFormat::Json => crate::output::print_json(&response),
        OutputFormat::Yaml => crate::output::print_yaml(&response),
        _ => {
            if args.dry_run {
                print_info("Manifest preview:");
                if let Some(manifest) = &response.manifest_preview {
                    println!("{manifest}");
                }
            } else {
                print_success(&format!(
                    "Application '{}' deployed successfully!",
                    args.name.cyan().bold()
                ));
                println!("  {} {}", "Deploy ID:".dimmed(), response.deploy_id.yellow());
                println!(
                    "  {} ${:.2}/month",
                    "Est. Cost:".dimmed(),
                    response.estimated_cost_monthly
                );
                if let Some(url) = &response.app_url {
                    println!("  {} {}", "URL:      ".dimmed(), url.cyan());
                }

                if args.wait {
                    wait_for_ready(&client, namespace, &args.name).await?;
                }
            }
        }
    }

    Ok(())
}

async fn wait_for_ready(client: &ApiClient, namespace: &str, name: &str) -> Result<()> {
    let pb = ProgressBar::new_spinner();
    pb.set_style(
        ProgressStyle::default_spinner()
            .template("{spinner:.blue} {msg}")
            .unwrap(),
    );
    pb.set_message("Waiting for application to be ready...");
    pb.enable_steady_tick(Duration::from_millis(100));

    for _ in 0..60 {
        tokio::time::sleep(Duration::from_secs(2)).await;
        if let Ok(app) = client.get_app(namespace, name).await {
            if app.ready_replicas == app.replicas {
                pb.finish_and_clear();
                print_success("Application is ready!");
                return Ok(());
            }
            pb.set_message(format!(
                "Waiting... ({}/{} replicas ready)",
                app.ready_replicas, app.replicas
            ));
        }
    }

    pb.finish_and_clear();
    print_warning("Timeout waiting for application to be ready");
    Ok(())
}