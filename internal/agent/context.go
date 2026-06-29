package agent

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type HistoryNeedHint string

const (
	HistoryNeedNone     HistoryNeedHint = "none"
	HistoryNeedPossible HistoryNeedHint = "possible"
	HistoryNeedRequired HistoryNeedHint = "required"
)

type UserContextProvider interface {
	BuildUserContextBlock(ctx context.Context, userID int64) (ContextBlock, error)
}

type ConversationMemoryProvider interface {
	BuildConversationMemory(ctx context.Context, input ContextBuildInput) (ConversationMemory, error)
}

type ConversationMemory struct {
	Messages             []ContextMessage
	HistoryNeedHint      HistoryNeedHint
	HistoryQueried       bool
	HistoryResults       []ContextMessage
	HistoryResultContent string
}

type CapabilityExecutor interface {
	Execute(ctx context.Context, input CapabilityExecuteInput) (CapabilityExecuteResult, error)
}

type CapabilityExecuteInput struct {
	Capability      Capability
	UserID          int64
	SessionID       int64
	TurnID          int64
	ControllerRunID int64
	Message         string
	RawArguments    string
}

type CapabilityExecuteResult struct {
	Blocks      []ContextBlock
	Observation CapabilityObservation
}

type DefaultContextBuilder struct {
	registry            *CapabilityRegistry
	policy              *PolicyEngine
	userContextProvider UserContextProvider
	conversationMemory  ConversationMemoryProvider
	executor            CapabilityExecutor
	capabilityKeys      []string
	now                 func() time.Time
}

type DefaultContextBuilderOptions struct {
	Registry            *CapabilityRegistry
	Policy              *PolicyEngine
	UserContextProvider UserContextProvider
	ConversationMemory  ConversationMemoryProvider
	Executor            CapabilityExecutor
	CapabilityKeys      []string
	Now                 func() time.Time
}

func NewDefaultContextBuilder(options DefaultContextBuilderOptions) *DefaultContextBuilder {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	registry := options.Registry
	if registry == nil {
		registry = NewP0CapabilityRegistry()
	}
	policy := options.Policy
	if policy == nil {
		policy = NewPolicyEngine()
	}
	capabilityKeys := append([]string(nil), options.CapabilityKeys...)
	if len(capabilityKeys) == 0 {
		capabilityKeys = []string{"feed.query_recent_items", "source.query_latest_items"}
	}
	return &DefaultContextBuilder{
		registry:            registry,
		policy:              policy,
		userContextProvider: options.UserContextProvider,
		conversationMemory:  options.ConversationMemory,
		executor:            options.Executor,
		capabilityKeys:      capabilityKeys,
		now:                 now,
	}
}

