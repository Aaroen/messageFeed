package service

import (
	"context"
	"errors"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAgentConversationServiceReceivesBoundAccountAndSendsAIReply(t *testing.T) {
	now := time.Date(2026, 6, 24, 17, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	userContext := &fakeAgentUserContextProvider{}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "这是 AI 回复",
		},
	}
	sender := &fakeAgentConversationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-msg-1"}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationUserContextProvider(userContext),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "最近有什么更新",
		RequestID:         "request-1",
		TraceID:           "trace-1",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}

	if result.ExternalAccount.ID != resolver.account.ID {
		t.Fatalf("external account ID = %d, want %d", result.ExternalAccount.ID, resolver.account.ID)
	}
	if result.ExternalAccount.UserID != resolver.account.UserID {
		t.Fatalf("external account UserID = %d, want %d", result.ExternalAccount.UserID, resolver.account.UserID)
	}
	if result.InboundMessage.ProviderMessageID != "msg-1" {
		t.Fatalf("ProviderMessageID = %q", result.InboundMessage.ProviderMessageID)
	}
	if result.Session.ChannelSessionKey != "corp-a:1000002:zhangsan" {
		t.Fatalf("ChannelSessionKey = %q", result.Session.ChannelSessionKey)
	}
	if result.Turn.Status != domain.AgentTurnStatusSucceeded {
		t.Fatalf("Turn status = %q", result.Turn.Status)
	}
	if result.Turn.ModelProvider != "openai_compatible" || result.Turn.Model != "custom-model" {
		t.Fatalf("model info = %q/%q", result.Turn.ModelProvider, result.Turn.Model)
	}
	if result.Reply != "这是 AI 回复" {
		t.Fatalf("Reply = %q", result.Reply)
	}
	if sender.sent.ToUser != "zhangsan" {
		t.Fatalf("sent ToUser = %q", sender.sent.ToUser)
	}
	if sender.sent.Content != "这是 AI 回复" {
		t.Fatalf("sent Content = %q", sender.sent.Content)
	}
	if sender.templateCalls != 1 || sender.sentTemplate.ToUser != "zhangsan" || sender.sentTemplate.URL == "" {
		t.Fatalf("final report template = %#v calls=%d", sender.sentTemplate, sender.templateCalls)
	}
	if len(repository.transcripts) != 2 {
		t.Fatalf("transcript count = %d, want 2", len(repository.transcripts))
	}
	if !fakeAuditContains(repository.audits, "agent.plan_governance_recorded") || !fakeAuditContains(repository.audits, "wechat_work.reply_sent") {
		t.Fatalf("audits = %#v", repository.audits)
	}
	replyAudit := fakeAuditByType(repository.audits, "wechat_work.reply_sent")
	if replyAudit.Metadata["message_type"] != "template_card_with_text" ||
		replyAudit.Metadata["template_status"] != "succeeded" ||
		replyAudit.Metadata["text_status"] != "succeeded" ||
		replyAudit.Metadata["progress_url"] == "" {
		t.Fatalf("reply audit metadata = %#v", replyAudit.Metadata)
	}
	if len(llmClient.lastRequest.Messages) != 2 {
		t.Fatalf("llm messages = %#v", llmClient.lastRequest.Messages)
	}
	systemPrompt := llmClient.lastRequest.Messages[0].Content
	if !strings.Contains(systemPrompt, "普通微信聊天文本") || !strings.Contains(systemPrompt, "不使用 Markdown") {
		t.Fatalf("system prompt does not require plain WeChat text: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "当前用户：aroen") || !strings.Contains(systemPrompt, "只能读取和操作 user_id=1") {
		t.Fatalf("system prompt does not contain user context: %q", systemPrompt)
	}
	if llmClient.lastRequest.MaxTokens != agentReplyMaxTokens {
		t.Fatalf("MaxTokens = %d, want %d", llmClient.lastRequest.MaxTokens, agentReplyMaxTokens)
	}
	if len(repository.runs) < 2 {
		t.Fatalf("agent run count = %d, want controller and executor runs", len(repository.runs))
	}
	controllerRun := repository.runs[0]
	if controllerRun.Role != domain.AgentRunRoleController || controllerRun.Status != domain.AgentRunStatusSucceeded {
		t.Fatalf("controller run = %#v", controllerRun)
	}
	executorFound := false
	for _, run := range repository.runs[1:] {
		if run.Role == domain.AgentRunRoleExecutor && run.ParentRunID == controllerRun.ID && len(run.CapabilityScope) == 1 {
			executorFound = true
			break
		}
	}
	if !executorFound {
		t.Fatalf("executor run with parent %d was not recorded: %#v", controllerRun.ID, repository.runs)
	}
	if len(repository.contextTraces) < 3 {
		t.Fatalf("context trace count = %d, want controller and executor traces", len(repository.contextTraces))
	}
	if len(repository.observations) == 0 {
		t.Fatal("executor observations were not recorded")
	}
	if len(repository.plans) != 1 {
		t.Fatalf("plan count = %d, want 1", len(repository.plans))
	}
	plan := repository.plans[0]
	if plan.Status != domain.AgentPlanStatusCompleted {
		t.Fatalf("plan status = %q, want completed", plan.Status)
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("plan steps were not recorded: %#v", plan)
	}
	if plan.Steps[0].ExecutorRunID == 0 || plan.Steps[0].ObservationRef == "" {
		t.Fatalf("plan step was not bound to executor observation: %#v", plan.Steps[0])
	}
}

func TestAgentWebSearchNormalizesQueryAndParsesRSSResults(t *testing.T) {
	if got := normalizeWebSearchQuery("搜索最新港股消息并分析"); got != "港股" {
		t.Fatalf("normalized query = %q, want 港股", got)
	}
	bingHTML := `<html><body><ol><li class="b_algo"><h2><a href="https://example.com/hk-web">港股收评：恒指下跌，科技股走弱</a></h2><div class="b_caption"><p>港股主要指数回落，市场关注南向资金。</p></div></li><li class="b_algo"><h2><a href="https://example.com/us">美股新闻</a></h2><div class="b_caption"><p>美股科技股上涨。</p></div></li></ol></body></html>`
	bingResults := filterWebSearchResultsByQuery(parseBingResults([]byte(bingHTML), 5), "港股")
	if len(bingResults) != 1 || bingResults[0].Title != "港股收评：恒指下跌，科技股走弱" {
		t.Fatalf("bing results = %#v", bingResults)
	}
	mixedResults := []agentWebSearchResult{
		{
			Title:   "港美股交易怎么操作？新手完整交易流程讲解",
			Source:  "湾区阿瑟",
			URL:     "https://bayase.com/hk-us-stock-guide",
			Snippet: "大陆居民如何开户，讲解港股和美股交易规则、账户开通和零基础教程。",
		},
		{
			Title:       "港股收评：恒生指数下跌，科技股走弱",
			Source:      "财联社",
			URL:         "https://example.com/hk-news",
			Snippet:     "南向资金净卖出，腾讯、美团等科技股承压。",
			PublishedAt: "Fri, 26 Jun 2026 13:00:00 GMT",
		},
	}
	filtered := filterWebSearchResultsByTaskSpec(mixedResults, "港股", agent.BuildTaskSpec("搜索最新港股消息并分析"))
	if len(filtered) != 1 || filtered[0].Source != "财联社" {
		t.Fatalf("filtered mixed results = %#v", filtered)
	}
	body := `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>港股风向标：恒指反弹受阻</title><link>https://example.com/hk-news</link><description><![CDATA[<p>恒指反弹受阻，科技股成交额放大。</p>]]></description><pubDate>Fri, 26 Jun 2026 12:00:00 GMT</pubDate><source>财联社</source></item></channel></rss>`
	results := parseRSSSearchResults([]byte(body), 5)
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Title != "港股风向标：恒指反弹受阻" || results[0].Source != "财联社" || results[0].URL != "https://example.com/hk-news" {
		t.Fatalf("result = %#v", results[0])
	}
	if !strings.Contains(results[0].Snippet, "恒指反弹受阻") {
		t.Fatalf("snippet = %q", results[0].Snippet)
	}
}

func TestAgentConversationServiceCompletesWeChatSearchWhenLLMReturnsEmptyWithEvidence(t *testing.T) {
	now := time.Date(2026, 6, 26, 21, 15, 0, 0, time.UTC)
	publishedAt := now.Add(-30 * time.Minute)
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		err: domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "test", true, nil),
	}
	sender := &fakeAgentConversationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-msg-search"}}
	webFetchCalls := 0
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationUserContextProvider(&fakeAgentUserContextProvider{}),
		WithAgentConversationRecentItemsProvider(&fakeAgentRecentItemsProvider{itemsBySource: map[int64][]domain.Item{
			0: {
				{
					ID:          101,
					SourceName:  "财经源",
					Title:       "美伊谈判：中国用何利器削弱美国制裁",
					URL:         "https://example.com/feed/hk-tech",
					Summary:     "这是一条与港股走势无关的订阅源新闻。",
					PublishedAt: &publishedAt,
					FetchedAt:   now,
				},
			},
		}}),
		WithAgentConversationWebFetcher(func(_ context.Context, rawURL string) ([]byte, string, int, string, error) {
			webFetchCalls++
			if strings.Contains(rawURL, "duckduckgo.com") {
				body := `<html><body><div class="anomaly-modal">challenge</div></body></html>`
				return []byte(body), rawURL, 202, "text/html; charset=utf-8", nil
			}
			if strings.Contains(rawURL, "bing.com/search") {
				body := `<html><body><ol><li class="b_algo"><h2><a href="https://example.com/hk-market-close">港股收盘：恒指下跌，科技股走弱</a></h2><div class="b_caption"><p>恒生指数下跌，市场关注美元与资金流向。</p></div></li></ol></body></html>`
				return []byte(body), rawURL, 200, "text/html; charset=utf-8", nil
			}
			return nil, rawURL, 404, "text/plain", errors.New("unexpected test url")
		}),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-hk-search",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "搜索最新港股消息并分析",
		RequestID:         "request-hk-search",
		TraceID:           "trace-hk-search",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Turn.Status != domain.AgentTurnStatusSucceeded || result.Plan.Status != domain.AgentPlanStatusCompleted {
		t.Fatalf("turn=%q plan=%q error=%q", result.Turn.Status, result.Plan.Status, result.Plan.ErrorMessage)
	}
	if !strings.Contains(result.Reply, "结论：") || !strings.Contains(result.Reply, "依据：") || !strings.Contains(result.Reply, "分析过程：") || !strings.Contains(result.Reply, "港股收盘：恒指下跌") {
		t.Fatalf("reply = %q", result.Reply)
	}
	for _, forbidden := range []string{"模型生成阶段没有返回可用内容", "用户上下文", "web.search", "Evidence ref", "证据范围", "美伊谈判", "状态锚点", "企微动作组件", "https://example.com/hk-market-close"} {
		if strings.Contains(result.Reply, forbidden) {
			t.Fatalf("reply leaked %q: %q", forbidden, result.Reply)
		}
	}
	if webFetchCalls != 2 {
		t.Fatalf("web fetch calls = %d, want 2", webFetchCalls)
	}
	if !containsAgentString(result.Plan.AllowedScopes, "web.search") {
		t.Fatalf("allowed scopes = %#v", result.Plan.AllowedScopes)
	}
	completedSteps := 0
	for _, step := range result.Plan.Steps {
		if step.Status == domain.AgentPlanStepStatusCompleted {
			completedSteps++
		}
		if step.CapabilityKey == "web.search" && step.ObservationRef == "" {
			t.Fatalf("web search step missing observation ref: %#v", step)
		}
	}
	if completedSteps != 2 {
		t.Fatalf("completed steps = %d, steps = %#v", completedSteps, result.Plan.Steps)
	}
	controllerRun := repository.runs[0]
	if !containsAgentString(controllerRun.CapabilityScope, "web.search") || containsAgentString(controllerRun.CapabilityScope, "source.query_latest_items") {
		t.Fatalf("controller scope = %#v", controllerRun.CapabilityScope)
	}
	if !fakeContextTraceContains(repository.contextTraces, "controller_scope_aligned") || !fakeObservationContains(repository.observations, "web.search", "succeeded") {
		t.Fatalf("traces = %#v observations = %#v", repository.contextTraces, repository.observations)
	}
}

