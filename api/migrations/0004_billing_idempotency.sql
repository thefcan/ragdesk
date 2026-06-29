-- Stripe delivers webhooks at least once (retries, network blips, manual
-- resends). Record processed event ids so fulfilment is idempotent.
CREATE TABLE IF NOT EXISTS processed_webhook_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
