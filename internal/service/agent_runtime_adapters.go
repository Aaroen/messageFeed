package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"messagefeed/internal/agent"
	"messagefeed/internal/agent/timeintent"
	"messagefeed/internal/domain"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	conversationHistoryModeSearch    = "search"
	conversationHistoryModeTimeRange = "time_range"
	conversationHistoryModeEarliest  = "earliest"
	conversationHistoryModeLatest    = "latest"
	agentWebFetchMaxBytes            = int64(1 << 20)
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
	repository    AgentConversationRepository
	factRetriever *agentFactRetriever
	now           func() time.Time
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
		Limit: agent.RecentConversationCandidateLimit(input.BudgetProfile),
	})
	if err != nil {
		return memory, err
	}
	memory.Messages = transcriptEntriesToContextMessages(recent)
	memory.MemoryBlocks = append(memory.MemoryBlocks, p.stableMemoryBlocks(ctx, input)...)
	if conversationHistoryPlanHasContent(input.HistoryQueryPlan) {
		historyResults, historyContent, err := p.queryPlannedConversationHistory(ctx, input)
		if err != nil {
			return memory, err
		}
		memory.HistoryQueried = true
		memory.HistoryResults = historyResults
		memory.HistoryResultContent = historyContent
		memory.MemoryBlocks = append(memory.MemoryBlocks, p.factRecallBlocks(ctx, input)...)
	}
	return memory, nil
}

func (p agentConversationMemoryProvider) queryPlannedConversationHistory(ctx context.Context, input agent.ContextBuildInput) ([]agent.ContextMessage, string, error) {
	plan := input.HistoryQueryPlan
	mode := inferConversationHistoryMode(plan.Mode)
	keyword := strings.TrimSpace(plan.Query)
	if mode != conversationHistoryModeSearch {
		keyword = ""
	}
	now := time.Now().UTC()
	if p.now != nil {
		now = p.now().UTC()
	}
	timeRange := parseConversationHistoryTimeRange(input.MessageText, plan.TimeHint, now)
	if mode == conversationHistoryModeTimeRange && !timeRange.Valid {
		return nil, "历史查询计划的时间范围不明确，未执行历史回表查询。", nil
	}
	limit := plan.Limit
	if limit <= 0 {
		limit = 8
	}
	if mode == conversationHistoryModeEarliest || mode == conversationHistoryModeLatest {
		limit = 1
	}
	if limit > 20 {
		limit = 20
	}
	order := "desc"
	if mode == conversationHistoryModeEarliest {
		order = "asc"
	}
	entries, err := p.repository.QueryTranscriptEntries(ctx, domain.AgentTranscriptQueryOptions{
		SessionID:    input.SessionID,
		UserID:       input.UserID,
		Mode:         mode,
		Keyword:      keyword,
		TimeHint:     plan.TimeHint,
		Roles:        []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant},
		BeforeTurnID: input.TurnID,
		After:        timeRange.After,
		Before:       timeRange.Before,
		Order:        order,
		Limit:        limit,
	})
	if err != nil {
		return nil, "", err
	}
	messages := transcriptEntriesToContextMessages(entries)
	reason := strings.TrimSpace(plan.Reason)
	if reason == "" {
		reason = "main_agent_history_query_plan"
	}
	content := formatConversationHistoryResult(conversationHistoryResultInput{
		Mode:         mode,
		Scope:        "current_session",
		Reason:       reason,
		Entries:      entries,
		MatchedCount: len(messages),
		TimeRange:    timeRange,
	})
	_, _ = p.repository.CreateRecallEvent(ctx, domain.AgentRecallEvent{
		SessionID: input.SessionID,
		TurnID:    input.TurnID,
		UserID:    input.UserID,
		Query:     keyword,
		QueryParams: domain.AgentJSON{
			"mode":           mode,
			"query":          plan.Query,
			"keyword":        keyword,
			"time_hint":      plan.TimeHint,
			"reason":         reason,
			"limit":          limit,
			"trigger":        "main_agent_history_query_plan",
			"memory_scope":   "long_term_conversation",
			"before_turn_id": input.TurnID,
		},
		RecalledRefs: domain.AgentJSON{
			"transcript_entry_ids": transcriptEntryIDs(entries),
			"evidence_refs":        transcriptEvidenceRefs(entries),
		},
		Reason:      reason,
		BudgetChars: transcriptEntriesContentLength(entries),
		CreatedAt:   p.currentTime(),
	})
	return messages, content, nil
}

func (p agentConversationMemoryProvider) stableMemoryBlocks(ctx context.Context, input agent.ContextBuildInput) []agent.ContextBlock {
	if p.repository == nil || input.UserID == 0 {
		return nil
	}
	blocks, err := p.repository.ListAgentMemoryBlocks(ctx, domain.AgentMemoryBlockQueryOptions{
		UserID: input.UserID,
		Statuses: []domain.AgentMemoryBlockStatus{
			domain.AgentMemoryBlockActive,
		},
		Limit: 8,
	})
	if err != nil {
		return nil
	}
	output := make([]agent.ContextBlock, 0, len(blocks))
	for _, block := range blocks {
		content := strings.TrimSpace(block.Content)
		if content == "" {
			continue
		}
		output = append(output, agent.ContextBlock{
			Name:            firstNonEmptyString(block.Title, "稳定记忆"),
			CapabilityKey:   "memory.stable",
			Content:         content,
			ItemCount:       1,
			GeneratedAt:     block.UpdatedAt,
			TrustLevel:      "stable_memory",
			Source:          "stable_memory",
			EvidenceRefs:    agent.NormalizeCanonicalRefs(block.EvidenceRefs),
			CanonicalRef:    fmt.Sprintf("memory_block:%d", block.ID),
			RetentionReason: "stable_memory",
		})
	}
	return output
}

func (p agentConversationMemoryProvider) factRecallBlocks(ctx context.Context, input agent.ContextBuildInput) []agent.ContextBlock {
	if p.repository == nil || input.UserID == 0 {
		return nil
	}
	query := strings.TrimSpace(input.HistoryQueryPlan.Query)
	if query == "" {
		query = strings.TrimSpace(input.MessageText)
	}
	retriever := p.factRetriever
	if retriever == nil {
		retriever = newAgentFactRetriever(p.repository, nil, "", p.now)
	}
	result, err := retriever.Recall(ctx, domain.AgentFactRecallPlan{
		Mode:            domain.AgentFactRecallModeHybrid,
		Query:           query,
		UserID:          input.UserID,
		SessionID:       input.SessionID,
		TurnID:          input.TurnID,
		Limit:           8,
		NeedsSourceFact: true,
		MaxRiskLevel:    domain.AgentMemoryRiskMedium,
	})
	if err != nil || len(result.Projections) == 0 {
		return nil
	}
	output := make([]agent.ContextBlock, 0, len(result.Projections))
	refs := make([]string, 0, len(result.Projections))
	for _, projection := range result.Projections {
		content := strings.TrimSpace(formatFactProjectionForContext(projection))
		if content == "" {
			continue
		}
		ref := agent.NormalizeCanonicalRef(projection.CanonicalRef)
		refs = append(refs, ref)
		output = append(output, agent.ContextBlock{
			Name:            "长期事实召回",
			CapabilityKey:   "memory.fact_recall",
			Content:         content,
			ItemCount:       1,
			GeneratedAt:     projection.SourceFact.CreatedAt,
			TrustLevel:      "source_fact",
			Source:          "fact_recall",
			EvidenceRefs:    agent.NormalizeCanonicalRefs(append([]string{ref}, projection.SourceFact.SourceRefs...)),
			CanonicalRef:    ref,
			RetentionReason: "history_query_plan",
		})
	}
	if len(refs) > 0 {
		_, _ = p.repository.CreateRecallEvent(ctx, domain.AgentRecallEvent{
			SessionID: input.SessionID,
			TurnID:    input.TurnID,
			UserID:    input.UserID,
			Query:     query,
			QueryParams: domain.AgentJSON{
				"query":        query,
				"trigger":      "main_agent_fact_recall",
				"memory_scope": "long_term_fact_index",
				"limit":        8,
			},
			RecalledRefs: domain.AgentJSON{
				"canonical_refs": refs,
				"hit_count":      len(result.Hits),
				"sources":        recallHitSources(result.Hits),
			},
			Reason:      "main_agent_history_query_plan",
			BudgetChars: agentContextBlocksContentLength(output),
			CreatedAt:   p.currentTime(),
		})
	}
	return output
}

func conversationHistoryPlanHasContent(plan agent.PlanHistoryQueryPlan) bool {
	return strings.TrimSpace(plan.Mode) != "" ||
		strings.TrimSpace(plan.Query) != "" ||
		strings.TrimSpace(plan.TimeHint) != "" ||
		strings.TrimSpace(plan.Reason) != ""
}

type agentP0CapabilityExecutor struct {
	repository       AgentConversationRepository
	recentItems      AgentRecentItemsProvider
	sourceProvider   AgentSourceProvider
	notificationJobs AgentNotificationJobStore
	scheduledTasks   AgentScheduleEvalRepository
	webFetcher       agentWebFetcher
	now              func() time.Time
}