func TestAgentConversationServiceFallsBackToTextWhenFinalReportTemplateFails(t *testing.T) {
	now := time.Date(2026, 6, 24, 17, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "最终结果",
		},
	}
	sender := &fakeAgentConversationSender{
		result:      notifier.WeChatWorkSendResult{MessageID: "wx-text-1"},
		templateErr: domain.NewAppError(domain.ErrorKindUnavailable, "template_failed", "template failed", "test", true, nil),
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-final-fallback",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "执行任务",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Reply != "最终结果" || sender.templateCalls != 1 || sender.calls != 1 {
		t.Fatalf("reply=%q templateCalls=%d textCalls=%d", result.Reply, sender.templateCalls, sender.calls)
	}
	if sender.sent.Content != "最终结果" {
		t.Fatalf("fallback text = %#v", sender.sent)
	}
	replyAudit := fakeAuditByType(repository.audits, "wechat_work.reply_sent")
	if replyAudit.Metadata["message_type"] != "text_fallback" ||
		replyAudit.Metadata["template_status"] != "failed" ||
		replyAudit.Metadata["text_status"] != "succeeded" ||
		replyAudit.Metadata["progress_url"] == "" {
		t.Fatalf("reply audit metadata = %#v", replyAudit.Metadata)
	}
}

func TestAgentConversationServiceHandlesWeChatButtonCallback(t *testing.T) {
	now := time.Date(2026, 6, 24, 17, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{
		ID:                2,
		UserID:            1,
		ExternalAccountID: 10,
		Provider:          domain.AgentProviderWeChatWorkApp,
		Status:            domain.AgentSessionStatusActive,
		StartedAt:         now.Add(-time.Hour),
		LastActiveAt:      now.Add(-time.Minute),
	}
	repository.plans = []domain.AgentPlan{
		{ID: 9, UserID: 1, SessionID: 2, Status: domain.AgentPlanStatusExecuting, Summary: "执行任务", CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
	}
	account := testAgentExternalAccount(now)
	account.ActiveAgentSessionID = 2
	resolver := &fakeAgentExternalAccountResolver{account: account}
	sender := &fakeAgentConversationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-button-1"}}
	llmClient := &fakeAgentConversationLLM{response: llm.ChatResponse{Content: "不应调用"}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "button-msg-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "event",
		EventType:         "template_card_event",
		EventKey:          "view_progress",
		RequestID:         "request-button-1",
		TraceID:           "trace-button-1",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Plan.ID != 9 || !strings.Contains(result.Reply, "计划 #9") || !strings.Contains(result.Reply, "https://messagefeed.example/agent/plans/9") {
		t.Fatalf("result = %#v", result)
	}
	if llmClient.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", llmClient.calls)
	}
	if !fakeAuditContains(repository.audits, "agent.button_direct_control") {
		t.Fatalf("audits = %#v", repository.audits)
	}
	if repository.inbound.Status != domain.AgentInboundMessageStatusSucceeded {
		t.Fatalf("inbound status = %q", repository.inbound.Status)
	}
	if sender.calls == 0 || !strings.Contains(sender.sent.Content, "计划 #9") {
		t.Fatalf("sent = %#v calls=%d", sender.sent, sender.calls)
	}
}

func TestAgentConversationServiceButtonCallbackRetriesFailedPlan(t *testing.T) {
	now := time.Date(2026, 6, 24, 17, 40, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 2, UserID: 1, ExternalAccountID: 10, Provider: domain.AgentProviderWeChatWorkApp, Status: domain.AgentSessionStatusActive, StartedAt: now.Add(-time.Hour), LastActiveAt: now.Add(-time.Minute)}
	repository.plans = []domain.AgentPlan{
		{
			ID:        9,
			UserID:    1,
			SessionID: 2,
			Status:    domain.AgentPlanStatusFailed,
			Summary:   "执行失败任务",
			CreatedAt: now.Add(-time.Minute),
			UpdatedAt: now.Add(-time.Minute),
			Steps: []domain.AgentPlanStep{
				{ID: 11, PlanID: 9, Status: domain.AgentPlanStepStatusFailed, MaxRetries: 2, RetryCount: 0, FailureStrategy: "retry", ErrorMessage: "timeout", UpdatedAt: now.Add(-time.Minute)},
			},
		},
	}
	account := testAgentExternalAccount(now)
	account.ActiveAgentSessionID = 2
	service := NewAgentConversationService(
		repository,
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(&fakeAgentExternalAccountResolver{account: account}),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "button-retry-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "event",
		EventType:         "template_card_event",
		EventKey:          "retry_plan",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Plan.Status != domain.AgentPlanStatusExecuting || !strings.Contains(result.Reply, "重试按钮回调") {
		t.Fatalf("result = %#v", result)
	}
	if repository.plans[0].Steps[0].Status != domain.AgentPlanStepStatusApproved || repository.plans[0].Steps[0].RetryCount != 1 {
		t.Fatalf("step = %#v", repository.plans[0].Steps[0])
	}
	if !fakeAuditContains(repository.audits, "agent.button_direct_control") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceButtonCallbackCancelsScheduledTask(t *testing.T) {
	now := time.Date(2026, 6, 24, 17, 50, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 2, UserID: 1, ExternalAccountID: 10, Provider: domain.AgentProviderWeChatWorkApp, Status: domain.AgentSessionStatusActive, StartedAt: now.Add(-time.Hour), LastActiveAt: now.Add(-time.Minute)}
	repository.plans = []domain.AgentPlan{{ID: 9, UserID: 1, SessionID: 2, Status: domain.AgentPlanStatusExecuting, Summary: "执行定时任务", CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)}}
	repository.scheduledTasks = []domain.AgentScheduledTask{
		{ID: 30, UserID: 1, SessionID: 2, PlanID: 9, Status: domain.AgentScheduledTaskStatusQueued, Goal: "日报", ScheduledAt: now.Add(time.Hour), UpdatedAt: now.Add(-time.Minute)},
	}
	account := testAgentExternalAccount(now)
	account.ActiveAgentSessionID = 2
	service := NewAgentConversationService(
		repository,
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(&fakeAgentExternalAccountResolver{account: account}),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "button-cancel-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "event",
		EventType:         "template_card_event",
		EventKey:          "cancel_scheduled_task",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !strings.Contains(result.Reply, "任务 #30") || repository.scheduledTasks[0].Status != domain.AgentScheduledTaskStatusCanceled {
		t.Fatalf("result = %#v task = %#v", result, repository.scheduledTasks[0])
	}
	if !fakeAuditContains(repository.audits, "agent.button_direct_control") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceReceivesWebAgentTask(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.externalAccounts = []domain.ExternalAccount{
		{
			ID:             42,
			UserID:         7,
			Provider:       domain.AgentProviderWeChatWorkApp,
			CorpID:         "corp-a",
			AgentID:        "1000002",
			ExternalUserID: "zhangsan",
			DisplayName:    "张三",
			BindingStatus:  domain.ExternalAccountBindingStatusActive,
			VerifiedAt:     &now,
			LastSeenAt:     &now,
		},
	}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "Web 任务已处理",
		},
	}
	sender := &fakeAgentConversationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-msg-1"}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User: domain.User{
			ID:          7,
			Username:    "web-user",
			DisplayName: "Web User",
		},
	}, ReceiveWebAgentTaskInput{
		Message:   "请总结最近订阅内容",
		Channel:   "web",
		RequestID: "request-1",
		TraceID:   "trace-1",
	})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if repository.account.Provider != domain.AgentProviderWeb || repository.account.UserID != 7 {
		t.Fatalf("web account = %#v", repository.account)
	}
	if repository.inbound.Provider != domain.AgentProviderWeb || repository.inbound.ProviderMessageID != "web:7:request-1" {
		t.Fatalf("inbound = %#v", repository.inbound)
	}
	if result.Session.Provider != domain.AgentProviderWeb || result.Session.ID == 0 {
		t.Fatalf("session = %#v", result.Session)
	}
	if result.Turn.Status != string(domain.AgentTurnStatusSucceeded) {
		t.Fatalf("turn = %#v", result.Turn)
	}
	wantProgressURL := "https://messagefeed.example/agent/plans/" + strconv.FormatInt(result.Plan.ID, 10)
	if result.Plan.ID == 0 || result.ProgressURL != wantProgressURL {
		t.Fatalf("plan/progress = %#v / %q, want %q", result.Plan, result.ProgressURL, wantProgressURL)
	}
	if sender.templateCalls != 1 || sender.sentTemplate.ToUser != "zhangsan" || sender.sentTemplate.URL != wantProgressURL {
		t.Fatalf("web task final report template = %#v calls=%d, want url %q", sender.sentTemplate, sender.templateCalls, wantProgressURL)
	}
	if sender.calls != 1 || sender.sent.ToUser != "zhangsan" || !strings.Contains(sender.sent.Content, wantProgressURL) {
		t.Fatalf("web task final report text = %#v calls=%d, want progress url %q", sender.sent, sender.calls, wantProgressURL)
	}
	replyAudit := fakeAuditByType(repository.audits, "wechat_work.reply_sent")
	if replyAudit.Metadata["source_provider"] != domain.AgentProviderWeb ||
		replyAudit.Metadata["target_external_user_id"] != "zhangsan" ||
		replyAudit.Metadata["progress_url"] != wantProgressURL {
		t.Fatalf("web final report audit metadata = %#v", replyAudit.Metadata)
	}
}

