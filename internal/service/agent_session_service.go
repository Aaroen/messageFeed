package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"messagefeed/internal/domain"
	"sort"
	"strings"
	"time"
)

type AgentSessionRepository interface {
	ListAgentSessions(ctx context.Context, userID int64) ([]domain.AgentSession, error)
	GetAgentSession(ctx context.Context, userID int64, sessionID int64) (domain.AgentSession, error)
	CreateAgentSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error)
	ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error)
	GetExternalAccount(ctx context.Context, userID int64, accountID int64) (domain.ExternalAccount, error)
	SetExternalAccountActiveSession(ctx context.Context, userID int64, externalAccountID int64, sessionID int64) (domain.ExternalAccount, error)
	GetAgentSessionStats(ctx context.Context, userID int64, sessionID int64) (domain.AgentSessionStats, error)
	ClearAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error)
	RebuildAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error)
	DeleteAgentSession(ctx context.Context, userID int64, sessionID int64) error
	ListRecentTranscriptEntries(ctx context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error)
	ListAgentRunsByTurn(ctx context.Context, userID int64, turnID int64) ([]domain.AgentRun, error)
	GetAgentRunDetail(ctx context.Context, userID int64, runID int64) (domain.AgentRun, error)
	ListAgentPlans(ctx context.Context, userID int64, sessionID int64, turnID int64, limit int) ([]domain.AgentPlan, error)
	GetAgentPlan(ctx context.Context, userID int64, planID int64) (domain.AgentPlan, error)
	GetAgentScheduledTask(ctx context.Context, userID int64, taskID int64) (domain.AgentScheduledTask, error)
	ListAgentScheduledTasks(ctx context.Context, options domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error)
	UpdateAgentScheduledTask(ctx context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error)
	CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error)
	ListAgentScheduledTasksByRefs(ctx context.Context, userID int64, planID int64, turnID int64, runID int64, limit int) ([]domain.AgentScheduledTask, error)
	ListAuditLogsByRefs(ctx context.Context, options domain.AgentAuditLogListOptions) ([]domain.AgentAuditLog, error)
}

type AgentSessionService struct {
	repository AgentSessionRepository
	now        func() time.Time
}

type AgentSessionServiceOption func(*AgentSessionService)

