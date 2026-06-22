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
import { useMotionTimings } from '@/composables/useMotionTimings'
import type { ReaderSource } from '@/composables/useReaderSession'
import { useTimedNotice } from '@/composables/useTimedNotice'

type SourceNotice = {
  type: 'running' | 'success' | 'warning'
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
  canShowNotice?: () => boolean
  onSubscriptionContentChanged?: (sourceID: number) => void
}

export function useReaderSourceSubscription(options: ReaderSourceSubscriptionOptions) {
  const motionTimings = useMotionTimings()
  const sourceNoticeRuntime = useTimedNotice<SourceNotice['type']>({
    duration: motionTimings.noticeDuration,
    canShow: () => options.canShowNotice?.() !== false,
    setNotice: options.setSourceNotice,
  })
  let sourceRequestToken = 0
  const sourceToggleLabel = computed(() => {
    if (options.sourceSubscriptionLoading.value) {
      return '处理中'
    }
    return options.sourceSubscription.value?.status === 'active' ? '关闭' : '开启'
  })
  const sourceToggleActive = computed(() => options.sourceSubscription.value?.status === 'active')
  const sourceToggleDisabled = computed(() => options.sourceSubscriptionLoading.value)

  function nextSourceRequestToken() {
    sourceRequestToken += 1
    return sourceRequestToken
  }

  function invalidateSourceRequests() {
    sourceRequestToken += 1
  }

  function readerSourceMatches(source: ReaderSource | null) {
    const current = options.getReaderSource()
    return Boolean(current && source && current.id === source.id && current.kind === source.kind)
  }

  function sourceRequestIsCurrent(token: number, source: ReaderSource | null) {
    return token === sourceRequestToken && readerSourceMatches(source)
  }

  function showSourceNotice(type: SourceNotice['type'], message: string, durationMS?: number) {
    sourceNoticeRuntime.show(type, message, durationMS)
  }

  function resetSourceSubscriptionState() {
    invalidateSourceRequests()
    sourceNoticeRuntime.clear()
    options.setSourceCatalogEntry(null)
    options.setSourceSubscription(null)
    options.setSourceSubscriptionLoading(false)
  }

  function clearSourceSubscriptionRuntime() {
    invalidateSourceRequests()
    sourceNoticeRuntime.dispose()
  }

  async function fetchNow(source: Source, token: number, readerSource: ReaderSource): Promise<FetchNowResult> {
    if (sourceRequestIsCurrent(token, readerSource)) {
      showSourceNotice('running', `抓取中：正在抓取 ${source.name} 的最新内容`)
    }
    try {
      await fetchSource(source.id)
      return { success: true }
    } catch (err) {
      return { success: false, error: formatAPIError(err) }
    }
  }

  async function loadSourceReaderSubscription(
    source: ReaderSource,
    requestOptions: { token?: number; silent?: boolean } = {},
  ) {
    const token = requestOptions.token ?? nextSourceRequestToken()
    if (!readerSourceMatches(source)) {
      return
    }
    options.setSourceSubscriptionLoading(true)
    try {
      const [sources, catalogResult] = await Promise.all([
        listSources(),
        listSourceCatalog({ limit: 200, offset: 0 }),
      ])
      if (!sourceRequestIsCurrent(token, source)) {
        return
      }
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
      if (sourceRequestIsCurrent(token, source) && !requestOptions.silent) {
        showSourceNotice('warning', `加载失败：来源状态未同步。详细原因：${formatAPIError(err)}`)
      }
    } finally {
      if (sourceRequestIsCurrent(token, source)) {
        options.setSourceSubscriptionLoading(false)
      }
    }
  }

  async function toggleSourceReaderSubscription() {
    const readerSource = options.getReaderSource()
    if (!readerSource || options.sourceSubscriptionLoading.value) {
      return
    }

    const token = nextSourceRequestToken()
    options.setSourceSubscriptionLoading(true)
    try {
      const currentSubscription = options.sourceSubscription.value
      if (currentSubscription) {
        const nextStatus = currentSubscription.status === 'active' ? 'inactive' : 'active'
        const updated = await updateSourceStatus(currentSubscription.id, nextStatus)
        if (!sourceRequestIsCurrent(token, readerSource)) {
          return
        }
        options.setSourceSubscription(updated)
        let fetchResult: FetchNowResult = { success: true }
        if (updated.status === 'active') {
          fetchResult = await fetchNow(updated, token, readerSource)
          if (!sourceRequestIsCurrent(token, readerSource)) {
            return
          }
        }
        options.onSubscriptionContentChanged?.(updated.id)
        if (updated.status === 'active' && !fetchResult.success) {
          showSourceNotice(
            'warning',
            `${updated.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`,
          )
        } else {
          showSourceNotice('success', `${updated.name} 已${updated.status === 'active' ? '开启并抓取最新内容' : '关闭'}`)
        }
        await loadSourceReaderSubscription(readerSource, { token })
        return
      }

      const currentCatalogEntry = options.sourceCatalogEntry.value
      if (!currentCatalogEntry) {
        showSourceNotice('warning', '该来源不在官方目录中，暂不支持直接开启')
        return
      }

      const result = await importCatalogSources([currentCatalogEntry.id])
      if (!sourceRequestIsCurrent(token, readerSource)) {
        return
      }
      const imported = result.sources[0]
      let fetchResult: FetchNowResult = { success: true }
      if (imported) {
        options.setSourceSubscription(imported)
        fetchResult = await fetchNow(imported, token, readerSource)
        if (!sourceRequestIsCurrent(token, readerSource)) {
          return
        }
        options.onSubscriptionContentChanged?.(imported.id)
      }
      if (!fetchResult.success) {
        showSourceNotice(
          'warning',
          `${currentCatalogEntry.name} 已开启，但抓取失败。详细原因：${
            fetchResult.error || '服务未返回具体错误原因'
          }`,
        )
      } else {
        showSourceNotice('success', `${currentCatalogEntry.name} 已开启并抓取最新内容`)
      }
      await loadSourceReaderSubscription(readerSource, { token })
    } catch (err) {
      if (sourceRequestIsCurrent(token, readerSource)) {
        showSourceNotice('warning', `操作失败：来源订阅状态未更新。详细原因：${formatAPIError(err)}`)
      }
    } finally {
      if (sourceRequestIsCurrent(token, readerSource)) {
        options.setSourceSubscriptionLoading(false)
      }
    }
  }

  return {
    sourceToggleLabel,
    sourceToggleActive,
    sourceToggleDisabled,
    clearNoticeTimer: sourceNoticeRuntime.clearTimer,
    clearSourceSubscriptionRuntime,
    showSourceNotice,
    resetSourceSubscriptionState,
    loadSourceReaderSubscription,
    toggleSourceReaderSubscription,
  }
}
