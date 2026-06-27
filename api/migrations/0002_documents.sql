-- Vector storage for document chunks.
CREATE EXTENSION IF NOT EXISTS vector;

-- Documents are owned by the Go API (workspace-scoped, tenant FK).
CREATE TABLE IF NOT EXISTS documents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    source_text  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending',
    chunk_count  INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_documents_workspace ON documents (workspace_id);

-- Chunks are written by the Python AI service. They intentionally carry no
-- cross-table foreign keys (service-boundary decoupling); document_id and
-- workspace_id are denormalised so vector search can filter by tenant directly.
CREATE TABLE IF NOT EXISTS chunks (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  UUID NOT NULL,
    workspace_id UUID NOT NULL,
    idx          INTEGER NOT NULL,
    content      TEXT NOT NULL,
    embedding    vector(768),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks (document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_workspace ON chunks (workspace_id);
CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON chunks USING hnsw (embedding vector_cosine_ops);
