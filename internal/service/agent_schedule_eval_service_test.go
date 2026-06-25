package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestAgentScheduleEvalServiceCreateScheduledTask(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	task, err := service.CreateScheduledTask(context.Background(), CreateAgentScheduledTaskInput{
		UserID:              1,
		SessionID:           2,
		TurnID:              3,
		Goal:                "明天早上总结最新 AI 新闻",
		AllowedCapabilities: []string{"web.search", "content.summarize_text"},
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask() error = %v", err)
	}
	if task.Status != domain.AgentScheduledTaskStatusQueued {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.ScheduledAt != now {
		t.Fatalf("scheduled_at = %v, want %v", task.ScheduledAt, now)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("stored tasks = %d, want 1", len(store.tasks))
	}
}

func TestAgentScheduleEvalServiceFailScheduledTaskQueuesRetry(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	store := &fakeAgentScheduleEvalRepository{}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	task, err := service.FailScheduledTask(context.Background(), domain.AgentScheduledTask{
		ID:           8,
		UserID:       1,
		Status:       domain.AgentScheduledTaskStatusRunning,
		AttemptCount: 1,
		MaxAttempts:  3,
	}, "temporary failure", &next)
	if err != nil {
		t.Fatalf("FailScheduledTask() error = %v", err)
	}
	if task.Status != domain.AgentScheduledTaskStatusQueued {
		t.Fatalf("status = %q, want queued", task.Status)
	}
	if task.NextRunAt == nil || !task.NextRunAt.Equal(next) {
		t.Fatalf("next_run_at = %v, want %v", task.NextRunAt, next)
	}
}

func TestAgentScheduleEvalServiceRecordEvalResult(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RecordEvalResult(context.Background(), RecordAgentEvalResultInput{
		RunID:        10,
		CaseID:       20,
		Status:       domain.AgentEvalResultStatusPassed,
		Score:        0.9,
		Expected:     "reject unsafe action",
		Actual:       "rejected",
		EvidenceRefs: []string{"agent_run:1"},
	})
	if err != nil {
		t.Fatalf("RecordEvalResult() error = %v", err)
	}
	if result.Status != domain.AgentEvalResultStatusPassed {
		t.Fatalf("status = %q, want passed", result.Status)
	}
	if len(store.results) != 1 {
		t.Fatalf("stored results = %d, want 1", len(store.results))
	}
}

func TestAgentScheduleEvalServiceRunBuiltinEvalRecordsSafetyResults(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RunBuiltinEval(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, RunBuiltinAgentEvalInput{Trigger: "test"})
	if err != nil {
		t.Fatalf("RunBuiltinEval() error = %v", err)
	}
	wantCases := len(builtinAgentEvalCases(now))
	if result.Run.CaseCount != wantCases || result.Run.PassedCount != wantCases || result.Run.FailedCount != 0 {
		t.Fatalf("run counts = %#v", result.Run)
	}
	if result.Run.Status != string(domain.AgentEvalRunStatusCompleted) {
		t.Fatalf("run status = %q, want completed", result.Run.Status)
	}
	foundPromptInjection := false
	for _, evalCase := range store.evalCases {
		if evalCase.CaseKey == "prompt_injection" {
			foundPromptInjection = true
			break
		}
	}
	if !foundPromptInjection {
		t.Fatalf("builtin cases = %#v, want prompt_injection", store.evalCases)
	}
	if len(result.Run.Results) != wantCases {
		t.Fatalf("results = %d, want %d", len(result.Run.Results), wantCases)
	}
	for _, evalResult := range result.Run.Results {
		if evalResult.Status != string(domain.AgentEvalResultStatusPassed) || len(evalResult.EvidenceRefs) == 0 {
			t.Fatalf("eval result = %#v", evalResult)
		}
	}
}

func TestAgentScheduleEvalServiceListEvalRunsIncludesTrend(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		evalRuns: []domain.AgentEvalRun{
			{ID: 1, UserID: 1, Status: domain.AgentEvalRunStatusCompleted, CaseCount: 10, PassedCount: 9, FailedCount: 1, UpdatedAt: now.Add(-time.Hour)},
			{ID: 2, UserID: 1, Status: domain.AgentEvalRunStatusFailed, CaseCount: 10, PassedCount: 8, FailedCount: 2, ErrorMessage: "oracle failed", UpdatedAt: now},
		},
	}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.ListEvalRuns(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, 20)
	if err != nil {
		t.Fatalf("ListEvalRuns() error = %v", err)
	}
	if len(result.Runs) != 2 || result.Trend.RunCount != 2 || result.Trend.PassedCount != 17 || result.Trend.FailedResultCount != 3 {
		t.Fatalf("result = %#v", result)
	}
	if result.Trend.PassRate != 0.85 || len(result.Trend.FailureSummary) == 0 {
		t.Fatalf("trend = %#v", result.Trend)
	}
}

func TestAgentScheduleEvalServiceNotificationPreferenceDefaultsAndUpdates(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	defaults, err := service.GetNotificationPreference(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}})
	if err != nil {
		t.Fatalf("GetNotificationPreference() error = %v", err)
	}
	if !defaults.ProcessNotificationsEnabled || !defaults.FinalReportsEnabled || !defaults.FailureNotificationsEnabled || !defaults.RecoveryNotificationsEnabled {
		t.Fatalf("defaults = %#v", defaults)
	}
	if defaults.MaxConcurrentTasks != 2 || defaults.MaxQueuedTasks != 20 || !defaults.AutoRecoveryEnabled || defaults.QualityHandoffThreshold != 0.65 || !defaults.HandoffOnFailure || !defaults.HandoffOnPermission || !defaults.HandoffOnBudget {
		t.Fatalf("policy defaults = %#v", defaults)
	}
	if len(defaults.CapabilityPolicy) != 0 {
		t.Fatalf("capability policy default = %#v", defaults.CapabilityPolicy)
	}
	if defaults.DailyTaskQuota != 50 || defaults.DailyExternalCallQuota != 200 || defaults.DailyCapabilityCallQuota != 500 {
		t.Fatalf("quota defaults = %#v", defaults)
	}

	disabled := false
	maxConcurrent := 1
	threshold := 0.8
	capabilityPolicy := domain.AgentJSON{"web.search": "confirm", "repo.*": "reject"}
	dailyTaskQuota := 3
	updated, err := service.UpdateNotificationPreference(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, UpdateAgentNotificationPreferenceInput{
		FinalReportsEnabled:     &disabled,
		MaxConcurrentTasks:      &maxConcurrent,
		QualityHandoffThreshold: &threshold,
		CapabilityPolicy:        capabilityPolicy,
		DailyTaskQuota:          &dailyTaskQuota,
	})
	if err != nil {
		t.Fatalf("UpdateNotificationPreference() error = %v", err)
	}
	if updated.FinalReportsEnabled || updated.MaxConcurrentTasks != 1 || updated.QualityHandoffThreshold != 0.8 || updated.CapabilityPolicy["web.search"] != "confirm" || updated.DailyTaskQuota != 3 {
		t.Fatalf("updated = %#v", updated)
	}
	if store.preference.UserID != 1 || store.preference.FinalReportsEnabled || store.preference.MaxConcurrentTasks != 1 || store.preference.CapabilityPolicy["repo.*"] != "reject" || store.preference.DailyTaskQuota != 3 {
		t.Fatalf("stored preference = %#v", store.preference)
	}
}

