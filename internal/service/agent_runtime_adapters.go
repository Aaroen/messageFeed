package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
	"time"
)

const (
	conversationHistoryModeSearch   = "search"
	conversationHistoryModeEarliest = "earliest"
	conversationHistoryModeLatest   = "latest"
)

type agentUserContextBlockProvider struct {
	provider AgentUserContextProvider
	now      func() time.Time
}

func (p agentUserContextBlockProvider) BuildUserContextBlock(ctx context.Context, userID int64) (agent.ContextBlock, error) {
	if p.provider == nil {
		return agent.ContextBlock{}, nil
	}
	result, err := p.provider.BuildAgentUserContext(ctx, userID)
	if err != nil {
		return agent.ContextBlock{}, err
	}
	content := strings.TrimSpace(result.Prompt.PlainText)
	if content == "" {
		return agent.ContextBlock{}, nil
	}
	now := time.Now
	if p.now != nil {
		now = p.now
	}
	return agent.ContextBlock{
		Name:          "用户上下文",
		CapabilityKey: "user.profile.read",
		Content:       content,
		ItemCount:     1,
		GeneratedAt:   now().UTC(),
		TrustLevel:    "user_profile",
	}, nil
}

type agentConversationMemoryProvider struct {
	repository AgentConversationRepository
	now        func() time.Time
}

func (p agentConversationMemoryProvider) BuildConversationMemory(ctx context.Context, input agent.ContextBuildInput) (agent.ConversationMemory, error) {
	hint := agent.ClassifyHistoryNeed(input.MessageText)
	memory := agent.ConversationMemory{HistoryNeedHint: hint}
	if p.repository == nil || input.UserID == 0 || input.SessionID == 0 {
		return memory, nil
	}

	recent, err := p.repository.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:    input.SessionID,
		UserID:       input.UserID,
		BeforeTurnID: input.TurnID,
		Roles: []domain.AgentTranscriptRole{
			domain.AgentTranscriptRoleUser,
			domain.AgentTranscriptRoleAssistant,
		},
		Limit: 12,
	})
	if err != nil {
		return memory, err
	}
	memory.Messages = transcriptEntriesToContextMessages(recent)
	if !agent.ShouldQueryConversationHistory(hint, input.MessageText, memory.Messages) {
		return memory, nil
	}

	mode := inferConversationHistoryMode(input.MessageText, "")
	keyword := ""
	order := "desc"
	limit := 8
	beforeEntryID := int64(0)
	if mode == conversationHistoryModeSearch {
		keyword = agent.HistorySearchKeyword(input.MessageText)
		beforeEntryID = earliestTranscriptEntryID(recent)
	} else if mode == conversationHistoryModeEarliest {
		order = "asc"
		limit = 1
	} else if mode == conversationHistoryModeLatest {
		limit = 1
	}
	results, err := p.repository.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{
		SessionID:     input.SessionID,
		UserID:        input.UserID,
		Mode:          mode,
		Keyword:       keyword,
		Roles:         []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant},
		BeforeEntryID: beforeEntryID,
		BeforeTurnID:  input.TurnID,
		Order:         order,
		Limit:         limit,
	})
	if err != nil {
		return memory, err
	}
	memory.HistoryQueried = true
	memory.HistoryResults = transcriptEntriesToContextMessages(results)
	memory.HistoryResultContent = formatConversationHistoryResult(conversationHistoryResultInput{
		Mode:         mode,
		Scope:        "current_session",
		Entries:      results,
		MatchedCount: len(results),
	})

	_, err = p.repository.CreateRecallEvent(ctx, domain.AgentRecallEvent{
		SessionID: input.SessionID,
		TurnID:    input.TurnID,
		UserID:    input.UserID,
		Query:     keyword,
		QueryParams: domain.AgentJSON{
			"message":           input.MessageText,
			"history_need_hint": string(hint),
			"mode":              mode,
			"keyword":           keyword,
			"order":             order,
			"before_entry_id":   beforeEntryID,
			"before_turn_id":    input.TurnID,
			"limit":             limit,
			"roles":             []string{string(domain.AgentTranscriptRoleUser), string(domain.AgentTranscriptRoleAssistant)},
			"boundary":          conversationHistoryBoundaryMetadata(mode, len(results)),
		},
		RecalledRefs: domain.AgentJSON{
			"transcript_entry_ids": transcriptEntryIDs(results),
		},
		Reason:      historyRecallReason(hint),
		BudgetChars: transcriptEntriesContentLength(results),
		CreatedAt:   p.currentTime(),
	})
	if err != nil {
		return memory, err
	}
	return memory, nil
}

