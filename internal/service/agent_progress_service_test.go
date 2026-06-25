package service

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestAgentSessionServiceGetProgressAggregatesPlanRunsAndScheduledTasks(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	plan := domain.AgentPlan{
		ID:                 10,
		UserID:             1,
		SessionID:          2,
		TurnID:             3,
		ControllerRunID:    4,
		Status:             domain.AgentPlanStatusExecuting,
		Summary:            "执行定时摘要",
		ConfirmationPolicy: "auto",
		UpdatedAt:          now,
		Metadata: domain.AgentJSON{
			"permission_governance": domain.AgentJSON{"has_external_access": true, "requires_confirmation": false},
			"budget_governance":     domain.AgentJSON{"status": "within_budget", "tool_calls": 1, "tool_call_budget": 8, "external_calls": 1, "external_call_budget": 4},
			"result_quality":        domain.AgentJSON{"status": "passed", "score": 0.9, "evidence_completeness": 1.0, "goal_coverage": 1.0},
			"cost_summary":          domain.AgentJSON{"tool_calls": 1, "external_calls": 1, "estimated_tokens": 12, "retry_count": 0, "notification_count": 1, "scheduled_task_count": 0},
			"deployment_acceptance": domain.AgentJSON{"status": "ready", "checks": []any{domain.AgentJSON{"key": "web_entry", "status": "ready"}}},
			"runtime_observability": domain.AgentJSON{"status": "executing", "summary": "status executing, failed steps 0, recoveries 0, quality 0.90"},
			"handoff":               domain.AgentJSON{"status": "not_required", "next_action": "无需人工接管"},
		},
		Steps: []domain.AgentPlanStep{
			{ID: 11, PlanID: 10, StepOrder: 1, Status: domain.AgentPlanStepStatusCompleted, CapabilityKey: "web.search", Title: "搜索网页", OutputSummary: "已搜索", UpdatedAt: now},
		},
	}
	run := domain.AgentRun{
		ID:        4,
		SessionID: 2,
		TurnID:    3,
		Role:      domain.AgentRunRoleController,
		Status:    domain.AgentRunStatusRunning,
		ModelKey:  "model-a",
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
		Observations: []domain.AgentObservation{
			{ID: 21, RunID: 4, CapabilityKey: "web.search", OutputSummary: "2 条结果", Status: "succeeded", CreatedAt: now},
		},
	}
	task := domain.AgentScheduledTask{
		ID:                  30,
		UserID:              1,
		SessionID:           2,
		TurnID:              3,
		PlanID:              10,
		SourceRunID:         4,
		Status:              domain.AgentScheduledTaskStatusQueued,
		TaskType:            "digest",
		Goal:                "汇总 AI 新闻",
		ScheduledAt:         now.Add(time.Hour),
		AllowedCapabilities: []string{"web.search"},
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	repository := &fakeAgentProgressRepository{plan: plan, runs: []domain.AgentRun{run}, tasks: []domain.AgentScheduledTask{task}}
	service := NewAgentSessionService(repository, WithAgentSessionNow(func() time.Time { return now }))

	result, err := service.GetProgress(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, AgentProgressQuery{PlanID: 10})
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}
	progress := result.Progress
	if progress.SubjectType != "plan" || progress.SubjectID != 10 {
		t.Fatalf("subject = %s/%d, want plan/10", progress.SubjectType, progress.SubjectID)
	}
	if progress.Plan == nil || progress.Plan.ID != 10 {
		t.Fatalf("plan = %#v", progress.Plan)
	}
	if len(progress.Runs) != 1 || len(progress.Runs[0].Observations) != 1 {
		t.Fatalf("runs = %#v", progress.Runs)
	}
	if len(progress.ScheduledTasks) != 1 || progress.ScheduledTasks[0].ID != 30 {
		t.Fatalf("scheduled tasks = %#v", progress.ScheduledTasks)
	}
	if len(progress.Phases) < 4 {
		t.Fatalf("phases = %#v", progress.Phases)
	}
	if !fakeProgressPhaseExists(progress.Phases, "permission") || !fakeProgressPhaseExists(progress.Phases, "budget") || !fakeProgressPhaseExists(progress.Phases, "quality") || !fakeProgressPhaseExists(progress.Phases, "cost") || !fakeProgressPhaseExists(progress.Phases, "deployment_acceptance") || !fakeProgressPhaseExists(progress.Phases, "cluster_consistency") || !fakeProgressPhaseExists(progress.Phases, "runtime_observability") || !fakeProgressPhaseExists(progress.Phases, "handoff") {
		t.Fatalf("governance phases missing: %#v", progress.Phases)
	}
	if len(progress.RecentEvents) == 0 {
		t.Fatal("recent events should not be empty")
	}
	if progress.Version == 0 || progress.EventCursor == "" || progress.UpdatedAt == "" {
		t.Fatalf("incremental fields missing: %#v", progress)
	}
	foundObservation := false
	for _, event := range progress.RecentEvents {
		if event.Kind == "observation" {
			foundObservation = true
			if event.Title != "能力调用：web.search" || event.Ref != "run:4" {
				t.Fatalf("observation event = %#v", event)
			}
		}
	}
	if !foundObservation {
		t.Fatalf("events = %#v, want observation event", progress.RecentEvents)
	}
	summary := AgentProgressTextSummary(progress)
	if summary == "" || !strings.Contains(summary, "状态：执行中") || !strings.Contains(summary, "质量：") || !strings.Contains(summary, "成本：") || !strings.Contains(summary, "运行观测：") || !strings.Contains(summary, "人工接管：") || !strings.Contains(summary, "进度版本：") {
		t.Fatalf("summary = %q", summary)
	}
}

