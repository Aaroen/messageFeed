<script setup lang="ts">
import QRCode from 'qrcode'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  changePassword,
  createInvite,
  deactivateAccount,
  deactivateUser,
  deleteInvite,
  disableAuthBinding,
  getCurrentAuth,
  getUserContext,
  getWeChatWorkOAuthURL,
  listInvites,
  listSessions,
  listUsers,
  logout,
  revokeSession,
  restoreUser,
  updateUserProfile,
  type AdminUser,
  type CurrentAuth,
  type InviteCode,
  type UserContext,
  type UserProfile,
  type UserSession,
} from '@/api/auth'
import {
  getAdminConfigStatus,
  testAdminLLM,
  testAdminWeChatWork,
  type AdminConfigStatus,
  type AdminLLMTestResult,
  type AdminWeChatWorkTestResult,
} from '@/api/adminConfig'
import {
  clearAgentSessionContext,
  createAgentSession,
  deleteAgentSession,
  getAgentNotificationPreference,
  listAgentSessions,
  listAgentTranscripts,
  rebuildAgentSessionContext,
  selectAgentSession,
  updateAgentNotificationPreference,
  type AgentExternalAccount,
  type AgentNotificationPreference,
  type AgentSession,
  type AgentTranscriptEntry,
} from '@/api/agent'
import { formatAPIError } from '@/api/client'
import {
  readSourceTimelinePreloadSetting,
  updateSourceTimelinePreloadSetting,
} from '@/composables/useReaderSettingsSync'

const route = useRoute()
const router = useRouter()
const sourceTimelinePreload = ref(true)
const agentNotificationPreference = ref<AgentNotificationPreference>({
  process_notifications_enabled: true,
  final_reports_enabled: true,
  failure_notifications_enabled: true,
  recovery_notifications_enabled: true,
  max_concurrent_tasks: 2,
  max_queued_tasks: 20,
  auto_recovery_enabled: true,
  quality_handoff_threshold: 0.65,
  handoff_on_failure: true,
  handoff_on_permission: true,
  handoff_on_budget: true,
  capability_policy: {},
  daily_task_quota: 50,
  daily_external_call_quota: 200,
  daily_capability_call_quota: 500,
})
const capabilityPolicyText = ref('')
const agentNotificationPreferenceLoading = ref(false)
const agentNotificationPreferenceSaving = ref(false)
const agentNotificationPreferenceResult = ref('')
const authStatus = ref<CurrentAuth | null>(null)
const authLoading = ref(false)
const authError = ref('')
const bindingActionID = ref<number | null>(null)
const logoutLoading = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const passwordChanging = ref(false)
const passwordChangeResult = ref('')
const profileSaving = ref(false)
const profileSaveResult = ref('')
const profileForm = ref<UserProfile>({
  display_name: '',
  email: '',
  timezone: 'Asia/Shanghai',
  language: 'zh-CN',
  region: '',
  bio: '',
  focus_topics: [],
  blocked_topics: [],
  market_focus: [],
  instrument_focus: [],
  risk_preference: '',
  notification_quiet_hours: '',
  agent_notes: '',
  reply_style: 'plain_text_short',
})
const focusTopicsText = ref('')
const blockedTopicsText = ref('')
const marketFocusText = ref('')
const instrumentFocusText = ref('')
const sessions = ref<UserSession[]>([])
const sessionsLoading = ref(false)
const sessionRevokingID = ref<number | null>(null)
const deletePassword = ref('')
const accountDeleting = ref(false)
const accountDeleteResult = ref('')
const userContext = ref<UserContext | null>(null)
const agentAccounts = ref<AgentExternalAccount[]>([])
const agentSessions = ref<AgentSession[]>([])
const agentSessionsLoading = ref(false)
const selectedAgentSessionID = ref<number | null>(null)
const agentTranscripts = ref<AgentTranscriptEntry[]>([])
const agentTranscriptsLoading = ref(false)
const newAgentSessionAccountID = ref<number | null>(null)
const newAgentSessionTitle = ref('企业微信对话')
const agentSessionCreating = ref(false)
const agentSessionActionID = ref<number | null>(null)
const agentSessionDeleteID = ref<number | null>(null)
const agentSessionDeletePassword = ref('')
const agentSessionResult = ref('')
const users = ref<AdminUser[]>([])
const usersLoading = ref(false)
const userDeletingID = ref<number | null>(null)
const userRestoringID = ref<number | null>(null)
const invites = ref<InviteCode[]>([])
const invitesLoading = ref(false)
const inviteCreating = ref(false)
const inviteDeletingID = ref<number | null>(null)
const inviteTTLSeconds = ref(604800)
const generatedInviteCode = ref('')
const inviteCopyResult = ref('')
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
const wechatBindLoading = ref(false)
const wechatBindURL = ref('')
const wechatBindQRCode = ref('')
const wechatBindExpiresAt = ref('')
let inviteCopyTimer: number | undefined

const isOwner = computed(() => authStatus.value?.user?.role === 'owner')
const activeSettingsSection = computed(() => {
  const section = route.meta.settingsSection
  return typeof section === 'string' ? section : 'account'
})

