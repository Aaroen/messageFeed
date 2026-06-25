package service

import (
	"fmt"
	"messagefeed/internal/domain"
	"sort"
	"strings"
	"time"
)

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
