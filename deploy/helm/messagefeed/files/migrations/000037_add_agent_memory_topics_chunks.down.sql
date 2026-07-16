DROP TRIGGER IF EXISTS update_agent_memory_chunks_updated_at ON agent_memory_chunks;
DROP TRIGGER IF EXISTS update_agent_memory_topics_updated_at ON agent_memory_topics;
DROP TABLE IF EXISTS agent_memory_chunks;
DROP TABLE IF EXISTS agent_memory_topics;

ALTER TABLE agent_fact_archive_index
    DROP CONSTRAINT IF EXISTS chk_agent_fact_archive_fact_type;

ALTER TABLE agent_fact_archive_index
    ADD CONSTRAINT chk_agent_fact_archive_fact_type CHECK (fact_type IN ('transcript', 'observation', 'artifact', 'plan', 'plan_step', 'run_trace', 'item', 'web_snapshot'));