const settingsSections = computed(() => {
  const sections = [
    { id: 'account', title: '账户', meta: '登录状态' },
    { id: 'profile', title: '资料', meta: isOwner.value ? '用户画像' : '基础信息' },
    { id: 'security', title: '安全', meta: '密码与会话' },
    { id: 'wechat', title: '企业微信', meta: '账号绑定' },
    { id: 'preferences', title: '偏好', meta: '阅读行为' },
  ]

  if (!isOwner.value) {
    return sections
  }

  return [
    ...sections,
    { id: 'overview', title: '系统概览', meta: '运行状态' },
    { id: 'invites', title: '邀请码', meta: '注册入口' },
    { id: 'users', title: '用户管理', meta: '账号列表' },
    { id: 'runtime', title: '运行配置', meta: '环境与端点' },
    { id: 'tests', title: '连通性测试', meta: 'AI 与企微' },
    { id: 'context', title: '上下文', meta: 'Agent 边界' },
  ]
})

const activeSectionTitle = computed(() => settingsSections.value.find((section) => section.id === activeSettingsSection.value)?.title || '设置')
const activeSectionMeta = computed(() => settingsSections.value.find((section) => section.id === activeSettingsSection.value)?.meta || '')

function isSettingsSectionActive(id: string) {
  return activeSettingsSection.value === id
}

function ensureActiveSettingsSection() {
  if (!settingsSections.value.some((section) => section.id === activeSettingsSection.value)) {
    void router.replace('/settings/account')
  }
}

function sectionNeedsAdminConfig(section = activeSettingsSection.value) {
  return section === 'overview' || section === 'wechat' || section === 'runtime' || section === 'tests'
}

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
    if (!authStatus.value.authenticated) {
      sessions.value = []
      userContext.value = null
      users.value = []
      invites.value = []
      return
    }
    if (authStatus.value.profile) {
      applyProfile(authStatus.value.profile)
    }
    const section = activeSettingsSection.value
    const tasks: Promise<void>[] = []
    if (section === 'security') {
      tasks.push(loadSessions())
    }
    if (authStatus.value.user?.role === 'owner') {
      if (section === 'context') {
        tasks.push(loadUserContext())
        tasks.push(loadAgentSessions())
      }
      if (section === 'invites') {
        tasks.push(loadInvites())
      }
      if (section === 'users') {
        tasks.push(loadUsers())
      }
    } else {
      userContext.value = null
      invites.value = []
      users.value = []
    }
    await Promise.all(tasks)
    ensureActiveSettingsSection()
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

async function loadSessions() {
  sessionsLoading.value = true
  try {
    sessions.value = await listSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    sessionsLoading.value = false
  }
}

async function loadUsers() {
  usersLoading.value = true
  try {
    users.value = await listUsers()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    usersLoading.value = false
  }
}

async function loadUserContext() {
  try {
    userContext.value = await getUserContext()
  } catch (error) {
    authError.value = formatAPIError(error)
  }
}

async function loadAgentSessions() {
  agentSessionsLoading.value = true
  authError.value = ''
  try {
    const result = await listAgentSessions()
    agentAccounts.value = result.accounts
    agentSessions.value = result.sessions
    if (!newAgentSessionAccountID.value && result.accounts.length) {
      newAgentSessionAccountID.value = result.accounts[0].id
    }
    if (!selectedAgentSessionID.value && result.sessions.length) {
      selectedAgentSessionID.value = result.sessions[0].id
    }
    if (selectedAgentSessionID.value) {
      await loadAgentTranscripts(selectedAgentSessionID.value)
    }
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionsLoading.value = false
  }
}

async function loadAgentTranscripts(sessionID: number) {
  selectedAgentSessionID.value = sessionID
  agentTranscriptsLoading.value = true
  authError.value = ''
  try {
    agentTranscripts.value = await listAgentTranscripts(sessionID, { limit: 80 })
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentTranscriptsLoading.value = false
  }
}

async function createNewAgentSession() {
  authError.value = ''
  if (!newAgentSessionAccountID.value) {
    authError.value = '请选择企业微信绑定账号'
    return
  }
  agentSessionCreating.value = true
  agentSessionResult.value = ''
  try {
    const created = await createAgentSession({
      external_account_id: newAgentSessionAccountID.value,
      title: newAgentSessionTitle.value,
    })
    selectedAgentSessionID.value = created.id
    agentSessionResult.value = 'session 已创建'
    await loadAgentSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionCreating.value = false
  }
}

async function selectCurrentAgentSession(id: number) {
  agentSessionActionID.value = id
  agentSessionResult.value = ''
  authError.value = ''
  try {
    await selectAgentSession(id)
    agentSessionResult.value = '当前企微 session 已切换'
    await loadAgentSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionActionID.value = null
  }
}

async function rebuildAgentContext(id: number) {
  agentSessionActionID.value = id
  agentSessionResult.value = ''
  authError.value = ''
  try {
    await rebuildAgentSessionContext(id)
    agentSessionResult.value = '上下文索引已重建'
    await loadAgentSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionActionID.value = null
  }
}

