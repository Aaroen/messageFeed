package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strings"
)

func buildAgentWeChatComponentSet(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask) AgentWeChatComponentSetResponse {
	actions := []AgentWeChatActionResponse{}
	var planID int64
	for _, plan := range plans {
		if plan.ID > 0 {
			planID = plan.ID
			break
		}
	}
	if planID > 0 {
		progressURL := fmt.Sprintf("/agent/plans/%d", planID)
		actions = append(actions,
			AgentWeChatActionResponse{Key: "view_progress", Label: "查看进度", URL: progressURL, Fallback: "打开进度地址查看实时细节"},
			AgentWeChatActionResponse{Key: "retry_plan", Label: "重试计划", URL: progressURL, Fallback: "在进度页选择重试计划"},
			AgentWeChatActionResponse{Key: "recover_plan", Label: "恢复计划", URL: progressURL, Fallback: "在进度页选择恢复执行"},
			AgentWeChatActionResponse{Key: "approval", Label: "处理审批", URL: progressURL, Fallback: "在进度页进入审批处理"},
		)
	}
	for _, task := range tasks {
		if task.ID < 1 {
			continue
		}
		url := fmt.Sprintf("/agent?scheduled_task_id=%d", task.ID)
		if task.PlanID > 0 {
			url = fmt.Sprintf("/agent/plans/%d?scheduled_task_id=%d", task.PlanID, task.ID)
		}
		actions = append(actions, AgentWeChatActionResponse{Key: "cancel_scheduled_task", Label: "取消定时任务", URL: url, Fallback: "在任务工作台取消定时任务"})
		break
	}
	mode := "text_fallback"
	if len(actions) > 0 {
		mode = "component_fallback"
	}
	return AgentWeChatComponentSetResponse{
		Mode:    mode,
		Summary: fmt.Sprintf("%d wechat action fallbacks available", len(actions)),
		Actions: actions,
	}
}

func buildAgentWeChatCallbackReadiness(audits []domain.AgentAuditLog, components AgentWeChatComponentSetResponse) AgentWeChatCallbackReadinessResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "signature", Status: "ready", Summary: "wechat callback signature verification is configured in callback handler"},
		{Key: "decrypt", Status: "ready", Summary: "wechat encrypted payload decrypt path is available in channel adapter"},
		{Key: "idempotency", Status: readyIf(auditEventExists(audits, "agent.inbound") || auditEventExists(audits, "agent.plan")), Summary: "provider message id is used for inbound idempotency"},
		{Key: "binding", Status: "ready", Summary: "external account binding is required before task execution"},
		{Key: "reply", Status: readyIf(auditEventContains(audits, "reply")), Summary: "wechat reply audit records are available"},
		{Key: "async_notification", Status: readyIf(auditEventContains(audits, "notification") || auditEventContains(audits, "progress")), Summary: "process notifications can be sent asynchronously"},
		{Key: "final_report", Status: readyIf(auditEventContains(audits, "report")), Summary: "final report audit records are available"},
		{Key: "action_fallback", Status: readyIf(len(components.Actions) > 0), Summary: components.Summary},
	}
	return AgentWeChatCallbackReadinessResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("%d wechat callback readiness checks", len(checks)),
		Checks:  checks,
	}
}

func buildAgentWeChatNativeActions(components AgentWeChatComponentSetResponse) AgentWeChatNativeActionSetResponse {
	actions := make([]AgentWeChatNativeActionResponse, 0, len(components.Actions)+1)
	for _, action := range components.Actions {
		actions = append(actions, AgentWeChatNativeActionResponse{
			Key:      action.Key,
			Label:    action.Label,
			Style:    agentWeChatActionStyle(action.Key),
			URL:      action.URL,
			Fallback: action.Fallback,
		})
	}
	hasReport := false
	for _, action := range actions {
		if action.Key == "view_final_report" {
			hasReport = true
			break
		}
	}
	if !hasReport {
		reportURL := "/agent"
		if len(components.Actions) > 0 && strings.TrimSpace(components.Actions[0].URL) != "" {
			reportURL = components.Actions[0].URL
		}
		actions = append(actions, AgentWeChatNativeActionResponse{
			Key:      "view_final_report",
			Label:    "查看最终报告",
			Style:    "secondary",
			URL:      reportURL,
			Fallback: "打开进度地址查看最终报告",
		})
	}
	mode := "native_button_schema"
	if len(actions) == 0 {
		mode = "text_fallback"
	}
	return AgentWeChatNativeActionSetResponse{
		Mode:    mode,
		Summary: fmt.Sprintf("%d wechat native action definitions available", len(actions)),
		Actions: actions,
	}
}

func buildAgentWeChatNativePayload(native AgentWeChatNativeActionSetResponse) AgentWeChatNativePayloadResponse {
	buttons := make([]AgentWeChatNativeButtonResponse, 0, len(native.Actions))
	payloadButtons := make([]domain.AgentJSON, 0, len(native.Actions))
	fallbacks := []string{}
	for _, action := range native.Actions {
		button := AgentWeChatNativeButtonResponse{
			Key:      action.Key,
			Label:    action.Label,
			Style:    action.Style,
			URL:      action.URL,
			Fallback: action.Fallback,
		}
		buttons = append(buttons, button)
		payloadButtons = append(payloadButtons, domain.AgentJSON{
			"key":   button.Key,
			"text":  button.Label,
			"style": button.Style,
			"url":   button.URL,
		})
		fallbacks = append(fallbacks, button.Label+"："+button.Fallback)
	}
	messageType := "template_card"
	status := "ready"
	if len(buttons) == 0 {
		messageType = "text"
		status = "review"
	}
	fallbackText := strings.Join(fallbacks, "\n")
	if fallbackText == "" {
		fallbackText = "请打开 Web 任务工作台查看 Agent 进度。"
	}
	return AgentWeChatNativePayloadResponse{
		Status:       status,
		Summary:      fmt.Sprintf("%d native wechat buttons prepared", len(buttons)),
		MessageType:  messageType,
		FallbackText: fallbackText,
		Buttons:      buttons,
		Payload: domain.AgentJSON{
			"msgtype":       messageType,
			"title":         "Agent 任务操作",
			"description":   "查看进度、审批、重试、恢复、取消或查看最终报告",
			"buttons":       payloadButtons,
			"fallback_text": fallbackText,
		},
	}
}

func nativeButtonHasURL(buttons []AgentWeChatNativeButtonResponse) bool {
	for _, button := range buttons {
		if strings.TrimSpace(button.URL) != "" {
			return true
		}
	}
	return false
}

func nativeButtonExists(buttons []AgentWeChatNativeButtonResponse, key string) bool {
	for _, button := range buttons {
		if button.Key == key {
			return true
		}
	}
	return false
}

func agentWeChatActionStyle(key string) string {
	switch key {
	case "approval", "recover_plan", "retry_plan":
		return "primary"
	case "cancel_scheduled_task":
		return "danger"
	default:
		return "secondary"
	}
}
