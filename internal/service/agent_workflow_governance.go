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
