<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { IconArrowUp, IconClose, IconCode, IconPlus, IconStop } from '@arco-design/web-vue/es/icon'
import { listAssistantModels, modelSelectionKey, streamGuestChat, type AgentModel, type GuestChatMessage } from '@/api/agentModels'
import { formatAPIError } from '@/api/client'
import AgentMarkdown from '@/components/agent/AgentMarkdown.vue'
import AgentModelSelect from '@/components/agent/AgentModelSelect.vue'

const models = ref<AgentModel[]>([])
const selectedKey = ref('')
const messages = ref<(GuestChatMessage & { model?: string; reasoning?: string; failed?: boolean })[]>([])
const draft = ref('')
const error = ref('')
const loading = ref(true)
const sending = ref(false)
const detailsOpen = ref(false)
const scrollArea = ref<HTMLElement | null>(null)
const selectedModel = computed(() => models.value.find((model) => modelSelectionKey(model) === selectedKey.value))
const latestAnswer = computed(() => [...messages.value].reverse().find((message) => message.role === 'assistant'))
let controller: AbortController | null = null

async function scrollBottom() {
  await nextTick()
  scrollArea.value?.scrollTo({ top: scrollArea.value.scrollHeight })
}

async function send() {
  const model = selectedModel.value
  const content = draft.value.trim()
  if (sending.value || !model || !content) return
  const previous = messages.value.filter((message) => !message.failed && message.content).map(({ role, content }) => ({ role, content }))
  if (previous.length >= 40) { error.value = '当前游客对话已达到消息数量限制，请新建对话'; return }
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
    await streamGuestChat(model, [...previous, { role: 'user', content }], controller.signal, (kind, event) => {
      if (kind === 'completed') reply.content = event.content || ''
      if (kind === 'delta' && (event.kind === 'text' || event.kind === 'reasoning')) {
        const item = items.get(event.item_id || event.kind) || { kind: event.kind, text: '' }
        item.text = event.replace ? event.delta || '' : item.text + (event.delta || '')
        items.set(event.item_id || event.kind, item)
        reply.content = [...items.values()].filter((entry) => entry.kind === 'text').map((entry) => entry.text).join('')
        reply.reasoning = [...items.values()].filter((entry) => entry.kind === 'reasoning').map((entry) => entry.text).join('\n')
      }
      void scrollBottom()
    })
  } catch (cause) {
    messages.value[userIndex]!.failed = true
    reply.failed = true
    error.value = controller.signal.aborted ? '已停止生成' : formatAPIError(cause)
  } finally {
    sending.value = false
    controller = null
  }
}

function clearConversation() {
  if (sending.value) return
  messages.value = []
  error.value = ''
}

onMounted(async () => {
  try {
    models.value = await listAssistantModels(true)
    selectedKey.value = models.value[0] ? modelSelectionKey(models.value[0]) : ''
  } catch (cause) { error.value = formatAPIError(cause) }
  finally { loading.value = false }
})
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="guest-assistant" aria-label="游客助理" @touchstart.stop @touchmove.stop @touchend.stop @pointerdown.stop>
    <header><span><IconCode /> 助理 <small>游客</small></span><div><button type="button" :disabled="sending" aria-label="新建游客对话" @click="clearConversation"><IconPlus /></button><button type="button" :aria-expanded="detailsOpen" aria-label="游客执行详情" @click="detailsOpen = !detailsOpen"><IconCode /></button><router-link :to="{ name: 'login', query: { redirect: '/agent' } }">登录</router-link></div></header>
    <main ref="scrollArea" class="guest-messages" aria-live="polite">
      <div v-if="!messages.length" class="guest-welcome"><IconCode /><h1>向助理发送消息</h1><p>{{ loading ? '正在读取可用模型…' : models.length ? '选择管理员开放的模型开始对话。' : '管理员暂未开放游客模型。' }}</p><small>游客对话保留在当前页面。每个网络地址每日最多调用 20 次。</small></div>
      <article v-for="(message, index) in messages" :key="index" :class="['guest-message', message.role]">
        <p v-if="message.role === 'user'">{{ message.content }}</p>
        <template v-else><div class="guest-answer-heading"><IconCode /><span>助理</span><small>{{ message.model }}</small></div><AgentMarkdown v-if="message.content" :content="message.content" /><p v-else>{{ sending && index === messages.length - 1 ? '正在生成…' : '本次未完成回答' }}</p><small v-if="message.failed" class="guest-error">回答未完成</small></template>
      </article>
    </main>
    <aside v-if="detailsOpen" class="guest-details" aria-label="游客执行详情面板"><div><strong>执行详情</strong><button type="button" aria-label="关闭游客执行详情" @click="detailsOpen = false"><IconClose /></button></div><dl><dt>模型</dt><dd>{{ latestAnswer?.model || selectedModel?.model || '尚未选择' }}</dd><dt>状态</dt><dd>{{ sending ? '正在生成' : latestAnswer?.failed ? '未完成' : latestAnswer ? '已完成' : '尚未开始' }}</dd><dt>公开推理摘要</dt><dd class="guest-reasoning">{{ latestAnswer?.reasoning || '供应商尚未返回公开推理摘要。' }}</dd></dl></aside>
    <div class="guest-composer"><p v-if="error" class="guest-error" role="alert">{{ error }}</p><form @submit.prevent="send"><textarea v-model="draft" aria-label="发送给游客助理的消息" placeholder="向助理发送消息…" rows="2" :disabled="sending || !models.length" @keydown.enter.exact.prevent="send" /><div><AgentModelSelect v-model="selectedKey" :models="models" :disabled="sending" /><button v-if="sending" type="button" aria-label="停止游客回答" @click="controller?.abort()"><IconStop /></button><button v-else type="submit" aria-label="发送游客消息" :disabled="!draft.trim() || !selectedModel"><IconArrowUp /></button></div></form></div>
  </section>
</template>

<style scoped>
.guest-assistant { height: 100%; min-height: 0; position: relative; display: flex; flex-direction: column; border: 1px solid var(--mf-border); border-radius: 12px; background: var(--mf-surface, #fff); overflow: hidden; }
.guest-assistant header { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--mf-border); }
.guest-assistant header span, .guest-assistant header div { display: flex; align-items: center; gap: 12px; }
.guest-assistant header small, .guest-answer-heading small { color: var(--mf-text-muted); font-size: 11px; }
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
.guest-details { position: absolute; top: 60px; right: 0; bottom: 0; width: min(360px, 100%); box-sizing: border-box; background: var(--mf-surface, #fff); border-left: 1px solid var(--mf-border); padding: 18px; overflow: auto; z-index: 5; box-shadow: -6px 0 20px #0000000d; }
.guest-details > div { display: flex; justify-content: space-between; align-items: center; }
.guest-details dt { margin-top: 20px; color: var(--mf-text-muted); font-size: 12px; }
.guest-details dd { margin: 6px 0 0; font-size: 13px; overflow-wrap: anywhere; }
.guest-reasoning { white-space: pre-wrap; line-height: 1.8; }
</style>
