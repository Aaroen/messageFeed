package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"sort"
	"strconv"
	"strings"
	"time"
)

type agentTaskAdmissionInput struct {
	UserID                 int64
	Entry                  string
	Preference             domain.AgentNotificationPreference
	Plans                  []domain.AgentPlan
	ScheduledTasks         []domain.AgentScheduledTask
	CurrentScheduledTaskID int64
	Now                    time.Time
}

type agentTaskAdmissionDecision struct {
	Allowed    bool
	Status     string
	Reason     string
	NextAction string
	Metadata   domain.AgentJSON
}

func normalizeAgentPolicyPreference(preference domain.AgentNotificationPreference) domain.AgentNotificationPreference {
	if preference.MaxConcurrentTasks < 1 {
		preference.MaxConcurrentTasks = 2
	}
	if preference.MaxConcurrentTasks > 20 {
		preference.MaxConcurrentTasks = 20
	}
	if preference.MaxQueuedTasks < 1 {
		preference.MaxQueuedTasks = 20
	}
	if preference.MaxQueuedTasks > 200 {
		preference.MaxQueuedTasks = 200
	}
	if preference.QualityHandoffThreshold <= 0 {
		preference.QualityHandoffThreshold = 0.65
	}
	if preference.QualityHandoffThreshold > 1 {
		preference.QualityHandoffThreshold = 1
	}
	if preference.DailyTaskQuota < 1 {
		preference.DailyTaskQuota = 50
	}
	if preference.DailyExternalCallQuota < 1 {
		preference.DailyExternalCallQuota = 200
	}
	if preference.DailyCapabilityCallQuota < 1 {
		preference.DailyCapabilityCallQuota = 500
	}
	preference.CapabilityPolicy = normalizeAgentCapabilityPolicy(preference.CapabilityPolicy)
	return preference
}

func normalizeAgentCapabilityPolicy(input domain.AgentJSON) domain.AgentJSON {
	output := domain.AgentJSON{}
	for key, value := range input {
		capabilityKey := strings.TrimSpace(key)
		if capabilityKey == "" {
			continue
		}
		decision := normalizeAgentCapabilityPolicyDecision(fmt.Sprint(value))
		if decision == "" {
			continue
		}
		output[capabilityKey] = decision
	}
	return output
}

func normalizeAgentCapabilityPolicyDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "allow", "allowed", "允许":
		return "allow"
	case "degrade", "degraded", "降级":
		return "degrade"
	case "confirm", "prompt", "approval", "require_approval", "确认", "审批":
		return "confirm"
	case "reject", "deny", "forbid", "拒绝":
		return "reject"
	default:
		return ""
	}
}

func evaluateAgentTaskAdmission(input agentTaskAdmissionInput) agentTaskAdmissionDecision {
	preference := normalizeAgentPolicyPreference(input.Preference)
	activePlans, queuedPlans := countAgentPlansForAdmission(input.Plans)
	activeScheduled, queuedScheduled := countAgentScheduledTasksForAdmission(input.ScheduledTasks, input.CurrentScheduledTaskID)
	dayStart := time.Date(input.Now.UTC().Year(), input.Now.UTC().Month(), input.Now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	dailyTasks, dailyExternalCalls, dailyCapabilityCalls := countAgentDailyQuotaUsage(input.Plans, input.ScheduledTasks, dayStart)
	active := activePlans + activeScheduled
	queued := queuedPlans + queuedScheduled
	status := "allowed"
	reason := "within user agent task policy"
	nextAction := "继续执行任务"
	allowed := true
	if active >= preference.MaxConcurrentTasks {
		allowed = false
		status = "throttled"
		reason = fmt.Sprintf("active agent tasks %d reached max concurrent limit %d", active, preference.MaxConcurrentTasks)
		nextAction = "等待运行中的 Agent 任务完成或调整最大并发数"
	} else if queued >= preference.MaxQueuedTasks {
		allowed = false
		status = "queue_full"
		reason = fmt.Sprintf("queued agent tasks %d reached max queued limit %d", queued, preference.MaxQueuedTasks)
		nextAction = "清理排队任务、等待调度消费或调整最大排队数"
	} else if dailyTasks >= preference.DailyTaskQuota {
		allowed = false
		status = "quota_exceeded"
		reason = fmt.Sprintf("daily agent tasks %d reached quota %d", dailyTasks, preference.DailyTaskQuota)
		nextAction = "等待明日配额刷新或调整每日任务配额"
	} else if dailyExternalCalls >= preference.DailyExternalCallQuota {
		allowed = false
		status = "quota_exceeded"
		reason = fmt.Sprintf("daily external calls %d reached quota %d", dailyExternalCalls, preference.DailyExternalCallQuota)
		nextAction = "降低联网能力使用、等待明日配额刷新或调整每日外部访问配额"
	} else if dailyCapabilityCalls >= preference.DailyCapabilityCallQuota {
		allowed = false
		status = "quota_exceeded"
		reason = fmt.Sprintf("daily capability calls %d reached quota %d", dailyCapabilityCalls, preference.DailyCapabilityCallQuota)
		nextAction = "减少 capability 调用、等待明日配额刷新或调整每日能力调用配额"
	}
	metadata := domain.AgentJSON{
		"user_id":                     input.UserID,
		"entry":                       strings.TrimSpace(input.Entry),
		"status":                      status,
		"reason":                      reason,
		"next_action":                 nextAction,
		"active_tasks":                active,
		"queued_tasks":                queued,
		"active_plans":                activePlans,
		"queued_plans":                queuedPlans,
		"active_scheduled_tasks":      activeScheduled,
		"queued_scheduled_tasks":      queuedScheduled,
		"max_concurrent_tasks":        preference.MaxConcurrentTasks,
		"max_queued_tasks":            preference.MaxQueuedTasks,
		"daily_tasks":                 dailyTasks,
		"daily_external_calls":        dailyExternalCalls,
		"daily_capability_calls":      dailyCapabilityCalls,
		"daily_task_quota":            preference.DailyTaskQuota,
		"daily_external_call_quota":   preference.DailyExternalCallQuota,
		"daily_capability_call_quota": preference.DailyCapabilityCallQuota,
		"auto_recovery_enabled":       preference.AutoRecoveryEnabled,
		"quality_handoff_threshold":   preference.QualityHandoffThreshold,
		"handoff_on_failure":          preference.HandoffOnFailure,
		"handoff_on_permission":       preference.HandoffOnPermission,
		"handoff_on_budget":           preference.HandoffOnBudget,
		"recorded_at":                 input.Now.UTC().Format(time.RFC3339),
	}
	return agentTaskAdmissionDecision{Allowed: allowed, Status: status, Reason: reason, NextAction: nextAction, Metadata: metadata}
}

func countAgentPlansForAdmission(plans []domain.AgentPlan) (int, int) {
	active := 0
	queued := 0
	for _, plan := range plans {
		switch plan.Status {
		case domain.AgentPlanStatusExecuting:
			active++
		case domain.AgentPlanStatusAwaitingApproval, domain.AgentPlanStatusApproved:
			queued++
		}
	}
	return active, queued
}

func countAgentScheduledTasksForAdmission(tasks []domain.AgentScheduledTask, currentTaskID int64) (int, int) {
	active := 0
	queued := 0
	for _, task := range tasks {
		if currentTaskID > 0 && task.ID == currentTaskID {
			continue
		}
		switch task.Status {
		case domain.AgentScheduledTaskStatusRunning:
			active++
		case domain.AgentScheduledTaskStatusQueued, domain.AgentScheduledTaskStatusInputRequired:
			queued++
		}
	}
	return active, queued
}

func countAgentDailyQuotaUsage(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, dayStart time.Time) (int, int, int) {
	taskCount := 0
	externalCalls := 0
	capabilityCalls := 0
	for _, plan := range plans {
		if plan.CreatedAt.IsZero() || plan.CreatedAt.UTC().Before(dayStart) {
			continue
		}
		taskCount++
		for _, step := range plan.Steps {
			if strings.TrimSpace(step.CapabilityKey) == "" {
				continue
			}
			capabilityCalls++
			if agentCapabilityIsExternal(step.CapabilityKey) {
				externalCalls++
			}
		}
	}
	for _, task := range tasks {
		if task.CreatedAt.IsZero() || task.CreatedAt.UTC().Before(dayStart) {
			continue
		}
		taskCount++
		for _, key := range task.AllowedCapabilities {
			if strings.TrimSpace(key) == "" {
				continue
			}
			capabilityCalls++
			if agentCapabilityIsExternal(key) {
				externalCalls++
			}
		}
	}
	return taskCount, externalCalls, capabilityCalls
}

func agentCapabilityIsExternal(key string) bool {
	key = strings.TrimSpace(key)
	return strings.HasPrefix(key, "web.") || strings.HasPrefix(key, "repo.")
}

func planResultQualitySummary(plan domain.AgentPlan) string {
	quality := metadataMap(plan.Metadata, "result_quality")
	if quality == nil {
		return "暂无结果质量评分"
	}
	score := metadataFloat(quality, "score")
	status := metadataString(quality, "status")
	if status == "" {
		status = "unknown"
	}
	return fmt.Sprintf("%s，评分 %.2f，证据完整性 %.2f，目标覆盖 %.2f", status, score, metadataFloat(quality, "evidence_completeness"), metadataFloat(quality, "goal_coverage"))
}

func planCostSummary(plan domain.AgentPlan) string {
	cost := metadataMap(plan.Metadata, "cost_summary")
	if cost == nil {
		return "暂无成本摘要"
	}
	return fmt.Sprintf("工具 %d，外部访问 %d，估算 token %d，重试 %d，通知 %d",
		metadataNumber(cost, "tool_calls"),
		metadataNumber(cost, "external_calls"),
		metadataNumber(cost, "estimated_tokens"),
		metadataNumber(cost, "retry_count"),
		metadataNumber(cost, "notification_count"),
	)
}

func buildAgentFeedbackSLAReport(sla AgentFeedbackTicketSLAResponse) AgentFeedbackSLAReportResponse {
	firstResponseRate := 1.0
	resolveRate := 1.0
	timeoutCount := 0
	waitingUserCount := 0
	if sla.WaitingUserStatus != "" && sla.WaitingUserStatus != "not_required" {
		waitingUserCount = 1
	}
	handoffCount := 0
	if strings.TrimSpace(sla.HandoffPath) != "" {
		handoffCount = 1
	}
	reportAuditEvent := "agent.feedback_sla_report_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "first_response_rate", Status: readyIf(firstResponseRate >= 0), Summary: fmt.Sprintf("%.2f", firstResponseRate)},
		{Key: "resolve_rate", Status: readyIf(resolveRate >= 0), Summary: fmt.Sprintf("%.2f", resolveRate)},
		{Key: "timeout_count", Status: readyIf(timeoutCount >= 0), Summary: strconv.Itoa(timeoutCount)},
		{Key: "waiting_user_count", Status: readyIf(waitingUserCount >= 0), Summary: strconv.Itoa(waitingUserCount)},
		{Key: "handoff_count", Status: readyIf(handoffCount >= 0), Summary: strconv.Itoa(handoffCount)},
		{Key: "audit_event", Status: readyIf(reportAuditEvent != ""), Summary: reportAuditEvent},
	}
	return AgentFeedbackSLAReportResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("feedback sla report first response %.2f resolve %.2f", firstResponseRate, resolveRate),
		FirstResponseRate: firstResponseRate,
		ResolveRate:       resolveRate,
		TimeoutCount:      timeoutCount,
		WaitingUserCount:  waitingUserCount,
		HandoffCount:      handoffCount,
		ReportAuditEvent:  reportAuditEvent,
		Checks:            checks,
	}
}