func TestAgentScheduleEvalServiceRetryPlanStepQueuesApprovedStep(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		plan: domain.AgentPlan{
			ID:        7,
			UserID:    1,
			SessionID: 2,
			TurnID:    3,
			Status:    domain.AgentPlanStatusFailed,
			Steps: []domain.AgentPlanStep{
				{
					ID:              11,
					PlanID:          7,
					Status:          domain.AgentPlanStepStatusFailed,
					FailureStrategy: "retry once when transient",
					ExecutorRunID:   20,
					ObservationRef:  "observation:20:web.search",
					ErrorMessage:    "network timeout",
					MaxRetries:      1,
				},
			},
		},
	}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RetryPlanStep(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, RetryAgentPlanStepInput{
		PlanID: 7,
		StepID: 11,
		Reason: "temporary network failure",
	})
	if err != nil {
		t.Fatalf("RetryPlanStep() error = %v", err)
	}
	if result.Step.Status != string(domain.AgentPlanStepStatusApproved) || result.Step.RetryCount != 1 {
		t.Fatalf("step = %#v", result.Step)
	}
	if result.Step.RetryMetadata["previous_observation_ref"] != "observation:20:web.search" {
		t.Fatalf("retry metadata = %#v", result.Step.RetryMetadata)
	}
	if store.plan.Status != domain.AgentPlanStatusExecuting {
		t.Fatalf("plan status = %q, want executing", store.plan.Status)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.plan_step_retry_queued" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAgentScheduleEvalServiceRetryPlanQueuesRetryableFailedSteps(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		plan: domain.AgentPlan{
			ID:        7,
			UserID:    1,
			SessionID: 2,
			TurnID:    3,
			Status:    domain.AgentPlanStatusFailed,
			Steps: []domain.AgentPlanStep{
				{ID: 11, PlanID: 7, Status: domain.AgentPlanStepStatusFailed, FailureStrategy: "retry once", ErrorMessage: "timeout", MaxRetries: 1},
				{ID: 12, PlanID: 7, Status: domain.AgentPlanStepStatusFailed, FailureStrategy: "no_retry", ErrorMessage: "policy denied", MaxRetries: 1},
				{ID: 13, PlanID: 7, Status: domain.AgentPlanStepStatusFailed, FailureStrategy: "retry once", RetryCount: 1, MaxRetries: 1},
				{ID: 14, PlanID: 7, Status: domain.AgentPlanStepStatusCompleted},
			},
		},
	}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RetryPlan(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, RetryAgentPlanInput{
		PlanID: 7,
		Reason: "batch retry",
	})
	if err != nil {
		t.Fatalf("RetryPlan() error = %v", err)
	}
	if result.Queued != 1 || result.Skipped != 2 || result.Exhausted != 1 || len(result.Steps) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if store.plan.Status != domain.AgentPlanStatusExecuting {
		t.Fatalf("plan status = %q, want executing", store.plan.Status)
	}
	if store.plan.Steps[0].Status != domain.AgentPlanStepStatusApproved || store.plan.Steps[0].RetryCount != 1 {
		t.Fatalf("step = %#v", store.plan.Steps[0])
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.plan_retry_queued" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAgentScheduleEvalServiceRecoverPlanQueuesExecutingSteps(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-10 * time.Minute)
	store := &fakeAgentScheduleEvalRepository{
		plan: domain.AgentPlan{
			ID:        7,
			UserID:    1,
			SessionID: 2,
			TurnID:    3,
			Status:    domain.AgentPlanStatusExecuting,
			Steps: []domain.AgentPlanStep{
				{ID: 11, PlanID: 7, Status: domain.AgentPlanStepStatusExecuting, StartedAt: &startedAt, ErrorMessage: "worker stopped"},
				{ID: 12, PlanID: 7, Status: domain.AgentPlanStepStatusCompleted},
			},
		},
	}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RecoverPlan(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, RecoverAgentPlanInput{
		PlanID: 7,
		Reason: "worker restart",
	})
	if err != nil {
		t.Fatalf("RecoverPlan() error = %v", err)
	}
	if result.Plan.Status != string(domain.AgentPlanStatusExecuting) {
		t.Fatalf("plan = %#v", result.Plan)
	}
	if store.plan.Steps[0].Status != domain.AgentPlanStepStatusApproved || store.plan.Steps[0].StartedAt != nil || store.plan.Steps[0].RetryMetadata["recover_reason"] != "worker restart" {
		t.Fatalf("step = %#v", store.plan.Steps[0])
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.plan_recovered" {
		t.Fatalf("audits = %#v", store.audits)
	}
	if store.audits[0].Metadata["recovery_strategy"] != "recover_interrupted_step" || store.audits[0].Metadata["recovery_result"] != "queued" {
		t.Fatalf("audit metadata = %#v", store.audits[0].Metadata)
	}
	recovery := store.plan.Metadata["recovery"].(domain.AgentJSON)
	if recovery["recovery_strategy"] != "recover_interrupted_step" || recovery["recovered_steps"] != 1 {
		t.Fatalf("plan recovery metadata = %#v", recovery)
	}
}

