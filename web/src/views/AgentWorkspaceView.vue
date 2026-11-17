<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ArcoDrawer from '@arco-design/web-vue/es/drawer'
import '@arco-design/web-vue/es/drawer/style/css.js'
import { IconArrowUp, IconClose, IconCode, IconCopy, IconHistory, IconMenu, IconPlus, IconRefresh, IconStop } from '@arco-design/web-vue/es/icon'
import { getAgentRun, type AgentRun, type AgentTurn } from '@/api/agent'
import { cancelAgentTurn } from '@/api/assistant'
import { formatAPIError } from '@/api/client'
import { getCurrentAuth } from '@/api/auth'
import AgentModelSelect from '@/components/agent/AgentModelSelect.vue'
import AgentGuestChat from '@/components/agent/AgentGuestChat.vue'
import AgentMarkdown from '@/components/agent/AgentMarkdown.vue'
import AgentRunDetails from '@/components/agent/AgentRunDetails.vue'
import AgentTurnActivity from '@/components/agent/AgentTurnActivity.vue'
import { useAssistantStore } from '@/stores/assistant'
import { assistantErrorMessage, isActiveStatus, liveTurnText, subagentRuns, statusLabel } from '@/utils/assistant'

const store = useAssistantStore()
const authenticated = ref(false)
const authLoaded = ref(false)
const route = useRoute()
const router = useRouter()
const draft = ref('')
const drafts = new Map<number, string>()
const messageList = ref<HTMLElement | null>(null)
const composer = ref<HTMLTextAreaElement | null>(null)
const sessionDrawer = ref(false)
const detailDrawer = ref(false)
const selectedRunID = ref(0)
const selectedTurnID = ref(0)
const runDetail = ref<AgentRun | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const stopping = ref(false)
const loadingOlder = ref(false)
const followBottom = ref(true)
const detailWidth = ref(360)
const sessionsCollapsed = ref(localStorage.getItem('mf-assistant-sessions-collapsed') === 'true')
const detailsCollapsed = ref(localStorage.getItem('mf-assistant-details-collapsed') === 'true')
const desktopMedia = window.matchMedia('(min-width: 1200px)')
const mobileMedia = window.matchMedia('(max-width: 767px)')
const desktopViewport = ref(desktopMedia.matches)
const mobileViewport = ref(mobileMedia.matches)
const copiedTurnID = ref(0)
let detailVersion = 0
let resizeCleanup: (() => void) | null = null
let copyTimer: ReturnType<typeof setTimeout> | null = null

const turns = computed(() => store.workspace?.turns || [])
const latestTurn = computed(() => turns.value[turns.value.length - 1])
const detailRuns = computed(() => subagentRuns((store.workspace?.runs || []).filter((run) => !selectedTurnID.value || run.turn_id === selectedTurnID.value)))
const detailItems = computed(() => (store.workspace?.items || []).filter((item) => !selectedTurnID.value || item.turn_id === selectedTurnID.value))
const connectionLabel = computed(() => ({ idle: '尚未连接', connecting: '正在连接', connected: '实时连接', reconnecting: '正在重连', failed: '连接失败' })[store.connection])
const title = computed(() => store.workspace?.session.title || '新对话')
const pendingVisible = computed(() => store.pendingMessage && !turns.value.some((turn) => turn.input_text === store.pendingMessage && isActiveStatus(turn.status)))
const sessionToggleLabel = computed(() => mobileViewport.value ? '打开会话列表' : sessionsCollapsed.value ? '展开会话侧栏' : '收起会话侧栏')
const detailToggleLabel = computed(() => desktopViewport.value ? detailsCollapsed.value ? '展开子代理侧栏' : '收起子代理侧栏' : '查看子代理')

function updateViewport() {
  desktopViewport.value = desktopMedia.matches
  mobileViewport.value = mobileMedia.matches
  if (desktopViewport.value) detailDrawer.value = false
  if (!mobileViewport.value) sessionDrawer.value = false
}

function toggleSessions() {
  if (mobileViewport.value) sessionDrawer.value = !sessionDrawer.value
  else sessionsCollapsed.value = !sessionsCollapsed.value
}

