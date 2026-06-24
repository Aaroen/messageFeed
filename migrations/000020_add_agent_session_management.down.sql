DROP INDEX IF EXISTS idx_external_accounts_active_agent_session;

ALTER TABLE external_accounts
    DROP COLUMN IF EXISTS active_agent_session_id;

ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS transcript_count_indexed,
    DROP COLUMN IF EXISTS context_version,
    DROP COLUMN IF EXISTS context_rebuild_finished_at,
    DROP COLUMN IF EXISTS context_rebuild_started_at,
    DROP COLUMN IF EXISTS context_initialized_at;
