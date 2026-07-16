-- Helm 打包副本；源迁移位于项目 migrations 目录。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS role VARCHAR(32) NOT NULL DEFAULT 'owner',
    ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'active';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_role'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT chk_users_role CHECK (role IN ('owner', 'user'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_status'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT chk_users_status CHECK (status IN ('active', 'disabled'));
    END IF;
END $$;

UPDATE users
SET
    display_name = CASE WHEN display_name = '' THEN username ELSE display_name END,
    role = CASE WHEN role = '' THEN 'owner' ELSE role END,
    status = CASE WHEN status = '' THEN 'active' ELSE status END
WHERE id = 1;

CREATE TABLE IF NOT EXISTS user_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    user_agent_hash TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active
    ON user_sessions(user_id, expires_at DESC)
    WHERE revoked_at IS NULL;

DROP TRIGGER IF EXISTS update_user_sessions_updated_at ON user_sessions;
CREATE TRIGGER update_user_sessions_updated_at
    BEFORE UPDATE ON user_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS auth_oauth_states (
    id BIGSERIAL PRIMARY KEY,
    state_hash TEXT NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    purpose VARCHAR(32) NOT NULL,
    redirect_path TEXT NOT NULL DEFAULT '/',
    corp_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    consumed_at TIMESTAMP WITH TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_auth_oauth_states_purpose CHECK (purpose IN ('bind', 'confirm'))
);

CREATE INDEX IF NOT EXISTS idx_auth_oauth_states_user_created
    ON auth_oauth_states(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_approvals (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_account_id BIGINT REFERENCES external_accounts(id) ON DELETE SET NULL,
    approval_token_hash TEXT NOT NULL UNIQUE,
    channel VARCHAR(64) NOT NULL DEFAULT 'web',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    decided_at TIMESTAMP WITH TIME ZONE,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_approvals_status CHECK (status IN ('pending', 'approved', 'rejected', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_agent_approvals_user_status
    ON agent_approvals(user_id, status, expires_at DESC);

DROP TRIGGER IF EXISTS update_agent_approvals_updated_at ON agent_approvals;
CREATE TRIGGER update_agent_approvals_updated_at
    BEFORE UPDATE ON agent_approvals
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
