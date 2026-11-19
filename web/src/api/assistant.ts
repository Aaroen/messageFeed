import { apiClient } from '@/api/client'
import type { AgentPlan, AgentSession, AgentTurn } from '@/api/agent'

export interface AgentActivity {
  id: number
  parent_run_id: number
  turn_id: number
  kind: 'controller' | 'model' | 'tool' | 'subagent'
  stage?: string
  capability?: string
  title?: string
  status: string
  started_at: string
  completed_at?: string
  updated_at: string
  results?: string[]
}

export interface AgentStreamItem {
  run_id: number
  turn_id: number
  item_id: string
  kind: string
  name: string
  content: string
  stage: string
  phase: string
  model_key: string
}

export interface AgentStreamEvent {
  id: number
  session_id: number
  turn_id: number
  run_id: number
  kind: string
  item_id: string
  name: string
  delta: string
  metadata: { replace?: boolean; stage?: string; phase?: string; model_key?: string }
}

export interface AgentWorkspace {
  session: AgentSession
  turns: AgentTurn[]
  runs: AgentActivity[]
  plans: AgentPlan[]
  items: AgentStreamItem[]
  cursor: number
  has_more: boolean
}

export async function getAgentWorkspace(sessionID: number, beforeTurnID = 0) {
  const response = await apiClient.get<{ data: AgentWorkspace }>(`/api/v1/agent/sessions/${sessionID}/workspace`, {
    params: { before_turn_id: beforeTurnID },
  })
  return response.data.data
}

export async function cancelAgentTurn(turnID: number) {
  await apiClient.post(`/api/v1/agent/turns/${turnID}/cancel`)
}

export async function getAgentActivity(runID: number) {
  const response = await apiClient.get<{ data: AgentActivity }>(`/api/v1/agent/runs/${runID}`, { params: { view: 'activity' } })
  return response.data.data
}
