package agent

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strings"
	"testing"
	"time"
)

func TestTurnRunnerExecutesToolCallAndContinuesChat(t *testing.T) {
	now := time.Date(2026, 6, 24, 21, 0, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"keyword":"偏好"}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "历史记录显示你偏好 Go。"},
		},
	}
	toolExecutor := &runnerFakeToolExecutor{content: "2026-06-24 12:00 用户：我的偏好是 Go。"}
	store := &runnerFakeTurnStore{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        store,
		ToolExecutor: toolExecutor,
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID: 1,
		Session: domain.AgentSession{
			ID:     10,
			UserID: 1,
		},
		Turn: domain.AgentTurn{
			ID:        20,
			SessionID: 10,
			UserID:    1,
			Status:    domain.AgentTurnStatusRunning,
		},
		InboundMessage: domain.AgentInboundMessage{ID: 30, UserID: 1},
		MessageType:    "text",
		MessageText:    "查一下我的偏好",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Reply != "历史记录显示你偏好 Go。" {
		t.Fatalf("Reply = %q", result.Reply)
	}
	if chat.calls != 2 {
		t.Fatalf("chat calls = %d, want 2", chat.calls)
	}
	if chat.requests[0].ToolChoice != "required" {
		t.Fatalf("first tool choice = %q, want required", chat.requests[0].ToolChoice)
	}
	if toolExecutor.input.Capability.Key != "conversation.query_history" {
		t.Fatalf("tool capability = %q", toolExecutor.input.Capability.Key)
	}
	finalMessages := chat.requests[1].Messages
	toolMessage := finalMessages[len(finalMessages)-1]
	if toolMessage.Role != "tool" || !strings.Contains(toolMessage.Content, "我的偏好是 Go") {
		t.Fatalf("tool message = %#v", toolMessage)
	}
	if len(result.Context.Observations) != 1 || result.Context.Observations[0].Capability != "conversation.query_history" {
		t.Fatalf("observations = %#v", result.Context.Observations)
	}
	if len(store.transcripts) != 1 || store.transcripts[0].Role != domain.AgentTranscriptRoleAssistant {
		t.Fatalf("assistant transcript = %#v", store.transcripts)
	}
}

func TestTurnRunnerRetriesWhenModelSkipsRequiredTool(t *testing.T) {
	now := time.Date(2026, 6, 28, 18, 20, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{Provider: "openai_compatible", Model: "custom-model", Content: "我可以直接回答。"},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"query":"能力"}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "历史记录显示你询问过 Agent 能力。"},
		},
	}
	audit := &runnerFakeAuditLogger{}
	toolExecutor := &runnerFakeToolExecutor{content: "2026-06-28 18:00 用户：Agent 能力有哪些？"}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: toolExecutor,
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "结合历史回答我的 Agent 能力",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Reply != "历史记录显示你询问过 Agent 能力。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if chat.calls != 3 {
		t.Fatalf("chat calls = %d, want 3", chat.calls)
	}
	if chat.requests[0].ToolChoice != "required" || chat.requests[1].ToolChoice != "required" || chat.requests[2].ToolChoice != "auto" {
		t.Fatalf("tool choices = %#v", []string{chat.requests[0].ToolChoice, chat.requests[1].ToolChoice, chat.requests[2].ToolChoice})
	}
	if !runnerAuditContains(audit.events, "agent.required_tool_call_retry") {
		t.Fatalf("audit events = %#v", audit.events)
	}
	if toolExecutor.input.Capability.Key != "conversation.query_history" {
		t.Fatalf("tool capability = %q", toolExecutor.input.Capability.Key)
	}
}

