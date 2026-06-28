"""ragdesk AI service.

Probes plus the document ingestion pipeline (chunk -> embed -> pgvector).
RAG chat arrives in Phase 3.
"""

import json
import logging
from collections.abc import Iterator

import psycopg
from fastapi import FastAPI, Header, HTTPException
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel

from app.chat import build_prompt, get_chat_provider
from app.config import settings
from app.ingest import ingest_document
from app.retrieval import retrieve

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


class IngestRequest(BaseModel):
    document_id: str
    workspace_id: str
    text: str


class IngestResponse(BaseModel):
    chunk_count: int


@app.post("/ingest", response_model=IngestResponse)
def ingest(
    req: IngestRequest,
    x_internal_token: str | None = Header(default=None),
) -> IngestResponse:
    """Chunk, embed and store a document's text. Idempotent per document."""
    if settings.internal_token and x_internal_token != settings.internal_token:
        raise HTTPException(status_code=401, detail="unauthorized")
    try:
        count = ingest_document(req.document_id, req.workspace_id, req.text)
    except Exception as exc:
        logger.exception("ingest failed for document %s", req.document_id)
        raise HTTPException(status_code=500, detail="ingestion failed") from exc
    return IngestResponse(chunk_count=count)


class ChatRequest(BaseModel):
    workspace_id: str
    question: str


@app.post("/chat")
def chat(
    req: ChatRequest,
    x_internal_token: str | None = Header(default=None),
) -> StreamingResponse:
    """Retrieve relevant chunks and stream a grounded answer (NDJSON)."""
    if settings.internal_token and x_internal_token != settings.internal_token:
        raise HTTPException(status_code=401, detail="unauthorized")

    sources = retrieve(
        req.workspace_id, req.question, settings.retrieval_k, settings.retrieval_max_distance
    )
    prompt = build_prompt(sources, req.question)

    def generate() -> Iterator[str]:
        seen: set[str] = set()
        cited: list[dict] = []
        for src in sources:
            if src["document_id"] in seen:
                continue
            seen.add(src["document_id"])
            cited.append(
                {
                    "document_id": src["document_id"],
                    "title": src["title"],
                    "snippet": src["content"][:200],
                }
            )
        yield json.dumps({"type": "sources", "sources": cited}) + "\n"
        for token in get_chat_provider().stream(prompt):
            yield json.dumps({"type": "token", "content": token}) + "\n"
        yield json.dumps({"type": "done"}) + "\n"

    return StreamingResponse(generate(), media_type="application/x-ndjson")
