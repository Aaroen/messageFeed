<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  changePassword,
  createInvite,
  deleteInvite,
  disableAuthBinding,
  getCurrentAuth,
  getWeChatWorkOAuthURL,
  listInvites,
  logout,
  type CurrentAuth,
  type InviteCode,
} from '@/api/auth'
import {
  getAdminConfigStatus,
  testAdminLLM,
  testAdminWeChatWork,
  type AdminConfigStatus,
  type AdminLLMTestResult,
  type AdminWeChatWorkTestResult,
} from '@/api/adminConfig'
import { formatAPIError } from '@/api/client'
import {
  readSourceTimelinePreloadSetting,
  updateSourceTimelinePreloadSetting,
} from '@/composables/useReaderSettingsSync'

const sourceTimelinePreload = ref(true)
const authStatus = ref<CurrentAuth | null>(null)
const authLoading = ref(false)
const authError = ref('')
const bindingActionID = ref<number | null>(null)
const logoutLoading = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const passwordChanging = ref(false)
const passwordChangeResult = ref('')
const invites = ref<InviteCode[]>([])
const invitesLoading = ref(false)
const inviteCreating = ref(false)
const inviteDeletingID = ref<number | null>(null)
const inviteTTLSeconds = ref(604800)
const generatedInviteCode = ref('')
const configStatus = ref<AdminConfigStatus | null>(null)
const configLoading = ref(false)
const configError = ref('')
const llmTestMessage = ref('')
const llmTesting = ref(false)
const llmTestResult = ref<AdminLLMTestResult | null>(null)
const llmTestError = ref('')
const wechatWorkToUser = ref('')
const wechatWorkContent = ref('messageFeed 管理后台测试消息')
const wechatWorkTesting = ref(false)
const wechatWorkTestResult = ref<AdminWeChatWorkTestResult | null>(null)
const wechatWorkTestError = ref('')

const statusCards = computed(() => {
  const status = configStatus.value
  const auth = authStatus.value
  return [
    {
      title: '登录',
      ready: Boolean(auth?.authenticated),
      value: auth?.user?.username || '未登录',
      meta: auth?.user ? `${auth.user.role} / ${auth.user.status}` : 'session 未建立',
    },
    {
      title: '企业微信',
      ready: Boolean(status?.wechat_work.callback_configured && status.wechat_work.sender_configured),
      value: status?.wechat_work.enabled ? '已启用' : '未启用',
      meta: status?.wechat_work.agent_id ? `AgentID ${status.wechat_work.agent_id}` : '未配置 AgentID',
    },
    {
      title: 'AI 提供商',
      ready: Boolean(status?.llm.enabled && status.llm.client_ready),
      value: status?.llm.provider || '未配置',
      meta: status?.llm.model || '未配置模型',
    },
    {
      title: '数据库',
      ready: Boolean(status?.database.configured),
      value: status?.database.configured ? '已配置' : '未配置',
      meta: status?.runtime.deployment_mode || 'unknown',
    },
    {
      title: '观测',
      ready: Boolean(status),
      value: status?.observability.trace_enabled ? 'Trace 已启用' : '指标可用',
      meta: status?.observability.trace_enabled ? `采样 ${status.observability.trace_sample_ratio}` : 'Prometheus / Grafana',
    },
  ]
})

function loadSettings() {
  sourceTimelinePreload.value = readSourceTimelinePreloadSetting()
}

async function loadAdminConfig() {
  configLoading.value = true
  configError.value = ''
  try {
    configStatus.value = await getAdminConfigStatus()
  } catch (error) {
    configError.value = formatAPIError(error)
  } finally {
    configLoading.value = false
  }
}

async function loadAuthStatus() {
  authLoading.value = true
  authError.value = ''
  try {
    authStatus.value = await getCurrentAuth()
    if (authStatus.value.user?.role === 'owner') {
      await loadInvites()
    }
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    authLoading.value = false
  }
}

async function loadInvites() {
  invitesLoading.value = true
  try {
    invites.value = await listInvites()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    invitesLoading.value = false
  }
}

async function refreshPage() {
  loadSettings()
  await Promise.all([loadAuthStatus(), loadAdminConfig()])
}

function updateSourceTimelinePreload() {
  updateSourceTimelinePreloadSetting(sourceTimelinePreload.value)
}

async function runLLMTest() {
  llmTesting.value = true
  llmTestError.value = ''
  llmTestResult.value = null
  try {
    llmTestResult.value = await testAdminLLM(llmTestMessage.value)
  } catch (error) {
    llmTestError.value = formatAPIError(error)
  } finally {
    llmTesting.value = false
  }
}

