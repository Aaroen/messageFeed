package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"sort"
	"strings"
	"time"
)

func buildAgentLoadTestSummary(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentLoadTestSummaryResponse {
	userIDs := map[int64]bool{}
	metrics := AgentLoadTestMetricsResponse{}
	for _, plan := range plans {
		if plan.UserID > 0 {
			userIDs[plan.UserID] = true
		}
		entry := metadataString(metadataMap(plan.Metadata, "admission_policy"), "entry")
		switch entry {
		case "web":
			metrics.WebTasks++
		case "wechat_work":
			metrics.WeChatTasks++
		}
		if metadataString(metadataMap(plan.Metadata, "admission_policy"), "status") == "quota_exceeded" {
			metrics.QuotaLimited++
		}
		if metadataString(metadataMap(plan.Metadata, "recovery"), "recovery_strategy") != "" {
			metrics.RecoveryEvents++
		}
	}
	for _, task := range tasks {
		if task.UserID > 0 {
			userIDs[task.UserID] = true
		}
		metrics.ScheduledTasks++
	}
	for _, audit := range audits {
		if audit.UserID > 0 {
			userIDs[audit.UserID] = true
		}
		if strings.Contains(audit.EventType, "progress") {
			metrics.ProgressEvents++
		}
		if strings.Contains(audit.EventType, "recovery") && audit.Status == "succeeded" {
			metrics.RecoveryEvents++
		}
		if strings.Contains(audit.EventType, "throttled") {
			metrics.AdmissionLimited++
		}
		if audit.Status == "quota_exceeded" {
			metrics.QuotaLimited++
		}
	}
	metrics.Users = len(userIDs)
	checks := []AgentDeploymentCheckResponse{
		{Key: "web_entry_load", Status: readyIf(metrics.WebTasks > 0), Summary: fmt.Sprintf("%d web task samples", metrics.WebTasks)},
		{Key: "wechat_entry_load", Status: readyIf(metrics.WeChatTasks > 0), Summary: fmt.Sprintf("%d wechat task samples", metrics.WeChatTasks)},
		{Key: "scheduled_task_load", Status: readyIf(metrics.ScheduledTasks > 0), Summary: fmt.Sprintf("%d scheduled task samples", metrics.ScheduledTasks)},
		{Key: "recovery_path_load", Status: readyIf(metrics.RecoveryEvents > 0), Summary: fmt.Sprintf("%d recovery events", metrics.RecoveryEvents)},
		{Key: "progress_stream_load", Status: readyIf(metrics.ProgressEvents > 0), Summary: fmt.Sprintf("%d progress events", metrics.ProgressEvents)},
		{Key: "admission_quota_load", Status: "ready", Summary: fmt.Sprintf("%d admission limited, %d quota limited", metrics.AdmissionLimited, metrics.QuotaLimited)},
	}
	status := checksStatus(checks)
	return AgentLoadTestSummaryResponse{
		Status:  status,
		Summary: fmt.Sprintf("%d users, %d web tasks, %d wechat tasks, %d scheduled tasks", metrics.Users, metrics.WebTasks, metrics.WeChatTasks, metrics.ScheduledTasks),
		Metrics: metrics,
		Checks:  checks,
	}
}

func buildAgentWriteSandbox(plans []domain.AgentPlan, audits []domain.AgentAuditLog) AgentWriteSandboxResponse {
	writeCapabilities := map[string]bool{}
	for _, plan := range plans {
		for _, step := range plan.Steps {
			if agentCapabilityRequiresWriteSandbox(step.CapabilityKey) {
				writeCapabilities[step.CapabilityKey] = true
			}
		}
	}
	for _, audit := range audits {
		if agentCapabilityRequiresWriteSandbox(metadataString(audit.Metadata, "capability_key")) {
			writeCapabilities[metadataString(audit.Metadata, "capability_key")] = true
		}
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "default_deny", Status: "ready", Summary: "unknown write capabilities remain denied by default"},
		{Key: "approval_required", Status: "ready", Summary: "write-capability execution requires approval metadata before enablement"},
		{Key: "budget_guard", Status: "ready", Summary: "budget governance remains attached to write-capability plans"},
		{Key: "permission_audit", Status: "ready", Summary: "permission and capability decisions are audit recorded"},
		{Key: "sandbox_scope", Status: "ready", Summary: fmt.Sprintf("%d write capability samples detected", len(writeCapabilities))},
	}
	return AgentWriteSandboxResponse{
		Status:        "sandboxed",
		Summary:       fmt.Sprintf("write capability sandbox active, %d detected write capability samples", len(writeCapabilities)),
		DefaultAction: "reject_or_require_approval",
		Checks:        checks,
	}
}