type agentWebFetcher func(ctx context.Context, rawURL string) ([]byte, string, int, string, error)

func (e agentP0CapabilityExecutor) Execute(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	switch input.Capability.Key {
	case "feed.query_recent_items":
		return e.queryRecentItems(ctx, input)
	case "source.query_latest_items":
		return e.querySourceLatestItems(ctx, input)
	case "web.search":
		return e.webSearchCapability(ctx, input)
	case "content.summarize_text":
		return e.summarizeTextCapability(input), nil
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

func (e agentP0CapabilityExecutor) CallTool(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	switch input.Capability.Key {
	case "feed.query_recent_items":
		result, err := e.queryRecentItems(ctx, agent.CapabilityExecuteInput{
			Capability:      input.Capability,
			UserID:          input.UserID,
			SessionID:       input.SessionID,
			TurnID:          input.TurnID,
			ControllerRunID: input.ControllerRunID,
			Message:         input.Message,
			RawArguments:    input.RawArguments,
		})
		return capabilityExecuteResultToToolResult(result), err
	case "source.query_latest_items":
		result, err := e.querySourceLatestItems(ctx, agent.CapabilityExecuteInput{
			Capability:      input.Capability,
			UserID:          input.UserID,
			SessionID:       input.SessionID,
			TurnID:          input.TurnID,
			ControllerRunID: input.ControllerRunID,
			Message:         input.Message,
			RawArguments:    input.RawArguments,
		})
		return capabilityExecuteResultToToolResult(result), err
	case "conversation.query_history":
		return e.queryConversationHistory(ctx, input)
	case "agent.schedule_task":
		return e.scheduleTask(ctx, input)
	case "agent.schedule_message":
		return e.scheduleMessage(ctx, input)
	case "web.search":
		return e.webSearch(ctx, input)
	case "web.fetch_page":
		return e.webFetchPage(ctx, input)
	case "web.extract_page":
		return e.webExtractPage(ctx, input)
	case "repo.search":
		return e.repoSearch(ctx, input)
	case "repo.inspect_remote":
		return e.repoInspectRemote(ctx, input)
	case "content.summarize_text":
		return e.summarizeTextTool(input), nil
	default:
		return agent.NewMCPTextCallToolResult("当前 MCP tools/call 执行器不支持该工具。", true, agent.CapabilityObservation{
			Capability: input.Capability.Key,
			Decision:   string(agent.PolicyDecisionForbidden),
			Status:     "skipped",
			Summary:    "tool executor does not support this capability",
		}), nil
	}
}

func capabilityExecuteResultToToolResult(result agent.CapabilityExecuteResult) agent.MCPCallToolResult {
	parts := make([]string, 0, len(result.Blocks))
	for _, block := range result.Blocks {
		content := strings.TrimSpace(block.Content)
		if content != "" {
			parts = append(parts, content)
		}
	}
	return agent.NewMCPTextCallToolResult(strings.Join(parts, "\n\n"), false, result.Observation)
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
	args := parseAgentItemToolArgs(input.RawArguments)
	query, err := e.parseItemQuery(ctx, input.UserID, args, 5)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		SourceID:      query.SourceID,
		IsRead:        query.IsRead,
		Limit:         50,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	items := filterAgentItems(result.Items, query)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d recent items with filters %s", len(items), formatItemQuerySummary(query))
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "最近条目",
				CapabilityKey: input.Capability.Key,
				Content:       "新鲜度提示：订阅源结果 12 小时后建议刷新。\n" + formatRecentItemsBlock(items),
				ItemCount:     len(items),
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
	args := parseAgentItemToolArgs(input.RawArguments)
	source, found, err := e.resolveItemSource(ctx, input.UserID, args)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	if !found {
		observation.Status = "skipped"
		observation.Summary = "source parameter did not match an active source"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	query, err := e.parseItemQuery(ctx, input.UserID, args, 3)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	query.SourceID = source.ID
	query.SourceName = source.Name
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		SourceID:      source.ID,
		IsRead:        query.IsRead,
		Limit:         50,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	items := filterAgentItems(result.Items, query)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d latest items for source %s with filters %s", len(items), source.Name, formatItemQuerySummary(query))
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "匹配来源最新条目",
				CapabilityKey: input.Capability.Key,
				Content:       "新鲜度提示：订阅源结果 12 小时后建议刷新。\n" + formatSourceLatestItemsBlock(source, items),
				ItemCount:     len(items),
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "database",
			},
		},
		Observation: observation,
	}, nil
}

func (e agentP0CapabilityExecutor) webSearchCapability(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	content, observation, itemCount, err := e.runWebSearch(ctx, input.Capability.Key, input.Message, 5, input.Message)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	result := agent.CapabilityExecuteResult{Observation: observation}
	if strings.TrimSpace(content) != "" {
		result.Blocks = []agent.ContextBlock{
			{
				Name:          "联网搜索结果",
				CapabilityKey: input.Capability.Key,
				Content:       content,
				ItemCount:     itemCount,
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "external_web",
			},
		}
	}
	return result, nil
}

type agentItemToolArgs struct {
	Query      string `json:"query"`
	Keyword    string `json:"keyword"`
	SourceID   int64  `json:"source_id"`
	SourceName string `json:"source_name"`
	TimeHint   string `json:"time_hint"`
	IsRead     *bool  `json:"is_read"`
	Limit      int    `json:"limit"`
}

func parseAgentItemToolArgs(raw string) agentItemToolArgs {
	var args agentItemToolArgs
	if strings.TrimSpace(raw) == "" {
		return args
	}
	_ = json.Unmarshal([]byte(raw), &args)
	return args
}

func (e agentP0CapabilityExecutor) resolveItemSource(ctx context.Context, userID int64, args agentItemToolArgs) (domain.Source, bool, error) {
	sourceID := args.SourceID
	sourceName := strings.ToLower(strings.TrimSpace(args.SourceName))
	if sourceID <= 0 && sourceName == "" {
		return domain.Source{}, false, nil
	}
	sources, err := e.sourceProvider.ListSources(ctx, userID)
	if err != nil {
		return domain.Source{}, false, err
	}
	if sourceID > 0 {
		for _, source := range sources {
			if source.ID == sourceID {
				return source, true, nil
			}
		}
	}
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source.Name))
		if sourceName != "" && name == sourceName {
			return source, true, nil
		}
	}
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source.Name))
		if sourceName != "" && name != "" && (strings.Contains(name, sourceName) || strings.Contains(sourceName, name)) {
			return source, true, nil
		}
	}
	return domain.Source{}, false, nil
}

type agentItemQuery struct {
	SourceID   int64
	SourceName string
	Keyword    string
	IsRead     *bool
	TimeRange  conversationHistoryTimeRange
	Limit      int
}

func (e agentP0CapabilityExecutor) parseItemQuery(ctx context.Context, userID int64, args agentItemToolArgs, defaultLimit int) (agentItemQuery, error) {
	query := agentItemQuery{
		Keyword: strings.TrimSpace(firstNonEmptyString(args.Query, args.Keyword)),
		IsRead:  args.IsRead,
		Limit:   args.Limit,
	}
	if query.Limit <= 0 {
		query.Limit = defaultLimit
	}
	if query.Limit > 20 {
		query.Limit = 20
	}
	query.TimeRange = parseConversationHistoryTimeRange("", args.TimeHint, e.currentTime())
	if e.sourceProvider != nil && (args.SourceID > 0 || strings.TrimSpace(args.SourceName) != "") {
		source, found, err := e.resolveItemSource(ctx, userID, args)
		if err != nil {
			return query, err
		}
		if found {
			query.SourceID = source.ID
			query.SourceName = source.Name
		}
	}
	return query, nil
}