func TestAgentConversationServiceThrottlesWebAgentTaskByUserPolicy(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.preference = defaultAgentNotificationPreference(7, now)
	repository.preference.MaxConcurrentTasks = 1
	repository.plans = append(repository.plans, domain.AgentPlan{
		ID:        99,
		UserID:    7,
		Status:    domain.AgentPlanStatusExecuting,
		Goal:      "已有任务",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	})
	service := NewAgentConversationService(
		repository,
		WithAgentConversationNow(func() time.Time { return now }),
	)

	_, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{Message: "再开一个任务", Channel: "web", RequestID: "request-throttle"})
	if err == nil {
		t.Fatal("ReceiveWebAgentTask() error = nil, want throttled")
	}
	if len(repository.audits) != 1 || repository.audits[0].EventType != "agent.task_admission_throttled" {
		t.Fatalf("audits = %#v", repository.audits)
	}
	if repository.inbound.ID != 0 {
		t.Fatalf("inbound should not be created: %#v", repository.inbound)
	}
}

func TestAgentConversationServiceRejectsWebAgentTaskWhenDailyQuotaExceeded(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 45, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.preference = defaultAgentNotificationPreference(7, now)
	repository.preference.DailyTaskQuota = 1
	repository.plans = append(repository.plans, domain.AgentPlan{
		ID:        99,
		UserID:    7,
		Status:    domain.AgentPlanStatusCompleted,
		Goal:      "今日任务",
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Minute),
	})
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	_, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{Message: "今日第二个任务", Channel: "web", RequestID: "request-quota"})
	if err == nil {
		t.Fatal("ReceiveWebAgentTask() error = nil, want quota exceeded")
	}
	if len(repository.audits) != 1 || repository.audits[0].Status != "quota_exceeded" {
		t.Fatalf("audits = %#v", repository.audits)
	}
	if repository.audits[0].Metadata["daily_task_quota"] != 1 {
		t.Fatalf("audit metadata = %#v", repository.audits[0].Metadata)
	}
}

func TestAgentConversationServiceAppendsConstraintToActiveWebPlan(t *testing.T) {
	now := time.Date(2026, 6, 25, 11, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{
		ID:                10,
		UserID:            7,
		ExternalAccountID: 1,
		Provider:          domain.AgentProviderWeb,
		Status:            domain.AgentSessionStatusActive,
		StartedAt:         now.Add(-time.Minute),
		LastActiveAt:      now.Add(-time.Minute),
	}
	repository.plans = []domain.AgentPlan{{
		ID:        20,
		UserID:    7,
		SessionID: 10,
		Status:    domain.AgentPlanStatusExecuting,
		Goal:      "汇总订阅源",
		Metadata:  domain.AgentJSON{},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{
		SessionID: 10,
		Message:   "补充：只看未读内容",
		Channel:   "web",
		RequestID: "request-append",
	})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if result.Plan.ID != 20 || len(repository.plans) != 1 {
		t.Fatalf("plan result = %#v, plans = %#v", result.Plan, repository.plans)
	}
	multiTurn, _ := repository.plans[0].Metadata["multi_turn"].(map[string]any)
	if multiTurn["latest_intent"] != string(agentFollowupIntentAppendConstraints) || !strings.Contains(result.Reply, "已将补充要求追加") {
		t.Fatalf("multi_turn = %#v, reply = %q", multiTurn, result.Reply)
	}
	if !fakeAuditContains(repository.audits, "agent.plan_input_appended") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceStopsActiveWebPlan(t *testing.T) {
	now := time.Date(2026, 6, 25, 11, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 11, UserID: 7, ExternalAccountID: 1, Provider: domain.AgentProviderWeb, Status: domain.AgentSessionStatusActive}
	repository.plans = []domain.AgentPlan{{
		ID:        21,
		UserID:    7,
		SessionID: 11,
		Status:    domain.AgentPlanStatusExecuting,
		Goal:      "分析订阅源",
		Metadata:  domain.AgentJSON{},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	}}
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{SessionID: 11, Message: "停止这个任务", Channel: "web"})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if result.Plan.Status != string(domain.AgentPlanStatusFailed) || repository.plans[0].ErrorMessage != "stopped by user" {
		t.Fatalf("plan = %#v stored = %#v", result.Plan, repository.plans[0])
	}
	if !fakeAuditContains(repository.audits, "agent.plan_stopped") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceReusesCompletedPlanForFollowup(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 12, UserID: 7, ExternalAccountID: 1, Provider: domain.AgentProviderWeb, Status: domain.AgentSessionStatusActive}
	repository.plans = []domain.AgentPlan{{
		ID:            22,
		UserID:        7,
		SessionID:     12,
		Status:        domain.AgentPlanStatusCompleted,
		Goal:          "汇总 AI 新闻",
		Summary:       "已汇总三条 AI 新闻",
		ImpactSummary: "主要影响为模型发布节奏加快",
		Metadata:      domain.AgentJSON{},
		AllowedScopes: []string{"web.search", "content.summarize_text"},
		Steps: []domain.AgentPlanStep{{
			ID:             2201,
			PlanID:         22,
			CapabilityKey:  "web.search",
			ObservationRef: "agent_observation:31",
			ArtifactRefs:   []string{"agent_artifact:41"},
		}},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}}
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{SessionID: 12, Message: "刚才结果的依据是什么", Channel: "web"})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if result.Plan.ID != 22 || !strings.Contains(result.Reply, "已关联到计划 #22") {
		t.Fatalf("result = %#v reply = %q", result.Plan, result.Reply)
	}
	if !strings.Contains(result.Reply, "结果新鲜度：fresh") || !strings.Contains(result.Reply, "agent_artifact:41") {
		t.Fatalf("reply missing freshness or evidence refs: %q", result.Reply)
	}
	multiTurn, _ := repository.plans[0].Metadata["multi_turn"].(map[string]any)
	reuse, _ := multiTurn["result_reuse"].(map[string]any)
	if reuse["source_plan_id"] != int64(22) && reuse["source_plan_id"] != 22 {
		t.Fatalf("result_reuse = %#v", reuse)
	}
	if !fakeAuditContains(repository.audits, "agent.plan_result_reused") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceDoesNotReuseStaleCompletedPlanAsCurrentFact(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 13, UserID: 7, ExternalAccountID: 1, Provider: domain.AgentProviderWeb, Status: domain.AgentSessionStatusActive}
	oldCompletedAt := now.Add(-8 * time.Hour)
	repository.plans = []domain.AgentPlan{{
		ID:            23,
		UserID:        7,
		SessionID:     13,
		Status:        domain.AgentPlanStatusCompleted,
		Goal:          "联网检索模型新闻",
		Summary:       "旧新闻摘要",
		AllowedScopes: []string{"web.search"},
		CompletedAt:   &oldCompletedAt,
		Metadata:      domain.AgentJSON{},
		CreatedAt:     oldCompletedAt,
		UpdatedAt:     oldCompletedAt,
	}}
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{SessionID: 13, Message: "刚才结果是什么", Channel: "web"})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if result.Plan.ID != 23 || !strings.Contains(result.Reply, "该结果已过期") {
		t.Fatalf("result = %#v reply = %q", result.Plan, result.Reply)
	}
	if strings.Contains(result.Reply, "旧新闻摘要") {
		t.Fatalf("stale reply reused old fact: %q", result.Reply)
	}
	if !fakeAuditContains(repository.audits, "agent.plan_result_stale") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceRecordsParentPlanForDerivedWebTask(t *testing.T) {
	now := time.Date(2026, 6, 25, 13, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 14, UserID: 7, ExternalAccountID: 1, Provider: domain.AgentProviderWeb, Status: domain.AgentSessionStatusActive}
	repository.plans = []domain.AgentPlan{{
		ID:        24,
		UserID:    7,
		SessionID: 14,
		Status:    domain.AgentPlanStatusCompleted,
		Goal:      "汇总数据库新闻",
		Metadata:  domain.AgentJSON{},
		Steps: []domain.AgentPlanStep{{
			ID:             2401,
			PlanID:         24,
			CapabilityKey:  "feed.query_recent_items",
			ObservationRef: "agent_observation:51",
			ArtifactRefs:   []string{"agent_artifact:61"},
		}},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}}
	llmClient := &fakeAgentConversationLLM{response: llm.ChatResponse{Provider: "openai_compatible", Model: "custom-model", Content: "派生任务已处理"}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationNow(func() time.Time { return now }),
	)

	result, err := service.ReceiveWebAgentTask(context.Background(), CurrentAuth{
		Authenticated: true,
		User:          domain.User{ID: 7, Username: "web-user"},
	}, ReceiveWebAgentTaskInput{SessionID: 14, Message: "基于刚才结果创建一个刷新汇总任务", Channel: "web"})
	if err != nil {
		t.Fatalf("ReceiveWebAgentTask() error = %v", err)
	}
	if result.Plan.ID == 0 || result.Plan.ID == 24 {
		t.Fatalf("derived plan = %#v", result.Plan)
	}
	created := repository.plans[len(repository.plans)-1]
	parent, _ := created.Metadata["parent_plan"].(domain.AgentJSON)
	if parent == nil {
		if typed, ok := created.Metadata["parent_plan"].(map[string]any); ok {
			parent = domain.AgentJSON(typed)
		}
	}
	if parent["id"] != int64(24) && parent["id"] != 24 {
		t.Fatalf("parent_plan = %#v metadata = %#v", parent, created.Metadata)
	}
	if !fakeAuditContains(repository.audits, "agent.plan_derived") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceSplitsLongWeChatWorkReply(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(time.Now().UTC())}
	reply := strings.Repeat("你", notifier.WeChatWorkTextByteLimit)
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{Provider: "hyb", Model: "custom-model", Content: reply},
	}
	sender := &fakeAgentConversationSender{}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "详细介绍",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Reply != reply {
		t.Fatalf("Reply was changed")
	}
	if sender.calls < 2 {
		t.Fatalf("sender calls = %d, want at least 2", sender.calls)
	}
	var sent strings.Builder
	for _, message := range sender.sentMessages {
		if len(message.Content) > notifier.WeChatWorkTextByteLimit {
			t.Fatalf("chunk byte length = %d, limit %d", len(message.Content), notifier.WeChatWorkTextByteLimit)
		}
		sent.WriteString(message.Content)
	}
	if sent.String() != reply {
		t.Fatalf("sent content does not match reply")
	}
}