func TestTurnRunnerRequiresToolWhenSnapshotOnlyHasContextObservation(t *testing.T) {
	now := time.Date(2026, 6, 28, 18, 25, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{Provider: "openai_compatible", Model: "custom-model", Content: "我先直接回答。"},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"query":"偏好"}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "历史记录显示你关注 Go。"},
		},
	}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store: &runnerFakeTurnStore{},
		ContextBuilder: runnerFakeContextBuilder{snapshot: ContextSnapshot{
			Observations: []CapabilityObservation{
				{Capability: "user.context", Decision: string(PolicyDecisionAllow), Status: "succeeded", Summary: "loaded user context"},
				{Capability: "conversation.query_recent", Decision: string(PolicyDecisionAllow), Status: "succeeded", Summary: "loaded recent messages"},
			},
		}},
		ToolExecutor: &runnerFakeToolExecutor{content: "2026-06-24 用户：我关注 Go。"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "请结合历史回答我的偏好",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if chat.calls != 3 {
		t.Fatalf("chat calls = %d, want 3", chat.calls)
	}
	if result.Reply != "历史记录显示你关注 Go。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if !hasObservationForMCPTools(result.Context.Observations, runner.listMCPTools([]string{"conversation.query_history"})) {
		t.Fatalf("tool observation missing: %#v", result.Context.Observations)
	}
}

func TestTurnRunnerFallsBackToPromptedToolActionWhenNativeToolsReturnEmpty(t *testing.T) {
	now := time.Date(2026, 6, 28, 19, 10, 0, 0, time.UTC)
	emptyErr := domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "test", true, nil)
	chat := &runnerFakeChatClient{
		errs: []error{
			emptyErr,
			nil,
			emptyErr,
			nil,
		},
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `{"action":"tool_call","tool_name":"conversation.query_history","arguments":{"query":"长期偏好","limit":8},"reason":"需要读取历史"}`,
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `{"action":"final","content":"历史记录显示你长期关注 Go 和 AI 基础设施。","reason":"已有历史观察"}`,
			},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: &runnerFakeToolExecutor{content: "2026-06-24 用户：我的长期偏好是 Go 和 AI 基础设施。"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "请根据历史聊天原文查一下我的长期偏好是什么",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Reply != "历史记录显示你长期关注 Go 和 AI 基础设施。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if chat.calls != 4 {
		t.Fatalf("chat calls = %d, want 4", chat.calls)
	}
	if len(chat.requests[1].Tools) != 0 || chat.requests[1].ToolChoice != "" {
		t.Fatalf("prompted tool request should not use native tools: %#v", chat.requests[1])
	}
	if !strings.Contains(chat.requests[1].Messages[len(chat.requests[1].Messages)-1].Content, "available_tools") {
		t.Fatalf("prompted tool request payload = %q", chat.requests[1].Messages[len(chat.requests[1].Messages)-1].Content)
	}
	if !runnerAuditContains(audit.events, "agent.prompted_tool_action_fallback") {
		t.Fatalf("audit events = %#v", audit.events)
	}
	if len(result.Context.Observations) == 0 || result.Context.Observations[len(result.Context.Observations)-1].Capability != "conversation.query_history" {
		t.Fatalf("observations = %#v", result.Context.Observations)
	}
}

func TestTurnRunnerRejectsUnparsedToolCallMarkupAsFinalReply(t *testing.T) {
	now := time.Date(2026, 6, 28, 22, 55, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation.query_history", Arguments: `{"query":"市场分析","limit":5}`},
				},
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `<|tool_calls_section_begin|> <|tool_call_begin|> chatcmpl-tool-abc <|tool_call_argument_begin|> {"url":"https://example.com/report"} <|tool_call_end|> <|tool_calls_section_end|>`,
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `{"action":"final","content":"基于已有证据，市场消息面偏谨慎。","reason":"已有工具观察足够形成简短结论"}`,
			},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: &runnerFakeToolExecutor{content: "历史记录：用户关注港美股和A股。"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "分析市场",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(result.Reply, "<|tool_calls_section_begin|>") || strings.Contains(result.Reply, "chatcmpl-tool-") {
		t.Fatalf("reply leaked unparsed tool markup: %q", result.Reply)
	}
	if result.Reply != "基于已有证据，市场消息面偏谨慎。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if chat.calls != 3 {
		t.Fatalf("chat calls = %d, want 3", chat.calls)
	}
	if !runnerAuditContains(audit.events, "agent.prompted_tool_action_fallback") {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestTurnRunnerRepairsMalformedPromptedToolAction(t *testing.T) {
	now := time.Date(2026, 6, 28, 23, 40, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation.query_history", Arguments: `{"query":"市场分析","limit":5}`},
				},
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `<|tool_calls_section_begin|> <|tool_call_begin|> chatcmpl-tool-abc <|tool_call_argument_begin|> {"url":"https://example.com/report"} <|tool_call_end|> <|tool_calls_section_end|>`,
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `{"url":"https://example.com/report"}`,
			},
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				Content:  `{"action":"final","content":"已有证据显示，市场消息面需要分市场审慎观察。","reason":"修复格式后收敛回答"}`,
			},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: &runnerFakeToolExecutor{content: "历史记录：用户关注港美股和A股。"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "分析市场",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Reply != "已有证据显示，市场消息面需要分市场审慎观察。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if chat.calls != 4 {
		t.Fatalf("chat calls = %d, want 4", chat.calls)
	}
	if !runnerAuditContains(audit.events, "agent.prompted_tool_action_repair_retry") {
		t.Fatalf("audit events = %#v", audit.events)
	}
	lastRequest := chat.requests[len(chat.requests)-1]
	lastMessage := lastRequest.Messages[len(lastRequest.Messages)-1]
	if lastMessage.Role != "user" || !strings.Contains(lastMessage.Content, "previous_error") {
		t.Fatalf("repair prompt message = %#v", lastMessage)
	}
}