func filterAgentItems(items []domain.Item, query agentItemQuery) []domain.Item {
	filtered := make([]domain.Item, 0, len(items))
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	for _, item := range items {
		if query.TimeRange.Valid {
			itemTime := item.PublishedAt
			if itemTime == nil {
				fetchedAt := item.FetchedAt
				itemTime = &fetchedAt
			}
			if itemTime == nil || itemTime.Before(query.TimeRange.After.UTC()) || !itemTime.Before(query.TimeRange.Before.UTC()) {
				continue
			}
		}
		if keyword != "" {
			text := strings.ToLower(strings.Join([]string{item.Title, item.Summary, item.ContentSnippet, item.SourceName}, " "))
			if !strings.Contains(text, keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
		if query.Limit > 0 && len(filtered) >= query.Limit {
			break
		}
	}
	return filtered
}

func formatItemQuerySummary(query agentItemQuery) string {
	parts := make([]string, 0, 4)
	if query.SourceName != "" {
		parts = append(parts, "source="+query.SourceName)
	} else if query.SourceID > 0 {
		parts = append(parts, "source_id="+strconv.FormatInt(query.SourceID, 10))
	}
	if query.Keyword != "" {
		parts = append(parts, "keyword="+query.Keyword)
	}
	if query.IsRead != nil {
		parts = append(parts, fmt.Sprintf("is_read=%t", *query.IsRead))
	}
	if query.TimeRange.Valid {
		parts = append(parts, "time_range="+query.TimeRange.After.UTC().Format(time.RFC3339)+"/"+query.TimeRange.Before.UTC().Format(time.RFC3339))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
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
	TaskType            string   `json:"task_type"`
	Goal                string   `json:"goal"`
	Content             string   `json:"content"`
	ScheduledAt         string   `json:"scheduled_at"`
	TimeHint            string   `json:"time_hint"`
	TimeZone            string   `json:"time_zone"`
	Importance          string   `json:"importance"`
	TargetChannel       string   `json:"target_channel"`
	FreshnessPolicy     string   `json:"freshness_policy"`
	AllowedCapabilities []string `json:"allowed_capabilities"`
	Confirmed           bool     `json:"confirmed"`
}

type webSearchToolArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type webURLToolArgs struct {
	URL string `json:"url"`
}

type summarizeTextToolArgs struct {
	Text       string                `json:"text"`
	Sources    []summarizeTextSource `json:"sources"`
	SourceRefs []string              `json:"source_refs"`
}

type summarizeTextSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

type repoSearchToolArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type repoInspectToolArgs struct {
	Repo string `json:"repo"`
}

func (e agentP0CapabilityExecutor) queryConversationHistory(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.repository == nil {
		observation.Status = "skipped"
		observation.Summary = "conversation repository is unavailable"
		return agent.NewMCPTextCallToolResult("历史聊天查询能力暂不可用。", true, observation), nil
	}

	args := parseConversationHistoryToolArgs(input.RawArguments)
	mode := inferConversationHistoryMode(args.Mode)
	keyword := strings.TrimSpace(args.Query)
	if keyword == "" {
		keyword = strings.TrimSpace(args.Keyword)
	}
	if mode != conversationHistoryModeSearch {
		keyword = ""
	}
	timeRange := parseConversationHistoryTimeRange("", args.TimeHint, e.currentTime())
	if mode == conversationHistoryModeTimeRange && !timeRange.Valid {
		return agent.NewMCPTextCallToolResult("没有识别出明确时间范围。请让用户补充具体时间，例如昨天上午、上周或 2026-06-23 晚上。", true, agent.CapabilityObservation{
			Capability: input.Capability.Key,
			Decision:   string(agent.PolicyDecisionAllow),
			Status:     "empty",
			Summary:    "time range is ambiguous",
		}), nil
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
		return agent.MCPCallToolResult{}, err
	}

	contextMessages := transcriptEntriesToContextMessages(entries)
	content := formatConversationHistoryResult(conversationHistoryResultInput{
		Mode:         mode,
		Scope:        "current_session",
		Reason:       "model_tool_call",
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
			"tool_call_id":    input.CallID,
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
			"memory_scope":    "long_term_conversation",
			"reusable":        true,
		},
		RecalledRefs: domain.AgentJSON{
			"transcript_entry_ids": transcriptEntryIDs(entries),
			"evidence_refs":        transcriptEvidenceRefs(entries),
		},
		Reason:      "model_tool_call",
		BudgetChars: transcriptEntriesContentLength(entries),
		CreatedAt:   e.currentTime(),
	})
	if err != nil {
		return agent.MCPCallToolResult{}, err
	}
	return agent.NewMCPTextCallToolResult(content, false, observation), nil
}

func (e agentP0CapabilityExecutor) scheduleMessage(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	if e.scheduledTasks != nil {
		return e.scheduleTask(ctx, input)
	}
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionPrompt),
	}
	if e.notificationJobs == nil {
		observation.Status = "skipped"
		observation.Summary = "notification job store is unavailable"
		return agent.NewMCPTextCallToolResult("定时消息能力暂不可用。", true, observation), nil
	}
	args := parseScheduleMessageToolArgs(input.RawArguments)
	if args.TaskType == "" {
		args.TaskType = "reminder"
	}
	if args.TaskType != "reminder" && args.TaskType != "send_message" {
		observation.Status = "failed"
		observation.Summary = "unsupported scheduled task type"
		return agent.NewMCPTextCallToolResult("不支持该定时任务类型。", true, observation), nil
	}
	content := strings.TrimSpace(args.Content)
	if content == "" {
		observation.Status = "failed"
		observation.Summary = "scheduled content is empty"
		return agent.NewMCPTextCallToolResult("定时消息内容不能为空。", true, observation), nil
	}
	if strings.TrimSpace(input.ExternalUserID) == "" {
		observation.Status = "failed"
		observation.Summary = "wechat work recipient is missing"
		return agent.NewMCPTextCallToolResult("无法确定当前企微接收人，不能创建定时消息。", true, observation), nil
	}
	scheduledAt, parseResult := parseScheduleInstant(args.ScheduledAt, args.TimeHint, args.TimeZone, e.currentTime())
	if scheduledAt.IsZero() {
		observation.Status = "failed"
		observation.Summary = "scheduled time is ambiguous"
		return agent.NewMCPTextCallToolResult("工具状态：requires_clarification\n原因：没有明确的 scheduled_at，且 time_hint 无法被后端校验为具体时间点。请结合当前时间和最近上下文，让用户补充日期、上午/下午/晚上，或由模型归一化为 RFC3339 scheduled_at 后再次调用工具。", true, observation), nil
	}
	if scheduledAt.Before(e.currentTime().Add(-time.Minute)) {
		observation.Status = "failed"
		observation.Summary = "scheduled time is in the past"
		return agent.NewMCPTextCallToolResult("工具状态：failed\n原因：scheduled_at 已经过期，不能创建定时消息。", true, observation), nil
	}
	if !args.Confirmed {
		observation.Status = "requires_confirmation"
		observation.Summary = "scheduled message requires user confirmation"
		return agent.NewMCPTextCallToolResult(fmt.Sprintf("工具状态：requires_confirmation\n计划时间：%s\n提醒内容：%s\n说明：需要用户明确确认后才能创建；用户确认后必须再次调用 agent.schedule_message，并传 confirmed=true。", scheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), content), false, observation), nil
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
		return agent.MCPCallToolResult{}, err
	}
	observation.Decision = string(agent.PolicyDecisionAllow)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("scheduled notification job %d", created.ID)
	return agent.NewMCPTextCallToolResult(fmt.Sprintf("工具状态：created\n任务 ID：%d\n计划时间：%s\n提醒内容：%s", created.ID, created.ScheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), content), false, observation), nil
}

func (e agentP0CapabilityExecutor) scheduleTask(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionPrompt),
	}
	if e.scheduledTasks == nil {
		observation.Status = "skipped"
		observation.Summary = "scheduled task store is unavailable"
		return agent.NewMCPTextCallToolResult("定时 Agent 任务能力暂不可用。", true, observation), nil
	}
	args := parseScheduleMessageToolArgs(input.RawArguments)
	if args.TaskType == "" {
		args.TaskType = "agent_task"
		if input.Capability.Key == "agent.schedule_message" {
			args.TaskType = "reminder"
		}
	}
	goal := strings.TrimSpace(args.Goal)
	content := strings.TrimSpace(args.Content)
	if goal == "" {
		goal = content
	}
	if goal == "" {
		observation.Status = "failed"
		observation.Summary = "scheduled task goal is empty"
		return agent.NewMCPTextCallToolResult("定时任务目标不能为空。", true, observation), nil
	}
	scheduledAt, parseResult := parseScheduleInstant(args.ScheduledAt, args.TimeHint, args.TimeZone, e.currentTime())
	if scheduledAt.IsZero() {
		observation.Status = "failed"
		observation.Summary = "scheduled time is ambiguous"
		return agent.NewMCPTextCallToolResult("工具状态：requires_clarification\n原因：没有明确的 scheduled_at，且 time_hint 无法被后端校验为具体时间点。请结合当前时间和最近上下文，让用户补充日期、上午/下午/晚上，或由模型归一化为 RFC3339 scheduled_at 后再次调用工具。", true, observation), nil
	}
	if scheduledAt.Before(e.currentTime().Add(-time.Minute)) {
		observation.Status = "failed"
		observation.Summary = "scheduled time is in the past"
		return agent.NewMCPTextCallToolResult("工具状态：failed\n原因：scheduled_at 已经过期，不能创建定时任务。", true, observation), nil
	}
	if !args.Confirmed {
		observation.Status = "requires_confirmation"
		observation.Summary = "scheduled agent task requires user confirmation"
		return agent.NewMCPTextCallToolResult(fmt.Sprintf("工具状态：requires_confirmation\n计划时间：%s\n任务目标：%s\n说明：需要用户明确确认后才能创建；用户确认后必须再次调用 agent.schedule_task，并传 confirmed=true。", scheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), goal), false, observation), nil
	}
	targetChannel := args.TargetChannel
	if targetChannel == "" {
		targetChannel = "web"
		if strings.TrimSpace(input.ExternalUserID) != "" {
			targetChannel = "wechat_work_app"
		}
	}
	freshnessPolicy := args.FreshnessPolicy
	if freshnessPolicy == "" {
		freshnessPolicy = "latest_at_run"
	}
	allowedCapabilities := compactNonEmptyStrings(args.AllowedCapabilities)
	if len(allowedCapabilities) == 0 {
		allowedCapabilities = []string{"feed.query_recent_items", "conversation.query_history", "web.search", "web.fetch_page", "web.extract_page", "content.summarize_text"}
	}
	now := e.currentTime()
	task, err := e.scheduledTasks.CreateAgentScheduledTask(ctx, domain.AgentScheduledTask{
		UserID:              input.UserID,
		SessionID:           input.SessionID,
		TurnID:              input.TurnID,
		SourceRunID:         input.ControllerRunID,
		Status:              domain.AgentScheduledTaskStatusQueued,
		TaskType:            args.TaskType,
		Goal:                goal,
		TargetChannel:       targetChannel,
		TargetRef:           strings.TrimSpace(input.ExternalUserID),
		ScheduledAt:         scheduledAt.UTC(),
		FreshnessPolicy:     freshnessPolicy,
		AllowedCapabilities: allowedCapabilities,
		ModelPolicy: domain.AgentJSON{
			"model_key": "default",
		},
		FailurePolicy: domain.AgentJSON{
			"max_attempts": 3,
			"on_failure":   "report_to_user",
		},
		Payload: domain.AgentJSON{
			"content":         content,
			"time_hint":       args.TimeHint,
			"time_zone":       parseResult.TimeZone,
			"importance":      normalizedScheduleImportance(args.Importance),
			"source":          input.Capability.Key,
			"trigger_message": input.Message,
			"request_id":      input.RequestID,
			"trace_id":        input.TraceID,
		},
		MaxAttempts: 3,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return agent.MCPCallToolResult{}, err
	}
	observation.Decision = string(agent.PolicyDecisionAllow)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("scheduled agent task %d", task.ID)
	return agent.NewMCPTextCallToolResult(fmt.Sprintf("工具状态：created\n任务 ID：%d\n计划时间：%s\n任务目标：%s", task.ID, task.ScheduledAt.In(agentTimeLocation()).Format("2006-01-02 15:04"), task.Goal), false, observation), nil
}

