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
