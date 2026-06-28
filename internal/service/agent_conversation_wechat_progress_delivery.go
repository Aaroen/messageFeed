package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
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
	sendCtx, cancelSend := s.outboundNotificationContext(ctx)
	defer cancelSend()
	templateData := agentWeChatFeedbackRequest{
		Stage:       stage,
		Plan:        plan,
		ProgressURL: progressURL,
	}.templateData()
	cardTitle := renderAgentWeChatFeedbackTemplate("progress_card_title", templateData)
	cardDescription := renderAgentWeChatFeedbackTemplate("progress_card_description", templateData)
	buttonText := renderAgentWeChatFeedbackTemplate("progress_card_button_text", templateData)
	result := agentWeChatProgressDeliveryResult{
		DeliveryMode:   "template_card",
		TemplateStatus: "not_attempted",
		FallbackStatus: "not_attempted",
	}
	templateSender, ok := s.sender.(agentWeChatTemplateCardSender)
	if ok && strings.TrimSpace(cardTitle) != "" && strings.TrimSpace(cardDescription) != "" && strings.TrimSpace(buttonText) != "" {
		sendResult, err := templateSender.SendTemplateCard(sendCtx, notifier.WeChatWorkTemplateCardMessage{
			ToUser:       toUser,
			Title:        cardTitle,
			Description:  cardDescription,
			URL:          progressURL,
			FallbackText: fallbackText,
			Buttons: []notifier.WeChatWorkTemplateCardButton{
				{Key: "view_progress", Text: buttonText, URL: progressURL},
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
	sendResult, sendCount, err := s.sendWeChatWorkReply(sendCtx, toUser, fallbackText)
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
