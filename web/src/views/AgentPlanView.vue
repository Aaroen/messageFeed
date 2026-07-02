<script setup lang="ts">
import {
  IconArrowLeft,
  IconCheckCircleFill,
  IconClockCircle,
  IconCloseCircleFill,
  IconExclamationCircleFill,
  IconPlayCircle,
  IconRefresh,
  IconThunderbolt,
} from '@arco-design/web-vue/es/icon'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  agentProgressStreamURL,
  getAgentProgress,
  getAgentTraceBundle,
  listAgentTasks,
  retryAgentPlan,
  retryAgentPlanStep,
  stopAgentPlan,
  type AgentPlan,
  type AgentPlanStep,
  type AgentProgressSnapshot,
  type AgentRecallTrace,
  type AgentRun,
  type AgentTaskSummary,
  type AgentTraceBundle,
  type AgentTraceEvent,
} from '@/api/agent'
import { formatAPIError } from '@/api/client'
import { isTerminalAgentProgressStatus, resolveAgentProgressPollingInterval } from '@/utils/agentProgress'

type Tone = 'ok' | 'bad' | 'active' | 'warn' | 'neutral'

interface DisplayMetric {
  label: string
  value: string
}

interface SubAgentDetail {
  key: string
  title: string
  status: string
  capability: string
  prompt: string
  summary: string
  expectedOutput: string
  duration: string
  tokens: string
  model: string
  retry: string
  tone: Tone
  canRetry: boolean
  step: AgentPlanStep
}

const route = useRoute()
const router = useRouter()

const progress = ref<AgentProgressSnapshot | null>(null)
const plan = ref<AgentPlan | null>(null)
const runs = ref<AgentRun[]>([])
const traceBundle = ref<AgentTraceBundle | null>(null)
const latestTask = ref<AgentTaskSummary | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const traceLoading = ref(false)
const errorMessage = ref('')
const controlError = ref('')
const refreshNotice = ref('')
const streamStatus = ref<'idle' | 'connecting' | 'connected' | 'fallback' | 'closed'>('idle')
const retryingPlan = ref(false)
const retryingStepID = ref(0)
const stoppingPlan = ref(false)

let requestSeq = 0
let pollTimer: number | undefined
let progressStream: EventSource | undefined
let progressStreamKey = ''

const planID = computed(() => {
  return positiveRouteNumber(route.params.id)
})

const scheduledTaskID = computed(() => {
  return (
    positiveQueryNumber(route.query.scheduled_task_id) ||
    positiveQueryNumber(route.query.task_id) ||
    positiveQueryNumber(route.query.scheduledTaskID)
  )
})

const effectivePlanID = computed(() => planID.value || latestTask.value?.plan_id || 0)
const effectiveScheduledTaskID = computed(() => scheduledTaskID.value || latestTask.value?.scheduled_task_id || 0)

const sortedSteps = computed(() =>
  [...(plan.value?.steps || [])].sort((left, right) => left.step_order - right.step_order || left.id - right.id),
)

const completedStepCount = computed(() => sortedSteps.value.filter((step) => step.status === 'completed').length)
const failedStepCount = computed(() => sortedSteps.value.filter((step) => step.status === 'failed').length)
const activeStepCount = computed(() => sortedSteps.value.filter((step) => step.status === 'executing').length)

const progressPercent = computed(() => {
  if (!sortedSteps.value.length) {
    return plan.value && isTerminalAgentProgressStatus(plan.value.status) ? 100 : 0
  }
  return Math.round((completedStepCount.value / sortedSteps.value.length) * 100)
})

const traceEvents = computed(() => traceBundle.value?.events || [])
const recallTraces = computed(() => traceBundle.value?.recall_traces || [])
const orderedRuns = computed(() => [...runs.value].sort((left, right) => left.id - right.id))
const executorRuns = computed(() => orderedRuns.value.filter((run) => run.role === 'executor'))

const mainPlan = computed(() => recordValue(plan.value?.metadata?.main_agent_plan))
const taskRoute = computed(() => recordValue(plan.value?.metadata?.task_route))
const subtasks = computed(() => recordList(mainPlan.value?.subtasks))