function toggleDetails() {
  if (desktopViewport.value) detailsCollapsed.value = !detailsCollapsed.value
  else detailDrawer.value = !detailDrawer.value
  if (!detailsCollapsed.value || detailDrawer.value) showDetails(selectedTurnID.value || latestTurn.value?.id || 0, false)
}

function turnRuns(turnID: number) { return (store.workspace?.runs || []).filter((run) => run.turn_id === turnID) }
function turnText(turn: AgentTurn) { return turn.output_text || liveTurnText(store.workspace?.items || [], turn.id) }
function turnState(turn: AgentTurn) { return turn.cancel_requested && isActiveStatus(turn.status) ? '正在停止' : turn.error_message === '用户停止执行' ? '已停止' : statusLabel(turn.status) }
function turnItems(turnID: number) { return (store.workspace?.items || []).filter((item) => item.turn_id === turnID) }

async function selectSession(id: number) {
  if (store.sending) return
  drafts.set(store.sessionID, draft.value)
  draft.value = drafts.get(id) || ''
  selectedRunID.value = 0
  selectedTurnID.value = 0
  runDetail.value = null
  detailError.value = ''
  followBottom.value = true
  sessionDrawer.value = false
  await store.selectSession(id)
  await router.replace({ name: 'agent', query: id ? { session: id } : {} })
  await nextTick()
  scrollBottom()
}

async function loadDetail(id: number) {
  const version = ++detailVersion
  selectedRunID.value = id
  runDetail.value = null
  detailError.value = ''
  detailLoading.value = true
  try {
    const detail = await getAgentRun(id)
    if (version === detailVersion) runDetail.value = detail
  } catch (cause) {
    if (version === detailVersion) detailError.value = formatAPIError(cause)
  } finally {
    if (version === detailVersion) detailLoading.value = false
  }
}

function showDetails(turnID: number, openDrawer = true) {
  selectedTurnID.value = turnID
  const runs = subagentRuns(turnRuns(turnID))
  const selected = [...runs].reverse().find((run) => isActiveStatus(run.status)) || runs[runs.length - 1]
  if (selected) void loadDetail(selected.id)
  else { selectedRunID.value = 0; runDetail.value = null; detailLoading.value = false; detailError.value = ''; ++detailVersion }
  if (openDrawer) {
    if (desktopViewport.value) detailsCollapsed.value = false
    else detailDrawer.value = true
  }
}

async function sendMessage() {
  const message = draft.value.trim()
  if (!message || store.sending || store.active) return
  draft.value = ''
  drafts.delete(store.sessionID)
  selectedRunID.value = 0
  selectedTurnID.value = 0
  runDetail.value = null
  ++detailVersion
  followBottom.value = true
  const succeeded = await store.send(message)
  if (!succeeded) draft.value = message
  else if (latestTurn.value) showDetails(latestTurn.value.id, false)
  if (store.sessionID) await router.replace({ name: 'agent', query: { session: store.sessionID } })
  await nextTick()
  scrollBottom()
  composer.value?.focus()
}

function handleComposerKey(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && window.matchMedia('(min-width: 768px)').matches) {
    event.preventDefault()
    void sendMessage()
  }
}

function handleScroll() {
  const element = messageList.value
  if (element) followBottom.value = element.scrollHeight - element.scrollTop - element.clientHeight < 100
}

