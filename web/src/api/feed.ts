import { AxiosError } from 'axios'

import { apiClient, formatAPIError } from '@/api/client'

const sourceFetchTimeoutMS = 25000
const activeSourcesFetchTimeoutMS = 60000
const activeSourcesFetchConcurrency = 4
const sourceFetchStatusTimeoutMS = 10000

interface APIEnvelope<T> {
  data: T
}

export interface Source {
  id: number
  name: string
  type: string
  url: string
  normalized_url: string
  status: 'active' | 'inactive'
  fetch_interval_seconds: number
  tags: string[]
  weight: number
  last_fetched_at?: string
  last_fetch_status?: string
  last_fetch_error?: string
  last_fetch_duration_ms?: number
  last_fetch_item_count?: number
  created_at: string
  updated_at: string
}

export interface SourceCatalogEntry {
  id: number
  source_key: string
  name: string
  site_url?: string
  feed_url: string
  normalized_url: string
  type: string
  category: string
  tags: string[]
  language: string
  country?: string
  official: boolean
  source_origin: string
  health_status: 'healthy' | 'degraded' | 'unreachable' | 'unknown'
  subscribed: boolean
  source_id?: number
  source_status?: 'active' | 'inactive'
}

export interface SourceCatalogList {
  entries: SourceCatalogEntry[]
  total: number
  limit: number
  offset: number
}

export interface FeedItem {
  id: number
  source_id: number
  source_name?: string
  title: string
  url: string
  summary?: string
  content_snippet?: string
  content_text?: string
  content_html?: string
  author?: string
  published_at?: string
  fetched_at: string
  is_read: boolean
  is_favorite: boolean
  is_hidden: boolean
}

export interface FeedItemList {
  items: FeedItem[]
  total: number
  limit: number
  offset: number
}

export interface ImportSourcesResult {
  requested_count: number
  success_count: number
  failure_count: number
  sources: Source[]
  errors: Array<{ reference: string; message: string }>
}

export interface FetchSourceResult {
  source: Source
  item_count: number
  created_count: number
  updated_count: number
}

export interface FetchSourcesResult {
  requested_count: number
  success_count: number
  failure_count: number
  async?: boolean
  queued_count?: number
  job_ids?: number[]
  sources: Source[]
  errors: Array<{ source_id: number; source_name: string; message: string }>
}

export interface FetchSourcesStatus {
  requested_count: number
  completed_count: number
  queued_count: number
  running_count: number
  success_count: number
  failure_count: number
  created_count: number
  updated_count: number
  updated_source_count: number
  done: boolean
  sources: Source[]
  errors: Array<{ source_id: number; source_name: string; message: string }>
}

export async function listSources() {
  const response = await apiClient.get<APIEnvelope<Source[]>>('/api/v1/sources')
  return response.data.data
}

export async function updateSourceStatus(id: number, status: Source['status']) {
  const response = await apiClient.patch<APIEnvelope<Source>>(`/api/v1/sources/${id}`, { status })
  return response.data.data
}

export async function fetchSource(id: number) {
  try {
    const response = await apiClient.post<APIEnvelope<FetchSourceResult>>(`/api/v1/sources/${id}/fetch`, undefined, {
      timeout: sourceFetchTimeoutMS,
    })
    return response.data.data
  } catch (err) {
    throw new Error(await fetchSourceFailureMessage(id, err))
  }
}

export async function fetchActiveSources() {
  try {
    const response = await apiClient.post<APIEnvelope<FetchSourcesResult>>('/api/v1/source-fetches', undefined, {
      timeout: activeSourcesFetchTimeoutMS,
    })
    return response.data.data
  } catch (err) {
    if (err instanceof AxiosError && err.response?.status === 404) {
      return fetchActiveSourcesWithSingleSourceFallback()
    }
    throw err
  }
}

export async function getSourceFetchStatus(jobIDs: number[]) {
  const response = await apiClient.get<APIEnvelope<FetchSourcesStatus>>('/api/v1/source-fetches/status', {
    params: { job_ids: jobIDs.join(',') },
    timeout: sourceFetchStatusTimeoutMS,
  })
  return response.data.data
}

async function fetchActiveSourcesWithSingleSourceFallback(): Promise<FetchSourcesResult> {
  const activeSources = (await listSources()).filter((source) => source.status === 'active')
  const result: FetchSourcesResult = {
    requested_count: activeSources.length,
    success_count: 0,
    failure_count: 0,
    async: false,
    queued_count: 0,
    job_ids: [],
    sources: [],
    errors: [],
  }
  let cursor = 0
  const workerCount = Math.min(activeSourcesFetchConcurrency, activeSources.length)
  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (cursor < activeSources.length) {
        const source = activeSources[cursor]
        cursor += 1
        try {
          const fetchResult = await fetchSource(source.id)
          result.success_count += 1
          result.sources.push(fetchResult.source)
        } catch (err) {
          result.failure_count += 1
          result.errors.push({
            source_id: source.id,
            source_name: source.name,
            message: formatAPIError(err),
          })
        }
      }
    }),
  )
  return result
}

async function fetchSourceFailureMessage(id: number, cause: unknown) {
  const apiMessage = formatAPIError(cause)
  try {
    const sources = await listSources()
    const source = sources.find((item) => item.id === id)
    const fetchError = source?.last_fetch_error?.trim()
    if (!fetchError) {
      return apiMessage
    }
    if (!apiMessage || apiMessage === fetchError) {
      return fetchError
    }
    return `${fetchError} (${apiMessage})`
  } catch {
    return apiMessage
  }
}

export async function listSourceCatalog(params: { category?: string; q?: string; limit?: number; offset?: number } = {}) {
  const response = await apiClient.get<APIEnvelope<SourceCatalogList>>('/api/v1/source-catalogs', {
    params,
  })
  return response.data.data
}

export async function importCatalogSources(catalogIDs: number[]) {
  const response = await apiClient.post<APIEnvelope<ImportSourcesResult>>('/api/v1/sources/import/catalog', {
    catalog_ids: catalogIDs,
  })
  return response.data.data
}

export async function importURLSources(urls: string[]) {
  const response = await apiClient.post<APIEnvelope<ImportSourcesResult>>('/api/v1/sources/import/urls', {
    urls,
  })
  return response.data.data
}

export async function importOPMLSource(file: File) {
  const form = new FormData()
  form.append('file', file)
  const response = await apiClient.post<APIEnvelope<ImportSourcesResult>>('/api/v1/sources/import/opml', form)
  return response.data.data
}

export async function getFeedItem(id: number) {
  const response = await apiClient.get<APIEnvelope<FeedItem>>(`/api/v1/items/${id}`)
  return response.data.data
}

export async function listTimelineItems(
  params: { limit?: number; offset?: number; source_id?: number; order?: 'asc' | 'desc' } = {},
) {
  const response = await apiClient.get<APIEnvelope<FeedItemList>>('/api/v1/feed/timeline', {
    params: {
      limit: 10,
      offset: 0,
      ...params,
    },
  })
  return response.data.data
}

export async function listRecommendationItems(
  params: { limit?: number; offset?: number; source_id?: number; order?: 'asc' | 'desc'; refresh?: boolean } = {},
) {
  const response = await apiClient.get<APIEnvelope<FeedItemList>>('/api/v1/feed/recommendations', {
    params: {
      limit: 10,
      offset: 0,
      ...params,
    },
  })
  return response.data.data
}