func TestAgentConversationServiceDoesNotSendDuplicateInboundMessage(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	repository.forceDuplicate = true
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(time.Now().UTC())}
	llmClient := &fakeAgentConversationLLM{}
	sender := &fakeAgentConversationSender{}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-1",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "最近有什么更新",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !result.Duplicate {
		t.Fatal("Duplicate = false, want true")
	}
	if llmClient.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", llmClient.calls)
	}
	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
}

func TestAgentConversationServiceUsesFallbackReplyWithoutLLM(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(time.Now().UTC())}
	sender := &fakeAgentConversationSender{}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-2",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "你好",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Reply != "已收到：你好" {
		t.Fatalf("Reply = %q", result.Reply)
	}
	if sender.sent.Content != "已收到：你好" {
		t.Fatalf("sent Content = %q", sender.sent.Content)
	}
}

func TestAgentConversationServiceRequiresWeChatWorkBinding(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{err: domain.ErrNotFound}
	sender := &fakeAgentConversationSender{}
	llmClient := &fakeAgentConversationLLM{}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-unbound",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "unbound",
		MsgType:           "text",
		TextContent:       "你好",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !result.BindingRequired {
		t.Fatal("BindingRequired = false, want true")
	}
	if !strings.Contains(result.Reply, "完成企业微信绑定") {
		t.Fatalf("Reply = %q", result.Reply)
	}
	if llmClient.calls != 0 {
		t.Fatalf("llm calls = %d, want 0", llmClient.calls)
	}
	if len(repository.turns) != 0 {
		t.Fatalf("turn count = %d, want 0", len(repository.turns))
	}
}

