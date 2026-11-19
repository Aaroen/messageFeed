<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { IconApps, IconArrowUp, IconBranch, IconClose, IconCode, IconCopy, IconHistory, IconStop } from '@arco-design/web-vue/es/icon'
import type { AgentTurn } from '@/api/agent'
import { cancelAgentTurn, getAgentActivity, type AgentActivity } from '@/api/assistant'
import { formatAPIError } from '@/api/client'
import { getCurrentAuth } from '@/api/auth'
import AgentModelSelect from '@/components/agent/AgentModelSelect.vue'
import AgentGuestChat from '@/components/agent/AgentGuestChat.vue'
import AgentComposer from '@/components/agent/AgentComposer.vue'
import AgentMarkdown from '@/components/agent/AgentMarkdown.vue'
import AgentWorkspaceShell from '@/components/agent/AgentWorkspaceShell.vue'
import AgentSubagentList from '@/components/agent/AgentSubagentList.vue'
import AgentTurnActivity from '@/components/agent/AgentTurnActivity.vue'
import { useAssistantStore } from '@/stores/assistant'
import { useAgentScroll } from '@/composables/useAgentScroll'
import { assistantErrorMessage, flattenRuns, formatDuration, groupByTurn, isActiveStatus, liveTurnText, readableActivityText, runLabel, subagentRuns, statusLabel } from '@/utils/assistant'

const store = useAssistantStore()
const fullscreen = defineModel<boolean>('fullscreen', { default: false })
const authenticated = ref(false)
const authLoaded = ref(false)
const route = useRoute()
const router = useRouter()
const draft = ref('')
const reasoningEffort = ref('')
const drafts = new Map<number, string>()
const messageList = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof AgentComposer> | null>(null)
const shell = ref<InstanceType<typeof AgentWorkspaceShell> | null>(null)
const selectedRunID = ref(0)
const selectedTurnID = ref(0)
const runDetail = ref<AgentActivity | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const stopping = ref(false)
const loadingOlder = ref(false)
const { followBottom, handleScroll, scrollBottom } = useAgentScroll(messageList)
const detailWidth = ref(288)
const copiedTurnID = ref(0)
let detailVersion = 0
let resizeCleanup: (() => void) | null = null
let copyTimer: ReturnType<typeof setTimeout> | null = null

const turns = computed(() => store.workspace?.turns || [])
const latestTurn = computed(() => turns.value[turns.value.length - 1])
const runsByTurn = computed(() => groupByTurn(store.workspace?.runs || []))
const itemsByTurn = computed(() => groupByTurn(store.workspace?.items || []))
const detailRuns = computed(() => subagentRuns(turnRuns(selectedTurnID.value || latestTurn.value?.id || 0)))
const detailEntries = computed(() => flattenRuns(detailRuns.value).map(({ run, depth }) => ({ id: run.id, title: runLabel(run), status: statusLabel(run.status), complete: run.status === 'succeeded' || run.status === 'completed', depth })))
const detailHeading = computed(() => {
  if (!selectedTurnID.value || selectedTurnID.value === latestTurn.value?.id) return '最新一轮'
  if (store.workspace?.has_more) return '所选轮次'
  return `第 ${turns.value.findIndex((turn) => turn.id === selectedTurnID.value) + 1} 轮`
})
const selectedRun = computed(() => runDetail.value ?? detailRuns.value.find((run) => run.id === selectedRunID.value))
const detailResults = computed(() => (runDetail.value?.results || []).map(readableActivityText).filter(Boolean))
const sessions = computed(() => store.sessions.map((session) => ({ id: session.id, title: session.title || '未命名对话' })))
const title = computed(() => store.workspace?.session.title || '新建对话')
const pendingVisible = computed(() => store.pendingMessage && !turns.value.some((turn) => turn.input_text === store.pendingMessage && isActiveStatus(turn.status)))

function turnRuns(turnID: number) { return runsByTurn.value.get(turnID) || [] }
function turnText(turn: AgentTurn) {
  if (turn.output_text.trim()) return turn.output_text
  return isActiveStatus(turn.status) ? liveTurnText(turnItems(turn.id), turn.id) : ''
}
function missingAnswer(turn: AgentTurn) { return (turn.status === 'succeeded' || turn.status === 'completed') && !turn.output_text.trim() }
function modelName(turn: AgentTurn) {
  return store.models.find((entry) => entry.config_id === turn.model_config_id && entry.model === turn.model)?.display_name || turn.model
}
function turnState(turn: AgentTurn) { return turn.cancel_requested && isActiveStatus(turn.status) ? '正在停止' : turn.error_message === '用户停止执行' ? '已停止' : statusLabel(turn.status) }
function turnItems(turnID: number) { return itemsByTurn.value.get(turnID) || [] }