async function clearAgentContext(id: number) {
  agentSessionActionID.value = id
  agentSessionResult.value = ''
  authError.value = ''
  try {
    await clearAgentSessionContext(id)
    agentSessionResult.value = '上下文索引已清空，原文 transcript 保留'
    await loadAgentSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionActionID.value = null
  }
}

function prepareDeleteAgentSession(id: number) {
  agentSessionDeleteID.value = id
  agentSessionDeletePassword.value = ''
  agentSessionResult.value = ''
  authError.value = ''
}

async function confirmDeleteAgentSession() {
  if (!agentSessionDeleteID.value) {
    return
  }
  if (!agentSessionDeletePassword.value.trim()) {
    authError.value = '请输入当前登录密码'
    return
  }
  agentSessionActionID.value = agentSessionDeleteID.value
  agentSessionResult.value = ''
  authError.value = ''
  try {
    await deleteAgentSession(agentSessionDeleteID.value, { current_password: agentSessionDeletePassword.value })
    agentSessionResult.value = 'session 已删除'
    agentSessionDeleteID.value = null
    agentSessionDeletePassword.value = ''
    agentTranscripts.value = []
    selectedAgentSessionID.value = null
    await loadAgentSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    agentSessionActionID.value = null
  }
}

async function refreshPage() {
  loadSettings()
  await loadAuthStatus()
  if (isSettingsSectionActive('preferences')) {
    await loadAgentNotificationPreference()
  }
  if (isOwner.value && sectionNeedsAdminConfig()) {
    await loadAdminConfig()
    return
  }
  if (!isOwner.value) {
    configStatus.value = null
    configError.value = ''
  }
}

function updateSourceTimelinePreload() {
  updateSourceTimelinePreloadSetting(sourceTimelinePreload.value)
}

async function loadAgentNotificationPreference() {
  agentNotificationPreferenceLoading.value = true
  agentNotificationPreferenceResult.value = ''
  try {
    agentNotificationPreference.value = await getAgentNotificationPreference()
    capabilityPolicyText.value = formatCapabilityPolicy(agentNotificationPreference.value.capability_policy)
  } catch (error) {
    agentNotificationPreferenceResult.value = formatAPIError(error)
  } finally {
    agentNotificationPreferenceLoading.value = false
  }
}

async function saveAgentNotificationPreference() {
  agentNotificationPreferenceSaving.value = true
  agentNotificationPreferenceResult.value = ''
  try {
    agentNotificationPreference.value = await updateAgentNotificationPreference({
      ...agentNotificationPreference.value,
      capability_policy: parseCapabilityPolicy(capabilityPolicyText.value),
    })
    capabilityPolicyText.value = formatCapabilityPolicy(agentNotificationPreference.value.capability_policy)
    agentNotificationPreferenceResult.value = 'Agent 通知偏好已保存'
  } catch (error) {
    agentNotificationPreferenceResult.value = formatAPIError(error)
  } finally {
    agentNotificationPreferenceSaving.value = false
  }
}

