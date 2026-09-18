CREATE TABLE IF NOT EXISTS credentials (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    ciphertext BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    environment_type TEXT NOT NULL,
    connection_type TEXT NOT NULL,
    runner_url TEXT,
    ssh_host TEXT,
    ssh_port INTEGER NOT NULL DEFAULT 0,
    ssh_user TEXT,
    ssh_host_key_fingerprint TEXT,
    credential_id TEXT REFERENCES credentials(id),
    os TEXT NOT NULL DEFAULT '',
    hostname TEXT NOT NULL DEFAULT '',
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
