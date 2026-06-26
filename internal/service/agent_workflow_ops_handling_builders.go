package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strings"
)

func buildAgentWriteRampStage(recommendation AgentWriteRampRecommendationResponse) AgentWriteRampStageResponse {
	currentStage := "stage_1_controlled"
	nextStage := "stage_2_limited_expansion"
	if recommendation.RecommendedPercent > 10 {
		nextStage = "stage_3_gradual_expansion"
	}
	entryConditions := append([]string(nil), recommendation.LimitConditions...)
	exitConditions := []string{"monitor_readback_failed", "approval_gate_failed", "budget_gate_failed"}
	rollbackConditions := append([]string(nil), recommendation.RollbackConditions...)
	checks := []AgentDeploymentCheckResponse{
		{Key: "current_stage", Status: readyIf(strings.TrimSpace(currentStage) != ""), Summary: currentStage},
		{Key: "next_stage", Status: readyIf(strings.TrimSpace(nextStage) != ""), Summary: nextStage},
		{Key: "entry_conditions", Status: readyIf(len(entryConditions) > 0), Summary: strings.Join(entryConditions, ", ")},
		{Key: "exit_conditions", Status: readyIf(len(exitConditions) > 0), Summary: strings.Join(exitConditions, ", ")},
		{Key: "rollback_conditions", Status: readyIf(len(rollbackConditions) > 0), Summary: strings.Join(rollbackConditions, ", ")},
		{Key: "default_deny", Status: readyIf(recommendation.DefaultAction == "reject_or_require_approval"), Summary: recommendation.DefaultAction},
	}
	return AgentWriteRampStageResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("write ramp stage %s -> %s", currentStage, nextStage),
		CurrentStage:       currentStage,
		NextStage:          nextStage,
		EntryConditions:    entryConditions,
		ExitConditions:     exitConditions,
		RollbackConditions: rollbackConditions,
		DefaultAction:      recommendation.DefaultAction,
		Checks:             checks,
	}
}

func buildAgentWeChatFeedbackLoop(feedback AgentWeChatUserFeedbackResponse, finalReport AgentWeChatFinalReportResponse, direct AgentButtonDirectControlResponse) AgentWeChatFeedbackLoopResponse {
	processingState := "feedback_ready_for_followup"
	if feedback.Status != "ready" {
		processingState = "feedback_needs_review"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "completion_feedback", Status: feedback.CompletionFeedback, Summary: finalReport.FinalReportEntry},
		{Key: "failure_feedback", Status: feedback.FailureFeedback, Summary: finalReport.FailureSummary},
		{Key: "button_feedback", Status: feedback.ButtonFeedback, Summary: direct.Summary},
		{Key: "web_tracking_feedback", Status: feedback.WebTrackingFeedback, Summary: "web progress and task workspace are linked"},
		{Key: "processing_state", Status: readyIf(strings.TrimSpace(processingState) != ""), Summary: processingState},
		{Key: "next_action", Status: readyIf(strings.TrimSpace(feedback.NextAction) != ""), Summary: feedback.NextAction},
	}
	return AgentWeChatFeedbackLoopResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat feedback loop %s with %d checks", processingState, len(checks)),
		CompletionState: feedback.CompletionFeedback,
		FailureState:    feedback.FailureFeedback,
		ButtonState:     feedback.ButtonFeedback,
		WebTraceState:   feedback.WebTrackingFeedback,
		ProcessingState: processingState,
		NextAction:      feedback.NextAction,
		Checks:          checks,
	}
}

func buildAgentOperationsClosedLoop(panel AgentOpsPanelConfigResponse, report AgentMonitorAutoReportResponse, stage AgentWriteRampStageResponse, feedback AgentWeChatFeedbackLoopResponse, audits []domain.AgentAuditLog) AgentOperationsClosedLoopResponse {
	auditStatus := readyIf(auditEventContains(audits, "operations") || auditEventContains(audits, "monitor") || auditEventContains(audits, "write") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "ops_panel", Status: panel.Status, Summary: panel.Summary},
		{Key: "monitor_auto_report", Status: report.Status, Summary: report.Summary},
		{Key: "write_ramp_stage", Status: stage.Status, Summary: stage.Summary},
		{Key: "wechat_feedback_loop", Status: feedback.Status, Summary: feedback.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations closed loop is audit-backed"},
	}
	nextAction := "进入运营面板交互配置与异常汇报去重升级"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运营闭环缺口后再进入交互配置"
	}
	return AgentOperationsClosedLoopResponse{
		Status:               checksStatus(checks),
		Summary:              fmt.Sprintf("operations closed loop has %d checks", len(checks)),
		OpsPanelStatus:       panel.Status,
		MonitorReportStatus:  report.Status,
		WriteRampStageStatus: stage.Status,
		FeedbackLoopStatus:   feedback.Status,
		AuditStatus:          auditStatus,
		NextAction:           nextAction,
		Checks:               checks,
	}
}