async function selectSession(id: number) {
  if (store.sending) return
  await router.replace({ name: 'agent', query: id ? { session: id } : { session: '0' } })
}

async function loadSession(id: number) {
  if (id === store.sessionID) return
  drafts.set(store.sessionID, draft.value)
  draft.value = drafts.get(id) || ''
  selectedRunID.value = 0
  selectedTurnID.value = 0
  runDetail.value = null
  detailError.value = ''
  detailLoading.value = false
  ++detailVersion
  followBottom.value = true
  await store.selectSession(id)
  if (store.sessionID === id) scrollBottom()
}

async function loadDetail(id: number) {
  const version = ++detailVersion
  selectedRunID.value = id
  runDetail.value = null
  detailError.value = ''
  detailLoading.value = true
  try {
    const detail = await getAgentActivity(id)
    if (version === detailVersion) runDetail.value = detail
  } catch (cause) {
    if (version === detailVersion) detailError.value = formatAPIError(cause)
  } finally {
    if (version === detailVersion) detailLoading.value = false
  }
}

function showDetails(turnID = 0, openDrawer = true) {
  if (selectedTurnID.value !== turnID) {
    selectedTurnID.value = turnID
    selectedRunID.value = 0
    runDetail.value = null
    detailLoading.value = false
    detailError.value = ''
    ++detailVersion
  }
  if (openDrawer) shell.value?.openDetails()
}

function toggleRun(id: number) {
  if (selectedRunID.value !== id) void loadDetail(id)
  else {
    selectedRunID.value = 0
    runDetail.value = null
    ++detailVersion
  }
}

async function sendMessage() {
  const message = draft.value.trim()
  if (!message || store.sending || store.active) return
  const startSession = store.sessionID
  const startRouteSession = route.query.session
  draft.value = ''
  drafts.delete(store.sessionID)
  selectedRunID.value = 0
  selectedTurnID.value = 0
  runDetail.value = null
  ++detailVersion
  followBottom.value = true
  const succeeded = await store.send(message, reasoningEffort.value)
  if (route.name !== 'agent' || route.query.session !== startRouteSession) {
    if (!succeeded) drafts.set(startSession, message)
    return
  }
  if (!succeeded) draft.value = message
  if (store.sessionID) {
    await router.replace({ name: 'agent', query: { session: store.sessionID } })
  }
  await nextTick()
  scrollBottom()
  composer.value?.focus()
}

async function loadOlder() {
  const element = messageList.value
  const sessionID = store.sessionID
  const previousHeight = element?.scrollHeight || 0
  loadingOlder.value = true
  followBottom.value = false
  try {
    await store.loadOlder()
    await nextTick()
    if (element && sessionID === store.sessionID) element.scrollTop += element.scrollHeight - previousHeight
  } catch (cause) { store.error = formatAPIError(cause) }
  finally { loadingOlder.value = false }
}

async function stopTurn() {
  const turn = [...turns.value].reverse().find((entry) => isActiveStatus(entry.status))
  if (!turn) return
  stopping.value = true
  try { await cancelAgentTurn(turn.id) }
  catch (cause) { store.error = formatAPIError(cause) }
  finally { stopping.value = false }
}

async function copyReply(turn: AgentTurn) {
  try {
    await navigator.clipboard.writeText(turnText(turn))
    copiedTurnID.value = turn.id
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copiedTurnID.value = 0 }, 1800)
  } catch (cause) { store.error = formatAPIError(cause) }
}

function beginResize(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  resizeCleanup?.()
  const start = event.clientX
  const width = detailWidth.value
  let frame = 0
  let nextWidth = width
  const move = (value: PointerEvent) => {
    if (value.pointerId !== event.pointerId) return
    nextWidth = Math.max(280, Math.min(560, width + start - value.clientX))
    if (!frame) frame = requestAnimationFrame(() => { frame = 0; detailWidth.value = nextWidth })
  }
  const stop = () => {
    cancelAnimationFrame(frame)
    detailWidth.value = nextWidth
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stop)
    window.removeEventListener('pointercancel', stop)
    resizeCleanup = null
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop, { once: true })
  window.addEventListener('pointercancel', stop, { once: true })
  resizeCleanup = stop
}

