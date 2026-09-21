CREATE TABLE IF NOT EXISTS repository_integrations (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL UNIQUE REFERENCES environments(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    owner_name TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    credential_id TEXT NOT NULL REFERENCES credentials(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    status TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    root_cause TEXT NOT NULL DEFAULT '',
    confidence TEXT NOT NULL DEFAULT '',
    recommended_action TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    timeline JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