func buildAgentOpsDashboardInteraction(panel AgentOpsPanelConfigResponse, loop AgentOperationsClosedLoopResponse) AgentOpsDashboardInteractionResponse {
	actions := []string{"view_progress", "open_alerts", "review_write_ramp", "open_wechat_feedback"}
	filters := []string{"status", "entry", "capability", "audit_status"}
	links := []string{panel.AlertEntry, panel.WriteRampEntry, panel.WeChatFeedbackEntry}
	refreshStrategy := fmt.Sprintf("poll_%ds", panel.RefreshIntervalSeconds)
	checks := []AgentDeploymentCheckResponse{
		{Key: "actions", Status: readyIf(len(actions) > 0), Summary: strings.Join(actions, ", ")},
		{Key: "refresh_strategy", Status: readyIf(panel.RefreshIntervalSeconds > 0), Summary: refreshStrategy},
		{Key: "filters", Status: readyIf(len(filters) > 0), Summary: strings.Join(filters, ", ")},
		{Key: "links", Status: readyIf(len(links) > 0), Summary: strings.Join(links, ", ")},
		{Key: "audit", Status: loop.AuditStatus, Summary: "ops dashboard interaction is audit-backed"},
	}
	return AgentOpsDashboardInteractionResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("ops dashboard interaction exposes %d actions and %d filters", len(actions), len(filters)),
		Actions:         actions,
		RefreshStrategy: refreshStrategy,
		Filters:         filters,
		Links:           links,
		AuditEvent:      "agent.ops_dashboard_interaction_snapshot",
		Checks:          checks,
	}
}

func buildAgentAlertDedupeEscalation(report AgentMonitorAutoReportResponse, readback AgentMonitorReadbackResponse) AgentAlertDedupeEscalationResponse {
	dedupeKey := strings.Join(readback.EventNames, "|")
	if strings.TrimSpace(dedupeKey) == "" {
		dedupeKey = "agent.monitor.default"
	}
	dedupeWindowSeconds := 300
	escalationCondition := "same_dedupe_key_repeats_or_status_failed"
	checks := []AgentDeploymentCheckResponse{
		{Key: "dedupe_key", Status: readyIf(strings.TrimSpace(dedupeKey) != ""), Summary: dedupeKey},
		{Key: "dedupe_window", Status: readyIf(dedupeWindowSeconds > 0), Summary: fmt.Sprintf("%ds", dedupeWindowSeconds)},
		{Key: "escalation_condition", Status: readyIf(strings.TrimSpace(escalationCondition) != ""), Summary: escalationCondition},
		{Key: "wechat_notify", Status: report.WeChatSendStatus, Summary: report.Summary},
		{Key: "web_visibility", Status: report.WebVisibilityStatus, Summary: "web operations panel exposes deduped alert"},
	}
	return AgentAlertDedupeEscalationResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("alert dedupe %s over %ds", dedupeKey, dedupeWindowSeconds),
		DedupeKey:           dedupeKey,
		DedupeWindowSeconds: dedupeWindowSeconds,
		EscalationCondition: escalationCondition,
		WeChatNotifyStatus:  report.WeChatSendStatus,
		WebVisibilityStatus: report.WebVisibilityStatus,
		Checks:              checks,
	}
}

func buildAgentWriteStageRecord(stage AgentWriteRampStageResponse, recommendation AgentWriteRampRecommendationResponse) AgentWriteStageRecordResponse {
	promotionReason := fmt.Sprintf("recommended ramp %d%%", recommendation.RecommendedPercent)
	blockers := append([]string(nil), stage.ExitConditions...)
	checks := []AgentDeploymentCheckResponse{
		{Key: "current_stage", Status: readyIf(strings.TrimSpace(stage.CurrentStage) != ""), Summary: stage.CurrentStage},
		{Key: "target_stage", Status: readyIf(strings.TrimSpace(stage.NextStage) != ""), Summary: stage.NextStage},
		{Key: "promotion_reason", Status: readyIf(strings.TrimSpace(promotionReason) != ""), Summary: promotionReason},
		{Key: "blockers", Status: readyIf(len(blockers) > 0), Summary: strings.Join(blockers, ", ")},
		{Key: "rollback_conditions", Status: readyIf(len(stage.RollbackConditions) > 0), Summary: strings.Join(stage.RollbackConditions, ", ")},
		{Key: "default_deny", Status: readyIf(stage.DefaultAction == "reject_or_require_approval"), Summary: stage.DefaultAction},
	}
	return AgentWriteStageRecordResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("write stage record %s -> %s", stage.CurrentStage, stage.NextStage),
		CurrentStage:       stage.CurrentStage,
		TargetStage:        stage.NextStage,
		PromotionReason:    promotionReason,
		Blockers:           blockers,
		RollbackConditions: append([]string(nil), stage.RollbackConditions...),
		DefaultAction:      stage.DefaultAction,
		Checks:             checks,
	}
}