func (e agentP0CapabilityExecutor) summarizeTextCapability(input agent.CapabilityExecuteInput) agent.CapabilityExecuteResult {
	content := formatSummarizeTextResult(summarizeTextToolArgs{Text: input.Message}, input.Message)
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "内容总结",
				CapabilityKey: input.Capability.Key,
				Content:       content,
				ItemCount:     1,
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "model_assisted",
			},
		},
		Observation: agent.CapabilityObservation{
			Capability: input.Capability.Key,
			Decision:   string(agent.PolicyDecisionAllow),
			Status:     "succeeded",
			Summary:    "generated structured text summary",
		},
	}
}

func (e agentP0CapabilityExecutor) summarizeTextTool(input agent.MCPCallToolInput) agent.MCPCallToolResult {
	args := parseSummarizeTextToolArgs(input.RawArguments)
	content := formatSummarizeTextResult(args, input.Message)
	return agent.NewMCPTextCallToolResult(content, false, agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
		Status:     "succeeded",
		Summary:    "generated structured text summary",
	})
}

func (e agentP0CapabilityExecutor) webSearch(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	args := parseWebSearchToolArgs(input.RawArguments)
	if args.Query == "" {
		args.Query = strings.TrimSpace(input.Message)
	}
	content, observation, _, err := e.runWebSearch(ctx, input.Capability.Key, args.Query, args.Limit, input.Message)
	return agent.NewMCPTextCallToolResult(content, observation.Status == "failed", observation), err
}

