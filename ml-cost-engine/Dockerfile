# ── Stage 1: Base ──────────────────────────────────────────────────────────────
FROM python:3.11-slim AS base

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# ── Stage 2: Dependencies ──────────────────────────────────────────────────────
FROM base AS deps

COPY pyproject.toml ./
RUN pip install hatchling && pip install -e ".[dev]"

# ── Stage 3: Development ───────────────────────────────────────────────────────
FROM deps AS development

COPY . .
EXPOSE 8000
CMD ["uvicorn", "src.api.main:app", "--host", "0.0.0.0", "--port", "8000", "--reload"]

# ── Stage 4: Production ────────────────────────────────────────────────────────
FROM base AS production

COPY pyproject.toml ./
RUN pip install hatchling && pip install -e "."

COPY src/ ./src/
COPY models/ ./models/

RUN adduser --disabled-password --gecos "" appuser
USER appuser

EXPOSE 8000
CMD ["uvicorn", "src.api.main:app", "--host", "0.0.0.0", "--port", "8000", \
    "--workers", "4", "--log-level", "info"]