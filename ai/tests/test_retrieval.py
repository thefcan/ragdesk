"""Integration test for tenant-filtered vector retrieval."""

import os

import psycopg
import pytest

from app.config import settings
from app.ingest import ingest_document
from app.retrieval import retrieve

SCHEMA = """
CREATE EXTENSION IF NOT EXISTS vector;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS documents;
CREATE TABLE documents (id uuid primary key, workspace_id uuid, title text);
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

WS_A = "11111111-1111-1111-1111-111111111111"
WS_B = "22222222-2222-2222-2222-222222222222"
DOC_A = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
DOC_B = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"


@pytest.mark.skipif(not os.getenv("DATABASE_URL"), reason="DATABASE_URL not set")
def test_retrieval_is_tenant_scoped():
    settings.embedding_provider = "fake"
    with psycopg.connect(settings.database_url, autocommit=True) as conn:
        for stmt in SCHEMA.split(";"):
            if stmt.strip():
                conn.execute(stmt)
        conn.execute(
            "INSERT INTO documents (id, workspace_id, title) VALUES (%s, %s, %s)",
            (DOC_A, WS_A, "Workspace A Handbook"),
        )
        conn.execute(
            "INSERT INTO documents (id, workspace_id, title) VALUES (%s, %s, %s)",
            (DOC_B, WS_B, "Workspace B Secret"),
        )

    ingest_document(DOC_A, WS_A, "Alpha content about remote work. " * 30)
    ingest_document(DOC_B, WS_B, "Beta confidential roadmap. " * 30)

    results = retrieve(WS_A, "remote work policy", k=4)
    assert results, "expected at least one chunk"
    # Only workspace A's document is ever returned.
    assert all(r["title"] == "Workspace A Handbook" for r in results)
