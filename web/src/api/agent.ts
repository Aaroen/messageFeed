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

export interface AgentRun {
  id: number
  parent_run_id: number
  session_id: number
  turn_id: number
  role: string
  status: string
  task_packet: Record<string, unknown>
  capability_scope: string[]
  model_key: string
  context_budget: Record<string, unknown>
  context_trace_ref: string
  result_ref: string
  error_message: string
  trace_id: string
  started_at: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface AgentPlanStep {
  id: number
  plan_id: number
  step_order: number
  status: string
  capability_key: string
  capability_scope: string[]
  title: string
  input_summary: string
  output_summary: string
  expected_output: string
  failure_strategy: string
  executor_run_id: number
  observation_ref: string
  artifact_refs: string[]
  error_message: string
  started_at?: string
  completed_at?: string
  created_at: string
  updated_at: string
}

export interface AgentPlanApproval {
  id: number
  plan_id?: number
  channel: string
  status: string
  expires_at: string
  decided_at?: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface AgentPlan {
  id: number
  user_id: number
  session_id: number
  turn_id: number
  controller_run_id: number
  status: string
  goal: string
  summary: string
  impact_summary: string
  risk_level: string
  confirmation_policy: string
  allowed_scopes: string[]
  dedupe_key: string
  policy_decision: string
  policy_reason: string
  expires_at?: string
  approved_at?: string
  rejected_at?: string
  completed_at?: string
  failed_at?: string
  error_message: string
  metadata: Record<string, unknown>
  created_at: string
  updated_at: string
  steps?: AgentPlanStep[]
  approvals?: AgentPlanApproval[]
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

export async function listAgentPlans(input: { session_id?: number; turn_id?: number; limit?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<{ plans: AgentPlan[] }>>('/api/v1/agent/plans', { params: input })
  return response.data.data.plans
}

export async function getAgentPlan(id: number) {
  const response = await apiClient.get<APIEnvelope<{ plan: AgentPlan }>>(`/api/v1/agent/plans/${id}`)
  return response.data.data.plan
}

export async function listAgentRunsByTurn(turnID: number) {
  const response = await apiClient.get<APIEnvelope<{ runs: AgentRun[] }>>(`/api/v1/agent/turns/${turnID}/runs`)
  return response.data.data.runs
}

export async function getAgentRun(id: number) {
  const response = await apiClient.get<APIEnvelope<{ run: AgentRun }>>(`/api/v1/agent/runs/${id}`)
  return response.data.data.run
}
