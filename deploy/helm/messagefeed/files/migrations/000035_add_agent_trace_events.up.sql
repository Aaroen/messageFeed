-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS agent_trace_events (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    plan_id BIGINT REFERENCES agent_plans(id) ON DELETE SET NULL,
    run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    parent_run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    step_id BIGINT REFERENCES agent_plan_steps(id) ON DELETE SET NULL,
    event_kind VARCHAR(48) NOT NULL,
    event_name TEXT NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    model_key TEXT NOT NULL DEFAULT '',
    capability_key TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    job_id TEXT NOT NULL DEFAULT '',
    artifact_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    input_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_trace_events_kind CHECK (
        event_kind IN (
            'inbound',
            'transcript',
            'context_projection',
            'planner',
            'subagent_dispatch',
            'tool_execution',
            'approval',
            'recall',
            'memory',
            'embedding',
            'llm',
            'notification',
            'worker',
            'recovery'
        )
    ),
    CONSTRAINT chk_agent_trace_events_status CHECK (
        status IN ('started', 'succeeded', 'failed', 'skipped', 'degraded')
    ),
    CONSTRAINT chk_agent_trace_events_duration CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_request
    ON agent_trace_events(request_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_trace
    ON agent_trace_events(trace_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_turn
    ON agent_trace_events(turn_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_plan
    ON agent_trace_events(plan_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_run
    ON agent_trace_events(run_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_agent_trace_events_kind_status
    ON agent_trace_events(event_kind, status, created_at DESC);
