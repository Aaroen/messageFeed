-- Helm 打包副本；源迁移位于项目 migrations 目录。
ALTER TABLE agent_fact_archive_index
    DROP CONSTRAINT IF EXISTS chk_agent_fact_archive_fact_type;

ALTER TABLE agent_fact_archive_index
    ADD CONSTRAINT chk_agent_fact_archive_fact_type CHECK (fact_type IN ('transcript', 'observation', 'artifact', 'plan', 'plan_step', 'run_trace', 'item', 'web_snapshot', 'memory_chunk'));

CREATE TABLE IF NOT EXISTS agent_memory_topics (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE CASCADE,
    topic_key TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    keywords_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    intent TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    message_count INTEGER NOT NULL DEFAULT 0,
    token_estimate INTEGER NOT NULL DEFAULT 0,
    start_turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    end_turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    last_message_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_memory_topics_status CHECK (status IN ('active', 'closed')),
    CONSTRAINT chk_agent_memory_topics_counts CHECK (message_count >= 0 AND token_estimate >= 0)
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_topics_user_status
    ON agent_memory_topics(user_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_topics_session_status
    ON agent_memory_topics(session_id, status, updated_at DESC);

DROP TRIGGER IF EXISTS update_agent_memory_topics_updated_at ON agent_memory_topics;
CREATE TRIGGER update_agent_memory_topics_updated_at
    BEFORE UPDATE ON agent_memory_topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_memory_chunks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE CASCADE,
    topic_id BIGINT REFERENCES agent_memory_topics(id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    memory_kind VARCHAR(32) NOT NULL DEFAULT 'unknown',
    importance INTEGER NOT NULL DEFAULT 0,
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    relation_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    start_turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    end_turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    content_hash TEXT NOT NULL DEFAULT '',
    embedding_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    consolidation_reason VARCHAR(32) NOT NULL DEFAULT 'unknown',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_memory_chunks_memory_kind CHECK (memory_kind IN ('preference', 'task', 'fact', 'decision', 'casual', 'unknown')),
    CONSTRAINT chk_agent_memory_chunks_importance CHECK (importance >= 0 AND importance <= 100),
    CONSTRAINT chk_agent_memory_chunks_embedding_status CHECK (embedding_status IN ('pending', 'ready', 'failed', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_chunks_user_created
    ON agent_memory_chunks(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_chunks_topic
    ON agent_memory_chunks(topic_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_chunks_embedding_status
    ON agent_memory_chunks(embedding_status, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_memory_chunks_content_hash
    ON agent_memory_chunks(user_id, content_hash)
    WHERE content_hash <> '';

DROP TRIGGER IF EXISTS update_agent_memory_chunks_updated_at ON agent_memory_chunks;
CREATE TRIGGER update_agent_memory_chunks_updated_at
    BEFORE UPDATE ON agent_memory_chunks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