func buildAgentAlertAutoRecovery(receipt AgentAlertEscalationReceiptResponse) AgentAlertAutoRecoveryResponse {
	recoveryTrigger := "alert_status_returns_ready"
	recoveryNotice := receipt.RecoveryNoticeStatus
	if strings.TrimSpace(recoveryNotice) == "" {
		recoveryNotice = "pending"
	}
	suppressionRelease := "release_after_" + receipt.SuppressionResult
	reopenCondition := "same_alert_repeats_after_recovery"
	handoffState := receipt.HandoffEntry
	if strings.TrimSpace(handoffState) == "" {
		handoffState = "agent_operations_oncall"
	}
	auditEvidence := "agent.alert_auto_recovery_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "recovery_trigger", Status: readyIf(strings.TrimSpace(recoveryTrigger) != ""), Summary: recoveryTrigger},
		{Key: "recovery_notice", Status: readyIf(strings.TrimSpace(recoveryNotice) != ""), Summary: recoveryNotice},
		{Key: "suppression_release", Status: readyIf(strings.TrimSpace(suppressionRelease) != ""), Summary: suppressionRelease},
		{Key: "reopen_condition", Status: readyIf(strings.TrimSpace(reopenCondition) != ""), Summary: reopenCondition},
		{Key: "handoff", Status: readyIf(strings.TrimSpace(handoffState) != ""), Summary: handoffState},
		{Key: "audit_evidence", Status: readyIf(auditEvidence != ""), Summary: auditEvidence},
	}
	return AgentAlertAutoRecoveryResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("alert auto recovery triggers on %s", recoveryTrigger),
		RecoveryTrigger:    recoveryTrigger,
		RecoveryNotice:     recoveryNotice,
		SuppressionRelease: suppressionRelease,
		ReopenCondition:    reopenCondition,
		HandoffState:       handoffState,
		AuditEvidence:      auditEvidence,
		Checks:             checks,
	}
}

func buildAgentOperationsEvidence(record AgentOpsExecutionRecordResponse, callback AgentWeChatApprovalCallbackResponse, report AgentFeedbackSLAReportResponse, recovery AgentAlertAutoRecoveryResponse, audits []domain.AgentAuditLog) AgentOperationsEvidenceResponse {
	auditStatus := readyIf(auditEventContains(audits, "ops") || auditEventContains(audits, "approval") || auditEventContains(audits, "sla") || auditEventContains(audits, "alert") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "execution_record", Status: record.Status, Summary: record.Summary},
		{Key: "approval_callback", Status: callback.Status, Summary: callback.Summary},
		{Key: "sla_report", Status: report.Status, Summary: report.Summary},
		{Key: "auto_recovery", Status: recovery.Status, Summary: recovery.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations evidence is audit-backed"},
	}
	nextAction := "进入 Web/企业微信统一进度组件、证据明细页和回调重放工具"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运营证据闭环缺口后再进入统一进度组件"
	}
	return AgentOperationsEvidenceResponse{
		Status:                 checksStatus(checks),
		Summary:                fmt.Sprintf("operations evidence has %d checks", len(checks)),
		ExecutionRecordStatus:  record.Status,
		ApprovalCallbackStatus: callback.Status,
		SLAReportStatus:        report.Status,
		AutoRecoveryStatus:     recovery.Status,
		AuditStatus:            auditStatus,
		NextAction:             nextAction,
		Checks:                 checks,
	}
}

func buildAgentUnifiedProgressComponent(tasks []AgentTaskSummaryResponse, evidence AgentOperationsEvidenceResponse) AgentUnifiedProgressComponentResponse {
	componentKey := "agent.unified_progress"
	webStatus := readyIf(len(tasks) > 0 || evidence.Status != "")
	wechatStatus := readyIf(evidence.Status != "")
	eventCursor := "agent-progress-latest"
	refreshStrategy := "sse_with_polling_fallback"
	auditEvidence := "agent.unified_progress_component_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "component_key", Status: readyIf(componentKey != ""), Summary: componentKey},
		{Key: "web_status", Status: webStatus, Summary: "web task workbench uses unified progress component"},
		{Key: "wechat_status", Status: wechatStatus, Summary: "wechat progress card uses unified progress fields"},
		{Key: "event_cursor", Status: readyIf(eventCursor != ""), Summary: eventCursor},
		{Key: "refresh_strategy", Status: readyIf(refreshStrategy != ""), Summary: refreshStrategy},
		{Key: "audit_evidence", Status: readyIf(auditEvidence != ""), Summary: auditEvidence},
	}
	return AgentUnifiedProgressComponentResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("unified progress component covers web and wechat with %d tasks", len(tasks)),
		ComponentKey:    componentKey,
		WebStatus:       webStatus,
		WeChatStatus:    wechatStatus,
		EventCursor:     eventCursor,
		RefreshStrategy: refreshStrategy,
		AuditEvidence:   auditEvidence,
		Checks:          checks,
	}
}

