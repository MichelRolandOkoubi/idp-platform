from contextlib import asynccontextmanager

import structlog
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor

from ..config import Settings
from ..predictor.cost_predictor import CostPredictor
from ..anomaly.detector import AnomalyDetector
from . import routes

logger = structlog.get_logger(__name__)
settings = Settings()

predictor: CostPredictor
anomaly_detector: AnomalyDetector


@asynccontextmanager
async def lifespan(app: FastAPI):
    global predictor, anomaly_detector
    logger.info("startup", service="ml-cost-engine")

    predictor = CostPredictor(model_dir=settings.model_path)
    anomaly_detector = AnomalyDetector()

    app.state.predictor = predictor
    app.state.anomaly_detector = anomaly_detector

    yield

    logger.info("shutdown", service="ml-cost-engine")


def create_app() -> FastAPI:
    app = FastAPI(
        title="IDP ML Cost Engine",
        description="Cost prediction and anomaly detection",
        version="0.1.0",
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(routes.router, prefix="/api/v1")
    app.include_router(routes.health_router)

    FastAPIInstrumentor.instrument_app(app)

    return app


app = create_app()