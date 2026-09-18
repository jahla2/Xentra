ALTER TABLE repository_integrations
    ADD COLUMN IF NOT EXISTS webhook_secret_credential_id TEXT REFERENCES credentials(id);

CREATE TABLE IF NOT EXISTS github_webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,
    integration_id TEXT NOT NULL REFERENCES repository_integrations(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_integration
    ON github_webhook_deliveries(integration_id, received_at DESC);