func buildAgentEvidenceDetailPage(record AgentOpsExecutionRecordResponse, evidence AgentOperationsEvidenceResponse) AgentEvidenceDetailPageResponse {
	detailEntry := "web.agent.evidence.detail"
	replayEntry := "web.agent.evidence.replay"
	if len(record.Records) > 0 && strings.TrimSpace(record.Records[0].ReplayEntry) != "" {
		replayEntry = record.Records[0].ReplayEntry
	}
	visibility := "task_owner_and_ops"
	retentionPolicy := "retain_90_days"
	auditEvent := "agent.evidence_detail_page_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "detail_entry", Status: readyIf(detailEntry != ""), Summary: detailEntry},
		{Key: "record_count", Status: readyIf(len(record.Records) > 0), Summary: strconv.Itoa(len(record.Records))},
		{Key: "audit_event", Status: readyIf(auditEvent != ""), Summary: auditEvent},
		{Key: "replay_entry", Status: readyIf(replayEntry != ""), Summary: replayEntry},
		{Key: "visibility", Status: readyIf(visibility != ""), Summary: visibility},
		{Key: "retention_policy", Status: readyIf(retentionPolicy != ""), Summary: retentionPolicy},
		{Key: "operations_evidence", Status: evidence.Status, Summary: evidence.Summary},
	}
	return AgentEvidenceDetailPageResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("evidence detail page exposes %d records", len(record.Records)),
		DetailEntry:     detailEntry,
		RecordCount:     len(record.Records),
		AuditEvent:      auditEvent,
		ReplayEntry:     replayEntry,
		Visibility:      visibility,
		RetentionPolicy: retentionPolicy,
		Checks:          checks,
	}
}

func buildAgentCallbackReplayTool(callback AgentWeChatApprovalCallbackResponse) AgentCallbackReplayToolResponse {
	replayEntry := "web.agent.callback.replay." + callback.CallbackKey
	signatureReview := callback.Signature
	if strings.TrimSpace(signatureReview) == "" {
		signatureReview = "required"
	}
	idempotencyGuard := "callback_key_and_decision"
	failureFallback := callback.FallbackPath
	if strings.TrimSpace(failureFallback) == "" {
		failureFallback = "manual_review"
	}
	auditEvidence := "agent.callback_replay_tool_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "callback_key", Status: readyIf(callback.CallbackKey != ""), Summary: callback.CallbackKey},
		{Key: "replay_entry", Status: readyIf(replayEntry != ""), Summary: replayEntry},
		{Key: "signature_review", Status: readyIf(signatureReview != ""), Summary: signatureReview},
		{Key: "idempotency_guard", Status: readyIf(idempotencyGuard != ""), Summary: idempotencyGuard},
		{Key: "failure_fallback", Status: readyIf(failureFallback != ""), Summary: failureFallback},
		{Key: "audit_evidence", Status: readyIf(auditEvidence != ""), Summary: auditEvidence},
	}
	return AgentCallbackReplayToolResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("callback replay tool handles %s", callback.CallbackKey),
		CallbackKey:      callback.CallbackKey,
		ReplayEntry:      replayEntry,
		SignatureReview:  signatureReview,
		IdempotencyGuard: idempotencyGuard,
		FailureFallback:  failureFallback,
		AuditEvidence:    auditEvidence,
		Checks:           checks,
	}
}

func buildAgentRecoveryPolicyConfig(recovery AgentAlertAutoRecoveryResponse) AgentRecoveryPolicyConfigResponse {
	policyKey := "agent.alert.recovery.default"
	suppressionWindow := "300s"
	defaultPolicy := "conservative_manual_handoff_on_uncertainty"
	checks := []AgentDeploymentCheckResponse{
		{Key: "policy_key", Status: readyIf(policyKey != ""), Summary: policyKey},
		{Key: "recovery_trigger", Status: readyIf(recovery.RecoveryTrigger != ""), Summary: recovery.RecoveryTrigger},
		{Key: "suppression_window", Status: readyIf(suppressionWindow != ""), Summary: suppressionWindow},
		{Key: "reopen_condition", Status: readyIf(recovery.ReopenCondition != ""), Summary: recovery.ReopenCondition},
		{Key: "handoff_state", Status: readyIf(recovery.HandoffState != ""), Summary: recovery.HandoffState},
		{Key: "default_policy", Status: readyIf(defaultPolicy != ""), Summary: defaultPolicy},
	}
	return AgentRecoveryPolicyConfigResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("recovery policy %s uses %s trigger", policyKey, recovery.RecoveryTrigger),
		PolicyKey:         policyKey,
		RecoveryTrigger:   recovery.RecoveryTrigger,
		SuppressionWindow: suppressionWindow,
		ReopenCondition:   recovery.ReopenCondition,
		HandoffState:      recovery.HandoffState,
		DefaultPolicy:     defaultPolicy,
		Checks:            checks,
	}
}

func buildAgentDualEndProgressEvidence(progress AgentUnifiedProgressComponentResponse, detail AgentEvidenceDetailPageResponse, replay AgentCallbackReplayToolResponse, recovery AgentRecoveryPolicyConfigResponse, audits []domain.AgentAuditLog) AgentDualEndProgressEvidenceResponse {
	auditStatus := readyIf(auditEventContains(audits, "progress") || auditEventContains(audits, "evidence") || auditEventContains(audits, "callback") || auditEventContains(audits, "recovery") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "unified_progress", Status: progress.Status, Summary: progress.Summary},
		{Key: "evidence_detail", Status: detail.Status, Summary: detail.Summary},
		{Key: "callback_replay", Status: replay.Status, Summary: replay.Summary},
		{Key: "recovery_policy", Status: recovery.Status, Summary: recovery.Summary},
		{Key: "audit", Status: auditStatus, Summary: "dual-end progress evidence is audit-backed"},
	}
	nextAction := "进入企业微信可视化进度卡片细化、Web 证据明细交互和权限控制"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐双端进度证据缺口后再进入交互细化"
	}
	return AgentDualEndProgressEvidenceResponse{
		Status:                checksStatus(checks),
		Summary:               fmt.Sprintf("dual-end progress evidence has %d checks", len(checks)),
		UnifiedProgressStatus: progress.Status,
		EvidenceDetailStatus:  detail.Status,
		CallbackReplayStatus:  replay.Status,
		RecoveryPolicyStatus:  recovery.Status,
		AuditStatus:           auditStatus,
		NextAction:            nextAction,
		Checks:                checks,
	}
}

func buildAgentWeChatProgressCard(progress AgentUnifiedProgressComponentResponse, detail AgentEvidenceDetailPageResponse) AgentWeChatProgressCardResponse {
	phaseStatus := progress.WeChatStatus
	progressPercent := 100
	if progress.Status != "ready" {
		progressPercent = 60
	}
	actions := []AgentButtonCallbackActionResponse{
		{Key: "open_detail", Label: "查看详情", Handler: "agent.wechat.progress.open_detail", URL: detail.DetailEntry, Fallback: "查看 Web 任务详情", Status: readyIf(detail.DetailEntry != "")},
		{Key: "refresh_progress", Label: "刷新进度", Handler: "agent.wechat.progress.refresh", URL: progress.EventCursor, Fallback: "刷新任务进度", Status: readyIf(progress.EventCursor != "")},
	}
	fallbackText := "任务进度已更新，可在 Web 查看详细证据"
	checks := []AgentDeploymentCheckResponse{
		{Key: "card_key", Status: readyIf(progress.ComponentKey != ""), Summary: progress.ComponentKey},
		{Key: "phase_status", Status: readyIf(phaseStatus != ""), Summary: phaseStatus},
		{Key: "progress_percent", Status: readyIf(progressPercent >= 0 && progressPercent <= 100), Summary: strconv.Itoa(progressPercent)},
		{Key: "detail_entry", Status: readyIf(detail.DetailEntry != ""), Summary: detail.DetailEntry},
		{Key: "actions", Status: readyIf(len(actions) > 0), Summary: fmt.Sprintf("%d actions", len(actions))},
		{Key: "fallback_text", Status: readyIf(fallbackText != ""), Summary: fallbackText},
	}
	return AgentWeChatProgressCardResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("wechat progress card %s at %d%%", progress.ComponentKey, progressPercent),
		CardKey:         "wechat." + progress.ComponentKey,
		PhaseStatus:     phaseStatus,
		ProgressPercent: progressPercent,
		DetailEntry:     detail.DetailEntry,
		Actions:         actions,
		FallbackText:    fallbackText,
		Checks:          checks,
	}
}

