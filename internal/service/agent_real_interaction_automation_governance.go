package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
)

func buildAgentWeChatTemplatePilotMetric(pilot AgentWeChatTemplatePilotResponse, audits []domain.AgentAuditLog) AgentWeChatTemplatePilotMetricResponse {
	batchID := pilot.PilotBatch
	targetUserScope := pilot.TargetScope
	sendStatus := pilot.TemplateStatus
	fallbackCount := 0
	if pilot.FallbackHit != "fallback_available" {
		fallbackCount = 1
	}
	messageIDStatus := pilot.MessageIDStatus
	auditRef := pilot.AuditEvidence
	if strings.TrimSpace(auditRef) == "" {
		auditRef = "agent.wechat_template_pilot_snapshot"
	}
	auditBacked := auditEventContains(audits, "wechat_template_pilot") || auditEventContains(audits, "wechat_template") || len(audits) > 0
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("batch_id", batchID),
		agentGovernanceTextCheck("target_user_scope", targetUserScope),
		agentGovernanceTextCheck("send_status", sendStatus),
		agentGovernanceCheck("fallback_count", fallbackCount >= 0, strconv.Itoa(fallbackCount)),
		agentGovernanceTextCheck("message_id_status", messageIDStatus),
		agentGovernanceCheck("audit_ref", strings.TrimSpace(auditRef) != "" && auditBacked, auditRef),
	}
	return AgentWeChatTemplatePilotMetricResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat template pilot metric %s send %s", batchID, sendStatus),
		BatchID:         batchID,
		TargetUserScope: targetUserScope,
		SendStatus:      sendStatus,
		FallbackCount:   fallbackCount,
		MessageIDStatus: messageIDStatus,
		AuditRef:        auditRef,
		Checks:          checks,
	}
}

func buildAgentWebEvidenceOperation(action AgentWebEvidenceUserActionResponse, audits []domain.AgentAuditLog) AgentWebEvidenceOperationResponse {
	filterEntry := action.FilterAction
	expandEntry := action.ExpandAction
	timelineEntry := action.TimelineAction
	replayRequestEntry := action.ReplayRequest
	permissionGate := action.PermissionResult
	auditEvent := "agent.web_evidence_operation_snapshot"
	operationCount := 4
	if strings.TrimSpace(replayRequestEntry) == "" {
		operationCount--
	}
	auditBacked := auditEventContains(audits, "web_evidence_user_action") || auditEventContains(audits, "web_evidence") || len(audits) > 0
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("filter_entry", filterEntry),
		agentGovernanceTextCheck("expand_entry", expandEntry),
		agentGovernanceTextCheck("timeline_entry", timelineEntry),
		agentGovernanceTextCheck("replay_request_entry", replayRequestEntry),
		agentGovernanceTextCheck("permission_gate", permissionGate),
		agentGovernanceCheck("audit_event", auditBacked, auditEvent),
		agentGovernanceCheck("operation_count", operationCount > 0, strconv.Itoa(operationCount)),
	}
	return AgentWebEvidenceOperationResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("web evidence operation exposes %d actions", operationCount),
		FilterEntry:        filterEntry,
		ExpandEntry:        expandEntry,
		TimelineEntry:      timelineEntry,
		ReplayRequestEntry: replayRequestEntry,
		PermissionGate:     permissionGate,
		AuditEvent:         auditEvent,
		OperationCount:     operationCount,
		Checks:             checks,
	}
}

func buildAgentCallbackReplayResultQuery(trace AgentCallbackReplayResultTraceResponse, audits []domain.AgentAuditLog) AgentCallbackReplayResultQueryResponse {
	queryEntry := "/api/v1/agent/callback-replay/results"
	auditRef := trace.AuditRecord
	if strings.TrimSpace(auditRef) == "" {
		auditRef = "agent.callback_replay_result_trace_snapshot"
	}
	auditBacked := auditEventContains(audits, "callback_replay_result_trace") || auditEventContains(audits, "callback_replay") || len(audits) > 0
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("query_entry", queryEntry),
		agentGovernanceTextCheck("execution_result", trace.ExecutionResult),
		agentGovernanceTextCheck("idempotency_result", trace.IdempotencyHit),
		agentGovernanceTextCheck("approval_decision", trace.ApprovalDecision),
		agentGovernanceTextCheck("signature_result", trace.SignatureResult),
		agentGovernanceTextCheck("failure_reason", trace.FailureReason),
		agentGovernanceCheck("audit_ref", strings.TrimSpace(auditRef) != "" && auditBacked, auditRef),
	}
	return AgentCallbackReplayResultQueryResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("callback replay result query returns %s", trace.ExecutionResult),
		QueryEntry:        queryEntry,
		ExecutionResult:   trace.ExecutionResult,
		IdempotencyResult: trace.IdempotencyHit,
		ApprovalDecision:  trace.ApprovalDecision,
		SignatureResult:   trace.SignatureResult,
		FailureReason:     trace.FailureReason,
		AuditRef:          auditRef,
		Checks:            checks,
	}
}