func WithAgentSessionNow(now func() time.Time) AgentSessionServiceOption {
	return func(service *AgentSessionService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAgentSessionService(repository AgentSessionRepository, options ...AgentSessionServiceOption) *AgentSessionService {
	service := &AgentSessionService{repository: repository, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

type AgentSessionListResult struct {
	Accounts []AgentExternalAccountResponse `json:"accounts"`
	Sessions []AgentSessionResponse         `json:"sessions"`
}

type AgentExternalAccountResponse struct {
	ID                   int64  `json:"id"`
	Provider             string `json:"provider"`
	CorpID               string `json:"corp_id"`
	AgentID              string `json:"agent_id"`
	ExternalUserID       string `json:"external_user_id"`
	DisplayName          string `json:"display_name"`
	BindingStatus        string `json:"binding_status"`
	ActiveAgentSessionID int64  `json:"active_agent_session_id"`
	UpdatedAt            string `json:"updated_at"`
}

type AgentSessionResponse struct {
	ID                       int64             `json:"id"`
	ExternalAccountID        int64             `json:"external_account_id"`
	Provider                 string            `json:"provider"`
	ChannelSessionKey        string            `json:"channel_session_key"`
	Status                   string            `json:"status"`
	Title                    string            `json:"title"`
	ActiveForAccount         bool              `json:"active_for_account"`
	ContextInitializedAt     string            `json:"context_initialized_at,omitempty"`
	ContextRebuildStartedAt  string            `json:"context_rebuild_started_at,omitempty"`
	ContextRebuildFinishedAt string            `json:"context_rebuild_finished_at,omitempty"`
	ContextVersion           int64             `json:"context_version"`
	TranscriptCountIndexed   int64             `json:"transcript_count_indexed"`
	Stats                    AgentSessionStats `json:"stats"`
	StartedAt                string            `json:"started_at"`
	LastActiveAt             string            `json:"last_active_at"`
	CreatedAt                string            `json:"created_at"`
	UpdatedAt                string            `json:"updated_at"`
}

type AgentSessionStats struct {
	TranscriptCount   int64  `json:"transcript_count"`
	ArchiveIndexCount int64  `json:"archive_index_count"`
	RecallCount       int64  `json:"recall_count"`
	FirstTranscriptAt string `json:"first_transcript_at,omitempty"`
	LastTranscriptAt  string `json:"last_transcript_at,omitempty"`
}

type AgentTranscriptListResult struct {
	Entries []AgentTranscriptEntryResponse `json:"entries"`
}

type AgentTranscriptEntryResponse struct {
	ID        int64  `json:"id"`
	TurnID    int64  `json:"turn_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type AgentRunListResult struct {
	Runs []AgentRunResponse `json:"runs"`
}

type AgentRunDetailResult struct {
	Run AgentRunResponse `json:"run"`
}

type AgentPlanListResult struct {
	Plans []AgentPlanResponse `json:"plans"`
}

type AgentPlanDetailResult struct {
	Plan AgentPlanResponse `json:"plan"`
}

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

type AgentDeploymentVerificationResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDeploymentCheckResponse struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type AgentProductionDrillResponse struct {
	Status      string                         `json:"status"`
	Summary     string                         `json:"summary"`
	Source      string                         `json:"source"`
	GeneratedAt string                         `json:"generated_at"`
	Checks      []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatComponentSetResponse struct {
	Mode    string                      `json:"mode"`
	Summary string                      `json:"summary"`
	Actions []AgentWeChatActionResponse `json:"actions"`
}

type AgentWeChatActionResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	Fallback string `json:"fallback"`
}

type AgentLoadTestSummaryResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Metrics AgentLoadTestMetricsResponse   `json:"metrics"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentLoadTestMetricsResponse struct {
	Users            int `json:"users"`
	WebTasks         int `json:"web_tasks"`
	WeChatTasks      int `json:"wechat_tasks"`
	ScheduledTasks   int `json:"scheduled_tasks"`
	RecoveryEvents   int `json:"recovery_events"`
	AdmissionLimited int `json:"admission_limited"`
	QuotaLimited     int `json:"quota_limited"`
	ProgressEvents   int `json:"progress_events"`
}

type AgentWeChatCallbackReadinessResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteSandboxResponse struct {
	Status        string                         `json:"status"`
	Summary       string                         `json:"summary"`
	DefaultAction string                         `json:"default_action"`
	Checks        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentE2EAcceptanceResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRealIntegrationResponse struct {
	Status     string                         `json:"status"`
	Summary    string                         `json:"summary"`
	Risks      []string                       `json:"risks"`
	Blockers   []string                       `json:"blockers"`
	NextAction string                         `json:"next_action"`
	Checks     []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatNativeActionSetResponse struct {
	Mode    string                            `json:"mode"`
	Summary string                            `json:"summary"`
	Actions []AgentWeChatNativeActionResponse `json:"actions"`
}

type AgentWeChatNativeActionResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Style    string `json:"style"`
	URL      string `json:"url"`
	Fallback string `json:"fallback"`
}

type AgentWriteLeastPrivilegeResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	DefaultAction     string                         `json:"default_action"`
	AllowedCandidates []string                       `json:"allowed_candidates"`
	DeniedPatterns    []string                       `json:"denied_patterns"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsAcceptanceResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatNativePayloadResponse struct {
	Status       string                            `json:"status"`
	Summary      string                            `json:"summary"`
	MessageType  string                            `json:"message_type"`
	FallbackText string                            `json:"fallback_text"`
	Buttons      []AgentWeChatNativeButtonResponse `json:"buttons"`
	Payload      domain.AgentJSON                  `json:"payload"`
}

type AgentWeChatNativeButtonResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Style    string `json:"style"`
	URL      string `json:"url"`
	Fallback string `json:"fallback"`
}

type AgentWriteGrayPolicyResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	Candidates       []string                       `json:"candidates"`
	AllowedUserScope string                         `json:"allowed_user_scope"`
	RequiresApproval bool                           `json:"requires_approval"`
	RequiresBudget   bool                           `json:"requires_budget"`
	RequiresAudit    bool                           `json:"requires_audit"`
	RollbackTriggers []string                       `json:"rollback_triggers"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentAlertChannelResponse struct {
	Status   string                            `json:"status"`
	Summary  string                            `json:"summary"`
	Channels []AgentAlertChannelTargetResponse `json:"channels"`
}

type AgentAlertChannelTargetResponse struct {
	Key      string   `json:"key"`
	Status   string   `json:"status"`
	Reasons  []string `json:"reasons"`
	Fallback string   `json:"fallback"`
}

type AgentLaunchDrillRecordResponse struct {
	BatchID     string                         `json:"batch_id"`
	Status      string                         `json:"status"`
	Summary     string                         `json:"summary"`
	TriggeredBy string                         `json:"triggered_by"`
	Result      string                         `json:"result"`
	Risks       []string                       `json:"risks"`
	Blockers    []string                       `json:"blockers"`
	NextAction  string                         `json:"next_action"`
	Checks      []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatNativeIntegrationResponse struct {
	Status     string                         `json:"status"`
	Summary    string                         `json:"summary"`
	Risks      []string                       `json:"risks"`
	Blockers   []string                       `json:"blockers"`
	NextAction string                         `json:"next_action"`
	Checks     []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteReplayResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	Candidates       []string                       `json:"candidates"`
	ApprovalStatus   string                         `json:"approval_status"`
	BudgetStatus     string                         `json:"budget_status"`
	PermissionStatus string                         `json:"permission_status"`
	ExecutionStatus  string                         `json:"execution_status"`
	AuditStatus      string                         `json:"audit_status"`
	RollbackTriggers []string                       `json:"rollback_triggers"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentLaunchApprovalResponse struct {
	Status       string                         `json:"status"`
	Summary      string                         `json:"summary"`
	RequestID    string                         `json:"request_id"`
	ReviewState  string                         `json:"review_state"`
	Approved     int                            `json:"approved"`
	Rejected     int                            `json:"rejected"`
	Expired      int                            `json:"expired"`
	HandoffPath  string                         `json:"handoff_path"`
	RollbackPath string                         `json:"rollback_path"`
	Checks       []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDailyReportResponse struct {
	Date               string                         `json:"date"`
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	TaskCount          int                            `json:"task_count"`
	SuccessRate        float64                        `json:"success_rate"`
	FailureCount       int                            `json:"failure_count"`
	AlertCount         int                            `json:"alert_count"`
	EstimatedTokens    int                            `json:"estimated_tokens"`
	TrendBuckets       int                            `json:"trend_buckets"`
	HandoffCount       int                            `json:"handoff_count"`
	RecoveryCount      int                            `json:"recovery_count"`
	NotificationCount  int                            `json:"notification_count"`
	NotificationFailed int                            `json:"notification_failed"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentPreprodAcceptanceResponse struct {
	Status     string                         `json:"status"`
	Summary    string                         `json:"summary"`
	Risks      []string                       `json:"risks"`
	Blockers   []string                       `json:"blockers"`
	NextAction string                         `json:"next_action"`
	Checks     []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentButtonLoopResponse struct {
	Status       string                            `json:"status"`
	Summary      string                            `json:"summary"`
	FallbackText string                            `json:"fallback_text"`
	Actions      []AgentWeChatNativeButtonResponse `json:"actions"`
	Checks       []AgentDeploymentCheckResponse    `json:"checks"`
}

type AgentWriteExecuteResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	Candidates       []string                       `json:"candidates"`
	DefaultAction    string                         `json:"default_action"`
	ApprovalStatus   string                         `json:"approval_status"`
	BudgetStatus     string                         `json:"budget_status"`
	PermissionStatus string                         `json:"permission_status"`
	ExecutionStatus  string                         `json:"execution_status"`
	AuditStatus      string                         `json:"audit_status"`
	RollbackTriggers []string                       `json:"rollback_triggers"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDailyPersistResponse struct {
	Status    string                         `json:"status"`
	Summary   string                         `json:"summary"`
	RecordKey string                         `json:"record_key"`
	Source    string                         `json:"source"`
	Retained  bool                           `json:"retained"`
	Checks    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentPostLaunchMonitorResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentReleaseApprovalResponse struct {
	Status        string                         `json:"status"`
	Summary       string                         `json:"summary"`
	RequestID     string                         `json:"request_id"`
	ReviewState   string                         `json:"review_state"`
	Executable    bool                           `json:"executable"`
	Approved      int                            `json:"approved"`
	Rejected      int                            `json:"rejected"`
	Expired       int                            `json:"expired"`
	DecisionPath  string                         `json:"decision_path"`
	RejectionPath string                         `json:"rejection_path"`
	RollbackPath  string                         `json:"rollback_path"`
	AuditEvent    string                         `json:"audit_event"`
	Checks        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentButtonCallbackActionResponse struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Handler  string `json:"handler"`
	URL      string `json:"url"`
	Fallback string `json:"fallback"`
	Status   string `json:"status"`
}

type AgentButtonCallbackResponse struct {
	Status       string                              `json:"status"`
	Summary      string                              `json:"summary"`
	FallbackText string                              `json:"fallback_text"`
	Actions      []AgentButtonCallbackActionResponse `json:"actions"`
	Checks       []AgentDeploymentCheckResponse      `json:"checks"`
}

type AgentWriteAuditReviewResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	Candidates         []string                       `json:"candidates"`
	ApprovalEvidence   string                         `json:"approval_evidence"`
	BudgetEvidence     string                         `json:"budget_evidence"`
	PermissionEvidence string                         `json:"permission_evidence"`
	ExecutionEvidence  string                         `json:"execution_evidence"`
	FailureEvidence    string                         `json:"failure_evidence"`
	RollbackEvidence   string                         `json:"rollback_evidence"`
	HandoffEvidence    string                         `json:"handoff_evidence"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDailySendResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	RecordKey          string                         `json:"record_key"`
	ScheduleStatus     string                         `json:"schedule_status"`
	DeliveryStatus     string                         `json:"delivery_status"`
	RetryStatus        string                         `json:"retry_status"`
	WeChatReportStatus string                         `json:"wechat_report_status"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentMonitorAlertDrillResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	TriggerStatus      string                         `json:"trigger_status"`
	NotificationStatus string                         `json:"notification_status"`
	HandoffStatus      string                         `json:"handoff_status"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentButtonDirectControlResponse struct {
	Status   string                              `json:"status"`
	Summary  string                              `json:"summary"`
	Executed int                                 `json:"executed"`
	Skipped  int                                 `json:"skipped"`
	Actions  []AgentButtonCallbackActionResponse `json:"actions"`
	Checks   []AgentDeploymentCheckResponse      `json:"checks"`
}

type AgentWeChatE2EAcceptanceResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentReleaseWindowReadinessResponse struct {
	Status      string                         `json:"status"`
	Summary     string                         `json:"summary"`
	WindowState string                         `json:"window_state"`
	Checks      []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteGrayExpansionResponse struct {
	Status        string                         `json:"status"`
	Summary       string                         `json:"summary"`
	Candidates    []string                       `json:"candidates"`
	DefaultAction string                         `json:"default_action"`
	Checks        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentExternalMonitorIntegrationResponse struct {
	Status      string                         `json:"status"`
	Summary     string                         `json:"summary"`
	Metrics     []string                       `json:"metrics"`
	AlertEvents []string                       `json:"alert_events"`
	Channels    []string                       `json:"channels"`
	Checks      []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentReleaseWindowExecutionResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	WindowState        string                         `json:"window_state"`
	ExecutionState     string                         `json:"execution_state"`
	ApprovalStatus     string                         `json:"approval_status"`
	FailureExitStatus  string                         `json:"failure_exit_status"`
	RollbackStatus     string                         `json:"rollback_status"`
	NotificationStatus string                         `json:"notification_status"`
	AuditEvent         string                         `json:"audit_event"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentExternalMonitorRuntimeResponse struct {
	Status                    string                         `json:"status"`
	Summary                   string                         `json:"summary"`
	HealthStatus              string                         `json:"health_status"`
	SLAStatus                 string                         `json:"sla_status"`
	ErrorStatus               string                         `json:"error_status"`
	CostStatus                string                         `json:"cost_status"`
	QueueStatus               string                         `json:"queue_status"`
	WorkerStatus              string                         `json:"worker_status"`
	NotificationFailureStatus string                         `json:"notification_failure_status"`
	ButtonControlStatus       string                         `json:"button_control_status"`
	DailySendStatus           string                         `json:"daily_send_status"`
	Metrics                   []string                       `json:"metrics"`
	AlertEvents               []string                       `json:"alert_events"`
	Channels                  []string                       `json:"channels"`
	Checks                    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteGrayReviewResponse struct {
	Status         string                         `json:"status"`
	Summary        string                         `json:"summary"`
	Candidates     []string                       `json:"candidates"`
	DefaultAction  string                         `json:"default_action"`
	Decision       string                         `json:"decision"`
	NextAction     string                         `json:"next_action"`
	DeniedPatterns []string                       `json:"denied_patterns"`
	Checks         []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatAcceptanceReviewResponse struct {
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	EntryStatus           string                         `json:"entry_status"`
	ProgressStatus        string                         `json:"progress_status"`
	ButtonControlStatus   string                         `json:"button_control_status"`
	WebSyncStatus         string                         `json:"web_sync_status"`
	FinalReportStatus     string                         `json:"final_report_status"`
	FailureFallbackStatus string                         `json:"failure_fallback_status"`
	NextAction            string                         `json:"next_action"`
	Checks                []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsDailyClosureResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	ReportStatus        string                         `json:"report_status"`
	MonitorStatus       string                         `json:"monitor_status"`
	ButtonControlStatus string                         `json:"button_control_status"`
	ReleaseWindowStatus string                         `json:"release_window_status"`
	AuditStatus         string                         `json:"audit_status"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentProductionReleaseResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	BatchID            string                         `json:"batch_id"`
	ApprovalSource     string                         `json:"approval_source"`
	PrecheckStatus     string                         `json:"precheck_status"`
	ExecutionStatus    string                         `json:"execution_status"`
	RollbackGateStatus string                         `json:"rollback_gate_status"`
	NotificationStatus string                         `json:"notification_status"`
	AuditEvent         string                         `json:"audit_event"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentExternalMonitorConfigResponse struct {
	Status         string                         `json:"status"`
	Summary        string                         `json:"summary"`
	PlatformStatus string                         `json:"platform_status"`
	MetricNames    []string                       `json:"metric_names"`
	EventNames     []string                       `json:"event_names"`
	AlertChannels  []string                       `json:"alert_channels"`
	DailyChannels  []string                       `json:"daily_channels"`
	Checks         []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteRampResponse struct {
	Status        string                         `json:"status"`
	Summary       string                         `json:"summary"`
	Candidates    []string                       `json:"candidates"`
	RampPercent   int                            `json:"ramp_percent"`
	DefaultAction string                         `json:"default_action"`
	Decision      string                         `json:"decision"`
	ApprovalGate  string                         `json:"approval_gate"`
	BudgetGate    string                         `json:"budget_gate"`
	AuditGate     string                         `json:"audit_gate"`
	RollbackGate  string                         `json:"rollback_gate"`
	Checks        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatSignoffResponse struct {
	Status                   string                         `json:"status"`
	Summary                  string                         `json:"summary"`
	SignoffState             string                         `json:"signoff_state"`
	EntryConfirmed           string                         `json:"entry_confirmed"`
	ProgressConfirmed        string                         `json:"progress_confirmed"`
	ButtonControlConfirmed   string                         `json:"button_control_confirmed"`
	WebSyncConfirmed         string                         `json:"web_sync_confirmed"`
	FinalReportConfirmed     string                         `json:"final_report_confirmed"`
	FailureFallbackConfirmed string                         `json:"failure_fallback_confirmed"`
	AuditEvent               string                         `json:"audit_event"`
	Checks                   []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsHandoffResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	ReleaseStatus       string                         `json:"release_status"`
	MonitorConfigStatus string                         `json:"monitor_config_status"`
	WriteRampStatus     string                         `json:"write_ramp_status"`
	WeChatSignoffStatus string                         `json:"wechat_signoff_status"`
	AuditStatus         string                         `json:"audit_status"`
	NextAction          string                         `json:"next_action"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentProductionExecutionResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	BatchID            string                         `json:"batch_id"`
	Executor           string                         `json:"executor"`
	ExecutionStatus    string                         `json:"execution_status"`
	RollbackGateStatus string                         `json:"rollback_gate_status"`
	FailureExitStatus  string                         `json:"failure_exit_status"`
	NotificationStatus string                         `json:"notification_status"`
	AuditEvent         string                         `json:"audit_event"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentMonitorIntegrationResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	MetricWriteStatus  string                         `json:"metric_write_status"`
	EventWriteStatus   string                         `json:"event_write_status"`
	AlertChannelStatus string                         `json:"alert_channel_status"`
	DailyChannelStatus string                         `json:"daily_channel_status"`
	IntegrationResult  string                         `json:"integration_result"`
	MetricNames        []string                       `json:"metric_names"`
	EventNames         []string                       `json:"event_names"`
	Channels           []string                       `json:"channels"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteRampPolicyResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	Candidates        []string                       `json:"candidates"`
	RampPercent       int                            `json:"ramp_percent"`
	UserScope         string                         `json:"user_scope"`
	ApprovalGate      string                         `json:"approval_gate"`
	BudgetGate        string                         `json:"budget_gate"`
	AuditGate         string                         `json:"audit_gate"`
	RollbackThreshold string                         `json:"rollback_threshold"`
	DefaultAction     string                         `json:"default_action"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatFinalReportResponse struct {
	Status                 string                         `json:"status"`
	Summary                string                         `json:"summary"`
	CompletionNoticeStatus string                         `json:"completion_notice_status"`
	FinalReportEntry       string                         `json:"final_report_entry"`
	FailureSummary         string                         `json:"failure_summary"`
	NextAction             string                         `json:"next_action"`
	AuditEvent             string                         `json:"audit_event"`
	Checks                 []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentLaunchRuntimeOverviewResponse struct {
	Status                    string                         `json:"status"`
	Summary                   string                         `json:"summary"`
	ProductionExecutionStatus string                         `json:"production_execution_status"`
	MonitorIntegrationStatus  string                         `json:"monitor_integration_status"`
	WriteRampPolicyStatus     string                         `json:"write_ramp_policy_status"`
	WeChatFinalReportStatus   string                         `json:"wechat_final_report_status"`
	AuditStatus               string                         `json:"audit_status"`
	NextAction                string                         `json:"next_action"`
	Checks                    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRuntimeParametersResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	RampPercent         int                            `json:"ramp_percent"`
	UserScope           string                         `json:"user_scope"`
	NotificationChannel string                         `json:"notification_channel"`
	MonitorChannel      string                         `json:"monitor_channel"`
	ApprovalGate        string                         `json:"approval_gate"`
	BudgetGate          string                         `json:"budget_gate"`
	RollbackThreshold   string                         `json:"rollback_threshold"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentMonitorReadbackResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	MetricReadStatus string                         `json:"metric_read_status"`
	EventReadStatus  string                         `json:"event_read_status"`
	AlertStatus      string                         `json:"alert_status"`
	DailyStatus      string                         `json:"daily_status"`
	FreshnessStatus  string                         `json:"freshness_status"`
	MetricNames      []string                       `json:"metric_names"`
	EventNames       []string                       `json:"event_names"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteRampRecommendationResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	CurrentPercent     int                            `json:"current_percent"`
	RecommendedPercent int                            `json:"recommended_percent"`
	Candidates         []string                       `json:"candidates"`
	LimitConditions    []string                       `json:"limit_conditions"`
	RollbackConditions []string                       `json:"rollback_conditions"`
	DefaultAction      string                         `json:"default_action"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatUserFeedbackResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	CompletionFeedback  string                         `json:"completion_feedback"`
	FailureFeedback     string                         `json:"failure_feedback"`
	ButtonFeedback      string                         `json:"button_feedback"`
	WebTrackingFeedback string                         `json:"web_tracking_feedback"`
	NextAction          string                         `json:"next_action"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsRuntimeClosureResponse struct {
	Status                        string                         `json:"status"`
	Summary                       string                         `json:"summary"`
	RuntimeParameterStatus        string                         `json:"runtime_parameter_status"`
	MonitorReadbackStatus         string                         `json:"monitor_readback_status"`
	WriteRampRecommendationStatus string                         `json:"write_ramp_recommendation_status"`
	WeChatUserFeedbackStatus      string                         `json:"wechat_user_feedback_status"`
	AuditStatus                   string                         `json:"audit_status"`
	NextAction                    string                         `json:"next_action"`
	Checks                        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsPanelConfigResponse struct {
	Status                 string                         `json:"status"`
	Summary                string                         `json:"summary"`
	ParameterGroup         string                         `json:"parameter_group"`
	DisplayItems           []string                       `json:"display_items"`
	RefreshIntervalSeconds int                            `json:"refresh_interval_seconds"`
	AlertEntry             string                         `json:"alert_entry"`
	WriteRampEntry         string                         `json:"write_ramp_entry"`
	WeChatFeedbackEntry    string                         `json:"wechat_feedback_entry"`
	Checks                 []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentMonitorAutoReportResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	AnomalyStatus       string                         `json:"anomaly_status"`
	WeChatSendStatus    string                         `json:"wechat_send_status"`
	WebVisibilityStatus string                         `json:"web_visibility_status"`
	DailyLinkStatus     string                         `json:"daily_link_status"`
	AuditEvent          string                         `json:"audit_event"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteRampStageResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	CurrentStage       string                         `json:"current_stage"`
	NextStage          string                         `json:"next_stage"`
	EntryConditions    []string                       `json:"entry_conditions"`
	ExitConditions     []string                       `json:"exit_conditions"`
	RollbackConditions []string                       `json:"rollback_conditions"`
	DefaultAction      string                         `json:"default_action"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatFeedbackLoopResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	CompletionState string                         `json:"completion_state"`
	FailureState    string                         `json:"failure_state"`
	ButtonState     string                         `json:"button_state"`
	WebTraceState   string                         `json:"web_trace_state"`
	ProcessingState string                         `json:"processing_state"`
	NextAction      string                         `json:"next_action"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsClosedLoopResponse struct {
	Status               string                         `json:"status"`
	Summary              string                         `json:"summary"`
	OpsPanelStatus       string                         `json:"ops_panel_status"`
	MonitorReportStatus  string                         `json:"monitor_report_status"`
	WriteRampStageStatus string                         `json:"write_ramp_stage_status"`
	FeedbackLoopStatus   string                         `json:"feedback_loop_status"`
	AuditStatus          string                         `json:"audit_status"`
	NextAction           string                         `json:"next_action"`
	Checks               []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsDashboardInteractionResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	Actions         []string                       `json:"actions"`
	RefreshStrategy string                         `json:"refresh_strategy"`
	Filters         []string                       `json:"filters"`
	Links           []string                       `json:"links"`
	AuditEvent      string                         `json:"audit_event"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentAlertDedupeEscalationResponse struct {
	Status              string                         `json:"status"`
	Summary             string                         `json:"summary"`
	DedupeKey           string                         `json:"dedupe_key"`
	DedupeWindowSeconds int                            `json:"dedupe_window_seconds"`
	EscalationCondition string                         `json:"escalation_condition"`
	WeChatNotifyStatus  string                         `json:"wechat_notify_status"`
	WebVisibilityStatus string                         `json:"web_visibility_status"`
	Checks              []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteStageRecordResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	CurrentStage       string                         `json:"current_stage"`
	TargetStage        string                         `json:"target_stage"`
	PromotionReason    string                         `json:"promotion_reason"`
	Blockers           []string                       `json:"blockers"`
	RollbackConditions []string                       `json:"rollback_conditions"`
	DefaultAction      string                         `json:"default_action"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatFeedbackTicketResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	TicketType      string                         `json:"ticket_type"`
	ProcessingState string                         `json:"processing_state"`
	OwnerEntry      string                         `json:"owner_entry"`
	UserNextAction  string                         `json:"user_next_action"`
	AuditEvent      string                         `json:"audit_event"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsHandlingResponse struct {
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	DashboardStatus       string                         `json:"dashboard_status"`
	AlertEscalationStatus string                         `json:"alert_escalation_status"`
	WriteStageStatus      string                         `json:"write_stage_status"`
	FeedbackTicketStatus  string                         `json:"feedback_ticket_status"`
	AuditStatus           string                         `json:"audit_status"`
	NextAction            string                         `json:"next_action"`
	Checks                []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsActionItemResponse struct {
	Key                  string `json:"key"`
	Label                string `json:"label"`
	HandlerEntry         string `json:"handler_entry"`
	PermissionConstraint string `json:"permission_constraint"`
	IdempotencyKey       string `json:"idempotency_key"`
	AuditEvent           string `json:"audit_event"`
}

type AgentOpsActionDefinitionResponse struct {
	Status  string                         `json:"status"`
	Summary string                         `json:"summary"`
	Actions []AgentOpsActionItemResponse   `json:"actions"`
	Checks  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentAlertEscalationPolicyResponse struct {
	Status               string                         `json:"status"`
	Summary              string                         `json:"summary"`
	EscalationLevel      string                         `json:"escalation_level"`
	NotificationChannels []string                       `json:"notification_channels"`
	RepeatSuppression    string                         `json:"repeat_suppression"`
	Recipients           []string                       `json:"recipients"`
	RecoveryNoticeStatus string                         `json:"recovery_notice_status"`
	AuditEvidence        string                         `json:"audit_evidence"`
	Checks               []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteStageApprovalResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	ApprovalStatus    string                         `json:"approval_status"`
	ApprovalSource    string                         `json:"approval_source"`
	TargetStage       string                         `json:"target_stage"`
	AuthorizedScope   string                         `json:"authorized_scope"`
	RollbackThreshold string                         `json:"rollback_threshold"`
	DefaultAction     string                         `json:"default_action"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentFeedbackTicketLifecycleResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	CreatedState     string                         `json:"created_state"`
	AssignedState    string                         `json:"assigned_state"`
	ProcessingState  string                         `json:"processing_state"`
	WaitingUserState string                         `json:"waiting_user_state"`
	ClosedState      string                         `json:"closed_state"`
	HandoffState     string                         `json:"handoff_state"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsActionClosureResponse struct {
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	OpsActionStatus       string                         `json:"ops_action_status"`
	AlertEscalationStatus string                         `json:"alert_escalation_status"`
	WriteApprovalStatus   string                         `json:"write_approval_status"`
	TicketLifecycleStatus string                         `json:"ticket_lifecycle_status"`
	AuditStatus           string                         `json:"audit_status"`
	NextAction            string                         `json:"next_action"`
	Checks                []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsAPIExecutionItemResponse struct {
	ActionKey         string `json:"action_key"`
	ExecutionEntry    string `json:"execution_entry"`
	ExecutionStatus   string `json:"execution_status"`
	PermissionCheck   string `json:"permission_check"`
	IdempotencyResult string `json:"idempotency_result"`
	AuditEvent        string `json:"audit_event"`
}

type AgentOpsAPIExecutionResponse struct {
	Status     string                             `json:"status"`
	Summary    string                             `json:"summary"`
	Executions []AgentOpsAPIExecutionItemResponse `json:"executions"`
	Checks     []AgentDeploymentCheckResponse     `json:"checks"`
}

type AgentAlertEscalationReceiptResponse struct {
	Status               string                         `json:"status"`
	Summary              string                         `json:"summary"`
	NotificationChannels []string                       `json:"notification_channels"`
	Recipients           []string                       `json:"recipients"`
	DeliveryStatus       string                         `json:"delivery_status"`
	SuppressionResult    string                         `json:"suppression_result"`
	RecoveryNoticeStatus string                         `json:"recovery_notice_status"`
	HandoffEntry         string                         `json:"handoff_entry"`
	Checks               []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWriteApprovalButtonItemResponse struct {
	ButtonKey         string `json:"button_key"`
	Channel           string `json:"channel"`
	ApprovalStatus    string `json:"approval_status"`
	PermissionScope   string `json:"permission_scope"`
	RollbackThreshold string `json:"rollback_threshold"`
	RejectionPath     string `json:"rejection_path"`
	AuditEvidence     string `json:"audit_evidence"`
}

type AgentWriteApprovalButtonResponse struct {
	Status  string                                 `json:"status"`
	Summary string                                 `json:"summary"`
	Buttons []AgentWriteApprovalButtonItemResponse `json:"buttons"`
	Checks  []AgentDeploymentCheckResponse         `json:"checks"`
}

type AgentFeedbackTicketSLAResponse struct {
	Status               string                         `json:"status"`
	Summary              string                         `json:"summary"`
	FirstResponseSeconds int                            `json:"first_response_seconds"`
	ResolveSeconds       int                            `json:"resolve_seconds"`
	TimeoutEscalation    string                         `json:"timeout_escalation"`
	WaitingUserStatus    string                         `json:"waiting_user_status"`
	CloseCondition       string                         `json:"close_condition"`
	HandoffPath          string                         `json:"handoff_path"`
	Checks               []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsExecutionResponse struct {
	Status                    string                         `json:"status"`
	Summary                   string                         `json:"summary"`
	OpsAPIExecutionStatus     string                         `json:"ops_api_execution_status"`
	AlertReceiptStatus        string                         `json:"alert_receipt_status"`
	WriteApprovalButtonStatus string                         `json:"write_approval_button_status"`
	FeedbackSLAStatus         string                         `json:"feedback_sla_status"`
	AuditStatus               string                         `json:"audit_status"`
	NextAction                string                         `json:"next_action"`
	Checks                    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOpsExecutionRecordItemResponse struct {
	RecordKey         string `json:"record_key"`
	ActionKey         string `json:"action_key"`
	ExecutionStatus   string `json:"execution_status"`
	IdempotencyStatus string `json:"idempotency_status"`
	AuditEvent        string `json:"audit_event"`
	ReplayEntry       string `json:"replay_entry"`
}

type AgentOpsExecutionRecordResponse struct {
	Status  string                                `json:"status"`
	Summary string                                `json:"summary"`
	Records []AgentOpsExecutionRecordItemResponse `json:"records"`
	Checks  []AgentDeploymentCheckResponse        `json:"checks"`
}

type AgentWeChatApprovalCallbackResponse struct {
	Status       string                         `json:"status"`
	Summary      string                         `json:"summary"`
	CallbackKey  string                         `json:"callback_key"`
	Source       string                         `json:"source"`
	Decision     string                         `json:"decision"`
	Signature    string                         `json:"signature"`
	StorageState string                         `json:"storage_state"`
	FallbackPath string                         `json:"fallback_path"`
	Checks       []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentFeedbackSLAReportResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	FirstResponseRate float64                        `json:"first_response_rate"`
	ResolveRate       float64                        `json:"resolve_rate"`
	TimeoutCount      int                            `json:"timeout_count"`
	WaitingUserCount  int                            `json:"waiting_user_count"`
	HandoffCount      int                            `json:"handoff_count"`
	ReportAuditEvent  string                         `json:"report_audit_event"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentAlertAutoRecoveryResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	RecoveryTrigger    string                         `json:"recovery_trigger"`
	RecoveryNotice     string                         `json:"recovery_notice"`
	SuppressionRelease string                         `json:"suppression_release"`
	ReopenCondition    string                         `json:"reopen_condition"`
	HandoffState       string                         `json:"handoff_state"`
	AuditEvidence      string                         `json:"audit_evidence"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentOperationsEvidenceResponse struct {
	Status                 string                         `json:"status"`
	Summary                string                         `json:"summary"`
	ExecutionRecordStatus  string                         `json:"execution_record_status"`
	ApprovalCallbackStatus string                         `json:"approval_callback_status"`
	SLAReportStatus        string                         `json:"sla_report_status"`
	AutoRecoveryStatus     string                         `json:"auto_recovery_status"`
	AuditStatus            string                         `json:"audit_status"`
	NextAction             string                         `json:"next_action"`
	Checks                 []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentUnifiedProgressComponentResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	ComponentKey    string                         `json:"component_key"`
	WebStatus       string                         `json:"web_status"`
	WeChatStatus    string                         `json:"wechat_status"`
	EventCursor     string                         `json:"event_cursor"`
	RefreshStrategy string                         `json:"refresh_strategy"`
	AuditEvidence   string                         `json:"audit_evidence"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentEvidenceDetailPageResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	DetailEntry     string                         `json:"detail_entry"`
	RecordCount     int                            `json:"record_count"`
	AuditEvent      string                         `json:"audit_event"`
	ReplayEntry     string                         `json:"replay_entry"`
	Visibility      string                         `json:"visibility"`
	RetentionPolicy string                         `json:"retention_policy"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayToolResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	CallbackKey      string                         `json:"callback_key"`
	ReplayEntry      string                         `json:"replay_entry"`
	SignatureReview  string                         `json:"signature_review"`
	IdempotencyGuard string                         `json:"idempotency_guard"`
	FailureFallback  string                         `json:"failure_fallback"`
	AuditEvidence    string                         `json:"audit_evidence"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyConfigResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	PolicyKey         string                         `json:"policy_key"`
	RecoveryTrigger   string                         `json:"recovery_trigger"`
	SuppressionWindow string                         `json:"suppression_window"`
	ReopenCondition   string                         `json:"reopen_condition"`
	HandoffState      string                         `json:"handoff_state"`
	DefaultPolicy     string                         `json:"default_policy"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndProgressEvidenceResponse struct {
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	UnifiedProgressStatus string                         `json:"unified_progress_status"`
	EvidenceDetailStatus  string                         `json:"evidence_detail_status"`
	CallbackReplayStatus  string                         `json:"callback_replay_status"`
	RecoveryPolicyStatus  string                         `json:"recovery_policy_status"`
	AuditStatus           string                         `json:"audit_status"`
	NextAction            string                         `json:"next_action"`
	Checks                []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatProgressCardResponse struct {
	Status          string                              `json:"status"`
	Summary         string                              `json:"summary"`
	CardKey         string                              `json:"card_key"`
	PhaseStatus     string                              `json:"phase_status"`
	ProgressPercent int                                 `json:"progress_percent"`
	DetailEntry     string                              `json:"detail_entry"`
	Actions         []AgentButtonCallbackActionResponse `json:"actions"`
	FallbackText    string                              `json:"fallback_text"`
	Checks          []AgentDeploymentCheckResponse      `json:"checks"`
}

type AgentWebEvidenceInteractionResponse struct {
	Status        string                         `json:"status"`
	Summary       string                         `json:"summary"`
	Filters       []string                       `json:"filters"`
	Expandable    string                         `json:"expandable"`
	ReplayEntry   string                         `json:"replay_entry"`
	AuditDisplay  string                         `json:"audit_display"`
	RetentionHint string                         `json:"retention_hint"`
	Visibility    string                         `json:"visibility"`
	Checks        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayPermissionResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	PermissionKey    string                         `json:"permission_key"`
	AllowedRoles     []string                       `json:"allowed_roles"`
	IdempotencyGuard string                         `json:"idempotency_guard"`
	SignatureReview  string                         `json:"signature_review"`
	FailureFallback  string                         `json:"failure_fallback"`
	AuditEvent       string                         `json:"audit_event"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyAuditResponse struct {
	Status         string                         `json:"status"`
	Summary        string                         `json:"summary"`
	ChangeKey      string                         `json:"change_key"`
	OldPolicy      string                         `json:"old_policy"`
	NewPolicy      string                         `json:"new_policy"`
	ApprovalStatus string                         `json:"approval_status"`
	RollbackPath   string                         `json:"rollback_path"`
	AuditEvidence  string                         `json:"audit_evidence"`
	Checks         []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndInteractionResponse struct {
	Status                    string                         `json:"status"`
	Summary                   string                         `json:"summary"`
	WeChatProgressCardStatus  string                         `json:"wechat_progress_card_status"`
	WebEvidenceStatus         string                         `json:"web_evidence_status"`
	CallbackPermissionStatus  string                         `json:"callback_permission_status"`
	RecoveryPolicyAuditStatus string                         `json:"recovery_policy_audit_status"`
	AuditStatus               string                         `json:"audit_status"`
	NextAction                string                         `json:"next_action"`
	Checks                    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatTemplateRenderResponse struct {
	Status       string                         `json:"status"`
	Summary      string                         `json:"summary"`
	TemplateKey  string                         `json:"template_key"`
	RenderStatus string                         `json:"render_status"`
	PhaseFields  []string                       `json:"phase_fields"`
	ButtonFields []string                       `json:"button_fields"`
	FallbackText string                         `json:"fallback_text"`
	SendEntry    string                         `json:"send_entry"`
	Checks       []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWebEvidenceRouteResponse struct {
	Status                string                         `json:"status"`
	Summary               string                         `json:"summary"`
	RouteName             string                         `json:"route_name"`
	PathParams            []string                       `json:"path_params"`
	FilterParams          []string                       `json:"filter_params"`
	PermissionRequirement string                         `json:"permission_requirement"`
	ReplayEntry           string                         `json:"replay_entry"`
	AuditDisplay          string                         `json:"audit_display"`
	Checks                []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayApprovalResponse struct {
	Status         string                         `json:"status"`
	Summary        string                         `json:"summary"`
	ApprovalKey    string                         `json:"approval_key"`
	RequestEntry   string                         `json:"request_entry"`
	ApprovalRoles  []string                       `json:"approval_roles"`
	ApprovalStatus string                         `json:"approval_status"`
	ExecutionGate  string                         `json:"execution_gate"`
	AuditEvent     string                         `json:"audit_event"`
	Checks         []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyPersistResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	ConfigKey         string                         `json:"config_key"`
	CurrentVersion    string                         `json:"current_version"`
	PendingVersion    string                         `json:"pending_version"`
	PersistenceStatus string                         `json:"persistence_status"`
	RollbackVersion   string                         `json:"rollback_version"`
	AuditEvidence     string                         `json:"audit_evidence"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndInteractionLaunchResponse struct {
	Status                          string                         `json:"status"`
	Summary                         string                         `json:"summary"`
	WeChatTemplateRenderStatus      string                         `json:"wechat_template_render_status"`
	WebEvidenceRouteStatus          string                         `json:"web_evidence_route_status"`
	CallbackReplayApprovalStatus    string                         `json:"callback_replay_approval_status"`
	RecoveryPolicyPersistenceStatus string                         `json:"recovery_policy_persistence_status"`
	AuditStatus                     string                         `json:"audit_status"`
	NextAction                      string                         `json:"next_action"`
	Checks                          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatTemplateSendResponse struct {
	Status       string                         `json:"status"`
	Summary      string                         `json:"summary"`
	MessageType  string                         `json:"message_type"`
	Title        string                         `json:"title"`
	PhaseFields  []string                       `json:"phase_fields"`
	ButtonFields []string                       `json:"button_fields"`
	FallbackText string                         `json:"fallback_text"`
	SendEntry    string                         `json:"send_entry"`
	SendResult   string                         `json:"send_result"`
	AuditEvent   string                         `json:"audit_event"`
	Checks       []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWebEvidenceDetailViewResponse struct {
	Status         string                         `json:"status"`
	Summary        string                         `json:"summary"`
	RouteName      string                         `json:"route_name"`
	RoutePath      string                         `json:"route_path"`
	PlanParam      string                         `json:"plan_param"`
	RecordParam    string                         `json:"record_param"`
	RecordSource   string                         `json:"record_source"`
	FilterParams   []string                       `json:"filter_params"`
	AuditEvents    []string                       `json:"audit_events"`
	ReplayEntry    string                         `json:"replay_entry"`
	PermissionHint string                         `json:"permission_hint"`
	Checks         []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayExecutionResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	RequestEntry    string                         `json:"request_entry"`
	ExecuteEntry    string                         `json:"execute_entry"`
	ApprovalStatus  string                         `json:"approval_status"`
	ExecutionGate   string                         `json:"execution_gate"`
	IdempotencyKey  string                         `json:"idempotency_key"`
	AuditEvent      string                         `json:"audit_event"`
	FailureFallback string                         `json:"failure_fallback"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyVersionResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	PolicyKey       string                         `json:"policy_key"`
	CurrentVersion  string                         `json:"current_version"`
	PendingVersion  string                         `json:"pending_version"`
	RollbackVersion string                         `json:"rollback_version"`
	ReleaseStatus   string                         `json:"release_status"`
	ConfigSource    string                         `json:"config_source"`
	AuditEvent      string                         `json:"audit_event"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndRealInteractionResponse struct {
	Status                        string                         `json:"status"`
	Summary                       string                         `json:"summary"`
	WeChatTemplateSendStatus      string                         `json:"wechat_template_send_status"`
	WebEvidenceDetailStatus       string                         `json:"web_evidence_detail_status"`
	CallbackReplayExecutionStatus string                         `json:"callback_replay_execution_status"`
	RecoveryPolicyVersionStatus   string                         `json:"recovery_policy_version_status"`
	AuditStatus                   string                         `json:"audit_status"`
	NextAction                    string                         `json:"next_action"`
	Checks                        []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatTemplateIntegrationResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	SendPath          string                         `json:"send_path"`
	TemplateStatus    string                         `json:"template_status"`
	FallbackStatus    string                         `json:"fallback_status"`
	DegradeStrategy   string                         `json:"degrade_strategy"`
	MessageIDReadback string                         `json:"message_id_readback"`
	AuditEvidence     string                         `json:"audit_evidence"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWebEvidenceInteractionDetailResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	FilterMode         string                         `json:"filter_mode"`
	ExpandMode         string                         `json:"expand_mode"`
	AuditTimeline      string                         `json:"audit_timeline"`
	ReplayRequestEntry string                         `json:"replay_request_entry"`
	PermissionHint     string                         `json:"permission_hint"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplaySafetyAuditResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	IdempotencyCheck string                         `json:"idempotency_check"`
	ApprovalCheck    string                         `json:"approval_check"`
	SignatureCheck   string                         `json:"signature_check"`
	ExecutionResult  string                         `json:"execution_result"`
	FailureAudit     string                         `json:"failure_audit"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyGrayReleaseResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	GrayStage         string                         `json:"gray_stage"`
	ReleasePercent    int                            `json:"release_percent"`
	RollbackCondition string                         `json:"rollback_condition"`
	ApprovalStatus    string                         `json:"approval_status"`
	AuditEvidence     string                         `json:"audit_evidence"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndRunLoopResponse struct {
	Status                          string                         `json:"status"`
	Summary                         string                         `json:"summary"`
	WeChatTemplateIntegrationStatus string                         `json:"wechat_template_integration_status"`
	WebEvidenceInteractionStatus    string                         `json:"web_evidence_interaction_status"`
	CallbackReplaySafetyStatus      string                         `json:"callback_replay_safety_status"`
	RecoveryPolicyGrayStatus        string                         `json:"recovery_policy_gray_status"`
	AuditStatus                     string                         `json:"audit_status"`
	NextAction                      string                         `json:"next_action"`
	Checks                          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatTemplatePilotResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	PilotBatch      string                         `json:"pilot_batch"`
	TargetScope     string                         `json:"target_scope"`
	TemplateStatus  string                         `json:"template_status"`
	FallbackHit     string                         `json:"fallback_hit"`
	MessageIDStatus string                         `json:"message_id_status"`
	AuditEvidence   string                         `json:"audit_evidence"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWebEvidenceUserActionResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	FilterAction     string                         `json:"filter_action"`
	ExpandAction     string                         `json:"expand_action"`
	TimelineAction   string                         `json:"timeline_action"`
	ReplayRequest    string                         `json:"replay_request"`
	PermissionResult string                         `json:"permission_result"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayResultTraceResponse struct {
	Status           string                         `json:"status"`
	Summary          string                         `json:"summary"`
	ExecutionResult  string                         `json:"execution_result"`
	IdempotencyHit   string                         `json:"idempotency_hit"`
	ApprovalDecision string                         `json:"approval_decision"`
	SignatureResult  string                         `json:"signature_result"`
	FailureReason    string                         `json:"failure_reason"`
	AuditRecord      string                         `json:"audit_record"`
	Checks           []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryPolicyAutomationResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	AutoAdvance       string                         `json:"auto_advance"`
	PauseCondition    string                         `json:"pause_condition"`
	RollbackCondition string                         `json:"rollback_condition"`
	CurrentPercent    int                            `json:"current_percent"`
	NextPercent       int                            `json:"next_percent"`
	AuditEvidence     string                         `json:"audit_evidence"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentDualEndTaskClosureResponse struct {
	Status                    string                         `json:"status"`
	Summary                   string                         `json:"summary"`
	WeChatPilotStatus         string                         `json:"wechat_pilot_status"`
	WebEvidenceActionStatus   string                         `json:"web_evidence_action_status"`
	CallbackReplayTraceStatus string                         `json:"callback_replay_trace_status"`
	RecoveryAutomationStatus  string                         `json:"recovery_automation_status"`
	AuditStatus               string                         `json:"audit_status"`
	NextAction                string                         `json:"next_action"`
	Checks                    []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWeChatTemplatePilotMetricResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	BatchID         string                         `json:"batch_id"`
	TargetUserScope string                         `json:"target_user_scope"`
	SendStatus      string                         `json:"send_status"`
	FallbackCount   int                            `json:"fallback_count"`
	MessageIDStatus string                         `json:"message_id_status"`
	AuditRef        string                         `json:"audit_ref"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentWebEvidenceOperationResponse struct {
	Status             string                         `json:"status"`
	Summary            string                         `json:"summary"`
	FilterEntry        string                         `json:"filter_entry"`
	ExpandEntry        string                         `json:"expand_entry"`
	TimelineEntry      string                         `json:"timeline_entry"`
	ReplayRequestEntry string                         `json:"replay_request_entry"`
	PermissionGate     string                         `json:"permission_gate"`
	AuditEvent         string                         `json:"audit_event"`
	OperationCount     int                            `json:"operation_count"`
	Checks             []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayResultQueryResponse struct {
	Status            string                         `json:"status"`
	Summary           string                         `json:"summary"`
	QueryEntry        string                         `json:"query_entry"`
	ExecutionResult   string                         `json:"execution_result"`
	IdempotencyResult string                         `json:"idempotency_result"`
	ApprovalDecision  string                         `json:"approval_decision"`
	SignatureResult   string                         `json:"signature_result"`
	FailureReason     string                         `json:"failure_reason"`
	AuditRef          string                         `json:"audit_ref"`
	Checks            []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRecoveryAutomationExecutionResponse struct {
	Status          string                         `json:"status"`
	Summary         string                         `json:"summary"`
	ExecutionMode   string                         `json:"execution_mode"`
	CurrentPercent  int                            `json:"current_percent"`
	NextPercent     int                            `json:"next_percent"`
	AdvanceDecision string                         `json:"advance_decision"`
	PauseGate       string                         `json:"pause_gate"`
	RollbackGate    string                         `json:"rollback_gate"`
	ApprovalGate    string                         `json:"approval_gate"`
	AuditRef        string                         `json:"audit_ref"`
	Checks          []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentRealInteractionAutomationResponse struct {
	Status                  string                         `json:"status"`
	Summary                 string                         `json:"summary"`
	PilotMetricStatus       string                         `json:"pilot_metric_status"`
	EvidenceOperationStatus string                         `json:"evidence_operation_status"`
	ReplayQueryStatus       string                         `json:"replay_query_status"`
	RecoveryExecutionStatus string                         `json:"recovery_execution_status"`
	AuditStatus             string                         `json:"audit_status"`
	NextAction              string                         `json:"next_action"`
	Checks                  []AgentDeploymentCheckResponse `json:"checks"`
}

type AgentCallbackReplayInput struct {
	PlanID      int64  `json:"plan_id"`
	CallbackKey string `json:"callback_key"`
	ReplayEntry string `json:"replay_entry"`
	Reason      string `json:"reason"`
	Approved    bool   `json:"approved"`
}

type AgentCallbackReplayAPIResult struct {
	ReplayExecution AgentCallbackReplayExecutionResponse `json:"replay_execution"`
	AuditEvent      string                               `json:"audit_event"`
}

type AgentTaskReportResponse struct {
	ByStatus     map[string]int `json:"by_status"`
	ByEntry      map[string]int `json:"by_entry"`
	ByCapability map[string]int `json:"by_capability"`
	ByHandoff    map[string]int `json:"by_handoff"`
}

type CancelAgentScheduledTaskResult struct {
	Task AgentScheduledTaskResponse `json:"task"`
}

type AgentTaskSummaryResponse struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	SessionID        int64  `json:"session_id"`
	TurnID           int64  `json:"turn_id"`
	PlanID           int64  `json:"plan_id"`
	ScheduledTaskID  int64  `json:"scheduled_task_id"`
	Status           string `json:"status"`
	Goal             string `json:"goal"`
	Summary          string `json:"summary"`
	PermissionStatus string `json:"permission_status"`
	BudgetStatus     string `json:"budget_status"`
	QualityStatus    string `json:"quality_status"`
	HandoffStatus    string `json:"handoff_status"`
	Observability    string `json:"observability"`
	LatestProgress   string `json:"latest_progress"`
	NextAction       string `json:"next_action"`
	ProgressURL      string `json:"progress_url"`
	UpdatedAt        string `json:"updated_at"`
}

type AgentProgressQuery struct {
	PlanID          int64
	TurnID          int64
	RunID           int64
	ScheduledTaskID int64
}

type AgentProgressResult struct {
	Progress AgentProgressSnapshot `json:"progress"`
}

type AgentProgressSnapshot struct {
	SubjectType    string                       `json:"subject_type"`
	SubjectID      int64                        `json:"subject_id"`
	Status         string                       `json:"status"`
	Summary        string                       `json:"summary"`
	NextAction     string                       `json:"next_action"`
	Version        int64                        `json:"version"`
	EventCursor    string                       `json:"event_cursor"`
	UpdatedAt      string                       `json:"updated_at"`
	RefreshedAt    string                       `json:"refreshed_at"`
	Plan           *AgentPlanResponse           `json:"plan,omitempty"`
	Runs           []AgentRunResponse           `json:"runs"`
	ScheduledTasks []AgentScheduledTaskResponse `json:"scheduled_tasks"`
	Phases         []AgentProgressPhaseResponse `json:"phases"`
	RecentEvents   []AgentProgressEventResponse `json:"recent_events"`
}

type AgentProgressPhaseResponse struct {
	Key       string `json:"key"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type AgentProgressEventResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Source    string `json:"source,omitempty"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Ref       string `json:"ref,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type AgentScheduledTaskResponse struct {
	ID                  int64            `json:"id"`
	UserID              int64            `json:"user_id"`
	SessionID           int64            `json:"session_id"`
	TurnID              int64            `json:"turn_id"`
	PlanID              int64            `json:"plan_id"`
	SourceRunID         int64            `json:"source_run_id"`
	Status              string           `json:"status"`
	TaskType            string           `json:"task_type"`
	Goal                string           `json:"goal"`
	TargetChannel       string           `json:"target_channel"`
	TargetRef           string           `json:"target_ref"`
	ScheduledAt         string           `json:"scheduled_at"`
	DeliverAt           string           `json:"deliver_at,omitempty"`
	FreshnessPolicy     string           `json:"freshness_policy"`
	AllowedCapabilities []string         `json:"allowed_capabilities"`
	ModelPolicy         domain.AgentJSON `json:"model_policy"`
	FailurePolicy       domain.AgentJSON `json:"failure_policy"`
	Payload             domain.AgentJSON `json:"payload"`
	AttemptCount        int              `json:"attempt_count"`
	MaxAttempts         int              `json:"max_attempts"`
	LastError           string           `json:"last_error"`
	NextRunAt           string           `json:"next_run_at,omitempty"`
	CompletedAt         string           `json:"completed_at,omitempty"`
	CreatedAt           string           `json:"created_at"`
	UpdatedAt           string           `json:"updated_at"`
}

type AgentRunResponse struct {
	ID              int64                          `json:"id"`
	ParentRunID     int64                          `json:"parent_run_id"`
	SessionID       int64                          `json:"session_id"`
	TurnID          int64                          `json:"turn_id"`
	Role            string                         `json:"role"`
	Status          string                         `json:"status"`
	TaskPacket      domain.AgentJSON               `json:"task_packet"`
	CapabilityScope []string                       `json:"capability_scope"`
	ModelKey        string                         `json:"model_key"`
	ContextBudget   domain.AgentJSON               `json:"context_budget"`
	ContextTraceRef string                         `json:"context_trace_ref"`
	ResultRef       string                         `json:"result_ref"`
	ErrorMessage    string                         `json:"error_message"`
	TraceID         string                         `json:"trace_id"`
	StartedAt       string                         `json:"started_at"`
	CompletedAt     string                         `json:"completed_at,omitempty"`
	CreatedAt       string                         `json:"created_at"`
	UpdatedAt       string                         `json:"updated_at"`
	ContextTraces   []AgentRunContextTraceResponse `json:"context_traces,omitempty"`
	Observations    []AgentObservationResponse     `json:"observations,omitempty"`
	Artifacts       []AgentArtifactResponse        `json:"artifacts,omitempty"`
	ChildRuns       []AgentRunResponse             `json:"child_runs,omitempty"`
}

type AgentRunContextTraceResponse struct {
	ID              int64            `json:"id"`
	RunID           int64            `json:"run_id"`
	TraceKind       string           `json:"trace_kind"`
	PromptVersion   string           `json:"prompt_version"`
	ModelKey        string           `json:"model_key"`
	Content         domain.AgentJSON `json:"content"`
	ContentHash     string           `json:"content_hash"`
	RedactionStatus string           `json:"redaction_status"`
	TokenEstimate   int              `json:"token_estimate"`
	CreatedAt       string           `json:"created_at"`
}

type AgentObservationResponse struct {
	ID            int64    `json:"id"`
	RunID         int64    `json:"run_id"`
	CapabilityKey string   `json:"capability_key"`
	InputSummary  string   `json:"input_summary"`
	OutputSummary string   `json:"output_summary"`
	Status        string   `json:"status"`
	Error         string   `json:"error"`
	ArtifactRefs  []string `json:"artifact_refs"`
	CreatedAt     string   `json:"created_at"`
}

type AgentArtifactResponse struct {
	ID           int64    `json:"id"`
	RunID        int64    `json:"run_id"`
	ArtifactType string   `json:"artifact_type"`
	ContentRef   string   `json:"content_ref"`
	Summary      string   `json:"summary"`
	SourceRefs   []string `json:"source_refs"`
	ContentHash  string   `json:"content_hash"`
	CreatedAt    string   `json:"created_at"`
}

type AgentPlanResponse struct {
	ID                 int64                       `json:"id"`
	UserID             int64                       `json:"user_id"`
	SessionID          int64                       `json:"session_id"`
	TurnID             int64                       `json:"turn_id"`
	ControllerRunID    int64                       `json:"controller_run_id"`
	Status             string                      `json:"status"`
	Goal               string                      `json:"goal"`
	Summary            string                      `json:"summary"`
	ImpactSummary      string                      `json:"impact_summary"`
	RiskLevel          string                      `json:"risk_level"`
	ConfirmationPolicy string                      `json:"confirmation_policy"`
	AllowedScopes      []string                    `json:"allowed_scopes"`
	DedupeKey          string                      `json:"dedupe_key"`
	PolicyDecision     string                      `json:"policy_decision"`
	PolicyReason       string                      `json:"policy_reason"`
	ExpiresAt          string                      `json:"expires_at,omitempty"`
	ApprovedAt         string                      `json:"approved_at,omitempty"`
	RejectedAt         string                      `json:"rejected_at,omitempty"`
	CompletedAt        string                      `json:"completed_at,omitempty"`
	FailedAt           string                      `json:"failed_at,omitempty"`
	ErrorMessage       string                      `json:"error_message"`
	Metadata           domain.AgentJSON            `json:"metadata"`
	CreatedAt          string                      `json:"created_at"`
	UpdatedAt          string                      `json:"updated_at"`
	Steps              []AgentPlanStepResponse     `json:"steps,omitempty"`
	Approvals          []AgentPlanApprovalResponse `json:"approvals,omitempty"`
}

type AgentPlanStepResponse struct {
	ID              int64            `json:"id"`
	PlanID          int64            `json:"plan_id"`
	StepOrder       int              `json:"step_order"`
	Status          string           `json:"status"`
	CapabilityKey   string           `json:"capability_key"`
	CapabilityScope []string         `json:"capability_scope"`
	Title           string           `json:"title"`
	InputSummary    string           `json:"input_summary"`
	OutputSummary   string           `json:"output_summary"`
	ExpectedOutput  string           `json:"expected_output"`
	FailureStrategy string           `json:"failure_strategy"`
	ExecutorRunID   int64            `json:"executor_run_id"`
	ObservationRef  string           `json:"observation_ref"`
	ArtifactRefs    []string         `json:"artifact_refs"`
	ErrorMessage    string           `json:"error_message"`
	RetryCount      int              `json:"retry_count"`
	MaxRetries      int              `json:"max_retries"`
	LastRetryAt     string           `json:"last_retry_at,omitempty"`
	RetryReason     string           `json:"retry_reason"`
	RetryMetadata   domain.AgentJSON `json:"retry_metadata"`
	StartedAt       string           `json:"started_at,omitempty"`
	CompletedAt     string           `json:"completed_at,omitempty"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

type AgentPlanApprovalResponse struct {
	ID        int64            `json:"id"`
	PlanID    *int64           `json:"plan_id,omitempty"`
	Channel   string           `json:"channel"`
	Status    string           `json:"status"`
	ExpiresAt string           `json:"expires_at"`
	DecidedAt string           `json:"decided_at,omitempty"`
	Metadata  domain.AgentJSON `json:"metadata"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
}

func (s *AgentSessionService) ListSessions(ctx context.Context, auth CurrentAuth) (AgentSessionListResult, error) {
	if s == nil || s.repository == nil {
		return AgentSessionListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.list", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	accounts, err := s.repository.ListExternalAccounts(ctx, auth.User.ID)
	if err != nil {
		return AgentSessionListResult{}, err
	}
	sessions, err := s.repository.ListAgentSessions(ctx, auth.User.ID)
	if err != nil {
		return AgentSessionListResult{}, err
	}
	activeByAccount := map[int64]int64{}
	accountResponses := make([]AgentExternalAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		activeByAccount[account.ID] = account.ActiveAgentSessionID
		accountResponses = append(accountResponses, agentExternalAccountResponse(account))
	}
	sessionResponses := make([]AgentSessionResponse, 0, len(sessions))
	for _, session := range sessions {
		stats, err := s.repository.GetAgentSessionStats(ctx, auth.User.ID, session.ID)
		if err != nil {
			return AgentSessionListResult{}, err
		}
		sessionResponses = append(sessionResponses, agentSessionResponse(session, stats, activeByAccount[session.ExternalAccountID] == session.ID))
	}
	return AgentSessionListResult{Accounts: accountResponses, Sessions: sessionResponses}, nil
}

func (s *AgentSessionService) CreateSession(ctx context.Context, auth CurrentAuth, externalAccountID int64, title string) (AgentSessionResponse, error) {
	if s == nil || s.repository == nil {
		return AgentSessionResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.create", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionResponse{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	account, err := s.repository.GetExternalAccount(ctx, auth.User.ID, externalAccountID)
	if err != nil {
		return AgentSessionResponse{}, err
	}
	now := s.now().UTC()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "企业微信对话"
	}
	session, err := s.repository.CreateAgentSession(ctx, domain.AgentSession{
		UserID:            auth.User.ID,
		ExternalAccountID: account.ID,
		Provider:          account.Provider,
		ChannelSessionKey: manualAgentSessionKey(account),
		Status:            domain.AgentSessionStatusActive,
		Title:             title,
		StartedAt:         now,
		LastActiveAt:      now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return AgentSessionResponse{}, err
	}
	stats, err := s.repository.GetAgentSessionStats(ctx, auth.User.ID, session.ID)
	if err != nil {
		return AgentSessionResponse{}, err
	}
	return agentSessionResponse(session, stats, false), nil
}

func (s *AgentSessionService) SelectSession(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentExternalAccountResponse, error) {
	if s == nil || s.repository == nil {
		return AgentExternalAccountResponse{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.select", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentExternalAccountResponse{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	session, err := s.repository.GetAgentSession(ctx, auth.User.ID, sessionID)
	if err != nil {
		return AgentExternalAccountResponse{}, err
	}
	account, err := s.repository.SetExternalAccountActiveSession(ctx, auth.User.ID, session.ExternalAccountID, session.ID)
	if err != nil {
		return AgentExternalAccountResponse{}, err
	}
	return agentExternalAccountResponse(account), nil
}

func (s *AgentSessionService) RebuildContext(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentSessionStats, error) {
	if s == nil || s.repository == nil {
		return AgentSessionStats{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.rebuild", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionStats{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	stats, err := s.repository.RebuildAgentSessionContext(ctx, auth.User.ID, sessionID, s.now().UTC())
	if err != nil {
		return AgentSessionStats{}, err
	}
	return agentSessionStats(stats), nil
}

func (s *AgentSessionService) ClearContext(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentSessionStats, error) {
	if s == nil || s.repository == nil {
		return AgentSessionStats{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.clear", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentSessionStats{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	stats, err := s.repository.ClearAgentSessionContext(ctx, auth.User.ID, sessionID, s.now().UTC())
	if err != nil {
		return AgentSessionStats{}, err
	}
	return agentSessionStats(stats), nil
}

func (s *AgentSessionService) DeleteSession(ctx context.Context, auth CurrentAuth, sessionID int64) error {
	if s == nil || s.repository == nil {
		return domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.delete", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if sessionID < 1 {
		return fmt.Errorf("%w: session id is required", domain.ErrInvalidInput)
	}
	return s.repository.DeleteAgentSession(ctx, auth.User.ID, sessionID)
}

func (s *AgentSessionService) ListTranscripts(ctx context.Context, auth CurrentAuth, sessionID int64, beforeEntryID int64, limit int) (AgentTranscriptListResult, error) {
	if s == nil || s.repository == nil {
		return AgentTranscriptListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_sessions_unavailable", "agent session service is unavailable", "service.agent_session.transcripts", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentTranscriptListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if _, err := s.repository.GetAgentSession(ctx, auth.User.ID, sessionID); err != nil {
		return AgentTranscriptListResult{}, err
	}
	entries, err := s.repository.ListRecentTranscriptEntries(ctx, domain.AgentTranscriptListOptions{
		SessionID:     sessionID,
		UserID:        auth.User.ID,
		BeforeEntryID: beforeEntryID,
		Roles:         []domain.AgentTranscriptRole{domain.AgentTranscriptRoleUser, domain.AgentTranscriptRoleAssistant, domain.AgentTranscriptRoleTool, domain.AgentTranscriptRoleSystem},
		Limit:         limit,
	})
	if err != nil {
		return AgentTranscriptListResult{}, err
	}
	responses := make([]AgentTranscriptEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, AgentTranscriptEntryResponse{
			ID:        entry.ID,
			TurnID:    entry.TurnID,
			Role:      string(entry.Role),
			Content:   entry.Content,
			CreatedAt: formatOptionalTime(&entry.CreatedAt),
		})
	}
	return AgentTranscriptListResult{Entries: responses}, nil
}

func (s *AgentSessionService) ListRunsByTurn(ctx context.Context, auth CurrentAuth, turnID int64) (AgentRunListResult, error) {
	if s == nil || s.repository == nil {
		return AgentRunListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_runs_unavailable", "agent run service is unavailable", "service.agent_session.runs", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentRunListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if turnID < 1 {
		return AgentRunListResult{}, fmt.Errorf("%w: turn id is required", domain.ErrInvalidInput)
	}
	runs, err := s.repository.ListAgentRunsByTurn(ctx, auth.User.ID, turnID)
	if err != nil {
		return AgentRunListResult{}, err
	}
	responses := make([]AgentRunResponse, 0, len(runs))
	for _, run := range runs {
		responses = append(responses, agentRunResponse(run, false))
	}
	return AgentRunListResult{Runs: responses}, nil
}

func (s *AgentSessionService) GetRunDetail(ctx context.Context, auth CurrentAuth, runID int64) (AgentRunDetailResult, error) {
	if s == nil || s.repository == nil {
		return AgentRunDetailResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_runs_unavailable", "agent run service is unavailable", "service.agent_session.run_detail", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentRunDetailResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if runID < 1 {
		return AgentRunDetailResult{}, fmt.Errorf("%w: run id is required", domain.ErrInvalidInput)
	}
	run, err := s.repository.GetAgentRunDetail(ctx, auth.User.ID, runID)
	if err != nil {
		return AgentRunDetailResult{}, err
	}
	return AgentRunDetailResult{Run: agentRunResponse(run, true)}, nil
}

func (s *AgentSessionService) ListPlans(ctx context.Context, auth CurrentAuth, sessionID int64, turnID int64, limit int) (AgentPlanListResult, error) {
	if s == nil || s.repository == nil {
		return AgentPlanListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_plans_unavailable", "agent plan service is unavailable", "service.agent_session.plans", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentPlanListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	plans, err := s.repository.ListAgentPlans(ctx, auth.User.ID, sessionID, turnID, limit)
	if err != nil {
		return AgentPlanListResult{}, err
	}
	responses := make([]AgentPlanResponse, 0, len(plans))
	for _, plan := range plans {
		responses = append(responses, agentPlanResponse(plan, false))
	}
	return AgentPlanListResult{Plans: responses}, nil
}

func (s *AgentSessionService) GetPlanDetail(ctx context.Context, auth CurrentAuth, planID int64) (AgentPlanDetailResult, error) {
	if s == nil || s.repository == nil {
		return AgentPlanDetailResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_plans_unavailable", "agent plan service is unavailable", "service.agent_session.plan_detail", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentPlanDetailResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if planID < 1 {
		return AgentPlanDetailResult{}, fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}
	plan, err := s.repository.GetAgentPlan(ctx, auth.User.ID, planID)
	if err != nil {
		return AgentPlanDetailResult{}, err
	}
	return AgentPlanDetailResult{Plan: agentPlanResponse(plan, true)}, nil
}

func (s *AgentSessionService) ListTasks(ctx context.Context, auth CurrentAuth, limit int) (AgentTaskListResult, error) {
	if s == nil || s.repository == nil {
		return AgentTaskListResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tasks_unavailable", "agent task service is unavailable", "service.agent_session.tasks", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentTaskListResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	plans, err := s.repository.ListAgentPlans(ctx, auth.User.ID, 0, 0, limit)
	if err != nil {
		return AgentTaskListResult{}, err
	}
	scheduledTasks, err := s.repository.ListAgentScheduledTasks(ctx, domain.AgentScheduledTaskListOptions{UserID: auth.User.ID, Limit: limit})
	if err != nil {
		return AgentTaskListResult{}, err
	}
	items := make([]AgentTaskSummaryResponse, 0, len(plans)+len(scheduledTasks))
	for _, plan := range plans {
		items = append(items, agentTaskSummaryFromPlan(plan))
	}
	for _, task := range scheduledTasks {
		items = append(items, agentTaskSummaryFromScheduledTask(task))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	if len(items) > limit {
		items = items[:limit]
	}
	auditLogs, _ := s.repository.ListAuditLogsByRefs(ctx, domain.AgentAuditLogListOptions{UserID: auth.User.ID, Limit: 100})
	preference := s.agentNotificationPreference(ctx, auth.User.ID)
	alerts := buildAgentAlertSummary(plans, scheduledTasks, auditLogs)
	alertPolicy := buildAgentAlertPolicy(alerts, preference)
	sla := buildAgentSLASummary(plans, scheduledTasks, auditLogs)
	cost := buildAgentTaskCostSummary(plans, scheduledTasks, auditLogs)
	deployment := buildAgentDeploymentVerification(plans)
	trendSnapshot := buildAgentTrendSnapshot(plans, scheduledTasks, auditLogs)
	drill := buildAgentProductionDrill(deployment, auditLogs, s.now().UTC())
	wechatComponents := buildAgentWeChatComponentSet(plans, scheduledTasks)
	loadTest := buildAgentLoadTestSummary(plans, scheduledTasks, auditLogs)
	wechatCallback := buildAgentWeChatCallbackReadiness(auditLogs, wechatComponents)
	writeSandbox := buildAgentWriteSandbox(plans, auditLogs)
	e2e := buildAgentE2EAcceptance(plans, scheduledTasks, auditLogs, wechatComponents)
	realIntegration := buildAgentRealIntegrationReadiness(deployment, wechatCallback, drill, alertPolicy, auditLogs)
	wechatNative := buildAgentWeChatNativeActions(wechatComponents)
	writeLeastPrivilege := buildAgentWriteLeastPrivilege(writeSandbox, plans, auditLogs)
	opsAcceptance := buildAgentOpsAcceptance(deployment, drill, alerts, alertPolicy, trendSnapshot, loadTest, wechatCallback, writeLeastPrivilege)
	wechatNativePayload := buildAgentWeChatNativePayload(wechatNative)
	writeGray := buildAgentWriteGrayPolicy(writeLeastPrivilege, alertPolicy)
	alertChannel := buildAgentAlertChannel(alerts, alertPolicy, wechatComponents, wechatNativePayload)
	launchDrill := buildAgentLaunchDrillRecord(opsAcceptance, realIntegration, writeGray, alertChannel, s.now().UTC())
	wechatNativeIntegration := buildAgentWeChatNativeIntegration(wechatNativePayload, launchDrill)
	writeReplay := buildAgentWriteReplay(writeGray, writeLeastPrivilege, plans, auditLogs)
	launchApproval := buildAgentLaunchApproval(launchDrill, plans, auditLogs)
	dailyReport := buildAgentDailyReport(plans, scheduledTasks, auditLogs, alerts, trendSnapshot, s.now().UTC())
	preprod := buildAgentPreprodAcceptance(deployment, realIntegration, opsAcceptance, alertChannel)
	buttonLoop := buildAgentButtonLoop(wechatNativePayload, wechatNativeIntegration)
	writeExecute := buildAgentWriteExecute(writeReplay, writeLeastPrivilege)
	dailyPersist := buildAgentDailyPersist(dailyReport, s.now().UTC())
	postLaunchMonitor := buildAgentPostLaunchMonitor(deployment, sla, alerts, cost, trendSnapshot, scheduledTasks)
	releaseApproval := buildAgentReleaseApprovalExecution(launchApproval, auditLogs)
	buttonCallback := buildAgentButtonCallback(buttonLoop, wechatCallback, auditLogs)
	writeAudit := buildAgentWriteAuditReview(writeExecute, plans, auditLogs)
	dailySend := buildAgentDailySend(dailyPersist, dailyReport, scheduledTasks, auditLogs)
	monitorAlert := buildAgentMonitorAlertDrill(postLaunchMonitor, alertChannel, alerts, sla, auditLogs)
	buttonDirectControl := buildAgentButtonDirectControl(buttonCallback, auditLogs)
	wechatE2E := buildAgentWeChatE2EAcceptance(wechatCallback, buttonDirectControl, dailySend, buttonLoop, auditLogs)
	releaseWindow := buildAgentReleaseWindowReadiness(preprod, releaseApproval, monitorAlert, dailySend, auditLogs)
	writeGrayExpansion := buildAgentWriteGrayExpansion(writeGray, writeAudit)
	externalMonitor := buildAgentExternalMonitorIntegration(monitorAlert, alertChannel)
	releaseWindowExecution := buildAgentReleaseWindowExecution(releaseWindow, releaseApproval, monitorAlert, dailySend, auditLogs)
	externalMonitorRuntime := buildAgentExternalMonitorRuntime(externalMonitor, monitorAlert, dailySend, buttonDirectControl)
	writeGrayReview := buildAgentWriteGrayReview(writeGrayExpansion, writeLeastPrivilege, writeAudit)
	wechatAcceptanceReview := buildAgentWeChatAcceptanceReview(wechatE2E, buttonDirectControl, dailySend, buttonLoop, auditLogs)
	operationsDailyClosure := buildAgentOperationsDailyClosure(dailySend, monitorAlert, buttonDirectControl, releaseWindowExecution, auditLogs)
	productionRelease := buildAgentProductionRelease(releaseWindowExecution, releaseApproval, preprod, dailySend, auditLogs)
	externalMonitorConfig := buildAgentExternalMonitorConfig(externalMonitorRuntime, dailySend)
	writeRamp := buildAgentWriteRamp(writeGrayReview)
	wechatSignoff := buildAgentWeChatSignoff(wechatAcceptanceReview)
	operationsHandoff := buildAgentOperationsHandoff(productionRelease, externalMonitorConfig, writeRamp, wechatSignoff, auditLogs)
	productionExecution := buildAgentProductionExecution(productionRelease, operationsHandoff, auditLogs)
	monitorIntegration := buildAgentMonitorIntegration(externalMonitorConfig, externalMonitorRuntime)
	writeRampPolicy := buildAgentWriteRampPolicy(writeRamp)
	wechatFinalReport := buildAgentWeChatFinalReport(wechatSignoff, dailySend, dailyReport, auditLogs)
	launchRuntimeOverview := buildAgentLaunchRuntimeOverview(productionExecution, monitorIntegration, writeRampPolicy, wechatFinalReport, auditLogs)
	runtimeParameters := buildAgentRuntimeParameters(writeRampPolicy, monitorIntegration, wechatFinalReport)
	monitorReadback := buildAgentMonitorReadback(monitorIntegration, wechatFinalReport, s.now().UTC())
	writeRampRecommendation := buildAgentWriteRampRecommendation(writeRampPolicy, monitorReadback)
	wechatUserFeedback := buildAgentWeChatUserFeedback(wechatFinalReport, wechatSignoff, buttonDirectControl)
	operationsRuntimeClosure := buildAgentOperationsRuntimeClosure(runtimeParameters, monitorReadback, writeRampRecommendation, wechatUserFeedback, auditLogs)
	opsPanelConfig := buildAgentOpsPanelConfig(runtimeParameters, monitorReadback, writeRampRecommendation, wechatUserFeedback)
	monitorAutoReport := buildAgentMonitorAutoReport(monitorReadback, wechatFinalReport, wechatUserFeedback, auditLogs)
	writeRampStage := buildAgentWriteRampStage(writeRampRecommendation)
	wechatFeedbackLoop := buildAgentWeChatFeedbackLoop(wechatUserFeedback, wechatFinalReport, buttonDirectControl)
	operationsClosedLoop := buildAgentOperationsClosedLoop(opsPanelConfig, monitorAutoReport, writeRampStage, wechatFeedbackLoop, auditLogs)
	opsDashboardInteraction := buildAgentOpsDashboardInteraction(opsPanelConfig, operationsClosedLoop)
	alertDedupeEscalation := buildAgentAlertDedupeEscalation(monitorAutoReport, monitorReadback)
	writeStageRecord := buildAgentWriteStageRecord(writeRampStage, writeRampRecommendation)
	wechatFeedbackTicket := buildAgentWeChatFeedbackTicket(wechatFeedbackLoop)
	operationsHandling := buildAgentOperationsHandling(opsDashboardInteraction, alertDedupeEscalation, writeStageRecord, wechatFeedbackTicket, auditLogs)
	opsActionDefinition := buildAgentOpsActionDefinition(opsDashboardInteraction, operationsHandling)
	alertEscalationPolicy := buildAgentAlertEscalationPolicy(alertDedupeEscalation, alertChannel)
	writeStageApproval := buildAgentWriteStageApproval(writeStageRecord, writeRampPolicy, releaseApproval)
	feedbackTicketLifecycle := buildAgentFeedbackTicketLifecycle(wechatFeedbackTicket, wechatFeedbackLoop)
	operationsActionClosure := buildAgentOperationsActionClosure(opsActionDefinition, alertEscalationPolicy, writeStageApproval, feedbackTicketLifecycle, auditLogs)
	opsAPIExecution := buildAgentOpsAPIExecution(opsActionDefinition, operationsActionClosure)
	alertEscalationReceipt := buildAgentAlertEscalationReceipt(alertEscalationPolicy)
	writeApprovalButton := buildAgentWriteApprovalButton(writeStageApproval)
	feedbackTicketSLA := buildAgentFeedbackTicketSLA(feedbackTicketLifecycle)
	operationsExecution := buildAgentOperationsExecution(opsAPIExecution, alertEscalationReceipt, writeApprovalButton, feedbackTicketSLA, auditLogs)
	opsExecutionRecord := buildAgentOpsExecutionRecord(opsAPIExecution)
	wechatApprovalCallback := buildAgentWeChatApprovalCallback(writeApprovalButton)
	feedbackSLAReport := buildAgentFeedbackSLAReport(feedbackTicketSLA)
	alertAutoRecovery := buildAgentAlertAutoRecovery(alertEscalationReceipt)
	operationsEvidence := buildAgentOperationsEvidence(opsExecutionRecord, wechatApprovalCallback, feedbackSLAReport, alertAutoRecovery, auditLogs)
	unifiedProgressComponent := buildAgentUnifiedProgressComponent(items, operationsEvidence)
	evidenceDetailPage := buildAgentEvidenceDetailPage(opsExecutionRecord, operationsEvidence)
	callbackReplayTool := buildAgentCallbackReplayTool(wechatApprovalCallback)
	recoveryPolicyConfig := buildAgentRecoveryPolicyConfig(alertAutoRecovery)
	dualEndProgressEvidence := buildAgentDualEndProgressEvidence(unifiedProgressComponent, evidenceDetailPage, callbackReplayTool, recoveryPolicyConfig, auditLogs)
	wechatProgressCard := buildAgentWeChatProgressCard(unifiedProgressComponent, evidenceDetailPage)
	webEvidenceInteraction := buildAgentWebEvidenceInteraction(evidenceDetailPage)
	callbackReplayPermission := buildAgentCallbackReplayPermission(callbackReplayTool)
	recoveryPolicyAudit := buildAgentRecoveryPolicyAudit(recoveryPolicyConfig)
	dualEndInteraction := buildAgentDualEndInteraction(wechatProgressCard, webEvidenceInteraction, callbackReplayPermission, recoveryPolicyAudit, auditLogs)
	wechatTemplateRender := buildAgentWeChatTemplateRender(wechatProgressCard)
	webEvidenceRoute := buildAgentWebEvidenceRoute(webEvidenceInteraction)
	callbackReplayApproval := buildAgentCallbackReplayApproval(callbackReplayPermission)
	recoveryPolicyPersist := buildAgentRecoveryPolicyPersist(recoveryPolicyAudit)
	dualEndInteractionLaunch := buildAgentDualEndInteractionLaunch(wechatTemplateRender, webEvidenceRoute, callbackReplayApproval, recoveryPolicyPersist, auditLogs)
	wechatTemplateSend := buildAgentWeChatTemplateSend(wechatTemplateRender)
	webEvidenceDetailView := buildAgentWebEvidenceDetailView(webEvidenceRoute)
	callbackReplayExecution := buildAgentCallbackReplayExecution(callbackReplayApproval)
	recoveryPolicyVersion := buildAgentRecoveryPolicyVersion(recoveryPolicyPersist)
	dualEndRealInteraction := buildAgentDualEndRealInteraction(wechatTemplateSend, webEvidenceDetailView, callbackReplayExecution, recoveryPolicyVersion, auditLogs)
	wechatTemplateIntegration := buildAgentWeChatTemplateIntegration(wechatTemplateSend)
	webEvidenceInteractionDetail := buildAgentWebEvidenceInteractionDetail(webEvidenceDetailView)
	callbackReplaySafetyAudit := buildAgentCallbackReplaySafetyAudit(callbackReplayExecution)
	recoveryPolicyGrayRelease := buildAgentRecoveryPolicyGrayRelease(recoveryPolicyVersion)
	dualEndRunLoop := buildAgentDualEndRunLoop(wechatTemplateIntegration, webEvidenceInteractionDetail, callbackReplaySafetyAudit, recoveryPolicyGrayRelease, auditLogs)
	wechatTemplatePilot := buildAgentWeChatTemplatePilot(wechatTemplateIntegration, s.now().UTC())
	webEvidenceUserAction := buildAgentWebEvidenceUserAction(webEvidenceInteractionDetail)
	callbackReplayResultTrace := buildAgentCallbackReplayResultTrace(callbackReplaySafetyAudit)
	recoveryPolicyAutomation := buildAgentRecoveryPolicyAutomation(recoveryPolicyGrayRelease)
	dualEndTaskClosure := buildAgentDualEndTaskClosure(wechatTemplatePilot, webEvidenceUserAction, callbackReplayResultTrace, recoveryPolicyAutomation, auditLogs)
	wechatTemplatePilotMetric := buildAgentWeChatTemplatePilotMetric(wechatTemplatePilot, auditLogs)
	webEvidenceOperation := buildAgentWebEvidenceOperation(webEvidenceUserAction, auditLogs)
	callbackReplayResultQuery := buildAgentCallbackReplayResultQuery(callbackReplayResultTrace, auditLogs)
	recoveryAutomationExecution := buildAgentRecoveryAutomationExecution(recoveryPolicyAutomation, auditLogs)
	realInteractionAutomation := buildAgentRealInteractionAutomation(wechatTemplatePilotMetric, webEvidenceOperation, callbackReplayResultQuery, recoveryAutomationExecution, auditLogs)
	s.recordAgentProductionDrillSnapshot(ctx, auth.User.ID, drill, trendSnapshot)
	s.recordAgentAlertPolicyDecision(ctx, auth.User.ID, alertPolicy, alerts)
	s.recordAgentWriteSandboxSnapshot(ctx, auth.User.ID, writeSandbox)
	s.recordAgentE2EAcceptanceSnapshot(ctx, auth.User.ID, e2e, loadTest, wechatCallback)
	s.recordAgentRealIntegrationSnapshot(ctx, auth.User.ID, realIntegration)
	s.recordAgentOpsAcceptanceSnapshot(ctx, auth.User.ID, opsAcceptance, writeLeastPrivilege)
	s.recordAgentWriteGraySnapshot(ctx, auth.User.ID, writeGray)
	s.recordAgentAlertChannelSnapshot(ctx, auth.User.ID, alertChannel)
	s.recordAgentLaunchDrillRecord(ctx, auth.User.ID, launchDrill)
	s.recordAgentWeChatNativeIntegrationSnapshot(ctx, auth.User.ID, wechatNativeIntegration)
	s.recordAgentWriteReplaySnapshot(ctx, auth.User.ID, writeReplay)
	s.recordAgentLaunchApprovalSnapshot(ctx, auth.User.ID, launchApproval)
	s.recordAgentDailyReportSnapshot(ctx, auth.User.ID, dailyReport)
	s.recordAgentPreprodAcceptanceSnapshot(ctx, auth.User.ID, preprod)
	s.recordAgentButtonLoopSnapshot(ctx, auth.User.ID, buttonLoop)
	s.recordAgentWriteExecuteSnapshot(ctx, auth.User.ID, writeExecute)
	s.recordAgentDailyPersistSnapshot(ctx, auth.User.ID, dailyPersist)
	s.recordAgentPostLaunchMonitorSnapshot(ctx, auth.User.ID, postLaunchMonitor)
	s.recordAgentReleaseApprovalSnapshot(ctx, auth.User.ID, releaseApproval)
	s.recordAgentButtonCallbackSnapshot(ctx, auth.User.ID, buttonCallback)
	s.recordAgentWriteAuditReviewSnapshot(ctx, auth.User.ID, writeAudit)
	s.recordAgentDailySendSnapshot(ctx, auth.User.ID, dailySend)
	s.recordAgentMonitorAlertDrillSnapshot(ctx, auth.User.ID, monitorAlert)
	s.recordAgentButtonDirectControlSnapshot(ctx, auth.User.ID, buttonDirectControl)
	s.recordAgentWeChatE2EAcceptanceSnapshot(ctx, auth.User.ID, wechatE2E)
	s.recordAgentReleaseWindowReadinessSnapshot(ctx, auth.User.ID, releaseWindow)
	s.recordAgentWriteGrayExpansionSnapshot(ctx, auth.User.ID, writeGrayExpansion)
	s.recordAgentExternalMonitorIntegrationSnapshot(ctx, auth.User.ID, externalMonitor)
	s.recordAgentReleaseWindowExecutionSnapshot(ctx, auth.User.ID, releaseWindowExecution)
	s.recordAgentExternalMonitorRuntimeSnapshot(ctx, auth.User.ID, externalMonitorRuntime)
	s.recordAgentWriteGrayReviewSnapshot(ctx, auth.User.ID, writeGrayReview)
	s.recordAgentWeChatAcceptanceReviewSnapshot(ctx, auth.User.ID, wechatAcceptanceReview)
	s.recordAgentOperationsDailyClosureSnapshot(ctx, auth.User.ID, operationsDailyClosure)
	s.recordAgentProductionReleaseSnapshot(ctx, auth.User.ID, productionRelease)
	s.recordAgentExternalMonitorConfigSnapshot(ctx, auth.User.ID, externalMonitorConfig)
	s.recordAgentWriteRampSnapshot(ctx, auth.User.ID, writeRamp)
	s.recordAgentWeChatSignoffSnapshot(ctx, auth.User.ID, wechatSignoff)
	s.recordAgentOperationsHandoffSnapshot(ctx, auth.User.ID, operationsHandoff)
	s.recordAgentProductionExecutionSnapshot(ctx, auth.User.ID, productionExecution)
	s.recordAgentMonitorIntegrationSnapshot(ctx, auth.User.ID, monitorIntegration)
	s.recordAgentWriteRampPolicySnapshot(ctx, auth.User.ID, writeRampPolicy)
	s.recordAgentWeChatFinalReportSnapshot(ctx, auth.User.ID, wechatFinalReport)
	s.recordAgentLaunchRuntimeOverviewSnapshot(ctx, auth.User.ID, launchRuntimeOverview)
	s.recordAgentRuntimeParametersSnapshot(ctx, auth.User.ID, runtimeParameters)
	s.recordAgentMonitorReadbackSnapshot(ctx, auth.User.ID, monitorReadback)
	s.recordAgentWriteRampRecommendationSnapshot(ctx, auth.User.ID, writeRampRecommendation)
	s.recordAgentWeChatUserFeedbackSnapshot(ctx, auth.User.ID, wechatUserFeedback)
	s.recordAgentOperationsRuntimeClosureSnapshot(ctx, auth.User.ID, operationsRuntimeClosure)
	s.recordAgentOpsPanelConfigSnapshot(ctx, auth.User.ID, opsPanelConfig)
	s.recordAgentMonitorAutoReportSnapshot(ctx, auth.User.ID, monitorAutoReport)
	s.recordAgentWriteRampStageSnapshot(ctx, auth.User.ID, writeRampStage)
	s.recordAgentWeChatFeedbackLoopSnapshot(ctx, auth.User.ID, wechatFeedbackLoop)
	s.recordAgentOperationsClosedLoopSnapshot(ctx, auth.User.ID, operationsClosedLoop)
	s.recordAgentOpsDashboardInteractionSnapshot(ctx, auth.User.ID, opsDashboardInteraction)
	s.recordAgentAlertDedupeEscalationSnapshot(ctx, auth.User.ID, alertDedupeEscalation)
	s.recordAgentWriteStageRecordSnapshot(ctx, auth.User.ID, writeStageRecord)
	s.recordAgentWeChatFeedbackTicketSnapshot(ctx, auth.User.ID, wechatFeedbackTicket)
	s.recordAgentOperationsHandlingSnapshot(ctx, auth.User.ID, operationsHandling)
	s.recordAgentOpsActionDefinitionSnapshot(ctx, auth.User.ID, opsActionDefinition)
	s.recordAgentAlertEscalationPolicySnapshot(ctx, auth.User.ID, alertEscalationPolicy)
	s.recordAgentWriteStageApprovalSnapshot(ctx, auth.User.ID, writeStageApproval)
	s.recordAgentFeedbackTicketLifecycleSnapshot(ctx, auth.User.ID, feedbackTicketLifecycle)
	s.recordAgentOperationsActionClosureSnapshot(ctx, auth.User.ID, operationsActionClosure)
	s.recordAgentOpsAPIExecutionSnapshot(ctx, auth.User.ID, opsAPIExecution)
	s.recordAgentAlertEscalationReceiptSnapshot(ctx, auth.User.ID, alertEscalationReceipt)
	s.recordAgentWriteApprovalButtonSnapshot(ctx, auth.User.ID, writeApprovalButton)
	s.recordAgentFeedbackTicketSLASnapshot(ctx, auth.User.ID, feedbackTicketSLA)
	s.recordAgentOperationsExecutionSnapshot(ctx, auth.User.ID, operationsExecution)
	s.recordAgentOpsExecutionRecordSnapshot(ctx, auth.User.ID, opsExecutionRecord)
	s.recordAgentWeChatApprovalCallbackSnapshot(ctx, auth.User.ID, wechatApprovalCallback)
	s.recordAgentFeedbackSLAReportSnapshot(ctx, auth.User.ID, feedbackSLAReport)
	s.recordAgentAlertAutoRecoverySnapshot(ctx, auth.User.ID, alertAutoRecovery)
	s.recordAgentOperationsEvidenceSnapshot(ctx, auth.User.ID, operationsEvidence)
	s.recordAgentUnifiedProgressComponentSnapshot(ctx, auth.User.ID, unifiedProgressComponent)
	s.recordAgentEvidenceDetailPageSnapshot(ctx, auth.User.ID, evidenceDetailPage)
	s.recordAgentCallbackReplayToolSnapshot(ctx, auth.User.ID, callbackReplayTool)
	s.recordAgentRecoveryPolicyConfigSnapshot(ctx, auth.User.ID, recoveryPolicyConfig)
	s.recordAgentDualEndProgressEvidenceSnapshot(ctx, auth.User.ID, dualEndProgressEvidence)
	s.recordAgentWeChatProgressCardSnapshot(ctx, auth.User.ID, wechatProgressCard)
	s.recordAgentWebEvidenceInteractionSnapshot(ctx, auth.User.ID, webEvidenceInteraction)
	s.recordAgentCallbackReplayPermissionSnapshot(ctx, auth.User.ID, callbackReplayPermission)
	s.recordAgentRecoveryPolicyAuditSnapshot(ctx, auth.User.ID, recoveryPolicyAudit)
	s.recordAgentDualEndInteractionSnapshot(ctx, auth.User.ID, dualEndInteraction)
	s.recordAgentWeChatTemplateRenderSnapshot(ctx, auth.User.ID, wechatTemplateRender)
	s.recordAgentWebEvidenceRouteSnapshot(ctx, auth.User.ID, webEvidenceRoute)
	s.recordAgentCallbackReplayApprovalSnapshot(ctx, auth.User.ID, callbackReplayApproval)
	s.recordAgentRecoveryPolicyPersistSnapshot(ctx, auth.User.ID, recoveryPolicyPersist)
	s.recordAgentDualEndInteractionLaunchSnapshot(ctx, auth.User.ID, dualEndInteractionLaunch)
	s.recordAgentWeChatTemplateSendSnapshot(ctx, auth.User.ID, wechatTemplateSend)
	s.recordAgentWebEvidenceDetailViewSnapshot(ctx, auth.User.ID, webEvidenceDetailView)
	s.recordAgentCallbackReplayExecutionSnapshot(ctx, auth.User.ID, callbackReplayExecution)
	s.recordAgentRecoveryPolicyVersionSnapshot(ctx, auth.User.ID, recoveryPolicyVersion)
	s.recordAgentDualEndRealInteractionSnapshot(ctx, auth.User.ID, dualEndRealInteraction)
	s.recordAgentWeChatTemplateIntegrationSnapshot(ctx, auth.User.ID, wechatTemplateIntegration)
	s.recordAgentWebEvidenceInteractionDetailSnapshot(ctx, auth.User.ID, webEvidenceInteractionDetail)
	s.recordAgentCallbackReplaySafetyAuditSnapshot(ctx, auth.User.ID, callbackReplaySafetyAudit)
	s.recordAgentRecoveryPolicyGrayReleaseSnapshot(ctx, auth.User.ID, recoveryPolicyGrayRelease)
	s.recordAgentDualEndRunLoopSnapshot(ctx, auth.User.ID, dualEndRunLoop)
	s.recordAgentWeChatTemplatePilotSnapshot(ctx, auth.User.ID, wechatTemplatePilot)
	s.recordAgentWebEvidenceUserActionSnapshot(ctx, auth.User.ID, webEvidenceUserAction)
	s.recordAgentCallbackReplayResultTraceSnapshot(ctx, auth.User.ID, callbackReplayResultTrace)
	s.recordAgentRecoveryPolicyAutomationSnapshot(ctx, auth.User.ID, recoveryPolicyAutomation)
	s.recordAgentDualEndTaskClosureSnapshot(ctx, auth.User.ID, dualEndTaskClosure)
	s.recordAgentWeChatTemplatePilotMetricSnapshot(ctx, auth.User.ID, wechatTemplatePilotMetric)
	s.recordAgentWebEvidenceOperationSnapshot(ctx, auth.User.ID, webEvidenceOperation)
	s.recordAgentCallbackReplayResultQuerySnapshot(ctx, auth.User.ID, callbackReplayResultQuery)
	s.recordAgentRecoveryAutomationExecutionSnapshot(ctx, auth.User.ID, recoveryAutomationExecution)
	s.recordAgentRealInteractionAutomationSnapshot(ctx, auth.User.ID, realInteractionAutomation)
	return AgentTaskListResult{
		Tasks:                        items,
		SLA:                          sla,
		Cost:                         cost,
		Alerts:                       alerts,
		AlertPolicy:                  alertPolicy,
		CostTrend:                    buildAgentCostTrend(plans, auditLogs),
		TrendSnapshot:                trendSnapshot,
		Deployment:                   deployment,
		Drill:                        drill,
		WeChatComponents:             wechatComponents,
		LoadTest:                     loadTest,
		WeChatCallback:               wechatCallback,
		WriteSandbox:                 writeSandbox,
		E2E:                          e2e,
		RealIntegration:              realIntegration,
		WeChatNative:                 wechatNative,
		WriteLeastPrivilege:          writeLeastPrivilege,
		OpsAcceptance:                opsAcceptance,
		WeChatNativePayload:          wechatNativePayload,
		WriteGray:                    writeGray,
		AlertChannel:                 alertChannel,
		LaunchDrill:                  launchDrill,
		WeChatNativeIntegration:      wechatNativeIntegration,
		WriteReplay:                  writeReplay,
		LaunchApproval:               launchApproval,
		DailyReport:                  dailyReport,
		Preprod:                      preprod,
		ButtonLoop:                   buttonLoop,
		WriteExecute:                 writeExecute,
		DailyPersist:                 dailyPersist,
		PostLaunchMonitor:            postLaunchMonitor,
		ReleaseApproval:              releaseApproval,
		ButtonCallback:               buttonCallback,
		WriteAudit:                   writeAudit,
		DailySend:                    dailySend,
		MonitorAlert:                 monitorAlert,
		ButtonDirectControl:          buttonDirectControl,
		WeChatE2E:                    wechatE2E,
		ReleaseWindow:                releaseWindow,
		WriteGrayExpansion:           writeGrayExpansion,
		ExternalMonitor:              externalMonitor,
		ReleaseWindowExecution:       releaseWindowExecution,
		ExternalMonitorRuntime:       externalMonitorRuntime,
		WriteGrayReview:              writeGrayReview,
		WeChatAcceptanceReview:       wechatAcceptanceReview,
		OperationsDailyClosure:       operationsDailyClosure,
		ProductionRelease:            productionRelease,
		ExternalMonitorConfig:        externalMonitorConfig,
		WriteRamp:                    writeRamp,
		WeChatSignoff:                wechatSignoff,
		OperationsHandoff:            operationsHandoff,
		ProductionExecution:          productionExecution,
		MonitorIntegration:           monitorIntegration,
		WriteRampPolicy:              writeRampPolicy,
		WeChatFinalReport:            wechatFinalReport,
		LaunchRuntimeOverview:        launchRuntimeOverview,
		RuntimeParameters:            runtimeParameters,
		MonitorReadback:              monitorReadback,
		WriteRampRecommendation:      writeRampRecommendation,
		WeChatUserFeedback:           wechatUserFeedback,
		OperationsRuntimeClosure:     operationsRuntimeClosure,
		OpsPanelConfig:               opsPanelConfig,
		MonitorAutoReport:            monitorAutoReport,
		WriteRampStage:               writeRampStage,
		WeChatFeedbackLoop:           wechatFeedbackLoop,
		OperationsClosedLoop:         operationsClosedLoop,
		OpsDashboardInteraction:      opsDashboardInteraction,
		AlertDedupeEscalation:        alertDedupeEscalation,
		WriteStageRecord:             writeStageRecord,
		WeChatFeedbackTicket:         wechatFeedbackTicket,
		OperationsHandling:           operationsHandling,
		OpsActionDefinition:          opsActionDefinition,
		AlertEscalationPolicy:        alertEscalationPolicy,
		WriteStageApproval:           writeStageApproval,
		FeedbackTicketLifecycle:      feedbackTicketLifecycle,
		OperationsActionClosure:      operationsActionClosure,
		OpsAPIExecution:              opsAPIExecution,
		AlertEscalationReceipt:       alertEscalationReceipt,
		WriteApprovalButton:          writeApprovalButton,
		FeedbackTicketSLA:            feedbackTicketSLA,
		OperationsExecution:          operationsExecution,
		OpsExecutionRecord:           opsExecutionRecord,
		WeChatApprovalCallback:       wechatApprovalCallback,
		FeedbackSLAReport:            feedbackSLAReport,
		AlertAutoRecovery:            alertAutoRecovery,
		OperationsEvidence:           operationsEvidence,
		UnifiedProgressComponent:     unifiedProgressComponent,
		EvidenceDetailPage:           evidenceDetailPage,
		CallbackReplayTool:           callbackReplayTool,
		RecoveryPolicyConfig:         recoveryPolicyConfig,
		DualEndProgressEvidence:      dualEndProgressEvidence,
		WeChatProgressCard:           wechatProgressCard,
		WebEvidenceInteraction:       webEvidenceInteraction,
		CallbackReplayPermission:     callbackReplayPermission,
		RecoveryPolicyAudit:          recoveryPolicyAudit,
		DualEndInteraction:           dualEndInteraction,
		WeChatTemplateRender:         wechatTemplateRender,
		WebEvidenceRoute:             webEvidenceRoute,
		CallbackReplayApproval:       callbackReplayApproval,
		RecoveryPolicyPersist:        recoveryPolicyPersist,
		DualEndInteractionLaunch:     dualEndInteractionLaunch,
		WeChatTemplateSend:           wechatTemplateSend,
		WebEvidenceDetailView:        webEvidenceDetailView,
		CallbackReplayExecution:      callbackReplayExecution,
		RecoveryPolicyVersion:        recoveryPolicyVersion,
		DualEndRealInteraction:       dualEndRealInteraction,
		WeChatTemplateIntegration:    wechatTemplateIntegration,
		WebEvidenceInteractionDetail: webEvidenceInteractionDetail,
		CallbackReplaySafetyAudit:    callbackReplaySafetyAudit,
		RecoveryPolicyGrayRelease:    recoveryPolicyGrayRelease,
		DualEndRunLoop:               dualEndRunLoop,
		WeChatTemplatePilot:          wechatTemplatePilot,
		WebEvidenceUserAction:        webEvidenceUserAction,
		CallbackReplayResultTrace:    callbackReplayResultTrace,
		RecoveryPolicyAutomation:     recoveryPolicyAutomation,
		DualEndTaskClosure:           dualEndTaskClosure,
		WeChatTemplatePilotMetric:    wechatTemplatePilotMetric,
		WebEvidenceOperation:         webEvidenceOperation,
		CallbackReplayResultQuery:    callbackReplayResultQuery,
		RecoveryAutomationExecution:  recoveryAutomationExecution,
		RealInteractionAutomation:    realInteractionAutomation,
		Report:                       buildAgentTaskReport(plans, scheduledTasks),
	}, nil
}

func (s *AgentSessionService) agentNotificationPreference(ctx context.Context, userID int64) domain.AgentNotificationPreference {
	if s == nil || s.repository == nil || userID < 1 {
		return defaultAgentNotificationPreference(userID, time.Time{})
	}
	type preferenceRepository interface {
		GetAgentNotificationPreference(context.Context, int64) (domain.AgentNotificationPreference, error)
	}
	store, ok := any(s.repository).(preferenceRepository)
	if !ok {
		return defaultAgentNotificationPreference(userID, s.now().UTC())
	}
	preference, err := store.GetAgentNotificationPreference(ctx, userID)
	if err != nil {
		return defaultAgentNotificationPreference(userID, s.now().UTC())
	}
	return normalizeAgentPolicyPreference(preference)
}

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

func (s *AgentSessionService) recordAgentWriteStageApprovalSnapshot(ctx context.Context, userID int64, approval AgentWriteStageApprovalResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_stage_approval_snapshot",
		Status:    approval.Status,
		Message:   approval.Summary,
		Metadata: domain.AgentJSON{
			"approval_status":    approval.ApprovalStatus,
			"approval_source":    approval.ApprovalSource,
			"target_stage":       approval.TargetStage,
			"authorized_scope":   approval.AuthorizedScope,
			"rollback_threshold": approval.RollbackThreshold,
			"default_action":     approval.DefaultAction,
			"check_count":        len(approval.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentFeedbackTicketLifecycleSnapshot(ctx context.Context, userID int64, lifecycle AgentFeedbackTicketLifecycleResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.feedback_ticket_lifecycle_snapshot",
		Status:    lifecycle.Status,
		Message:   lifecycle.Summary,
		Metadata: domain.AgentJSON{
			"created_state":      lifecycle.CreatedState,
			"assigned_state":     lifecycle.AssignedState,
			"processing_state":   lifecycle.ProcessingState,
			"waiting_user_state": lifecycle.WaitingUserState,
			"closed_state":       lifecycle.ClosedState,
			"handoff_state":      lifecycle.HandoffState,
			"check_count":        len(lifecycle.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsActionClosureSnapshot(ctx context.Context, userID int64, closure AgentOperationsActionClosureResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_action_closure_snapshot",
		Status:    closure.Status,
		Message:   closure.Summary,
		Metadata: domain.AgentJSON{
			"ops_action_status":       closure.OpsActionStatus,
			"alert_escalation_status": closure.AlertEscalationStatus,
			"write_approval_status":   closure.WriteApprovalStatus,
			"ticket_lifecycle_status": closure.TicketLifecycleStatus,
			"audit_status":            closure.AuditStatus,
			"next_action":             closure.NextAction,
			"check_count":             len(closure.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsAPIExecutionSnapshot(ctx context.Context, userID int64, execution AgentOpsAPIExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_api_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"executions":  execution.Executions,
			"check_count": len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentAlertEscalationReceiptSnapshot(ctx context.Context, userID int64, receipt AgentAlertEscalationReceiptResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_escalation_receipt_snapshot",
		Status:    receipt.Status,
		Message:   receipt.Summary,
		Metadata: domain.AgentJSON{
			"notification_channels":  receipt.NotificationChannels,
			"recipients":             receipt.Recipients,
			"delivery_status":        receipt.DeliveryStatus,
			"suppression_result":     receipt.SuppressionResult,
			"recovery_notice_status": receipt.RecoveryNoticeStatus,
			"handoff_entry":          receipt.HandoffEntry,
			"check_count":            len(receipt.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWriteApprovalButtonSnapshot(ctx context.Context, userID int64, button AgentWriteApprovalButtonResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.write_approval_button_snapshot",
		Status:    button.Status,
		Message:   button.Summary,
		Metadata: domain.AgentJSON{
			"buttons":     button.Buttons,
			"check_count": len(button.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentFeedbackTicketSLASnapshot(ctx context.Context, userID int64, sla AgentFeedbackTicketSLAResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.feedback_ticket_sla_snapshot",
		Status:    sla.Status,
		Message:   sla.Summary,
		Metadata: domain.AgentJSON{
			"first_response_seconds": sla.FirstResponseSeconds,
			"resolve_seconds":        sla.ResolveSeconds,
			"timeout_escalation":     sla.TimeoutEscalation,
			"waiting_user_status":    sla.WaitingUserStatus,
			"close_condition":        sla.CloseCondition,
			"handoff_path":           sla.HandoffPath,
			"check_count":            len(sla.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsExecutionSnapshot(ctx context.Context, userID int64, execution AgentOperationsExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"ops_api_execution_status":     execution.OpsAPIExecutionStatus,
			"alert_receipt_status":         execution.AlertReceiptStatus,
			"write_approval_button_status": execution.WriteApprovalButtonStatus,
			"feedback_sla_status":          execution.FeedbackSLAStatus,
			"audit_status":                 execution.AuditStatus,
			"next_action":                  execution.NextAction,
			"check_count":                  len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOpsExecutionRecordSnapshot(ctx context.Context, userID int64, record AgentOpsExecutionRecordResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.ops_execution_record_snapshot",
		Status:    record.Status,
		Message:   record.Summary,
		Metadata: domain.AgentJSON{
			"records":     record.Records,
			"check_count": len(record.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatApprovalCallbackSnapshot(ctx context.Context, userID int64, callback AgentWeChatApprovalCallbackResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_approval_callback_snapshot",
		Status:    callback.Status,
		Message:   callback.Summary,
		Metadata: domain.AgentJSON{
			"callback_key":  callback.CallbackKey,
			"source":        callback.Source,
			"decision":      callback.Decision,
			"signature":     callback.Signature,
			"storage_state": callback.StorageState,
			"fallback_path": callback.FallbackPath,
			"check_count":   len(callback.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentFeedbackSLAReportSnapshot(ctx context.Context, userID int64, report AgentFeedbackSLAReportResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.feedback_sla_report_snapshot",
		Status:    report.Status,
		Message:   report.Summary,
		Metadata: domain.AgentJSON{
			"first_response_rate": report.FirstResponseRate,
			"resolve_rate":        report.ResolveRate,
			"timeout_count":       report.TimeoutCount,
			"waiting_user_count":  report.WaitingUserCount,
			"handoff_count":       report.HandoffCount,
			"report_audit_event":  report.ReportAuditEvent,
			"check_count":         len(report.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentAlertAutoRecoverySnapshot(ctx context.Context, userID int64, recovery AgentAlertAutoRecoveryResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.alert_auto_recovery_snapshot",
		Status:    recovery.Status,
		Message:   recovery.Summary,
		Metadata: domain.AgentJSON{
			"recovery_trigger":    recovery.RecoveryTrigger,
			"recovery_notice":     recovery.RecoveryNotice,
			"suppression_release": recovery.SuppressionRelease,
			"reopen_condition":    recovery.ReopenCondition,
			"handoff_state":       recovery.HandoffState,
			"audit_evidence":      recovery.AuditEvidence,
			"check_count":         len(recovery.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentOperationsEvidenceSnapshot(ctx context.Context, userID int64, evidence AgentOperationsEvidenceResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.operations_evidence_snapshot",
		Status:    evidence.Status,
		Message:   evidence.Summary,
		Metadata: domain.AgentJSON{
			"execution_record_status":  evidence.ExecutionRecordStatus,
			"approval_callback_status": evidence.ApprovalCallbackStatus,
			"sla_report_status":        evidence.SLAReportStatus,
			"auto_recovery_status":     evidence.AutoRecoveryStatus,
			"audit_status":             evidence.AuditStatus,
			"next_action":              evidence.NextAction,
			"check_count":              len(evidence.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentUnifiedProgressComponentSnapshot(ctx context.Context, userID int64, component AgentUnifiedProgressComponentResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.unified_progress_component_snapshot",
		Status:    component.Status,
		Message:   component.Summary,
		Metadata: domain.AgentJSON{
			"component_key":    component.ComponentKey,
			"web_status":       component.WebStatus,
			"wechat_status":    component.WeChatStatus,
			"event_cursor":     component.EventCursor,
			"refresh_strategy": component.RefreshStrategy,
			"audit_evidence":   component.AuditEvidence,
			"check_count":      len(component.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentEvidenceDetailPageSnapshot(ctx context.Context, userID int64, page AgentEvidenceDetailPageResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.evidence_detail_page_snapshot",
		Status:    page.Status,
		Message:   page.Summary,
		Metadata: domain.AgentJSON{
			"detail_entry":     page.DetailEntry,
			"record_count":     page.RecordCount,
			"audit_event":      page.AuditEvent,
			"replay_entry":     page.ReplayEntry,
			"visibility":       page.Visibility,
			"retention_policy": page.RetentionPolicy,
			"check_count":      len(page.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayToolSnapshot(ctx context.Context, userID int64, tool AgentCallbackReplayToolResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_tool_snapshot",
		Status:    tool.Status,
		Message:   tool.Summary,
		Metadata: domain.AgentJSON{
			"callback_key":      tool.CallbackKey,
			"replay_entry":      tool.ReplayEntry,
			"signature_review":  tool.SignatureReview,
			"idempotency_guard": tool.IdempotencyGuard,
			"failure_fallback":  tool.FailureFallback,
			"audit_evidence":    tool.AuditEvidence,
			"check_count":       len(tool.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyConfigSnapshot(ctx context.Context, userID int64, config AgentRecoveryPolicyConfigResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_config_snapshot",
		Status:    config.Status,
		Message:   config.Summary,
		Metadata: domain.AgentJSON{
			"policy_key":         config.PolicyKey,
			"recovery_trigger":   config.RecoveryTrigger,
			"suppression_window": config.SuppressionWindow,
			"reopen_condition":   config.ReopenCondition,
			"handoff_state":      config.HandoffState,
			"default_policy":     config.DefaultPolicy,
			"check_count":        len(config.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndProgressEvidenceSnapshot(ctx context.Context, userID int64, evidence AgentDualEndProgressEvidenceResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_progress_evidence_snapshot",
		Status:    evidence.Status,
		Message:   evidence.Summary,
		Metadata: domain.AgentJSON{
			"unified_progress_status": evidence.UnifiedProgressStatus,
			"evidence_detail_status":  evidence.EvidenceDetailStatus,
			"callback_replay_status":  evidence.CallbackReplayStatus,
			"recovery_policy_status":  evidence.RecoveryPolicyStatus,
			"audit_status":            evidence.AuditStatus,
			"next_action":             evidence.NextAction,
			"check_count":             len(evidence.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatProgressCardSnapshot(ctx context.Context, userID int64, card AgentWeChatProgressCardResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_progress_card_snapshot",
		Status:    card.Status,
		Message:   card.Summary,
		Metadata: domain.AgentJSON{
			"card_key":         card.CardKey,
			"phase_status":     card.PhaseStatus,
			"progress_percent": card.ProgressPercent,
			"detail_entry":     card.DetailEntry,
			"actions":          card.Actions,
			"fallback_text":    card.FallbackText,
			"check_count":      len(card.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceInteractionSnapshot(ctx context.Context, userID int64, interaction AgentWebEvidenceInteractionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_interaction_snapshot",
		Status:    interaction.Status,
		Message:   interaction.Summary,
		Metadata: domain.AgentJSON{
			"filters":        interaction.Filters,
			"expandable":     interaction.Expandable,
			"replay_entry":   interaction.ReplayEntry,
			"audit_display":  interaction.AuditDisplay,
			"retention_hint": interaction.RetentionHint,
			"visibility":     interaction.Visibility,
			"check_count":    len(interaction.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayPermissionSnapshot(ctx context.Context, userID int64, permission AgentCallbackReplayPermissionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_permission_snapshot",
		Status:    permission.Status,
		Message:   permission.Summary,
		Metadata: domain.AgentJSON{
			"permission_key":    permission.PermissionKey,
			"allowed_roles":     permission.AllowedRoles,
			"idempotency_guard": permission.IdempotencyGuard,
			"signature_review":  permission.SignatureReview,
			"failure_fallback":  permission.FailureFallback,
			"audit_event":       permission.AuditEvent,
			"check_count":       len(permission.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyAuditSnapshot(ctx context.Context, userID int64, audit AgentRecoveryPolicyAuditResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_audit_snapshot",
		Status:    audit.Status,
		Message:   audit.Summary,
		Metadata: domain.AgentJSON{
			"change_key":      audit.ChangeKey,
			"old_policy":      audit.OldPolicy,
			"new_policy":      audit.NewPolicy,
			"approval_status": audit.ApprovalStatus,
			"rollback_path":   audit.RollbackPath,
			"audit_evidence":  audit.AuditEvidence,
			"check_count":     len(audit.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndInteractionSnapshot(ctx context.Context, userID int64, interaction AgentDualEndInteractionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_interaction_snapshot",
		Status:    interaction.Status,
		Message:   interaction.Summary,
		Metadata: domain.AgentJSON{
			"wechat_progress_card_status":  interaction.WeChatProgressCardStatus,
			"web_evidence_status":          interaction.WebEvidenceStatus,
			"callback_permission_status":   interaction.CallbackPermissionStatus,
			"recovery_policy_audit_status": interaction.RecoveryPolicyAuditStatus,
			"audit_status":                 interaction.AuditStatus,
			"next_action":                  interaction.NextAction,
			"check_count":                  len(interaction.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatTemplateRenderSnapshot(ctx context.Context, userID int64, render AgentWeChatTemplateRenderResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_template_render_snapshot",
		Status:    render.Status,
		Message:   render.Summary,
		Metadata: domain.AgentJSON{
			"template_key":  render.TemplateKey,
			"render_status": render.RenderStatus,
			"phase_fields":  render.PhaseFields,
			"button_fields": render.ButtonFields,
			"fallback_text": render.FallbackText,
			"send_entry":    render.SendEntry,
			"check_count":   len(render.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceRouteSnapshot(ctx context.Context, userID int64, route AgentWebEvidenceRouteResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_route_snapshot",
		Status:    route.Status,
		Message:   route.Summary,
		Metadata: domain.AgentJSON{
			"route_name":             route.RouteName,
			"path_params":            route.PathParams,
			"filter_params":          route.FilterParams,
			"permission_requirement": route.PermissionRequirement,
			"replay_entry":           route.ReplayEntry,
			"audit_display":          route.AuditDisplay,
			"check_count":            len(route.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayApprovalSnapshot(ctx context.Context, userID int64, approval AgentCallbackReplayApprovalResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_approval_snapshot",
		Status:    approval.Status,
		Message:   approval.Summary,
		Metadata: domain.AgentJSON{
			"approval_key":    approval.ApprovalKey,
			"request_entry":   approval.RequestEntry,
			"approval_roles":  approval.ApprovalRoles,
			"approval_status": approval.ApprovalStatus,
			"execution_gate":  approval.ExecutionGate,
			"audit_event":     approval.AuditEvent,
			"check_count":     len(approval.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyPersistSnapshot(ctx context.Context, userID int64, persist AgentRecoveryPolicyPersistResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_persist_snapshot",
		Status:    persist.Status,
		Message:   persist.Summary,
		Metadata: domain.AgentJSON{
			"config_key":         persist.ConfigKey,
			"current_version":    persist.CurrentVersion,
			"pending_version":    persist.PendingVersion,
			"persistence_status": persist.PersistenceStatus,
			"rollback_version":   persist.RollbackVersion,
			"audit_evidence":     persist.AuditEvidence,
			"check_count":        len(persist.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndInteractionLaunchSnapshot(ctx context.Context, userID int64, launch AgentDualEndInteractionLaunchResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_interaction_launch_snapshot",
		Status:    launch.Status,
		Message:   launch.Summary,
		Metadata: domain.AgentJSON{
			"wechat_template_render_status":      launch.WeChatTemplateRenderStatus,
			"web_evidence_route_status":          launch.WebEvidenceRouteStatus,
			"callback_replay_approval_status":    launch.CallbackReplayApprovalStatus,
			"recovery_policy_persistence_status": launch.RecoveryPolicyPersistenceStatus,
			"audit_status":                       launch.AuditStatus,
			"next_action":                        launch.NextAction,
			"check_count":                        len(launch.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatTemplateSendSnapshot(ctx context.Context, userID int64, send AgentWeChatTemplateSendResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_template_send_snapshot",
		Status:    send.Status,
		Message:   send.Summary,
		Metadata: domain.AgentJSON{
			"message_type":  send.MessageType,
			"title":         send.Title,
			"phase_fields":  send.PhaseFields,
			"button_fields": send.ButtonFields,
			"fallback_text": send.FallbackText,
			"send_entry":    send.SendEntry,
			"send_result":   send.SendResult,
			"audit_event":   send.AuditEvent,
			"check_count":   len(send.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceDetailViewSnapshot(ctx context.Context, userID int64, view AgentWebEvidenceDetailViewResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_detail_view_snapshot",
		Status:    view.Status,
		Message:   view.Summary,
		Metadata: domain.AgentJSON{
			"route_name":      view.RouteName,
			"route_path":      view.RoutePath,
			"plan_param":      view.PlanParam,
			"record_param":    view.RecordParam,
			"record_source":   view.RecordSource,
			"filter_params":   view.FilterParams,
			"audit_events":    view.AuditEvents,
			"replay_entry":    view.ReplayEntry,
			"permission_hint": view.PermissionHint,
			"check_count":     len(view.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayExecutionSnapshot(ctx context.Context, userID int64, execution AgentCallbackReplayExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"request_entry":    execution.RequestEntry,
			"execute_entry":    execution.ExecuteEntry,
			"approval_status":  execution.ApprovalStatus,
			"execution_gate":   execution.ExecutionGate,
			"idempotency_key":  execution.IdempotencyKey,
			"audit_event":      execution.AuditEvent,
			"failure_fallback": execution.FailureFallback,
			"check_count":      len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyVersionSnapshot(ctx context.Context, userID int64, version AgentRecoveryPolicyVersionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_version_snapshot",
		Status:    version.Status,
		Message:   version.Summary,
		Metadata: domain.AgentJSON{
			"policy_key":       version.PolicyKey,
			"current_version":  version.CurrentVersion,
			"pending_version":  version.PendingVersion,
			"rollback_version": version.RollbackVersion,
			"release_status":   version.ReleaseStatus,
			"config_source":    version.ConfigSource,
			"audit_event":      version.AuditEvent,
			"check_count":      len(version.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndRealInteractionSnapshot(ctx context.Context, userID int64, interaction AgentDualEndRealInteractionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_real_interaction_snapshot",
		Status:    interaction.Status,
		Message:   interaction.Summary,
		Metadata: domain.AgentJSON{
			"wechat_template_send_status":      interaction.WeChatTemplateSendStatus,
			"web_evidence_detail_status":       interaction.WebEvidenceDetailStatus,
			"callback_replay_execution_status": interaction.CallbackReplayExecutionStatus,
			"recovery_policy_version_status":   interaction.RecoveryPolicyVersionStatus,
			"audit_status":                     interaction.AuditStatus,
			"next_action":                      interaction.NextAction,
			"check_count":                      len(interaction.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatTemplateIntegrationSnapshot(ctx context.Context, userID int64, integration AgentWeChatTemplateIntegrationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_template_integration_snapshot",
		Status:    integration.Status,
		Message:   integration.Summary,
		Metadata: domain.AgentJSON{
			"send_path":           integration.SendPath,
			"template_status":     integration.TemplateStatus,
			"fallback_status":     integration.FallbackStatus,
			"degrade_strategy":    integration.DegradeStrategy,
			"message_id_readback": integration.MessageIDReadback,
			"audit_evidence":      integration.AuditEvidence,
			"check_count":         len(integration.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceInteractionDetailSnapshot(ctx context.Context, userID int64, detail AgentWebEvidenceInteractionDetailResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_interaction_detail_snapshot",
		Status:    detail.Status,
		Message:   detail.Summary,
		Metadata: domain.AgentJSON{
			"filter_mode":          detail.FilterMode,
			"expand_mode":          detail.ExpandMode,
			"audit_timeline":       detail.AuditTimeline,
			"replay_request_entry": detail.ReplayRequestEntry,
			"permission_hint":      detail.PermissionHint,
			"check_count":          len(detail.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplaySafetyAuditSnapshot(ctx context.Context, userID int64, audit AgentCallbackReplaySafetyAuditResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_safety_audit_snapshot",
		Status:    audit.Status,
		Message:   audit.Summary,
		Metadata: domain.AgentJSON{
			"idempotency_check": audit.IdempotencyCheck,
			"approval_check":    audit.ApprovalCheck,
			"signature_check":   audit.SignatureCheck,
			"execution_result":  audit.ExecutionResult,
			"failure_audit":     audit.FailureAudit,
			"check_count":       len(audit.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyGrayReleaseSnapshot(ctx context.Context, userID int64, release AgentRecoveryPolicyGrayReleaseResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_gray_release_snapshot",
		Status:    release.Status,
		Message:   release.Summary,
		Metadata: domain.AgentJSON{
			"gray_stage":         release.GrayStage,
			"release_percent":    release.ReleasePercent,
			"rollback_condition": release.RollbackCondition,
			"approval_status":    release.ApprovalStatus,
			"audit_evidence":     release.AuditEvidence,
			"check_count":        len(release.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndRunLoopSnapshot(ctx context.Context, userID int64, loop AgentDualEndRunLoopResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_run_loop_snapshot",
		Status:    loop.Status,
		Message:   loop.Summary,
		Metadata: domain.AgentJSON{
			"wechat_template_integration_status": loop.WeChatTemplateIntegrationStatus,
			"web_evidence_interaction_status":    loop.WebEvidenceInteractionStatus,
			"callback_replay_safety_status":      loop.CallbackReplaySafetyStatus,
			"recovery_policy_gray_status":        loop.RecoveryPolicyGrayStatus,
			"audit_status":                       loop.AuditStatus,
			"next_action":                        loop.NextAction,
			"check_count":                        len(loop.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatTemplatePilotSnapshot(ctx context.Context, userID int64, pilot AgentWeChatTemplatePilotResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_template_pilot_snapshot",
		Status:    pilot.Status,
		Message:   pilot.Summary,
		Metadata: domain.AgentJSON{
			"pilot_batch":       pilot.PilotBatch,
			"target_scope":      pilot.TargetScope,
			"template_status":   pilot.TemplateStatus,
			"fallback_hit":      pilot.FallbackHit,
			"message_id_status": pilot.MessageIDStatus,
			"audit_evidence":    pilot.AuditEvidence,
			"check_count":       len(pilot.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceUserActionSnapshot(ctx context.Context, userID int64, action AgentWebEvidenceUserActionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_user_action_snapshot",
		Status:    action.Status,
		Message:   action.Summary,
		Metadata: domain.AgentJSON{
			"filter_action":     action.FilterAction,
			"expand_action":     action.ExpandAction,
			"timeline_action":   action.TimelineAction,
			"replay_request":    action.ReplayRequest,
			"permission_result": action.PermissionResult,
			"check_count":       len(action.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayResultTraceSnapshot(ctx context.Context, userID int64, trace AgentCallbackReplayResultTraceResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_result_trace_snapshot",
		Status:    trace.Status,
		Message:   trace.Summary,
		Metadata: domain.AgentJSON{
			"execution_result":  trace.ExecutionResult,
			"idempotency_hit":   trace.IdempotencyHit,
			"approval_decision": trace.ApprovalDecision,
			"signature_result":  trace.SignatureResult,
			"failure_reason":    trace.FailureReason,
			"audit_record":      trace.AuditRecord,
			"check_count":       len(trace.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryPolicyAutomationSnapshot(ctx context.Context, userID int64, automation AgentRecoveryPolicyAutomationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_policy_automation_snapshot",
		Status:    automation.Status,
		Message:   automation.Summary,
		Metadata: domain.AgentJSON{
			"auto_advance":       automation.AutoAdvance,
			"pause_condition":    automation.PauseCondition,
			"rollback_condition": automation.RollbackCondition,
			"current_percent":    automation.CurrentPercent,
			"next_percent":       automation.NextPercent,
			"audit_evidence":     automation.AuditEvidence,
			"check_count":        len(automation.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentDualEndTaskClosureSnapshot(ctx context.Context, userID int64, closure AgentDualEndTaskClosureResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.dual_end_task_closure_snapshot",
		Status:    closure.Status,
		Message:   closure.Summary,
		Metadata: domain.AgentJSON{
			"wechat_pilot_status":          closure.WeChatPilotStatus,
			"web_evidence_action_status":   closure.WebEvidenceActionStatus,
			"callback_replay_trace_status": closure.CallbackReplayTraceStatus,
			"recovery_automation_status":   closure.RecoveryAutomationStatus,
			"audit_status":                 closure.AuditStatus,
			"next_action":                  closure.NextAction,
			"check_count":                  len(closure.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWeChatTemplatePilotMetricSnapshot(ctx context.Context, userID int64, metric AgentWeChatTemplatePilotMetricResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.wechat_template_pilot_metric_snapshot",
		Status:    metric.Status,
		Message:   metric.Summary,
		Metadata: domain.AgentJSON{
			"batch_id":          metric.BatchID,
			"target_user_scope": metric.TargetUserScope,
			"send_status":       metric.SendStatus,
			"fallback_count":    metric.FallbackCount,
			"message_id_status": metric.MessageIDStatus,
			"audit_ref":         metric.AuditRef,
			"check_count":       len(metric.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentWebEvidenceOperationSnapshot(ctx context.Context, userID int64, operation AgentWebEvidenceOperationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.web_evidence_operation_snapshot",
		Status:    operation.Status,
		Message:   operation.Summary,
		Metadata: domain.AgentJSON{
			"filter_entry":         operation.FilterEntry,
			"expand_entry":         operation.ExpandEntry,
			"timeline_entry":       operation.TimelineEntry,
			"replay_request_entry": operation.ReplayRequestEntry,
			"permission_gate":      operation.PermissionGate,
			"audit_event":          operation.AuditEvent,
			"operation_count":      operation.OperationCount,
			"check_count":          len(operation.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentCallbackReplayResultQuerySnapshot(ctx context.Context, userID int64, query AgentCallbackReplayResultQueryResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.callback_replay_result_query_snapshot",
		Status:    query.Status,
		Message:   query.Summary,
		Metadata: domain.AgentJSON{
			"query_entry":        query.QueryEntry,
			"execution_result":   query.ExecutionResult,
			"idempotency_result": query.IdempotencyResult,
			"approval_decision":  query.ApprovalDecision,
			"signature_result":   query.SignatureResult,
			"failure_reason":     query.FailureReason,
			"audit_ref":          query.AuditRef,
			"check_count":        len(query.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRecoveryAutomationExecutionSnapshot(ctx context.Context, userID int64, execution AgentRecoveryAutomationExecutionResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.recovery_automation_execution_snapshot",
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"execution_mode":   execution.ExecutionMode,
			"current_percent":  execution.CurrentPercent,
			"next_percent":     execution.NextPercent,
			"advance_decision": execution.AdvanceDecision,
			"pause_gate":       execution.PauseGate,
			"rollback_gate":    execution.RollbackGate,
			"approval_gate":    execution.ApprovalGate,
			"audit_ref":        execution.AuditRef,
			"check_count":      len(execution.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) recordAgentRealInteractionAutomationSnapshot(ctx context.Context, userID int64, automation AgentRealInteractionAutomationResponse) {
	if s == nil || s.repository == nil || userID < 1 {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    userID,
		EventType: "agent.real_interaction_automation_snapshot",
		Status:    automation.Status,
		Message:   automation.Summary,
		Metadata: domain.AgentJSON{
			"pilot_metric_status":       automation.PilotMetricStatus,
			"evidence_operation_status": automation.EvidenceOperationStatus,
			"replay_query_status":       automation.ReplayQueryStatus,
			"recovery_execution_status": automation.RecoveryExecutionStatus,
			"audit_status":              automation.AuditStatus,
			"next_action":               automation.NextAction,
			"check_count":               len(automation.Checks),
		},
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentSessionService) RequestCallbackReplayApproval(ctx context.Context, auth CurrentAuth, input AgentCallbackReplayInput) (AgentCallbackReplayAPIResult, error) {
	if s == nil || s.repository == nil {
		return AgentCallbackReplayAPIResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_callback_replay_unavailable", "agent callback replay service is unavailable", "service.agent_session.callback_replay.request", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentCallbackReplayAPIResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	callbackKey := strings.TrimSpace(input.CallbackKey)
	if callbackKey == "" {
		callbackKey = "wechat_approval_callback"
	}
	replayEntry := strings.TrimSpace(input.ReplayEntry)
	if replayEntry == "" {
		replayEntry = "web.agent.callback.replay." + callbackKey
	}
	execution := AgentCallbackReplayExecutionResponse{
		Status:          "approval_required",
		Summary:         fmt.Sprintf("callback replay request for %s requires approval", callbackKey),
		RequestEntry:    "/api/v1/agent/callback-replay/requests",
		ExecuteEntry:    "/api/v1/agent/callback-replay/execute",
		ApprovalStatus:  "approval_required",
		ExecutionGate:   "blocked_until_approved",
		IdempotencyKey:  callbackReplayIdempotencyKey(auth.User.ID, input.PlanID, callbackKey, replayEntry),
		AuditEvent:      "agent.callback_replay_requested",
		FailureFallback: "manual_review_without_replay",
	}
	execution.Checks = callbackReplayExecutionChecks(execution)
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    auth.User.ID,
		EventType: execution.AuditEvent,
		Status:    execution.Status,
		Message:   execution.Summary,
		Metadata: domain.AgentJSON{
			"plan_id":         input.PlanID,
			"callback_key":    callbackKey,
			"replay_entry":    replayEntry,
			"reason":          strings.TrimSpace(input.Reason),
			"idempotency_key": execution.IdempotencyKey,
			"execution_gate":  execution.ExecutionGate,
		},
		CreatedAt: s.now().UTC(),
	})
	return AgentCallbackReplayAPIResult{ReplayExecution: execution, AuditEvent: execution.AuditEvent}, nil
}

func (s *AgentSessionService) ExecuteCallbackReplay(ctx context.Context, auth CurrentAuth, input AgentCallbackReplayInput) (AgentCallbackReplayAPIResult, error) {
	if s == nil || s.repository == nil {
		return AgentCallbackReplayAPIResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_callback_replay_unavailable", "agent callback replay service is unavailable", "service.agent_session.callback_replay.execute", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentCallbackReplayAPIResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	callbackKey := strings.TrimSpace(input.CallbackKey)
	if callbackKey == "" {
		callbackKey = "wechat_approval_callback"
	}
	replayEntry := strings.TrimSpace(input.ReplayEntry)
	if replayEntry == "" {
		replayEntry = "web.agent.callback.replay." + callbackKey
	}
	status := "blocked"
	approvalStatus := "approval_required"
	executionGate := "blocked_until_approved"
	auditEvent := "agent.callback_replay_execute_blocked"
	summary := fmt.Sprintf("callback replay execution for %s is blocked until approval", callbackKey)
	if input.Approved {
		status = "ready"
		approvalStatus = "approved"
		executionGate = "approved_and_idempotency_verified"
		auditEvent = "agent.callback_replay_execute_requested"
		summary = fmt.Sprintf("callback replay execution for %s passed execution gate", callbackKey)
	}
	execution := AgentCallbackReplayExecutionResponse{
		Status:          status,
		Summary:         summary,
		RequestEntry:    "/api/v1/agent/callback-replay/requests",
		ExecuteEntry:    "/api/v1/agent/callback-replay/execute",
		ApprovalStatus:  approvalStatus,
		ExecutionGate:   executionGate,
		IdempotencyKey:  callbackReplayIdempotencyKey(auth.User.ID, input.PlanID, callbackKey, replayEntry),
		AuditEvent:      auditEvent,
		FailureFallback: "manual_review_without_replay",
	}
	execution.Checks = callbackReplayExecutionChecks(execution)
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		UserID:    auth.User.ID,
		EventType: auditEvent,
		Status:    status,
		Message:   summary,
		Metadata: domain.AgentJSON{
			"plan_id":         input.PlanID,
			"callback_key":    callbackKey,
			"replay_entry":    replayEntry,
			"approved":        input.Approved,
			"idempotency_key": execution.IdempotencyKey,
			"execution_gate":  execution.ExecutionGate,
			"fallback":        execution.FailureFallback,
		},
		CreatedAt: s.now().UTC(),
	})
	return AgentCallbackReplayAPIResult{ReplayExecution: execution, AuditEvent: auditEvent}, nil
}

func callbackReplayIdempotencyKey(userID int64, planID int64, callbackKey string, replayEntry string) string {
	return fmt.Sprintf("agent_callback_replay:%d:%d:%s:%s", userID, planID, strings.TrimSpace(callbackKey), strings.TrimSpace(replayEntry))
}

func callbackReplayExecutionChecks(execution AgentCallbackReplayExecutionResponse) []AgentDeploymentCheckResponse {
	return []AgentDeploymentCheckResponse{
		{Key: "request_entry", Status: readyIf(execution.RequestEntry != ""), Summary: execution.RequestEntry},
		{Key: "execute_entry", Status: readyIf(execution.ExecuteEntry != ""), Summary: execution.ExecuteEntry},
		{Key: "approval_status", Status: readyIf(execution.ApprovalStatus != ""), Summary: execution.ApprovalStatus},
		{Key: "execution_gate", Status: readyIf(execution.ExecutionGate != ""), Summary: execution.ExecutionGate},
		{Key: "idempotency_key", Status: readyIf(execution.IdempotencyKey != ""), Summary: execution.IdempotencyKey},
		{Key: "audit_event", Status: readyIf(execution.AuditEvent != ""), Summary: execution.AuditEvent},
		{Key: "failure_fallback", Status: readyIf(execution.FailureFallback != ""), Summary: execution.FailureFallback},
	}
}

func (s *AgentSessionService) CancelScheduledTask(ctx context.Context, auth CurrentAuth, taskID int64) (CancelAgentScheduledTaskResult, error) {
	if s == nil || s.repository == nil {
		return CancelAgentScheduledTaskResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_tasks_unavailable", "agent task service is unavailable", "service.agent_session.cancel_scheduled_task", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 || taskID < 1 {
		return CancelAgentScheduledTaskResult{}, fmt.Errorf("%w: authenticated user and task id are required", domain.ErrInvalidInput)
	}
	task, err := s.repository.GetAgentScheduledTask(ctx, auth.User.ID, taskID)
	if err != nil {
		return CancelAgentScheduledTaskResult{}, err
	}
	if task.Status != domain.AgentScheduledTaskStatusQueued && task.Status != domain.AgentScheduledTaskStatusRunning && task.Status != domain.AgentScheduledTaskStatusInputRequired {
		return CancelAgentScheduledTaskResult{}, domain.NewAppError(domain.ErrorKindConflict, "agent_scheduled_task_not_cancelable", "scheduled task is not cancelable", "service.agent_session.cancel_scheduled_task", false, nil)
	}
	now := s.now().UTC()
	task.Status = domain.AgentScheduledTaskStatusCanceled
	task.LastError = ""
	task.NextRunAt = nil
	task.CompletedAt = &now
	task.UpdatedAt = now
	task, err = s.repository.UpdateAgentScheduledTask(ctx, task)
	if err != nil {
		return CancelAgentScheduledTaskResult{}, err
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: task.SessionID,
		TurnID:    task.TurnID,
		UserID:    task.UserID,
		EventType: "agent.scheduled_task_canceled",
		Status:    "canceled",
		Message:   "agent scheduled task canceled by user",
		Metadata: domain.AgentJSON{
			"scheduled_task_id": task.ID,
			"plan_id":           task.PlanID,
		},
		CreatedAt: now,
	})
	return CancelAgentScheduledTaskResult{Task: agentScheduledTaskResponse(task)}, nil
}

func (s *AgentSessionService) GetProgress(ctx context.Context, auth CurrentAuth, query AgentProgressQuery) (AgentProgressResult, error) {
	if s == nil || s.repository == nil {
		return AgentProgressResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_progress_unavailable", "agent progress service is unavailable", "service.agent_session.progress", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return AgentProgressResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	if query.PlanID < 1 && query.TurnID < 1 && query.RunID < 1 && query.ScheduledTaskID < 1 {
		return AgentProgressResult{}, fmt.Errorf("%w: progress query id is required", domain.ErrInvalidInput)
	}

	var plan domain.AgentPlan
	var planLoaded bool
	var scheduledTasks []domain.AgentScheduledTask
	if query.ScheduledTaskID > 0 {
		task, err := s.repository.GetAgentScheduledTask(ctx, auth.User.ID, query.ScheduledTaskID)
		if err != nil {
			return AgentProgressResult{}, err
		}
		scheduledTasks = append(scheduledTasks, task)
		if query.PlanID < 1 {
			query.PlanID = task.PlanID
		}
		if query.TurnID < 1 {
			query.TurnID = task.TurnID
		}
		if query.RunID < 1 {
			query.RunID = task.SourceRunID
		}
	}
	if query.PlanID > 0 {
		loaded, err := s.repository.GetAgentPlan(ctx, auth.User.ID, query.PlanID)
		if err != nil {
			return AgentProgressResult{}, err
		}
		plan = loaded
		planLoaded = true
		if query.TurnID < 1 {
			query.TurnID = plan.TurnID
		}
		if query.RunID < 1 {
			query.RunID = plan.ControllerRunID
		}
	}
	if query.RunID > 0 && query.TurnID < 1 {
		run, err := s.repository.GetAgentRunDetail(ctx, auth.User.ID, query.RunID)
		if err != nil {
			return AgentProgressResult{}, err
		}
		query.TurnID = run.TurnID
	}
	if !planLoaded && query.TurnID > 0 {
		plans, err := s.repository.ListAgentPlans(ctx, auth.User.ID, 0, query.TurnID, 10)
		if err != nil {
			return AgentProgressResult{}, err
		}
		if len(plans) > 0 {
			plan = plans[0]
			planLoaded = true
			if query.PlanID < 1 {
				query.PlanID = plan.ID
			}
		}
	}

	if query.TurnID < 1 && !planLoaded && len(scheduledTasks) == 0 {
		return AgentProgressResult{}, domain.ErrNotFound
	}

	runs := make([]domain.AgentRun, 0)
	if query.TurnID > 0 {
		list, err := s.repository.ListAgentRunsByTurn(ctx, auth.User.ID, query.TurnID)
		if err != nil {
			return AgentProgressResult{}, err
		}
		for _, run := range list {
			detail, err := s.repository.GetAgentRunDetail(ctx, auth.User.ID, run.ID)
			if err != nil {
				return AgentProgressResult{}, err
			}
			runs = append(runs, detail)
		}
	} else if query.RunID > 0 {
		run, err := s.repository.GetAgentRunDetail(ctx, auth.User.ID, query.RunID)
		if err != nil {
			return AgentProgressResult{}, err
		}
		runs = append(runs, run)
	}
	if query.ScheduledTaskID < 1 {
		tasks, err := s.repository.ListAgentScheduledTasksByRefs(ctx, auth.User.ID, query.PlanID, query.TurnID, query.RunID, 50)
		if err != nil {
			return AgentProgressResult{}, err
		}
		scheduledTasks = append(scheduledTasks, tasks...)
	}
	auditLogs, err := s.repository.ListAuditLogsByRefs(ctx, domain.AgentAuditLogListOptions{
		UserID:    auth.User.ID,
		SessionID: plan.SessionID,
		TurnID:    query.TurnID,
		Limit:     50,
	})
	if err != nil {
		return AgentProgressResult{}, err
	}

	progress := s.buildProgressSnapshot(plan, planLoaded, runs, scheduledTasks, auditLogs, query)
	return AgentProgressResult{Progress: progress}, nil
}

func manualAgentSessionKey(account domain.ExternalAccount) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%s:%s:%s:manual:%d", account.CorpID, account.AgentID, account.ExternalUserID, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%s:%s:manual:%s", account.CorpID, account.AgentID, account.ExternalUserID, hex.EncodeToString(random[:]))
}

func agentExternalAccountResponse(account domain.ExternalAccount) AgentExternalAccountResponse {
	return AgentExternalAccountResponse{
		ID:                   account.ID,
		Provider:             account.Provider,
		CorpID:               account.CorpID,
		AgentID:              account.AgentID,
		ExternalUserID:       account.ExternalUserID,
		DisplayName:          account.DisplayName,
		BindingStatus:        string(account.BindingStatus),
		ActiveAgentSessionID: account.ActiveAgentSessionID,
		UpdatedAt:            formatOptionalTime(&account.UpdatedAt),
	}
}

func agentSessionResponse(session domain.AgentSession, stats domain.AgentSessionStats, active bool) AgentSessionResponse {
	return AgentSessionResponse{
		ID:                       session.ID,
		ExternalAccountID:        session.ExternalAccountID,
		Provider:                 session.Provider,
		ChannelSessionKey:        session.ChannelSessionKey,
		Status:                   string(session.Status),
		Title:                    session.Title,
		ActiveForAccount:         active,
		ContextInitializedAt:     formatOptionalTime(session.ContextInitializedAt),
		ContextRebuildStartedAt:  formatOptionalTime(session.ContextRebuildStartedAt),
		ContextRebuildFinishedAt: formatOptionalTime(session.ContextRebuildFinishedAt),
		ContextVersion:           session.ContextVersion,
		TranscriptCountIndexed:   session.TranscriptCountIndexed,
		Stats:                    agentSessionStats(stats),
		StartedAt:                formatOptionalTime(&session.StartedAt),
		LastActiveAt:             formatOptionalTime(&session.LastActiveAt),
		CreatedAt:                formatOptionalTime(&session.CreatedAt),
		UpdatedAt:                formatOptionalTime(&session.UpdatedAt),
	}
}

func agentSessionStats(stats domain.AgentSessionStats) AgentSessionStats {
	return AgentSessionStats{
		TranscriptCount:   stats.TranscriptCount,
		ArchiveIndexCount: stats.ArchiveIndexCount,
		RecallCount:       stats.RecallCount,
		FirstTranscriptAt: formatOptionalTime(stats.FirstTranscriptAt),
		LastTranscriptAt:  formatOptionalTime(stats.LastTranscriptAt),
	}
}

func agentRunResponse(run domain.AgentRun, includeDetail bool) AgentRunResponse {
	response := AgentRunResponse{
		ID:              run.ID,
		ParentRunID:     run.ParentRunID,
		SessionID:       run.SessionID,
		TurnID:          run.TurnID,
		Role:            string(run.Role),
		Status:          string(run.Status),
		TaskPacket:      run.TaskPacket,
		CapabilityScope: append([]string(nil), run.CapabilityScope...),
		ModelKey:        run.ModelKey,
		ContextBudget:   run.ContextBudget,
		ContextTraceRef: run.ContextTraceRef,
		ResultRef:       run.ResultRef,
		ErrorMessage:    run.ErrorMessage,
		TraceID:         run.TraceID,
		StartedAt:       formatOptionalTime(&run.StartedAt),
		CompletedAt:     formatOptionalTime(run.CompletedAt),
		CreatedAt:       formatOptionalTime(&run.CreatedAt),
		UpdatedAt:       formatOptionalTime(&run.UpdatedAt),
	}
	if !includeDetail {
		return response
	}
	for _, trace := range run.ContextTraces {
		response.ContextTraces = append(response.ContextTraces, AgentRunContextTraceResponse{
			ID:              trace.ID,
			RunID:           trace.RunID,
			TraceKind:       trace.TraceKind,
			PromptVersion:   trace.PromptVersion,
			ModelKey:        trace.ModelKey,
			Content:         trace.Content,
			ContentHash:     trace.ContentHash,
			RedactionStatus: trace.RedactionStatus,
			TokenEstimate:   trace.TokenEstimate,
			CreatedAt:       formatOptionalTime(&trace.CreatedAt),
		})
	}
	for _, observation := range run.Observations {
		response.Observations = append(response.Observations, AgentObservationResponse{
			ID:            observation.ID,
			RunID:         observation.RunID,
			CapabilityKey: observation.CapabilityKey,
			InputSummary:  observation.InputSummary,
			OutputSummary: observation.OutputSummary,
			Status:        observation.Status,
			Error:         observation.Error,
			ArtifactRefs:  append([]string(nil), observation.ArtifactRefs...),
			CreatedAt:     formatOptionalTime(&observation.CreatedAt),
		})
	}
	for _, artifact := range run.Artifacts {
		response.Artifacts = append(response.Artifacts, AgentArtifactResponse{
			ID:           artifact.ID,
			RunID:        artifact.RunID,
			ArtifactType: artifact.ArtifactType,
			ContentRef:   artifact.ContentRef,
			Summary:      artifact.Summary,
			SourceRefs:   append([]string(nil), artifact.SourceRefs...),
			ContentHash:  artifact.ContentHash,
			CreatedAt:    formatOptionalTime(&artifact.CreatedAt),
		})
	}
	for _, child := range run.ChildRuns {
		response.ChildRuns = append(response.ChildRuns, agentRunResponse(child, false))
	}
	return response
}

func agentPlanResponse(plan domain.AgentPlan, includeDetail bool) AgentPlanResponse {
	response := AgentPlanResponse{
		ID:                 plan.ID,
		UserID:             plan.UserID,
		SessionID:          plan.SessionID,
		TurnID:             plan.TurnID,
		ControllerRunID:    plan.ControllerRunID,
		Status:             string(plan.Status),
		Goal:               plan.Goal,
		Summary:            plan.Summary,
		ImpactSummary:      plan.ImpactSummary,
		RiskLevel:          plan.RiskLevel,
		ConfirmationPolicy: plan.ConfirmationPolicy,
		AllowedScopes:      append([]string(nil), plan.AllowedScopes...),
		DedupeKey:          plan.DedupeKey,
		PolicyDecision:     plan.PolicyDecision,
		PolicyReason:       plan.PolicyReason,
		ExpiresAt:          formatOptionalTime(plan.ExpiresAt),
		ApprovedAt:         formatOptionalTime(plan.ApprovedAt),
		RejectedAt:         formatOptionalTime(plan.RejectedAt),
		CompletedAt:        formatOptionalTime(plan.CompletedAt),
		FailedAt:           formatOptionalTime(plan.FailedAt),
		ErrorMessage:       plan.ErrorMessage,
		Metadata:           cloneApprovalMetadata(plan.Metadata),
		CreatedAt:          formatOptionalTime(&plan.CreatedAt),
		UpdatedAt:          formatOptionalTime(&plan.UpdatedAt),
	}
	if !includeDetail {
		return response
	}
	for _, step := range plan.Steps {
		response.Steps = append(response.Steps, AgentPlanStepResponse{
			ID:              step.ID,
			PlanID:          step.PlanID,
			StepOrder:       step.StepOrder,
			Status:          string(step.Status),
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
			LastRetryAt:     formatOptionalTime(step.LastRetryAt),
			RetryReason:     step.RetryReason,
			RetryMetadata:   cloneApprovalMetadata(step.RetryMetadata),
			StartedAt:       formatOptionalTime(step.StartedAt),
			CompletedAt:     formatOptionalTime(step.CompletedAt),
			CreatedAt:       formatOptionalTime(&step.CreatedAt),
			UpdatedAt:       formatOptionalTime(&step.UpdatedAt),
		})
	}
	for _, approval := range plan.Approvals {
		response.Approvals = append(response.Approvals, AgentPlanApprovalResponse{
			ID:        approval.ID,
			PlanID:    approval.PlanID,
			Channel:   approval.Channel,
			Status:    string(approval.Status),
			ExpiresAt: formatOptionalTime(&approval.ExpiresAt),
			DecidedAt: formatOptionalTime(approval.DecidedAt),
			Metadata:  cloneApprovalMetadata(approval.Metadata),
			CreatedAt: formatOptionalTime(&approval.CreatedAt),
			UpdatedAt: formatOptionalTime(&approval.UpdatedAt),
		})
	}
	return response
}

func agentTaskSummaryFromPlan(plan domain.AgentPlan) AgentTaskSummaryResponse {
	permissionStatus := planPermissionStatus(plan)
	budgetStatus := planBudgetStatus(plan)
	return AgentTaskSummaryResponse{
		ID:               fmt.Sprintf("plan:%d", plan.ID),
		Kind:             "plan",
		SessionID:        plan.SessionID,
		TurnID:           plan.TurnID,
		PlanID:           plan.ID,
		Status:           string(plan.Status),
		Goal:             plan.Goal,
		Summary:          plan.Summary,
		PermissionStatus: permissionStatus,
		BudgetStatus:     budgetStatus,
		QualityStatus:    metadataString(metadataMap(plan.Metadata, "result_quality"), "status"),
		HandoffStatus:    metadataString(metadataMap(plan.Metadata, "handoff"), "status"),
		Observability:    planRuntimeObservabilitySummary(plan),
		LatestProgress:   planLatestProgress(plan),
		NextAction:       agentProgressNextAction(string(plan.Status), true, plan, nil),
		ProgressURL:      fmt.Sprintf("/agent/plans/%d", plan.ID),
		UpdatedAt:        formatOptionalTime(&plan.UpdatedAt),
	}
}

func agentTaskSummaryFromScheduledTask(task domain.AgentScheduledTask) AgentTaskSummaryResponse {
	progressURL := fmt.Sprintf("/agent?scheduled_task_id=%d", task.ID)
	if task.PlanID > 0 {
		progressURL = fmt.Sprintf("/agent/plans/%d", task.PlanID)
	}
	return AgentTaskSummaryResponse{
		ID:               fmt.Sprintf("scheduled_task:%d", task.ID),
		Kind:             "scheduled_task",
		SessionID:        task.SessionID,
		TurnID:           task.TurnID,
		PlanID:           task.PlanID,
		ScheduledTaskID:  task.ID,
		Status:           string(task.Status),
		Goal:             task.Goal,
		Summary:          strings.TrimSpace(task.TaskType + " " + task.TargetChannel),
		PermissionStatus: "scheduled_task",
		BudgetStatus:     scheduledTaskBudgetStatus(task),
		QualityStatus:    "not_applicable",
		HandoffStatus:    scheduledTaskHandoffStatus(task),
		Observability:    scheduledTaskObservabilitySummary(task),
		LatestProgress:   agentProgressFirstNonEmpty(task.LastError, task.FreshnessPolicy, "等待调度"),
		NextAction:       agentProgressNextAction(string(task.Status), false, domain.AgentPlan{}, []domain.AgentScheduledTask{task}),
		ProgressURL:      progressURL,
		UpdatedAt:        formatOptionalTime(&task.UpdatedAt),
	}
}

func planPermissionStatus(plan domain.AgentPlan) string {
	if plan.PolicyDecision != "" {
		return plan.PolicyDecision
	}
	governance := metadataMap(plan.Metadata, "permission_governance")
	if metadataBool(governance, "requires_confirmation") {
		return "prompt"
	}
	if metadataBool(governance, "has_state_change") {
		return "prompt"
	}
	return "allow"
}

func planBudgetStatus(plan domain.AgentPlan) string {
	governance := metadataMap(plan.Metadata, "budget_governance")
	status := metadataString(governance, "status")
	if status == "" {
		return "unknown"
	}
	return status
}

func planPermissionSummary(plan domain.AgentPlan) string {
	governance := metadataMap(plan.Metadata, "permission_governance")
	parts := make([]string, 0, 4)
	if metadataBool(governance, "has_external_access") {
		parts = append(parts, "包含外部只读访问")
	}
	if metadataBool(governance, "has_state_change") {
		parts = append(parts, "包含状态变更能力")
	}
	if metadataBool(governance, "requires_confirmation") {
		parts = append(parts, "需要用户确认")
	}
	if len(parts) == 0 {
		parts = append(parts, "只读本地能力可直接执行")
	}
	return strings.Join(parts, "；")
}

func planBudgetSummary(plan domain.AgentPlan) string {
	governance := metadataMap(plan.Metadata, "budget_governance")
	if governance == nil {
		return "暂无预算摘要"
	}
	toolCalls := metadataNumber(governance, "tool_calls")
	toolBudget := metadataNumber(governance, "tool_call_budget")
	externalCalls := metadataNumber(governance, "external_calls")
	externalBudget := metadataNumber(governance, "external_call_budget")
	degradation := metadataString(governance, "degradation_strategy")
	summary := fmt.Sprintf("工具 %d/%d，联网 %d/%d", toolCalls, toolBudget, externalCalls, externalBudget)
	if degradation != "" {
		summary += "；降级：" + degradation
	}
	return summary
}

func scheduledTaskBudgetStatus(task domain.AgentScheduledTask) string {
	if strings.TrimSpace(task.LastError) != "" {
		return "failed"
	}
	if task.AttemptCount >= task.MaxAttempts && task.MaxAttempts > 0 {
		return "exhausted"
	}
	return "within_budget"
}

func scheduledTaskHandoffStatus(task domain.AgentScheduledTask) string {
	if task.Status == domain.AgentScheduledTaskStatusFailed || task.Status == domain.AgentScheduledTaskStatusInputRequired {
		return "required"
	}
	return "not_required"
}

func scheduledTaskObservabilitySummary(task domain.AgentScheduledTask) string {
	if strings.TrimSpace(task.LastError) != "" {
		return "最近失败：" + strings.TrimSpace(task.LastError)
	}
	return fmt.Sprintf("状态 %s，尝试 %d/%d", task.Status, task.AttemptCount, task.MaxAttempts)
}

func planLatestProgress(plan domain.AgentPlan) string {
	for _, step := range plan.Steps {
		if step.Status == domain.AgentPlanStepStatusFailed {
			return agentProgressFirstNonEmpty(step.ErrorMessage, step.OutputSummary, step.Title)
		}
	}
	for index := len(plan.Steps) - 1; index >= 0; index-- {
		step := plan.Steps[index]
		if step.OutputSummary != "" || step.InputSummary != "" {
			return agentProgressFirstNonEmpty(step.OutputSummary, step.InputSummary, step.Title)
		}
	}
	return agentProgressFirstNonEmpty(plan.ErrorMessage, plan.Summary, plan.Goal)
}

func metadataMap(metadata domain.AgentJSON, key string) map[string]any {
	raw := metadata[key]
	if typed, ok := raw.(map[string]any); ok {
		return typed
	}
	if typed, ok := raw.(domain.AgentJSON); ok {
		return map[string]any(typed)
	}
	return nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	value, _ := metadata[key].(bool)
	return value
}

func metadataNumber(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func (s *AgentSessionService) buildProgressSnapshot(plan domain.AgentPlan, planLoaded bool, runs []domain.AgentRun, scheduledTasks []domain.AgentScheduledTask, auditLogs []domain.AgentAuditLog, query AgentProgressQuery) AgentProgressSnapshot {
	now := s.now().UTC()
	status := "unknown"
	summary := "暂无可用进度"
	subjectType := "turn"
	subjectID := query.TurnID
	var planResponse *AgentPlanResponse
	if planLoaded {
		response := agentPlanResponse(plan, true)
		planResponse = &response
		status = string(plan.Status)
		summary = plan.Summary
		subjectType = "plan"
		subjectID = plan.ID
	}
	if !planLoaded && len(scheduledTasks) > 0 {
		status = string(scheduledTasks[0].Status)
		summary = scheduledTasks[0].Goal
		subjectType = "scheduled_task"
		subjectID = scheduledTasks[0].ID
	}
	if !planLoaded && len(scheduledTasks) == 0 && len(runs) > 0 {
		status = string(runs[0].Status)
		summary = runs[0].ResultRef
		subjectType = "run"
		subjectID = runs[0].ID
	}

	runResponses := make([]AgentRunResponse, 0, len(runs))
	for _, run := range runs {
		runResponses = append(runResponses, agentRunResponse(run, true))
	}
	taskResponses := make([]AgentScheduledTaskResponse, 0, len(scheduledTasks))
	for _, task := range scheduledTasks {
		taskResponses = append(taskResponses, agentScheduledTaskResponse(task))
	}

	phases := buildAgentProgressPhases(plan, planLoaded, runs, scheduledTasks)
	events := buildAgentProgressEvents(plan, planLoaded, runs, scheduledTasks, auditLogs)
	updatedAt := latestProgressUpdatedAt(plan, planLoaded, runs, scheduledTasks, events, now)
	eventCursor := agentProgressEventCursor(events, updatedAt)
	return AgentProgressSnapshot{
		SubjectType:    subjectType,
		SubjectID:      subjectID,
		Status:         status,
		Summary:        summary,
		NextAction:     agentProgressNextAction(status, planLoaded, plan, scheduledTasks),
		Version:        agentProgressVersion(updatedAt, events),
		EventCursor:    eventCursor,
		UpdatedAt:      formatOptionalTime(&updatedAt),
		RefreshedAt:    formatOptionalTime(&now),
		Plan:           planResponse,
		Runs:           runResponses,
		ScheduledTasks: taskResponses,
		Phases:         phases,
		RecentEvents:   events,
	}
}

func buildAgentProgressPhases(plan domain.AgentPlan, planLoaded bool, runs []domain.AgentRun, scheduledTasks []domain.AgentScheduledTask) []AgentProgressPhaseResponse {
	phases := []AgentProgressPhaseResponse{
		{Key: "input", Title: "入口消息", Status: "completed", Summary: "session / turn 已归档"},
	}
	if planLoaded {
		phases = append(phases, AgentProgressPhaseResponse{
			Key:       "plan",
			Title:     "结构化计划",
			Status:    string(plan.Status),
			Summary:   plan.Summary,
			UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
		})
		phases = append(phases, AgentProgressPhaseResponse{
			Key:       "permission",
			Title:     "权限策略",
			Status:    planPermissionStatus(plan),
			Summary:   planPermissionSummary(plan),
			UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
		})
		phases = append(phases, AgentProgressPhaseResponse{
			Key:       "budget",
			Title:     "预算治理",
			Status:    planBudgetStatus(plan),
			Summary:   planBudgetSummary(plan),
			UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
		})
		if metadataMap(plan.Metadata, "recovery") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "recovery",
				Title:     "恢复策略",
				Status:    metadataString(metadataMap(plan.Metadata, "recovery"), "recovery_result"),
				Summary:   planRecoverySummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		if metadataMap(plan.Metadata, "result_quality") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "quality",
				Title:     "结果质量",
				Status:    metadataString(metadataMap(plan.Metadata, "result_quality"), "status"),
				Summary:   planResultQualitySummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		if metadataMap(plan.Metadata, "cost_summary") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "cost",
				Title:     "成本摘要",
				Status:    "recorded",
				Summary:   planCostSummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		if metadataMap(plan.Metadata, "deployment_acceptance") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "deployment_acceptance",
				Title:     "部署验收",
				Status:    metadataString(metadataMap(plan.Metadata, "deployment_acceptance"), "status"),
				Summary:   planDeploymentAcceptanceSummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "cluster_consistency",
				Title:     "多节点一致性",
				Status:    metadataString(metadataMap(plan.Metadata, "deployment_acceptance"), "status"),
				Summary:   planClusterConsistencySummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		if metadataMap(plan.Metadata, "runtime_observability") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "runtime_observability",
				Title:     "运行观测",
				Status:    metadataString(metadataMap(plan.Metadata, "runtime_observability"), "status"),
				Summary:   planRuntimeObservabilitySummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		if metadataMap(plan.Metadata, "handoff") != nil {
			phases = append(phases, AgentProgressPhaseResponse{
				Key:       "handoff",
				Title:     "人工接管",
				Status:    metadataString(metadataMap(plan.Metadata, "handoff"), "status"),
				Summary:   planHandoffSummary(plan),
				UpdatedAt: formatOptionalTime(&plan.UpdatedAt),
			})
		}
		approvalStatus := "not_required"
		approvalSummary := "无需额外确认"
		approvalUpdatedAt := ""
		if len(plan.Approvals) > 0 {
			approval := plan.Approvals[0]
			approvalStatus = string(approval.Status)
			approvalSummary = approval.Channel
			approvalUpdatedAt = formatOptionalTime(&approval.UpdatedAt)
		} else if plan.ConfirmationPolicy == "prompt" || plan.Status == domain.AgentPlanStatusAwaitingApproval {
			approvalStatus = "pending"
			approvalSummary = "等待用户确认"
		}
		phases = append(phases, AgentProgressPhaseResponse{Key: "approval", Title: "确认审批", Status: approvalStatus, Summary: approvalSummary, UpdatedAt: approvalUpdatedAt})
	}
	phases = append(phases, AgentProgressPhaseResponse{
		Key:       "execution",
		Title:     "运行执行",
		Status:    aggregateRunStatus(runs),
		Summary:   fmt.Sprintf("%d 个 run，%d 条 observation", len(runs), countAgentObservations(runs)),
		UpdatedAt: latestRunUpdatedAt(runs),
	})
	if len(scheduledTasks) > 0 {
		phases = append(phases, AgentProgressPhaseResponse{
			Key:       "schedule",
			Title:     "定时任务",
			Status:    aggregateScheduledTaskStatus(scheduledTasks),
			Summary:   fmt.Sprintf("%d 个调度任务", len(scheduledTasks)),
			UpdatedAt: latestScheduledTaskUpdatedAt(scheduledTasks),
		})
	}
	phases = append(phases, AgentProgressPhaseResponse{
		Key:     "response",
		Title:   "用户响应",
		Status:  aggregateResponseStatus(plan, planLoaded, runs, scheduledTasks),
		Summary: "企微或 Web 可基于当前快照展示结果",
	})
	return phases
}

type agentProgressEvent struct {
	response AgentProgressEventResponse
	at       time.Time
}

func buildAgentProgressEvents(plan domain.AgentPlan, planLoaded bool, runs []domain.AgentRun, scheduledTasks []domain.AgentScheduledTask, auditLogs []domain.AgentAuditLog) []AgentProgressEventResponse {
	events := make([]agentProgressEvent, 0)
	if planLoaded {
		events = append(events, agentProgressEvent{
			response: AgentProgressEventResponse{ID: fmt.Sprintf("plan:%d", plan.ID), Kind: "plan", Title: "计划状态", Status: string(plan.Status), Summary: plan.Summary, CreatedAt: formatOptionalTime(&plan.UpdatedAt)},
			at:       plan.UpdatedAt,
		})
		for _, step := range plan.Steps {
			events = append(events, agentProgressEvent{
				response: AgentProgressEventResponse{ID: fmt.Sprintf("plan_step:%d", step.ID), Kind: "plan_step", Title: step.Title, Status: string(step.Status), Summary: agentProgressFirstNonEmpty(step.OutputSummary, step.InputSummary, step.CapabilityKey), CreatedAt: formatOptionalTime(&step.UpdatedAt)},
				at:       step.UpdatedAt,
			})
		}
		for _, approval := range plan.Approvals {
			events = append(events, agentProgressEvent{
				response: AgentProgressEventResponse{ID: fmt.Sprintf("approval:%d", approval.ID), Kind: "approval", Title: approval.Channel, Status: string(approval.Status), Summary: "用户确认记录", CreatedAt: formatOptionalTime(&approval.UpdatedAt)},
				at:       approval.UpdatedAt,
			})
		}
	}
	for _, run := range runs {
		events = append(events, agentProgressEvent{
			response: AgentProgressEventResponse{ID: fmt.Sprintf("run:%d", run.ID), Kind: "run", Title: string(run.Role), Status: string(run.Status), Summary: agentProgressFirstNonEmpty(run.ErrorMessage, run.ResultRef, run.ContextTraceRef, run.ModelKey), CreatedAt: formatOptionalTime(&run.UpdatedAt)},
			at:       run.UpdatedAt,
		})
		for _, observation := range run.Observations {
			events = append(events, agentProgressEvent{
				response: AgentProgressEventResponse{
					ID:        fmt.Sprintf("observation:%d", observation.ID),
					Kind:      "observation",
					Title:     agentProgressObservationTitle(observation),
					Status:    agentProgressFirstNonEmpty(observation.Status, "created"),
					Summary:   agentProgressFirstNonEmpty(observation.OutputSummary, observation.InputSummary, observation.Error, "能力调用已记录"),
					Ref:       fmt.Sprintf("run:%d", observation.RunID),
					CreatedAt: formatOptionalTime(&observation.CreatedAt),
				},
				at: observation.CreatedAt,
			})
		}
		for _, artifact := range run.Artifacts {
			events = append(events, agentProgressEvent{
				response: AgentProgressEventResponse{
					ID:        fmt.Sprintf("artifact:%d", artifact.ID),
					Kind:      "artifact",
					Title:     agentProgressFirstNonEmpty(artifact.ArtifactType, "artifact"),
					Status:    "created",
					Summary:   agentProgressFirstNonEmpty(artifact.Summary, artifact.ContentRef, "执行产物已记录"),
					Ref:       artifact.ContentRef,
					CreatedAt: formatOptionalTime(&artifact.CreatedAt),
				},
				at: artifact.CreatedAt,
			})
		}
	}
	for _, task := range scheduledTasks {
		events = append(events, agentProgressEvent{
			response: AgentProgressEventResponse{ID: fmt.Sprintf("scheduled_task:%d", task.ID), Kind: "scheduled_task", Title: task.TaskType, Status: string(task.Status), Summary: task.Goal, CreatedAt: formatOptionalTime(&task.UpdatedAt)},
			at:       task.UpdatedAt,
		})
	}
	for _, log := range auditLogs {
		events = append(events, agentProgressEvent{
			response: AgentProgressEventResponse{
				ID:        fmt.Sprintf("audit:%d", log.ID),
				Kind:      "audit",
				Source:    agentAuditEventSource(log.EventType),
				Title:     log.EventType,
				Status:    agentProgressFirstNonEmpty(log.Status, "recorded"),
				Summary:   agentProgressFirstNonEmpty(log.Message, agentAuditEventSummary(log)),
				Ref:       agentAuditEventRef(log),
				CreatedAt: formatOptionalTime(&log.CreatedAt),
			},
			at: log.CreatedAt,
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].at.After(events[j].at)
	})
	if len(events) > 20 {
		events = events[:20]
	}
	responses := make([]AgentProgressEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, event.response)
	}
	return responses
}

func agentAuditEventSource(eventType string) string {
	eventType = strings.TrimSpace(eventType)
	if strings.Contains(eventType, "retry") || strings.Contains(eventType, "recover") || strings.Contains(eventType, "approval") || strings.Contains(eventType, "canceled") {
		return "user_action"
	}
	if strings.Contains(eventType, "notification") || strings.Contains(eventType, "reply") || strings.Contains(eventType, "report") || strings.Contains(eventType, "feedback") {
		return "notification"
	}
	if strings.Contains(eventType, "capability") || strings.Contains(eventType, "observation") {
		return "capability"
	}
	return "system"
}

func agentAuditEventSummary(log domain.AgentAuditLog) string {
	if log.Metadata != nil {
		if stage, ok := log.Metadata["stage"].(string); ok && strings.TrimSpace(stage) != "" {
			return strings.TrimSpace(stage)
		}
		if reason, ok := log.Metadata["reason"].(string); ok && strings.TrimSpace(reason) != "" {
			return strings.TrimSpace(reason)
		}
	}
	return log.EventType
}

func agentAuditEventRef(log domain.AgentAuditLog) string {
	if log.Metadata != nil {
		for _, key := range []string{"plan_id", "step_id", "scheduled_task_id", "run_id"} {
			if value, ok := log.Metadata[key]; ok {
				return fmt.Sprintf("%s:%v", key, value)
			}
		}
	}
	if log.TurnID > 0 {
		return fmt.Sprintf("turn:%d", log.TurnID)
	}
	if log.SessionID > 0 {
		return fmt.Sprintf("session:%d", log.SessionID)
	}
	return ""
}

func agentScheduledTaskResponse(task domain.AgentScheduledTask) AgentScheduledTaskResponse {
	return AgentScheduledTaskResponse{
		ID:                  task.ID,
		UserID:              task.UserID,
		SessionID:           task.SessionID,
		TurnID:              task.TurnID,
		PlanID:              task.PlanID,
		SourceRunID:         task.SourceRunID,
		Status:              string(task.Status),
		TaskType:            task.TaskType,
		Goal:                task.Goal,
		TargetChannel:       task.TargetChannel,
		TargetRef:           task.TargetRef,
		ScheduledAt:         formatOptionalTime(&task.ScheduledAt),
		DeliverAt:           formatOptionalTime(task.DeliverAt),
		FreshnessPolicy:     task.FreshnessPolicy,
		AllowedCapabilities: append([]string(nil), task.AllowedCapabilities...),
		ModelPolicy:         cloneApprovalMetadata(task.ModelPolicy),
		FailurePolicy:       cloneApprovalMetadata(task.FailurePolicy),
		Payload:             cloneApprovalMetadata(task.Payload),
		AttemptCount:        task.AttemptCount,
		MaxAttempts:         task.MaxAttempts,
		LastError:           task.LastError,
		NextRunAt:           formatOptionalTime(task.NextRunAt),
		CompletedAt:         formatOptionalTime(task.CompletedAt),
		CreatedAt:           formatOptionalTime(&task.CreatedAt),
		UpdatedAt:           formatOptionalTime(&task.UpdatedAt),
	}
}

func aggregateRunStatus(runs []domain.AgentRun) string {
	if len(runs) == 0 {
		return "pending"
	}
	hasRunning := false
	hasFailed := false
	allSucceeded := true
	for _, run := range runs {
		switch run.Status {
		case domain.AgentRunStatusRunning:
			hasRunning = true
			allSucceeded = false
		case domain.AgentRunStatusFailed, domain.AgentRunStatusCanceled:
			hasFailed = true
			allSucceeded = false
		case domain.AgentRunStatusSucceeded:
		default:
			allSucceeded = false
		}
	}
	if hasFailed {
		return "failed"
	}
	if hasRunning {
		return "running"
	}
	if allSucceeded {
		return "succeeded"
	}
	return "pending"
}

func aggregateScheduledTaskStatus(tasks []domain.AgentScheduledTask) string {
	if len(tasks) == 0 {
		return "none"
	}
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusRunning {
			return "running"
		}
	}
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusQueued {
			return "queued"
		}
	}
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusFailed || task.Status == domain.AgentScheduledTaskStatusExpired || task.Status == domain.AgentScheduledTaskStatusCanceled {
			return string(task.Status)
		}
	}
	return string(tasks[0].Status)
}

func aggregateResponseStatus(plan domain.AgentPlan, planLoaded bool, runs []domain.AgentRun, tasks []domain.AgentScheduledTask) string {
	if planLoaded {
		switch plan.Status {
		case domain.AgentPlanStatusCompleted:
			return "succeeded"
		case domain.AgentPlanStatusFailed, domain.AgentPlanStatusRejected, domain.AgentPlanStatusExpired:
			return "failed"
		case domain.AgentPlanStatusAwaitingApproval:
			return "input_required"
		}
	}
	if aggregateRunStatus(runs) == "running" || aggregateScheduledTaskStatus(tasks) == "running" {
		return "running"
	}
	return "pending"
}

func agentProgressNextAction(status string, planLoaded bool, plan domain.AgentPlan, tasks []domain.AgentScheduledTask) string {
	if planLoaded {
		switch plan.Status {
		case domain.AgentPlanStatusAwaitingApproval:
			return "等待用户在 Web 或企业微信中确认计划"
		case domain.AgentPlanStatusApproved, domain.AgentPlanStatusExecuting:
			return "等待 executor 写入 observation 和 artifact"
		case domain.AgentPlanStatusCompleted:
			return "任务已完成，可查看结果与审计细节"
		case domain.AgentPlanStatusFailed:
			return "任务失败，需要查看错误和失败步骤"
		case domain.AgentPlanStatusRejected:
			return "计划已拒绝，不会继续执行"
		case domain.AgentPlanStatusExpired:
			return "确认已过期，需要重新发起任务"
		}
	}
	for _, task := range tasks {
		if task.Status == domain.AgentScheduledTaskStatusQueued {
			return "等待调度时间到达后创建 controller run"
		}
		if task.Status == domain.AgentScheduledTaskStatusRunning {
			return "调度任务正在执行"
		}
	}
	if status == "running" {
		return "等待当前 run 完成"
	}
	return "暂无后续动作"
}

func countAgentObservations(runs []domain.AgentRun) int {
	count := 0
	for _, run := range runs {
		count += len(run.Observations)
	}
	return count
}

func latestRunUpdatedAt(runs []domain.AgentRun) string {
	var latest *time.Time
	for index := range runs {
		if latest == nil || runs[index].UpdatedAt.After(*latest) {
			latest = &runs[index].UpdatedAt
		}
	}
	return formatOptionalTime(latest)
}

func latestScheduledTaskUpdatedAt(tasks []domain.AgentScheduledTask) string {
	var latest *time.Time
	for index := range tasks {
		if latest == nil || tasks[index].UpdatedAt.After(*latest) {
			latest = &tasks[index].UpdatedAt
		}
	}
	return formatOptionalTime(latest)
}

func agentProgressFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func latestProgressUpdatedAt(plan domain.AgentPlan, planLoaded bool, runs []domain.AgentRun, tasks []domain.AgentScheduledTask, events []AgentProgressEventResponse, fallback time.Time) time.Time {
	latest := fallback
	if planLoaded && plan.UpdatedAt.After(latest) {
		latest = plan.UpdatedAt
	}
	for _, step := range plan.Steps {
		if step.UpdatedAt.After(latest) {
			latest = step.UpdatedAt
		}
	}
	for _, approval := range plan.Approvals {
		if approval.UpdatedAt.After(latest) {
			latest = approval.UpdatedAt
		}
	}
	for _, run := range runs {
		if run.UpdatedAt.After(latest) {
			latest = run.UpdatedAt
		}
		for _, observation := range run.Observations {
			if observation.CreatedAt.After(latest) {
				latest = observation.CreatedAt
			}
		}
		for _, artifact := range run.Artifacts {
			if artifact.CreatedAt.After(latest) {
				latest = artifact.CreatedAt
			}
		}
	}
	for _, task := range tasks {
		if task.UpdatedAt.After(latest) {
			latest = task.UpdatedAt
		}
	}
	for _, event := range events {
		if event.CreatedAt == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, event.CreatedAt); err == nil && parsed.After(latest) {
			latest = parsed
		}
	}
	return latest.UTC()
}

func agentProgressVersion(updatedAt time.Time, events []AgentProgressEventResponse) int64 {
	if updatedAt.IsZero() {
		return int64(len(events))
	}
	return updatedAt.UnixNano() + int64(len(events))
}

func agentProgressEventCursor(events []AgentProgressEventResponse, updatedAt time.Time) string {
	if len(events) == 0 {
		return fmt.Sprintf("%s:0", updatedAt.UTC().Format(time.RFC3339Nano))
	}
	return fmt.Sprintf("%s:%s:%d", updatedAt.UTC().Format(time.RFC3339Nano), events[0].ID, len(events))
}

func agentProgressObservationTitle(observation domain.AgentObservation) string {
	if observation.CapabilityKey == "" {
		return "能力调用"
	}
	return "能力调用：" + observation.CapabilityKey
}

func AgentProgressTextSummary(progress AgentProgressSnapshot) string {
	var builder strings.Builder
	if progress.Summary != "" {
		builder.WriteString(progress.Summary)
	} else {
		builder.WriteString("Agent 任务")
	}
	builder.WriteString("\n状态：")
	builder.WriteString(progressStatusText(progress.Status))
	if progress.NextAction != "" {
		builder.WriteString("\n下一步：")
		builder.WriteString(progress.NextAction)
	}
	if progress.Plan != nil {
		builder.WriteString("\n权限：")
		builder.WriteString(planPermissionSummary(agentPlanResponseToDomain(*progress.Plan)))
		builder.WriteString("\n预算：")
		builder.WriteString(planBudgetSummary(agentPlanResponseToDomain(*progress.Plan)))
		if metadataMap(progress.Plan.Metadata, "result_quality") != nil {
			builder.WriteString("\n质量：")
			builder.WriteString(planResultQualitySummary(agentPlanResponseToDomain(*progress.Plan)))
		}
		if metadataMap(progress.Plan.Metadata, "cost_summary") != nil {
			builder.WriteString("\n成本：")
			builder.WriteString(planCostSummary(agentPlanResponseToDomain(*progress.Plan)))
		}
		if metadataMap(progress.Plan.Metadata, "runtime_observability") != nil {
			builder.WriteString("\n运行观测：")
			builder.WriteString(planRuntimeObservabilitySummary(agentPlanResponseToDomain(*progress.Plan)))
		}
		if metadataMap(progress.Plan.Metadata, "handoff") != nil {
			builder.WriteString("\n人工接管：")
			builder.WriteString(planHandoffSummary(agentPlanResponseToDomain(*progress.Plan)))
		}
		refs := agentPlanResponseEvidenceRefs(*progress.Plan)
		if len(refs) > 0 {
			builder.WriteString("\n证据引用：")
			builder.WriteString(strings.Join(refs, ", "))
		}
	}
	if progress.EventCursor != "" {
		builder.WriteString("\n进度版本：")
		builder.WriteString(progress.EventCursor)
	}
	return strings.TrimSpace(builder.String())
}

func progressStatusText(status string) string {
	switch status {
	case "awaiting_approval", "input_required":
		return "等待确认"
	case "approved", "executing", "running":
		return "执行中"
	case "completed", "succeeded":
		return "已完成"
	case "failed":
		return "失败"
	case "rejected":
		return "已拒绝"
	case "expired":
		return "已过期"
	case "queued":
		return "排队中"
	case "canceled":
		return "已取消"
	default:
		if status == "" {
			return "未知"
		}
		return status
	}
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
