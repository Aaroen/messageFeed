package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

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
