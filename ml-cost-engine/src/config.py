from pydantic_settings import BaseSettings
from pathlib import Path


class Settings(BaseSettings):
    app_env: str = "development"
    log_level: str = "info"
    host: str = "0.0.0.0"
    port: int = 8000

    db_url: str = "postgresql://idp:idppassword@localhost:5433/idp_metrics"
    redis_url: str = "redis://localhost:6379"

    model_path: Path = Path("/app/models")
    otel_endpoint: str = "http://localhost:4317"

    # Pricing (USD/hour)
    cpu_cost_per_core_hour: float = 0.048
    memory_cost_per_gb_hour: float = 0.006

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"