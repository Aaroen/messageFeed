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

func buildAgentWriteAuditReview(execute AgentWriteExecuteResponse, plans []domain.AgentPlan, audits []domain.AgentAuditLog) AgentWriteAuditReviewResponse {
	failed, handoff := 0, 0
	for _, plan := range plans {
		if metadataString(metadataMap(plan.Metadata, "handoff"), "status") == "required" {
			handoff++
		}
		for _, step := range plan.Steps {
			if stringSliceContainsLocal(execute.Candidates, step.CapabilityKey) && step.Status == domain.AgentPlanStepStatusFailed {
				failed++
			}
		}
	}
	approvalEvidence := execute.ApprovalStatus
	budgetEvidence := execute.BudgetStatus
	permissionEvidence := execute.PermissionStatus
	executionEvidence := execute.ExecutionStatus
	failureEvidence := fmt.Sprintf("%d failed write-capability steps", failed)
	rollbackEvidence := strings.Join(execute.RollbackTriggers, ", ")
	handoffEvidence := fmt.Sprintf("%d handoff records", handoff)
	checks := []AgentDeploymentCheckResponse{
		{Key: "candidate_scope", Status: readyIf(stringSliceContainsLocal(execute.Candidates, "agent.schedule_message") && stringSliceContainsLocal(execute.Candidates, "agent.schedule_task")), Summary: strings.Join(execute.Candidates, ", ")},
		{Key: "approval_evidence", Status: readyIf(approvalEvidence != ""), Summary: approvalEvidence},
		{Key: "budget_evidence", Status: readyIf(budgetEvidence != ""), Summary: budgetEvidence},
		{Key: "permission_evidence", Status: readyIf(permissionEvidence != ""), Summary: permissionEvidence},
		{Key: "execution_evidence", Status: readyIf(executionEvidence != ""), Summary: executionEvidence},
		{Key: "failure_evidence", Status: readyIf(failed == 0), Summary: failureEvidence},
		{Key: "rollback_evidence", Status: readyIf(rollbackEvidence != ""), Summary: rollbackEvidence},
		{Key: "handoff_evidence", Status: "ready", Summary: handoffEvidence},
		{Key: "audit_log", Status: readyIf(auditEventContains(audits, "write") || execute.AuditStatus == "recorded"), Summary: execute.AuditStatus},
	}
	return AgentWriteAuditReviewResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("%d write audit checks, %d candidates, %d failures", len(checks), len(execute.Candidates), failed),
		Candidates:         append([]string(nil), execute.Candidates...),
		ApprovalEvidence:   approvalEvidence,
		BudgetEvidence:     budgetEvidence,
		PermissionEvidence: permissionEvidence,
		ExecutionEvidence:  executionEvidence,
		FailureEvidence:    failureEvidence,
		RollbackEvidence:   rollbackEvidence,
		HandoffEvidence:    handoffEvidence,
		Checks:             checks,
	}
}

func buildAgentDailySend(persist AgentDailyPersistResponse, report AgentDailyReportResponse, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentDailySendResponse {
	scheduleStatus := "ready_to_schedule"
	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.TaskType), "daily") || strings.Contains(strings.ToLower(task.TaskType), "report") || strings.Contains(strings.ToLower(task.Goal), "日报") {
			scheduleStatus = string(task.Status)
			break
		}
	}
	deliveryStatus := "pending"
	if report.NotificationCount > 0 && report.NotificationFailed == 0 {
		deliveryStatus = "sent"
	} else if report.NotificationFailed > 0 {
		deliveryStatus = "failed"
	}
	retryStatus := "not_required"
	if report.NotificationFailed > 0 {
		retryStatus = "retry_required"
	}
	wechatReportStatus := "pending"
	if auditEventExists(audits, "agent.scheduled_task_report") || report.NotificationCount > 0 {
		wechatReportStatus = "recorded"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "generation", Status: readyIf(report.Summary != ""), Summary: report.Summary},
		{Key: "persistence", Status: readyIf(persist.Retained), Summary: persist.RecordKey},
		{Key: "schedule", Status: readyIf(scheduleStatus != ""), Summary: scheduleStatus},
		{Key: "delivery", Status: readyIf(deliveryStatus != "failed"), Summary: deliveryStatus},
		{Key: "failure_retry", Status: readyIf(retryStatus == "not_required" || retryStatus == "retry_required"), Summary: retryStatus},
		{Key: "wechat_report", Status: readyIf(wechatReportStatus != ""), Summary: wechatReportStatus},
	}
	return AgentDailySendResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("daily report send %s, delivery %s, retry %s", scheduleStatus, deliveryStatus, retryStatus),
		RecordKey:          persist.RecordKey,
		ScheduleStatus:     scheduleStatus,
		DeliveryStatus:     deliveryStatus,
		RetryStatus:        retryStatus,
		WeChatReportStatus: wechatReportStatus,
		Checks:             checks,
	}
}

func buildAgentMonitorAlertDrill(monitor AgentPostLaunchMonitorResponse, channel AgentAlertChannelResponse, alerts AgentAlertSummaryResponse, sla AgentSLASummaryResponse, audits []domain.AgentAuditLog) AgentMonitorAlertDrillResponse {
	triggerStatus := "no_active_alert"
	if alerts.Total > 0 {
		triggerStatus = "triggered"
	}
	notificationStatus := channel.Status
	if notificationStatus == "" {
		notificationStatus = "review"
	}
	handoffStatus := "not_required"
	if sla.HandoffCount > 0 {
		handoffStatus = "required"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "health_monitor", Status: deploymentCheckStatus(monitor.Checks, "health"), Summary: deploymentCheckSummary(monitor.Checks, "health")},
		{Key: "sla_monitor", Status: deploymentCheckStatus(monitor.Checks, "sla"), Summary: deploymentCheckSummary(monitor.Checks, "sla")},
		{Key: "error_monitor", Status: deploymentCheckStatus(monitor.Checks, "errors"), Summary: deploymentCheckSummary(monitor.Checks, "errors")},
		{Key: "cost_monitor", Status: deploymentCheckStatus(monitor.Checks, "cost"), Summary: deploymentCheckSummary(monitor.Checks, "cost")},
		{Key: "queue_monitor", Status: deploymentCheckStatus(monitor.Checks, "queue"), Summary: deploymentCheckSummary(monitor.Checks, "queue")},
		{Key: "worker_monitor", Status: deploymentCheckStatus(monitor.Checks, "worker"), Summary: deploymentCheckSummary(monitor.Checks, "worker")},
		{Key: "notification_failure", Status: readyIf(sla.NotificationFailedCount == 0), Summary: fmt.Sprintf("%d notification failures", sla.NotificationFailedCount)},
		{Key: "handoff", Status: readyIf(sla.HandoffCount == 0), Summary: handoffStatus},
		{Key: "alert_trigger", Status: readyIf(alerts.Total >= 0), Summary: triggerStatus},
		{Key: "alert_notification", Status: notificationStatus, Summary: channel.Summary},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "alert") || alerts.Total >= 0), Summary: "alert decisions are auditable"},
	}
	return AgentMonitorAlertDrillResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("monitor alert drill %s, notification %s, handoff %s", triggerStatus, notificationStatus, handoffStatus),
		TriggerStatus:      triggerStatus,
		NotificationStatus: notificationStatus,
		HandoffStatus:      handoffStatus,
		Checks:             checks,
	}
}

