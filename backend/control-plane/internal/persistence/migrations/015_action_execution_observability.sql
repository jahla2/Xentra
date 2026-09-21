ALTER TABLE actions
    ADD COLUMN IF NOT EXISTS execution_stage TEXT NOT NULL DEFAULT 'awaiting_approval',
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT NOT NULL DEFAULT 0;

UPDATE actions
SET execution_stage = CASE status
    WHEN 'pending_approval' THEN 'awaiting_approval'
    WHEN 'rejected' THEN 'rejected'
    WHEN 'approved' THEN 'queued'
    WHEN 'completed' THEN 'completed'
    WHEN 'failed' THEN 'failed'
    WHEN 'verification_failed' THEN 'verification_failed'
    ELSE execution_stage
END;

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS action_id TEXT REFERENCES actions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS tool TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approval TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS result TEXT NOT NULL DEFAULT '';
