package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
)

// createControllerRun 创建主 Agent 控制运行记录，并保存初始上下文。
func (s *AgentConversationService) createControllerRun(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (domain.AgentRun, error) {
	if s.runManager == nil {
		return domain.AgentRun{}, nil
	}
	run, err := s.runManager.CreateControllerRun(ctx, agent.CreateRunInput{
		SessionID: session.ID,
		TurnID:    turn.ID,
		TaskPacket: domain.AgentJSON{
			"provider":            input.Provider,
			"provider_message_id": input.ProviderMessageID,
			"inbound_message_id":  inbound.ID,
			"user_id":             account.UserID,
			"message_type":        input.MsgType,
			"message":             safeSummary(input.TextContent, 1000),
		},
		CapabilityScope: []string{"feed.query_recent_items", "source.query_latest_items", "content.summarize_text"},
		ModelKey:        "controller:" + llmModelKey(s.llmClient),
		ContextBudget: domain.AgentJSON{
			"max_reply_tokens": agentReplyMaxTokens,
			"mode":             "p0_controller_single_executor",
		},
		TraceID: input.TraceID,
	})
	if err != nil {
		return domain.AgentRun{}, err
	}
	if run.ID > 0 {
		_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "controller_input",
			ModelKey:  run.ModelKey,
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
				"context_budget":   run.ContextBudget,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.TextContent),
		})
	}
	return run, nil
}

func (s *AgentConversationService) recordControllerTrace(ctx context.Context, run domain.AgentRun, result agent.TurnRunResult, traceKind string) {
	if s.runManager == nil || run.ID == 0 {
		return
	}
	observations := agent.ObservationMetadata(result.Context.Observations)
	content := domain.AgentJSON{
		"reply":                  safeSummary(result.Reply, 2000),
		"model_provider":         result.ModelProvider,
		"model":                  result.Model,
		"context_blocks":         contextBlockMetadata(result.Context.Blocks),
		"context_messages":       contextMessageMetadata(result.Context.Messages),
		"context_semantic_units": contextSemanticUnitMetadata(result.Context.SemanticUnits),
		"context_budget_profile": string(result.Context.BudgetProfile),
		"context_budget_report":  contextBudgetReportMetadata(result.Context.BudgetReport),
		"observations":           observations,
		"history_need_hint":      string(result.Context.HistoryNeedHint),
		"redaction_policy":       "secret, token, webhook url and database dsn are excluded from trace content",
	}
	_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       traceKind,
		ModelKey:        run.ModelKey,
		Content:         content,
		RedactionStatus: "redacted",
		TokenEstimate:   controllerContextTraceTokenEstimate(result),
	})
}

func contextBlockMetadata(blocks []agent.ContextBlock) []domain.AgentJSON {
	output := make([]domain.AgentJSON, 0, len(blocks))
	for _, block := range blocks {
		output = append(output, domain.AgentJSON{
			"name":             block.Name,
			"capability_key":   block.CapabilityKey,
			"content":          safeSummary(block.Content, 2000),
			"item_count":       block.ItemCount,
			"truncated":        block.Truncated,
			"trust_level":      block.TrustLevel,
			"source":           block.Source,
			"evidence_refs":    append([]string(nil), block.EvidenceRefs...),
			"canonical_ref":    block.CanonicalRef,
			"token_estimate":   block.TokenEstimate,
			"retention_reason": block.RetentionReason,
			"omitted_reason":   block.OmittedReason,
			"generated_at":     formatOptionalTime(&block.GeneratedAt),
		})
	}
	return output
}

func contextMessageMetadata(messages []agent.ContextMessage) []domain.AgentJSON {
	output := make([]domain.AgentJSON, 0, len(messages))
	for _, message := range messages {
		output = append(output, domain.AgentJSON{
			"role":                string(message.Role),
			"content":             safeSummary(message.Content, 1000),
			"transcript_entry_id": message.TranscriptEntryID,
			"turn_id":             message.TurnID,
			"created_at":          formatOptionalTime(&message.CreatedAt),
		})
	}
	return output
}

func contextSemanticUnitMetadata(units []agent.ContextSemanticUnit) []domain.AgentJSON {
	output := make([]domain.AgentJSON, 0, len(units))
	for _, unit := range units {
		evidenceRefs := make([]domain.AgentJSON, 0, len(unit.EvidenceRefs))
		for _, ref := range unit.EvidenceRefs {
			evidenceRefs = append(evidenceRefs, domain.AgentJSON{
				"ref":           ref.Ref,
				"canonical_ref": ref.CanonicalRef,
				"source":        ref.Source,
			})
		}
		output = append(output, domain.AgentJSON{
			"id":               unit.ID,
			"type":             string(unit.Type),
			"source":           unit.Source,
			"content":          safeSummary(unit.Content, 1000),
			"message_count":    len(unit.Messages),
			"token_estimate":   unit.TokenEstimate,
			"protected":        unit.Protected,
			"selected":         unit.Selected,
			"projected":        unit.Projected,
			"retention_reason": unit.RetentionReason,
			"omitted_reason":   unit.OmittedReason,
			"canonical_ref":    unit.CanonicalRef,
			"evidence_refs":    evidenceRefs,
		})
	}
	return output
}

func contextBudgetReportMetadata(report agent.ContextBudgetReport) domain.AgentJSON {
	units := make([]domain.AgentJSON, 0, len(report.Units))
	for _, unit := range report.Units {
		units = append(units, domain.AgentJSON{
			"unit_id":          unit.UnitID,
			"unit_type":        string(unit.UnitType),
			"token_estimate":   unit.TokenEstimate,
			"selected":         unit.Selected,
			"protected":        unit.Protected,
			"projected":        unit.Projected,
			"retention_reason": unit.RetentionReason,
			"omitted_reason":   unit.OmittedReason,
		})
	}
	return domain.AgentJSON{
		"profile":                string(report.Profile),
		"total_budget_tokens":    report.TotalBudgetTokens,
		"recent_messages_tokens": report.RecentMessagesTokens,
		"output_reserve_tokens":  report.OutputReserveTokens,
		"safety_margin_tokens":   report.SafetyMarginTokens,
		"available_input_tokens": report.AvailableInputTokens,
		"used_tokens":            report.UsedTokens,
		"selected_unit_count":    report.SelectedUnitCount,
		"skipped_unit_count":     report.SkippedUnitCount,
		"protected_unit_count":   report.ProtectedUnitCount,
		"oversized_unit_count":   report.OversizedUnitCount,
		"selected_message_count": report.SelectedMessageCount,
		"skipped_message_count":  report.SkippedMessageCount,
		"units":                  units,
	}
}

func controllerContextTraceTokenEstimate(result agent.TurnRunResult) int {
	estimate := estimateTokenCount(result.Reply)
	if result.Context.BudgetReport.UsedTokens > 0 {
		estimate += result.Context.BudgetReport.UsedTokens
	}
	return estimate
}

func llmModelKey(client AgentConversationLLM) string {
	if client == nil {
		return "fallback"
	}
	return "configured"
}