watch(() => `${store.workspace?.cursor}:${latestTurn.value?.status}:${turns.value.length}:${store.pendingMessage}`, async () => { await nextTick(); scrollBottom() })
watch(() => store.sessionID, () => { selectedRunID.value = 0; selectedTurnID.value = 0; runDetail.value = null; ++detailVersion })
function currentRouteSession() {
  const id = Number(route.query.session ?? store.sessions[0]?.id ?? 0)
  return Number.isSafeInteger(id) && id >= 0 ? id : 0
}

watch(() => route.query.session, () => {
  if (authenticated.value && route.name === 'agent') void loadSession(currentRouteSession())
})
watch(() => store.workspace?.runs, (runs) => {
  const selected = runs?.find((run) => run.id === selectedRunID.value)
  if (selected && runDetail.value && runDetail.value.updated_at !== selected.updated_at) void loadDetail(selected.id)
})

onMounted(async () => {
  store.reset()
  try {
    authenticated.value = (await getCurrentAuth()).authenticated
    authLoaded.value = true
    if (!authenticated.value) return
    await store.loadModels()
    await store.loadSessions()
    await loadSession(currentRouteSession())
  } catch (cause) { store.error = formatAPIError(cause) }
})

onBeforeUnmount(() => { store.reset(); ++detailVersion; resizeCleanup?.(); if (copyTimer) clearTimeout(copyTimer) })
defineExpose({ refreshPage: () => store.sessionID ? store.selectSession(store.sessionID) : store.loadSessions() })
</script>