type agentP0CapabilityExecutor struct {
	repository     AgentConversationRepository
	recentItems    AgentRecentItemsProvider
	sourceProvider AgentSourceProvider
	now            func() time.Time
}

func (e agentP0CapabilityExecutor) Execute(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	switch input.Capability.Key {
	case "feed.query_recent_items":
		return e.queryRecentItems(ctx, input)
	case "source.query_latest_items":
		return e.querySourceLatestItems(ctx, input)
	default:
		return agent.CapabilityExecuteResult{
			Observation: agent.CapabilityObservation{
				Capability: input.Capability.Key,
				Decision:   string(agent.PolicyDecisionForbidden),
				Status:     "skipped",
				Summary:    "capability executor does not support this capability",
			},
		}, nil
	}
}

func (e agentP0CapabilityExecutor) ExecuteTool(ctx context.Context, input agent.ToolExecuteInput) (agent.ToolExecuteResult, error) {
	switch input.Capability.Key {
	case "conversation.query_history":
		return e.queryConversationHistory(ctx, input)
	default:
		return agent.ToolExecuteResult{
			Content: "当前工具执行器不支持该能力。",
			Observation: agent.CapabilityObservation{
				Capability: input.Capability.Key,
				Decision:   string(agent.PolicyDecisionForbidden),
				Status:     "skipped",
				Summary:    "tool executor does not support this capability",
			},
		}, nil
	}
}

func (e agentP0CapabilityExecutor) queryRecentItems(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "recent items provider is unavailable"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		Limit:         5,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d recent items", len(result.Items))
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "最近条目",
				CapabilityKey: input.Capability.Key,
				Content:       formatRecentItemsBlock(result.Items),
				ItemCount:     len(result.Items),
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "database",
			},
		},
		Observation: observation,
	}, nil
}

func (e agentP0CapabilityExecutor) querySourceLatestItems(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.sourceProvider == nil || e.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "source or item provider is unavailable"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	source, found, err := e.matchSourceByText(ctx, input.UserID, input.Message)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	if !found {
		observation.Status = "skipped"
		observation.Summary = "no source name matched user input"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		SourceID:      source.ID,
		Limit:         3,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d latest items for source %s", len(result.Items), source.Name)
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "匹配来源最新条目",
				CapabilityKey: input.Capability.Key,
				Content:       formatSourceLatestItemsBlock(source, result.Items),
				ItemCount:     len(result.Items),
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "database",
			},
		},
		Observation: observation,
	}, nil
}

func (e agentP0CapabilityExecutor) matchSourceByText(ctx context.Context, userID int64, text string) (domain.Source, bool, error) {
	sources, err := e.sourceProvider.ListSources(ctx, userID)
	if err != nil {
		return domain.Source{}, false, err
	}
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return domain.Source{}, false, nil
	}
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source.Name))
		if name != "" && strings.Contains(text, name) {
			return source, true, nil
		}
	}
	return domain.Source{}, false, nil
}

func (e agentP0CapabilityExecutor) currentTime() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

type conversationHistoryToolArgs struct {
	Keyword       string `json:"keyword"`
	Role          string `json:"role"`
	Mode          string `json:"mode"`
	Order         string `json:"order"`
	Limit         int    `json:"limit"`
	BeforeEntryID int64  `json:"before_entry_id"`
	AfterEntryID  int64  `json:"after_entry_id"`
	Offset        int    `json:"offset"`
}

