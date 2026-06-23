DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_status'
    ) THEN
        ALTER TABLE users DROP CONSTRAINT chk_users_status;
    END IF;

    ALTER TABLE users
        ADD CONSTRAINT chk_users_status CHECK (status IN ('active', 'disabled', 'deleted'));
END $$;

UPDATE users
SET username = 'aroen_archived_' || id::text,
    updated_at = NOW()
WHERE id <> 1 AND username = 'aroen';

INSERT INTO users (id, username, email, display_name, password_hash, role, status, created_at, updated_at)
VALUES (
    1,
    'aroen',
    'aroen@messagefeed.local',
    'aroen',
    '$2a$10$DTKcuvnsad7405UJYtMIxOQDrpO6PN5bQJGgwgJDlJz8AIkcYicYO',
    'owner',
    'active',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO UPDATE
SET username = 'aroen',
    email = CASE WHEN users.email = '' OR users.email IS NULL OR users.email = 'default@messagefeed.local' THEN 'aroen@messagefeed.local' ELSE users.email END,
    display_name = 'aroen',
    password_hash = EXCLUDED.password_hash,
    role = 'owner',
    status = 'active',
    updated_at = NOW();

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
    language TEXT NOT NULL DEFAULT 'zh-CN',
    region TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    focus_topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    blocked_topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    market_focus JSONB NOT NULL DEFAULT '[]'::jsonb,
    instrument_focus JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_preference TEXT NOT NULL DEFAULT '',
    notification_quiet_hours TEXT NOT NULL DEFAULT '',
    agent_notes TEXT NOT NULL DEFAULT '',
    reply_style TEXT NOT NULL DEFAULT 'plain_text_short',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_user_profiles_updated_at ON user_profiles;
CREATE TRIGGER update_user_profiles_updated_at
    BEFORE UPDATE ON user_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

INSERT INTO user_profiles (user_id, timezone, language, reply_style)
VALUES (1, 'Asia/Shanghai', 'zh-CN', 'plain_text_short')
ON CONFLICT (user_id) DO UPDATE
SET timezone = CASE WHEN user_profiles.timezone = '' THEN EXCLUDED.timezone ELSE user_profiles.timezone END,
    language = CASE WHEN user_profiles.language = '' THEN EXCLUDED.language ELSE user_profiles.language END,
    reply_style = CASE WHEN user_profiles.reply_style = '' THEN EXCLUDED.reply_style ELSE user_profiles.reply_style END,
    updated_at = NOW();

UPDATE sources SET user_id = 1;
UPDATE user_item_states SET user_id = 1;
UPDATE feed_view_preferences SET user_id = 1;
UPDATE source_import_jobs SET user_id = 1;
UPDATE source_fetch_jobs SET user_id = 1;
UPDATE item_events SET user_id = 1;
UPDATE alert_rules SET user_id = 1;
UPDATE alert_candidates SET user_id = 1;
UPDATE ai_analysis_jobs SET user_id = 1;
UPDATE notification_jobs SET user_id = 1;
UPDATE notification_deliveries SET user_id = 1;
UPDATE external_accounts SET user_id = 1;
UPDATE agent_inbound_messages SET user_id = 1;
UPDATE agent_sessions SET user_id = 1;
UPDATE agent_turns SET user_id = 1;
UPDATE agent_transcript_entries SET user_id = 1;
UPDATE agent_audit_logs SET user_id = 1;
UPDATE agent_approvals SET user_id = 1;
UPDATE auth_invite_codes SET created_by_user_id = 1;
UPDATE auth_invite_redemptions SET user_id = 1;

UPDATE user_sessions
SET revoked_at = COALESCE(revoked_at, NOW()),
    updated_at = NOW();

UPDATE auth_oauth_states
SET consumed_at = COALESCE(consumed_at, NOW());
