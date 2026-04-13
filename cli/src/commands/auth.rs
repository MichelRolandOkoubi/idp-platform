use anyhow::Result;
use clap::{Args, Subcommand};
use colored::Colorize;

use crate::config::Config;
use crate::output::{print_success};

#[derive(Args, Debug)]
pub struct AuthArgs {
    #[command(subcommand)]
    pub action: AuthAction,
}

#[derive(Subcommand, Debug)]
pub enum AuthAction {
    /// Login to the IDP
    Login {
        #[arg(long)]
        url: Option<String>,
        #[arg(short, long)]
        username: Option<String>,
    },
    /// Show current login status
    Status,
    /// Logout
    Logout,
}

pub async fn run(args: AuthArgs, mut cfg: Config) -> Result<()> {
    match args.action {
        AuthAction::Login { url, username } => {
            let api_url = url
                .or(cfg.api_url.clone())
                .unwrap_or_else(|| "http://localhost:8080".to_string());

            println!("{}", "🔐 IDP Login".cyan().bold());
            println!("  API URL: {}", api_url.yellow());
            println!();

            let username = match username {
                Some(u) => u,
                None => dialoguer::Input::<String>::new()
                    .with_prompt("Username")
                    .interact()?,
            };

            let password = dialoguer::Password::new()
                .with_prompt("Password")
                .interact()?;

            // Build temp client without token for login
            let client = crate::api_client::ApiClient::new(api_url.clone(), None)?;
            let resp = client.login(&username, &password).await?;

            cfg.set_api_url(api_url)?;
            cfg.set_token(resp.token)?;

            print_success(&format!(
                "Logged in as {} (expires: {})",
                resp.user.username.cyan(),
                resp.expires_at
            ));
        }

        AuthAction::Status => {
            if cfg.token.is_some() {
                println!("{} Authenticated", "✓".green().bold());
                println!("  API URL: {}", cfg.api_url.as_deref().unwrap_or("not set").yellow());
            } else {
                println!("{} Not authenticated. Run: idpctl auth login", "✗".red().bold());
            }
        }

        AuthAction::Logout => {
            cfg.token = None;
            cfg.save()?;
            print_success("Logged out successfully");
        }
    }
    Ok(())
}