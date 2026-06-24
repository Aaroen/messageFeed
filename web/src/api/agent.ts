import { apiClient } from '@/api/client'

interface APIEnvelope<T> {
  data: T
}

export interface AgentExternalAccount {
  id: number
  provider: string
  corp_id: string
  agent_id: string
  external_user_id: string
  display_name: string
  binding_status: string
  active_agent_session_id: number
  updated_at: string
}

export interface AgentSessionStats {
  transcript_count: number
  archive_index_count: number
  recall_count: number
  first_transcript_at?: string
  last_transcript_at?: string
}

export interface AgentSession {
  id: number
  external_account_id: number
  provider: string
  channel_session_key: string
  status: string
  title: string
  active_for_account: boolean
  context_initialized_at?: string
  context_rebuild_started_at?: string
  context_rebuild_finished_at?: string
  context_version: number
  transcript_count_indexed: number
  stats: AgentSessionStats
  started_at: string
  last_active_at: string
  created_at: string
  updated_at: string
}

export interface AgentSessionListResult {
  accounts: AgentExternalAccount[]
  sessions: AgentSession[]
}

export interface AgentTranscriptEntry {
  id: number
  turn_id: number
  role: string
  content: string
  created_at: string
}

export async function listAgentSessions() {
  const response = await apiClient.get<APIEnvelope<AgentSessionListResult>>('/api/v1/agent/sessions')
  return response.data.data
}

export async function createAgentSession(input: { external_account_id: number; title: string }) {
  const response = await apiClient.post<APIEnvelope<AgentSession>>('/api/v1/agent/sessions', input)
  return response.data.data
}

export async function selectAgentSession(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentExternalAccount>>(`/api/v1/agent/sessions/${id}/select`)
  return response.data.data
}

export async function rebuildAgentSessionContext(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentSessionStats>>(`/api/v1/agent/sessions/${id}/rebuild-context`)
  return response.data.data
}

export async function clearAgentSessionContext(id: number) {
  const response = await apiClient.delete<APIEnvelope<AgentSessionStats>>(`/api/v1/agent/sessions/${id}/context`)
  return response.data.data
}

export async function deleteAgentSession(id: number, input: { current_password: string }) {
  const response = await apiClient.delete<APIEnvelope<{ deleted: boolean }>>(`/api/v1/agent/sessions/${id}`, {
    data: input,
  })
  return response.data.data
}

export async function listAgentTranscripts(id: number, input: { before_entry_id?: number; limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<{ entries: AgentTranscriptEntry[] }>>(
    `/api/v1/agent/sessions/${id}/transcripts`,
    { params: input },
  )
  return response.data.data.entries
}
