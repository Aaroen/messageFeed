package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

type AgentGuestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AgentGuestChatInput struct {
	ConfigID       int64               `json:"config_id"`
	Model          string              `json:"model"`
	Messages       []AgentGuestMessage `json:"messages"`
	ConversationID int64               `json:"conversation_id"`
}

type AgentGuestChatResult struct {
	llm.ChatResponse
	ConversationID int64
	TurnID         int64
}

type AgentGuestHistory struct {
	Turns              []domain.AgentGuestTurn `json:"turns"`
	RetentionDays      int                     `json:"retention_days"`
	ClientDailyLimit   int                     `json:"client_daily_limit"`
	TotalDailyLimit    int                     `json:"total_daily_limit"`
	DailyResetTimeZone string                  `json:"daily_reset_time_zone"`
}

func (s *AgentLLMConfigService) guestClientHash(clientAddress string) (string, error) {
	address, err := netip.ParseAddr(clientAddress)
	if err != nil {
		return "", fmt.Errorf("%w: 游客来源 IP 无效", domain.ErrInvalidInput)
	}
	hash := hmac.New(sha256.New, s.codec.key[:])
	if _, err := hash.Write([]byte(address.Unmap().String())); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *AgentLLMConfigService) GuestHistory(ctx context.Context, clientAddress string) (AgentGuestHistory, error) {
	clientHash, err := s.guestClientHash(clientAddress)
	if err != nil {
		return AgentGuestHistory{}, err
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return AgentGuestHistory{}, fmt.Errorf("游客历史存储不可用")
	}
	turns, err := store.ListGuestAgentHistory(ctx, clientHash)
	if err != nil {
		return AgentGuestHistory{}, err
	}
	return AgentGuestHistory{Turns: turns, RetentionDays: domain.AgentGuestRetentionDays, ClientDailyLimit: domain.AgentGuestClientDailyLimit, TotalDailyLimit: domain.AgentGuestTotalDailyLimit, DailyResetTimeZone: time.UTC.String()}, nil
}

func (s *AgentLLMConfigService) GuestChat(ctx context.Context, clientAddress string, input AgentGuestChatInput, emit func(llm.StreamEvent) error) (result AgentGuestChatResult, resultErr error) {
	if input.ConfigID < 1 || input.ConversationID < 0 || strings.TrimSpace(input.Model) == "" || len(input.Messages) < 1 || len(input.Messages) > 40 || input.Messages[len(input.Messages)-1].Role != "user" {
		return AgentGuestChatResult{}, fmt.Errorf("%w: 游客对话参数无效", domain.ErrInvalidInput)
	}
	messages := []llm.ChatMessage{{Role: "system", Content: "你是网页助理。遵守用户指定的语言和输出格式；用户没有指定语言时使用简体中文。"}}
	total := 0
	for index, message := range input.Messages {
		expected := "user"
		if index%2 == 1 {
			expected = "assistant"
		}
		if message.Role != expected || strings.TrimSpace(message.Content) == "" {
			return AgentGuestChatResult{}, fmt.Errorf("%w: 对话消息顺序或内容无效", domain.ErrInvalidInput)
		}
		total += len(message.Content)
		if total > 64000 {
			return AgentGuestChatResult{}, fmt.Errorf("%w: 游客对话内容超过长度限制", domain.ErrInvalidInput)
		}
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return AgentGuestChatResult{}, fmt.Errorf("模型目录存储不可用")
	}
	configuration, err := store.GetGuestAgentModelConfig(ctx, input.ConfigID, input.Model)
	if err != nil {
		return AgentGuestChatResult{}, err
	}
	client, err := s.clientForConfig(configuration)
	if err != nil {
		return AgentGuestChatResult{}, err
	}
	clientHash, err := s.guestClientHash(clientAddress)
	if err != nil {
		return AgentGuestChatResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	turn, err := store.ReserveGuestAgentRequest(ctx, clientHash, domain.AgentGuestTurn{ConfigID: input.ConfigID, Model: input.Model, InputText: input.Messages[len(input.Messages)-1].Content, ConversationID: input.ConversationID})
	if err != nil {
		return AgentGuestChatResult{}, err
	}
	result.ConversationID, result.TurnID = turn.ConversationID, turn.ID
	defer func() {
		turn.Status = "succeeded"
		turn.OutputText, turn.ReasoningContent = result.Content, result.ReasoningContent
		if resultErr != nil {
			turn.Status = "failed"
		}
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer finishCancel()
		resultErr = errors.Join(resultErr, store.CompleteGuestAgentRequest(finishCtx, clientHash, turn))
	}()
	if input.ConversationID > 0 {
		history, err := store.ListGuestAgentHistory(ctx, clientHash)
		if err != nil {
			return result, err
		}
		previous := []domain.AgentGuestTurn{}
		for _, turn := range history {
			if turn.ConversationID == input.ConversationID && turn.Status == "succeeded" {
				previous = append(previous, turn)
			}
		}
		if len(previous) > 19 {
			return result, fmt.Errorf("%w: 当前游客对话已达到消息数量限制，请新建对话", domain.ErrInvalidInput)
		}
		total = len(input.Messages[len(input.Messages)-1].Content)
		for _, turn := range previous {
			total += len(turn.InputText) + len(turn.OutputText)
			if total > 64000 {
				return result, fmt.Errorf("%w: 当前游客对话已达到内容长度限制，请新建对话", domain.ErrInvalidInput)
			}
			messages = append(messages, llm.ChatMessage{Role: "user", Content: turn.InputText}, llm.ChatMessage{Role: "assistant", Content: turn.OutputText})
		}
	}
	messages = append(messages, llm.ChatMessage{Role: "user", Content: input.Messages[len(input.Messages)-1].Content})
	response, err := client.Chat(ctx, llm.ChatRequest{Messages: messages, MaxTokens: 2048, OnEvent: emit})
	if err == nil && strings.TrimSpace(response.Content) == "" {
		err = fmt.Errorf("供应商未返回回答正文")
	}
	result.ChatResponse = response
	return result, err
}
