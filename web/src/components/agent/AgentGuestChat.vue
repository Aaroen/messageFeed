<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { IconApps, IconArrowUp, IconStop, IconUser } from '@arco-design/web-vue/es/icon'
import { getGuestHistory, listAssistantModels, modelSelectionKey, streamGuestChat, type AgentModel, type GuestChatMessage, type GuestHistory } from '@/api/agentModels'
import { formatAPIError } from '@/api/client'
import AgentMarkdown from '@/components/agent/AgentMarkdown.vue'
import AgentModelSelect from '@/components/agent/AgentModelSelect.vue'
import AgentReasoningStream from '@/components/agent/AgentReasoningStream.vue'
import AgentComposer from '@/components/agent/AgentComposer.vue'
import AgentWorkspaceShell from '@/components/agent/AgentWorkspaceShell.vue'
import AgentSubagentList from '@/components/agent/AgentSubagentList.vue'
import { useAgentScroll } from '@/composables/useAgentScroll'

const models = ref<AgentModel[]>([])
const fullscreen = defineModel<boolean>('fullscreen', { default: false })
const selectedKey = ref('')
const reasoningEffort = ref('')
type Message = GuestChatMessage & { id: string; model?: string; reasoning?: string; failed?: boolean }
const messages = ref<Message[]>([])
const history = ref<GuestHistory | null>(null)
const conversationID = ref(0)
const draft = ref('')
const drafts = new Map<number, string>()
const error = ref('')
const loading = ref(true)
const sending = ref(false)
const scrollArea = ref<HTMLElement | null>(null)
const { followBottom, handleScroll, scrollBottom } = useAgentScroll(scrollArea)
const selectedModel = computed(() => models.value.find((model) => modelSelectionKey(model) === selectedKey.value))
const conversations = computed(() => {
  const entries = new Map<number, string>()
  for (const turn of history.value?.turns || []) {
    const title = entries.get(turn.conversation_id) ?? (turn.input_text.replace(/\s+/g, ' ').trim() || '未命名对话')
    entries.delete(turn.conversation_id)
    entries.set(turn.conversation_id, title)
  }
  return [...entries].reverse().map(([id, title]) => ({ id, title }))
})
const title = computed(() => conversations.value.find((entry) => entry.id === conversationID.value)?.title || messages.value[0]?.content || '新建对话')
let controller: AbortController | null = null
let nextMessageID = 0

async function send() {
  const model = selectedModel.value
  const content = draft.value.trim()
  if (sending.value || !model || !content) return
  sending.value = true
  error.value = ''
  draft.value = ''
  drafts.delete(conversationID.value)
  const userIndex = messages.value.length
  const id = `local-${++nextMessageID}`
  messages.value.push({ id: `${id}:user`, role: 'user', content }, { id: `${id}:assistant`, role: 'assistant', content: '', model: model.display_name || model.model, reasoning: '' })
  const reply = messages.value[messages.value.length - 1]!
  const items = new Map<string, { kind: string; text: string }>()
  controller = new AbortController()
  scrollBottom(true)
  try {
    await streamGuestChat(model, [{ role: 'user', content }], controller.signal, (kind, event) => {
      if (kind === 'completed') {
        reply.content = event.content || ''
        conversationID.value = event.conversation_id || conversationID.value
        if (!reply.content.trim()) throw new Error('没有收到回答正文，请重试。')
      }
      if (kind === 'delta' && (event.kind === 'text' || event.kind === 'reasoning')) {
        const item = items.get(event.item_id || event.kind) || { kind: event.kind, text: '' }
        item.text = event.replace ? event.delta || '' : item.text + (event.delta || '')
        items.set(event.item_id || event.kind, item)
        reply.content = [...items.values()].filter((entry) => entry.kind === 'text').map((entry) => entry.text).join('')
        reply.reasoning = [...items.values()].filter((entry) => entry.kind === 'reasoning').map((entry) => entry.text).join('\n')
      }
      scrollBottom()
    }, conversationID.value, reasoningEffort.value)
  } catch (cause) {
    messages.value[userIndex]!.failed = true
    reply.failed = true
    error.value = controller.signal.aborted ? '已停止生成' : formatAPIError(cause)
  } finally {
    controller = null
    try { history.value = await getGuestHistory() }
    catch (cause) { error.value = formatAPIError(cause) }
    finally { sending.value = false }
  }
}

function selectConversation(id: number) {
  if (sending.value || loading.value || id === conversationID.value) return
  drafts.set(conversationID.value, draft.value)
  conversationID.value = id
  draft.value = drafts.get(id) || ''
  restoreConversation()
}

function restoreConversation() {
  error.value = ''
  messages.value = (history.value?.turns || []).filter((turn) => turn.conversation_id === conversationID.value).flatMap((turn): Message[] => [
    { id: `${turn.id}:user`, role: 'user', content: turn.input_text },
    { id: `${turn.id}:assistant`, role: 'assistant', content: turn.output_text, model: models.value.find((model) => model.config_id === turn.config_id && model.model === turn.model)?.display_name || turn.model, reasoning: turn.reasoning_content, failed: turn.status !== 'succeeded' },
  ])
  scrollBottom(true)
}

