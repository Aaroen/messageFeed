ALTER TABLE agent_sessions
    ADD COLUMN IF NOT EXISTS context_initialized_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS context_rebuild_started_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS context_rebuild_finished_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS context_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS transcript_count_indexed BIGINT NOT NULL DEFAULT 0;

ALTER TABLE external_accounts
    ADD COLUMN IF NOT EXISTS active_agent_session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_external_accounts_active_agent_session
    ON external_accounts(active_agent_session_id);
