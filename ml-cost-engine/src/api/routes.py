from fastapi import APIRouter, Request, HTTPException
from pydantic import BaseModel, Field
import structlog

logger = structlog.get_logger(__name__)

router = APIRouter(tags=["cost"])
health_router = APIRouter(tags=["health"])


# ── Schemas ────────────────────────────────────────────────────────────────────

class DeploySpec(BaseModel):
    name: str
    namespace: str
    replicas: int = Field(ge=1, le=100)
    cpu_limit: str = "500m"
    memory_limit: str = "512Mi"
    region: str = "eu-west-1"


class CostBreakdown(BaseModel):
    compute: float
    memory: float
    networking: float
    storage: float
    overhead: float


class CostPrediction(BaseModel):
    monthly_estimate: float
    daily_estimate: float
    breakdown: CostBreakdown
    confidence: float
    recommendations: list[str]


class AnomalyResult(BaseModel):
    detected_at: str
    severity: str
    message: str
    current_value: float
    expected_value: float
    z_score: float


# ── Routes ─────────────────────────────────────────────────────────────────────

@router.post("/predict/cost", response_model=CostPrediction)
async def predict_cost(spec: DeploySpec, request: Request) -> CostPrediction:
    """Predict monthly cost for a deployment spec."""
    predictor = request.app.state.predictor
    try:
        result = await predictor.predict(spec)
        logger.info("cost_predicted", app=spec.name, monthly=result.monthly_estimate)
        return result
    except Exception as e:
        logger.error("prediction_error", error=str(e))
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/anomalies/{namespace}", response_model=list[AnomalyResult])
async def detect_anomalies(
    namespace: str,
    hours: int = 24,
    request: Request = None,
) -> list[AnomalyResult]:
    """Detect cost anomalies in a namespace."""
    detector = request.app.state.anomaly_detector
    try:
        return await detector.detect(namespace, hours)
    except Exception as e:
        logger.error("anomaly_detection_error", error=str(e))
        raise HTTPException(status_code=500, detail=str(e))


@health_router.get("/health")
async def health() -> dict:
    return {"status": "healthy", "service": "ml-cost-engine", "version": "0.1.0"}