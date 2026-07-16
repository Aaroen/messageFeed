DROP TRIGGER IF EXISTS update_item_events_updated_at ON item_events;
DROP INDEX IF EXISTS idx_item_events_item;
DROP INDEX IF EXISTS idx_item_events_user_created;
DROP INDEX IF EXISTS idx_item_events_claim;
DROP TABLE IF EXISTS item_events;

DROP TRIGGER IF EXISTS update_source_fetch_attempts_updated_at ON source_fetch_attempts;
DROP INDEX IF EXISTS idx_source_fetch_attempts_source_started;
DROP INDEX IF EXISTS idx_source_fetch_attempts_job;
DROP TABLE IF EXISTS source_fetch_attempts;

DROP TRIGGER IF EXISTS update_source_fetch_jobs_updated_at ON source_fetch_jobs;
DROP INDEX IF EXISTS idx_source_fetch_jobs_user_created;
DROP INDEX IF EXISTS idx_source_fetch_jobs_source_created;
DROP INDEX IF EXISTS idx_source_fetch_jobs_claim;
DROP TABLE IF EXISTS source_fetch_jobs;

DROP INDEX IF EXISTS idx_sources_next_fetch;
ALTER TABLE sources DROP CONSTRAINT IF EXISTS chk_sources_fetch_priority;
ALTER TABLE sources
    DROP COLUMN IF EXISTS fetch_priority,
    DROP COLUMN IF EXISTS last_modified,
    DROP COLUMN IF EXISTS etag,
    DROP COLUMN IF EXISTS next_fetch_at;