const mainDecisionMetrics = computed<DisplayMetric[]>(() => {
  const routeType = textValue(taskRoute.value?.task_type)
  const planType = textValue(mainPlan.value?.task_type)
  const complexity = textValue(mainPlan.value?.complexity)
  const reason = textValue(taskRoute.value?.reason)
  return [
    { label: '任务类型', value: taskTypeLabel(routeType || planType) },
    { label: '复杂度', value: complexityLabel(complexity) },
    { label: '需要历史召回', value: booleanLabel(boolValue(taskRoute.value?.needs_history_recall ?? mainPlan.value?.needs_history_recall)) },
    { label: '需要子 Agent', value: booleanLabel(boolValue(taskRoute.value?.requires_sub_agent ?? mainPlan.value?.requires_sub_agent)) },
    { label: '轻量计划', value: booleanLabel(boolValue(taskRoute.value?.light_plan)) },
    { label: '判断依据', value: reason || textValue(mainPlan.value?.intent) || plan.value?.summary || '暂无' },
  ]
})

const headerMetrics = computed<DisplayMetric[]>(() => [
  { label: '当前状态', value: statusLabel(progress.value?.status || plan.value?.status || '') },
  { label: '完成进度', value: `${progressPercent.value}%` },
  { label: '步骤', value: `${completedStepCount.value}/${sortedSteps.value.length || 0}` },
  { label: '进行中', value: String(activeStepCount.value) },
  { label: '失败', value: String(failedStepCount.value) },
  { label: '实时状态', value: streamStatusLabel(streamStatus.value) },
])

const latencyMetrics = computed<DisplayMetric[]>(() => [
  { label: '总耗时', value: durationBetween(plan.value?.created_at, plan.value?.completed_at || plan.value?.failed_at || plan.value?.updated_at) },
  { label: '慢段 P95 线索', value: slowestTraceEvent.value ? traceEventLabel(slowestTraceEvent.value) : '暂无' },
  { label: 'LLM/工具事件', value: String(traceEvents.value.length) },
  { label: '召回记录', value: String(recallTraces.value.length) },
])

const slowestTraceEvent = computed(() => {
  return [...traceEvents.value].sort((left, right) => (right.duration_ms || 0) - (left.duration_ms || 0))[0] || null
})

const recallCards = computed(() =>
  recallTraces.value.map((trace) => ({
    id: trace.id,
    mode: recallModeLabel(trace.mode),
    status: statusLabel(trace.status),
    query: trace.query_text || textValue(trace.history_query_plan?.query) || '自动召回',
    total: ms(trace.total_ms),
    fulltext: `${trace.fulltext_count} 条 / ${ms(trace.fulltext_ms)}`,
    vector: `${trace.vector_candidate_count} 条 / ${ms(trace.vector_ms)}`,
    final: `${trace.final_hit_count} 条`,
    embedding: trace.embedding_attempted ? `${trace.embedding_model || 'embedding'} / ${ms(trace.embedding_ms)}` : '未使用',
  })),
)

const subAgentDetails = computed<SubAgentDetail[]>(() => {
  return sortedSteps.value.map((step, index) => {
    const subtask = matchSubtask(step, index)
    const run = executorRuns.value.find((candidate) => candidate.id === step.executor_run_id)
    const tokens = runTokenEstimate(run)
    return {
      key: String(step.id),
      title: step.title || textValue(subtask?.title) || `步骤 ${index + 1}`,
      status: statusLabel(step.status),
      capability: step.capability_key || stringList(subtask?.capability_keys).join('、') || '直接处理',
      prompt: textValue(subtask?.prompt) || step.input_summary || '主 Agent 未下发独立提示词。',
      summary: step.output_summary || step.error_message || step.input_summary || textValue(subtask?.context_summary) || '等待执行结果。',
      expectedOutput: step.expected_output || textValue(subtask?.expected_output) || '返回可供最终回复使用的结果。',
      duration: durationBetween(step.started_at || run?.started_at, step.completed_at || run?.completed_at),
      tokens: tokens > 0 ? String(tokens) : '暂无',
      model: run?.model_key || '暂无',
      retry: `${step.retry_count || 0}/${step.max_retries || 0}`,
      tone: statusTone(step.status),
      canRetry: isRetryableStep(step),
      step,
    }
  })
})

