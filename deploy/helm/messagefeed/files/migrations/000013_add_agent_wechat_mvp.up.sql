CREATE TABLE IF NOT EXISTS external_accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    corp_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    external_user_id TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    binding_status VARCHAR(32) NOT NULL DEFAULT 'active',
    verified_at TIMESTAMP WITH TIME ZONE,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_external_accounts_binding_status CHECK (binding_status IN ('active', 'disabled')),
    CONSTRAINT uq_external_accounts_identity UNIQUE (provider, corp_id, agent_id, external_user_id)
);

CREATE INDEX IF NOT EXISTS idx_external_accounts_user
    ON external_accounts(user_id, provider);

DROP TRIGGER IF EXISTS update_external_accounts_updated_at ON external_accounts;
CREATE TRIGGER update_external_accounts_updated_at
    BEFORE UPDATE ON external_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_inbound_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_account_id BIGINT NOT NULL REFERENCES external_accounts(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    provider_message_id TEXT NOT NULL,
    corp_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    external_user_id TEXT NOT NULL DEFAULT '',
    chat_id TEXT NOT NULL DEFAULT '',
    chat_type VARCHAR(32) NOT NULL DEFAULT 'direct',
    msg_type VARCHAR(64) NOT NULL DEFAULT '',
    text_content TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'received',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_inbound_messages_status CHECK (status IN ('received', 'succeeded', 'failed')),
    CONSTRAINT uq_agent_inbound_messages_provider_message UNIQUE (provider, provider_message_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_inbound_messages_user_created
    ON agent_inbound_messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_inbound_messages_external_account
    ON agent_inbound_messages(external_account_id, created_at DESC);

DROP TRIGGER IF EXISTS update_agent_inbound_messages_updated_at ON agent_inbound_messages;
CREATE TRIGGER update_agent_inbound_messages_updated_at
    BEFORE UPDATE ON agent_inbound_messages
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_account_id BIGINT NOT NULL REFERENCES external_accounts(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    channel_session_key TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    title TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_sessions_status CHECK (status IN ('active', 'closed')),
    CONSTRAINT uq_agent_sessions_channel_key UNIQUE (provider, channel_session_key)
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_user_active
    ON agent_sessions(user_id, status, last_active_at DESC);

DROP TRIGGER IF EXISTS update_agent_sessions_updated_at ON agent_sessions;
CREATE TRIGGER update_agent_sessions_updated_at
    BEFORE UPDATE ON agent_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_turns (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    inbound_message_id BIGINT NOT NULL REFERENCES agent_inbound_messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    input_text TEXT NOT NULL DEFAULT '',
    output_text TEXT NOT NULL DEFAULT '',
    model_provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_turns_status CHECK (status IN ('running', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_agent_turns_session_created
    ON agent_turns(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_turns_inbound_message
    ON agent_turns(inbound_message_id);

DROP TRIGGER IF EXISTS update_agent_turns_updated_at ON agent_turns;
CREATE TRIGGER update_agent_turns_updated_at
    BEFORE UPDATE ON agent_turns
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_transcript_entries (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES agent_sessions(id) ON DELETE CASCADE,
    turn_id BIGINT NOT NULL REFERENCES agent_turns(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_transcript_entries_role CHECK (role IN ('user', 'assistant', 'system', 'tool'))
);

CREATE INDEX IF NOT EXISTS idx_agent_transcript_entries_session_created
    ON agent_transcript_entries(session_id, created_at ASC, id ASC);

CREATE TABLE IF NOT EXISTS agent_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_user_created
    ON agent_audit_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_turn
    ON agent_audit_logs(turn_id);