func buildAgentWebEvidenceInteraction(detail AgentEvidenceDetailPageResponse) AgentWebEvidenceInteractionResponse {
	filters := []string{"status", "action_key", "audit_event", "created_at"}
	expandable := "record_detail_and_audit_payload"
	auditDisplay := "inline_audit_timeline"
	retentionHint := detail.RetentionPolicy
	visibility := detail.Visibility
	checks := []AgentDeploymentCheckResponse{
		{Key: "filters", Status: readyIf(len(filters) > 0), Summary: strings.Join(filters, ", ")},
		{Key: "expandable", Status: readyIf(expandable != ""), Summary: expandable},
		{Key: "replay_entry", Status: readyIf(detail.ReplayEntry != ""), Summary: detail.ReplayEntry},
		{Key: "audit_display", Status: readyIf(auditDisplay != ""), Summary: auditDisplay},
		{Key: "retention_hint", Status: readyIf(retentionHint != ""), Summary: retentionHint},
		{Key: "visibility", Status: readyIf(visibility != ""), Summary: visibility},
	}
	return AgentWebEvidenceInteractionResponse{
		Status:        checksStatus(checks),
		Summary:       fmt.Sprintf("web evidence interaction exposes %d filters", len(filters)),
		Filters:       filters,
		Expandable:    expandable,
		ReplayEntry:   detail.ReplayEntry,
		AuditDisplay:  auditDisplay,
		RetentionHint: retentionHint,
		Visibility:    visibility,
		Checks:        checks,
	}
}

func buildAgentCallbackReplayPermission(tool AgentCallbackReplayToolResponse) AgentCallbackReplayPermissionResponse {
	permissionKey := "agent.callback.replay"
	allowedRoles := []string{"task_owner", "agent_operations"}
	auditEvent := "agent.callback_replay_permission_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "permission_key", Status: readyIf(permissionKey != ""), Summary: permissionKey},
		{Key: "allowed_roles", Status: readyIf(len(allowedRoles) > 0), Summary: strings.Join(allowedRoles, ", ")},
		{Key: "idempotency_guard", Status: readyIf(tool.IdempotencyGuard != ""), Summary: tool.IdempotencyGuard},
		{Key: "signature_review", Status: readyIf(tool.SignatureReview != ""), Summary: tool.SignatureReview},
		{Key: "failure_fallback", Status: readyIf(tool.FailureFallback != ""), Summary: tool.FailureFallback},
		{Key: "audit_event", Status: readyIf(auditEvent != ""), Summary: auditEvent},
	}
	return AgentCallbackReplayPermissionResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("callback replay permission %s allows %d roles", permissionKey, len(allowedRoles)),
		PermissionKey:    permissionKey,
		AllowedRoles:     allowedRoles,
		IdempotencyGuard: tool.IdempotencyGuard,
		SignatureReview:  tool.SignatureReview,
		FailureFallback:  tool.FailureFallback,
		AuditEvent:       auditEvent,
		Checks:           checks,
	}
}

func buildAgentRecoveryPolicyAudit(config AgentRecoveryPolicyConfigResponse) AgentRecoveryPolicyAuditResponse {
	changeKey := "recovery_policy_change:" + config.PolicyKey
	oldPolicy := "previous:" + config.DefaultPolicy
	newPolicy := "current:" + config.DefaultPolicy
	approvalStatus := "approval_required"
	rollbackPath := "restore_previous_recovery_policy"
	auditEvidence := "agent.recovery_policy_audit_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "change_key", Status: readyIf(changeKey != ""), Summary: changeKey},
		{Key: "old_policy", Status: readyIf(oldPolicy != ""), Summary: oldPolicy},
		{Key: "new_policy", Status: readyIf(newPolicy != ""), Summary: newPolicy},
		{Key: "approval_status", Status: readyIf(approvalStatus != ""), Summary: approvalStatus},
		{Key: "rollback_path", Status: readyIf(rollbackPath != ""), Summary: rollbackPath},
		{Key: "audit_evidence", Status: readyIf(auditEvidence != ""), Summary: auditEvidence},
	}
	return AgentRecoveryPolicyAuditResponse{
		Status:         checksStatus(checks),
		Summary:        fmt.Sprintf("recovery policy audit tracks %s", config.PolicyKey),
		ChangeKey:      changeKey,
		OldPolicy:      oldPolicy,
		NewPolicy:      newPolicy,
		ApprovalStatus: approvalStatus,
		RollbackPath:   rollbackPath,
		AuditEvidence:  auditEvidence,
		Checks:         checks,
	}
}

func buildAgentDualEndInteraction(card AgentWeChatProgressCardResponse, web AgentWebEvidenceInteractionResponse, permission AgentCallbackReplayPermissionResponse, audit AgentRecoveryPolicyAuditResponse, audits []domain.AgentAuditLog) AgentDualEndInteractionResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat") || auditEventContains(audits, "evidence") || auditEventContains(audits, "callback") || auditEventContains(audits, "recovery") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_progress_card", Status: card.Status, Summary: card.Summary},
		{Key: "web_evidence_interaction", Status: web.Status, Summary: web.Summary},
		{Key: "callback_replay_permission", Status: permission.Status, Summary: permission.Summary},
		{Key: "recovery_policy_audit", Status: audit.Status, Summary: audit.Summary},
		{Key: "audit", Status: auditStatus, Summary: "dual-end interaction governance is audit-backed"},
	}
	nextAction := "进入企业微信进度卡片真实模板渲染、Web 证据明细路由和权限审批流"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐双端交互治理缺口后再进入真实模板渲染"
	}
	return AgentDualEndInteractionResponse{
		Status:                    checksStatus(checks),
		Summary:                   fmt.Sprintf("dual-end interaction governance has %d checks", len(checks)),
		WeChatProgressCardStatus:  card.Status,
		WebEvidenceStatus:         web.Status,
		CallbackPermissionStatus:  permission.Status,
		RecoveryPolicyAuditStatus: audit.Status,
		AuditStatus:               auditStatus,
		NextAction:                nextAction,
		Checks:                    checks,
	}
}

func buildAgentWeChatTemplateRender(card AgentWeChatProgressCardResponse) AgentWeChatTemplateRenderResponse {
	templateKey := "agent.wechat.progress.template_card"
	if strings.TrimSpace(card.CardKey) != "" {
		templateKey = card.CardKey + ".template"
	}
	renderStatus := "template_ready"
	if card.Status != "ready" {
		renderStatus = "template_review"
	}
	phaseFields := []string{"card_key", "phase_status", "progress_percent", "detail_entry"}
	buttonFields := make([]string, 0, len(card.Actions))
	for _, action := range card.Actions {
		if strings.TrimSpace(action.Key) != "" {
			buttonFields = append(buttonFields, action.Key)
		}
	}
	sendEntry := "service.agent.send_wechat_work_progress_template"
	checks := []AgentDeploymentCheckResponse{
		{Key: "template_key", Status: readyIf(templateKey != ""), Summary: templateKey},
		{Key: "render_status", Status: readyIf(renderStatus != ""), Summary: renderStatus},
		{Key: "phase_fields", Status: readyIf(len(phaseFields) > 0), Summary: strings.Join(phaseFields, ", ")},
		{Key: "button_fields", Status: readyIf(len(buttonFields) > 0), Summary: strings.Join(buttonFields, ", ")},
		{Key: "fallback_text", Status: readyIf(strings.TrimSpace(card.FallbackText) != ""), Summary: card.FallbackText},
		{Key: "send_entry", Status: readyIf(sendEntry != ""), Summary: sendEntry},
	}
	return AgentWeChatTemplateRenderResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("wechat template render %s with %d button fields", templateKey, len(buttonFields)),
		TemplateKey:  templateKey,
		RenderStatus: renderStatus,
		PhaseFields:  phaseFields,
		ButtonFields: buttonFields,
		FallbackText: card.FallbackText,
		SendEntry:    sendEntry,
		Checks:       checks,
	}
}