const canStopPlan = computed(() => Boolean(plan.value && !isTerminalAgentProgressStatus(plan.value.status)))
const canRetryPlan = computed(() => Boolean(plan.value && plan.value.status === 'failed' && subAgentDetails.value.some((item) => item.canRetry)))

const pageTitle = computed(() => {
  if (plan.value?.goal) {
    return plan.value.goal
  }
  if (progress.value?.summary) {
    return progress.value.summary
  }
  return planID.value > 0 ? `任务 #${planID.value}` : '任务进度'
})

const pageSummary = computed(() => {
  return progress.value?.next_action || plan.value?.summary || '正在读取任务进度。'
})

watch([planID, scheduledTaskID], () => {
  latestTask.value = null
  closeProgressStream()
  void loadPlan()
})

onMounted(() => {
  void loadPlan()
})

onBeforeUnmount(() => {
  clearPollTimer()
  closeProgressStream()
})

async function loadPlan(options: { silent?: boolean } = {}) {
  const seq = ++requestSeq
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  errorMessage.value = ''
  try {
    if (!effectivePlanID.value && !effectiveScheduledTaskID.value) {
      await loadLatestTask()
      if (seq !== requestSeq) {
        return
      }
    }
    const query = progressQuery()
    if (!query.plan_id && !query.scheduled_task_id) {
      errorMessage.value = '暂无可展示任务。'
      clearPollTimer()
      closeProgressStream()
      return
    }
    const nextProgress = await getAgentProgress({
      plan_id: query.plan_id,
      scheduled_task_id: query.scheduled_task_id,
    })
    if (seq !== requestSeq) {
      return
    }
    applyProgress(nextProgress)
    await loadTrace(nextProgress)
    refreshNotice.value = `更新于 ${formatTime(new Date().toISOString())}`
    scheduleNextRefresh()
    openProgressStream()
  } catch (error) {
    if (seq !== requestSeq) {
      return
    }
    errorMessage.value = formatAPIError(error)
    streamStatus.value = 'fallback'
    scheduleNextRefresh()
  } finally {
    if (seq === requestSeq) {
      loading.value = false
      refreshing.value = false
    }
  }
}

function applyProgress(nextProgress: AgentProgressSnapshot) {
  progress.value = nextProgress
  plan.value = nextProgress.plan || null
  runs.value = nextProgress.runs || []
  if (nextProgress.plan?.id && latestTask.value) {
    latestTask.value = {
      ...latestTask.value,
      plan_id: nextProgress.plan.id,
      scheduled_task_id: effectiveScheduledTaskID.value,
    }
  }
}

async function loadTrace(nextProgress: AgentProgressSnapshot) {
  const currentPlanID = nextProgress.plan?.id || effectivePlanID.value
  if (!currentPlanID) {
    traceBundle.value = null
    return
  }
  traceLoading.value = true
  try {
    traceBundle.value = await getAgentTraceBundle({ plan_id: currentPlanID, limit: 80 })
  } catch {
    traceBundle.value = null
  } finally {
    traceLoading.value = false
  }
}

function openProgressStream() {
  const url = agentProgressStreamURL(progressQuery())
  if (!url || progressStreamKey === url) {
    return
  }
  closeProgressStream()
  progressStreamKey = url
  streamStatus.value = 'connecting'
  progressStream = new EventSource(url)
  progressStream.onopen = () => {
    streamStatus.value = 'connected'
  }
  progressStream.onerror = () => {
    streamStatus.value = 'fallback'
    closeProgressStream(false)
    scheduleNextRefresh()
  }
  progressStream.onmessage = (event) => {
    const data = parseProgressEvent(event.data)
    if (!data) {
      return
    }
    applyProgress(data)
    void loadTrace(data)
    if (isTerminalAgentProgressStatus(data.status)) {
      closeProgressStream()
    }
  }
}

function parseProgressEvent(raw: string): AgentProgressSnapshot | null {
  try {
    const decoded = JSON.parse(raw) as { progress?: AgentProgressSnapshot } | AgentProgressSnapshot
    if ('progress' in decoded && decoded.progress) {
      return decoded.progress
    }
    return decoded as AgentProgressSnapshot
  } catch {
    return null
  }
}

