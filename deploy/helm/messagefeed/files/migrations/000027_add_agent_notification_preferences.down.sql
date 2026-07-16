-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP TRIGGER IF EXISTS update_agent_notification_preferences_updated_at ON agent_notification_preferences;
DROP TABLE IF EXISTS agent_notification_preferences;