func buildAgentWeChatFeedbackTicket(loop AgentWeChatFeedbackLoopResponse) AgentWeChatFeedbackTicketResponse {
	ticketType := "wechat_feedback_followup"
	ownerEntry := "agent_operations_owner"
	checks := []AgentDeploymentCheckResponse{
		{Key: "ticket_type", Status: readyIf(strings.TrimSpace(ticketType) != ""), Summary: ticketType},
		{Key: "processing_state", Status: readyIf(strings.TrimSpace(loop.ProcessingState) != ""), Summary: loop.ProcessingState},
		{Key: "owner_entry", Status: readyIf(strings.TrimSpace(ownerEntry) != ""), Summary: ownerEntry},
		{Key: "user_next_action", Status: readyIf(strings.TrimSpace(loop.NextAction) != ""), Summary: loop.NextAction},
		{Key: "audit", Status: loop.Status, Summary: "wechat feedback ticket is audit-backed"},
	}
	return AgentWeChatFeedbackTicketResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat feedback ticket %s is %s", ticketType, loop.ProcessingState),
		TicketType:      ticketType,
		ProcessingState: loop.ProcessingState,
		OwnerEntry:      ownerEntry,
		UserNextAction:  loop.NextAction,
		AuditEvent:      "agent.wechat_feedback_ticket_snapshot",
		Checks:          checks,
	}
}

func buildAgentOperationsHandling(dashboard AgentOpsDashboardInteractionResponse, escalation AgentAlertDedupeEscalationResponse, stage AgentWriteStageRecordResponse, ticket AgentWeChatFeedbackTicketResponse, audits []domain.AgentAuditLog) AgentOperationsHandlingResponse {
	auditStatus := readyIf(auditEventContains(audits, "operations") || auditEventContains(audits, "monitor") || auditEventContains(audits, "write") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "dashboard_interaction", Status: dashboard.Status, Summary: dashboard.Summary},
		{Key: "alert_dedupe_escalation", Status: escalation.Status, Summary: escalation.Summary},
		{Key: "write_stage_record", Status: stage.Status, Summary: stage.Summary},
		{Key: "wechat_feedback_ticket", Status: ticket.Status, Summary: ticket.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations handling is audit-backed"},
	}
	nextAction := "进入运营面板真实交互动作和异常升级通知策略"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运营处理缺口后再进入真实交互动作"
	}
	return AgentOperationsHandlingResponse{
		Status:                checksStatus(checks),
		Summary:               fmt.Sprintf("operations handling has %d checks", len(checks)),
		DashboardStatus:       dashboard.Status,
		AlertEscalationStatus: escalation.Status,
		WriteStageStatus:      stage.Status,
		FeedbackTicketStatus:  ticket.Status,
		AuditStatus:           auditStatus,
		NextAction:            nextAction,
		Checks:                checks,
	}
}

