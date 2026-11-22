<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { AxiosError } from 'axios'
import { apiClient, formatAPIError } from '@/api/client'

const endpoint = '/api/v1/community/x'
type Envelope<Value> = { data: Value }
type Subscription = { source: { id: number; name: string }; item_count: number }
const emit = defineEmits<{ subscribed: []; openSource: [source: { id: number; name: string; kind: 'subscriptions' }] }>()
const props = defineProps<{ followingSubscribed: boolean }>()
const available = ref(false)
const connected = ref(false)
const error = ref('')
const pending = ref(false)
const account = ref('')
const subscription = ref<Subscription | null>(null)
const open = ref(false)
const dialog = ref<HTMLDialogElement | null>(null)
const clipboardInput = ref<HTMLInputElement | null>(null)
const pasteFeedback = ref('')
const pasting = ref(false)
const displayStatus = computed(() => connected.value ? '已连接。推文显示在订阅信息流中。' : '连接 X 账号，同步关注动态和指定账号的推文。')
const lifecycle = new AbortController()
let login: AbortController | undefined
let pollTimer: ReturnType<typeof setTimeout> | undefined

async function readStatus(signal: AbortSignal) {
  const response = await apiClient.get<Envelope<{ connected: boolean }>>(endpoint, { signal })
  connected.value = response.data.data.connected
  return connected.value
}

onMounted(async () => {
  try {
    await readStatus(lifecycle.signal)
    available.value = true
  } catch (cause) {
    if (lifecycle.signal.aborted || (cause instanceof AxiosError && [401, 404].includes(cause.response?.status ?? 0))) return
    available.value = true
    error.value = formatAPIError(cause)
  }
})

onUnmounted(() => {
  lifecycle.abort()
  closeBrowser()
})

function openBrowser() {
  if (!dialog.value || open.value) return
  error.value = ''
  login = new AbortController()
  dialog.value.showModal()
  open.value = true
  pollAuthorization(login.signal, Date.now() + 10 * 60 * 1000)
}

function closeBrowser() {
  login?.abort()
  clearTimeout(pollTimer)
  open.value = false
  pasteFeedback.value = ''
  if (clipboardInput.value) clipboardInput.value.value = ''
}

function pollAuthorization(signal: AbortSignal, expires: number) {
  pollTimer = setTimeout(async () => {
    if (signal.aborted) return
    if (Date.now() >= expires) {
      dialog.value?.close()
      error.value = '授权窗口已超时，请重新打开。'
      return
    }
    try {
      if (await readStatus(signal)) {
        dialog.value?.close()
        if (!props.followingSubscribed) await subscribe('')
        return
      }
    } catch (cause) {
      if (signal.aborted) return
      pasteFeedback.value = formatAPIError(cause)
    }
    pollAuthorization(signal, expires)
  }, 4000)
}

async function insertText(text: string) {
  const signal = login?.signal
  if (pasting.value || !signal || signal.aborted || !text) return
  pasting.value = true
  pasteFeedback.value = ''
  try {
    await apiClient.post(`${endpoint}/input`, { text }, { signal })
    pasteFeedback.value = '已输入到 X 登录窗口。'
  } catch (cause) {
    if (!signal.aborted) pasteFeedback.value = formatAPIError(cause)
  } finally {
    pasting.value = false
  }
}

async function pasteClipboard() {
  if (!navigator.clipboard?.readText) {
    clipboardInput.value?.focus()
    pasteFeedback.value = '请在下方输入框长按或使用粘贴快捷键。'
    return
  }
  try {
    await insertText(await navigator.clipboard.readText())
  } catch {
    clipboardInput.value?.focus()
    pasteFeedback.value = '浏览器未授予剪贴板权限，请在下方输入框粘贴。'
  }
}

function sendInput() {
  const field = clipboardInput.value
  if (!field || pasting.value) return
  const text = field.value
  field.value = ''
  void insertText(text)
}

function attachPaste(event: Event) {
  const frame = event.currentTarget as HTMLIFrameElement
  const signal = login?.signal
  frame.contentWindow?.addEventListener('keydown', (key) => {
    if ((key.ctrlKey || key.metaKey) && key.key.toLowerCase() === 'v' && !key.altKey) key.stopImmediatePropagation()
  }, { signal, capture: true })
  frame.contentWindow?.addEventListener('paste', (paste) => {
    const text = paste.clipboardData?.getData('text/plain')
    if (!text) return
    paste.preventDefault()
    void insertText(text)
  }, { signal, capture: true })
}

