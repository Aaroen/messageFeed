-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP TRIGGER IF EXISTS update_user_profiles_updated_at ON user_profiles;
DROP TABLE IF EXISTS user_profiles;

UPDATE users
SET status = 'disabled',
    updated_at = NOW()
WHERE status = 'deleted';

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_status;
ALTER TABLE users
    ADD CONSTRAINT chk_users_status CHECK (status IN ('active', 'disabled'));
