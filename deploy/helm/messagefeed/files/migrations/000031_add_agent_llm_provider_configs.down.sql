-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP INDEX IF EXISTS idx_agent_llm_provider_configs_user_enabled;
DROP INDEX IF EXISTS idx_agent_llm_provider_configs_default;
DROP TABLE IF EXISTS agent_llm_provider_configs;
