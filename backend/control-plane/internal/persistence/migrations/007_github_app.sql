ALTER TABLE repository_integrations
    ALTER COLUMN credential_id DROP NOT NULL;

ALTER TABLE repository_integrations
    ADD COLUMN IF NOT EXISTS auth_mode TEXT NOT NULL DEFAULT 'token',
    ADD COLUMN IF NOT EXISTS installation_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_repository_integrations_installation
    ON repository_integrations(installation_id)
    WHERE installation_id IS NOT NULL;
