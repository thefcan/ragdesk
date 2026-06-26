// Package store is the PostgreSQL data layer. Tenant isolation is enforced
// inside the queries themselves: workspace-scoped reads join through
// workspace_members on the requesting user.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors mapped to HTTP status codes by the handlers.
var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("already exists")
	ErrForbidden = errors.New("forbidden")
)

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Ping verifies database connectivity for the readiness probe.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
