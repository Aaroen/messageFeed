import { apiClient } from '@/api/client'

interface APIEnvelope<T> {
  data: T
}

export interface AgentExternalAccount {
  id: number
  provider: string
  corp_id: string
  agent_id: string
  external_user_id: string
  display_name: string
  binding_status: string
  active_agent_session_id: number
  updated_at: string
}

export interface AgentSessionStats {
  transcript_count: number
  archive_index_count: number
  recall_count: number
  first_transcript_at?: string
  last_transcript_at?: string
}

export interface AgentSession {
  id: number
  external_account_id: number
  provider: string
  channel_session_key: string
  status: string
  title: string
  active_for_account: boolean
  context_initialized_at?: string
  context_rebuild_started_at?: string
  context_rebuild_finished_at?: string
  context_version: number
  transcript_count_indexed: number
  stats: AgentSessionStats
  started_at: string
  last_active_at: string
  created_at: string
  updated_at: string
}

export interface AgentSessionListResult {
  accounts: AgentExternalAccount[]
  sessions: AgentSession[]
}

export interface AgentTranscriptEntry {
  id: number
  turn_id: number
  role: string
  content: string
  created_at: string
}

export interface AgentTurn {
  id: number
  session_id: number
  inbound_message_id: number
  status: string
  input_text: string
  output_text: string
  error_message: string
  started_at: string
  finished_at?: string
  created_at: string
  updated_at: string
}

export interface AgentRun {
  id: number
  parent_run_id: number
  session_id: number
  turn_id: number
  role: string
  status: string
  task_packet: Record<string, unknown>
  capability_scope: string[]
  model_key: string
  context_budget: Record<string, unknown>
  context_trace_ref: string
  result_ref: string
  error_message: string
  trace_id: string
  started_at: string
  completed_at?: string
  created_at: string
  updated_at: string
  context_traces?: AgentRunContextTrace[]
  observations?: AgentObservation[]
  artifacts?: AgentArtifact[]
  child_runs?: AgentRun[]
}

export interface AgentRunContextTrace {
  id: number
  run_id: number
  trace_kind: string
  prompt_version: string
  model_key: string
  content: Record<string, unknown>
  content_hash: string
  redaction_status: string
  token_estimate: number
  created_at: string
}

export interface AgentObservation {
  id: number
  run_id: number
  capability_key: string
  input_summary: string
  output_summary: string
  status: string
  error: string
  artifact_refs: string[]
  created_at: string
}

export interface AgentArtifact {
  id: number
  run_id: number
  artifact_type: string
  content_ref: string
  summary: string
  source_refs: string[]
  content_hash: string
  created_at: string
}