func (e agentP0CapabilityExecutor) queryConversationHistory(ctx context.Context, input agent.ToolExecuteInput) (agent.ToolExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.repository == nil {
		observation.Status = "skipped"
		observation.Summary = "conversation repository is unavailable"
		return agent.ToolExecuteResult{Content: "历史聊天查询能力暂不可用。", Observation: observation}, nil
	}

	args := parseConversationHistoryToolArgs(input.RawArguments)
	mode := inferConversationHistoryMode(input.Message, args.Mode)
	keyword := strings.TrimSpace(args.Keyword)
	if keyword == "" && mode == conversationHistoryModeSearch {
		keyword = agent.HistorySearchKeyword(input.Message)
	}
	if mode != conversationHistoryModeSearch {
		keyword = ""
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 8
	}
	if mode == conversationHistoryModeEarliest || mode == conversationHistoryModeLatest {
		if args.Limit <= 0 {
			limit = 1
		}
	}
	if limit > 20 {
		limit = 20
	}
	order := strings.TrimSpace(args.Order)
	if order != "asc" && order != "desc" {
		order = "desc"
		if mode == conversationHistoryModeEarliest {
			order = "asc"
		}
	}
	roles := []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant}
	switch strings.TrimSpace(args.Role) {
	case string(domain.AgentTranscriptRoleUser):
		roles = []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser}
	case string(domain.AgentTranscriptRoleAssistant):
		roles = []domain.AgentTranscriptRole{domain.AgentTranscriptRoleAssistant}
	}

	entries, err := e.repository.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{
		SessionID:     input.SessionID,
		UserID:        input.UserID,
		Mode:          mode,
		Keyword:       keyword,
		Roles:         roles,
		BeforeEntryID: args.BeforeEntryID,
		AfterEntryID:  args.AfterEntryID,
		BeforeTurnID:  input.TurnID,
		Order:         order,
		Offset:        args.Offset,
		Limit:         limit,
	})
	if err != nil {
		return agent.ToolExecuteResult{}, err
	}

	contextMessages := transcriptEntriesToContextMessages(entries)
	content := formatConversationHistoryResult(conversationHistoryResultInput{
		Mode:         mode,
		Scope:        "current_session",
		Entries:      entries,
		MatchedCount: len(contextMessages),
	})
	if len(contextMessages) == 0 {
		observation.Status = "empty"
		observation.Summary = "no matching history messages"
	} else {
		observation.Status = "succeeded"
		observation.Summary = fmt.Sprintf("loaded %d history messages", len(contextMessages))
	}

	_, err = e.repository.CreateRecallEvent(ctx, domain.AgentRecallEvent{
		SessionID: input.SessionID,
		TurnID:    input.TurnID,
		UserID:    input.UserID,
		Query:     keyword,
		QueryParams: domain.AgentJSON{
			"tool_call_id":    input.ToolCallID,
			"raw_arguments":   input.RawArguments,
			"mode":            mode,
			"keyword":         keyword,
			"role":            args.Role,
			"order":           order,
			"limit":           limit,
			"offset":          args.Offset,
			"before_entry_id": args.BeforeEntryID,
			"after_entry_id":  args.AfterEntryID,
			"before_turn_id":  input.TurnID,
			"trigger_message": input.Message,
			"capability_key":  input.Capability.Key,
			"request_id":      input.RequestID,
			"trace_id":        input.TraceID,
			"boundary":        conversationHistoryBoundaryMetadata(mode, len(entries)),
		},
		RecalledRefs: domain.AgentJSON{
			"transcript_entry_ids": transcriptEntryIDs(entries),
		},
		Reason:      "model_tool_call",
		BudgetChars: transcriptEntriesContentLength(entries),
		CreatedAt:   e.currentTime(),
	})
	if err != nil {
		return agent.ToolExecuteResult{}, err
	}
	return agent.ToolExecuteResult{Content: content, Observation: observation}, nil
}

func parseConversationHistoryToolArgs(raw string) conversationHistoryToolArgs {
	var args conversationHistoryToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return conversationHistoryToolArgs{}
	}
	args.Keyword = strings.TrimSpace(args.Keyword)
	args.Role = strings.TrimSpace(args.Role)
	args.Mode = strings.TrimSpace(args.Mode)
	args.Order = strings.TrimSpace(args.Order)
	if args.Offset < 0 {
		args.Offset = 0
	}
	return args
}

type conversationHistoryResultInput struct {
	Mode         string
	Scope        string
	Entries      []domain.AgentTranscriptEntry
	MatchedCount int
}

func formatConversationHistoryResult(input conversationHistoryResultInput) string {
	mode := inferConversationHistoryMode("", input.Mode)
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = "current_session"
	}
	metadata := conversationHistoryBoundaryMetadata(mode, len(input.Entries))
	var builder strings.Builder
	builder.WriteString("查询模式：")
	builder.WriteString(mode)
	builder.WriteString("\n查询范围：")
	builder.WriteString(scope)
	builder.WriteString("\n是否已确认会话边界：")
	builder.WriteString(formatHistoryBool(metadata["is_session_boundary"]))
	builder.WriteString("\n是否存在更早记录：")
	builder.WriteString(formatHistoryBool(metadata["has_older"]))
	builder.WriteString("\n是否存在更新记录：")
	builder.WriteString(formatHistoryBool(metadata["has_newer"]))
	builder.WriteString("\n命中条数：")
	builder.WriteString(strconv.Itoa(input.MatchedCount))
	if len(input.Entries) == 0 {
		builder.WriteString("\n没有查到符合条件的历史聊天原文。")
		if mode == conversationHistoryModeEarliest {
			builder.WriteString("\n边界说明：当前 session 在查询范围内没有当前 turn 之前的历史聊天记录。")
		}
		return builder.String()
	}
	builder.WriteString("\n命中原文：")
	builder.WriteString("\n")
	builder.WriteString(agent.FormatContextMessages(transcriptEntriesToContextMessages(input.Entries)))
	return builder.String()
}

