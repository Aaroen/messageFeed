package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/notifier"
	"strings"
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
	if len(repository.audits) != 1 || repository.audits[0].Status != "succeeded" {
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
	service := NewAgentConversationService(repository, WithAgentConversationSender(sender), WithAgentConversationExternalAccountResolver(resolver))

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

type fakeAgentConversationRepository struct {
	nextID         int64
	forceDuplicate bool
	account        domain.ExternalAccount
	inbound        domain.AgentInboundMessage
	session        domain.AgentSession
	turns          []domain.AgentTurn
	transcripts    []domain.AgentTranscriptEntry
	audits         []domain.AgentAuditLog
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

func (r *fakeAgentConversationRepository) GetOrCreateSession(_ context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	if r.session.ID == 0 {
		session.ID = r.id()
		r.session = session
	}
	return r.session, nil
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

func (r *fakeAgentConversationRepository) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = r.id()
	r.audits = append(r.audits, log)
	return log, nil
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
	err         error
}

func (f *fakeAgentConversationLLM) Chat(_ context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	f.calls++
	f.lastRequest = request
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	return f.response, nil
}

type fakeAgentConversationSender struct {
	calls        int
	sent         notifier.WeChatWorkTextMessage
	sentMessages []notifier.WeChatWorkTextMessage
	result       notifier.WeChatWorkSendResult
	err          error
}

func (f *fakeAgentConversationSender) SendText(_ context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error) {
	f.calls++
	f.sent = message
	f.sentMessages = append(f.sentMessages, message)
	if f.err != nil {
		return notifier.WeChatWorkSendResult{}, f.err
	}
	return f.result, nil
}