function closeProgressStream(markClosed = true) {
  if (progressStream) {
    progressStream.close()
  }
  progressStream = undefined
  progressStreamKey = ''
  if (markClosed) {
    streamStatus.value = 'closed'
  }
}

function scheduleNextRefresh() {
  clearPollTimer()
  const status = progress.value?.status || plan.value?.status || ''
  if (isTerminalAgentProgressStatus(status)) {
    return
  }
  pollTimer = window.setTimeout(() => {
    void loadPlan({ silent: true })
  }, resolveAgentProgressPollingInterval(status))
}

function clearPollTimer() {
  if (pollTimer) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

async function loadLatestTask() {
  const result = await listAgentTasks({ limit: 1 })
  latestTask.value = result.tasks.find((task) => task.plan_id > 0 || task.scheduled_task_id > 0) || null
}

function progressQuery() {
  const query: { plan_id?: number; scheduled_task_id?: number } = {}
  if (effectivePlanID.value > 0) {
    query.plan_id = effectivePlanID.value
  }
  if (effectiveScheduledTaskID.value > 0) {
    query.scheduled_task_id = effectiveScheduledTaskID.value
  }
  return query
}

function positiveRouteNumber(value: unknown) {
  if (Array.isArray(value)) {
    return positiveRouteNumber(value[0])
  }
  if (typeof value !== 'string') {
    return 0
  }
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : 0
}

function positiveQueryNumber(value: unknown) {
  if (Array.isArray(value)) {
    return positiveQueryNumber(value[0])
  }
  if (typeof value !== 'string') {
    return 0
  }
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : 0
}

async function retryStep(step: AgentPlanStep) {
  if (!plan.value) {
    return
  }
  retryingStepID.value = step.id
  controlError.value = ''
  try {
    await retryAgentPlanStep(plan.value.id, step.id, { reason: 'web_user_retry' })
    await loadPlan({ silent: true })
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    retryingStepID.value = 0
  }
}

async function retryCurrentPlan() {
  if (!plan.value) {
    return
  }
  retryingPlan.value = true
  controlError.value = ''
  try {
    await retryAgentPlan(plan.value.id, { reason: 'web_user_retry' })
    await loadPlan({ silent: true })
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    retryingPlan.value = false
  }
}

async function stopCurrentPlan() {
  if (!plan.value) {
    return
  }
  stoppingPlan.value = true
  controlError.value = ''
  try {
    await stopAgentPlan(plan.value.id, { reason: 'web_user_stop' })
    await loadPlan({ silent: true })
  } catch (error) {
    controlError.value = formatAPIError(error)
  } finally {
    stoppingPlan.value = false
  }
}

function back() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'feed' })
  }
}

function matchSubtask(step: AgentPlanStep, index: number) {
  return (
    subtasks.value.find((item) => {
      const capabilities = stringList(item.capability_keys)
      return capabilities.includes(step.capability_key) || textValue(item.title) === step.title
    }) || subtasks.value[index]
  )
}

function runTokenEstimate(run?: AgentRun) {
  if (!run) {
    return 0
  }
  const contextTokens = numberValue(run.context_budget?.used_tokens) || numberValue(run.context_budget?.estimated_tokens)
  const traceTokens = (run.context_traces || []).reduce((sum, trace) => sum + (trace.token_estimate || 0), 0)
  return Math.round(contextTokens + traceTokens)
}

function isRetryableStep(step: AgentPlanStep) {
  return step.status === 'failed' && (step.max_retries || 0) > (step.retry_count || 0)
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    completed: '已完成',
    succeeded: '已成功',
    executing: '执行中',
    running: '运行中',
    failed: '失败',
    rejected: '已拒绝',
    expired: '已过期',
    skipped: '已跳过',
    pending: '待处理',
    approved: '已批准',
    awaiting_approval: '待确认',
    degraded: '已降级',
    started: '已开始',
    queued: '排队中',
  }
  return labels[status] || status || '未知'
}

function taskTypeLabel(value: string) {
  const labels: Record<string, string> = {
    quick_answer: '快速回答',
    rag_answer: '历史召回答复',
    deep_task: '复杂任务',
  }
  return labels[value] || value || '未判断'
}