onMounted(async () => {
  try {
    const [available, saved] = await Promise.all([listAssistantModels(true), getGuestHistory()])
    models.value = available
    history.value = saved
    conversationID.value = conversations.value[0]?.id || 0
    restoreConversation()
    selectedKey.value = models.value[0] ? modelSelectionKey(models.value[0]) : ''
  } catch (cause) { error.value = formatAPIError(cause) }
  finally { loading.value = false }
})
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <AgentWorkspaceShell v-model:fullscreen="fullscreen" :title="title" :sessions="conversations" :active-i-d="conversationID" :session-busy="sending || loading" @select-session="selectConversation">
    <template #sessions-footer><router-link :to="{ name: 'login', query: { redirect: '/agent' } }"><IconUser aria-hidden="true" />登录</router-link></template>
    <template #details><AgentSubagentList :entries="[]" /></template>
    <div ref="scrollArea" class="guest-messages" aria-label="对话消息" @scroll.passive="handleScroll">
      <div class="guest-reading-column">
      <div v-if="!messages.length" class="guest-welcome">
        <h2>{{ loading ? '正在读取模型' : models.length ? '有什么需要处理？' : '游客模型暂未开放' }}</h2>
        <router-link v-if="!loading && !models.length" :to="{ name: 'login', query: { redirect: '/agent' } }">登录使用助理</router-link>
        <small v-if="history">同一公网 IP 的历史记录保留 {{ history.retention_days }} 天。每个 IP 每日最多 {{ history.client_daily_limit }} 次，全部游客每日合计最多 {{ history.total_daily_limit }} 次。</small>
      </div>
      <article v-for="(message, index) in messages" :key="message.id" :class="['guest-message', message.role]">
        <p v-if="message.role === 'user'">{{ message.content }}</p>
        <template v-else><div class="guest-answer-heading"><span class="guest-answer-mark"><IconApps aria-hidden="true" /></span><strong>助理</strong><small>{{ message.model }}</small></div><AgentReasoningStream v-if="message.reasoning" :content="message.reasoning" :active="sending && index === messages.length - 1 && !message.content" /><AgentMarkdown v-if="message.content" :content="message.content" /><p v-else>{{ sending && index === messages.length - 1 ? '正在生成…' : '本次未完成回答' }}</p><small v-if="message.failed" class="guest-error">回答未完成</small></template>
      </article>
      </div>
    </div>
    <div class="guest-composer">
      <p v-if="error" class="guest-error" role="alert">{{ error }}</p>
      <button v-if="!followBottom" class="guest-jump" type="button" @click="scrollBottom(true)">查看最新消息</button>
      <AgentComposer v-if="models.length" v-model="draft" label="发送给游客助理的消息" :disabled="sending" @submit="send">
        <AgentModelSelect v-model="selectedKey" v-model:effort="reasoningEffort" :models="models" :disabled="sending" guest />
        <button v-if="sending" type="button" aria-label="停止游客回答" @click="controller?.abort()"><IconStop /></button>
        <button v-else type="submit" aria-label="发送游客消息" :disabled="!draft.trim() || !selectedModel"><IconArrowUp /></button>
      </AgentComposer>
    </div>
  </AgentWorkspaceShell>
</template>

<style scoped>
.guest-answer-heading small { color: var(--mf-text-muted); font-size: 11px; }
.guest-jump { display: block; min-height: 34px; margin: 0 auto 12px; padding: 0 12px; border: 0; border-radius: 7px; background: var(--mf-surface); color: var(--mf-text); cursor: pointer; }
.guest-composer :is(button[type='submit'], button[aria-label='停止游客回答']) { flex: none; display: grid; place-items: center; width: 35px; height: 35px; border: 0; border-radius: 50%; background: var(--mf-text); color: var(--mf-surface); cursor: pointer; }
.guest-composer button:disabled { opacity: .4; cursor: default; }
.guest-messages { flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; }
.guest-reading-column { width: min(100%, 748px); margin-inline: auto; padding: 44px 28px 16px; }
.guest-welcome { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; min-height: 200px; gap: 12px; }
.guest-welcome h2 { font-size: 20px; font-weight: 500; margin: 0; }
.guest-welcome p, .guest-welcome small { color: var(--mf-text-muted); margin: 0; font-size: 12px; line-height: 1.8; }
.guest-message { margin-bottom: 30px; font-size: 14px; line-height: 1.8; }
.guest-message.user { display: flex; justify-content: flex-end; margin-bottom: 26px; }
.guest-message.assistant + .guest-message.user { padding-top: 25px; }
.guest-message.user p { white-space: pre-wrap; overflow-wrap: anywhere; max-width: 88%; background: var(--mf-surface-muted); padding: 10px 14px; border-radius: 8px; margin: 0; font-size: 15px; line-height: 1.6; }
.guest-answer-heading { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.guest-answer-mark { display: grid; place-items: center; width: 26px; height: 26px; border-radius: 7px; background: var(--mf-tab-active-bg); color: var(--mf-primary-strong); }
.guest-answer-heading small { overflow-wrap: anywhere; }
.guest-composer { flex: none; width: min(100%, 748px); margin: 0 auto; padding: 8px 28px max(9px, env(safe-area-inset-bottom)); }
.guest-welcome a { color: var(--mf-primary-strong); font-weight: 600; }
@container (max-width: 630px) { .guest-reading-column { padding-inline: 17px; } .guest-composer { padding-inline: 14px; } }
@media (max-width: 799px) { .guest-reading-column { padding-top: 24px; } .guest-message.user { margin-bottom: 20px; } .guest-message.user p { max-width: 94%; } .guest-composer { padding-top: 3px; } }
@media (max-width: 799px), (pointer: coarse) { .guest-composer :is(button[type='submit'], button[aria-label='停止游客回答']) { min-width: 44px; min-height: 44px; } }
.guest-error { color: #bf4343; font-size: 12px; }
</style>
