use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ── Deploy ─────────────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize, Default)]
pub struct DeployRequest {
    pub name: String,
    pub image: String,
    pub replicas: i32,
    pub namespace: String,
    pub dry_run: bool,
    pub resources: ResourceSpec,
    pub env_vars: HashMap<String, String>,
    pub labels: HashMap<String, String>,
    pub annotations: HashMap<String, String>,
    pub port: Option<u16>,
}

#[derive(Debug, Serialize, Deserialize, Default)]
pub struct ResourceSpec {
    pub cpu_request: String,
    pub cpu_limit: String,
    pub memory_request: String,
    pub memory_limit: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct DeployResponse {
    pub deploy_id: String,
    pub status: String,
    pub estimated_cost_monthly: f64,
    pub manifest_preview: Option<String>,
    pub app_url: Option<String>,
}

// ── Application ────────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize)]
pub struct Application {
    pub name: String,
    pub namespace: String,
    pub image: String,
    pub replicas: i32,
    pub ready_replicas: i32,
    pub status: AppStatus,
    pub created_at: String,
    pub updated_at: String,
    pub app_url: Option<String>,
    pub labels: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AppStatus {
    Running,
    Pending,
    Failed,
    Stopped,
    Deploying,
}

impl std::fmt::Display for AppStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        use colored::Colorize;
        match self {
            AppStatus::Running => write!(f, "{}", "Running".green()),
            AppStatus::Pending => write!(f, "{}", "Pending".yellow()),
            AppStatus::Deploying => write!(f, "{}", "Deploying".cyan()),
            AppStatus::Failed => write!(f, "{}", "Failed".red()),
            AppStatus::Stopped => write!(f, "{}", "Stopped".dimmed()),
        }
    }
}

// ── Cost ───────────────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize)]
pub struct CostEstimateRequest {
    pub name: String,
    pub namespace: String,
    pub replicas: i32,
    pub cpu_limit: String,
    pub memory_limit: String,
    pub region: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CostEstimateResponse {
    pub monthly_estimate: f64,
    pub daily_estimate: f64,
    pub breakdown: CostBreakdown,
    pub confidence: f64,
    pub recommendations: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CostBreakdown {
    pub compute: f64,
    pub memory: f64,
    pub networking: f64,
    pub storage: f64,
    pub overhead: f64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CostDataPoint {
    pub date: String,
    pub amount: f64,
    pub currency: String,
    pub service: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CostAnomaly {
    pub detected_at: String,
    pub severity: String,
    pub message: String,
    pub current_value: f64,
    pub expected_value: f64,
    pub z_score: f64,
}

// ── Env Vars ───────────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct EnvVar {
    pub key: String,
    pub value: String,
    pub secret: bool,
}

// ── Auth ───────────────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize)]
pub struct LoginResponse {
    pub token: String,
    pub expires_at: String,
    pub user: UserInfo,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UserInfo {
    pub id: String,
    pub username: String,
    pub email: String,
    pub roles: Vec<String>,
}

// ── Health / Status ────────────────────────────────────────────────────────
#[derive(Debug, Serialize, Deserialize)]
pub struct HealthResponse {
    pub status: String,
    pub version: String,
    pub uptime: u64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PlatformStatus {
    pub control_plane: ComponentStatus,
    pub ml_engine: ComponentStatus,
    pub kubernetes: ComponentStatus,
    pub database: ComponentStatus,
    pub total_apps: u32,
    pub total_namespaces: u32,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ComponentStatus {
    pub healthy: bool,
    pub latency_ms: u64,
    pub message: Option<String>,
}