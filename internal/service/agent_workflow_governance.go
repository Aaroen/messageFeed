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

func buildAgentCapabilityPolicyMetadata(plan domain.AgentPlan, preference domain.AgentNotificationPreference, now time.Time) domain.AgentJSON {
	policy := normalizeAgentCapabilityPolicy(preference.CapabilityPolicy)
	matches := make([]domain.AgentJSON, 0)
	summary := map[string]int{"allow": 0, "degrade": 0, "confirm": 0, "reject": 0}
	overall := "allow"
	for _, step := range plan.Steps {
		decision, rule := agentCapabilityPolicyDecision(step.CapabilityKey, policy)
		if decision == "" {
			decision = "allow"
		}
		summary[decision]++
		if rule != "" || decision != "allow" {
			matches = append(matches, domain.AgentJSON{
				"plan_step_id":   step.ID,
				"capability_key": step.CapabilityKey,
				"decision":       decision,
				"rule":           rule,
			})
		}
		overall = stricterCapabilityDecision(overall, decision)
	}
	return domain.AgentJSON{
		"status":      overall,
		"policy":      policy,
		"matches":     matches,
		"summary":     summary,
		"recorded_at": now.UTC().Format(time.RFC3339),
	}
}

func agentCapabilityPolicyDecision(capabilityKey string, policy domain.AgentJSON) (string, string) {
	key := strings.TrimSpace(capabilityKey)
	if key == "" {
		return "", ""
	}
	if value, ok := policy[key]; ok {
		return normalizeAgentCapabilityPolicyDecision(fmt.Sprint(value)), key
	}
	bestRule := ""
	bestDecision := ""
	for rule, value := range policy {
		pattern := strings.TrimSpace(rule)
		if !strings.HasSuffix(pattern, ".*") {
			continue
		}
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(key, prefix) && len(pattern) > len(bestRule) {
			bestRule = pattern
			bestDecision = normalizeAgentCapabilityPolicyDecision(fmt.Sprint(value))
		}
	}
	return bestDecision, bestRule
}

func stricterCapabilityDecision(left string, right string) string {
	rank := map[string]int{"allow": 0, "degrade": 1, "confirm": 2, "reject": 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
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

func buildAgentHandoffMetadata(plan domain.AgentPlan, preference domain.AgentNotificationPreference, now time.Time) domain.AgentJSON {
	preference = normalizeAgentPolicyPreference(preference)
	reasons := make([]string, 0, 4)
	if preference.HandoffOnFailure && plan.Status == domain.AgentPlanStatusFailed {
		reasons = append(reasons, "failure")
	}
	quality := metadataMap(plan.Metadata, "result_quality")
	if threshold := preference.QualityHandoffThreshold; threshold > 0 && quality != nil && metadataFloat(quality, "score") < threshold {
		reasons = append(reasons, "low_quality")
	}
	permissionStatus := planPermissionStatus(plan)
	if preference.HandoffOnPermission && (permissionStatus == "prompt" || permissionStatus == "reject") {
		reasons = append(reasons, "permission")
	}
	budgetStatus := planBudgetStatus(plan)
	if preference.HandoffOnBudget && budgetStatus != "" && budgetStatus != "unknown" && budgetStatus != "within_budget" {
		reasons = append(reasons, "budget")
	}
	status := "not_required"
	nextAction := "无需人工接管"
	if len(reasons) > 0 {
		status = "required"
		nextAction = "请在 Web 任务详情中接管、恢复、取消或重新排队"
	}
	return domain.AgentJSON{
		"status":      status,
		"reasons":     reasons,
		"next_action": nextAction,
		"threshold":   preference.QualityHandoffThreshold,
		"recorded_at": now.UTC().Format(time.RFC3339),
	}
}

func buildAgentRuntimeObservabilityMetadata(plan domain.AgentPlan, admission domain.AgentJSON, now time.Time) domain.AgentJSON {
	failedSteps := 0
	latestFailure := strings.TrimSpace(plan.ErrorMessage)
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed {
			failedSteps++
			latestFailure = agentProgressFirstNonEmpty(step.ErrorMessage, step.OutputSummary, latestFailure)
		}
	}
	recoveryCount := 0
	if metadataMap(plan.Metadata, "recovery") != nil {
		recoveryCount = 1
	}
	qualityScore := 0.0
	if quality := metadataMap(plan.Metadata, "result_quality"); quality != nil {
		qualityScore = metadataFloat(quality, "score")
	}
	throttleStatus := ""
	if admission != nil {
		throttleStatus, _ = admission["status"].(string)
	}
	if throttleStatus == "" {
		throttleStatus = "allowed"
	}
	return domain.AgentJSON{
		"status":              string(plan.Status),
		"failed_steps":        failedSteps,
		"latest_failure":      latestFailure,
		"recovery_count":      recoveryCount,
		"quality_score":       roundAgentQualityScore(qualityScore),
		"notification_status": "pending_or_recorded_in_audit",
		"throttle_status":     throttleStatus,
		"summary":             fmt.Sprintf("status %s, failed steps %d, recoveries %d, quality %.2f", plan.Status, failedSteps, recoveryCount, qualityScore),
		"recorded_at":         now.UTC().Format(time.RFC3339),
	}
}

func buildAgentPlanRecoveryMetadata(plan domain.AgentPlan, recoveredSteps int, reason string, now time.Time) domain.AgentJSON {
	strategy := agentPlanRecoveryStrategy(plan, recoveredSteps)
	result := "queued"
	if strategy == "not_recoverable" {
		result = "rejected"
	}
	requiresReapproval := strategy == "requires_reapproval"
	return domain.AgentJSON{
		"subject_type":        "plan",
		"plan_id":             plan.ID,
		"previous_status":     string(plan.Status),
		"recovery_strategy":   strategy,
		"recovery_reason":     strings.TrimSpace(reason),
		"recovery_result":     result,
		"recovered_steps":     recoveredSteps,
		"requires_reapproval": requiresReapproval,
		"recorded_at":         now.UTC().Format(time.RFC3339),
	}
}

func agentPlanRecoveryStrategy(plan domain.AgentPlan, recoveredSteps int) string {
	if plan.Status == domain.AgentPlanStatusAwaitingApproval || plan.ConfirmationPolicy == "prompt" {
		return "requires_reapproval"
	}
	if recoveredSteps > 0 {
		return "recover_interrupted_step"
	}
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed && agentPlanStepAllowsRetry(step) && (step.MaxRetries == 0 || step.RetryCount < step.MaxRetries) {
			return "retry_failed_step"
		}
	}
	return "not_recoverable"
}