func buildAgentOpsActionDefinition(dashboard AgentOpsDashboardInteractionResponse, handling AgentOperationsHandlingResponse) AgentOpsActionDefinitionResponse {
	labels := map[string]string{
		"view_progress":        "查看实时进度",
		"open_alerts":          "打开异常告警",
		"review_write_ramp":    "复核写阶段推进",
		"open_wechat_feedback": "打开企微反馈工单",
	}
	permissions := map[string]string{
		"view_progress":        "agent:progress:read",
		"open_alerts":          "agent:alerts:read",
		"review_write_ramp":    "agent:write_ramp:review",
		"open_wechat_feedback": "agent:feedback:review",
	}
	actions := make([]AgentOpsActionItemResponse, 0, len(dashboard.Actions))
	for _, key := range dashboard.Actions {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		label := labels[key]
		if label == "" {
			label = key
		}
		permission := permissions[key]
		if permission == "" {
			permission = "agent:ops:read"
		}
		actions = append(actions, AgentOpsActionItemResponse{
			Key:                  key,
			Label:                label,
			HandlerEntry:         "web.agent.ops." + key,
			PermissionConstraint: permission,
			IdempotencyKey:       "ops_action:" + key,
			AuditEvent:           "agent.ops_action_definition_snapshot",
		})
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "actions", Status: readyIf(len(actions) > 0), Summary: fmt.Sprintf("%d actions", len(actions))},
		{Key: "handler_entry", Status: readyIf(allOpsActionsHave(actions, func(action AgentOpsActionItemResponse) bool { return action.HandlerEntry != "" })), Summary: "all actions have handler entry"},
		{Key: "permission_constraint", Status: readyIf(allOpsActionsHave(actions, func(action AgentOpsActionItemResponse) bool { return action.PermissionConstraint != "" })), Summary: "all actions have permission constraint"},
		{Key: "idempotency_key", Status: readyIf(allOpsActionsHave(actions, func(action AgentOpsActionItemResponse) bool { return action.IdempotencyKey != "" })), Summary: "all actions have idempotency key"},
		{Key: "audit_event", Status: readyIf(allOpsActionsHave(actions, func(action AgentOpsActionItemResponse) bool { return action.AuditEvent != "" })), Summary: "all actions have audit event"},
		{Key: "operations_handling", Status: handling.Status, Summary: handling.Summary},
	}
	return AgentOpsActionDefinitionResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("ops action definition exposes %d auditable actions", len(actions)),
		Actions: actions,
		Checks:  checks,
	}
}

func buildAgentAlertEscalationPolicy(escalation AgentAlertDedupeEscalationResponse, channel AgentAlertChannelResponse) AgentAlertEscalationPolicyResponse {
	channels := make([]string, 0, len(channel.Channels))
	for _, target := range channel.Channels {
		if strings.TrimSpace(target.Key) != "" {
			channels = append(channels, target.Key)
		}
	}
	if len(channels) == 0 {
		channels = []string{"wechat_work", "web"}
	}
	level := "warning"
	if escalation.Status == "failed" || escalation.Status == "error" {
		level = "critical"
	}
	recipients := []string{"agent_operations_oncall", "wechat_task_owner"}
	repeatSuppression := fmt.Sprintf("%ds_by_%s", escalation.DedupeWindowSeconds, escalation.DedupeKey)
	recoveryNoticeStatus := readyIf(escalation.WeChatNotifyStatus == "ready" || channel.Status == "ready")
	auditEvidence := "agent.alert_escalation_policy_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "escalation_level", Status: readyIf(level != ""), Summary: level},
		{Key: "notification_channels", Status: readyIf(len(channels) > 0), Summary: strings.Join(channels, ", ")},
		{Key: "repeat_suppression", Status: readyIf(strings.TrimSpace(repeatSuppression) != ""), Summary: repeatSuppression},
		{Key: "recipients", Status: readyIf(len(recipients) > 0), Summary: strings.Join(recipients, ", ")},
		{Key: "recovery_notice", Status: recoveryNoticeStatus, Summary: "send recovery notice after alert clears"},
		{Key: "audit_evidence", Status: readyIf(auditEvidence != ""), Summary: auditEvidence},
	}
	return AgentAlertEscalationPolicyResponse{
		Status:               checksStatus(checks),
		Summary:              fmt.Sprintf("alert escalation policy level %s over %d channels", level, len(channels)),
		EscalationLevel:      level,
		NotificationChannels: channels,
		RepeatSuppression:    repeatSuppression,
		Recipients:           recipients,
		RecoveryNoticeStatus: recoveryNoticeStatus,
		AuditEvidence:        auditEvidence,
		Checks:               checks,
	}
}

