DROP INDEX IF EXISTS idx_agent_fact_index_jobs_claim;
DROP INDEX IF EXISTS idx_item_events_lease;
DROP INDEX IF EXISTS idx_agent_scheduled_tasks_lease;
DROP INDEX IF EXISTS idx_ai_analysis_jobs_lease;
DROP INDEX IF EXISTS idx_notification_jobs_lease;
DROP INDEX IF EXISTS idx_source_fetch_jobs_lease;

ALTER TABLE agent_fact_index_jobs DROP CONSTRAINT IF EXISTS chk_agent_fact_index_jobs_attempts;
ALTER TABLE item_events DROP CONSTRAINT IF EXISTS chk_item_events_attempts;

ALTER TABLE agent_fact_index_jobs
    DROP COLUMN IF EXISTS next_run_at,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS max_attempts,
    DROP COLUMN IF EXISTS attempt_count;

ALTER TABLE item_events
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS max_attempts;

ALTER TABLE agent_scheduled_tasks DROP COLUMN IF EXISTS lease_until;
ALTER TABLE ai_analysis_jobs DROP COLUMN IF EXISTS lease_until;
ALTER TABLE notification_jobs DROP COLUMN IF EXISTS lease_until;
ALTER TABLE source_fetch_jobs DROP COLUMN IF EXISTS lease_until;