func buildAgentWebEvidenceRoute(interaction AgentWebEvidenceInteractionResponse) AgentWebEvidenceRouteResponse {
	routeName := "agent-evidence-detail"
	pathParams := []string{"plan_id", "record_key"}
	filterParams := append([]string(nil), interaction.Filters...)
	if len(filterParams) == 0 {
		filterParams = []string{"status", "audit_event"}
	}
	permissionRequirement := "task_owner_or_agent_operations"
	checks := []AgentDeploymentCheckResponse{
		{Key: "route_name", Status: readyIf(routeName != ""), Summary: routeName},
		{Key: "path_params", Status: readyIf(len(pathParams) > 0), Summary: strings.Join(pathParams, ", ")},
		{Key: "filter_params", Status: readyIf(len(filterParams) > 0), Summary: strings.Join(filterParams, ", ")},
		{Key: "permission_requirement", Status: readyIf(permissionRequirement != ""), Summary: permissionRequirement},
		{Key: "replay_entry", Status: readyIf(interaction.ReplayEntry != ""), Summary: interaction.ReplayEntry},
		{Key: "audit_display", Status: readyIf(interaction.AuditDisplay != ""), Summary: interaction.AuditDisplay},
	}
	return AgentWebEvidenceRouteResponse{
		Status:                checksStatus(checks),
		Summary:               fmt.Sprintf("web evidence route %s has %d filters", routeName, len(filterParams)),
		RouteName:             routeName,
		PathParams:            pathParams,
		FilterParams:          filterParams,
		PermissionRequirement: permissionRequirement,
		ReplayEntry:           interaction.ReplayEntry,
		AuditDisplay:          interaction.AuditDisplay,
		Checks:                checks,
	}
}

func buildAgentCallbackReplayApproval(permission AgentCallbackReplayPermissionResponse) AgentCallbackReplayApprovalResponse {
	approvalKey := permission.PermissionKey + ".approval"
	if strings.TrimSpace(permission.PermissionKey) == "" {
		approvalKey = "agent.callback.replay.approval"
	}
	requestEntry := "web.agent.callback.replay.request_approval"
	approvalStatus := "approval_required"
	executionGate := "approved_and_idempotency_verified"
	checks := []AgentDeploymentCheckResponse{
		{Key: "approval_key", Status: readyIf(approvalKey != ""), Summary: approvalKey},
		{Key: "request_entry", Status: readyIf(requestEntry != ""), Summary: requestEntry},
		{Key: "approval_roles", Status: readyIf(len(permission.AllowedRoles) > 0), Summary: strings.Join(permission.AllowedRoles, ", ")},
		{Key: "approval_status", Status: readyIf(approvalStatus != ""), Summary: approvalStatus},
		{Key: "execution_gate", Status: readyIf(executionGate != ""), Summary: executionGate},
		{Key: "audit_event", Status: readyIf(permission.AuditEvent != ""), Summary: permission.AuditEvent},
	}
	return AgentCallbackReplayApprovalResponse{
		Status:         checksStatus(checks),
		Summary:        fmt.Sprintf("callback replay approval %s gates execution", approvalKey),
		ApprovalKey:    approvalKey,
		RequestEntry:   requestEntry,
		ApprovalRoles:  append([]string(nil), permission.AllowedRoles...),
		ApprovalStatus: approvalStatus,
		ExecutionGate:  executionGate,
		AuditEvent:     permission.AuditEvent,
		Checks:         checks,
	}
}

func buildAgentRecoveryPolicyPersist(audit AgentRecoveryPolicyAuditResponse) AgentRecoveryPolicyPersistResponse {
	configKey := strings.TrimPrefix(audit.ChangeKey, "recovery_policy_change:")
	if strings.TrimSpace(configKey) == "" || configKey == audit.ChangeKey {
		configKey = "agent.alert.recovery.default"
	}
	currentVersion := "current:v1"
	if strings.TrimSpace(audit.NewPolicy) != "" {
		currentVersion = audit.NewPolicy
	}
	pendingVersion := "pending:v2"
	persistenceStatus := "persisted_with_pending_review"
	rollbackVersion := "rollback:v1"
	if strings.TrimSpace(audit.OldPolicy) != "" {
		rollbackVersion = audit.OldPolicy
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "config_key", Status: readyIf(configKey != ""), Summary: configKey},
		{Key: "current_version", Status: readyIf(currentVersion != ""), Summary: currentVersion},
		{Key: "pending_version", Status: readyIf(pendingVersion != ""), Summary: pendingVersion},
		{Key: "persistence_status", Status: readyIf(persistenceStatus != ""), Summary: persistenceStatus},
		{Key: "rollback_version", Status: readyIf(rollbackVersion != ""), Summary: rollbackVersion},
		{Key: "audit_evidence", Status: readyIf(audit.AuditEvidence != ""), Summary: audit.AuditEvidence},
	}
	return AgentRecoveryPolicyPersistResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("recovery policy config %s persistence %s", configKey, persistenceStatus),
		ConfigKey:         configKey,
		CurrentVersion:    currentVersion,
		PendingVersion:    pendingVersion,
		PersistenceStatus: persistenceStatus,
		RollbackVersion:   rollbackVersion,
		AuditEvidence:     audit.AuditEvidence,
		Checks:            checks,
	}
}

func buildAgentDualEndInteractionLaunch(render AgentWeChatTemplateRenderResponse, route AgentWebEvidenceRouteResponse, approval AgentCallbackReplayApprovalResponse, persist AgentRecoveryPolicyPersistResponse, audits []domain.AgentAuditLog) AgentDualEndInteractionLaunchResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat_template") || auditEventContains(audits, "web_evidence_route") || auditEventContains(audits, "callback_replay_approval") || auditEventContains(audits, "recovery_policy_persist") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_template_render", Status: render.Status, Summary: render.Summary},
		{Key: "web_evidence_route", Status: route.Status, Summary: route.Summary},
		{Key: "callback_replay_approval", Status: approval.Status, Summary: approval.Summary},
		{Key: "recovery_policy_persistence", Status: persist.Status, Summary: persist.Summary},
		{Key: "audit", Status: auditStatus, Summary: "dual-end interaction launch snapshots are audit-backed"},
	}
	nextAction := "进入企业微信模板真实发送、Web 证据路由页面、回放审批执行 API 和恢复策略版本管理"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐双端交互落地缺口后再进入真实执行 API"
	}
	return AgentDualEndInteractionLaunchResponse{
		Status:                          checksStatus(checks),
		Summary:                         fmt.Sprintf("dual-end interaction launch has %d checks", len(checks)),
		WeChatTemplateRenderStatus:      render.Status,
		WebEvidenceRouteStatus:          route.Status,
		CallbackReplayApprovalStatus:    approval.Status,
		RecoveryPolicyPersistenceStatus: persist.Status,
		AuditStatus:                     auditStatus,
		NextAction:                      nextAction,
		Checks:                          checks,
	}
}