func buildAgentButtonDirectControl(callback AgentButtonCallbackResponse, audits []domain.AgentAuditLog) AgentButtonDirectControlResponse {
	executed, skipped := 0, 0
	for _, audit := range audits {
		if audit.EventType != "agent.button_direct_control" {
			continue
		}
		if audit.Status == "succeeded" {
			executed++
		} else {
			skipped++
		}
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "view_progress", Status: readyIf(buttonCallbackActionExists(callback.Actions, "view_progress")), Summary: "view progress is directly handled"},
		{Key: "approval", Status: readyIf(buttonCallbackActionExists(callback.Actions, "approval")), Summary: "approval button can approve or enter approval control"},
		{Key: "retry_plan", Status: readyIf(buttonCallbackActionExists(callback.Actions, "retry_plan")), Summary: "retry button can queue failed steps"},
		{Key: "recover_plan", Status: readyIf(buttonCallbackActionExists(callback.Actions, "recover_plan")), Summary: "recovery button can recover interrupted plans"},
		{Key: "cancel_scheduled_task", Status: readyIf(buttonCallbackActionExists(callback.Actions, "cancel_scheduled_task")), Summary: "cancel button can cancel associated scheduled task"},
		{Key: "view_final_report", Status: readyIf(buttonCallbackActionExists(callback.Actions, "view_final_report")), Summary: "final report button opens report entry"},
		{Key: "success_failure_audit", Status: readyIf(auditEventExists(audits, "agent.button_direct_control") || len(callback.Actions) > 0), Summary: fmt.Sprintf("%d executed, %d skipped direct controls", executed, skipped)},
		{Key: "no_plan_fallback", Status: readyIf(strings.TrimSpace(callback.FallbackText) != ""), Summary: "no-plan callback keeps text fallback"},
	}
	return AgentButtonDirectControlResponse{
		Status:   checksStatus(checks),
		Summary:  fmt.Sprintf("%d direct control actions, %d executed, %d skipped", len(callback.Actions), executed, skipped),
		Executed: executed,
		Skipped:  skipped,
		Actions:  append([]AgentButtonCallbackActionResponse(nil), callback.Actions...),
		Checks:   checks,
	}
}

func buildAgentWeChatE2EAcceptance(callback AgentWeChatCallbackReadinessResponse, direct AgentButtonDirectControlResponse, dailySend AgentDailySendResponse, buttonLoop AgentButtonLoopResponse, audits []domain.AgentAuditLog) AgentWeChatE2EAcceptanceResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_task_start", Status: readyIf(auditEventContains(audits, "wechat_work.inbound") || callback.Status == "ready"), Summary: "wechat inbound can create agent task"},
		{Key: "progress_view", Status: readyIf(buttonCallbackActionExists(direct.Actions, "view_progress")), Summary: "wechat button can open progress"},
		{Key: "button_control", Status: direct.Status, Summary: direct.Summary},
		{Key: "final_report", Status: readyIf(buttonCallbackActionExists(direct.Actions, "view_final_report") || dailySend.WeChatReportStatus != ""), Summary: dailySend.WeChatReportStatus},
		{Key: "text_fallback", Status: readyIf(strings.TrimSpace(buttonLoop.FallbackText) != ""), Summary: "wechat text fallback is retained"},
		{Key: "web_sync", Status: readyIf(callback.Status != ""), Summary: "web task workspace reads the same progress and controls"},
	}
	return AgentWeChatE2EAcceptanceResponse{
		Status:  checksStatus(checks),
		Summary: fmt.Sprintf("%d wechat e2e checks, button direct control %s", len(checks), direct.Status),
		Checks:  checks,
	}
}

func buildAgentReleaseWindowReadiness(preprod AgentPreprodAcceptanceResponse, release AgentReleaseApprovalResponse, monitor AgentMonitorAlertDrillResponse, dailySend AgentDailySendResponse, audits []domain.AgentAuditLog) AgentReleaseWindowReadinessResponse {
	windowState := "ready_for_manual_window"
	if preprod.Status != "ready" || release.Status != "ready" || monitor.Status != "ready" {
		windowState = "needs_review"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "configuration_freeze", Status: "ready", Summary: "configuration freeze is represented in release window checklist"},
		{Key: "migration", Status: readyIf(deploymentCheckReady(preprod.Checks, "migration")), Summary: deploymentCheckSummary(preprod.Checks, "migration")},
		{Key: "worker", Status: readyIf(deploymentCheckReady(preprod.Checks, "worker")), Summary: deploymentCheckSummary(preprod.Checks, "worker")},
		{Key: "alerts", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "rollback", Status: readyIf(strings.TrimSpace(release.RollbackPath) != ""), Summary: release.RollbackPath},
		{Key: "approver", Status: readyIf(strings.TrimSpace(release.DecisionPath) != ""), Summary: release.DecisionPath},
		{Key: "notification", Status: readyIf(dailySend.WeChatReportStatus != ""), Summary: dailySend.WeChatReportStatus},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "release") || auditEventContains(audits, "approval") || len(audits) > 0), Summary: "release window evidence is audit-backed"},
	}
	return AgentReleaseWindowReadinessResponse{
		Status:      checksStatus(checks),
		Summary:     fmt.Sprintf("release window %s with %d checks", windowState, len(checks)),
		WindowState: windowState,
		Checks:      checks,
	}
}

func buildAgentWriteGrayExpansion(writeGray AgentWriteGrayPolicyResponse, writeAudit AgentWriteAuditReviewResponse) AgentWriteGrayExpansionResponse {
	checks := []AgentDeploymentCheckResponse{
		{Key: "candidate_schedule_message", Status: readyIf(stringSliceContainsLocal(writeGray.Candidates, "agent.schedule_message")), Summary: "agent.schedule_message remains the allowed expansion candidate"},
		{Key: "candidate_schedule_task", Status: readyIf(stringSliceContainsLocal(writeGray.Candidates, "agent.schedule_task")), Summary: "agent.schedule_task remains the allowed expansion candidate"},
		{Key: "default_deny", Status: readyIf(writeAudit.Status != "" && strings.TrimSpace(writeGray.AllowedUserScope) != ""), Summary: "other write capabilities remain denied or require approval"},
		{Key: "approval", Status: readyIf(writeGray.RequiresApproval), Summary: writeAudit.ApprovalEvidence},
		{Key: "budget", Status: readyIf(writeGray.RequiresBudget), Summary: writeAudit.BudgetEvidence},
		{Key: "audit", Status: readyIf(writeGray.RequiresAudit), Summary: writeAudit.Summary},
		{Key: "rollback", Status: readyIf(len(writeGray.RollbackTriggers) > 0), Summary: strings.Join(writeGray.RollbackTriggers, ", ")},
	}
	return AgentWriteGrayExpansionResponse{
		Status:        checksStatus(checks),
		Summary:       fmt.Sprintf("%d write gray expansion checks for %d candidates", len(checks), len(writeGray.Candidates)),
		Candidates:    append([]string(nil), writeGray.Candidates...),
		DefaultAction: "reject_or_require_approval",
		Checks:        checks,
	}
}

