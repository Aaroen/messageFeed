package repository

import (
	"time"

	"gorm.io/gorm"
)

const agentUsableTranscriptCondition = `(agent_transcript_entries.role <> 'assistant' OR NOT EXISTS (
	SELECT 1 FROM agent_turns WHERE id = agent_transcript_entries.turn_id AND status = 'failed'))`

const agentUsableFactCondition = `NOT EXISTS (
	SELECT 1 FROM agent_transcript_entries JOIN agent_turns ON agent_turns.id = agent_transcript_entries.turn_id
	WHERE agent_transcript_entries.id = %[1]s.fact_id AND agent_transcript_entries.role = 'assistant'
	AND agent_turns.status = 'failed' AND %[1]s.fact_type = 'transcript')`

func reconcileFailedAgentTurns(tx *gorm.DB, now time.Time) error {
	return tx.Exec(`WITH target AS MATERIALIZED (
		SELECT t.id, t.user_id, t.session_id, t.inbound_message_id, t.cancel_requested,
		       COALESCE(NULLIF(t.error_message, ''), 'agent turn ended without a failure reason') AS reason
		FROM agent_turns t
		WHERE t.status = 'failed' AND (
			t.error_message = '' OR EXISTS (SELECT 1 FROM agent_inbound_messages i WHERE i.id = t.inbound_message_id AND i.status = 'received')
			OR EXISTS (SELECT 1 FROM agent_plans p WHERE p.turn_id = t.id AND p.status IN ('draft','awaiting_approval','approved','executing'))
			OR EXISTS (SELECT 1 FROM agent_runs r WHERE r.turn_id = t.id AND r.status IN ('running','input_required'))
		)
		ORDER BY t.id LIMIT 32 FOR UPDATE OF t SKIP LOCKED
	), failed_plans AS (
		UPDATE agent_plans p SET status = 'failed', error_message = t.reason, failed_at = @now
		FROM target t WHERE p.turn_id = t.id AND p.status IN ('draft','awaiting_approval','approved','executing')
		RETURNING p.id, t.reason
	), closed_steps AS (
		UPDATE agent_plan_steps s SET status = CASE WHEN s.status = 'executing' THEN 'failed' ELSE 'skipped' END,
			error_message = p.reason, completed_at = @now
		FROM failed_plans p WHERE s.plan_id = p.id AND s.status IN ('pending','approved','executing')
	), closed_runs AS (
		UPDATE agent_runs r SET status = CASE WHEN t.cancel_requested THEN 'canceled' ELSE 'failed' END,
			error_message = t.reason, completed_at = @now, updated_at = @now
		FROM target t WHERE r.turn_id = t.id AND r.status IN ('running','input_required')
	), closed_messages AS (
		UPDATE agent_inbound_messages i SET status = 'failed', updated_at = @now
		FROM target t WHERE i.id = t.inbound_message_id AND i.status = 'received'
	), normalized_reasons AS (
		UPDATE agent_turns a SET error_message = t.reason FROM target t WHERE a.id = t.id AND a.error_message = ''
	)
	INSERT INTO agent_audit_logs (turn_id, session_id, user_id, event_type, status, message, created_at)
	SELECT id, session_id, user_id, 'agent.turn_reconciled', 'reconciled', 'Agent terminal state reconciled', @now FROM target`, map[string]any{"now": now}).Error
}