function formatCapabilityPolicy(policy: Record<string, string> | undefined) {
  if (!policy) {
    return ''
  }
  return Object.entries(policy)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

function parseCapabilityPolicy(text: string) {
  const policy: Record<string, string> = {}
  text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .forEach((line) => {
      const separator = line.includes('=') ? '=' : ':'
      const [rawKey, rawValue] = line.split(separator, 2)
      const key = rawKey?.trim()
      const value = rawValue?.trim()
      if (key && value) {
        policy[key] = value
      }
    })
  return policy
}

function applyProfile(profile: UserProfile) {
  profileForm.value = {
    display_name: profile.display_name || '',
    email: profile.email || '',
    timezone: profile.timezone || 'Asia/Shanghai',
    language: profile.language || 'zh-CN',
    region: profile.region || '',
    bio: profile.bio || '',
    focus_topics: profile.focus_topics || [],
    blocked_topics: profile.blocked_topics || [],
    market_focus: profile.market_focus || [],
    instrument_focus: profile.instrument_focus || [],
    risk_preference: profile.risk_preference || '',
    notification_quiet_hours: profile.notification_quiet_hours || '',
    agent_notes: profile.agent_notes || '',
    reply_style: profile.reply_style || 'plain_text_short',
    updated_at: profile.updated_at,
  }
  focusTopicsText.value = profileForm.value.focus_topics.join('、')
  blockedTopicsText.value = profileForm.value.blocked_topics.join('、')
  marketFocusText.value = profileForm.value.market_focus.join('、')
  instrumentFocusText.value = profileForm.value.instrument_focus.join('、')
}

function parseList(value: string) {
  return value
    .split(/[,\n，、]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

async function saveProfile() {
  profileSaving.value = true
  authError.value = ''
  profileSaveResult.value = ''
  try {
    const saved = await updateUserProfile({
      ...profileForm.value,
      focus_topics: parseList(focusTopicsText.value),
      blocked_topics: parseList(blockedTopicsText.value),
      market_focus: parseList(marketFocusText.value),
      instrument_focus: parseList(instrumentFocusText.value),
    })
    applyProfile(saved)
    profileSaveResult.value = '资料已更新'
    await loadAuthStatus()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    profileSaving.value = false
  }
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
  wechatBindLoading.value = true
  authError.value = ''
  wechatBindURL.value = ''
  wechatBindQRCode.value = ''
  wechatBindExpiresAt.value = ''
  try {
    const result = await getWeChatWorkOAuthURL({ redirect: '/settings', purpose: 'bind' })
    wechatBindURL.value = result.url
    wechatBindExpiresAt.value = result.expires_at
    wechatBindQRCode.value = await QRCode.toDataURL(result.url, {
      width: 224,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: {
        dark: '#111827',
        light: '#ffffff',
      },
    })
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    wechatBindLoading.value = false
  }
}

function openWeChatWorkBindURL() {
  if (!wechatBindURL.value) {
    return
  }
  window.location.assign(wechatBindURL.value)
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

async function revokeUserSession(id: number, current: boolean) {
  sessionRevokingID.value = id
  authError.value = ''
  try {
    await revokeSession(id)
    if (current) {
      window.location.assign('/auth/login')
      return
    }
    await loadSessions()
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    sessionRevokingID.value = null
  }
}

async function deleteCurrentAccount() {
  accountDeleting.value = true
  authError.value = ''
  accountDeleteResult.value = ''
  try {
    await deactivateAccount({ current_password: deletePassword.value })
    accountDeleteResult.value = '账号已注销'
    window.location.assign('/auth/login')
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    accountDeleting.value = false
  }
}

async function deleteUser(id: number) {
  userDeletingID.value = id
  authError.value = ''
  try {
    const deleted = await deactivateUser(id)
    replaceUserInList(deleted)
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    userDeletingID.value = null
  }
}

async function restoreDeletedUser(id: number) {
  userRestoringID.value = id
  authError.value = ''
  try {
    const restored = await restoreUser(id)
    replaceUserInList(restored)
  } catch (error) {
    authError.value = formatAPIError(error)
  } finally {
    userRestoringID.value = null
  }
}

function replaceUserInList(nextUser: AdminUser) {
  users.value = users.value.map((user) => (user.id === nextUser.id ? nextUser : user))
}

async function createInviteCode() {
  inviteCreating.value = true
  authError.value = ''
  generatedInviteCode.value = ''
  inviteCopyResult.value = ''
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

async function copyGeneratedInviteCode() {
  const code = generatedInviteCode.value.trim()
  if (!code) {
    return
  }
  try {
    await navigator.clipboard.writeText(code)
    inviteCopyResult.value = '已复制'
  } catch {
    inviteCopyResult.value = '复制失败'
  }
  if (inviteCopyTimer) {
    window.clearTimeout(inviteCopyTimer)
  }
  inviteCopyTimer = window.setTimeout(() => {
    inviteCopyResult.value = ''
  }, 1800)
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

function parseOAuthRedirectURL(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.searchParams.get('redirect_uri') || ''
  } catch {
    return ''
  }
}

function localURLWarning(value: string) {
  const target = parseOAuthRedirectURL(value) || value
  if (!target) {
    return ''
  }
  try {
    const parsed = new URL(target)
    const host = parsed.hostname.toLowerCase()
    if (host === 'localhost' || host === '127.0.0.1' || host === '0.0.0.0' || host.endsWith('.local')) {
      return `当前回调地址为 ${target}，手机扫码无法回到该本机地址`
    }
  } catch {
    return ''
  }
  return ''
}

const wechatBindWarning = computed(() => localURLWarning(wechatBindURL.value || configStatus.value?.wechat_work.oauth_callback_url || ''))

function userStatusLabel(status: string) {
  if (status === 'deleted') {
    return 'delete'
  }
  return status || 'unknown'
}

function userMeta(user: AdminUser) {
  const base = `${user.role} / ${user.display_name || '无显示名'} / ${user.email || '无邮箱'}`
  if (user.status !== 'deleted') {
    return base
  }
  return `${base} / 删除于 ${formatTime(user.deleted_at)} / 恢复截止 ${formatTime(user.restore_expires_at)}`
}

onMounted(() => {
  void refreshPage().catch(() => undefined)
})

watch(activeSettingsSection, () => {
  void refreshPage().catch(() => undefined)
})

defineExpose({ refreshPage })
</script>

<template>
  <section class="settings-page">
      <main class="settings-content">
        <header class="settings-content__header">
          <div>
            <h2>{{ activeSectionTitle }}</h2>
            <p>{{ activeSectionMeta }}</p>
          </div>
          <button class="settings-action-button" type="button" :disabled="authLoading || configLoading" @click="refreshPage">
            {{ authLoading || configLoading ? '刷新中' : '刷新' }}
          </button>
        </header>

        <div v-if="authError" class="settings-inline-alert settings-inline-alert--warning">
          {{ authError }}
        </div>

        <section v-if="isSettingsSectionActive('account')" class="settings-panel">
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">用户登录</div>
              <div class="settings-panel__meta">{{ authStatus?.authenticated ? 'session 已建立' : 'session 未建立' }}</div>
            </div>
            <button class="settings-action-button" type="button" :disabled="logoutLoading" @click="logoutCurrentUser">
              {{ logoutLoading ? '退出中' : '退出' }}
            </button>
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

        <section v-if="isSettingsSectionActive('profile')" class="settings-panel">
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">用户资料</div>
              <div class="settings-panel__meta">{{ isOwner ? '供通知和后续 Agent 上下文使用' : '基础账号信息' }}</div>
            </div>
            <button class="settings-action-button" type="button" :disabled="profileSaving" @click="saveProfile">
              {{ profileSaving ? '保存中' : '保存' }}
            </button>
          </div>
          <div class="settings-form-grid settings-form-grid--split">
            <label class="settings-field">
              <span>显示名</span>
              <input v-model="profileForm.display_name" class="settings-input" type="text" autocomplete="name" />
            </label>
            <label class="settings-field">
              <span>邮箱</span>
              <input v-model="profileForm.email" class="settings-input" type="email" autocomplete="email" />
            </label>
            <label class="settings-field">
              <span>时区</span>
              <input v-model="profileForm.timezone" class="settings-input" type="text" autocomplete="off" />
            </label>
            <label class="settings-field">
              <span>语言</span>
              <input v-model="profileForm.language" class="settings-input" type="text" autocomplete="off" />
            </label>
            <label class="settings-field">
              <span>地区</span>
              <input v-model="profileForm.region" class="settings-input" type="text" autocomplete="off" />
            </label>
            <label v-if="isOwner" class="settings-field">
              <span>回复风格</span>
              <input v-model="profileForm.reply_style" class="settings-input" type="text" autocomplete="off" />
            </label>
          </div>
          <label v-if="isOwner" class="settings-field">
            <span>个人画像</span>
            <textarea v-model="profileForm.bio" class="settings-textarea" rows="3" />
          </label>
          <div v-if="isOwner" class="settings-form-grid settings-form-grid--split">
            <label class="settings-field">
              <span>关注主题</span>
              <textarea v-model="focusTopicsText" class="settings-textarea" rows="2" />
            </label>
            <label class="settings-field">
              <span>屏蔽主题</span>
              <textarea v-model="blockedTopicsText" class="settings-textarea" rows="2" />
            </label>
            <label class="settings-field">
              <span>关注市场</span>
              <textarea v-model="marketFocusText" class="settings-textarea" rows="2" />
            </label>
            <label class="settings-field">
              <span>关注标的</span>
              <textarea v-model="instrumentFocusText" class="settings-textarea" rows="2" />
            </label>
            <label class="settings-field">
              <span>风险偏好</span>
              <input v-model="profileForm.risk_preference" class="settings-input" type="text" autocomplete="off" />
            </label>
            <label class="settings-field">
              <span>免打扰时间</span>
              <input v-model="profileForm.notification_quiet_hours" class="settings-input" type="text" autocomplete="off" />
            </label>
          </div>
          <label v-if="isOwner" class="settings-field">
            <span>Agent 备注</span>
            <textarea v-model="profileForm.agent_notes" class="settings-textarea" rows="3" />
          </label>
          <div v-if="profileSaveResult" class="settings-inline-alert settings-inline-alert--success">
            {{ profileSaveResult }}
          </div>
        </section>

        <div v-if="isSettingsSectionActive('security')" class="settings-section-stack">
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
            <div class="settings-form-grid settings-form-grid--split">
              <label class="settings-field">
                <span>当前密码</span>
                <input v-model="currentPassword" class="settings-input" type="password" autocomplete="current-password" />
              </label>
              <label class="settings-field">
                <span>新密码</span>
                <input v-model="newPassword" class="settings-input" type="password" minlength="6" autocomplete="new-password" />
              </label>
            </div>
            <div v-if="passwordChangeResult" class="settings-inline-alert settings-inline-alert--success">
              {{ passwordChangeResult }}
            </div>
          </section>

          <section class="settings-panel">
            <div class="settings-panel__header">
              <div>
                <div class="settings-panel__title">登录会话</div>
                <div class="settings-panel__meta">{{ sessionsLoading ? '加载中' : `${sessions.length} 个有效会话` }}</div>
              </div>
              <button class="settings-action-button" type="button" :disabled="sessionsLoading" @click="loadSessions">
                刷新
              </button>
            </div>
            <div class="settings-bindings">
              <div v-for="session in sessions" :key="session.id" class="settings-binding-row">
                <div>
                  <div class="settings-binding-row__title">
                    {{ session.current ? '当前会话' : `会话 ${session.id}` }}
                  </div>
                  <div class="settings-binding-row__meta">
                    {{ session.ip_address || '无 IP' }} / 最近 {{ formatTime(session.last_seen_at) }}
                  </div>
                </div>
                <button
                  class="settings-action-button"
                  type="button"
                  :disabled="sessionRevokingID === session.id"
                  @click="revokeUserSession(session.id, session.current)"
                >
                  {{ sessionRevokingID === session.id ? '撤销中' : '撤销' }}
                </button>
              </div>
              <div v-if="!sessions.length" class="settings-panel__meta">暂无有效会话</div>
            </div>
          </section>

          <section v-if="authStatus?.user?.role !== 'owner'" class="settings-panel">
            <div class="settings-panel__header">
              <div>
                <div class="settings-panel__title">账号注销</div>
                <div class="settings-panel__meta">注销后账号会被软删除并退出所有会话</div>
              </div>
              <button
                class="settings-action-button"
                type="button"
                :disabled="accountDeleting || !deletePassword"
                @click="deleteCurrentAccount"
              >
                {{ accountDeleting ? '注销中' : '注销' }}
              </button>
            </div>
            <label class="settings-field">
              <span>当前密码</span>
              <input v-model="deletePassword" class="settings-input" type="password" autocomplete="current-password" />
            </label>
            <div v-if="accountDeleteResult" class="settings-inline-alert settings-inline-alert--success">
              {{ accountDeleteResult }}
            </div>
          </section>
        </div>

        <section v-if="isSettingsSectionActive('wechat')" class="settings-panel">
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">企业微信绑定</div>
              <div class="settings-panel__meta">网页授权用于身份绑定和后续确认</div>
            </div>
            <button
              class="settings-action-button"
              type="button"
              :disabled="authLoading || wechatBindLoading || !authStatus?.wechat_work_oauth_enabled"
              @click="bindWeChatWork"
            >
              {{ wechatBindLoading ? '生成中' : '生成二维码' }}
            </button>
          </div>
          <div v-if="!authStatus?.wechat_work_oauth_enabled" class="settings-inline-alert settings-inline-alert--warning">
            企业微信网页授权尚未就绪
          </div>
          <dl v-if="isOwner && configStatus" class="settings-description-list">
            <div>
              <dt>CorpID</dt>
              <dd>{{ configStatus.wechat_work.corp_id_masked || '未配置' }}</dd>
            </div>
            <div>
              <dt>AgentID</dt>
              <dd>{{ configStatus.wechat_work.agent_id || '未配置' }}</dd>
            </div>
            <div>
              <dt>OAuth</dt>
              <dd>{{ statusLabel(configStatus.wechat_work.oauth_configured) }}</dd>
            </div>
            <div>
              <dt>网页授权回调</dt>
              <dd class="settings-mono">{{ configStatus.wechat_work.oauth_callback_url || '未配置' }}</dd>
            </div>
            <div>
              <dt>消息回调</dt>
              <dd class="settings-mono">{{ configStatus.wechat_work.callback_url || '未配置' }}</dd>
            </div>
          </dl>
          <div v-if="wechatBindWarning" class="settings-inline-alert settings-inline-alert--warning">
            {{ wechatBindWarning }}
          </div>
          <div v-if="wechatBindQRCode" class="settings-wechat-qr">
            <img class="settings-wechat-qr__image" :src="wechatBindQRCode" alt="企业微信绑定二维码" />
            <div class="settings-wechat-qr__body">
              <div class="settings-binding-row__title">企业微信扫码绑定</div>
              <div class="settings-binding-row__meta">有效至 {{ formatTime(wechatBindExpiresAt) }}</div>
              <div class="settings-wechat-qr__actions">
                <button class="settings-action-button" type="button" @click="openWeChatWorkBindURL">
                  当前浏览器打开
                </button>
                <button class="settings-action-button" type="button" :disabled="authLoading" @click="refreshPage">
                  刷新绑定状态
                </button>
              </div>
            </div>
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

        <section v-if="isSettingsSectionActive('preferences')" class="settings-panel">
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">源时间线预加载</div>
              <div class="settings-panel__meta">详情页左右滑动时提前准备对应来源内容</div>
            </div>
            <label class="settings-switch">
              <input v-model="sourceTimelinePreload" type="checkbox" @change="updateSourceTimelinePreload" />
              <span />
            </label>
          </div>
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">Agent 通知</div>
              <div class="settings-panel__meta">
                {{ agentNotificationPreferenceLoading ? '加载中' : '企业微信过程通知与最终汇报' }}
              </div>
            </div>
            <button
              class="settings-action-button"
              type="button"
              :disabled="agentNotificationPreferenceSaving"
              @click="saveAgentNotificationPreference"
            >
              {{ agentNotificationPreferenceSaving ? '保存中' : '保存' }}
            </button>
          </div>
          <div class="settings-form-grid">
            <div class="settings-switch-row">
              <span>过程通知</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.process_notifications_enabled" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>最终汇报</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.final_reports_enabled" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>失败通知</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.failure_notifications_enabled" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>恢复通知</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.recovery_notifications_enabled" type="checkbox" />
                <span />
              </label>
            </div>
            <label class="settings-field">
              <span>最大并发任务</span>
              <input v-model.number="agentNotificationPreference.max_concurrent_tasks" type="number" min="1" max="20" />
            </label>
            <label class="settings-field">
              <span>最大排队任务</span>
              <input v-model.number="agentNotificationPreference.max_queued_tasks" type="number" min="1" max="200" />
            </label>
            <label class="settings-field">
              <span>低质量接管阈值</span>
              <input
                v-model.number="agentNotificationPreference.quality_handoff_threshold"
                type="number"
                min="0.1"
                max="1"
                step="0.05"
              />
            </label>
            <label class="settings-field">
              <span>每日任务配额</span>
              <input v-model.number="agentNotificationPreference.daily_task_quota" type="number" min="1" max="10000" />
            </label>
            <label class="settings-field">
              <span>每日外部访问配额</span>
              <input v-model.number="agentNotificationPreference.daily_external_call_quota" type="number" min="1" max="100000" />
            </label>
            <label class="settings-field">
              <span>每日能力调用配额</span>
              <input v-model.number="agentNotificationPreference.daily_capability_call_quota" type="number" min="1" max="100000" />
            </label>
            <div class="settings-switch-row">
              <span>自动恢复</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.auto_recovery_enabled" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>失败接管</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.handoff_on_failure" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>权限接管</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.handoff_on_permission" type="checkbox" />
                <span />
              </label>
            </div>
            <div class="settings-switch-row">
              <span>预算接管</span>
              <label class="settings-switch">
                <input v-model="agentNotificationPreference.handoff_on_budget" type="checkbox" />
                <span />
              </label>
            </div>
            <label class="settings-field settings-field--wide">
              <span>Capability 策略</span>
              <textarea
                v-model="capabilityPolicyText"
                class="settings-textarea"
                rows="5"
                placeholder="web.search=confirm&#10;repo.*=reject"
              />
            </label>
          </div>
          <div v-if="agentNotificationPreferenceResult" class="settings-inline-alert">
            {{ agentNotificationPreferenceResult }}
          </div>
        </section>

        <section v-if="isSettingsSectionActive('overview') && isOwner" class="settings-panel settings-panel--wide">
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
        </section>

        <section v-if="isSettingsSectionActive('invites') && isOwner" class="settings-panel">
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
          <button
            v-if="generatedInviteCode"
            class="settings-copy-alert"
            type="button"
            @click="copyGeneratedInviteCode"
          >
            <span class="settings-copy-alert__code">{{ generatedInviteCode }}</span>
            <span>{{ inviteCopyResult || '点击复制' }}</span>
          </button>
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
                :disabled="inviteDeletingID === invite.id"
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

        <section v-if="isSettingsSectionActive('users') && isOwner" class="settings-panel">
          <div class="settings-panel__header">
            <div>
              <div class="settings-panel__title">用户列表</div>
              <div class="settings-panel__meta">{{ usersLoading ? '加载中' : `${users.length} 个用户` }}</div>
            </div>
            <button class="settings-action-button" type="button" :disabled="usersLoading" @click="loadUsers">
              刷新
            </button>
          </div>
          <div class="settings-bindings">
            <div v-for="user in users" :key="user.id" class="settings-binding-row">
              <div>
                <div class="settings-binding-row__title">{{ user.username }} / {{ userStatusLabel(user.status) }}</div>
                <div class="settings-binding-row__meta">
                  {{ userMeta(user) }}
                </div>
              </div>
              <button
                v-if="user.role !== 'owner' && user.status !== 'deleted'"
                class="settings-action-button"
                type="button"
                :disabled="userDeletingID === user.id"
                @click="deleteUser(user.id)"
              >
                {{ userDeletingID === user.id ? '删除中' : '删除' }}
              </button>
              <button
                v-else-if="user.role !== 'owner' && user.restorable"
                class="settings-action-button"
                type="button"
                :disabled="userRestoringID === user.id"
                @click="restoreDeletedUser(user.id)"
              >
                {{ userRestoringID === user.id ? '恢复中' : '恢复' }}
              </button>
            </div>
            <div v-if="!users.length" class="settings-panel__meta">暂无用户</div>
          </div>
        </section>

        <div v-if="isSettingsSectionActive('runtime') && isOwner && configStatus" class="settings-config-grid">
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

        <div v-if="isSettingsSectionActive('tests') && isOwner" class="settings-config-grid">
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

        <div v-if="isSettingsSectionActive('context') && isOwner" class="settings-section-stack">
          <section v-if="userContext" class="settings-panel settings-panel--wide">
            <div class="settings-panel__title">用户上下文</div>
            <dl class="settings-description-list">
              <div>
                <dt>User ID</dt>
                <dd>{{ userContext.data_scope.user_id }}</dd>
              </div>
              <div>
                <dt>可读范围</dt>
                <dd>{{ userContext.data_scope.readable_domains.join(' / ') }}</dd>
              </div>
              <div>
                <dt>渠道</dt>
                <dd>{{ userContext.data_scope.external_providers.join(' / ') || '暂无' }}</dd>
              </div>
            </dl>
            <textarea class="settings-textarea" rows="5" :value="userContext.prompt.plain_text" readonly />
          </section>

          <section class="settings-panel settings-panel--wide">
            <div class="settings-panel__header">
              <div>
                <div class="settings-panel__title">企微 Agent Session</div>
                <div class="settings-panel__meta">
                  {{ agentSessionsLoading ? '加载中' : `${agentSessions.length} 个 session` }}
                </div>
              </div>
              <button class="settings-action-button" type="button" :disabled="agentSessionsLoading" @click="loadAgentSessions">
                刷新
              </button>
            </div>

            <div class="settings-form-grid settings-form-grid--split">
              <label class="settings-field">
                <span class="settings-field__label">绑定账号</span>
                <select v-model.number="newAgentSessionAccountID" class="settings-input">
                  <option v-for="account in agentAccounts" :key="account.id" :value="account.id">
                    {{ account.external_user_id }} / Agent {{ account.agent_id }}
                  </option>
                </select>
              </label>
              <label class="settings-field">
                <span class="settings-field__label">新 session 标题</span>
                <input v-model="newAgentSessionTitle" class="settings-input" type="text" autocomplete="off" />
              </label>
            </div>
            <button
              class="settings-action-button"
              type="button"
              :disabled="agentSessionCreating || !agentAccounts.length"
              @click="createNewAgentSession"
            >
              {{ agentSessionCreating ? '创建中' : '新建 session' }}
            </button>

            <div v-if="agentSessionResult" class="settings-inline-alert settings-inline-alert--success">
              {{ agentSessionResult }}
            </div>

            <div class="settings-bindings">
              <div v-for="session in agentSessions" :key="session.id" class="settings-binding-row">
                <div>
                  <div class="settings-binding-row__title">
                    {{ session.title || `Session ${session.id}` }}
                    <span v-if="session.active_for_account" class="settings-status-pill settings-status-pill--ok">当前</span>
                  </div>
                  <div class="settings-binding-row__meta">
                    ID {{ session.id }} / 原文 {{ session.stats.transcript_count }} / 索引
                    {{ session.stats.archive_index_count }} / 召回 {{ session.stats.recall_count }}
                  </div>
                  <div class="settings-binding-row__meta">
                    首条 {{ formatTime(session.stats.first_transcript_at) }} / 最新 {{ formatTime(session.stats.last_transcript_at) }}
                  </div>
                </div>
                <div class="settings-binding-row__actions">
                  <button class="settings-action-button" type="button" @click="loadAgentTranscripts(session.id)">查看</button>
                  <button
                    class="settings-action-button"
                    type="button"
                    :disabled="agentSessionActionID === session.id || session.active_for_account"
                    @click="selectCurrentAgentSession(session.id)"
                  >
                    设为当前
                  </button>
                  <button
                    class="settings-action-button"
                    type="button"
                    :disabled="agentSessionActionID === session.id"
                    @click="rebuildAgentContext(session.id)"
                  >
                    重建索引
                  </button>
                  <button
                    class="settings-action-button"
                    type="button"
                    :disabled="agentSessionActionID === session.id"
                    @click="clearAgentContext(session.id)"
                  >
                    清空上下文
                  </button>
                  <button
                    class="settings-action-button"
                    type="button"
                    :disabled="agentSessionActionID === session.id"
                    @click="prepareDeleteAgentSession(session.id)"
                  >
                    删除
                  </button>
                </div>
              </div>
              <div v-if="!agentSessions.length" class="settings-panel__meta">暂无 agent session</div>
            </div>

            <div v-if="agentSessionDeleteID" class="settings-inline-alert settings-inline-alert--warning">
              <label class="settings-field">
                <span class="settings-field__label">输入当前登录密码后删除 Session {{ agentSessionDeleteID }}</span>
                <input
                  v-model="agentSessionDeletePassword"
                  class="settings-input"
                  type="password"
                  autocomplete="current-password"
                />
              </label>
              <button
                class="settings-action-button"
                type="button"
                :disabled="agentSessionActionID === agentSessionDeleteID"
                @click="confirmDeleteAgentSession"
              >
                确认删除整个 session
              </button>
            </div>
          </section>

          <section class="settings-panel settings-panel--wide">
            <div class="settings-panel__header">
              <div>
                <div class="settings-panel__title">Session 原文记录</div>
                <div class="settings-panel__meta">
                  {{ selectedAgentSessionID ? `Session ${selectedAgentSessionID}` : '未选择 session' }}
                </div>
              </div>
            </div>
            <div class="settings-transcript-list">
              <div v-if="agentTranscriptsLoading" class="settings-panel__meta">加载中</div>
              <div v-for="entry in agentTranscripts" :key="entry.id" class="settings-transcript-row">
                <div class="settings-binding-row__meta">
                  #{{ entry.id }} / turn {{ entry.turn_id }} / {{ entry.role }} / {{ formatTime(entry.created_at) }}
                </div>
                <div>{{ entry.content }}</div>
              </div>
              <div v-if="!agentTranscriptsLoading && !agentTranscripts.length" class="settings-panel__meta">
                暂无 transcript
              </div>
            </div>
          </section>
        </div>
      </main>
  </section>
</template>