func TestAgentSessionServiceListTasksCombinesPlansAndScheduledTasks(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := &fakeAgentProgressRepository{
		plans: []domain.AgentPlan{
			{ID: 10, UserID: 1, SessionID: 2, TurnID: 3, Status: domain.AgentPlanStatusCompleted, Goal: "执行 Web 任务", Summary: "已执行", Metadata: domain.AgentJSON{"budget_governance": domain.AgentJSON{"status": "within_budget"}, "cost_summary": domain.AgentJSON{"tool_calls": 1, "external_calls": 1, "estimated_tokens": 20, "retry_count": 1, "notification_count": 1}, "deployment_acceptance": domain.AgentJSON{"status": "ready", "checks": []domain.AgentJSON{{"key": "web_entry", "status": "ready", "summary": "web ready"}}}, "handoff": domain.AgentJSON{"status": "required"}, "admission_policy": domain.AgentJSON{"entry": "web"}}, Steps: []domain.AgentPlanStep{{ID: 11, CapabilityKey: "web.search"}}, CreatedAt: now.Add(-2 * time.Minute), CompletedAt: timePtr(now.Add(-time.Minute)), UpdatedAt: now.Add(-time.Minute)},
		},
		tasks: []domain.AgentScheduledTask{
			{ID: 20, UserID: 1, SessionID: 2, TurnID: 3, PlanID: 10, Status: domain.AgentScheduledTaskStatusSucceeded, TaskType: "digest", Goal: "定时摘要", TargetChannel: domain.AgentProviderWeChatWorkApp, UpdatedAt: now},
		},
		audits: []domain.AgentAuditLog{
			{ID: 30, UserID: 1, EventType: "wechat_work.reply_sent", Status: "succeeded", CreatedAt: now},
			{ID: 31, UserID: 1, EventType: "agent.plan_progress_notification", Status: "succeeded", Metadata: domain.AgentJSON{"progress_url": "https://messagefeed.example/agent/plans/10", "target_channel": "wechat_work", "template_status": "succeeded", "fallback_status": "not_attempted"}, CreatedAt: now.Add(time.Second)},
		},
	}
	service := NewAgentSessionService(repository, WithAgentSessionNow(func() time.Time { return now }))

	result, err := service.ListTasks(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, 10)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("tasks = %#v", result.Tasks)
	}
	if result.Tasks[0].Kind != "scheduled_task" || result.Tasks[0].ProgressURL != "/agent/plans/10" {
		t.Fatalf("first task = %#v", result.Tasks[0])
	}
	if result.Tasks[1].Kind != "plan" || result.Tasks[1].ProgressURL != "/agent/plans/10" {
		t.Fatalf("second task = %#v", result.Tasks[1])
	}
	if result.Tasks[1].PermissionStatus == "" || result.Tasks[1].BudgetStatus != "within_budget" || result.Tasks[1].NextAction == "" {
		t.Fatalf("second task governance fields = %#v", result.Tasks[1])
	}
	if result.SLA.PlanSucceeded != 1 || result.SLA.ScheduledTaskSucceeded != 1 || result.SLA.HandoffCount != 1 || result.SLA.NotificationSentCount != 1 {
		t.Fatalf("sla = %#v", result.SLA)
	}
	if result.Cost.ToolCalls < 1 || result.Cost.ExternalCalls < 1 || result.Cost.EstimatedTokens < 20 || result.Cost.NotificationCount < 1 {
		t.Fatalf("cost = %#v", result.Cost)
	}
	if result.Alerts.Warning < 1 || !stringSliceContains(result.Alerts.Reasons, "handoff_required") {
		t.Fatalf("alerts = %#v", result.Alerts)
	}
	if result.AlertPolicy.Status != "active" || !stringSliceContains(result.AlertPolicy.EnabledReasons, "handoff_required") {
		t.Fatalf("alert policy = %#v", result.AlertPolicy)
	}
	if len(result.CostTrend) != 1 || result.CostTrend[0].ToolCalls != 1 || result.CostTrend[0].NotificationCount < 1 {
		t.Fatalf("cost trend = %#v", result.CostTrend)
	}
	if result.TrendSnapshot.RetentionDays != 30 || len(result.TrendSnapshot.Buckets) != 1 || result.TrendSnapshot.Buckets[0].HandoffCount != 1 {
		t.Fatalf("trend snapshot = %#v", result.TrendSnapshot)
	}
	if result.Deployment.Status != "ready" || len(result.Deployment.Checks) != 1 || result.Deployment.Checks[0].Key != "web_entry" {
		t.Fatalf("deployment = %#v", result.Deployment)
	}
	if result.Drill.Status != "ready" || len(result.Drill.Checks) < len(result.Deployment.Checks) {
		t.Fatalf("drill = %#v", result.Drill)
	}
	if result.WeChatComponents.Mode != "component_fallback" || !agentActionExists(result.WeChatComponents.Actions, "view_progress") || !agentActionExists(result.WeChatComponents.Actions, "cancel_scheduled_task") {
		t.Fatalf("wechat components = %#v", result.WeChatComponents)
	}
	if result.LoadTest.Metrics.WebTasks != 1 || result.LoadTest.Metrics.ScheduledTasks != 1 || len(result.LoadTest.Checks) == 0 {
		t.Fatalf("load test = %#v", result.LoadTest)
	}
	if len(result.WeChatCallback.Checks) == 0 || result.WeChatCallback.Summary == "" {
		t.Fatalf("wechat callback = %#v", result.WeChatCallback)
	}
	if result.WriteSandbox.DefaultAction != "reject_or_require_approval" || result.WriteSandbox.Status != "sandboxed" {
		t.Fatalf("write sandbox = %#v", result.WriteSandbox)
	}
	if len(result.E2E.Checks) == 0 || result.E2E.Summary == "" {
		t.Fatalf("e2e = %#v", result.E2E)
	}
	if len(result.RealIntegration.Checks) == 0 || result.RealIntegration.NextAction == "" {
		t.Fatalf("real integration = %#v", result.RealIntegration)
	}
	if result.WeChatNative.Mode != "native_button_schema" || !agentNativeActionExists(result.WeChatNative.Actions, "view_progress") || !agentNativeActionExists(result.WeChatNative.Actions, "view_final_report") {
		t.Fatalf("wechat native = %#v", result.WeChatNative)
	}
	if result.WriteLeastPrivilege.DefaultAction != "reject_or_require_approval" || !stringSliceContains(result.WriteLeastPrivilege.AllowedCandidates, "agent.schedule_message") {
		t.Fatalf("write least privilege = %#v", result.WriteLeastPrivilege)
	}
	if len(result.OpsAcceptance.Checks) == 0 || result.OpsAcceptance.Summary == "" {
		t.Fatalf("ops acceptance = %#v", result.OpsAcceptance)
	}
	if result.WeChatNativePayload.MessageType != "template_card" || len(result.WeChatNativePayload.Buttons) == 0 || result.WeChatNativePayload.FallbackText == "" {
		t.Fatalf("wechat native payload = %#v", result.WeChatNativePayload)
	}
	if result.WriteGray.Status != "approval_required" || !result.WriteGray.RequiresApproval || !result.WriteGray.RequiresBudget || !result.WriteGray.RequiresAudit {
		t.Fatalf("write gray = %#v", result.WriteGray)
	}
	if len(result.AlertChannel.Channels) < 3 || result.AlertChannel.Summary == "" {
		t.Fatalf("alert channel = %#v", result.AlertChannel)
	}
	if result.LaunchDrill.BatchID == "" || result.LaunchDrill.TriggeredBy != "agent_task_workspace" || result.LaunchDrill.NextAction == "" {
		t.Fatalf("launch drill = %#v", result.LaunchDrill)
	}
	if len(result.WeChatNativeIntegration.Checks) == 0 || result.WeChatNativeIntegration.NextAction == "" {
		t.Fatalf("wechat native integration = %#v", result.WeChatNativeIntegration)
	}
	if result.WriteReplay.ApprovalStatus == "" || result.WriteReplay.AuditStatus == "" || len(result.WriteReplay.RollbackTriggers) == 0 {
		t.Fatalf("write replay = %#v", result.WriteReplay)
	}
	if result.LaunchApproval.RequestID == "" || result.LaunchApproval.ReviewState == "" || result.LaunchApproval.HandoffPath == "" || result.LaunchApproval.RollbackPath == "" {
		t.Fatalf("launch approval = %#v", result.LaunchApproval)
	}
	if result.DailyReport.Date == "" || result.DailyReport.TaskCount != 2 || result.DailyReport.AlertCount != result.Alerts.Total {
		t.Fatalf("daily report = %#v", result.DailyReport)
	}
	if result.Preprod.Status == "" || len(result.Preprod.Checks) == 0 || result.Preprod.NextAction == "" {
		t.Fatalf("preprod = %#v", result.Preprod)
	}
	if result.ButtonLoop.Status == "" || len(result.ButtonLoop.Actions) == 0 || result.ButtonLoop.FallbackText == "" {
		t.Fatalf("button loop = %#v", result.ButtonLoop)
	}
	if result.WriteExecute.DefaultAction != "reject_or_require_approval" || result.WriteExecute.AuditStatus == "" || len(result.WriteExecute.RollbackTriggers) == 0 {
		t.Fatalf("write execute = %#v", result.WriteExecute)
	}
	if result.DailyPersist.RecordKey == "" || result.DailyPersist.Source != "agent.production_daily_report" || !result.DailyPersist.Retained {
		t.Fatalf("daily persist = %#v", result.DailyPersist)
	}
	if result.PostLaunchMonitor.Status == "" || len(result.PostLaunchMonitor.Checks) == 0 || result.PostLaunchMonitor.Summary == "" {
		t.Fatalf("post launch monitor = %#v", result.PostLaunchMonitor)
	}
	if result.ReleaseApproval.RequestID == "" || result.ReleaseApproval.DecisionPath == "" || result.ReleaseApproval.AuditEvent == "" {
		t.Fatalf("release approval = %#v", result.ReleaseApproval)
	}
	if len(result.ButtonCallback.Actions) == 0 || !agentButtonCallbackExists(result.ButtonCallback.Actions, "view_progress") || result.ButtonCallback.FallbackText == "" {
		t.Fatalf("button callback = %#v", result.ButtonCallback)
	}
	if !stringSliceContains(result.WriteAudit.Candidates, "agent.schedule_task") || result.WriteAudit.ApprovalEvidence == "" || result.WriteAudit.RollbackEvidence == "" {
		t.Fatalf("write audit = %#v", result.WriteAudit)
	}
	if result.DailySend.RecordKey == "" || result.DailySend.ScheduleStatus == "" || result.DailySend.WeChatReportStatus == "" {
		t.Fatalf("daily send = %#v", result.DailySend)
	}
	if result.MonitorAlert.TriggerStatus == "" || result.MonitorAlert.NotificationStatus == "" || len(result.MonitorAlert.Checks) == 0 {
		t.Fatalf("monitor alert = %#v", result.MonitorAlert)
	}
	if len(result.ButtonDirectControl.Actions) == 0 || len(result.ButtonDirectControl.Checks) == 0 {
		t.Fatalf("button direct control = %#v", result.ButtonDirectControl)
	}
	if result.WeChatE2E.Status == "" || len(result.WeChatE2E.Checks) == 0 {
		t.Fatalf("wechat e2e = %#v", result.WeChatE2E)
	}
	if result.ReleaseWindow.WindowState == "" || len(result.ReleaseWindow.Checks) == 0 {
		t.Fatalf("release window = %#v", result.ReleaseWindow)
	}
	if result.WriteGrayExpansion.DefaultAction != "reject_or_require_approval" || !stringSliceContains(result.WriteGrayExpansion.Candidates, "agent.schedule_task") {
		t.Fatalf("write gray expansion = %#v", result.WriteGrayExpansion)
	}
	if len(result.ExternalMonitor.Metrics) == 0 || len(result.ExternalMonitor.AlertEvents) == 0 || len(result.ExternalMonitor.Channels) == 0 {
		t.Fatalf("external monitor = %#v", result.ExternalMonitor)
	}
	if result.ReleaseWindowExecution.ExecutionState == "" || result.ReleaseWindowExecution.AuditEvent == "" || len(result.ReleaseWindowExecution.Checks) == 0 {
		t.Fatalf("release window execution = %#v", result.ReleaseWindowExecution)
	}
	if len(result.ExternalMonitorRuntime.Metrics) == 0 || len(result.ExternalMonitorRuntime.AlertEvents) == 0 || result.ExternalMonitorRuntime.DailySendStatus == "" {
		t.Fatalf("external monitor runtime = %#v", result.ExternalMonitorRuntime)
	}
	if result.WriteGrayReview.Decision == "" || !stringSliceContains(result.WriteGrayReview.Candidates, "agent.schedule_task") || len(result.WriteGrayReview.DeniedPatterns) == 0 {
		t.Fatalf("write gray review = %#v", result.WriteGrayReview)
	}
	if result.WeChatAcceptanceReview.NextAction == "" || result.WeChatAcceptanceReview.ButtonControlStatus == "" || len(result.WeChatAcceptanceReview.Checks) == 0 {
		t.Fatalf("wechat acceptance review = %#v", result.WeChatAcceptanceReview)
	}
	if result.OperationsDailyClosure.AuditStatus == "" || result.OperationsDailyClosure.ReleaseWindowStatus == "" || len(result.OperationsDailyClosure.Checks) == 0 {
		t.Fatalf("operations daily closure = %#v", result.OperationsDailyClosure)
	}
	if result.ProductionRelease.BatchID == "" || result.ProductionRelease.AuditEvent == "" || len(result.ProductionRelease.Checks) == 0 {
		t.Fatalf("production release = %#v", result.ProductionRelease)
	}
	if len(result.ExternalMonitorConfig.MetricNames) == 0 || len(result.ExternalMonitorConfig.EventNames) == 0 || result.ExternalMonitorConfig.PlatformStatus == "" {
		t.Fatalf("external monitor config = %#v", result.ExternalMonitorConfig)
	}
	if result.WriteRamp.Decision == "" || !stringSliceContains(result.WriteRamp.Candidates, "agent.schedule_task") || result.WriteRamp.DefaultAction != "reject_or_require_approval" {
		t.Fatalf("write ramp = %#v", result.WriteRamp)
	}
	if result.WeChatSignoff.SignoffState == "" || result.WeChatSignoff.AuditEvent == "" || len(result.WeChatSignoff.Checks) == 0 {
		t.Fatalf("wechat signoff = %#v", result.WeChatSignoff)
	}
	if result.OperationsHandoff.NextAction == "" || result.OperationsHandoff.ReleaseStatus == "" || len(result.OperationsHandoff.Checks) == 0 {
		t.Fatalf("operations handoff = %#v", result.OperationsHandoff)
	}
	if result.ProductionExecution.BatchID == "" || result.ProductionExecution.Executor == "" || result.ProductionExecution.AuditEvent == "" {
		t.Fatalf("production execution = %#v", result.ProductionExecution)
	}
	if len(result.MonitorIntegration.MetricNames) == 0 || len(result.MonitorIntegration.EventNames) == 0 || result.MonitorIntegration.IntegrationResult == "" {
		t.Fatalf("monitor integration = %#v", result.MonitorIntegration)
	}
	if result.WriteRampPolicy.UserScope == "" || result.WriteRampPolicy.DefaultAction != "reject_or_require_approval" || result.WriteRampPolicy.RollbackThreshold == "" {
		t.Fatalf("write ramp policy = %#v", result.WriteRampPolicy)
	}
	if result.WeChatFinalReport.FinalReportEntry == "" || result.WeChatFinalReport.AuditEvent == "" || len(result.WeChatFinalReport.Checks) == 0 {
		t.Fatalf("wechat final report = %#v", result.WeChatFinalReport)
	}
	if result.LaunchRuntimeOverview.NextAction == "" || result.LaunchRuntimeOverview.ProductionExecutionStatus == "" || len(result.LaunchRuntimeOverview.Checks) == 0 {
		t.Fatalf("launch runtime overview = %#v", result.LaunchRuntimeOverview)
	}
	if result.RuntimeParameters.UserScope == "" || result.RuntimeParameters.NotificationChannel == "" || result.RuntimeParameters.RollbackThreshold == "" {
		t.Fatalf("runtime parameters = %#v", result.RuntimeParameters)
	}
	if len(result.MonitorReadback.MetricNames) == 0 || len(result.MonitorReadback.EventNames) == 0 || result.MonitorReadback.FreshnessStatus == "" {
		t.Fatalf("monitor readback = %#v", result.MonitorReadback)
	}
	if result.WriteRampRecommendation.RecommendedPercent < result.WriteRampRecommendation.CurrentPercent || result.WriteRampRecommendation.DefaultAction != "reject_or_require_approval" {
		t.Fatalf("write ramp recommendation = %#v", result.WriteRampRecommendation)
	}
	if result.WeChatUserFeedback.NextAction == "" || result.WeChatUserFeedback.ButtonFeedback == "" || len(result.WeChatUserFeedback.Checks) == 0 {
		t.Fatalf("wechat user feedback = %#v", result.WeChatUserFeedback)
	}
	if result.OperationsRuntimeClosure.NextAction == "" || result.OperationsRuntimeClosure.RuntimeParameterStatus == "" || len(result.OperationsRuntimeClosure.Checks) == 0 {
		t.Fatalf("operations runtime closure = %#v", result.OperationsRuntimeClosure)
	}
	if result.OpsPanelConfig.ParameterGroup == "" || len(result.OpsPanelConfig.DisplayItems) == 0 || result.OpsPanelConfig.RefreshIntervalSeconds == 0 {
		t.Fatalf("ops panel config = %#v", result.OpsPanelConfig)
	}
	if result.MonitorAutoReport.AuditEvent == "" || result.MonitorAutoReport.WeChatSendStatus == "" || len(result.MonitorAutoReport.Checks) == 0 {
		t.Fatalf("monitor auto report = %#v", result.MonitorAutoReport)
	}
	if result.WriteRampStage.CurrentStage == "" || result.WriteRampStage.NextStage == "" || result.WriteRampStage.DefaultAction != "reject_or_require_approval" {
		t.Fatalf("write ramp stage = %#v", result.WriteRampStage)
	}
	if result.WeChatFeedbackLoop.ProcessingState == "" || result.WeChatFeedbackLoop.NextAction == "" || len(result.WeChatFeedbackLoop.Checks) == 0 {
		t.Fatalf("wechat feedback loop = %#v", result.WeChatFeedbackLoop)
	}
	if result.OperationsClosedLoop.NextAction == "" || result.OperationsClosedLoop.OpsPanelStatus == "" || len(result.OperationsClosedLoop.Checks) == 0 {
		t.Fatalf("operations closed loop = %#v", result.OperationsClosedLoop)
	}
	if len(result.OpsDashboardInteraction.Actions) == 0 || result.OpsDashboardInteraction.AuditEvent == "" || len(result.OpsDashboardInteraction.Checks) == 0 {
		t.Fatalf("ops dashboard interaction = %#v", result.OpsDashboardInteraction)
	}
	if result.AlertDedupeEscalation.DedupeKey == "" || result.AlertDedupeEscalation.DedupeWindowSeconds == 0 || result.AlertDedupeEscalation.EscalationCondition == "" {
		t.Fatalf("alert dedupe escalation = %#v", result.AlertDedupeEscalation)
	}
	if result.WriteStageRecord.CurrentStage == "" || result.WriteStageRecord.TargetStage == "" || result.WriteStageRecord.DefaultAction != "reject_or_require_approval" {
		t.Fatalf("write stage record = %#v", result.WriteStageRecord)
	}
	if result.WeChatFeedbackTicket.TicketType == "" || result.WeChatFeedbackTicket.OwnerEntry == "" || result.WeChatFeedbackTicket.AuditEvent == "" {
		t.Fatalf("wechat feedback ticket = %#v", result.WeChatFeedbackTicket)
	}
	if result.OperationsHandling.NextAction == "" || result.OperationsHandling.DashboardStatus == "" || len(result.OperationsHandling.Checks) == 0 {
		t.Fatalf("operations handling = %#v", result.OperationsHandling)
	}
	if len(result.OpsActionDefinition.Actions) == 0 || result.OpsActionDefinition.Actions[0].HandlerEntry == "" || result.OpsActionDefinition.Actions[0].IdempotencyKey == "" {
		t.Fatalf("ops action definition = %#v", result.OpsActionDefinition)
	}
	if result.AlertEscalationPolicy.EscalationLevel == "" || len(result.AlertEscalationPolicy.NotificationChannels) == 0 || result.AlertEscalationPolicy.RepeatSuppression == "" {
		t.Fatalf("alert escalation policy = %#v", result.AlertEscalationPolicy)
	}
	if result.WriteStageApproval.ApprovalStatus == "" || result.WriteStageApproval.TargetStage == "" || result.WriteStageApproval.DefaultAction != "reject_or_require_approval" {
		t.Fatalf("write stage approval = %#v", result.WriteStageApproval)
	}
	if result.FeedbackTicketLifecycle.CreatedState == "" || result.FeedbackTicketLifecycle.HandoffState == "" || len(result.FeedbackTicketLifecycle.Checks) == 0 {
		t.Fatalf("feedback ticket lifecycle = %#v", result.FeedbackTicketLifecycle)
	}
	if result.OperationsActionClosure.NextAction == "" || result.OperationsActionClosure.OpsActionStatus == "" || len(result.OperationsActionClosure.Checks) == 0 {
		t.Fatalf("operations action closure = %#v", result.OperationsActionClosure)
	}
	if len(result.OpsAPIExecution.Executions) == 0 || result.OpsAPIExecution.Executions[0].ExecutionEntry == "" || result.OpsAPIExecution.Executions[0].AuditEvent == "" {
		t.Fatalf("ops api execution = %#v", result.OpsAPIExecution)
	}
	if result.AlertEscalationReceipt.DeliveryStatus == "" || result.AlertEscalationReceipt.SuppressionResult == "" || result.AlertEscalationReceipt.HandoffEntry == "" {
		t.Fatalf("alert escalation receipt = %#v", result.AlertEscalationReceipt)
	}
	if len(result.WriteApprovalButton.Buttons) == 0 || result.WriteApprovalButton.Buttons[0].ButtonKey == "" || result.WriteApprovalButton.Buttons[0].AuditEvidence == "" {
		t.Fatalf("write approval button = %#v", result.WriteApprovalButton)
	}
	if result.FeedbackTicketSLA.FirstResponseSeconds == 0 || result.FeedbackTicketSLA.ResolveSeconds == 0 || result.FeedbackTicketSLA.HandoffPath == "" {
		t.Fatalf("feedback ticket sla = %#v", result.FeedbackTicketSLA)
	}
	if result.OperationsExecution.NextAction == "" || result.OperationsExecution.OpsAPIExecutionStatus == "" || len(result.OperationsExecution.Checks) == 0 {
		t.Fatalf("operations execution = %#v", result.OperationsExecution)
	}
	if len(result.OpsExecutionRecord.Records) == 0 || result.OpsExecutionRecord.Records[0].RecordKey == "" || result.OpsExecutionRecord.Records[0].ReplayEntry == "" {
		t.Fatalf("ops execution record = %#v", result.OpsExecutionRecord)
	}
	if result.WeChatApprovalCallback.CallbackKey == "" || result.WeChatApprovalCallback.Source != "wechat_work" || result.WeChatApprovalCallback.StorageState == "" {
		t.Fatalf("wechat approval callback = %#v", result.WeChatApprovalCallback)
	}
	if result.FeedbackSLAReport.ReportAuditEvent == "" || result.FeedbackSLAReport.FirstResponseRate < 0 || result.FeedbackSLAReport.ResolveRate < 0 {
		t.Fatalf("feedback sla report = %#v", result.FeedbackSLAReport)
	}
	if result.AlertAutoRecovery.RecoveryTrigger == "" || result.AlertAutoRecovery.SuppressionRelease == "" || result.AlertAutoRecovery.AuditEvidence == "" {
		t.Fatalf("alert auto recovery = %#v", result.AlertAutoRecovery)
	}
	if result.OperationsEvidence.NextAction == "" || result.OperationsEvidence.ExecutionRecordStatus == "" || len(result.OperationsEvidence.Checks) == 0 {
		t.Fatalf("operations evidence = %#v", result.OperationsEvidence)
	}
	if result.UnifiedProgressComponent.ComponentKey == "" || result.UnifiedProgressComponent.WebStatus == "" || result.UnifiedProgressComponent.WeChatStatus == "" {
		t.Fatalf("unified progress component = %#v", result.UnifiedProgressComponent)
	}
	if result.EvidenceDetailPage.DetailEntry == "" || result.EvidenceDetailPage.RecordCount == 0 || result.EvidenceDetailPage.RetentionPolicy == "" {
		t.Fatalf("evidence detail page = %#v", result.EvidenceDetailPage)
	}
	if result.CallbackReplayTool.CallbackKey == "" || result.CallbackReplayTool.ReplayEntry == "" || result.CallbackReplayTool.IdempotencyGuard == "" {
		t.Fatalf("callback replay tool = %#v", result.CallbackReplayTool)
	}
	if result.RecoveryPolicyConfig.PolicyKey == "" || result.RecoveryPolicyConfig.DefaultPolicy == "" || result.RecoveryPolicyConfig.SuppressionWindow == "" {
		t.Fatalf("recovery policy config = %#v", result.RecoveryPolicyConfig)
	}
	if result.DualEndProgressEvidence.NextAction == "" || result.DualEndProgressEvidence.UnifiedProgressStatus == "" || len(result.DualEndProgressEvidence.Checks) == 0 {
		t.Fatalf("dual end progress evidence = %#v", result.DualEndProgressEvidence)
	}
	if result.WeChatProgressCard.CardKey == "" || result.WeChatProgressCard.ProgressPercent == 0 || len(result.WeChatProgressCard.Actions) == 0 {
		t.Fatalf("wechat progress card = %#v", result.WeChatProgressCard)
	}
	if len(result.WebEvidenceInteraction.Filters) == 0 || result.WebEvidenceInteraction.ReplayEntry == "" || result.WebEvidenceInteraction.Visibility == "" {
		t.Fatalf("web evidence interaction = %#v", result.WebEvidenceInteraction)
	}
	if result.CallbackReplayPermission.PermissionKey == "" || len(result.CallbackReplayPermission.AllowedRoles) == 0 || result.CallbackReplayPermission.AuditEvent == "" {
		t.Fatalf("callback replay permission = %#v", result.CallbackReplayPermission)
	}
	if result.RecoveryPolicyAudit.ChangeKey == "" || result.RecoveryPolicyAudit.ApprovalStatus == "" || result.RecoveryPolicyAudit.RollbackPath == "" {
		t.Fatalf("recovery policy audit = %#v", result.RecoveryPolicyAudit)
	}
	if result.DualEndInteraction.NextAction == "" || result.DualEndInteraction.WeChatProgressCardStatus == "" || len(result.DualEndInteraction.Checks) == 0 {
		t.Fatalf("dual end interaction = %#v", result.DualEndInteraction)
	}
	if result.WeChatTemplateRender.TemplateKey == "" || result.WeChatTemplateRender.RenderStatus == "" || result.WeChatTemplateRender.SendEntry == "" || len(result.WeChatTemplateRender.ButtonFields) == 0 {
		t.Fatalf("wechat template render = %#v", result.WeChatTemplateRender)
	}
	if result.WebEvidenceRoute.RouteName == "" || len(result.WebEvidenceRoute.PathParams) == 0 || result.WebEvidenceRoute.PermissionRequirement == "" || result.WebEvidenceRoute.ReplayEntry == "" {
		t.Fatalf("web evidence route = %#v", result.WebEvidenceRoute)
	}
	if result.CallbackReplayApproval.ApprovalKey == "" || result.CallbackReplayApproval.RequestEntry == "" || result.CallbackReplayApproval.ExecutionGate == "" || len(result.CallbackReplayApproval.ApprovalRoles) == 0 {
		t.Fatalf("callback replay approval = %#v", result.CallbackReplayApproval)
	}
	if result.RecoveryPolicyPersist.ConfigKey == "" || result.RecoveryPolicyPersist.CurrentVersion == "" || result.RecoveryPolicyPersist.PendingVersion == "" || result.RecoveryPolicyPersist.RollbackVersion == "" {
		t.Fatalf("recovery policy persist = %#v", result.RecoveryPolicyPersist)
	}
	if result.DualEndInteractionLaunch.NextAction == "" || result.DualEndInteractionLaunch.WeChatTemplateRenderStatus == "" || len(result.DualEndInteractionLaunch.Checks) == 0 {
		t.Fatalf("dual end interaction launch = %#v", result.DualEndInteractionLaunch)
	}
	if result.WeChatTemplateSend.MessageType != "template_card" || result.WeChatTemplateSend.SendEntry == "" || result.WeChatTemplateSend.FallbackText == "" || result.WeChatTemplateSend.AuditEvent == "" {
		t.Fatalf("wechat template send = %#v", result.WeChatTemplateSend)
	}
	if result.WebEvidenceDetailView.RoutePath == "" || result.WebEvidenceDetailView.PlanParam == "" || result.WebEvidenceDetailView.RecordParam == "" || result.WebEvidenceDetailView.PermissionHint == "" {
		t.Fatalf("web evidence detail view = %#v", result.WebEvidenceDetailView)
	}
	if result.CallbackReplayExecution.RequestEntry == "" || result.CallbackReplayExecution.ExecuteEntry == "" || result.CallbackReplayExecution.IdempotencyKey == "" || result.CallbackReplayExecution.FailureFallback == "" {
		t.Fatalf("callback replay execution = %#v", result.CallbackReplayExecution)
	}
	if result.RecoveryPolicyVersion.PolicyKey == "" || result.RecoveryPolicyVersion.CurrentVersion == "" || result.RecoveryPolicyVersion.ReleaseStatus == "" || result.RecoveryPolicyVersion.AuditEvent == "" {
		t.Fatalf("recovery policy version = %#v", result.RecoveryPolicyVersion)
	}
	if result.DualEndRealInteraction.NextAction == "" || result.DualEndRealInteraction.WeChatTemplateSendStatus == "" || len(result.DualEndRealInteraction.Checks) == 0 {
		t.Fatalf("dual end real interaction = %#v", result.DualEndRealInteraction)
	}
	if result.WeChatTemplateIntegration.SendPath == "" || result.WeChatTemplateIntegration.FallbackStatus == "" || result.WeChatTemplateIntegration.DegradeStrategy == "" || result.WeChatTemplateIntegration.MessageIDReadback == "" {
		t.Fatalf("wechat template integration = %#v", result.WeChatTemplateIntegration)
	}
	if result.WebEvidenceInteractionDetail.FilterMode == "" || result.WebEvidenceInteractionDetail.ExpandMode == "" || result.WebEvidenceInteractionDetail.AuditTimeline == "" || result.WebEvidenceInteractionDetail.ReplayRequestEntry == "" {
		t.Fatalf("web evidence interaction detail = %#v", result.WebEvidenceInteractionDetail)
	}
	if result.CallbackReplaySafetyAudit.IdempotencyCheck == "" || result.CallbackReplaySafetyAudit.ApprovalCheck == "" || result.CallbackReplaySafetyAudit.SignatureCheck == "" || result.CallbackReplaySafetyAudit.FailureAudit == "" {
		t.Fatalf("callback replay safety audit = %#v", result.CallbackReplaySafetyAudit)
	}
	if result.RecoveryPolicyGrayRelease.GrayStage == "" || result.RecoveryPolicyGrayRelease.RollbackCondition == "" || result.RecoveryPolicyGrayRelease.ApprovalStatus == "" || result.RecoveryPolicyGrayRelease.AuditEvidence == "" {
		t.Fatalf("recovery policy gray release = %#v", result.RecoveryPolicyGrayRelease)
	}
	if result.DualEndRunLoop.NextAction == "" || result.DualEndRunLoop.WeChatTemplateIntegrationStatus == "" || len(result.DualEndRunLoop.Checks) == 0 {
		t.Fatalf("dual end run loop = %#v", result.DualEndRunLoop)
	}
	if result.WeChatTemplatePilot.PilotBatch == "" || result.WeChatTemplatePilot.TemplateStatus == "" || result.WeChatTemplatePilot.MessageIDStatus == "" {
		t.Fatalf("wechat template pilot = %#v", result.WeChatTemplatePilot)
	}
	if result.WebEvidenceUserAction.FilterAction == "" || result.WebEvidenceUserAction.ExpandAction == "" || result.WebEvidenceUserAction.PermissionResult == "" {
		t.Fatalf("web evidence user action = %#v", result.WebEvidenceUserAction)
	}
	if result.CallbackReplayResultTrace.ExecutionResult == "" || result.CallbackReplayResultTrace.IdempotencyHit == "" || result.CallbackReplayResultTrace.AuditRecord == "" {
		t.Fatalf("callback replay result trace = %#v", result.CallbackReplayResultTrace)
	}
	if result.RecoveryPolicyAutomation.AutoAdvance == "" || result.RecoveryPolicyAutomation.NextPercent < result.RecoveryPolicyAutomation.CurrentPercent || result.RecoveryPolicyAutomation.AuditEvidence == "" {
		t.Fatalf("recovery policy automation = %#v", result.RecoveryPolicyAutomation)
	}
	if result.DualEndTaskClosure.NextAction == "" || result.DualEndTaskClosure.WeChatPilotStatus == "" || len(result.DualEndTaskClosure.Checks) == 0 {
		t.Fatalf("dual end task closure = %#v", result.DualEndTaskClosure)
	}
	if result.WeChatTemplatePilotMetric.BatchID == "" || result.WeChatTemplatePilotMetric.SendStatus == "" || result.WeChatTemplatePilotMetric.AuditRef == "" {
		t.Fatalf("wechat template pilot metric = %#v", result.WeChatTemplatePilotMetric)
	}
	if result.WebEvidenceOperation.FilterEntry == "" || result.WebEvidenceOperation.ReplayRequestEntry == "" || result.WebEvidenceOperation.OperationCount == 0 {
		t.Fatalf("web evidence operation = %#v", result.WebEvidenceOperation)
	}
	if result.CallbackReplayResultQuery.QueryEntry == "" || result.CallbackReplayResultQuery.IdempotencyResult == "" || result.CallbackReplayResultQuery.AuditRef == "" {
		t.Fatalf("callback replay result query = %#v", result.CallbackReplayResultQuery)
	}
	if result.RecoveryAutomationExecution.ExecutionMode == "" || result.RecoveryAutomationExecution.ApprovalGate == "" || result.RecoveryAutomationExecution.NextPercent < result.RecoveryAutomationExecution.CurrentPercent {
		t.Fatalf("recovery automation execution = %#v", result.RecoveryAutomationExecution)
	}
	if result.RealInteractionAutomation.NextAction == "" || result.RealInteractionAutomation.PilotMetricStatus == "" || len(result.RealInteractionAutomation.Checks) == 0 {
		t.Fatalf("real interaction automation = %#v", result.RealInteractionAutomation)
	}
	if result.WeChatWebProgressLink.ProgressURL == "" || result.WeChatWebProgressLink.DeliveryChannel != "wechat_work" || result.WeChatWebProgressLink.BrowserTarget != "web_browser" {
		t.Fatalf("wechat web progress link = %#v", result.WeChatWebProgressLink)
	}
	if result.WeChatWebProgressLink.URLSource != "agent_progress_notification" || result.WeChatWebProgressLink.TemplateStatus != "succeeded" || result.WeChatWebProgressLink.FallbackStatus != "not_attempted" {
		t.Fatalf("wechat web progress link audit fields = %#v", result.WeChatWebProgressLink)
	}
	if result.Report.ByEntry["web"] != 1 || result.Report.ByCapability["web.search"] != 1 || result.Report.ByHandoff["required"] != 1 {
		t.Fatalf("report = %#v", result.Report)
	}
	if !auditEventExists(repository.audits, "agent.production_drill_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_policy_decision") ||
		!auditEventExists(repository.audits, "agent.write_sandbox_snapshot") ||
		!auditEventExists(repository.audits, "agent.e2e_acceptance_snapshot") ||
		!auditEventExists(repository.audits, "agent.real_integration_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_acceptance_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_gray_policy_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_channel_snapshot") ||
		!auditEventExists(repository.audits, "agent.launch_drill_record") ||
		!auditEventExists(repository.audits, "agent.wechat_native_integration_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_replay_snapshot") ||
		!auditEventExists(repository.audits, "agent.launch_approval_snapshot") ||
		!auditEventExists(repository.audits, "agent.production_daily_report") ||
		!auditEventExists(repository.audits, "agent.preprod_acceptance_snapshot") ||
		!auditEventExists(repository.audits, "agent.button_loop_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_execute_snapshot") ||
		!auditEventExists(repository.audits, "agent.daily_report_persist_snapshot") ||
		!auditEventExists(repository.audits, "agent.post_launch_monitor_snapshot") ||
		!auditEventExists(repository.audits, "agent.release_approval_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.button_callback_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_audit_review_snapshot") ||
		!auditEventExists(repository.audits, "agent.daily_report_send_snapshot") ||
		!auditEventExists(repository.audits, "agent.monitor_alert_drill_snapshot") ||
		!auditEventExists(repository.audits, "agent.button_direct_control_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_e2e_acceptance_snapshot") ||
		!auditEventExists(repository.audits, "agent.release_window_readiness_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_gray_expansion_snapshot") ||
		!auditEventExists(repository.audits, "agent.external_monitor_integration_snapshot") ||
		!auditEventExists(repository.audits, "agent.release_window_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.external_monitor_runtime_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_gray_review_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_acceptance_review_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_daily_closure_snapshot") ||
		!auditEventExists(repository.audits, "agent.production_release_snapshot") ||
		!auditEventExists(repository.audits, "agent.external_monitor_config_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_ramp_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_signoff_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_handoff_snapshot") ||
		!auditEventExists(repository.audits, "agent.production_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.monitor_integration_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_ramp_policy_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_final_report_snapshot") ||
		!auditEventExists(repository.audits, "agent.launch_runtime_overview_snapshot") ||
		!auditEventExists(repository.audits, "agent.runtime_parameters_snapshot") ||
		!auditEventExists(repository.audits, "agent.monitor_readback_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_ramp_recommendation_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_user_feedback_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_runtime_closure_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_panel_config_snapshot") ||
		!auditEventExists(repository.audits, "agent.monitor_auto_report_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_ramp_stage_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_feedback_loop_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_closed_loop_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_dashboard_interaction_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_dedupe_escalation_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_stage_record_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_feedback_ticket_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_handling_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_action_definition_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_escalation_policy_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_stage_approval_snapshot") ||
		!auditEventExists(repository.audits, "agent.feedback_ticket_lifecycle_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_action_closure_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_api_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_escalation_receipt_snapshot") ||
		!auditEventExists(repository.audits, "agent.write_approval_button_snapshot") ||
		!auditEventExists(repository.audits, "agent.feedback_ticket_sla_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.ops_execution_record_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_approval_callback_snapshot") ||
		!auditEventExists(repository.audits, "agent.feedback_sla_report_snapshot") ||
		!auditEventExists(repository.audits, "agent.alert_auto_recovery_snapshot") ||
		!auditEventExists(repository.audits, "agent.operations_evidence_snapshot") ||
		!auditEventExists(repository.audits, "agent.unified_progress_component_snapshot") ||
		!auditEventExists(repository.audits, "agent.evidence_detail_page_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_tool_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_config_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_progress_evidence_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_progress_card_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_interaction_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_permission_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_audit_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_interaction_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_template_render_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_route_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_approval_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_persist_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_interaction_launch_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_template_send_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_detail_view_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_version_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_real_interaction_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_template_integration_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_interaction_detail_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_safety_audit_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_gray_release_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_run_loop_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_template_pilot_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_user_action_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_result_trace_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_policy_automation_snapshot") ||
		!auditEventExists(repository.audits, "agent.dual_end_task_closure_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_template_pilot_metric_snapshot") ||
		!auditEventExists(repository.audits, "agent.web_evidence_operation_snapshot") ||
		!auditEventExists(repository.audits, "agent.callback_replay_result_query_snapshot") ||
		!auditEventExists(repository.audits, "agent.recovery_automation_execution_snapshot") ||
		!auditEventExists(repository.audits, "agent.real_interaction_automation_snapshot") ||
		!auditEventExists(repository.audits, "agent.wechat_web_progress_link_snapshot") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func agentActionExists(actions []AgentWeChatActionResponse, key string) bool {
	for _, action := range actions {
		if action.Key == key {
			return true
		}
	}
	return false
}

