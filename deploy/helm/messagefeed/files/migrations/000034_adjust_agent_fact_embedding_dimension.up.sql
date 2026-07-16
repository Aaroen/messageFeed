-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP INDEX IF EXISTS idx_agent_fact_embeddings_vector_hnsw;

ALTER TABLE agent_fact_embeddings
    ALTER COLUMN embedding TYPE vector(4096);
