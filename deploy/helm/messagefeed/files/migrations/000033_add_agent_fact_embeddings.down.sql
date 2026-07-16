DROP TRIGGER IF EXISTS update_agent_fact_index_jobs_updated_at ON agent_fact_index_jobs;
DROP TABLE IF EXISTS agent_fact_index_jobs;

DROP TRIGGER IF EXISTS update_agent_fact_embeddings_updated_at ON agent_fact_embeddings;
DROP INDEX IF EXISTS idx_agent_fact_embeddings_vector_hnsw;
DROP INDEX IF EXISTS idx_agent_fact_embeddings_ref;
DROP INDEX IF EXISTS idx_agent_fact_embeddings_user_model;
DROP TABLE IF EXISTS agent_fact_embeddings;