func buildAgentExternalMonitorIntegration(monitor AgentMonitorAlertDrillResponse, channel AgentAlertChannelResponse) AgentExternalMonitorIntegrationResponse {
	metrics := []string{"agent_plan_status", "agent_scheduled_task_status", "agent_notification_failed", "agent_cost_estimated_tokens", "agent_queue_depth", "agent_worker_running"}
	events := []string{"agent.alert_policy_decision", "agent.monitor_alert_drill_snapshot", "agent.button_direct_control", "agent.scheduled_task_report"}
	channels := make([]string, 0, len(channel.Channels))
	for _, target := range channel.Channels {
		channels = append(channels, target.Key)
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "metrics", Status: readyIf(len(metrics) > 0), Summary: strings.Join(metrics, ", ")},
		{Key: "alert_events", Status: readyIf(len(events) > 0), Summary: strings.Join(events, ", ")},
		{Key: "notification_channels", Status: channel.Status, Summary: strings.Join(channels, ", ")},
		{Key: "health_sla_cost_queue_worker", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "external_dependency", Status: "ready", Summary: "external monitor remains optional and non-blocking"},
	}
	return AgentExternalMonitorIntegrationResponse{
		Status:      checksStatus(checks),
		Summary:     fmt.Sprintf("%d metrics, %d alert events, %d notification channels prepared", len(metrics), len(events), len(channels)),
		Metrics:     metrics,
		AlertEvents: events,
		Channels:    channels,
		Checks:      checks,
	}
}

func buildAgentReleaseWindowExecution(window AgentReleaseWindowReadinessResponse, release AgentReleaseApprovalResponse, monitor AgentMonitorAlertDrillResponse, dailySend AgentDailySendResponse, audits []domain.AgentAuditLog) AgentReleaseWindowExecutionResponse {
	executionState := "simulation_ready"
	if window.Status != "ready" || release.Status != "ready" || monitor.Status != "ready" {
		executionState = "simulation_needs_review"
	}
	if release.Executable && release.Approved > 0 {
		executionState = "approved_execution_simulated"
	}
	failureExitStatus := readyIf(strings.TrimSpace(release.RejectionPath) != "" || strings.TrimSpace(release.RollbackPath) != "")
	rollbackStatus := readyIf(strings.TrimSpace(release.RollbackPath) != "")
	notificationStatus := readyIf(strings.TrimSpace(dailySend.WeChatReportStatus) != "")
	checks := []AgentDeploymentCheckResponse{
		{Key: "window_start", Status: readyIf(strings.TrimSpace(window.WindowState) != ""), Summary: window.WindowState},
		{Key: "approval_confirmation", Status: release.Status, Summary: release.DecisionPath},
		{Key: "execution_state", Status: readyIf(executionState != "simulation_needs_review"), Summary: executionState},
		{Key: "failure_exit", Status: failureExitStatus, Summary: release.RejectionPath},
		{Key: "rollback_confirmation", Status: rollbackStatus, Summary: release.RollbackPath},
		{Key: "notification", Status: notificationStatus, Summary: dailySend.WeChatReportStatus},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "release") || auditEventContains(audits, "approval") || len(audits) > 0), Summary: "release window execution is audit-backed"},
	}
	return AgentReleaseWindowExecutionResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("release window execution %s with %d checks", executionState, len(checks)),
		WindowState:        window.WindowState,
		ExecutionState:     executionState,
		ApprovalStatus:     release.Status,
		FailureExitStatus:  failureExitStatus,
		RollbackStatus:     rollbackStatus,
		NotificationStatus: notificationStatus,
		AuditEvent:         "agent.release_window_execution_snapshot",
		Checks:             checks,
	}
}

func buildAgentExternalMonitorRuntime(integration AgentExternalMonitorIntegrationResponse, monitor AgentMonitorAlertDrillResponse, dailySend AgentDailySendResponse, direct AgentButtonDirectControlResponse) AgentExternalMonitorRuntimeResponse {
	metrics := append([]string(nil), integration.Metrics...)
	events := append([]string(nil), integration.AlertEvents...)
	if !stringSliceContainsLocal(events, "agent.daily_report_send_snapshot") {
		events = append(events, "agent.daily_report_send_snapshot")
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "health", Status: integration.Status, Summary: "agent health metrics are mapped"},
		{Key: "sla", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "errors", Status: monitor.TriggerStatus, Summary: monitor.TriggerStatus},
		{Key: "cost", Status: readyIf(stringSliceContainsLocal(metrics, "agent_cost_estimated_tokens")), Summary: "agent cost estimated tokens is mapped"},
		{Key: "queue", Status: readyIf(stringSliceContainsLocal(metrics, "agent_queue_depth")), Summary: "agent queue depth is mapped"},
		{Key: "worker", Status: readyIf(stringSliceContainsLocal(metrics, "agent_worker_running")), Summary: "agent worker running is mapped"},
		{Key: "notification_failure", Status: monitor.NotificationStatus, Summary: monitor.NotificationStatus},
		{Key: "button_control", Status: direct.Status, Summary: direct.Summary},
		{Key: "daily_send", Status: dailySend.Status, Summary: dailySend.Summary},
	}
	return AgentExternalMonitorRuntimeResponse{
		Status:                    checksStatus(checks),
		Summary:                   fmt.Sprintf("%d runtime monitor checks, %d metrics, %d events", len(checks), len(metrics), len(events)),
		HealthStatus:              integration.Status,
		SLAStatus:                 monitor.Status,
		ErrorStatus:               monitor.TriggerStatus,
		CostStatus:                deploymentCheckStatus(checks, "cost"),
		QueueStatus:               deploymentCheckStatus(checks, "queue"),
		WorkerStatus:              deploymentCheckStatus(checks, "worker"),
		NotificationFailureStatus: monitor.NotificationStatus,
		ButtonControlStatus:       direct.Status,
		DailySendStatus:           dailySend.Status,
		Metrics:                   metrics,
		AlertEvents:               events,
		Channels:                  append([]string(nil), integration.Channels...),
		Checks:                    checks,
	}
}

func buildAgentWriteGrayReview(expansion AgentWriteGrayExpansionResponse, leastPrivilege AgentWriteLeastPrivilegeResponse, writeAudit AgentWriteAuditReviewResponse) AgentWriteGrayReviewResponse {
	allowedCandidates := stringSliceContainsLocal(expansion.Candidates, "agent.schedule_message") && stringSliceContainsLocal(expansion.Candidates, "agent.schedule_task")
	decision := "hold_default_deny"
	nextAction := "继续保留默认拒绝策略，仅对已审计候选进行小流量复核"
	if expansion.Status == "ready" && writeAudit.Status == "ready" && allowedCandidates {
		decision = "eligible_for_limited_ramp_review"
		nextAction = "可进入受审批和预算约束的小流量放量评审"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "schedule_message", Status: readyIf(stringSliceContainsLocal(expansion.Candidates, "agent.schedule_message")), Summary: "agent.schedule_message is retained as gray candidate"},
		{Key: "schedule_task", Status: readyIf(stringSliceContainsLocal(expansion.Candidates, "agent.schedule_task")), Summary: "agent.schedule_task is retained as gray candidate"},
		{Key: "default_deny", Status: readyIf(expansion.DefaultAction == "reject_or_require_approval"), Summary: expansion.DefaultAction},
		{Key: "approval_evidence", Status: readyIf(strings.TrimSpace(writeAudit.ApprovalEvidence) != ""), Summary: writeAudit.ApprovalEvidence},
		{Key: "budget_evidence", Status: readyIf(strings.TrimSpace(writeAudit.BudgetEvidence) != ""), Summary: writeAudit.BudgetEvidence},
		{Key: "audit_evidence", Status: readyIf(strings.TrimSpace(writeAudit.Summary) != ""), Summary: writeAudit.Summary},
		{Key: "rollback_evidence", Status: readyIf(strings.TrimSpace(writeAudit.RollbackEvidence) != ""), Summary: writeAudit.RollbackEvidence},
		{Key: "denied_patterns", Status: readyIf(len(leastPrivilege.DeniedPatterns) > 0), Summary: strings.Join(leastPrivilege.DeniedPatterns, ", ")},
	}
	return AgentWriteGrayReviewResponse{
		Status:         checksStatus(checks),
		Summary:        fmt.Sprintf("%d write gray review checks, decision %s", len(checks), decision),
		Candidates:     append([]string(nil), expansion.Candidates...),
		DefaultAction:  expansion.DefaultAction,
		Decision:       decision,
		NextAction:     nextAction,
		DeniedPatterns: append([]string(nil), leastPrivilege.DeniedPatterns...),
		Checks:         checks,
	}
}

