package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/agent/timeintent"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
	"time"
)

const (
	conversationHistoryModeSearch    = "search"
	conversationHistoryModeTimeRange = "time_range"
	conversationHistoryModeEarliest  = "earliest"
	conversationHistoryModeLatest    = "latest"
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
	timeRange := parseConversationHistoryTimeRange(input.MessageText, "", p.currentTime())
	results, err := p.repository.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{
		SessionID:     input.SessionID,
		UserID:        input.UserID,
		Mode:          mode,
		Keyword:       keyword,
		TimeHint:      strings.TrimSpace(input.MessageText),
		Roles:         []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant},
		BeforeEntryID: beforeEntryID,
		BeforeTurnID:  input.TurnID,
		After:         timeRange.After,
		Before:        timeRange.Before,
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
		TimeRange:    timeRange,
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
			"time_range":        timeRange.Metadata(),
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
	repository       AgentConversationRepository
	recentItems      AgentRecentItemsProvider
	sourceProvider   AgentSourceProvider
	notificationJobs AgentNotificationJobStore
	now              func() time.Time
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
	case "agent.schedule_message":
		return e.scheduleMessage(ctx, input)
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
	Query         string `json:"query"`
	Keyword       string `json:"keyword"`
	TimeHint      string `json:"time_hint"`
	Role          string `json:"role"`
	Mode          string `json:"mode"`
	Order         string `json:"order"`
	Limit         int    `json:"limit"`
	BeforeEntryID int64  `json:"before_entry_id"`
	AfterEntryID  int64  `json:"after_entry_id"`
	Offset        int    `json:"offset"`
}