func buildAgentWriteStageApproval(record AgentWriteStageRecordResponse, policy AgentWriteRampPolicyResponse, approval AgentReleaseApprovalResponse) AgentWriteStageApprovalResponse {
	approvalStatus := approval.ReviewState
	if strings.TrimSpace(approvalStatus) == "" {
		approvalStatus = "awaiting_approval"
	}
	approvalSource := approval.DecisionPath
	if strings.TrimSpace(approvalSource) == "" {
		approvalSource = approval.AuditEvent
	}
	if strings.TrimSpace(approvalSource) == "" {
		approvalSource = "agent.write_stage_approval_snapshot"
	}
	authorizedScope := policy.UserScope
	if strings.TrimSpace(authorizedScope) == "" {
		authorizedScope = "approved_write_ramp_scope"
	}
	rollbackThreshold := policy.RollbackThreshold
	if strings.TrimSpace(rollbackThreshold) == "" {
		rollbackThreshold = strings.Join(record.RollbackConditions, ", ")
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "approval_status", Status: readyIf(strings.TrimSpace(approvalStatus) != ""), Summary: approvalStatus},
		{Key: "approval_source", Status: readyIf(strings.TrimSpace(approvalSource) != ""), Summary: approvalSource},
		{Key: "target_stage", Status: readyIf(strings.TrimSpace(record.TargetStage) != ""), Summary: record.TargetStage},
		{Key: "authorized_scope", Status: readyIf(strings.TrimSpace(authorizedScope) != ""), Summary: authorizedScope},
		{Key: "rollback_threshold", Status: readyIf(strings.TrimSpace(rollbackThreshold) != ""), Summary: rollbackThreshold},
		{Key: "default_deny", Status: readyIf(record.DefaultAction == "reject_or_require_approval"), Summary: record.DefaultAction},
	}
	return AgentWriteStageApprovalResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("write stage approval targets %s with %s", record.TargetStage, approvalStatus),
		ApprovalStatus:    approvalStatus,
		ApprovalSource:    approvalSource,
		TargetStage:       record.TargetStage,
		AuthorizedScope:   authorizedScope,
		RollbackThreshold: rollbackThreshold,
		DefaultAction:     record.DefaultAction,
		Checks:            checks,
	}
}

func buildAgentFeedbackTicketLifecycle(ticket AgentWeChatFeedbackTicketResponse, loop AgentWeChatFeedbackLoopResponse) AgentFeedbackTicketLifecycleResponse {
	createdState := "created"
	assignedState := readyIf(strings.TrimSpace(ticket.OwnerEntry) != "")
	processingState := ticket.ProcessingState
	if strings.TrimSpace(processingState) == "" {
		processingState = "pending"
	}
	waitingUserState := "not_required"
	if strings.TrimSpace(ticket.UserNextAction) != "" {
		waitingUserState = "waiting_user_followup"
	}
	closedState := loop.CompletionState
	if strings.TrimSpace(closedState) == "" {
		closedState = "open_until_final_report"
	}
	handoffState := "manual_handoff_on_failure"
	if loop.FailureState != "" {
		handoffState = loop.FailureState
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "created", Status: readyIf(createdState != ""), Summary: createdState},
		{Key: "assigned", Status: assignedState, Summary: ticket.OwnerEntry},
		{Key: "processing", Status: readyIf(strings.TrimSpace(processingState) != ""), Summary: processingState},
		{Key: "waiting_user", Status: readyIf(strings.TrimSpace(waitingUserState) != ""), Summary: waitingUserState},
		{Key: "closed", Status: readyIf(strings.TrimSpace(closedState) != ""), Summary: closedState},
		{Key: "handoff", Status: readyIf(strings.TrimSpace(handoffState) != ""), Summary: handoffState},
	}
	return AgentFeedbackTicketLifecycleResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("feedback ticket lifecycle is %s and %s", processingState, waitingUserState),
		CreatedState:     createdState,
		AssignedState:    assignedState,
		ProcessingState:  processingState,
		WaitingUserState: waitingUserState,
		ClosedState:      closedState,
		HandoffState:     handoffState,
		Checks:           checks,
	}
}

func buildAgentOperationsActionClosure(action AgentOpsActionDefinitionResponse, escalation AgentAlertEscalationPolicyResponse, approval AgentWriteStageApprovalResponse, lifecycle AgentFeedbackTicketLifecycleResponse, audits []domain.AgentAuditLog) AgentOperationsActionClosureResponse {
	auditStatus := readyIf(auditEventContains(audits, "ops") || auditEventContains(audits, "alert") || auditEventContains(audits, "write") || auditEventContains(audits, "feedback") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "ops_action_definition", Status: action.Status, Summary: action.Summary},
		{Key: "alert_escalation_policy", Status: escalation.Status, Summary: escalation.Summary},
		{Key: "write_stage_approval", Status: approval.Status, Summary: approval.Summary},
		{Key: "feedback_ticket_lifecycle", Status: lifecycle.Status, Summary: lifecycle.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations action closure is audit-backed"},
	}
	nextAction := "进入运营动作真实 API 执行、升级通知回执和工单 SLA"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运营动作闭环缺口后再进入真实 API 执行"
	}
	return AgentOperationsActionClosureResponse{
		Status:                checksStatus(checks),
		Summary:               fmt.Sprintf("operations action closure has %d checks", len(checks)),
		OpsActionStatus:       action.Status,
		AlertEscalationStatus: escalation.Status,
		WriteApprovalStatus:   approval.Status,
		TicketLifecycleStatus: lifecycle.Status,
		AuditStatus:           auditStatus,
		NextAction:            nextAction,
		Checks:                checks,
	}
}