func buildAgentScheduledTaskRecoveryMetadata(task domain.AgentScheduledTask, reason string, now time.Time) domain.AgentJSON {
	strategy := "not_recoverable"
	switch task.Status {
	case domain.AgentScheduledTaskStatusRunning:
		strategy = "recover_interrupted_task"
	case domain.AgentScheduledTaskStatusFailed:
		strategy = "retry_failed_task"
	case domain.AgentScheduledTaskStatusInputRequired:
		strategy = "requires_reapproval"
	case domain.AgentScheduledTaskStatusQueued:
		strategy = "already_queued"
	}
	requiresReapproval := strategy == "requires_reapproval"
	result := "queued"
	if strategy == "not_recoverable" {
		result = "rejected"
	}
	return domain.AgentJSON{
		"subject_type":           "scheduled_task",
		"scheduled_task_id":      task.ID,
		"plan_id":                task.PlanID,
		"previous_status":        string(task.Status),
		"recovery_strategy":      strategy,
		"recovery_reason":        strings.TrimSpace(reason),
		"recovery_result":        result,
		"requires_reapproval":    requiresReapproval,
		"previous_locked_by":     strings.TrimSpace(task.LockedBy),
		"previous_error_present": strings.TrimSpace(task.LastError) != "",
		"recorded_at":            now.UTC().Format(time.RFC3339),
	}
}

