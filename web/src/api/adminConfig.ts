import { apiClient } from '@/api/client'

interface APIEnvelope<T> {
  data: T
}

export interface AdminConfigRequirement {
  key: string
  configured: boolean
  secret: boolean
}

export interface AdminConfigRequirementCategory {
  name: string
  items: AdminConfigRequirement[]
}

export interface AdminConfigStatus {
  updated_at: string
  runtime: {
    environment: string
    service_name: string
    service_version: string
    app_node_id: string
    deployment_mode: string
    public_base_url: string
    bind_addr: string
  }
  database: {
    configured: boolean
  }
  auth: {
    local_login_enabled: boolean
    session_cookie: string
    session_secure: boolean
    session_ttl_seconds: number
    oauth_state_ttl_seconds: number
  }
  wechat_work: {
    enabled: boolean
    oauth_configured: boolean
    callback_configured: boolean
    sender_configured: boolean
    corp_id_masked?: string
    agent_id?: string
    callback_url?: string
    oauth_callback_url?: string
  }
  llm: {
    enabled: boolean
    client_ready: boolean
    provider?: string
    model?: string
    base_url?: string
    api_key_present: boolean
  }
  observability: {
    trace_enabled: boolean
    otlp_endpoint_set: boolean
    otlp_insecure: boolean
    trace_sample_ratio: number
    prometheus_endpoint: string
    grafana_url: string
    agent: {
      configured: boolean
      ready: boolean
      error_message?: string
      trace_event_rows: number
      recall_trace_rows: number
      embedding_trace_rows: number
      memory_topic_rows: number
      memory_chunk_rows: number
      memory_chunk_ready_rows: number
      memory_chunk_embedding_coverage_ratio: number
      pending_embedding_jobs: number
      running_embedding_jobs: number
      failed_embedding_jobs: number
      last_embedding_job_updated_at?: string
      last_embedding_error?: string
      embedding_worker_enabled: boolean
      embedding_worker_configured: boolean
      embedding_model_configured: boolean
    }
    metrics: Array<{
      name: string
      purpose: string
    }>
  }
  endpoints: {
    wechat_work_callback: string
    metrics: string
    health: string
    readiness: string
  }
  requirements: AdminConfigRequirementCategory[]
}

export interface AdminLLMTestResult {
  status: string
  provider: string
  model: string
  latency_ms: number
  response_text: string
  checked_at: string
}

export interface AdminWeChatWorkTestResult {
  status: string
  errcode: number
  errmsg?: string
  message_id?: string
  invalid_user?: string
  invalid_party?: string
  invalid_tag?: string
  unlicensed_user?: string
  latency_ms: number
  checked_at: string
}

export async function getAdminConfigStatus() {
  const response = await apiClient.get<APIEnvelope<AdminConfigStatus>>('/api/v1/admin/config')
  return response.data.data
}

export async function testAdminLLM(message: string) {
  const response = await apiClient.post<APIEnvelope<AdminLLMTestResult>>('/api/v1/admin/config/tests/llm', {
    message,
  })
  return response.data.data
}

export async function testAdminWeChatWork(input: { to_user: string; content: string }) {
  const response = await apiClient.post<APIEnvelope<AdminWeChatWorkTestResult>>(
    '/api/v1/admin/config/tests/wechat-work',
    input,
  )
  return response.data.data
}