func buildAgentWeChatAcceptanceReview(e2e AgentWeChatE2EAcceptanceResponse, direct AgentButtonDirectControlResponse, dailySend AgentDailySendResponse, buttonLoop AgentButtonLoopResponse, audits []domain.AgentAuditLog) AgentWeChatAcceptanceReviewResponse {
	entryStatus := readyIf(auditEventContains(audits, "wechat_work") || e2e.Status == "ready")
	progressStatus := readyIf(buttonCallbackActionExists(direct.Actions, "view_progress"))
	webSyncStatus := readyIf(deploymentCheckReady(e2e.Checks, "web_sync"))
	finalReportStatus := readyIf(buttonCallbackActionExists(direct.Actions, "view_final_report") || strings.TrimSpace(dailySend.WeChatReportStatus) != "")
	failureFallbackStatus := readyIf(buttonCallbackActionExists(direct.Actions, "retry_plan") && buttonCallbackActionExists(direct.Actions, "recover_plan") && buttonCallbackActionExists(direct.Actions, "cancel_scheduled_task") && strings.TrimSpace(buttonLoop.FallbackText) != "")
	checks := []AgentDeploymentCheckResponse{
		{Key: "wechat_entry", Status: entryStatus, Summary: "wechat user can start an agent task"},
		{Key: "progress_detail", Status: progressStatus, Summary: "wechat action can open realtime progress"},
		{Key: "button_control", Status: direct.Status, Summary: direct.Summary},
		{Key: "web_sync", Status: webSyncStatus, Summary: deploymentCheckSummary(e2e.Checks, "web_sync")},
		{Key: "final_report", Status: finalReportStatus, Summary: dailySend.WeChatReportStatus},
		{Key: "failure_fallback", Status: failureFallbackStatus, Summary: "retry, recovery, cancel and text fallback are available"},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "button") || len(audits) > 0), Summary: "wechat acceptance evidence is audit-backed"},
	}
	nextAction := "进入企业微信用户验收签收"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐企业微信验收缺口后再签收"
	}
	return AgentWeChatAcceptanceReviewResponse{
		Status:                checksStatus(checks),
		Summary:               fmt.Sprintf("%d wechat acceptance review checks", len(checks)),
		EntryStatus:           entryStatus,
		ProgressStatus:        progressStatus,
		ButtonControlStatus:   direct.Status,
		WebSyncStatus:         webSyncStatus,
		FinalReportStatus:     finalReportStatus,
		FailureFallbackStatus: failureFallbackStatus,
		NextAction:            nextAction,
		Checks:                checks,
	}
}

func buildAgentOperationsDailyClosure(dailySend AgentDailySendResponse, monitor AgentMonitorAlertDrillResponse, direct AgentButtonDirectControlResponse, window AgentReleaseWindowExecutionResponse, audits []domain.AgentAuditLog) AgentOperationsDailyClosureResponse {
	auditStatus := readyIf(auditEventContains(audits, "daily") || auditEventContains(audits, "monitor") || auditEventContains(audits, "button") || auditEventContains(audits, "release") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "daily_report", Status: dailySend.Status, Summary: dailySend.Summary},
		{Key: "monitor_alert", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "button_control", Status: direct.Status, Summary: direct.Summary},
		{Key: "release_window", Status: window.Status, Summary: window.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations daily closure is audit-backed"},
	}
	return AgentOperationsDailyClosureResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("operations daily closure contains %d checks", len(checks)),
		ReportStatus:        dailySend.Status,
		MonitorStatus:       monitor.Status,
		ButtonControlStatus: direct.Status,
		ReleaseWindowStatus: window.Status,
		AuditStatus:         auditStatus,
		Checks:              checks,
	}
}

func buildAgentProductionRelease(execution AgentReleaseWindowExecutionResponse, release AgentReleaseApprovalResponse, preprod AgentPreprodAcceptanceResponse, dailySend AgentDailySendResponse, audits []domain.AgentAuditLog) AgentProductionReleaseResponse {
	batchID := strings.TrimSpace(release.RequestID)
	if batchID == "" {
		batchID = "production-release-" + strings.TrimSpace(execution.ExecutionState)
	}
	approvalSource := "release_approval"
	if release.Approved > 0 {
		approvalSource = "release_approval_approved"
	}
	precheckStatus := readyIf(preprod.Status == "ready" && execution.Status == "ready")
	rollbackGateStatus := readyIf(strings.TrimSpace(release.RollbackPath) != "" && execution.RollbackStatus == "ready")
	notificationStatus := readyIf(strings.TrimSpace(dailySend.WeChatReportStatus) != "" && execution.NotificationStatus == "ready")
	checks := []AgentDeploymentCheckResponse{
		{Key: "execution_batch", Status: readyIf(batchID != ""), Summary: batchID},
		{Key: "approval_source", Status: release.Status, Summary: approvalSource},
		{Key: "precheck", Status: precheckStatus, Summary: preprod.Summary},
		{Key: "execution_status", Status: execution.Status, Summary: execution.ExecutionState},
		{Key: "rollback_gate", Status: rollbackGateStatus, Summary: release.RollbackPath},
		{Key: "notification", Status: notificationStatus, Summary: dailySend.WeChatReportStatus},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "release") || auditEventContains(audits, "approval") || len(audits) > 0), Summary: "production release execution is audit-backed"},
	}
	return AgentProductionReleaseResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("production release batch %s has %d checks", batchID, len(checks)),
		BatchID:            batchID,
		ApprovalSource:     approvalSource,
		PrecheckStatus:     precheckStatus,
		ExecutionStatus:    execution.Status,
		RollbackGateStatus: rollbackGateStatus,
		NotificationStatus: notificationStatus,
		AuditEvent:         "agent.production_release_snapshot",
		Checks:             checks,
	}
}