func (e agentP0CapabilityExecutor) runWebSearch(ctx context.Context, capabilityKey string, query string, limit int, userMessage string) (string, agent.CapabilityObservation, int, error) {
	query = normalizeWebSearchQuery(query)
	if limit < 1 {
		limit = 5
	}
	if limit > 8 {
		limit = 8
	}
	observation := agent.CapabilityObservation{
		Capability: capabilityKey,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if query == "" {
		observation.Status = "failed"
		observation.Summary = "web search query is empty"
		return "web.search 需要非空 query。", observation, 0, nil
	}
	now := e.currentTime()
	temporal := validateAgentToolTemporalRequest(query, userMessage, now)
	if !temporal.OK {
		observation.Status = "failed"
		observation.Summary = "web search query temporal validation failed: " + temporal.Status
		return formatAgentToolTemporalValidationFailure("web.search", query, temporal), observation, 0, nil
	}
	endpoint := "https://duckduckgo.com/html/?" + url.Values{"q": []string{query}}.Encode()
	body, finalURL, statusCode, contentType, err := e.fetchWebURL(ctx, endpoint)
	results := []agentWebSearchResult{}
	if err == nil && !isDuckDuckGoSearchChallenge(body, statusCode) {
		results = parseDuckDuckGoResults(body, limit)
	}
	if len(results) == 0 {
		for _, webEndpoint := range webHTMLSearchEndpoints(query) {
			webBody, webFinalURL, webStatusCode, webContentType, webErr := e.fetchWebURL(ctx, webEndpoint)
			if webErr != nil {
				err = webErr
				continue
			}
			webResults := parseBingResults(webBody, limit)
			if len(webResults) == 0 {
				finalURL = webFinalURL
				statusCode = webStatusCode
				contentType = webContentType
				continue
			}
			results = webResults
			finalURL = webFinalURL
			statusCode = webStatusCode
			contentType = webContentType
			err = nil
			break
		}
	}
	if len(results) == 0 {
		for _, rssEndpoint := range newsRSSSearchEndpoints(query) {
			rssBody, rssFinalURL, rssStatusCode, rssContentType, rssErr := e.fetchWebURL(ctx, rssEndpoint)
			if rssErr != nil {
				err = rssErr
				continue
			}
			rssResults := parseRSSSearchResults(rssBody, limit)
			if len(rssResults) == 0 {
				finalURL = rssFinalURL
				statusCode = rssStatusCode
				contentType = rssContentType
				continue
			}
			results = rssResults
			finalURL = rssFinalURL
			statusCode = rssStatusCode
			contentType = rssContentType
			err = nil
			break
		}
	}
	if err != nil && len(results) == 0 && finalURL == "" {
		observation.Status = "failed"
		observation.Summary = safeSummary(err.Error(), 300)
		return "web.search 执行失败：" + err.Error(), observation, 0, nil
	}
	rawCount := len(results)
	results, temporalCounts := filterAgentWebSearchResultsByTemporalEvidence(query, userMessage, now, results)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d web search results after temporal filter", len(results))
	if len(results) == 0 {
		observation.Status = "empty"
		if rawCount == 0 {
			observation.Summary = "no web search result parsed"
		} else {
			observation.Summary = "no current web search result remained after temporal filter"
		}
	}
	return formatWebSearchResult(query, finalURL, statusCode, contentType, now, results, temporalCounts), observation, len(results), nil
}

func (e agentP0CapabilityExecutor) webFetchPage(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	args := parseWebURLToolArgs(input.RawArguments)
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if args.URL == "" {
		observation.Status = "failed"
		observation.Summary = "web fetch url is empty"
		return agent.NewMCPTextCallToolResult("web.fetch_page 需要非空 url。", true, observation), nil
	}
	now := e.currentTime()
	temporal := validateAgentToolTemporalRequest(args.URL, input.Message, now)
	if !temporal.OK {
		observation.Status = "empty"
		observation.Summary = "web fetch url temporal validation failed: " + temporal.Status
		return agent.NewMCPTextCallToolResult(formatAgentToolTemporalValidationFailure("web.fetch_page", args.URL, temporal), true, observation), nil
	}
	body, finalURL, statusCode, contentType, err := e.fetchWebURL(ctx, args.URL)
	if err != nil {
		observation.Status = "failed"
		observation.Summary = safeSummary(err.Error(), 300)
		return agent.NewMCPTextCallToolResult("web.fetch_page 执行失败："+err.Error(), true, observation), nil
	}
	evidenceTemporal := assessAgentWebEvidenceTemporalStatus(args.URL, input.Message, strings.Join([]string{finalURL, string(body[:minInt(len(body), 4000)])}, " "), "", now)
	if !evidenceTemporal.OK {
		observation.Status = "empty"
		observation.Summary = "web fetch evidence temporal validation failed: " + evidenceTemporal.Status
		return agent.NewMCPTextCallToolResult(formatAgentToolTemporalValidationFailure("web.fetch_page", finalURL, evidenceTemporal), true, observation), nil
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("fetched %d bytes from %s", len(body), finalURL)
	return agent.NewMCPTextCallToolResult(formatWebFetchResult(finalURL, statusCode, contentType, now, body, evidenceTemporal), false, observation), nil
}

func (e agentP0CapabilityExecutor) webExtractPage(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	args := parseWebURLToolArgs(input.RawArguments)
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if args.URL == "" {
		observation.Status = "failed"
		observation.Summary = "web extract url is empty"
		return agent.NewMCPTextCallToolResult("web.extract_page 需要非空 url。", true, observation), nil
	}
	now := e.currentTime()
	temporal := validateAgentToolTemporalRequest(args.URL, input.Message, now)
	if !temporal.OK {
		observation.Status = "empty"
		observation.Summary = "web extract url temporal validation failed: " + temporal.Status
		return agent.NewMCPTextCallToolResult(formatAgentToolTemporalValidationFailure("web.extract_page", args.URL, temporal), true, observation), nil
	}
	body, finalURL, statusCode, contentType, err := e.fetchWebURL(ctx, args.URL)
	if err != nil {
		observation.Status = "failed"
		observation.Summary = safeSummary(err.Error(), 300)
		return agent.NewMCPTextCallToolResult("web.extract_page 执行失败："+err.Error(), true, observation), nil
	}
	extracted := extractAgentWebPage(body, finalURL)
	evidenceTemporal := assessAgentWebEvidenceTemporalStatus(args.URL, input.Message, strings.Join([]string{finalURL, extracted.Title, extracted.Summary}, " "), extracted.PublishedAt, now)
	if !evidenceTemporal.OK {
		observation.Status = "empty"
		observation.Summary = "web extract evidence temporal validation failed: " + evidenceTemporal.Status
		return agent.NewMCPTextCallToolResult(formatAgentToolTemporalValidationFailure("web.extract_page", finalURL, evidenceTemporal), true, observation), nil
	}
	observation.Status = "succeeded"
	observation.Summary = "extracted web page content"
	if extracted.Summary == "" && extracted.Title == "" {
		observation.Status = "empty"
		observation.Summary = "no readable page content extracted"
	}
	return agent.NewMCPTextCallToolResult(formatWebExtractResult(finalURL, statusCode, contentType, now, extracted, evidenceTemporal), false, observation), nil
}

func (e agentP0CapabilityExecutor) repoSearch(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	args := parseRepoSearchToolArgs(input.RawArguments)
	if args.Query == "" {
		args.Query = strings.TrimSpace(input.Message)
	}
	limit := args.Limit
	if limit < 1 {
		limit = 5
	}
	if limit > 8 {
		limit = 8
	}
	observation := agent.CapabilityObservation{Capability: input.Capability.Key, Decision: string(agent.PolicyDecisionAllow)}
	if args.Query == "" {
		observation.Status = "failed"
		observation.Summary = "repo search query is empty"
		return agent.NewMCPTextCallToolResult("repo.search 需要非空 query。", true, observation), nil
	}
	endpoint := "https://api.github.com/search/repositories?" + url.Values{
		"q":        []string{args.Query},
		"per_page": []string{strconv.Itoa(limit)},
	}.Encode()
	body, finalURL, statusCode, contentType, err := e.fetchWebURL(ctx, endpoint)
	if err != nil {
		observation.Status = "failed"
		observation.Summary = safeSummary(err.Error(), 300)
		return agent.NewMCPTextCallToolResult("repo.search 执行失败："+err.Error(), true, observation), nil
	}
	results := parseGitHubRepoSearchResults(body, limit)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d repository results", len(results))
	if len(results) == 0 {
		observation.Status = "empty"
		observation.Summary = "no repository result parsed"
	}
	return agent.NewMCPTextCallToolResult(formatRepoSearchResult(args.Query, finalURL, statusCode, contentType, e.currentTime(), results), false, observation), nil
}

func (e agentP0CapabilityExecutor) repoInspectRemote(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	args := parseRepoInspectToolArgs(input.RawArguments)
	if args.Repo == "" {
		args.Repo = extractRepoRef(input.Message)
	}
	observation := agent.CapabilityObservation{Capability: input.Capability.Key, Decision: string(agent.PolicyDecisionAllow)}
	owner, repo, ok := parseGitHubRepoRef(args.Repo)
	if !ok {
		observation.Status = "failed"
		observation.Summary = "github repository reference is invalid"
		return agent.NewMCPTextCallToolResult("repo.inspect_remote 需要 GitHub URL 或 owner/repo。", true, observation), nil
	}
	metaURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	metaBody, finalURL, statusCode, contentType, err := e.fetchWebURL(ctx, metaURL)
	if err != nil {
		observation.Status = "failed"
		observation.Summary = safeSummary(err.Error(), 300)
		return agent.NewMCPTextCallToolResult("repo.inspect_remote 执行失败："+err.Error(), true, observation), nil
	}
	meta := parseGitHubRepoMetadata(metaBody)
	readme := fetchGitHubReadmeSummary(ctx, owner, repo)
	license := fetchGitHubLicenseSummary(ctx, owner, repo)
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("inspected remote repository %s/%s", owner, repo)
	return agent.NewMCPTextCallToolResult(formatRepoInspectResult(finalURL, statusCode, contentType, e.currentTime(), meta, readme, license), false, observation), nil
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
	args.Goal = strings.TrimSpace(args.Goal)
	args.Content = strings.TrimSpace(args.Content)
	args.ScheduledAt = strings.TrimSpace(args.ScheduledAt)
	args.TimeHint = strings.TrimSpace(args.TimeHint)
	args.TimeZone = strings.TrimSpace(args.TimeZone)
	args.Importance = strings.TrimSpace(args.Importance)
	args.TargetChannel = strings.TrimSpace(args.TargetChannel)
	args.FreshnessPolicy = strings.TrimSpace(args.FreshnessPolicy)
	for index, capability := range args.AllowedCapabilities {
		args.AllowedCapabilities[index] = strings.TrimSpace(capability)
	}
	return args
}

func parseWebSearchToolArgs(raw string) webSearchToolArgs {
	var args webSearchToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return webSearchToolArgs{}
	}
	args.Query = strings.TrimSpace(args.Query)
	return args
}

func parseWebURLToolArgs(raw string) webURLToolArgs {
	var args webURLToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return webURLToolArgs{}
	}
	args.URL = strings.TrimSpace(args.URL)
	return args
}

func parseSummarizeTextToolArgs(raw string) summarizeTextToolArgs {
	var args summarizeTextToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return summarizeTextToolArgs{}
	}
	args.Text = strings.TrimSpace(args.Text)
	for index := range args.Sources {
		args.Sources[index].Title = strings.TrimSpace(args.Sources[index].Title)
		args.Sources[index].URL = strings.TrimSpace(args.Sources[index].URL)
		args.Sources[index].Content = strings.TrimSpace(args.Sources[index].Content)
		args.Sources[index].Summary = strings.TrimSpace(args.Sources[index].Summary)
	}
	for index, ref := range args.SourceRefs {
		args.SourceRefs[index] = strings.TrimSpace(ref)
	}
	return args
}

func parseRepoSearchToolArgs(raw string) repoSearchToolArgs {
	var args repoSearchToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return repoSearchToolArgs{}
	}
	args.Query = strings.TrimSpace(args.Query)
	return args
}

func parseRepoInspectToolArgs(raw string) repoInspectToolArgs {
	var args repoInspectToolArgs
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return repoInspectToolArgs{}
	}
	args.Repo = strings.TrimSpace(args.Repo)
	return args
}

type conversationHistoryResultInput struct {
	Mode         string
	Scope        string
	Reason       string
	Entries      []domain.AgentTranscriptEntry
	MatchedCount int
	TimeRange    conversationHistoryTimeRange
}

func formatConversationHistoryResult(input conversationHistoryResultInput) string {
	mode := inferConversationHistoryMode(input.Mode)
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
	builder.WriteString("\n新鲜度提示：历史对话结果属于同用户会话记忆，30 天内可作为上下文引用。")
	if strings.TrimSpace(input.Reason) != "" {
		builder.WriteString("\n召回原因：")
		builder.WriteString(strings.TrimSpace(input.Reason))
	}
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
	builder.WriteString("\nEvidence refs：")
	refs := make([]string, 0, len(input.Entries))
	for _, entry := range input.Entries {
		if entry.ID > 0 {
			refs = append(refs, "agent_transcript_entry:"+strconv.FormatInt(entry.ID, 10))
		}
	}
	builder.WriteString(strings.Join(refs, ", "))
	return builder.String()
}