func agentNativeActionExists(actions []AgentWeChatNativeActionResponse, key string) bool {
	for _, action := range actions {
		if action.Key == key {
			return true
		}
	}
	return false
}

func agentButtonCallbackExists(actions []AgentButtonCallbackActionResponse, key string) bool {
	for _, action := range actions {
		if action.Key == key && action.Handler != "" {
			return true
		}
	}
	return false
}

func fakeProgressPhaseExists(phases []AgentProgressPhaseResponse, key string) bool {
	for _, phase := range phases {
		if phase.Key == key {
			return true
		}
	}
	return false
}

func TestAgentSessionServiceCancelScheduledTaskWritesAuditAndProgressEvent(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := &fakeAgentProgressRepository{
		tasks: []domain.AgentScheduledTask{
			{ID: 20, UserID: 1, SessionID: 2, TurnID: 3, Status: domain.AgentScheduledTaskStatusQueued, Goal: "定时摘要", UpdatedAt: now.Add(-time.Minute)},
		},
	}
	service := NewAgentSessionService(repository, WithAgentSessionNow(func() time.Time { return now }))

	result, err := service.CancelScheduledTask(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, 20)
	if err != nil {
		t.Fatalf("CancelScheduledTask() error = %v", err)
	}
	if result.Task.Status != string(domain.AgentScheduledTaskStatusCanceled) {
		t.Fatalf("task = %#v", result.Task)
	}
	if len(repository.audits) != 1 || repository.audits[0].EventType != "agent.scheduled_task_canceled" {
		t.Fatalf("audits = %#v", repository.audits)
	}
	progress, err := service.GetProgress(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, AgentProgressQuery{ScheduledTaskID: 20})
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}
	found := false
	foundAudit := false
	for _, event := range progress.Progress.RecentEvents {
		if event.Kind == "scheduled_task" && event.Status == string(domain.AgentScheduledTaskStatusCanceled) {
			found = true
		}
		if event.Kind == "audit" && event.Source == "user_action" && event.Title == "agent.scheduled_task_canceled" {
			foundAudit = true
		}
	}
	if !found {
		t.Fatalf("events = %#v", progress.Progress.RecentEvents)
	}
	if !foundAudit {
		t.Fatalf("events = %#v, want audit event", progress.Progress.RecentEvents)
	}
}