func buildAgentWeChatTemplateSend(render AgentWeChatTemplateRenderResponse) AgentWeChatTemplateSendResponse {
	messageType := "template_card"
	title := "Agent 实时工作进度"
	sendResult := "adapter_ready_fallback_retained"
	auditEvent := "agent.wechat_template_send_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "message_type", Status: readyIf(messageType == "template_card"), Summary: messageType},
		{Key: "title", Status: readyIf(title != ""), Summary: title},
		{Key: "phase_fields", Status: readyIf(len(render.PhaseFields) > 0), Summary: strings.Join(render.PhaseFields, ", ")},
		{Key: "button_fields", Status: readyIf(len(render.ButtonFields) > 0), Summary: strings.Join(render.ButtonFields, ", ")},
		{Key: "fallback_text", Status: readyIf(strings.TrimSpace(render.FallbackText) != ""), Summary: render.FallbackText},
		{Key: "send_entry", Status: readyIf(strings.TrimSpace(render.SendEntry) != ""), Summary: render.SendEntry},
		{Key: "send_result", Status: readyIf(sendResult != ""), Summary: sendResult},
		{Key: "audit_event", Status: readyIf(auditEvent != ""), Summary: auditEvent},
	}
	return AgentWeChatTemplateSendResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("wechat template send adapter uses %s with %d buttons", messageType, len(render.ButtonFields)),
		MessageType:  messageType,
		Title:        title,
		PhaseFields:  append([]string(nil), render.PhaseFields...),
		ButtonFields: append([]string(nil), render.ButtonFields...),
		FallbackText: render.FallbackText,
		SendEntry:    render.SendEntry,
		SendResult:   sendResult,
		AuditEvent:   auditEvent,
		Checks:       checks,
	}
}

func buildAgentWebEvidenceDetailView(route AgentWebEvidenceRouteResponse) AgentWebEvidenceDetailViewResponse {
	routePath := "/agent/plans/:plan_id/evidence/:record_key"
	planParam := "plan_id"
	recordParam := "record_key"
	recordSource := "agent.ops_execution_record.records"
	auditEvents := []string{"agent.ops_execution_record_snapshot", "agent.web_evidence_route_snapshot"}
	permissionHint := route.PermissionRequirement
	if strings.TrimSpace(permissionHint) == "" {
		permissionHint = "task_owner_or_agent_operations"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "route_name", Status: readyIf(route.RouteName != ""), Summary: route.RouteName},
		{Key: "route_path", Status: readyIf(routePath != ""), Summary: routePath},
		{Key: "plan_param", Status: readyIf(planParam != ""), Summary: planParam},
		{Key: "record_param", Status: readyIf(recordParam != ""), Summary: recordParam},
		{Key: "record_source", Status: readyIf(recordSource != ""), Summary: recordSource},
		{Key: "filter_params", Status: readyIf(len(route.FilterParams) > 0), Summary: strings.Join(route.FilterParams, ", ")},
		{Key: "audit_events", Status: readyIf(len(auditEvents) > 0), Summary: strings.Join(auditEvents, ", ")},
		{Key: "replay_entry", Status: readyIf(route.ReplayEntry != ""), Summary: route.ReplayEntry},
		{Key: "permission_hint", Status: readyIf(permissionHint != ""), Summary: permissionHint},
	}
	return AgentWebEvidenceDetailViewResponse{
		Status:         checksStatus(checks),
		Summary:        fmt.Sprintf("web evidence detail view %s uses %d filters", route.RouteName, len(route.FilterParams)),
		RouteName:      route.RouteName,
		RoutePath:      routePath,
		PlanParam:      planParam,
		RecordParam:    recordParam,
		RecordSource:   recordSource,
		FilterParams:   append([]string(nil), route.FilterParams...),
		AuditEvents:    auditEvents,
		ReplayEntry:    route.ReplayEntry,
		PermissionHint: permissionHint,
		Checks:         checks,
	}
}

func buildAgentCallbackReplayExecution(approval AgentCallbackReplayApprovalResponse) AgentCallbackReplayExecutionResponse {
	requestEntry := "/api/v1/agent/callback-replay/requests"
	executeEntry := "/api/v1/agent/callback-replay/execute"
	approvalStatus := approval.ApprovalStatus
	if strings.TrimSpace(approvalStatus) == "" {
		approvalStatus = "approval_required"
	}
	executionGate := approval.ExecutionGate
	if strings.TrimSpace(executionGate) == "" {
		executionGate = "approved_and_idempotency_verified"
	}
	idempotencyKey := approval.ApprovalKey + ":callback_key:decision"
	if strings.TrimSpace(approval.ApprovalKey) == "" {
		idempotencyKey = "agent.callback.replay.approval:callback_key:decision"
	}
	auditEvent := "agent.callback_replay_execution_snapshot"
	failureFallback := "manual_review_without_replay"
	checks := []AgentDeploymentCheckResponse{
		{Key: "request_entry", Status: readyIf(requestEntry != ""), Summary: requestEntry},
		{Key: "execute_entry", Status: readyIf(executeEntry != ""), Summary: executeEntry},
		{Key: "approval_status", Status: readyIf(approvalStatus != ""), Summary: approvalStatus},
		{Key: "execution_gate", Status: readyIf(executionGate != ""), Summary: executionGate},
		{Key: "idempotency_key", Status: readyIf(idempotencyKey != ""), Summary: idempotencyKey},
		{Key: "audit_event", Status: readyIf(auditEvent != ""), Summary: auditEvent},
		{Key: "failure_fallback", Status: readyIf(failureFallback != ""), Summary: failureFallback},
	}
	return AgentCallbackReplayExecutionResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("callback replay execution is gated by %s", executionGate),
		RequestEntry:    requestEntry,
		ExecuteEntry:    executeEntry,
		ApprovalStatus:  approvalStatus,
		ExecutionGate:   executionGate,
		IdempotencyKey:  idempotencyKey,
		AuditEvent:      auditEvent,
		FailureFallback: failureFallback,
		Checks:          checks,
	}
}

func buildAgentRecoveryPolicyVersion(persist AgentRecoveryPolicyPersistResponse) AgentRecoveryPolicyVersionResponse {
	releaseStatus := "pending_review"
	configSource := "agent_task_governance_snapshot"
	auditEvent := "agent.recovery_policy_version_snapshot"
	checks := []AgentDeploymentCheckResponse{
		{Key: "policy_key", Status: readyIf(persist.ConfigKey != ""), Summary: persist.ConfigKey},
		{Key: "current_version", Status: readyIf(persist.CurrentVersion != ""), Summary: persist.CurrentVersion},
		{Key: "pending_version", Status: readyIf(persist.PendingVersion != ""), Summary: persist.PendingVersion},
		{Key: "rollback_version", Status: readyIf(persist.RollbackVersion != ""), Summary: persist.RollbackVersion},
		{Key: "release_status", Status: readyIf(releaseStatus != ""), Summary: releaseStatus},
		{Key: "config_source", Status: readyIf(configSource != ""), Summary: configSource},
		{Key: "audit_event", Status: readyIf(auditEvent != ""), Summary: auditEvent},
	}
	return AgentRecoveryPolicyVersionResponse{
		Status:          checksStatus(checks),
		Summary:         fmt.Sprintf("recovery policy %s version state %s", persist.ConfigKey, releaseStatus),
		PolicyKey:       persist.ConfigKey,
		CurrentVersion:  persist.CurrentVersion,
		PendingVersion:  persist.PendingVersion,
		RollbackVersion: persist.RollbackVersion,
		ReleaseStatus:   releaseStatus,
		ConfigSource:    configSource,
		AuditEvent:      auditEvent,
		Checks:          checks,
	}
}

func buildAgentDualEndRealInteraction(send AgentWeChatTemplateSendResponse, detail AgentWebEvidenceDetailViewResponse, replay AgentCallbackReplayExecutionResponse, version AgentRecoveryPolicyVersionResponse, audits []domain.AgentAuditLog) AgentDualEndRealInteractionResponse {
	auditStatus := readyIf(auditEventContains(audits, "wechat_template_send") || auditEventContains(audits, "web_evidence_detail") || auditEventContains(audits, "callback_replay_execution") || auditEventContains(audits, "recovery_policy_version") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_template_send", Status: send.Status, Summary: send.Summary},
		{Key: "web_evidence_detail", Status: detail.Status, Summary: detail.Summary},
		{Key: "callback_replay_execution", Status: replay.Status, Summary: replay.Summary},
		{Key: "recovery_policy_version", Status: version.Status, Summary: version.Summary},
		{Key: "audit", Status: auditStatus, Summary: "real dual-end interaction is audit-backed"},
	}
	nextAction := "进入企业微信模板发送联调、证据页面交互细化和恢复策略灰度发布"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐真实交互缺口后再进入联调和灰度"
	}
	return AgentDualEndRealInteractionResponse{
		Status:                        checksStatus(checks),
		Summary:                       fmt.Sprintf("dual-end real interaction has %d checks", len(checks)),
		WeChatTemplateSendStatus:      send.Status,
		WebEvidenceDetailStatus:       detail.Status,
		CallbackReplayExecutionStatus: replay.Status,
		RecoveryPolicyVersionStatus:   version.Status,
		AuditStatus:                   auditStatus,
		NextAction:                    nextAction,
		Checks:                        checks,
	}
}