func conversationHistoryBoundaryMetadata(mode string, matchedCount int) domain.AgentJSON {
	mode = inferConversationHistoryMode(mode)
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

func transcriptEvidenceRefs(entries []domain.AgentTranscriptEntry) []string {
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.ID > 0 {
			refs = append(refs, "agent_transcript_entry:"+strconv.FormatInt(entry.ID, 10))
		}
	}
	return refs
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

func inferConversationHistoryMode(requested string) string {
	requested = strings.ToLower(strings.TrimSpace(requested))
	switch requested {
	case conversationHistoryModeEarliest, conversationHistoryModeLatest, conversationHistoryModeSearch, conversationHistoryModeTimeRange:
		return requested
	}
	return conversationHistoryModeSearch
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

type agentWebSearchResult struct {
	Title       string
	URL         string
	Snippet     string
	Source      string
	PublishedAt string
}

type agentWebExtractedPage struct {
	Title       string
	SiteName    string
	Summary     string
	PublishedAt string
	Author      string
	Links       []agentWebSearchResult
}

type agentRepoSearchResult struct {
	FullName    string
	URL         string
	Description string
	Language    string
	License     string
	Stars       int
	UpdatedAt   string
}

type agentRepoMetadata struct {
	FullName      string
	URL           string
	Description   string
	DefaultBranch string
	Language      string
	License       string
	Stars         int
	Forks         int
	UpdatedAt     string
}

type agentRepoDocumentSummary struct {
	Source  string
	Summary string
	Error   string
}

func (e agentP0CapabilityExecutor) fetchWebURL(ctx context.Context, rawURL string) ([]byte, string, int, string, error) {
	if e.webFetcher != nil {
		return e.webFetcher(ctx, rawURL)
	}
	return fetchAgentWebURL(ctx, rawURL)
}

func fetchAgentWebURL(ctx context.Context, rawURL string) ([]byte, string, int, string, error) {
	parsed, err := validateAgentWebURL(rawURL)
	if err != nil {
		return nil, "", 0, "", err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", 0, "", err
	}
	request.Header.Set("User-Agent", "messageFeed-agent/0.1")
	response, err := client.Do(request)
	if err != nil {
		return nil, "", 0, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, agentWebFetchMaxBytes))
	if err != nil {
		return nil, "", response.StatusCode, response.Header.Get("Content-Type"), err
	}
	finalURL := parsed.String()
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return body, finalURL, response.StatusCode, response.Header.Get("Content-Type"), fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	return body, finalURL, response.StatusCode, response.Header.Get("Content-Type"), nil
}

func validateAgentWebURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return nil, fmt.Errorf("missing URL host")
	}
	if isBlockedAgentWebHost(host) {
		return nil, fmt.Errorf("blocked local or private host")
	}
	return parsed, nil
}

func isBlockedAgentWebHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func parseDuckDuckGoResults(body []byte, limit int) []agentWebSearchResult {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	results := make([]agentWebSearchResult, 0, limit)
	document.Find("a.result__a").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if len(results) >= limit {
			return false
		}
		title := strings.TrimSpace(selection.Text())
		href, _ := selection.Attr("href")
		href = normalizeDuckDuckGoResultURL(href)
		if title == "" || href == "" {
			return true
		}
		snippet := strings.TrimSpace(selection.ParentsFiltered(".result").First().Find(".result__snippet").Text())
		results = append(results, agentWebSearchResult{Title: title, URL: href, Snippet: cleanWhitespace(snippet), Source: hostNameForURL(href)})
		return true
	})
	if len(results) > 0 {
		return results
	}
	document.Find("a[href]").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if len(results) >= limit {
			return false
		}
		title := strings.TrimSpace(selection.Text())
		href, _ := selection.Attr("href")
		href = normalizeDuckDuckGoResultURL(href)
		if title == "" || href == "" || strings.HasPrefix(href, "#") {
			return true
		}
		parsed, err := url.Parse(href)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return true
		}
		results = append(results, agentWebSearchResult{Title: title, URL: href, Source: hostNameForURL(href)})
		return true
	})
	return results
}

func parseBingResults(body []byte, limit int) []agentWebSearchResult {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	results := make([]agentWebSearchResult, 0, limit)
	document.Find("li.b_algo").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if len(results) >= limit {
			return false
		}
		link := selection.Find("h2 a").First()
		title := cleanWhitespace(link.Text())
		href, _ := link.Attr("href")
		href = strings.TrimSpace(href)
		if title == "" || href == "" {
			return true
		}
		parsed, err := url.Parse(href)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return true
		}
		snippet := cleanWhitespace(selection.Find(".b_caption p").First().Text())
		if snippet == "" {
			snippet = cleanWhitespace(selection.Find("p").First().Text())
		}
		results = append(results, agentWebSearchResult{
			Title:   title,
			URL:     href,
			Snippet: snippet,
			Source:  hostNameForURL(href),
		})
		return true
	})
	return results
}

func normalizeWebSearchQuery(value string) string {
	return safeSummary(cleanWhitespace(value), 160)
}

func newsRSSSearchEndpoints(query string) []string {
	values := url.Values{
		"q":    []string{query},
		"hl":   []string{"zh-CN"},
		"gl":   []string{"CN"},
		"ceid": []string{"CN:zh-Hans"},
	}
	googleNews := "https://news.google.com/rss/search?" + values.Encode()
	bingNews := "https://www.bing.com/news/search?" + url.Values{
		"q":       []string{query},
		"format":  []string{"rss"},
		"setlang": []string{"zh-Hans"},
		"mkt":     []string{"zh-CN"},
	}.Encode()
	return []string{googleNews, bingNews}
}

func webHTMLSearchEndpoints(query string) []string {
	bingWeb := "https://www.bing.com/search?" + url.Values{
		"q":       []string{query},
		"setlang": []string{"zh-Hans"},
		"mkt":     []string{"zh-CN"},
	}.Encode()
	return []string{bingWeb}
}

func isDuckDuckGoSearchChallenge(body []byte, statusCode int) bool {
	if statusCode == http.StatusAccepted {
		return true
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "anomaly-modal") ||
		strings.Contains(lower, "duckduckgo") && strings.Contains(lower, "challenge")
}

type rssSearchFeed struct {
	Channel struct {
		Items []rssSearchItem `xml:"item"`
	} `xml:"channel"`
	Entries []rssSearchItem `xml:"entry"`
}

type rssSearchItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Summary     string `xml:"summary"`
	PubDate     string `xml:"pubDate"`
	Updated     string `xml:"updated"`
	Published   string `xml:"published"`
	Source      struct {
		Name string `xml:",chardata"`
	} `xml:"source"`
}

func parseRSSSearchResults(body []byte, limit int) []agentWebSearchResult {
	if limit < 1 {
		limit = 5
	}
	var feed rssSearchFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil
	}
	items := feed.Channel.Items
	if len(items) == 0 {
		items = feed.Entries
	}
	results := make([]agentWebSearchResult, 0, limit)
	seen := map[string]struct{}{}
	for _, item := range items {
		if len(results) >= limit {
			break
		}
		title := cleanWhitespace(html.UnescapeString(item.Title))
		rawURL := cleanWhitespace(html.UnescapeString(item.Link))
		if title == "" || rawURL == "" {
			continue
		}
		key := rawURL
		if key == "" {
			key = title
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		summary := cleanHTMLText(item.Description)
		if summary == "" {
			summary = cleanHTMLText(item.Summary)
		}
		source := cleanWhitespace(html.UnescapeString(item.Source.Name))
		if source == "" {
			source = hostNameForURL(rawURL)
		}
		results = append(results, agentWebSearchResult{
			Title:       title,
			URL:         rawURL,
			Snippet:     safeSummary(summary, 300),
			Source:      source,
			PublishedAt: firstNonEmptyString(item.PubDate, item.Published, item.Updated),
		})
	}
	return results
}

func cleanHTMLText(value string) string {
	value = html.UnescapeString(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return cleanWhitespace(value)
	}
	return cleanWhitespace(document.Text())
}

func hostNameForURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := cleanWhitespace(html.UnescapeString(value)); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeDuckDuckGoResultURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil {
		if uddg := strings.TrimSpace(parsed.Query().Get("uddg")); uddg != "" {
			if decoded, decodeErr := url.QueryUnescape(uddg); decodeErr == nil {
				return decoded
			}
			return uddg
		}
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	return value
}

func extractAgentWebPage(body []byte, sourceURL string) agentWebExtractedPage {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return agentWebExtractedPage{}
	}
	document.Find("script, style, noscript, svg").Each(func(_ int, selection *goquery.Selection) {
		selection.Remove()
	})
	page := agentWebExtractedPage{
		Title:       cleanWhitespace(document.Find("title").First().Text()),
		SiteName:    cleanWhitespace(firstMetaContent(document, "property", "og:site_name")),
		PublishedAt: cleanWhitespace(firstMetaContent(document, "property", "article:published_time")),
		Author:      cleanWhitespace(firstMetaContent(document, "name", "author")),
	}
	if page.Title == "" {
		page.Title = cleanWhitespace(firstMetaContent(document, "property", "og:title"))
	}
	summary := cleanWhitespace(firstMetaContent(document, "name", "description"))
	if summary == "" {
		summary = cleanWhitespace(firstMetaContent(document, "property", "og:description"))
	}
	text := cleanWhitespace(document.Find("article").First().Text())
	if text == "" {
		text = cleanWhitespace(document.Find("main").First().Text())
	}
	if text == "" {
		text = cleanWhitespace(document.Find("body").Text())
	}
	if summary != "" && !strings.Contains(text, summary) {
		text = summary + "\n" + text
	}
	page.Summary = safeSummary(text, 2500)
	page.Links = extractAgentWebLinks(document, sourceURL, 8)
	return page
}

func firstMetaContent(document *goquery.Document, attr string, value string) string {
	content, _ := document.Find(fmt.Sprintf("meta[%s='%s']", attr, value)).First().Attr("content")
	return content
}