func TestTurnRunnerFailsWhenModelKeepsSkippingRequiredTool(t *testing.T) {
	now := time.Date(2026, 6, 28, 18, 30, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{Provider: "openai_compatible", Model: "custom-model", Content: "第一次直接回答。"},
			{Provider: "openai_compatible", Model: "custom-model", Content: "第二次仍然直接回答。"},
			{Provider: "openai_compatible", Model: "custom-model", Content: "第三次仍然没有工具。"},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: &runnerFakeToolExecutor{content: "不会执行"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "必须结合历史再回答",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, result = %#v", result)
	}
	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "agent_required_tool_skipped" {
		t.Fatalf("error = %T %v, want agent_required_tool_skipped", err, err)
	}
	if chat.calls != 3 {
		t.Fatalf("chat calls = %d, want 3", chat.calls)
	}
	if result.Turn.Status != domain.AgentTurnStatusFailed {
		t.Fatalf("turn status = %q, want failed", result.Turn.Status)
	}
	if !runnerAuditContains(audit.events, "agent.required_tool_call_retry") {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestTurnRunnerRetriesEmptyResponseAfterToolObservation(t *testing.T) {
	now := time.Date(2026, 6, 28, 16, 10, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		errs: []error{
			nil,
			domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "test", true, nil),
			nil,
		},
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "conversation__query_history", Arguments: `{"keyword":"偏好"}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "历史记录显示你偏好 Go 和 AI。"},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: &runnerFakeToolExecutor{content: "2026-06-24 12:00 用户：我的偏好是 Go 和 AI。"},
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"conversation.query_history"},
		MessageType:     "text",
		MessageText:     "查一下我的偏好",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Reply, "Go") || !strings.Contains(result.Reply, "AI") {
		t.Fatalf("reply = %q", result.Reply)
	}
	if chat.calls != 4 {
		t.Fatalf("chat calls = %d, want 4", chat.calls)
	}
	if !runnerAuditContains(audit.events, "agent.llm_empty_response_retry") {
		t.Fatalf("audit events = %#v", audit.events)
	}
	lastRequest := chat.requests[len(chat.requests)-1]
	lastMessage := lastRequest.Messages[len(lastRequest.Messages)-1]
	if lastMessage.Role != "user" || !strings.Contains(lastMessage.Content, "上一轮模型没有返回内容") {
		t.Fatalf("retry prompt message = %#v", lastMessage)
	}
}

func TestTurnRunnerSystemPromptGuidesScheduledMessageConfirmation(t *testing.T) {
	now := time.Date(2026, 6, 24, 13, 55, 0, 0, time.UTC)
	runner := NewTurnRunner(TurnRunnerOptions{
		ToolExecutor: &runnerFakeToolExecutor{},
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	prompt := runner.buildSystemPrompt(ContextSnapshot{}, "搜索最新港股消息并分析")
	for _, required := range []string{
		"任务规格：",
		"结构化 PlanSpec",
		"当前时间：2026-06-24 21:55:00 Asia/Shanghai",
		"web.search",
		"web.fetch_page",
		"web.extract_page",
		"repo.search",
		"repo.inspect_remote",
		"不得克隆仓库",
		"来源、抓取时间和摘要",
		"归一化为 scheduled_at",
		"优先使用 agent.schedule_task",
		"再次调用对应定时工具",
		"confirmed=true",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing %q: %s", required, prompt)
		}
	}
}

func TestTurnRunnerRejectsToolOutsideCurrentScope(t *testing.T) {
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{
				Provider: "openai_compatible",
				Model:    "custom-model",
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", Name: "web__search", Arguments: `{"query":"messageFeed"}`},
				},
			},
			{Provider: "openai_compatible", Model: "custom-model", Content: "该能力未获批准，不能执行联网搜索。"},
		},
	}
	toolExecutor := &runnerFakeToolExecutor{}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		ToolExecutor: toolExecutor,
		ToolKeys:     []string{"conversation.query_history", "web.search"},
		LLMClient:    chat,
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		AllowedToolKeys: []string{"conversation.query_history"},
		InboundMessage: domain.AgentInboundMessage{
			ID:     30,
			UserID: 1,
		},
		MessageType: "text",
		MessageText: "联网搜索 messageFeed",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if toolExecutor.input.Capability.Key != "" {
		t.Fatalf("tool was executed outside scope: %#v", toolExecutor.input)
	}
	if len(result.Context.Observations) != 1 || result.Context.Observations[0].Decision != string(PolicyDecisionForbidden) {
		t.Fatalf("observations = %#v", result.Context.Observations)
	}
	if len(audit.events) == 0 || audit.events[0].EventType != "agent.capability_scope_denied" {
		t.Fatalf("audit events = %#v", audit.events)
	}
}

func TestTurnRunnerFailsOnLLMEmptyResponseWithEvidence(t *testing.T) {
	now := time.Date(2026, 6, 26, 21, 0, 0, 0, time.UTC)
	store := &runnerFakeTurnStore{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store: store,
		ContextBuilder: runnerFakeContextBuilder{snapshot: ContextSnapshot{
			Blocks: []ContextBlock{
				{
					Name:          "最近条目",
					CapabilityKey: "feed.query_recent_items",
					Content:       "结果：\n1. 美伊谈判：中国用何利器削弱美国制裁（RFI 中文）\nURL：https://example.com/rfi",
					ItemCount:     1,
				},
				{
					Name:          "联网搜索结果",
					CapabilityKey: "web.search",
					Content: "工具：web.search\n查询：港股\n结果：\n" +
						"1. 港股市场新闻（财经新闻）\n发布时间：Fri, 26 Jun 2026 13:00:00 GMT\nURL：https://example.com/hk\n摘要：恒生指数下跌，科技股走弱。\n" +
						"2. 港股通资金观察（财联社）\n发布时间：Fri, 26 Jun 2026 13:30:00 GMT\nURL：https://example.com/hk-flow\n摘要：南向资金净卖出，科技股成交额放大。",
					ItemCount: 2,
				},
			},
			Observations: []CapabilityObservation{
				{Capability: "web.search", Decision: string(PolicyDecisionAllow), Status: "succeeded", Summary: "loaded 1 web search results"},
			},
		}},
		LLMClient: &runnerFakeChatClient{
			err: domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "test", true, nil),
		},
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"web.search"},
		MessageType:     "text",
		MessageText:     "搜索最新港股消息并分析",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, result = %#v", result)
	}
	if result.Turn.Status != domain.AgentTurnStatusFailed {
		t.Fatalf("turn status = %q", result.Turn.Status)
	}
	if strings.TrimSpace(result.Reply) != "" {
		t.Fatalf("reply = %q, want empty on model failure", result.Reply)
	}
	if len(result.Context.Observations) != 1 || result.Context.Observations[0].Capability != "web.search" {
		t.Fatalf("observations = %#v", result.Context.Observations)
	}
	if len(store.transcripts) != 0 {
		t.Fatalf("transcripts = %#v", store.transcripts)
	}
}

func TestFallbackEvidenceKeepsParsedSearchResultsWithoutKeywordFiltering(t *testing.T) {
	items := fallbackEvidenceItems("搜索最新港股消息并分析", []ContextBlock{
		{
			Name:          "联网搜索结果",
			CapabilityKey: "web.search",
			Content: "工具：web.search\n查询：港股\n结果：\n" +
				"1. 港美股交易怎么操作？新手完整交易流程讲解（湾区阿瑟）\nURL：https://bayase.com/hk-us-stock-guide\n摘要：大陆居民如何开户，讲解港股和美股交易规则、账户开通和零基础教程。\n" +
				"2. 港股收评：恒生指数下跌，科技股走弱（财联社）\n发布时间：Fri, 26 Jun 2026 13:00:00 GMT\nURL：https://example.com/hk-news\n摘要：南向资金净卖出，腾讯、美团等科技股承压。",
			ItemCount: 2,
		},
	})
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if !strings.Contains(items[0].Title, "港美股交易怎么操作") || !strings.Contains(items[1].Title, "港股收评") {
		t.Fatalf("unexpected items = %#v", items)
	}
}

func TestTurnRunnerReturnsModelErrorWithoutEvidenceFallback(t *testing.T) {
	now := time.Date(2026, 6, 26, 21, 30, 0, 0, time.UTC)
	runner := NewTurnRunner(TurnRunnerOptions{
		Store: &runnerFakeTurnStore{},
		ContextBuilder: runnerFakeContextBuilder{snapshot: ContextSnapshot{
			Blocks: []ContextBlock{
				{
					Name:          "联网搜索结果",
					CapabilityKey: "web.search",
					Content:       "工具：web.search\n查询：港股\n结果：\n1. 港美股交易怎么操作？新手完整交易流程讲解（湾区阿瑟）\nURL：https://bayase.com/hk-us-stock-guide\n摘要：大陆居民如何开户，讲解港股和美股交易规则、账户开通和零基础教程。",
					ItemCount:     1,
				},
			},
		}},
		LLMClient: &runnerFakeChatClient{
			err: domain.NewAppError(domain.ErrorKindUnavailable, "llm_empty_response", "llm response is empty", "test", true, nil),
		},
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"web.search"},
		MessageType:     "text",
		MessageText:     "搜索最新港股消息并分析",
	})
	if err == nil {
		t.Fatalf("Run() error = nil, result = %#v", result)
	}
}

func TestTurnRunnerDoesNotReplaceModelReplyWithKeywordQualityGate(t *testing.T) {
	now := time.Date(2026, 6, 26, 21, 45, 0, 0, time.UTC)
	runner := NewTurnRunner(TurnRunnerOptions{
		Store: &runnerFakeTurnStore{},
		ContextBuilder: runnerFakeContextBuilder{snapshot: ContextSnapshot{
			Blocks: []ContextBlock{
				{
					Name:          "联网搜索结果",
					CapabilityKey: "web.search",
					Content: "工具：web.search\n查询：港股\n结果：\n" +
						"1. 港股收评：恒生指数上涨（财联社）\n发布时间：Fri, 26 Jun 2026 13:00:00 GMT\nURL：https://example.com/hk-up\n摘要：恒生科技指数反弹，南向资金净买入。\n" +
						"2. 港股科技股走强（AASTOCKS）\n发布时间：Fri, 26 Jun 2026 13:30:00 GMT\nURL：https://example.com/hk-tech\n摘要：腾讯、美团等权重股上涨。",
					ItemCount: 2,
				},
			},
		}},
		LLMClient: &runnerFakeChatClient{
			responses: []llm.ChatResponse{{Provider: "openai_compatible", Model: "custom-model", Content: "结论：当前港股消息面偏弱，下跌压力较大。"}},
		},
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:          1,
		Session:         domain.AgentSession{ID: 10, UserID: 1},
		Turn:            domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage:  domain.AgentInboundMessage{ID: 30, UserID: 1},
		AllowedToolKeys: []string{"web.search"},
		MessageType:     "text",
		MessageText:     "搜索最新港股消息并分析",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Reply, "偏弱") {
		t.Fatalf("reply = %q", result.Reply)
	}
}

func TestTurnRunnerRepairsMarkdownFinalReply(t *testing.T) {
	now := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	chat := &runnerFakeChatClient{
		responses: []llm.ChatResponse{
			{Provider: "openai_compatible", Model: "custom-model", Content: "## 结论\n- 当前消息面偏谨慎\n- 继续观察资金流"},
			{Provider: "openai_compatible", Model: "custom-model", Content: "结论：当前消息面偏谨慎。继续观察资金流。"},
		},
	}
	audit := &runnerFakeAuditLogger{}
	runner := NewTurnRunner(TurnRunnerOptions{
		Store:        &runnerFakeTurnStore{},
		AuditLogger:  audit,
		LLMClient:    chat,
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	result, err := runner.Run(context.Background(), TurnRunInput{
		UserID:         1,
		Session:        domain.AgentSession{ID: 10, UserID: 1},
		Turn:           domain.AgentTurn{ID: 20, SessionID: 10, UserID: 1, Status: domain.AgentTurnStatusRunning},
		InboundMessage: domain.AgentInboundMessage{ID: 30, UserID: 1},
		MessageType:    "text",
		MessageText:    "分析市场",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Reply != "结论：当前消息面偏谨慎。继续观察资金流。" {
		t.Fatalf("reply = %q", result.Reply)
	}
	if PlainTextReplyViolation(result.Reply) != "" {
		t.Fatalf("reply still violates plain text format: %q", result.Reply)
	}
	if chat.calls != 2 {
		t.Fatalf("chat calls = %d, want 2", chat.calls)
	}
	if !runnerAuditContains(audit.events, "agent.plain_text_reply_repair_retry") {
		t.Fatalf("audit events = %#v", audit.events)
	}
	lastRequest := chat.requests[len(chat.requests)-1]
	lastMessage := lastRequest.Messages[len(lastRequest.Messages)-1]
	if lastMessage.Role != "user" || !strings.Contains(lastMessage.Content, "violation") {
		t.Fatalf("repair prompt message = %#v", lastMessage)
	}
}

type runnerFakeChatClient struct {
	calls       int
	responseIdx int
	requests    []llm.ChatRequest
	responses   []llm.ChatResponse
	errs        []error
	err         error
}

func (c *runnerFakeChatClient) Chat(_ context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	c.calls++
	c.requests = append(c.requests, request)
	if c.err != nil {
		return llm.ChatResponse{}, c.err
	}
	callIndex := c.calls - 1
	if callIndex < len(c.errs) && c.errs[callIndex] != nil {
		return llm.ChatResponse{}, c.errs[callIndex]
	}
	if len(c.responses) == 0 {
		return llm.ChatResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "test_llm_response_missing", "test llm response is missing", "test", false, nil)
	}
	index := c.responseIdx
	if index >= len(c.responses) {
		index = len(c.responses) - 1
	}
	c.responseIdx++
	return c.responses[index], nil
}

func runnerAuditContains(events []AuditEvent, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

type runnerFakeContextBuilder struct {
	input    ContextBuildInput
	snapshot ContextSnapshot
	err      error
}

func (b runnerFakeContextBuilder) Build(_ context.Context, input ContextBuildInput) (ContextSnapshot, error) {
	if b.err != nil {
		return b.snapshot, b.err
	}
	return b.snapshot, nil
}

type runnerFakeToolExecutor struct {
	input   MCPCallToolInput
	content string
}

func (e *runnerFakeToolExecutor) CallTool(_ context.Context, input MCPCallToolInput) (MCPCallToolResult, error) {
	e.input = input
	return NewMCPTextCallToolResult(e.content, false, CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(PolicyDecisionAllow),
		Status:     "succeeded",
		Summary:    "loaded 1 history messages",
	}), nil
}

type runnerFakeTurnStore struct {
	turn        domain.AgentTurn
	transcripts []domain.AgentTranscriptEntry
}

func (s *runnerFakeTurnStore) UpdateTurn(_ context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	s.turn = turn
	return turn, nil
}

func (s *runnerFakeTurnStore) AppendTranscriptEntry(_ context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error) {
	entry.ID = int64(len(s.transcripts) + 1)
	s.transcripts = append(s.transcripts, entry)
	return entry, nil
}

func (s *runnerFakeTurnStore) UpdateInboundMessageStatus(_ context.Context, _ int64, _ int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error) {
	return domain.AgentInboundMessage{Status: status, UpdatedAt: now}, nil
}

type runnerFakeAuditLogger struct {
	events []AuditEvent
}

func (l *runnerFakeAuditLogger) Record(_ context.Context, event AuditEvent) error {
	l.events = append(l.events, event)
	return nil
}
