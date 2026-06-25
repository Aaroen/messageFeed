package service

type AgentTaskListResult struct {
	Tasks                        []AgentTaskSummaryResponse                `json:"tasks"`
	SLA                          AgentSLASummaryResponse                   `json:"sla"`
	Cost                         AgentCostSummaryResponse                  `json:"cost"`
	Alerts                       AgentAlertSummaryResponse                 `json:"alerts"`
	AlertPolicy                  AgentAlertPolicyResponse                  `json:"alert_policy"`
	CostTrend                    []AgentCostTrendBucketResponse            `json:"cost_trend"`
	TrendSnapshot                AgentTrendSnapshotResponse                `json:"trend_snapshot"`
	Deployment                   AgentDeploymentVerificationResponse       `json:"deployment"`
	Drill                        AgentProductionDrillResponse              `json:"drill"`
	WeChatComponents             AgentWeChatComponentSetResponse           `json:"wechat_components"`
	LoadTest                     AgentLoadTestSummaryResponse              `json:"load_test"`
	WeChatCallback               AgentWeChatCallbackReadinessResponse      `json:"wechat_callback"`
	WriteSandbox                 AgentWriteSandboxResponse                 `json:"write_sandbox"`
	E2E                          AgentE2EAcceptanceResponse                `json:"e2e"`
	RealIntegration              AgentRealIntegrationResponse              `json:"real_integration"`
	WeChatNative                 AgentWeChatNativeActionSetResponse        `json:"wechat_native"`
	WriteLeastPrivilege          AgentWriteLeastPrivilegeResponse          `json:"write_least_privilege"`
	OpsAcceptance                AgentOpsAcceptanceResponse                `json:"ops_acceptance"`
	WeChatNativePayload          AgentWeChatNativePayloadResponse          `json:"wechat_native_payload"`
	WriteGray                    AgentWriteGrayPolicyResponse              `json:"write_gray"`
	AlertChannel                 AgentAlertChannelResponse                 `json:"alert_channel"`
	LaunchDrill                  AgentLaunchDrillRecordResponse            `json:"launch_drill"`
	WeChatNativeIntegration      AgentWeChatNativeIntegrationResponse      `json:"wechat_native_integration"`
	WriteReplay                  AgentWriteReplayResponse                  `json:"write_replay"`
	LaunchApproval               AgentLaunchApprovalResponse               `json:"launch_approval"`
	DailyReport                  AgentDailyReportResponse                  `json:"daily_report"`
	Preprod                      AgentPreprodAcceptanceResponse            `json:"preprod"`
	ButtonLoop                   AgentButtonLoopResponse                   `json:"button_loop"`
	WriteExecute                 AgentWriteExecuteResponse                 `json:"write_execute"`
	DailyPersist                 AgentDailyPersistResponse                 `json:"daily_persist"`
	PostLaunchMonitor            AgentPostLaunchMonitorResponse            `json:"post_launch_monitor"`
	ReleaseApproval              AgentReleaseApprovalResponse              `json:"release_approval"`
	ButtonCallback               AgentButtonCallbackResponse               `json:"button_callback"`
	WriteAudit                   AgentWriteAuditReviewResponse             `json:"write_audit"`
	DailySend                    AgentDailySendResponse                    `json:"daily_send"`
	MonitorAlert                 AgentMonitorAlertDrillResponse            `json:"monitor_alert"`
	ButtonDirectControl          AgentButtonDirectControlResponse          `json:"button_direct_control"`
	WeChatE2E                    AgentWeChatE2EAcceptanceResponse          `json:"wechat_e2e"`
	ReleaseWindow                AgentReleaseWindowReadinessResponse       `json:"release_window"`
	WriteGrayExpansion           AgentWriteGrayExpansionResponse           `json:"write_gray_expansion"`
	ExternalMonitor              AgentExternalMonitorIntegrationResponse   `json:"external_monitor"`
	ReleaseWindowExecution       AgentReleaseWindowExecutionResponse       `json:"release_window_execution"`
	ExternalMonitorRuntime       AgentExternalMonitorRuntimeResponse       `json:"external_monitor_runtime"`
	WriteGrayReview              AgentWriteGrayReviewResponse              `json:"write_gray_review"`
	WeChatAcceptanceReview       AgentWeChatAcceptanceReviewResponse       `json:"wechat_acceptance_review"`
	OperationsDailyClosure       AgentOperationsDailyClosureResponse       `json:"operations_daily_closure"`
	ProductionRelease            AgentProductionReleaseResponse            `json:"production_release"`
	ExternalMonitorConfig        AgentExternalMonitorConfigResponse        `json:"external_monitor_config"`
	WriteRamp                    AgentWriteRampResponse                    `json:"write_ramp"`
	WeChatSignoff                AgentWeChatSignoffResponse                `json:"wechat_signoff"`
	OperationsHandoff            AgentOperationsHandoffResponse            `json:"operations_handoff"`
	ProductionExecution          AgentProductionExecutionResponse          `json:"production_execution"`
	MonitorIntegration           AgentMonitorIntegrationResponse           `json:"monitor_integration"`
	WriteRampPolicy              AgentWriteRampPolicyResponse              `json:"write_ramp_policy"`
	WeChatFinalReport            AgentWeChatFinalReportResponse            `json:"wechat_final_report"`
	LaunchRuntimeOverview        AgentLaunchRuntimeOverviewResponse        `json:"launch_runtime_overview"`
	RuntimeParameters            AgentRuntimeParametersResponse            `json:"runtime_parameters"`
	MonitorReadback              AgentMonitorReadbackResponse              `json:"monitor_readback"`
	WriteRampRecommendation      AgentWriteRampRecommendationResponse      `json:"write_ramp_recommendation"`
	WeChatUserFeedback           AgentWeChatUserFeedbackResponse           `json:"wechat_user_feedback"`
	OperationsRuntimeClosure     AgentOperationsRuntimeClosureResponse     `json:"operations_runtime_closure"`
	OpsPanelConfig               AgentOpsPanelConfigResponse               `json:"ops_panel_config"`
	MonitorAutoReport            AgentMonitorAutoReportResponse            `json:"monitor_auto_report"`
	WriteRampStage               AgentWriteRampStageResponse               `json:"write_ramp_stage"`
	WeChatFeedbackLoop           AgentWeChatFeedbackLoopResponse           `json:"wechat_feedback_loop"`
	OperationsClosedLoop         AgentOperationsClosedLoopResponse         `json:"operations_closed_loop"`
	OpsDashboardInteraction      AgentOpsDashboardInteractionResponse      `json:"ops_dashboard_interaction"`
	AlertDedupeEscalation        AgentAlertDedupeEscalationResponse        `json:"alert_dedupe_escalation"`
	WriteStageRecord             AgentWriteStageRecordResponse             `json:"write_stage_record"`
	WeChatFeedbackTicket         AgentWeChatFeedbackTicketResponse         `json:"wechat_feedback_ticket"`
	OperationsHandling           AgentOperationsHandlingResponse           `json:"operations_handling"`
	OpsActionDefinition          AgentOpsActionDefinitionResponse          `json:"ops_action_definition"`
	AlertEscalationPolicy        AgentAlertEscalationPolicyResponse        `json:"alert_escalation_policy"`
	WriteStageApproval           AgentWriteStageApprovalResponse           `json:"write_stage_approval"`
	FeedbackTicketLifecycle      AgentFeedbackTicketLifecycleResponse      `json:"feedback_ticket_lifecycle"`
	OperationsActionClosure      AgentOperationsActionClosureResponse      `json:"operations_action_closure"`
	OpsAPIExecution              AgentOpsAPIExecutionResponse              `json:"ops_api_execution"`
	AlertEscalationReceipt       AgentAlertEscalationReceiptResponse       `json:"alert_escalation_receipt"`
	WriteApprovalButton          AgentWriteApprovalButtonResponse          `json:"write_approval_button"`
	FeedbackTicketSLA            AgentFeedbackTicketSLAResponse            `json:"feedback_ticket_sla"`
	OperationsExecution          AgentOperationsExecutionResponse          `json:"operations_execution"`
	OpsExecutionRecord           AgentOpsExecutionRecordResponse           `json:"ops_execution_record"`
	WeChatApprovalCallback       AgentWeChatApprovalCallbackResponse       `json:"wechat_approval_callback"`
	FeedbackSLAReport            AgentFeedbackSLAReportResponse            `json:"feedback_sla_report"`
	AlertAutoRecovery            AgentAlertAutoRecoveryResponse            `json:"alert_auto_recovery"`
	OperationsEvidence           AgentOperationsEvidenceResponse           `json:"operations_evidence"`
	UnifiedProgressComponent     AgentUnifiedProgressComponentResponse     `json:"unified_progress_component"`
	EvidenceDetailPage           AgentEvidenceDetailPageResponse           `json:"evidence_detail_page"`
	CallbackReplayTool           AgentCallbackReplayToolResponse           `json:"callback_replay_tool"`
	RecoveryPolicyConfig         AgentRecoveryPolicyConfigResponse         `json:"recovery_policy_config"`
	DualEndProgressEvidence      AgentDualEndProgressEvidenceResponse      `json:"dual_end_progress_evidence"`
	WeChatProgressCard           AgentWeChatProgressCardResponse           `json:"wechat_progress_card"`
	WebEvidenceInteraction       AgentWebEvidenceInteractionResponse       `json:"web_evidence_interaction"`
	CallbackReplayPermission     AgentCallbackReplayPermissionResponse     `json:"callback_replay_permission"`
	RecoveryPolicyAudit          AgentRecoveryPolicyAuditResponse          `json:"recovery_policy_audit"`
	DualEndInteraction           AgentDualEndInteractionResponse           `json:"dual_end_interaction"`
	WeChatTemplateRender         AgentWeChatTemplateRenderResponse         `json:"wechat_template_render"`
	WebEvidenceRoute             AgentWebEvidenceRouteResponse             `json:"web_evidence_route"`
	CallbackReplayApproval       AgentCallbackReplayApprovalResponse       `json:"callback_replay_approval"`
	RecoveryPolicyPersist        AgentRecoveryPolicyPersistResponse        `json:"recovery_policy_persist"`
	DualEndInteractionLaunch     AgentDualEndInteractionLaunchResponse     `json:"dual_end_interaction_launch"`
	WeChatTemplateSend           AgentWeChatTemplateSendResponse           `json:"wechat_template_send"`
	WebEvidenceDetailView        AgentWebEvidenceDetailViewResponse        `json:"web_evidence_detail_view"`
	CallbackReplayExecution      AgentCallbackReplayExecutionResponse      `json:"callback_replay_execution"`
	RecoveryPolicyVersion        AgentRecoveryPolicyVersionResponse        `json:"recovery_policy_version"`
	DualEndRealInteraction       AgentDualEndRealInteractionResponse       `json:"dual_end_real_interaction"`
	WeChatTemplateIntegration    AgentWeChatTemplateIntegrationResponse    `json:"wechat_template_integration"`
	WebEvidenceInteractionDetail AgentWebEvidenceInteractionDetailResponse `json:"web_evidence_interaction_detail"`
	CallbackReplaySafetyAudit    AgentCallbackReplaySafetyAuditResponse    `json:"callback_replay_safety_audit"`
	RecoveryPolicyGrayRelease    AgentRecoveryPolicyGrayReleaseResponse    `json:"recovery_policy_gray_release"`
	DualEndRunLoop               AgentDualEndRunLoopResponse               `json:"dual_end_run_loop"`
	WeChatTemplatePilot          AgentWeChatTemplatePilotResponse          `json:"wechat_template_pilot"`
	WebEvidenceUserAction        AgentWebEvidenceUserActionResponse        `json:"web_evidence_user_action"`
	CallbackReplayResultTrace    AgentCallbackReplayResultTraceResponse    `json:"callback_replay_result_trace"`
	RecoveryPolicyAutomation     AgentRecoveryPolicyAutomationResponse     `json:"recovery_policy_automation"`
	DualEndTaskClosure           AgentDualEndTaskClosureResponse           `json:"dual_end_task_closure"`
	WeChatTemplatePilotMetric    AgentWeChatTemplatePilotMetricResponse    `json:"wechat_template_pilot_metric"`
	WebEvidenceOperation         AgentWebEvidenceOperationResponse         `json:"web_evidence_operation"`
	CallbackReplayResultQuery    AgentCallbackReplayResultQueryResponse    `json:"callback_replay_result_query"`
	RecoveryAutomationExecution  AgentRecoveryAutomationExecutionResponse  `json:"recovery_automation_execution"`
	RealInteractionAutomation    AgentRealInteractionAutomationResponse    `json:"real_interaction_automation"`
	WeChatWebProgressLink        AgentWeChatWebProgressLinkResponse        `json:"wechat_web_progress_link"`
	Report                       AgentTaskReportResponse                   `json:"report"`
}

