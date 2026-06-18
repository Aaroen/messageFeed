import { defineStore } from 'pinia'

import { apiClient, formatAPIError } from '@/api/client'

interface HealthResponse {
  status: string
}

interface ReadinessCheck {
  name: string
  status: string
  message: string
}

interface ReadinessResponse {
  status?: string
  ready?: boolean
  checks?: ReadinessCheck[]
  checked_at?: string
}

interface RuntimeNodeResponse {
  node_id?: string
  deployment_mode?: string
  public_base_url?: string
  bind_addr?: string
  [key: string]: unknown
}

interface EndpointState<T> {
  data: T | null
  error: string
}

export const useAppStatusStore = defineStore('appStatus', {
  state: () => ({
    loading: false,
    lastCheckedAt: '',
    health: { data: null, error: '' } as EndpointState<HealthResponse>,
    readiness: { data: null, error: '' } as EndpointState<ReadinessResponse>,
    node: { data: null, error: '' } as EndpointState<RuntimeNodeResponse>,
  }),
  getters: {
    apiReachable(state) {
      return Boolean(state.health.data?.status === 'ok' && state.node.data)
    },
    hasError(state) {
      return Boolean(state.health.error || state.readiness.error || state.node.error)
    },
  },
  actions: {
    async refresh() {
      this.loading = true

      const [health, readiness, node] = await Promise.allSettled([
        apiClient.get<HealthResponse>('/healthz'),
        apiClient.get<ReadinessResponse>('/readyz'),
        apiClient.get<RuntimeNodeResponse>('/api/runtime/node'),
      ])

      this.health =
        health.status === 'fulfilled'
          ? { data: health.value.data, error: '' }
          : { data: null, error: formatAPIError(health.reason) }

      this.readiness =
        readiness.status === 'fulfilled'
          ? { data: readiness.value.data, error: '' }
          : { data: null, error: formatAPIError(readiness.reason) }

      this.node =
        node.status === 'fulfilled'
          ? { data: node.value.data, error: '' }
          : { data: null, error: formatAPIError(node.reason) }

      this.lastCheckedAt = new Date().toLocaleString('zh-CN', { hour12: false })
      this.loading = false
    },
  },
})