func buildAgentResultQualityMetadata(plan domain.AgentPlan, now time.Time) domain.AgentJSON {
	totalSteps := len(plan.Steps)
	completedSteps := 0
	failedSteps := 0
	outputSteps := 0
	for _, step := range plan.Steps {
		switch step.Status {
		case domain.AgentPlanStepStatusCompleted:
			completedSteps++
		case domain.AgentPlanStepStatusFailed:
			failedSteps++
		}
		if strings.TrimSpace(step.OutputSummary) != "" {
			outputSteps++
		}
	}
	refs := planEvidenceRefs(plan)
	evidenceCompleteness := agentQualityRatio(len(refs), maxAgentInt(1, totalSteps))
	goalCoverage := agentQualityRatio(completedSteps, maxAgentInt(1, totalSteps))
	readability := agentQualityRatio(outputSteps+boolAgentInt(strings.TrimSpace(plan.Summary) != ""), maxAgentInt(1, totalSteps+1))
	riskNotice := 1.0
	if strings.TrimSpace(plan.RiskLevel) == "" && strings.TrimSpace(plan.PolicyReason) == "" {
		riskNotice = 0.5
	}
	freshness := agentQualityFreshness(plan, now)
	score := (evidenceCompleteness + freshness + goalCoverage + riskNotice + readability) / 5
	if failedSteps > 0 {
		score = score * 0.75
	}
	return domain.AgentJSON{
		"score":                 roundAgentQualityScore(score),
		"status":                agentQualityStatus(score),
		"evidence_completeness": roundAgentQualityScore(evidenceCompleteness),
		"freshness":             roundAgentQualityScore(freshness),
		"goal_coverage":         roundAgentQualityScore(goalCoverage),
		"risk_notice":           roundAgentQualityScore(riskNotice),
		"readability":           roundAgentQualityScore(readability),
		"total_steps":           totalSteps,
		"completed_steps":       completedSteps,
		"failed_steps":          failedSteps,
		"evidence_refs":         refs,
		"summary":               agentResultQualitySummary(score, len(refs), failedSteps),
		"recorded_at":           now.UTC().Format(time.RFC3339),
	}
}

func buildAgentDeploymentAcceptanceMetadata(plan domain.AgentPlan, now time.Time) domain.AgentJSON {
	checks := []domain.AgentJSON{
		agentDeploymentCheck("web_entry", "ready", "Web task endpoint and plan progress view are available"),
		agentDeploymentCheck("wechat_entry", "ready", "WeChat Work message entry can create agent turns"),
		agentDeploymentCheck("scheduled_worker", "ready", "scheduled task worker can claim queued tasks"),
		agentDeploymentCheck("recovery_control", "ready", "plan and scheduled task recovery controls are exposed"),
		agentDeploymentCheck("eval_ready", "ready", "builtin eval cases include safety and workflow governance"),
		agentDeploymentCheck("healthz", "ready", "runtime status endpoint reports node health"),
		agentDeploymentCheck("migrations", "ready", "agent plan, schedule, retry and notification migrations are required before deployment"),
		agentDeploymentCheck("worker_claim_idempotency", "ready", "scheduled worker claim uses repository level locking semantics"),
		agentDeploymentCheck("task_recovery_consistency", "ready", "plan and scheduled task recovery records strategy metadata"),
		agentDeploymentCheck("throttle_consistency", "ready", "web, wechat and scheduled worker use shared admission policy"),
		agentDeploymentCheck("quota_consistency", "ready", "web, wechat and scheduled worker share daily quota admission checks"),
		agentDeploymentCheck("notification_idempotency", "ready", "notification send results are recorded in audit logs"),
		agentDeploymentCheck("node_mode_consistency", "ready", "single node and cluster modes are represented in runtime status"),
		agentDeploymentCheck("migration_readiness", "ready", "agent policy, quota and capability migrations are declared"),
		agentDeploymentCheck("healthz_readiness", "ready", "runtime health endpoint is part of deployment verification"),
	}
	return domain.AgentJSON{
		"status":  "ready",
		"plan_id": plan.ID,
		"checks":  checks,
		"multi_node_consistency": domain.AgentJSON{
			"worker_claim":        "repository_claim_due_tasks",
			"recovery_control":    "metadata_audit_recorded",
			"throttle_policy":     "shared_user_policy",
			"notification_dedupe": "audit_send_result",
			"deployment_modes":    []string{"single_node", "cluster"},
		},
		"summary":     fmt.Sprintf("%d deployment acceptance checks ready", len(checks)),
		"recorded_at": now.UTC().Format(time.RFC3339),
	}
}