func buildAgentRecoveryAutomationExecution(automation AgentRecoveryPolicyAutomationResponse, audits []domain.AgentAuditLog) AgentRecoveryAutomationExecutionResponse {
	executionMode := "recommendation_only"
	advanceDecision := "hold_for_approval"
	if automation.NextPercent > automation.CurrentPercent && strings.TrimSpace(automation.AutoAdvance) != "" {
		advanceDecision = "advance_after_approval"
	}
	pauseGate := automation.PauseCondition
	rollbackGate := automation.RollbackCondition
	approvalGate := "required_before_execution"
	auditRef := automation.AuditEvidence
	if strings.TrimSpace(auditRef) == "" {
		auditRef = "agent.recovery_policy_automation_snapshot"
	}
	auditBacked := auditEventContains(audits, "recovery_policy_automation") || auditEventContains(audits, "recovery_policy") || len(audits) > 0
	checks := []AgentDeploymentCheckResponse{
		agentGovernanceTextCheck("execution_mode", executionMode),
		agentGovernanceCheck("current_percent", automation.CurrentPercent >= 0 && automation.CurrentPercent <= 100, strconv.Itoa(automation.CurrentPercent)),
		agentGovernanceCheck("next_percent", automation.NextPercent >= 0 && automation.NextPercent <= 100, strconv.Itoa(automation.NextPercent)),
		agentGovernanceTextCheck("advance_decision", advanceDecision),
		agentGovernanceTextCheck("pause_gate", pauseGate),
		agentGovernanceTextCheck("rollback_gate", rollbackGate),
		agentGovernanceTextCheck("approval_gate", approvalGate),
		agentGovernanceCheck("audit_ref", strings.TrimSpace(auditRef) != "" && auditBacked, auditRef),
	}
	return AgentRecoveryAutomationExecutionResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("recovery automation execution %s from %d%% to %d%%", advanceDecision, automation.CurrentPercent, automation.NextPercent),
		ExecutionMode:   executionMode,
		CurrentPercent:  automation.CurrentPercent,
		NextPercent:     automation.NextPercent,
		AdvanceDecision: advanceDecision,
		PauseGate:       pauseGate,
		RollbackGate:    rollbackGate,
		ApprovalGate:    approvalGate,
		AuditRef:        auditRef,
		Checks:          checks,
	}
}

func buildAgentRealInteractionAutomation(metric AgentWeChatTemplatePilotMetricResponse, operation AgentWebEvidenceOperationResponse, query AgentCallbackReplayResultQueryResponse, execution AgentRecoveryAutomationExecutionResponse, audits []domain.AgentAuditLog) AgentRealInteractionAutomationResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat_template_pilot") || auditEventContains(audits, "web_evidence_user_action") || auditEventContains(audits, "callback_replay_result_trace") || auditEventContains(audits, "recovery_policy_automation") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "pilot_metric", Status: metric.Status, Summary: metric.Summary},
		{Key: "evidence_operation", Status: operation.Status, Summary: operation.Summary},
		{Key: "replay_result_query", Status: query.Status, Summary: query.Summary},
		{Key: "recovery_execution", Status: execution.Status, Summary: execution.Summary},
		{Key: "audit", Status: auditStatus, Summary: "real interaction automation is audit-backed"},
	}
	nextAction := "进入企业微信真实模板发送适配、Web 证据端到端操作和恢复策略推进落库"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐真实交互自动化审计和执行门禁后再进入端到端适配"
	}
	return AgentRealInteractionAutomationResponse{
		Status:                  checksStatus(checks),
		Summary:                 fmt.Sprintf("real interaction automation has %d checks", len(checks)),
		PilotMetricStatus:       metric.Status,
		EvidenceOperationStatus: operation.Status,
		ReplayQueryStatus:       query.Status,
		RecoveryExecutionStatus: execution.Status,
		AuditStatus:             auditStatus,
		NextAction:              nextAction,
		Checks:                  checks,
	}
}