func TestAgentConversationServiceQueuesTurnAndProcessesAsync(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(time.Now().UTC())}
	llmStarted := make(chan struct{})
	llmRelease := make(chan struct{})
	sentEvents := make(chan notifier.WeChatWorkTextMessage, 4)
	llmClient := &fakeAgentConversationLLM{
		started: llmStarted,
		release: llmRelease,
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "异步回复",
		},
	}
	sender := &fakeAgentConversationSender{sentEvents: sentEvents}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationProcessTimeout(time.Second),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-async",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "最近有什么更新",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !result.ProcessingAsync {
		t.Fatal("ProcessingAsync = false, want true")
	}
	if result.Turn.Status != domain.AgentTurnStatusRunning {
		t.Fatalf("initial turn status = %q, want running", result.Turn.Status)
	}
	if result.Reply != agentTaskAcceptedFeedbackText() {
		t.Fatalf("initial reply = %q", result.Reply)
	}

	var acceptedMessage notifier.WeChatWorkTextMessage
	select {
	case acceptedMessage = <-sentEvents:
	case <-time.After(time.Second):
		t.Fatal("task acceptance feedback was not sent")
	}
	if acceptedMessage.Content != agentTaskAcceptedFeedbackText() || acceptedMessage.ToUser != "zhangsan" {
		t.Fatalf("acceptance feedback = %#v", acceptedMessage)
	}
	for _, forbidden := range []string{"状态锚点", "企微动作组件", "调度方式", "权限：", "预算：", "详情："} {
		if strings.Contains(acceptedMessage.Content, forbidden) {
			t.Fatalf("acceptance feedback leaked %q: %q", forbidden, acceptedMessage.Content)
		}
	}

	var startedMessage notifier.WeChatWorkTextMessage
	select {
	case startedMessage = <-sentEvents:
	case <-time.After(time.Second):
		t.Fatal("agent start feedback was not sent")
	}
	if !strings.Contains(startedMessage.Content, "已开始处理") ||
		!strings.Contains(startedMessage.Content, "详情：") ||
		!strings.Contains(startedMessage.Content, "https://messagefeed.example/agent/plans/") {
		t.Fatalf("start feedback = %q", startedMessage.Content)
	}
	for _, forbidden := range []string{"状态锚点", "企微动作组件", "调度方式", "权限：", "预算："} {
		if strings.Contains(startedMessage.Content, forbidden) {
			t.Fatalf("start feedback leaked %q: %q", forbidden, startedMessage.Content)
		}
	}

	select {
	case <-llmStarted:
	case <-time.After(time.Second):
		t.Fatal("llm was not started asynchronously")
	}
	close(llmRelease)
	var completionMessage notifier.WeChatWorkTextMessage
	select {
	case completionMessage = <-sentEvents:
	case <-time.After(time.Second):
		t.Fatal("async reply was not sent")
	}
	if !strings.Contains(completionMessage.Content, "异步回复") {
		t.Fatalf("completion feedback = %q", completionMessage.Content)
	}
	for _, forbidden := range []string{"状态锚点", "状态：", "下一步：", "企微动作组件", "权限：", "预算：", "质量：", "成本：", "运行观测"} {
		if strings.Contains(completionMessage.Content, forbidden) {
			t.Fatalf("completion feedback leaked %q: %q", forbidden, completionMessage.Content)
		}
	}
	if !fakeAuditContains(repository.audits, "wechat_work.task_accepted_feedback") ||
		!fakeAuditContains(repository.audits, "agent.plan_started_feedback") {
		t.Fatalf("audits = %#v", repository.audits)
	}
	acceptedAudit := fakeAuditByType(repository.audits, "wechat_work.task_accepted_feedback")
	if acceptedAudit.Status != "succeeded" ||
		acceptedAudit.Metadata["target_channel"] != domain.AgentProviderWeChatWorkApp ||
		acceptedAudit.Metadata["send_count"] != 1 {
		t.Fatalf("acceptance audit = %#v", acceptedAudit)
	}
	deadline := time.After(time.Second)
	for repository.inbound.Status != domain.AgentInboundMessageStatusSucceeded {
		select {
		case <-deadline:
			t.Fatalf("inbound status = %q", repository.inbound.Status)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if repository.turns[0].Status != domain.AgentTurnStatusSucceeded {
		t.Fatalf("final turn status = %q", repository.turns[0].Status)
	}
}

func TestAgentConversationServiceSendsWeChatProgressNotificationWithAudit(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	sender := &fakeAgentConversationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-progress-1"}}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationSender(sender),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)
	plan := domain.AgentPlan{
		ID:      9,
		UserID:  1,
		Status:  domain.AgentPlanStatusFailed,
		Summary: "汇总订阅",
		Metadata: domain.AgentJSON{
			"permission_governance": domain.AgentJSON{"has_external_access": true, "requires_confirmation": false},
			"budget_governance":     domain.AgentJSON{"status": "within_budget", "tool_calls": 1, "tool_call_budget": 8, "external_calls": 1, "external_call_budget": 4},
		},
		Steps: []domain.AgentPlanStep{
			{ID: 10, Status: domain.AgentPlanStepStatusFailed, Title: "联网查询", CapabilityKey: "web.search", ErrorMessage: "network timeout"},
		},
		UpdatedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
	}

	service.sendPlanProgressNotification(
		context.Background(),
		domain.ExternalAccount{UserID: 1},
		domain.AgentSession{ID: 2},
		domain.AgentTurn{ID: 3},
		ReceiveWeChatWorkAppMessageInput{
			Provider:          domain.AgentProviderWeChatWorkApp,
			ProviderMessageID: "msg-progress",
			ExternalUserID:    "zhangsan",
		},
		plan,
		"step_failed",
		"计划步骤失败",
	)

	if sender.templateCalls != 1 || sender.sentTemplate.ToUser != "zhangsan" {
		t.Fatalf("template sent = %#v calls=%d", sender.sentTemplate, sender.templateCalls)
	}
	if sender.calls != 0 {
		t.Fatalf("text fallback calls = %d, want 0", sender.calls)
	}
	if sender.sentTemplate.URL != "https://messagefeed.example/agent/plans/9" ||
		!strings.Contains(sender.sentTemplate.Description, "计划 #9") ||
		!strings.Contains(sender.sentTemplate.FallbackText, "详情：https://messagefeed.example/agent/plans/9") ||
		!strings.Contains(sender.sentTemplate.FallbackText, "https://messagefeed.example/agent/plans/9") {
		t.Fatalf("template = %#v", sender.sentTemplate)
	}
	if strings.Contains(sender.sentTemplate.FallbackText, "企微动作组件") || strings.Contains(sender.sentTemplate.FallbackText, "状态锚点") {
		t.Fatalf("template fallback leaked execution details = %#v", sender.sentTemplate)
	}
	if len(repository.audits) != 1 || repository.audits[0].EventType != "agent.plan_progress_notification" || repository.audits[0].Status != "succeeded" {
		t.Fatalf("audits = %#v", repository.audits)
	}
	if repository.audits[0].Metadata["target_channel"] != domain.AgentProviderWeChatWorkApp ||
		repository.audits[0].Metadata["message_type"] != "template_card" ||
		repository.audits[0].Metadata["template_status"] != "succeeded" ||
		repository.audits[0].Metadata["progress_url"] != "https://messagefeed.example/agent/plans/9" {
		t.Fatalf("audit metadata = %#v", repository.audits[0].Metadata)
	}
}

func TestAgentConversationServiceFallsBackToTextWhenProgressTemplateFails(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	sender := &fakeAgentConversationSender{
		result:      notifier.WeChatWorkSendResult{MessageID: "wx-progress-fallback"},
		templateErr: domain.NewAppError(domain.ErrorKindUnavailable, "template_failed", "template failed", "test", true, nil),
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationSender(sender),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
	)
	plan := domain.AgentPlan{
		ID:        10,
		UserID:    1,
		Status:    domain.AgentPlanStatusExecuting,
		Summary:   "汇总订阅",
		UpdatedAt: time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
	}

	service.sendPlanProgressNotification(
		context.Background(),
		domain.ExternalAccount{UserID: 1},
		domain.AgentSession{ID: 2},
		domain.AgentTurn{ID: 3},
		ReceiveWeChatWorkAppMessageInput{
			Provider:          domain.AgentProviderWeChatWorkApp,
			ProviderMessageID: "msg-progress-fallback",
			ExternalUserID:    "zhangsan",
		},
		plan,
		"started",
		"工作已开始",
	)

	if sender.templateCalls != 1 || sender.calls != 1 {
		t.Fatalf("calls template=%d text=%d", sender.templateCalls, sender.calls)
	}
	if sender.sent.ToUser != "zhangsan" ||
		!strings.Contains(sender.sent.Content, "https://messagefeed.example/agent/plans/10") {
		t.Fatalf("fallback text = %#v", sender.sent)
	}
	if len(repository.audits) != 1 || repository.audits[0].Status != "succeeded" {
		t.Fatalf("audits = %#v", repository.audits)
	}
	if repository.audits[0].Metadata["message_type"] != "text_fallback" ||
		repository.audits[0].Metadata["template_status"] != "failed" ||
		repository.audits[0].Metadata["fallback_status"] != "succeeded" ||
		repository.audits[0].Metadata["progress_url"] != "https://messagefeed.example/agent/plans/10" {
		t.Fatalf("audit metadata = %#v", repository.audits[0].Metadata)
	}
}

func TestAgentConversationServiceBindPlanStepsWritesQualityAndDeploymentMetadata(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	plan := domain.AgentPlan{
		ID:        10,
		UserID:    1,
		SessionID: 2,
		TurnID:    3,
		Status:    domain.AgentPlanStatusExecuting,
		Goal:      "汇总订阅",
		Summary:   "执行订阅汇总",
		RiskLevel: "low",
		Metadata:  domain.AgentJSON{},
		Steps: []domain.AgentPlanStep{
			{ID: 11, PlanID: 10, Status: domain.AgentPlanStepStatusExecuting, CapabilityKey: "web.search", Title: "联网查询"},
		},
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	}
	repository.plans = append(repository.plans, plan)
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	updated, err := service.bindPlanStepsToObservations(context.Background(), 1, plan, []agent.CapabilityObservation{
		{
			Capability:     "web.search",
			Status:         "succeeded",
			Summary:        "完成查询",
			RunID:          20,
			ObservationRef: "observation:20:web.search",
			ArtifactRefs:   []string{"artifact:web:1"},
		},
	})
	if err != nil {
		t.Fatalf("bindPlanStepsToObservations() error = %v", err)
	}
	if updated.Status != domain.AgentPlanStatusCompleted {
		t.Fatalf("updated status = %q", updated.Status)
	}
	if metadataMap(updated.Metadata, "result_quality") == nil || metadataMap(updated.Metadata, "cost_summary") == nil || metadataMap(updated.Metadata, "deployment_acceptance") == nil {
		t.Fatalf("metadata = %#v", updated.Metadata)
	}
	if metadataNumber(metadataMap(updated.Metadata, "cost_summary"), "tool_calls") != 1 || metadataNumber(metadataMap(updated.Metadata, "cost_summary"), "external_calls") != 1 {
		t.Fatalf("cost summary = %#v", updated.Metadata["cost_summary"])
	}
	if metadataString(metadataMap(updated.Metadata, "deployment_acceptance"), "status") != "ready" {
		t.Fatalf("deployment acceptance = %#v", updated.Metadata["deployment_acceptance"])
	}
}

func TestAgentConversationServiceAppliesCapabilityPolicyToPlan(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.preference = defaultAgentNotificationPreference(1, now)
	repository.preference.CapabilityPolicy = domain.AgentJSON{"web.search": "reject"}
	plan := domain.AgentPlan{
		ID:        10,
		UserID:    1,
		SessionID: 2,
		TurnID:    3,
		Status:    domain.AgentPlanStatusApproved,
		Summary:   "联网查询",
		Metadata:  domain.AgentJSON{},
		Steps: []domain.AgentPlanStep{
			{ID: 11, PlanID: 10, Status: domain.AgentPlanStepStatusApproved, CapabilityKey: "web.search", Title: "搜索网页"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	repository.plans = append(repository.plans, plan)
	service := NewAgentConversationService(repository, WithAgentConversationNow(func() time.Time { return now }))

	updated, err := service.applyCapabilityPolicyToPlan(context.Background(), 1, 2, 3, plan, ReceiveWeChatWorkAppMessageInput{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("applyCapabilityPolicyToPlan() error = %v", err)
	}
	if updated.Status != domain.AgentPlanStatusRejected {
		t.Fatalf("status = %q", updated.Status)
	}
	policy := metadataMap(updated.Metadata, "capability_policy")
	if metadataString(policy, "status") != "reject" {
		t.Fatalf("policy = %#v", policy)
	}
	if len(repository.audits) != 1 || repository.audits[0].EventType != "agent.capability_policy_applied" || repository.audits[0].Status != "reject" {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceSkipsWebProgressNotification(t *testing.T) {
	repository := newFakeAgentConversationRepository()
	sender := &fakeAgentConversationSender{}
	service := NewAgentConversationService(repository, WithAgentConversationSender(sender))

	service.sendPlanProgressNotification(
		context.Background(),
		domain.ExternalAccount{UserID: 1},
		domain.AgentSession{ID: 2},
		domain.AgentTurn{ID: 3},
		ReceiveWeChatWorkAppMessageInput{Provider: domain.AgentProviderWeb, ExternalUserID: "user:1"},
		domain.AgentPlan{ID: 9, UserID: 1, Status: domain.AgentPlanStatusExecuting},
		"started",
		"工作已开始",
	)

	if sender.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", sender.calls)
	}
	if len(repository.audits) != 0 {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func TestAgentConversationServiceInjectsReadOnlyCapabilityContextWithoutPublishingAIFeedReport(t *testing.T) {
	now := time.Date(2026, 6, 24, 18, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	recentItems := &fakeAgentRecentItemsProvider{
		itemsBySource: map[int64][]domain.Item{
			0: {
				{ID: 1, SourceName: "Go 官方博客", Title: "Go 1.26 发布", Summary: "Go 1.26 带来工具链更新。"},
			},
			42: {
				{ID: 2, SourceName: "Go 官方博客", Title: "Go 工具链说明"},
			},
		},
	}
	sourceProvider := &fakeAgentSourceProvider{
		sources: []domain.Source{{ID: 42, UserID: 1, Name: "Go 官方博客", Status: domain.SourceStatusActive}},
	}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "基于最近条目，Go 官方博客有工具链更新。",
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationRecentItemsProvider(recentItems),
		WithAgentConversationSourceProvider(sourceProvider),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	_, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-capability",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "Go 官方博客最近有什么",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	systemPrompt := llmClient.lastRequest.Messages[0].Content
	if !strings.Contains(systemPrompt, "最近条目") || !strings.Contains(systemPrompt, "Go 工具链说明") || !strings.Contains(systemPrompt, "Evidence ref：item:2") {
		t.Fatalf("system prompt missing recent items context: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "匹配来源最新条目") || !strings.Contains(systemPrompt, "Go 官方博客") || !strings.Contains(systemPrompt, "Go 工具链说明") {
		t.Fatalf("system prompt missing source latest context: %q", systemPrompt)
	}
}

func TestAgentConversationServiceUsesSelectedActiveSession(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{
		ID:                300,
		UserID:            1,
		ExternalAccountID: 10,
		Provider:          domain.AgentProviderWeChatWorkApp,
		ChannelSessionKey: "manual-session",
		Status:            domain.AgentSessionStatusActive,
	}
	account := testAgentExternalAccount(now)
	account.ActiveAgentSessionID = 300
	resolver := &fakeAgentExternalAccountResolver{account: account}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{Provider: "openai_compatible", Model: "custom-model", Content: "进入选中的 session。"},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-selected-session",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "你好",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Session.ID != 300 {
		t.Fatalf("session ID = %d, want selected active session 300", result.Session.ID)
	}
	if len(repository.transcripts) == 0 || repository.transcripts[0].SessionID != 300 {
		t.Fatalf("transcripts = %#v", repository.transcripts)
	}
}

func TestAgentConversationServiceInjectsRecentConversationWindowWithoutCurrentTurn(t *testing.T) {
	now := time.Date(2026, 6, 24, 19, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 100, UserID: 1, Provider: domain.AgentProviderWeChatWorkApp, ChannelSessionKey: "corp-a:1000002:zhangsan"}
	repository.transcripts = []domain.AgentTranscriptEntry{
		{ID: 11, SessionID: 100, TurnID: 1, UserID: 1, Role: domain.AgentTranscriptRoleUser, Content: "我想关注 Go 官方博客", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: 12, SessionID: 100, TurnID: 1, UserID: 1, Role: domain.AgentTranscriptRoleAssistant, Content: "已理解。", CreatedAt: now.Add(-time.Minute)},
	}
	repository.nextID = 101
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		response: llm.ChatResponse{Provider: "openai_compatible", Model: "custom-model", Content: "Go 官方博客最近有工具链更新。"},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	_, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-recent-window",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "刚才 Go 官方博客 最近有什么",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	messages := llmClient.lastRequest.Messages
	if len(messages) != 4 {
		t.Fatalf("llm message count = %d, want 4: %#v", len(messages), messages)
	}
	if messages[1].Role != "user" || messages[1].Content != "我想关注 Go 官方博客" {
		t.Fatalf("recent user message = %#v", messages[1])
	}
	if messages[2].Role != "assistant" || messages[2].Content != "已理解。" {
		t.Fatalf("recent assistant message = %#v", messages[2])
	}
	if messages[3].Role != "user" || messages[3].Content != "刚才 Go 官方博客 最近有什么" {
		t.Fatalf("current message = %#v", messages[3])
	}
	if len(repository.recalls) != 0 {
		t.Fatalf("recall count = %d, want 0", len(repository.recalls))
	}
}

func TestAgentConversationServiceExecutesConversationHistoryToolCall(t *testing.T) {
	now := time.Date(2026, 6, 24, 20, 0, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 200, UserID: 1, Provider: domain.AgentProviderWeChatWorkApp, ChannelSessionKey: "corp-a:1000002:zhangsan"}
	repository.transcripts = []domain.AgentTranscriptEntry{
		{ID: 21, SessionID: 200, TurnID: 1, UserID: 1, Role: domain.AgentTranscriptRoleUser, Content: "我的偏好是关注 Go 和 AI 基础设施。", CreatedAt: now.Add(-24 * time.Hour)},
	}
	repository.nextID = 201
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"keyword":"偏好","limit":3}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "你之前说过偏好关注 Go 和 AI 基础设施。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-tool-history",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "查一下我的偏好",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Reply != "你之前说过偏好关注 Go 和 AI 基础设施。" {
		t.Fatalf("Reply = %q", result.Reply)
	}
	if llmClient.calls != 2 {
		t.Fatalf("llm calls = %d, want 2", llmClient.calls)
	}
	if len(llmClient.lastRequest.Messages) < 3 {
		t.Fatalf("final llm messages = %#v", llmClient.lastRequest.Messages)
	}
	toolMessage := llmClient.lastRequest.Messages[len(llmClient.lastRequest.Messages)-1]
	if toolMessage.Role != "tool" || !strings.Contains(toolMessage.Content, "我的偏好是关注 Go 和 AI 基础设施") {
		t.Fatalf("tool message = %#v", toolMessage)
	}
	if len(repository.recalls) != 1 {
		t.Fatalf("recall count = %d, want 1", len(repository.recalls))
	}
	if repository.recalls[0].Reason != "model_tool_call" {
		t.Fatalf("recall reason = %q", repository.recalls[0].Reason)
	}
}

func TestAgentConversationServiceHistoryToolReturnsEarliestBoundary(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 210, UserID: 1, Provider: domain.AgentProviderWeChatWorkApp, ChannelSessionKey: "corp-a:1000002:zhangsan"}
	repository.transcripts = []domain.AgentTranscriptEntry{
		{ID: 31, SessionID: 210, TurnID: 1, UserID: 1, Role: domain.AgentTranscriptRoleUser, Content: "介绍你的能力", CreatedAt: now.Add(-48 * time.Hour)},
		{ID: 32, SessionID: 210, TurnID: 2, UserID: 1, Role: domain.AgentTranscriptRoleAssistant, Content: "我可以帮助你查询订阅和历史对话。", CreatedAt: now.Add(-47 * time.Hour)},
	}
	repository.nextID = 211
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"mode":"earliest","limit":1}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "当前 session 的第一条消息是：介绍你的能力。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-earliest-history",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "我发的第一条消息是什么",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Reply != "当前 session 的第一条消息是：介绍你的能力。" {
		t.Fatalf("Reply = %q", result.Reply)
	}
	toolMessage := llmClient.lastRequest.Messages[len(llmClient.lastRequest.Messages)-1]
	for _, required := range []string{"查询模式：earliest", "是否存在更早记录：否", "介绍你的能力"} {
		if !strings.Contains(toolMessage.Content, required) {
			t.Fatalf("tool message missing %q: %q", required, toolMessage.Content)
		}
	}
	if len(repository.recalls) == 0 {
		t.Fatal("recall event is missing")
	}
	if got := repository.recalls[len(repository.recalls)-1].QueryParams["mode"]; got != conversationHistoryModeEarliest {
		t.Fatalf("recall mode = %#v", got)
	}
}

func TestAgentConversationServiceHistoryToolUsesTimeRange(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.session = domain.AgentSession{ID: 220, UserID: 1, Provider: domain.AgentProviderWeChatWorkApp, ChannelSessionKey: "corp-a:1000002:zhangsan"}
	repository.transcripts = []domain.AgentTranscriptEntry{
		{ID: 41, SessionID: 220, TurnID: 1, UserID: 1, Role: domain.AgentTranscriptRoleUser, Content: "昨天的记录", CreatedAt: time.Date(2026, 6, 23, 2, 30, 0, 0, time.UTC)},
		{ID: 42, SessionID: 220, TurnID: 2, UserID: 1, Role: domain.AgentTranscriptRoleUser, Content: "今天的记录", CreatedAt: time.Date(2026, 6, 24, 2, 30, 0, 0, time.UTC)},
	}
	repository.nextID = 221
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"mode":"time_range","time_hint":"昨天","limit":5}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "昨天你说过：昨天的记录。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	if _, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-time-range-history",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "查一下昨天的聊天",
	}); err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	toolMessage := llmClient.lastRequest.Messages[len(llmClient.lastRequest.Messages)-1]
	if !strings.Contains(toolMessage.Content, "查询模式：time_range") || !strings.Contains(toolMessage.Content, "昨天的记录") || strings.Contains(toolMessage.Content, "今天的记录") {
		t.Fatalf("tool message = %q", toolMessage.Content)
	}
}

func TestAgentConversationServiceScheduleMessageRequiresConfirmation(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	notificationStore := &fakeAgentNotificationJobStore{}
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "agent__schedule_message", Arguments: `{"task_type":"reminder","content":"检查部署状态","time_hint":"明天上午9点","confirmed":false}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "需要你确认后我才能创建该提醒。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNotificationJobStore(notificationStore),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	if _, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-schedule-confirm",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "明天上午9点提醒我检查部署状态",
	}); err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if len(notificationStore.jobs) != 0 {
		t.Fatalf("notification jobs = %#v, want none", notificationStore.jobs)
	}
}

func TestAgentConversationServicePlanStartedReplyIncludesProgressSummary(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	service := NewAgentConversationService(newFakeAgentConversationRepository(), WithAgentConversationNow(func() time.Time { return now }))
	reply := service.agentPlanStartedReply(domain.AgentPlan{
		ID:            7,
		Status:        domain.AgentPlanStatusExecuting,
		Summary:       "执行网页检索",
		AllowedScopes: []string{"web.search"},
		UpdatedAt:     now,
	})
	if !strings.Contains(reply, "已开始处理") || !strings.Contains(reply, "进度：") || !strings.Contains(reply, "详情：") {
		t.Fatalf("reply = %q", reply)
	}
	for _, forbidden := range []string{"状态锚点", "企微动作组件", "授权范围", "预算：", "权限：", "实施步骤"} {
		if strings.Contains(reply, forbidden) {
			t.Fatalf("reply leaked %q: %q", forbidden, reply)
		}
	}
}

func TestAgentConversationServiceApprovalRequiredReplyIncludesActionAnchors(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	service := NewAgentConversationService(
		newFakeAgentConversationRepository(),
		WithAgentConversationPublicBaseURL("https://messagefeed.example"),
		WithAgentConversationNow(func() time.Time { return now }),
	)
	reply := service.approvalRequiredReply(domain.AgentPlan{
		ID:            11,
		Status:        domain.AgentPlanStatusAwaitingApproval,
		Summary:       "发送外部通知",
		ImpactSummary: "将向绑定用户发送通知",
		AllowedScopes: []string{"agent.schedule_message"},
		UpdatedAt:     now,
	}, "approval-token")
	if !strings.Contains(reply, "状态锚点：approval_required/awaiting_approval") ||
		!strings.Contains(reply, "审批地址：https://messagefeed.example/agent/approvals/approval-token") ||
		!strings.Contains(reply, "进度地址：https://messagefeed.example/agent/plans/11") ||
		!strings.Contains(reply, "下一步：") ||
		!strings.Contains(reply, "企微动作组件：view_progress=https://messagefeed.example/agent/plans/11") ||
		!strings.Contains(reply, "approval=https://messagefeed.example/agent/approvals/approval-token") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestAgentConversationServiceScheduleMessageCreatesNotificationJobWhenConfirmed(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	notificationStore := &fakeAgentNotificationJobStore{}
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "agent__schedule_message", Arguments: `{"task_type":"reminder","content":"检查部署状态","scheduled_at":"2026-06-25T09:00:00+08:00","time_hint":"明天上午9点","confirmed":true}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "已创建提醒。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNotificationJobStore(notificationStore),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	if _, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-schedule-create",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "确认创建上一条任务",
	}); err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if len(notificationStore.jobs) != 1 {
		t.Fatalf("notification job count = %d, want 1", len(notificationStore.jobs))
	}
	job := notificationStore.jobs[0]
	if job.Status != domain.NotificationJobStatusQueued || job.Channel != domain.NotificationChannelWeChatWork {
		t.Fatalf("job = %#v", job)
	}
	if job.Payload["content"] != "检查部署状态" || job.Payload["to_user"] != "zhangsan" {
		t.Fatalf("job payload = %#v", job.Payload)
	}
	wantScheduledAt := time.Date(2026, 6, 25, 1, 0, 0, 0, time.UTC)
	if !job.ScheduledAt.Equal(wantScheduledAt) {
		t.Fatalf("scheduled_at = %s, want %s", job.ScheduledAt, wantScheduledAt)
	}
}

func TestAgentConversationServiceScheduleMessageCreatesFromNormalizedConfirmation(t *testing.T) {
	now := time.Date(2026, 6, 24, 13, 54, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	notificationStore := &fakeAgentNotificationJobStore{}
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "agent__schedule_message", Arguments: `{"task_type":"reminder","content":"到点了","scheduled_at":"2026-06-24T21:55:00+08:00","time_hint":"今天晚上9点55","confirmed":true}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "已创建提醒。"},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(&fakeAgentConversationSender{}),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNotificationJobStore(notificationStore),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	if _, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-schedule-normalized-confirm",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "是的",
	}); err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if len(notificationStore.jobs) != 1 {
		t.Fatalf("notification job count = %d, want 1", len(notificationStore.jobs))
	}
	job := notificationStore.jobs[0]
	wantScheduledAt := time.Date(2026, 6, 24, 13, 55, 0, 0, time.UTC)
	if !job.ScheduledAt.Equal(wantScheduledAt) {
		t.Fatalf("scheduled_at = %s, want %s", job.ScheduledAt, wantScheduledAt)
	}
	if job.Payload["content"] != "到点了" || job.Payload["scheduled_at"] != "2026-06-24T13:55:00Z" {
		t.Fatalf("job payload = %#v", job.Payload)
	}
}

func TestAgentConversationServiceSendsFallbackWhenScheduleCreationFails(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 7, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	notificationStore := &fakeAgentNotificationJobStore{err: errors.New("repository write failed")}
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	sender := &fakeAgentConversationSender{}
	llmClient := &fakeAgentConversationLLM{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "agent__schedule_message", Arguments: `{"task_type":"reminder","content":"到点了","scheduled_at":"2026-06-24T22:10:00+08:00","time_hint":"今天晚上10点10分","confirmed":true}`},
				},
			},
		},
	}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNotificationJobStore(notificationStore),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-schedule-write-failed",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "确认创建上一条任务",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !strings.Contains(result.Reply, "没有成功") {
		t.Fatalf("fallback reply = %q", result.Reply)
	}
	if sender.calls == 0 || !strings.Contains(sender.sent.Content, "没有成功") {
		t.Fatalf("sent fallback = %#v", sender.sent)
	}
	if len(repository.transcripts) < 2 || !strings.Contains(repository.transcripts[len(repository.transcripts)-1].Content, "没有成功") {
		t.Fatalf("transcripts = %#v", repository.transcripts)
	}
}

func TestAgentConversationServiceFailsTurnWhenPlanCreationFails(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 32, 0, 0, time.UTC)
	repository := newFakeAgentConversationRepository()
	repository.createPlanErr = errors.New("repository: invalid input syntax for type json")
	resolver := &fakeAgentExternalAccountResolver{account: testAgentExternalAccount(now)}
	sender := &fakeAgentConversationSender{}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(&fakeAgentConversationLLM{}),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationNow(func() time.Time { return now }),
		WithAgentConversationInlineProcessing(true),
	)

	result, err := service.ReceiveWeChatWorkAppMessage(context.Background(), ReceiveWeChatWorkAppMessageInput{
		ProviderMessageID: "msg-plan-json-failed",
		CorpID:            "corp-a",
		AgentID:           "1000002",
		ExternalUserID:    "zhangsan",
		MsgType:           "text",
		TextContent:       "搜索最新港股消息并分析",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if result.Turn.Status != domain.AgentTurnStatusFailed {
		t.Fatalf("turn status = %q, want failed", result.Turn.Status)
	}
	if repository.inbound.Status != domain.AgentInboundMessageStatusFailed {
		t.Fatalf("inbound status = %q, want failed", repository.inbound.Status)
	}
	if sender.calls == 0 || !strings.Contains(sender.sent.Content, "没有成功") {
		t.Fatalf("failure feedback was not sent: calls=%d sent=%#v", sender.calls, sender.sent)
	}
	if !fakeAuditContains(repository.audits, "agent.turn_failed") || !fakeAuditContains(repository.audits, "agent.turn_failure_feedback") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

type fakeAgentConversationRepository struct {
	nextID           int64
	forceDuplicate   bool
	createPlanErr    error
	account          domain.ExternalAccount
	inbound          domain.AgentInboundMessage
	session          domain.AgentSession
	turns            []domain.AgentTurn
	transcripts      []domain.AgentTranscriptEntry
	recalls          []domain.AgentRecallEvent
	audits           []domain.AgentAuditLog
	runs             []domain.AgentRun
	contextTraces    []domain.AgentRunContextTrace
	observations     []domain.AgentObservation
	artifacts        []domain.AgentArtifact
	plans            []domain.AgentPlan
	scheduledTasks   []domain.AgentScheduledTask
	approvals        []domain.AgentApproval
	preference       domain.AgentNotificationPreference
	capabilityLogs   []domain.AgentCapabilityAuditLog
	externalAccounts []domain.ExternalAccount
}

func newFakeAgentConversationRepository() *fakeAgentConversationRepository {
	return &fakeAgentConversationRepository{nextID: 1}
}

func (r *fakeAgentConversationRepository) id() int64 {
	id := r.nextID
	r.nextID++
	return id
}

func (r *fakeAgentConversationRepository) EnsureExternalAccount(_ context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error) {
	if r.account.ID == 0 {
		account.ID = r.id()
		r.account = account
	}
	return r.account, nil
}

func (r *fakeAgentConversationRepository) ListExternalAccounts(_ context.Context, userID int64) ([]domain.ExternalAccount, error) {
	accounts := make([]domain.ExternalAccount, 0, len(r.externalAccounts))
	for _, account := range r.externalAccounts {
		if account.UserID == userID {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *fakeAgentConversationRepository) CreateInboundMessage(_ context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error) {
	if r.inbound.ID == 0 {
		message.ID = r.id()
		r.inbound = message
	}
	if r.forceDuplicate {
		return r.inbound, false, nil
	}
	return r.inbound, true, nil
}

func (r *fakeAgentConversationRepository) UpdateInboundMessageStatus(_ context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error) {
	if r.inbound.ID == id && r.inbound.UserID == userID {
		r.inbound.Status = status
		r.inbound.UpdatedAt = now
		return r.inbound, nil
	}
	return domain.AgentInboundMessage{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) GetOrCreateSession(_ context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	if r.session.ID == 0 {
		session.ID = r.id()
		r.session = session
	}
	return r.session, nil
}

func (r *fakeAgentConversationRepository) GetAgentSession(_ context.Context, userID int64, sessionID int64) (domain.AgentSession, error) {
	if r.session.ID == sessionID && r.session.UserID == userID {
		return r.session, nil
	}
	return domain.AgentSession{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) TouchAgentSession(_ context.Context, userID int64, sessionID int64, now time.Time) error {
	if r.session.ID == sessionID && r.session.UserID == userID {
		r.session.LastActiveAt = now
		return nil
	}
	return domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) CreateTurn(_ context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	turn.ID = r.id()
	r.turns = append(r.turns, turn)
	return turn, nil
}

func (r *fakeAgentConversationRepository) UpdateTurn(_ context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	for i := range r.turns {
		if r.turns[i].ID == turn.ID {
			r.turns[i] = turn
			return turn, nil
		}
	}
	r.turns = append(r.turns, turn)
	return turn, nil
}

func (r *fakeAgentConversationRepository) AppendTranscriptEntry(_ context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error) {
	entry.ID = r.id()
	r.transcripts = append(r.transcripts, entry)
	return entry, nil
}

func (r *fakeAgentConversationRepository) ListRecentTranscriptEntries(_ context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error) {
	entries := make([]domain.AgentTranscriptEntry, 0, len(r.transcripts))
	for _, entry := range r.transcripts {
		if entry.SessionID != options.SessionID || entry.UserID != options.UserID {
			continue
		}
		if options.BeforeTurnID > 0 && entry.TurnID >= options.BeforeTurnID {
			continue
		}
		if len(options.Roles) > 0 && !fakeTranscriptRoleAllowed(entry.Role, options.Roles) {
			continue
		}
		entries = append(entries, entry)
	}
	if options.Limit > 0 && len(entries) > options.Limit {
		entries = entries[len(entries)-options.Limit:]
	}
	return entries, nil
}

func (r *fakeAgentConversationRepository) QueryTranscriptEntries(_ context.Context, options domain.AgentTranscriptQueryOptions) ([]domain.AgentTranscriptEntry, error) {
	entries := make([]domain.AgentTranscriptEntry, 0, len(r.transcripts))
	keyword := strings.ToLower(strings.TrimSpace(options.Keyword))
	for _, entry := range r.transcripts {
		if entry.SessionID != options.SessionID || entry.UserID != options.UserID {
			continue
		}
		if options.BeforeTurnID > 0 && entry.TurnID >= options.BeforeTurnID {
			continue
		}
		if options.After != nil && entry.CreatedAt.Before(options.After.UTC()) {
			continue
		}
		if options.Before != nil && entry.CreatedAt.After(options.Before.UTC()) {
			continue
		}
		if options.BeforeEntryID > 0 && entry.ID >= options.BeforeEntryID {
			continue
		}
		if options.AfterEntryID > 0 && entry.ID <= options.AfterEntryID {
			continue
		}
		if len(options.Roles) > 0 && !fakeTranscriptRoleAllowed(entry.Role, options.Roles) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(entry.Content), keyword) {
			continue
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			if strings.ToLower(strings.TrimSpace(options.Order)) == "asc" {
				return entries[i].ID < entries[j].ID
			}
			return entries[i].ID > entries[j].ID
		}
		if strings.ToLower(strings.TrimSpace(options.Order)) == "asc" {
			return entries[i].CreatedAt.Before(entries[j].CreatedAt)
		}
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	if options.Offset > 0 {
		if options.Offset >= len(entries) {
			return nil, nil
		}
		entries = entries[options.Offset:]
	}
	if options.Limit > 0 && len(entries) > options.Limit {
		entries = entries[:options.Limit]
	}
	if strings.ToLower(strings.TrimSpace(options.Order)) != "asc" {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}
	return entries, nil
}

type fakeAgentNotificationJobStore struct {
	jobs []domain.NotificationJob
	err  error
}

func (s *fakeAgentNotificationJobStore) CreateJob(_ context.Context, job domain.NotificationJob) (domain.NotificationJob, error) {
	if s.err != nil {
		return domain.NotificationJob{}, s.err
	}
	job.ID = int64(len(s.jobs) + 1)
	s.jobs = append(s.jobs, job)
	return job, nil
}

func (r *fakeAgentConversationRepository) CreateRecallEvent(_ context.Context, event domain.AgentRecallEvent) (domain.AgentRecallEvent, error) {
	event.ID = r.id()
	r.recalls = append(r.recalls, event)
	return event, nil
}

func (r *fakeAgentConversationRepository) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = r.id()
	r.audits = append(r.audits, log)
	return log, nil
}

func (r *fakeAgentConversationRepository) CreateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	run.ID = r.id()
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAgentConversationRepository) UpdateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	for i := range r.runs {
		if r.runs[i].ID == run.ID {
			r.runs[i] = run
			return run, nil
		}
	}
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAgentConversationRepository) CreateAgentRunContextTrace(_ context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error) {
	trace.ID = r.id()
	r.contextTraces = append(r.contextTraces, trace)
	return trace, nil
}

func (r *fakeAgentConversationRepository) CreateAgentObservation(_ context.Context, observation domain.AgentObservation) (domain.AgentObservation, error) {
	observation.ID = r.id()
	r.observations = append(r.observations, observation)
	return observation, nil
}

func (r *fakeAgentConversationRepository) CreateAgentArtifact(_ context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error) {
	artifact.ID = r.id()
	r.artifacts = append(r.artifacts, artifact)
	return artifact, nil
}

func (r *fakeAgentConversationRepository) CreateAgentPlan(_ context.Context, plan domain.AgentPlan, steps []domain.AgentPlanStep) (domain.AgentPlan, error) {
	if r.createPlanErr != nil {
		return domain.AgentPlan{}, r.createPlanErr
	}
	plan.ID = r.id()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now().UTC()
	}
	if plan.UpdatedAt.IsZero() {
		plan.UpdatedAt = plan.CreatedAt
	}
	for index := range steps {
		steps[index].ID = r.id()
		steps[index].PlanID = plan.ID
		if steps[index].StepOrder == 0 {
			steps[index].StepOrder = index + 1
		}
	}
	plan.Steps = append([]domain.AgentPlanStep(nil), steps...)
	r.plans = append(r.plans, plan)
	return plan, nil
}

func (r *fakeAgentConversationRepository) ListAgentPlans(_ context.Context, userID int64, sessionID int64, turnID int64, limit int) ([]domain.AgentPlan, error) {
	plans := make([]domain.AgentPlan, 0, len(r.plans))
	for _, plan := range r.plans {
		if userID > 0 && plan.UserID != userID {
			continue
		}
		if sessionID > 0 && plan.SessionID != sessionID {
			continue
		}
		if turnID > 0 && plan.TurnID != turnID {
			continue
		}
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].CreatedAt.Equal(plans[j].CreatedAt) {
			return plans[i].ID > plans[j].ID
		}
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})
	if limit > 0 && len(plans) > limit {
		plans = plans[:limit]
	}
	return plans, nil
}

func (r *fakeAgentConversationRepository) ListAgentScheduledTasks(_ context.Context, options domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error) {
	tasks := make([]domain.AgentScheduledTask, 0, len(r.scheduledTasks))
	for _, task := range r.scheduledTasks {
		if options.UserID > 0 && task.UserID != options.UserID {
			continue
		}
		tasks = append(tasks, task)
	}
	if options.Limit > 0 && len(tasks) > options.Limit {
		tasks = tasks[:options.Limit]
	}
	return tasks, nil
}

func (r *fakeAgentConversationRepository) UpdateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	for index := range r.scheduledTasks {
		if r.scheduledTasks[index].ID == task.ID && r.scheduledTasks[index].UserID == task.UserID {
			r.scheduledTasks[index] = task
			return task, nil
		}
	}
	return domain.AgentScheduledTask{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) UpdateAgentPlanStatus(_ context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time, errorMessage string) (domain.AgentPlan, error) {
	for i := range r.plans {
		if r.plans[i].ID == planID && r.plans[i].UserID == userID {
			r.plans[i].Status = status
			r.plans[i].ErrorMessage = errorMessage
			r.plans[i].UpdatedAt = now
			switch status {
			case domain.AgentPlanStatusCompleted:
				r.plans[i].CompletedAt = &now
			case domain.AgentPlanStatusFailed:
				r.plans[i].FailedAt = &now
			}
			return r.plans[i], nil
		}
	}
	return domain.AgentPlan{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) UpdateAgentPlanMetadata(_ context.Context, userID int64, planID int64, metadata domain.AgentJSON, now time.Time) (domain.AgentPlan, error) {
	for i := range r.plans {
		if r.plans[i].ID == planID && r.plans[i].UserID == userID {
			r.plans[i].Metadata = cloneApprovalMetadata(metadata)
			r.plans[i].UpdatedAt = now
			return r.plans[i], nil
		}
	}
	return domain.AgentPlan{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) UpdateAgentPlanStepStatus(_ context.Context, userID int64, step domain.AgentPlanStep) (domain.AgentPlanStep, error) {
	for planIndex := range r.plans {
		if r.plans[planIndex].UserID != userID {
			continue
		}
		for stepIndex := range r.plans[planIndex].Steps {
			if r.plans[planIndex].Steps[stepIndex].ID == step.ID {
				r.plans[planIndex].Steps[stepIndex] = step
				return step, nil
			}
		}
	}
	return domain.AgentPlanStep{}, domain.ErrNotFound
}

func (r *fakeAgentConversationRepository) CreateAgentApproval(_ context.Context, approval domain.AgentApproval) (domain.AgentApproval, error) {
	approval.ID = r.id()
	r.approvals = append(r.approvals, approval)
	return approval, nil
}

func (r *fakeAgentConversationRepository) CreateAgentCapabilityAuditLog(_ context.Context, log domain.AgentCapabilityAuditLog) (domain.AgentCapabilityAuditLog, error) {
	log.ID = r.id()
	r.capabilityLogs = append(r.capabilityLogs, log)
	return log, nil
}

func (r *fakeAgentConversationRepository) GetAgentNotificationPreference(_ context.Context, userID int64) (domain.AgentNotificationPreference, error) {
	if r.preference.UserID == userID && userID > 0 {
		return r.preference, nil
	}
	return defaultAgentNotificationPreference(userID, time.Now().UTC()), nil
}

func fakeTranscriptRoleAllowed(role domain.AgentTranscriptRole, roles []domain.AgentTranscriptRole) bool {
	for _, allowed := range roles {
		if role == allowed {
			return true
		}
	}
	return false
}

func fakeAuditContains(audits []domain.AgentAuditLog, eventType string) bool {
	for _, audit := range audits {
		if audit.EventType == eventType {
			return true
		}
	}
	return false
}

func fakeAuditByType(audits []domain.AgentAuditLog, eventType string) domain.AgentAuditLog {
	for _, audit := range audits {
		if audit.EventType == eventType {
			return audit
		}
	}
	return domain.AgentAuditLog{}
}

func fakeContextTraceContains(traces []domain.AgentRunContextTrace, traceKind string) bool {
	for _, trace := range traces {
		if trace.TraceKind == traceKind {
			return true
		}
	}
	return false
}

func fakeObservationContains(observations []domain.AgentObservation, capabilityKey string, status string) bool {
	for _, observation := range observations {
		if observation.CapabilityKey == capabilityKey && observation.Status == status {
			return true
		}
	}
	return false
}

type fakeAgentExternalAccountResolver struct {
	account domain.ExternalAccount
	err     error
}

func (f *fakeAgentExternalAccountResolver) ResolveExternalAccount(_ context.Context, provider string, corpID string, agentID string, externalUserID string) (domain.ExternalAccount, error) {
	if f.err != nil {
		return domain.ExternalAccount{}, f.err
	}
	return f.account, nil
}

type fakeAgentUserContextProvider struct{}

func (fakeAgentUserContextProvider) BuildAgentUserContext(_ context.Context, userID int64) (UserContextResult, error) {
	return UserContextResult{
		User: AuthUserResponse{
			ID:          userID,
			Username:    "aroen",
			DisplayName: "aroen",
			Role:        string(domain.UserRoleOwner),
			Status:      string(domain.UserStatusActive),
		},
		Profile: UserProfileResponse{
			DisplayName: "aroen",
			TimeZone:    "Asia/Shanghai",
			Language:    "zh-CN",
			ReplyStyle:  "plain_text_short",
		},
		DataScope: UserDataScopeResponse{UserID: userID},
		Prompt:    UserPromptContext{PlainText: "当前用户：aroen\n数据边界：只能读取和操作 user_id=1 的数据。"},
	}, nil
}

func testAgentExternalAccount(now time.Time) domain.ExternalAccount {
	return domain.ExternalAccount{
		ID:             10,
		UserID:         1,
		Provider:       domain.AgentProviderWeChatWorkApp,
		CorpID:         "corp-a",
		AgentID:        "1000002",
		ExternalUserID: "zhangsan",
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
		VerifiedAt:     &now,
		LastSeenAt:     &now,
	}
}

type fakeAgentConversationLLM struct {
	calls       int
	lastRequest llm.ChatRequest
	response    llm.ChatResponse
	responses   []llm.ChatResponse
	err         error
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
}

func (f *fakeAgentConversationLLM) Chat(_ context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	f.calls++
	f.lastRequest = request
	if f.started != nil {
		f.startOnce.Do(func() { close(f.started) })
	}
	if f.release != nil {
		<-f.release
	}
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	if len(f.responses) > 0 {
		index := f.calls - 1
		if index >= len(f.responses) {
			index = len(f.responses) - 1
		}
		return f.responses[index], nil
	}
	return f.response, nil
}

type fakeAgentConversationSender struct {
	calls          int
	templateCalls  int
	sent           notifier.WeChatWorkTextMessage
	sentTemplate   notifier.WeChatWorkTemplateCardMessage
	sentMessages   []notifier.WeChatWorkTextMessage
	sentTemplates  []notifier.WeChatWorkTemplateCardMessage
	result         notifier.WeChatWorkSendResult
	templateResult notifier.WeChatWorkSendResult
	err            error
	templateErr    error
	sentEvents     chan notifier.WeChatWorkTextMessage
	sentSignal     chan struct{}
	sentOnce       sync.Once
}

func (f *fakeAgentConversationSender) SendText(_ context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error) {
	f.calls++
	f.sent = message
	f.sentMessages = append(f.sentMessages, message)
	if f.sentEvents != nil {
		f.sentEvents <- message
	}
	if f.sentSignal != nil {
		f.sentOnce.Do(func() { close(f.sentSignal) })
	}
	if f.err != nil {
		return notifier.WeChatWorkSendResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeAgentConversationSender) SendTemplateCard(_ context.Context, message notifier.WeChatWorkTemplateCardMessage) (notifier.WeChatWorkSendResult, error) {
	f.templateCalls++
	f.sentTemplate = message
	f.sentTemplates = append(f.sentTemplates, message)
	if f.templateErr != nil {
		return notifier.WeChatWorkSendResult{}, f.templateErr
	}
	if f.sentEvents != nil {
		f.sentEvents <- notifier.WeChatWorkTextMessage{ToUser: message.ToUser, Content: message.FallbackText}
	}
	if f.sentSignal != nil {
		f.sentOnce.Do(func() { close(f.sentSignal) })
	}
	if f.templateResult.MessageID != "" || f.templateResult.ErrCode != 0 || f.templateResult.ErrMsg != "" {
		return f.templateResult, nil
	}
	return f.result, nil
}

type fakeAgentRecentItemsProvider struct {
	itemsBySource map[int64][]domain.Item
}

func (f *fakeAgentRecentItemsProvider) ListItems(_ context.Context, input ListItemsInput) (ListItemsResult, error) {
	items := f.itemsBySource[input.SourceID]
	if len(items) > input.Limit && input.Limit > 0 {
		items = items[:input.Limit]
	}
	return ListItemsResult{Items: items, Total: int64(len(items)), Limit: input.Limit, Offset: input.Offset}, nil
}

type fakeAgentSourceProvider struct {
	sources []domain.Source
}

func (f *fakeAgentSourceProvider) ListSources(_ context.Context, _ int64) ([]domain.Source, error) {
	return f.sources, nil
}
