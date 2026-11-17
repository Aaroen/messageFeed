package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
)

type AgentGuestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AgentGuestChatInput struct {
	ConfigID int64               `json:"config_id"`
	Model    string              `json:"model"`
	Messages []AgentGuestMessage `json:"messages"`
}

func (s *AgentLLMConfigService) GuestChat(ctx context.Context, clientAddress string, input AgentGuestChatInput, emit func(llm.StreamEvent) error) (llm.ChatResponse, error) {
	if input.ConfigID < 1 || strings.TrimSpace(input.Model) == "" || len(input.Messages) < 1 || len(input.Messages) > 40 || input.Messages[len(input.Messages)-1].Role != "user" {
		return llm.ChatResponse{}, fmt.Errorf("%w: 游客对话参数无效", domain.ErrInvalidInput)
	}
	messages := []llm.ChatMessage{{Role: "system", Content: "你是网页助理。遵守用户指定的语言和输出格式；用户没有指定语言时使用简体中文。"}}
	total := 0
	for index, message := range input.Messages {
		expected := "user"
		if index%2 == 1 {
			expected = "assistant"
		}
		if message.Role != expected || strings.TrimSpace(message.Content) == "" {
			return llm.ChatResponse{}, fmt.Errorf("%w: 对话消息顺序或内容无效", domain.ErrInvalidInput)
		}
		total += len(message.Content)
		if total > 64000 {
			return llm.ChatResponse{}, fmt.Errorf("%w: 游客对话内容超过长度限制", domain.ErrInvalidInput)
		}
		messages = append(messages, llm.ChatMessage{Role: message.Role, Content: message.Content})
	}
	store, ok := s.store.(agentLLMModelStore)
	if !ok {
		return llm.ChatResponse{}, fmt.Errorf("模型目录存储不可用")
	}
	configuration, err := store.GetGuestAgentModelConfig(ctx, input.ConfigID, input.Model)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	client, err := s.clientForConfig(configuration)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	hash := hmac.New(sha256.New, s.codec.key[:])
	_, err = hash.Write([]byte(clientAddress))
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if _, err := store.ReserveGuestAgentRequest(ctx, hex.EncodeToString(hash.Sum(nil)), input.ConfigID, input.Model); err != nil {
		return llm.ChatResponse{}, err
	}
	response, err := client.Chat(ctx, llm.ChatRequest{Messages: messages, MaxTokens: 2048, OnEvent: emit})
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if strings.TrimSpace(response.Content) == "" {
		return llm.ChatResponse{}, fmt.Errorf("供应商未返回回答正文")
	}
	return response, nil
}