func (b *DefaultContextBuilder) Build(ctx context.Context, input ContextBuildInput) (ContextSnapshot, error) {
	if b == nil {
		return ContextSnapshot{}, nil
	}
	capabilityKeys := b.capabilityKeysForInput(input)
	budgetSpec := ContextBudgetForProfile(input.BudgetProfile)
	input.BudgetProfile = budgetSpec.Profile
	snapshot := ContextSnapshot{
		Blocks:        make([]ContextBlock, 0, len(capabilityKeys)+2),
		Messages:      []ContextMessage{},
		Observations:  make([]CapabilityObservation, 0, len(capabilityKeys)+1),
		BudgetProfile: budgetSpec.Profile,
	}
	currentUnit := NewCurrentMessageSemanticUnit(input.MessageText)
	if currentUnit.TokenEstimate > 0 {
		snapshot.SemanticUnits = append(snapshot.SemanticUnits, currentUnit)
	}

	if b.userContextProvider != nil {
		block, err := b.userContextProvider.BuildUserContextBlock(ctx, input.UserID)
		if err != nil {
			return snapshot, err
		}
		if strings.TrimSpace(block.Content) != "" {
			if block.Name == "" {
				block.Name = "用户上下文"
			}
			if block.GeneratedAt.IsZero() {
				block.GeneratedAt = b.now().UTC()
			}
			snapshot.Blocks = append(snapshot.Blocks, block)
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: "user.context",
				Decision:   string(PolicyDecisionAllow),
				Status:     "succeeded",
				Summary:    "loaded user context",
			})
		}
	}

	if b.conversationMemory != nil {
		memory, err := b.conversationMemory.BuildConversationMemory(ctx, input)
		if err != nil {
			return snapshot, err
		}
		units := BuildConversationSemanticUnits(memory.Messages)
		selectedUnits, budgetReport := SelectSemanticUnitsByTokenBudget(units, budgetSpec.RecentMessagesTokens, budgetSpec.Profile)
		snapshot.SemanticUnits = append(snapshot.SemanticUnits, selectedUnits...)
		snapshot.BudgetReport = budgetReport
		snapshot.Messages = append(snapshot.Messages, SelectedMessagesFromSemanticUnits(selectedUnits)...)
		snapshot.HistoryNeedHint = memory.HistoryNeedHint
		snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
			Capability: "conversation.query_recent",
			Decision:   string(PolicyDecisionAllow),
			Status:     "succeeded",
			Summary:    fmt.Sprintf("selected %d of %d recent conversation messages", len(snapshot.Messages), len(memory.Messages)),
		})
		if memory.HistoryQueried {
			status := "succeeded"
			summary := fmt.Sprintf("loaded %d history messages", len(memory.HistoryResults))
			content := strings.TrimSpace(memory.HistoryResultContent)
			if content == "" {
				content = FormatContextMessages(memory.HistoryResults)
			}
			if strings.TrimSpace(content) == "" {
				status = "empty"
				summary = "no matching history messages"
				content = "没有查到明确历史聊天记录。"
			}
			snapshot.Blocks = append(snapshot.Blocks, ContextBlock{
				Name:            "历史聊天查询结果",
				CapabilityKey:   "conversation.query_history",
				Content:         content,
				ItemCount:       len(memory.HistoryResults),
				GeneratedAt:     b.now().UTC(),
				TrustLevel:      "transcript",
				Source:          "history_query_plan",
				EvidenceRefs:    contextMessageCanonicalRefs(memory.HistoryResults),
				RetentionReason: "history_query_plan",
			})
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: "conversation.query_history",
				Decision:   string(PolicyDecisionAllow),
				Status:     status,
				Summary:    summary,
			})
		}
	}

	for _, key := range capabilityKeys {
		capability, ok := b.registry.Get(key)
		if !ok {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: key,
				Decision:   string(PolicyDecisionForbidden),
				Status:     "skipped",
				Summary:    "capability is not registered",
			})
			continue
		}
		if !canPrefetchContextCapability(capability.Key) {
			continue
		}
		decision := b.policy.Decide(ctx, PolicyInput{Capability: capability, UserID: input.UserID})
		if decision.Decision != PolicyDecisionAllow {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: capability.Key,
				Decision:   string(decision.Decision),
				Status:     "blocked",
				Summary:    decision.Reason,
			})
			continue
		}
		if b.executor == nil {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: capability.Key,
				Decision:   string(decision.Decision),
				Status:     "skipped",
				Summary:    "capability executor is unavailable",
			})
			continue
		}
		result, err := b.executor.Execute(ctx, CapabilityExecuteInput{
			Capability:      capability,
			UserID:          input.UserID,
			SessionID:       input.SessionID,
			TurnID:          input.TurnID,
			ControllerRunID: input.ControllerRunID,
			Message:         input.MessageText,
		})
		if err != nil {
			return snapshot, fmt.Errorf("%s: %w", capability.Key, err)
		}
		observation := result.Observation
		if observation.Capability == "" {
			observation.Capability = capability.Key
		}
		if observation.Decision == "" {
			observation.Decision = string(decision.Decision)
		}
		if observation.Status == "" {
			observation.Status = "succeeded"
		}
		snapshot.Observations = append(snapshot.Observations, observation)
		for _, block := range result.Blocks {
			if strings.TrimSpace(block.Content) == "" {
				continue
			}
			if block.CapabilityKey == "" {
				block.CapabilityKey = capability.Key
			}
			if block.GeneratedAt.IsZero() {
				block.GeneratedAt = b.now().UTC()
			}
			snapshot.Blocks = append(snapshot.Blocks, block)
		}
	}

	snapshot.Blocks = append(snapshot.Blocks, ContextBlock{
		Name:          "可用能力边界",
		CapabilityKey: "capability.list_available",
		Content:       b.capabilityBoundaryText(capabilityKeys),
		ItemCount:     len(capabilityKeys),
		GeneratedAt:   b.now().UTC(),
		TrustLevel:    "system",
	})
	return finalizeContextSnapshot(snapshot, input, budgetSpec), nil
}

func (b *DefaultContextBuilder) capabilityKeysForInput(input ContextBuildInput) []string {
	keys := append([]string(nil), b.capabilityKeys...)
	if len(input.CapabilityKeys) > 0 {
		keys = append([]string(nil), input.CapabilityKeys...)
	}
	return uniqueStrings(keys)
}

func canPrefetchContextCapability(key string) bool {
	return false
}

func ClassifyHistoryNeed(text string) HistoryNeedHint {
	return HistoryNeedNone
}

func ShouldQueryConversationHistory(hint HistoryNeedHint, message string, recent []ContextMessage) bool {
	return false
}

func recentWindowHasEvidence(message string, recent []ContextMessage, requireKeyword bool) bool {
	return false
}

func HistorySearchKeyword(message string) string {
	return ""
}

func FormatContextMessages(messages []ContextMessage) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(formatContextMessageTime(message.CreatedAt))
		builder.WriteString(" ")
		builder.WriteString(formatContextMessageRole(message.Role))
		builder.WriteString("：")
		builder.WriteString(content)
	}
	return builder.String()
}

