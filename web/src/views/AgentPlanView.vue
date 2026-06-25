<script setup lang="ts">
import {
  IconCheckCircleFill,
  IconClockCircle,
  IconCloseCircleFill,
  IconExclamationCircleFill,
  IconInfoCircle,
  IconPlayCircle,
  IconRefresh,
  IconThunderbolt,
} from '@arco-design/web-vue/es/icon'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  approveAgentApprovalRecord,
  rejectAgentApprovalRecord,
} from '@/api/approvals'
import {
  agentProgressStreamURL,
  cancelAgentScheduledTask,
  createAgentTask,
  getAgentProgress,
  listAgentEvalRuns,
  listAgentTasks,
  recoverAgentPlan,
  recoverAgentScheduledTask,
  retryAgentPlan,
  retryAgentPlanStep,
  runBuiltinAgentEval,
  type AgentAlertChannel,
  type AgentAlertAutoRecovery,
  type AgentAlertDedupeEscalation,
  type AgentAlertEscalationReceipt,
  type AgentAlertEscalationPolicy,
  type AgentAlertPolicy,
  type AgentAlertSummary,
  type AgentButtonLoop,
  type AgentButtonCallback,
  type AgentButtonDirectControl,
  type AgentCallbackReplayApproval,
  type AgentCallbackReplayExecution,
  type AgentCallbackReplayPermission,
  type AgentCallbackReplayResultQuery,
  type AgentCallbackReplayResultTrace,
  type AgentCallbackReplaySafetyAudit,
  type AgentCallbackReplayTool,
  type AgentCostSummary,
  type AgentCostTrendBucket,
  type AgentDailyPersist,
  type AgentDailyReport,
  type AgentDailySend,
  type AgentDeploymentVerification,
  type AgentDualEndProgressEvidence,
  type AgentDualEndInteraction,
  type AgentDualEndInteractionLaunch,
  type AgentDualEndRealInteraction,
  type AgentDualEndRunLoop,
  type AgentDualEndTaskClosure,
  type AgentE2EAcceptance,
  type AgentEvidenceDetailPage,
  type AgentEvalTrend,
  type AgentEvalRun,
  type AgentExternalMonitorIntegration,
  type AgentExternalMonitorRuntime,
  type AgentFeedbackTicketLifecycle,
  type AgentFeedbackSLAReport,
  type AgentLoadTestSummary,
  type AgentLaunchDrillRecord,
  type AgentLaunchApproval,
  type AgentOperationsDailyClosure,
  type AgentOperationsClosedLoop,
  type AgentOperationsHandoff,
  type AgentLaunchRuntimeOverview,
  type AgentOperationsExecution,
  type AgentOperationsEvidence,
  type AgentMonitorAutoReport,
  type AgentOpsAcceptance,
  type AgentPlan,
  type AgentPlanStep,
  type AgentPostLaunchMonitor,
  type AgentPreprodAcceptance,
  type AgentProductionRelease,
  type AgentProductionExecution,
  type AgentProductionDrill,
  type AgentProgressSnapshot,
  type AgentRealIntegration,
  type AgentRealInteractionAutomation,
  type AgentReleaseApproval,
  type AgentReleaseWindowExecution,
  type AgentReleaseWindowReadiness,
  type AgentRecoveryPolicyConfig,
  type AgentRecoveryAutomationExecution,
  type AgentRecoveryPolicyAutomation,
  type AgentRecoveryPolicyAudit,
  type AgentRecoveryPolicyGrayRelease,
  type AgentRecoveryPolicyPersist,
  type AgentRecoveryPolicyVersion,
  type AgentRun,
  type AgentSLASummary,
  type AgentTaskReport,
  type AgentTaskSummary,
  type AgentTrendSnapshot,
  type AgentUnifiedProgressComponent,
  type AgentWeChatCallbackReadiness,
  type AgentWeChatComponentSet,
  type AgentWeChatNativeActionSet,
  type AgentWeChatNativeIntegration,
  type AgentWeChatNativePayload,
  type AgentWebEvidenceInteraction,
  type AgentWebEvidenceDetailView,
  type AgentWebEvidenceInteractionDetail,
  type AgentWebEvidenceOperation,
  type AgentWebEvidenceUserAction,
  type AgentWebEvidenceRoute,
  type AgentMonitorAlertDrill,
  type AgentMonitorIntegration,
  type AgentMonitorReadback,
  type AgentOpsDashboardInteraction,
  type AgentOpsActionDefinition,
  type AgentOpsAPIExecution,
  type AgentOpsExecutionRecord,
  type AgentOpsPanelConfig,
  type AgentOperationsActionClosure,
  type AgentOperationsHandling,
  type AgentOperationsRuntimeClosure,
  type AgentWeChatE2EAcceptance,
  type AgentWeChatAcceptanceReview,
  type AgentWeChatFinalReport,
  type AgentWeChatUserFeedback,
  type AgentWeChatFeedbackLoop,
  type AgentWeChatFeedbackTicket,
  type AgentWeChatApprovalCallback,
  type AgentWeChatProgressCard,
  type AgentWeChatTemplateIntegration,
  type AgentWeChatTemplatePilot,
  type AgentWeChatTemplatePilotMetric,
  type AgentWeChatTemplateSend,
  type AgentWeChatTemplateRender,
  type AgentWeChatWebProgressLink,
  type AgentWeChatSignoff,
  type AgentWriteApprovalButton,
  type AgentWriteAuditReview,
  type AgentWriteExecute,
  type AgentWriteGrayPolicy,
  type AgentWriteGrayExpansion,
  type AgentWriteGrayReview,
  type AgentWriteLeastPrivilege,
  type AgentWriteRamp,
  type AgentWriteRampPolicy,
  type AgentWriteRampRecommendation,
  type AgentWriteRampStage,
  type AgentWriteStageApproval,
  type AgentWriteStageRecord,
  type AgentFeedbackTicketSLA,
  type AgentWriteReplay,
  type AgentWriteSandbox,
  type AgentExternalMonitorConfig,
  type AgentRuntimeParameters,
} from '@/api/agent'
import { formatAPIError } from '@/api/client'
import { isTerminalAgentProgressStatus, resolveAgentProgressPollingInterval } from '@/utils/agentProgress'

const route = useRoute()
const router = useRouter()
const progress = ref<AgentProgressSnapshot | null>(null)
const plan = ref<AgentPlan | null>(null)
const runs = ref<AgentRun[]>([])
const loading = ref(false)
const refreshing = ref(false)
const taskMessage = ref('')
const taskSubmitting = ref(false)
const taskError = ref('')
const taskNotice = ref('')
const tasks = ref<AgentTaskSummary[]>([])
const taskSLA = ref<AgentSLASummary | null>(null)
const taskCost = ref<AgentCostSummary | null>(null)
const taskAlerts = ref<AgentAlertSummary | null>(null)
const taskAlertPolicy = ref<AgentAlertPolicy | null>(null)
const taskCostTrend = ref<AgentCostTrendBucket[]>([])
const taskTrendSnapshot = ref<AgentTrendSnapshot | null>(null)
const taskDeployment = ref<AgentDeploymentVerification | null>(null)
const taskDrill = ref<AgentProductionDrill | null>(null)
const taskWeChatComponents = ref<AgentWeChatComponentSet | null>(null)
const taskLoadTest = ref<AgentLoadTestSummary | null>(null)
const taskWeChatCallback = ref<AgentWeChatCallbackReadiness | null>(null)
const taskWriteSandbox = ref<AgentWriteSandbox | null>(null)
const taskE2E = ref<AgentE2EAcceptance | null>(null)
const taskRealIntegration = ref<AgentRealIntegration | null>(null)
const taskWeChatNative = ref<AgentWeChatNativeActionSet | null>(null)
const taskWriteLeastPrivilege = ref<AgentWriteLeastPrivilege | null>(null)
const taskOpsAcceptance = ref<AgentOpsAcceptance | null>(null)
const taskWeChatNativePayload = ref<AgentWeChatNativePayload | null>(null)
const taskWriteGray = ref<AgentWriteGrayPolicy | null>(null)
const taskAlertChannel = ref<AgentAlertChannel | null>(null)
const taskLaunchDrill = ref<AgentLaunchDrillRecord | null>(null)
const taskWeChatNativeIntegration = ref<AgentWeChatNativeIntegration | null>(null)
const taskWriteReplay = ref<AgentWriteReplay | null>(null)
const taskLaunchApproval = ref<AgentLaunchApproval | null>(null)
const taskDailyReport = ref<AgentDailyReport | null>(null)
const taskPreprod = ref<AgentPreprodAcceptance | null>(null)
const taskButtonLoop = ref<AgentButtonLoop | null>(null)
const taskWriteExecute = ref<AgentWriteExecute | null>(null)
const taskDailyPersist = ref<AgentDailyPersist | null>(null)
const taskPostLaunchMonitor = ref<AgentPostLaunchMonitor | null>(null)
const taskReleaseApproval = ref<AgentReleaseApproval | null>(null)
const taskButtonCallback = ref<AgentButtonCallback | null>(null)
const taskWriteAudit = ref<AgentWriteAuditReview | null>(null)
const taskDailySend = ref<AgentDailySend | null>(null)
const taskMonitorAlert = ref<AgentMonitorAlertDrill | null>(null)
const taskButtonDirectControl = ref<AgentButtonDirectControl | null>(null)
const taskWeChatE2E = ref<AgentWeChatE2EAcceptance | null>(null)
const taskReleaseWindow = ref<AgentReleaseWindowReadiness | null>(null)
const taskWriteGrayExpansion = ref<AgentWriteGrayExpansion | null>(null)
const taskExternalMonitor = ref<AgentExternalMonitorIntegration | null>(null)
const taskReleaseWindowExecution = ref<AgentReleaseWindowExecution | null>(null)
const taskExternalMonitorRuntime = ref<AgentExternalMonitorRuntime | null>(null)
const taskWriteGrayReview = ref<AgentWriteGrayReview | null>(null)
const taskWeChatAcceptanceReview = ref<AgentWeChatAcceptanceReview | null>(null)
const taskOperationsDailyClosure = ref<AgentOperationsDailyClosure | null>(null)
const taskProductionRelease = ref<AgentProductionRelease | null>(null)
const taskExternalMonitorConfig = ref<AgentExternalMonitorConfig | null>(null)
const taskWriteRamp = ref<AgentWriteRamp | null>(null)
const taskWeChatSignoff = ref<AgentWeChatSignoff | null>(null)
const taskOperationsHandoff = ref<AgentOperationsHandoff | null>(null)
const taskProductionExecution = ref<AgentProductionExecution | null>(null)
const taskMonitorIntegration = ref<AgentMonitorIntegration | null>(null)
const taskWriteRampPolicy = ref<AgentWriteRampPolicy | null>(null)
const taskWeChatFinalReport = ref<AgentWeChatFinalReport | null>(null)
const taskLaunchRuntimeOverview = ref<AgentLaunchRuntimeOverview | null>(null)
const taskRuntimeParameters = ref<AgentRuntimeParameters | null>(null)
const taskMonitorReadback = ref<AgentMonitorReadback | null>(null)
const taskWriteRampRecommendation = ref<AgentWriteRampRecommendation | null>(null)
const taskWeChatUserFeedback = ref<AgentWeChatUserFeedback | null>(null)
const taskOperationsRuntimeClosure = ref<AgentOperationsRuntimeClosure | null>(null)
const taskOpsPanelConfig = ref<AgentOpsPanelConfig | null>(null)
const taskMonitorAutoReport = ref<AgentMonitorAutoReport | null>(null)
const taskWriteRampStage = ref<AgentWriteRampStage | null>(null)
const taskWeChatFeedbackLoop = ref<AgentWeChatFeedbackLoop | null>(null)
const taskOperationsClosedLoop = ref<AgentOperationsClosedLoop | null>(null)
const taskOpsDashboardInteraction = ref<AgentOpsDashboardInteraction | null>(null)
const taskAlertDedupeEscalation = ref<AgentAlertDedupeEscalation | null>(null)
const taskWriteStageRecord = ref<AgentWriteStageRecord | null>(null)
const taskWeChatFeedbackTicket = ref<AgentWeChatFeedbackTicket | null>(null)
const taskOperationsHandling = ref<AgentOperationsHandling | null>(null)
const taskOpsActionDefinition = ref<AgentOpsActionDefinition | null>(null)
const taskAlertEscalationPolicy = ref<AgentAlertEscalationPolicy | null>(null)
const taskWriteStageApproval = ref<AgentWriteStageApproval | null>(null)
const taskFeedbackTicketLifecycle = ref<AgentFeedbackTicketLifecycle | null>(null)
const taskOperationsActionClosure = ref<AgentOperationsActionClosure | null>(null)
const taskOpsAPIExecution = ref<AgentOpsAPIExecution | null>(null)
const taskAlertEscalationReceipt = ref<AgentAlertEscalationReceipt | null>(null)
const taskWriteApprovalButton = ref<AgentWriteApprovalButton | null>(null)
const taskFeedbackTicketSLA = ref<AgentFeedbackTicketSLA | null>(null)
const taskOperationsExecution = ref<AgentOperationsExecution | null>(null)
const taskOpsExecutionRecord = ref<AgentOpsExecutionRecord | null>(null)
const taskWeChatApprovalCallback = ref<AgentWeChatApprovalCallback | null>(null)
const taskFeedbackSLAReport = ref<AgentFeedbackSLAReport | null>(null)
const taskAlertAutoRecovery = ref<AgentAlertAutoRecovery | null>(null)
const taskOperationsEvidence = ref<AgentOperationsEvidence | null>(null)
const taskUnifiedProgressComponent = ref<AgentUnifiedProgressComponent | null>(null)
const taskEvidenceDetailPage = ref<AgentEvidenceDetailPage | null>(null)
const taskCallbackReplayTool = ref<AgentCallbackReplayTool | null>(null)
const taskRecoveryPolicyConfig = ref<AgentRecoveryPolicyConfig | null>(null)
const taskDualEndProgressEvidence = ref<AgentDualEndProgressEvidence | null>(null)
const taskWeChatProgressCard = ref<AgentWeChatProgressCard | null>(null)
const taskWebEvidenceInteraction = ref<AgentWebEvidenceInteraction | null>(null)
const taskCallbackReplayPermission = ref<AgentCallbackReplayPermission | null>(null)
const taskRecoveryPolicyAudit = ref<AgentRecoveryPolicyAudit | null>(null)
const taskDualEndInteraction = ref<AgentDualEndInteraction | null>(null)
const taskWeChatTemplateRender = ref<AgentWeChatTemplateRender | null>(null)
const taskWebEvidenceRoute = ref<AgentWebEvidenceRoute | null>(null)
const taskCallbackReplayApproval = ref<AgentCallbackReplayApproval | null>(null)
const taskRecoveryPolicyPersist = ref<AgentRecoveryPolicyPersist | null>(null)
const taskDualEndInteractionLaunch = ref<AgentDualEndInteractionLaunch | null>(null)
const taskWeChatTemplateSend = ref<AgentWeChatTemplateSend | null>(null)
const taskWebEvidenceDetailView = ref<AgentWebEvidenceDetailView | null>(null)
const taskCallbackReplayExecution = ref<AgentCallbackReplayExecution | null>(null)
const taskRecoveryPolicyVersion = ref<AgentRecoveryPolicyVersion | null>(null)
const taskDualEndRealInteraction = ref<AgentDualEndRealInteraction | null>(null)
const taskWeChatTemplateIntegration = ref<AgentWeChatTemplateIntegration | null>(null)
const taskWebEvidenceInteractionDetail = ref<AgentWebEvidenceInteractionDetail | null>(null)
const taskCallbackReplaySafetyAudit = ref<AgentCallbackReplaySafetyAudit | null>(null)
const taskRecoveryPolicyGrayRelease = ref<AgentRecoveryPolicyGrayRelease | null>(null)
const taskDualEndRunLoop = ref<AgentDualEndRunLoop | null>(null)
const taskWeChatTemplatePilot = ref<AgentWeChatTemplatePilot | null>(null)
const taskWebEvidenceUserAction = ref<AgentWebEvidenceUserAction | null>(null)
const taskCallbackReplayResultTrace = ref<AgentCallbackReplayResultTrace | null>(null)
const taskRecoveryPolicyAutomation = ref<AgentRecoveryPolicyAutomation | null>(null)
const taskDualEndTaskClosure = ref<AgentDualEndTaskClosure | null>(null)
const taskWeChatTemplatePilotMetric = ref<AgentWeChatTemplatePilotMetric | null>(null)
const taskWebEvidenceOperation = ref<AgentWebEvidenceOperation | null>(null)
const taskCallbackReplayResultQuery = ref<AgentCallbackReplayResultQuery | null>(null)
const taskRecoveryAutomationExecution = ref<AgentRecoveryAutomationExecution | null>(null)
const taskRealInteractionAutomation = ref<AgentRealInteractionAutomation | null>(null)
const taskWeChatWebProgressLink = ref<AgentWeChatWebProgressLink | null>(null)
const taskReport = ref<AgentTaskReport | null>(null)
const tasksLoading = ref(false)
const tasksError = ref('')
const evalRuns = ref<AgentEvalRun[]>([])
const evalTrend = ref<AgentEvalTrend | null>(null)
const evalRunDetail = ref<AgentEvalRun | null>(null)
const evalLoading = ref(false)
const evalRunning = ref(false)
const evalError = ref('')
const controlError = ref('')
const decidingApprovalID = ref(0)
const cancelingTaskID = ref(0)
const recoveringTaskID = ref(0)
const retryingStepID = ref(0)
const retryingPlan = ref(false)
const recoveringPlan = ref(false)
const errorMessage = ref('')
const refreshNotice = ref('')
const lastLoadedAt = ref('')
const streamStatus = ref<'idle' | 'connecting' | 'connected' | 'fallback' | 'closed'>('idle')
const streamError = ref('')
const streamCursor = ref('')
let requestSeq = 0
let pollTimer: number | undefined
let progressStream: EventSource | undefined
let progressStreamKey = ''

const planID = computed(() => {
  const value = route.params.id
  const id = typeof value === 'string' ? Number(value) : 0
  return Number.isFinite(id) ? id : 0
})

const scheduledTaskID = computed(() => {
  const value = route.query.scheduled_task_id
  const raw = typeof value === 'string' ? value : ''
  const id = Number(raw)
  return Number.isFinite(id) ? id : 0
})

const sortedSteps = computed(() => {
  return [...(plan.value?.steps || [])].sort((left, right) => left.step_order - right.step_order || left.id - right.id)
})

const completedStepCount = computed(
  () => sortedSteps.value.filter((step) => step.status === 'completed').length,
)
const failedStepCount = computed(() => sortedSteps.value.filter((step) => step.status === 'failed').length)
const activeStepCount = computed(() => sortedSteps.value.filter((step) => step.status === 'executing').length)
const retryableStepCount = computed(() => sortedSteps.value.filter((step) => isRetryableStep(step)).length)

const progressPercent = computed(() => {
  const total = sortedSteps.value.length
  if (!total) {
    return 0
  }
  return Math.round((completedStepCount.value / total) * 100)
})

const orderedRuns = computed(() => {
  return [...runs.value].sort((left, right) => left.id - right.id)
})

const controllerRun = computed(() => orderedRuns.value.find((run) => run.role === 'controller') || null)
const executorRuns = computed(() => orderedRuns.value.filter((run) => run.role === 'executor'))
const scheduledTasks = computed(() => progress.value?.scheduled_tasks || [])
const progressPhases = computed(() => progress.value?.phases || [])
const recentEvents = computed(() => progress.value?.recent_events || [])

const isTerminalPlan = computed(() => {
  const status = progress.value?.status || plan.value?.status
  return isTerminalAgentProgressStatus(status || '')
})

const statusMeta = computed(() => {
  const currentPlan = plan.value
  if (!currentPlan && !progress.value) {
    return '尚未加载'
  }
  if (progress.value?.next_action) {
    return progress.value.next_action
  }
  const total = sortedSteps.value.length
  if (!total) {
    return statusLabel(currentPlan?.status || progress.value?.status || '')
  }
  return `${completedStepCount.value}/${total} 步完成`
})

const streamStatusLabel = computed(() => {
  const labels: Record<string, string> = {
    idle: '未连接',
    connecting: '连接中',
    connected: '实时连接',
    fallback: '轮询降级',
    closed: '已关闭',
  }
  return labels[streamStatus.value] || streamStatus.value
})

