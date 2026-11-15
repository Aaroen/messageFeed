package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strings"
)

func (s *AgentConversationService) buildAgentConversationContext(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	profile agent.ContextBudgetProfile,
	plan *domain.AgentPlan,
) (agent.ContextSnapshot, error) {
	if s == nil || s.contextBuilder == nil {
		return agent.ContextSnapshot{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_context_unavailable", "Agent 上下文构建器不可用。", "service.agent.context", true, nil)
	}
	buildInput := agent.ContextBuildInput{
		UserID:         account.UserID,
		SessionID:      session.ID,
		TurnID:         turn.ID,
		CapabilityKeys: []string{},
		MessageText:    input.TextContent,
		MessageType:    input.MsgType,
		RequestID:      input.RequestID,
		TraceID:        input.TraceID,
		BudgetProfile:  profile,
	}
	if plan != nil {
		buildInput.ActiveGoal = plan.Goal
		buildInput.ActivePlan = activePlanContextBlock(*plan)
	}
	return s.contextBuilder.Build(ctx, buildInput)
}

func agentConversationContextProjection(snapshot agent.ContextSnapshot) domain.AgentJSON {
	units := make([]domain.AgentJSON, 0, len(snapshot.SemanticUnits))
	for _, unit := range snapshot.SemanticUnits {
		projection := mainAgentSemanticUnitProjection(unit)
		delete(projection, "content")
		units = append(units, projection)
	}
	current := domain.AgentJSON{}
	if message := snapshot.Bundle.CurrentMessage; message != nil {
		current = domain.AgentJSON{"role": string(message.Role), "content": message.Content}
	}
	return domain.AgentJSON{
		"budget_profile":          string(snapshot.BudgetProfile),
		"projection_policy":       "近期对话按完整语义单元选择，message_refs 指向原始记录；计划约束与当前用户要求冲突时，以当前用户要求为准。",
		"current_message":         current,
		"candidate_message_count": snapshot.BudgetReport.SelectedMessageCount + snapshot.BudgetReport.SkippedMessageCount,
		"candidate_query_limit":   agent.RecentConversationCandidateLimit(snapshot.BudgetProfile),
		"recent_messages":         mainAgentContextMessageProjection(snapshot.Bundle.RecentMessages),
		"active_goal":             snapshot.Bundle.ActiveGoal,
		"active_plan":             snapshot.Bundle.ActivePlan,
		"user_constraints":        snapshot.Bundle.UserConstraints,
		"memory_blocks":           snapshot.Bundle.MemoryBlocks,
		"semantic_units":          units,
		"context_budget_report":   contextBudgetReportMetadata(snapshot.BudgetReport),
	}
}

func (s *AgentConversationService) agentPlanSourceMessages(ctx context.Context, userID int64, plan domain.AgentPlan) ([]agent.ContextMessage, error) {
	if plan.TurnID == 0 {
		return nil, nil
	}
	entries, err := s.repository.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:    plan.SessionID,
		UserID:       userID,
		BeforeTurnID: plan.TurnID + 1,
		Roles:        []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant},
		Limit:        2,
	})
	if err != nil {
		return nil, err
	}
	source := make([]domain.AgentTranscriptEntry, 0, len(entries))
	hasAnswer := false
	for _, entry := range entries {
		if entry.TurnID != plan.TurnID {
			continue
		}
		source = append(source, entry)
		if entry.Role == domain.AgentTranscriptRoleAssistant && strings.TrimSpace(entry.Content) != "" {
			hasAnswer = true
		}
	}
	if plan.Status == domain.AgentPlanStatusCompleted && !hasAnswer {
		return nil, domain.NewAppError(domain.ErrorKindUnavailable, "agent_source_answer_missing", "已完成计划缺少原始回答记录。", "service.agent.context", true, nil)
	}
	return transcriptEntriesToContextMessages(source), nil
}

func agentPlanArtifactFacts(userID int64, plan domain.AgentPlan) []domain.AgentFactArchiveIndex {
	facts := []domain.AgentFactArchiveIndex{}
	seen := map[string]struct{}{}
	for _, step := range plan.Steps {
		for _, ref := range step.ArtifactRefs {
			parsed, ok := agent.ParseCanonicalFactRef(ref)
			if !ok || parsed.FactType != string(domain.AgentFactTypeArtifact) {
				continue
			}
			if _, exists := seen[parsed.CanonicalRef]; exists {
				continue
			}
			seen[parsed.CanonicalRef] = struct{}{}
			facts = append(facts, domain.AgentFactArchiveIndex{
				CanonicalRef: parsed.CanonicalRef,
				FactType:     domain.AgentFactTypeArtifact,
				FactID:       parsed.FactID,
				UserID:       userID,
				SessionID:    plan.SessionID,
				TurnID:       plan.TurnID,
			})
		}
	}
	return facts
}
