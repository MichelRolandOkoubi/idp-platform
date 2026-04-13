use anyhow::Result;
use clap::Args;
use futures_util::StreamExt;

#[derive(Args, Debug)]
pub struct LogsArgs {
    /// Application name
    pub name: String,

    /// Follow log output
    #[arg(short, long)]
    pub follow: bool,

    /// Number of lines to show from the end
    #[arg(short, long, default_value = "100")]
    pub tail: u32,

    /// Show logs since duration (e.g., 1h, 30m)
    #[arg(long)]
    pub since: Option<String>,
}

pub async fn run(args: LogsArgs, client: crate::api_client::ApiClient, namespace: &str) -> Result<()> {
    let resp = client
        .get_logs_stream(
            namespace,
            &args.name,
            args.tail,
            args.follow,
            args.since.as_deref(),
        )
        .await?;

    if !resp.status().is_success() {
        anyhow::bail!("Failed to get logs: {}", resp.status());
    }

    let mut stream = resp.bytes_stream();

    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        let text = String::from_utf8_lossy(&chunk);
        print!("{text}");
    }

    Ok(())
}