-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS agent_plans (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    controller_run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    goal TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    impact_summary TEXT NOT NULL DEFAULT '',
    risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
    confirmation_policy VARCHAR(32) NOT NULL DEFAULT 'auto',
    allowed_scopes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    dedupe_key TEXT NOT NULL DEFAULT '',
    policy_decision VARCHAR(32) NOT NULL DEFAULT '',
    policy_reason TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMP WITH TIME ZONE,
    approved_at TIMESTAMP WITH TIME ZONE,
    rejected_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    failed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_plans_status CHECK (status IN ('draft', 'awaiting_approval', 'approved', 'rejected', 'expired', 'executing', 'completed', 'failed')),
    CONSTRAINT chk_agent_plans_risk CHECK (risk_level IN ('low', 'medium', 'high')),
    CONSTRAINT chk_agent_plans_confirmation CHECK (confirmation_policy IN ('auto', 'prompt', 'forbidden'))
);

CREATE INDEX IF NOT EXISTS idx_agent_plans_user_created
    ON agent_plans(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_plans_session_turn
    ON agent_plans(session_id, turn_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_plans_user_dedupe
    ON agent_plans(user_id, dedupe_key)
    WHERE dedupe_key <> '';

DROP TRIGGER IF EXISTS update_agent_plans_updated_at ON agent_plans;
CREATE TRIGGER update_agent_plans_updated_at
    BEFORE UPDATE ON agent_plans
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_plan_steps (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL REFERENCES agent_plans(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    capability_key TEXT NOT NULL DEFAULT '',
    capability_scope_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    title TEXT NOT NULL DEFAULT '',
    input_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    expected_output TEXT NOT NULL DEFAULT '',
    failure_strategy TEXT NOT NULL DEFAULT '',
    executor_run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    observation_ref TEXT NOT NULL DEFAULT '',
    artifact_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_plan_steps_status CHECK (status IN ('pending', 'approved', 'executing', 'completed', 'failed', 'skipped')),
    CONSTRAINT uq_agent_plan_steps_order UNIQUE (plan_id, step_order)
);

CREATE INDEX IF NOT EXISTS idx_agent_plan_steps_plan
    ON agent_plan_steps(plan_id, step_order ASC);

DROP TRIGGER IF EXISTS update_agent_plan_steps_updated_at ON agent_plan_steps;
CREATE TRIGGER update_agent_plan_steps_updated_at
    BEFORE UPDATE ON agent_plan_steps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_capability_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    plan_id BIGINT REFERENCES agent_plans(id) ON DELETE SET NULL,
    plan_step_id BIGINT REFERENCES agent_plan_steps(id) ON DELETE SET NULL,
    capability_key TEXT NOT NULL DEFAULT '',
    decision VARCHAR(32) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    input_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_capability_audit_user_created
    ON agent_capability_audit_logs(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_capability_audit_plan_step
    ON agent_capability_audit_logs(plan_id, plan_step_id, created_at ASC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_agent_approvals_plan'
    ) THEN
        ALTER TABLE agent_approvals
            ADD CONSTRAINT fk_agent_approvals_plan
            FOREIGN KEY (plan_id) REFERENCES agent_plans(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_agent_approvals_plan_status
    ON agent_approvals(plan_id, status)
    WHERE plan_id IS NOT NULL;