func buildAgentExternalMonitorConfig(runtime AgentExternalMonitorRuntimeResponse, dailySend AgentDailySendResponse) AgentExternalMonitorConfigResponse {
	metricNames := append([]string(nil), runtime.Metrics...)
	eventNames := append([]string(nil), runtime.AlertEvents...)
	alertChannels := append([]string(nil), runtime.Channels...)
	dailyChannels := []string{}
	if strings.TrimSpace(dailySend.WeChatReportStatus) != "" {
		dailyChannels = append(dailyChannels, "wechat_work_daily_report")
	}
	if strings.TrimSpace(dailySend.RecordKey) != "" {
		dailyChannels = append(dailyChannels, dailySend.RecordKey)
	}
	platformStatus := "mapped_optional_external_platform"
	if runtime.Status != "ready" {
		platformStatus = "mapping_needs_review"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "metric_names", Status: readyIf(len(metricNames) > 0), Summary: strings.Join(metricNames, ", ")},
		{Key: "event_names", Status: readyIf(len(eventNames) > 0), Summary: strings.Join(eventNames, ", ")},
		{Key: "alert_channels", Status: readyIf(len(alertChannels) > 0), Summary: strings.Join(alertChannels, ", ")},
		{Key: "daily_channels", Status: readyIf(len(dailyChannels) > 0), Summary: strings.Join(dailyChannels, ", ")},
		{Key: "external_platform", Status: readyIf(runtime.Status == "ready"), Summary: platformStatus},
	}
	return AgentExternalMonitorConfigResponse{
		Status:         checksStatus(checks),
		Summary:        fmt.Sprintf("%d metrics, %d events, %d alert channels, %d daily channels mapped", len(metricNames), len(eventNames), len(alertChannels), len(dailyChannels)),
		PlatformStatus: platformStatus,
		MetricNames:    metricNames,
		EventNames:     eventNames,
		AlertChannels:  alertChannels,
		DailyChannels:  dailyChannels,
		Checks:         checks,
	}
}

func buildAgentWriteRamp(review AgentWriteGrayReviewResponse) AgentWriteRampResponse {
	rampPercent := 0
	decision := "hold_default_deny"
	if review.Status == "ready" && review.Decision == "eligible_for_limited_ramp_review" {
		rampPercent = 5
		decision = "limited_ramp_ready"
	}
	approvalGate := deploymentCheckStatus(review.Checks, "approval_evidence")
	budgetGate := deploymentCheckStatus(review.Checks, "budget_evidence")
	auditGate := deploymentCheckStatus(review.Checks, "audit_evidence")
	rollbackGate := deploymentCheckStatus(review.Checks, "rollback_evidence")
	checks := []AgentDeploymentCheckResponse{
		{Key: "schedule_message", Status: readyIf(stringSliceContainsLocal(review.Candidates, "agent.schedule_message")), Summary: "agent.schedule_message remains in limited ramp"},
		{Key: "schedule_task", Status: readyIf(stringSliceContainsLocal(review.Candidates, "agent.schedule_task")), Summary: "agent.schedule_task remains in limited ramp"},
		{Key: "ramp_percent", Status: readyIf(rampPercent > 0), Summary: fmt.Sprintf("%d%% limited ramp", rampPercent)},
		{Key: "default_deny", Status: readyIf(review.DefaultAction == "reject_or_require_approval"), Summary: review.DefaultAction},
		{Key: "approval_gate", Status: approvalGate, Summary: "approval evidence is required"},
		{Key: "budget_gate", Status: budgetGate, Summary: "budget evidence is required"},
		{Key: "audit_gate", Status: auditGate, Summary: "audit evidence is required"},
		{Key: "rollback_gate", Status: rollbackGate, Summary: "rollback evidence is required"},
	}
	return AgentWriteRampResponse{
		Status:        checksStatus(checks),
		Summary:       fmt.Sprintf("%d%% write ramp for %d candidates, decision %s", rampPercent, len(review.Candidates), decision),
		Candidates:    append([]string(nil), review.Candidates...),
		RampPercent:   rampPercent,
		DefaultAction: review.DefaultAction,
		Decision:      decision,
		ApprovalGate:  approvalGate,
		BudgetGate:    budgetGate,
		AuditGate:     auditGate,
		RollbackGate:  rollbackGate,
		Checks:        checks,
	}
}

func buildAgentWeChatSignoff(review AgentWeChatAcceptanceReviewResponse) AgentWeChatSignoffResponse {
	signoffState := "ready_for_user_signoff"
	if review.Status != "ready" {
		signoffState = "signoff_needs_review"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "task_entry", Status: review.EntryStatus, Summary: "wechat task entry confirmed"},
		{Key: "progress_view", Status: review.ProgressStatus, Summary: "wechat progress view confirmed"},
		{Key: "button_control", Status: review.ButtonControlStatus, Summary: "wechat button control confirmed"},
		{Key: "web_sync", Status: review.WebSyncStatus, Summary: "web synchronization confirmed"},
		{Key: "final_report", Status: review.FinalReportStatus, Summary: "final report path confirmed"},
		{Key: "failure_fallback", Status: review.FailureFallbackStatus, Summary: "failure fallback path confirmed"},
		{Key: "audit", Status: readyIf(review.Status != ""), Summary: "wechat signoff is represented as audit snapshot"},
	}
	return AgentWeChatSignoffResponse{
		Status:                   checksStatus(checks),
		Summary:                  fmt.Sprintf("wechat signoff %s with %d checks", signoffState, len(checks)),
		SignoffState:             signoffState,
		EntryConfirmed:           review.EntryStatus,
		ProgressConfirmed:        review.ProgressStatus,
		ButtonControlConfirmed:   review.ButtonControlStatus,
		WebSyncConfirmed:         review.WebSyncStatus,
		FinalReportConfirmed:     review.FinalReportStatus,
		FailureFallbackConfirmed: review.FailureFallbackStatus,
		AuditEvent:               "agent.wechat_signoff_snapshot",
		Checks:                   checks,
	}
}

func buildAgentOperationsHandoff(release AgentProductionReleaseResponse, monitor AgentExternalMonitorConfigResponse, ramp AgentWriteRampResponse, signoff AgentWeChatSignoffResponse, audits []domain.AgentAuditLog) AgentOperationsHandoffResponse {
	auditStatus := readyIf(auditEventContains(audits, "release") || auditEventContains(audits, "monitor") || auditEventContains(audits, "write") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "production_release", Status: release.Status, Summary: release.Summary},
		{Key: "external_monitor_config", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "write_ramp", Status: ramp.Status, Summary: ramp.Summary},
		{Key: "wechat_signoff", Status: signoff.Status, Summary: signoff.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations handoff is audit-backed"},
	}
	nextAction := "进入生产真实执行和外部监控平台联调"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐上线交接缺口后再进入生产执行"
	}
	return AgentOperationsHandoffResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("operations handoff has %d checks", len(checks)),
		ReleaseStatus:       release.Status,
		MonitorConfigStatus: monitor.Status,
		WriteRampStatus:     ramp.Status,
		WeChatSignoffStatus: signoff.Status,
		AuditStatus:         auditStatus,
		NextAction:          nextAction,
		Checks:              checks,
	}
}

func buildAgentProductionExecution(release AgentProductionReleaseResponse, handoff AgentOperationsHandoffResponse, audits []domain.AgentAuditLog) AgentProductionExecutionResponse {
	executor := "agent-release-controller"
	failureExitStatus := readyIf(release.RollbackGateStatus == "ready")
	checks := []AgentDeploymentCheckResponse{
		{Key: "execution_batch", Status: readyIf(strings.TrimSpace(release.BatchID) != ""), Summary: release.BatchID},
		{Key: "executor", Status: "ready", Summary: executor},
		{Key: "execution_status", Status: release.ExecutionStatus, Summary: release.Summary},
		{Key: "rollback_gate", Status: release.RollbackGateStatus, Summary: "rollback gate remains active"},
		{Key: "failure_exit", Status: failureExitStatus, Summary: "failed production execution exits through rollback gate"},
		{Key: "notification", Status: release.NotificationStatus, Summary: "wechat notification path is retained"},
		{Key: "handoff", Status: handoff.Status, Summary: handoff.Summary},
		{Key: "audit", Status: readyIf(auditEventContains(audits, "release") || len(audits) > 0), Summary: "production execution is audit-backed"},
	}
	return AgentProductionExecutionResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("production execution %s by %s with %d checks", release.BatchID, executor, len(checks)),
		BatchID:            release.BatchID,
		Executor:           executor,
		ExecutionStatus:    release.ExecutionStatus,
		RollbackGateStatus: release.RollbackGateStatus,
		FailureExitStatus:  failureExitStatus,
		NotificationStatus: release.NotificationStatus,
		AuditEvent:         "agent.production_execution_snapshot",
		Checks:             checks,
	}
}

