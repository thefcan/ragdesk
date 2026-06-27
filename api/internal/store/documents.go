package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Document is an uploaded source whose text is chunked and embedded.
type Document struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	ChunkCount  int       `json:"chunk_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateDocument inserts a pending document, enforcing workspace membership.
func (s *Store) CreateDocument(ctx context.Context, userID, workspaceID, title, sourceText string) (Document, error) {
	if _, err := s.GetWorkspaceForUser(ctx, userID, workspaceID); err != nil {
		return Document{}, err // ErrNotFound hides workspaces the user can't see
	}
	var d Document
	err := s.pool.QueryRow(ctx,
		`INSERT INTO documents (workspace_id, title, source_text) VALUES ($1, $2, $3)
		 RETURNING id::text, workspace_id::text, title, status, chunk_count, created_at`,
		workspaceID, title, sourceText,
	).Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Status, &d.ChunkCount, &d.CreatedAt)
	if err != nil {
		return Document{}, err
	}
	return d, nil
}

// ListDocuments returns a workspace's documents (membership enforced).
func (s *Store) ListDocuments(ctx context.Context, userID, workspaceID string) ([]Document, error) {
	if _, err := s.GetWorkspaceForUser(ctx, userID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, workspace_id::text, title, status, chunk_count, created_at
		 FROM documents WHERE workspace_id = $1 ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Title, &d.Status, &d.ChunkCount, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DocumentText returns a document's workspace and source text for the ingest
// worker. It is internal (no user check); the worker is trusted.
func (s *Store) DocumentText(ctx context.Context, documentID string) (workspaceID, text string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT workspace_id::text, source_text FROM documents WHERE id = $1`,
		documentID,
	).Scan(&workspaceID, &text)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return "", "", ErrNotFound
	}
	return workspaceID, text, err
}

// MarkDocumentReady records a successful ingestion.
func (s *Store) MarkDocumentReady(ctx context.Context, documentID string, chunkCount int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE documents SET status = 'ready', chunk_count = $2, error = NULL WHERE id = $1`,
		documentID, chunkCount,
	)
	return err
}

// MarkDocumentFailed records a failed ingestion with a reason.
func (s *Store) MarkDocumentFailed(ctx context.Context, documentID, reason string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE documents SET status = 'failed', error = $2 WHERE id = $1`,
		documentID, reason,
	)
	return err
}
