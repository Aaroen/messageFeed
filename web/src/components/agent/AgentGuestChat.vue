<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { IconArrowUp, IconCode, IconPlus, IconStop } from '@arco-design/web-vue/es/icon'
import { getGuestHistory, listAssistantModels, modelSelectionKey, streamGuestChat, type AgentModel, type GuestChatMessage, type GuestHistory } from '@/api/agentModels'
import { formatAPIError } from '@/api/client'
import AgentMarkdown from '@/components/agent/AgentMarkdown.vue'
import AgentModelSelect from '@/components/agent/AgentModelSelect.vue'

const models = ref<AgentModel[]>([])
const selectedKey = ref('')
type Message = GuestChatMessage & { model?: string; reasoning?: string; failed?: boolean; expanded?: boolean }
const messages = ref<Message[]>([])
const history = ref<GuestHistory | null>(null)
const conversationID = ref(0)
const draft = ref('')
const error = ref('')
const loading = ref(true)
const sending = ref(false)
const scrollArea = ref<HTMLElement | null>(null)
const selectedModel = computed(() => models.value.find((model) => modelSelectionKey(model) === selectedKey.value))
const conversations = computed(() => {
  const entries = new Map<number, string>()
  for (const turn of history.value?.turns || []) if (!entries.has(turn.conversation_id)) entries.set(turn.conversation_id, turn.input_text.slice(0, 32))
  return [...entries].reverse()
})
let controller: AbortController | null = null

async function scrollBottom() {
  await nextTick()
  scrollArea.value?.scrollTo({ top: scrollArea.value.scrollHeight })
}

async function send() {
  const model = selectedModel.value
  const content = draft.value.trim()
  if (sending.value || !model || !content) return
  sending.value = true
  error.value = ''
  draft.value = ''
  const userIndex = messages.value.length
  messages.value.push({ role: 'user', content }, { role: 'assistant', content: '', model: model.model, reasoning: '' })
  const reply = messages.value[messages.value.length - 1]!
  const items = new Map<string, { kind: string; text: string }>()
  controller = new AbortController()
  await scrollBottom()
  try {
    await streamGuestChat(model, [{ role: 'user', content }], controller.signal, (kind, event) => {
      if (kind === 'completed') { reply.content = event.content || ''; conversationID.value = event.conversation_id || conversationID.value }
      if (kind === 'delta' && (event.kind === 'text' || event.kind === 'reasoning')) {
        const item = items.get(event.item_id || event.kind) || { kind: event.kind, text: '' }
        item.text = event.replace ? event.delta || '' : item.text + (event.delta || '')
        items.set(event.item_id || event.kind, item)
        reply.content = [...items.values()].filter((entry) => entry.kind === 'text').map((entry) => entry.text).join('')
        reply.reasoning = [...items.values()].filter((entry) => entry.kind === 'reasoning').map((entry) => entry.text).join('\n')
      }
      void scrollBottom()
    }, conversationID.value)
  } catch (cause) {
    messages.value[userIndex]!.failed = true
    reply.failed = true
    error.value = controller.signal.aborted ? '已停止生成' : formatAPIError(cause)
  } finally {
    sending.value = false
    controller = null
    try { history.value = await getGuestHistory() }
    catch (cause) { error.value = formatAPIError(cause) }
  }
}

function clearConversation() {
  if (sending.value) return
  messages.value = []
  conversationID.value = 0
  error.value = ''
}

function restoreConversation() {
  error.value = ''
  messages.value = (history.value?.turns || []).filter((turn) => turn.conversation_id === conversationID.value).flatMap((turn): Message[] => [
    { role: 'user', content: turn.input_text },
    { role: 'assistant', content: turn.output_text, model: turn.model, reasoning: turn.reasoning_content, failed: turn.status !== 'succeeded' },
  ])
  void scrollBottom()
}

function reasoningOpen(message: Message, index: number) {
  return message.expanded ?? (sending.value && index === messages.value.length - 1 && (message.reasoning?.length || 0) <= 800)
}