function scrollBottom() {
  if (followBottom.value && messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
}

async function loadOlder() {
  const element = messageList.value
  const previousHeight = element?.scrollHeight || 0
  loadingOlder.value = true
  followBottom.value = false
  try {
    await store.loadOlder()
    await nextTick()
    if (element) element.scrollTop += element.scrollHeight - previousHeight
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
    await navigator.clipboard.writeText(turn.output_text)
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
  const move = (value: PointerEvent) => { detailWidth.value = Math.max(280, Math.min(560, width + start - value.clientX)) }
  const stop = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', stop); resizeCleanup = null }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop, { once: true })
  resizeCleanup = stop
}

watch(() => `${store.workspace?.cursor}:${latestTurn.value?.status}:${turns.value.length}:${store.pendingMessage}`, async () => { await nextTick(); scrollBottom() })
watch(sessionsCollapsed, (value) => localStorage.setItem('mf-assistant-sessions-collapsed', String(value)))
watch(detailsCollapsed, (value) => localStorage.setItem('mf-assistant-details-collapsed', String(value)))
watch(() => store.sessionID, () => { selectedRunID.value = 0; selectedTurnID.value = 0; runDetail.value = null; ++detailVersion })
watch(() => route.query.session, (value) => {
  const id = Number(value || 0)
  if (route.name === 'agent' && Number.isInteger(id) && id >= 0 && id !== store.sessionID) void selectSession(id)
})
watch(() => store.workspace?.runs, (runs) => {
  if (!runs?.length) return
  if (!selectedTurnID.value) selectedTurnID.value = latestTurn.value?.id || 0
  const selected = runs.find((run) => run.id === selectedRunID.value)
  if (!selected) {
    const candidates = subagentRuns(runs.filter((run) => run.turn_id === selectedTurnID.value))
    const latest = candidates[candidates.length - 1]
    if (latest) void loadDetail(latest.id)
  } else if (runDetail.value && runDetail.value.updated_at !== selected.updated_at) void loadDetail(selected.id)
})

onMounted(async () => {
  desktopMedia.addEventListener('change', updateViewport)
  mobileMedia.addEventListener('change', updateViewport)
  store.reset()
  try {
    authenticated.value = (await getCurrentAuth()).authenticated
    authLoaded.value = true
    if (!authenticated.value) return
    await store.loadModels()
    await store.loadSessions()
    const requested = Number(route.query.session || 0)
    const initial = requested > 0 ? requested : store.sessions[0]?.id || 0
    await selectSession(initial)
  } catch (cause) { store.error = formatAPIError(cause) }
})

onBeforeUnmount(() => { store.reset(); ++detailVersion; resizeCleanup?.(); if (copyTimer) clearTimeout(copyTimer); desktopMedia.removeEventListener('change', updateViewport); mobileMedia.removeEventListener('change', updateViewport) })
defineExpose({ refreshPage: () => store.sessionID ? store.selectSession(store.sessionID) : store.loadSessions() })
</script>

<template>
  <AgentGuestChat v-if="authLoaded && !authenticated" />
  <p v-else-if="!authLoaded" role="status">{{ store.error || '正在加载助理…' }}</p>
  <section v-else class="assistant-workspace" :class="{ 'assistant-sessions-collapsed': sessionsCollapsed, 'assistant-details-collapsed': detailsCollapsed }" :style="{ '--assistant-details-width': detailsCollapsed ? '0px' : `${detailWidth}px` }" aria-label="助理工作区" @touchstart.stop @touchmove.stop @touchend.stop @pointerdown.stop>
    <aside class="assistant-sessions" aria-label="会话列表">
      <div class="assistant-sidebar-heading"><span class="assistant-wordmark"><IconCode /> 助理</span><button class="assistant-icon-button" type="button" aria-label="新建对话" @click="selectSession(0)"><IconPlus /></button></div>
      <button class="assistant-new-session" type="button" @click="selectSession(0)"><IconPlus /> 新建对话</button>
      <div class="assistant-section-label">最近对话</div>
      <div class="assistant-session-list"><button v-for="session in store.sessions" :key="session.id" type="button" :class="['assistant-session', { selected: store.sessionID === session.id }]" @click="selectSession(session.id)"><span>{{ session.title || '未命名对话' }}</span><small>{{ session.provider === 'web' ? '网页' : session.provider === 'wechat_work_app' ? '企业微信' : session.provider }}</small></button><p v-if="!store.sessions.length" class="assistant-sidebar-empty">暂无会话记录</p></div>
      <router-link class="assistant-history-link" :to="{ name: 'agent-plans' }"><IconHistory /> 任务与执行记录</router-link>
    </aside>

    <main class="assistant-conversation">
      <header class="assistant-conversation-header"><button type="button" class="assistant-icon-button" :aria-label="sessionToggleLabel" :title="sessionToggleLabel" :aria-expanded="mobileViewport ? sessionDrawer : !sessionsCollapsed" @click="toggleSessions"><IconMenu /></button><div class="assistant-conversation-title"><strong>{{ title }}</strong><span v-if="store.sessionID" class="assistant-connection" :class="store.connection"><i />{{ connectionLabel }}</span></div><button v-if="store.connection === 'failed' || store.connection === 'reconnecting'" type="button" class="assistant-icon-button" aria-label="重新连接" @click="store.selectSession(store.sessionID)"><IconRefresh /></button><button type="button" class="assistant-detail-toggle" :aria-label="detailToggleLabel" :title="detailToggleLabel" :aria-expanded="desktopViewport ? !detailsCollapsed : detailDrawer" @click="toggleDetails"><IconCode /><span>子代理</span></button></header>
      <div v-if="store.error" class="assistant-error" role="alert"><span>{{ assistantErrorMessage(store.error) }}</span><button class="assistant-icon-button" type="button" aria-label="关闭错误提示" @click="store.error = ''"><IconClose /></button></div>
      <div ref="messageList" class="assistant-message-list" aria-label="对话消息" @scroll.passive="handleScroll">
        <div v-if="store.loading && !turns.length" class="assistant-state-note" role="status">正在读取对话…</div>
        <div v-else-if="!turns.length && !store.pendingMessage" class="assistant-welcome"><span class="assistant-welcome-symbol"><IconCode /></span><h1>有什么需要处理？</h1><p>继续已有对话，或者开始新的任务。<br />执行过程随回答实时显示，子代理活动可在右侧查看。</p><div class="assistant-suggestions"><button type="button" @click="draft = '整理最近订阅中的重要信息'; composer?.focus()">整理最近订阅</button><button type="button" @click="draft = '查看当前任务的执行情况'; composer?.focus()">查看任务进展</button></div></div>
        <div v-if="store.workspace?.has_more" class="assistant-load-older"><button type="button" :disabled="loadingOlder" @click="loadOlder">{{ loadingOlder ? '正在读取…' : '读取更早的对话' }}</button></div>
        <div class="assistant-message-column">
          <article v-for="turn in turns" :key="turn.id" class="assistant-turn">
            <div class="assistant-user-message"><p>{{ turn.input_text }}</p></div>
            <div class="assistant-answer"><div class="assistant-answer-heading"><span class="assistant-avatar"><IconCode /></span><span>助理</span><small v-if="turn.model">{{ turn.model }}</small></div>
              <AgentTurnActivity v-if="turnRuns(turn.id).length" :runs="turnRuns(turn.id)" :items="turnItems(turn.id)" :status="turn.status" />
              <AgentMarkdown v-if="turnText(turn)" :content="turnText(turn)" />
              <div v-else-if="isActiveStatus(turn.status)" class="assistant-state-note" role="status"><span class="assistant-thinking-dot" />{{ turnState(turn) }}…</div>
              <p v-if="turn.error_message" class="assistant-turn-error" role="alert">{{ assistantErrorMessage(turn.error_message) }}</p>
              <p v-if="turn.delivery_status === 'failed'" class="assistant-delivery-error" role="alert">回复已保存，渠道发送失败。{{ turn.delivery_error }}</p>
              <p v-else-if="turn.delivery_status === 'pending'" class="assistant-state-note">回复已保存，等待渠道发送。</p>
              <div class="assistant-answer-footer"><span>{{ isActiveStatus(turn.status) && turnText(turn) ? '生成中，等待结果校验' : turnState(turn) }}</span><button v-if="turn.output_text" class="assistant-icon-button" type="button" :aria-label="copiedTurnID === turn.id ? '已经复制' : '复制回答'" @click="copyReply(turn)"><IconCopy /><span v-if="copiedTurnID === turn.id">已复制</span></button></div>
            </div>
          </article>
          <div v-if="pendingVisible" class="assistant-user-message assistant-pending-message"><p>{{ store.pendingMessage }}</p><small>正在提交…</small></div>
        </div>
      </div>
      <div class="assistant-composer-area"><button v-if="!followBottom" class="assistant-jump-button" type="button" @click="followBottom = true; scrollBottom()">查看最新消息 ↓</button><form class="assistant-composer" @submit.prevent="sendMessage"><textarea ref="composer" v-model="draft" rows="2" :disabled="store.sending" aria-label="发送给助理的消息" placeholder="向助理发送消息…" @keydown="handleComposerKey" /><div class="assistant-composer-toolbar"><AgentModelSelect v-model="store.selectedModelKey" :models="store.models" :disabled="store.sending || store.active" /><button v-if="store.active" type="button" class="assistant-send-button stop" :disabled="stopping || latestTurn?.cancel_requested" aria-label="停止当前任务" @click="stopTurn"><IconStop /></button><button v-else type="submit" class="assistant-send-button" :disabled="!draft.trim() || store.sending || !store.selectedModel" aria-label="发送消息"><IconArrowUp /></button></div></form><p class="assistant-composer-hint"><router-link v-if="!store.models.length" :to="{ name: 'settings-model' }">前往设置启用模型</router-link><template v-else>回答请结合来源核验。完整执行过程可在详情中查看。</template></p></div>
    </main>

    <div class="assistant-resizer" role="separator" aria-label="调整子代理侧栏宽度" aria-orientation="vertical" :aria-valuenow="detailWidth" :aria-valuemin="280" :aria-valuemax="560" tabindex="0" @pointerdown="beginResize" @keydown.left.prevent="detailWidth = Math.min(560, detailWidth + 20)" @keydown.right.prevent="detailWidth = Math.max(280, detailWidth - 20)" />
    <aside class="assistant-details" aria-label="子代理"><header><span><IconCode /> 子代理</span><small>{{ detailRuns.filter((run) => !run.task_packet.stage && !run.task_packet.capability_key).length }} 项任务</small><button type="button" class="assistant-icon-button" aria-label="收起子代理侧栏" @click="detailsCollapsed = true"><IconClose /></button></header><AgentRunDetails :runs="detailRuns" :items="detailItems" :selected-i-d="selectedRunID" :detail="runDetail" :loading="detailLoading" :error="detailError" @select="loadDetail" /></aside>

    <ArcoDrawer :visible="sessionDrawer" placement="left" :width="'min(320px, 90vw)'" title="会话列表" :footer="false" @cancel="sessionDrawer = false"><button class="assistant-new-session" type="button" @click="selectSession(0)"><IconPlus /> 新建对话</button><div class="assistant-session-list"><button v-for="session in store.sessions" :key="session.id" type="button" :class="['assistant-session', { selected: store.sessionID === session.id }]" @click="selectSession(session.id)"><span>{{ session.title || '未命名对话' }}</span></button></div><router-link class="assistant-history-link" :to="{ name: 'agent-plans' }"><IconHistory /> 任务与执行记录</router-link></ArcoDrawer>
    <ArcoDrawer :visible="detailDrawer" placement="right" :width="'min(520px, 100vw)'" title="子代理" :footer="false" @cancel="detailDrawer = false"><AgentRunDetails :runs="detailRuns" :items="detailItems" :selected-i-d="selectedRunID" :detail="runDetail" :loading="detailLoading" :error="detailError" @select="loadDetail" /></ArcoDrawer>
  </section>
</template>

<style scoped>
.assistant-workspace { --assistant-expanded-sidebar-width: 222px; --assistant-sidebar-width: var(--assistant-expanded-sidebar-width); --assistant-resizer-width: 5px; display: grid; grid-template-columns: var(--assistant-sidebar-width) minmax(0, 1fr) var(--assistant-resizer-width) var(--assistant-details-width); height: 100%; min-height: 0; overflow: hidden; border: 1px solid var(--mf-border); border-radius: 12px; background: var(--mf-surface, #fff); color: var(--mf-text); }
.assistant-sessions { grid-column: 1; }
.assistant-conversation { grid-column: 2; }
.assistant-resizer { grid-column: 3; }
.assistant-details { grid-column: 4; }
.assistant-sessions-collapsed { --assistant-sidebar-width: 0px; }
.assistant-details-collapsed { --assistant-resizer-width: 0px; }
.assistant-sessions-collapsed > .assistant-sessions, .assistant-details-collapsed > .assistant-details, .assistant-details-collapsed > .assistant-resizer { display: none; }
.assistant-sessions { display: flex; flex-direction: column; min-height: 0; padding: 15px 10px 10px; border-right: 1px solid var(--mf-border); background: var(--mf-surface-muted, #f7f8fa); }
.assistant-sidebar-heading { display: flex; justify-content: space-between; align-items: center; padding: 0 8px 18px; }
.assistant-wordmark { display: inline-flex; align-items: center; gap: 9px; font-size: 14px; font-weight: 650; letter-spacing: 1px; }
.assistant-icon-button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; border: 0; background: transparent; padding: 7px; border-radius: 6px; color: var(--mf-text-muted); cursor: pointer; font-size: 15px; }
.assistant-icon-button:hover { background: var(--mf-surface-muted, #eef0f3); color: var(--mf-text); }
.assistant-new-session { display: flex; align-items: center; gap: 9px; width: 100%; padding: 10px 12px; border: 1px solid var(--mf-border); border-radius: 8px; background: var(--mf-surface, #fff); color: inherit; font-size: 12px; text-align: left; cursor: pointer; }
.assistant-section-label { color: var(--mf-text-muted); font-size: 10px; padding: 23px 12px 10px; }
.assistant-session-list { flex: 1; min-height: 0; overflow-y: auto; padding: 2px 0; }
.assistant-session { display: grid; gap: 6px; width: 100%; border: 0; border-radius: 7px; background: transparent; padding: 11px 12px; text-align: left; color: inherit; cursor: pointer; margin-bottom: 3px; font-size: 12px; }
.assistant-session > span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.assistant-session small { color: var(--mf-text-muted); font-size: 10px; }
.assistant-session:hover, .assistant-session.selected { background: var(--mf-surface, #e9ecef); }
.assistant-history-link { display: flex; align-items: center; gap: 8px; padding: 14px 12px 6px; color: var(--mf-text-muted); text-decoration: none; font-size: 11px; }
.assistant-sidebar-empty { padding: 12px; font-size: 12px; color: var(--mf-text-muted); }
.assistant-conversation { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.assistant-conversation-header { display: flex; align-items: center; gap: 12px; padding: 15px 22px; min-height: 64px; border-bottom: 1px solid var(--mf-border); }
.assistant-conversation-title { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; min-width: 0; flex: 1; }
.assistant-conversation-title strong { font-size: 12px; font-weight: 550; max-width: 250px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.assistant-connection { display: inline-flex; align-items: center; gap: 5px; font-size: 10px; color: var(--mf-text-muted); white-space: nowrap; }
.assistant-connection i { width: 5px; height: 5px; border-radius: 50%; background: #ad8c53; }
.assistant-connection.connected i { background: #399b72; }
.assistant-connection.failed i { background: #c15454; }
.assistant-detail-toggle { display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--mf-border); border-radius: 6px; padding: 6px 8px; background: transparent; color: var(--mf-text-muted); font-size: 11px; cursor: pointer; white-space: nowrap; }
.assistant-message-list { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 32px 30px 16px; scrollbar-gutter: stable; }
.assistant-message-column { width: min(100%, 760px); margin: 0 auto; }
.assistant-turn { margin-bottom: 36px; }
.assistant-user-message { display: flex; flex-direction: column; align-items: flex-end; margin-bottom: 26px; }
.assistant-user-message p { max-width: 87%; margin: 0; padding: 11px 16px; border-radius: 14px 14px 4px 14px; background: var(--mf-surface-muted, #f2f3f5); font-size: 14px; line-height: 1.8; white-space: pre-wrap; overflow-wrap: anywhere; }
.assistant-user-message small { margin-top: 6px; color: var(--mf-text-muted); }
.assistant-answer-heading { display: flex; align-items: center; gap: 8px; font-size: 12px; margin-bottom: 17px; font-weight: 550; }
.assistant-answer-heading small { font-size: 10px; font-weight: 400; color: var(--mf-text-muted); }
.assistant-avatar { display: grid; place-items: center; width: 23px; height: 23px; border: 1px solid var(--mf-border); border-radius: 7px; font-size: 13px; }
.assistant-activity { display: flex; align-items: center; gap: 8px; max-width: 100%; padding: 9px 0 14px; background: transparent; border: 0; color: var(--mf-text-muted); font-size: 11px; text-align: left; cursor: pointer; }
.assistant-activity i { width: 6px; height: 6px; border-radius: 50%; background: #399b72; flex-shrink: 0; }
.assistant-activity i.active { background: #b98336; animation: assistant-pulse 1.6s ease-in-out infinite; }
.assistant-activity-state { opacity: .75; white-space: nowrap; }
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
.assistant-composer-area { position: relative; flex-shrink: 0; padding: 6px 30px 12px; }
.assistant-composer { max-width: 760px; margin: 0 auto; border: 1px solid var(--mf-border); border-radius: 13px; padding: 13px 14px 10px; background: var(--mf-surface, #fff); box-shadow: 0 4px 22px #19223006; }
.assistant-composer:focus-within { border-color: var(--mf-text-muted); }
.assistant-composer textarea { display: block; width: 100%; min-height: 46px; max-height: 160px; resize: vertical; border: 0; outline: 0; background: transparent; color: var(--mf-text); font: inherit; font-size: 14px; line-height: 1.7; }
.assistant-composer textarea::placeholder { color: var(--mf-text-muted); }
.assistant-composer-toolbar { display: flex; justify-content: space-between; align-items: center; margin-top: 7px; }
.assistant-composer-toolbar > span { font-size: 10px; color: var(--mf-text-muted); }
.assistant-send-button { display: grid; place-items: center; width: 31px; height: 31px; border: 0; border-radius: 8px; background: var(--mf-text); color: var(--mf-surface, #fff); cursor: pointer; font-size: 17px; }
.assistant-send-button:disabled { opacity: .28; cursor: default; }
.assistant-send-button.stop { background: var(--mf-surface-muted, #eef0f3); color: var(--mf-text); border: 1px solid var(--mf-border); }
.assistant-composer-hint { text-align: center; color: var(--mf-text-muted); font-size: 10px; margin: 9px 0 0; line-height: 1.6; }
.assistant-jump-button { position: absolute; top: -36px; left: 50%; transform: translateX(-50%); border: 1px solid var(--mf-border); background: var(--mf-surface, #fff); padding: 6px 12px; border-radius: 20px; font-size: 11px; color: var(--mf-text-muted); cursor: pointer; }
.assistant-load-older { text-align: center; margin-bottom: 24px; }
.assistant-load-older button { border: 0; background: transparent; color: var(--mf-text-muted); font-size: 11px; cursor: pointer; padding: 8px; }
.assistant-resizer { background: transparent; border-left: 1px solid var(--mf-border); cursor: col-resize; touch-action: none; }
.assistant-resizer:hover, .assistant-resizer:focus-visible { background: var(--mf-border); outline: none; }
.assistant-details { overflow: auto; min-height: 0; }
.assistant-details > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 64px; padding: 15px 18px; border-bottom: 1px solid var(--mf-border); }
.assistant-details > header > span { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 550; }
.assistant-details > header small { font-size: 10px; color: var(--mf-text-muted); }
.assistant-mobile-only { display: none; }
@keyframes assistant-pulse { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .assistant-thinking-dot, .assistant-activity i.active { animation: none; } }
@media (max-width: 1199px) { .assistant-workspace { --assistant-expanded-sidebar-width: 200px; grid-template-columns: var(--assistant-sidebar-width) minmax(0, 1fr); } .assistant-details, .assistant-resizer { display: none; } }
@media (max-width: 767px) { .assistant-workspace { grid-template-columns: minmax(0, 1fr); border-radius: 10px; } .assistant-conversation { grid-column: 1; } .assistant-sessions { display: none; } .assistant-mobile-only { display: inline-flex; } .assistant-conversation-header { min-height: 56px; padding: 10px 12px; gap: 8px; } .assistant-conversation-title { gap: 5px; } .assistant-conversation-title strong { max-width: 165px; } .assistant-conversation-title .assistant-connection { font-size: 9px; } .assistant-message-list { padding: 24px 16px 10px; } .assistant-composer-area { padding: 4px 12px max(10px, env(safe-area-inset-bottom)); } .assistant-user-message p { max-width: 94%; font-size: 14px; } .assistant-composer-hint { font-size: 9px; } .assistant-turn { margin-bottom: 28px; } }
</style>
