"""Ingestion pipeline: chunk -> embed -> store in pgvector."""

import psycopg
from pgvector.psycopg import register_vector

from app.chunking import chunk_text
from app.config import settings
from app.embeddings import get_embedder


def ingest_document(document_id: str, workspace_id: str, text: str) -> int:
    """Chunk and embed `text`, replacing any existing chunks for the document.

    Returns the number of chunks written.
    """
    chunks = chunk_text(text, settings.chunk_size, settings.chunk_overlap)
    if not chunks:
        return 0

    embeddings = get_embedder().embed(chunks)

    with psycopg.connect(settings.database_url) as conn:
        register_vector(conn)
        with conn.cursor() as cur:
            cur.execute("DELETE FROM chunks WHERE document_id = %s", (document_id,))
            for idx, (content, embedding) in enumerate(zip(chunks, embeddings, strict=True)):
                cur.execute(
                    "INSERT INTO chunks (document_id, workspace_id, idx, content, embedding) "
                    "VALUES (%s, %s, %s, %s, %s)",
                    (document_id, workspace_id, idx, content, embedding),
                )
        conn.commit()
    return len(chunks)