func allOpsActionsHave(actions []AgentOpsActionItemResponse, predicate func(AgentOpsActionItemResponse) bool) bool {
	if len(actions) == 0 {
		return false
	}
	for _, action := range actions {
		if !predicate(action) {
			return false
		}
	}
	return true
}

func allOpsAPIExecutionsHave(executions []AgentOpsAPIExecutionItemResponse, predicate func(AgentOpsAPIExecutionItemResponse) bool) bool {
	if len(executions) == 0 {
		return false
	}
	for _, execution := range executions {
		if !predicate(execution) {
			return false
		}
	}
	return true
}

func allWriteApprovalButtonsHave(buttons []AgentWriteApprovalButtonItemResponse, predicate func(AgentWriteApprovalButtonItemResponse) bool) bool {
	if len(buttons) == 0 {
		return false
	}
	for _, button := range buttons {
		if !predicate(button) {
			return false
		}
	}
	return true
}

func allOpsExecutionRecordsHave(records []AgentOpsExecutionRecordItemResponse, predicate func(AgentOpsExecutionRecordItemResponse) bool) bool {
	if len(records) == 0 {
		return false
	}
	for _, record := range records {
		if !predicate(record) {
			return false
		}
	}
	return true
}

func alertChannelChecks(channels []AgentAlertChannelTargetResponse) []AgentDeploymentCheckResponse {
	checks := make([]AgentDeploymentCheckResponse, 0, len(channels))
	for _, channel := range channels {
		checks = append(checks, AgentDeploymentCheckResponse{Key: channel.Key, Status: channel.Status, Summary: channel.Fallback})
	}
	return checks
}

func agentButtonCallbackHandler(key string) string {
	switch strings.TrimSpace(key) {
	case "view_progress":
		return "agent.progress.view"
	case "approval":
		return "agent.approval.decide"
	case "retry_plan":
		return "agent.plan.retry"
	case "recover_plan":
		return "agent.plan.recover"
	case "cancel_scheduled_task":
		return "agent.scheduled_task.cancel"
	case "view_final_report":
		return "agent.report.view_final"
	default:
		return ""
	}
}

func buttonCallbackActionExists(actions []AgentButtonCallbackActionResponse, key string) bool {
	for _, action := range actions {
		if action.Key == key && action.Handler != "" {
			return true
		}
	}
	return false
}

func deploymentCheckStatus(checks []AgentDeploymentCheckResponse, key string) string {
	for _, check := range checks {
		if check.Key == key {
			if check.Status != "" {
				return check.Status
			}
			break
		}
	}
	return "review"
}

func deploymentCheckSummary(checks []AgentDeploymentCheckResponse, key string) string {
	for _, check := range checks {
		if check.Key == key {
			return check.Summary
		}
	}
	return key + " not represented"
}

func risksAndBlockersFromChecks(checks []AgentDeploymentCheckResponse) ([]string, []string) {
	risks := []string{}
	blockers := []string{}
	for _, check := range checks {
		if check.Status == "review" {
			risks = append(risks, check.Key)
		}
		if check.Status == "blocked" || check.Status == "failed" {
			blockers = append(blockers, check.Key)
		}
	}
	return uniqueStrings(risks), uniqueStrings(blockers)
}

func stringSliceContainsLocal(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	output := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		output = append(output, value)
	}
	sort.Strings(output)
	return output
}

func readyIf(condition bool) string {
	if condition {
		return "ready"
	}
	return "review"
}

func checksStatus(checks []AgentDeploymentCheckResponse) string {
	for _, check := range checks {
		if check.Status != "ready" && check.Status != "passed" {
			return "review"
		}
	}
	return "ready"
}

func auditEventExists(audits []domain.AgentAuditLog, eventType string) bool {
	for _, audit := range audits {
		if audit.EventType == eventType {
			return true
		}
	}
	return false
}

func auditEventContains(audits []domain.AgentAuditLog, fragment string) bool {
	for _, audit := range audits {
		if strings.Contains(audit.EventType, fragment) {
			return true
		}
	}
	return false
}

func deploymentCheckReady(checks []AgentDeploymentCheckResponse, key string) bool {
	for _, check := range checks {
		if check.Key == key && (check.Status == "ready" || check.Status == "passed") {
			return true
		}
	}
	return false
}

func planEntryExists(plans []domain.AgentPlan, entry string) bool {
	for _, plan := range plans {
		if metadataString(metadataMap(plan.Metadata, "admission_policy"), "entry") == entry {
			return true
		}
	}
	return false
}

func planStatusExists(plans []domain.AgentPlan, status domain.AgentPlanStatus) bool {
	for _, plan := range plans {
		if plan.Status == status {
			return true
		}
	}
	return false
}

func recoveryMetadataExists(plans []domain.AgentPlan) bool {
	for _, plan := range plans {
		if metadataMap(plan.Metadata, "recovery") != nil {
			return true
		}
	}
	return false
}

func agentCapabilityRequiresWriteSandbox(key string) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return false
	}
	writeHints := []string{".write", ".create", ".update", ".delete", ".send", ".publish", ".mutate", "repo.commit", "repo.push"}
	for _, hint := range writeHints {
		if strings.Contains(key, hint) {
			return true
		}
	}
	return false
}

func metadataRecordSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []domain.AgentJSON:
		output := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			output = append(output, map[string]any(item))
		}
		return output
	case []map[string]any:
		return typed
	case []any:
		output := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if record, ok := item.(map[string]any); ok {
				output = append(output, record)
			} else if record, ok := item.(domain.AgentJSON); ok {
				output = append(output, map[string]any(record))
			}
		}
		return output
	default:
		return nil
	}
}

func planDeploymentAcceptanceSummary(plan domain.AgentPlan) string {
	acceptance := metadataMap(plan.Metadata, "deployment_acceptance")
	if acceptance == nil {
		return "暂无部署验收摘要"
	}
	status := metadataString(acceptance, "status")
	if status == "" {
		status = "unknown"
	}
	count := 0
	if checks, ok := acceptance["checks"].([]domain.AgentJSON); ok {
		count = len(checks)
	} else if checks, ok := acceptance["checks"].([]any); ok {
		count = len(checks)
	}
	return fmt.Sprintf("%s，%d 项检查", status, count)
}

func planClusterConsistencySummary(plan domain.AgentPlan) string {
	acceptance := metadataMap(plan.Metadata, "deployment_acceptance")
	if acceptance == nil {
		return "暂无多节点一致性摘要"
	}
	consistency := metadataMap(domain.AgentJSON(acceptance), "multi_node_consistency")
	if consistency == nil {
		return "暂无多节点一致性摘要"
	}
	return "worker claim、恢复控制、限流策略和通知幂等已纳入一致性检查"
}