func buildAgentOpsAPIExecution(definition AgentOpsActionDefinitionResponse, closure AgentOperationsActionClosureResponse) AgentOpsAPIExecutionResponse {
	executions := make([]AgentOpsAPIExecutionItemResponse, 0, len(definition.Actions))
	for _, action := range definition.Actions {
		executions = append(executions, AgentOpsAPIExecutionItemResponse{
			ActionKey:         action.Key,
			ExecutionEntry:    action.HandlerEntry,
			ExecutionStatus:   readyIf(action.HandlerEntry != ""),
			PermissionCheck:   readyIf(action.PermissionConstraint != ""),
			IdempotencyResult: readyIf(action.IdempotencyKey != ""),
			AuditEvent:        "agent.ops_api_execution_snapshot",
		})
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "executions", Status: readyIf(len(executions) > 0), Summary: fmt.Sprintf("%d executions", len(executions))},
		{Key: "execution_entry", Status: readyIf(allOpsAPIExecutionsHave(executions, func(item AgentOpsAPIExecutionItemResponse) bool { return item.ExecutionEntry != "" })), Summary: "all executions have entry"},
		{Key: "permission_check", Status: readyIf(allOpsAPIExecutionsHave(executions, func(item AgentOpsAPIExecutionItemResponse) bool { return item.PermissionCheck != "" })), Summary: "all executions have permission check"},
		{Key: "idempotency_result", Status: readyIf(allOpsAPIExecutionsHave(executions, func(item AgentOpsAPIExecutionItemResponse) bool { return item.IdempotencyResult != "" })), Summary: "all executions have idempotency result"},
		{Key: "audit_event", Status: readyIf(allOpsAPIExecutionsHave(executions, func(item AgentOpsAPIExecutionItemResponse) bool { return item.AuditEvent != "" })), Summary: "all executions have audit event"},
		{Key: "operations_action_closure", Status: closure.Status, Summary: closure.Summary},
	}
	return AgentOpsAPIExecutionResponse{
		Status:     checksStatus(checks),
		Summary:    fmt.Sprintf("ops api execution tracks %d action executions", len(executions)),
		Executions: executions,
		Checks:     checks,
	}
}

func buildAgentAlertEscalationReceipt(policy AgentAlertEscalationPolicyResponse) AgentAlertEscalationReceiptResponse {
	deliveryStatus := readyIf(len(policy.NotificationChannels) > 0 && len(policy.Recipients) > 0)
	suppressionResult := "suppressed_by_" + policy.RepeatSuppression
	if strings.TrimSpace(policy.RepeatSuppression) == "" {
		suppressionResult = "not_suppressed"
	}
	handoffEntry := "agent_operations_oncall"
	checks := []AgentDeploymentCheckResponse{
		{Key: "notification_channels", Status: readyIf(len(policy.NotificationChannels) > 0), Summary: strings.Join(policy.NotificationChannels, ", ")},
		{Key: "recipients", Status: readyIf(len(policy.Recipients) > 0), Summary: strings.Join(policy.Recipients, ", ")},
		{Key: "delivery_status", Status: deliveryStatus, Summary: "delivery receipt is tracked"},
		{Key: "suppression_result", Status: readyIf(strings.TrimSpace(suppressionResult) != ""), Summary: suppressionResult},
		{Key: "recovery_notice", Status: policy.RecoveryNoticeStatus, Summary: "recovery notice receipt is tracked"},
		{Key: "handoff_entry", Status: readyIf(strings.TrimSpace(handoffEntry) != ""), Summary: handoffEntry},
	}
	return AgentAlertEscalationReceiptResponse{
		Status:               checksStatus(checks),
		Summary:              fmt.Sprintf("alert escalation receipt uses %d channels and %d recipients", len(policy.NotificationChannels), len(policy.Recipients)),
		NotificationChannels: append([]string(nil), policy.NotificationChannels...),
		Recipients:           append([]string(nil), policy.Recipients...),
		DeliveryStatus:       deliveryStatus,
		SuppressionResult:    suppressionResult,
		RecoveryNoticeStatus: policy.RecoveryNoticeStatus,
		HandoffEntry:         handoffEntry,
		Checks:               checks,
	}
}