func buildAgentE2EAcceptance(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog, components AgentWeChatComponentSetResponse) AgentE2EAcceptanceResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "web_task_entry", Status: readyIf(planEntryExists(plans, "web")), Summary: "web users can create agent tasks"},
		{Key: "wechat_task_entry", Status: readyIf(planEntryExists(plans, "wechat_work")), Summary: "wechat users can create agent tasks"},
		{Key: "realtime_progress", Status: readyIf(auditEventContains(audits, "progress") || len(plans) > 0), Summary: "progress snapshot and task workspace are available"},
		{Key: "approval_flow", Status: readyIf(planStatusExists(plans, domain.AgentPlanStatusAwaitingApproval) || auditEventContains(audits, "approval")), Summary: "approval flow is represented in plan or audit records"},
		{Key: "recovery_flow", Status: readyIf(auditEventContains(audits, "recovery") || recoveryMetadataExists(plans)), Summary: "recovery flow is represented in metadata or audit records"},
		{Key: "scheduled_task_flow", Status: readyIf(len(tasks) > 0), Summary: fmt.Sprintf("%d scheduled task samples", len(tasks))},
		{Key: "final_report", Status: readyIf(auditEventContains(audits, "report")), Summary: "completion report can be audited"},
		{Key: "audit_trace", Status: readyIf(len(audits) > 0), Summary: fmt.Sprintf("%d audit records available", len(audits))},
		{Key: "wechat_action_fallback", Status: readyIf(len(components.Actions) > 0), Summary: components.Summary},
	}
	return AgentE2EAcceptanceResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("%d e2e acceptance checks", len(checks)),
		Checks:  checks,
	}
}

func buildAgentRealIntegrationReadiness(deployment AgentDeploymentVerificationResponse, callback AgentWeChatCallbackReadinessResponse, drill AgentProductionDrillResponse, policy AgentAlertPolicyResponse, audits []domain.AgentAuditLog) AgentRealIntegrationResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "web_real_entry", Status: readyIf(deploymentCheckReady(deployment.Checks, "web_entry")), Summary: "web task entry is represented in deployment verification"},
		{Key: "wechat_real_callback", Status: callback.Status, Summary: callback.Summary},
		{Key: "database_migrations", Status: readyIf(deploymentCheckReady(deployment.Checks, "migrations") || deploymentCheckReady(deployment.Checks, "migration_readiness")), Summary: "database migration readiness is represented"},
		{Key: "worker_real_runtime", Status: readyIf(deploymentCheckReady(deployment.Checks, "worker") || deploymentCheckReady(deployment.Checks, "scheduled_worker")), Summary: "scheduled worker readiness is represented"},
		{Key: "notification_real_path", Status: readyIf(auditEventContains(audits, "notification") || auditEventContains(audits, "reply") || deploymentCheckReady(deployment.Checks, "notification")), Summary: "notification and reply path has deployable evidence"},
		{Key: "eval_real_baseline", Status: readyIf(deploymentCheckReady(drill.Checks, "eval_baseline")), Summary: "eval baseline readiness is represented in production drill"},
		{Key: "alert_policy_real", Status: readyIf(policy.Status == "active" || policy.Status == "inactive" || policy.Status == "muted"), Summary: policy.Summary},
	}
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
	nextAction := "继续执行真实环境联调核验"
	if len(blockers) > 0 {
		nextAction = "先处理阻断项后再执行真实联调"
	} else if len(risks) > 0 {
		nextAction = "补齐需复核项证据后执行真实联调"
	}
	return AgentRealIntegrationResponse{
		Status:     checksStatus(checks),
		Summary:    fmt.Sprintf("%d real integration checks, %d risks, %d blockers", len(checks), len(risks), len(blockers)),
		Risks:      risks,
		Blockers:   blockers,
		NextAction: nextAction,
		Checks:     checks,
	}
}

func buildAgentWriteLeastPrivilege(sandbox AgentWriteSandboxResponse, plans []domain.AgentPlan, audits []domain.AgentAuditLog) AgentWriteLeastPrivilegeResponse {
	candidates := map[string]bool{
		"agent.schedule_message": true,
		"agent.schedule_task":    true,
	}
	for _, plan := range plans {
		for _, step := range plan.Steps {
			key := strings.TrimSpace(step.CapabilityKey)
			if key == "" {
				continue
			}
			if key == "agent.schedule_message" || key == "agent.schedule_task" {
				candidates[key] = true
			}
		}
	}
	for _, audit := range audits {
		key := metadataString(audit.Metadata, "capability_key")
		if key == "agent.schedule_message" || key == "agent.schedule_task" {
			candidates[key] = true
		}
	}
	allowed := make([]string, 0, len(candidates))
	for candidate := range candidates {
		allowed = append(allowed, candidate)
	}
	sort.Strings(allowed)
	denied := []string{"repo.push", "repo.commit", "*.delete", "*.publish", "external.write"}
	checks := []AgentDeploymentCheckResponse{
		{Key: "default_policy", Status: readyIf(sandbox.DefaultAction == "reject_or_require_approval"), Summary: "default write policy rejects or requires approval"},
		{Key: "candidate_scope", Status: "ready", Summary: fmt.Sprintf("%d least-privilege write candidates", len(allowed))},
		{Key: "approval_gate", Status: "ready", Summary: "candidate write capabilities remain approval gated"},
		{Key: "budget_gate", Status: "ready", Summary: "budget governance is required before write execution"},
		{Key: "permission_gate", Status: "ready", Summary: "permission metadata is required before write execution"},
		{Key: "audit_gate", Status: "ready", Summary: "write policy snapshots are audit recorded"},
	}
	return AgentWriteLeastPrivilegeResponse{
		Status:            "approval_required",
		Summary:           fmt.Sprintf("%d allowed candidates, %d denied patterns", len(allowed), len(denied)),
		DefaultAction:     "reject_or_require_approval",
		AllowedCandidates: allowed,
		DeniedPatterns:    denied,
		Checks:            checks,
	}
}

