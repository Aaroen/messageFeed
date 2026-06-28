CREATE TABLE IF NOT EXISTS agent_llm_provider_configs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    api_key_ciphertext TEXT NOT NULL,
    api_key_hint TEXT NOT NULL DEFAULT '',
    protocol_mode TEXT NOT NULL DEFAULT 'auto',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    timeout_seconds INTEGER NOT NULL DEFAULT 600,
    max_retries INTEGER NOT NULL DEFAULT 6,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_llm_provider_configs_user_name_key UNIQUE (user_id, name),
    CONSTRAINT agent_llm_provider_configs_protocol_mode_check CHECK (protocol_mode IN ('auto', 'responses', 'chat_completions')),
    CONSTRAINT agent_llm_provider_configs_timeout_seconds_check CHECK (timeout_seconds BETWEEN 10 AND 3600),
    CONSTRAINT agent_llm_provider_configs_max_retries_check CHECK (max_retries BETWEEN 1 AND 50)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_llm_provider_configs_default
    ON agent_llm_provider_configs(user_id)
    WHERE is_default;

CREATE INDEX IF NOT EXISTS idx_agent_llm_provider_configs_user_enabled
    ON agent_llm_provider_configs(user_id, enabled, is_default);
