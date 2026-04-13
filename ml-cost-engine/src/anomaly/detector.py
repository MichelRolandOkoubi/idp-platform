from __future__ import annotations

import asyncio
import statistics
from datetime import datetime, timezone
from typing import TYPE_CHECKING

import structlog

if TYPE_CHECKING:
    from ..api.routes import AnomalyResult

logger = structlog.get_logger(__name__)

SPIKE_THRESHOLD_Z = 2.5
CRITICAL_THRESHOLD_Z = 3.5


class AnomalyDetector:
    def __init__(self) -> None:
        # TODO: Inject TimescaleDB repository for real historical data
        # Reference: docs/adr/003-cost-engine-persistence.md
        # Simulated historical data per namespace for now
        self._baseline: dict[str, list[float]] = {}

    def _z_score(self, values: list[float], latest: float) -> float:
        if len(values) < 3:
            return 0.0
        mean = statistics.mean(values)
        stdev = statistics.stdev(values)
        if stdev == 0:
            return 0.0
        return (latest - mean) / stdev

    async def detect(self, namespace: str, hours: int = 24) -> list["AnomalyResult"]:
        from ..api.routes import AnomalyResult

        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(
            None, self._detect_sync, namespace, hours
        )

    def _detect_sync(self, namespace: str, hours: int) -> list["AnomalyResult"]:
        from ..api.routes import AnomalyResult

        # In production: fetch metrics from TimescaleDB hypertable 'container_costs'
        # Query: SELECT cost FROM container_costs WHERE namespace = %s AND time > now() - interval '%s hours'
        # For now, we use the simulated memory-based baseline
        historical = self._baseline.get(namespace, [10.0, 11.0, 10.5, 12.0, 10.8])
        if not historical:
            return []

        latest = historical[-1]
        baseline = historical[:-1]

        if len(baseline) < 3:
            return []

        mean = statistics.mean(baseline)
        z = self._z_score(baseline, latest)

        anomalies = []
        if abs(z) > SPIKE_THRESHOLD_Z:
            severity = "CRITICAL" if abs(z) > CRITICAL_THRESHOLD_Z else "WARNING"
            anomalies.append(
                AnomalyResult(
                    detected_at=datetime.now(timezone.utc).isoformat(),
                    severity=severity,
                    message=(
                        f"Cost spike detected: ${latest:.2f} "
                        f"({abs(z):.1f}σ above average of ${mean:.2f})"
                    ),
                    current_value=latest,
                    expected_value=round(mean, 4),
                    z_score=round(z, 4),
                )
            )

        return anomalies

    def update_baseline(self, namespace: str, value: float) -> None:
        if namespace not in self._baseline:
            self._baseline[namespace] = []
        self._baseline[namespace].append(value)
        # Keep only last 100 data points
        self._baseline[namespace] = self._baseline[namespace][-100:]