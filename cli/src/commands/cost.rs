use anyhow::Result;
use clap::{Args, Subcommand};
use colored::Colorize;

use crate::api_client::{ApiClient, CostEstimateRequest};
use crate::output::{print_json, print_table, print_yaml, OutputFormat};

#[derive(Args, Debug)]
pub struct CostArgs {
    #[command(subcommand)]
    pub action: CostAction,
}

#[derive(Subcommand, Debug)]
pub enum CostAction {
    /// Estimate cost for an app configuration
    Estimate {
        #[arg(short, long)]
        name: String,
        #[arg(short, long, default_value = "1")]
        replicas: i32,
        #[arg(long, default_value = "500m")]
        cpu_limit: String,
        #[arg(long, default_value = "512Mi")]
        memory_limit: String,
        #[arg(long, default_value = "eu-west-1")]
        region: String,
    },
    /// Show cost history
    History {
        #[arg(long, default_value = "30")]
        days: u32,
    },
    /// Show cost anomalies
    Anomalies,
}

pub async fn run(
    args: CostArgs,
    client: ApiClient,
    namespace: &str,
    output: &OutputFormat,
) -> Result<()> {
    match args.action {
        CostAction::Estimate {
            name,
            replicas,
            cpu_limit,
            memory_limit,
            region,
        } => {
            let est = client
                .estimate_cost(CostEstimateRequest {
                    name: name.clone(),
                    namespace: namespace.to_string(),
                    replicas,
                    cpu_limit: cpu_limit.clone(),
                    memory_limit: memory_limit.clone(),
                    region: region.clone(),
                })
                .await?;

            match output {
                OutputFormat::Json => print_json(&est),
                OutputFormat::Yaml => print_yaml(&est),
                _ => {
                    println!();
                    println!("{}", "💰 Cost Estimation".cyan().bold());
                    println!();
                    println!(
                        "  {} {}",
                        "Application:".dimmed(),
                        name.cyan()
                    );
                    println!("  {} {} replicas", "Replicas:   ".dimmed(), replicas);
                    println!("  {} {}", "CPU Limit:  ".dimmed(), cpu_limit);
                    println!("  {} {}", "Mem Limit:  ".dimmed(), memory_limit);
                    println!("  {} {}", "Region:     ".dimmed(), region);
                    println!();
                    println!(
                        "  {} {}",
                        "Monthly:    ".dimmed(),
                        format!("${:.2}", est.monthly_estimate).green().bold()
                    );
                    println!(
                        "  {} {}",
                        "Daily:      ".dimmed(),
                        format!("${:.2}", est.daily_estimate).green()
                    );
                    println!("  {} {:.0}%", "Confidence: ".dimmed(), est.confidence * 100.0);
                    println!();
                    println!("{}", "Breakdown:".cyan());
                    println!("  Compute:    ${:.2}", est.breakdown.compute);
                    println!("  Memory:     ${:.2}", est.breakdown.memory);
                    println!("  Networking: ${:.2}", est.breakdown.networking);
                    println!("  Storage:    ${:.2}", est.breakdown.storage);
                    println!("  Overhead:   ${:.2}", est.breakdown.overhead);

                    if !est.recommendations.is_empty() {
                        println!();
                        println!("{}", "💡 Recommendations:".yellow().bold());
                        for rec in &est.recommendations {
                            println!("  • {rec}");
                        }
                    }
                    println!();
                }
            }
        }

        CostAction::History { days } => {
            let history = client.get_cost_history(namespace, days).await?;
            match output {
                OutputFormat::Json => print_json(&history),
                _ => {
                    print_table(
                        vec!["DATE", "AMOUNT", "CURRENCY"],
                        history
                            .iter()
                            .map(|p| {
                                vec![
                                    p.date.clone(),
                                    format!("{:.4}", p.amount),
                                    p.currency.clone(),
                                ]
                            })
                            .collect(),
                    );
                }
            }
        }

        CostAction::Anomalies => {
            let anomalies = client.get_anomalies(namespace).await?;
            if anomalies.is_empty() {
                println!("{} No anomalies detected", "✓".green().bold());
                return Ok(());
            }
            print_table(
                vec!["DETECTED AT", "SEVERITY", "CURRENT", "EXPECTED", "Z-SCORE", "MESSAGE"],
                anomalies
                    .iter()
                    .map(|a| {
                        vec![
                            a.detected_at.clone(),
                            a.severity.clone(),
                            format!("${:.2}", a.current_value),
                            format!("${:.2}", a.expected_value),
                            format!("{:.2}", a.z_score),
                            a.message.clone(),
                        ]
                    })
                    .collect(),
            );
        }
    }
    Ok(())
}