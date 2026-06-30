DROP INDEX IF EXISTS idx_agent_fact_embeddings_vector_hnsw;

ALTER TABLE agent_fact_embeddings
    ALTER COLUMN embedding TYPE vector(1536);

CREATE INDEX IF NOT EXISTS idx_agent_fact_embeddings_vector_hnsw
    ON agent_fact_embeddings USING hnsw (embedding vector_cosine_ops);
