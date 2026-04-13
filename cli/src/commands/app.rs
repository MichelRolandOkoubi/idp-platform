use anyhow::Result;
use clap::{Args, Subcommand};
use colored::Colorize;

use crate::api_client::ApiClient;
use crate::output::{print_json, print_success, print_table, print_yaml, OutputFormat};

#[derive(Args, Debug)]
pub struct AppArgs {
    #[command(subcommand)]
    pub action: AppAction,
}

#[derive(Subcommand, Debug)]
pub enum AppAction {
    /// List all applications
    #[command(alias = "ls")]
    List {
        /// Show all namespaces
        #[arg(short = 'A', long)]
        all_namespaces: bool,
    },
    /// Get application details
    Get { name: String },
    /// Delete an application
    #[command(alias = "rm")]
    Delete {
        name: String,
        #[arg(short, long)]
        force: bool,
    },
    /// Scale an application
    Scale {
        name: String,
        #[arg(short, long)]
        replicas: i32,
    },
    /// Restart an application
    Restart { name: String },
}

pub async fn run(
    args: AppArgs,
    client: ApiClient,
    namespace: &str,
    output: &OutputFormat,
) -> Result<()> {
    match args.action {
        AppAction::List { all_namespaces } => {
            let ns = if all_namespaces { "" } else { namespace };
            let apps = client.list_apps(ns).await?;

            match output {
                OutputFormat::Json => print_json(&apps),
                OutputFormat::Yaml => print_yaml(&apps),
                _ => {
                    if apps.is_empty() {
                        println!("No applications found in namespace '{namespace}'");
                        return Ok(());
                    }
                    print_table(
                        vec!["NAME", "NAMESPACE", "IMAGE", "REPLICAS", "STATUS", "URL"],
                        apps.iter()
                            .map(|app| {
                                vec![
                                    app.name.clone(),
                                    app.namespace.clone(),
                                    app.image.clone(),
                                    format!("{}/{}", app.ready_replicas, app.replicas),
                                    app.status.to_string(),
                                    app.app_url.clone().unwrap_or_else(|| "-".to_string()),
                                ]
                            })
                            .collect(),
                    );
                }
            }
        }

        AppAction::Get { name } => {
            let app = client.get_app(namespace, &name).await?;

            match output {
                OutputFormat::Json => print_json(&app),
                OutputFormat::Yaml => print_yaml(&app),
                _ => {
                    println!("{}", "Application Details".cyan().bold());
                    println!("  {} {}", "Name:     ".dimmed(), app.name.cyan());
                    println!("  {} {}", "Namespace:".dimmed(), app.namespace);
                    println!("  {} {}", "Image:    ".dimmed(), app.image.yellow());
                    println!(
                        "  {} {}/{}",
                        "Replicas: ".dimmed(),
                        app.ready_replicas,
                        app.replicas
                    );
                    println!("  {} {}", "Status:   ".dimmed(), app.status);
                    println!("  {} {}", "Created:  ".dimmed(), app.created_at);
                    println!("  {} {}", "Updated:  ".dimmed(), app.updated_at);
                    if let Some(url) = app.app_url {
                        println!("  {} {}", "URL:      ".dimmed(), url.cyan());
                    }
                }
            }
        }

        AppAction::Delete { name, force } => {
            if !force {
                let confirm = dialoguer::Confirm::new()
                    .with_prompt(format!(
                        "Delete application '{}'? This cannot be undone.",
                        name.red()
                    ))
                    .default(false)
                    .interact()?;
                if !confirm {
                    println!("Aborted.");
                    return Ok(());
                }
            }
            client.delete_app(namespace, &name).await?;
            print_success(&format!("Application '{}' deleted", name));
        }

        AppAction::Scale { name, replicas } => {
            let app = client.scale_app(namespace, &name, replicas).await?;
            print_success(&format!(
                "Application '{}' scaled to {} replicas",
                app.name, replicas
            ));
        }

        AppAction::Restart { name } => {
            println!("Restarting '{name}'...");
            // Scale down to 0 then back to original
            let app = client.get_app(namespace, &name).await?;
            let original_replicas = app.replicas;
            client.scale_app(namespace, &name, 0).await?;
            tokio::time::sleep(std::time::Duration::from_secs(2)).await;
            client.scale_app(namespace, &name, original_replicas).await?;
            print_success(&format!("Application '{name}' restarted"));
        }
    }

    Ok(())
}