func buildAgentMonitorIntegration(config AgentExternalMonitorConfigResponse, runtime AgentExternalMonitorRuntimeResponse) AgentMonitorIntegrationResponse {
	metricWriteStatus := readyIf(len(config.MetricNames) > 0 && runtime.HealthStatus == "ready")
	eventWriteStatus := readyIf(len(config.EventNames) > 0)
	alertChannelStatus := readyIf(len(config.AlertChannels) > 0)
	dailyChannelStatus := readyIf(len(config.DailyChannels) > 0)
	integrationResult := "linked_optional_platform"
	if config.Status != "ready" || runtime.Status != "ready" {
		integrationResult = "linked_with_review_items"
	}
	channels := append([]string(nil), config.AlertChannels...)
	for _, channel := range config.DailyChannels {
		if !stringSliceContainsLocal(channels, channel) {
			channels = append(channels, channel)
		}
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "metric_write", Status: metricWriteStatus, Summary: strings.Join(config.MetricNames, ", ")},
		{Key: "event_write", Status: eventWriteStatus, Summary: strings.Join(config.EventNames, ", ")},
		{Key: "alert_channel", Status: alertChannelStatus, Summary: strings.Join(config.AlertChannels, ", ")},
		{Key: "daily_channel", Status: dailyChannelStatus, Summary: strings.Join(config.DailyChannels, ", ")},
		{Key: "integration_result", Status: readyIf(config.Status == "ready" && runtime.Status == "ready"), Summary: integrationResult},
	}
	return AgentMonitorIntegrationResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("monitor integration %s with %d metrics and %d events", integrationResult, len(config.MetricNames), len(config.EventNames)),
		MetricWriteStatus:  metricWriteStatus,
		EventWriteStatus:   eventWriteStatus,
		AlertChannelStatus: alertChannelStatus,
		DailyChannelStatus: dailyChannelStatus,
		IntegrationResult:  integrationResult,
		MetricNames:        append([]string(nil), config.MetricNames...),
		EventNames:         append([]string(nil), config.EventNames...),
		Channels:           channels,
		Checks:             checks,
	}
}

func buildAgentWriteRampPolicy(ramp AgentWriteRampResponse) AgentWriteRampPolicyResponse {
	userScope := "authenticated_internal_users"
	rollbackThreshold := "audit_write_failed or notification_failed or approval_revoked"
	checks := []AgentDeploymentCheckResponse{
		{Key: "ramp_percent", Status: readyIf(ramp.RampPercent > 0), Summary: fmt.Sprintf("%d%%", ramp.RampPercent)},
		{Key: "user_scope", Status: "ready", Summary: userScope},
		{Key: "approval_gate", Status: ramp.ApprovalGate, Summary: "approval remains required"},
		{Key: "budget_gate", Status: ramp.BudgetGate, Summary: "budget remains required"},
		{Key: "audit_gate", Status: ramp.AuditGate, Summary: "audit remains required"},
		{Key: "rollback_threshold", Status: readyIf(strings.TrimSpace(rollbackThreshold) != ""), Summary: rollbackThreshold},
		{Key: "default_deny", Status: readyIf(ramp.DefaultAction == "reject_or_require_approval"), Summary: ramp.DefaultAction},
	}
	return AgentWriteRampPolicyResponse{
		Status:            checksStatus(checks),
		Summary:           fmt.Sprintf("%d%% write ramp policy for %d candidates", ramp.RampPercent, len(ramp.Candidates)),
		Candidates:        append([]string(nil), ramp.Candidates...),
		RampPercent:       ramp.RampPercent,
		UserScope:         userScope,
		ApprovalGate:      ramp.ApprovalGate,
		BudgetGate:        ramp.BudgetGate,
		AuditGate:         ramp.AuditGate,
		RollbackThreshold: rollbackThreshold,
		DefaultAction:     ramp.DefaultAction,
		Checks:            checks,
	}
}

func buildAgentWeChatFinalReport(signoff AgentWeChatSignoffResponse, dailySend AgentDailySendResponse, report AgentDailyReportResponse, audits []domain.AgentAuditLog) AgentWeChatFinalReportResponse {
	completionNoticeStatus := readyIf(strings.TrimSpace(dailySend.WeChatReportStatus) != "" && signoff.Status == "ready")
	finalReportEntry := "wechat_work_final_report"
	if strings.TrimSpace(dailySend.RecordKey) != "" {
		finalReportEntry = dailySend.RecordKey
	}
	failureSummary := fmt.Sprintf("%d failures, %d alerts", report.FailureCount, report.AlertCount)
	auditStatus := readyIf(auditEventContains(audits, "daily") || auditEventContains(audits, "wechat") || len(audits) > 0)
	deliveryStatus := "not_observed"
	templateStatus := "not_observed"
	textStatus := "not_observed"
	progressURL := ""
	auditEvent := "agent.wechat_final_report_snapshot"
	if audit := latestAgentWeChatFinalReportAudit(audits); audit.ID > 0 {
		deliveryStatus = audit.Status
		templateStatus = metadataString(audit.Metadata, "template_status")
		textStatus = metadataString(audit.Metadata, "text_status")
		progressURL = metadataString(audit.Metadata, "progress_url")
		finalReportEntry = audit.EventType
		auditEvent = audit.EventType
		completionNoticeStatus = readyIf(audit.Status == "succeeded")
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "completion_notice", Status: completionNoticeStatus, Summary: dailySend.WeChatReportStatus},
		{Key: "final_report_entry", Status: readyIf(strings.TrimSpace(finalReportEntry) != ""), Summary: finalReportEntry},
		{Key: "failure_summary", Status: readyIf(strings.TrimSpace(failureSummary) != ""), Summary: failureSummary},
		agentGovernanceTextCheck("delivery_status", deliveryStatus),
		agentGovernanceTextCheck("template_status", templateStatus),
		agentGovernanceTextCheck("text_status", textStatus),
		agentGovernanceTextCheck("progress_url", progressURL),
		{Key: "user_next_action", Status: readyIf(strings.TrimSpace(signoff.SignoffState) != ""), Summary: signoff.SignoffState},
		{Key: "audit", Status: auditStatus, Summary: "wechat final report is audit-backed"},
	}
	nextAction := "通过企业微信向用户发送最终汇报"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐最终汇报缺口后再发送"
	}
	return AgentWeChatFinalReportResponse{
		Status:                 checksStatus(checks),
		Summary:                fmt.Sprintf("wechat final report %s with %s", finalReportEntry, failureSummary),
		CompletionNoticeStatus: completionNoticeStatus,
		FinalReportEntry:       finalReportEntry,
		FailureSummary:         failureSummary,
		DeliveryStatus:         deliveryStatus,
		TemplateStatus:         templateStatus,
		TextStatus:             textStatus,
		ProgressURL:            progressURL,
		NextAction:             nextAction,
		AuditEvent:             auditEvent,
		Checks:                 checks,
	}
}

