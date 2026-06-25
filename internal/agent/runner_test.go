package agent

import (
	"context"
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

func TestTurnRunnerSystemPromptGuidesScheduledMessageConfirmation(t *testing.T) {
	now := time.Date(2026, 6, 24, 13, 55, 0, 0, time.UTC)
	runner := NewTurnRunner(TurnRunnerOptions{
		ToolExecutor: &runnerFakeToolExecutor{},
		Now:          func() time.Time { return now },
		SystemPrompt: "系统提示",
	})

	prompt := runner.buildSystemPrompt(ContextSnapshot{})
	for _, required := range []string{
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
		"再次调用 agent.schedule_task",
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

type runnerFakeChatClient struct {
	calls     int
	requests  []llm.ChatRequest
	responses []llm.ChatResponse
}

func (c *runnerFakeChatClient) Chat(_ context.Context, request llm.ChatRequest) (llm.ChatResponse, error) {
	c.calls++
	c.requests = append(c.requests, request)
	index := c.calls - 1
	if index >= len(c.responses) {
		index = len(c.responses) - 1
	}
	return c.responses[index], nil
}

type runnerFakeToolExecutor struct {
	input   ToolExecuteInput
	content string
}

func (e *runnerFakeToolExecutor) ExecuteTool(_ context.Context, input ToolExecuteInput) (ToolExecuteResult, error) {
	e.input = input
	return ToolExecuteResult{
		Content: e.content,
		Observation: CapabilityObservation{
			Capability: input.Capability.Key,
			Decision:   string(PolicyDecisionAllow),
			Status:     "succeeded",
			Summary:    "loaded 1 history messages",
		},
	}, nil
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
