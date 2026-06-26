"""ragdesk AI service.

Phase 0 exposes liveness, readiness and version probes. Later phases add the
document ingestion pipeline (chunk -> embed -> pgvector) and provider-agnostic
RAG chat.
"""

import logging

import psycopg
from fastapi import FastAPI
from fastapi.responses import JSONResponse

from app.config import settings

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("ragdesk-ai")

VERSION = "dev"

app = FastAPI(title="ragdesk-ai", version="0.1.0")


@app.get("/healthz")
def healthz() -> dict:
    """Liveness probe: the process is running."""
    return {"status": "ok", "service": "ragdesk-ai"}


@app.get("/version")
def version() -> dict:
    """Report build metadata, useful for verifying what is deployed."""
    return {"service": "ragdesk-ai", "version": VERSION}


@app.get("/readyz")
def readyz() -> JSONResponse:
    """Readiness probe: verify downstream dependencies are reachable."""
    checks: dict[str, str] = {}
    status_code = 200

    try:
        with psycopg.connect(settings.database_url, connect_timeout=3) as conn:
            conn.execute("SELECT 1")
        checks["postgres"] = "ok"
    except Exception as exc:
        checks["postgres"] = f"down: {exc}"
        status_code = 503

    state = "ok" if status_code == 200 else "degraded"
    return JSONResponse(
        status_code=status_code,
        content={"status": state, "service": "ragdesk-ai", "checks": checks},
    )
