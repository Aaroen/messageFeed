-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP INDEX IF EXISTS idx_agent_audit_logs_turn;
DROP INDEX IF EXISTS idx_agent_audit_logs_user_created;
DROP TABLE IF EXISTS agent_audit_logs;

DROP INDEX IF EXISTS idx_agent_transcript_entries_session_created;
DROP TABLE IF EXISTS agent_transcript_entries;

DROP TRIGGER IF EXISTS update_agent_turns_updated_at ON agent_turns;
DROP INDEX IF EXISTS idx_agent_turns_inbound_message;
DROP INDEX IF EXISTS idx_agent_turns_session_created;
DROP TABLE IF EXISTS agent_turns;

DROP TRIGGER IF EXISTS update_agent_sessions_updated_at ON agent_sessions;
DROP INDEX IF EXISTS idx_agent_sessions_user_active;
DROP TABLE IF EXISTS agent_sessions;

DROP TRIGGER IF EXISTS update_agent_inbound_messages_updated_at ON agent_inbound_messages;
DROP INDEX IF EXISTS idx_agent_inbound_messages_external_account;
DROP INDEX IF EXISTS idx_agent_inbound_messages_user_created;
DROP TABLE IF EXISTS agent_inbound_messages;

DROP TRIGGER IF EXISTS update_external_accounts_updated_at ON external_accounts;
DROP INDEX IF EXISTS idx_external_accounts_user;
DROP TABLE IF EXISTS external_accounts;
