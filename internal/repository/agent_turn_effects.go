package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

func enqueueAgentTurnEffects(tx *gorm.DB, completion domain.AgentTurnCompletion, turn domain.AgentTurn) error {
	if !completion.QueueEffects || turn.CancelRequested {
		return nil
	}
	return tx.Exec(`INSERT INTO agent_turn_effects (turn_id, operation, plan_id)
		SELECT @id, operation, CASE WHEN operation = 'delivery' THEN @plan ELSE 0 END
		FROM (VALUES ('memory'), ('delivery')) AS effects(operation)
		WHERE operation = 'memory' OR @delivery
		ON CONFLICT (turn_id, operation) DO NOTHING`, map[string]any{
		"id": turn.ID, "plan": completion.DeliveryPlanID, "delivery": turn.DeliveryStatus == "pending",
	}).Error
}

func (r *AgentRepository) ClaimAgentTurnEffect(ctx context.Context, operation string) (domain.AgentTurnEffect, error) {
	if operation == "delivery" {
		if err := r.db.WithContext(ctx).Exec(`WITH expired AS (
			UPDATE agent_turn_effects SET status = 'failed', lease_until = NULL, error_message = '通知发送结果未知，回答已保存'
			WHERE operation = 'delivery' AND status = 'running' AND lease_until <= NOW()
			RETURNING turn_id, error_message
		)
		UPDATE agent_turns t SET delivery_status = 'failed', delivery_error = e.error_message
		FROM expired e WHERE t.id = e.turn_id AND t.delivery_status = 'pending'`).Error; err != nil {
			return domain.AgentTurnEffect{}, mapRepositoryError(err)
		}
	}
	var effect domain.AgentTurnEffect
	err := r.db.WithContext(ctx).Raw(`WITH candidate AS (
		SELECT e.turn_id, e.operation FROM agent_turn_effects e
		WHERE e.operation = ? AND (e.status = 'queued' AND e.available_at <= NOW()
			OR e.operation = 'memory' AND e.status = 'running' AND e.lease_until <= NOW())
		ORDER BY e.available_at, e.turn_id LIMIT 1 FOR UPDATE SKIP LOCKED
	)
	UPDATE agent_turn_effects e SET status = 'running', attempt = e.attempt + 1, lease_until = NOW() + INTERVAL '2 minutes'
	FROM candidate c, agent_turns t
	WHERE e.turn_id = c.turn_id AND e.operation = c.operation AND t.id = e.turn_id
	RETURNING e.turn_id, t.user_id, e.operation, e.plan_id, e.attempt`, operation).Scan(&effect).Error
	if err != nil {
		return domain.AgentTurnEffect{}, mapRepositoryError(err)
	}
	return effect, nil
}

func (r *AgentRepository) AgentTurnUserTranscript(ctx context.Context, userID, turnID int64) (domain.AgentTranscriptEntry, error) {
	var model agentTranscriptEntryModel
	if err := r.db.WithContext(ctx).Where("user_id = ? AND turn_id = ? AND role = ?", userID, turnID, domain.AgentTranscriptRoleUser).First(&model).Error; err != nil {
		return domain.AgentTranscriptEntry{}, mapRepositoryError(err)
	}
	return agentTranscriptEntryModelToDomain(model), nil
}

func (r *AgentRepository) FinishAgentTurnEffect(ctx context.Context, effect domain.AgentTurnEffect, cause error) error {
	status, message, delay := "succeeded", "", time.Duration(0)
	if cause != nil {
		status, message = "failed", cause.Error()
		if effect.Operation == "memory" && effect.Attempt < 3 {
			status, delay = "queued", time.Duration(effect.Attempt)*10*time.Second
		}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`UPDATE agent_turn_effects SET status = ?, error_message = ?,
			available_at = NOW() + make_interval(secs => ?), lease_until = NULL
			WHERE turn_id = ? AND operation = ? AND attempt = ? AND status = 'running'`,
			status, message, delay.Seconds(), effect.TurnID, effect.Operation, effect.Attempt)
		if result.Error != nil {
			return mapRepositoryError(result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if effect.Operation == "delivery" && cause != nil {
			return tx.Model(&agentTurnModel{}).Where("id = ? AND delivery_status = 'pending'", effect.TurnID).
				Updates(map[string]any{"delivery_status": "failed", "delivery_error": message}).Error
		}
		return nil
	})
}