function complexityLabel(value: string) {
  const labels: Record<string, string> = {
    simple: '简单',
    standard: '标准',
    complex: '复杂',
  }
  return labels[value] || value || '未标注'
}

function recallModeLabel(value: string) {
  const labels: Record<string, string> = {
    hybrid: '混合召回',
    vector: '向量召回',
    fulltext: '全文召回',
    relation: '关系召回',
  }
  return labels[value] || value || '召回'
}

function booleanLabel(value: boolean) {
  return value ? '是' : '否'
}

function streamStatusLabel(status: string) {
  const labels: Record<string, string> = {
    idle: '未连接',
    connecting: '连接中',
    connected: '实时同步',
    fallback: '轮询同步',
    closed: '已关闭',
  }
  return labels[status] || status
}

function statusTone(status: string): Tone {
  if (['completed', 'succeeded', 'approved'].includes(status)) {
    return 'ok'
  }
  if (['failed', 'rejected', 'expired', 'error'].includes(status)) {
    return 'bad'
  }
  if (['executing', 'running', 'started'].includes(status)) {
    return 'active'
  }
  if (['awaiting_approval', 'pending', 'queued', 'degraded'].includes(status)) {
    return 'warn'
  }
  return 'neutral'
}

function statusIcon(status: string) {
  const tone = statusTone(status)
  if (tone === 'ok') return IconCheckCircleFill
  if (tone === 'bad') return IconCloseCircleFill
  if (tone === 'active') return IconPlayCircle
  if (tone === 'warn') return IconExclamationCircleFill
  return IconClockCircle
}

function traceEventLabel(event: AgentTraceEvent) {
  const label = traceNameLabel(event.event_name || event.event_kind)
  return `${label} ${ms(event.duration_ms)}`
}

function traceNameLabel(name: string) {
  const labels: Record<string, string> = {
    process_turn: '整体处理',
    main_agent_task_route: '任务分级',
    main_agent_plan_spec: '主 Agent 规划',
    final_report_delivery: '结果发送',
    periodic_progress_delivery: '进度同步',
    plan_progress_delivery: '进度同步',
    controller_output: '模型执行',
  }
  return labels[name] || name || '处理阶段'
}

function formatTime(value?: string) {
  if (!value) {
    return '暂无'
  }
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function durationBetween(start?: string, end?: string) {
  if (!start || !end) {
    return '暂无'
  }
  const duration = new Date(end).getTime() - new Date(start).getTime()
  return duration > 0 ? ms(duration) : '暂无'
}

function ms(value?: number) {
  const duration = Number(value || 0)
  if (duration <= 0) {
    return '0 ms'
  }
  if (duration < 1000) {
    return `${Math.round(duration)} ms`
  }
  return `${(duration / 1000).toFixed(duration < 10000 ? 1 : 0)} s`
}

function recordValue(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null
}

function recordList(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? value.map(recordValue).filter((item): item is Record<string, unknown> => Boolean(item)) : []
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item || '').trim()).filter(Boolean) : []
}

