package service

import (
	"context"
	"encoding/json"
	"errors"
	"messagefeed/internal/domain"
	"testing"
)

func TestAgentModelSelectionRoundTrip(test *testing.T) {
	selection := domain.AgentModelSelection{UserID: 7, ConfigID: 9, Model: "group/deepseek-flash"}
	encoded, err := json.Marshal(domain.AgentJSON{"model_selection": selection})
	if err != nil {
		test.Fatal(err)
	}
	inbound := domain.AgentInboundMessage{UserID: selection.UserID}
	if err := json.Unmarshal(encoded, &inbound.Payload); err != nil {
		test.Fatal(err)
	}
	ctx, err := agentSelectionFromInbound(context.Background(), inbound)
	if err != nil || ctx.Value(agentModelSelectionKey{}) != selection {
		test.Fatal("持久化模型选择未完整恢复")
	}
	for _, payload := range []domain.AgentJSON{{"model_selection": domain.AgentModelSelection{UserID: 8, ConfigID: 9, Model: selection.Model}}, {"model_selection": map[string]any{"config_id": 9, "model": ""}}, {"model_selection": nil}, {"model_selection": "invalid"}} {
		if _, err := agentSelectionFromInbound(context.Background(), domain.AgentInboundMessage{UserID: 7, Payload: payload}); err == nil {
			test.Fatal("无效或越权模型选择没有被拒绝")
		}
	}
	ctx, err = agentSelectionFromInbound(context.Background(), domain.AgentInboundMessage{UserID: 7})
	if err != nil || ctx.Value(agentModelSelectionKey{}) != nil {
		test.Fatal("既有回合解析失败")
	}
	if _, err := NewAgentLLMRuntime(nil).clientForUser(withAgentModelSelection(context.Background(), selection), 8); !errors.Is(err, domain.ErrInvalidInput) {
		test.Fatal("模型运行时没有拒绝跨用户选择")
	}
}

func TestGuestChatRejectsUntrustedRoles(test *testing.T) {
	service := NewAgentLLMConfigService(nil)
	for _, messages := range [][]AgentGuestMessage{nil, {{Role: "system", Content: "私有指令"}}, {{Role: "user", Content: ""}}, {{Role: "user", Content: "问题"}, {Role: "user", Content: "另一个问题"}}, {{Role: "tool", Content: "私有工具结果"}, {Role: "assistant", Content: "内容"}, {Role: "user", Content: "问题"}}} {
		if _, err := service.GuestChat(context.Background(), "127.0.0.1", AgentGuestChatInput{ConfigID: 1, Model: "group/deepseek-flash", Messages: messages}, nil); !errors.Is(err, domain.ErrInvalidInput) {
			test.Fatal("不可信游客消息没有被拒绝")
		}
	}
}
