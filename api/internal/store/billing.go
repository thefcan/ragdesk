package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Billing is a workspace's plan and current-period usage, for enforcement and
// display. Role is the requesting user's role (for owner-only billing actions).
type Billing struct {
	Plan      string
	Status    string
	Role      string
	Documents int
	ChatUsed  int
}

// WorkspaceBilling returns a workspace's plan and current usage, enforcing
// membership. period is the UTC month key, e.g. "2026-06".
func (s *Store) WorkspaceBilling(ctx context.Context, userID, workspaceID, period string) (Billing, error) {
	ws, err := s.GetWorkspaceForUser(ctx, userID, workspaceID)
	if err != nil {
		return Billing{}, err // ErrNotFound also hides workspaces the user can't see
	}
	b := Billing{Plan: ws.Plan, Status: ws.SubscriptionStatus, Role: ws.Role}
	if err := s.pool.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM documents WHERE workspace_id = $1),
		   COALESCE((SELECT count FROM usage_counters
		             WHERE workspace_id = $1 AND period = $2 AND metric = 'chat_messages'), 0)`,
		workspaceID, period,
	).Scan(&b.Documents, &b.ChatUsed); err != nil {
		return Billing{}, err
	}
	return b, nil
}

// CountDocuments returns the number of documents in a workspace.
func (s *Store) CountDocuments(ctx context.Context, workspaceID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM documents WHERE workspace_id = $1`, workspaceID).Scan(&n)
	return n, err
}

// CurrentUsage returns the counter value for a metric in a billing period (0 if
// no row exists yet).
func (s *Store) CurrentUsage(ctx context.Context, workspaceID, metric, period string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE((SELECT count FROM usage_counters
		         WHERE workspace_id = $1 AND period = $2 AND metric = $3), 0)`,
		workspaceID, period, metric,
	).Scan(&n)
	return n, err
}

// IncrementUsage atomically bumps a usage counter and returns the new value.
func (s *Store) IncrementUsage(ctx context.Context, workspaceID, metric, period string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO usage_counters (workspace_id, period, metric, count) VALUES ($1, $2, $3, 1)
		 ON CONFLICT (workspace_id, period, metric)
		 DO UPDATE SET count = usage_counters.count + 1, updated_at = now()
		 RETURNING count`,
		workspaceID, period, metric,
	).Scan(&n)
	return n, err
}

// SetWorkspacePlanByID sets a workspace's plan and (when non-empty) its Stripe
// identifiers, keyed by workspace id. Used by checkout fulfilment and the dev
// confirm path, both of which know the workspace directly.
func (s *Store) SetWorkspacePlanByID(ctx context.Context, workspaceID, plan, status, customerID, subscriptionID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workspaces SET
		   plan = $2,
		   subscription_status = $3,
		   stripe_customer_id = COALESCE(NULLIF($4, ''), stripe_customer_id),
		   stripe_subscription_id = COALESCE(NULLIF($5, ''), stripe_subscription_id)
		 WHERE id = $1`,
		workspaceID, plan, status, customerID, subscriptionID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" { // invalid uuid input
			return ErrNotFound
		}
	}
	return err
}

// SetWorkspacePlanByCustomer sets a workspace's plan keyed by Stripe customer id,
// for subscription lifecycle webhooks that don't carry the workspace id.
func (s *Store) SetWorkspacePlanByCustomer(ctx context.Context, customerID, plan, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workspaces SET plan = $2, subscription_status = $3 WHERE stripe_customer_id = $1`,
		customerID, plan, status,
	)
	return err
}

// WorkspaceStripeCustomer returns a workspace's stored Stripe customer id, or ""
// if none has been recorded yet. Used to reuse the customer across checkouts and
// to open the billing portal.
func (s *Store) WorkspaceStripeCustomer(ctx context.Context, workspaceID string) (string, error) {
	var customer *string
	err := s.pool.QueryRow(ctx, `SELECT stripe_customer_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&customer)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" { // invalid uuid input
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if customer == nil {
		return "", nil
	}
	return *customer, nil
}

// MarkWebhookProcessed records a Stripe event id, returning true if it was newly
// inserted (i.e. not seen before). At-least-once delivery means duplicates must
// be ignored; a false result tells the caller to skip reprocessing.
func (s *Store) MarkWebhookProcessed(ctx context.Context, eventID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO processed_webhook_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		eventID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
