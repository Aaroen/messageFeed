package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
	"strconv"
	"strings"
)

type agentWeChatFinalReportDeliveryResult struct {
	SendResult     notifier.WeChatWorkSendResult
	SendCount      int
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
	result := agentWeChatFinalReportDeliveryResult{
		DeliveryMode:   "text",
		TemplateStatus: "not_attempted",
		TextStatus:     "not_attempted",
		ProgressURL:    progressURL,
	}
	if progressURL != "" {
		if templateSender, ok := s.sender.(agentWeChatTemplateCardSender); ok {
			sendResult, err := templateSender.SendTemplateCard(ctx, notifier.WeChatWorkTemplateCardMessage{
				ToUser:       toUser,
				Title:        agentWeChatFinalReportCardTitle(outcome),
				Description:  agentWeChatFinalReportCardDescription(plan, outcome),
				URL:          progressURL,
				FallbackText: reply,
				Buttons: []notifier.WeChatWorkTemplateCardButton{
					{Key: "view_final_report", Text: "查看结果", URL: progressURL},
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
		} else {
			result.TemplateStatus = "unsupported"
		}
	}

	textResult, textCount, err := s.sendWeChatWorkReply(ctx, toUser, reply)
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

func agentWeChatFinalReportCardTitle(outcome string) string {
	switch strings.TrimSpace(outcome) {
	case "failed", "failure":
		return "Agent 任务处理失败"
	case "awaiting_approval":
		return "Agent 任务等待确认"
	case "rejected":
		return "Agent 任务已拒绝"
	default:
		return "Agent 任务结果"
	}
}

func agentWeChatFinalReportCardDescription(plan domain.AgentPlan, outcome string) string {
	parts := []string{"最终结果已生成"}
	if plan.ID > 0 {
		parts = append(parts, "计划 #"+strconv.FormatInt(plan.ID, 10))
	}
	if plan.Status != "" {
		parts = append(parts, "状态 "+string(plan.Status))
	}
	if outcome = strings.TrimSpace(outcome); outcome != "" {
		parts = append(parts, "结果 "+outcome)
	}
	return strings.Join(parts, " / ")
}

func agentWeChatFinalReportMetadata(delivery agentWeChatFinalReportDeliveryResult) domain.AgentJSON {
	return domain.AgentJSON{
		"message_type":    delivery.DeliveryMode,
		"template_status": delivery.TemplateStatus,
		"text_status":     delivery.TextStatus,
		"template_error":  delivery.TemplateError,
		"text_error":      delivery.TextError,
		"progress_url":    delivery.ProgressURL,
	}
}