async function runWeChatWorkTest() {
  const toUser = wechatWorkToUser.value.trim()
  if (!toUser) {
    wechatWorkTestError.value = '请输入企业微信用户账号'
    return
  }
  wechatWorkTesting.value = true
  wechatWorkTestError.value = ''
  wechatWorkTestResult.value = null
  try {
    wechatWorkTestResult.value = await testAdminWeChatWork({
      to_user: toUser,
      content: wechatWorkContent.value,
    })
  } catch (error) {
    wechatWorkTestError.value = formatAPIError(error)
  } finally {
    wechatWorkTesting.value = false
  }
}

async function bindWeChatWork() {
  authError.value = ''
  try {
    const result = await getWeChatWorkOAuthURL({ redirect: '/settings', purpose: 'bind' })
    window.location.assign(result.url)
  } catch (error) {
    authError.value = formatAPIError(error)
  }
}

async function disableBinding(id: number) {
  bindingActionID.value = id
  authError.value = ''
  try {
    await disableAuthBinding(id)
    await loadAuthStatus()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    bindingActionID.value = null
  }
}

async function logoutCurrentUser() {
  logoutLoading.value = true
  authError.value = ''
  try {
    await logout()
    window.location.assign('/auth/login')
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    logoutLoading.value = false
  }
}

async function changeCurrentPassword() {
  passwordChanging.value = true
  authError.value = ''
  passwordChangeResult.value = ''
  try {
    await changePassword({
      current_password: currentPassword.value,
      new_password: newPassword.value,
    })
    currentPassword.value = ''
    newPassword.value = ''
    passwordChangeResult.value = '密码已更新'
    await loadAuthStatus()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    passwordChanging.value = false
  }
}

async function createInviteCode() {
  inviteCreating.value = true
  authError.value = ''
  generatedInviteCode.value = ''
  try {
    const result = await createInvite({
      role: 'user',
      ttl_seconds: inviteTTLSeconds.value,
    })
    generatedInviteCode.value = result.code
    await loadInvites()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    inviteCreating.value = false
  }
}

async function deleteInviteCode(id: number) {
  inviteDeletingID.value = id
  authError.value = ''
  try {
    await deleteInvite(id)
    await loadInvites()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    inviteDeletingID.value = null
  }
}

function statusLabel(value: boolean) {
  return value ? '正常' : '待配置'
}

function yesNo(value: boolean | undefined) {
  return value ? '是' : '否'
}

function formatTime(value: string | undefined) {
  if (!value) {
    return '尚未检查'
  }
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  void refreshPage().catch(() => undefined)
})

defineExpose({ refreshPage })
</script>