async function subscribe(name: string) {
  if (pending.value) return
  error.value = ''
  subscription.value = null
  pending.value = true
  try {
    const response = await apiClient.post<Envelope<Subscription>>(`${endpoint}/subscribe`, { account: name }, {
      signal: lifecycle.signal, timeout: 43000,
    })
    subscription.value = response.data.data
    if (name) account.value = ''
  } catch (cause) {
    if (lifecycle.signal.aborted) return
    if (cause instanceof AxiosError && cause.response?.status === 422) connected.value = false
    error.value = formatAPIError(cause)
  } finally {
    pending.value = false
    if (!lifecycle.signal.aborted) emit('subscribed')
  }
}
</script>

<template>
  <section v-if="available || error" class="sources-panel x-browser" aria-label="X">
    <div>
      <h2>X</h2>
      <p role="status">{{ displayStatus }}</p>
    </div>
    <button v-if="available && !connected" class="sources-button" type="button" @click="openBrowser">连接 X</button>
    <button v-if="connected" class="sources-button" type="button" :disabled="pending" @click="subscribe('')">
      {{ followingSubscribed ? '刷新关注动态' : '订阅关注动态' }}
    </button>
    <form v-if="connected" class="x-browser__row" @submit.prevent="subscribe(account)">
      <input v-model.trim="account" class="sources-input" aria-label="X 用户名" placeholder="@用户名" maxlength="16" autocapitalize="none" :spellcheck="false" required />
      <button class="sources-button" type="submit" :disabled="pending || !account">订阅此账号</button>
    </form>
    <p v-if="error" class="x-browser__wide" role="alert">{{ error }}</p>
    <p v-if="pending" class="x-browser__wide" role="status">正在同步推文…</p>
    <div v-if="subscription" class="x-browser__row" role="status">
      <span>{{ subscription.source.name }}已同步 {{ subscription.item_count }} 条推文</span>
      <button class="sources-button" type="button" @click="emit('openSource', { ...subscription.source, kind: 'subscriptions' })">查看订阅</button>
    </div>

    <dialog ref="dialog" class="x-browser__dialog" aria-labelledby="x-browser-title" @close="closeBrowser">
      <div class="x-browser__toolbar">
        <h2 id="x-browser-title">授权 X 账号</h2>
        <button class="sources-button" type="button" :disabled="pasting" @click="pasteClipboard">粘贴到 X</button>
        <form method="dialog">
          <button class="sources-button" type="submit" autofocus>关闭</button>
        </form>
      </div>
      <form class="x-browser__row" @submit.prevent="sendInput">
        <input ref="clipboardInput" class="sources-input" type="pass&#119;ord" autocomplete="off" maxlength="1024" aria-label="要发送到 X 输入框的文本" placeholder="选中 X 输入框后，在此粘贴或输入" />
        <button class="sources-button" type="submit" :disabled="pasting">发送文本</button>
        <p class="x-browser__wide" role="status">{{ pasteFeedback }}</p>
      </form>
      <iframe v-if="open" :src="`${endpoint}/browser/?resize=remote&quality=9`" title="X 授权页面" referrerpolicy="no-referrer" @load="attachPaste" />
    </dialog>
  </section>
</template>

<style scoped>
.x-browser,
.x-browser__toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px 12px;
}

.x-browser p {
  margin: 0.375rem 0;
  color: var(--mf-text-muted);
}

.x-browser__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.x-browser__row input {
  flex: 1 1 200px;
  min-width: 0;
  font-size: max(16px, 1em);
}

.x-browser__wide {
  flex-basis: 100%;
}

.x-browser__row p:empty {
  margin: 0;
}

.x-browser__dialog {
  box-sizing: border-box;
  width: min(760px, calc(100% - 20px));
  height: min(800px, calc(100dvh - 24px));
  padding: 12px;
  border: 1px solid var(--mf-border);
  border-radius: 16px;
  background: var(--mf-surface);
  color: var(--mf-text);
}

.x-browser__dialog:modal {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  gap: 8px;
  overscroll-behavior: contain;
}

.x-browser__dialog::backdrop {
  overflow: hidden;
  overscroll-behavior: contain;
  background: rgb(0 0 0 / 48%);
}

.x-browser__toolbar h2 {
  margin: 0;
  font-size: 1rem;
}

.x-browser__dialog iframe {
  display: block;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  border: 0;
  border-radius: 10px;
  background: var(--mf-surface);
}

@media (max-width: 600px) {
  .x-browser__dialog {
    width: calc(100% - 12px);
    height: calc(100dvh - 12px);
    max-height: none;
    padding: 8px;
  }

  .x-browser__dialog .x-browser__row input {
    flex-basis: 140px;
  }

  .x-browser__dialog .sources-button {
    min-height: 44px;
  }
}
</style>