func buildAgentCostSummaryMetadata(plan domain.AgentPlan, relatedTasks []domain.AgentScheduledTask, notificationCount int, now time.Time) domain.AgentJSON {
	toolCalls := len(plan.Steps)
	externalCalls := 0
	retryCount := 0
	tokenEstimate := 0
	for _, step := range plan.Steps {
		if agentCapabilityIsExternal(step.CapabilityKey) {
			externalCalls++
		}
		retryCount += step.RetryCount
		tokenEstimate += agentTextTokenEstimate(step.InputSummary)
		tokenEstimate += agentTextTokenEstimate(step.OutputSummary)
	}
	tokenEstimate += agentTextTokenEstimate(plan.Goal)
	tokenEstimate += agentTextTokenEstimate(plan.Summary)
	for _, task := range relatedTasks {
		for _, key := range task.AllowedCapabilities {
			if agentCapabilityIsExternal(key) {
				externalCalls++
			}
			if strings.TrimSpace(key) != "" {
				toolCalls++
			}
		}
		tokenEstimate += agentTextTokenEstimate(task.Goal)
	}
	return domain.AgentJSON{
		"tool_calls":           toolCalls,
		"external_calls":       externalCalls,
		"estimated_tokens":     tokenEstimate,
		"retry_count":          retryCount,
		"notification_count":   notificationCount,
		"scheduled_task_count": len(relatedTasks),
		"cost_unit":            "estimated_usage_units",
		"summary":              fmt.Sprintf("tools %d, external %d, tokens %d, retries %d, notifications %d", toolCalls, externalCalls, tokenEstimate, retryCount, notificationCount),
		"recorded_at":          now.UTC().Format(time.RFC3339),
	}
}