func TestAgentScheduleEvalServiceRecoverScheduledTaskQueuesRunningTask(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	lockedAt := now.Add(-30 * time.Minute)
	completedAt := now.Add(-time.Minute)
	store := &fakeAgentScheduleEvalRepository{
		tasks: []domain.AgentScheduledTask{
			{
				ID:          9,
				UserID:      1,
				SessionID:   2,
				TurnID:      3,
				PlanID:      7,
				Status:      domain.AgentScheduledTaskStatusRunning,
				ScheduledAt: now.Add(time.Hour),
				LockedBy:    "worker-1",
				LockedAt:    &lockedAt,
				LastError:   "stale lock",
				CompletedAt: &completedAt,
			},
		},
	}
	service := NewAgentScheduleEvalService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := service.RecoverScheduledTask(context.Background(), CurrentAuth{Authenticated: true, User: domain.User{ID: 1}}, RecoverAgentScheduledTaskInput{
		TaskID: 9,
		Reason: "lock expired",
	})
	if err != nil {
		t.Fatalf("RecoverScheduledTask() error = %v", err)
	}
	if result.Task.Status != string(domain.AgentScheduledTaskStatusQueued) {
		t.Fatalf("task = %#v", result.Task)
	}
	task := store.tasks[0]
	if task.LockedBy != "" || task.LockedAt != nil || task.LastError != "" || task.CompletedAt != nil || task.NextRunAt == nil || !task.NextRunAt.Equal(now) || !task.ScheduledAt.Equal(now) {
		t.Fatalf("stored task = %#v", task)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.scheduled_task_recovered" {
		t.Fatalf("audits = %#v", store.audits)
	}
	if store.audits[0].Metadata["recovery_strategy"] != "recover_interrupted_task" || store.audits[0].Metadata["recovery_result"] != "queued" {
		t.Fatalf("audit metadata = %#v", store.audits[0].Metadata)
	}
}

