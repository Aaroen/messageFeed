ALTER TABLE agent_guest_model_requests
    ADD COLUMN IF NOT EXISTS conversation_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS input_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS output_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reasoning_content TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'succeeded',
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS agent_guest_history_client_conversation_idx
    ON agent_guest_model_requests (client_hash, conversation_id, created_at);

CREATE INDEX IF NOT EXISTS agent_guest_history_expiration_idx
    ON agent_guest_model_requests (created_at);
