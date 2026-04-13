use anyhow::Result;
use clap::{Args, Subcommand};

use crate::api_client::{ApiClient, EnvVar};
use crate::output::{print_success, print_table, OutputFormat};

#[derive(Args, Debug)]
pub struct EnvArgs {
    #[command(subcommand)]
    pub action: EnvAction,
}

#[derive(Subcommand, Debug)]
pub enum EnvAction {
    /// List env vars for an app
    #[command(alias = "ls")]
    List { app: String },
    /// Set env var(s) for an app
    Set {
        app: String,
        /// KEY=VALUE pairs
        #[arg(required = true)]
        vars: Vec<String>,
        /// Mark as secret (value hidden in UI)
        #[arg(long)]
        secret: bool,
    },
    /// Delete an env var
    Delete { app: String, key: String },
}

pub async fn run(
    args: EnvArgs,
    client: ApiClient,
    namespace: &str,
    output: &OutputFormat,
) -> Result<()> {
    match args.action {
        EnvAction::List { app } => {
            let vars = client.list_env(namespace, &app).await?;
            if vars.is_empty() {
                println!("No environment variables set for '{app}'");
                return Ok(());
            }
            print_table(
                vec!["KEY", "VALUE", "SECRET"],
                vars.iter()
                    .map(|v| {
                        vec![
                            v.key.clone(),
                            if v.secret {
                                "***".to_string()
                            } else {
                                v.value.clone()
                            },
                            v.secret.to_string(),
                        ]
                    })
                    .collect(),
            );
        }

        EnvAction::Set { app, vars, secret } => {
            let env_vars: Vec<EnvVar> = vars
                .iter()
                .filter_map(|kv| {
                    let mut parts = kv.splitn(2, '=');
                    Some(EnvVar {
                        key: parts.next()?.to_string(),
                        value: parts.next()?.to_string(),
                        secret,
                    })
                })
                .collect();

            client.set_env(namespace, &app, env_vars.clone()).await?;
            print_success(&format!("Set {} env var(s) for '{app}'", env_vars.len()));
        }

        EnvAction::Delete { app, key } => {
            client.delete_env(namespace, &app, &key).await?;
            print_success(&format!("Deleted env var '{key}' from '{app}'"));
        }
    }
    Ok(())
}