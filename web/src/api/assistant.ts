import { apiClient } from '@/api/client'
import type { AgentPlan, AgentRun, AgentSession, AgentTurn } from '@/api/agent'

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
  runs: AgentRun[]
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
