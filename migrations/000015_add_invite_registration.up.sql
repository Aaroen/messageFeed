ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS auth_invite_codes (
    id BIGSERIAL PRIMARY KEY,
    code_hash TEXT NOT NULL UNIQUE,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL DEFAULT 'user',
    max_uses INTEGER NOT NULL DEFAULT 1,
    use_count INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_auth_invite_codes_role CHECK (role IN ('owner', 'user')),
    CONSTRAINT chk_auth_invite_codes_status CHECK (status IN ('active', 'revoked')),
    CONSTRAINT chk_auth_invite_codes_max_uses CHECK (max_uses = 1),
    CONSTRAINT chk_auth_invite_codes_use_count CHECK (use_count >= 0 AND use_count <= max_uses)
);

CREATE INDEX IF NOT EXISTS idx_auth_invite_codes_created_by
    ON auth_invite_codes(created_by_user_id, created_at DESC);

DROP TRIGGER IF EXISTS update_auth_invite_codes_updated_at ON auth_invite_codes;
CREATE TRIGGER update_auth_invite_codes_updated_at
    BEFORE UPDATE ON auth_invite_codes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS auth_invite_redemptions (
    id BIGSERIAL PRIMARY KEY,
    invite_code_id BIGINT NOT NULL REFERENCES auth_invite_codes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redeemed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent_hash TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_auth_invite_redemptions_invite
    ON auth_invite_redemptions(invite_code_id, redeemed_at DESC);