func buildAgentWriteApprovalButton(approval AgentWriteStageApprovalResponse) AgentWriteApprovalButtonResponse {
	channels := []string{"web", "wechat_work"}
	buttons := make([]AgentWriteApprovalButtonItemResponse, 0, len(channels)*2)
	for _, channel := range channels {
		for _, action := range []string{"approve", "reject"} {
			buttons = append(buttons, AgentWriteApprovalButtonItemResponse{
				ButtonKey:         fmt.Sprintf("write_stage_%s_%s", action, channel),
				Channel:           channel,
				ApprovalStatus:    approval.ApprovalStatus,
				PermissionScope:   approval.AuthorizedScope,
				RollbackThreshold: approval.RollbackThreshold,
				RejectionPath:     "reject_or_require_approval",
				AuditEvidence:     "agent.write_approval_button_snapshot",
			})
		}
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "buttons", Status: readyIf(len(buttons) > 0), Summary: fmt.Sprintf("%d buttons", len(buttons))},
		{Key: "approval_status", Status: readyIf(strings.TrimSpace(approval.ApprovalStatus) != ""), Summary: approval.ApprovalStatus},
		{Key: "permission_scope", Status: readyIf(allWriteApprovalButtonsHave(buttons, func(button AgentWriteApprovalButtonItemResponse) bool { return button.PermissionScope != "" })), Summary: approval.AuthorizedScope},
		{Key: "rollback_threshold", Status: readyIf(allWriteApprovalButtonsHave(buttons, func(button AgentWriteApprovalButtonItemResponse) bool { return button.RollbackThreshold != "" })), Summary: approval.RollbackThreshold},
		{Key: "rejection_path", Status: readyIf(allWriteApprovalButtonsHave(buttons, func(button AgentWriteApprovalButtonItemResponse) bool { return button.RejectionPath != "" })), Summary: "reject path is explicit"},
		{Key: "audit_evidence", Status: readyIf(allWriteApprovalButtonsHave(buttons, func(button AgentWriteApprovalButtonItemResponse) bool { return button.AuditEvidence != "" })), Summary: "button actions are audit-backed"},
	}
	return AgentWriteApprovalButtonResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("write approval button exposes %d web/wechat decisions", len(buttons)),
		Buttons: buttons,
		Checks:  checks,
	}
}

func buildAgentFeedbackTicketSLA(lifecycle AgentFeedbackTicketLifecycleResponse) AgentFeedbackTicketSLAResponse {
	firstResponseSeconds := 300
	resolveSeconds := 86400
	timeoutEscalation := "escalate_to_agent_operations_oncall"
	closeCondition := "closed_after_final_report_or_user_ack"
	handoffPath := lifecycle.HandoffState
	if strings.TrimSpace(handoffPath) == "" {
		handoffPath = "manual_handoff_on_failure"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "first_response", Status: readyIf(firstResponseSeconds > 0), Summary: fmt.Sprintf("%ds", firstResponseSeconds)},
		{Key: "resolve_time", Status: readyIf(resolveSeconds > 0), Summary: fmt.Sprintf("%ds", resolveSeconds)},
		{Key: "timeout_escalation", Status: readyIf(strings.TrimSpace(timeoutEscalation) != ""), Summary: timeoutEscalation},
		{Key: "waiting_user", Status: readyIf(strings.TrimSpace(lifecycle.WaitingUserState) != ""), Summary: lifecycle.WaitingUserState},
		{Key: "close_condition", Status: readyIf(strings.TrimSpace(closeCondition) != ""), Summary: closeCondition},
		{Key: "handoff_path", Status: readyIf(strings.TrimSpace(handoffPath) != ""), Summary: handoffPath},
	}
	return AgentFeedbackTicketSLAResponse{
		Status:               checksStatus(checks),
		Summary:              fmt.Sprintf("feedback ticket sla first response %ds resolve %ds", firstResponseSeconds, resolveSeconds),
		FirstResponseSeconds: firstResponseSeconds,
		ResolveSeconds:       resolveSeconds,
		TimeoutEscalation:    timeoutEscalation,
		WaitingUserStatus:    lifecycle.WaitingUserState,
		CloseCondition:       closeCondition,
		HandoffPath:          handoffPath,
		Checks:               checks,
	}
}

