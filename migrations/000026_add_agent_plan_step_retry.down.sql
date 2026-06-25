ALTER TABLE agent_plan_steps
    DROP CONSTRAINT IF EXISTS chk_agent_plan_steps_retry;

ALTER TABLE agent_plan_steps
    DROP COLUMN IF EXISTS retry_metadata_json,
    DROP COLUMN IF EXISTS retry_reason,
    DROP COLUMN IF EXISTS last_retry_at,
    DROP COLUMN IF EXISTS max_retries,
    DROP COLUMN IF EXISTS retry_count;
