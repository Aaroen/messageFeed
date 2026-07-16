-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS agent_fact_archive_index (
    id BIGSERIAL PRIMARY KEY,
    canonical_ref VARCHAR(128) NOT NULL,
    fact_type VARCHAR(32) NOT NULL,
    fact_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    memory_kind VARCHAR(32) NOT NULL DEFAULT 'unknown',
    topics_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    keywords_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    entities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    summary_for_index TEXT NOT NULL DEFAULT '',
    contextual_text TEXT NOT NULL DEFAULT '',
    full_text_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(summary_for_index, '') || ' ' || coalesce(contextual_text, ''))
    ) STORED,
    embedding_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    importance INTEGER NOT NULL DEFAULT 0,
    confidence NUMERIC(5,4) NOT NULL DEFAULT 0,
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    relation_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    index_status VARCHAR(16) NOT NULL DEFAULT 'ready',
    risk_level VARCHAR(16) NOT NULL DEFAULT 'low',
    access_count INTEGER NOT NULL DEFAULT 0,
    last_accessed_at TIMESTAMP WITH TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_fact_archive_canonical_ref UNIQUE (canonical_ref),
    CONSTRAINT chk_agent_fact_archive_fact_type CHECK (fact_type IN ('transcript', 'observation', 'artifact', 'plan', 'plan_step', 'run_trace', 'item', 'web_snapshot')),
    CONSTRAINT chk_agent_fact_archive_memory_kind CHECK (memory_kind IN ('preference', 'task', 'fact', 'decision', 'casual', 'unknown')),
    CONSTRAINT chk_agent_fact_archive_importance CHECK (importance >= 0 AND importance <= 100),
    CONSTRAINT chk_agent_fact_archive_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_agent_fact_archive_status CHECK (index_status IN ('ready', 'pending', 'failed', 'archived')),
    CONSTRAINT chk_agent_fact_archive_risk CHECK (risk_level IN ('low', 'medium', 'high'))
);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_user_kind
    ON agent_fact_archive_index(user_id, memory_kind, importance DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_session_turn
    ON agent_fact_archive_index(session_id, turn_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_type
    ON agent_fact_archive_index(fact_type, fact_id);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_status
    ON agent_fact_archive_index(index_status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_full_text
    ON agent_fact_archive_index USING GIN(full_text_vector);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_topics
    ON agent_fact_archive_index USING GIN(topics_json);

CREATE INDEX IF NOT EXISTS idx_agent_fact_archive_entities
    ON agent_fact_archive_index USING GIN(entities_json);

DROP TRIGGER IF EXISTS update_agent_fact_archive_index_updated_at ON agent_fact_archive_index;
CREATE TRIGGER update_agent_fact_archive_index_updated_at
    BEFORE UPDATE ON agent_fact_archive_index
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_memory_candidates (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    memory_kind VARCHAR(32) NOT NULL DEFAULT 'unknown',
    candidate_text TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    evidence_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    confidence NUMERIC(5,4) NOT NULL DEFAULT 0,
    importance INTEGER NOT NULL DEFAULT 0,
    risk_level VARCHAR(16) NOT NULL DEFAULT 'low',
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    proposed_by VARCHAR(32) NOT NULL DEFAULT 'system',
    expires_at TIMESTAMP WITH TIME ZONE,
    reviewed_at TIMESTAMP WITH TIME ZONE,
    reviewer_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    memory_block_id BIGINT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_memory_candidates_memory_kind CHECK (memory_kind IN ('preference', 'task', 'fact', 'decision', 'casual', 'unknown')),
    CONSTRAINT chk_agent_memory_candidates_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_agent_memory_candidates_importance CHECK (importance >= 0 AND importance <= 100),
    CONSTRAINT chk_agent_memory_candidates_risk CHECK (risk_level IN ('low', 'medium', 'high')),
    CONSTRAINT chk_agent_memory_candidates_status CHECK (status IN ('pending', 'applied', 'requires_confirmation', 'rejected', 'revoked', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_candidates_user_status
    ON agent_memory_candidates(user_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_candidates_session_turn
    ON agent_memory_candidates(session_id, turn_id, created_at DESC);

DROP TRIGGER IF EXISTS update_agent_memory_candidates_updated_at ON agent_memory_candidates;
CREATE TRIGGER update_agent_memory_candidates_updated_at
    BEFORE UPDATE ON agent_memory_candidates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_memory_blocks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    memory_kind VARCHAR(32) NOT NULL DEFAULT 'preference',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    evidence_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_candidate_id BIGINT REFERENCES agent_memory_candidates(id) ON DELETE SET NULL,
    confidence NUMERIC(5,4) NOT NULL DEFAULT 0,
    importance INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    version INTEGER NOT NULL DEFAULT 1,
    last_used_at TIMESTAMP WITH TIME ZONE,
    use_count INTEGER NOT NULL DEFAULT 0,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_memory_blocks_memory_kind CHECK (memory_kind IN ('preference', 'task', 'fact', 'decision', 'casual', 'unknown')),
    CONSTRAINT chk_agent_memory_blocks_confidence CHECK (confidence >= 0 AND confidence <= 1),
    CONSTRAINT chk_agent_memory_blocks_importance CHECK (importance >= 0 AND importance <= 100),
    CONSTRAINT chk_agent_memory_blocks_status CHECK (status IN ('active', 'superseded', 'revoked', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_blocks_user_status
    ON agent_memory_blocks(user_id, status, importance DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_blocks_kind
    ON agent_memory_blocks(user_id, memory_kind, importance DESC);

DROP TRIGGER IF EXISTS update_agent_memory_blocks_updated_at ON agent_memory_blocks;
CREATE TRIGGER update_agent_memory_blocks_updated_at
    BEFORE UPDATE ON agent_memory_blocks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE agent_memory_candidates
    ADD CONSTRAINT fk_agent_memory_candidates_block
    FOREIGN KEY (memory_block_id) REFERENCES agent_memory_blocks(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS agent_memory_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    candidate_id BIGINT REFERENCES agent_memory_candidates(id) ON DELETE SET NULL,
    memory_block_id BIGINT REFERENCES agent_memory_blocks(id) ON DELETE SET NULL,
    event_type VARCHAR(32) NOT NULL,
    actor_type VARCHAR(16) NOT NULL DEFAULT 'system',
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_memory_events_type CHECK (event_type IN ('candidate_generated', 'candidate_applied', 'candidate_requires_confirmation', 'candidate_rejected', 'candidate_revoked', 'memory_created', 'memory_updated', 'memory_revoked', 'memory_used')),
    CONSTRAINT chk_agent_memory_events_actor CHECK (actor_type IN ('system', 'model', 'user', 'admin'))
);

CREATE INDEX IF NOT EXISTS idx_agent_memory_events_user_created
    ON agent_memory_events(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_events_candidate
    ON agent_memory_events(candidate_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_memory_events_block
    ON agent_memory_events(memory_block_id, created_at DESC);