func buildAgentOperationsExecution(api AgentOpsAPIExecutionResponse, receipt AgentAlertEscalationReceiptResponse, button AgentWriteApprovalButtonResponse, sla AgentFeedbackTicketSLAResponse, audits []domain.AgentAuditLog) AgentOperationsExecutionResponse {
	auditStatus := readyIf(auditEventContains(audits, "ops") || auditEventContains(audits, "alert") || auditEventContains(audits, "write") || auditEventContains(audits, "feedback") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "ops_api_execution", Status: api.Status, Summary: api.Summary},
		{Key: "alert_escalation_receipt", Status: receipt.Status, Summary: receipt.Summary},
		{Key: "write_approval_button", Status: button.Status, Summary: button.Summary},
		{Key: "feedback_ticket_sla", Status: sla.Status, Summary: sla.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations execution is audit-backed"},
	}
	nextAction := "进入运营动作持久化执行记录、审批回调入库和 SLA 报表"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运营执行闭环缺口后再进入持久化执行记录"
	}
	return AgentOperationsExecutionResponse{
		Status:                    checksStatus(checks),
		Summary:                   fmt.Sprintf("operations execution has %d checks", len(checks)),
		OpsAPIExecutionStatus:     api.Status,
		AlertReceiptStatus:        receipt.Status,
		WriteApprovalButtonStatus: button.Status,
		FeedbackSLAStatus:         sla.Status,
		AuditStatus:               auditStatus,
		NextAction:                nextAction,
		Checks:                    checks,
	}
}

func buildAgentOpsExecutionRecord(api AgentOpsAPIExecutionResponse) AgentOpsExecutionRecordResponse {
	records := make([]AgentOpsExecutionRecordItemResponse, 0, len(api.Executions))
	for _, execution := range api.Executions {
		records = append(records, AgentOpsExecutionRecordItemResponse{
			RecordKey:         "ops_execution:" + execution.ActionKey,
			ActionKey:         execution.ActionKey,
			ExecutionStatus:   execution.ExecutionStatus,
			IdempotencyStatus: execution.IdempotencyResult,
			AuditEvent:        "agent.ops_execution_record_snapshot",
			ReplayEntry:       "web.agent.ops.replay." + execution.ActionKey,
		})
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "records", Status: readyIf(len(records) > 0), Summary: fmt.Sprintf("%d records", len(records))},
		{Key: "record_key", Status: readyIf(allOpsExecutionRecordsHave(records, func(record AgentOpsExecutionRecordItemResponse) bool { return record.RecordKey != "" })), Summary: "all records have keys"},
		{Key: "idempotency", Status: readyIf(allOpsExecutionRecordsHave(records, func(record AgentOpsExecutionRecordItemResponse) bool { return record.IdempotencyStatus != "" })), Summary: "all records keep idempotency status"},
		{Key: "audit_event", Status: readyIf(allOpsExecutionRecordsHave(records, func(record AgentOpsExecutionRecordItemResponse) bool { return record.AuditEvent != "" })), Summary: "all records have audit event"},
		{Key: "replay_entry", Status: readyIf(allOpsExecutionRecordsHave(records, func(record AgentOpsExecutionRecordItemResponse) bool { return record.ReplayEntry != "" })), Summary: "all records have replay entry"},
	}
	return AgentOpsExecutionRecordResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("ops execution record persists %d actions", len(records)),
		Records: records,
		Checks:  checks,
	}
}

func buildAgentWeChatApprovalCallback(button AgentWriteApprovalButtonResponse) AgentWeChatApprovalCallbackResponse {
	callbackKey := "wechat_write_stage_callback"
	source := "wechat_work"
	decision := "awaiting_callback"
	for _, item := range button.Buttons {
		if item.Channel == "wechat_work" && strings.TrimSpace(item.ApprovalStatus) != "" {
			decision = item.ApprovalStatus
			break
		}
	}
	signature := "verified_or_pending"
	storageState := readyIf(len(button.Buttons) > 0)
	fallbackPath := "web_write_approval_review"
	checks := []AgentDeploymentCheckResponse{
		{Key: "callback_key", Status: readyIf(callbackKey != ""), Summary: callbackKey},
		{Key: "source", Status: readyIf(source == "wechat_work"), Summary: source},
		{Key: "decision", Status: readyIf(strings.TrimSpace(decision) != ""), Summary: decision},
		{Key: "signature", Status: readyIf(strings.TrimSpace(signature) != ""), Summary: signature},
		{Key: "storage", Status: storageState, Summary: "callback is stored with approval evidence"},
		{Key: "fallback", Status: readyIf(strings.TrimSpace(fallbackPath) != ""), Summary: fallbackPath},
	}
	return AgentWeChatApprovalCallbackResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("wechat approval callback %s is %s", callbackKey, decision),
		CallbackKey:  callbackKey,
		Source:       source,
		Decision:     decision,
		Signature:    signature,
		StorageState: storageState,
		FallbackPath: fallbackPath,
		Checks:       checks,
	}
}
