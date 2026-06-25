ALTER TABLE agent_plan_steps
    ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_retries INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS last_retry_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS retry_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS retry_metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE agent_plan_steps
    DROP CONSTRAINT IF EXISTS chk_agent_plan_steps_retry;

ALTER TABLE agent_plan_steps
    ADD CONSTRAINT chk_agent_plan_steps_retry CHECK (retry_count >= 0 AND max_retries >= 0);
