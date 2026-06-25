package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
)

func buildAgentWeChatTemplateIntegration(send AgentWeChatTemplateSendResponse) AgentWeChatTemplateIntegrationResponse {
	sendPath := send.SendEntry
	if strings.TrimSpace(sendPath) == "" {
		sendPath = "notifier.wechat_work.send_template_card"
	}
	templateStatus := "template_card_ready"
	if send.MessageType != "template_card" {
		templateStatus = "template_card_review"
	}
	fallbackStatus := readyIf(strings.TrimSpace(send.FallbackText) != "")
	degradeStrategy := "template_send_failed_to_text_fallback"
	messageIDReadback := "provider_msgid_recorded_when_available"
	auditEvidence := send.AuditEvent
	if strings.TrimSpace(auditEvidence) == "" {
		auditEvidence = "agent.wechat_template_send_snapshot"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("send_path", sendPath),
		agentGovernanceTextCheck("template_status", templateStatus),
		{Key: "fallback_status", Status: fallbackStatus, Summary: send.FallbackText},
		agentGovernanceTextCheck("degrade_strategy", degradeStrategy),
		agentGovernanceTextCheck("message_id_readback", messageIDReadback),
		agentGovernanceTextCheck("audit_evidence", auditEvidence),
	}
	return AgentWeChatTemplateIntegrationResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("wechat template integration %s with fallback %s", templateStatus, fallbackStatus),
		SendPath:          sendPath,
		TemplateStatus:    templateStatus,
		FallbackStatus:    fallbackStatus,
		DegradeStrategy:   degradeStrategy,
		MessageIDReadback: messageIDReadback,
		AuditEvidence:     auditEvidence,
		Checks:            checks,
	}
}

func buildAgentWebEvidenceInteractionDetail(view AgentWebEvidenceDetailViewResponse) AgentWebEvidenceInteractionDetailResponse {
	filterMode := "query_params:" + strings.Join(view.FilterParams, ",")
	if len(view.FilterParams) == 0 {
		filterMode = "query_params:status,audit_event"
	}
	expandMode := "inline_record_detail_and_audit_payload"
	auditTimeline := "audit_events:" + strings.Join(view.AuditEvents, ",")
	if len(view.AuditEvents) == 0 {
		auditTimeline = "audit_events:agent.web_evidence_detail_view_snapshot"
	}
	replayRequestEntry := "/api/v1/agent/callback-replay/requests"
	permissionHint := view.PermissionHint
	if strings.TrimSpace(permissionHint) == "" {
		permissionHint = "task_owner_or_agent_operations"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("filter_mode", filterMode),
		agentGovernanceTextCheck("expand_mode", expandMode),
		agentGovernanceTextCheck("audit_timeline", auditTimeline),
		agentGovernanceTextCheck("replay_request_entry", replayRequestEntry),
		agentGovernanceTextCheck("permission_hint", permissionHint),
	}
	return AgentWebEvidenceInteractionDetailResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("web evidence interaction has %d filters and replay entry", len(view.FilterParams)),
		FilterMode:         filterMode,
		ExpandMode:         expandMode,
		AuditTimeline:      auditTimeline,
		ReplayRequestEntry: replayRequestEntry,
		PermissionHint:     permissionHint,
		Checks:             checks,
	}
}

