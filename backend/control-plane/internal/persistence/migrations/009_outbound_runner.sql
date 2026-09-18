CREATE TABLE IF NOT EXISTS outbound_runners (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL UNIQUE REFERENCES environments(id) ON DELETE CASCADE,
    credential_id TEXT NOT NULL REFERENCES credentials(id) ON DELETE RESTRICT,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS runner_tasks (
    id TEXT PRIMARY KEY,
    runner_id TEXT NOT NULL REFERENCES outbound_runners(id) ON DELETE CASCADE,
    tool TEXT NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'queued',
    result_success BOOLEAN,
    result_output TEXT NOT NULL DEFAULT '',
    result_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbound_runners_org_env
    ON outbound_runners(organization_id, environment_id);

CREATE INDEX IF NOT EXISTS idx_runner_tasks_queue
    ON runner_tasks(runner_id, status, created_at);
