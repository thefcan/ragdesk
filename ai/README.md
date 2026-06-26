# ragdesk-ai

Python / FastAPI service for the AI layer: document ingestion
(chunk → embed → `pgvector`), retrieval, and provider-agnostic RAG chat.

## Run locally

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness probe (checks Postgres) |

More endpoints (ingest, chat) arrive in Phase 2–3.