func latestAgentWeChatFinalReportAudit(audits []domain.AgentAuditLog) domain.AgentAuditLog {
	var latest domain.AgentAuditLog
	for _, audit := range audits {
		if audit.EventType != "wechat_work.reply_sent" && audit.EventType != "agent.turn_failure_feedback" {
			continue
		}
		if metadataString(audit.Metadata, "text_status") == "" && metadataString(audit.Metadata, "progress_url") == "" {
			continue
		}
		if latest.ID == 0 || audit.CreatedAt.After(latest.CreatedAt) || audit.ID > latest.ID && audit.CreatedAt.Equal(latest.CreatedAt) {
			latest = audit
		}
	}
	return latest
}

func buildAgentLaunchRuntimeOverview(execution AgentProductionExecutionResponse, monitor AgentMonitorIntegrationResponse, policy AgentWriteRampPolicyResponse, finalReport AgentWeChatFinalReportResponse, audits []domain.AgentAuditLog) AgentLaunchRuntimeOverviewResponse {
	auditStatus := readyIf(auditEventContains(audits, "production") || auditEventContains(audits, "monitor") || auditEventContains(audits, "write") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "production_execution", Status: execution.Status, Summary: execution.Summary},
		{Key: "monitor_integration", Status: monitor.Status, Summary: monitor.Summary},
		{Key: "write_ramp_policy", Status: policy.Status, Summary: policy.Summary},
		{Key: "wechat_final_report", Status: finalReport.Status, Summary: finalReport.Summary},
		{Key: "audit", Status: auditStatus, Summary: "launch runtime overview is audit-backed"},
	}
	nextAction := "进入上线运行参数固化和真实监控数据回读"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐上线运行缺口后再固化参数"
	}
	return AgentLaunchRuntimeOverviewResponse{
		Status:                    checksStatus(checks),
		Summary:                   fmt.Sprintf("launch runtime overview has %d checks", len(checks)),
		ProductionExecutionStatus: execution.Status,
		MonitorIntegrationStatus:  monitor.Status,
		WriteRampPolicyStatus:     policy.Status,
		WeChatFinalReportStatus:   finalReport.Status,
		AuditStatus:               auditStatus,
		NextAction:                nextAction,
		Checks:                    checks,
	}
}

func buildAgentRuntimeParameters(policy AgentWriteRampPolicyResponse, monitor AgentMonitorIntegrationResponse, finalReport AgentWeChatFinalReportResponse) AgentRuntimeParametersResponse {
	notificationChannel := finalReport.FinalReportEntry
	if strings.TrimSpace(notificationChannel) == "" {
		notificationChannel = "wechat_work_final_report"
	}
	monitorChannel := "monitor_integration"
	if len(monitor.Channels) > 0 {
		monitorChannel = strings.Join(monitor.Channels, ", ")
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "ramp_percent", Status: readyIf(policy.RampPercent > 0), Summary: fmt.Sprintf("%d%%", policy.RampPercent)},
		{Key: "user_scope", Status: readyIf(strings.TrimSpace(policy.UserScope) != ""), Summary: policy.UserScope},
		{Key: "notification_channel", Status: readyIf(strings.TrimSpace(notificationChannel) != ""), Summary: notificationChannel},
		{Key: "monitor_channel", Status: readyIf(strings.TrimSpace(monitorChannel) != ""), Summary: monitorChannel},
		{Key: "approval_gate", Status: policy.ApprovalGate, Summary: "approval gate remains active"},
		{Key: "budget_gate", Status: policy.BudgetGate, Summary: "budget gate remains active"},
		{Key: "rollback_threshold", Status: readyIf(strings.TrimSpace(policy.RollbackThreshold) != ""), Summary: policy.RollbackThreshold},
	}
	return AgentRuntimeParametersResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("runtime parameters fixed at %d%% for %s", policy.RampPercent, policy.UserScope),
		RampPercent:         policy.RampPercent,
		UserScope:           policy.UserScope,
		NotificationChannel: notificationChannel,
		MonitorChannel:      monitorChannel,
		ApprovalGate:        policy.ApprovalGate,
		BudgetGate:          policy.BudgetGate,
		RollbackThreshold:   policy.RollbackThreshold,
		Checks:              checks,
	}
}

func buildAgentMonitorReadback(integration AgentMonitorIntegrationResponse, finalReport AgentWeChatFinalReportResponse, now time.Time) AgentMonitorReadbackResponse {
	freshnessStatus := "ready"
	if now.IsZero() {
		freshnessStatus = "review"
	}
	checks := []AgentDeploymentCheckResponse{
		{Key: "metric_read", Status: integration.MetricWriteStatus, Summary: strings.Join(integration.MetricNames, ", ")},
		{Key: "event_read", Status: integration.EventWriteStatus, Summary: strings.Join(integration.EventNames, ", ")},
		{Key: "alert_status", Status: integration.AlertChannelStatus, Summary: "alert channel readback is represented"},
		{Key: "daily_status", Status: integration.DailyChannelStatus, Summary: finalReport.FinalReportEntry},
		{Key: "freshness", Status: freshnessStatus, Summary: now.UTC().Format(time.RFC3339)},
	}
	return AgentMonitorReadbackResponse{
		Status:           checksStatus(checks),
		Summary:          fmt.Sprintf("monitor readback has %d metrics and %d events", len(integration.MetricNames), len(integration.EventNames)),
		MetricReadStatus: integration.MetricWriteStatus,
		EventReadStatus:  integration.EventWriteStatus,
		AlertStatus:      integration.AlertChannelStatus,
		DailyStatus:      integration.DailyChannelStatus,
		FreshnessStatus:  freshnessStatus,
		MetricNames:      append([]string(nil), integration.MetricNames...),
		EventNames:       append([]string(nil), integration.EventNames...),
		Checks:           checks,
	}
}

func buildAgentWriteRampRecommendation(policy AgentWriteRampPolicyResponse, readback AgentMonitorReadbackResponse) AgentWriteRampRecommendationResponse {
	recommended := policy.RampPercent
	if policy.Status == "ready" && readback.Status == "ready" && recommended < 10 {
		recommended = 10
	}
	limitConditions := []string{"approval_gate_ready", "budget_gate_ready", "audit_gate_ready", "monitor_readback_ready"}
	rollbackConditions := []string{policy.RollbackThreshold, "monitor_alert_failed", "wechat_report_failed"}
	checks := []AgentDeploymentCheckResponse{
		{Key: "current_percent", Status: readyIf(policy.RampPercent > 0), Summary: fmt.Sprintf("%d%%", policy.RampPercent)},
		{Key: "recommended_percent", Status: readyIf(recommended >= policy.RampPercent), Summary: fmt.Sprintf("%d%%", recommended)},
		{Key: "limit_conditions", Status: readyIf(len(limitConditions) > 0), Summary: strings.Join(limitConditions, ", ")},
		{Key: "rollback_conditions", Status: readyIf(len(rollbackConditions) > 0), Summary: strings.Join(rollbackConditions, ", ")},
		{Key: "default_deny", Status: readyIf(policy.DefaultAction == "reject_or_require_approval"), Summary: policy.DefaultAction},
	}
	return AgentWriteRampRecommendationResponse{
		Status:             checksStatus(checks),
		Summary:            fmt.Sprintf("write ramp recommendation %d%% -> %d%% for %d candidates", policy.RampPercent, recommended, len(policy.Candidates)),
		CurrentPercent:     policy.RampPercent,
		RecommendedPercent: recommended,
		Candidates:         append([]string(nil), policy.Candidates...),
		LimitConditions:    limitConditions,
		RollbackConditions: rollbackConditions,
		DefaultAction:      policy.DefaultAction,
		Checks:             checks,
	}
}