func finalizeContextSnapshot(snapshot ContextSnapshot, input ContextBuildInput, budgetSpec ContextBudgetSpec) ContextSnapshot {
	snapshot.BudgetProfile = budgetSpec.Profile
	report := snapshot.BudgetReport
	if report.Profile == "" {
		report = ContextBudgetReport{
			Profile:              budgetSpec.Profile,
			RecentMessagesTokens: budgetSpec.RecentMessagesTokens,
			AvailableInputTokens: budgetSpec.TotalTokens - budgetSpec.OutputReserveTokens - budgetSpec.SafetyMarginTokens,
		}
	}
	report = CompleteContextBudgetReport(report, budgetSpec)
	for index := range snapshot.Blocks {
		if snapshot.Blocks[index].TokenEstimate == 0 {
			snapshot.Blocks[index].TokenEstimate = estimateContextTokenCount(snapshot.Blocks[index].Content)
		}
		if strings.TrimSpace(snapshot.Blocks[index].Source) == "" {
			snapshot.Blocks[index].Source = "context_block"
		}
		if strings.TrimSpace(snapshot.Blocks[index].RetentionReason) == "" {
			snapshot.Blocks[index].RetentionReason = "context_block"
		}
	}
	for _, unit := range snapshot.SemanticUnits {
		if !unit.Selected {
			continue
		}
		exists := false
		for _, traced := range report.Units {
			if traced.UnitID == unit.ID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		report.UsedTokens += unit.TokenEstimate
		report.SelectedUnitCount++
		report.Units = append(report.Units, ContextBudgetUnitTrace{
			UnitID:          unit.ID,
			UnitType:        unit.Type,
			TokenEstimate:   unit.TokenEstimate,
			Selected:        unit.Selected,
			Protected:       unit.Protected,
			Projected:       unit.Projected,
			RetentionReason: unit.RetentionReason,
			OmittedReason:   unit.OmittedReason,
		})
	}
	snapshot.BudgetReport = report
	snapshot.Bundle = ContextBundle{
		BudgetProfile:   budgetSpec.Profile,
		SystemBlocks:    append([]ContextBlock(nil), snapshot.Blocks...),
		RecentMessages:  append([]ContextMessage(nil), snapshot.Messages...),
		CurrentMessage:  currentContextMessage(input.MessageText),
		KeyObservations: append([]CapabilityObservation(nil), snapshot.Observations...),
		SemanticUnits:   append([]ContextSemanticUnit(nil), snapshot.SemanticUnits...),
		BudgetReport:    snapshot.BudgetReport,
	}
	return snapshot
}

func currentContextMessage(message string) *ContextMessage {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	return &ContextMessage{
		Role:    domain.AgentTranscriptRoleUser,
		Content: message,
	}
}

func contextMessageCanonicalRefs(messages []ContextMessage) []string {
	refs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.TranscriptEntryID <= 0 {
			continue
		}
		refs = append(refs, "transcript:"+fmt.Sprint(message.TranscriptEntryID))
	}
	return NormalizeCanonicalRefs(refs)
}

func formatContextMessageRole(role domain.AgentTranscriptRole) string {
	if role == domain.AgentTranscriptRoleAssistant {
		return "Agent"
	}
	if role == domain.AgentTranscriptRoleUser {
		return "用户"
	}
	return string(role)
}

func formatContextMessageTime(value time.Time) string {
	if value.IsZero() {
		return "时间未知"
	}
	return value.UTC().Format("2006-01-02 15:04")
}

func historyNeedPrompt(hint HistoryNeedHint) string {
	switch hint {
	case HistoryNeedRequired:
		return "用户明确要求回忆较早聊天。若用户询问第一条、最早、最开始或开头消息，必须依据 conversation.query_history 的 earliest 结果或历史查询结果中的边界信息回答；当结果显示 has_older=false 且返回了记录时，这表示已确认当前 session 没有更早记录，应回答该记录就是当前 session 的第一条，不得说没有查到第一条。若最近聊天窗口和历史聊天查询结果均无明确原文证据，必须说明没有查到明确记录，不得凭印象编造。"
	case HistoryNeedPossible:
		return "用户可能在指代最近上下文。优先使用最近聊天窗口；若证据不足，才依据历史聊天查询结果回答。"
	default:
		return "通常不需要查询历史聊天。若回答依赖更早对话且当前上下文没有证据，必须说明需要查询历史，不能编造。没有更早记录属于边界确认，不等同于查询失败。"
	}
}

func (b *DefaultContextBuilder) capabilityBoundaryText(capabilityKeys []string) string {
	if b == nil || b.registry == nil {
		return "当前没有可用能力。"
	}
	var builder strings.Builder
	builder.WriteString("当前只允许使用只读能力。")
	for _, key := range capabilityKeys {
		capability, ok := b.registry.Get(key)
		if !ok {
			continue
		}
		builder.WriteString("\n")
		builder.WriteString(capability.Key)
		builder.WriteString("：")
		builder.WriteString(capability.Description)
	}
	return builder.String()
}
