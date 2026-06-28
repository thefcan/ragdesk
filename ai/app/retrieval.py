"""Vector retrieval: find the chunks most similar to a question, per tenant."""

import psycopg

from app.config import settings
from app.embeddings import get_embedder


def _to_vector_literal(embedding: list[float]) -> str:
    return "[" + ",".join(str(x) for x in embedding) + "]"


def retrieve(workspace_id: str, question: str, k: int) -> list[dict]:
    """Return up to k chunks from the workspace ranked by cosine similarity.

    Tenant isolation is enforced in the query: only chunks belonging to
    workspace_id are considered.
    """
    embedding = get_embedder().embed([question])[0]
    vec = _to_vector_literal(embedding)
    with psycopg.connect(settings.database_url) as conn:
        rows = conn.execute(
            """
            SELECT c.document_id::text, d.title, c.content
            FROM chunks c
            JOIN documents d ON d.id = c.document_id
            WHERE c.workspace_id = %s
            ORDER BY c.embedding <=> %s::vector
            LIMIT %s
            """,
            (workspace_id, vec, k),
        ).fetchall()
    return [{"document_id": r[0], "title": r[1], "content": r[2]} for r in rows]