const canRetryPlan = computed(() => Boolean(plan.value && plan.value.status === 'failed' && retryableStepCount.value > 0))
const canRecoverPlan = computed(() =>
  Boolean(plan.value && (plan.value.status === 'executing' || sortedSteps.value.some((step) => step.status === 'executing'))),
)
const multiTurnMetadata = computed(() => {
  const value = plan.value?.metadata?.multi_turn
  return isRecord(value) ? value : null
})
const multiTurnOriginalGoal = computed(() => asText(multiTurnMetadata.value?.original_goal))
const multiTurnLatestInstruction = computed(() => asText(multiTurnMetadata.value?.latest_user_instruction))
const multiTurnStoppedReason = computed(() => asText(multiTurnMetadata.value?.stopped_reason))
const multiTurnAppendedInputs = computed(() => metadataEntryMessages(multiTurnMetadata.value?.appended_inputs))
const multiTurnFollowupQuestions = computed(() => metadataEntryMessages(multiTurnMetadata.value?.followup_questions))
const parentPlanMetadata = computed(() => {
  const value = plan.value?.metadata?.parent_plan
  return isRecord(value) ? value : null
})
const resultReuseMetadata = computed(() => {
  const direct = plan.value?.metadata?.result_reuse
  if (isRecord(direct)) {
    return direct
  }
  const nested = multiTurnMetadata.value?.result_reuse
  return isRecord(nested) ? nested : null
})
const resultReuseEvidenceRefs = computed(() => metadataStringList(resultReuseMetadata.value?.evidence_refs))
const parentPlanEvidenceRefs = computed(() => metadataStringList(parentPlanMetadata.value?.evidence_refs))
const resultFreshnessStatus = computed(() => asText(resultReuseMetadata.value?.freshness_status || parentPlanMetadata.value?.freshness_status))
const resultFreshnessHint = computed(() => asText(resultReuseMetadata.value?.freshness_hint || parentPlanMetadata.value?.freshness_hint))
const parentPlanID = computed(() => asNumber(parentPlanMetadata.value?.id))
const parentPlanGoal = computed(() => asText(parentPlanMetadata.value?.goal))
const permissionGovernance = computed(() => {
  const value = plan.value?.metadata?.permission_governance
  return isRecord(value) ? value : null
})
const budgetGovernance = computed(() => {
  const value = plan.value?.metadata?.budget_governance
  return isRecord(value) ? value : null
})
const resultQuality = computed(() => {
  const value = plan.value?.metadata?.result_quality
  return isRecord(value) ? value : null
})
const recoveryMetadata = computed(() => {
  const value = plan.value?.metadata?.recovery
  return isRecord(value) ? value : null
})
const deploymentAcceptance = computed(() => {
  const value = plan.value?.metadata?.deployment_acceptance
  return isRecord(value) ? value : null
})
const runtimeObservability = computed(() => {
  const value = plan.value?.metadata?.runtime_observability
  return isRecord(value) ? value : null
})
const handoffMetadata = computed(() => {
  const value = plan.value?.metadata?.handoff
  return isRecord(value) ? value : null
})
const costSummaryMetadata = computed(() => {
  const value = plan.value?.metadata?.cost_summary
  return isRecord(value) ? value : null
})
const planPermissionSummary = computed(() => {
  if (!permissionGovernance.value) {
    return ''
  }
  const parts = []
  if (permissionGovernance.value.has_external_access === true) {
    parts.push('外部只读')
  }
  if (permissionGovernance.value.has_state_change === true) {
    parts.push('状态变更')
  }
  if (permissionGovernance.value.requires_confirmation === true) {
    parts.push('需确认')
  }
  return parts.length ? parts.join(' / ') : '只读可执行'
})
const planBudgetSummary = computed(() => {
  if (!budgetGovernance.value) {
    return ''
  }
  const status = asText(budgetGovernance.value.status)
  const toolCalls = asNumber(budgetGovernance.value.tool_calls)
  const toolBudget = asNumber(budgetGovernance.value.tool_call_budget)
  const externalCalls = asNumber(budgetGovernance.value.external_calls)
  const externalBudget = asNumber(budgetGovernance.value.external_call_budget)
  return `${status || 'unknown'} / 工具 ${toolCalls}/${toolBudget} / 联网 ${externalCalls}/${externalBudget}`
})
const planQualitySummary = computed(() => {
  if (!resultQuality.value) {
    return ''
  }
  const status = asText(resultQuality.value.status)
  const score = asNumber(resultQuality.value.score)
  const evidence = asNumber(resultQuality.value.evidence_completeness)
  const coverage = asNumber(resultQuality.value.goal_coverage)
  return `${status || 'unknown'} / score ${score.toFixed(2)} / 证据 ${evidence.toFixed(2)} / 覆盖 ${coverage.toFixed(2)}`
})
const planRecoverySummary = computed(() => {
  if (!recoveryMetadata.value) {
    return ''
  }
  const strategy = asText(recoveryMetadata.value.recovery_strategy)
  const result = asText(recoveryMetadata.value.recovery_result)
  const reason = asText(recoveryMetadata.value.recovery_reason)
  return `${strategy || 'unknown'} / ${result || 'unknown'}${reason ? ` / ${reason}` : ''}`
})
const deploymentAcceptanceSummary = computed(() => {
  if (!deploymentAcceptance.value) {
    return ''
  }
  const status = asText(deploymentAcceptance.value.status)
  const checks = metadataRecordList(deploymentAcceptance.value.checks)
  return `${status || 'unknown'} / ${checks.length} 项检查`
})
const deploymentAcceptanceChecks = computed(() => metadataRecordList(deploymentAcceptance.value?.checks))
const runtimeObservabilitySummary = computed(() => asText(runtimeObservability.value?.summary))
const handoffSummary = computed(() => {
  if (!handoffMetadata.value) {
    return ''
  }
  const status = asText(handoffMetadata.value.status)
  const nextAction = asText(handoffMetadata.value.next_action)
  return `${status || 'unknown'}${nextAction ? ` / ${nextAction}` : ''}`
})
const planCostSummary = computed(() => {
  if (!costSummaryMetadata.value) {
    return ''
  }
  return `工具 ${asNumber(costSummaryMetadata.value.tool_calls)} / 外部 ${asNumber(costSummaryMetadata.value.external_calls)} / token ${asNumber(costSummaryMetadata.value.estimated_tokens)} / 重试 ${asNumber(costSummaryMetadata.value.retry_count)} / 通知 ${asNumber(costSummaryMetadata.value.notification_count)}`
})
const taskAlertSummary = computed(() => {
  if (!taskAlerts.value) {
    return ''
  }
  const reasons = taskAlerts.value.reasons.map((reason) => alertReasonLabel(reason)).join('、') || '无'
  return `总计 ${taskAlerts.value.total} / 严重 ${taskAlerts.value.critical} / 警告 ${taskAlerts.value.warning} / 原因 ${reasons}`
})
const taskCostTrendSummary = computed(() => {
  if (!taskCostTrend.value.length) {
    return ''
  }
  return taskCostTrend.value
    .map(
      (bucket) =>
        `${bucket.date} 工具 ${bucket.tool_calls} / 外部 ${bucket.external_calls} / token ${bucket.estimated_tokens} / 重试 ${bucket.retry_count} / 通知 ${bucket.notification_count}`,
    )
    .join('；')
})
const taskDeploymentSummary = computed(() => {
  if (!taskDeployment.value) {
    return ''
  }
  return `${statusLabel(taskDeployment.value.status)} / ${taskDeployment.value.summary || `${taskDeployment.value.checks.length} 项检查`}`
})
const visibleDeploymentChecks = computed(() => taskDeployment.value?.checks?.slice(0, 8) || [])
const taskAlertPolicySummary = computed(() => {
  if (!taskAlertPolicy.value) {
    return ''
  }
  const enabled = taskAlertPolicy.value.enabled_reasons.map((reason) => alertReasonLabel(reason)).join('、') || '无'
  const muted = taskAlertPolicy.value.muted_reasons.map((reason) => alertReasonLabel(reason)).join('、') || '无'
  return `${statusLabel(taskAlertPolicy.value.status)} / 启用 ${enabled} / 静默 ${muted}`
})
const taskTrendSnapshotSummary = computed(() => {
  const buckets = taskTrendSnapshot.value?.buckets || []
  if (!taskTrendSnapshot.value || !buckets.length) {
    return ''
  }
  const latest = buckets[buckets.length - 1]
  return `${taskTrendSnapshot.value.retention_days} 日留存 / ${taskTrendSnapshot.value.summary} / 最新 ${latest.date} 失败 ${latest.plan_failed + latest.scheduled_task_failed} / 通知失败 ${latest.notification_failed} / 恢复 ${latest.recovery_count} / 接管 ${latest.handoff_count}`
})
const taskDrillSummary = computed(() => {
  if (!taskDrill.value) {
    return ''
  }
  return `${statusLabel(taskDrill.value.status)} / ${taskDrill.value.summary} / ${taskDrill.value.source}`
})
const visibleDrillChecks = computed(() => taskDrill.value?.checks?.slice(0, 8) || [])
const taskWeChatComponentSummary = computed(() => {
  if (!taskWeChatComponents.value) {
    return ''
  }
  return `${taskWeChatComponents.value.mode} / ${taskWeChatComponents.value.summary}`
})
const visibleWeChatActions = computed(() => taskWeChatComponents.value?.actions?.slice(0, 6) || [])
const taskLoadTestSummary = computed(() => {
  if (!taskLoadTest.value) {
    return ''
  }
  const metrics = taskLoadTest.value.metrics
  return `${statusLabel(taskLoadTest.value.status)} / ${taskLoadTest.value.summary} / 用户 ${metrics.users} / Web ${metrics.web_tasks} / 企微 ${metrics.wechat_tasks} / 定时 ${metrics.scheduled_tasks} / 恢复 ${metrics.recovery_events}`
})
const visibleLoadTestChecks = computed(() => taskLoadTest.value?.checks?.slice(0, 8) || [])
const taskWeChatCallbackSummary = computed(() => {
  if (!taskWeChatCallback.value) {
    return ''
  }
  return `${statusLabel(taskWeChatCallback.value.status)} / ${taskWeChatCallback.value.summary}`
})
const visibleWeChatCallbackChecks = computed(() => taskWeChatCallback.value?.checks?.slice(0, 8) || [])
const taskWriteSandboxSummary = computed(() => {
  if (!taskWriteSandbox.value) {
    return ''
  }
  return `${statusLabel(taskWriteSandbox.value.status)} / ${taskWriteSandbox.value.default_action} / ${taskWriteSandbox.value.summary}`
})
const visibleWriteSandboxChecks = computed(() => taskWriteSandbox.value?.checks?.slice(0, 8) || [])
const taskE2ESummary = computed(() => {
  if (!taskE2E.value) {
    return ''
  }
  return `${statusLabel(taskE2E.value.status)} / ${taskE2E.value.summary}`
})
const visibleE2EChecks = computed(() => taskE2E.value?.checks?.slice(0, 8) || [])
const taskRealIntegrationSummary = computed(() => {
  if (!taskRealIntegration.value) {
    return ''
  }
  return `${statusLabel(taskRealIntegration.value.status)} / ${taskRealIntegration.value.summary} / 风险 ${taskRealIntegration.value.risks.length} / 阻断 ${taskRealIntegration.value.blockers.length} / ${taskRealIntegration.value.next_action}`
})
const visibleRealIntegrationChecks = computed(() => taskRealIntegration.value?.checks?.slice(0, 8) || [])
const taskWeChatNativeSummary = computed(() => {
  if (!taskWeChatNative.value) {
    return ''
  }
  return `${taskWeChatNative.value.mode} / ${taskWeChatNative.value.summary}`
})
const visibleWeChatNativeActions = computed(() => taskWeChatNative.value?.actions?.slice(0, 6) || [])
const taskWriteLeastPrivilegeSummary = computed(() => {
  if (!taskWriteLeastPrivilege.value) {
    return ''
  }
  const allowed = taskWriteLeastPrivilege.value.allowed_candidates.join('、') || '无'
  const denied = taskWriteLeastPrivilege.value.denied_patterns.join('、') || '无'
  return `${statusLabel(taskWriteLeastPrivilege.value.status)} / ${taskWriteLeastPrivilege.value.default_action} / 允许 ${allowed} / 拒绝 ${denied}`
})
const visibleWriteLeastPrivilegeChecks = computed(() => taskWriteLeastPrivilege.value?.checks?.slice(0, 8) || [])
const taskOpsAcceptanceSummary = computed(() => {
  if (!taskOpsAcceptance.value) {
    return ''
  }
  return `${statusLabel(taskOpsAcceptance.value.status)} / ${taskOpsAcceptance.value.summary}`
})
const visibleOpsAcceptanceChecks = computed(() => taskOpsAcceptance.value?.checks?.slice(0, 8) || [])
const taskWeChatNativePayloadSummary = computed(() => {
  if (!taskWeChatNativePayload.value) {
    return ''
  }
  return `${statusLabel(taskWeChatNativePayload.value.status)} / ${taskWeChatNativePayload.value.message_type} / ${taskWeChatNativePayload.value.summary}`
})
const visibleWeChatNativeButtons = computed(() => taskWeChatNativePayload.value?.buttons?.slice(0, 6) || [])
const taskWriteGraySummary = computed(() => {
  if (!taskWriteGray.value) {
    return ''
  }
  const gates = [
    taskWriteGray.value.requires_approval ? '审批' : '',
    taskWriteGray.value.requires_budget ? '预算' : '',
    taskWriteGray.value.requires_audit ? '审计' : '',
  ].filter(Boolean).join('、') || '无'
  return `${statusLabel(taskWriteGray.value.status)} / ${taskWriteGray.value.allowed_user_scope} / 候选 ${taskWriteGray.value.candidates.join('、') || '无'} / 约束 ${gates}`
})
const visibleWriteGrayChecks = computed(() => taskWriteGray.value?.checks?.slice(0, 8) || [])
const taskAlertChannelSummary = computed(() => {
  if (!taskAlertChannel.value) {
    return ''
  }
  return `${statusLabel(taskAlertChannel.value.status)} / ${taskAlertChannel.value.summary}`
})
const visibleAlertChannels = computed(() => taskAlertChannel.value?.channels?.slice(0, 6) || [])
const taskLaunchDrillSummary = computed(() => {
  if (!taskLaunchDrill.value) {
    return ''
  }
  return `${statusLabel(taskLaunchDrill.value.status)} / ${taskLaunchDrill.value.batch_id} / ${taskLaunchDrill.value.result} / 风险 ${taskLaunchDrill.value.risks.length} / 阻断 ${taskLaunchDrill.value.blockers.length} / ${taskLaunchDrill.value.next_action}`
})
const visibleLaunchDrillChecks = computed(() => taskLaunchDrill.value?.checks?.slice(0, 8) || [])
const taskWeChatNativeIntegrationSummary = computed(() => {
  if (!taskWeChatNativeIntegration.value) {
    return ''
  }
  return `${statusLabel(taskWeChatNativeIntegration.value.status)} / ${taskWeChatNativeIntegration.value.summary} / 风险 ${taskWeChatNativeIntegration.value.risks.length} / 阻断 ${taskWeChatNativeIntegration.value.blockers.length} / ${taskWeChatNativeIntegration.value.next_action}`
})
const visibleWeChatNativeIntegrationChecks = computed(() => taskWeChatNativeIntegration.value?.checks?.slice(0, 8) || [])
const taskWriteReplaySummary = computed(() => {
  if (!taskWriteReplay.value) {
    return ''
  }
  return `${statusLabel(taskWriteReplay.value.status)} / 候选 ${taskWriteReplay.value.candidates.join('、') || '无'} / 审批 ${taskWriteReplay.value.approval_status} / 预算 ${taskWriteReplay.value.budget_status} / 权限 ${taskWriteReplay.value.permission_status} / 执行 ${taskWriteReplay.value.execution_status} / 审计 ${taskWriteReplay.value.audit_status}`
})
const visibleWriteReplayChecks = computed(() => taskWriteReplay.value?.checks?.slice(0, 8) || [])
const taskLaunchApprovalSummary = computed(() => {
  if (!taskLaunchApproval.value) {
    return ''
  }
  return `${statusLabel(taskLaunchApproval.value.status)} / ${taskLaunchApproval.value.review_state} / 批准 ${taskLaunchApproval.value.approved} / 拒绝 ${taskLaunchApproval.value.rejected} / 过期 ${taskLaunchApproval.value.expired}`
})
const visibleLaunchApprovalChecks = computed(() => taskLaunchApproval.value?.checks?.slice(0, 8) || [])
const taskDailyReportSummary = computed(() => {
  if (!taskDailyReport.value) {
    return ''
  }
  return `${statusLabel(taskDailyReport.value.status)} / ${taskDailyReport.value.date} / 任务 ${taskDailyReport.value.task_count} / 成功率 ${taskDailyReport.value.success_rate.toFixed(2)} / 失败 ${taskDailyReport.value.failure_count} / 告警 ${taskDailyReport.value.alert_count} / token ${taskDailyReport.value.estimated_tokens}`
})
const visibleDailyReportChecks = computed(() => taskDailyReport.value?.checks?.slice(0, 8) || [])
const taskPreprodSummary = computed(() => {
  if (!taskPreprod.value) {
    return ''
  }
  return `${statusLabel(taskPreprod.value.status)} / ${taskPreprod.value.summary} / 风险 ${taskPreprod.value.risks.length} / 阻断 ${taskPreprod.value.blockers.length} / ${taskPreprod.value.next_action}`
})
const visiblePreprodChecks = computed(() => taskPreprod.value?.checks?.slice(0, 8) || [])
const taskButtonLoopSummary = computed(() => {
  if (!taskButtonLoop.value) {
    return ''
  }
  return `${statusLabel(taskButtonLoop.value.status)} / ${taskButtonLoop.value.summary} / 动作 ${taskButtonLoop.value.actions.length} / fallback ${taskButtonLoop.value.fallback_text ? '保留' : '缺失'}`
})
const visibleButtonLoopActions = computed(() => taskButtonLoop.value?.actions?.slice(0, 6) || [])
const visibleButtonLoopChecks = computed(() => taskButtonLoop.value?.checks?.slice(0, 8) || [])
const taskWriteExecuteSummary = computed(() => {
  if (!taskWriteExecute.value) {
    return ''
  }
  return `${statusLabel(taskWriteExecute.value.status)} / ${taskWriteExecute.value.default_action} / 候选 ${taskWriteExecute.value.candidates.join('、') || '无'} / 审批 ${taskWriteExecute.value.approval_status} / 预算 ${taskWriteExecute.value.budget_status} / 权限 ${taskWriteExecute.value.permission_status} / 执行 ${taskWriteExecute.value.execution_status} / 审计 ${taskWriteExecute.value.audit_status}`
})
const visibleWriteExecuteChecks = computed(() => taskWriteExecute.value?.checks?.slice(0, 8) || [])
const taskDailyPersistSummary = computed(() => {
  if (!taskDailyPersist.value) {
    return ''
  }
  return `${statusLabel(taskDailyPersist.value.status)} / ${taskDailyPersist.value.record_key} / ${taskDailyPersist.value.source} / ${taskDailyPersist.value.retained ? '已留存' : '未留存'}`
})
const visibleDailyPersistChecks = computed(() => taskDailyPersist.value?.checks?.slice(0, 8) || [])
const taskPostLaunchMonitorSummary = computed(() => {
  if (!taskPostLaunchMonitor.value) {
    return ''
  }
  return `${statusLabel(taskPostLaunchMonitor.value.status)} / ${taskPostLaunchMonitor.value.summary}`
})
const visiblePostLaunchMonitorChecks = computed(() => taskPostLaunchMonitor.value?.checks?.slice(0, 8) || [])
const taskReleaseApprovalSummary = computed(() => {
  if (!taskReleaseApproval.value) {
    return ''
  }
  return `${statusLabel(taskReleaseApproval.value.status)} / ${taskReleaseApproval.value.review_state} / 可执行 ${taskReleaseApproval.value.executable ? '是' : '否'} / 批准 ${taskReleaseApproval.value.approved} / 拒绝 ${taskReleaseApproval.value.rejected} / 过期 ${taskReleaseApproval.value.expired} / 审计 ${taskReleaseApproval.value.audit_event}`
})
const visibleReleaseApprovalChecks = computed(() => taskReleaseApproval.value?.checks?.slice(0, 8) || [])
const taskButtonCallbackSummary = computed(() => {
  if (!taskButtonCallback.value) {
    return ''
  }
  return `${statusLabel(taskButtonCallback.value.status)} / ${taskButtonCallback.value.summary} / 动作 ${taskButtonCallback.value.actions.length} / fallback ${taskButtonCallback.value.fallback_text ? '保留' : '缺失'}`
})
const visibleButtonCallbackActions = computed(() => taskButtonCallback.value?.actions?.slice(0, 6) || [])
const visibleButtonCallbackChecks = computed(() => taskButtonCallback.value?.checks?.slice(0, 8) || [])
const taskWriteAuditSummary = computed(() => {
  if (!taskWriteAudit.value) {
    return ''
  }
  return `${statusLabel(taskWriteAudit.value.status)} / 候选 ${taskWriteAudit.value.candidates.join('、') || '无'} / 审批 ${taskWriteAudit.value.approval_evidence} / 预算 ${taskWriteAudit.value.budget_evidence} / 权限 ${taskWriteAudit.value.permission_evidence} / 执行 ${taskWriteAudit.value.execution_evidence} / 失败 ${taskWriteAudit.value.failure_evidence} / 接管 ${taskWriteAudit.value.handoff_evidence}`
})
const visibleWriteAuditChecks = computed(() => taskWriteAudit.value?.checks?.slice(0, 8) || [])
const taskDailySendSummary = computed(() => {
  if (!taskDailySend.value) {
    return ''
  }
  return `${statusLabel(taskDailySend.value.status)} / ${taskDailySend.value.record_key} / 调度 ${taskDailySend.value.schedule_status} / 投递 ${taskDailySend.value.delivery_status} / 重试 ${taskDailySend.value.retry_status} / 企微 ${taskDailySend.value.wechat_report_status}`
})
const visibleDailySendChecks = computed(() => taskDailySend.value?.checks?.slice(0, 8) || [])
const taskMonitorAlertSummary = computed(() => {
  if (!taskMonitorAlert.value) {
    return ''
  }
  return `${statusLabel(taskMonitorAlert.value.status)} / ${taskMonitorAlert.value.summary} / 触发 ${taskMonitorAlert.value.trigger_status} / 通知 ${taskMonitorAlert.value.notification_status} / 接管 ${taskMonitorAlert.value.handoff_status}`
})
const visibleMonitorAlertChecks = computed(() => taskMonitorAlert.value?.checks?.slice(0, 8) || [])
const taskButtonDirectControlSummary = computed(() => {
  if (!taskButtonDirectControl.value) {
    return ''
  }
  return `${statusLabel(taskButtonDirectControl.value.status)} / ${taskButtonDirectControl.value.summary} / 执行 ${taskButtonDirectControl.value.executed} / 跳过 ${taskButtonDirectControl.value.skipped}`
})
const visibleButtonDirectControlActions = computed(() => taskButtonDirectControl.value?.actions?.slice(0, 6) || [])
const visibleButtonDirectControlChecks = computed(() => taskButtonDirectControl.value?.checks?.slice(0, 8) || [])
const taskWeChatE2ESummary = computed(() => {
  if (!taskWeChatE2E.value) {
    return ''
  }
  return `${statusLabel(taskWeChatE2E.value.status)} / ${taskWeChatE2E.value.summary}`
})
const visibleWeChatE2EChecks = computed(() => taskWeChatE2E.value?.checks?.slice(0, 8) || [])
const taskReleaseWindowSummary = computed(() => {
  if (!taskReleaseWindow.value) {
    return ''
  }
  return `${statusLabel(taskReleaseWindow.value.status)} / ${taskReleaseWindow.value.window_state} / ${taskReleaseWindow.value.summary}`
})
const visibleReleaseWindowChecks = computed(() => taskReleaseWindow.value?.checks?.slice(0, 8) || [])
const taskWriteGrayExpansionSummary = computed(() => {
  if (!taskWriteGrayExpansion.value) {
    return ''
  }
  return `${statusLabel(taskWriteGrayExpansion.value.status)} / ${taskWriteGrayExpansion.value.default_action} / 候选 ${taskWriteGrayExpansion.value.candidates.join('、') || '无'} / ${taskWriteGrayExpansion.value.summary}`
})
const visibleWriteGrayExpansionChecks = computed(() => taskWriteGrayExpansion.value?.checks?.slice(0, 8) || [])
const taskExternalMonitorSummary = computed(() => {
  if (!taskExternalMonitor.value) {
    return ''
  }
  return `${statusLabel(taskExternalMonitor.value.status)} / ${taskExternalMonitor.value.summary} / 指标 ${taskExternalMonitor.value.metrics.length} / 事件 ${taskExternalMonitor.value.alert_events.length} / 通道 ${taskExternalMonitor.value.channels.length}`
})
const visibleExternalMonitorChecks = computed(() => taskExternalMonitor.value?.checks?.slice(0, 8) || [])
const taskReleaseWindowExecutionSummary = computed(() => {
  if (!taskReleaseWindowExecution.value) {
    return ''
  }
  return `${statusLabel(taskReleaseWindowExecution.value.status)} / ${taskReleaseWindowExecution.value.execution_state} / 审批 ${taskReleaseWindowExecution.value.approval_status} / 回滚 ${taskReleaseWindowExecution.value.rollback_status} / 通知 ${taskReleaseWindowExecution.value.notification_status}`
})
const visibleReleaseWindowExecutionChecks = computed(() => taskReleaseWindowExecution.value?.checks?.slice(0, 8) || [])
const taskExternalMonitorRuntimeSummary = computed(() => {
  if (!taskExternalMonitorRuntime.value) {
    return ''
  }
  return `${statusLabel(taskExternalMonitorRuntime.value.status)} / ${taskExternalMonitorRuntime.value.summary} / SLA ${taskExternalMonitorRuntime.value.sla_status} / 队列 ${taskExternalMonitorRuntime.value.queue_status} / worker ${taskExternalMonitorRuntime.value.worker_status}`
})
const visibleExternalMonitorRuntimeChecks = computed(() => taskExternalMonitorRuntime.value?.checks?.slice(0, 9) || [])
const taskWriteGrayReviewSummary = computed(() => {
  if (!taskWriteGrayReview.value) {
    return ''
  }
  return `${statusLabel(taskWriteGrayReview.value.status)} / ${taskWriteGrayReview.value.decision} / 候选 ${taskWriteGrayReview.value.candidates.join('、') || '无'} / 拒绝 ${taskWriteGrayReview.value.denied_patterns.length}`
})
const visibleWriteGrayReviewChecks = computed(() => taskWriteGrayReview.value?.checks?.slice(0, 8) || [])
const taskWeChatAcceptanceReviewSummary = computed(() => {
  if (!taskWeChatAcceptanceReview.value) {
    return ''
  }
  return `${statusLabel(taskWeChatAcceptanceReview.value.status)} / 入口 ${taskWeChatAcceptanceReview.value.entry_status} / 进度 ${taskWeChatAcceptanceReview.value.progress_status} / 终报 ${taskWeChatAcceptanceReview.value.final_report_status} / ${taskWeChatAcceptanceReview.value.next_action}`
})
const visibleWeChatAcceptanceReviewChecks = computed(() => taskWeChatAcceptanceReview.value?.checks?.slice(0, 8) || [])
const taskOperationsDailyClosureSummary = computed(() => {
  if (!taskOperationsDailyClosure.value) {
    return ''
  }
  return `${statusLabel(taskOperationsDailyClosure.value.status)} / 日报 ${taskOperationsDailyClosure.value.report_status} / 监控 ${taskOperationsDailyClosure.value.monitor_status} / 发布 ${taskOperationsDailyClosure.value.release_window_status} / 审计 ${taskOperationsDailyClosure.value.audit_status}`
})
const visibleOperationsDailyClosureChecks = computed(() => taskOperationsDailyClosure.value?.checks?.slice(0, 8) || [])
const taskProductionReleaseSummary = computed(() => {
  if (!taskProductionRelease.value) {
    return ''
  }
  return `${statusLabel(taskProductionRelease.value.status)} / 批次 ${taskProductionRelease.value.batch_id} / 审批 ${taskProductionRelease.value.approval_source} / 执行 ${taskProductionRelease.value.execution_status} / 回滚 ${taskProductionRelease.value.rollback_gate_status}`
})
const visibleProductionReleaseChecks = computed(() => taskProductionRelease.value?.checks?.slice(0, 8) || [])
const taskExternalMonitorConfigSummary = computed(() => {
  if (!taskExternalMonitorConfig.value) {
    return ''
  }
  return `${statusLabel(taskExternalMonitorConfig.value.status)} / ${taskExternalMonitorConfig.value.platform_status} / 指标 ${taskExternalMonitorConfig.value.metric_names.length} / 事件 ${taskExternalMonitorConfig.value.event_names.length} / 通道 ${taskExternalMonitorConfig.value.alert_channels.length}`
})
const visibleExternalMonitorConfigChecks = computed(() => taskExternalMonitorConfig.value?.checks?.slice(0, 8) || [])
const taskWriteRampSummary = computed(() => {
  if (!taskWriteRamp.value) {
    return ''
  }
  return `${statusLabel(taskWriteRamp.value.status)} / ${taskWriteRamp.value.ramp_percent}% / ${taskWriteRamp.value.decision} / 候选 ${taskWriteRamp.value.candidates.join('、') || '无'} / 默认 ${taskWriteRamp.value.default_action}`
})
const visibleWriteRampChecks = computed(() => taskWriteRamp.value?.checks?.slice(0, 8) || [])
const taskWeChatSignoffSummary = computed(() => {
  if (!taskWeChatSignoff.value) {
    return ''
  }
  return `${statusLabel(taskWeChatSignoff.value.status)} / ${taskWeChatSignoff.value.signoff_state} / 入口 ${taskWeChatSignoff.value.entry_confirmed} / 终报 ${taskWeChatSignoff.value.final_report_confirmed} / 回退 ${taskWeChatSignoff.value.failure_fallback_confirmed}`
})
const visibleWeChatSignoffChecks = computed(() => taskWeChatSignoff.value?.checks?.slice(0, 8) || [])
const taskOperationsHandoffSummary = computed(() => {
  if (!taskOperationsHandoff.value) {
    return ''
  }
  return `${statusLabel(taskOperationsHandoff.value.status)} / 发布 ${taskOperationsHandoff.value.release_status} / 监控 ${taskOperationsHandoff.value.monitor_config_status} / 写放量 ${taskOperationsHandoff.value.write_ramp_status} / ${taskOperationsHandoff.value.next_action}`
})
const visibleOperationsHandoffChecks = computed(() => taskOperationsHandoff.value?.checks?.slice(0, 8) || [])
const taskProductionExecutionSummary = computed(() => {
  if (!taskProductionExecution.value) {
    return ''
  }
  return `${statusLabel(taskProductionExecution.value.status)} / 批次 ${taskProductionExecution.value.batch_id} / 执行人 ${taskProductionExecution.value.executor} / 执行 ${taskProductionExecution.value.execution_status} / 失败退出 ${taskProductionExecution.value.failure_exit_status}`
})
const visibleProductionExecutionChecks = computed(() => taskProductionExecution.value?.checks?.slice(0, 8) || [])
const taskMonitorIntegrationSummary = computed(() => {
  if (!taskMonitorIntegration.value) {
    return ''
  }
  return `${statusLabel(taskMonitorIntegration.value.status)} / ${taskMonitorIntegration.value.integration_result} / 指标 ${taskMonitorIntegration.value.metric_write_status} / 事件 ${taskMonitorIntegration.value.event_write_status} / 通道 ${taskMonitorIntegration.value.channels.length}`
})
const visibleMonitorIntegrationChecks = computed(() => taskMonitorIntegration.value?.checks?.slice(0, 8) || [])
const taskWriteRampPolicySummary = computed(() => {
  if (!taskWriteRampPolicy.value) {
    return ''
  }
  return `${statusLabel(taskWriteRampPolicy.value.status)} / ${taskWriteRampPolicy.value.ramp_percent}% / 范围 ${taskWriteRampPolicy.value.user_scope} / 回滚 ${taskWriteRampPolicy.value.rollback_threshold} / 默认 ${taskWriteRampPolicy.value.default_action}`
})
const visibleWriteRampPolicyChecks = computed(() => taskWriteRampPolicy.value?.checks?.slice(0, 8) || [])
const taskWeChatFinalReportSummary = computed(() => {
  if (!taskWeChatFinalReport.value) {
    return ''
  }
  return `${statusLabel(taskWeChatFinalReport.value.status)} / 通知 ${taskWeChatFinalReport.value.completion_notice_status} / 投递 ${taskWeChatFinalReport.value.delivery_status} / 模板 ${taskWeChatFinalReport.value.template_status} / 文本 ${taskWeChatFinalReport.value.text_status} / 入口 ${taskWeChatFinalReport.value.final_report_entry} / ${taskWeChatFinalReport.value.next_action}`
})
const visibleWeChatFinalReportChecks = computed(() => taskWeChatFinalReport.value?.checks?.slice(0, 8) || [])
const taskLaunchRuntimeOverviewSummary = computed(() => {
  if (!taskLaunchRuntimeOverview.value) {
    return ''
  }
  return `${statusLabel(taskLaunchRuntimeOverview.value.status)} / 生产 ${taskLaunchRuntimeOverview.value.production_execution_status} / 监控 ${taskLaunchRuntimeOverview.value.monitor_integration_status} / 写策略 ${taskLaunchRuntimeOverview.value.write_ramp_policy_status} / ${taskLaunchRuntimeOverview.value.next_action}`
})
const visibleLaunchRuntimeOverviewChecks = computed(() => taskLaunchRuntimeOverview.value?.checks?.slice(0, 8) || [])
const taskRuntimeParametersSummary = computed(() => {
  if (!taskRuntimeParameters.value) {
    return ''
  }
  return `${statusLabel(taskRuntimeParameters.value.status)} / ${taskRuntimeParameters.value.ramp_percent}% / 范围 ${taskRuntimeParameters.value.user_scope} / 通知 ${taskRuntimeParameters.value.notification_channel} / 回滚 ${taskRuntimeParameters.value.rollback_threshold}`
})
const visibleRuntimeParametersChecks = computed(() => taskRuntimeParameters.value?.checks?.slice(0, 8) || [])
const taskMonitorReadbackSummary = computed(() => {
  if (!taskMonitorReadback.value) {
    return ''
  }
  return `${statusLabel(taskMonitorReadback.value.status)} / 指标 ${taskMonitorReadback.value.metric_read_status} / 事件 ${taskMonitorReadback.value.event_read_status} / 告警 ${taskMonitorReadback.value.alert_status} / 新鲜度 ${taskMonitorReadback.value.freshness_status}`
})
const visibleMonitorReadbackChecks = computed(() => taskMonitorReadback.value?.checks?.slice(0, 8) || [])
const taskWriteRampRecommendationSummary = computed(() => {
  if (!taskWriteRampRecommendation.value) {
    return ''
  }
  return `${statusLabel(taskWriteRampRecommendation.value.status)} / ${taskWriteRampRecommendation.value.current_percent}% -> ${taskWriteRampRecommendation.value.recommended_percent}% / 候选 ${taskWriteRampRecommendation.value.candidates.join('、') || '无'} / 默认 ${taskWriteRampRecommendation.value.default_action}`
})
const visibleWriteRampRecommendationChecks = computed(() => taskWriteRampRecommendation.value?.checks?.slice(0, 8) || [])
const taskWeChatUserFeedbackSummary = computed(() => {
  if (!taskWeChatUserFeedback.value) {
    return ''
  }
  return `${statusLabel(taskWeChatUserFeedback.value.status)} / 完成 ${taskWeChatUserFeedback.value.completion_feedback} / 失败 ${taskWeChatUserFeedback.value.failure_feedback} / 按钮 ${taskWeChatUserFeedback.value.button_feedback} / ${taskWeChatUserFeedback.value.next_action}`
})
const visibleWeChatUserFeedbackChecks = computed(() => taskWeChatUserFeedback.value?.checks?.slice(0, 8) || [])
const taskOperationsRuntimeClosureSummary = computed(() => {
  if (!taskOperationsRuntimeClosure.value) {
    return ''
  }
  return `${statusLabel(taskOperationsRuntimeClosure.value.status)} / 参数 ${taskOperationsRuntimeClosure.value.runtime_parameter_status} / 回读 ${taskOperationsRuntimeClosure.value.monitor_readback_status} / 放量 ${taskOperationsRuntimeClosure.value.write_ramp_recommendation_status} / ${taskOperationsRuntimeClosure.value.next_action}`
})
const visibleOperationsRuntimeClosureChecks = computed(() => taskOperationsRuntimeClosure.value?.checks?.slice(0, 8) || [])
const taskOpsPanelConfigSummary = computed(() => {
  if (!taskOpsPanelConfig.value) {
    return ''
  }
  return `${statusLabel(taskOpsPanelConfig.value.status)} / ${taskOpsPanelConfig.value.parameter_group} / 展示 ${taskOpsPanelConfig.value.display_items.length} / 刷新 ${taskOpsPanelConfig.value.refresh_interval_seconds}s / 告警 ${taskOpsPanelConfig.value.alert_entry}`
})
const visibleOpsPanelConfigChecks = computed(() => taskOpsPanelConfig.value?.checks?.slice(0, 8) || [])
const taskMonitorAutoReportSummary = computed(() => {
  if (!taskMonitorAutoReport.value) {
    return ''
  }
  return `${statusLabel(taskMonitorAutoReport.value.status)} / 异常 ${taskMonitorAutoReport.value.anomaly_status} / 企微 ${taskMonitorAutoReport.value.wechat_send_status} / Web ${taskMonitorAutoReport.value.web_visibility_status} / 日报 ${taskMonitorAutoReport.value.daily_link_status}`
})
const visibleMonitorAutoReportChecks = computed(() => taskMonitorAutoReport.value?.checks?.slice(0, 8) || [])
const taskWriteRampStageSummary = computed(() => {
  if (!taskWriteRampStage.value) {
    return ''
  }
  return `${statusLabel(taskWriteRampStage.value.status)} / ${taskWriteRampStage.value.current_stage} -> ${taskWriteRampStage.value.next_stage} / 进入 ${taskWriteRampStage.value.entry_conditions.length} / 回滚 ${taskWriteRampStage.value.rollback_conditions.length} / 默认 ${taskWriteRampStage.value.default_action}`
})
const visibleWriteRampStageChecks = computed(() => taskWriteRampStage.value?.checks?.slice(0, 8) || [])
const taskWeChatFeedbackLoopSummary = computed(() => {
  if (!taskWeChatFeedbackLoop.value) {
    return ''
  }
  return `${statusLabel(taskWeChatFeedbackLoop.value.status)} / ${taskWeChatFeedbackLoop.value.processing_state} / 完成 ${taskWeChatFeedbackLoop.value.completion_state} / 按钮 ${taskWeChatFeedbackLoop.value.button_state} / ${taskWeChatFeedbackLoop.value.next_action}`
})
const visibleWeChatFeedbackLoopChecks = computed(() => taskWeChatFeedbackLoop.value?.checks?.slice(0, 8) || [])
const taskOperationsClosedLoopSummary = computed(() => {
  if (!taskOperationsClosedLoop.value) {
    return ''
  }
  return `${statusLabel(taskOperationsClosedLoop.value.status)} / 面板 ${taskOperationsClosedLoop.value.ops_panel_status} / 监控 ${taskOperationsClosedLoop.value.monitor_report_status} / 放量 ${taskOperationsClosedLoop.value.write_ramp_stage_status} / ${taskOperationsClosedLoop.value.next_action}`
})
const visibleOperationsClosedLoopChecks = computed(() => taskOperationsClosedLoop.value?.checks?.slice(0, 8) || [])
const taskOpsDashboardInteractionSummary = computed(() => {
  if (!taskOpsDashboardInteraction.value) {
    return ''
  }
  return `${statusLabel(taskOpsDashboardInteraction.value.status)} / 动作 ${taskOpsDashboardInteraction.value.actions.length} / 刷新 ${taskOpsDashboardInteraction.value.refresh_strategy} / 筛选 ${taskOpsDashboardInteraction.value.filters.length} / 审计 ${taskOpsDashboardInteraction.value.audit_event}`
})
const visibleOpsDashboardInteractionChecks = computed(() => taskOpsDashboardInteraction.value?.checks?.slice(0, 8) || [])
const visibleOpsDashboardInteractionActions = computed(() => taskOpsDashboardInteraction.value?.actions?.slice(0, 6) || [])
const taskAlertDedupeEscalationSummary = computed(() => {
  if (!taskAlertDedupeEscalation.value) {
    return ''
  }
  return `${statusLabel(taskAlertDedupeEscalation.value.status)} / ${taskAlertDedupeEscalation.value.dedupe_key} / ${taskAlertDedupeEscalation.value.dedupe_window_seconds}s / 升级 ${taskAlertDedupeEscalation.value.escalation_condition} / 企微 ${taskAlertDedupeEscalation.value.wechat_notify_status} / Web ${taskAlertDedupeEscalation.value.web_visibility_status}`
})
const visibleAlertDedupeEscalationChecks = computed(() => taskAlertDedupeEscalation.value?.checks?.slice(0, 8) || [])
const taskWriteStageRecordSummary = computed(() => {
  if (!taskWriteStageRecord.value) {
    return ''
  }
  return `${statusLabel(taskWriteStageRecord.value.status)} / ${taskWriteStageRecord.value.current_stage} -> ${taskWriteStageRecord.value.target_stage} / 阻断 ${taskWriteStageRecord.value.blockers.length} / 回滚 ${taskWriteStageRecord.value.rollback_conditions.length} / 默认 ${taskWriteStageRecord.value.default_action}`
})
const visibleWriteStageRecordChecks = computed(() => taskWriteStageRecord.value?.checks?.slice(0, 8) || [])
const taskWeChatFeedbackTicketSummary = computed(() => {
  if (!taskWeChatFeedbackTicket.value) {
    return ''
  }
  return `${statusLabel(taskWeChatFeedbackTicket.value.status)} / ${taskWeChatFeedbackTicket.value.ticket_type} / ${taskWeChatFeedbackTicket.value.processing_state} / 责任 ${taskWeChatFeedbackTicket.value.owner_entry} / ${taskWeChatFeedbackTicket.value.user_next_action}`
})
const visibleWeChatFeedbackTicketChecks = computed(() => taskWeChatFeedbackTicket.value?.checks?.slice(0, 8) || [])
const taskOperationsHandlingSummary = computed(() => {
  if (!taskOperationsHandling.value) {
    return ''
  }
  return `${statusLabel(taskOperationsHandling.value.status)} / 面板 ${taskOperationsHandling.value.dashboard_status} / 告警 ${taskOperationsHandling.value.alert_escalation_status} / 写阶段 ${taskOperationsHandling.value.write_stage_status} / 工单 ${taskOperationsHandling.value.feedback_ticket_status} / ${taskOperationsHandling.value.next_action}`
})
const visibleOperationsHandlingChecks = computed(() => taskOperationsHandling.value?.checks?.slice(0, 8) || [])
const taskOpsActionDefinitionSummary = computed(() => {
  if (!taskOpsActionDefinition.value) {
    return ''
  }
  return `${statusLabel(taskOpsActionDefinition.value.status)} / 动作 ${taskOpsActionDefinition.value.actions.length}`
})
const visibleOpsActionDefinitionActions = computed(() => taskOpsActionDefinition.value?.actions?.slice(0, 6) || [])
const visibleOpsActionDefinitionChecks = computed(() => taskOpsActionDefinition.value?.checks?.slice(0, 8) || [])
const taskAlertEscalationPolicySummary = computed(() => {
  if (!taskAlertEscalationPolicy.value) {
    return ''
  }
  return `${statusLabel(taskAlertEscalationPolicy.value.status)} / 等级 ${taskAlertEscalationPolicy.value.escalation_level} / 通道 ${taskAlertEscalationPolicy.value.notification_channels.join('、') || '无'} / 抑制 ${taskAlertEscalationPolicy.value.repeat_suppression} / 恢复 ${taskAlertEscalationPolicy.value.recovery_notice_status}`
})
const visibleAlertEscalationPolicyChecks = computed(() => taskAlertEscalationPolicy.value?.checks?.slice(0, 8) || [])
const taskWriteStageApprovalSummary = computed(() => {
  if (!taskWriteStageApproval.value) {
    return ''
  }
  return `${statusLabel(taskWriteStageApproval.value.status)} / 审批 ${taskWriteStageApproval.value.approval_status} / 目标 ${taskWriteStageApproval.value.target_stage} / 范围 ${taskWriteStageApproval.value.authorized_scope} / 回滚 ${taskWriteStageApproval.value.rollback_threshold} / 默认 ${taskWriteStageApproval.value.default_action}`
})
const visibleWriteStageApprovalChecks = computed(() => taskWriteStageApproval.value?.checks?.slice(0, 8) || [])
const taskFeedbackTicketLifecycleSummary = computed(() => {
  if (!taskFeedbackTicketLifecycle.value) {
    return ''
  }
  return `${statusLabel(taskFeedbackTicketLifecycle.value.status)} / 创建 ${taskFeedbackTicketLifecycle.value.created_state} / 分派 ${taskFeedbackTicketLifecycle.value.assigned_state} / 处理 ${taskFeedbackTicketLifecycle.value.processing_state} / 用户 ${taskFeedbackTicketLifecycle.value.waiting_user_state} / 转人工 ${taskFeedbackTicketLifecycle.value.handoff_state}`
})
const visibleFeedbackTicketLifecycleChecks = computed(() => taskFeedbackTicketLifecycle.value?.checks?.slice(0, 8) || [])
const taskOperationsActionClosureSummary = computed(() => {
  if (!taskOperationsActionClosure.value) {
    return ''
  }
  return `${statusLabel(taskOperationsActionClosure.value.status)} / 动作 ${taskOperationsActionClosure.value.ops_action_status} / 升级 ${taskOperationsActionClosure.value.alert_escalation_status} / 审批 ${taskOperationsActionClosure.value.write_approval_status} / 工单 ${taskOperationsActionClosure.value.ticket_lifecycle_status} / ${taskOperationsActionClosure.value.next_action}`
})
const visibleOperationsActionClosureChecks = computed(() => taskOperationsActionClosure.value?.checks?.slice(0, 8) || [])
const taskOpsAPIExecutionSummary = computed(() => {
  if (!taskOpsAPIExecution.value) {
    return ''
  }
  return `${statusLabel(taskOpsAPIExecution.value.status)} / 执行 ${taskOpsAPIExecution.value.executions.length}`
})
const visibleOpsAPIExecutionItems = computed(() => taskOpsAPIExecution.value?.executions?.slice(0, 6) || [])
const visibleOpsAPIExecutionChecks = computed(() => taskOpsAPIExecution.value?.checks?.slice(0, 8) || [])
const taskAlertEscalationReceiptSummary = computed(() => {
  if (!taskAlertEscalationReceipt.value) {
    return ''
  }
  return `${statusLabel(taskAlertEscalationReceipt.value.status)} / 投递 ${taskAlertEscalationReceipt.value.delivery_status} / 抑制 ${taskAlertEscalationReceipt.value.suppression_result} / 恢复 ${taskAlertEscalationReceipt.value.recovery_notice_status} / 转人工 ${taskAlertEscalationReceipt.value.handoff_entry}`
})
const visibleAlertEscalationReceiptChecks = computed(() => taskAlertEscalationReceipt.value?.checks?.slice(0, 8) || [])
const taskWriteApprovalButtonSummary = computed(() => {
  if (!taskWriteApprovalButton.value) {
    return ''
  }
  return `${statusLabel(taskWriteApprovalButton.value.status)} / 按钮 ${taskWriteApprovalButton.value.buttons.length}`
})
const visibleWriteApprovalButtons = computed(() => taskWriteApprovalButton.value?.buttons?.slice(0, 6) || [])
const visibleWriteApprovalButtonChecks = computed(() => taskWriteApprovalButton.value?.checks?.slice(0, 8) || [])
const taskFeedbackTicketSLASummary = computed(() => {
  if (!taskFeedbackTicketSLA.value) {
    return ''
  }
  return `${statusLabel(taskFeedbackTicketSLA.value.status)} / 首响 ${taskFeedbackTicketSLA.value.first_response_seconds}s / 处理 ${taskFeedbackTicketSLA.value.resolve_seconds}s / 超时 ${taskFeedbackTicketSLA.value.timeout_escalation} / 用户 ${taskFeedbackTicketSLA.value.waiting_user_status}`
})
const visibleFeedbackTicketSLAChecks = computed(() => taskFeedbackTicketSLA.value?.checks?.slice(0, 8) || [])
const taskOperationsExecutionSummary = computed(() => {
  if (!taskOperationsExecution.value) {
    return ''
  }
  return `${statusLabel(taskOperationsExecution.value.status)} / API ${taskOperationsExecution.value.ops_api_execution_status} / 回执 ${taskOperationsExecution.value.alert_receipt_status} / 按钮 ${taskOperationsExecution.value.write_approval_button_status} / SLA ${taskOperationsExecution.value.feedback_sla_status} / ${taskOperationsExecution.value.next_action}`
})
const visibleOperationsExecutionChecks = computed(() => taskOperationsExecution.value?.checks?.slice(0, 8) || [])
const taskOpsExecutionRecordSummary = computed(() => {
  if (!taskOpsExecutionRecord.value) {
    return ''
  }
  return `${statusLabel(taskOpsExecutionRecord.value.status)} / 记录 ${taskOpsExecutionRecord.value.records.length}`
})
const visibleOpsExecutionRecords = computed(() => taskOpsExecutionRecord.value?.records?.slice(0, 6) || [])
const visibleOpsExecutionRecordChecks = computed(() => taskOpsExecutionRecord.value?.checks?.slice(0, 8) || [])
const taskWeChatApprovalCallbackSummary = computed(() => {
  if (!taskWeChatApprovalCallback.value) {
    return ''
  }
  return `${statusLabel(taskWeChatApprovalCallback.value.status)} / ${taskWeChatApprovalCallback.value.callback_key} / 来源 ${taskWeChatApprovalCallback.value.source} / 决策 ${taskWeChatApprovalCallback.value.decision} / 验签 ${taskWeChatApprovalCallback.value.signature} / 入库 ${taskWeChatApprovalCallback.value.storage_state}`
})
const visibleWeChatApprovalCallbackChecks = computed(() => taskWeChatApprovalCallback.value?.checks?.slice(0, 8) || [])
const taskFeedbackSLAReportSummary = computed(() => {
  if (!taskFeedbackSLAReport.value) {
    return ''
  }
  return `${statusLabel(taskFeedbackSLAReport.value.status)} / 首响 ${(taskFeedbackSLAReport.value.first_response_rate * 100).toFixed(0)}% / 处理 ${(taskFeedbackSLAReport.value.resolve_rate * 100).toFixed(0)}% / 超时 ${taskFeedbackSLAReport.value.timeout_count} / 等待 ${taskFeedbackSLAReport.value.waiting_user_count} / 转人工 ${taskFeedbackSLAReport.value.handoff_count}`
})
const visibleFeedbackSLAReportChecks = computed(() => taskFeedbackSLAReport.value?.checks?.slice(0, 8) || [])
const taskAlertAutoRecoverySummary = computed(() => {
  if (!taskAlertAutoRecovery.value) {
    return ''
  }
  return `${statusLabel(taskAlertAutoRecovery.value.status)} / 触发 ${taskAlertAutoRecovery.value.recovery_trigger} / 通知 ${taskAlertAutoRecovery.value.recovery_notice} / 解除 ${taskAlertAutoRecovery.value.suppression_release} / 重开 ${taskAlertAutoRecovery.value.reopen_condition}`
})
const visibleAlertAutoRecoveryChecks = computed(() => taskAlertAutoRecovery.value?.checks?.slice(0, 8) || [])
const taskOperationsEvidenceSummary = computed(() => {
  if (!taskOperationsEvidence.value) {
    return ''
  }
  return `${statusLabel(taskOperationsEvidence.value.status)} / 执行 ${taskOperationsEvidence.value.execution_record_status} / 回调 ${taskOperationsEvidence.value.approval_callback_status} / SLA ${taskOperationsEvidence.value.sla_report_status} / 恢复 ${taskOperationsEvidence.value.auto_recovery_status} / ${taskOperationsEvidence.value.next_action}`
})
const visibleOperationsEvidenceChecks = computed(() => taskOperationsEvidence.value?.checks?.slice(0, 8) || [])
const taskUnifiedProgressComponentSummary = computed(() => {
  if (!taskUnifiedProgressComponent.value) {
    return ''
  }
  return `${statusLabel(taskUnifiedProgressComponent.value.status)} / ${taskUnifiedProgressComponent.value.component_key} / Web ${taskUnifiedProgressComponent.value.web_status} / 企微 ${taskUnifiedProgressComponent.value.wechat_status} / 刷新 ${taskUnifiedProgressComponent.value.refresh_strategy}`
})
const visibleUnifiedProgressComponentChecks = computed(() => taskUnifiedProgressComponent.value?.checks?.slice(0, 8) || [])
const taskEvidenceDetailPageSummary = computed(() => {
  if (!taskEvidenceDetailPage.value) {
    return ''
  }
  return `${statusLabel(taskEvidenceDetailPage.value.status)} / 入口 ${taskEvidenceDetailPage.value.detail_entry} / 记录 ${taskEvidenceDetailPage.value.record_count} / 回放 ${taskEvidenceDetailPage.value.replay_entry} / 保留 ${taskEvidenceDetailPage.value.retention_policy}`
})
const visibleEvidenceDetailPageChecks = computed(() => taskEvidenceDetailPage.value?.checks?.slice(0, 8) || [])
const taskCallbackReplayToolSummary = computed(() => {
  if (!taskCallbackReplayTool.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayTool.value.status)} / ${taskCallbackReplayTool.value.callback_key} / 重放 ${taskCallbackReplayTool.value.replay_entry} / 验签 ${taskCallbackReplayTool.value.signature_review} / 幂等 ${taskCallbackReplayTool.value.idempotency_guard}`
})
const visibleCallbackReplayToolChecks = computed(() => taskCallbackReplayTool.value?.checks?.slice(0, 8) || [])
const taskRecoveryPolicyConfigSummary = computed(() => {
  if (!taskRecoveryPolicyConfig.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyConfig.value.status)} / ${taskRecoveryPolicyConfig.value.policy_key} / 触发 ${taskRecoveryPolicyConfig.value.recovery_trigger} / 窗口 ${taskRecoveryPolicyConfig.value.suppression_window} / 默认 ${taskRecoveryPolicyConfig.value.default_policy}`
})
const visibleRecoveryPolicyConfigChecks = computed(() => taskRecoveryPolicyConfig.value?.checks?.slice(0, 8) || [])
const taskDualEndProgressEvidenceSummary = computed(() => {
  if (!taskDualEndProgressEvidence.value) {
    return ''
  }
  return `${statusLabel(taskDualEndProgressEvidence.value.status)} / 进度 ${taskDualEndProgressEvidence.value.unified_progress_status} / 明细 ${taskDualEndProgressEvidence.value.evidence_detail_status} / 重放 ${taskDualEndProgressEvidence.value.callback_replay_status} / 恢复 ${taskDualEndProgressEvidence.value.recovery_policy_status} / ${taskDualEndProgressEvidence.value.next_action}`
})
const visibleDualEndProgressEvidenceChecks = computed(() => taskDualEndProgressEvidence.value?.checks?.slice(0, 8) || [])
const taskWeChatProgressCardSummary = computed(() => {
  if (!taskWeChatProgressCard.value) {
    return ''
  }
  return `${statusLabel(taskWeChatProgressCard.value.status)} / ${taskWeChatProgressCard.value.card_key} / 阶段 ${taskWeChatProgressCard.value.phase_status} / 进度 ${taskWeChatProgressCard.value.progress_percent}% / 按钮 ${taskWeChatProgressCard.value.actions.length}`
})
const visibleWeChatProgressCardActions = computed(() => taskWeChatProgressCard.value?.actions?.slice(0, 6) || [])
const visibleWeChatProgressCardChecks = computed(() => taskWeChatProgressCard.value?.checks?.slice(0, 8) || [])
const taskWebEvidenceInteractionSummary = computed(() => {
  if (!taskWebEvidenceInteraction.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceInteraction.value.status)} / 筛选 ${taskWebEvidenceInteraction.value.filters.length} / 展开 ${taskWebEvidenceInteraction.value.expandable} / 回放 ${taskWebEvidenceInteraction.value.replay_entry} / 可见 ${taskWebEvidenceInteraction.value.visibility}`
})
const visibleWebEvidenceInteractionChecks = computed(() => taskWebEvidenceInteraction.value?.checks?.slice(0, 8) || [])
const taskCallbackReplayPermissionSummary = computed(() => {
  if (!taskCallbackReplayPermission.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayPermission.value.status)} / ${taskCallbackReplayPermission.value.permission_key} / 角色 ${taskCallbackReplayPermission.value.allowed_roles.join('、') || '无'} / 幂等 ${taskCallbackReplayPermission.value.idempotency_guard} / 验签 ${taskCallbackReplayPermission.value.signature_review}`
})
const visibleCallbackReplayPermissionChecks = computed(() => taskCallbackReplayPermission.value?.checks?.slice(0, 8) || [])
const taskRecoveryPolicyAuditSummary = computed(() => {
  if (!taskRecoveryPolicyAudit.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyAudit.value.status)} / ${taskRecoveryPolicyAudit.value.change_key} / 审批 ${taskRecoveryPolicyAudit.value.approval_status} / 回滚 ${taskRecoveryPolicyAudit.value.rollback_path}`
})
const visibleRecoveryPolicyAuditChecks = computed(() => taskRecoveryPolicyAudit.value?.checks?.slice(0, 8) || [])
const taskDualEndInteractionSummary = computed(() => {
  if (!taskDualEndInteraction.value) {
    return ''
  }
  return `${statusLabel(taskDualEndInteraction.value.status)} / 企微 ${taskDualEndInteraction.value.wechat_progress_card_status} / Web ${taskDualEndInteraction.value.web_evidence_status} / 权限 ${taskDualEndInteraction.value.callback_permission_status} / 恢复 ${taskDualEndInteraction.value.recovery_policy_audit_status} / ${taskDualEndInteraction.value.next_action}`
})
const visibleDualEndInteractionChecks = computed(() => taskDualEndInteraction.value?.checks?.slice(0, 8) || [])
const taskWeChatTemplateRenderSummary = computed(() => {
  if (!taskWeChatTemplateRender.value) {
    return ''
  }
  return `${statusLabel(taskWeChatTemplateRender.value.status)} / ${taskWeChatTemplateRender.value.template_key} / 渲染 ${taskWeChatTemplateRender.value.render_status} / 阶段 ${taskWeChatTemplateRender.value.phase_fields.length} / 按钮 ${taskWeChatTemplateRender.value.button_fields.length} / 发送 ${taskWeChatTemplateRender.value.send_entry}`
})
const visibleWeChatTemplateRenderChecks = computed(() => taskWeChatTemplateRender.value?.checks?.slice(0, 8) || [])
const taskWebEvidenceRouteSummary = computed(() => {
  if (!taskWebEvidenceRoute.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceRoute.value.status)} / ${taskWebEvidenceRoute.value.route_name} / 路径 ${taskWebEvidenceRoute.value.path_params.join('、') || '无'} / 筛选 ${taskWebEvidenceRoute.value.filter_params.length} / 权限 ${taskWebEvidenceRoute.value.permission_requirement}`
})
const visibleWebEvidenceRouteChecks = computed(() => taskWebEvidenceRoute.value?.checks?.slice(0, 8) || [])
const taskCallbackReplayApprovalSummary = computed(() => {
  if (!taskCallbackReplayApproval.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayApproval.value.status)} / ${taskCallbackReplayApproval.value.approval_key} / 申请 ${taskCallbackReplayApproval.value.request_entry} / 审批 ${taskCallbackReplayApproval.value.approval_status} / 门禁 ${taskCallbackReplayApproval.value.execution_gate}`
})
const visibleCallbackReplayApprovalChecks = computed(() => taskCallbackReplayApproval.value?.checks?.slice(0, 8) || [])
const taskRecoveryPolicyPersistSummary = computed(() => {
  if (!taskRecoveryPolicyPersist.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyPersist.value.status)} / ${taskRecoveryPolicyPersist.value.config_key} / 当前 ${taskRecoveryPolicyPersist.value.current_version} / 待发布 ${taskRecoveryPolicyPersist.value.pending_version} / 持久化 ${taskRecoveryPolicyPersist.value.persistence_status}`
})
const visibleRecoveryPolicyPersistChecks = computed(() => taskRecoveryPolicyPersist.value?.checks?.slice(0, 8) || [])
const taskDualEndInteractionLaunchSummary = computed(() => {
  if (!taskDualEndInteractionLaunch.value) {
    return ''
  }
  return `${statusLabel(taskDualEndInteractionLaunch.value.status)} / 模板 ${taskDualEndInteractionLaunch.value.wechat_template_render_status} / 路由 ${taskDualEndInteractionLaunch.value.web_evidence_route_status} / 审批 ${taskDualEndInteractionLaunch.value.callback_replay_approval_status} / 持久化 ${taskDualEndInteractionLaunch.value.recovery_policy_persistence_status} / ${taskDualEndInteractionLaunch.value.next_action}`
})
const visibleDualEndInteractionLaunchChecks = computed(() => taskDualEndInteractionLaunch.value?.checks?.slice(0, 8) || [])
const taskWeChatTemplateSendSummary = computed(() => {
  if (!taskWeChatTemplateSend.value) {
    return ''
  }
  return `${statusLabel(taskWeChatTemplateSend.value.status)} / ${taskWeChatTemplateSend.value.message_type} / ${taskWeChatTemplateSend.value.title} / 发送 ${taskWeChatTemplateSend.value.send_result} / 审计 ${taskWeChatTemplateSend.value.audit_event}`
})
const visibleWeChatTemplateSendChecks = computed(() => taskWeChatTemplateSend.value?.checks?.slice(0, 8) || [])
const taskWebEvidenceDetailViewSummary = computed(() => {
  if (!taskWebEvidenceDetailView.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceDetailView.value.status)} / ${taskWebEvidenceDetailView.value.route_path} / 来源 ${taskWebEvidenceDetailView.value.record_source} / 回放 ${taskWebEvidenceDetailView.value.replay_entry} / 权限 ${taskWebEvidenceDetailView.value.permission_hint}`
})
const visibleWebEvidenceDetailViewChecks = computed(() => taskWebEvidenceDetailView.value?.checks?.slice(0, 8) || [])
const taskCallbackReplayExecutionSummary = computed(() => {
  if (!taskCallbackReplayExecution.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayExecution.value.status)} / 申请 ${taskCallbackReplayExecution.value.request_entry} / 执行 ${taskCallbackReplayExecution.value.execute_entry} / 审批 ${taskCallbackReplayExecution.value.approval_status} / 门禁 ${taskCallbackReplayExecution.value.execution_gate}`
})
const visibleCallbackReplayExecutionChecks = computed(() => taskCallbackReplayExecution.value?.checks?.slice(0, 8) || [])
const taskRecoveryPolicyVersionSummary = computed(() => {
  if (!taskRecoveryPolicyVersion.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyVersion.value.status)} / ${taskRecoveryPolicyVersion.value.policy_key} / 当前 ${taskRecoveryPolicyVersion.value.current_version} / 待发布 ${taskRecoveryPolicyVersion.value.pending_version} / 发布 ${taskRecoveryPolicyVersion.value.release_status}`
})
const visibleRecoveryPolicyVersionChecks = computed(() => taskRecoveryPolicyVersion.value?.checks?.slice(0, 8) || [])
const taskDualEndRealInteractionSummary = computed(() => {
  if (!taskDualEndRealInteraction.value) {
    return ''
  }
  return `${statusLabel(taskDualEndRealInteraction.value.status)} / 企微 ${taskDualEndRealInteraction.value.wechat_template_send_status} / 证据 ${taskDualEndRealInteraction.value.web_evidence_detail_status} / 回放 ${taskDualEndRealInteraction.value.callback_replay_execution_status} / 恢复 ${taskDualEndRealInteraction.value.recovery_policy_version_status} / ${taskDualEndRealInteraction.value.next_action}`
})
const visibleDualEndRealInteractionChecks = computed(() => taskDualEndRealInteraction.value?.checks?.slice(0, 8) || [])
const taskWeChatTemplateIntegrationSummary = computed(() => {
  if (!taskWeChatTemplateIntegration.value) {
    return ''
  }
  return `${statusLabel(taskWeChatTemplateIntegration.value.status)} / 发送 ${taskWeChatTemplateIntegration.value.send_path} / 模板 ${taskWeChatTemplateIntegration.value.template_status} / fallback ${taskWeChatTemplateIntegration.value.fallback_status} / 降级 ${taskWeChatTemplateIntegration.value.degrade_strategy}`
})
const visibleWeChatTemplateIntegrationChecks = computed(() => taskWeChatTemplateIntegration.value?.checks?.slice(0, 8) || [])
const taskWebEvidenceInteractionDetailSummary = computed(() => {
  if (!taskWebEvidenceInteractionDetail.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceInteractionDetail.value.status)} / 筛选 ${taskWebEvidenceInteractionDetail.value.filter_mode} / 展开 ${taskWebEvidenceInteractionDetail.value.expand_mode} / 回放 ${taskWebEvidenceInteractionDetail.value.replay_request_entry}`
})
const visibleWebEvidenceInteractionDetailChecks = computed(() => taskWebEvidenceInteractionDetail.value?.checks?.slice(0, 8) || [])
const taskCallbackReplaySafetyAuditSummary = computed(() => {
  if (!taskCallbackReplaySafetyAudit.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplaySafetyAudit.value.status)} / 幂等 ${taskCallbackReplaySafetyAudit.value.idempotency_check} / 审批 ${taskCallbackReplaySafetyAudit.value.approval_check} / 签名 ${taskCallbackReplaySafetyAudit.value.signature_check} / 结果 ${taskCallbackReplaySafetyAudit.value.execution_result}`
})
const visibleCallbackReplaySafetyAuditChecks = computed(() => taskCallbackReplaySafetyAudit.value?.checks?.slice(0, 8) || [])
const taskRecoveryPolicyGrayReleaseSummary = computed(() => {
  if (!taskRecoveryPolicyGrayRelease.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyGrayRelease.value.status)} / 阶段 ${taskRecoveryPolicyGrayRelease.value.gray_stage} / 比例 ${taskRecoveryPolicyGrayRelease.value.release_percent}% / 回滚 ${taskRecoveryPolicyGrayRelease.value.rollback_condition} / 审批 ${taskRecoveryPolicyGrayRelease.value.approval_status}`
})
const visibleRecoveryPolicyGrayReleaseChecks = computed(() => taskRecoveryPolicyGrayRelease.value?.checks?.slice(0, 8) || [])
const taskDualEndRunLoopSummary = computed(() => {
  if (!taskDualEndRunLoop.value) {
    return ''
  }
  return `${statusLabel(taskDualEndRunLoop.value.status)} / 企微 ${taskDualEndRunLoop.value.wechat_template_integration_status} / 证据 ${taskDualEndRunLoop.value.web_evidence_interaction_status} / 回放 ${taskDualEndRunLoop.value.callback_replay_safety_status} / 恢复 ${taskDualEndRunLoop.value.recovery_policy_gray_status} / ${taskDualEndRunLoop.value.next_action}`
})
const visibleDualEndRunLoopChecks = computed(() => taskDualEndRunLoop.value?.checks?.slice(0, 8) || [])
const taskWeChatTemplatePilotSummary = computed(() => {
  if (!taskWeChatTemplatePilot.value) {
    return ''
  }
  return `${statusLabel(taskWeChatTemplatePilot.value.status)} / 批次 ${taskWeChatTemplatePilot.value.pilot_batch} / 范围 ${taskWeChatTemplatePilot.value.target_scope} / 模板 ${taskWeChatTemplatePilot.value.template_status} / 消息 ${taskWeChatTemplatePilot.value.message_id_status}`
})
const taskWebEvidenceUserActionSummary = computed(() => {
  if (!taskWebEvidenceUserAction.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceUserAction.value.status)} / 筛选 ${taskWebEvidenceUserAction.value.filter_action} / 展开 ${taskWebEvidenceUserAction.value.expand_action} / 时间线 ${taskWebEvidenceUserAction.value.timeline_action} / 权限 ${taskWebEvidenceUserAction.value.permission_result}`
})
const taskCallbackReplayResultTraceSummary = computed(() => {
  if (!taskCallbackReplayResultTrace.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayResultTrace.value.status)} / 结果 ${taskCallbackReplayResultTrace.value.execution_result} / 幂等 ${taskCallbackReplayResultTrace.value.idempotency_hit} / 审批 ${taskCallbackReplayResultTrace.value.approval_decision} / 签名 ${taskCallbackReplayResultTrace.value.signature_result}`
})
const taskRecoveryPolicyAutomationSummary = computed(() => {
  if (!taskRecoveryPolicyAutomation.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryPolicyAutomation.value.status)} / 推进 ${taskRecoveryPolicyAutomation.value.auto_advance} / 当前 ${taskRecoveryPolicyAutomation.value.current_percent}% / 下一 ${taskRecoveryPolicyAutomation.value.next_percent}% / 回滚 ${taskRecoveryPolicyAutomation.value.rollback_condition}`
})
const taskDualEndTaskClosureSummary = computed(() => {
  if (!taskDualEndTaskClosure.value) {
    return ''
  }
  return `${statusLabel(taskDualEndTaskClosure.value.status)} / 企微 ${taskDualEndTaskClosure.value.wechat_pilot_status} / 证据 ${taskDualEndTaskClosure.value.web_evidence_action_status} / 回放 ${taskDualEndTaskClosure.value.callback_replay_trace_status} / 恢复 ${taskDualEndTaskClosure.value.recovery_automation_status} / ${taskDualEndTaskClosure.value.next_action}`
})
const taskClosureSummaryItems = computed(() =>
  [
    {
      key: 'wechat-template-pilot',
      title: '企微模板试运行',
      summary: taskWeChatTemplatePilotSummary.value,
      checks: taskWeChatTemplatePilot.value?.checks?.slice(0, 8) || [],
    },
    {
      key: 'web-evidence-user-action',
      title: 'Web 证据操作',
      summary: taskWebEvidenceUserActionSummary.value,
      checks: taskWebEvidenceUserAction.value?.checks?.slice(0, 8) || [],
    },
    {
      key: 'callback-replay-result-trace',
      title: '回放结果留痕',
      summary: taskCallbackReplayResultTraceSummary.value,
      checks: taskCallbackReplayResultTrace.value?.checks?.slice(0, 8) || [],
    },
    {
      key: 'recovery-policy-automation',
      title: '恢复策略自动化',
      summary: taskRecoveryPolicyAutomationSummary.value,
      checks: taskRecoveryPolicyAutomation.value?.checks?.slice(0, 8) || [],
    },
    {
      key: 'dual-end-task-closure',
      title: '双端任务闭环',
      summary: taskDualEndTaskClosureSummary.value,
      checks: taskDualEndTaskClosure.value?.checks?.slice(0, 8) || [],
    },
  ].filter((item) => item.summary),
)
const taskWeChatTemplatePilotMetricSummary = computed(() => {
  if (!taskWeChatTemplatePilotMetric.value) {
    return ''
  }
  return `${statusLabel(taskWeChatTemplatePilotMetric.value.status)} / 批次 ${taskWeChatTemplatePilotMetric.value.batch_id} / 发送 ${taskWeChatTemplatePilotMetric.value.send_status} / fallback ${taskWeChatTemplatePilotMetric.value.fallback_count} / 消息 ${taskWeChatTemplatePilotMetric.value.message_id_status}`
})
const taskWebEvidenceOperationSummary = computed(() => {
  if (!taskWebEvidenceOperation.value) {
    return ''
  }
  return `${statusLabel(taskWebEvidenceOperation.value.status)} / 操作 ${taskWebEvidenceOperation.value.operation_count} / 筛选 ${taskWebEvidenceOperation.value.filter_entry} / 回放 ${taskWebEvidenceOperation.value.replay_request_entry} / 权限 ${taskWebEvidenceOperation.value.permission_gate}`
})
const taskCallbackReplayResultQuerySummary = computed(() => {
  if (!taskCallbackReplayResultQuery.value) {
    return ''
  }
  return `${statusLabel(taskCallbackReplayResultQuery.value.status)} / 查询 ${taskCallbackReplayResultQuery.value.query_entry} / 结果 ${taskCallbackReplayResultQuery.value.execution_result} / 幂等 ${taskCallbackReplayResultQuery.value.idempotency_result} / 审批 ${taskCallbackReplayResultQuery.value.approval_decision}`
})
const taskRecoveryAutomationExecutionSummary = computed(() => {
  if (!taskRecoveryAutomationExecution.value) {
    return ''
  }
  return `${statusLabel(taskRecoveryAutomationExecution.value.status)} / 模式 ${taskRecoveryAutomationExecution.value.execution_mode} / 当前 ${taskRecoveryAutomationExecution.value.current_percent}% / 下一 ${taskRecoveryAutomationExecution.value.next_percent}% / 决策 ${taskRecoveryAutomationExecution.value.advance_decision}`
})
const taskRealInteractionAutomationSummary = computed(() => {
  if (!taskRealInteractionAutomation.value) {
    return ''
  }
  return `${statusLabel(taskRealInteractionAutomation.value.status)} / 指标 ${taskRealInteractionAutomation.value.pilot_metric_status} / 证据 ${taskRealInteractionAutomation.value.evidence_operation_status} / 回放 ${taskRealInteractionAutomation.value.replay_query_status} / 恢复 ${taskRealInteractionAutomation.value.recovery_execution_status} / ${taskRealInteractionAutomation.value.next_action}`
})
const taskWeChatWebProgressLinkSummary = computed(() => {
  if (!taskWeChatWebProgressLink.value) {
    return ''
  }
  return `${statusLabel(taskWeChatWebProgressLink.value.status)} / 地址 ${taskWeChatWebProgressLink.value.url_source} / 通道 ${taskWeChatWebProgressLink.value.delivery_channel} / 模板 ${taskWeChatWebProgressLink.value.template_status} / fallback ${taskWeChatWebProgressLink.value.fallback_status} / ${taskWeChatWebProgressLink.value.next_action}`
})
const taskRealInteractionSummaryItems = computed(() =>
  [
    {
      key: 'wechat-template-pilot-metric',
      title: '企微试运行指标',
      summary: taskWeChatTemplatePilotMetricSummary.value,
      checks: taskWeChatTemplatePilotMetric.value?.checks?.slice(0, 8) || [],
      progressURL: '',
    },
    {
      key: 'web-evidence-operation',
      title: 'Web 证据操作',
      summary: taskWebEvidenceOperationSummary.value,
      checks: taskWebEvidenceOperation.value?.checks?.slice(0, 8) || [],
      progressURL: '',
    },
    {
      key: 'callback-replay-result-query',
      title: '回放结果查询',
      summary: taskCallbackReplayResultQuerySummary.value,
      checks: taskCallbackReplayResultQuery.value?.checks?.slice(0, 8) || [],
      progressURL: '',
    },
    {
      key: 'recovery-automation-execution',
      title: '恢复自动化执行',
      summary: taskRecoveryAutomationExecutionSummary.value,
      checks: taskRecoveryAutomationExecution.value?.checks?.slice(0, 8) || [],
      progressURL: '',
    },
    {
      key: 'real-interaction-automation',
      title: '真实交互自动化',
      summary: taskRealInteractionAutomationSummary.value,
      checks: taskRealInteractionAutomation.value?.checks?.slice(0, 8) || [],
      progressURL: '',
    },
    {
      key: 'wechat-web-progress-link',
      title: '企微 Web 进度地址',
      summary: taskWeChatWebProgressLinkSummary.value,
      checks: taskWeChatWebProgressLink.value?.checks?.slice(0, 8) || [],
      progressURL: taskWeChatWebProgressLink.value?.progress_url || '',
    },
  ].filter((item) => item.summary),
)
const hasMultiTurnContext = computed(() =>
  Boolean(
    multiTurnOriginalGoal.value ||
      multiTurnLatestInstruction.value ||
      multiTurnStoppedReason.value ||
      multiTurnAppendedInputs.value.length ||
      multiTurnFollowupQuestions.value.length ||
      parentPlanMetadata.value ||
      resultReuseMetadata.value,
  ),
)

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function asText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function asNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function metadataStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }
  return value.map((item) => (typeof item === 'string' ? item.trim() : '')).filter(Boolean)
}

function metadataEntryMessages(value: unknown) {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((entry) => {
      if (typeof entry === 'string') {
        return entry.trim()
      }
      if (isRecord(entry)) {
        return asText(entry.message)
      }
      return ''
    })
    .filter(Boolean)
}

function metadataRecordList(value: unknown): Record<string, unknown>[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter(isRecord)
}

async function loadPlan(options: { silent?: boolean } = {}) {
  if (planID.value < 1 && scheduledTaskID.value < 1) {
    clearProgressStream()
    clearPolling()
    errorMessage.value = ''
    return
  }
  const token = ++requestSeq
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  if (!options.silent) {
    errorMessage.value = ''
  }
  try {
    const nextProgress = await getAgentProgress(
      planID.value > 0 ? { plan_id: planID.value } : { scheduled_task_id: scheduledTaskID.value },
    )
    if (token !== requestSeq) {
      return
    }
    applyProgressSnapshot(nextProgress)
    syncProgressTransport()
  } catch (error) {
    if (token === requestSeq) {
      if (options.silent && progress.value) {
        refreshNotice.value = formatAPIError(error)
        syncPolling()
      } else {
        errorMessage.value = formatAPIError(error)
      }
    }
  } finally {
    if (token === requestSeq) {
      loading.value = false
      refreshing.value = false
    }
  }
}

async function submitTask() {
  const message = taskMessage.value.trim()
  if (!message) {
    taskError.value = '请输入任务内容'
    return
  }
  taskSubmitting.value = true
  taskError.value = ''
  taskNotice.value = ''
  try {
    const result = await createAgentTask({
      message,
      session_id: plan.value?.session_id,
      channel: 'web',
    })
    taskMessage.value = ''
    taskNotice.value = result.reply || '任务已创建'
    void loadTasks()
    if (result.plan?.id) {
      await router.push({ name: 'agent-plan', params: { id: String(result.plan.id) } })
    } else if (result.progress_url) {
      await router.push(result.progress_url)
    }
  } catch (error) {
    taskError.value = formatAPIError(error)
  } finally {
    taskSubmitting.value = false
  }
}

async function loadTasks() {
  tasksLoading.value = true
  tasksError.value = ''
  try {
    const result = await listAgentTasks({ limit: 20 })
    tasks.value = result.tasks
    taskSLA.value = result.sla
    taskCost.value = result.cost
    taskAlerts.value = result.alerts
    taskAlertPolicy.value = result.alert_policy
    taskCostTrend.value = result.cost_trend || []
    taskTrendSnapshot.value = result.trend_snapshot
    taskDeployment.value = result.deployment
    taskDrill.value = result.drill
    taskWeChatComponents.value = result.wechat_components
    taskLoadTest.value = result.load_test
    taskWeChatCallback.value = result.wechat_callback
    taskWriteSandbox.value = result.write_sandbox
    taskE2E.value = result.e2e
    taskRealIntegration.value = result.real_integration
    taskWeChatNative.value = result.wechat_native
    taskWriteLeastPrivilege.value = result.write_least_privilege
    taskOpsAcceptance.value = result.ops_acceptance
    taskWeChatNativePayload.value = result.wechat_native_payload
    taskWriteGray.value = result.write_gray
    taskAlertChannel.value = result.alert_channel
    taskLaunchDrill.value = result.launch_drill
    taskWeChatNativeIntegration.value = result.wechat_native_integration
    taskWriteReplay.value = result.write_replay
    taskLaunchApproval.value = result.launch_approval
    taskDailyReport.value = result.daily_report
    taskPreprod.value = result.preprod
    taskButtonLoop.value = result.button_loop
    taskWriteExecute.value = result.write_execute
    taskDailyPersist.value = result.daily_persist
    taskPostLaunchMonitor.value = result.post_launch_monitor
    taskReleaseApproval.value = result.release_approval
    taskButtonCallback.value = result.button_callback
    taskWriteAudit.value = result.write_audit
    taskDailySend.value = result.daily_send
    taskMonitorAlert.value = result.monitor_alert
    taskButtonDirectControl.value = result.button_direct_control
    taskWeChatE2E.value = result.wechat_e2e
    taskReleaseWindow.value = result.release_window
    taskWriteGrayExpansion.value = result.write_gray_expansion
    taskExternalMonitor.value = result.external_monitor
    taskReleaseWindowExecution.value = result.release_window_execution
    taskExternalMonitorRuntime.value = result.external_monitor_runtime
    taskWriteGrayReview.value = result.write_gray_review
    taskWeChatAcceptanceReview.value = result.wechat_acceptance_review
    taskOperationsDailyClosure.value = result.operations_daily_closure
    taskProductionRelease.value = result.production_release
    taskExternalMonitorConfig.value = result.external_monitor_config
    taskWriteRamp.value = result.write_ramp
    taskWeChatSignoff.value = result.wechat_signoff
    taskOperationsHandoff.value = result.operations_handoff
    taskProductionExecution.value = result.production_execution
    taskMonitorIntegration.value = result.monitor_integration
    taskWriteRampPolicy.value = result.write_ramp_policy
    taskWeChatFinalReport.value = result.wechat_final_report
    taskLaunchRuntimeOverview.value = result.launch_runtime_overview
    taskRuntimeParameters.value = result.runtime_parameters
    taskMonitorReadback.value = result.monitor_readback
    taskWriteRampRecommendation.value = result.write_ramp_recommendation
    taskWeChatUserFeedback.value = result.wechat_user_feedback
    taskOperationsRuntimeClosure.value = result.operations_runtime_closure
    taskOpsPanelConfig.value = result.ops_panel_config
    taskMonitorAutoReport.value = result.monitor_auto_report
    taskWriteRampStage.value = result.write_ramp_stage
    taskWeChatFeedbackLoop.value = result.wechat_feedback_loop
    taskOperationsClosedLoop.value = result.operations_closed_loop
    taskOpsDashboardInteraction.value = result.ops_dashboard_interaction
    taskAlertDedupeEscalation.value = result.alert_dedupe_escalation
    taskWriteStageRecord.value = result.write_stage_record
    taskWeChatFeedbackTicket.value = result.wechat_feedback_ticket
    taskOperationsHandling.value = result.operations_handling
    taskOpsActionDefinition.value = result.ops_action_definition
    taskAlertEscalationPolicy.value = result.alert_escalation_policy
    taskWriteStageApproval.value = result.write_stage_approval
    taskFeedbackTicketLifecycle.value = result.feedback_ticket_lifecycle
    taskOperationsActionClosure.value = result.operations_action_closure
    taskOpsAPIExecution.value = result.ops_api_execution
    taskAlertEscalationReceipt.value = result.alert_escalation_receipt
    taskWriteApprovalButton.value = result.write_approval_button
    taskFeedbackTicketSLA.value = result.feedback_ticket_sla
    taskOperationsExecution.value = result.operations_execution
    taskOpsExecutionRecord.value = result.ops_execution_record
    taskWeChatApprovalCallback.value = result.wechat_approval_callback
    taskFeedbackSLAReport.value = result.feedback_sla_report
    taskAlertAutoRecovery.value = result.alert_auto_recovery
    taskOperationsEvidence.value = result.operations_evidence
    taskUnifiedProgressComponent.value = result.unified_progress_component
    taskEvidenceDetailPage.value = result.evidence_detail_page
    taskCallbackReplayTool.value = result.callback_replay_tool
    taskRecoveryPolicyConfig.value = result.recovery_policy_config
    taskDualEndProgressEvidence.value = result.dual_end_progress_evidence
    taskWeChatProgressCard.value = result.wechat_progress_card
    taskWebEvidenceInteraction.value = result.web_evidence_interaction
    taskCallbackReplayPermission.value = result.callback_replay_permission
    taskRecoveryPolicyAudit.value = result.recovery_policy_audit
    taskDualEndInteraction.value = result.dual_end_interaction
    taskWeChatTemplateRender.value = result.wechat_template_render
    taskWebEvidenceRoute.value = result.web_evidence_route
    taskCallbackReplayApproval.value = result.callback_replay_approval
    taskRecoveryPolicyPersist.value = result.recovery_policy_persist
    taskDualEndInteractionLaunch.value = result.dual_end_interaction_launch
    taskWeChatTemplateSend.value = result.wechat_template_send
    taskWebEvidenceDetailView.value = result.web_evidence_detail_view
    taskCallbackReplayExecution.value = result.callback_replay_execution
    taskRecoveryPolicyVersion.value = result.recovery_policy_version
    taskDualEndRealInteraction.value = result.dual_end_real_interaction
    taskWeChatTemplateIntegration.value = result.wechat_template_integration
    taskWebEvidenceInteractionDetail.value = result.web_evidence_interaction_detail
    taskCallbackReplaySafetyAudit.value = result.callback_replay_safety_audit
    taskRecoveryPolicyGrayRelease.value = result.recovery_policy_gray_release
    taskDualEndRunLoop.value = result.dual_end_run_loop
    taskWeChatTemplatePilot.value = result.wechat_template_pilot
    taskWebEvidenceUserAction.value = result.web_evidence_user_action
    taskCallbackReplayResultTrace.value = result.callback_replay_result_trace
    taskRecoveryPolicyAutomation.value = result.recovery_policy_automation
    taskDualEndTaskClosure.value = result.dual_end_task_closure
    taskWeChatTemplatePilotMetric.value = result.wechat_template_pilot_metric
    taskWebEvidenceOperation.value = result.web_evidence_operation
    taskCallbackReplayResultQuery.value = result.callback_replay_result_query
    taskRecoveryAutomationExecution.value = result.recovery_automation_execution
    taskRealInteractionAutomation.value = result.real_interaction_automation
    taskWeChatWebProgressLink.value = result.wechat_web_progress_link
    taskReport.value = result.report
  } catch (error) {
    tasksError.value = formatAPIError(error)
  } finally {
    tasksLoading.value = false
  }
}

async function loadEvalRuns() {
  evalLoading.value = true
  evalError.value = ''
  try {
    const result = await listAgentEvalRuns({ limit: 5 })
    evalRuns.value = result.runs
    evalTrend.value = result.trend
    if (!evalRunDetail.value && evalRuns.value.length) {
      evalRunDetail.value = evalRuns.value[0]
    }
  } catch (error) {
    evalError.value = formatAPIError(error)
  } finally {
    evalLoading.value = false
  }
}

async function runEval() {
  evalRunning.value = true
  evalError.value = ''
  try {
    evalRunDetail.value = await runBuiltinAgentEval({ trigger: 'web' })
    await loadEvalRuns()
  } catch (error) {
    evalError.value = formatAPIError(error)
  } finally {
    evalRunning.value = false
  }
}

async function openTask(task: AgentTaskSummary) {
  if (task.progress_url) {
    await router.push(task.progress_url)
  }
}

async function retryStep(step: AgentPlanStep) {
  if (!plan.value || !isRetryableStep(step)) {
    return
  }
  retryingStepID.value = step.id
  controlError.value = ''
  try {
    await retryAgentPlanStep(plan.value.id, step.id, { reason: 'web retry request' })
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    retryingStepID.value = 0
  }
}

async function retryCurrentPlan() {
  if (!plan.value || !canRetryPlan.value) {
    return
  }
  retryingPlan.value = true
  controlError.value = ''
  try {
    await retryAgentPlan(plan.value.id, { reason: 'web batch retry request' })
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    retryingPlan.value = false
  }
}

async function recoverCurrentPlan() {
  if (!plan.value || !canRecoverPlan.value) {
    return
  }
  recoveringPlan.value = true
  controlError.value = ''
  try {
    await recoverAgentPlan(plan.value.id, { reason: 'web recovery request' })
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    recoveringPlan.value = false
  }
}

async function decideApproval(id: number, action: 'approve' | 'reject') {
  decidingApprovalID.value = id
  controlError.value = ''
  try {
    if (action === 'approve') {
      await approveAgentApprovalRecord(id)
    } else {
      await rejectAgentApprovalRecord(id)
    }
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    decidingApprovalID.value = 0
  }
}

async function cancelScheduledTask(id: number) {
  cancelingTaskID.value = id
  controlError.value = ''
  try {
    await cancelAgentScheduledTask(id)
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    cancelingTaskID.value = 0
  }
}

async function recoverScheduledTask(id: number) {
  recoveringTaskID.value = id
  controlError.value = ''
  try {
    await recoverAgentScheduledTask(id, { reason: 'web recovery request' })
    await loadPlan({ silent: true })
    await loadTasks()
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    recoveringTaskID.value = 0
  }
}

function isCancelableScheduledTask(status: string) {
  return status === 'queued' || status === 'running' || status === 'input_required'
}

function isRecoverableScheduledTask(status: string) {
  return status === 'running' || status === 'failed' || status === 'input_required'
}

function isRetryableStep(step: AgentPlanStep) {
  const maxRetries = step.max_retries > 0 ? step.max_retries : 1
  const strategy = (step.failure_strategy || '').toLowerCase()
  return step.status === 'failed' && step.retry_count < maxRetries && !strategy.includes('no_retry')
}

function isRetryExhausted(step: AgentPlanStep) {
  const maxRetries = step.max_retries > 0 ? step.max_retries : 1
  return step.status === 'failed' && step.retry_count >= maxRetries
}

function retryPreviousError(step: AgentPlanStep) {
  const value = step.retry_metadata?.previous_error_message
  return typeof value === 'string' && value.trim() ? value.trim() : ''
}

function applyProgressSnapshot(nextProgress: AgentProgressSnapshot) {
  progress.value = nextProgress
  plan.value = nextProgress.plan || null
  runs.value = nextProgress.runs || []
  streamCursor.value = nextProgress.event_cursor || ''
  refreshNotice.value = ''
  lastLoadedAt.value = new Date().toLocaleString('zh-CN', { hour12: false })
}

function progressStreamInput() {
  if (planID.value > 0) {
    return { plan_id: planID.value }
  }
  if (scheduledTaskID.value > 0) {
    return { scheduled_task_id: scheduledTaskID.value }
  }
  if (progress.value?.subject_type === 'run' && progress.value.subject_id > 0) {
    return { run_id: progress.value.subject_id }
  }
  return {}
}

function syncProgressTransport() {
  if (isTerminalPlan.value) {
    clearProgressStream()
    clearPolling()
    streamStatus.value = 'closed'
    return
  }
  if (typeof window !== 'undefined' && 'EventSource' in window) {
    startProgressStream()
    return
  }
  streamStatus.value = 'fallback'
  syncPolling()
}

function startProgressStream() {
  const url = agentProgressStreamURL(progressStreamInput())
  if (!url) {
    syncPolling()
    return
  }
  if (progressStream && progressStreamKey === url) {
    return
  }
  clearProgressStream()
  progressStreamKey = url
  streamStatus.value = 'connecting'
  streamError.value = ''
  progressStream = new EventSource(url, { withCredentials: true })
  progressStream.onopen = () => {
    streamStatus.value = 'connected'
    streamError.value = ''
    clearPolling()
  }
  progressStream.addEventListener('progress', (event) => {
    try {
      const payload = JSON.parse((event as MessageEvent).data) as { progress?: AgentProgressSnapshot }
      if (payload.progress) {
        applyProgressSnapshot(payload.progress)
        if (isTerminalAgentProgressStatus(payload.progress.status)) {
          clearProgressStream()
          clearPolling()
          streamStatus.value = 'closed'
        }
      }
    } catch (error) {
      streamError.value = error instanceof Error ? error.message : '实时事件解析失败'
    }
  })
  progressStream.addEventListener('heartbeat', (event) => {
    try {
      const payload = JSON.parse((event as MessageEvent).data) as { event_cursor?: string }
      streamCursor.value = payload.event_cursor || streamCursor.value
    } catch {
      streamCursor.value = streamCursor.value || progress.value?.event_cursor || ''
    }
  })
  progressStream.onerror = () => {
    streamStatus.value = 'fallback'
    streamError.value = '实时连接暂不可用，已切换为轮询'
    syncPolling()
  }
}

function clearProgressStream() {
  if (progressStream) {
    progressStream.close()
    progressStream = undefined
  }
  progressStreamKey = ''
}

function syncPolling() {
  clearPolling()
  if (streamStatus.value === 'connecting' || streamStatus.value === 'connected') {
    return
  }
  if (!isTerminalPlan.value) {
    pollTimer = window.setInterval(() => {
      void loadPlan({ silent: true })
    }, pollingIntervalMs())
  }
}

function pollingIntervalMs() {
  const status = progress.value?.status || plan.value?.status || ''
  return resolveAgentProgressPollingInterval(status, Boolean(refreshNotice.value))
}

function clearPolling() {
  if (pollTimer !== undefined) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    draft: '草稿',
    awaiting_approval: '等待确认',
    approved: '已批准',
    rejected: '已拒绝',
    expired: '已过期',
    executing: '执行中',
    completed: '已完成',
	    failed: '失败',
	    passed: '通过',
	    skipped: '跳过',
	    error: '错误',
    running: '运行中',
    succeeded: '成功',
    input_required: '需要输入',
    canceled: '已取消',
    pending: '待处理',
    queued: '排队中',
    not_required: '无需确认',
    created: '已创建',
    none: '无',
    ready: '就绪',
    review: '需复核',
    quota_exceeded: '配额超限',
    throttled: '已限流',
    active: '启用',
    muted: '静默',
    inactive: '未触发',
    sandboxed: '沙箱中',
    approval_required: '需审批',
  }
  return labels[status] || status || '未知'
}

function statusTone(status: string) {
  if (status === 'completed' || status === 'succeeded' || status === 'approved' || status === 'passed') {
    return 'ok'
  }
  if (status === 'failed' || status === 'rejected' || status === 'expired' || status === 'canceled' || status === 'error') {
    return 'bad'
  }
  if (status === 'executing' || status === 'running') {
    return 'active'
  }
  if (status === 'awaiting_approval' || status === 'input_required') {
    return 'warn'
  }
  return 'neutral'
}

function statusIcon(status: string) {
  const tone = statusTone(status)
  if (tone === 'ok') {
    return IconCheckCircleFill
  }
  if (tone === 'bad') {
    return IconCloseCircleFill
  }
  if (tone === 'active') {
    return IconPlayCircle
  }
  if (tone === 'warn') {
    return IconExclamationCircleFill
  }
  return IconClockCircle
}

function runRoleLabel(role: string) {
  if (role === 'controller') {
    return 'controller'
  }
  if (role === 'executor') {
    return 'executor'
  }
  return role || 'run'
}

function stepTitle(step: AgentPlanStep) {
  return step.title || step.capability_key || '计划步骤'
}

function formatTime(value?: string) {
  if (!value) {
    return '无'
  }
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function joined(values: string[] | undefined) {
  return values && values.length ? values.join('、') : '无'
}

function reportPairs(value: Record<string, number> | undefined) {
  if (!value) {
    return '无'
  }
  const pairs = Object.entries(value)
    .filter(([, count]) => count > 0)
    .sort(([left], [right]) => left.localeCompare(right))
    .slice(0, 6)
    .map(([key, count]) => `${key}:${count}`)
  return pairs.length ? pairs.join(' / ') : '无'
}

function alertReasonLabel(reason: string) {
  const labels: Record<string, string> = {
    plan_failed: '计划失败',
    handoff_required: '等待接管',
    low_quality: '低质量评分',
    quota_exceeded: '配额超限',
    scheduled_task_failed: '定时任务失败',
    scheduled_task_warning: '定时任务警告',
    notification_failed: '通知失败',
    admission_limited: '准入受限',
  }
  return labels[reason] || reason
}

function runSummary(run: AgentRun) {
  if (run.error_message) {
    return run.error_message
  }
  if (run.result_ref) {
    return run.result_ref
  }
  if (run.context_trace_ref) {
    return run.context_trace_ref
  }
  return run.model_key || '未记录输出引用'
}

function eventKindLabel(kind: string) {
  const labels: Record<string, string> = {
    plan: '计划',
    plan_step: '步骤',
    approval: '确认',
    run: '运行',
    observation: '观察',
    artifact: '产物',
    scheduled_task: '定时',
    audit: '审计',
  }
  return labels[kind] || kind || '事件'
}

function eventSourceLabel(source?: string) {
  const labels: Record<string, string> = {
    user_action: '用户操作',
    notification: '通知',
    capability: '能力',
    system: '系统',
  }
  return labels[source || ''] || source || ''
}

watch([planID, scheduledTaskID], () => {
  clearProgressStream()
  clearPolling()
  streamStatus.value = 'idle'
  streamError.value = ''
  streamCursor.value = ''
  progress.value = null
  plan.value = null
  runs.value = []
  if (planID.value > 0 || scheduledTaskID.value > 0) {
    void loadPlan()
  }
})

onMounted(() => {
	  void loadTasks()
	  void loadEvalRuns()
	  if (planID.value > 0 || scheduledTaskID.value > 0) {
    void loadPlan()
  }
})

onBeforeUnmount(() => {
  requestSeq++
  clearProgressStream()
  clearPolling()
})
</script>

<template>
  <section class="settings-page agent-plan-page">
    <section class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">发起任务</div>
          <div class="settings-panel__meta">{{ plan ? `当前会话 #${plan.session_id}` : 'Web Agent' }}</div>
        </div>
      </div>
      <form class="settings-form-grid" @submit.prevent="submitTask">
        <textarea
          v-model="taskMessage"
          class="settings-textarea"
          rows="3"
          placeholder="输入任务目标"
          :disabled="taskSubmitting"
        />
        <div v-if="taskError" class="settings-inline-alert settings-inline-alert--warning">
          {{ taskError }}
        </div>
        <div v-if="taskNotice" class="settings-inline-alert">
          {{ taskNotice }}
        </div>
        <button class="settings-action-button" type="submit" :disabled="taskSubmitting">
          <IconPlayCircle />
          {{ taskSubmitting ? '提交中' : '提交' }}
        </button>
      </form>
    </section>

    <section class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">最近任务</div>
          <div class="settings-panel__meta">{{ tasksLoading ? '加载中' : `${tasks.length} 条` }}</div>
        </div>
        <button class="settings-action-button" type="button" :disabled="tasksLoading" @click="loadTasks">
          <IconRefresh />
          刷新
        </button>
      </div>
      <div v-if="tasksError" class="settings-inline-alert settings-inline-alert--warning">
        {{ tasksError }}
      </div>
      <div v-if="taskSLA" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>计划 {{ taskSLA.plan_succeeded }}/{{ taskSLA.plan_count }} 成功</span>
          <span>计划失败 {{ taskSLA.plan_failed }}</span>
          <span>定时 {{ taskSLA.scheduled_task_succeeded }}/{{ taskSLA.scheduled_task_count }} 成功</span>
          <span>平均耗时 {{ taskSLA.average_plan_seconds }}s</span>
          <span>恢复 {{ taskSLA.recovery_count }}</span>
          <span>接管 {{ taskSLA.handoff_count }}</span>
          <span>通知 {{ taskSLA.notification_sent_count }}/{{ taskSLA.notification_failed_count }}</span>
        </div>
      </div>
      <div v-if="taskCost" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>工具 {{ taskCost.tool_calls }}</span>
          <span>外部访问 {{ taskCost.external_calls }}</span>
          <span>估算 token {{ taskCost.estimated_tokens }}</span>
          <span>重试 {{ taskCost.retry_count }}</span>
          <span>通知 {{ taskCost.notification_count }}</span>
          <span>定时 {{ taskCost.scheduled_tasks }}</span>
        </div>
      </div>
      <div v-if="taskAlertSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运行告警 {{ taskAlertSummary }}</span>
        </div>
      </div>
      <div v-if="taskAlertPolicySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>告警策略 {{ taskAlertPolicySummary }}</span>
        </div>
      </div>
      <div v-if="taskCostTrendSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>近 7 日成本趋势 {{ taskCostTrendSummary }}</span>
        </div>
      </div>
      <div v-if="taskTrendSnapshotSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>长期趋势 {{ taskTrendSnapshotSummary }}</span>
        </div>
      </div>
      <div v-if="taskDeploymentSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>部署验收 {{ taskDeploymentSummary }}</span>
          <span v-for="check in visibleDeploymentChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDrillSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>生产演练 {{ taskDrillSummary }}</span>
          <span v-for="check in visibleDrillChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatComponentSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微动作 {{ taskWeChatComponentSummary }}</span>
          <span v-for="action in visibleWeChatActions" :key="action.key">
            {{ action.label }} {{ action.fallback }}
          </span>
        </div>
      </div>
      <div v-if="taskLoadTestSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>多用户压测 {{ taskLoadTestSummary }}</span>
          <span v-for="check in visibleLoadTestChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatCallbackSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微回调联调 {{ taskWeChatCallbackSummary }}</span>
          <span v-for="check in visibleWeChatCallbackChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteSandboxSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写操作沙箱 {{ taskWriteSandboxSummary }}</span>
          <span v-for="check in visibleWriteSandboxChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskE2ESummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>端到端验收 {{ taskE2ESummary }}</span>
          <span v-for="check in visibleE2EChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRealIntegrationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>真实联调 {{ taskRealIntegrationSummary }}</span>
          <span v-for="check in visibleRealIntegrationChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatNativeSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微原生按钮 {{ taskWeChatNativeSummary }}</span>
          <span v-for="action in visibleWeChatNativeActions" :key="action.key">
            {{ action.label }} {{ action.style }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteLeastPrivilegeSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写最小权限 {{ taskWriteLeastPrivilegeSummary }}</span>
          <span v-for="check in visibleWriteLeastPrivilegeChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsAcceptanceSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运维验收 {{ taskOpsAcceptanceSummary }}</span>
          <span v-for="check in visibleOpsAcceptanceChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatNativePayloadSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微原生发送 {{ taskWeChatNativePayloadSummary }}</span>
          <span v-for="button in visibleWeChatNativeButtons" :key="button.key">
            {{ button.label }} {{ button.style }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteGraySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写能力灰度 {{ taskWriteGraySummary }}</span>
          <span v-for="check in visibleWriteGrayChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskAlertChannelSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>告警通道 {{ taskAlertChannelSummary }}</span>
          <span v-for="channel in visibleAlertChannels" :key="channel.key">
            {{ channel.key }} {{ statusLabel(channel.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskLaunchDrillSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>上线演练 {{ taskLaunchDrillSummary }}</span>
          <span v-for="check in visibleLaunchDrillChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatNativeIntegrationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微按钮联调 {{ taskWeChatNativeIntegrationSummary }}</span>
          <span v-for="check in visibleWeChatNativeIntegrationChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteReplaySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写操作回放 {{ taskWriteReplaySummary }}</span>
          <span v-for="check in visibleWriteReplayChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskLaunchApprovalSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>上线审批 {{ taskLaunchApprovalSummary }}</span>
          <span v-for="check in visibleLaunchApprovalChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDailyReportSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>生产日报 {{ taskDailyReportSummary }}</span>
          <span v-for="check in visibleDailyReportChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskPreprodSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>预发布验收 {{ taskPreprodSummary }}</span>
          <span v-for="check in visiblePreprodChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskButtonLoopSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>按钮闭环 {{ taskButtonLoopSummary }}</span>
          <span v-for="action in visibleButtonLoopActions" :key="action.key">
            {{ action.label }} {{ action.style }}
          </span>
          <span v-for="check in visibleButtonLoopChecks" :key="`button-loop-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteExecuteSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写能力执行 {{ taskWriteExecuteSummary }}</span>
          <span v-for="check in visibleWriteExecuteChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDailyPersistSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>日报持久化 {{ taskDailyPersistSummary }}</span>
          <span v-for="check in visibleDailyPersistChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskPostLaunchMonitorSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>上线后监控 {{ taskPostLaunchMonitorSummary }}</span>
          <span v-for="check in visiblePostLaunchMonitorChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskReleaseApprovalSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>发布审批执行 {{ taskReleaseApprovalSummary }}</span>
          <span v-for="check in visibleReleaseApprovalChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskButtonCallbackSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>按钮回调处理 {{ taskButtonCallbackSummary }}</span>
          <span v-for="action in visibleButtonCallbackActions" :key="action.key">
            {{ action.label }} {{ action.handler }}
          </span>
          <span v-for="check in visibleButtonCallbackChecks" :key="`button-callback-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteAuditSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写审计复盘 {{ taskWriteAuditSummary }}</span>
          <span v-for="check in visibleWriteAuditChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDailySendSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>日报定时发送 {{ taskDailySendSummary }}</span>
          <span v-for="check in visibleDailySendChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskMonitorAlertSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>监控告警实测 {{ taskMonitorAlertSummary }}</span>
          <span v-for="check in visibleMonitorAlertChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskButtonDirectControlSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>按钮直接控制 {{ taskButtonDirectControlSummary }}</span>
          <span v-for="action in visibleButtonDirectControlActions" :key="action.key">
            {{ action.label }} {{ action.handler }}
          </span>
          <span v-for="check in visibleButtonDirectControlChecks" :key="`button-direct-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatE2ESummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微端到端 {{ taskWeChatE2ESummary }}</span>
          <span v-for="check in visibleWeChatE2EChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskReleaseWindowSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>发布窗口 {{ taskReleaseWindowSummary }}</span>
          <span v-for="check in visibleReleaseWindowChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteGrayExpansionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写灰度扩容 {{ taskWriteGrayExpansionSummary }}</span>
          <span v-for="check in visibleWriteGrayExpansionChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskExternalMonitorSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>外部监控对接 {{ taskExternalMonitorSummary }}</span>
          <span v-for="check in visibleExternalMonitorChecks" :key="check.key">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskReleaseWindowExecutionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>发布窗口执行 {{ taskReleaseWindowExecutionSummary }}</span>
          <span v-for="check in visibleReleaseWindowExecutionChecks" :key="`release-execution-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskExternalMonitorRuntimeSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>外部监控运行 {{ taskExternalMonitorRuntimeSummary }}</span>
          <span v-for="check in visibleExternalMonitorRuntimeChecks" :key="`monitor-runtime-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteGrayReviewSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写灰度复核 {{ taskWriteGrayReviewSummary }}</span>
          <span v-for="check in visibleWriteGrayReviewChecks" :key="`write-gray-review-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatAcceptanceReviewSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微验收复盘 {{ taskWeChatAcceptanceReviewSummary }}</span>
          <span v-for="check in visibleWeChatAcceptanceReviewChecks" :key="`wechat-acceptance-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsDailyClosureSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营日报闭环 {{ taskOperationsDailyClosureSummary }}</span>
          <span v-for="check in visibleOperationsDailyClosureChecks" :key="`daily-closure-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskProductionReleaseSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>生产发布 {{ taskProductionReleaseSummary }}</span>
          <span v-for="check in visibleProductionReleaseChecks" :key="`production-release-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskExternalMonitorConfigSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>监控配置 {{ taskExternalMonitorConfigSummary }}</span>
          <span v-for="check in visibleExternalMonitorConfigChecks" :key="`monitor-config-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteRampSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写能力放量 {{ taskWriteRampSummary }}</span>
          <span v-for="check in visibleWriteRampChecks" :key="`write-ramp-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatSignoffSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微签收 {{ taskWeChatSignoffSummary }}</span>
          <span v-for="check in visibleWeChatSignoffChecks" :key="`wechat-signoff-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsHandoffSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营交接 {{ taskOperationsHandoffSummary }}</span>
          <span v-for="check in visibleOperationsHandoffChecks" :key="`operations-handoff-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskProductionExecutionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>生产执行 {{ taskProductionExecutionSummary }}</span>
          <span v-for="check in visibleProductionExecutionChecks" :key="`production-execution-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskMonitorIntegrationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>监控联调 {{ taskMonitorIntegrationSummary }}</span>
          <span v-for="check in visibleMonitorIntegrationChecks" :key="`monitor-integration-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteRampPolicySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写放量策略 {{ taskWriteRampPolicySummary }}</span>
          <span v-for="check in visibleWriteRampPolicyChecks" :key="`write-ramp-policy-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatFinalReportSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微最终汇报 {{ taskWeChatFinalReportSummary }}</span>
          <a v-if="taskWeChatFinalReport?.progress_url" :href="taskWeChatFinalReport.progress_url" target="_blank" rel="noreferrer">
            {{ taskWeChatFinalReport.progress_url }}
          </a>
          <span v-for="check in visibleWeChatFinalReportChecks" :key="`wechat-final-report-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskLaunchRuntimeOverviewSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>上线运行总览 {{ taskLaunchRuntimeOverviewSummary }}</span>
          <span v-for="check in visibleLaunchRuntimeOverviewChecks" :key="`launch-runtime-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRuntimeParametersSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运行参数 {{ taskRuntimeParametersSummary }}</span>
          <span v-for="check in visibleRuntimeParametersChecks" :key="`runtime-parameters-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskMonitorReadbackSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>监控回读 {{ taskMonitorReadbackSummary }}</span>
          <span v-for="check in visibleMonitorReadbackChecks" :key="`monitor-readback-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteRampRecommendationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写放量建议 {{ taskWriteRampRecommendationSummary }}</span>
          <span v-for="check in visibleWriteRampRecommendationChecks" :key="`write-ramp-recommendation-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatUserFeedbackSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微用户反馈 {{ taskWeChatUserFeedbackSummary }}</span>
          <span v-for="check in visibleWeChatUserFeedbackChecks" :key="`wechat-user-feedback-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsRuntimeClosureSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运行运营闭环 {{ taskOperationsRuntimeClosureSummary }}</span>
          <span v-for="check in visibleOperationsRuntimeClosureChecks" :key="`operations-runtime-closure-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsPanelConfigSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营面板 {{ taskOpsPanelConfigSummary }}</span>
          <span v-for="check in visibleOpsPanelConfigChecks" :key="`ops-panel-config-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskMonitorAutoReportSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>监控自动汇报 {{ taskMonitorAutoReportSummary }}</span>
          <span v-for="check in visibleMonitorAutoReportChecks" :key="`monitor-auto-report-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteRampStageSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写分阶段放量 {{ taskWriteRampStageSummary }}</span>
          <span v-for="check in visibleWriteRampStageChecks" :key="`write-ramp-stage-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatFeedbackLoopSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微反馈处理 {{ taskWeChatFeedbackLoopSummary }}</span>
          <span v-for="check in visibleWeChatFeedbackLoopChecks" :key="`wechat-feedback-loop-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsClosedLoopSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营闭环总览 {{ taskOperationsClosedLoopSummary }}</span>
          <span v-for="check in visibleOperationsClosedLoopChecks" :key="`operations-closed-loop-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsDashboardInteractionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营交互面板 {{ taskOpsDashboardInteractionSummary }}</span>
          <span v-for="action in visibleOpsDashboardInteractionActions" :key="`ops-dashboard-action-${action}`">
            {{ action }}
          </span>
          <span v-for="check in visibleOpsDashboardInteractionChecks" :key="`ops-dashboard-interaction-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskAlertDedupeEscalationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>异常去重升级 {{ taskAlertDedupeEscalationSummary }}</span>
          <span v-for="check in visibleAlertDedupeEscalationChecks" :key="`alert-dedupe-escalation-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteStageRecordSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写阶段推进记录 {{ taskWriteStageRecordSummary }}</span>
          <span v-for="check in visibleWriteStageRecordChecks" :key="`write-stage-record-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatFeedbackTicketSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微反馈工单 {{ taskWeChatFeedbackTicketSummary }}</span>
          <span v-for="check in visibleWeChatFeedbackTicketChecks" :key="`wechat-feedback-ticket-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsHandlingSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营处理总览 {{ taskOperationsHandlingSummary }}</span>
          <span v-for="check in visibleOperationsHandlingChecks" :key="`operations-handling-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsActionDefinitionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营动作定义 {{ taskOpsActionDefinitionSummary }}</span>
          <span v-for="action in visibleOpsActionDefinitionActions" :key="`ops-action-definition-${action.key}`">
            {{ action.label }} {{ action.permission_constraint }}
          </span>
          <span v-for="check in visibleOpsActionDefinitionChecks" :key="`ops-action-definition-check-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskAlertEscalationPolicySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>异常升级策略 {{ taskAlertEscalationPolicySummary }}</span>
          <span v-for="check in visibleAlertEscalationPolicyChecks" :key="`alert-escalation-policy-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteStageApprovalSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写阶段审批 {{ taskWriteStageApprovalSummary }}</span>
          <span v-for="check in visibleWriteStageApprovalChecks" :key="`write-stage-approval-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskFeedbackTicketLifecycleSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>反馈工单生命周期 {{ taskFeedbackTicketLifecycleSummary }}</span>
          <span v-for="check in visibleFeedbackTicketLifecycleChecks" :key="`feedback-ticket-lifecycle-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsActionClosureSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营动作闭环 {{ taskOperationsActionClosureSummary }}</span>
          <span v-for="check in visibleOperationsActionClosureChecks" :key="`operations-action-closure-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsAPIExecutionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营 API 执行 {{ taskOpsAPIExecutionSummary }}</span>
          <span v-for="item in visibleOpsAPIExecutionItems" :key="`ops-api-execution-${item.action_key}`">
            {{ item.action_key }} {{ statusLabel(item.execution_status) }}
          </span>
          <span v-for="check in visibleOpsAPIExecutionChecks" :key="`ops-api-execution-check-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskAlertEscalationReceiptSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>异常升级回执 {{ taskAlertEscalationReceiptSummary }}</span>
          <span v-for="check in visibleAlertEscalationReceiptChecks" :key="`alert-escalation-receipt-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWriteApprovalButtonSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>写审批按钮 {{ taskWriteApprovalButtonSummary }}</span>
          <span v-for="button in visibleWriteApprovalButtons" :key="`write-approval-button-${button.button_key}`">
            {{ button.channel }} {{ button.approval_status }}
          </span>
          <span v-for="check in visibleWriteApprovalButtonChecks" :key="`write-approval-button-check-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskFeedbackTicketSLASummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>反馈工单 SLA {{ taskFeedbackTicketSLASummary }}</span>
          <span v-for="check in visibleFeedbackTicketSLAChecks" :key="`feedback-ticket-sla-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsExecutionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营执行闭环 {{ taskOperationsExecutionSummary }}</span>
          <span v-for="check in visibleOperationsExecutionChecks" :key="`operations-execution-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOpsExecutionRecordSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营执行记录 {{ taskOpsExecutionRecordSummary }}</span>
          <span v-for="record in visibleOpsExecutionRecords" :key="`ops-execution-record-${record.record_key}`">
            {{ record.action_key }} {{ statusLabel(record.execution_status) }}
          </span>
          <span v-for="check in visibleOpsExecutionRecordChecks" :key="`ops-execution-record-check-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatApprovalCallbackSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微审批回调 {{ taskWeChatApprovalCallbackSummary }}</span>
          <span v-for="check in visibleWeChatApprovalCallbackChecks" :key="`wechat-approval-callback-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskFeedbackSLAReportSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>工单 SLA 报表 {{ taskFeedbackSLAReportSummary }}</span>
          <span v-for="check in visibleFeedbackSLAReportChecks" :key="`feedback-sla-report-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskAlertAutoRecoverySummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>异常自动恢复 {{ taskAlertAutoRecoverySummary }}</span>
          <span v-for="check in visibleAlertAutoRecoveryChecks" :key="`alert-auto-recovery-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskOperationsEvidenceSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>运营证据闭环 {{ taskOperationsEvidenceSummary }}</span>
          <span v-for="check in visibleOperationsEvidenceChecks" :key="`operations-evidence-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskUnifiedProgressComponentSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>统一进度组件 {{ taskUnifiedProgressComponentSummary }}</span>
          <span v-for="check in visibleUnifiedProgressComponentChecks" :key="`unified-progress-component-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskEvidenceDetailPageSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>证据明细页 {{ taskEvidenceDetailPageSummary }}</span>
          <span v-for="check in visibleEvidenceDetailPageChecks" :key="`evidence-detail-page-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskCallbackReplayToolSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>回调重放工具 {{ taskCallbackReplayToolSummary }}</span>
          <span v-for="check in visibleCallbackReplayToolChecks" :key="`callback-replay-tool-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRecoveryPolicyConfigSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>恢复策略配置 {{ taskRecoveryPolicyConfigSummary }}</span>
          <span v-for="check in visibleRecoveryPolicyConfigChecks" :key="`recovery-policy-config-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDualEndProgressEvidenceSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>双端进度证据 {{ taskDualEndProgressEvidenceSummary }}</span>
          <span v-for="check in visibleDualEndProgressEvidenceChecks" :key="`dual-end-progress-evidence-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatProgressCardSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微进度卡片 {{ taskWeChatProgressCardSummary }}</span>
          <span v-for="action in visibleWeChatProgressCardActions" :key="`wechat-progress-card-${action.key}`">
            {{ action.label }} {{ statusLabel(action.status) }}
          </span>
          <span v-for="check in visibleWeChatProgressCardChecks" :key="`wechat-progress-card-check-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWebEvidenceInteractionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>Web 证据交互 {{ taskWebEvidenceInteractionSummary }}</span>
          <span v-for="check in visibleWebEvidenceInteractionChecks" :key="`web-evidence-interaction-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskCallbackReplayPermissionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>回调重放权限 {{ taskCallbackReplayPermissionSummary }}</span>
          <span v-for="check in visibleCallbackReplayPermissionChecks" :key="`callback-replay-permission-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRecoveryPolicyAuditSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>恢复策略审计 {{ taskRecoveryPolicyAuditSummary }}</span>
          <span v-for="check in visibleRecoveryPolicyAuditChecks" :key="`recovery-policy-audit-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDualEndInteractionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>双端交互治理 {{ taskDualEndInteractionSummary }}</span>
          <span v-for="check in visibleDualEndInteractionChecks" :key="`dual-end-interaction-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatTemplateRenderSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微模板渲染 {{ taskWeChatTemplateRenderSummary }}</span>
          <span v-for="check in visibleWeChatTemplateRenderChecks" :key="`wechat-template-render-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWebEvidenceRouteSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>Web 证据路由 {{ taskWebEvidenceRouteSummary }}</span>
          <span v-for="check in visibleWebEvidenceRouteChecks" :key="`web-evidence-route-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskCallbackReplayApprovalSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>回调重放审批 {{ taskCallbackReplayApprovalSummary }}</span>
          <span v-for="check in visibleCallbackReplayApprovalChecks" :key="`callback-replay-approval-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRecoveryPolicyPersistSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>恢复策略持久化 {{ taskRecoveryPolicyPersistSummary }}</span>
          <span v-for="check in visibleRecoveryPolicyPersistChecks" :key="`recovery-policy-persist-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDualEndInteractionLaunchSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>双端交互落地 {{ taskDualEndInteractionLaunchSummary }}</span>
          <span v-for="check in visibleDualEndInteractionLaunchChecks" :key="`dual-end-interaction-launch-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatTemplateSendSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微模板发送 {{ taskWeChatTemplateSendSummary }}</span>
          <span v-for="check in visibleWeChatTemplateSendChecks" :key="`wechat-template-send-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWebEvidenceDetailViewSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>Web 证据详情 {{ taskWebEvidenceDetailViewSummary }}</span>
          <span v-for="check in visibleWebEvidenceDetailViewChecks" :key="`web-evidence-detail-view-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskCallbackReplayExecutionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>回调重放执行 {{ taskCallbackReplayExecutionSummary }}</span>
          <span v-for="check in visibleCallbackReplayExecutionChecks" :key="`callback-replay-execution-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRecoveryPolicyVersionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>恢复策略版本 {{ taskRecoveryPolicyVersionSummary }}</span>
          <span v-for="check in visibleRecoveryPolicyVersionChecks" :key="`recovery-policy-version-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDualEndRealInteractionSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>双端真实交互 {{ taskDualEndRealInteractionSummary }}</span>
          <span v-for="check in visibleDualEndRealInteractionChecks" :key="`dual-end-real-interaction-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWeChatTemplateIntegrationSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>企微模板联调 {{ taskWeChatTemplateIntegrationSummary }}</span>
          <span v-for="check in visibleWeChatTemplateIntegrationChecks" :key="`wechat-template-integration-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskWebEvidenceInteractionDetailSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>Web 证据交互 {{ taskWebEvidenceInteractionDetailSummary }}</span>
          <span v-for="check in visibleWebEvidenceInteractionDetailChecks" :key="`web-evidence-interaction-detail-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskCallbackReplaySafetyAuditSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>回放安全审计 {{ taskCallbackReplaySafetyAuditSummary }}</span>
          <span v-for="check in visibleCallbackReplaySafetyAuditChecks" :key="`callback-replay-safety-audit-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskRecoveryPolicyGrayReleaseSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>恢复策略灰度 {{ taskRecoveryPolicyGrayReleaseSummary }}</span>
          <span v-for="check in visibleRecoveryPolicyGrayReleaseChecks" :key="`recovery-policy-gray-release-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskDualEndRunLoopSummary" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>双端运行闭环 {{ taskDualEndRunLoopSummary }}</span>
          <span v-for="check in visibleDualEndRunLoopChecks" :key="`dual-end-run-loop-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-for="item in taskClosureSummaryItems" :key="item.key" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>{{ item.title }} {{ item.summary }}</span>
          <span v-for="check in item.checks" :key="`${item.key}-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-for="item in taskRealInteractionSummaryItems" :key="item.key" class="agent-plan-summary">
        <div class="agent-plan-summary__meta">
          <span>{{ item.title }} {{ item.summary }}</span>
          <a v-if="item.progressURL" :href="item.progressURL" target="_blank" rel="noreferrer">
            {{ item.progressURL }}
          </a>
          <span v-for="check in item.checks" :key="`${item.key}-${check.key}`">
            {{ check.key }} {{ statusLabel(check.status) }}
          </span>
        </div>
      </div>
      <div v-if="taskReport" class="agent-plan-step__meta">
        <span>状态 {{ reportPairs(taskReport.by_status) }}</span>
        <span>入口 {{ reportPairs(taskReport.by_entry) }}</span>
        <span>能力 {{ reportPairs(taskReport.by_capability) }}</span>
        <span>接管 {{ reportPairs(taskReport.by_handoff) }}</span>
      </div>
      <div v-if="tasks.length" class="agent-run-list">
        <article v-for="task in tasks" :key="task.id" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(task.status)}`">
            <component :is="statusIcon(task.status)" />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ task.goal || task.summary || task.id }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(task.status)}`">
                {{ statusLabel(task.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>{{ task.kind === 'scheduled_task' ? '定时任务' : '执行计划' }}</span>
              <span v-if="task.plan_id">plan #{{ task.plan_id }}</span>
              <span v-if="task.scheduled_task_id">task #{{ task.scheduled_task_id }}</span>
              <span v-if="task.permission_status">权限 {{ task.permission_status }}</span>
              <span v-if="task.budget_status">预算 {{ task.budget_status }}</span>
              <span v-if="task.quality_status">质量 {{ task.quality_status }}</span>
              <span v-if="task.handoff_status">接管 {{ task.handoff_status }}</span>
              <span>{{ formatTime(task.updated_at) }}</span>
            </div>
            <p>{{ task.latest_progress || task.summary || '无摘要' }}</p>
            <p v-if="task.observability">运行观测：{{ task.observability }}</p>
            <p v-if="task.next_action">下一步：{{ task.next_action }}</p>
            <button class="settings-action-button" type="button" @click="openTask(task)">
              <IconPlayCircle />
              查看
            </button>
          </div>
        </article>
      </div>
      <div v-else class="agent-plan-empty">暂无任务</div>
	    </section>

	    <section class="settings-panel settings-panel--wide">
	      <div class="settings-panel__header">
	        <div>
	          <div class="settings-panel__title">评测基线</div>
	          <div class="settings-panel__meta">{{ evalLoading ? '加载中' : `${evalRuns.length} 次运行` }}</div>
	        </div>
	        <button class="settings-action-button" type="button" :disabled="evalRunning" @click="runEval">
	          <IconPlayCircle />
	          {{ evalRunning ? '运行中' : '运行评测' }}
	        </button>
	      </div>
	      <div v-if="evalError" class="settings-inline-alert settings-inline-alert--warning">
	        {{ evalError }}
	      </div>
	      <dl v-if="evalTrend" class="settings-description-list">
	        <div>
	          <dt>通过率</dt>
	          <dd>{{ Math.round(evalTrend.pass_rate * 100) }}%</dd>
	        </div>
	        <div>
	          <dt>运行</dt>
	          <dd>{{ evalTrend.run_count }} 次 / {{ evalTrend.completed_count }} 完成</dd>
	        </div>
	        <div>
	          <dt>失败</dt>
	          <dd>{{ evalTrend.failed_result_count }} 个结果 / {{ evalTrend.failed_run_count }} 次运行</dd>
	        </div>
	        <div>
	          <dt>最近</dt>
	          <dd>{{ formatTime(evalTrend.latest_run_at) }}</dd>
	        </div>
	        <div>
	          <dt>类型</dt>
	          <dd>{{ joined(evalTrend.failure_summary) }}</dd>
	        </div>
	      </dl>
	      <div v-if="evalRunDetail" class="agent-plan-summary">
	        <div class="agent-plan-summary__status" :class="`agent-plan-summary__status--${statusTone(evalRunDetail.status)}`">
	          <component :is="statusIcon(evalRunDetail.status)" />
	          <span>{{ statusLabel(evalRunDetail.status) }}</span>
	        </div>
	        <div class="agent-plan-summary__meta">
	          <span>run #{{ evalRunDetail.id }}</span>
	          <span>{{ evalRunDetail.passed_count }}/{{ evalRunDetail.case_count }} 通过</span>
	          <span v-if="evalRunDetail.failed_count">失败 {{ evalRunDetail.failed_count }}</span>
	          <span>{{ formatTime(evalRunDetail.completed_at || evalRunDetail.updated_at) }}</span>
	        </div>
	      </div>
	      <div v-if="evalRunDetail?.results?.length" class="agent-run-list">
	        <article v-for="result in evalRunDetail.results" :key="result.id" class="agent-run-row">
	          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(result.status)}`">
	            <component :is="statusIcon(result.status)" />
	          </div>
	          <div class="agent-run-row__body">
	            <div class="agent-run-row__top">
	              <strong>case #{{ result.case_id }}</strong>
	              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(result.status)}`">
	                {{ statusLabel(result.status) }}
	              </span>
	            </div>
	            <div class="agent-run-row__meta">
	              <span>score {{ result.score }}</span>
	              <span>expected {{ result.expected || '无' }}</span>
	              <span>actual {{ result.actual || '无' }}</span>
	            </div>
	            <p v-if="result.failure_reason" class="agent-plan-step__error">{{ result.failure_reason }}</p>
	            <p>{{ joined(result.evidence_refs) }}</p>
	          </div>
	        </article>
	      </div>
	      <div v-else-if="!evalRunDetail" class="agent-plan-empty">暂无评测运行</div>
	    </section>

	    <section class="settings-panel settings-panel--wide">
	      <div class="settings-panel__header agent-plan-page__header">
        <div>
          <div class="settings-panel__title">Agent 执行进度</div>
          <div class="settings-panel__meta">
            {{ plan ? `计划 #${plan.id} / ${statusMeta}` : '加载进度数据' }}
          </div>
        </div>
        <button
          v-if="canRetryPlan"
          class="settings-action-button agent-plan-page__refresh"
          type="button"
          :disabled="retryingPlan"
          @click="retryCurrentPlan"
        >
          <IconRefresh />
          {{ retryingPlan ? '重试中' : '重试失败步骤' }}
        </button>
        <button
          v-if="canRecoverPlan"
          class="settings-action-button agent-plan-page__refresh"
          type="button"
          :disabled="recoveringPlan"
          @click="recoverCurrentPlan"
        >
          <IconPlayCircle />
          {{ recoveringPlan ? '恢复中' : '恢复计划' }}
        </button>
        <button class="settings-action-button agent-plan-page__refresh" type="button" :disabled="loading" @click="loadPlan()">
          <IconRefresh :class="{ 'agent-plan-page__spin': refreshing }" />
          {{ refreshing ? '刷新中' : '刷新' }}
        </button>
      </div>

      <div v-if="errorMessage" class="settings-inline-alert settings-inline-alert--warning">
        {{ errorMessage }}
      </div>
      <div v-if="refreshNotice && progress" class="settings-inline-alert settings-inline-alert--warning">
        最近一次刷新失败：{{ refreshNotice }}
      </div>
      <div v-if="controlError" class="settings-inline-alert settings-inline-alert--warning">
        {{ controlError }}
      </div>

      <div v-if="loading && !plan" class="agent-plan-empty">加载中</div>

      <template v-if="plan">
        <div class="agent-plan-summary">
          <div class="agent-plan-summary__status" :class="`agent-plan-summary__status--${statusTone(plan.status)}`">
            <component :is="statusIcon(plan.status)" />
            <span>{{ statusLabel(plan.status) }}</span>
          </div>
          <div class="agent-plan-summary__progress" aria-hidden="true">
            <span :style="{ width: `${progressPercent}%` }"></span>
          </div>
          <div class="agent-plan-summary__meta">
            <span>{{ progressPercent }}%</span>
            <span v-if="activeStepCount">执行中 {{ activeStepCount }}</span>
            <span v-if="failedStepCount">失败 {{ failedStepCount }}</span>
            <span v-if="progress">版本 {{ progress.version }}</span>
            <span>最近刷新 {{ lastLoadedAt || '无' }}</span>
          </div>
        </div>

        <dl class="settings-description-list">
          <div>
            <dt>目标</dt>
            <dd>{{ plan.goal || '无' }}</dd>
          </div>
          <div>
            <dt>计划</dt>
            <dd>{{ plan.summary || '无' }}</dd>
          </div>
          <div>
            <dt>影响</dt>
            <dd>{{ plan.impact_summary || '无' }}</dd>
          </div>
          <div>
            <dt>调度方式</dt>
            <dd>controller 生成执行计划，executor 按步骤调用授权能力</dd>
          </div>
          <div>
            <dt>下一步</dt>
            <dd>{{ progress?.next_action || '无' }}</dd>
          </div>
          <template v-if="hasMultiTurnContext">
            <div v-if="multiTurnOriginalGoal">
              <dt>原始目标</dt>
              <dd>{{ multiTurnOriginalGoal }}</dd>
            </div>
            <div v-if="multiTurnLatestInstruction">
              <dt>最近指令</dt>
              <dd>{{ multiTurnLatestInstruction }}</dd>
            </div>
            <div v-if="multiTurnAppendedInputs.length">
              <dt>追加约束</dt>
              <dd>{{ multiTurnAppendedInputs.join('；') }}</dd>
            </div>
            <div v-if="multiTurnFollowupQuestions.length">
              <dt>追问记录</dt>
              <dd>{{ multiTurnFollowupQuestions.join('；') }}</dd>
            </div>
            <div v-if="multiTurnStoppedReason">
              <dt>停止原因</dt>
              <dd>{{ multiTurnStoppedReason }}</dd>
            </div>
            <div v-if="parentPlanID">
              <dt>父计划</dt>
              <dd>#{{ parentPlanID }} {{ parentPlanGoal || '无' }}</dd>
            </div>
            <div v-if="resultFreshnessStatus">
              <dt>结果新鲜度</dt>
              <dd>{{ resultFreshnessStatus }}{{ resultFreshnessHint ? ` / ${resultFreshnessHint}` : '' }}</dd>
            </div>
            <div v-if="resultReuseEvidenceRefs.length || parentPlanEvidenceRefs.length">
              <dt>复用证据</dt>
              <dd>{{ joined(resultReuseEvidenceRefs.length ? resultReuseEvidenceRefs : parentPlanEvidenceRefs) }}</dd>
            </div>
          </template>
	          <div>
	            <dt>事件游标</dt>
	            <dd>{{ streamCursor || progress?.event_cursor || '无' }}</dd>
	          </div>
	          <div>
	            <dt>实时连接</dt>
	            <dd>{{ streamStatusLabel }}</dd>
	          </div>
          <div>
            <dt>授权范围</dt>
            <dd>{{ joined(plan.allowed_scopes) }}</dd>
          </div>
          <div>
            <dt>风控策略</dt>
            <dd>{{ plan.policy_decision || '无' }} / {{ plan.risk_level || 'unknown' }}</dd>
          </div>
          <div v-if="planPermissionSummary">
            <dt>权限治理</dt>
            <dd>{{ planPermissionSummary }}</dd>
          </div>
          <div v-if="planBudgetSummary">
            <dt>预算治理</dt>
            <dd>{{ planBudgetSummary }}</dd>
          </div>
          <div v-if="planQualitySummary">
            <dt>结果质量</dt>
            <dd>{{ planQualitySummary }}</dd>
          </div>
          <div v-if="planCostSummary">
            <dt>成本摘要</dt>
            <dd>{{ planCostSummary }}</dd>
          </div>
          <div v-if="planRecoverySummary">
            <dt>恢复策略</dt>
            <dd>{{ planRecoverySummary }}</dd>
          </div>
          <div v-if="deploymentAcceptanceSummary">
            <dt>部署验收</dt>
            <dd>{{ deploymentAcceptanceSummary }}</dd>
          </div>
          <div v-if="deploymentAcceptanceChecks.length">
            <dt>验收检查</dt>
            <dd>
              <span v-for="(check, index) in deploymentAcceptanceChecks" :key="`${asText(check.key)}-${index}`">
                {{ asText(check.key) }}: {{ asText(check.status) }}
              </span>
            </dd>
          </div>
          <div v-if="runtimeObservabilitySummary">
            <dt>运行观测</dt>
            <dd>{{ runtimeObservabilitySummary }}</dd>
          </div>
          <div v-if="handoffSummary">
            <dt>人工接管</dt>
            <dd>{{ handoffSummary }}</dd>
          </div>
          <div v-if="plan.error_message">
            <dt>错误</dt>
            <dd>{{ plan.error_message }}</dd>
          </div>
	        </dl>
	        <div v-if="streamError" class="settings-inline-alert settings-inline-alert--warning">
	          {{ streamError }}
	        </div>
	      </template>
    </section>

    <section v-if="progressPhases.length" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">统一进度阶段</div>
          <div class="settings-panel__meta">{{ progressPhases.length }} 个阶段</div>
        </div>
      </div>

      <div class="agent-run-list">
        <article v-for="phase in progressPhases" :key="phase.key" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(phase.status)}`">
            <component :is="statusIcon(phase.status)" />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ phase.title }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(phase.status)}`">
                {{ statusLabel(phase.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>{{ phase.key }}</span>
              <span v-if="phase.updated_at">{{ formatTime(phase.updated_at) }}</span>
            </div>
            <p>{{ phase.summary || '无' }}</p>
          </div>
        </article>
      </div>
    </section>

    <section v-if="plan" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">实施步骤</div>
          <div class="settings-panel__meta">{{ sortedSteps.length }} 个步骤</div>
        </div>
      </div>

      <div v-if="sortedSteps.length" class="agent-plan-steps">
        <article v-for="step in sortedSteps" :key="step.id" class="agent-plan-step">
          <div class="agent-plan-step__index">{{ step.step_order }}</div>
          <div class="agent-plan-step__body">
            <div class="agent-plan-step__top">
              <strong>{{ stepTitle(step) }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(step.status)}`">
                <component :is="statusIcon(step.status)" />
                {{ statusLabel(step.status) }}
              </span>
            </div>
	            <div class="agent-plan-step__meta">
	              <span>{{ step.capability_key || '未指定能力' }}</span>
	              <span v-if="step.executor_run_id">run #{{ step.executor_run_id }}</span>
	              <span v-if="step.max_retries">retry {{ step.retry_count }}/{{ step.max_retries }}</span>
	              <span v-if="step.last_retry_at">上次重试 {{ formatTime(step.last_retry_at) }}</span>
	              <span v-if="step.completed_at">{{ formatTime(step.completed_at) }}</span>
	            </div>
	            <p v-if="step.input_summary">{{ step.input_summary }}</p>
	            <p v-if="step.output_summary">{{ step.output_summary }}</p>
	            <p v-if="step.error_message" class="agent-plan-step__error">{{ step.error_message }}</p>
	            <p v-if="retryPreviousError(step)" class="agent-plan-step__error">上次失败：{{ retryPreviousError(step) }}</p>
	            <p v-if="step.retry_reason">重试原因：{{ step.retry_reason }}</p>
	            <p v-if="isRetryExhausted(step)" class="agent-plan-step__error">重试次数已用尽</p>
	            <p v-if="step.artifact_refs?.length">证据引用：{{ joined(step.artifact_refs) }}</p>
	            <button
	              v-if="isRetryableStep(step)"
	              class="settings-action-button"
	              type="button"
	              :disabled="retryingStepID === step.id"
	              @click="retryStep(step)"
	            >
	              <IconRefresh />
	              {{ retryingStepID === step.id ? '重试中' : '重试步骤' }}
	            </button>
	          </div>
	        </article>
      </div>
      <div v-else class="agent-plan-empty">暂无步骤</div>
    </section>

    <section v-if="scheduledTasks.length" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">定时任务</div>
          <div class="settings-panel__meta">{{ scheduledTasks.length }} 个任务</div>
        </div>
      </div>

      <div class="agent-run-list">
        <article v-for="task in scheduledTasks" :key="task.id" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(task.status)}`">
            <IconClockCircle />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ task.task_type || 'agent_task' }} #{{ task.id }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(task.status)}`">
                {{ statusLabel(task.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>{{ task.target_channel || '无投递通道' }}</span>
              <span>计划 {{ formatTime(task.scheduled_at) }}</span>
              <span>{{ task.attempt_count }}/{{ task.max_attempts }} 次</span>
            </div>
            <p>{{ task.goal || '无目标' }}</p>
            <p v-if="task.last_error" class="agent-plan-step__error">{{ task.last_error }}</p>
            <button
              v-if="isRecoverableScheduledTask(task.status)"
              class="settings-action-button"
              type="button"
              :disabled="recoveringTaskID === task.id"
              @click="recoverScheduledTask(task.id)"
            >
              <IconPlayCircle />
              {{ recoveringTaskID === task.id ? '恢复中' : '恢复任务' }}
            </button>
            <button
              v-if="isCancelableScheduledTask(task.status)"
              class="settings-action-button"
              type="button"
              :disabled="cancelingTaskID === task.id"
              @click="cancelScheduledTask(task.id)"
            >
              <IconCloseCircleFill />
              {{ cancelingTaskID === task.id ? '取消中' : '取消任务' }}
            </button>
          </div>
        </article>
      </div>
    </section>

    <section v-if="plan" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">调度记录</div>
          <div class="settings-panel__meta">
            controller {{ controllerRun ? 1 : 0 }} / executor {{ executorRuns.length }}
          </div>
        </div>
      </div>

      <div v-if="orderedRuns.length" class="agent-run-list">
        <article v-for="run in orderedRuns" :key="run.id" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(run.status)}`">
            <IconThunderbolt v-if="run.role === 'executor'" />
            <IconInfoCircle v-else />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ runRoleLabel(run.role) }} #{{ run.id }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(run.status)}`">
                <component :is="statusIcon(run.status)" />
                {{ statusLabel(run.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>{{ run.model_key || '无模型' }}</span>
              <span>{{ joined(run.capability_scope) }}</span>
              <span>{{ formatTime(run.completed_at || run.started_at) }}</span>
            </div>
            <p>{{ runSummary(run) }}</p>
            <div v-if="run.observations?.length" class="agent-plan-step__meta">
              <span v-for="observation in run.observations" :key="observation.id">
                {{ observation.capability_key || 'observation' }} #{{ observation.id }} / {{ statusLabel(observation.status) }}
              </span>
            </div>
            <p v-for="observation in run.observations || []" :key="`observation-${observation.id}`">
              证据引用：{{ joined(observation.artifact_refs) }}
            </p>
            <div v-if="run.artifacts?.length" class="agent-plan-step__meta">
              <span v-for="artifact in run.artifacts" :key="artifact.id">
                {{ artifact.artifact_type || 'artifact' }} #{{ artifact.id }}
              </span>
            </div>
            <p v-for="artifact in run.artifacts || []" :key="`artifact-${artifact.id}`">
              产物引用：{{ artifact.content_ref || '无' }}；来源：{{ joined(artifact.source_refs) }}
            </p>
          </div>
        </article>
      </div>
      <div v-else class="agent-plan-empty">暂无调度记录</div>
    </section>

    <section v-if="plan?.approvals?.length" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">确认记录</div>
          <div class="settings-panel__meta">{{ plan.approvals.length }} 条记录</div>
        </div>
      </div>

      <div class="agent-run-list">
        <article v-for="approval in plan.approvals" :key="approval.id" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(approval.status)}`">
            <component :is="statusIcon(approval.status)" />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ approval.channel || 'web' }} #{{ approval.id }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(approval.status)}`">
                {{ statusLabel(approval.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>过期 {{ formatTime(approval.expires_at) }}</span>
              <span v-if="approval.decided_at">决策 {{ formatTime(approval.decided_at) }}</span>
            </div>
            <div v-if="approval.status === 'pending'" class="approval-actions">
              <button
                class="settings-action-button"
                type="button"
                :disabled="decidingApprovalID === approval.id"
                @click="decideApproval(approval.id, 'reject')"
              >
                <IconCloseCircleFill />
                {{ decidingApprovalID === approval.id ? '处理中' : '拒绝' }}
              </button>
              <button
                class="settings-action-button"
                type="button"
                :disabled="decidingApprovalID === approval.id"
                @click="decideApproval(approval.id, 'approve')"
              >
                <IconCheckCircleFill />
                {{ decidingApprovalID === approval.id ? '处理中' : '批准' }}
              </button>
            </div>
          </div>
        </article>
      </div>
    </section>

    <section v-if="recentEvents.length" class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">最近事件</div>
          <div class="settings-panel__meta">{{ recentEvents.length }} 条事件</div>
        </div>
      </div>

      <div class="agent-run-list">
        <article v-for="event in recentEvents" :key="event.id" class="agent-run-row">
          <div class="agent-run-row__icon" :class="`agent-run-row__icon--${statusTone(event.status)}`">
            <component :is="statusIcon(event.status)" />
          </div>
          <div class="agent-run-row__body">
            <div class="agent-run-row__top">
              <strong>{{ eventKindLabel(event.kind) }} / {{ event.title || event.id }}</strong>
              <span class="agent-plan-status" :class="`agent-plan-status--${statusTone(event.status)}`">
                {{ statusLabel(event.status) }}
              </span>
            </div>
            <div class="agent-run-row__meta">
              <span>{{ event.id }}</span>
              <span v-if="event.source">{{ eventSourceLabel(event.source) }}</span>
              <span v-if="event.ref">{{ event.ref }}</span>
              <span v-if="event.created_at">{{ formatTime(event.created_at) }}</span>
            </div>
            <p>{{ event.summary || '无' }}</p>
          </div>
        </article>
      </div>
    </section>
  </section>
</template>
