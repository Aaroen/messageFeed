package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
	"strconv"
	"strings"
)

type agentWeChatTemplateCardSender interface {
	SendTemplateCard(ctx context.Context, message notifier.WeChatWorkTemplateCardMessage) (notifier.WeChatWorkSendResult, error)
}

type agentWeChatProgressDeliveryResult struct {
	SendResult     notifier.WeChatWorkSendResult
	SendCount      int
	DeliveryMode   string
	TemplateStatus string
	FallbackStatus string
	TemplateError  string
	FallbackError  string
}

func (s *AgentConversationService) sendWeChatWorkProgressDelivery(ctx context.Context, toUser string, plan domain.AgentPlan, stage string, title string, fallbackText string) agentWeChatProgressDeliveryResult {
	progressURL := s.agentPlanURL(plan.ID)
	result := agentWeChatProgressDeliveryResult{
		DeliveryMode:   "template_card",
		TemplateStatus: "not_attempted",
		FallbackStatus: "not_attempted",
	}
	templateSender, ok := s.sender.(agentWeChatTemplateCardSender)
	if ok {
		sendResult, err := templateSender.SendTemplateCard(ctx, notifier.WeChatWorkTemplateCardMessage{
			ToUser:       toUser,
			Title:        agentWeChatProgressCardTitle(title),
			Description:  agentWeChatProgressCardDescription(plan, stage),
			URL:          progressURL,
			FallbackText: fallbackText,
			Buttons: []notifier.WeChatWorkTemplateCardButton{
				{Key: "view_progress", Text: "查看进度", URL: progressURL},
			},
		})
		if err == nil {
			result.SendResult = sendResult
			result.SendCount = 1
			result.TemplateStatus = "succeeded"
			return result
		}
		result.SendResult = sendResult
		result.TemplateStatus = "failed"
		result.TemplateError = strings.TrimSpace(err.Error())
	} else {
		result.TemplateStatus = "unsupported"
	}

	result.DeliveryMode = "text_fallback"
	sendResult, sendCount, err := s.sendWeChatWorkReply(ctx, toUser, fallbackText)
	result.SendResult = sendResult
	result.SendCount = sendCount
	if err != nil {
		result.FallbackStatus = "failed"
		result.FallbackError = strings.TrimSpace(err.Error())
		return result
	}
	result.FallbackStatus = "succeeded"
	return result
}

func agentWeChatProgressCardTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Agent 实时工作进度"
	}
	return title
}

func agentWeChatProgressCardDescription(plan domain.AgentPlan, stage string) string {
	parts := []string{"计划进度可在 Web 浏览器查看"}
	if plan.ID > 0 {
		parts = append(parts, "计划 #"+strconv.FormatInt(plan.ID, 10))
	}
	if stage = strings.TrimSpace(stage); stage != "" {
		parts = append(parts, "阶段 "+stage)
	}
	if plan.Status != "" {
		parts = append(parts, "状态 "+string(plan.Status))
	}
	return strings.Join(parts, " / ")
}