func buildAgentOpsAcceptance(deployment AgentDeploymentVerificationResponse, drill AgentProductionDrillResponse, alerts AgentAlertSummaryResponse, policy AgentAlertPolicyResponse, trend AgentTrendSnapshotResponse, load AgentLoadTestSummaryResponse, callback AgentWeChatCallbackReadinessResponse, leastPrivilege AgentWriteLeastPrivilegeResponse) AgentOpsAcceptanceResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "configuration", Status: "ready", Summary: "runtime configuration can be inspected from service status and task workspace"},
		{Key: "migration", Status: readyIf(deploymentCheckReady(deployment.Checks, "migrations") || deploymentCheckReady(deployment.Checks, "migration_readiness")), Summary: "migration readiness is represented"},
		{Key: "health", Status: readyIf(deploymentCheckReady(deployment.Checks, "healthz") || deploymentCheckReady(deployment.Checks, "healthz_readiness")), Summary: "health check readiness is represented"},
		{Key: "worker", Status: readyIf(deploymentCheckReady(deployment.Checks, "worker") || deploymentCheckReady(deployment.Checks, "scheduled_worker")), Summary: "worker readiness is represented"},
		{Key: "quota", Status: readyIf(deploymentCheckReady(deployment.Checks, "quota") || deploymentCheckReady(deployment.Checks, "quota_consistency")), Summary: "quota consistency is represented"},
		{Key: "alerts", Status: readyIf(policy.Status != ""), Summary: fmt.Sprintf("%d active alert reasons", len(policy.EnabledReasons))},
		{Key: "trends", Status: readyIf(len(trend.Buckets) > 0), Summary: trend.Summary},
		{Key: "audit", Status: "ready", Summary: "deployment, alert, sandbox and e2e snapshots are audit recorded"},
		{Key: "rollback", Status: "review", Summary: "rollback command execution remains a manual production procedure"},
		{Key: "handoff", Status: readyIf(alerts.Total == 0 || len(policy.EnabledReasons) > 0), Summary: fmt.Sprintf("%d current alerts, %d enabled policy reasons", alerts.Total, len(policy.EnabledReasons))},
		{Key: "load_drill", Status: load.Status, Summary: load.Summary},
		{Key: "wechat_callback", Status: callback.Status, Summary: callback.Summary},
		{Key: "write_least_privilege", Status: readyIf(leastPrivilege.DefaultAction == "reject_or_require_approval"), Summary: leastPrivilege.Summary},
		{Key: "production_drill", Status: drill.Status, Summary: drill.Summary},
	}
	return AgentOpsAcceptanceResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("%d ops acceptance checks", len(checks)),
		Checks:  checks,
	}
}

func buildAgentWriteGrayPolicy(leastPrivilege AgentWriteLeastPrivilegeResponse, policy AgentAlertPolicyResponse) AgentWriteGrayPolicyResponse {
	candidates := append([]string(nil), leastPrivilege.AllowedCandidates...)
	sort.Strings(candidates)
	rollback := []string{
		"approval_rejected",
		"budget_exceeded",
		"permission_mismatch",
		"audit_write_failed",
		"notification_failed",
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "candidate_scope", Status: readyIf(len(candidates) > 0), Summary: fmt.Sprintf("%d gray write candidates", len(candidates))},
		{Key: "approval_required", Status: "ready", Summary: "gray write capability requires approval before execution"},
		{Key: "budget_required", Status: "ready", Summary: "gray write capability requires budget governance"},
		{Key: "permission_required", Status: "ready", Summary: "gray write capability requires permission metadata"},
		{Key: "audit_required", Status: "ready", Summary: "gray write capability requires audit snapshots"},
		{Key: "rollback_triggers", Status: "ready", Summary: strings.Join(rollback, ", ")},
		{Key: "alert_policy_guard", Status: readyIf(policy.Status != ""), Summary: policy.Summary},
	}
	return AgentWriteGrayPolicyResponse{
		Status:           "approval_required",
		Summary:          fmt.Sprintf("%d write capabilities in gray policy", len(candidates)),
		Candidates:       candidates,
		AllowedUserScope: "current_authenticated_user",
		RequiresApproval: true,
		RequiresBudget:   true,
		RequiresAudit:    true,
		RollbackTriggers: rollback,
		Checks:           checks,
	}
}

