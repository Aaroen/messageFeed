<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { AxiosError } from 'axios'
import { apiClient, formatAPIError } from '@/api/client'

type APIEnvelope<Data> = { data: Data }
type Publisher = { book_id: string; title: string }
type Article = { title: string; url: string }

const emit = defineEmits<{ subscribed: []; openSource: [source: { id: number; name: string; kind: 'subscriptions' }] }>()
const endpoint = '/api/v1/community/wechat'
const available = ref(false)
const connected = ref(false)
const pending = ref(false)
const error = ref('')
const qrCode = ref('')
const challenge = ref('')
const otp = ref('')
const otpError = ref('')
const submittingOTP = ref(false)
const dialog = ref<HTMLDialogElement | null>(null)
const publishers = ref<Publisher[] | null>(null)
const currentPublisher = ref<Publisher | null>(null)
const articles = ref<Article[] | null>(null)
const subscription = ref<{ id: number; name: string; itemCount: number } | null>(null)
const lifecycle = new AbortController()
let loginRequest: AbortController | undefined
let pollTimer: ReturnType<typeof setTimeout> | undefined

onMounted(async () => {
  try {
    const response = await apiClient.get<APIEnvelope<{ connected: boolean }>>(endpoint, { signal: lifecycle.signal })
    available.value = true
    connected.value = response.data.data.connected
  } catch (cause) {
    if (lifecycle.signal.aborted || (cause instanceof AxiosError && [401, 404].includes(cause.response?.status ?? 0))) return
    available.value = true
    error.value = formatAPIError(cause)
  }
})

function closeLogin() {
  loginRequest?.abort()
  clearTimeout(pollTimer)
  qrCode.value = ''
  challenge.value = ''
  otp.value = ''
  otpError.value = ''
}

onUnmounted(() => {
  lifecycle.abort()
  closeLogin()
})

async function openLogin() {
  if (pending.value) return
  const request = new AbortController()
  loginRequest = request
  pending.value = true
  error.value = ''
  dialog.value?.showModal()
  try {
    const response = await apiClient.post<APIEnvelope<{ qr_code: string }>>(`${endpoint}/login`, {}, { signal: request.signal, timeout: 38000 })
    qrCode.value = response.data.data.qr_code
    pollLogin(request.signal, Date.now() + 300000)
  } catch (cause) {
    if (!request.signal.aborted) {
      dialog.value?.close()
      error.value = formatAPIError(cause)
    }
  } finally {
    pending.value = false
  }
}

function pollLogin(signal: AbortSignal, expiresAt: number) {
  pollTimer = setTimeout(async () => {
    if (signal.aborted) return
    if (Date.now() >= expiresAt) {
      dialog.value?.close()
      error.value = '二维码已过期，请重新发起扫码。'
      return
    }
    try {
      const response = await apiClient.get<APIEnvelope<{ connected: boolean; challenge?: string }>>(`${endpoint}/login`, { signal })
      if (response.data.data.connected) {
        connected.value = true
        dialog.value?.close()
        await loadPublishers()
        return
      }
      const nextChallenge = response.data.data.challenge ?? ''
      if (nextChallenge === 'COOKIE_INVALID') {
        dialog.value?.close()
        error.value = '微信授权未能保存，请重新扫码。'
        return
      }
      challenge.value = nextChallenge
      if (nextChallenge === 'OTP_EXPIRED') return
      pollLogin(signal, expiresAt)
    } catch (cause) {
      if (!signal.aborted) {
        dialog.value?.close()
        error.value = formatAPIError(cause)
      }
    }
  }, 3000)
}

async function submitOTP() {
  const signal = loginRequest?.signal
  if (!signal || signal.aborted || submittingOTP.value || challenge.value === 'OTP_CHECKING') return
  submittingOTP.value = true
  otpError.value = ''
  try {
    await apiClient.post(`${endpoint}/login/otp`, { otp: otp.value }, { signal })
    challenge.value = 'OTP_CHECKING'
    otp.value = ''
  } catch (cause) {
    if (!signal.aborted) otpError.value = formatAPIError(cause)
  } finally {
    submittingOTP.value = false
  }
}

async function loadPublishers() {
  if (pending.value) return
  pending.value = true
  error.value = ''
  try {
    const response = await apiClient.get<APIEnvelope<Publisher[]>>(`${endpoint}/publishers`, { signal: lifecycle.signal, timeout: 38000 })
    publishers.value = response.data.data
    currentPublisher.value = null
    articles.value = null
    subscription.value = null
  } catch (cause) {
    if (!lifecycle.signal.aborted) error.value = formatAPIError(cause)
  } finally {
    pending.value = false
  }
}

async function loadArticles(publisher: Publisher) {
  if (pending.value) return
  pending.value = true
  error.value = ''
  currentPublisher.value = publisher
  articles.value = null
  subscription.value = null
  try {
    const response = await apiClient.post<APIEnvelope<Article[]>>(`${endpoint}/publishers/${encodeURIComponent(publisher.book_id)}/articles`, {}, { signal: lifecycle.signal, timeout: 38000 })
    articles.value = response.data.data
  } catch (cause) {
    if (!lifecycle.signal.aborted) error.value = formatAPIError(cause)
  } finally {
    pending.value = false
  }
}

