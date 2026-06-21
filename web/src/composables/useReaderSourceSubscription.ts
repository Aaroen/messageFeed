import { computed } from 'vue'

import { formatAPIError } from '@/api/client'
import {
  fetchSource,
  importCatalogSources,
  listSourceCatalog,
  listSources,
  updateSourceStatus,
  type Source,
  type SourceCatalogEntry,
} from '@/api/feed'
import type { ReaderSource } from '@/composables/useReaderSession'

type SourceNotice = {
  type: 'success' | 'warning'
  message: string
}

type FetchNowResult = {
  success: boolean
  error?: string
}

type ReadableRef<T> = {
  readonly value: T
}

type ReaderSourceSubscriptionOptions = {
  sourceCatalogEntry: ReadableRef<SourceCatalogEntry | null>
  sourceSubscription: ReadableRef<Source | null>
  sourceSubscriptionLoading: ReadableRef<boolean>
  sourceNotice: ReadableRef<SourceNotice | null>
  getReaderSource: () => ReaderSource | null
  setSourceCatalogEntry: (entry: SourceCatalogEntry | null) => void
  setSourceSubscription: (source: Source | null) => void
  setSourceSubscriptionLoading: (loading: boolean) => void
  setSourceNotice: (notice: SourceNotice | null) => void
}

export function useReaderSourceSubscription(options: ReaderSourceSubscriptionOptions) {
  let sourceNoticeTimer = 0
  const sourceToggleLabel = computed(() => {
    if (options.sourceSubscriptionLoading.value) {
      return '处理中'
    }
    return options.sourceSubscription.value?.status === 'active' ? '关闭' : '开启'
  })
  const sourceToggleActive = computed(() => options.sourceSubscription.value?.status === 'active')
  const sourceToggleDisabled = computed(() => options.sourceSubscriptionLoading.value)

  function clearNoticeTimer() {
    if (typeof window === 'undefined') {
      return
    }
    window.clearTimeout(sourceNoticeTimer)
  }

  function showSourceNotice(type: SourceNotice['type'], message: string, durationMS?: number) {
    const normalized = message.trim()
    if (!normalized) {
      clearNoticeTimer()
      options.setSourceNotice(null)
      return
    }
    options.setSourceNotice({ type, message: normalized })
    clearNoticeTimer()
    const duration = durationMS ?? (type === 'success' ? 1000 : 3000)
    if (duration > 0 && typeof window !== 'undefined') {
      sourceNoticeTimer = window.setTimeout(() => {
        options.setSourceNotice(null)
      }, duration)
    }
  }

  function resetSourceSubscriptionState() {
    clearNoticeTimer()
    options.setSourceCatalogEntry(null)
    options.setSourceSubscription(null)
    options.setSourceNotice(null)
  }

  async function fetchNow(source: Source): Promise<FetchNowResult> {
    showSourceNotice('success', `抓取中：正在抓取 ${source.name} 的最新内容`, 0)
    try {
      await fetchSource(source.id)
      return { success: true }
    } catch (err) {
      return { success: false, error: formatAPIError(err) }
    }
  }

  async function loadSourceReaderSubscription(source: ReaderSource) {
    options.setSourceSubscriptionLoading(true)
    try {
      const [sources, catalogResult] = await Promise.all([
        listSources(),
        listSourceCatalog({ limit: 200, offset: 0 }),
      ])
      const directSource = sources.find((item) => item.id === source.id)
      const catalogEntry =
        catalogResult.entries.find((entry) => entry.id === source.id) ??
        catalogResult.entries.find((entry) => entry.source_id === source.id) ??
        catalogResult.entries.find((entry) => entry.name === source.name)
      const catalogSource = catalogEntry?.source_id
        ? sources.find((item) => item.id === catalogEntry.source_id)
        : undefined

      options.setSourceCatalogEntry(catalogEntry ?? null)
      options.setSourceSubscription(directSource ?? catalogSource ?? null)
    } catch (err) {
      showSourceNotice('warning', `加载失败：来源状态未同步。详细原因：${formatAPIError(err)}`)
    } finally {
      options.setSourceSubscriptionLoading(false)
    }
  }

  async function toggleSourceReaderSubscription() {
    const readerSource = options.getReaderSource()
    if (!readerSource || options.sourceSubscriptionLoading.value) {
      return
    }

    options.setSourceSubscriptionLoading(true)
    try {
      if (options.sourceSubscription.value) {
        const nextStatus = options.sourceSubscription.value.status === 'active' ? 'inactive' : 'active'
        const updated = await updateSourceStatus(options.sourceSubscription.value.id, nextStatus)
        options.setSourceSubscription(updated)
        let fetchResult: FetchNowResult = { success: true }
        if (updated.status === 'active') {
          fetchResult = await fetchNow(updated)
        }
        if (updated.status === 'active' && !fetchResult.success) {
          showSourceNotice(
            'warning',
            `${updated.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`,
          )
        } else {
          showSourceNotice('success', `${updated.name} 已${updated.status === 'active' ? '开启并抓取最新内容' : '关闭'}`)
        }
        await loadSourceReaderSubscription(readerSource)
        return
      }

      if (!options.sourceCatalogEntry.value) {
        showSourceNotice('warning', '该来源不在官方目录中，暂不支持直接开启')
        return
      }

      const result = await importCatalogSources([options.sourceCatalogEntry.value.id])
      const imported = result.sources[0]
      let fetchResult: FetchNowResult = { success: true }
      if (imported) {
        options.setSourceSubscription(imported)
        fetchResult = await fetchNow(imported)
      }
      if (!fetchResult.success) {
        showSourceNotice(
          'warning',
          `${options.sourceCatalogEntry.value.name} 已开启，但抓取失败。详细原因：${
            fetchResult.error || '服务未返回具体错误原因'
          }`,
        )
      } else {
        showSourceNotice('success', `${options.sourceCatalogEntry.value.name} 已开启并抓取最新内容`)
      }
      await loadSourceReaderSubscription(readerSource)
    } catch (err) {
      showSourceNotice('warning', `操作失败：来源订阅状态未更新。详细原因：${formatAPIError(err)}`)
    } finally {
      options.setSourceSubscriptionLoading(false)
    }
  }

  return {
    sourceToggleLabel,
    sourceToggleActive,
    sourceToggleDisabled,
    clearNoticeTimer,
    showSourceNotice,
    resetSourceSubscriptionState,
    loadSourceReaderSubscription,
    toggleSourceReaderSubscription,
  }
}
