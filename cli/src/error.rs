use thiserror::Error;

#[derive(Error, Debug)]
pub enum IdpError {
    #[error("Not authenticated. Run: idpctl auth login")]
    NotAuthenticated,

    #[error("Application '{0}' not found")]
    AppNotFound(String),

    #[error("API error ({status}): {message}")]
    ApiError { status: u16, message: String },

    #[error("Connection refused. Is the control plane running?")]
    ConnectionRefused,

    #[error("Invalid response from server: {0}")]
    InvalidResponse(String),

    #[error("Config error: {0}")]
    ConfigError(String),
}