type scheduleMessageToolArgs struct {
	TaskType    string `json:"task_type"`
	Content     string `json:"content"`
	ScheduledAt string `json:"scheduled_at"`
	TimeHint    string `json:"time_hint"`
	TimeZone    string `json:"time_zone"`
	Importance  string `json:"importance"`
	Confirmed   bool   `json:"confirmed"`
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
	keyword := strings.TrimSpace(args.Query)
	if keyword == "" {
		keyword = strings.TrimSpace(args.Keyword)
	}
	if keyword == "" && mode == conversationHistoryModeSearch {
		keyword = agent.HistorySearchKeyword(input.Message)
	}
	if mode != conversationHistoryModeSearch {
		keyword = ""
	}
	timeRange := parseConversationHistoryTimeRange(input.Message, args.TimeHint, e.currentTime())
	if mode == conversationHistoryModeTimeRange && !timeRange.Valid {
		return agent.ToolExecuteResult{
			Content: "没有识别出明确时间范围。请让用户补充具体时间，例如昨天上午、上周或 2026-06-23 晚上。",
			Observation: agent.CapabilityObservation{
				Capability: input.Capability.Key,
				Decision:   string(agent.PolicyDecisionAllow),
				Status:     "empty",
				Summary:    "time range is ambiguous",
			},
		}, nil
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
		TimeHint:      args.TimeHint,
		Roles:         roles,
		BeforeEntryID: args.BeforeEntryID,
		AfterEntryID:  args.AfterEntryID,
		BeforeTurnID:  input.TurnID,
		After:         timeRange.After,
		Before:        timeRange.Before,
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
		TimeRange:    timeRange,
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
			"query":           args.Query,
			"time_hint":       args.TimeHint,
			"time_range":      timeRange.Metadata(),
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

func (e agentP0CapabilityExecutor) scheduleMessage(ctx context.Context, input agent.ToolExecuteInput) (agent.ToolExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionPrompt),
	}
	if e.notificationJobs == nil {
		observation.Status = "skipped"
		observation.Summary = "notification job store is unavailable"
		return agent.ToolExecuteResult{Content: "定时消息能力暂不可用。", Observation: observation}, nil
	}
	args := parseScheduleMessageToolArgs(input.RawArguments)
	if args.TaskType == "" {
		args.TaskType = "reminder"
	}
	if args.TaskType != "reminder" && args.TaskType != "send_message" {
		observation.Status = "failed"
		observation.Summary = "unsupported scheduled task type"
		return agent.ToolExecuteResult{Content: "不支持该定时任务类型。", Observation: observation}, nil
	}
	content := strings.TrimSpace(args.Content)
	if content == "" {
		observation.Status = "failed"
		observation.Summary = "scheduled content is empty"
		return agent.ToolExecuteResult{Content: "定时消息内容不能为空。", Observation: observation}, nil
	}
	if strings.TrimSpace(input.ExternalUserID) == "" {
		observation.Status = "failed"
		observation.Summary = "wechat work recipient is missing"
		return agent.ToolExecuteResult{Content: "无法确定当前企微接收人，不能创建定时消息。", Observation: observation}, nil
	}
	scheduledAt, parseResult := parseScheduleInstant(args.ScheduledAt, args.TimeHint, args.TimeZone, e.currentTime())
	if scheduledAt.IsZero() {
		observation.Status = "failed"
		observation.Summary = "scheduled time is ambiguous"
		return agent.ToolExecuteResult{Content: "工具状态：requires_clarification\n原因：没有明确的 scheduled_at，且 time_hint 无法被后端校验为具体时间点。请结合当前时间和最近上下文，让用户补充日期、上午/下午/晚上，或由模型归一化为 RFC3339 scheduled_at 后再次调用工具。", Observation: observation}, nil
	}
	if scheduledAt.Before(e.currentTime().Add(-time.Minute)) {
		observation.Status = "failed"
		observation.Summary = "scheduled time is in the past"
		return agent.ToolExecuteResult{Content: "工具状态：failed\n原因：scheduled_at 已经过期，不能创建定时消息。", Observation: observation}, nil
	}
	if !args.Confirmed {
		observation.Status = "requires_confirmation"
		observation.Summary = "scheduled message requires user confirmation"
		return agent.ToolExecuteResult{
			Content:     fmt.Sprintf("工具状态：requires_confirmation\n计划时间：%s\n提醒内容：%s\n说明：需要用户明确确认后才能创建；用户确认后必须再次调用 agent.schedule_message，并传 confirmed=true。", scheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), content),
			Observation: observation,
		}, nil
	}
	now := e.currentTime()
	job := domain.NotificationJob{
		UserID:  input.UserID,
		Status:  domain.NotificationJobStatusQueued,
		Channel: domain.NotificationChannelWeChatWork,
		PolicyDecision: domain.AlertPolicyDecision{
			Status:     domain.AlertPolicyDecisionStatusAllow,
			AutoNotify: true,
			Reasons:    []string{"agent scheduled message confirmed by user"},
			Channel:    string(domain.NotificationChannelWeChatWork),
		},
		Payload: domain.NotificationPayload{
			"task_type":        args.TaskType,
			"content":          content,
			"to_user":          strings.TrimSpace(input.ExternalUserID),
			"scheduled_at":     scheduledAt.UTC().Format(time.RFC3339),
			"time_hint":        args.TimeHint,
			"time_zone":        parseResult.TimeZone,
			"importance":       normalizedScheduleImportance(args.Importance),
			"source":           "agent.schedule_message",
			"session_id":       input.SessionID,
			"turn_id":          input.TurnID,
			"trigger_message":  input.Message,
			"requires_confirm": false,
		},
		RequestID:   input.RequestID,
		TraceID:     input.TraceID,
		DedupeKey:   scheduledMessageDedupeKey(input.UserID, input.ExternalUserID, args.TaskType, content, scheduledAt),
		ScheduledAt: scheduledAt.UTC(),
		MaxAttempts: 3,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := e.notificationJobs.CreateJob(ctx, job)
	if err != nil {
		return agent.ToolExecuteResult{}, err
	}
	observation.Decision = string(agent.PolicyDecisionAllow)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("scheduled notification job %d", created.ID)
	return agent.ToolExecuteResult{
		Content:     fmt.Sprintf("工具状态：created\n任务 ID：%d\n计划时间：%s\n提醒内容：%s", created.ID, created.ScheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), content),
		Observation: observation,
	}, nil
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
	args.Query = strings.TrimSpace(args.Query)
	args.TimeHint = strings.TrimSpace(args.TimeHint)
	args.Role = strings.TrimSpace(args.Role)
	args.Mode = strings.TrimSpace(args.Mode)
	args.Order = strings.TrimSpace(args.Order)
	if args.Offset < 0 {
		args.Offset = 0
	}
	return args
}

func parseScheduleMessageToolArgs(raw string) scheduleMessageToolArgs {
	var args scheduleMessageToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return scheduleMessageToolArgs{}
	}
	args.TaskType = strings.TrimSpace(args.TaskType)
	args.Content = strings.TrimSpace(args.Content)
	args.ScheduledAt = strings.TrimSpace(args.ScheduledAt)
	args.TimeHint = strings.TrimSpace(args.TimeHint)
	args.TimeZone = strings.TrimSpace(args.TimeZone)
	args.Importance = strings.TrimSpace(args.Importance)
	return args
}

