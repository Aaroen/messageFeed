package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
	"sort"
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
	if len(repository.transcripts) != 2 {
		t.Fatalf("transcript count = %d, want 2", len(repository.transcripts))
	}
	if len(repository.audits) != 3 || repository.audits[2].Status != "succeeded" {
		t.Fatalf("audits = %#v", repository.audits)
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
	sendDone := make(chan struct{})
	llmClient := &fakeAgentConversationLLM{
		started: llmStarted,
		release: llmRelease,
		response: llm.ChatResponse{
			Provider: "openai_compatible",
			Model:    "custom-model",
			Content:  "异步回复",
		},
	}
	sender := &fakeAgentConversationSender{sentSignal: sendDone}
	service := NewAgentConversationService(
		repository,
		WithAgentConversationLLM(llmClient),
		WithAgentConversationSender(sender),
		WithAgentConversationExternalAccountResolver(resolver),
		WithAgentConversationProcessTimeout(time.Second),
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
	if sender.calls != 0 {
		t.Fatalf("sender calls before release = %d, want 0", sender.calls)
	}

	select {
	case <-llmStarted:
	case <-time.After(time.Second):
		t.Fatal("llm was not started asynchronously")
	}
	close(llmRelease)
	select {
	case <-sendDone:
	case <-time.After(time.Second):
		t.Fatal("async reply was not sent")
	}
	if repository.turns[0].Status != domain.AgentTurnStatusSucceeded {
		t.Fatalf("final turn status = %q", repository.turns[0].Status)
	}
	if repository.inbound.Status != domain.AgentInboundMessageStatusSucceeded {
		t.Fatalf("inbound status = %q", repository.inbound.Status)
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
	if !strings.Contains(systemPrompt, "最近条目") || !strings.Contains(systemPrompt, "Go 1.26 发布") {
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
		TextContent:       "确认创建明天上午9点提醒我检查部署状态",
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
		TextContent:       "再在今晚10点10分提醒我到点了，我已经确认",
	})
	if err != nil {
		t.Fatalf("ReceiveWeChatWorkAppMessage() error = %v", err)
	}
	if !strings.Contains(result.Reply, "没有设置成功") {
		t.Fatalf("fallback reply = %q", result.Reply)
	}
	if sender.calls == 0 || !strings.Contains(sender.sent.Content, "没有设置成功") {
		t.Fatalf("sent fallback = %#v", sender.sent)
	}
	if len(repository.transcripts) < 2 || !strings.Contains(repository.transcripts[len(repository.transcripts)-1].Content, "没有设置成功") {
		t.Fatalf("transcripts = %#v", repository.transcripts)
	}
}

type fakeAgentConversationRepository struct {
	nextID         int64
	forceDuplicate bool
	account        domain.ExternalAccount
	inbound        domain.AgentInboundMessage
	session        domain.AgentSession
	turns          []domain.AgentTurn
	transcripts    []domain.AgentTranscriptEntry
	recalls        []domain.AgentRecallEvent
	audits         []domain.AgentAuditLog
	runs           []domain.AgentRun
	contextTraces  []domain.AgentRunContextTrace
	observations   []domain.AgentObservation
	artifacts      []domain.AgentArtifact
	plans          []domain.AgentPlan
	approvals      []domain.AgentApproval
	capabilityLogs []domain.AgentCapabilityAuditLog
}

func newFakeAgentConversationRepository() *fakeAgentConversationRepository {
	return &fakeAgentConversationRepository{nextID: 1}
}

func (r *fakeAgentConversationRepository) id() int64 {
	id := r.nextID
	r.nextID++
	return id
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
	plan.ID = r.id()
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

func fakeTranscriptRoleAllowed(role domain.AgentTranscriptRole, roles []domain.AgentTranscriptRole) bool {
	for _, allowed := range roles {
		if role == allowed {
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
	calls        int
	sent         notifier.WeChatWorkTextMessage
	sentMessages []notifier.WeChatWorkTextMessage
	result       notifier.WeChatWorkSendResult
	err          error
	sentSignal   chan struct{}
	sentOnce     sync.Once
}

func (f *fakeAgentConversationSender) SendText(_ context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error) {
	f.calls++
	f.sent = message
	f.sentMessages = append(f.sentMessages, message)
	if f.sentSignal != nil {
		f.sentOnce.Do(func() { close(f.sentSignal) })
	}
	if f.err != nil {
		return notifier.WeChatWorkSendResult{}, f.err
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
