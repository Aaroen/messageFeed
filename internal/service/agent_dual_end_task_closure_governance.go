package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
	"time"
)

func buildAgentWeChatTemplatePilot(integration AgentWeChatTemplateIntegrationResponse, now time.Time) AgentWeChatTemplatePilotResponse {
	pilotBatch := "wechat-template-pilot-" + now.UTC().Format("20060102")
	targetScope := "current_authenticated_user"
	templateStatus := integration.TemplateStatus
	if strings.TrimSpace(templateStatus) == "" {
		templateStatus = "template_card_ready"
	}
	fallbackHit := "fallback_available"
	if integration.FallbackStatus != "ready" {
		fallbackHit = "fallback_review"
	}
	messageIDStatus := integration.MessageIDReadback
	if strings.TrimSpace(messageIDStatus) == "" {
		messageIDStatus = "provider_msgid_recorded_when_available"
	}
	auditEvidence := integration.AuditEvidence
	if strings.TrimSpace(auditEvidence) == "" {
		auditEvidence = "agent.wechat_template_integration_snapshot"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("pilot_batch", pilotBatch),
		agentGovernanceTextCheck("target_scope", targetScope),
		agentGovernanceTextCheck("template_status", templateStatus),
		agentGovernanceTextCheck("fallback_hit", fallbackHit),
		agentGovernanceTextCheck("message_id_status", messageIDStatus),
		agentGovernanceTextCheck("audit_evidence", auditEvidence),
	}
	return AgentWeChatTemplatePilotResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat template pilot %s targets %s", pilotBatch, targetScope),
		PilotBatch:      pilotBatch,
		TargetScope:     targetScope,
		TemplateStatus:  templateStatus,
		FallbackHit:     fallbackHit,
		MessageIDStatus: messageIDStatus,
		AuditEvidence:   auditEvidence,
		Checks:          checks,
	}
}

func buildAgentWebEvidenceUserAction(detail AgentWebEvidenceInteractionDetailResponse) AgentWebEvidenceUserActionResponse {
	filterAction := "filter:" + detail.FilterMode
	expandAction := "expand:" + detail.ExpandMode
	timelineAction := "timeline:" + detail.AuditTimeline
	replayRequest := detail.ReplayRequestEntry
	if strings.TrimSpace(replayRequest) == "" {
		replayRequest = "/api/v1/agent/callback-replay/requests"
	}
	permissionResult := "allowed:" + detail.PermissionHint
	if strings.TrimSpace(detail.PermissionHint) == "" {
		permissionResult = "allowed:task_owner_or_agent_operations"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("filter_action", filterAction),
		agentGovernanceTextCheck("expand_action", expandAction),
		agentGovernanceTextCheck("timeline_action", timelineAction),
		agentGovernanceTextCheck("replay_request", replayRequest),
		agentGovernanceTextCheck("permission_result", permissionResult),
	}
	return AgentWebEvidenceUserActionResponse{
		Status:           checksStatus(checks),
		Summary:          "web evidence user actions are represented",
		FilterAction:     filterAction,
		ExpandAction:     expandAction,
		TimelineAction:   timelineAction,
		ReplayRequest:    replayRequest,
		PermissionResult: permissionResult,
		Checks:           checks,
	}
}

func buildAgentCallbackReplayResultTrace(safety AgentCallbackReplaySafetyAuditResponse) AgentCallbackReplayResultTraceResponse {
	executionResult := safety.ExecutionResult
	if strings.TrimSpace(executionResult) == "" {
		executionResult = "not_executed"
	}
	idempotencyHit := "checked:" + safety.IdempotencyCheck
	approvalDecision := "checked:" + safety.ApprovalCheck
	signatureResult := "checked:" + safety.SignatureCheck
	failureReason := safety.FailureAudit
	if strings.TrimSpace(failureReason) == "" {
		failureReason = "none_or_manual_review"
	}
	auditRecord := "agent.callback_replay_safety_audit_snapshot"
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("execution_result", executionResult),
		agentGovernanceTextCheck("idempotency_hit", idempotencyHit),
		agentGovernanceTextCheck("approval_decision", approvalDecision),
		agentGovernanceTextCheck("signature_result", signatureResult),
		agentGovernanceTextCheck("failure_reason", failureReason),
		agentGovernanceTextCheck("audit_record", auditRecord),
	}
	return AgentCallbackReplayResultTraceResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("callback replay result trace records %s", executionResult),
		ExecutionResult:  executionResult,
		IdempotencyHit:   idempotencyHit,
		ApprovalDecision: approvalDecision,
		SignatureResult:  signatureResult,
		FailureReason:    failureReason,
		AuditRecord:      auditRecord,
		Checks:           checks,
	}
}

