use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Debug, Serialize, Deserialize, Default, Clone)]
pub struct Config {
    pub api_url: Option<String>,
    pub token: Option<String>,
    pub default_namespace: Option<String>,
    pub default_output: Option<String>,
}

impl Config {
    pub fn config_path() -> Result<PathBuf> {
        let home = dirs::home_dir().context("Cannot find home directory")?;
        Ok(home.join(".idpctl").join("config.yaml"))
    }

    pub fn load() -> Result<Self> {
        let path = Self::config_path()?;
        if !path.exists() {
            return Ok(Self::default());
        }
        let content =
            std::fs::read_to_string(&path).context("Failed to read config file")?;
        let cfg: Config = serde_yaml::from_str(&content).context("Failed to parse config")?;
        Ok(cfg)
    }

    pub fn save(&self) -> Result<()> {
        let path = Self::config_path()?;
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let content = serde_yaml::to_string(self)?;
        std::fs::write(&path, content)?;
        Ok(())
    }

    pub fn set_token(&mut self, token: String) -> Result<()> {
        self.token = Some(token);
        self.save()
    }

    pub fn set_api_url(&mut self, url: String) -> Result<()> {
        self.api_url = Some(url);
        self.save()
    }
}