onMounted(async () => {
  try {
    const [available, saved] = await Promise.all([listAssistantModels(true), getGuestHistory()])
    models.value = available
    history.value = saved
    conversationID.value = conversations.value[0]?.[0] || 0
    restoreConversation()
    selectedKey.value = models.value[0] ? modelSelectionKey(models.value[0]) : ''
  } catch (cause) { error.value = formatAPIError(cause) }
  finally { loading.value = false }
})
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="guest-assistant" aria-label="游客助理" @touchstart.stop @touchmove.stop @touchend.stop @pointerdown.stop>
    <header><span><IconCode /> 助理 <small>游客</small></span><div><select v-model="conversationID" aria-label="游客历史对话" :disabled="sending || loading" @change="restoreConversation"><option :value="0">新建对话</option><option v-for="[identifier, title] in conversations" :key="identifier" :value="identifier">{{ title }}</option></select><button type="button" :disabled="sending" aria-label="新建游客对话" @click="clearConversation"><IconPlus /></button><router-link :to="{ name: 'login', query: { redirect: '/agent' } }">登录</router-link></div></header>
    <main ref="scrollArea" class="guest-messages" aria-live="polite">
      <div v-if="!messages.length" class="guest-welcome"><IconCode /><h1>向助理发送消息</h1><p>{{ loading ? '正在读取可用模型…' : models.length ? '选择管理员开放的模型开始对话。' : '管理员暂未开放游客模型。' }}</p><small v-if="history">历史最多保留 {{ history.retention_days }} 天，同一公网 IP 可以访问对应历史。<br />每个 IP 每日最多 {{ history.client_daily_limit }} 次，全部游客每日合计最多 {{ history.total_daily_limit }} 次。</small></div>
      <article v-for="(message, index) in messages" :key="index" :class="['guest-message', message.role]">
        <p v-if="message.role === 'user'">{{ message.content }}</p>
        <template v-else><div class="guest-answer-heading"><IconCode /><span>助理</span><small>{{ message.model }}</small></div><section v-if="message.reasoning || (sending && index === messages.length - 1)" class="guest-activity" aria-label="执行详情"><button type="button" :aria-expanded="reasoningOpen(message, index)" @click="message.expanded = !reasoningOpen(message, index)">执行详情 · {{ reasoningOpen(message, index) ? '收起' : '展开' }}</button><div v-if="reasoningOpen(message, index)"><p>{{ sending && index === messages.length - 1 ? '正在生成回答' : message.failed ? '回答未完成' : '回答已完成' }}</p><small v-if="message.reasoning">公开推理摘要</small><AgentMarkdown v-if="message.reasoning" :content="message.reasoning" /></div></section><AgentMarkdown v-if="message.content" :content="message.content" /><p v-else>{{ sending && index === messages.length - 1 ? '正在生成…' : '本次未完成回答' }}</p><small v-if="message.failed" class="guest-error">回答未完成</small></template>
      </article>
    </main>
    <div class="guest-composer"><p v-if="error" class="guest-error" role="alert">{{ error }}</p><form @submit.prevent="send"><textarea v-model="draft" aria-label="发送给游客助理的消息" placeholder="向助理发送消息…" rows="2" :disabled="sending || !models.length" @keydown.enter.exact.prevent="send" /><div><AgentModelSelect v-model="selectedKey" :models="models" :disabled="sending" /><button v-if="sending" type="button" aria-label="停止游客回答" @click="controller?.abort()"><IconStop /></button><button v-else type="submit" aria-label="发送游客消息" :disabled="!draft.trim() || !selectedModel"><IconArrowUp /></button></div></form></div>
  </section>
</template>

<style scoped>
.guest-assistant { height: 100%; min-height: 0; position: relative; display: flex; flex-direction: column; border: 1px solid var(--mf-border); border-radius: 12px; background: var(--mf-surface, #fff); overflow: hidden; }
.guest-assistant header { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--mf-border); }
.guest-assistant header span, .guest-assistant header div { display: flex; align-items: center; gap: 12px; }
.guest-assistant header small, .guest-answer-heading small { color: var(--mf-text-muted); font-size: 11px; }
.guest-assistant header select { max-width: min(30vw, 240px); padding: 6px; border: 1px solid var(--mf-border); border-radius: 6px; background: var(--mf-surface, #fff); color: var(--mf-text); }
.guest-assistant button { display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; padding: 0; border: 1px solid var(--mf-border); border-radius: 7px; background: var(--mf-surface, #fff); color: var(--mf-text); cursor: pointer; }
.guest-assistant button:disabled { opacity: .4; cursor: default; }
.guest-messages { flex: 1; min-height: 0; overflow: auto; padding: 24px max(18px, calc((100% - 760px) / 2)); }
.guest-welcome { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; min-height: 200px; gap: 12px; }
.guest-welcome h1 { font-size: 20px; font-weight: 500; margin: 0; }
.guest-welcome p, .guest-welcome small { color: var(--mf-text-muted); margin: 0; font-size: 12px; line-height: 1.8; }
.guest-message { margin-bottom: 24px; font-size: 14px; line-height: 1.8; }
.guest-message.user { display: flex; justify-content: flex-end; }
.guest-message.user p { white-space: pre-wrap; overflow-wrap: anywhere; max-width: 90%; background: var(--mf-surface-soft, #f2f3f5); padding: 10px 15px; border-radius: 16px; margin: 0; }
.guest-answer-heading { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.guest-answer-heading small { overflow-wrap: anywhere; }
.guest-composer { width: min(100%, 800px); margin: 0 auto; padding: 12px 18px max(14px, env(safe-area-inset-bottom)); box-sizing: border-box; }
.guest-composer form { border: 1px solid var(--mf-border); border-radius: 14px; padding: 12px; }
.guest-composer textarea { width: 100%; resize: none; border: 0; background: transparent; color: var(--mf-text); font: inherit; outline: none; }
.guest-composer form > div { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.guest-error { color: #bf4343; font-size: 12px; }
.guest-activity { border: 1px solid var(--mf-border); border-radius: 9px; margin: 0 0 16px; overflow: hidden; }
.guest-activity > button { width: 100%; height: auto; padding: 10px 12px; border: 0; border-radius: 0; justify-content: flex-start; color: var(--mf-text-muted); font-size: 12px; }
.guest-activity > div { padding: 0 12px 12px; max-height: 400px; overflow: auto; font-size: 12px; }
</style>
