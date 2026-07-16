-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS agent_notification_preferences (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    process_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    final_reports_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    failure_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    recovery_notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_agent_notification_preferences_updated_at ON agent_notification_preferences;
CREATE TRIGGER update_agent_notification_preferences_updated_at
    BEFORE UPDATE ON agent_notification_preferences
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
