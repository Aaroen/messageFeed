CREATE TABLE IF NOT EXISTS agent_turn_effects (
    turn_id BIGINT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
    operation TEXT NOT NULL CHECK (operation IN ('memory', 'delivery')),
    plan_id BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    attempt INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_until TIMESTAMPTZ,
    error_message TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (turn_id, operation)
);

CREATE INDEX IF NOT EXISTS idx_agent_turn_effects_queue ON agent_turn_effects (operation, available_at, turn_id)
    WHERE status IN ('queued', 'running');

ALTER TABLE agent_stream_events ADD COLUMN IF NOT EXISTS sequence BIGINT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_stream_events_run_sequence ON agent_stream_events (run_id, sequence);
ALTER TABLE agent_turns ADD COLUMN IF NOT EXISTS error_code TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_memory_candidates_capture_turn ON agent_memory_candidates (turn_id)
    WHERE metadata_json @> '{"transcript_capture":true}'::jsonb;
