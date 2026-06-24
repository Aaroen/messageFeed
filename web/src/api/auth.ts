import { apiClient } from '@/api/client'

interface APIEnvelope<T> {
  data: T
}

export interface AuthUser {
  id: number
  username: string
  display_name: string
  role: string
  status: string
  password_configured: boolean
}

export interface AuthBinding {
  id: number
  provider: string
  corp_id_masked: string
  agent_id: string
  external_user_id: string
  display_name: string
  binding_status: string
  verified_at?: string
  last_seen_at?: string
}

export interface UserProfile {
  display_name: string
  email: string
  timezone: string
  language: string
  region: string
  bio: string
  focus_topics: string[]
  blocked_topics: string[]
  market_focus: string[]
  instrument_focus: string[]
  risk_preference: string
  notification_quiet_hours: string
  agent_notes: string
  reply_style: string
  updated_at?: string
}

export interface CurrentAuth {
  authenticated: boolean
  login_enabled: boolean
  registration_enabled: boolean
  wechat_work_oauth_enabled: boolean
  user?: AuthUser
  profile?: UserProfile
  bindings: AuthBinding[]
}

export interface LoginResult {
  user: AuthUser
  expires_at: string
}

export interface WeChatWorkOAuthURLResult {
  url: string
  expires_at: string
}

export interface InviteCode {
  id: number
  role: string
  max_uses: number
  use_count: number
  status: string
  expires_at?: string
  created_at: string
  updated_at: string
}

export interface CreateInviteResult {
  invite: InviteCode
  code: string
}

export interface UserSession {
  id: number
  expires_at: string
  ip_address: string
  created_at: string
  updated_at: string
  last_seen_at: string
  current: boolean
}

export interface AdminUser {
  id: number
  username: string
  display_name: string
  email: string
  role: string
  status: string
  password_configured: boolean
  deleted_at?: string
  restore_expires_at?: string
  restorable: boolean
  created_at: string
  updated_at: string
}

export interface UserContext {
  user: AuthUser
  profile: UserProfile
  bindings: AuthBinding[]
  data_scope: {
    user_id: number
    readable_domains: string[]
    writable_domains: string[]
    external_providers: string[]
  }
  prompt: {
    plain_text: string
  }
}

const currentAuthCacheTTL = 5000
let currentAuthCache: { value: CurrentAuth; expiresAt: number } | null = null
let currentAuthRequest: Promise<CurrentAuth> | null = null

export function invalidateCurrentAuthCache() {
  currentAuthCache = null
  currentAuthRequest = null
}

export async function getCurrentAuth(input: { force?: boolean } = {}) {
  const now = Date.now()
  if (!input.force && currentAuthCache && currentAuthCache.expiresAt > now) {
    return currentAuthCache.value
  }
  if (!input.force && currentAuthRequest) {
    return currentAuthRequest
  }

  currentAuthRequest = apiClient
    .get<APIEnvelope<CurrentAuth>>('/api/v1/auth/me')
    .then((nextResponse) => {
      currentAuthCache = {
        value: nextResponse.data.data,
        expiresAt: Date.now() + currentAuthCacheTTL,
      }
      return nextResponse.data.data
    })
    .finally(() => {
      currentAuthRequest = null
    })
  return currentAuthRequest
}

export async function login(input: { username: string; password: string }) {
  const response = await apiClient.post<APIEnvelope<LoginResult>>('/api/v1/auth/login', input)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function registerWithInvite(input: {
  invite_code: string
  username: string
  password: string
  display_name?: string
  email?: string
}) {
  const response = await apiClient.post<APIEnvelope<LoginResult>>('/api/v1/auth/register', input)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function logout() {
  const response = await apiClient.post<APIEnvelope<{ logged_out: boolean }>>('/api/v1/auth/logout')
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function changePassword(input: { current_password: string; new_password: string }) {
  const response = await apiClient.post<APIEnvelope<AuthUser>>('/api/v1/auth/password', input)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function getUserProfile() {
  const response = await apiClient.get<APIEnvelope<UserProfile>>('/api/v1/auth/profile')
  return response.data.data
}

export async function updateUserProfile(input: UserProfile) {
  const response = await apiClient.patch<APIEnvelope<UserProfile>>('/api/v1/auth/profile', input)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function listSessions() {
  const response = await apiClient.get<APIEnvelope<UserSession[]>>('/api/v1/auth/sessions')
  return response.data.data
}

export async function revokeSession(id: number) {
  const response = await apiClient.delete<APIEnvelope<{ revoked: boolean }>>(`/api/v1/auth/sessions/${id}`)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function deactivateAccount(input: { current_password: string }) {
  const response = await apiClient.delete<APIEnvelope<{ deleted: boolean }>>('/api/v1/auth/account', { data: input })
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function getUserContext() {
  const response = await apiClient.get<APIEnvelope<UserContext>>('/api/v1/auth/context')
  return response.data.data
}

export async function getWeChatWorkOAuthURL(input: { redirect: string; purpose?: string }) {
  const response = await apiClient.get<APIEnvelope<WeChatWorkOAuthURLResult>>('/api/v1/auth/wechat-work/oauth-url', {
    params: {
      redirect: input.redirect,
      purpose: input.purpose || 'bind',
    },
  })
  return response.data.data
}

export async function listAuthBindings() {
  const response = await apiClient.get<APIEnvelope<AuthBinding[]>>('/api/v1/auth/bindings')
  return response.data.data
}

export async function disableAuthBinding(id: number) {
  const response = await apiClient.post<APIEnvelope<AuthBinding>>(`/api/v1/auth/bindings/${id}/disable`)
  invalidateCurrentAuthCache()
  return response.data.data
}

export async function listInvites() {
  const response = await apiClient.get<APIEnvelope<InviteCode[]>>('/api/v1/admin/invites')
  return response.data.data
}

export async function createInvite(input: { role: string; ttl_seconds: number }) {
  const response = await apiClient.post<APIEnvelope<CreateInviteResult>>('/api/v1/admin/invites', input)
  return response.data.data
}

export async function deleteInvite(id: number) {
  const response = await apiClient.delete<APIEnvelope<InviteCode>>(`/api/v1/admin/invites/${id}`)
  return response.data.data
}

export async function listUsers() {
  const response = await apiClient.get<APIEnvelope<AdminUser[]>>('/api/v1/admin/users')
  return response.data.data
}

export async function deactivateUser(id: number) {
  const response = await apiClient.delete<APIEnvelope<AdminUser>>(`/api/v1/admin/users/${id}`)
  return response.data.data
}

export async function restoreUser(id: number) {
  const response = await apiClient.post<APIEnvelope<AdminUser>>(`/api/v1/admin/users/${id}/restore`)
  return response.data.data
}
