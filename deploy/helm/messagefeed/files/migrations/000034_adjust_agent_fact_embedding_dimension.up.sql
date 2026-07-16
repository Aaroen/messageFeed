DROP INDEX IF EXISTS idx_agent_fact_embeddings_vector_hnsw;

ALTER TABLE agent_fact_embeddings
    ALTER COLUMN embedding TYPE vector(4096);
