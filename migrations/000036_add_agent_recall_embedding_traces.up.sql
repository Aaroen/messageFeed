CREATE TABLE IF NOT EXISTS agent_recall_traces (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    plan_id BIGINT REFERENCES agent_plans(id) ON DELETE SET NULL,
    mode VARCHAR(32) NOT NULL DEFAULT 'hybrid',
    query_text TEXT NOT NULL DEFAULT '',
    needs_history_recall BOOLEAN NOT NULL DEFAULT TRUE,
    history_query_plan_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    fulltext_attempted BOOLEAN NOT NULL DEFAULT FALSE,
    fulltext_count INTEGER NOT NULL DEFAULT 0,
    fulltext_ms BIGINT NOT NULL DEFAULT 0,
    embedding_attempted BOOLEAN NOT NULL DEFAULT FALSE,
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_dimension INTEGER NOT NULL DEFAULT 0,
    embedding_ms BIGINT NOT NULL DEFAULT 0,
    embedding_status VARCHAR(32) NOT NULL DEFAULT '',
    embedding_error TEXT NOT NULL DEFAULT '',
    vector_attempted BOOLEAN NOT NULL DEFAULT FALSE,
    vector_candidate_count INTEGER NOT NULL DEFAULT 0,
    vector_ms BIGINT NOT NULL DEFAULT 0,
    relation_attempted BOOLEAN NOT NULL DEFAULT FALSE,
    relation_count INTEGER NOT NULL DEFAULT 0,
    relation_ms BIGINT NOT NULL DEFAULT 0,
    final_hit_count INTEGER NOT NULL DEFAULT 0,
    final_sources_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    fallback_reason TEXT NOT NULL DEFAULT '',
    total_ms BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(24) NOT NULL DEFAULT 'succeeded',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_recall_traces_status CHECK (status IN ('succeeded', 'failed', 'degraded')),
    CONSTRAINT chk_agent_recall_traces_counts CHECK (
        fulltext_count >= 0
        AND embedding_dimension >= 0
        AND vector_candidate_count >= 0
        AND relation_count >= 0
        AND final_hit_count >= 0
        AND total_ms >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_recall_traces_request
    ON agent_recall_traces(request_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_recall_traces_turn
    ON agent_recall_traces(turn_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_recall_traces_user_created
    ON agent_recall_traces(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_embedding_traces (
    id BIGSERIAL PRIMARY KEY,
    job_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    canonical_ref VARCHAR(128) NOT NULL DEFAULT '',
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_dimension INTEGER NOT NULL DEFAULT 0,
    input_chars INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'succeeded',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_embedding_traces_status CHECK (status IN ('succeeded', 'failed', 'skipped')),
    CONSTRAINT chk_agent_embedding_traces_counts CHECK (
        embedding_dimension >= 0
        AND input_chars >= 0
        AND duration_ms >= 0
        AND retry_count >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_embedding_traces_request
    ON agent_embedding_traces(request_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_embedding_traces_job
    ON agent_embedding_traces(job_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_embedding_traces_ref
    ON agent_embedding_traces(user_id, canonical_ref, created_at DESC);
