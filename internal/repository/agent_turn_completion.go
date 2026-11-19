package repository

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AgentRepository) CommitAgentTurnResult(ctx context.Context, completion domain.AgentTurnCompletion) (domain.AgentTurnCompletionResult, error) {
	turn := completion.Turn
	if turn.ID <= 0 || turn.UserID <= 0 || turn.SessionID <= 0 || completion.CompletedAt.IsZero() ||
		(turn.Status != domain.AgentTurnStatusSucceeded && turn.Status != domain.AgentTurnStatusFailed) ||
		(turn.Status == domain.AgentTurnStatusSucceeded && strings.TrimSpace(turn.OutputText) == "") {
		return domain.AgentTurnCompletionResult{}, domain.ErrInvalidInput
	}
	now := completion.CompletedAt.UTC()
	var result domain.AgentTurnCompletionResult
	err := r.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		store := NewAgentRepository(transaction)
		var current agentTurnModel
		if err := transaction.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND session_id = ?", turn.ID, turn.UserID, turn.SessionID).First(&current).Error; err != nil {
			return mapRepositoryError(err)
		}
		workerID := strings.TrimSpace(completion.WorkerID)
		if current.Status != string(domain.AgentTurnStatusRunning) && current.Status != string(domain.AgentTurnStatusQueued) {
			return domain.NewAppError(domain.ErrorKindConflict, "agent_turn_already_finished", "agent turn is already terminal", "repository.agent.turn.commit", false, nil)
		}
		if current.LockedBy != workerID || current.AttemptCount != turn.AttemptCount || workerID != "" && (current.LeaseUntil == nil || !current.LeaseUntil.After(now)) {
			return domain.NewAppError(domain.ErrorKindConflict, "agent_turn_ownership_lost", "agent turn worker ownership or lease is no longer valid", "repository.agent.turn.commit", false, nil)
		}
		if current.CancelRequested && turn.Status == domain.AgentTurnStatusSucceeded {
			return domain.NewAppError(domain.ErrorKindConflict, "agent_turn_canceled", "agent turn cancellation was requested", "repository.agent.turn.commit", false, nil)
		}
		turn.InboundMessageID = current.InboundMessageID
		turn.CancelRequested = current.CancelRequested
		turn.CancelReason = current.CancelReason
		if current.CancelRequested {
			turn.ErrorMessage = current.CancelReason
			turn.ErrorCode = "agent_turn_canceled"
			turn.OutputText = ""
			turn.DeliveryStatus = "not_required"
		}
		turn.FinishedAt = &now
		updates := map[string]any{
			"status": turn.Status, "output_text": turn.OutputText, "model_provider": turn.ModelProvider,
			"model": turn.Model, "error_message": turn.ErrorMessage, "finished_at": now, "updated_at": now,
			"error_code":      turn.ErrorCode,
			"delivery_status": turn.DeliveryStatus, "delivery_error": turn.DeliveryError,
			"locked_by": "", "locked_at": nil, "lease_until": nil,
		}
		if err := transaction.Model(&current).Updates(updates).Error; err != nil {
			return mapRepositoryError(err)
		}
		if completion.Plan.ID > 0 {
			plan, err := store.commitAgentTurnPlan(ctx, completion.Plan, turn, now)
			if err != nil {
				return err
			}
			result.Plan = plan
		}
		if turn.Status == domain.AgentTurnStatusFailed {
			status := domain.AgentRunStatusFailed
			if current.CancelRequested {
				status = domain.AgentRunStatusCanceled
			}
			if err := transaction.Model(&agentRunModel{}).
				Where("session_id = ? AND turn_id = ? AND status IN ?", turn.SessionID, turn.ID, []domain.AgentRunStatus{domain.AgentRunStatusRunning, domain.AgentRunStatusInputRequired}).
				Updates(map[string]any{"status": status, "result_ref": fmt.Sprintf("turn:%d", turn.ID), "error_message": turn.ErrorMessage, "completed_at": now, "updated_at": now}).Error; err != nil {
				return mapRepositoryError(err)
			}
		} else if completion.ControllerRunID > 0 {
			status := domain.AgentRunStatusSucceeded
			update := transaction.Model(&agentRunModel{}).
				Where("id = ? AND session_id = ? AND turn_id = ? AND role = ? AND status = ?", completion.ControllerRunID, turn.SessionID, turn.ID, domain.AgentRunRoleController, domain.AgentRunStatusRunning).
				Updates(map[string]any{"status": status, "result_ref": fmt.Sprintf("turn:%d", turn.ID), "error_message": turn.ErrorMessage, "completed_at": now, "updated_at": now})
			if update.Error != nil {
				return mapRepositoryError(update.Error)
			}
			if update.RowsAffected != 1 {
				return domain.ErrNotFound
			}
		}
		if turn.Status == domain.AgentTurnStatusSucceeded && strings.TrimSpace(turn.OutputText) != "" {
			entry := agentTranscriptEntryModelFromDomain(normalizeTranscriptEntry(domain.AgentTranscriptEntry{
				UserID: turn.UserID, SessionID: turn.SessionID, TurnID: turn.ID,
				Role: domain.AgentTranscriptRoleAssistant, Content: turn.OutputText,
				Metadata: completion.TranscriptMetadata, CreatedAt: now,
			}))
			if err := transaction.Create(&entry).Error; err != nil {
				return mapRepositoryError(err)
			}
			persisted := agentTranscriptEntryModelToDomain(entry)
			if err := store.ensureTranscriptArchiveIndex(ctx, persisted); err != nil {
				return mapRepositoryError(err)
			}
			if fact, valid := newAgentFactIndexBuilder(func() time.Time { return now }).BuildTranscript(persisted); valid {
				if _, err := store.UpsertAgentFactArchiveIndex(ctx, fact); err != nil {
					return err
				}
			}
		}
		inboundStatus := domain.AgentInboundMessageStatusSucceeded
		if turn.Status == domain.AgentTurnStatusFailed {
			inboundStatus = domain.AgentInboundMessageStatusFailed
		}
		inbound, err := store.UpdateInboundMessageStatus(ctx, turn.UserID, turn.InboundMessageID, inboundStatus, now)
		if err != nil {
			return err
		}
		result.InboundMessage = inbound
		if err := enqueueAgentTurnEffects(transaction, completion, turn); err != nil {
			return mapRepositoryError(err)
		}
		result.Turn, err = store.GetAgentTurn(ctx, turn.UserID, turn.ID)
		return err
	})
	if err != nil {
		return domain.AgentTurnCompletionResult{}, err
	}
	return result, nil
}