func buildAgentRecoveryPolicyAutomation(gray AgentRecoveryPolicyGrayReleaseResponse) AgentRecoveryPolicyAutomationResponse {
	autoAdvance := "advance_when_error_rate_stable_and_approval_granted"
	pauseCondition := "pause_on_alert_or_manual_hold"
	rollbackCondition := gray.RollbackCondition
	if strings.TrimSpace(rollbackCondition) == "" {
		rollbackCondition = "alert_failure_rate_or_manual_reject"
	}
	currentPercent := gray.ReleasePercent
	nextPercent := currentPercent
	if nextPercent < 5 {
		nextPercent = 5
	} else if nextPercent < 100 {
		nextPercent = nextPercent * 2
		if nextPercent > 100 {
			nextPercent = 100
		}
	}
	auditEvidence := gray.AuditEvidence
	if strings.TrimSpace(auditEvidence) == "" {
		auditEvidence = "agent.recovery_policy_gray_release_snapshot"
	}
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("auto_advance", autoAdvance),
		agentGovernanceTextCheck("pause_condition", pauseCondition),
		agentGovernanceTextCheck("rollback_condition", rollbackCondition),
		agentGovernanceCheck("current_percent", currentPercent >= 0 && currentPercent <= 100, strconv.Itoa(currentPercent)),
		agentGovernanceCheck("next_percent", nextPercent >= 0 && nextPercent <= 100, strconv.Itoa(nextPercent)),
		agentGovernanceTextCheck("audit_evidence", auditEvidence),
	}
	return AgentRecoveryPolicyAutomationResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("recovery automation moves from %d%% to %d%%", currentPercent, nextPercent),
		AutoAdvance:       autoAdvance,
		PauseCondition:    pauseCondition,
		RollbackCondition: rollbackCondition,
		CurrentPercent:    currentPercent,
		NextPercent:       nextPercent,
		AuditEvidence:     auditEvidence,
		Checks:            checks,
	}
}

func buildAgentDualEndTaskClosure(pilot AgentWeChatTemplatePilotResponse, action AgentWebEvidenceUserActionResponse, trace AgentCallbackReplayResultTraceResponse, automation AgentRecoveryPolicyAutomationResponse, audits []domain.AgentAuditLog) AgentDualEndTaskClosureResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat_template_pilot") || auditEventContains(audits, "web_evidence_user_action") || auditEventContains(audits, "callback_replay_result_trace") || auditEventContains(audits, "recovery_policy_automation") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_template_pilot", Status: pilot.Status, Summary: pilot.Summary},
		{Key: "web_evidence_user_action", Status: action.Status, Summary: action.Summary},
		{Key: "callback_replay_result_trace", Status: trace.Status, Summary: trace.Summary},
		{Key: "recovery_policy_automation", Status: automation.Status, Summary: automation.Summary},
		{Key: "audit", Status: auditStatus, Summary: "dual-end task closure is audit-backed"},
	}
	nextAction := "进入企业微信模板试运行指标、证据页面操作 API 和恢复策略自动化执行"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐双端任务闭环缺口后再进入自动化执行"
	}
	return AgentDualEndTaskClosureResponse{
		Status:                    checksStatus(checks),
		Summary:                   fmt.Sprintf("dual-end task closure has %d checks", len(checks)),
		WeChatPilotStatus:         pilot.Status,
		WebEvidenceActionStatus:   action.Status,
		CallbackReplayTraceStatus: trace.Status,
		RecoveryAutomationStatus:  automation.Status,
		AuditStatus:               auditStatus,
		NextAction:                nextAction,
		Checks:                    checks,
	}
}
