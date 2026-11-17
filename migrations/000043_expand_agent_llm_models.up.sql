CREATE TABLE IF NOT EXISTS agent_llm_models (
    config_id BIGINT NOT NULL REFERENCES agent_llm_provider_configs(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    guest_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (config_id, model)
);

INSERT INTO agent_llm_models (config_id, model)
SELECT id, model FROM agent_llm_provider_configs WHERE model <> ''
ON CONFLICT (config_id, model) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_guest_model_requests (
    id BIGSERIAL PRIMARY KEY,
    client_hash TEXT NOT NULL,
    config_id BIGINT NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS agent_guest_model_requests_client_time_idx ON agent_guest_model_requests (client_hash, created_at);
