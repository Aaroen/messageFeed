import { apiClient } from '@/api/client'

export interface AgentModel {
  config_id: number
  provider_name: string
  model: string
  display_name: string
  enabled: boolean
  guest_enabled: boolean
}

export interface ProviderModelCatalog {
  models: { id: string; owned_by?: string }[]
  saved: AgentModel[]
}

export function modelSelectionKey(model: Pick<AgentModel, 'config_id' | 'model'>) {
  return JSON.stringify([model.config_id, model.model])
}

export async function listAssistantModels(guest = false) {
  const response = await apiClient.get<{ data: AgentModel[] }>(guest ? '/api/v1/public/agent/models' : '/api/v1/agent/models')
  return response.data.data
}

export async function getModelReasoningLevels(model: AgentModel, guest: boolean, signal: AbortSignal) {
  const route = guest ? '/api/v1/public/agent/models/reasoning-levels' : '/api/v1/agent/models/reasoning-levels'
  const response = await apiClient.get<{ data: string[] }>(route, { params: { config_id: model.config_id, model: model.model }, signal })
  return response.data.data
}

export async function discoverProviderModels(configID: number) {
  const response = await apiClient.get<{ data: ProviderModelCatalog }>(`/api/v1/agent/llm-provider-configs/${configID}/models`, { timeout: 35000 })
  return response.data.data
}

export async function listSavedProviderModels(configID: number) {
  const response = await apiClient.get<{ data: AgentModel[] }>(`/api/v1/agent/llm-provider-configs/${configID}/models`, { params: { saved_only: true } })
  return response.data.data
}

export async function saveProviderModels(configID: number, models: string[], guestModels: string[], displayNames: Record<string, string>) {
  await apiClient.put(`/api/v1/agent/llm-provider-configs/${configID}/models`, { models, guest_models: guestModels, display_names: displayNames })
}

export interface GuestChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface GuestChatEvent {
  item_id?: string
  kind?: string
  delta?: string
  replace?: boolean
  content?: string
  model?: string
  message?: string
  conversation_id?: number
  turn_id?: number
}

export interface GuestHistoryTurn {
  id: number
  config_id: number
  conversation_id: number
  input_text: string
  output_text: string
  reasoning_content: string
  model: string
  status: string
  created_at: string
}

export interface GuestHistory {
  turns: GuestHistoryTurn[]
  retention_days: number
  client_daily_limit: number
  total_daily_limit: number
  daily_reset_time_zone: string
}

export async function getGuestHistory() {
  const response = await apiClient.get<{ data: GuestHistory }>('/api/v1/public/agent/history')
  return response.data.data
}

export async function streamGuestChat(model: AgentModel, messages: GuestChatMessage[], signal: AbortSignal, onEvent: (kind: string, event: GuestChatEvent) => void, conversationID = 0, reasoningEffort = '') {
  const response = await fetch('/api/v1/public/agent/chat', {
    method: 'POST', credentials: 'same-origin', signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ config_id: model.config_id, model: model.model, messages, conversation_id: conversationID, reasoning_effort: reasoningEffort || undefined }),
  })
  if (!response.ok) {
    const error = await response.json() as { message?: string }
    throw new Error(error.message || `请求失败（HTTP ${response.status}）`)
  }
  if (!response.body || !response.headers.get('content-type')?.includes('text/event-stream')) throw new Error('模型返回的流式响应无效')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  try {
    while (true) {
      const chunk = await reader.read()
      buffer += decoder.decode(chunk.value, { stream: !chunk.done })
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const frame = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        const kind = frame.split('\n').find((line) => line.startsWith('event: '))?.slice(7)
        const data = frame.split('\n').filter((line) => line.startsWith('data: ')).map((line) => line.slice(6)).join('\n')
        if (kind && data) {
          const event = JSON.parse(data) as GuestChatEvent
          if (kind === 'failed') throw new Error(event.message || '模型未完成回答')
          if (kind === 'completed') completed = true
          onEvent(kind, event)
        }
        boundary = buffer.indexOf('\n\n')
      }
      if (chunk.done) break
    }
    if (!completed) throw new Error('连接已中断，模型未完成回答')
  } finally {
    reader.releaseLock()
  }
}
