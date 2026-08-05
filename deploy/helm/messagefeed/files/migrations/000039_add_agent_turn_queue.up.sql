ALTER TABLE agent_turns
    DROP CONSTRAINT IF EXISTS chk_agent_turns_status;

ALTER TABLE agent_turns
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS locked_by TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS cancel_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_turns
    ADD CONSTRAINT chk_agent_turns_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed'));

CREATE INDEX IF NOT EXISTS idx_agent_turns_queue_claim
    ON agent_turns(status, lease_until, created_at, id);

CREATE INDEX IF NOT EXISTS idx_agent_turns_cancel_requested
    ON agent_turns(cancel_requested, status, updated_at DESC)
    WHERE cancel_requested = TRUE;