func extractAgentWebLinks(document *goquery.Document, sourceURL string, limit int) []agentWebSearchResult {
	base, _ := url.Parse(sourceURL)
	links := make([]agentWebSearchResult, 0, limit)
	seen := map[string]struct{}{}
	document.Find("a[href]").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if len(links) >= limit {
			return false
		}
		title := cleanWhitespace(selection.Text())
		href, _ := selection.Attr("href")
		if href == "" || strings.HasPrefix(href, "#") {
			return true
		}
		parsed, err := url.Parse(href)
		if err != nil {
			return true
		}
		if base != nil {
			parsed = base.ResolveReference(parsed)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return true
		}
		normalized := parsed.String()
		if _, ok := seen[normalized]; ok {
			return true
		}
		seen[normalized] = struct{}{}
		if title == "" {
			title = normalized
		}
		links = append(links, agentWebSearchResult{Title: safeSummary(title, 120), URL: normalized})
		return true
	})
	return links
}

func formatWebSearchResult(query string, source string, statusCode int, contentType string, fetchedAt time.Time, results []agentWebSearchResult, temporalCounts map[string]int) string {
	var builder strings.Builder
	builder.WriteString("工具：web.search\n查询：")
	builder.WriteString(query)
	builder.WriteString("\n来源：")
	builder.WriteString(source)
	builder.WriteString("\n抓取时间：")
	builder.WriteString(fetchedAt.UTC().Format(time.RFC3339))
	builder.WriteString("\nHTTP 状态：")
	builder.WriteString(strconv.Itoa(statusCode))
	builder.WriteString("\n内容类型：")
	builder.WriteString(contentType)
	builder.WriteString("\n证据引用：web_search:")
	builder.WriteString(source)
	builder.WriteString("\n新鲜度提示：联网结果 6 小时后建议刷新。")
	builder.WriteString("\n证据时效过滤：")
	builder.WriteString(formatAgentTemporalFilterSummary(temporalCounts))
	builder.WriteString("\n结果：\n")
	for index, result := range results {
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(result.Title)
		if result.Source != "" {
			builder.WriteString("（")
			builder.WriteString(result.Source)
			builder.WriteString("）")
		}
		if result.PublishedAt != "" {
			builder.WriteString("\n发布时间：")
			builder.WriteString(result.PublishedAt)
		}
		builder.WriteString("\nURL：")
		builder.WriteString(result.URL)
		builder.WriteString("\nEvidence ref：url:")
		builder.WriteString(result.URL)
		if result.Snippet != "" {
			builder.WriteString("\n摘要：")
			builder.WriteString(result.Snippet)
		}
		builder.WriteString("\n")
	}
	if len(results) == 0 {
		builder.WriteString("没有解析到候选结果。\n")
	}
	return builder.String()
}

func formatWebFetchResult(source string, statusCode int, contentType string, fetchedAt time.Time, body []byte, temporal agentTemporalValidationResult) string {
	return fmt.Sprintf(
		"工具：web.fetch_page\n来源：%s\n抓取时间：%s\nHTTP 状态：%d\n内容类型：%s\n证据引用：url:%s\n新鲜度提示：联网结果 6 小时后建议刷新。\n证据时效状态：%s\n字节数：%d\n正文片段：\n%s",
		source,
		fetchedAt.UTC().Format(time.RFC3339),
		statusCode,
		contentType,
		source,
		temporal.Status,
		len(body),
		safeSummary(string(body), 4000),
	)
}

func formatWebExtractResult(source string, statusCode int, contentType string, fetchedAt time.Time, page agentWebExtractedPage, temporal agentTemporalValidationResult) string {
	var builder strings.Builder
	builder.WriteString("工具：web.extract_page\n来源：")
	builder.WriteString(source)
	builder.WriteString("\n抓取时间：")
	builder.WriteString(fetchedAt.UTC().Format(time.RFC3339))
	builder.WriteString("\nHTTP 状态：")
	builder.WriteString(strconv.Itoa(statusCode))
	builder.WriteString("\n内容类型：")
	builder.WriteString(contentType)
	builder.WriteString("\n证据引用：url:")
	builder.WriteString(source)
	builder.WriteString("\n新鲜度提示：联网结果 6 小时后建议刷新。")
	builder.WriteString("\n证据时效状态：")
	builder.WriteString(temporal.Status)
	builder.WriteString("\n标题：")
	builder.WriteString(page.Title)
	if page.SiteName != "" {
		builder.WriteString("\n站点：")
		builder.WriteString(page.SiteName)
	}
	if page.Author != "" {
		builder.WriteString("\n作者：")
		builder.WriteString(page.Author)
	}
	if page.PublishedAt != "" {
		builder.WriteString("\n发布时间：")
		builder.WriteString(page.PublishedAt)
	}
	builder.WriteString("\n正文摘要：\n")
	builder.WriteString(page.Summary)
	builder.WriteString("\n主要链接：\n")
	for index, link := range page.Links {
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(link.Title)
		builder.WriteString("\nURL：")
		builder.WriteString(link.URL)
		builder.WriteString("\nEvidence ref：url:")
		builder.WriteString(link.URL)
		builder.WriteString("\n")
	}
	if len(page.Links) == 0 {
		builder.WriteString("没有解析到主要链接。\n")
	}
	return builder.String()
}

func formatAgentToolTemporalValidationFailure(tool string, value string, result agentTemporalValidationResult) string {
	var builder strings.Builder
	builder.WriteString("工具：")
	builder.WriteString(tool)
	builder.WriteString("\n状态：时间校验未通过")
	builder.WriteString("\n校验类型：")
	builder.WriteString(result.Status)
	if strings.TrimSpace(result.Reason) != "" {
		builder.WriteString("\n原因：")
		builder.WriteString(result.Reason)
	}
	if len(result.Matched) > 0 {
		builder.WriteString("\n命中日期：")
		builder.WriteString(strings.Join(result.Matched, ", "))
	}
	builder.WriteString("\n原始参数：")
	builder.WriteString(value)
	builder.WriteString("\n处理建议：请重新生成与用户原始时间要求一致的工具参数，并避免使用未来日期、错年日期或过期证据。")
	return builder.String()
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func formatSummarizeTextResult(args summarizeTextToolArgs, fallback string) string {
	text := strings.TrimSpace(args.Text)
	if text == "" {
		text = strings.TrimSpace(fallback)
	}
	if text == "" {
		for _, source := range args.Sources {
			text = strings.TrimSpace(strings.Join([]string{text, source.Title, source.Summary, source.Content}, " "))
		}
	}
	conclusions := summarizeKeyConclusions(text, args.Sources)
	risks := summarizeRisks(text, args.Sources)
	refs := append([]string(nil), args.SourceRefs...)
	for _, source := range args.Sources {
		if source.URL != "" {
			refs = append(refs, "url:"+source.URL)
		} else if source.Title != "" {
			refs = append(refs, "source:"+source.Title)
		}
	}
	if len(refs) == 0 {
		refs = append(refs, "input:message")
	}
	var builder strings.Builder
	builder.WriteString("工具：content.summarize_text\n关键结论：\n")
	for index, conclusion := range conclusions {
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(conclusion)
		builder.WriteString("\n")
	}
	builder.WriteString("风险提示：\n")
	for index, risk := range risks {
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(risk)
		builder.WriteString("\n")
	}
	builder.WriteString("引用列表：\n")
	for index, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(ref)
		builder.WriteString("\n")
	}
	builder.WriteString("Evidence refs：")
	builder.WriteString(strings.Join(compactNonEmptyStrings(refs), ", "))
	builder.WriteString("\n新鲜度提示：文本总结继承输入来源的新鲜度；多来源总结需要按最旧来源复核。")
	return builder.String()
}

func summarizeKeyConclusions(text string, sources []summarizeTextSource) []string {
	conclusions := make([]string, 0, 4)
	for _, source := range sources {
		summary := strings.TrimSpace(source.Summary)
		if summary == "" {
			summary = strings.TrimSpace(source.Content)
		}
		if summary != "" {
			prefix := strings.TrimSpace(source.Title)
			if prefix != "" {
				conclusions = append(conclusions, prefix+"："+safeSummary(cleanWhitespace(summary), 180))
			} else {
				conclusions = append(conclusions, safeSummary(cleanWhitespace(summary), 180))
			}
		}
		if len(conclusions) >= 4 {
			break
		}
	}
	if len(conclusions) == 0 {
		conclusions = append(conclusions, safeSummary(cleanWhitespace(text), 240))
	}
	if len(conclusions) == 0 || strings.TrimSpace(conclusions[0]) == "" {
		return []string{"没有足够文本生成可靠结论。"}
	}
	return conclusions
}

func summarizeRisks(text string, sources []summarizeTextSource) []string {
	risks := make([]string, 0, 3)
	if len(sources) > 1 {
		risks = append(risks, "多来源内容可能存在时间差或口径差异，引用时应保留来源。")
	}
	if len(risks) == 0 {
		risks = append(risks, "风险判断需要结合原文上下文和最终回答复核。")
	}
	return risks
}

