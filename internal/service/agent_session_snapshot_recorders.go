package service

import (
	"context"
	"messagefeed/internal/domain"
)

func (s *AgentSessionService) recordAgentAlertPolicyDecision(ctx context.Context, userID int64, policy AgentAlertPolicyResponse, alerts AgentAlertSummaryResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	status := policy.Status
	if status == "" {
		status = "inactive"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_policy_decision",
		Status:    status,
		Message:   policy.Summary,
		Metadata: domain.AgentJSON{
			"alert_total":     alerts.Total,
			"critical":        alerts.Critical,
			"warning":         alerts.Warning,
			"reasons":         alerts.Reasons,
			"enabled_reasons": policy.EnabledReasons,
			"muted_reasons":   policy.MutedReasons,
			"decision_count":  len(policy.Decisions),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentProductionDrillSnapshot(ctx context.Context, userID int64, drill AgentProductionDrillResponse, trend AgentTrendSnapshotResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	status := drill.Status
	if status == "" {
		status = "unknown"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.production_drill_snapshot",
		Status:    status,
		Message:   drill.Summary,
		Metadata: domain.AgentJSON{
			"source":             drill.Source,
			"check_count":        len(drill.Checks),
			"trend_source":       trend.Source,
			"retention_days":     trend.RetentionDays,
			"trend_bucket_count": len(trend.Buckets),
			"generated_at":       drill.GeneratedAt,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteSandboxSnapshot(ctx context.Context, userID int64, sandbox AgentWriteSandboxResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_sandbox_snapshot",
		Status:    sandbox.Status,
		Message:   sandbox.Summary,
		Metadata: domain.AgentJSON{
			"default_action": sandbox.DefaultAction,
			"check_count":    len(sandbox.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentE2EAcceptanceSnapshot(ctx context.Context, userID int64, e2e AgentE2EAcceptanceResponse, loadTest AgentLoadTestSummaryResponse, callback AgentWeChatCallbackReadinessResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.e2e_acceptance_snapshot",
		Status:    e2e.Status,
		Message:   e2e.Summary,
		Metadata: domain.AgentJSON{
			"check_count":               len(e2e.Checks),
			"load_test_status":          loadTest.Status,
			"wechat_callback_status":    callback.Status,
			"load_test_user_count":      loadTest.Metrics.Users,
			"load_test_progress_events": loadTest.Metrics.ProgressEvents,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRealIntegrationSnapshot(ctx context.Context, userID int64, integration AgentRealIntegrationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.real_integration_snapshot",
		Status:    integration.Status,
		Message:   integration.Summary,
		Metadata: domain.AgentJSON{
			"risk_count":    len(integration.Risks),
			"blocker_count": len(integration.Blockers),
			"check_count":   len(integration.Checks),
			"next_action":   integration.NextAction,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsAcceptanceSnapshot(ctx context.Context, userID int64, ops AgentOpsAcceptanceResponse, leastPrivilege AgentWriteLeastPrivilegeResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_acceptance_snapshot",
		Status:    ops.Status,
		Message:   ops.Summary,
		Metadata: domain.AgentJSON{
			"check_count":              len(ops.Checks),
			"write_policy_status":      leastPrivilege.Status,
			"write_allowed_candidates": leastPrivilege.AllowedCandidates,
			"write_denied_patterns":    leastPrivilege.DeniedPatterns,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteGraySnapshot(ctx context.Context, userID int64, gray AgentWriteGrayPolicyResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_gray_policy_snapshot",
		Status:    gray.Status,
		Message:   gray.Summary,
		Metadata: domain.AgentJSON{
			"candidates":         gray.Candidates,
			"allowed_user_scope": gray.AllowedUserScope,
			"requires_approval":  gray.RequiresApproval,
			"requires_budget":    gray.RequiresBudget,
			"requires_audit":     gray.RequiresAudit,
			"rollback_triggers":  gray.RollbackTriggers,
			"check_count":        len(gray.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentAlertChannelSnapshot(ctx context.Context, userID int64, channel AgentAlertChannelResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_channel_snapshot",
		Status:    channel.Status,
		Message:   channel.Summary,
		Metadata: domain.AgentJSON{
			"channel_count": len(channel.Channels),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentLaunchDrillRecord(ctx context.Context, userID int64, drill AgentLaunchDrillRecordResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.launch_drill_record",
		Status:    drill.Status,
		Message:   drill.Summary,
		Metadata: domain.AgentJSON{
			"batch_id":      drill.BatchID,
			"triggered_by":  drill.TriggeredBy,
			"result":        drill.Result,
			"risk_count":    len(drill.Risks),
			"blocker_count": len(drill.Blockers),
			"check_count":   len(drill.Checks),
			"next_action":   drill.NextAction,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatNativeIntegrationSnapshot(ctx context.Context, userID int64, integration AgentWeChatNativeIntegrationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_native_integration_snapshot",
		Status:    integration.Status,
		Message:   integration.Summary,
		Metadata: domain.AgentJSON{
			"risk_count":    len(integration.Risks),
			"blocker_count": len(integration.Blockers),
			"check_count":   len(integration.Checks),
			"next_action":   integration.NextAction,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteReplaySnapshot(ctx context.Context, userID int64, replay AgentWriteReplayResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_replay_snapshot",
		Status:    replay.Status,
		Message:   replay.Summary,
		Metadata: domain.AgentJSON{
			"candidates":        replay.Candidates,
			"approval_status":   replay.ApprovalStatus,
			"budget_status":     replay.BudgetStatus,
			"permission_status": replay.PermissionStatus,
			"execution_status":  replay.ExecutionStatus,
			"audit_status":      replay.AuditStatus,
			"rollback_triggers": replay.RollbackTriggers,
			"check_count":       len(replay.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentLaunchApprovalSnapshot(ctx context.Context, userID int64, approval AgentLaunchApprovalResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.launch_approval_snapshot",
		Status:    approval.Status,
		Message:   approval.Summary,
		Metadata: domain.AgentJSON{
			"request_id":    approval.RequestID,
			"review_state":  approval.ReviewState,
			"approved":      approval.Approved,
			"rejected":      approval.Rejected,
			"expired":       approval.Expired,
			"handoff_path":  approval.HandoffPath,
			"rollback_path": approval.RollbackPath,
			"check_count":   len(approval.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDailyReportSnapshot(ctx context.Context, userID int64, report AgentDailyReportResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.production_daily_report",
		Status:    report.Status,
		Message:   report.Summary,
		Metadata: domain.AgentJSON{
			"date":                report.Date,
			"task_count":          report.TaskCount,
			"success_rate":        report.SuccessRate,
			"failure_count":       report.FailureCount,
			"alert_count":         report.AlertCount,
			"estimated_tokens":    report.EstimatedTokens,
			"trend_buckets":       report.TrendBuckets,
			"handoff_count":       report.HandoffCount,
			"recovery_count":      report.RecoveryCount,
			"notification_count":  report.NotificationCount,
			"notification_failed": report.NotificationFailed,
			"check_count":         len(report.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentPreprodAcceptanceSnapshot(ctx context.Context, userID int64, preprod AgentPreprodAcceptanceResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.preprod_acceptance_snapshot",
		Status:    preprod.Status,
		Message:   preprod.Summary,
		Metadata: domain.AgentJSON{
			"risk_count":    len(preprod.Risks),
			"blocker_count": len(preprod.Blockers),
			"check_count":   len(preprod.Checks),
			"next_action":   preprod.NextAction,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentButtonLoopSnapshot(ctx context.Context, userID int64, loop AgentButtonLoopResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.button_loop_snapshot",
		Status:    loop.Status,
		Message:   loop.Summary,
		Metadata: domain.AgentJSON{
			"action_count":  len(loop.Actions),
			"check_count":   len(loop.Checks),
			"fallback_text": loop.FallbackText,
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteExecuteSnapshot(ctx context.Context, userID int64, execute AgentWriteExecuteResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_execute_snapshot",
		Status:    execute.Status,
		Message:   execute.Summary,
		Metadata: domain.AgentJSON{
			"candidates":        execute.Candidates,
			"default_action":    execute.DefaultAction,
			"approval_status":   execute.ApprovalStatus,
			"budget_status":     execute.BudgetStatus,
			"permission_status": execute.PermissionStatus,
			"execution_status":  execute.ExecutionStatus,
			"audit_status":      execute.AuditStatus,
			"rollback_triggers": execute.RollbackTriggers,
			"check_count":       len(execute.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDailyPersistSnapshot(ctx context.Context, userID int64, persist AgentDailyPersistResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.daily_report_persist_snapshot",
		Status:    persist.Status,
		Message:   persist.Summary,
		Metadata: domain.AgentJSON{
			"record_key":  persist.RecordKey,
			"source":      persist.Source,
			"retained":    persist.Retained,
			"check_count": len(persist.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentPostLaunchMonitorSnapshot(ctx context.Context, userID int64, monitor AgentPostLaunchMonitorResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.post_launch_monitor_snapshot",
		Status:    monitor.Status,
		Message:   monitor.Summary,
		Metadata: domain.AgentJSON{
			"check_count": len(monitor.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentReleaseApprovalSnapshot(ctx context.Context, userID int64, approval AgentReleaseApprovalResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.release_approval_execution_snapshot",
		Status:    approval.Status,
		Message:   approval.Summary,
		Metadata: domain.AgentJSON{
			"request_id":     approval.RequestID,
			"review_state":   approval.ReviewState,
			"executable":     approval.Executable,
			"approved":       approval.Approved,
			"rejected":       approval.Rejected,
			"expired":        approval.Expired,
			"decision_path":  approval.DecisionPath,
			"rejection_path": approval.RejectionPath,
			"rollback_path":  approval.RollbackPath,
			"audit_event":    approval.AuditEvent,
			"check_count":    len(approval.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentButtonCallbackSnapshot(ctx context.Context, userID int64, callback AgentButtonCallbackResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.button_callback_snapshot",
		Status:    callback.Status,
		Message:   callback.Summary,
		Metadata: domain.AgentJSON{
			"action_count":  len(callback.Actions),
			"fallback_text": callback.FallbackText,
			"check_count":   len(callback.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteAuditReviewSnapshot(ctx context.Context, userID int64, review AgentWriteAuditReviewResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_audit_review_snapshot",
		Status:    review.Status,
		Message:   review.Summary,
		Metadata: domain.AgentJSON{
			"candidates":          review.Candidates,
			"approval_evidence":   review.ApprovalEvidence,
			"budget_evidence":     review.BudgetEvidence,
			"permission_evidence": review.PermissionEvidence,
			"execution_evidence":  review.ExecutionEvidence,
			"failure_evidence":    review.FailureEvidence,
			"rollback_evidence":   review.RollbackEvidence,
			"handoff_evidence":    review.HandoffEvidence,
			"check_count":         len(review.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDailySendSnapshot(ctx context.Context, userID int64, send AgentDailySendResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.daily_report_send_snapshot",
		Status:    send.Status,
		Message:   send.Summary,
		Metadata: domain.AgentJSON{
			"record_key":           send.RecordKey,
			"schedule_status":      send.ScheduleStatus,
			"delivery_status":      send.DeliveryStatus,
			"retry_status":         send.RetryStatus,
			"wechat_report_status": send.WeChatReportStatus,
			"check_count":          len(send.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentMonitorAlertDrillSnapshot(ctx context.Context, userID int64, drill AgentMonitorAlertDrillResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.monitor_alert_drill_snapshot",
		Status:    drill.Status,
		Message:   drill.Summary,
		Metadata: domain.AgentJSON{
			"trigger_status":      drill.TriggerStatus,
			"notification_status": drill.NotificationStatus,
			"handoff_status":      drill.HandoffStatus,
			"check_count":         len(drill.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentButtonDirectControlSnapshot(ctx context.Context, userID int64, control AgentButtonDirectControlResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.button_direct_control_snapshot",
		Status:    control.Status,
		Message:   control.Summary,
		Metadata: domain.AgentJSON{
			"executed":     control.Executed,
			"skipped":      control.Skipped,
			"action_count": len(control.Actions),
			"check_count":  len(control.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatE2EAcceptanceSnapshot(ctx context.Context, userID int64, e2e AgentWeChatE2EAcceptanceResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_e2e_acceptance_snapshot",
		Status:    e2e.Status,
		Message:   e2e.Summary,
		Metadata: domain.AgentJSON{
			"check_count": len(e2e.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentReleaseWindowReadinessSnapshot(ctx context.Context, userID int64, window AgentReleaseWindowReadinessResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.release_window_readiness_snapshot",
		Status:    window.Status,
		Message:   window.Summary,
		Metadata: domain.AgentJSON{
			"window_state": window.WindowState,
			"check_count":  len(window.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteGrayExpansionSnapshot(ctx context.Context, userID int64, expansion AgentWriteGrayExpansionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_gray_expansion_snapshot",
		Status:    expansion.Status,
		Message:   expansion.Summary,
		Metadata: domain.AgentJSON{
			"candidates":     expansion.Candidates,
			"default_action": expansion.DefaultAction,
			"check_count":    len(expansion.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentExternalMonitorIntegrationSnapshot(ctx context.Context, userID int64, integration AgentExternalMonitorIntegrationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.external_monitor_integration_snapshot",
		Status:    integration.Status,
		Message:   integration.Summary,
		Metadata: domain.AgentJSON{
			"metrics":      integration.Metrics,
			"alert_events": integration.AlertEvents,
			"channels":     integration.Channels,
			"check_count":  len(integration.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentReleaseWindowExecutionSnapshot(ctx context.Context, userID int64, execution AgentReleaseWindowExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.release_window_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"window_state":        execution.WindowState,
			"execution_state":     execution.ExecutionState,
			"approval_status":     execution.ApprovalStatus,
			"failure_exit_status": execution.FailureExitStatus,
			"rollback_status":     execution.RollbackStatus,
			"notification_status": execution.NotificationStatus,
			"audit_event":         execution.AuditEvent,
			"check_count":         len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentExternalMonitorRuntimeSnapshot(ctx context.Context, userID int64, runtime AgentExternalMonitorRuntimeResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.external_monitor_runtime_snapshot",
		Status:    runtime.Status,
		Message:   runtime.Summary,
		Metadata: domain.AgentJSON{
			"health_status":               runtime.HealthStatus,
			"sla_status":                  runtime.SLAStatus,
			"error_status":                runtime.ErrorStatus,
			"cost_status":                 runtime.CostStatus,
			"queue_status":                runtime.QueueStatus,
			"worker_status":               runtime.WorkerStatus,
			"notification_failure_status": runtime.NotificationFailureStatus,
			"button_control_status":       runtime.ButtonControlStatus,
			"daily_send_status":           runtime.DailySendStatus,
			"metrics":                     runtime.Metrics,
			"alert_events":                runtime.AlertEvents,
			"channels":                    runtime.Channels,
			"check_count":                 len(runtime.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteGrayReviewSnapshot(ctx context.Context, userID int64, review AgentWriteGrayReviewResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_gray_review_snapshot",
		Status:    review.Status,
		Message:   review.Summary,
		Metadata: domain.AgentJSON{
			"candidates":      review.Candidates,
			"default_action":  review.DefaultAction,
			"decision":        review.Decision,
			"next_action":     review.NextAction,
			"denied_patterns": review.DeniedPatterns,
			"check_count":     len(review.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatAcceptanceReviewSnapshot(ctx context.Context, userID int64, review AgentWeChatAcceptanceReviewResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_acceptance_review_snapshot",
		Status:    review.Status,
		Message:   review.Summary,
		Metadata: domain.AgentJSON{
			"entry_status":            review.EntryStatus,
			"progress_status":         review.ProgressStatus,
			"button_control_status":   review.ButtonControlStatus,
			"web_sync_status":         review.WebSyncStatus,
			"final_report_status":     review.FinalReportStatus,
			"failure_fallback_status": review.FailureFallbackStatus,
			"next_action":             review.NextAction,
			"check_count":             len(review.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsDailyClosureSnapshot(ctx context.Context, userID int64, closure AgentOperationsDailyClosureResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_daily_closure_snapshot",
		Status:    closure.Status,
		Message:   closure.Summary,
		Metadata: domain.AgentJSON{
			"report_status":         closure.ReportStatus,
			"monitor_status":        closure.MonitorStatus,
			"button_control_status": closure.ButtonControlStatus,
			"release_window_status": closure.ReleaseWindowStatus,
			"audit_status":          closure.AuditStatus,
			"check_count":           len(closure.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentProductionReleaseSnapshot(ctx context.Context, userID int64, release AgentProductionReleaseResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.production_release_snapshot",
		Status:    release.Status,
		Message:   release.Summary,
		Metadata: domain.AgentJSON{
			"batch_id":             release.BatchID,
			"approval_source":      release.ApprovalSource,
			"precheck_status":      release.PrecheckStatus,
			"execution_status":     release.ExecutionStatus,
			"rollback_gate_status": release.RollbackGateStatus,
			"notification_status":  release.NotificationStatus,
			"audit_event":          release.AuditEvent,
			"check_count":          len(release.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentExternalMonitorConfigSnapshot(ctx context.Context, userID int64, config AgentExternalMonitorConfigResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.external_monitor_config_snapshot",
		Status:    config.Status,
		Message:   config.Summary,
		Metadata: domain.AgentJSON{
			"platform_status": config.PlatformStatus,
			"metric_names":    config.MetricNames,
			"event_names":     config.EventNames,
			"alert_channels":  config.AlertChannels,
			"daily_channels":  config.DailyChannels,
			"check_count":     len(config.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteRampSnapshot(ctx context.Context, userID int64, ramp AgentWriteRampResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_ramp_snapshot",
		Status:    ramp.Status,
		Message:   ramp.Summary,
		Metadata: domain.AgentJSON{
			"candidates":     ramp.Candidates,
			"ramp_percent":   ramp.RampPercent,
			"default_action": ramp.DefaultAction,
			"decision":       ramp.Decision,
			"approval_gate":  ramp.ApprovalGate,
			"budget_gate":    ramp.BudgetGate,
			"audit_gate":     ramp.AuditGate,
			"rollback_gate":  ramp.RollbackGate,
			"check_count":    len(ramp.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatSignoffSnapshot(ctx context.Context, userID int64, signoff AgentWeChatSignoffResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_signoff_snapshot",
		Status:    signoff.Status,
		Message:   signoff.Summary,
		Metadata: domain.AgentJSON{
			"signoff_state":              signoff.SignoffState,
			"entry_confirmed":            signoff.EntryConfirmed,
			"progress_confirmed":         signoff.ProgressConfirmed,
			"button_control_confirmed":   signoff.ButtonControlConfirmed,
			"web_sync_confirmed":         signoff.WebSyncConfirmed,
			"final_report_confirmed":     signoff.FinalReportConfirmed,
			"failure_fallback_confirmed": signoff.FailureFallbackConfirmed,
			"audit_event":                signoff.AuditEvent,
			"check_count":                len(signoff.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsHandoffSnapshot(ctx context.Context, userID int64, handoff AgentOperationsHandoffResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_handoff_snapshot",
		Status:    handoff.Status,
		Message:   handoff.Summary,
		Metadata: domain.AgentJSON{
			"release_status":        handoff.ReleaseStatus,
			"monitor_config_status": handoff.MonitorConfigStatus,
			"write_ramp_status":     handoff.WriteRampStatus,
			"wechat_signoff_status": handoff.WeChatSignoffStatus,
			"audit_status":          handoff.AuditStatus,
			"next_action":           handoff.NextAction,
			"check_count":           len(handoff.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentProductionExecutionSnapshot(ctx context.Context, userID int64, execution AgentProductionExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.production_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"batch_id":             execution.BatchID,
			"executor":             execution.Executor,
			"execution_status":     execution.ExecutionStatus,
			"rollback_gate_status": execution.RollbackGateStatus,
			"failure_exit_status":  execution.FailureExitStatus,
			"notification_status":  execution.NotificationStatus,
			"audit_event":          execution.AuditEvent,
			"check_count":          len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentMonitorIntegrationSnapshot(ctx context.Context, userID int64, integration AgentMonitorIntegrationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.monitor_integration_snapshot",
		Status:    integration.Status,
		Message:   integration.Summary,
		Metadata: domain.AgentJSON{
			"metric_write_status":  integration.MetricWriteStatus,
			"event_write_status":   integration.EventWriteStatus,
			"alert_channel_status": integration.AlertChannelStatus,
			"daily_channel_status": integration.DailyChannelStatus,
			"integration_result":   integration.IntegrationResult,
			"metric_names":         integration.MetricNames,
			"event_names":          integration.EventNames,
			"channels":             integration.Channels,
			"check_count":          len(integration.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteRampPolicySnapshot(ctx context.Context, userID int64, policy AgentWriteRampPolicyResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_ramp_policy_snapshot",
		Status:    policy.Status,
		Message:   policy.Summary,
		Metadata: domain.AgentJSON{
			"candidates":         policy.Candidates,
			"ramp_percent":       policy.RampPercent,
			"user_scope":         policy.UserScope,
			"approval_gate":      policy.ApprovalGate,
			"budget_gate":        policy.BudgetGate,
			"audit_gate":         policy.AuditGate,
			"rollback_threshold": policy.RollbackThreshold,
			"default_action":     policy.DefaultAction,
			"check_count":        len(policy.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatFinalReportSnapshot(ctx context.Context, userID int64, report AgentWeChatFinalReportResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_final_report_snapshot",
		Status:    report.Status,
		Message:   report.Summary,
		Metadata: domain.AgentJSON{
			"completion_notice_status": report.CompletionNoticeStatus,
			"final_report_entry":       report.FinalReportEntry,
			"failure_summary":          report.FailureSummary,
			"delivery_status":          report.DeliveryStatus,
			"template_status":          report.TemplateStatus,
			"text_status":              report.TextStatus,
			"progress_url":             report.ProgressURL,
			"next_action":              report.NextAction,
			"audit_event":              report.AuditEvent,
			"check_count":              len(report.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentLaunchRuntimeOverviewSnapshot(ctx context.Context, userID int64, overview AgentLaunchRuntimeOverviewResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.launch_runtime_overview_snapshot",
		Status:    overview.Status,
		Message:   overview.Summary,
		Metadata: domain.AgentJSON{
			"production_execution_status": overview.ProductionExecutionStatus,
			"monitor_integration_status":  overview.MonitorIntegrationStatus,
			"write_ramp_policy_status":    overview.WriteRampPolicyStatus,
			"wechat_final_report_status":  overview.WeChatFinalReportStatus,
			"audit_status":                overview.AuditStatus,
			"next_action":                 overview.NextAction,
			"check_count":                 len(overview.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRuntimeParametersSnapshot(ctx context.Context, userID int64, params AgentRuntimeParametersResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.runtime_parameters_snapshot",
		Status:    params.Status,
		Message:   params.Summary,
		Metadata: domain.AgentJSON{
			"ramp_percent":         params.RampPercent,
			"user_scope":           params.UserScope,
			"notification_channel": params.NotificationChannel,
			"monitor_channel":      params.MonitorChannel,
			"approval_gate":        params.ApprovalGate,
			"budget_gate":          params.BudgetGate,
			"rollback_threshold":   params.RollbackThreshold,
			"check_count":          len(params.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentMonitorReadbackSnapshot(ctx context.Context, userID int64, readback AgentMonitorReadbackResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.monitor_readback_snapshot",
		Status:    readback.Status,
		Message:   readback.Summary,
		Metadata: domain.AgentJSON{
			"metric_read_status": readback.MetricReadStatus,
			"event_read_status":  readback.EventReadStatus,
			"alert_status":       readback.AlertStatus,
			"daily_status":       readback.DailyStatus,
			"freshness_status":   readback.FreshnessStatus,
			"metric_names":       readback.MetricNames,
			"event_names":        readback.EventNames,
			"check_count":        len(readback.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteRampRecommendationSnapshot(ctx context.Context, userID int64, recommendation AgentWriteRampRecommendationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_ramp_recommendation_snapshot",
		Status:    recommendation.Status,
		Message:   recommendation.Summary,
		Metadata: domain.AgentJSON{
			"current_percent":     recommendation.CurrentPercent,
			"recommended_percent": recommendation.RecommendedPercent,
			"candidates":          recommendation.Candidates,
			"limit_conditions":    recommendation.LimitConditions,
			"rollback_conditions": recommendation.RollbackConditions,
			"default_action":      recommendation.DefaultAction,
			"check_count":         len(recommendation.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatUserFeedbackSnapshot(ctx context.Context, userID int64, feedback AgentWeChatUserFeedbackResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_user_feedback_snapshot",
		Status:    feedback.Status,
		Message:   feedback.Summary,
		Metadata: domain.AgentJSON{
			"completion_feedback":   feedback.CompletionFeedback,
			"failure_feedback":      feedback.FailureFeedback,
			"button_feedback":       feedback.ButtonFeedback,
			"web_tracking_feedback": feedback.WebTrackingFeedback,
			"next_action":           feedback.NextAction,
			"check_count":           len(feedback.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsRuntimeClosureSnapshot(ctx context.Context, userID int64, closure AgentOperationsRuntimeClosureResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_runtime_closure_snapshot",
		Status:    closure.Status,
		Message:   closure.Summary,
		Metadata: domain.AgentJSON{
			"runtime_parameter_status":         closure.RuntimeParameterStatus,
			"monitor_readback_status":          closure.MonitorReadbackStatus,
			"write_ramp_recommendation_status": closure.WriteRampRecommendationStatus,
			"wechat_user_feedback_status":      closure.WeChatUserFeedbackStatus,
			"audit_status":                     closure.AuditStatus,
			"next_action":                      closure.NextAction,
			"check_count":                      len(closure.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsPanelConfigSnapshot(ctx context.Context, userID int64, config AgentOpsPanelConfigResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_panel_config_snapshot",
		Status:    config.Status,
		Message:   config.Summary,
		Metadata: domain.AgentJSON{
			"parameter_group":          config.ParameterGroup,
			"display_items":            config.DisplayItems,
			"refresh_interval_seconds": config.RefreshIntervalSeconds,
			"alert_entry":              config.AlertEntry,
			"write_ramp_entry":         config.WriteRampEntry,
			"wechat_feedback_entry":    config.WeChatFeedbackEntry,
			"check_count":              len(config.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentMonitorAutoReportSnapshot(ctx context.Context, userID int64, report AgentMonitorAutoReportResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.monitor_auto_report_snapshot",
		Status:    report.Status,
		Message:   report.Summary,
		Metadata: domain.AgentJSON{
			"anomaly_status":        report.AnomalyStatus,
			"wechat_send_status":    report.WeChatSendStatus,
			"web_visibility_status": report.WebVisibilityStatus,
			"daily_link_status":     report.DailyLinkStatus,
			"audit_event":           report.AuditEvent,
			"check_count":           len(report.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteRampStageSnapshot(ctx context.Context, userID int64, stage AgentWriteRampStageResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_ramp_stage_snapshot",
		Status:    stage.Status,
		Message:   stage.Summary,
		Metadata: domain.AgentJSON{
			"current_stage":       stage.CurrentStage,
			"next_stage":          stage.NextStage,
			"entry_conditions":    stage.EntryConditions,
			"exit_conditions":     stage.ExitConditions,
			"rollback_conditions": stage.RollbackConditions,
			"default_action":      stage.DefaultAction,
			"check_count":         len(stage.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatFeedbackLoopSnapshot(ctx context.Context, userID int64, loop AgentWeChatFeedbackLoopResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_feedback_loop_snapshot",
		Status:    loop.Status,
		Message:   loop.Summary,
		Metadata: domain.AgentJSON{
			"completion_state": loop.CompletionState,
			"failure_state":    loop.FailureState,
			"button_state":     loop.ButtonState,
			"web_trace_state":  loop.WebTraceState,
			"processing_state": loop.ProcessingState,
			"next_action":      loop.NextAction,
			"check_count":      len(loop.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsClosedLoopSnapshot(ctx context.Context, userID int64, loop AgentOperationsClosedLoopResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_closed_loop_snapshot",
		Status:    loop.Status,
		Message:   loop.Summary,
		Metadata: domain.AgentJSON{
			"ops_panel_status":        loop.OpsPanelStatus,
			"monitor_report_status":   loop.MonitorReportStatus,
			"write_ramp_stage_status": loop.WriteRampStageStatus,
			"feedback_loop_status":    loop.FeedbackLoopStatus,
			"audit_status":            loop.AuditStatus,
			"next_action":             loop.NextAction,
			"check_count":             len(loop.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsDashboardInteractionSnapshot(ctx context.Context, userID int64, dashboard AgentOpsDashboardInteractionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_dashboard_interaction_snapshot",
		Status:    dashboard.Status,
		Message:   dashboard.Summary,
		Metadata: domain.AgentJSON{
			"actions":          dashboard.Actions,
			"refresh_strategy": dashboard.RefreshStrategy,
			"filters":          dashboard.Filters,
			"links":            dashboard.Links,
			"audit_event":      dashboard.AuditEvent,
			"check_count":      len(dashboard.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentAlertDedupeEscalationSnapshot(ctx context.Context, userID int64, escalation AgentAlertDedupeEscalationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_dedupe_escalation_snapshot",
		Status:    escalation.Status,
		Message:   escalation.Summary,
		Metadata: domain.AgentJSON{
			"dedupe_key":            escalation.DedupeKey,
			"dedupe_window_seconds": escalation.DedupeWindowSeconds,
			"escalation_condition":  escalation.EscalationCondition,
			"wechat_notify_status":  escalation.WeChatNotifyStatus,
			"web_visibility_status": escalation.WebVisibilityStatus,
			"check_count":           len(escalation.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteStageRecordSnapshot(ctx context.Context, userID int64, record AgentWriteStageRecordResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_stage_record_snapshot",
		Status:    record.Status,
		Message:   record.Summary,
		Metadata: domain.AgentJSON{
			"current_stage":       record.CurrentStage,
			"target_stage":        record.TargetStage,
			"promotion_reason":    record.PromotionReason,
			"blockers":            record.Blockers,
			"rollback_conditions": record.RollbackConditions,
			"default_action":      record.DefaultAction,
			"check_count":         len(record.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatFeedbackTicketSnapshot(ctx context.Context, userID int64, ticket AgentWeChatFeedbackTicketResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_feedback_ticket_snapshot",
		Status:    ticket.Status,
		Message:   ticket.Summary,
		Metadata: domain.AgentJSON{
			"ticket_type":      ticket.TicketType,
			"processing_state": ticket.ProcessingState,
			"owner_entry":      ticket.OwnerEntry,
			"user_next_action": ticket.UserNextAction,
			"audit_event":      ticket.AuditEvent,
			"check_count":      len(ticket.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsHandlingSnapshot(ctx context.Context, userID int64, handling AgentOperationsHandlingResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_handling_snapshot",
		Status:    handling.Status,
		Message:   handling.Summary,
		Metadata: domain.AgentJSON{
			"dashboard_status":        handling.DashboardStatus,
			"alert_escalation_status": handling.AlertEscalationStatus,
			"write_stage_status":      handling.WriteStageStatus,
			"feedback_ticket_status":  handling.FeedbackTicketStatus,
			"audit_status":            handling.AuditStatus,
			"next_action":             handling.NextAction,
			"check_count":             len(handling.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsActionDefinitionSnapshot(ctx context.Context, userID int64, definition AgentOpsActionDefinitionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_action_definition_snapshot",
		Status:    definition.Status,
		Message:   definition.Summary,
		Metadata: domain.AgentJSON{
			"actions":     definition.Actions,
			"check_count": len(definition.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentAlertEscalationPolicySnapshot(ctx context.Context, userID int64, policy AgentAlertEscalationPolicyResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_escalation_policy_snapshot",
		Status:    policy.Status,
		Message:   policy.Summary,
		Metadata: domain.AgentJSON{
			"escalation_level":       policy.EscalationLevel,
			"notification_channels":  policy.NotificationChannels,
			"repeat_suppression":     policy.RepeatSuppression,
			"recipients":             policy.Recipients,
			"recovery_notice_status": policy.RecoveryNoticeStatus,
			"audit_evidence":         policy.AuditEvidence,
			"check_count":            len(policy.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}
