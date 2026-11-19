DROP INDEX IF EXISTS idx_agent_stream_events_run_sequence;
ALTER TABLE agent_stream_events DROP COLUMN IF EXISTS sequence;
DROP TABLE IF EXISTS agent_turn_effects;
ALTER TABLE agent_turns DROP COLUMN IF EXISTS error_code;
DROP INDEX IF EXISTS idx_agent_memory_candidates_capture_turn;
