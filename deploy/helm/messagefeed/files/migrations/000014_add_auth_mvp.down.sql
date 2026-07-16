-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP TRIGGER IF EXISTS update_agent_approvals_updated_at ON agent_approvals;
DROP INDEX IF EXISTS idx_agent_approvals_user_status;
DROP TABLE IF EXISTS agent_approvals;

DROP INDEX IF EXISTS idx_auth_oauth_states_user_created;
DROP TABLE IF EXISTS auth_oauth_states;

DROP TRIGGER IF EXISTS update_user_sessions_updated_at ON user_sessions;
DROP INDEX IF EXISTS idx_user_sessions_user_active;
DROP TABLE IF EXISTS user_sessions;

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE users DROP COLUMN IF EXISTS status;
ALTER TABLE users DROP COLUMN IF EXISTS role;
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