func buildAgentSLASummary(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentSLASummaryResponse {
	summary := AgentSLASummaryResponse{PlanCount: len(plans), ScheduledTaskCount: len(tasks)}
	totalPlanSeconds := 0.0
	timedPlans := 0
	for _, plan := range plans {
		switch plan.Status {
		case domain.AgentPlanStatusCompleted:
			summary.PlanSucceeded++
		case domain.AgentPlanStatusFailed, domain.AgentPlanStatusRejected, domain.AgentPlanStatusExpired:
			summary.PlanFailed++
		}
		if plan.CompletedAt != nil && !plan.CreatedAt.IsZero() {
			totalPlanSeconds += plan.CompletedAt.Sub(plan.CreatedAt).Seconds()
			timedPlans++
		}
		if metadataMap(plan.Metadata, "recovery") != nil {
			summary.RecoveryCount++
		}
		if metadataString(metadataMap(plan.Metadata, "handoff"), "status") == "required" {
			summary.HandoffCount++
		}
	}
	if timedPlans > 0 {
		summary.AveragePlanSeconds = roundAgentQualityScore(totalPlanSeconds / float64(timedPlans))
	}
	for _, task := range tasks {
		switch task.Status {
		case domain.AgentScheduledTaskStatusSucceeded:
			summary.ScheduledTaskSucceeded++
		case domain.AgentScheduledTaskStatusFailed, domain.AgentScheduledTaskStatusCanceled:
			summary.ScheduledTaskFailed++
		}
	}
	for _, audit := range audits {
		if strings.Contains(audit.EventType, "reply_sent") || (strings.Contains(audit.EventType, "report") && audit.Status == "succeeded") {
			summary.NotificationSentCount++
		}
		if strings.Contains(audit.EventType, "reply_failed") || (strings.Contains(audit.EventType, "report") && audit.Status == "failed") {
			summary.NotificationFailedCount++
		}
	}
	return summary
}

func buildAgentTaskReport(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask) AgentTaskReportResponse {
	report := AgentTaskReportResponse{
		ByStatus:     map[string]int{},
		ByEntry:      map[string]int{},
		ByCapability: map[string]int{},
		ByHandoff:    map[string]int{},
	}
	for _, plan := range plans {
		report.ByStatus[string(plan.Status)]++
		entry := metadataString(metadataMap(plan.Metadata, "admission_policy"), "entry")
		if entry == "" {
			entry = "plan"
		}
		report.ByEntry[entry]++
		handoff := metadataString(metadataMap(plan.Metadata, "handoff"), "status")
		if handoff == "" {
			handoff = "unknown"
		}
		report.ByHandoff[handoff]++
		for _, step := range plan.Steps {
			key := strings.TrimSpace(step.CapabilityKey)
			if key == "" {
				key = "unknown"
			}
			report.ByCapability[key]++
		}
	}
	for _, task := range tasks {
		report.ByStatus[string(task.Status)]++
		entry := strings.TrimSpace(task.TargetChannel)
		if entry == "" {
			entry = "scheduled_task"
		}
		report.ByEntry[entry]++
		report.ByHandoff[scheduledTaskHandoffStatus(task)]++
		for _, key := range task.AllowedCapabilities {
			if strings.TrimSpace(key) != "" {
				report.ByCapability[strings.TrimSpace(key)]++
			}
		}
	}
	return report
}

func planRecoverySummary(plan domain.AgentPlan) string {
	recovery := metadataMap(plan.Metadata, "recovery")
	if recovery == nil {
		return "暂无恢复记录"
	}
	strategy := metadataString(recovery, "recovery_strategy")
	result := metadataString(recovery, "recovery_result")
	if strategy == "" {
		strategy = "unknown"
	}
	if result == "" {
		result = "unknown"
	}
	return fmt.Sprintf("%s / %s", strategy, result)
}

func planRuntimeObservabilitySummary(plan domain.AgentPlan) string {
	metadata := metadataMap(plan.Metadata, "runtime_observability")
	if metadata == nil {
		return "暂无运行观测摘要"
	}
	summary := metadataString(metadata, "summary")
	if summary != "" {
		return summary
	}
	return fmt.Sprintf("状态 %s，失败步骤 %d，恢复 %d 次", metadataString(metadata, "status"), metadataNumber(metadata, "failed_steps"), metadataNumber(metadata, "recovery_count"))
}

func planHandoffSummary(plan domain.AgentPlan) string {
	metadata := metadataMap(plan.Metadata, "handoff")
	if metadata == nil {
		return "暂无人工接管摘要"
	}
	status := metadataString(metadata, "status")
	if status == "" {
		status = "unknown"
	}
	nextAction := metadataString(metadata, "next_action")
	if nextAction == "" {
		nextAction = "无下一步动作"
	}
	return status + " / " + nextAction
}

func planCapabilityPolicySummary(plan domain.AgentPlan) string {
	metadata := metadataMap(plan.Metadata, "capability_policy")
	if metadata == nil {
		return "暂无 capability 策略命中"
	}
	status := metadataString(metadata, "status")
	if status == "" {
		status = "allow"
	}
	return "策略状态 " + status
}

func agentPlanResponseToDomain(response AgentPlanResponse) domain.AgentPlan {
	plan := domain.AgentPlan{
		ID:                 response.ID,
		UserID:             response.UserID,
		SessionID:          response.SessionID,
		TurnID:             response.TurnID,
		ControllerRunID:    response.ControllerRunID,
		Status:             domain.AgentPlanStatus(response.Status),
		Goal:               response.Goal,
		Summary:            response.Summary,
		ImpactSummary:      response.ImpactSummary,
		RiskLevel:          response.RiskLevel,
		ConfirmationPolicy: response.ConfirmationPolicy,
		AllowedScopes:      append([]string(nil), response.AllowedScopes...),
		DedupeKey:          response.DedupeKey,
		PolicyDecision:     response.PolicyDecision,
		PolicyReason:       response.PolicyReason,
		ErrorMessage:       response.ErrorMessage,
		Metadata:           cloneApprovalMetadata(response.Metadata),
	}
	for _, step := range response.Steps {
		plan.Steps = append(plan.Steps, domain.AgentPlanStep{
			ID:              step.ID,
			PlanID:          step.PlanID,
			StepOrder:       step.StepOrder,
			Status:          domain.AgentPlanStepStatus(step.Status),
			CapabilityKey:   step.CapabilityKey,
			CapabilityScope: append([]string(nil), step.CapabilityScope...),
			Title:           step.Title,
			InputSummary:    step.InputSummary,
			OutputSummary:   step.OutputSummary,
			ExpectedOutput:  step.ExpectedOutput,
			FailureStrategy: step.FailureStrategy,
			ExecutorRunID:   step.ExecutorRunID,
			ObservationRef:  step.ObservationRef,
			ArtifactRefs:    append([]string(nil), step.ArtifactRefs...),
			ErrorMessage:    step.ErrorMessage,
			RetryCount:      step.RetryCount,
			MaxRetries:      step.MaxRetries,
			RetryReason:     step.RetryReason,
			RetryMetadata:   cloneApprovalMetadata(step.RetryMetadata),
		})
	}
	return plan
}

func agentPlanResponseEvidenceRefs(response AgentPlanResponse) []string {
	return planEvidenceRefs(agentPlanResponseToDomain(response))
}

func metadataFloat(metadata map[string]any, key string) float64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed
	default:
		return 0
	}
}

func agentQualityFreshness(plan domain.AgentPlan, now time.Time) float64 {
	reference := plan.UpdatedAt
	if plan.CompletedAt != nil && !plan.CompletedAt.IsZero() {
		reference = *plan.CompletedAt
	}
	if reference.IsZero() {
		return 0.5
	}
	age := now.UTC().Sub(reference.UTC())
	switch {
	case age <= 24*time.Hour:
		return 1
	case age <= 7*24*time.Hour:
		return 0.8
	case age <= 30*24*time.Hour:
		return 0.6
	default:
		return 0.4
	}
}

func agentQualityRatio(value int, total int) float64 {
	if total < 1 {
		return 0
	}
	ratio := float64(value) / float64(total)
	if ratio > 1 {
		return 1
	}
	if ratio < 0 {
		return 0
	}
	return ratio
}

func roundAgentQualityScore(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func agentQualityStatus(score float64) string {
	switch {
	case score >= 0.85:
		return "passed"
	case score >= 0.65:
		return "review"
	default:
		return "weak"
	}
}

func agentResultQualitySummary(score float64, evidenceCount int, failedSteps int) string {
	return fmt.Sprintf("score %.2f with %d evidence refs and %d failed steps", roundAgentQualityScore(score), evidenceCount, failedSteps)
}

func maxAgentInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func boolAgentInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