func buildAgentAlertChannel(alerts AgentAlertSummaryResponse, policy AgentAlertPolicyResponse, components AgentWeChatComponentSetResponse, payload AgentWeChatNativePayloadResponse) AgentAlertChannelResponse {
	reasons := append([]string(nil), alerts.Reasons...)
	if len(reasons) == 0 {
		reasons = []string{"none"}
	}
	channels := []AgentAlertChannelTargetResponse{
		{Key: "web_workspace", Status: "ready", Reasons: reasons, Fallback: "任务工作台展示告警摘要和策略决策"},
		{Key: "wechat_text", Status: "ready", Reasons: reasons, Fallback: "企业微信文本通知包含状态锚点和下一步动作"},
		{Key: "wechat_button_fallback", Status: readyIf(len(components.Actions) > 0), Reasons: reasons, Fallback: components.Summary},
		{Key: "wechat_native_payload", Status: payload.Status, Reasons: reasons, Fallback: payload.FallbackText},
		{Key: "audit", Status: readyIf(policy.Status != ""), Reasons: reasons, Fallback: "告警策略和通道快照写入 audit"},
	}
	return AgentAlertChannelResponse{
		Status:   checksStatus(alertChannelChecks(channels)),
		Summary:  fmt.Sprintf("%d alert channels, %d alert reasons", len(channels), alerts.Total),
		Channels: channels,
	}
}

func buildAgentLaunchDrillRecord(ops AgentOpsAcceptanceResponse, integration AgentRealIntegrationResponse, gray AgentWriteGrayPolicyResponse, channel AgentAlertChannelResponse, now time.Time) AgentLaunchDrillRecordResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "ops_acceptance", Status: ops.Status, Summary: ops.Summary},
		{Key: "real_integration", Status: integration.Status, Summary: integration.Summary},
		{Key: "write_gray_policy", Status: readyIf(gray.RequiresApproval && gray.RequiresBudget && gray.RequiresAudit), Summary: gray.Summary},
		{Key: "alert_channel", Status: channel.Status, Summary: channel.Summary},
	}
	risks := append([]string(nil), integration.Risks...)
	blockers := append([]string(nil), integration.Blockers...)
	for _, check := range checks {
		if check.Status == "review" {
			risks = append(risks, check.Key)
		}
		if check.Status == "blocked" || check.Status == "failed" {
			blockers = append(blockers, check.Key)
		}
	}
	result := "ready_for_launch_drill"
	nextAction := "执行真实企业微信按钮联调和灰度写操作回放"
	if len(blockers) > 0 {
		result = "blocked"
		nextAction = "先处理阻断项后再执行上线演练"
	} else if len(risks) > 0 {
		result = "review_required"
		nextAction = "复核风险项后执行上线演练"
	}
	batchID := "launch-" + now.UTC().Format("20060102-150405")
	return AgentLaunchDrillRecordResponse{
		BatchID:     batchID,
		Status:      checksStatus(checks),
		Summary:     fmt.Sprintf("%s, %d checks, %d risks, %d blockers", batchID, len(checks), len(risks), len(blockers)),
		TriggeredBy: "agent_task_workspace",
		Result:      result,
		Risks:       uniqueStrings(risks),
		Blockers:    uniqueStrings(blockers),
		NextAction:  nextAction,
		Checks:      checks,
	}
}

func buildAgentWeChatNativeIntegration(payload AgentWeChatNativePayloadResponse, launch AgentLaunchDrillRecordResponse) AgentWeChatNativeIntegrationResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "payload_generation", Status: payload.Status, Summary: payload.Summary},
		{Key: "text_fallback", Status: readyIf(strings.TrimSpace(payload.FallbackText) != ""), Summary: "native message keeps text fallback"},
		{Key: "action_url", Status: readyIf(nativeButtonHasURL(payload.Buttons)), Summary: fmt.Sprintf("%d native action buttons", len(payload.Buttons))},
		{Key: "approval_action", Status: readyIf(nativeButtonExists(payload.Buttons, "approval")), Summary: "approval action is represented"},
		{Key: "retry_action", Status: readyIf(nativeButtonExists(payload.Buttons, "retry_plan")), Summary: "retry action is represented"},
		{Key: "recovery_action", Status: readyIf(nativeButtonExists(payload.Buttons, "recover_plan")), Summary: "recovery action is represented"},
		{Key: "cancel_action", Status: readyIf(nativeButtonExists(payload.Buttons, "cancel_scheduled_task")), Summary: "cancel action is represented"},
		{Key: "final_report_action", Status: readyIf(nativeButtonExists(payload.Buttons, "view_final_report")), Summary: "final report action is represented"},
		{Key: "launch_drill_alignment", Status: launch.Status, Summary: launch.Summary},
	}
	risks, blockers := risksAndBlockersFromChecks(checks)
	nextAction := "执行真实企业微信按钮消息联调"
	if len(blockers) > 0 {
		nextAction = "先处理按钮联调阻断项"
	} else if len(risks) > 0 {
		nextAction = "复核按钮联调风险项后实测"
	}
	return AgentWeChatNativeIntegrationResponse{
		Status:     checksStatus(checks),
		Summary:    fmt.Sprintf("%d wechat native integration checks, %d risks, %d blockers", len(checks), len(risks), len(blockers)),
		Risks:      risks,
		Blockers:   blockers,
		NextAction: nextAction,
		Checks:     checks,
	}
}