type AgentCostSummaryResponse struct {
	ToolCalls         int `json:"tool_calls"`
	ExternalCalls     int `json:"external_calls"`
	EstimatedTokens   int `json:"estimated_tokens"`
	RetryCount        int `json:"retry_count"`
	NotificationCount int `json:"notification_count"`
	ScheduledTasks    int `json:"scheduled_tasks"`
}

type AgentSLASummaryResponse struct {
	PlanCount               int     `json:"plan_count"`
	PlanSucceeded           int     `json:"plan_succeeded"`
	PlanFailed              int     `json:"plan_failed"`
	ScheduledTaskCount      int     `json:"scheduled_task_count"`
	ScheduledTaskSucceeded  int     `json:"scheduled_task_succeeded"`
	ScheduledTaskFailed     int     `json:"scheduled_task_failed"`
	AveragePlanSeconds      float64 `json:"average_plan_seconds"`
	RecoveryCount           int     `json:"recovery_count"`
	HandoffCount            int     `json:"handoff_count"`
	NotificationSentCount   int     `json:"notification_sent_count"`
	NotificationFailedCount int     `json:"notification_failed_count"`
}

type AgentAlertSummaryResponse struct {
	Total    int      `json:"total"`
	Critical int      `json:"critical"`
	Warning  int      `json:"warning"`
	Reasons  []string `json:"reasons"`
}