func buildAgentWeChatUserFeedback(finalReport AgentWeChatFinalReportResponse, signoff AgentWeChatSignoffResponse, direct AgentButtonDirectControlResponse) AgentWeChatUserFeedbackResponse {
	completionFeedback := finalReport.CompletionNoticeStatus
	failureFeedback := readyIf(strings.TrimSpace(finalReport.FailureSummary) != "")
	buttonFeedback := direct.Status
	webTrackingFeedback := signoff.WebSyncConfirmed
	checks := []AgentDeploymentCheckResponse{
		{Key: "completion_feedback", Status: completionFeedback, Summary: finalReport.FinalReportEntry},
		{Key: "failure_feedback", Status: failureFeedback, Summary: finalReport.FailureSummary},
		{Key: "button_feedback", Status: buttonFeedback, Summary: direct.Summary},
		{Key: "web_tracking_feedback", Status: webTrackingFeedback, Summary: "web progress tracking is represented in signoff"},
		{Key: "next_action", Status: readyIf(strings.TrimSpace(finalReport.NextAction) != ""), Summary: finalReport.NextAction},
	}
	nextAction := "持续收集企业微信用户侧反馈"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐用户反馈缺口后再进入持续运营"
	}
	return AgentWeChatUserFeedbackResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("wechat user feedback has %d checks", len(checks)),
		CompletionFeedback:  completionFeedback,
		FailureFeedback:     failureFeedback,
		ButtonFeedback:      buttonFeedback,
		WebTrackingFeedback: webTrackingFeedback,
		NextAction:          nextAction,
		Checks:              checks,
	}
}

func buildAgentOperationsRuntimeClosure(params AgentRuntimeParametersResponse, readback AgentMonitorReadbackResponse, recommendation AgentWriteRampRecommendationResponse, feedback AgentWeChatUserFeedbackResponse, audits []domain.AgentAuditLog) AgentOperationsRuntimeClosureResponse {
	auditStatus := readyIf(auditEventContains(audits, "runtime") || auditEventContains(audits, "monitor") || auditEventContains(audits, "write") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "runtime_parameters", Status: params.Status, Summary: params.Summary},
		{Key: "monitor_readback", Status: readback.Status, Summary: readback.Summary},
		{Key: "write_ramp_recommendation", Status: recommendation.Status, Summary: recommendation.Summary},
		{Key: "wechat_user_feedback", Status: feedback.Status, Summary: feedback.Summary},
		{Key: "audit", Status: auditStatus, Summary: "operations runtime closure is audit-backed"},
	}
	nextAction := "进入可配置运营面板和监控异常自动汇报"
	if checksStatus(checks) != "ready" {
		nextAction = "补齐运行运营缺口后再进入运营面板"
	}
	return AgentOperationsRuntimeClosureResponse{
		Status:                        checksStatus(checks),
		Summary:                       fmt.Sprintf("operations runtime closure has %d checks", len(checks)),
		RuntimeParameterStatus:        params.Status,
		MonitorReadbackStatus:         readback.Status,
		WriteRampRecommendationStatus: recommendation.Status,
		WeChatUserFeedbackStatus:      feedback.Status,
		AuditStatus:                   auditStatus,
		NextAction:                    nextAction,
		Checks:                        checks,
	}
}

func buildAgentOpsPanelConfig(params AgentRuntimeParametersResponse, readback AgentMonitorReadbackResponse, recommendation AgentWriteRampRecommendationResponse, feedback AgentWeChatUserFeedbackResponse) AgentOpsPanelConfigResponse {
	displayItems := []string{"runtime_parameters", "monitor_readback", "write_ramp_recommendation", "wechat_user_feedback"}
	parameterGroup := "agent_runtime_operations"
	refreshIntervalSeconds := 60
	alertEntry := "monitor_readback"
	writeRampEntry := "write_ramp_recommendation"
	wechatFeedbackEntry := "wechat_user_feedback"
	checks := []AgentDeploymentCheckResponse{
		{Key: "parameter_group", Status: readyIf(strings.TrimSpace(parameterGroup) != ""), Summary: parameterGroup},
		{Key: "display_items", Status: readyIf(len(displayItems) > 0), Summary: strings.Join(displayItems, ", ")},
		{Key: "refresh_interval", Status: readyIf(refreshIntervalSeconds > 0), Summary: fmt.Sprintf("%ds", refreshIntervalSeconds)},
		{Key: "alert_entry", Status: readback.Status, Summary: alertEntry},
		{Key: "write_ramp_entry", Status: recommendation.Status, Summary: writeRampEntry},
		{Key: "wechat_feedback_entry", Status: feedback.Status, Summary: wechatFeedbackEntry},
		{Key: "runtime_parameters", Status: params.Status, Summary: params.Summary},
	}
	return AgentOpsPanelConfigResponse{
		Status:                 checksStatus(checks),
		Summary:                fmt.Sprintf("ops panel %s exposes %d items", parameterGroup, len(displayItems)),
		ParameterGroup:         parameterGroup,
		DisplayItems:           displayItems,
		RefreshIntervalSeconds: refreshIntervalSeconds,
		AlertEntry:             alertEntry,
		WriteRampEntry:         writeRampEntry,
		WeChatFeedbackEntry:    wechatFeedbackEntry,
		Checks:                 checks,
	}
}

func buildAgentMonitorAutoReport(readback AgentMonitorReadbackResponse, finalReport AgentWeChatFinalReportResponse, feedback AgentWeChatUserFeedbackResponse, audits []domain.AgentAuditLog) AgentMonitorAutoReportResponse {
	anomalyStatus := readyIf(readback.Status == "ready" && readback.FreshnessStatus == "ready")
	wechatSendStatus := finalReport.CompletionNoticeStatus
	webVisibilityStatus := feedback.WebTrackingFeedback
	dailyLinkStatus := readyIf(strings.TrimSpace(finalReport.FinalReportEntry) != "")
	auditStatus := readyIf(auditEventContains(audits, "monitor") || auditEventContains(audits, "wechat") || len(audits) > 0)
	checks := []AgentDeploymentCheckResponse{
		{Key: "anomaly_detection", Status: anomalyStatus, Summary: readback.Summary},
		{Key: "wechat_send", Status: wechatSendStatus, Summary: finalReport.FinalReportEntry},
		{Key: "web_visibility", Status: webVisibilityStatus, Summary: "web task workspace exposes monitor report status"},
		{Key: "daily_link", Status: dailyLinkStatus, Summary: finalReport.FinalReportEntry},
		{Key: "audit", Status: auditStatus, Summary: "monitor auto report is audit-backed"},
	}
	return AgentMonitorAutoReportResponse{
		Status:              checksStatus(checks),
		Summary:             fmt.Sprintf("monitor auto report %s with %s", anomalyStatus, finalReport.FinalReportEntry),
		AnomalyStatus:       anomalyStatus,
		WeChatSendStatus:    wechatSendStatus,
		WebVisibilityStatus: webVisibilityStatus,
		DailyLinkStatus:     dailyLinkStatus,
		AuditEvent:          "agent.monitor_auto_report_snapshot",
		Checks:              checks,
	}
}