<template>
  <AgentGuestChat v-if="authLoaded && !authenticated" v-model:fullscreen="fullscreen" />
  <p v-else-if="!authLoaded" role="status">{{ store.error || '正在加载助理…' }}</p>
  <AgentWorkspaceShell v-else ref="shell" v-model:fullscreen="fullscreen" :title="title" :sessions="sessions" :active-i-d="store.sessionID" :session-busy="store.sending" :style="{ '--assistant-panel-width': `${detailWidth}px` }" @select-session="selectSession" @toggle-details="showDetails(0, false)">
    <template #sessions-footer><router-link :to="{ name: 'agent-plans' }"><IconHistory aria-hidden="true" />任务与执行记录</router-link></template>
    <template #details>
      <AgentSubagentList :entries="detailEntries" :selected-i-d="selectedRunID" :heading="detailHeading" @select="toggleRun">
        <template #result>
          <p v-if="selectedRun">耗时：{{ formatDuration(selectedRun.started_at, selectedRun.completed_at) }}</p>
          <p v-if="detailLoading" role="status">正在读取详情…</p>
          <p v-else-if="detailError" role="alert">{{ assistantErrorMessage(detailError) }}</p>
          <AgentMarkdown v-for="(result, index) in detailResults" :key="index" :content="result" />
        </template>
      </AgentSubagentList>
    </template>
    <template #resize-handle><div class="assistant-resizer" role="separator" aria-label="调整子代理侧栏宽度" aria-orientation="vertical" :aria-valuenow="detailWidth" :aria-valuemin="280" :aria-valuemax="560" tabindex="0" @pointerdown="beginResize" @keydown.left.prevent="detailWidth = Math.min(560, detailWidth + 20)" @keydown.right.prevent="detailWidth = Math.max(280, detailWidth - 20)" /></template>
      <div v-if="store.error" class="assistant-error" role="alert"><span>{{ assistantErrorMessage(store.error) }}</span><button class="assistant-icon-button" type="button" aria-label="关闭错误提示" @click="store.error = ''"><IconClose /></button></div>
      <div ref="messageList" class="assistant-message-list" aria-label="对话消息" @scroll.passive="handleScroll">
        <div v-if="store.loading && !turns.length" class="assistant-state-note" role="status">正在读取对话…</div>
        <div v-else-if="!turns.length && !store.pendingMessage" class="assistant-welcome"><span class="assistant-welcome-symbol"><IconCode /></span><h1>有什么需要处理？</h1><p>继续已有对话，或者开始新的任务。<br />执行过程随回答实时显示，子代理活动可在右侧查看。</p><div class="assistant-suggestions"><button type="button" @click="draft = '整理最近订阅中的重要信息'; composer?.focus()">整理最近订阅</button><button type="button" @click="draft = '查看当前任务的执行情况'; composer?.focus()">查看任务进展</button></div></div>
        <div v-if="store.workspace?.has_more" class="assistant-load-older"><button type="button" :disabled="loadingOlder" @click="loadOlder">{{ loadingOlder ? '正在读取…' : '读取更早的对话' }}</button></div>
        <div class="assistant-message-column">
          <article v-for="turn in turns" :key="turn.id" class="assistant-turn">
            <div class="assistant-user-message"><p>{{ turn.input_text }}</p></div>
            <div class="assistant-answer"><div class="assistant-answer-heading"><span class="assistant-avatar"><IconApps aria-hidden="true" /></span><strong>助理</strong><small v-if="turn.model">{{ modelName(turn) }}</small></div>
              <AgentTurnActivity v-if="turnRuns(turn.id).length || turnItems(turn.id).some((item) => item.kind === 'reasoning')" :runs="turnRuns(turn.id)" :items="turnItems(turn.id)" :status="turn.status" :replying="Boolean(turnText(turn))" />
              <AgentMarkdown v-if="turnText(turn)" :content="turnText(turn)" />
              <p v-else-if="missingAnswer(turn)" class="assistant-turn-error" role="alert">没有收到回答正文，请重试。</p>
              <div v-else-if="isActiveStatus(turn.status)" class="assistant-state-note" role="status"><span class="assistant-thinking-dot" />{{ turnState(turn) }}…</div>
              <p v-if="turn.error_message" class="assistant-turn-error" role="alert">{{ assistantErrorMessage(turn.error_message) }}</p>
              <p v-if="turn.delivery_status === 'failed'" class="assistant-delivery-error" role="alert">回复已保存，渠道发送失败。{{ turn.delivery_error }}</p>
              <p v-else-if="turn.delivery_status === 'pending'" class="assistant-state-note">回复已保存，等待渠道发送。</p>
              <div class="assistant-answer-footer"><span v-if="!missingAnswer(turn)">{{ isActiveStatus(turn.status) && turnText(turn) ? '回答生成中' : turnState(turn) }}</span><button v-if="turn.output_text?.trim()" class="assistant-icon-button" type="button" :aria-label="copiedTurnID === turn.id ? '已经复制' : '复制回答'" @click="copyReply(turn)"><IconCopy /><span v-if="copiedTurnID === turn.id">已复制</span></button><button v-if="subagentRuns(turnRuns(turn.id)).length" type="button" class="assistant-detail-toggle" @click="showDetails(turn.id)"><IconBranch aria-hidden="true" />查看本次子代理</button></div>
            </div>
          </article>
          <div v-if="pendingVisible" class="assistant-user-message assistant-pending-message"><p>{{ store.pendingMessage }}</p><small>正在提交…</small></div>
        </div>
      </div>
      <div class="assistant-composer-area">
        <button v-if="!followBottom" class="assistant-jump-button" type="button" @click="scrollBottom(true)">查看最新消息</button>
        <AgentComposer ref="composer" v-model="draft" class="assistant-composer" label="发送给助理的消息" :disabled="store.sending" @submit="sendMessage">
          <AgentModelSelect v-model="store.selectedModelKey" v-model:effort="reasoningEffort" :models="store.models" :disabled="store.sending || store.active" />
          <button v-if="store.active" type="button" class="assistant-send-button stop" :disabled="stopping || latestTurn?.cancel_requested" aria-label="停止当前任务" @click="stopTurn"><IconStop /></button>
          <button v-else type="submit" class="assistant-send-button" :disabled="!draft.trim() || store.sending || !store.selectedModel" aria-label="发送消息"><IconArrowUp /></button>
        </AgentComposer>
        <p v-if="!store.models.length" class="assistant-composer-hint"><router-link :to="{ name: 'settings-model' }">前往设置启用模型</router-link></p>
      </div>
  </AgentWorkspaceShell>
</template>