func conversationHistoryBoundaryMetadata(mode string, matchedCount int) domain.AgentJSON {
	mode = inferConversationHistoryMode("", mode)
	metadata := domain.AgentJSON{
		"mode":                mode,
		"scope":               "current_session",
		"matched_count":       matchedCount,
		"is_session_boundary": false,
		"has_older":           "unknown",
		"has_newer":           "unknown",
	}
	switch mode {
	case conversationHistoryModeEarliest:
		metadata["is_session_boundary"] = true
		metadata["has_older"] = false
	case conversationHistoryModeLatest:
		metadata["is_session_boundary"] = true
		metadata["has_newer"] = false
	}
	return metadata
}

func formatHistoryBool(value any) string {
	switch typed := value.(type) {
	case bool:
		if typed {
			return "是"
		}
		return "否"
	case string:
		if typed == "" {
			return "未知"
		}
		if typed == "unknown" {
			return "未知"
		}
		return typed
	default:
		return "未知"
	}
}

func inferConversationHistoryMode(message string, requested string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	switch requested {
	case conversationHistoryModeEarliest, conversationHistoryModeLatest, conversationHistoryModeSearch:
		return requested
	}
	message = strings.TrimSpace(message)
	if containsAny(message, []string{"第一条", "第一句", "最早", "最开始", "最初", "开头"}) {
		return conversationHistoryModeEarliest
	}
	if containsAny(message, []string{"最后一条", "最新一条", "最近一条", "末尾"}) {
		return conversationHistoryModeLatest
	}
	return conversationHistoryModeSearch
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func (p agentConversationMemoryProvider) currentTime() time.Time {
	if p.now != nil {
		return p.now().UTC()
	}
	return time.Now().UTC()
}

func transcriptEntriesToContextMessages(entries []domain.AgentTranscriptEntry) []agent.ContextMessage {
	messages := make([]agent.ContextMessage, 0, len(entries))
	for _, entry := range entries {
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			continue
		}
		if entry.Role != domain.AgentTranscriptRoleUser && entry.Role != domain.AgentTranscriptRoleAssistant {
			continue
		}
		messages = append(messages, agent.ContextMessage{
			Role:              entry.Role,
			Content:           content,
			TranscriptEntryID: entry.ID,
			TurnID:            entry.TurnID,
			CreatedAt:         entry.CreatedAt,
		})
	}
	return messages
}

func earliestTranscriptEntryID(entries []domain.AgentTranscriptEntry) int64 {
	var earliest int64
	for _, entry := range entries {
		if entry.ID <= 0 {
			continue
		}
		if earliest == 0 || entry.ID < earliest {
			earliest = entry.ID
		}
	}
	return earliest
}

func transcriptEntryIDs(entries []domain.AgentTranscriptEntry) []int64 {
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.ID > 0 {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}

func transcriptEntriesContentLength(entries []domain.AgentTranscriptEntry) int {
	total := 0
	for _, entry := range entries {
		total += len([]rune(strings.TrimSpace(entry.Content)))
	}
	return total
}

func historyRecallReason(hint agent.HistoryNeedHint) string {
	switch hint {
	case agent.HistoryNeedRequired:
		return "required_history_recent_window_insufficient"
	case agent.HistoryNeedPossible:
		return "possible_history_recent_window_insufficient"
	default:
		return "history_query_requested"
	}
}

func formatRecentItemsBlock(items []domain.Item) string {
	if len(items) == 0 {
		return "暂无可用条目。"
	}
	var builder strings.Builder
	for i, item := range items {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
		if item.SourceName != "" {
			builder.WriteString("（")
			builder.WriteString(item.SourceName)
			builder.WriteString("）")
		}
		if item.Summary != "" {
			builder.WriteString("：")
			builder.WriteString(truncateError(item.Summary, 160))
		}
	}
	return builder.String()
}

func formatSourceLatestItemsBlock(source domain.Source, items []domain.Item) string {
	var builder strings.Builder
	builder.WriteString(source.Name)
	builder.WriteString("：")
	if len(items) == 0 {
		builder.WriteString("暂无可用条目。")
		return builder.String()
	}
	for i, item := range items {
		builder.WriteString("\n")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
	}
	return builder.String()
}
