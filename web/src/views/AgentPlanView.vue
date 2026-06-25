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
import { useRoute } from 'vue-router'

import {
  getAgentPlan,
  listAgentRunsByTurn,
  type AgentPlan,
  type AgentPlanStep,
  type AgentRun,
} from '@/api/agent'
import { formatAPIError } from '@/api/client'

const route = useRoute()
const plan = ref<AgentPlan | null>(null)
const runs = ref<AgentRun[]>([])
const loading = ref(false)
const refreshing = ref(false)
const errorMessage = ref('')
const lastLoadedAt = ref('')
let requestSeq = 0
let pollTimer: number | undefined

const planID = computed(() => {
  const value = route.params.id
  const id = typeof value === 'string' ? Number(value) : 0
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

const isTerminalPlan = computed(() => {
  const status = plan.value?.status
  return status === 'completed' || status === 'failed' || status === 'rejected' || status === 'expired'
})

const statusMeta = computed(() => {
  if (!plan.value) {
    return '尚未加载'
  }
  const total = sortedSteps.value.length
  if (!total) {
    return statusLabel(plan.value.status)
  }
  return `${completedStepCount.value}/${total} 步完成`
})

async function loadPlan(options: { silent?: boolean } = {}) {
  if (planID.value < 1) {
    clearPolling()
    errorMessage.value = '缺少有效计划 ID'
    return
  }
  const token = ++requestSeq
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  errorMessage.value = ''
  try {
    const nextPlan = await getAgentPlan(planID.value)
    const nextRuns = nextPlan.turn_id > 0 ? await listAgentRunsByTurn(nextPlan.turn_id) : []
    if (token !== requestSeq) {
      return
    }
    plan.value = nextPlan
    runs.value = nextRuns
    lastLoadedAt.value = new Date().toLocaleString('zh-CN', { hour12: false })
    syncPolling()
  } catch (error) {
    if (token === requestSeq) {
      errorMessage.value = formatAPIError(error)
    }
  } finally {
    if (token === requestSeq) {
      loading.value = false
      refreshing.value = false
    }
  }
}

function syncPolling() {
  clearPolling()
  if (!isTerminalPlan.value) {
    pollTimer = window.setInterval(() => {
      void loadPlan({ silent: true })
    }, 5000)
  }
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
    running: '运行中',
    succeeded: '成功',
    input_required: '需要输入',
    canceled: '已取消',
    pending: '待处理',
  }
  return labels[status] || status || '未知'
}

function statusTone(status: string) {
  if (status === 'completed' || status === 'succeeded' || status === 'approved') {
    return 'ok'
  }
  if (status === 'failed' || status === 'rejected' || status === 'expired' || status === 'canceled') {
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

watch(planID, () => {
  clearPolling()
  plan.value = null
  runs.value = []
  void loadPlan()
})

onMounted(() => {
  void loadPlan()
})

onBeforeUnmount(() => {
  requestSeq++
  clearPolling()
})
</script>

<template>
  <section class="settings-page agent-plan-page">
    <section class="settings-panel settings-panel--wide">
      <div class="settings-panel__header agent-plan-page__header">
        <div>
          <div class="settings-panel__title">Agent 执行进度</div>
          <div class="settings-panel__meta">
            {{ plan ? `计划 #${plan.id} / ${statusMeta}` : '加载计划数据' }}
          </div>
        </div>
        <button class="settings-action-button agent-plan-page__refresh" type="button" :disabled="loading" @click="loadPlan()">
          <IconRefresh :class="{ 'agent-plan-page__spin': refreshing }" />
          {{ refreshing ? '刷新中' : '刷新' }}
        </button>
      </div>

      <div v-if="errorMessage" class="settings-inline-alert settings-inline-alert--warning">
        {{ errorMessage }}
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
            <dt>授权范围</dt>
            <dd>{{ joined(plan.allowed_scopes) }}</dd>
          </div>
          <div>
            <dt>风控策略</dt>
            <dd>{{ plan.policy_decision || '无' }} / {{ plan.risk_level || 'unknown' }}</dd>
          </div>
          <div v-if="plan.error_message">
            <dt>错误</dt>
            <dd>{{ plan.error_message }}</dd>
          </div>
        </dl>
      </template>
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
              <span v-if="step.completed_at">{{ formatTime(step.completed_at) }}</span>
            </div>
            <p v-if="step.input_summary">{{ step.input_summary }}</p>
            <p v-if="step.output_summary">{{ step.output_summary }}</p>
            <p v-if="step.error_message" class="agent-plan-step__error">{{ step.error_message }}</p>
          </div>
        </article>
      </div>
      <div v-else class="agent-plan-empty">暂无步骤</div>
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
          </div>
        </article>
      </div>
    </section>
  </section>
</template>