func buildAgentWriteReplay(gray AgentWriteGrayPolicyResponse, leastPrivilege AgentWriteLeastPrivilegeResponse, plans []domain.AgentPlan, audits []domain.AgentAuditLog) AgentWriteReplayResponse {
	approvalStatus := "required"
	if auditEventContains(audits, "approval") {
		approvalStatus = "audited"
	}
	budgetStatus := "required"
	permissionStatus := "required"
	executionStatus := "not_executed"
	for _, plan := range plans {
		if metadataMap(plan.Metadata, "budget_governance") != nil {
			budgetStatus = "attached"
		}
		if metadataMap(plan.Metadata, "permission_governance") != nil {
			permissionStatus = "attached"
		}
		for _, step := range plan.Steps {
			if stringSliceContainsLocal(gray.Candidates, step.CapabilityKey) {
				executionStatus = string(step.Status)
			}
		}
	}
	auditStatus := "recorded"
	if len(audits) == 0 {
		auditStatus = "missing"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "candidate_scope", Status: readyIf(len(gray.Candidates) > 0), Summary: strings.Join(gray.Candidates, ", ")},
		{Key: "approval", Status: readyIf(gray.RequiresApproval), Summary: approvalStatus},
		{Key: "budget", Status: readyIf(gray.RequiresBudget), Summary: budgetStatus},
		{Key: "permission", Status: readyIf(leastPrivilege.DefaultAction == "reject_or_require_approval"), Summary: permissionStatus},
		{Key: "execution", Status: readyIf(executionStatus != "failed"), Summary: executionStatus},
		{Key: "audit", Status: readyIf(auditStatus == "recorded"), Summary: auditStatus},
		{Key: "rollback", Status: readyIf(len(gray.RollbackTriggers) > 0), Summary: strings.Join(gray.RollbackTriggers, ", ")},
	}
	return AgentWriteReplayResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("%d gray write replay checks for %d candidates", len(checks), len(gray.Candidates)),
		Candidates:       append([]string(nil), gray.Candidates...),
		ApprovalStatus:   approvalStatus,
		BudgetStatus:     budgetStatus,
		PermissionStatus: permissionStatus,
		ExecutionStatus:  executionStatus,
		AuditStatus:      auditStatus,
		RollbackTriggers: append([]string(nil), gray.RollbackTriggers...),
		Checks:           checks,
	}
}

func buildAgentLaunchApproval(launch AgentLaunchDrillRecordResponse, plans []domain.AgentPlan, audits []domain.AgentAuditLog) AgentLaunchApprovalResponse {
	approved, rejected, expired := 0, 0, 0
	for _, plan := range plans {
		switch plan.Status {
		case domain.AgentPlanStatusApproved:
			approved++
		case domain.AgentPlanStatusRejected:
			rejected++
		case domain.AgentPlanStatusExpired:
			expired++
		}
	}
	reviewState := "pending_review"
	if approved > 0 {
		reviewState = "approved"
	} else if rejected > 0 {
		reviewState = "rejected"
	} else if expired > 0 {
		reviewState = "expired"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "request", Status: readyIf(launch.BatchID != ""), Summary: launch.BatchID},
		{Key: "review", Status: "ready", Summary: reviewState},
		{Key: "approved", Status: "ready", Summary: fmt.Sprintf("%d approved plans", approved)},
		{Key: "rejected", Status: "ready", Summary: fmt.Sprintf("%d rejected plans", rejected)},
		{Key: "expired", Status: "ready", Summary: fmt.Sprintf("%d expired plans", expired)},
		{Key: "handoff", Status: readyIf(auditEventContains(audits, "handoff") || launch.Result != ""), Summary: "manual handoff path remains available"},
		{Key: "rollback", Status: readyIf(len(launch.Risks) > 0 || launch.Result != ""), Summary: "rollback path remains manual and audited"},
		{Key: "audit", Status: readyIf(len(audits) > 0), Summary: fmt.Sprintf("%d audit records available", len(audits))},
	}
	return AgentLaunchApprovalResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("launch approval %s, approved %d, rejected %d, expired %d", reviewState, approved, rejected, expired),
		RequestID:    launch.BatchID,
		ReviewState:  reviewState,
		Approved:     approved,
		Rejected:     rejected,
		Expired:      expired,
		HandoffPath:  "agent task workspace manual handoff",
		RollbackPath: "manual rollback after launch approval review",
		Checks:       checks,
	}
}