type AgentAlertPolicyResponse struct {
	Status         string                             `json:"status"`
	Summary        string                             `json:"summary"`
	EnabledReasons []string                           `json:"enabled_reasons"`
	MutedReasons   []string                           `json:"muted_reasons"`
	Decisions      []AgentAlertPolicyDecisionResponse `json:"decisions"`
}

type AgentAlertPolicyDecisionResponse struct {
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
	Enabled  bool   `json:"enabled"`
	Action   string `json:"action"`
}

type AgentCostTrendBucketResponse struct {
	Date              string `json:"date"`
	ToolCalls         int    `json:"tool_calls"`
	ExternalCalls     int    `json:"external_calls"`
	EstimatedTokens   int    `json:"estimated_tokens"`
	RetryCount        int    `json:"retry_count"`
	NotificationCount int    `json:"notification_count"`
}

type AgentTrendSnapshotResponse struct {
	Source        string                     `json:"source"`
	RetentionDays int                        `json:"retention_days"`
	Summary       string                     `json:"summary"`
	Buckets       []AgentTrendBucketResponse `json:"buckets"`
}

type AgentTrendBucketResponse struct {
	Date                string `json:"date"`
	ToolCalls           int    `json:"tool_calls"`
	ExternalCalls       int    `json:"external_calls"`
	EstimatedTokens     int    `json:"estimated_tokens"`
	RetryCount          int    `json:"retry_count"`
	NotificationCount   int    `json:"notification_count"`
	PlanFailed          int    `json:"plan_failed"`
	ScheduledTaskFailed int    `json:"scheduled_task_failed"`
	NotificationFailed  int    `json:"notification_failed"`
	RecoveryCount       int    `json:"recovery_count"`
	HandoffCount        int    `json:"handoff_count"`
}
