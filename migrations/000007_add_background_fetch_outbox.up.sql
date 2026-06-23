ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS next_fetch_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS etag TEXT,
    ADD COLUMN IF NOT EXISTS last_modified TEXT,
    ADD COLUMN IF NOT EXISTS fetch_priority INTEGER NOT NULL DEFAULT 0;

ALTER TABLE sources DROP CONSTRAINT IF EXISTS chk_sources_fetch_priority;
ALTER TABLE sources
    ADD CONSTRAINT chk_sources_fetch_priority CHECK (fetch_priority >= 0);

CREATE INDEX IF NOT EXISTS idx_sources_next_fetch
    ON sources(status, next_fetch_at, fetch_priority DESC, id)
    WHERE next_fetch_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS source_fetch_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    trigger_type VARCHAR(32) NOT NULL DEFAULT 'scheduled',
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    priority INTEGER NOT NULL DEFAULT 0,
    locked_by VARCHAR(255),
    locked_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_source_fetch_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT chk_source_fetch_jobs_trigger CHECK (trigger_type IN ('scheduled', 'manual', 'retry')),
    CONSTRAINT chk_source_fetch_jobs_attempts CHECK (attempt_count >= 0 AND max_attempts > 0),
    CONSTRAINT chk_source_fetch_jobs_priority CHECK (priority >= 0)
);

CREATE INDEX IF NOT EXISTS idx_source_fetch_jobs_claim
    ON source_fetch_jobs(status, scheduled_at, priority DESC, id);
CREATE INDEX IF NOT EXISTS idx_source_fetch_jobs_source_created
    ON source_fetch_jobs(source_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_source_fetch_jobs_user_created
    ON source_fetch_jobs(user_id, created_at DESC);

DROP TRIGGER IF EXISTS update_source_fetch_jobs_updated_at ON source_fetch_jobs;
CREATE TRIGGER update_source_fetch_jobs_updated_at
    BEFORE UPDATE ON source_fetch_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS source_fetch_attempts (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL REFERENCES source_fetch_jobs(id) ON DELETE CASCADE,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    http_status INTEGER,
    error_message TEXT,
    item_count INTEGER NOT NULL DEFAULT 0,
    created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_source_fetch_attempts_status CHECK (status IN ('running', 'succeeded', 'failed')),
    CONSTRAINT chk_source_fetch_attempts_number CHECK (attempt_number > 0),
    CONSTRAINT chk_source_fetch_attempts_counts CHECK (
        item_count >= 0 AND created_count >= 0 AND updated_count >= 0
    ),
    CONSTRAINT chk_source_fetch_attempts_duration CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT chk_source_fetch_attempts_http_status CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    CONSTRAINT uq_source_fetch_attempts_job_number UNIQUE (job_id, attempt_number)
);

CREATE INDEX IF NOT EXISTS idx_source_fetch_attempts_job
    ON source_fetch_attempts(job_id, attempt_number);
CREATE INDEX IF NOT EXISTS idx_source_fetch_attempts_source_started
    ON source_fetch_attempts(source_id, started_at DESC);

DROP TRIGGER IF EXISTS update_source_fetch_attempts_updated_at ON source_fetch_attempts;
CREATE TRIGGER update_source_fetch_attempts_updated_at
    BEFORE UPDATE ON source_fetch_attempts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS item_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    item_id BIGINT REFERENCES items(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key TEXT NOT NULL,
    available_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_item_events_type CHECK (event_type IN ('item.created', 'source.fetch_failed')),
    CONSTRAINT chk_item_events_status CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    CONSTRAINT chk_item_events_attempt_count CHECK (attempt_count >= 0),
    CONSTRAINT uq_item_events_dedupe_key UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_item_events_claim
    ON item_events(status, available_at, id);
CREATE INDEX IF NOT EXISTS idx_item_events_user_created
    ON item_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_item_events_item
    ON item_events(item_id)
    WHERE item_id IS NOT NULL;

DROP TRIGGER IF EXISTS update_item_events_updated_at ON item_events;
CREATE TRIGGER update_item_events_updated_at
    BEFORE UPDATE ON item_events
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
