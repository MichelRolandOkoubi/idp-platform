use anyhow::{Context, Result};
use reqwest::{Client, StatusCode, header};
use serde::{Deserialize, Serialize};
use crate::error::IdpError;

pub mod types;
pub use types::*;

#[derive(Clone)]
pub struct ApiClient {
    client: Client,
    base_url: String,
}

impl ApiClient {
    pub fn new(base_url: String, token: Option<String>) -> Result<Self> {
        let mut headers = header::HeaderMap::new();
        headers.insert(header::CONTENT_TYPE, "application/json".parse()?);
        headers.insert(header::ACCEPT, "application/json".parse()?);

        if let Some(t) = token {
            headers.insert(
                header::AUTHORIZATION,
                format!("Bearer {t}").parse()?,
            );
        }

        let client = Client::builder()
            .default_headers(headers)
            .timeout(std::time::Duration::from_secs(30))
            .user_agent(format!("idpctl/{}", env!("CARGO_PKG_VERSION")))
            .build()
            .context("Failed to build HTTP client")?;

        Ok(Self { client, base_url })
    }

    fn url(&self, path: &str) -> String {
        format!("{}/api/v1{}", self.base_url, path)
    }

    async fn handle_response<T: for<'de> Deserialize<'de>>(
        &self,
        resp: reqwest::Response,
    ) -> Result<T> {
        let status = resp.status();
        let body = resp.text().await?;

        if status.is_success() {
            serde_json::from_str(&body)
                .with_context(|| format!("Failed to parse response: {body}"))
        } else {
            let msg = serde_json::from_str::<serde_json::Value>(&body)
                .ok()
                .and_then(|v| v["error"].as_str().map(String::from))
                .unwrap_or(body);

            Err(IdpError::ApiError {
                status: status.as_u16(),
                message: msg,
            }
            .into())
        }
    }

    // ── Deploy ─────────────────────────────────────────────────────
    pub async fn deploy(&self, req: DeployRequest) -> Result<DeployResponse> {
        let resp = self
            .client
            .post(self.url("/deploy"))
            .json(&req)
            .send()
            .await
            .map_err(|e| {
                if e.is_connect() {
                    IdpError::ConnectionRefused.into()
                } else {
                    anyhow::anyhow!(e)
                }
            })?;
        self.handle_response(resp).await
    }

    // ── Applications ───────────────────────────────────────────────
    pub async fn list_apps(&self, namespace: &str) -> Result<Vec<Application>> {
        let resp = self
            .client
            .get(self.url("/applications"))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        self.handle_response::<Vec<Application>>(resp).await
    }

    pub async fn get_app(&self, namespace: &str, name: &str) -> Result<Application> {
        let resp = self
            .client
            .get(self.url(&format!("/applications/{name}")))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        self.handle_response(resp).await
    }

    pub async fn delete_app(&self, namespace: &str, name: &str) -> Result<()> {
        let resp = self
            .client
            .delete(self.url(&format!("/applications/{name}")))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        if resp.status().is_success() {
            Ok(())
        } else {
            self.handle_response(resp).await
        }
    }

    pub async fn scale_app(
        &self,
        namespace: &str,
        name: &str,
        replicas: i32,
    ) -> Result<Application> {
        let body = serde_json::json!({ "replicas": replicas });
        let resp = self
            .client
            .post(self.url(&format!("/applications/{name}/scale")))
            .query(&[("namespace", namespace)])
            .json(&body)
            .send()
            .await?;
        self.handle_response(resp).await
    }

    // ── Logs ───────────────────────────────────────────────────────
    pub async fn get_logs_stream(
        &self,
        namespace: &str,
        name: &str,
        tail: u32,
        follow: bool,
        since: Option<&str>,
    ) -> Result<reqwest::Response> {
        let mut query = vec![
            ("namespace", namespace.to_string()),
            ("tail", tail.to_string()),
            ("follow", follow.to_string()),
        ];
        if let Some(s) = since {
            query.push(("since", s.to_string()));
        }
        let resp = self
            .client
            .get(self.url(&format!("/applications/{name}/logs")))
            .query(&query)
            .send()
            .await?;
        Ok(resp)
    }

    // ── Cost ───────────────────────────────────────────────────────
    pub async fn estimate_cost(&self, req: CostEstimateRequest) -> Result<CostEstimateResponse> {
        let resp = self
            .client
            .post(self.url("/cost/estimate"))
            .json(&req)
            .send()
            .await?;
        self.handle_response(resp).await
    }

    pub async fn get_cost_history(
        &self,
        namespace: &str,
        days: u32,
    ) -> Result<Vec<CostDataPoint>> {
        let resp = self
            .client
            .get(self.url("/cost/history"))
            .query(&[("namespace", namespace), ("days", &days.to_string())])
            .send()
            .await?;
        self.handle_response(resp).await
    }

    pub async fn get_anomalies(&self, namespace: &str) -> Result<Vec<CostAnomaly>> {
        let resp = self
            .client
            .get(self.url("/cost/anomalies"))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        self.handle_response(resp).await
    }

    // ── Env Vars ───────────────────────────────────────────────────
    pub async fn list_env(&self, namespace: &str, app: &str) -> Result<Vec<EnvVar>> {
        let resp = self
            .client
            .get(self.url(&format!("/applications/{app}/env")))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        self.handle_response(resp).await
    }

    pub async fn set_env(
        &self,
        namespace: &str,
        app: &str,
        vars: Vec<EnvVar>,
    ) -> Result<()> {
        let resp = self
            .client
            .put(self.url(&format!("/applications/{app}/env")))
            .query(&[("namespace", namespace)])
            .json(&vars)
            .send()
            .await?;
        if resp.status().is_success() {
            Ok(())
        } else {
            self.handle_response(resp).await
        }
    }

    pub async fn delete_env(&self, namespace: &str, app: &str, key: &str) -> Result<()> {
        let resp = self
            .client
            .delete(self.url(&format!("/applications/{app}/env/{key}")))
            .query(&[("namespace", namespace)])
            .send()
            .await?;
        if resp.status().is_success() {
            Ok(())
        } else {
            self.handle_response(resp).await
        }
    }

    // ── Auth ───────────────────────────────────────────────────────
    pub async fn login(&self, username: &str, password: &str) -> Result<LoginResponse> {
        let body = serde_json::json!({ "username": username, "password": password });
        let resp = self
            .client
            .post(self.url("/auth/login"))
            .json(&body)
            .send()
            .await?;
        self.handle_response(resp).await
    }

    // ── Status ─────────────────────────────────────────────────────
    pub async fn health(&self) -> Result<HealthResponse> {
        let resp = self.client.get(self.url("/health")).send().await?;
        self.handle_response(resp).await
    }

    pub async fn platform_status(&self) -> Result<PlatformStatus> {
        let resp = self
            .client
            .get(self.url("/status"))
            .send()
            .await?;
        self.handle_response(resp).await
    }
}