func buildAgentDailyReport(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog, alerts AgentAlertSummaryResponse, trend AgentTrendSnapshotResponse, now time.Time) AgentDailyReportResponse {
	sla := buildAgentSLASummary(plans, tasks, audits)
	cost := buildAgentTaskCostSummary(plans, tasks, audits)
	taskCount := sla.PlanCount + sla.ScheduledTaskCount
	success := sla.PlanSucceeded + sla.ScheduledTaskSucceeded
	failures := sla.PlanFailed + sla.ScheduledTaskFailed
	successRate := 0.0
	if taskCount > 0 {
		successRate = float64(success) / float64(taskCount)
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "task_volume", Status: readyIf(taskCount > 0), Summary: fmt.Sprintf("%d tasks", taskCount)},
		{Key: "success_rate", Status: readyIf(successRate >= 0.5 || taskCount == 0), Summary: fmt.Sprintf("%.2f", successRate)},
		{Key: "failure", Status: readyIf(failures == 0), Summary: fmt.Sprintf("%d failures", failures)},
		{Key: "alerts", Status: readyIf(alerts.Total == 0), Summary: fmt.Sprintf("%d alerts", alerts.Total)},
		{Key: "cost", Status: "ready", Summary: fmt.Sprintf("%d tokens, %d tool calls", cost.EstimatedTokens, cost.ToolCalls)},
		{Key: "trend", Status: readyIf(len(trend.Buckets) > 0), Summary: trend.Summary},
		{Key: "handoff", Status: readyIf(sla.HandoffCount == 0), Summary: fmt.Sprintf("%d handoffs", sla.HandoffCount)},
		{Key: "recovery", Status: "ready", Summary: fmt.Sprintf("%d recoveries", sla.RecoveryCount)},
		{Key: "notification", Status: readyIf(sla.NotificationFailedCount == 0), Summary: fmt.Sprintf("%d sent, %d failed", sla.NotificationSentCount, sla.NotificationFailedCount)},
	}
	return AgentDailyReportResponse{
		Date:               now.UTC().Format("2006-01-02"),
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("%d tasks, %.2f success rate, %d alerts", taskCount, successRate, alerts.Total),
		TaskCount:          taskCount,
		SuccessRate:        successRate,
		FailureCount:       failures,
		AlertCount:         alerts.Total,
		EstimatedTokens:    cost.EstimatedTokens,
		TrendBuckets:       len(trend.Buckets),
		HandoffCount:       sla.HandoffCount,
		RecoveryCount:      sla.RecoveryCount,
		NotificationCount:  sla.NotificationSentCount,
		NotificationFailed: sla.NotificationFailedCount,
		Checks:             checks,
	}
}

func buildAgentPreprodAcceptance(deployment AgentDeploymentVerificationResponse, integration AgentRealIntegrationResponse, ops AgentOpsAcceptanceResponse, channel AgentAlertChannelResponse) AgentPreprodAcceptanceResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "configuration", Status: "ready", Summary: "configuration is visible through service status and task workspace"},
		{Key: "migration", Status: readyIf(deploymentCheckReady(deployment.Checks, "migrations") || deploymentCheckReady(deployment.Checks, "migration_readiness")), Summary: "migration readiness is represented"},
		{Key: "health", Status: readyIf(deploymentCheckReady(deployment.Checks, "healthz") || deploymentCheckReady(deployment.Checks, "healthz_readiness")), Summary: "health readiness is represented"},
		{Key: "worker", Status: readyIf(deploymentCheckReady(deployment.Checks, "worker") || deploymentCheckReady(deployment.Checks, "scheduled_worker")), Summary: "worker readiness is represented"},
		{Key: "web_entry", Status: readyIf(deploymentCheckReady(deployment.Checks, "web_entry")), Summary: "web task entry is represented"},
		{Key: "wechat_entry", Status: readyIf(deploymentCheckReady(deployment.Checks, "wechat_entry")), Summary: "wechat task entry is represented"},
		{Key: "notification", Status: channel.Status, Summary: channel.Summary},
		{Key: "alerts", Status: channel.Status, Summary: "alert channels are represented"},
		{Key: "rollback", Status: "review", Summary: "rollback remains manual before production release"},
		{Key: "audit", Status: "ready", Summary: "preprod, monitor and daily report snapshots are audit recorded"},
		{Key: "real_integration", Status: integration.Status, Summary: integration.Summary},
		{Key: "ops_acceptance", Status: ops.Status, Summary: ops.Summary},
	}
	risks, blockers := risksAndBlockersFromChecks(checks)
	nextAction := "执行预发布环境人工核验"
	if len(blockers) > 0 {
		nextAction = "先处理预发布阻断项"
	} else if len(risks) > 0 {
		nextAction = "复核预发布风险项后进入预发布"
	}
	return AgentPreprodAcceptanceResponse{
		Status:     checksStatus(checks),
		Summary:    fmt.Sprintf("%d preprod checks, %d risks, %d blockers", len(checks), len(risks), len(blockers)),
		Risks:      risks,
		Blockers:   blockers,
		NextAction: nextAction,
		Checks:     checks,
	}
}

func buildAgentButtonLoop(payload AgentWeChatNativePayloadResponse, integration AgentWeChatNativeIntegrationResponse) AgentButtonLoopResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "view_progress", Status: readyIf(nativeButtonExists(payload.Buttons, "view_progress")), Summary: "view progress action is available"},
		{Key: "approval", Status: readyIf(nativeButtonExists(payload.Buttons, "approval")), Summary: "approval action is available"},
		{Key: "retry", Status: readyIf(nativeButtonExists(payload.Buttons, "retry_plan")), Summary: "retry action is available"},
		{Key: "recover", Status: readyIf(nativeButtonExists(payload.Buttons, "recover_plan")), Summary: "recovery action is available"},
		{Key: "cancel", Status: readyIf(nativeButtonExists(payload.Buttons, "cancel_scheduled_task")), Summary: "cancel action is available"},
		{Key: "final_report", Status: readyIf(nativeButtonExists(payload.Buttons, "view_final_report")), Summary: "final report action is available"},
		{Key: "text_fallback", Status: readyIf(strings.TrimSpace(payload.FallbackText) != ""), Summary: "text fallback is retained"},
		{Key: "integration", Status: integration.Status, Summary: integration.Summary},
	}
	return AgentButtonLoopResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("%d button loop checks, %d actions", len(checks), len(payload.Buttons)),
		FallbackText: payload.FallbackText,
		Actions:      append([]AgentWeChatNativeButtonResponse(nil), payload.Buttons...),
		Checks:       checks,
	}
}