function textValue(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function numberValue(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function boolValue(value: unknown) {
  return value === true
}
</script>

<template>
  <main class="agent-progress-page">
    <header class="progress-header">
      <button class="icon-button" type="button" aria-label="返回" @click="back">
        <IconArrowLeft />
      </button>
      <div class="header-copy">
        <p class="eyebrow">Agent 任务进度</p>
        <h1>{{ pageTitle }}</h1>
        <p>{{ pageSummary }}</p>
      </div>
      <div class="header-actions">
        <button class="text-button" type="button" :disabled="refreshing" @click="loadPlan({ silent: true })">
          <IconRefresh />
          <span>{{ refreshing ? '刷新中' : '刷新' }}</span>
        </button>
        <button v-if="canStopPlan" class="text-button danger" type="button" :disabled="stoppingPlan" @click="stopCurrentPlan">
          <span>{{ stoppingPlan ? '停止中' : '停止任务' }}</span>
        </button>
        <button v-if="canRetryPlan" class="text-button" type="button" :disabled="retryingPlan" @click="retryCurrentPlan">
          <IconThunderbolt />
          <span>{{ retryingPlan ? '重试中' : '重试任务' }}</span>
        </button>
      </div>
    </header>

    <section v-if="loading" class="state-band">正在加载任务进度。</section>
    <section v-else-if="errorMessage" class="state-band state-band--error">{{ errorMessage }}</section>
    <section v-else class="progress-layout">
      <section class="progress-overview">
        <div class="progress-ring" :style="{ '--progress': `${progressPercent}%` }">
          <span>{{ progressPercent }}%</span>
        </div>
        <div class="overview-metrics">
          <div v-for="metric in headerMetrics" :key="metric.label" class="metric-item">
            <span>{{ metric.label }}</span>
            <strong>{{ metric.value }}</strong>
          </div>
        </div>
      </section>

      <section class="content-grid">
        <section class="panel panel--wide">
          <div class="section-heading">
            <h2>主 Agent 判断</h2>
            <span>{{ formatTime(plan?.created_at) }}</span>
          </div>
          <div class="decision-grid">
            <div v-for="metric in mainDecisionMetrics" :key="metric.label" class="decision-item">
              <span>{{ metric.label }}</span>
              <strong>{{ metric.value }}</strong>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="section-heading">
            <h2>耗时与消耗</h2>
            <span>{{ refreshNotice || (traceLoading ? 'Trace 加载中' : '') }}</span>
          </div>
          <div class="metric-list">
            <div v-for="metric in latencyMetrics" :key="metric.label">
              <span>{{ metric.label }}</span>
              <strong>{{ metric.value }}</strong>
            </div>
          </div>
        </section>
      </section>

      <section class="panel">
        <div class="section-heading">
          <h2>子 Agent 与步骤</h2>
          <span>{{ subAgentDetails.length }} 个步骤</span>
        </div>
        <div class="step-list">
          <article v-for="item in subAgentDetails" :key="item.key" class="step-row">
            <div class="step-status" :class="`tone-${item.tone}`">
              <component :is="statusIcon(item.step.status)" />
            </div>
            <div class="step-main">
              <div class="step-title-line">
                <h3>{{ item.title }}</h3>
                <span>{{ item.status }}</span>
              </div>
              <p>{{ item.summary }}</p>
              <dl class="step-facts">
                <div>
                  <dt>能力</dt>
                  <dd>{{ item.capability }}</dd>
                </div>
                <div>
                  <dt>耗时</dt>
                  <dd>{{ item.duration }}</dd>
                </div>
                <div>
                  <dt>Token</dt>
                  <dd>{{ item.tokens }}</dd>
                </div>
                <div>
                  <dt>模型</dt>
                  <dd>{{ item.model }}</dd>
                </div>
                <div>
                  <dt>重试</dt>
                  <dd>{{ item.retry }}</dd>
                </div>
              </dl>
              <details class="prompt-detail" open>
                <summary>子 Agent 完整提示词</summary>
                <p>{{ item.prompt }}</p>
              </details>
              <p class="expected-output">期望输出：{{ item.expectedOutput }}</p>
            </div>
            <button
              v-if="item.canRetry"
              class="compact-button"
              type="button"
              :disabled="retryingStepID === item.step.id"
              @click="retryStep(item.step)"
            >
              {{ retryingStepID === item.step.id ? '重试中' : '重试' }}
            </button>
          </article>
        </div>
      </section>

      <section class="content-grid" v-if="recallCards.length || traceEvents.length">
        <section class="panel">
          <div class="section-heading">
            <h2>RAG 召回</h2>
            <span>{{ recallCards.length }} 次</span>
          </div>
          <div v-if="recallCards.length" class="recall-list">
            <article v-for="trace in recallCards" :key="trace.id" class="recall-item">
              <div>
                <strong>{{ trace.mode }}</strong>
                <span>{{ trace.status }} / {{ trace.total }}</span>
              </div>
              <p>{{ trace.query }}</p>
              <dl>
                <div>
                  <dt>全文</dt>
                  <dd>{{ trace.fulltext }}</dd>
                </div>
                <div>
                  <dt>向量</dt>
                  <dd>{{ trace.vector }}</dd>
                </div>
                <div>
                  <dt>最终命中</dt>
                  <dd>{{ trace.final }}</dd>
                </div>
                <div>
                  <dt>Embedding</dt>
                  <dd>{{ trace.embedding }}</dd>
                </div>
              </dl>
            </article>
          </div>
          <p v-else class="empty-text">本任务未触发历史召回。</p>
        </section>

        <section class="panel">
          <div class="section-heading">
            <h2>关键慢段</h2>
            <span>{{ traceEvents.length }} 个事件</span>
          </div>
          <div class="trace-list">
            <div v-for="event in traceEvents.slice(0, 8)" :key="event.id" class="trace-row">
              <span>{{ traceNameLabel(event.event_name || event.event_kind) }}</span>
              <strong>{{ ms(event.duration_ms) }}</strong>
              <em>{{ statusLabel(event.status) }}</em>
            </div>
          </div>
        </section>
      </section>

      <section v-if="controlError" class="state-band state-band--error">{{ controlError }}</section>
    </section>
  </main>
</template>

<style scoped>
.agent-progress-page {
  min-height: var(--mf-viewport-height);
  padding: 28px clamp(16px, 4vw, 48px) 48px;
  color: var(--mf-text);
}

.progress-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 18px;
  align-items: start;
  max-width: 1320px;
  margin: 0 auto 22px;
}

.header-copy {
  min-width: 0;
}

.eyebrow {
  margin: 0 0 6px;
  color: var(--mf-aqua);
  font-size: 13px;
  font-weight: 700;
}

.header-copy h1 {
  margin: 0;
  font-size: clamp(24px, 3vw, 36px);
  line-height: 1.18;
  overflow-wrap: anywhere;
}

.header-copy p:last-child {
  margin: 8px 0 0;
  color: var(--mf-text-muted);
  line-height: 1.7;
}

.header-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.icon-button,
.text-button,
.compact-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--mf-border);
  background: var(--mf-surface);
  color: var(--mf-text);
  cursor: pointer;
}

