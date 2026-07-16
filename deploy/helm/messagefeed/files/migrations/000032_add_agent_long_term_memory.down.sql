DROP INDEX IF EXISTS idx_agent_memory_events_block;
DROP INDEX IF EXISTS idx_agent_memory_events_candidate;
DROP INDEX IF EXISTS idx_agent_memory_events_user_created;
DROP TABLE IF EXISTS agent_memory_events;

ALTER TABLE agent_memory_candidates
    DROP CONSTRAINT IF EXISTS fk_agent_memory_candidates_block;

DROP TRIGGER IF EXISTS update_agent_memory_blocks_updated_at ON agent_memory_blocks;
DROP INDEX IF EXISTS idx_agent_memory_blocks_kind;
DROP INDEX IF EXISTS idx_agent_memory_blocks_user_status;
DROP TABLE IF EXISTS agent_memory_blocks;

DROP TRIGGER IF EXISTS update_agent_memory_candidates_updated_at ON agent_memory_candidates;
DROP INDEX IF EXISTS idx_agent_memory_candidates_session_turn;
DROP INDEX IF EXISTS idx_agent_memory_candidates_user_status;
DROP TABLE IF EXISTS agent_memory_candidates;

DROP TRIGGER IF EXISTS update_agent_fact_archive_index_updated_at ON agent_fact_archive_index;
DROP INDEX IF EXISTS idx_agent_fact_archive_entities;
DROP INDEX IF EXISTS idx_agent_fact_archive_topics;
DROP INDEX IF EXISTS idx_agent_fact_archive_full_text;
DROP INDEX IF EXISTS idx_agent_fact_archive_status;
DROP INDEX IF EXISTS idx_agent_fact_archive_type;
DROP INDEX IF EXISTS idx_agent_fact_archive_session_turn;
DROP INDEX IF EXISTS idx_agent_fact_archive_user_kind;
DROP TABLE IF EXISTS agent_fact_archive_index;