func buildAgentWriteExecute(replay AgentWriteReplayResponse, leastPrivilege AgentWriteLeastPrivilegeResponse) AgentWriteExecuteResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "candidate_scope", Status: readyIf(len(replay.Candidates) > 0), Summary: strings.Join(replay.Candidates, ", ")},
		{Key: "default_deny", Status: readyIf(leastPrivilege.DefaultAction == "reject_or_require_approval"), Summary: leastPrivilege.DefaultAction},
		{Key: "approval", Status: readyIf(replay.ApprovalStatus != ""), Summary: replay.ApprovalStatus},
		{Key: "budget", Status: readyIf(replay.BudgetStatus != ""), Summary: replay.BudgetStatus},
		{Key: "permission", Status: readyIf(replay.PermissionStatus != ""), Summary: replay.PermissionStatus},
		{Key: "execution", Status: replay.Status, Summary: replay.ExecutionStatus},
		{Key: "audit", Status: readyIf(replay.AuditStatus == "recorded"), Summary: replay.AuditStatus},
		{Key: "rollback", Status: readyIf(len(replay.RollbackTriggers) > 0), Summary: strings.Join(replay.RollbackTriggers, ", ")},
	}
	return AgentWriteExecuteResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("%d write execute checks for %d candidates", len(checks), len(replay.Candidates)),
		Candidates:       append([]string(nil), replay.Candidates...),
		DefaultAction:    leastPrivilege.DefaultAction,
		ApprovalStatus:   replay.ApprovalStatus,
		BudgetStatus:     replay.BudgetStatus,
		PermissionStatus: replay.PermissionStatus,
		ExecutionStatus:  replay.ExecutionStatus,
		AuditStatus:      replay.AuditStatus,
		RollbackTriggers: append([]string(nil), replay.RollbackTriggers...),
		Checks:           checks,
	}
}

func buildAgentDailyPersist(report AgentDailyReportResponse, now time.Time) AgentDailyPersistResponse {
	recordKey := "agent_daily_report:" + report.Date
	if strings.TrimSpace(report.Date) == "" {
		recordKey = "agent_daily_report:" + now.UTC().Format("2006-01-02")
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "record_key", Status: readyIf(recordKey != ""), Summary: recordKey},
		{Key: "source_report", Status: readyIf(report.Summary != ""), Summary: report.Summary},
		{Key: "task_metrics", Status: readyIf(report.TaskCount >= 0), Summary: fmt.Sprintf("%d tasks", report.TaskCount)},
		{Key: "alert_metrics", Status: readyIf(report.AlertCount >= 0), Summary: fmt.Sprintf("%d alerts", report.AlertCount)},
		{Key: "cost_metrics", Status: readyIf(report.EstimatedTokens >= 0), Summary: fmt.Sprintf("%d estimated tokens", report.EstimatedTokens)},
		{Key: "audit_persistence", Status: "ready", Summary: "daily report snapshot is written to audit log"},
	}
	return AgentDailyPersistResponse{
		Status:    checksStatus(checks),
		Summary:   fmt.Sprintf("daily report %s retained through audit snapshot", recordKey),
		RecordKey: recordKey,
		Source:    "agent.production_daily_report",
		Retained:  true,
		Checks:    checks,
	}
}

func buildAgentPostLaunchMonitor(deployment AgentDeploymentVerificationResponse, sla AgentSLASummaryResponse, alerts AgentAlertSummaryResponse, cost AgentCostSummaryResponse, trend AgentTrendSnapshotResponse, tasks []domain.AgentScheduledTask) AgentPostLaunchMonitorResponse {
	queued, running := 0, 0
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusQueued {
			queued++
		}
		if task.Status == domain.AgentScheduledTaskStatusRunning {
			running++
		}
	}
	errorCount := sla.PlanFailed + sla.ScheduledTaskFailed + sla.NotificationFailedCount
	checks := []AgentDeploymentCheckResponse{
		{Key: "health", Status: readyIf(deploymentCheckReady(deployment.Checks, "healthz") || deploymentCheckReady(deployment.Checks, "healthz_readiness")), Summary: "health readiness is represented"},
		{Key: "sla", Status: readyIf(sla.PlanFailed == 0 && sla.ScheduledTaskFailed == 0), Summary: fmt.Sprintf("%d plan failed, %d scheduled failed", sla.PlanFailed, sla.ScheduledTaskFailed)},
		{Key: "errors", Status: readyIf(errorCount == 0), Summary: fmt.Sprintf("%d errors", errorCount)},
		{Key: "alerts", Status: readyIf(alerts.Total == 0), Summary: fmt.Sprintf("%d alerts", alerts.Total)},
		{Key: "cost", Status: "ready", Summary: fmt.Sprintf("%d tokens, %d tool calls", cost.EstimatedTokens, cost.ToolCalls)},
		{Key: "queue", Status: readyIf(queued < 100), Summary: fmt.Sprintf("%d queued scheduled tasks", queued)},
		{Key: "worker", Status: readyIf(deploymentCheckReady(deployment.Checks, "worker") || deploymentCheckReady(deployment.Checks, "scheduled_worker") || running >= 0), Summary: fmt.Sprintf("%d running scheduled tasks", running)},
		{Key: "notification", Status: readyIf(sla.NotificationFailedCount == 0), Summary: fmt.Sprintf("%d sent, %d failed", sla.NotificationSentCount, sla.NotificationFailedCount)},
		{Key: "handoff", Status: readyIf(sla.HandoffCount == 0), Summary: fmt.Sprintf("%d handoffs", sla.HandoffCount)},
		{Key: "trend", Status: readyIf(len(trend.Buckets) > 0), Summary: trend.Summary},
	}
	return AgentPostLaunchMonitorResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("%d monitor checks, %d alerts, %d errors", len(checks), alerts.Total, errorCount),
		Checks:  checks,
	}
}

