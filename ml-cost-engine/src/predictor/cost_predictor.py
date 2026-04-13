from __future__ import annotations

import asyncio
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

import joblib
import numpy as np
import structlog
from sklearn.ensemble import GradientBoostingRegressor
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

if TYPE_CHECKING:
    from ..api.routes import DeploySpec, CostPrediction

logger = structlog.get_logger(__name__)

REGION_MULTIPLIERS: dict[str, float] = {
    "us-east-1": 1.00,
    "us-west-2": 1.02,
    "eu-west-1": 1.08,
    "eu-central-1": 1.10,
    "ap-southeast-1": 1.15,
    "ap-northeast-1": 1.18,
}

# $/hour
CPU_COST_PER_CORE = 0.048
MEMORY_COST_PER_GB = 0.006
HOURS_PER_MONTH = 730
OVERHEAD_FACTOR = 1.25
NETWORKING_RATIO = 0.08
STORAGE_RATIO = 0.05


class CostPredictor:
    def __init__(self, model_dir: Path = Path("models")) -> None:
        self.model_dir = model_dir
        self.model_version = "1.0.0"
        self._model: Pipeline | None = None
        self._load_model()

    def _load_model(self) -> None:
        model_path = self.model_dir / "cost_model.pkl"
        if model_path.exists():
            try:
                self._model = joblib.load(model_path)
                logger.info("model_loaded", path=str(model_path))
            except Exception as e:
                logger.warning("model_load_failed", error=str(e))
        else:
            logger.info("model_not_found_using_heuristic")

    def _parse_cpu_to_cores(self, cpu: str) -> float:
        """Convert k8s CPU notation to cores."""
        if cpu.endswith("m"):
            return float(cpu[:-1]) / 1000.0
        return float(cpu)

    def _parse_memory_to_gb(self, memory: str) -> float:
        """Convert k8s memory notation to GB."""
        multipliers = {
            "Ki": 1 / (1024 * 1024),
            "Mi": 1 / 1024,
            "Gi": 1.0,
            "Ti": 1024.0,
            "K": 1 / (1000 * 1000),
            "M": 1 / 1000,
            "G": 1.0,
            "T": 1000.0,
        }
        for suffix, multiplier in multipliers.items():
            if memory.endswith(suffix):
                return float(memory[: -len(suffix)]) * multiplier
        return float(memory) / (1024**3)

    def _heuristic_estimate(self, spec: "DeploySpec") -> dict:
        cpu_cores = self._parse_cpu_to_cores(spec.cpu_limit) * spec.replicas
        memory_gb = self._parse_memory_to_gb(spec.memory_limit) * spec.replicas
        region_mult = REGION_MULTIPLIERS.get(spec.region, 1.0)

        cpu_monthly = cpu_cores * CPU_COST_PER_CORE * HOURS_PER_MONTH * region_mult
        mem_monthly = memory_gb * MEMORY_COST_PER_GB * HOURS_PER_MONTH * region_mult
        base_monthly = cpu_monthly + mem_monthly

        networking = base_monthly * NETWORKING_RATIO
        storage = base_monthly * STORAGE_RATIO
        overhead = base_monthly * (OVERHEAD_FACTOR - 1.0)
        total = base_monthly + networking + storage + overhead

        return {
            "total": total,
            "compute": cpu_monthly,
            "memory": mem_monthly,
            "networking": networking,
            "storage": storage,
            "overhead": overhead,
            "confidence": 0.70,
        }

    def _ml_estimate(self, spec: "DeploySpec") -> dict | None:
        if self._model is None:
            return None
        try:
            cpu_cores = self._parse_cpu_to_cores(spec.cpu_limit) * spec.replicas
            memory_gb = self._parse_memory_to_gb(spec.memory_limit) * spec.replicas
            region_mult = REGION_MULTIPLIERS.get(spec.region, 1.0)

            features = np.array([[cpu_cores, memory_gb, spec.replicas, region_mult]])
            total = float(self._model.predict(features)[0])

            h = self._heuristic_estimate(spec)
            ratio = total / max(h["total"], 0.01)

            return {
                "total": total,
                "compute": h["compute"] * ratio,
                "memory": h["memory"] * ratio,
                "networking": h["networking"] * ratio,
                "storage": h["storage"] * ratio,
                "overhead": h["overhead"] * ratio,
                "confidence": 0.88,
            }
        except Exception as e:
            logger.warning("ml_estimate_failed", error=str(e))
            return None

    def _generate_recommendations(
        self, spec: "DeploySpec", cpu_cores: float, memory_gb: float
    ) -> list[str]:
        recs = []

        cpu_req_cores = self._parse_cpu_to_cores(
            spec.cpu_limit.replace("500m", "100m")  # rough estimate
        )
        cpu_limit_cores = self._parse_cpu_to_cores(spec.cpu_limit)

        if cpu_limit_cores / max(cpu_req_cores, 0.001) > 5:
            recs.append(
                f"CPU limit ({spec.cpu_limit}) is much higher than typical request. "
                "Consider reducing to improve bin-packing."
            )

        if spec.replicas > 5:
            recs.append(
                "Consider using HorizontalPodAutoscaler instead of fixed replicas "
                "to reduce costs by up to 40% during low-traffic periods."
            )

        if memory_gb / spec.replicas > 4:
            recs.append(
                "High per-replica memory. Consider memory-optimized instance class "
                "or reviewing your application memory usage."
            )

        if spec.region in ("ap-northeast-1", "ap-southeast-1"):
            recs.append(
                f"Region {spec.region} has higher pricing. "
                "Consider eu-west-1 or us-east-1 if latency allows."
            )

        return recs

    async def predict(self, spec: "DeploySpec") -> "CostPrediction":
        from ..api.routes import CostBreakdown, CostPrediction

        loop = asyncio.get_event_loop()
        estimate = await loop.run_in_executor(
            None,
            lambda: self._ml_estimate(spec) or self._heuristic_estimate(spec),
        )

        cpu_cores = self._parse_cpu_to_cores(spec.cpu_limit) * spec.replicas
        memory_gb = self._parse_memory_to_gb(spec.memory_limit) * spec.replicas
        recommendations = self._generate_recommendations(spec, cpu_cores, memory_gb)

        monthly = round(estimate["total"], 4)

        return CostPrediction(
            monthly_estimate=monthly,
            daily_estimate=round(monthly / 30, 4),
            breakdown=CostBreakdown(
                compute=round(estimate["compute"], 4),
                memory=round(estimate["memory"], 4),
                networking=round(estimate["networking"], 4),
                storage=round(estimate["storage"], 4),
                overhead=round(estimate["overhead"], 4),
            ),
            confidence=estimate["confidence"],
            recommendations=recommendations,
        )

    def train(self, X: np.ndarray, y: np.ndarray) -> None:
        """Train the cost prediction model."""
        pipeline = Pipeline([
            ("scaler", StandardScaler()),
            ("model", GradientBoostingRegressor(
                n_estimators=200,
                learning_rate=0.05,
                max_depth=4,
                subsample=0.8,
                random_state=42,
            )),
        ])
        pipeline.fit(X, y)
        self._model = pipeline

        self.model_dir.mkdir(parents=True, exist_ok=True)
        joblib.dump(pipeline, self.model_dir / "cost_model.pkl")
        logger.info("model_trained_and_saved")