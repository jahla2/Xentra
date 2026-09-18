CREATE TABLE IF NOT EXISTS actions (
    id TEXT PRIMARY KEY,
    incident_id TEXT REFERENCES incidents(id) ON DELETE SET NULL,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL,
    approved_by TEXT,
    result TEXT NOT NULL DEFAULT '',
    verification JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    executed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    actor TEXT NOT NULL,
    event_type TEXT NOT NULL,
    detail TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
