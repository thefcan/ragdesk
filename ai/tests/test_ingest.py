"""Integration test for the ingestion pipeline against a real pgvector DB."""

import os

import psycopg
import pytest
from fastapi.testclient import TestClient

from app.config import settings
from app.ingest import ingest_document
from app.main import app

# Self-contained schema: chunks carries no cross-table FKs (service boundary),
# so the test needs no users/workspaces/documents rows.
SCHEMA = """
CREATE EXTENSION IF NOT EXISTS vector;
DROP TABLE IF EXISTS chunks;
CREATE TABLE chunks (
    id uuid primary key default gen_random_uuid(),
    document_id uuid not null,
    workspace_id uuid not null,
    idx int not null,
    content text not null,
    embedding vector(768),
    created_at timestamptz default now()
);
"""


@pytest.mark.skipif(not os.getenv("DATABASE_URL"), reason="DATABASE_URL not set")
def test_ingest_writes_chunks_with_embeddings():
    settings.embedding_provider = "fake"
    doc = "33333333-3333-3333-3333-333333333333"
    ws = "22222222-2222-2222-2222-222222222222"

    with psycopg.connect(settings.database_url, autocommit=True) as conn:
        for stmt in SCHEMA.split(";"):
            if stmt.strip():
                conn.execute(stmt)

    count = ingest_document(doc, ws, "abcdefghij" * 250)
    assert count >= 3

    with psycopg.connect(settings.database_url) as conn:
        stored = conn.execute(
            "SELECT count(*) FROM chunks WHERE document_id = %s", (doc,)
        ).fetchone()[0]
        assert stored == count

        dim = conn.execute(
            "SELECT vector_dims(embedding) FROM chunks WHERE document_id = %s ORDER BY idx LIMIT 1",
            (doc,),
        ).fetchone()[0]
        assert dim == 768


def test_ingest_rejects_missing_internal_token():
    settings.internal_token = "secret"
    try:
        client = TestClient(app)
        body = {"document_id": "d", "workspace_id": "w", "text": "x"}
        assert client.post("/ingest", json=body).status_code == 401
        assert (
            client.post("/ingest", headers={"X-Internal-Token": "wrong"}, json=body).status_code
            == 401
        )
    finally:
        settings.internal_token = ""
