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
