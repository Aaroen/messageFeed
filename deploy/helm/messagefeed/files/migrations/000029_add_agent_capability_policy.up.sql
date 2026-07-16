-- Helm 打包副本；源迁移位于项目 migrations 目录。
ALTER TABLE agent_notification_preferences
    ADD COLUMN IF NOT EXISTS capability_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb;