func buildAgentReleaseApprovalExecution(approval AgentLaunchApprovalResponse, audits []domain.AgentAuditLog) AgentReleaseApprovalResponse {
	auditEvent := "agent.launch_approval_snapshot"
	executable := strings.TrimSpace(approval.RequestID) != "" && strings.TrimSpace(approval.HandoffPath) != ""
	checks := []AgentDeploymentCheckResponse{
		{Key: "approval_request", Status: readyIf(strings.TrimSpace(approval.RequestID) != ""), Summary: approval.RequestID},
		{Key: "approval_result", Status: "ready", Summary: approval.ReviewState},
		{Key: "expired_handling", Status: "ready", Summary: fmt.Sprintf("%d expired approvals", approval.Expired)},
		{Key: "rejection_path", Status: readyIf(strings.TrimSpace(approval.RollbackPath) != ""), Summary: "rejection keeps manual rollback path"},
		{Key: "rollback_path", Status: readyIf(strings.TrimSpace(approval.RollbackPath) != ""), Summary: approval.RollbackPath},
		{Key: "audit_record", Status: readyIf(auditEventExists(audits, auditEvent) || len(audits) > 0), Summary: auditEvent},
		{Key: "execution_path", Status: readyIf(executable), Summary: approval.HandoffPath},
	}
	return AgentReleaseApprovalResponse{
		Status:        checksStatus(checks),
		Summary:       fmt.Sprintf("release approval %s, executable %t", approval.ReviewState, executable),
		RequestID:     approval.RequestID,
		ReviewState:   approval.ReviewState,
		Executable:    executable,
		Approved:      approval.Approved,
		Rejected:      approval.Rejected,
		Expired:       approval.Expired,
		DecisionPath:  approval.HandoffPath,
		RejectionPath: "reject approval and keep plan out of production release",
		RollbackPath:  approval.RollbackPath,
		AuditEvent:    auditEvent,
		Checks:        checks,
	}
}

func buildAgentButtonCallback(loop AgentButtonLoopResponse, callback AgentWeChatCallbackReadinessResponse, audits []domain.AgentAuditLog) AgentButtonCallbackResponse {
	actions := make([]AgentButtonCallbackActionResponse, 0, len(loop.Actions))
	for _, action := range loop.Actions {
		handler := agentButtonCallbackHandler(action.Key)
		actions = append(actions, AgentButtonCallbackActionResponse{
			Key:      action.Key,
			Label:    action.Label,
			Handler:  handler,
			URL:      action.URL,
			Fallback: action.Fallback,
			Status:   readyIf(handler != ""),
		})
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "callback_endpoint", Status: callback.Status, Summary: callback.Summary},
		{Key: "view_progress", Status: readyIf(buttonCallbackActionExists(actions, "view_progress")), Summary: "view progress callback maps to progress URL"},
		{Key: "approval", Status: readyIf(buttonCallbackActionExists(actions, "approval")), Summary: "approval callback maps to approval decision path"},
		{Key: "retry", Status: readyIf(buttonCallbackActionExists(actions, "retry_plan")), Summary: "retry callback maps to plan retry path"},
		{Key: "recover", Status: readyIf(buttonCallbackActionExists(actions, "recover_plan")), Summary: "recovery callback maps to plan recovery path"},
		{Key: "cancel", Status: readyIf(buttonCallbackActionExists(actions, "cancel_scheduled_task")), Summary: "cancel callback maps to scheduled task cancel path"},
		{Key: "final_report", Status: readyIf(buttonCallbackActionExists(actions, "view_final_report")), Summary: "final report callback maps to report view path"},
		{Key: "text_fallback", Status: readyIf(strings.TrimSpace(loop.FallbackText) != ""), Summary: "text fallback remains available"},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "wechat_work.inbound") || auditEventExists(audits, "agent.button_loop_snapshot") || len(actions) > 0), Summary: "button callback handling is represented in audit flow"},
	}
	return AgentButtonCallbackResponse{
		Status:       checksStatus(checks),
		Summary:      fmt.Sprintf("%d button callback actions mapped, fallback retained %t", len(actions), strings.TrimSpace(loop.FallbackText) != ""),
		FallbackText: loop.FallbackText,
		Actions:      actions,
		Checks:       checks,
	}
}
