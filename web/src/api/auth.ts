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

export async function getCurrentAuth() {
  const response = await apiClient.get<APIEnvelope<CurrentAuth>>('/api/v1/auth/me')
  return response.data.data
}

export async function login(input: { username: string; password: string }) {
  const response = await apiClient.post<APIEnvelope<LoginResult>>('/api/v1/auth/login', input)
  return response.data.data
}

export async function logout() {
  const response = await apiClient.post<APIEnvelope<{ logged_out: boolean }>>('/api/v1/auth/logout')
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