export interface AgentPlanStep {
  id: number
  plan_id: number
  step_order: number
  status: string
  capability_key: string
  capability_scope: string[]
  title: string
  input_summary: string
  output_summary: string
  expected_output: string
  failure_strategy: string
  executor_run_id: number
  observation_ref: string
  artifact_refs: string[]
  error_message: string
  retry_count: number
  max_retries: number
  last_retry_at?: string
  retry_reason: string
  retry_metadata: Record<string, unknown>
  started_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface AgentPlanApproval {
  id: number
  plan_id?: number
  channel: string
  status: string
  expires_at: string
  decided_at?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface AgentPlan {
  id: number
  user_id: number
  session_id: number
  turn_id: number
  controller_run_id: number
  status: string
  goal: string
  summary: string
  impact_summary: string
  risk_level: string
  confirmation_policy: string
  allowed_scopes: string[]
  dedupe_key: string
  policy_decision: string
  policy_reason: string
  expires_at?: string
  approved_at?: string
  rejected_at?: string
  completed_at?: string
  failed_at?: string
  error_message: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
  steps?: AgentPlanStep[]
  approvals?: AgentPlanApproval[]
}

export interface AgentScheduledTask {
  id: number
  user_id: number
  session_id: number
  turn_id: number
  plan_id: number
  source_run_id: number
  status: string
  task_type: string
  goal: string
  target_channel: string
  target_ref: string
  scheduled_at: string
  deliver_at?: string
  freshness_policy: string
  allowed_capabilities: string[]
  model_policy: Record<string, unknown>
  failure_policy: Record<string, unknown>
  payload: Record<string, unknown>
  attempt_count: number
  max_attempts: number
  last_error: string
  next_run_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface AgentProgressPhase {
  key: string
  title: string
  status: string
  summary: string
  updated_at?: string
}

export interface AgentProgressEvent {
  id: string
  kind: string
  source?: string
  title: string
  status: string
  summary: string
  ref?: string
  created_at?: string
}

export interface AgentProgressSnapshot {
  subject_type: string
  subject_id: number
  status: string
  summary: string
  next_action: string
  version: number
  event_cursor: string
  updated_at: string
  refreshed_at: string
  plan?: AgentPlan
  runs: AgentRun[]
  scheduled_tasks: AgentScheduledTask[]
  phases: AgentProgressPhase[]
  recent_events: AgentProgressEvent[]
}

export interface AgentPlanRetryResult {
  plan_id: number
  queued: number
  skipped: number
  exhausted: number
  steps: AgentPlanStep[]
}

export interface AgentTaskResult {
  session: AgentSession
  turn: AgentTurn
  plan: AgentPlan
  reply: string
  progress_url: string
  duplicate: boolean
}

export interface AgentTaskSummary {
  id: string
  kind: string
  session_id: number
  turn_id: number
  plan_id: number
  scheduled_task_id: number
  status: string
  goal: string
  summary: string
  permission_status: string
  budget_status: string
  quality_status: string
  handoff_status: string
  observability: string
  latest_progress: string
  next_action: string
  progress_url: string
  updated_at: string
}

export interface AgentSLASummary {
  plan_count: number
  plan_succeeded: number
  plan_failed: number
  scheduled_task_count: number
  scheduled_task_succeeded: number
  scheduled_task_failed: number
  average_plan_seconds: number
  recovery_count: number
  handoff_count: number
  notification_sent_count: number
  notification_failed_count: number
}

export interface AgentTaskReport {
  by_status: Record<string, number>
  by_entry: Record<string, number>
  by_capability: Record<string, number>
  by_handoff: Record<string, number>
}

export interface AgentCostSummary {
  tool_calls: number
  external_calls: number
  estimated_tokens: number
  retry_count: number
  notification_count: number
  scheduled_tasks: number
}

export interface AgentAlertSummary {
  total: number
  critical: number
  warning: number
  reasons: string[]
}

export interface AgentAlertPolicyDecision {
  reason: string
  severity: string
  enabled: boolean
  action: string
}

export interface AgentAlertPolicy {
  status: string
  summary: string
  enabled_reasons: string[]
  muted_reasons: string[]
  decisions: AgentAlertPolicyDecision[]
}

export interface AgentCostTrendBucket {
  date: string
  tool_calls: number
  external_calls: number
  estimated_tokens: number
  retry_count: number
  notification_count: number
}

export interface AgentTrendBucket {
  date: string
  tool_calls: number
  external_calls: number
  estimated_tokens: number
  retry_count: number
  notification_count: number
  plan_failed: number
  scheduled_task_failed: number
  notification_failed: number
  recovery_count: number
  handoff_count: number
}

export interface AgentTrendSnapshot {
  source: string
  retention_days: number
  summary: string
  buckets: AgentTrendBucket[]
}

export interface AgentDeploymentCheck {
  key: string
  status: string
  summary: string
}

export interface AgentDeploymentVerification {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentProductionDrill {
  status: string
  summary: string
  source: string
  generated_at: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatAction {
  key: string
  label: string
  url: string
  fallback: string
}

export interface AgentWeChatComponentSet {
  mode: string
  summary: string
  actions: AgentWeChatAction[]
}

export interface AgentLoadTestMetrics {
  users: number
  web_tasks: number
  wechat_tasks: number
  scheduled_tasks: number
  recovery_events: number
  admission_limited: number
  quota_limited: number
  progress_events: number
}

export interface AgentLoadTestSummary {
  status: string
  summary: string
  metrics: AgentLoadTestMetrics
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatCallbackReadiness {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteSandbox {
  status: string
  summary: string
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentE2EAcceptance {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRealIntegration {
  status: string
  summary: string
  risks: string[]
  blockers: string[]
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatNativeAction {
  key: string
  label: string
  style: string
  url: string
  fallback: string
}

export interface AgentWeChatNativeActionSet {
  mode: string
  summary: string
  actions: AgentWeChatNativeAction[]
}

export interface AgentWriteLeastPrivilege {
  status: string
  summary: string
  default_action: string
  allowed_candidates: string[]
  denied_patterns: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsAcceptance {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatNativeButton {
  key: string
  label: string
  style: string
  url: string
  fallback: string
}

export interface AgentWeChatNativePayload {
  status: string
  summary: string
  message_type: string
  fallback_text: string
  buttons: AgentWeChatNativeButton[]
  payload: Record<string, unknown>
}

export interface AgentWriteGrayPolicy {
  status: string
  summary: string
  candidates: string[]
  allowed_user_scope: string
  requires_approval: boolean
  requires_budget: boolean
  requires_audit: boolean
  rollback_triggers: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentAlertChannelTarget {
  key: string
  status: string
  reasons: string[]
  fallback: string
}

export interface AgentAlertChannel {
  status: string
  summary: string
  channels: AgentAlertChannelTarget[]
}

export interface AgentLaunchDrillRecord {
  batch_id: string
  status: string
  summary: string
  triggered_by: string
  result: string
  risks: string[]
  blockers: string[]
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatNativeIntegration {
  status: string
  summary: string
  risks: string[]
  blockers: string[]
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteReplay {
  status: string
  summary: string
  candidates: string[]
  approval_status: string
  budget_status: string
  permission_status: string
  execution_status: string
  audit_status: string
  rollback_triggers: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentLaunchApproval {
  status: string
  summary: string
  request_id: string
  review_state: string
  approved: number
  rejected: number
  expired: number
  handoff_path: string
  rollback_path: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDailyReport {
  date: string
  status: string
  summary: string
  task_count: number
  success_rate: number
  failure_count: number
  alert_count: number
  estimated_tokens: number
  trend_buckets: number
  handoff_count: number
  recovery_count: number
  notification_count: number
  notification_failed: number
  checks: AgentDeploymentCheck[]
}

export interface AgentPreprodAcceptance {
  status: string
  summary: string
  risks: string[]
  blockers: string[]
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentButtonLoop {
  status: string
  summary: string
  fallback_text: string
  actions: AgentWeChatNativeButton[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteExecute {
  status: string
  summary: string
  candidates: string[]
  default_action: string
  approval_status: string
  budget_status: string
  permission_status: string
  execution_status: string
  audit_status: string
  rollback_triggers: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentDailyPersist {
  status: string
  summary: string
  record_key: string
  source: string
  retained: boolean
  checks: AgentDeploymentCheck[]
}

export interface AgentPostLaunchMonitor {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentReleaseApproval {
  status: string
  summary: string
  request_id: string
  review_state: string
  executable: boolean
  approved: number
  rejected: number
  expired: number
  decision_path: string
  rejection_path: string
  rollback_path: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentButtonCallbackAction {
  key: string
  label: string
  handler: string
  url: string
  fallback: string
  status: string
}

export interface AgentButtonCallback {
  status: string
  summary: string
  fallback_text: string
  actions: AgentButtonCallbackAction[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteAuditReview {
  status: string
  summary: string
  candidates: string[]
  approval_evidence: string
  budget_evidence: string
  permission_evidence: string
  execution_evidence: string
  failure_evidence: string
  rollback_evidence: string
  handoff_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDailySend {
  status: string
  summary: string
  record_key: string
  schedule_status: string
  delivery_status: string
  retry_status: string
  wechat_report_status: string
  checks: AgentDeploymentCheck[]
}

export interface AgentMonitorAlertDrill {
  status: string
  summary: string
  trigger_status: string
  notification_status: string
  handoff_status: string
  checks: AgentDeploymentCheck[]
}

export interface AgentButtonDirectControl {
  status: string
  summary: string
  executed: number
  skipped: number
  actions: AgentButtonCallbackAction[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatE2EAcceptance {
  status: string
  summary: string
  checks: AgentDeploymentCheck[]
}

export interface AgentReleaseWindowReadiness {
  status: string
  summary: string
  window_state: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteGrayExpansion {
  status: string
  summary: string
  candidates: string[]
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentExternalMonitorIntegration {
  status: string
  summary: string
  metrics: string[]
  alert_events: string[]
  channels: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentReleaseWindowExecution {
  status: string
  summary: string
  window_state: string
  execution_state: string
  approval_status: string
  failure_exit_status: string
  rollback_status: string
  notification_status: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentExternalMonitorRuntime {
  status: string
  summary: string
  health_status: string
  sla_status: string
  error_status: string
  cost_status: string
  queue_status: string
  worker_status: string
  notification_failure_status: string
  button_control_status: string
  daily_send_status: string
  metrics: string[]
  alert_events: string[]
  channels: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteGrayReview {
  status: string
  summary: string
  candidates: string[]
  default_action: string
  decision: string
  next_action: string
  denied_patterns: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatAcceptanceReview {
  status: string
  summary: string
  entry_status: string
  progress_status: string
  button_control_status: string
  web_sync_status: string
  final_report_status: string
  failure_fallback_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsDailyClosure {
  status: string
  summary: string
  report_status: string
  monitor_status: string
  button_control_status: string
  release_window_status: string
  audit_status: string
  checks: AgentDeploymentCheck[]
}

export interface AgentProductionRelease {
  status: string
  summary: string
  batch_id: string
  approval_source: string
  precheck_status: string
  execution_status: string
  rollback_gate_status: string
  notification_status: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentExternalMonitorConfig {
  status: string
  summary: string
  platform_status: string
  metric_names: string[]
  event_names: string[]
  alert_channels: string[]
  daily_channels: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteRamp {
  status: string
  summary: string
  candidates: string[]
  ramp_percent: number
  default_action: string
  decision: string
  approval_gate: string
  budget_gate: string
  audit_gate: string
  rollback_gate: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatSignoff {
  status: string
  summary: string
  signoff_state: string
  entry_confirmed: string
  progress_confirmed: string
  button_control_confirmed: string
  web_sync_confirmed: string
  final_report_confirmed: string
  failure_fallback_confirmed: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsHandoff {
  status: string
  summary: string
  release_status: string
  monitor_config_status: string
  write_ramp_status: string
  wechat_signoff_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentProductionExecution {
  status: string
  summary: string
  batch_id: string
  executor: string
  execution_status: string
  rollback_gate_status: string
  failure_exit_status: string
  notification_status: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentMonitorIntegration {
  status: string
  summary: string
  metric_write_status: string
  event_write_status: string
  alert_channel_status: string
  daily_channel_status: string
  integration_result: string
  metric_names: string[]
  event_names: string[]
  channels: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteRampPolicy {
  status: string
  summary: string
  candidates: string[]
  ramp_percent: number
  user_scope: string
  approval_gate: string
  budget_gate: string
  audit_gate: string
  rollback_threshold: string
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatFinalReport {
  status: string
  summary: string
  completion_notice_status: string
  final_report_entry: string
  failure_summary: string
  delivery_status: string
  template_status: string
  text_status: string
  progress_url: string
  next_action: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentLaunchRuntimeOverview {
  status: string
  summary: string
  production_execution_status: string
  monitor_integration_status: string
  write_ramp_policy_status: string
  wechat_final_report_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRuntimeParameters {
  status: string
  summary: string
  ramp_percent: number
  user_scope: string
  notification_channel: string
  monitor_channel: string
  approval_gate: string
  budget_gate: string
  rollback_threshold: string
  checks: AgentDeploymentCheck[]
}

export interface AgentMonitorReadback {
  status: string
  summary: string
  metric_read_status: string
  event_read_status: string
  alert_status: string
  daily_status: string
  freshness_status: string
  metric_names: string[]
  event_names: string[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteRampRecommendation {
  status: string
  summary: string
  current_percent: number
  recommended_percent: number
  candidates: string[]
  limit_conditions: string[]
  rollback_conditions: string[]
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatUserFeedback {
  status: string
  summary: string
  completion_feedback: string
  failure_feedback: string
  button_feedback: string
  web_tracking_feedback: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsRuntimeClosure {
  status: string
  summary: string
  runtime_parameter_status: string
  monitor_readback_status: string
  write_ramp_recommendation_status: string
  wechat_user_feedback_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsPanelConfig {
  status: string
  summary: string
  parameter_group: string
  display_items: string[]
  refresh_interval_seconds: number
  alert_entry: string
  write_ramp_entry: string
  wechat_feedback_entry: string
  checks: AgentDeploymentCheck[]
}

export interface AgentMonitorAutoReport {
  status: string
  summary: string
  anomaly_status: string
  wechat_send_status: string
  web_visibility_status: string
  daily_link_status: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteRampStage {
  status: string
  summary: string
  current_stage: string
  next_stage: string
  entry_conditions: string[]
  exit_conditions: string[]
  rollback_conditions: string[]
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatFeedbackLoop {
  status: string
  summary: string
  completion_state: string
  failure_state: string
  button_state: string
  web_trace_state: string
  processing_state: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsClosedLoop {
  status: string
  summary: string
  ops_panel_status: string
  monitor_report_status: string
  write_ramp_stage_status: string
  feedback_loop_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsDashboardInteraction {
  status: string
  summary: string
  actions: string[]
  refresh_strategy: string
  filters: string[]
  links: string[]
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentAlertDedupeEscalation {
  status: string
  summary: string
  dedupe_key: string
  dedupe_window_seconds: number
  escalation_condition: string
  wechat_notify_status: string
  web_visibility_status: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteStageRecord {
  status: string
  summary: string
  current_stage: string
  target_stage: string
  promotion_reason: string
  blockers: string[]
  rollback_conditions: string[]
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatFeedbackTicket {
  status: string
  summary: string
  ticket_type: string
  processing_state: string
  owner_entry: string
  user_next_action: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsHandling {
  status: string
  summary: string
  dashboard_status: string
  alert_escalation_status: string
  write_stage_status: string
  feedback_ticket_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsActionItem {
  key: string
  label: string
  handler_entry: string
  permission_constraint: string
  idempotency_key: string
  audit_event: string
}

export interface AgentOpsActionDefinition {
  status: string
  summary: string
  actions: AgentOpsActionItem[]
  checks: AgentDeploymentCheck[]
}

export interface AgentAlertEscalationPolicy {
  status: string
  summary: string
  escalation_level: string
  notification_channels: string[]
  repeat_suppression: string
  recipients: string[]
  recovery_notice_status: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteStageApproval {
  status: string
  summary: string
  approval_status: string
  approval_source: string
  target_stage: string
  authorized_scope: string
  rollback_threshold: string
  default_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentFeedbackTicketLifecycle {
  status: string
  summary: string
  created_state: string
  assigned_state: string
  processing_state: string
  waiting_user_state: string
  closed_state: string
  handoff_state: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsActionClosure {
  status: string
  summary: string
  ops_action_status: string
  alert_escalation_status: string
  write_approval_status: string
  ticket_lifecycle_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsAPIExecutionItem {
  action_key: string
  execution_entry: string
  execution_status: string
  permission_check: string
  idempotency_result: string
  audit_event: string
}

export interface AgentOpsAPIExecution {
  status: string
  summary: string
  executions: AgentOpsAPIExecutionItem[]
  checks: AgentDeploymentCheck[]
}

export interface AgentAlertEscalationReceipt {
  status: string
  summary: string
  notification_channels: string[]
  recipients: string[]
  delivery_status: string
  suppression_result: string
  recovery_notice_status: string
  handoff_entry: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWriteApprovalButtonItem {
  button_key: string
  channel: string
  approval_status: string
  permission_scope: string
  rollback_threshold: string
  rejection_path: string
  audit_evidence: string
}

export interface AgentWriteApprovalButton {
  status: string
  summary: string
  buttons: AgentWriteApprovalButtonItem[]
  checks: AgentDeploymentCheck[]
}

export interface AgentFeedbackTicketSLA {
  status: string
  summary: string
  first_response_seconds: number
  resolve_seconds: number
  timeout_escalation: string
  waiting_user_status: string
  close_condition: string
  handoff_path: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsExecution {
  status: string
  summary: string
  ops_api_execution_status: string
  alert_receipt_status: string
  write_approval_button_status: string
  feedback_sla_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOpsExecutionRecordItem {
  record_key: string
  action_key: string
  execution_status: string
  idempotency_status: string
  audit_event: string
  replay_entry: string
}

export interface AgentOpsExecutionRecord {
  status: string
  summary: string
  records: AgentOpsExecutionRecordItem[]
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatApprovalCallback {
  status: string
  summary: string
  callback_key: string
  source: string
  decision: string
  signature: string
  storage_state: string
  fallback_path: string
  checks: AgentDeploymentCheck[]
}

export interface AgentFeedbackSLAReport {
  status: string
  summary: string
  first_response_rate: number
  resolve_rate: number
  timeout_count: number
  waiting_user_count: number
  handoff_count: number
  report_audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentAlertAutoRecovery {
  status: string
  summary: string
  recovery_trigger: string
  recovery_notice: string
  suppression_release: string
  reopen_condition: string
  handoff_state: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentOperationsEvidence {
  status: string
  summary: string
  execution_record_status: string
  approval_callback_status: string
  sla_report_status: string
  auto_recovery_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentUnifiedProgressComponent {
  status: string
  summary: string
  component_key: string
  web_status: string
  wechat_status: string
  event_cursor: string
  refresh_strategy: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentEvidenceDetailPage {
  status: string
  summary: string
  detail_entry: string
  record_count: number
  audit_event: string
  replay_entry: string
  visibility: string
  retention_policy: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayTool {
  status: string
  summary: string
  callback_key: string
  replay_entry: string
  signature_review: string
  idempotency_guard: string
  failure_fallback: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyConfig {
  status: string
  summary: string
  policy_key: string
  recovery_trigger: string
  suppression_window: string
  reopen_condition: string
  handoff_state: string
  default_policy: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndProgressEvidence {
  status: string
  summary: string
  unified_progress_status: string
  evidence_detail_status: string
  callback_replay_status: string
  recovery_policy_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatProgressCard {
  status: string
  summary: string
  card_key: string
  phase_status: string
  progress_percent: number
  detail_entry: string
  actions: AgentButtonCallbackAction[]
  fallback_text: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceInteraction {
  status: string
  summary: string
  filters: string[]
  expandable: string
  replay_entry: string
  audit_display: string
  retention_hint: string
  visibility: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayPermission {
  status: string
  summary: string
  permission_key: string
  allowed_roles: string[]
  idempotency_guard: string
  signature_review: string
  failure_fallback: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyAudit {
  status: string
  summary: string
  change_key: string
  old_policy: string
  new_policy: string
  approval_status: string
  rollback_path: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndInteraction {
  status: string
  summary: string
  wechat_progress_card_status: string
  web_evidence_status: string
  callback_permission_status: string
  recovery_policy_audit_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatTemplateRender {
  status: string
  summary: string
  template_key: string
  render_status: string
  phase_fields: string[]
  button_fields: string[]
  fallback_text: string
  send_entry: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceRoute {
  status: string
  summary: string
  route_name: string
  path_params: string[]
  filter_params: string[]
  permission_requirement: string
  replay_entry: string
  audit_display: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayApproval {
  status: string
  summary: string
  approval_key: string
  request_entry: string
  approval_roles: string[]
  approval_status: string
  execution_gate: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyPersist {
  status: string
  summary: string
  config_key: string
  current_version: string
  pending_version: string
  persistence_status: string
  rollback_version: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndInteractionLaunch {
  status: string
  summary: string
  wechat_template_render_status: string
  web_evidence_route_status: string
  callback_replay_approval_status: string
  recovery_policy_persistence_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatTemplateSend {
  status: string
  summary: string
  message_type: string
  title: string
  phase_fields: string[]
  button_fields: string[]
  fallback_text: string
  send_entry: string
  send_result: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceDetailView {
  status: string
  summary: string
  route_name: string
  route_path: string
  plan_param: string
  record_param: string
  record_source: string
  filter_params: string[]
  audit_events: string[]
  replay_entry: string
  permission_hint: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayExecution {
  status: string
  summary: string
  request_entry: string
  execute_entry: string
  approval_status: string
  execution_gate: string
  idempotency_key: string
  audit_event: string
  failure_fallback: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyVersion {
  status: string
  summary: string
  policy_key: string
  current_version: string
  pending_version: string
  rollback_version: string
  release_status: string
  config_source: string
  audit_event: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndRealInteraction {
  status: string
  summary: string
  wechat_template_send_status: string
  web_evidence_detail_status: string
  callback_replay_execution_status: string
  recovery_policy_version_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatTemplateIntegration {
  status: string
  summary: string
  send_path: string
  template_status: string
  fallback_status: string
  degrade_strategy: string
  message_id_readback: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceInteractionDetail {
  status: string
  summary: string
  filter_mode: string
  expand_mode: string
  audit_timeline: string
  replay_request_entry: string
  permission_hint: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplaySafetyAudit {
  status: string
  summary: string
  idempotency_check: string
  approval_check: string
  signature_check: string
  execution_result: string
  failure_audit: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyGrayRelease {
  status: string
  summary: string
  gray_stage: string
  release_percent: number
  rollback_condition: string
  approval_status: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndRunLoop {
  status: string
  summary: string
  wechat_template_integration_status: string
  web_evidence_interaction_status: string
  callback_replay_safety_status: string
  recovery_policy_gray_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatTemplatePilot {
  status: string
  summary: string
  pilot_batch: string
  target_scope: string
  template_status: string
  fallback_hit: string
  message_id_status: string
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceUserAction {
  status: string
  summary: string
  filter_action: string
  expand_action: string
  timeline_action: string
  replay_request: string
  permission_result: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayResultTrace {
  status: string
  summary: string
  execution_result: string
  idempotency_hit: string
  approval_decision: string
  signature_result: string
  failure_reason: string
  audit_record: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryPolicyAutomation {
  status: string
  summary: string
  auto_advance: string
  pause_condition: string
  rollback_condition: string
  current_percent: number
  next_percent: number
  audit_evidence: string
  checks: AgentDeploymentCheck[]
}

export interface AgentDualEndTaskClosure {
  status: string
  summary: string
  wechat_pilot_status: string
  web_evidence_action_status: string
  callback_replay_trace_status: string
  recovery_automation_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatTemplatePilotMetric {
  status: string
  summary: string
  batch_id: string
  target_user_scope: string
  send_status: string
  fallback_count: number
  message_id_status: string
  audit_ref: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWebEvidenceOperation {
  status: string
  summary: string
  filter_entry: string
  expand_entry: string
  timeline_entry: string
  replay_request_entry: string
  permission_gate: string
  audit_event: string
  operation_count: number
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayResultQuery {
  status: string
  summary: string
  query_entry: string
  execution_result: string
  idempotency_result: string
  approval_decision: string
  signature_result: string
  failure_reason: string
  audit_ref: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRecoveryAutomationExecution {
  status: string
  summary: string
  execution_mode: string
  current_percent: number
  next_percent: number
  advance_decision: string
  pause_gate: string
  rollback_gate: string
  approval_gate: string
  audit_ref: string
  checks: AgentDeploymentCheck[]
}

export interface AgentRealInteractionAutomation {
  status: string
  summary: string
  pilot_metric_status: string
  evidence_operation_status: string
  replay_query_status: string
  recovery_execution_status: string
  audit_status: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentWeChatWebProgressLink {
  status: string
  summary: string
  progress_url: string
  url_source: string
  delivery_channel: string
  template_status: string
  fallback_status: string
  browser_target: string
  audit_ref: string
  next_action: string
  checks: AgentDeploymentCheck[]
}

export interface AgentCallbackReplayInput {
  plan_id?: number
  callback_key?: string
  replay_entry?: string
  reason?: string
  approved?: boolean
}

export interface AgentCallbackReplayAPIResult {
  replay_execution: AgentCallbackReplayExecution
  audit_event: string
}

export interface AgentTaskListResult {
  tasks: AgentTaskSummary[]
  sla: AgentSLASummary
  cost: AgentCostSummary
  alerts: AgentAlertSummary
  alert_policy: AgentAlertPolicy
  cost_trend: AgentCostTrendBucket[]
  trend_snapshot: AgentTrendSnapshot
  deployment: AgentDeploymentVerification
  drill: AgentProductionDrill
  wechat_components: AgentWeChatComponentSet
  load_test: AgentLoadTestSummary
  wechat_callback: AgentWeChatCallbackReadiness
  write_sandbox: AgentWriteSandbox
  e2e: AgentE2EAcceptance
  real_integration: AgentRealIntegration
  wechat_native: AgentWeChatNativeActionSet
  write_least_privilege: AgentWriteLeastPrivilege
  ops_acceptance: AgentOpsAcceptance
  wechat_native_payload: AgentWeChatNativePayload
  write_gray: AgentWriteGrayPolicy
  alert_channel: AgentAlertChannel
  launch_drill: AgentLaunchDrillRecord
  wechat_native_integration: AgentWeChatNativeIntegration
  write_replay: AgentWriteReplay
  launch_approval: AgentLaunchApproval
  daily_report: AgentDailyReport
  preprod: AgentPreprodAcceptance
  button_loop: AgentButtonLoop
  write_execute: AgentWriteExecute
  daily_persist: AgentDailyPersist
  post_launch_monitor: AgentPostLaunchMonitor
  release_approval: AgentReleaseApproval
  button_callback: AgentButtonCallback
  write_audit: AgentWriteAuditReview
  daily_send: AgentDailySend
  monitor_alert: AgentMonitorAlertDrill
  button_direct_control: AgentButtonDirectControl
  wechat_e2e: AgentWeChatE2EAcceptance
  release_window: AgentReleaseWindowReadiness
  write_gray_expansion: AgentWriteGrayExpansion
  external_monitor: AgentExternalMonitorIntegration
  release_window_execution: AgentReleaseWindowExecution
  external_monitor_runtime: AgentExternalMonitorRuntime
  write_gray_review: AgentWriteGrayReview
  wechat_acceptance_review: AgentWeChatAcceptanceReview
  operations_daily_closure: AgentOperationsDailyClosure
  production_release: AgentProductionRelease
  external_monitor_config: AgentExternalMonitorConfig
  write_ramp: AgentWriteRamp
  wechat_signoff: AgentWeChatSignoff
  operations_handoff: AgentOperationsHandoff
  production_execution: AgentProductionExecution
  monitor_integration: AgentMonitorIntegration
  write_ramp_policy: AgentWriteRampPolicy
  wechat_final_report: AgentWeChatFinalReport
  launch_runtime_overview: AgentLaunchRuntimeOverview
  runtime_parameters: AgentRuntimeParameters
  monitor_readback: AgentMonitorReadback
  write_ramp_recommendation: AgentWriteRampRecommendation
  wechat_user_feedback: AgentWeChatUserFeedback
  operations_runtime_closure: AgentOperationsRuntimeClosure
  ops_panel_config: AgentOpsPanelConfig
  monitor_auto_report: AgentMonitorAutoReport
  write_ramp_stage: AgentWriteRampStage
  wechat_feedback_loop: AgentWeChatFeedbackLoop
  operations_closed_loop: AgentOperationsClosedLoop
  ops_dashboard_interaction: AgentOpsDashboardInteraction
  alert_dedupe_escalation: AgentAlertDedupeEscalation
  write_stage_record: AgentWriteStageRecord
  wechat_feedback_ticket: AgentWeChatFeedbackTicket
  operations_handling: AgentOperationsHandling
  ops_action_definition: AgentOpsActionDefinition
  alert_escalation_policy: AgentAlertEscalationPolicy
  write_stage_approval: AgentWriteStageApproval
  feedback_ticket_lifecycle: AgentFeedbackTicketLifecycle
  operations_action_closure: AgentOperationsActionClosure
  ops_api_execution: AgentOpsAPIExecution
  alert_escalation_receipt: AgentAlertEscalationReceipt
  write_approval_button: AgentWriteApprovalButton
  feedback_ticket_sla: AgentFeedbackTicketSLA
  operations_execution: AgentOperationsExecution
  ops_execution_record: AgentOpsExecutionRecord
  wechat_approval_callback: AgentWeChatApprovalCallback
  feedback_sla_report: AgentFeedbackSLAReport
  alert_auto_recovery: AgentAlertAutoRecovery
  operations_evidence: AgentOperationsEvidence
  unified_progress_component: AgentUnifiedProgressComponent
  evidence_detail_page: AgentEvidenceDetailPage
  callback_replay_tool: AgentCallbackReplayTool
  recovery_policy_config: AgentRecoveryPolicyConfig
  dual_end_progress_evidence: AgentDualEndProgressEvidence
  wechat_progress_card: AgentWeChatProgressCard
  web_evidence_interaction: AgentWebEvidenceInteraction
  callback_replay_permission: AgentCallbackReplayPermission
  recovery_policy_audit: AgentRecoveryPolicyAudit
  dual_end_interaction: AgentDualEndInteraction
  wechat_template_render: AgentWeChatTemplateRender
  web_evidence_route: AgentWebEvidenceRoute
  callback_replay_approval: AgentCallbackReplayApproval
  recovery_policy_persist: AgentRecoveryPolicyPersist
  dual_end_interaction_launch: AgentDualEndInteractionLaunch
  wechat_template_send: AgentWeChatTemplateSend
  web_evidence_detail_view: AgentWebEvidenceDetailView
  callback_replay_execution: AgentCallbackReplayExecution
  recovery_policy_version: AgentRecoveryPolicyVersion
  dual_end_real_interaction: AgentDualEndRealInteraction
  wechat_template_integration: AgentWeChatTemplateIntegration
  web_evidence_interaction_detail: AgentWebEvidenceInteractionDetail
  callback_replay_safety_audit: AgentCallbackReplaySafetyAudit
  recovery_policy_gray_release: AgentRecoveryPolicyGrayRelease
  dual_end_run_loop: AgentDualEndRunLoop
  wechat_template_pilot: AgentWeChatTemplatePilot
  web_evidence_user_action: AgentWebEvidenceUserAction
  callback_replay_result_trace: AgentCallbackReplayResultTrace
  recovery_policy_automation: AgentRecoveryPolicyAutomation
  dual_end_task_closure: AgentDualEndTaskClosure
  wechat_template_pilot_metric: AgentWeChatTemplatePilotMetric
  web_evidence_operation: AgentWebEvidenceOperation
  callback_replay_result_query: AgentCallbackReplayResultQuery
  recovery_automation_execution: AgentRecoveryAutomationExecution
  real_interaction_automation: AgentRealInteractionAutomation
  wechat_web_progress_link: AgentWeChatWebProgressLink
  report: AgentTaskReport
}

export interface AgentEvalResult {
  id: number
  run_id: number
  case_id: number
  status: string
  score: number
  input: Record<string, unknown>
  expected: string
  actual: string
  failure_reason: string
  metrics: Record<string, unknown>
  evidence_refs: string[]
  created_at: string
}

export interface AgentEvalRun {
  id: number
  user_id: number
  trigger: string
  status: string
  model_key: string
  case_count: number
  passed_count: number
  failed_count: number
  metrics: Record<string, unknown>
  started_at?: string
  completed_at?: string
  error_message: string
  created_at: string
  updated_at: string
  results?: AgentEvalResult[]
}

export interface AgentEvalTrend {
  run_count: number
  completed_count: number
  failed_run_count: number
  case_count: number
  passed_count: number
  failed_result_count: number
  pass_rate: number
  latest_run_at?: string
  failure_summary: string[]
}

export interface AgentEvalRunListResult {
  runs: AgentEvalRun[]
  trend: AgentEvalTrend
}

export interface AgentNotificationPreference {
  process_notifications_enabled: boolean
  final_reports_enabled: boolean
  failure_notifications_enabled: boolean
  recovery_notifications_enabled: boolean
  max_concurrent_tasks: number
  max_queued_tasks: number
  auto_recovery_enabled: boolean
  quality_handoff_threshold: number
  handoff_on_failure: boolean
  handoff_on_permission: boolean
  handoff_on_budget: boolean
  capability_policy: Record<string, string>
  daily_task_quota: number
  daily_external_call_quota: number
  daily_capability_call_quota: number
  updated_at?: string
}

export async function listAgentSessions() {
  const response = await apiClient.get<APIEnvelope<AgentSessionListResult>>('/api/v1/agent/sessions')
  return response.data.data
}

export async function createAgentSession(input: { external_account_id: number; title: string }) {
  const response = await apiClient.post<APIEnvelope<AgentSession>>('/api/v1/agent/sessions', input)
  return response.data.data
}

export async function selectAgentSession(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentExternalAccount>>(`/api/v1/agent/sessions/${id}/select`)
  return response.data.data
}

export async function createAgentTask(input: { message: string; session_id?: number; channel?: string }) {
  const response = await apiClient.post<APIEnvelope<AgentTaskResult>>('/api/v1/agent/tasks', input)
  return response.data.data
}

export async function listAgentTasks(input: { limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<AgentTaskListResult>>('/api/v1/agent/tasks', {
    params: input,
  })
  return response.data.data
}

export async function rebuildAgentSessionContext(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentSessionStats>>(`/api/v1/agent/sessions/${id}/rebuild-context`)
  return response.data.data
}

export async function clearAgentSessionContext(id: number) {
  const response = await apiClient.delete<APIEnvelope<AgentSessionStats>>(`/api/v1/agent/sessions/${id}/context`)
  return response.data.data
}

export async function deleteAgentSession(id: number, input: { current_password: string }) {
  const response = await apiClient.delete<APIEnvelope<{ deleted: boolean }>>(`/api/v1/agent/sessions/${id}`, {
    data: input,
  })
  return response.data.data
}

export async function listAgentTranscripts(id: number, input: { before_entry_id?: number; limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<{ entries: AgentTranscriptEntry[] }>>(
    `/api/v1/agent/sessions/${id}/transcripts`,
    { params: input },
  )
  return response.data.data.entries
}

export async function listAgentPlans(input: { session_id?: number; turn_id?: number; limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<{ plans: AgentPlan[] }>>('/api/v1/agent/plans', { params: input })
  return response.data.data.plans
}

export async function getAgentPlan(id: number) {
  const response = await apiClient.get<APIEnvelope<{ plan: AgentPlan }>>(`/api/v1/agent/plans/${id}`)
  return response.data.data.plan
}

export async function getAgentProgress(input: {
  plan_id?: number
  turn_id?: number
  run_id?: number
  scheduled_task_id?: number
}) {
  const response = await apiClient.get<APIEnvelope<{ progress: AgentProgressSnapshot }>>('/api/v1/agent/progress', {
    params: input,
  })
  return response.data.data.progress
}

export function agentProgressStreamURL(input: {
  plan_id?: number
  turn_id?: number
  run_id?: number
  scheduled_task_id?: number
}) {
  const params = new URLSearchParams()
  if (input.plan_id && input.plan_id > 0) {
    params.set('plan_id', String(input.plan_id))
  }
  if (input.turn_id && input.turn_id > 0) {
    params.set('turn_id', String(input.turn_id))
  }
  if (input.run_id && input.run_id > 0) {
    params.set('run_id', String(input.run_id))
  }
  if (input.scheduled_task_id && input.scheduled_task_id > 0) {
    params.set('scheduled_task_id', String(input.scheduled_task_id))
  }
  const query = params.toString()
  return query ? `/api/v1/agent/progress/stream?${query}` : ''
}

export async function cancelAgentScheduledTask(id: number) {
  const response = await apiClient.post<APIEnvelope<{ task: AgentScheduledTask }>>(
    `/api/v1/agent/scheduled-tasks/${id}/cancel`,
  )
  return response.data.data.task
}

export async function requestAgentCallbackReplay(input: AgentCallbackReplayInput) {
  const response = await apiClient.post<APIEnvelope<AgentCallbackReplayAPIResult>>(
    '/api/v1/agent/callback-replay/requests',
    input,
  )
  return response.data.data
}

export async function executeAgentCallbackReplay(input: AgentCallbackReplayInput) {
  const response = await apiClient.post<APIEnvelope<AgentCallbackReplayAPIResult>>(
    '/api/v1/agent/callback-replay/execute',
    input,
  )
  return response.data.data
}

export async function listAgentEvalRuns(input: { limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<AgentEvalRunListResult>>('/api/v1/agent/eval-runs', {
    params: input,
  })
  return response.data.data
}

export async function getAgentNotificationPreference() {
  const response = await apiClient.get<APIEnvelope<AgentNotificationPreference>>('/api/v1/agent/notification-preferences')
  return response.data.data
}

export async function updateAgentNotificationPreference(input: Partial<AgentNotificationPreference>) {
  const response = await apiClient.patch<APIEnvelope<AgentNotificationPreference>>(
    '/api/v1/agent/notification-preferences',
    input,
  )
  return response.data.data
}

export async function runBuiltinAgentEval(input: { trigger?: string; model_key?: string } = {}) {
  const response = await apiClient.post<APIEnvelope<{ run: AgentEvalRun }>>('/api/v1/agent/eval-runs', input)
  return response.data.data.run
}

export async function getAgentEvalRun(id: number) {
  const response = await apiClient.get<APIEnvelope<{ run: AgentEvalRun }>>(`/api/v1/agent/eval-runs/${id}`)
  return response.data.data.run
}

export async function retryAgentPlanStep(planID: number, stepID: number, input: { reason?: string } = {}) {
  const response = await apiClient.post<APIEnvelope<{ plan_id: number; step: AgentPlanStep }>>(
    `/api/v1/agent/plans/${planID}/steps/${stepID}/retry`,
    input,
  )
  return response.data.data
}

export async function retryAgentPlan(planID: number, input: { reason?: string } = {}) {
  const response = await apiClient.post<APIEnvelope<AgentPlanRetryResult>>(
    `/api/v1/agent/plans/${planID}/retry`,
    input,
  )
  return response.data.data
}

export async function recoverAgentPlan(planID: number, input: { reason?: string } = {}) {
  const response = await apiClient.post<APIEnvelope<{ plan: AgentPlan }>>(
    `/api/v1/agent/plans/${planID}/recover`,
    input,
  )
  return response.data.data.plan
}

export async function recoverAgentScheduledTask(id: number, input: { reason?: string } = {}) {
  const response = await apiClient.post<APIEnvelope<{ task: AgentScheduledTask }>>(
    `/api/v1/agent/scheduled-tasks/${id}/recover`,
    input,
  )
  return response.data.data.task
}

export async function listAgentRunsByTurn(turnID: number) {
  const response = await apiClient.get<APIEnvelope<{ runs: AgentRun[] }>>(`/api/v1/agent/turns/${turnID}/runs`)
  return response.data.data.runs
}

export async function getAgentRun(id: number) {
  const response = await apiClient.get<APIEnvelope<{ run: AgentRun }>>(`/api/v1/agent/runs/${id}`)
  return response.data.data.run
}