.icon-button {
  width: 42px;
  height: 42px;
  border-radius: 8px;
}

.text-button {
  min-height: 40px;
  gap: 8px;
  padding: 0 14px;
  border-radius: 8px;
}

.compact-button {
  min-width: 72px;
  height: 34px;
  border-radius: 8px;
}

.text-button.danger {
  border-color: rgba(220, 38, 38, 0.35);
  color: #b91c1c;
}

.icon-button:disabled,
.text-button:disabled,
.compact-button:disabled {
  cursor: not-allowed;
  opacity: 0.56;
}

.progress-layout {
  display: grid;
  gap: 18px;
  max-width: 1320px;
  margin: 0 auto;
}

.progress-overview,
.panel,
.state-band {
  border: 1px solid var(--mf-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 18px 48px rgba(31, 45, 61, 0.08);
  backdrop-filter: blur(18px) saturate(1.1);
}

body[arco-theme='dark'] .progress-overview,
body[arco-theme='dark'] .panel,
body[arco-theme='dark'] .state-band {
  background: rgba(23, 32, 51, 0.78);
}

.progress-overview {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 22px;
  align-items: center;
  padding: 20px;
}

.progress-ring {
  --progress: 0%;
  display: grid;
  place-items: center;
  width: 132px;
  aspect-ratio: 1;
  border-radius: 50%;
  background: conic-gradient(var(--mf-primary) var(--progress), rgba(88, 103, 121, 0.16) 0);
}

.progress-ring span {
  display: grid;
  place-items: center;
  width: 94px;
  aspect-ratio: 1;
  border-radius: 50%;
  background: var(--mf-surface);
  font-size: 24px;
  font-weight: 800;
}

.overview-metrics,
.decision-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.metric-item,
.decision-item,
.metric-list > div {
  min-width: 0;
  padding: 12px;
  border: 1px solid rgba(117, 132, 153, 0.18);
  border-radius: 8px;
  background: rgba(237, 243, 247, 0.58);
}

.metric-item span,
.decision-item span,
.metric-list span,
.step-facts dt,
.recall-item dt {
  display: block;
  color: var(--mf-text-muted);
  font-size: 12px;
}

.metric-item strong,
.decision-item strong,
.metric-list strong {
  display: block;
  margin-top: 5px;
  font-size: 16px;
  overflow-wrap: anywhere;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(320px, 0.75fr);
  gap: 18px;
}

.panel {
  padding: 18px;
}

.panel--wide {
  min-width: 0;
}

.section-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.section-heading h2 {
  margin: 0;
  font-size: 18px;
}

.section-heading span {
  color: var(--mf-text-muted);
  font-size: 13px;
}

.metric-list {
  display: grid;
  gap: 10px;
}

.step-list {
  display: grid;
  gap: 14px;
}

.step-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 14px;
  padding: 16px;
  border: 1px solid rgba(117, 132, 153, 0.18);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.5);
}