type conversationHistoryResultInput struct {
	Mode         string
	Scope        string
	Entries      []domain.AgentTranscriptEntry
	MatchedCount int
	TimeRange    conversationHistoryTimeRange
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
	if input.TimeRange.Valid {
		builder.WriteString("\n时间范围：")
		builder.WriteString(input.TimeRange.After.UTC().Format(time.RFC3339))
		builder.WriteString(" 至 ")
		builder.WriteString(input.TimeRange.Before.UTC().Format(time.RFC3339))
		builder.WriteString("（")
		builder.WriteString(input.TimeRange.TimeZone)
		builder.WriteString("）")
	}
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
	case conversationHistoryModeEarliest, conversationHistoryModeLatest, conversationHistoryModeSearch, conversationHistoryModeTimeRange:
		return requested
	}
	message = strings.TrimSpace(message)
	if containsAny(message, []string{"第一条", "第一句", "最早", "最开始", "最初", "开头"}) {
		return conversationHistoryModeEarliest
	}
	if containsAny(message, []string{"最后一条", "最新一条", "最近一条", "末尾"}) {
		return conversationHistoryModeLatest
	}
	if containsAny(message, []string{"昨天", "前天", "今天", "今日", "明天", "后天", "上周", "本周", "这周", "下周", "本月", "这个月", "上午", "下午", "晚上", "凌晨"}) {
		return conversationHistoryModeTimeRange
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

type conversationHistoryTimeRange struct {
	Valid    bool
	After    *time.Time
	Before   *time.Time
	TimeZone string
	Matched  string
}

func (r conversationHistoryTimeRange) Metadata() domain.AgentJSON {
	if !r.Valid {
		return domain.AgentJSON{"valid": false}
	}
	return domain.AgentJSON{
		"valid":     true,
		"after":     r.After.UTC().Format(time.RFC3339),
		"before":    r.Before.UTC().Format(time.RFC3339),
		"time_zone": r.TimeZone,
		"matched":   r.Matched,
	}
}

func parseConversationHistoryTimeRange(message string, timeHint string, now time.Time) conversationHistoryTimeRange {
	text := strings.TrimSpace(timeHint)
	if text == "" {
		text = strings.TrimSpace(message)
	}
	location := agentTimeLocation()
	parsed := timeintent.Parse(text, now, location)
	if !parsed.HasRange() {
		return conversationHistoryTimeRange{}
	}
	after := parsed.StartAt.UTC()
	before := parsed.EndAt.UTC()
	return conversationHistoryTimeRange{
		Valid:    true,
		After:    &after,
		Before:   &before,
		TimeZone: parsed.TimeZone,
		Matched:  parsed.Matched,
	}
}

func agentTimeLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Local
	}
	return location
}

func parseScheduleInstant(scheduledAt string, timeHint string, timeZone string, now time.Time) (time.Time, timeintent.Result) {
	location := scheduleTimeLocation(timeZone)
	scheduledAt = strings.TrimSpace(scheduledAt)
	if scheduledAt != "" {
		if parsed, err := time.Parse(time.RFC3339, scheduledAt); err == nil {
			return parsed.UTC(), timeintent.Result{
				Kind:       timeintent.KindInstant,
				InstantAt:  parsed.In(location),
				TimeZone:   location.String(),
				Confidence: "model_normalized",
				Matched:    scheduledAt,
			}
		}
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006/01/02 15:04:05", "2006/01/02 15:04"} {
			if parsed, err := time.ParseInLocation(layout, scheduledAt, location); err == nil {
				return parsed.UTC(), timeintent.Result{
					Kind:       timeintent.KindInstant,
					InstantAt:  parsed,
					TimeZone:   location.String(),
					Confidence: "model_normalized",
					Matched:    scheduledAt,
				}
			}
		}
	}
	parsed := timeintent.Parse(timeHint, now, location)
	if parsed.HasInstant() {
		return parsed.InstantAt.UTC(), parsed
	}
	if parsed.HasRange() {
		return parsed.StartAt.UTC(), parsed
	}
	return time.Time{}, parsed
}

func scheduleTimeLocation(timeZone string) *time.Location {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		return agentTimeLocation()
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return agentTimeLocation()
	}
	return location
}

func normalizedScheduleImportance(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "high" {
		return "high"
	}
	return "normal"
}

func scheduledMessageDedupeKey(userID int64, toUser string, taskType string, content string, scheduledAt time.Time) string {
	raw := strings.Join([]string{
		strconv.FormatInt(userID, 10),
		strings.TrimSpace(toUser),
		strings.TrimSpace(taskType),
		strings.TrimSpace(content),
		scheduledAt.UTC().Format(time.RFC3339),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("agent_scheduled_message:%x", sum[:16])
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