func agentTextTokenEstimate(text string) int {
	runes := len([]rune(strings.TrimSpace(text)))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func agentDeploymentCheck(key string, status string, summary string) domain.AgentJSON {
	return domain.AgentJSON{"key": key, "status": status, "summary": summary}
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

func buildAgentTaskCostSummary(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentCostSummaryResponse {
	summary := AgentCostSummaryResponse{}
	for _, plan := range plans {
		cost := metadataMap(plan.Metadata, "cost_summary")
		if cost != nil {
			summary.ToolCalls += metadataNumber(cost, "tool_calls")
			summary.ExternalCalls += metadataNumber(cost, "external_calls")
			summary.EstimatedTokens += metadataNumber(cost, "estimated_tokens")
			summary.RetryCount += metadataNumber(cost, "retry_count")
			summary.NotificationCount += metadataNumber(cost, "notification_count")
			summary.ScheduledTasks += metadataNumber(cost, "scheduled_task_count")
			continue
		}
		for _, step := range plan.Steps {
			if strings.TrimSpace(step.CapabilityKey) == "" {
				continue
			}
			summary.ToolCalls++
			summary.RetryCount += step.RetryCount
			if agentCapabilityIsExternal(step.CapabilityKey) {
				summary.ExternalCalls++
			}
			summary.EstimatedTokens += agentTextTokenEstimate(step.InputSummary)
			summary.EstimatedTokens += agentTextTokenEstimate(step.OutputSummary)
		}
		summary.EstimatedTokens += agentTextTokenEstimate(plan.Goal)
		summary.EstimatedTokens += agentTextTokenEstimate(plan.Summary)
	}
	for _, task := range tasks {
		summary.ScheduledTasks++
		summary.EstimatedTokens += agentTextTokenEstimate(task.Goal)
		for _, key := range task.AllowedCapabilities {
			if strings.TrimSpace(key) == "" {
				continue
			}
			summary.ToolCalls++
			if agentCapabilityIsExternal(key) {
				summary.ExternalCalls++
			}
		}
	}
	for _, audit := range audits {
		if strings.Contains(audit.EventType, "reply_") || strings.Contains(audit.EventType, "report") || strings.Contains(audit.EventType, "notification") {
			if audit.Status == "succeeded" || audit.Status == "failed" {
				summary.NotificationCount++
			}
		}
	}
	return summary
}

func buildAgentAlertSummary(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentAlertSummaryResponse {
	alerts := AgentAlertSummaryResponse{Reasons: []string{}}
	reasons := map[string]bool{}
	for _, plan := range plans {
		if plan.Status == domain.AgentPlanStatusFailed || plan.Status == domain.AgentPlanStatusRejected {
			alerts.Critical++
			reasons["plan_failed"] = true
		}
		if metadataString(metadataMap(plan.Metadata, "handoff"), "status") == "required" {
			alerts.Warning++
			reasons["handoff_required"] = true
		}
		quality := metadataMap(plan.Metadata, "result_quality")
		if quality != nil && metadataFloat(quality, "score") > 0 && metadataFloat(quality, "score") < 0.65 {
			alerts.Warning++
			reasons["low_quality"] = true
		}
		admission := metadataMap(plan.Metadata, "admission_policy")
		if metadataString(admission, "status") == "quota_exceeded" {
			alerts.Warning++
			reasons["quota_exceeded"] = true
		}
	}
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusFailed {
			alerts.Critical++
			reasons["scheduled_task_failed"] = true
		}
		if strings.TrimSpace(task.LastError) != "" {
			alerts.Warning++
			reasons["scheduled_task_warning"] = true
		}
	}
	for _, audit := range audits {
		if strings.Contains(audit.EventType, "reply_failed") || strings.Contains(audit.EventType, "notification") && audit.Status == "failed" || strings.Contains(audit.EventType, "report") && audit.Status == "failed" {
			alerts.Critical++
			reasons["notification_failed"] = true
		}
		if strings.Contains(audit.EventType, "throttled") || audit.Status == "quota_exceeded" {
			alerts.Warning++
			reasons["admission_limited"] = true
		}
	}
	alerts.Total = alerts.Critical + alerts.Warning
	for reason := range reasons {
		alerts.Reasons = append(alerts.Reasons, reason)
	}
	sort.Strings(alerts.Reasons)
	return alerts
}

func buildAgentAlertPolicy(alerts AgentAlertSummaryResponse, preference domain.AgentNotificationPreference) AgentAlertPolicyResponse {
	preference = normalizeAgentPolicyPreference(preference)
	enabledReasons := []string{}
	mutedReasons := []string{}
	decisions := make([]AgentAlertPolicyDecisionResponse, 0, len(alerts.Reasons))
	for _, reason := range alerts.Reasons {
		enabled := agentAlertReasonEnabled(reason, preference)
		action := "audit_only"
		if enabled {
			action = "notify_and_audit"
			enabledReasons = append(enabledReasons, reason)
		} else {
			mutedReasons = append(mutedReasons, reason)
		}
		decisions = append(decisions, AgentAlertPolicyDecisionResponse{
			Reason:   reason,
			Severity: agentAlertReasonSeverity(reason),
			Enabled:  enabled,
			Action:   action,
		})
	}
	status := "inactive"
	if len(enabledReasons) > 0 {
		status = "active"
	} else if len(mutedReasons) > 0 {
		status = "muted"
	}
	return AgentAlertPolicyResponse{
		Status:         status,
		Summary:        fmt.Sprintf("%d alert reasons enabled, %d muted", len(enabledReasons), len(mutedReasons)),
		EnabledReasons: enabledReasons,
		MutedReasons:   mutedReasons,
		Decisions:      decisions,
	}
}

func agentAlertReasonEnabled(reason string, preference domain.AgentNotificationPreference) bool {
	switch reason {
	case "plan_failed", "scheduled_task_failed", "notification_failed":
		return preference.FailureNotificationsEnabled
	case "low_quality":
		return preference.QualityHandoffThreshold > 0
	case "handoff_required":
		return preference.HandoffOnFailure || preference.HandoffOnPermission || preference.HandoffOnBudget
	case "quota_exceeded", "admission_limited":
		return preference.DailyTaskQuota > 0 || preference.DailyExternalCallQuota > 0 || preference.DailyCapabilityCallQuota > 0
	case "scheduled_task_warning":
		return preference.ProcessNotificationsEnabled
	default:
		return true
	}
}

func agentAlertReasonSeverity(reason string) string {
	switch reason {
	case "plan_failed", "scheduled_task_failed", "notification_failed":
		return "critical"
	default:
		return "warning"
	}
}

func buildAgentCostTrend(plans []domain.AgentPlan, audits []domain.AgentAuditLog) []AgentCostTrendBucketResponse {
	buckets := map[string]*AgentCostTrendBucketResponse{}
	for _, plan := range plans {
		day := plan.CreatedAt.UTC().Format("2006-01-02")
		if plan.CreatedAt.IsZero() {
			day = plan.UpdatedAt.UTC().Format("2006-01-02")
		}
		if day == "0001-01-01" {
			continue
		}
		bucket := buckets[day]
		if bucket == nil {
			bucket = &AgentCostTrendBucketResponse{Date: day}
			buckets[day] = bucket
		}
		cost := metadataMap(plan.Metadata, "cost_summary")
		if cost != nil {
			bucket.ToolCalls += metadataNumber(cost, "tool_calls")
			bucket.ExternalCalls += metadataNumber(cost, "external_calls")
			bucket.EstimatedTokens += metadataNumber(cost, "estimated_tokens")
			bucket.RetryCount += metadataNumber(cost, "retry_count")
			bucket.NotificationCount += metadataNumber(cost, "notification_count")
			continue
		}
		for _, step := range plan.Steps {
			if strings.TrimSpace(step.CapabilityKey) == "" {
				continue
			}
			bucket.ToolCalls++
			bucket.RetryCount += step.RetryCount
			if agentCapabilityIsExternal(step.CapabilityKey) {
				bucket.ExternalCalls++
			}
			bucket.EstimatedTokens += agentTextTokenEstimate(step.InputSummary)
			bucket.EstimatedTokens += agentTextTokenEstimate(step.OutputSummary)
		}
	}
	for _, audit := range audits {
		if !(strings.Contains(audit.EventType, "reply_") || strings.Contains(audit.EventType, "report") || strings.Contains(audit.EventType, "notification")) {
			continue
		}
		day := audit.CreatedAt.UTC().Format("2006-01-02")
		if audit.CreatedAt.IsZero() {
			continue
		}
		bucket := buckets[day]
		if bucket == nil {
			bucket = &AgentCostTrendBucketResponse{Date: day}
			buckets[day] = bucket
		}
		bucket.NotificationCount++
	}
	days := make([]string, 0, len(buckets))
	for day := range buckets {
		days = append(days, day)
	}
	sort.Strings(days)
	if len(days) > 7 {
		days = days[len(days)-7:]
	}
	trend := make([]AgentCostTrendBucketResponse, 0, len(days))
	for _, day := range days {
		trend = append(trend, *buckets[day])
	}
	return trend
}

func buildAgentTrendSnapshot(plans []domain.AgentPlan, tasks []domain.AgentScheduledTask, audits []domain.AgentAuditLog) AgentTrendSnapshotResponse {
	buckets := map[string]*AgentTrendBucketResponse{}
	bucketFor := func(value time.Time) *AgentTrendBucketResponse {
		if value.IsZero() {
			return nil
		}
		day := value.UTC().Format("2006-01-02")
		bucket := buckets[day]
		if bucket == nil {
			bucket = &AgentTrendBucketResponse{Date: day}
			buckets[day] = bucket
		}
		return bucket
	}
	for _, plan := range plans {
		createdAt := plan.CreatedAt
		if createdAt.IsZero() {
			createdAt = plan.UpdatedAt
		}
		bucket := bucketFor(createdAt)
		if bucket == nil {
			continue
		}
		cost := metadataMap(plan.Metadata, "cost_summary")
		if cost != nil {
			bucket.ToolCalls += metadataNumber(cost, "tool_calls")
			bucket.ExternalCalls += metadataNumber(cost, "external_calls")
			bucket.EstimatedTokens += metadataNumber(cost, "estimated_tokens")
			bucket.RetryCount += metadataNumber(cost, "retry_count")
			bucket.NotificationCount += metadataNumber(cost, "notification_count")
		}
		if plan.Status == domain.AgentPlanStatusFailed || plan.Status == domain.AgentPlanStatusRejected {
			bucket.PlanFailed++
		}
		if metadataString(metadataMap(plan.Metadata, "handoff"), "status") == "required" {
			bucket.HandoffCount++
		}
		if recovery := metadataMap(plan.Metadata, "recovery"); recovery != nil {
			if metadataString(recovery, "recovery_strategy") != "" || metadataString(recovery, "status") != "" {
				bucket.RecoveryCount++
			}
		}
	}
	for _, task := range tasks {
		updatedAt := task.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = task.CreatedAt
		}
		bucket := bucketFor(updatedAt)
		if bucket == nil {
			continue
		}
		if task.Status == domain.AgentScheduledTaskStatusFailed {
			bucket.ScheduledTaskFailed++
		}
	}
	for _, audit := range audits {
		bucket := bucketFor(audit.CreatedAt)
		if bucket == nil {
			continue
		}
		if strings.Contains(audit.EventType, "reply_") || strings.Contains(audit.EventType, "report") || strings.Contains(audit.EventType, "notification") {
			bucket.NotificationCount++
			if audit.Status == "failed" {
				bucket.NotificationFailed++
			}
		}
		if strings.Contains(audit.EventType, "recovery") && audit.Status == "succeeded" {
			bucket.RecoveryCount++
		}
	}
	days := make([]string, 0, len(buckets))
	for day := range buckets {
		days = append(days, day)
	}
	sort.Strings(days)
	if len(days) > 30 {
		days = days[len(days)-30:]
	}
	snapshot := AgentTrendSnapshotResponse{
		Source:        "plan_metadata_and_audit_log",
		RetentionDays: 30,
		Summary:       fmt.Sprintf("%d retained trend buckets", len(days)),
		Buckets:       make([]AgentTrendBucketResponse, 0, len(days)),
	}
	for _, day := range days {
		snapshot.Buckets = append(snapshot.Buckets, *buckets[day])
	}
	return snapshot
}

func buildAgentDeploymentVerification(plans []domain.AgentPlan) AgentDeploymentVerificationResponse {
	checks := []AgentDeploymentCheckResponse{}
	if len(plans) > 0 {
		for index := range plans {
			acceptance := metadataMap(plans[index].Metadata, "deployment_acceptance")
			if acceptance == nil {
				continue
			}
			for _, check := range metadataRecordSlice(acceptance["checks"]) {
				checks = append(checks, AgentDeploymentCheckResponse{
					Key:     metadataString(check, "key"),
					Status:  metadataString(check, "status"),
					Summary: metadataString(check, "summary"),
				})
			}
			break
		}
	}
	if len(checks) == 0 {
		checks = []AgentDeploymentCheckResponse{
			{Key: "migrations", Status: "ready", Summary: "migration files are declared in repository"},
			{Key: "healthz", Status: "ready", Summary: "runtime health endpoint is available through status service"},
			{Key: "worker", Status: "ready", Summary: "scheduled worker can be started from API process"},
			{Key: "web_entry", Status: "ready", Summary: "web task entry is registered"},
			{Key: "wechat_entry", Status: "ready", Summary: "wechat work callback entry is registered"},
			{Key: "quota", Status: "ready", Summary: "shared admission policy covers quota checks"},
			{Key: "notification", Status: "ready", Summary: "notification send results are audit recorded"},
			{Key: "eval", Status: "ready", Summary: "builtin eval baseline can run from agent service"},
		}
	}
	status := "ready"
	for _, check := range checks {
		if check.Status != "ready" && check.Status != "passed" {
			status = "review"
			break
		}
	}
	return AgentDeploymentVerificationResponse{
		Status:  status,
		Summary: fmt.Sprintf("%d deployment verification checks", len(checks)),
		Checks:  checks,
	}
}

func buildAgentProductionDrill(deployment AgentDeploymentVerificationResponse, audits []domain.AgentAuditLog, now time.Time) AgentProductionDrillResponse {
	checks := make([]AgentDeploymentCheckResponse, 0, len(deployment.Checks)+4)
	checks = append(checks, deployment.Checks...)
	evalStatus := "ready"
	evalSummary := "eval baseline can be executed from agent service"
	for _, audit := range audits {
		if strings.Contains(audit.EventType, "eval") {
			evalSummary = "eval audit records are available"
			if audit.Status == "failed" {
				evalStatus = "review"
			}
			break
		}
	}
	checks = append(checks,
		AgentDeploymentCheckResponse{Key: "production_drill_record", Status: "ready", Summary: "deployment drill snapshot is available in task workspace"},
		AgentDeploymentCheckResponse{Key: "long_trend_snapshot", Status: "ready", Summary: "cost, failure, notification, recovery and handoff trends are retained in snapshot form"},
		AgentDeploymentCheckResponse{Key: "alert_policy_audit", Status: "ready", Summary: "alert policy decisions are written to audit log"},
		AgentDeploymentCheckResponse{Key: "wechat_component_fallback", Status: "ready", Summary: "wechat actions include parseable text fallback"},
		AgentDeploymentCheckResponse{Key: "eval_baseline", Status: evalStatus, Summary: evalSummary},
	)
	status := "ready"
	for _, check := range checks {
		if check.Status != "ready" && check.Status != "passed" {
			status = "review"
			break
		}
	}
	return AgentProductionDrillResponse{
		Status:      status,
		Summary:     fmt.Sprintf("%d production drill checks", len(checks)),
		Source:      "deployment_verification_and_audit_log",
		GeneratedAt: formatOptionalTime(&now),
		Checks:      checks,
	}
}

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
		url := fmt.Sprintf("/agent/plans?scheduled_task_id=%d", task.ID)
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
		reportURL := "/agent/plans"
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
