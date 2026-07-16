-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS ai_analysis_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_candidate_id BIGINT NOT NULL REFERENCES alert_candidates(id) ON DELETE CASCADE,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    item_id BIGINT REFERENCES items(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
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
    CONSTRAINT chk_ai_analysis_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped', 'canceled')),
    CONSTRAINT chk_ai_analysis_jobs_attempts CHECK (attempt_count >= 0 AND max_attempts > 0),
    CONSTRAINT uq_ai_analysis_jobs_candidate UNIQUE (alert_candidate_id)
);

CREATE INDEX IF NOT EXISTS idx_ai_analysis_jobs_status_scheduled
    ON ai_analysis_jobs(status, scheduled_at ASC);
CREATE INDEX IF NOT EXISTS idx_ai_analysis_jobs_user_created
    ON ai_analysis_jobs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_analysis_jobs_candidate
    ON ai_analysis_jobs(alert_candidate_id);

DROP TRIGGER IF EXISTS update_ai_analysis_jobs_updated_at ON ai_analysis_jobs;
CREATE TRIGGER update_ai_analysis_jobs_updated_at
    BEFORE UPDATE ON ai_analysis_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