.step-status {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(88, 103, 121, 0.12);
}

.step-status svg {
  width: 18px;
  height: 18px;
}

.tone-ok {
  color: #15803d;
  background: rgba(21, 128, 61, 0.12);
}

.tone-bad {
  color: #b91c1c;
  background: rgba(185, 28, 28, 0.12);
}

.tone-active {
  color: var(--mf-primary);
  background: rgba(37, 99, 235, 0.12);
}

.tone-warn {
  color: var(--mf-amber);
  background: rgba(183, 121, 31, 0.14);
}

.step-main {
  min-width: 0;
}

.step-title-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.step-title-line h3 {
  margin: 0;
  font-size: 16px;
  overflow-wrap: anywhere;
}

.step-title-line span {
  flex: none;
  color: var(--mf-text-muted);
  font-size: 13px;
}

.step-main p {
  margin: 8px 0 0;
  color: var(--mf-text-muted);
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.step-facts,
.recall-item dl {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  margin: 12px 0 0;
}

.step-facts dd,
.recall-item dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}

.prompt-detail {
  margin-top: 12px;
  border: 1px solid rgba(117, 132, 153, 0.18);
  border-radius: 8px;
  background: rgba(237, 243, 247, 0.45);
}

.prompt-detail summary {
  cursor: pointer;
  padding: 10px 12px;
  font-weight: 700;
}

.prompt-detail p {
  margin: 0;
  padding: 0 12px 12px;
  color: var(--mf-text);
  white-space: pre-wrap;
}

.expected-output {
  font-size: 13px;
}

.recall-list,
.trace-list {
  display: grid;
  gap: 10px;
}

.recall-item {
  padding: 14px;
  border: 1px solid rgba(117, 132, 153, 0.18);
  border-radius: 8px;
}

.recall-item > div:first-child {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}

.recall-item p {
  margin: 8px 0 0;
  color: var(--mf-text-muted);
  line-height: 1.7;
}

.recall-item dl {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.trace-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 10px;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid rgba(117, 132, 153, 0.16);
}

.trace-row:last-child {
  border-bottom: 0;
}

.trace-row span {
  overflow-wrap: anywhere;
}

.trace-row em {
  color: var(--mf-text-muted);
  font-style: normal;
}

.state-band {
  max-width: 1320px;
  margin: 0 auto;
  padding: 16px 18px;
  color: var(--mf-text-muted);
}

.state-band--error {
  border-color: rgba(185, 28, 28, 0.28);
  color: #b91c1c;
}

.empty-text {
  margin: 0;
  color: var(--mf-text-muted);
}

@media (max-width: 900px) {
  .agent-progress-page {
    padding: 18px 12px 32px;
  }

  .progress-header,
  .progress-overview,
  .content-grid,
  .step-row {
    grid-template-columns: 1fr;
  }

  .header-actions {
    justify-content: stretch;
  }

  .text-button {
    flex: 1;
  }

  .progress-ring {
    width: 112px;
    margin: 0 auto;
  }

  .progress-ring span {
    width: 80px;
    font-size: 20px;
  }

  .overview-metrics,
  .decision-grid,
  .step-facts,
  .recall-item dl {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .compact-button {
    width: 100%;
  }
}

@media (max-width: 560px) {
  .overview-metrics,
  .decision-grid,
  .step-facts,
  .recall-item dl {
    grid-template-columns: 1fr;
  }

  .section-heading,
  .step-title-line,
  .recall-item > div:first-child {
    align-items: flex-start;
    flex-direction: column;
  }

  .trace-row {
    grid-template-columns: 1fr;
  }
}
</style>
