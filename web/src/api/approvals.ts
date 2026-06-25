import { apiClient } from '@/api/client'

interface APIEnvelope<T> {
  data: T
}

export interface AgentApprovalDetail {
  id: number
  plan_id?: number
  status: string
  channel: string
  expires_at: string
  decided_at?: string
  metadata: Record<string, unknown>
}

export async function getAgentApproval(token: string) {
  const response = await apiClient.get<APIEnvelope<AgentApprovalDetail>>(`/api/v1/agent/approvals/${token}`)
  return response.data.data
}

export async function approveAgentApproval(token: string) {
  const response = await apiClient.post<APIEnvelope<AgentApprovalDetail>>(`/api/v1/agent/approvals/${token}/approve`)
  return response.data.data
}

export async function rejectAgentApproval(token: string) {
  const response = await apiClient.post<APIEnvelope<AgentApprovalDetail>>(`/api/v1/agent/approvals/${token}/reject`)
  return response.data.data
}

export async function approveAgentApprovalRecord(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentApprovalDetail>>(`/api/v1/agent/approval-records/${id}/approve`)
  return response.data.data
}

export async function rejectAgentApprovalRecord(id: number) {
  const response = await apiClient.post<APIEnvelope<AgentApprovalDetail>>(`/api/v1/agent/approval-records/${id}/reject`)
  return response.data.data
}
