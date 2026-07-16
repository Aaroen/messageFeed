CREATE TABLE IF NOT EXISTS agent_transcript_archive_index (
    id BIGSERIAL PRIMARY KEY,
    transcript_entry_id BIGINT NOT NULL REFERENCES agent_transcript_entries(id) ON DELETE CASCADE,
    session_id BIGINT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    archive_status VARCHAR(16) NOT NULL DEFAULT 'hot',
    memory_kind VARCHAR(32) NOT NULL DEFAULT 'unknown',
    importance INTEGER NOT NULL DEFAULT 0,
    keywords_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_accessed_at TIMESTAMP WITH TIME ZONE,
    access_count INTEGER NOT NULL DEFAULT 0,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_transcript_archive_entry UNIQUE (transcript_entry_id),
    CONSTRAINT chk_agent_transcript_archive_status CHECK (archive_status IN ('hot', 'warm', 'cold')),
    CONSTRAINT chk_agent_transcript_memory_kind CHECK (memory_kind IN ('preference', 'task', 'fact', 'decision', 'casual', 'unknown')),
    CONSTRAINT chk_agent_transcript_importance CHECK (importance >= 0 AND importance <= 100)
);

CREATE INDEX IF NOT EXISTS idx_agent_transcript_archive_session_status
    ON agent_transcript_archive_index(session_id, archive_status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_transcript_archive_user_kind
    ON agent_transcript_archive_index(user_id, memory_kind, importance DESC);

DROP TRIGGER IF EXISTS update_agent_transcript_archive_index_updated_at ON agent_transcript_archive_index;
CREATE TRIGGER update_agent_transcript_archive_index_updated_at
    BEFORE UPDATE ON agent_transcript_archive_index
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_recall_events (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query_text TEXT NOT NULL DEFAULT '',
    query_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    recalled_refs_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    reason TEXT NOT NULL DEFAULT '',
    budget_chars INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_recall_events_session_created
    ON agent_recall_events(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_recall_events_user_created
    ON agent_recall_events(user_id, created_at DESC);
