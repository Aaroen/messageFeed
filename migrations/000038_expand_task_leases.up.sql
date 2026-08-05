ALTER TABLE source_fetch_jobs
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE;

ALTER TABLE notification_jobs
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE;

ALTER TABLE ai_analysis_jobs
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE;

ALTER TABLE agent_scheduled_tasks
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE;

ALTER TABLE item_events
    ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS locked_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE;

ALTER TABLE agent_fact_index_jobs
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS locked_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS lease_until TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE item_events DROP CONSTRAINT IF EXISTS chk_item_events_attempts;
ALTER TABLE item_events
    ADD CONSTRAINT chk_item_events_attempts CHECK (attempt_count >= 0 AND max_attempts > 0);

ALTER TABLE agent_fact_index_jobs DROP CONSTRAINT IF EXISTS chk_agent_fact_index_jobs_attempts;
ALTER TABLE agent_fact_index_jobs
    ADD CONSTRAINT chk_agent_fact_index_jobs_attempts CHECK (attempt_count >= 0 AND max_attempts > 0);

CREATE INDEX IF NOT EXISTS idx_source_fetch_jobs_lease
    ON source_fetch_jobs(status, lease_until, scheduled_at, id)
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_notification_jobs_lease
    ON notification_jobs(status, lease_until, scheduled_at, id)
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_ai_analysis_jobs_lease
    ON ai_analysis_jobs(status, lease_until, scheduled_at, id)
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_agent_scheduled_tasks_lease
    ON agent_scheduled_tasks(status, lease_until, scheduled_at, id)
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_item_events_lease
    ON item_events(status, lease_until, available_at, id)
    WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_agent_fact_index_jobs_claim
    ON agent_fact_index_jobs(job_type, status, next_run_at, lease_until, created_at, id);
