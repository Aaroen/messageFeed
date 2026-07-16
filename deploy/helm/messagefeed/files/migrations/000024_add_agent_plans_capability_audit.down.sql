-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP INDEX IF EXISTS idx_agent_approvals_plan_status;

ALTER TABLE agent_approvals
    DROP CONSTRAINT IF EXISTS fk_agent_approvals_plan;

DROP INDEX IF EXISTS idx_agent_capability_audit_plan_step;
DROP INDEX IF EXISTS idx_agent_capability_audit_user_created;
DROP TABLE IF EXISTS agent_capability_audit_logs;

DROP TRIGGER IF EXISTS update_agent_plan_steps_updated_at ON agent_plan_steps;
DROP INDEX IF EXISTS idx_agent_plan_steps_plan;
DROP TABLE IF EXISTS agent_plan_steps;

DROP TRIGGER IF EXISTS update_agent_plans_updated_at ON agent_plans;
DROP INDEX IF EXISTS idx_agent_plans_user_dedupe;
DROP INDEX IF EXISTS idx_agent_plans_session_turn;
DROP INDEX IF EXISTS idx_agent_plans_user_created;
DROP TABLE IF EXISTS agent_plans;