<template>
  <section class="settings-page">
    <div class="settings-panel settings-panel--wide">
      <div class="settings-panel__header">
        <div>
          <div class="settings-panel__title">系统配置</div>
          <div class="settings-panel__meta">
            {{ configStatus ? `更新于 ${formatTime(configStatus.updated_at)}` : '尚未加载' }}
          </div>
        </div>
        <button class="settings-action-button" type="button" :disabled="configLoading" @click="loadAdminConfig">
          {{ configLoading ? '刷新中' : '刷新' }}
        </button>
      </div>

      <div v-if="configError" class="settings-inline-alert settings-inline-alert--warning">
        {{ configError }}
      </div>

      <div class="settings-status-grid">
        <div v-for="card in statusCards" :key="card.title" class="settings-status-card">
          <div class="settings-status-card__top">
            <span>{{ card.title }}</span>
            <span class="settings-status-pill" :class="{ 'settings-status-pill--ok': card.ready }">
              {{ statusLabel(card.ready) }}
            </span>
          </div>
          <div class="settings-status-card__value">{{ card.value }}</div>
          <div class="settings-status-card__meta">{{ card.meta }}</div>
        </div>
      </div>
    </div>

    <div class="settings-config-grid">
      <section class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">用户登录</div>
            <div class="settings-panel__meta">{{ authStatus?.authenticated ? 'session 已建立' : 'session 未建立' }}</div>
          </div>
          <button class="settings-action-button" type="button" :disabled="logoutLoading" @click="logoutCurrentUser">
            {{ logoutLoading ? '退出中' : '退出' }}
          </button>
        </div>
        <div v-if="authError" class="settings-inline-alert settings-inline-alert--warning">
          {{ authError }}
        </div>
        <dl class="settings-description-list">
          <div>
            <dt>账号</dt>
            <dd>{{ authStatus?.user?.username || '未登录' }}</dd>
          </div>
          <div>
            <dt>角色</dt>
            <dd>{{ authStatus?.user?.role || '未知' }}</dd>
          </div>
          <div>
            <dt>密码登录</dt>
            <dd>{{ yesNo(authStatus?.login_enabled) }}</dd>
          </div>
          <div>
            <dt>数据库密码</dt>
            <dd>{{ yesNo(authStatus?.user?.password_configured) }}</dd>
          </div>
          <div v-if="configStatus">
            <dt>Cookie</dt>
            <dd>{{ configStatus.auth.session_cookie }} / {{ configStatus.auth.session_secure ? 'Secure' : 'Non-Secure' }}</dd>
          </div>
        </dl>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">账号密码</div>
            <div class="settings-panel__meta">更新当前登录账号的数据库密码</div>
          </div>
          <button
            class="settings-action-button"
            type="button"
            :disabled="passwordChanging || !currentPassword || !newPassword"
            @click="changeCurrentPassword"
          >
            {{ passwordChanging ? '更新中' : '更新' }}
          </button>
        </div>
        <div class="settings-form-grid">
          <label class="settings-field">
            <span>当前密码</span>
            <input v-model="currentPassword" class="settings-input" type="password" autocomplete="current-password" />
          </label>
          <label class="settings-field">
            <span>新密码</span>
            <input v-model="newPassword" class="settings-input" type="password" autocomplete="new-password" />
          </label>
        </div>
        <div v-if="passwordChangeResult" class="settings-inline-alert settings-inline-alert--success">
          {{ passwordChangeResult }}
        </div>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">企业微信绑定</div>
            <div class="settings-panel__meta">网页授权用于身份绑定和后续确认</div>
          </div>
          <button
            class="settings-action-button"
            type="button"
            :disabled="authLoading || !authStatus?.wechat_work_oauth_enabled"
            @click="bindWeChatWork"
          >
            绑定
          </button>
        </div>
        <div v-if="!authStatus?.wechat_work_oauth_enabled" class="settings-inline-alert settings-inline-alert--warning">
          企业微信网页授权尚未就绪
        </div>
        <div class="settings-bindings">
          <div v-for="binding in authStatus?.bindings || []" :key="binding.id" class="settings-binding-row">
            <div>
              <div class="settings-binding-row__title">{{ binding.external_user_id }}</div>
              <div class="settings-binding-row__meta">
                {{ binding.provider }} / {{ binding.binding_status }} / {{ binding.agent_id || '无 AgentID' }}
              </div>
            </div>
            <button
              class="settings-action-button"
              type="button"
              :disabled="bindingActionID === binding.id || binding.binding_status === 'disabled'"
              @click="disableBinding(binding.id)"
            >
              {{ bindingActionID === binding.id ? '处理中' : '禁用' }}
            </button>
          </div>
          <div v-if="!authStatus?.bindings?.length" class="settings-panel__meta">暂无绑定记录</div>
        </div>
      </section>

      <section v-if="authStatus?.user?.role === 'owner'" class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">邀请码</div>
            <div class="settings-panel__meta">生成后只显示一次明文邀请码</div>
          </div>
          <button class="settings-action-button" type="button" :disabled="inviteCreating" @click="createInviteCode">
            {{ inviteCreating ? '生成中' : '生成' }}
          </button>
        </div>
        <div class="settings-form-grid">
          <label class="settings-field">
            <span>有效期秒数</span>
            <input v-model.number="inviteTTLSeconds" class="settings-input" type="number" min="60" />
          </label>
        </div>
        <div v-if="generatedInviteCode" class="settings-inline-alert settings-inline-alert--success">
          {{ generatedInviteCode }}
        </div>
        <div class="settings-bindings">
          <div v-for="invite in invites" :key="invite.id" class="settings-binding-row">
            <div>
              <div class="settings-binding-row__title">
                {{ invite.status }} / {{ invite.use_count }} / {{ invite.max_uses }}
              </div>
              <div class="settings-binding-row__meta">
                {{ invite.role }} / 过期 {{ formatTime(invite.expires_at) }}
              </div>
            </div>
            <button
              class="settings-action-button"
              type="button"
              :disabled="inviteDeletingID === invite.id || invite.status === 'revoked'"
              @click="deleteInviteCode(invite.id)"
            >
              {{ inviteDeletingID === invite.id ? '删除中' : '删除' }}
            </button>
          </div>
          <div v-if="!invites.length" class="settings-panel__meta">
            {{ invitesLoading ? '加载中' : '暂无邀请码' }}
          </div>
        </div>
      </section>
    </div>

    <div v-if="configStatus" class="settings-config-grid">
      <section class="settings-panel">
        <div class="settings-panel__title">企业微信</div>
        <dl class="settings-description-list">
          <div>
            <dt>CorpID</dt>
            <dd>{{ configStatus.wechat_work.corp_id_masked || '未配置' }}</dd>
          </div>
          <div>
            <dt>回调</dt>
            <dd>{{ yesNo(configStatus.wechat_work.callback_configured) }}</dd>
          </div>
          <div>
            <dt>发送</dt>
            <dd>{{ yesNo(configStatus.wechat_work.sender_configured) }}</dd>
          </div>
          <div>
            <dt>回调地址</dt>
            <dd class="settings-mono">{{ configStatus.wechat_work.callback_url || '未配置' }}</dd>
          </div>
        </dl>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__title">AI 提供商</div>
        <dl class="settings-description-list">
          <div>
            <dt>Provider</dt>
            <dd>{{ configStatus.llm.provider || '未配置' }}</dd>
          </div>
          <div>
            <dt>Model</dt>
            <dd>{{ configStatus.llm.model || '未配置' }}</dd>
          </div>
          <div>
            <dt>Base URL</dt>
            <dd class="settings-mono">{{ configStatus.llm.base_url || '默认' }}</dd>
          </div>
          <div>
            <dt>API Key</dt>
            <dd>{{ yesNo(configStatus.llm.api_key_present) }}</dd>
          </div>
        </dl>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__title">运行端点</div>
        <dl class="settings-description-list">
          <div>
            <dt>健康</dt>
            <dd class="settings-mono">{{ configStatus.endpoints.health }}</dd>
          </div>
          <div>
            <dt>就绪</dt>
            <dd class="settings-mono">{{ configStatus.endpoints.readiness }}</dd>
          </div>
          <div>
            <dt>指标</dt>
            <dd class="settings-mono">{{ configStatus.endpoints.metrics }}</dd>
          </div>
          <div>
            <dt>Grafana</dt>
            <dd class="settings-mono">{{ configStatus.observability.grafana_url }}</dd>
          </div>
        </dl>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__title">环境项</div>
        <div class="settings-requirements">
          <div v-for="group in configStatus.requirements" :key="group.name" class="settings-requirements__group">
            <div class="settings-requirements__name">{{ group.name }}</div>
            <div class="settings-requirements__items">
              <span
                v-for="item in group.items"
                :key="item.key"
                class="settings-requirement-chip"
                :class="{ 'settings-requirement-chip--ok': item.configured }"
              >
                {{ item.key }} {{ item.configured ? '已配置' : '缺失' }}
              </span>
            </div>
          </div>
        </div>
      </section>
    </div>

    <div class="settings-config-grid">
      <section class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">AI 调用测试</div>
            <div class="settings-panel__meta">Provider 与模型配置验证</div>
          </div>
          <button
            class="settings-action-button"
            type="button"
            :disabled="llmTesting || !configStatus?.llm.client_ready"
            @click="runLLMTest"
          >
            {{ llmTesting ? '测试中' : '测试' }}
          </button>
        </div>
        <textarea v-model="llmTestMessage" class="settings-textarea" rows="3" placeholder="请回复 OK" />
        <div v-if="llmTestError" class="settings-inline-alert settings-inline-alert--warning">{{ llmTestError }}</div>
        <dl v-if="llmTestResult" class="settings-description-list">
          <div>
            <dt>结果</dt>
            <dd>{{ llmTestResult.status }} / {{ llmTestResult.latency_ms }}ms</dd>
          </div>
          <div>
            <dt>回复</dt>
            <dd>{{ llmTestResult.response_text }}</dd>
          </div>
        </dl>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__header">
          <div>
            <div class="settings-panel__title">企业微信发送测试</div>
            <div class="settings-panel__meta">发送器、token 与消息接口验证</div>
          </div>
          <button
            class="settings-action-button"
            type="button"
            :disabled="wechatWorkTesting || !configStatus?.wechat_work.sender_configured"
            @click="runWeChatWorkTest"
          >
            {{ wechatWorkTesting ? '发送中' : '发送' }}
          </button>
        </div>
        <div class="settings-form-grid">
          <label class="settings-field">
            <span>ToUser</span>
            <input v-model="wechatWorkToUser" class="settings-input" type="text" autocomplete="off" />
          </label>
          <label class="settings-field">
            <span>内容</span>
            <textarea v-model="wechatWorkContent" class="settings-textarea" rows="3" />
          </label>
        </div>
        <div v-if="wechatWorkTestError" class="settings-inline-alert settings-inline-alert--warning">
          {{ wechatWorkTestError }}
        </div>
        <dl v-if="wechatWorkTestResult" class="settings-description-list">
          <div>
            <dt>结果</dt>
            <dd>{{ wechatWorkTestResult.status }} / {{ wechatWorkTestResult.latency_ms }}ms</dd>
          </div>
          <div>
            <dt>消息 ID</dt>
            <dd>{{ wechatWorkTestResult.message_id || '无' }}</dd>
          </div>
        </dl>
      </section>
    </div>

    <article class="settings-row">
      <div>
        <div class="settings-row__title">源时间线预加载</div>
        <div class="settings-row__meta">详情页左右滑动时提前准备对应来源内容</div>
      </div>
      <label class="settings-switch">
        <input v-model="sourceTimelinePreload" type="checkbox" @change="updateSourceTimelinePreload" />
        <span />
      </label>
    </article>
  </section>
</template>
