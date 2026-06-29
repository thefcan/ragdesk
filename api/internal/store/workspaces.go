package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Workspace is a tenant. Role is the requesting user's role, when known.
type Workspace struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	OwnerID            string    `json:"owner_id"`
	Role               string    `json:"role,omitempty"`
	Plan               string    `json:"plan"`
	SubscriptionStatus string    `json:"subscription_status"`
	CreatedAt          time.Time `json:"created_at"`
}

// CreateWorkspace creates a workspace and enrols the owner, atomically.
func (s *Store) CreateWorkspace(ctx context.Context, ownerID, name, slug string) (Workspace, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Workspace{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var w Workspace
	if err := tx.QueryRow(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ($1, $2, $3)
		 RETURNING id::text, name, slug, owner_id::text, plan, subscription_status, created_at`,
		name, slug, ownerID,
	).Scan(&w.ID, &w.Name, &w.Slug, &w.OwnerID, &w.Plan, &w.SubscriptionStatus, &w.CreatedAt); err != nil {
		return Workspace{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`,
		w.ID, ownerID,
	); err != nil {
		return Workspace{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Workspace{}, err
	}
	w.Role = "owner"
	return w, nil
}

// ListWorkspacesForUser returns the workspaces the user is a member of.
func (s *Store) ListWorkspacesForUser(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT w.id::text, w.name, w.slug, w.owner_id::text, m.role, w.plan, w.subscription_status, w.created_at
		 FROM workspaces w
		 JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE m.user_id = $1
		 ORDER BY w.created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Workspace{}
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Slug, &w.OwnerID, &w.Role, &w.Plan, &w.SubscriptionStatus, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWorkspaceForUser returns a workspace only if the user is a member
// (tenant isolation). A malformed id is treated as not found.
func (s *Store) GetWorkspaceForUser(ctx context.Context, userID, workspaceID string) (Workspace, error) {
	var w Workspace
	err := s.pool.QueryRow(ctx,
		`SELECT w.id::text, w.name, w.slug, w.owner_id::text, m.role, w.plan, w.subscription_status, w.created_at
		 FROM workspaces w
		 JOIN workspace_members m ON m.workspace_id = w.id
		 WHERE w.id = $1 AND m.user_id = $2`,
		workspaceID, userID,
	).Scan(&w.ID, &w.Name, &w.Slug, &w.OwnerID, &w.Role, &w.Plan, &w.SubscriptionStatus, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" { // invalid uuid input
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, err
	}
	return w, nil
}
