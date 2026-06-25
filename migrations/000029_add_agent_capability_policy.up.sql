ALTER TABLE agent_notification_preferences
    ADD COLUMN IF NOT EXISTS capability_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb;