func TestAgentSessionServiceCallbackReplayAPIsWriteAuditAndGateExecution(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := &fakeAgentProgressRepository{}
	service := NewAgentSessionService(repository, WithAgentSessionNow(func() time.Time { return now }))
	auth := CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}

	requested, err := service.RequestCallbackReplayApproval(context.Background(), auth, AgentCallbackReplayInput{PlanID: 10, CallbackKey: "approval", ReplayEntry: "web.agent.callback.replay.approval", Reason: "验证回调"})
	if err != nil {
		t.Fatalf("RequestCallbackReplayApproval() error = %v", err)
	}
	if requested.ReplayExecution.ApprovalStatus != "approval_required" || requested.ReplayExecution.ExecutionGate == "" || requested.ReplayExecution.IdempotencyKey == "" {
		t.Fatalf("requested = %#v", requested)
	}

	blocked, err := service.ExecuteCallbackReplay(context.Background(), auth, AgentCallbackReplayInput{PlanID: 10, CallbackKey: "approval", ReplayEntry: "web.agent.callback.replay.approval"})
	if err != nil {
		t.Fatalf("ExecuteCallbackReplay() blocked error = %v", err)
	}
	if blocked.ReplayExecution.Status != "blocked" || blocked.ReplayExecution.ApprovalStatus != "approval_required" {
		t.Fatalf("blocked = %#v", blocked)
	}

	approved, err := service.ExecuteCallbackReplay(context.Background(), auth, AgentCallbackReplayInput{PlanID: 10, CallbackKey: "approval", ReplayEntry: "web.agent.callback.replay.approval", Approved: true})
	if err != nil {
		t.Fatalf("ExecuteCallbackReplay() approved error = %v", err)
	}
	if approved.ReplayExecution.Status != "ready" || approved.ReplayExecution.ApprovalStatus != "approved" || approved.ReplayExecution.ExecutionGate != "approved_and_idempotency_verified" {
		t.Fatalf("approved = %#v", approved)
	}
	if !auditEventExists(repository.audits, "agent.callback_replay_requested") ||
		!auditEventExists(repository.audits, "agent.callback_replay_execute_blocked") ||
		!auditEventExists(repository.audits, "agent.callback_replay_execute_requested") {
		t.Fatalf("audits = %#v", repository.audits)
	}
}

