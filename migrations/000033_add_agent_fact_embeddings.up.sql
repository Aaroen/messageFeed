CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS agent_fact_embeddings (
    id BIGSERIAL PRIMARY KEY,
    canonical_ref VARCHAR(160) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    embedding_model VARCHAR(128) NOT NULL,
    embedding_dimension INTEGER NOT NULL,
    content_hash VARCHAR(72) NOT NULL,
    embedding vector(1536) NOT NULL,
    embedding_status VARCHAR(16) NOT NULL DEFAULT 'ready',
    error_message TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_fact_embeddings_ref_model_hash
        UNIQUE (canonical_ref, embedding_model, content_hash),
    CONSTRAINT chk_agent_fact_embeddings_status
        CHECK (embedding_status IN ('ready', 'pending', 'failed', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_user_model
    ON agent_fact_embeddings(user_id, embedding_model, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_ref
    ON agent_fact_embeddings(canonical_ref);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_vector_hnsw
    ON agent_fact_embeddings USING hnsw (embedding vector_cosine_ops);

DROP TRIGGER IF EXISTS update_agent_fact_embeddings_updated_at ON agent_fact_embeddings;
CREATE TRIGGER update_agent_fact_embeddings_updated_at
    BEFORE UPDATE ON agent_fact_embeddings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_fact_index_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_type VARCHAR(32) NOT NULL,
    scope_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    cursor_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    total_count INTEGER NOT NULL DEFAULT 0,
    processed_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_fact_index_jobs_type
        CHECK (job_type IN ('backfill_fact_index', 'embed_fact_index', 'rebuild_fact_index')),
    CONSTRAINT chk_agent_fact_index_jobs_status
        CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_agent_fact_index_jobs_status
    ON agent_fact_index_jobs(status, created_at ASC);

DROP TRIGGER IF EXISTS update_agent_fact_index_jobs_updated_at ON agent_fact_index_jobs;
CREATE TRIGGER update_agent_fact_index_jobs_updated_at
    BEFORE UPDATE ON agent_fact_index_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
