CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS incident_memories (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    incident_id TEXT NOT NULL UNIQUE REFERENCES incidents(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    embedding vector(384) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incident_memories_scope
    ON incident_memories(organization_id, environment_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_incident_memories_embedding_hnsw
    ON incident_memories USING hnsw (embedding vector_cosine_ops);
