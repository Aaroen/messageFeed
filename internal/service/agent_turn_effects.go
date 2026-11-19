package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"time"
)

type agentTurnEffectStore interface {
	AgentTurnWorkerRepository
	ClaimAgentTurnEffect(context.Context, string) (domain.AgentTurnEffect, error)
	FinishAgentTurnEffect(context.Context, domain.AgentTurnEffect, error) error
	AgentTurnUserTranscript(context.Context, int64, int64) (domain.AgentTranscriptEntry, error)
}

func (s *AgentConversationService) RunAgentTurnEffectOnce(ctx context.Context, operation string) error {
	store, ok := s.repository.(agentTurnEffectStore)
	if !ok {
		return fmt.Errorf("agent turn effects are not configured")
	}
	effect, err := store.ClaimAgentTurnEffect(ctx, operation)
	if err != nil || effect.TurnID == 0 {
		return err
	}
	workCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	workErr := s.applyAgentTurnEffect(workCtx, store, effect)
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	if err := store.FinishAgentTurnEffect(finishCtx, effect, workErr); err != nil {
		return errors.Join(err, workErr)
	}
	if workErr != nil {
		return fmt.Errorf("agent turn %d %s: %w", effect.TurnID, effect.Operation, workErr)
	}
	return nil
}

func (s *AgentConversationService) applyAgentTurnEffect(ctx context.Context, store agentTurnEffectStore, effect domain.AgentTurnEffect) error {
	turn, err := store.GetAgentTurn(ctx, effect.UserID, effect.TurnID)
	if err != nil {
		return err
	}
	inbound, err := store.GetInboundMessage(ctx, effect.UserID, turn.InboundMessageID)
	if err != nil {
		return err
	}
	if effect.Operation == "memory" {
		if effect.Attempt > 3 {
			return fmt.Errorf("memory capture lease expired after final attempt")
		}
		entry, err := store.AgentTurnUserTranscript(ctx, effect.UserID, effect.TurnID)
		if err != nil {
			return err
		}
		ctx, err = agentSelectionFromInbound(ctx, inbound)
		if err != nil {
			return err
		}
		return s.captureMemoryCandidateFromTranscript(ctx, entry)
	}
	if turn.DeliveryStatus == "succeeded" || turn.DeliveryStatus == "not_required" {
		return nil
	}
	if s.sender == nil {
		return fmt.Errorf("notification sender is unavailable")
	}
	var plan domain.AgentPlan
	if effect.PlanID > 0 {
		plan, err = store.GetAgentPlan(ctx, effect.UserID, effect.PlanID)
		if err != nil {
			return err
		}
	}
	input := agentInputFromInbound(inbound)
	if input.Provider == domain.AgentProviderWeb {
		account := domain.ExternalAccount{ID: inbound.ExternalAccountID, UserID: inbound.UserID, Provider: inbound.Provider}
		if err := s.sendWebAgentTaskFinalReport(ctx, account, inbound, domain.AgentSession{ID: turn.SessionID}, turn, plan, turn.OutputText, input); err != nil {
			return err
		}
		latest, err := store.GetAgentTurn(ctx, turn.UserID, turn.ID)
		if err == nil && latest.DeliveryStatus == "pending" {
			_, err = store.UpdateAgentTurnDelivery(ctx, turn.UserID, turn.ID, "not_required", "", s.now().UTC())
		}
		return err
	}
	kind := "final"
	if turn.Status == domain.AgentTurnStatusFailed {
		kind = "failure"
	}
	status, message := "not_required", ""
	if s.shouldSendWeChatWorkNotification(ctx, turn.UserID, input, kind) {
		if plan.TurnID == turn.ID {
			_, err = s.sendWeChatWorkFinalReportDelivery(ctx, input.ExternalUserID, plan, turn.OutputText, string(plan.Status))
		} else {
			_, _, err = s.sendWeChatWorkReply(ctx, input.ExternalUserID, turn.OutputText)
		}
		status = "succeeded"
		if err != nil {
			status, message = "failed", err.Error()
		}
	}
	_, writeErr := store.UpdateAgentTurnDelivery(ctx, effect.UserID, effect.TurnID, status, message, s.now().UTC())
	return errors.Join(err, writeErr)
}
