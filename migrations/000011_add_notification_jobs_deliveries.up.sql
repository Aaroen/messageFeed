CREATE TABLE IF NOT EXISTS notification_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_candidate_id BIGINT REFERENCES alert_candidates(id) ON DELETE SET NULL,
    alert_rule_id BIGINT REFERENCES alert_rules(id) ON DELETE SET NULL,
    ai_analysis_job_id BIGINT REFERENCES ai_analysis_jobs(id) ON DELETE SET NULL,
    source_id BIGINT REFERENCES sources(id) ON DELETE SET NULL,
    item_id BIGINT REFERENCES items(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    channel VARCHAR(64) NOT NULL,
    policy_decision_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    dedupe_key TEXT NOT NULL,
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    locked_by TEXT NOT NULL DEFAULT '',
    locked_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_notification_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped', 'canceled')),
    CONSTRAINT chk_notification_jobs_channel CHECK (channel IN ('wechat_work', 'ntfy', 'in_app')),
    CONSTRAINT chk_notification_jobs_attempts CHECK (attempt_count >= 0 AND max_attempts > 0),
    CONSTRAINT uq_notification_jobs_dedupe_key UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_notification_jobs_status_scheduled
    ON notification_jobs(status, scheduled_at ASC);
CREATE INDEX IF NOT EXISTS idx_notification_jobs_user_created
    ON notification_jobs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_jobs_candidate
    ON notification_jobs(alert_candidate_id);

DROP TRIGGER IF EXISTS update_notification_jobs_updated_at ON notification_jobs;
CREATE TRIGGER update_notification_jobs_updated_at
    BEFORE UPDATE ON notification_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id BIGSERIAL PRIMARY KEY,
    notification_job_id BIGINT NOT NULL REFERENCES notification_jobs(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    provider_message_id TEXT NOT NULL DEFAULT '',
    response_status INTEGER,
    response_body TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    sent_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_notification_deliveries_channel CHECK (channel IN ('wechat_work', 'ntfy', 'in_app')),
    CONSTRAINT chk_notification_deliveries_status CHECK (status IN ('succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_job_created
    ON notification_deliveries(notification_job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_user_created
    ON notification_deliveries(user_id, created_at DESC);

DROP TRIGGER IF EXISTS update_notification_deliveries_updated_at ON notification_deliveries;
CREATE TRIGGER update_notification_deliveries_updated_at
    BEFORE UPDATE ON notification_deliveries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