<style scoped>
.assistant-icon-button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; border: 0; background: transparent; padding: 7px; border-radius: 6px; color: var(--mf-text-muted); cursor: pointer; font-size: 15px; }
.assistant-icon-button:hover { background: var(--mf-surface-muted, #eef0f3); color: var(--mf-text); }
.assistant-detail-toggle { display: inline-flex; align-items: center; gap: 6px; border: 0; border-radius: 6px; padding: 6px 8px; background: transparent; color: var(--mf-text-muted); font-size: 11px; cursor: pointer; white-space: nowrap; }
.assistant-message-list { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 44px 28px 16px; }
.assistant-message-column { width: min(100%, 692px); margin: 0 auto; }
.assistant-turn { margin-bottom: 30px; }
.assistant-user-message { display: flex; flex-direction: column; align-items: flex-end; margin-bottom: 26px; }
.assistant-user-message p { max-width: 88%; margin: 0; padding: 10px 14px; border-radius: 8px; background: var(--mf-surface-muted); font-size: 15px; line-height: 1.65; white-space: pre-wrap; overflow-wrap: anywhere; }
.assistant-user-message small { margin-top: 6px; color: var(--mf-text-muted); }
.assistant-answer-heading { display: flex; align-items: center; gap: 8px; font-size: 12px; margin-bottom: 17px; font-weight: 550; }
.assistant-answer-heading small { font-size: 10px; font-weight: 400; color: var(--mf-text-muted); }
.assistant-avatar { display: grid; place-items: center; width: 26px; height: 26px; background: var(--mf-tab-active-bg); border-radius: 7px; font-size: 13px; }
.assistant-answer-footer { display: flex; align-items: center; gap: 12px; color: var(--mf-text-muted); font-size: 10px; margin-top: 15px; }
.assistant-answer-footer .assistant-icon-button { font-size: 12px; }
.assistant-answer-footer .assistant-icon-button span { font-size: 10px; }
.assistant-state-note { color: var(--mf-text-muted); font-size: 12px; line-height: 1.8; }
.assistant-thinking-dot { display: inline-block; width: 7px; height: 7px; background: #9198a1; border-radius: 50%; margin-right: 8px; animation: assistant-pulse 1.6s ease-in-out infinite; }
.assistant-turn-error, .assistant-delivery-error { border-left: 2px solid #bd7664; background: #bd766409; padding: 10px 12px; color: #a25442; font-size: 12px; line-height: 1.8; overflow-wrap: anywhere; }
.assistant-error { display: flex; align-items: center; justify-content: space-between; padding: 8px 16px; color: #a25442; background: #bd766410; border-bottom: 1px solid #bd76642a; font-size: 12px; overflow-wrap: anywhere; }
.assistant-welcome { display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 90%; text-align: center; padding: 24px 0; }
.assistant-welcome-symbol { display: grid; place-items: center; width: 48px; height: 48px; border: 1px solid var(--mf-border); border-radius: 14px; font-size: 23px; margin-bottom: 20px; }
.assistant-welcome h1 { font-size: clamp(20px, 2vw, 28px); font-weight: 550; letter-spacing: -.5px; margin: 0 0 15px; }
.assistant-welcome p { font-size: 12px; color: var(--mf-text-muted); line-height: 1.9; margin: 0; }
.assistant-suggestions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; margin-top: 24px; }
.assistant-suggestions button { border: 1px solid var(--mf-border); background: transparent; color: var(--mf-text-muted); padding: 8px 12px; border-radius: 7px; font-size: 11px; cursor: pointer; }
.assistant-composer-area { position: relative; flex-shrink: 0; width: min(100%, 748px); margin: 0 auto; padding: 8px 28px max(9px, env(safe-area-inset-bottom)); }
.assistant-send-button { flex: none; display: grid; place-items: center; width: 35px; height: 35px; border: 0; border-radius: 50%; background: var(--mf-text); color: var(--mf-surface); cursor: pointer; font-size: 17px; }
.assistant-send-button:disabled { opacity: .28; cursor: default; }
.assistant-send-button.stop { background: var(--mf-surface-muted, #eef0f3); color: var(--mf-text); border: 1px solid var(--mf-border); }
.assistant-composer-hint { text-align: center; color: var(--mf-text-muted); font-size: 10px; margin: 9px 0 0; line-height: 1.6; }
.assistant-jump-button { position: absolute; top: -36px; left: 50%; transform: translateX(-50%); border: 1px solid var(--mf-border); background: var(--mf-surface, #fff); padding: 6px 12px; border-radius: 20px; font-size: 11px; color: var(--mf-text-muted); cursor: pointer; }
.assistant-load-older { text-align: center; margin-bottom: 24px; }
.assistant-load-older button { border: 0; background: transparent; color: var(--mf-text-muted); font-size: 11px; cursor: pointer; padding: 8px; }
.assistant-resizer { position: absolute; z-index: 3; inset-block: 0; inset-inline-end: calc(var(--assistant-panel-width) - 2px); width: 5px; background: transparent; cursor: col-resize; touch-action: none; }
.assistant-resizer:hover, .assistant-resizer:focus-visible { background: var(--mf-border); outline: none; }
@keyframes assistant-pulse { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .assistant-thinking-dot { animation: none; } }
@container (max-width: 630px) { .assistant-message-list { padding-inline: 17px; } .assistant-composer-area { padding-inline: 14px; } }
@media (max-width: 799px) { .assistant-message-list { padding-top: 24px; } .assistant-composer-area { padding-top: 3px; } .assistant-user-message p { max-width: 94%; } }
@media (max-width: 799px), (pointer: coarse) { .assistant-icon-button, .assistant-detail-toggle, .assistant-send-button { min-width: 44px; min-height: 44px; } }
</style>