func TestAgentScheduleEvalServiceBuildControllerRunTaskPacket(t *testing.T) {
	scheduledAt := time.Date(2026, 6, 26, 1, 0, 0, 0, time.UTC)
	service := NewAgentScheduleEvalService(&fakeAgentScheduleEvalRepository{})
	packet := service.BuildControllerRunTaskPacket(domain.AgentScheduledTask{
		ID:                  9,
		TaskType:            "digest",
		Goal:                "汇总 AI 新闻",
		TargetChannel:       "wechat_work_app",
		TargetRef:           "zhangsan",
		ScheduledAt:         scheduledAt,
		FreshnessPolicy:     "latest_at_run",
		AllowedCapabilities: []string{"web.search", "content.summarize_text"},
		ModelPolicy:         domain.AgentJSON{"model_key": "default"},
		FailurePolicy:       domain.AgentJSON{"on_failure": "report_to_user"},
		Payload:             domain.AgentJSON{"content": "汇总 AI 新闻"},
	})
	if packet["scheduled_task_id"] != int64(9) || packet["goal"] != "汇总 AI 新闻" {
		t.Fatalf("packet = %#v", packet)
	}
	if packet["scheduled_at"] != "2026-06-26T01:00:00Z" {
		t.Fatalf("scheduled_at = %#v", packet["scheduled_at"])
	}
}

