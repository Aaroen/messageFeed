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

export interface CurrentAuth {
  authenticated: boolean
  login_enabled: boolean
  registration_enabled: boolean
  wechat_work_oauth_enabled: boolean
  user?: AuthUser
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

export async function getCurrentAuth() {
  const response = await apiClient.get<APIEnvelope<CurrentAuth>>('/api/v1/auth/me')
  return response.data.data
}

export async function login(input: { username: string; password: string }) {
  const response = await apiClient.post<APIEnvelope<LoginResult>>('/api/v1/auth/login', input)
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
  return response.data.data
}

export async function logout() {
  const response = await apiClient.post<APIEnvelope<{ logged_out: boolean }>>('/api/v1/auth/logout')
  return response.data.data
}

export async function changePassword(input: { current_password: string; new_password: string }) {
  const response = await apiClient.post<APIEnvelope<AuthUser>>('/api/v1/auth/password', input)
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
