package service

import (
	"fmt"
	"strings"
)

func buildAgentWeChatWebProgressLink(tasks []AgentTaskSummaryResponse, send AgentWeChatTemplateSendResponse, automation AgentRealInteractionAutomationResponse) AgentWeChatWebProgressLinkResponse {
	progressURL := ""
	urlSource := "task_summary"
	for _, task := range tasks {
		if strings.TrimSpace(task.ProgressURL) != "" {
			progressURL = task.ProgressURL
			if task.Kind != "" {
				urlSource = task.Kind
			}
			break
		}
	}
	deliveryChannel := "wechat_work"
	templateStatus := send.Status
	if strings.TrimSpace(templateStatus) == "" {
		templateStatus = "template_pending"
	}
	fallbackStatus := readyIf(strings.TrimSpace(send.FallbackText) != "")
	browserTarget := "web_browser"
	auditRef := "agent.wechat_web_progress_link_snapshot"
	nextAction := "通过企业微信发送 Web 浏览器进度地址"
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("progress_url", progressURL),
		agentGovernanceTextCheck("url_source", urlSource),
		agentGovernanceTextCheck("delivery_channel", deliveryChannel),
		agentGovernanceTextCheck("template_status", templateStatus),
		{Key: "fallback_status", Status: fallbackStatus, Summary: send.FallbackText},
		agentGovernanceTextCheck("browser_target", browserTarget),
		agentGovernanceTextCheck("audit_ref", auditRef),
		{Key: "real_interaction_automation", Status: automation.Status, Summary: automation.Summary},
	}
	if checksStatus(checks) != "ready" {
		nextAction = "补齐 Web 进度地址、企业微信模板或 fallback 后再投递"
	}
	return AgentWeChatWebProgressLinkResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat web progress link targets %s via %s", progressURL, deliveryChannel),
		ProgressURL:     progressURL,
		URLSource:       urlSource,
		DeliveryChannel: deliveryChannel,
		TemplateStatus:  templateStatus,
		FallbackStatus:  fallbackStatus,
		BrowserTarget:   browserTarget,
		AuditRef:        auditRef,
		NextAction:      nextAction,
		Checks:          checks,
	}
}