func TestAgentScheduledTaskWorkerServiceRunDueOnceCreatesControllerRun(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		tasks: []domain.AgentScheduledTask{
			{
				ID:                  12,
				UserID:              1,
				SessionID:           2,
				TurnID:              3,
				Status:              domain.AgentScheduledTaskStatusQueued,
				TaskType:            "digest",
				Goal:                "汇总 AI 新闻",
				TargetChannel:       "wechat_work_app",
				TargetRef:           "zhangsan",
				ScheduledAt:         now.Add(-time.Minute),
				AllowedCapabilities: []string{"web.search", "content.summarize_text"},
				Payload:             domain.AgentJSON{"trace_id": "trace-1"},
				MaxAttempts:         1,
			},
		},
	}
	sender := &fakeNotificationSender{}
	worker := NewAgentScheduledTaskWorkerService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))
	worker.SetReportSender(sender)

	result, err := worker.RunDueOnce(context.Background(), RunDueAgentScheduledTasksInput{WorkerID: "worker-1", Limit: 5})
	if err != nil {
		t.Fatalf("RunDueOnce() error = %v", err)
	}
	if result.Claimed != 1 || result.Succeeded != 1 || result.ReportSent != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %#v", result)
	}
	item := result.Items[0]
	if item.TaskID != 12 || item.RunID == 0 || item.Status != string(domain.AgentScheduledTaskStatusSucceeded) || item.ReportStatus != "succeeded" {
		t.Fatalf("item = %#v", item)
	}
	if !strings.Contains(item.ReportText, "controller run") {
		t.Fatalf("report = %q", item.ReportText)
	}
	if !strings.Contains(item.ReportText, "证据引用") || !strings.Contains(item.ReportText, "agent_scheduled_task:12") || !strings.Contains(item.ReportText, "agent_run:1") {
		t.Fatalf("report evidence = %q", item.ReportText)
	}
	if len(store.runs) != 1 {
		t.Fatalf("runs = %#v", store.runs)
	}
	run := store.runs[0]
	if run.Role != domain.AgentRunRoleController || run.Status != domain.AgentRunStatusSucceeded {
		t.Fatalf("run = %#v", run)
	}
	if run.TaskPacket["scheduled_task_id"] != int64(12) || run.TraceID != "trace-1" {
		t.Fatalf("run packet/trace = %#v / %q", run.TaskPacket, run.TraceID)
	}
	if store.tasks[0].Status != domain.AgentScheduledTaskStatusSucceeded || store.tasks[0].SourceRunID != run.ID {
		t.Fatalf("task = %#v", store.tasks[0])
	}
	if len(store.traces) != 1 || store.traces[0].TraceKind != "scheduled_task_controller_input" {
		t.Fatalf("traces = %#v", store.traces)
	}
	if len(sender.messages) != 1 || sender.messages[0].ToUser != "zhangsan" {
		t.Fatalf("sent messages = %#v", sender.messages)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.scheduled_task_report" || store.audits[0].Status != "succeeded" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAgentScheduledTaskWorkerServiceDefersTaskWhenUserPolicyThrottles(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 30, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		preference: domain.AgentNotificationPreference{
			UserID:                       1,
			ProcessNotificationsEnabled:  true,
			FinalReportsEnabled:          true,
			FailureNotificationsEnabled:  true,
			RecoveryNotificationsEnabled: true,
			MaxConcurrentTasks:           1,
			MaxQueuedTasks:               20,
			AutoRecoveryEnabled:          true,
			QualityHandoffThreshold:      0.65,
			HandoffOnFailure:             true,
			HandoffOnPermission:          true,
			HandoffOnBudget:              true,
			CreatedAt:                    now,
			UpdatedAt:                    now,
		},
		tasks: []domain.AgentScheduledTask{
			{ID: 12, UserID: 1, SessionID: 2, TurnID: 3, Status: domain.AgentScheduledTaskStatusQueued, TaskType: "digest", Goal: "汇总 AI 新闻", ScheduledAt: now.Add(-time.Minute), MaxAttempts: 1},
			{ID: 13, UserID: 1, SessionID: 2, TurnID: 4, Status: domain.AgentScheduledTaskStatusRunning, TaskType: "digest", Goal: "运行中任务", ScheduledAt: now.Add(-time.Minute), MaxAttempts: 1},
		},
	}
	worker := NewAgentScheduledTaskWorkerService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))

	result, err := worker.RunDueOnce(context.Background(), RunDueAgentScheduledTasksInput{WorkerID: "worker-1", Limit: 1})
	if err != nil {
		t.Fatalf("RunDueOnce() error = %v", err)
	}
	if result.ReportSkipped != 1 || len(store.runs) != 0 {
		t.Fatalf("result = %#v runs = %#v", result, store.runs)
	}
	if store.tasks[0].Status != domain.AgentScheduledTaskStatusQueued || store.tasks[0].LockedBy != "" || store.tasks[0].LastError == "" {
		t.Fatalf("task = %#v", store.tasks[0])
	}
	if len(store.audits) != 1 || store.audits[0].EventType != "agent.scheduled_task_throttled" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAgentScheduledTaskWorkerServiceSkipsReportWhenPreferenceDisablesFinalReports(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{
		preference: domain.AgentNotificationPreference{
			UserID:                       1,
			ProcessNotificationsEnabled:  true,
			FinalReportsEnabled:          false,
			FailureNotificationsEnabled:  true,
			RecoveryNotificationsEnabled: true,
			CreatedAt:                    now,
			UpdatedAt:                    now,
		},
		tasks: []domain.AgentScheduledTask{
			{
				ID:            12,
				UserID:        1,
				SessionID:     2,
				TurnID:        3,
				Status:        domain.AgentScheduledTaskStatusQueued,
				TaskType:      "digest",
				Goal:          "汇总 AI 新闻",
				TargetChannel: domain.AgentProviderWeChatWorkApp,
				TargetRef:     "zhangsan",
				ScheduledAt:   now.Add(-time.Minute),
				MaxAttempts:   1,
			},
		},
	}
	sender := &fakeNotificationSender{}
	worker := NewAgentScheduledTaskWorkerService(store, WithAgentScheduleEvalNow(func() time.Time { return now }))
	worker.SetReportSender(sender)

	result, err := worker.RunDueOnce(context.Background(), RunDueAgentScheduledTasksInput{WorkerID: "worker-1", Limit: 5})
	if err != nil {
		t.Fatalf("RunDueOnce() error = %v", err)
	}
	if result.ReportSkipped != 1 || len(sender.messages) != 0 {
		t.Fatalf("result = %#v, messages = %#v", result, sender.messages)
	}
	if len(store.audits) != 1 || store.audits[0].Metadata["preference_skipped"] != true {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAgentScheduledTaskFinalReportRedactsSensitiveValues(t *testing.T) {
	report := AgentScheduledTaskFinalReport(domain.AgentScheduledTask{
		ID:     9,
		UserID: 1,
		Status: domain.AgentScheduledTaskStatusFailed,
	}, domain.AgentRun{ID: 3}, "api_key=secret-value token:abcdef database_url=postgres://user:pass@localhost/db")
	if strings.Contains(report, "secret-value") || strings.Contains(report, "abcdef") || strings.Contains(report, "postgres://") {
		t.Fatalf("report = %q", report)
	}
	if !strings.Contains(report, "[redacted]") {
		t.Fatalf("report = %q", report)
	}
}

func TestAgentP0CapabilityExecutorScheduleTaskCreatesScheduledTask(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	executor := agentP0CapabilityExecutor{
		scheduledTasks: store,
		now:            func() time.Time { return now },
	}
	result, err := executor.ExecuteTool(context.Background(), agent.ToolExecuteInput{
		Capability:     agent.Capability{Key: "agent.schedule_task"},
		UserID:         1,
		SessionID:      2,
		TurnID:         3,
		ExternalUserID: "zhangsan",
		Message:        "明天上午9点汇总 AI 新闻",
		RawArguments:   `{"task_type":"digest","goal":"汇总 AI 新闻","scheduled_at":"2026-06-25T09:00:00+08:00","allowed_capabilities":["web.search","content.summarize_text"],"confirmed":true}`,
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if result.Observation.Status != "succeeded" {
		t.Fatalf("observation = %#v", result.Observation)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("stored tasks = %d, want 1", len(store.tasks))
	}
	task := store.tasks[0]
	if task.TaskType != "digest" || task.Goal != "汇总 AI 新闻" || task.TargetChannel != "wechat_work_app" {
		t.Fatalf("task = %#v", task)
	}
	if task.Payload["source"] != "agent.schedule_task" {
		t.Fatalf("payload = %#v", task.Payload)
	}
}

func TestAgentRunRecordingExecutorSuccessWritesEvidenceRefs(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	runManager := agent.NewRunManager(agent.RunManagerOptions{Store: store, Now: func() time.Time { return now }})
	executor := agentRunRecordingExecutor{runManager: runManager}

	observation := executor.recordExecutorSuccess(context.Background(), domain.AgentRun{ID: 5, TurnID: 3}, "web.search", "query", agent.CapabilityObservation{
		Summary: "found result",
	}, "output content", 1)

	if len(store.artifacts) != 1 || !containsAgentString(store.artifacts[0].SourceRefs, "capability:web.search") || !containsAgentString(store.artifacts[0].SourceRefs, "agent_run:5") {
		t.Fatalf("artifact refs = %#v", store.artifacts)
	}
	if len(store.observations) != 1 || !containsAgentString(store.observations[0].ArtifactRefs, "agent_artifact:1") {
		t.Fatalf("stored observations = %#v", store.observations)
	}
	if observation.ObservationRef != "agent_observations/1" || !containsAgentString(observation.ArtifactRefs, "agent_observation:1") || !containsAgentString(observation.ArtifactRefs, "agent_turn:3") {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestAgentRunRecordingExecutorFailureWritesEvidenceRefs(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	store := &fakeAgentScheduleEvalRepository{}
	runManager := agent.NewRunManager(agent.RunManagerOptions{Store: store, Now: func() time.Time { return now }})
	executor := agentRunRecordingExecutor{runManager: runManager}

	observation := executor.recordExecutorFailure(context.Background(), domain.AgentRun{ID: 5, TurnID: 3}, "local.history.search", "query", domain.ErrInvalidInput)

	if len(store.observations) != 1 || !containsAgentString(store.observations[0].ArtifactRefs, "capability:local.history.search") || !containsAgentString(store.observations[0].ArtifactRefs, "agent_run:5") {
		t.Fatalf("stored observations = %#v", store.observations)
	}
	if observation.ObservationRef != "agent_observations/1" || !containsAgentString(observation.ArtifactRefs, "agent_observation:1") {
		t.Fatalf("observation = %#v", observation)
	}
}

type fakeAgentScheduleEvalRepository struct {
	tasks        []domain.AgentScheduledTask
	results      []domain.AgentEvalResult
	evalCases    []domain.AgentEvalCase
	evalRuns     []domain.AgentEvalRun
	plan         domain.AgentPlan
	runs         []domain.AgentRun
	traces       []domain.AgentRunContextTrace
	observations []domain.AgentObservation
	artifacts    []domain.AgentArtifact
	audits       []domain.AgentAuditLog
	preference   domain.AgentNotificationPreference
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	task.ID = int64(len(r.tasks) + 1)
	r.tasks = append(r.tasks, task)
	return task, nil
}

func (r *fakeAgentScheduleEvalRepository) GetAgentScheduledTask(_ context.Context, userID int64, taskID int64) (domain.AgentScheduledTask, error) {
	for _, task := range r.tasks {
		if task.UserID == userID && task.ID == taskID {
			return task, nil
		}
	}
	return domain.AgentScheduledTask{}, domain.ErrNotFound
}

func (r *fakeAgentScheduleEvalRepository) ListAgentScheduledTasks(_ context.Context, _ domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error) {
	return append([]domain.AgentScheduledTask(nil), r.tasks...), nil
}

func (r *fakeAgentScheduleEvalRepository) ClaimDueAgentScheduledTasks(_ context.Context, input domain.AgentScheduledTaskClaimInput) ([]domain.AgentScheduledTask, error) {
	var claimed []domain.AgentScheduledTask
	limit := input.Limit
	if limit < 1 {
		limit = 10
	}
	for index, task := range r.tasks {
		if len(claimed) >= limit {
			break
		}
		if task.Status != domain.AgentScheduledTaskStatusQueued || task.ScheduledAt.After(input.Now) {
			continue
		}
		task.Status = domain.AgentScheduledTaskStatusRunning
		task.LockedBy = input.WorkerID
		lockedAt := input.Now
		task.LockedAt = &lockedAt
		task.AttemptCount++
		r.tasks[index] = task
		claimed = append(claimed, task)
	}
	return claimed, nil
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	for index, existing := range r.tasks {
		if existing.ID == task.ID && existing.UserID == task.UserID {
			r.tasks[index] = task
			return task, nil
		}
	}
	r.tasks = append(r.tasks, task)
	return task, nil
}

func (r *fakeAgentScheduleEvalRepository) GetAgentNotificationPreference(_ context.Context, userID int64) (domain.AgentNotificationPreference, error) {
	if r.preference.UserID == userID && userID > 0 {
		return r.preference, nil
	}
	return domain.AgentNotificationPreference{}, domain.ErrNotFound
}

func (r *fakeAgentScheduleEvalRepository) UpsertAgentNotificationPreference(_ context.Context, preference domain.AgentNotificationPreference) (domain.AgentNotificationPreference, error) {
	r.preference = preference
	return preference, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentEvalCase(_ context.Context, evalCase domain.AgentEvalCase) (domain.AgentEvalCase, error) {
	evalCase.ID = int64(len(r.evalCases) + 1)
	r.evalCases = append(r.evalCases, evalCase)
	return evalCase, nil
}

func (r *fakeAgentScheduleEvalRepository) UpsertAgentEvalCase(_ context.Context, evalCase domain.AgentEvalCase) (domain.AgentEvalCase, error) {
	for index, existing := range r.evalCases {
		if existing.CaseKey == evalCase.CaseKey {
			evalCase.ID = existing.ID
			r.evalCases[index] = evalCase
			return evalCase, nil
		}
	}
	evalCase.ID = int64(len(r.evalCases) + 1)
	r.evalCases = append(r.evalCases, evalCase)
	return evalCase, nil
}

func (r *fakeAgentScheduleEvalRepository) ListAgentEvalCases(_ context.Context, _ domain.AgentEvalCaseListOptions) ([]domain.AgentEvalCase, error) {
	return append([]domain.AgentEvalCase(nil), r.evalCases...), nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentEvalRun(_ context.Context, run domain.AgentEvalRun) (domain.AgentEvalRun, error) {
	run.ID = int64(len(r.evalRuns) + 1)
	r.evalRuns = append(r.evalRuns, run)
	return run, nil
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentEvalRun(_ context.Context, run domain.AgentEvalRun) (domain.AgentEvalRun, error) {
	for index, existing := range r.evalRuns {
		if existing.ID == run.ID {
			r.evalRuns[index] = run
			return r.GetAgentEvalRunDetail(context.Background(), run.ID)
		}
	}
	r.evalRuns = append(r.evalRuns, run)
	return r.GetAgentEvalRunDetail(context.Background(), run.ID)
}

func (r *fakeAgentScheduleEvalRepository) ListAgentEvalRuns(_ context.Context, _ domain.AgentEvalRunListOptions) ([]domain.AgentEvalRun, error) {
	return append([]domain.AgentEvalRun(nil), r.evalRuns...), nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentEvalResult(_ context.Context, result domain.AgentEvalResult) (domain.AgentEvalResult, error) {
	result.ID = int64(len(r.results) + 1)
	r.results = append(r.results, result)
	return result, nil
}

func (r *fakeAgentScheduleEvalRepository) GetAgentEvalRunDetail(_ context.Context, runID int64) (domain.AgentEvalRun, error) {
	for _, run := range r.evalRuns {
		if run.ID == runID {
			run.Results = nil
			for _, result := range r.results {
				if result.RunID == runID {
					run.Results = append(run.Results, result)
				}
			}
			return run, nil
		}
	}
	return domain.AgentEvalRun{ID: runID, Results: append([]domain.AgentEvalResult(nil), r.results...)}, nil
}

func (r *fakeAgentScheduleEvalRepository) GetAgentPlan(_ context.Context, userID int64, planID int64) (domain.AgentPlan, error) {
	if r.plan.UserID == userID && r.plan.ID == planID {
		return r.plan, nil
	}
	return domain.AgentPlan{}, domain.ErrNotFound
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentPlanStepStatus(_ context.Context, userID int64, step domain.AgentPlanStep) (domain.AgentPlanStep, error) {
	if r.plan.UserID != userID || r.plan.ID != step.PlanID {
		return domain.AgentPlanStep{}, domain.ErrNotFound
	}
	for index, existing := range r.plan.Steps {
		if existing.ID == step.ID {
			r.plan.Steps[index] = step
			return step, nil
		}
	}
	return domain.AgentPlanStep{}, domain.ErrNotFound
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentPlanStatus(_ context.Context, userID int64, planID int64, status domain.AgentPlanStatus, _ time.Time, errorMessage string) (domain.AgentPlan, error) {
	if r.plan.UserID != userID || r.plan.ID != planID {
		return domain.AgentPlan{}, domain.ErrNotFound
	}
	r.plan.Status = status
	r.plan.ErrorMessage = errorMessage
	return r.plan, nil
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentPlanMetadata(_ context.Context, userID int64, planID int64, metadata domain.AgentJSON, _ time.Time) (domain.AgentPlan, error) {
	if r.plan.UserID != userID || r.plan.ID != planID {
		return domain.AgentPlan{}, domain.ErrNotFound
	}
	r.plan.Metadata = cloneServiceAgentJSON(metadata)
	return r.plan, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	run.ID = int64(len(r.runs) + 1)
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAgentScheduleEvalRepository) UpdateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	for index, existing := range r.runs {
		if existing.ID == run.ID {
			r.runs[index] = run
			return run, nil
		}
	}
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentRunContextTrace(_ context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error) {
	trace.ID = int64(len(r.traces) + 1)
	r.traces = append(r.traces, trace)
	return trace, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentObservation(_ context.Context, observation domain.AgentObservation) (domain.AgentObservation, error) {
	observation.ID = int64(len(r.observations) + 1)
	r.observations = append(r.observations, observation)
	return observation, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAgentArtifact(_ context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error) {
	artifact.ID = int64(len(r.artifacts) + 1)
	r.artifacts = append(r.artifacts, artifact)
	return artifact, nil
}

func (r *fakeAgentScheduleEvalRepository) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = int64(len(r.audits) + 1)
	r.audits = append(r.audits, log)
	return log, nil
}