func (r *AgentRepository) commitAgentTurnPlan(ctx context.Context, plan domain.AgentPlan, turn domain.AgentTurn, now time.Time) (domain.AgentPlan, error) {
	if plan.UserID != turn.UserID || plan.SessionID != turn.SessionID || plan.TurnID != turn.ID ||
		(plan.Status != domain.AgentPlanStatusCompleted && plan.Status != domain.AgentPlanStatusFailed) ||
		(turn.Status == domain.AgentTurnStatusSucceeded) != (plan.Status == domain.AgentPlanStatusCompleted) {
		return domain.AgentPlan{}, domain.ErrInvalidInput
	}
	var current agentPlanModel
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND session_id = ? AND turn_id = ?", plan.ID, turn.UserID, turn.SessionID, turn.ID).First(&current).Error; err != nil {
		return domain.AgentPlan{}, mapRepositoryError(err)
	}
	if current.Status == string(domain.AgentPlanStatusFailed) || current.Status == string(domain.AgentPlanStatusCompleted) || current.Status == string(domain.AgentPlanStatusRejected) {
		return domain.AgentPlan{}, domain.NewAppError(domain.ErrorKindConflict, "agent_plan_already_finished", "agent plan is already terminal", "repository.agent.turn.commit", false, nil)
	}
	metadata, err := repositoryJSONBExpr(plan.Metadata)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	updates := map[string]any{"status": plan.Status, "metadata_json": metadata, "error_message": plan.ErrorMessage, "updated_at": now}
	if plan.Status == domain.AgentPlanStatusCompleted {
		updates["completed_at"] = now
	} else {
		updates["failed_at"] = now
	}
	if err := r.db.WithContext(ctx).Model(&current).Updates(updates).Error; err != nil {
		return domain.AgentPlan{}, mapRepositoryError(err)
	}
	builder := newAgentFactIndexBuilder(func() time.Time { return now })
	for _, step := range plan.Steps {
		if step.PlanID != plan.ID || step.Status != domain.AgentPlanStepStatusCompleted && step.Status != domain.AgentPlanStepStatusSkipped && step.Status != domain.AgentPlanStepStatusFailed {
			return domain.AgentPlan{}, domain.ErrInvalidInput
		}
		if _, err := r.UpdateAgentPlanStepStatus(ctx, turn.UserID, step); err != nil {
			return domain.AgentPlan{}, err
		}
		if fact, valid := builder.BuildPlanStep(agentFactRunScope{UserID: turn.UserID, SessionID: turn.SessionID, TurnID: turn.ID}, step); valid {
			if _, err := r.UpsertAgentFactArchiveIndex(ctx, fact); err != nil {
				return domain.AgentPlan{}, err
			}
		}
	}
	stored, err := r.GetAgentPlan(ctx, turn.UserID, plan.ID)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	if fact, valid := builder.BuildPlan(stored); valid {
		if _, err := r.UpsertAgentFactArchiveIndex(ctx, fact); err != nil {
			return domain.AgentPlan{}, err
		}
	}
	return stored, nil
}

func (r *AgentRepository) UpdateAgentTurnDelivery(ctx context.Context, userID int64, turnID int64, status string, errorMessage string, now time.Time) (domain.AgentTurn, error) {
	if status != "pending" && status != "succeeded" && status != "failed" && status != "not_required" {
		return domain.AgentTurn{}, domain.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&agentTurnModel{}).Where("id = ? AND user_id = ? AND status IN ?", turnID, userID, []domain.AgentTurnStatus{domain.AgentTurnStatusSucceeded, domain.AgentTurnStatusFailed}).
		Updates(map[string]any{"delivery_status": status, "delivery_error": strings.TrimSpace(errorMessage), "updated_at": now.UTC()})
	if result.Error != nil {
		return domain.AgentTurn{}, mapRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.AgentTurn{}, domain.ErrNotFound
	}
	return r.GetAgentTurn(ctx, userID, turnID)
}