async function subscribePublisher(publisher: Publisher) {
  if (pending.value) return
  pending.value = true
  error.value = ''
  try {
    const response = await apiClient.post<APIEnvelope<{ source_id: number; item_count: number }>>(`${endpoint}/publishers/${encodeURIComponent(publisher.book_id)}/subscribe`, {}, { signal: lifecycle.signal, timeout: 75000 })
    subscription.value = { id: response.data.data.source_id, name: publisher.title, itemCount: response.data.data.item_count }
    emit('subscribed')
  } catch (cause) {
    if (!lifecycle.signal.aborted) error.value = formatAPIError(cause)
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <section v-if="available" class="sources-panel wechat-rss" aria-label="微信公众号">
    <div class="wechat-rss__header">
      <div>
        <h2>微信公众号</h2>
        <p>{{ connected ? '选择公众号，将文章加入订阅信息流。' : '使用微信扫码授权，选择需要订阅的公众号。' }}</p>
      </div>
      <div class="sources-toolbar__group">
        <button class="sources-button" type="button" :disabled="pending" @click="openLogin">
          {{ connected ? '重新扫码' : '微信扫码登录' }}
        </button>
        <button v-if="connected" class="sources-button" type="button" :disabled="pending" @click="loadPublishers">
          查看公众号
        </button>
      </div>
    </div>
    <p v-if="error" role="alert" class="wechat-rss__error">{{ error }}</p>
    <p v-if="pending" role="status">正在读取微信公众号数据。</p>
    <p v-if="publishers?.length === 0">微信读书书架中没有公众号。</p>
    <ul v-else-if="publishers?.length" class="wechat-rss__list">
      <li v-for="publisher in publishers" :key="publisher.book_id">
        <button class="sources-button" type="button" :disabled="pending" @click="loadArticles(publisher)">
          {{ publisher.title }}
        </button>
      </li>
    </ul>
    <div v-if="currentPublisher && articles" class="wechat-rss__articles">
      <h3>{{ currentPublisher.title }}</h3>
      <button v-if="subscription" class="sources-button" type="button" @click="emit('openSource', { id: subscription.id, name: subscription.name, kind: 'subscriptions' })">查看订阅信息流</button>
      <button v-else class="sources-button" type="button" :disabled="pending" @click="subscribePublisher(currentPublisher)">加入订阅信息流</button>
      <p v-if="subscription" role="status">已加入订阅信息流，本次取得 {{ subscription.itemCount }} 篇文章。</p>
      <p v-if="articles.length === 0">当前未返回可阅读的文章。</p>
      <ul v-else class="wechat-rss__list">
        <li v-for="article in articles" :key="article.url">
          <a :href="article.url" target="_blank" rel="noopener noreferrer">{{ article.title }}</a>
        </li>
      </ul>
    </div>

    <dialog ref="dialog" class="wechat-rss__dialog" aria-labelledby="wechat-rss-login" @close="closeLogin">
      <h2 id="wechat-rss-login">微信读书扫码授权</h2>
      <p v-if="!challenge" role="status">{{ qrCode ? '用微信扫一扫，确认后会自动显示公众号。' : '正在获取微信授权二维码。' }}</p>
      <img v-if="qrCode && !challenge" :src="qrCode" alt="微信读书授权二维码" width="224" height="224" />
      <p v-if="challenge === 'OTP_EXPIRED'" role="alert">验证码已过期，请关闭后重新扫码。</p>
      <form v-else-if="challenge" method="post" :action="`${endpoint}/login/otp`" class="wechat-rss__otp" @submit.prevent="submitOTP">
        <label for="wechat-rss-otp">输入微信显示的验证码</label>
        <input id="wechat-rss-otp" v-model="otp" class="sources-input" name="otp" type="text" inputmode="numeric" autocomplete="one-time-code" enterkeyhint="done" pattern="[0-9]{4,8}" maxlength="8" required />
        <p v-if="otpError || challenge === 'OTP_NOT_MATCH'" role="alert">{{ otpError || '验证码错误，请重新输入。' }}</p>
        <p v-if="challenge === 'OTP_CHECKING'" role="status">正在验证。</p>
        <button class="sources-button" type="submit" :disabled="submittingOTP || challenge === 'OTP_CHECKING'">提交验证码</button>
      </form>
      <form method="dialog">
        <button class="sources-button" type="submit" autofocus>关闭</button>
      </form>
    </dialog>
  </section>
</template>

<style scoped>
.wechat-rss__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.wechat-rss p {
  margin: 0.375rem 0;
  color: var(--mf-text-muted);
}

.wechat-rss .wechat-rss__error {
  color: var(--mf-text);
}

.wechat-rss__list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0;
  list-style: none;
}

.wechat-rss__articles .wechat-rss__list {
  display: grid;
  overflow-wrap: anywhere;
}

.wechat-rss__articles a {
  color: var(--mf-text);
  text-underline-offset: 0.2em;
}

.wechat-rss__dialog {
  width: min(360px, calc(100% - 28px));
  padding: 20px;
  border: 1px solid var(--mf-border);
  border-radius: 16px;
  background: var(--mf-surface-raised);
  color: var(--mf-text);
}

.wechat-rss__dialog:modal {
  overscroll-behavior: contain;
}

.wechat-rss__dialog::backdrop {
  overflow: hidden;
  overscroll-behavior: contain;
  background: rgb(0 0 0 / 48%);
}

.wechat-rss__dialog img {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 16px auto;
  border-radius: 10px;
}

.wechat-rss__dialog form {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.wechat-rss__dialog .wechat-rss__otp {
  display: grid;
  gap: 8px;
  justify-content: stretch;
}

.wechat-rss__otp input {
  width: 100%;
  min-height: 48px;
  font-size: 1rem;
}
</style>
