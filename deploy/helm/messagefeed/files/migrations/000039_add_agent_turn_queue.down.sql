DROP INDEX IF EXISTS idx_agent_turns_cancel_requested;
DROP INDEX IF EXISTS idx_agent_turns_queue_claim;

ALTER TABLE agent_turns
    DROP CONSTRAINT IF EXISTS chk_agent_turns_status;

ALTER TABLE agent_turns
    DROP COLUMN IF EXISTS cancel_reason,
    DROP COLUMN IF EXISTS cancel_requested,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS max_attempts,
    DROP COLUMN IF EXISTS attempt_count;

ALTER TABLE agent_turns
    ADD CONSTRAINT chk_agent_turns_status CHECK (status IN ('running', 'succeeded', 'failed'));
