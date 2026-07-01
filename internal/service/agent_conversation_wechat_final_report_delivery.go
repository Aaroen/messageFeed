package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
	"strings"
)

type agentWeChatFinalReportDeliveryResult struct {
	SendResult     notifier.WeChatWorkSendResult
	SendCount      int
	ReplyBytes     int
	TextChunkCount int
	TextChunkBytes []int
	DeliveryMode   string
	TemplateStatus string
	TextStatus     string
	TemplateError  string
	TextError      string
	ProgressURL    string
}

func (s *AgentConversationService) sendWeChatWorkFinalReportDelivery(ctx context.Context, toUser string, plan domain.AgentPlan, reply string, outcome string) (agentWeChatFinalReportDeliveryResult, error) {
	progressURL := ""
	if plan.ID > 0 {
		progressURL = s.agentPlanURL(plan.ID)
	}
	sendCtx, cancelSend := s.outboundNotificationContext(ctx)
	defer cancelSend()
	result := agentWeChatFinalReportDeliveryResult{
		DeliveryMode:   "text",
		TemplateStatus: "not_attempted",
		TextStatus:     "not_attempted",
		ProgressURL:    progressURL,
		ReplyBytes:     len([]byte(reply)),
	}
	textChunks := splitUTF8Bytes(reply, notifier.WeChatWorkTextByteLimit)
	result.TextChunkCount = len(textChunks)
	result.TextChunkBytes = utf8ByteLengths(textChunks)
	if progressURL != "" {
		templateData := agentWeChatFeedbackRequest{
			Stage:       "final",
			Plan:        plan,
			ProgressURL: progressURL,
		}.templateData()
		cardTitle := renderAgentWeChatFeedbackTemplate("final_card_title", templateData)
		cardDescription := renderAgentWeChatFeedbackTemplate("final_card_description", templateData)
		buttonText := renderAgentWeChatFeedbackTemplate("final_card_button_text", templateData)
		if templateSender, ok := s.sender.(agentWeChatTemplateCardSender); ok {
			if strings.TrimSpace(cardTitle) == "" || strings.TrimSpace(cardDescription) == "" || strings.TrimSpace(buttonText) == "" {
				result.TemplateStatus = "unsupported"
			} else {
				sendResult, err := templateSender.SendTemplateCard(sendCtx, notifier.WeChatWorkTemplateCardMessage{
					ToUser:       toUser,
					Title:        cardTitle,
					Description:  cardDescription,
					URL:          progressURL,
					FallbackText: reply,
					Buttons: []notifier.WeChatWorkTemplateCardButton{
						{Key: "view_final_report", Text: buttonText, URL: progressURL},
					},
				})
				if err != nil {
					result.TemplateStatus = "failed"
					result.TemplateError = strings.TrimSpace(err.Error())
					result.SendResult = sendResult
				} else {
					result.TemplateStatus = "succeeded"
					result.DeliveryMode = "template_card_with_text"
					result.SendResult = sendResult
					result.SendCount = 1
				}
			}
		} else {
			result.TemplateStatus = "unsupported"
		}
	}

	textResult, textCount, err := s.sendWeChatWorkReply(sendCtx, toUser, reply)
	result.SendResult = textResult
	result.SendCount += textCount
	if err != nil {
		result.TextStatus = "failed"
		result.TextError = strings.TrimSpace(err.Error())
		return result, err
	}
	result.TextStatus = "succeeded"
	if result.DeliveryMode == "text" {
		result.DeliveryMode = "text_fallback"
	}
	return result, nil
}

func agentWeChatFinalReportMetadata(delivery agentWeChatFinalReportDeliveryResult) domain.AgentJSON {
	return domain.AgentJSON{
		"message_type":     delivery.DeliveryMode,
		"template_status":  delivery.TemplateStatus,
		"text_status":      delivery.TextStatus,
		"template_error":   delivery.TemplateError,
		"text_error":       delivery.TextError,
		"progress_url":     delivery.ProgressURL,
		"reply_bytes":      delivery.ReplyBytes,
		"text_chunks":      delivery.TextChunkCount,
		"text_chunk_bytes": delivery.TextChunkBytes,
	}
}