type fakeAgentProgressRepository struct {
	plan   domain.AgentPlan
	plans  []domain.AgentPlan
	runs   []domain.AgentRun
	tasks  []domain.AgentScheduledTask
	audits []domain.AgentAuditLog
}

func (r *fakeAgentProgressRepository) ListAgentSessions(context.Context, int64) ([]domain.AgentSession, error) {
	return nil, nil
}

func (r *fakeAgentProgressRepository) GetAgentSession(context.Context, int64, int64) (domain.AgentSession, error) {
	return domain.AgentSession{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) CreateAgentSession(context.Context, domain.AgentSession) (domain.AgentSession, error) {
	return domain.AgentSession{}, nil
}

func (r *fakeAgentProgressRepository) ListExternalAccounts(context.Context, int64) ([]domain.ExternalAccount, error) {
	return nil, nil
}

func (r *fakeAgentProgressRepository) GetExternalAccount(context.Context, int64, int64) (domain.ExternalAccount, error) {
	return domain.ExternalAccount{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) SetExternalAccountActiveSession(context.Context, int64, int64, int64) (domain.ExternalAccount, error) {
	return domain.ExternalAccount{}, nil
}

func (r *fakeAgentProgressRepository) GetAgentSessionStats(context.Context, int64, int64) (domain.AgentSessionStats, error) {
	return domain.AgentSessionStats{}, nil
}

func (r *fakeAgentProgressRepository) ClearAgentSessionContext(context.Context, int64, int64, time.Time) (domain.AgentSessionStats, error) {
	return domain.AgentSessionStats{}, nil
}

func (r *fakeAgentProgressRepository) RebuildAgentSessionContext(context.Context, int64, int64, time.Time) (domain.AgentSessionStats, error) {
	return domain.AgentSessionStats{}, nil
}

func (r *fakeAgentProgressRepository) DeleteAgentSession(context.Context, int64, int64) error {
	return nil
}

func (r *fakeAgentProgressRepository) ListRecentTranscriptEntries(context.Context, domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error) {
	return nil, nil
}

func (r *fakeAgentProgressRepository) ListAgentRunsByTurn(_ context.Context, _ int64, turnID int64) ([]domain.AgentRun, error) {
	runs := make([]domain.AgentRun, 0)
	for _, run := range r.runs {
		if run.TurnID == turnID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (r *fakeAgentProgressRepository) GetAgentRunDetail(_ context.Context, _ int64, runID int64) (domain.AgentRun, error) {
	for _, run := range r.runs {
		if run.ID == runID {
			return run, nil
		}
	}
	return domain.AgentRun{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) ListAgentPlans(_ context.Context, _ int64, _ int64, turnID int64, _ int) ([]domain.AgentPlan, error) {
	if turnID == 0 && len(r.plans) > 0 {
		return append([]domain.AgentPlan(nil), r.plans...), nil
	}
	if r.plan.TurnID == turnID {
		return []domain.AgentPlan{r.plan}, nil
	}
	return nil, nil
}

func (r *fakeAgentProgressRepository) GetAgentPlan(_ context.Context, _ int64, planID int64) (domain.AgentPlan, error) {
	if r.plan.ID == planID {
		return r.plan, nil
	}
	return domain.AgentPlan{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) GetAgentScheduledTask(_ context.Context, _ int64, taskID int64) (domain.AgentScheduledTask, error) {
	for _, task := range r.tasks {
		if task.ID == taskID {
			return task, nil
		}
	}
	return domain.AgentScheduledTask{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) ListAgentScheduledTasks(_ context.Context, _ domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error) {
	return append([]domain.AgentScheduledTask(nil), r.tasks...), nil
}

func (r *fakeAgentProgressRepository) UpdateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	for index, existing := range r.tasks {
		if existing.ID == task.ID && existing.UserID == task.UserID {
			r.tasks[index] = task
			return task, nil
		}
	}
	return domain.AgentScheduledTask{}, domain.ErrNotFound
}

func (r *fakeAgentProgressRepository) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = int64(len(r.audits) + 1)
	r.audits = append(r.audits, log)
	return log, nil
}

func (r *fakeAgentProgressRepository) ListAgentScheduledTasksByRefs(_ context.Context, _ int64, planID int64, turnID int64, runID int64, _ int) ([]domain.AgentScheduledTask, error) {
	tasks := make([]domain.AgentScheduledTask, 0)
	for _, task := range r.tasks {
		if (planID > 0 && task.PlanID == planID) || (turnID > 0 && task.TurnID == turnID) || (runID > 0 && task.SourceRunID == runID) {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (r *fakeAgentProgressRepository) ListAuditLogsByRefs(_ context.Context, options domain.AgentAuditLogListOptions) ([]domain.AgentAuditLog, error) {
	logs := make([]domain.AgentAuditLog, 0)
	for _, log := range r.audits {
		if options.UserID > 0 && log.UserID != options.UserID {
			continue
		}
		if options.SessionID > 0 && log.SessionID != options.SessionID {
			continue
		}
		if options.TurnID > 0 && log.TurnID != options.TurnID {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}
