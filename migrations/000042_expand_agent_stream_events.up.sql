CREATE TABLE IF NOT EXISTS agent_stream_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    session_id BIGINT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn_id BIGINT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
    run_id BIGINT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    item_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    delta TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS agent_stream_events_session_cursor_idx ON agent_stream_events (user_id, session_id, id);
CREATE INDEX IF NOT EXISTS agent_stream_events_turn_idx ON agent_stream_events (turn_id, id);
