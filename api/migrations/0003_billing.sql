-- Subscription state lives on the workspace (the billing entity in ragdesk).
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS plan                   TEXT NOT NULL DEFAULT 'free',
    ADD COLUMN IF NOT EXISTS subscription_status    TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS stripe_customer_id     TEXT,
    ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT;

-- Webhooks that lack the workspace id look it up by Stripe customer.
CREATE INDEX IF NOT EXISTS idx_workspaces_stripe_customer ON workspaces (stripe_customer_id);

-- Per-period usage counters for metered billing (e.g. chat messages per month).
-- One row per (workspace, period, metric), incremented atomically. period is the
-- UTC month key 'YYYY-MM'. ON DELETE CASCADE ties usage to its workspace.
CREATE TABLE IF NOT EXISTS usage_counters (
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    period       TEXT        NOT NULL,
    metric       TEXT        NOT NULL,
    count        BIGINT      NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, period, metric)
);