func cleanWhitespace(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func parseGitHubRepoSearchResults(body []byte, limit int) []agentRepoSearchResult {
	var decoded struct {
		Items []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			Language    string `json:"language"`
			Stars       int    `json:"stargazers_count"`
			UpdatedAt   string `json:"updated_at"`
			License     *struct {
				Name string `json:"name"`
			} `json:"license"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil
	}
	results := make([]agentRepoSearchResult, 0, limit)
	for _, item := range decoded.Items {
		if len(results) >= limit {
			break
		}
		licenseName := ""
		if item.License != nil {
			licenseName = item.License.Name
		}
		results = append(results, agentRepoSearchResult{
			FullName:    item.FullName,
			URL:         item.HTMLURL,
			Description: cleanWhitespace(item.Description),
			Language:    item.Language,
			License:     licenseName,
			Stars:       item.Stars,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return results
}

func parseGitHubRepoMetadata(body []byte) agentRepoMetadata {
	var decoded struct {
		FullName      string `json:"full_name"`
		HTMLURL       string `json:"html_url"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		Language      string `json:"language"`
		Stars         int    `json:"stargazers_count"`
		Forks         int    `json:"forks_count"`
		UpdatedAt     string `json:"updated_at"`
		License       *struct {
			Name string `json:"name"`
		} `json:"license"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return agentRepoMetadata{}
	}
	licenseName := ""
	if decoded.License != nil {
		licenseName = decoded.License.Name
	}
	return agentRepoMetadata{
		FullName:      decoded.FullName,
		URL:           decoded.HTMLURL,
		Description:   cleanWhitespace(decoded.Description),
		DefaultBranch: decoded.DefaultBranch,
		Language:      decoded.Language,
		License:       licenseName,
		Stars:         decoded.Stars,
		Forks:         decoded.Forks,
		UpdatedAt:     decoded.UpdatedAt,
	}
}

func fetchGitHubReadmeSummary(ctx context.Context, owner string, repo string) agentRepoDocumentSummary {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/readme", url.PathEscape(owner), url.PathEscape(repo))
	body, finalURL, _, _, err := fetchAgentWebURL(ctx, endpoint)
	if err != nil {
		return agentRepoDocumentSummary{Source: endpoint, Error: err.Error()}
	}
	content, err := decodeGitHubContent(body)
	if err != nil {
		return agentRepoDocumentSummary{Source: finalURL, Error: err.Error()}
	}
	return agentRepoDocumentSummary{Source: finalURL, Summary: safeSummary(cleanWhitespace(content), 1600)}
}

func fetchGitHubLicenseSummary(ctx context.Context, owner string, repo string) agentRepoDocumentSummary {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/license", url.PathEscape(owner), url.PathEscape(repo))
	body, finalURL, _, _, err := fetchAgentWebURL(ctx, endpoint)
	if err != nil {
		return agentRepoDocumentSummary{Source: endpoint, Error: err.Error()}
	}
	content, err := decodeGitHubContent(body)
	if err != nil {
		return agentRepoDocumentSummary{Source: finalURL, Error: err.Error()}
	}
	return agentRepoDocumentSummary{Source: finalURL, Summary: safeSummary(cleanWhitespace(content), 800)}
}

func decodeGitHubContent(body []byte) (string, error) {
	var decoded struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", err
	}
	if decoded.Encoding != "base64" {
		return "", fmt.Errorf("unsupported GitHub content encoding")
	}
	raw := strings.ReplaceAll(decoded.Content, "\n", "")
	payload, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func parseGitHubRepoRef(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	if strings.Contains(value, "github.com/") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", "", false
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git"), true
		}
	}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return strings.TrimSpace(parts[0]), strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git"), true
	}
	return "", "", false
}

func extractRepoRef(message string) string {
	for _, field := range strings.Fields(message) {
		field = strings.Trim(field, "，。,. ")
		if _, _, ok := parseGitHubRepoRef(field); ok {
			return field
		}
	}
	return ""
}

func formatRepoSearchResult(query string, source string, statusCode int, contentType string, fetchedAt time.Time, results []agentRepoSearchResult) string {
	var builder strings.Builder
	builder.WriteString("工具：repo.search\n查询：")
	builder.WriteString(query)
	builder.WriteString("\n来源：")
	builder.WriteString(source)
	builder.WriteString("\n抓取时间：")
	builder.WriteString(fetchedAt.UTC().Format(time.RFC3339))
	builder.WriteString("\nHTTP 状态：")
	builder.WriteString(strconv.Itoa(statusCode))
	builder.WriteString("\n内容类型：")
	builder.WriteString(contentType)
	builder.WriteString("\n结果：\n")
	for index, result := range results {
		builder.WriteString(strconv.Itoa(index + 1))
		builder.WriteString(". ")
		builder.WriteString(result.FullName)
		builder.WriteString("\nURL：")
		builder.WriteString(result.URL)
		if result.Description != "" {
			builder.WriteString("\n摘要：")
			builder.WriteString(result.Description)
		}
		builder.WriteString("\n语言：")
		builder.WriteString(result.Language)
		builder.WriteString("\n许可：")
		builder.WriteString(result.License)
		builder.WriteString("\nStars：")
		builder.WriteString(strconv.Itoa(result.Stars))
		builder.WriteString("\n更新时间：")
		builder.WriteString(result.UpdatedAt)
		builder.WriteString("\n")
	}
	if len(results) == 0 {
		builder.WriteString("没有解析到仓库候选。\n")
	}
	return builder.String()
}

func formatRepoInspectResult(source string, statusCode int, contentType string, fetchedAt time.Time, meta agentRepoMetadata, readme agentRepoDocumentSummary, license agentRepoDocumentSummary) string {
	var builder strings.Builder
	builder.WriteString("工具：repo.inspect_remote\n来源：")
	builder.WriteString(source)
	builder.WriteString("\n抓取时间：")
	builder.WriteString(fetchedAt.UTC().Format(time.RFC3339))
	builder.WriteString("\nHTTP 状态：")
	builder.WriteString(strconv.Itoa(statusCode))
	builder.WriteString("\n内容类型：")
	builder.WriteString(contentType)
	builder.WriteString("\n仓库：")
	builder.WriteString(meta.FullName)
	builder.WriteString("\nURL：")
	builder.WriteString(meta.URL)
	builder.WriteString("\n描述：")
	builder.WriteString(meta.Description)
	builder.WriteString("\n默认分支：")
	builder.WriteString(meta.DefaultBranch)
	builder.WriteString("\n语言：")
	builder.WriteString(meta.Language)
	builder.WriteString("\n许可：")
	builder.WriteString(meta.License)
	builder.WriteString("\nStars：")
	builder.WriteString(strconv.Itoa(meta.Stars))
	builder.WriteString("\nForks：")
	builder.WriteString(strconv.Itoa(meta.Forks))
	builder.WriteString("\n更新时间：")
	builder.WriteString(meta.UpdatedAt)
	builder.WriteString("\nREADME 来源：")
	builder.WriteString(readme.Source)
	builder.WriteString("\nREADME 摘要：")
	if readme.Error != "" {
		builder.WriteString("读取失败：")
		builder.WriteString(readme.Error)
	} else {
		builder.WriteString(readme.Summary)
	}
	builder.WriteString("\nLicense 来源：")
	builder.WriteString(license.Source)
	builder.WriteString("\nLicense 摘要：")
	if license.Error != "" {
		builder.WriteString("读取失败：")
		builder.WriteString(license.Error)
	} else {
		builder.WriteString(license.Summary)
	}
	return builder.String()
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

func agentContextBlocksContentLength(blocks []agent.ContextBlock) int {
	total := 0
	for _, block := range blocks {
		total += len([]rune(strings.TrimSpace(block.Content)))
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

func compactNonEmptyStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		compacted = append(compacted, value)
	}
	return compacted
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
		if item.PublishedAt != nil {
			builder.WriteString("\n发布时间：")
			builder.WriteString(item.PublishedAt.UTC().Format(time.RFC3339))
		} else if !item.FetchedAt.IsZero() {
			builder.WriteString("\n抓取时间：")
			builder.WriteString(item.FetchedAt.UTC().Format(time.RFC3339))
		}
		builder.WriteString("\n已读：")
		if item.IsRead {
			builder.WriteString("是")
		} else {
			builder.WriteString("否")
		}
		if item.URL != "" {
			builder.WriteString("\nURL：")
			builder.WriteString(item.URL)
		}
		builder.WriteString("\nEvidence ref：item:")
		builder.WriteString(strconv.FormatInt(item.ID, 10))
		if item.Summary != "" {
			builder.WriteString("\n摘要：")
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
		if item.PublishedAt != nil {
			builder.WriteString("\n发布时间：")
			builder.WriteString(item.PublishedAt.UTC().Format(time.RFC3339))
		}
		if item.URL != "" {
			builder.WriteString("\nURL：")
			builder.WriteString(item.URL)
		}
		builder.WriteString("\nEvidence ref：item:")
		builder.WriteString(strconv.FormatInt(item.ID, 10))
	}
	return builder.String()
}