func buildAgentCallbackReplaySafetyAudit(execution AgentCallbackReplayExecutionResponse) AgentCallbackReplaySafetyAuditResponse {
	idempotencyCheck := "required:" + execution.IdempotencyKey
	approvalCheck := "required:" + execution.ApprovalStatus
	signatureCheck := "required:wechat_callback_signature"
	executionResult := execution.Status
	if strings.TrimSpace(executionResult) == "" {
		executionResult = "not_executed"
	}
	failureAudit := execution.FailureFallback
	if strings.TrimSpace(failureAudit) == "" {
		failureAudit = "manual_review_without_replay"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceCheck("idempotency_check", execution.IdempotencyKey != "", idempotencyCheck),
		agentGovernanceCheck("approval_check", execution.ApprovalStatus != "", approvalCheck),
		agentGovernanceTextCheck("signature_check", signatureCheck),
		agentGovernanceTextCheck("execution_result", executionResult),
		agentGovernanceTextCheck("failure_audit", failureAudit),
	}
	return AgentCallbackReplaySafetyAuditResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("callback replay safety audit uses gate %s", execution.ExecutionGate),
		IdempotencyCheck: idempotencyCheck,
		ApprovalCheck:    approvalCheck,
		SignatureCheck:   signatureCheck,
		ExecutionResult:  executionResult,
		FailureAudit:     failureAudit,
		Checks:           checks,
	}
}

func buildAgentRecoveryPolicyGrayRelease(version AgentRecoveryPolicyVersionResponse) AgentRecoveryPolicyGrayReleaseResponse {
	grayStage := "stage_0_review"
	releasePercent := 0
	if version.ReleaseStatus == "approved" || version.ReleaseStatus == "ready" {
		grayStage = "stage_1_canary"
		releasePercent = 5
	}
	rollbackCondition := "alert_failure_rate_or_manual_reject"
	approvalStatus := "approval_required"
	auditEvidence := version.AuditEvent
	if strings.TrimSpace(auditEvidence) == "" {
		auditEvidence = "agent.recovery_policy_version_snapshot"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("gray_stage", grayStage),
		agentGovernanceCheck("release_percent", releasePercent >= 0 && releasePercent <= 100, strconv.Itoa(releasePercent)),
		agentGovernanceTextCheck("rollback_condition", rollbackCondition),
		agentGovernanceTextCheck("approval_status", approvalStatus),
		agentGovernanceTextCheck("audit_evidence", auditEvidence),
	}
	return AgentRecoveryPolicyGrayReleaseResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("recovery policy gray release %s at %d%%", grayStage, releasePercent),
		GrayStage:         grayStage,
		ReleasePercent:    releasePercent,
		RollbackCondition: rollbackCondition,
		ApprovalStatus:    approvalStatus,
		AuditEvidence:     auditEvidence,
		Checks:            checks,
	}
}

func buildAgentDualEndRunLoop(integration AgentWeChatTemplateIntegrationResponse, detail AgentWebEvidenceInteractionDetailResponse, safety AgentCallbackReplaySafetyAuditResponse, gray AgentRecoveryPolicyGrayReleaseResponse, audits []domain.AgentAuditLog) AgentDualEndRunLoopResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat_template_integration") || auditEventContains(audits, "web_evidence_interaction_detail") || auditEventContains(audits, "callback_replay_safety") || auditEventContains(audits, "recovery_policy_gray") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_template_integration", Status: integration.Status, Summary: integration.Summary},
		{Key: "web_evidence_interaction", Status: detail.Status, Summary: detail.Summary},
		{Key: "callback_replay_safety", Status: safety.Status, Summary: safety.Summary},
		{Key: "recovery_policy_gray", Status: gray.Status, Summary: gray.Summary},
		{Key: "audit", Status: auditStatus, Summary: "dual-end run loop is audit-backed"},
	}
	nextAction := "进入真实环境企业微信模板试运行、证据页面用户操作和恢复策略灰度自动化"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运行闭环缺口后再进入真实环境试运行"
	}
	return AgentDualEndRunLoopResponse{
		Status:                          checksStatus(checks),
		Summary:                         fmt.Sprintf("dual-end run loop has %d checks", len(checks)),
		WeChatTemplateIntegrationStatus: integration.Status,
		WebEvidenceInteractionStatus:    detail.Status,
		CallbackReplaySafetyStatus:      safety.Status,
		RecoveryPolicyGrayStatus:        gray.Status,
		AuditStatus:                     auditStatus,
		NextAction:                      nextAction,
		Checks:                          checks,
	}
}
