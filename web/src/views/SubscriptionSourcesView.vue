<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { IconSearch, IconUpload } from '@arco-design/web-vue/es/icon'

import { formatAPIError } from '@/api/client'
import {
  importCatalogSources,
  importOPMLSource,
  importURLSources,
  listSourceCatalog,
  listSources,
  fetchActiveSources,
  fetchSource,
  updateSourceStatus,
  type ImportSourcesResult,
  type Source,
  type SourceCatalogEntry,
} from '@/api/feed'
import { useMotionTimings } from '@/composables/useMotionTimings'
import type { PageRefreshOptions } from '@/composables/usePageOutletState'
import { useRefreshLayoutFreeze } from '@/composables/useRefreshLayoutFreeze'
import { useRequestToken } from '@/composables/useRequestToken'
import { useTimedNotice } from '@/composables/useTimedNotice'
import { useFeedListCacheStore } from '@/stores/feedListCache'
import { formatSourceFetchErrors, subscriptionManagementFetchNotice } from '@/utils/sourceFetchMessages'

const sourcesPageRef = ref<HTMLElement | null>(null)
const sources = ref<Source[]>([])
const catalog = ref<SourceCatalogEntry[]>([])
const catalogQuery = ref('')
const urlInput = ref('')
const opmlFile = ref<File | null>(null)
const loading = ref(false)
const catalogLoading = ref(false)
const actionLoading = ref(false)
const pageRefreshing = ref(false)
const motionTimings = useMotionTimings()
const noticeRuntime = useTimedNotice<'running' | 'success' | 'warning'>({
  duration: motionTimings.noticeDuration,
})
const notice = noticeRuntime.notice
const showNotice = noticeRuntime.show
const clearNotice = noticeRuntime.clear
const refreshLayoutFreeze = useRefreshLayoutFreeze({ targetRef: sourcesPageRef })
const feedListCache = useFeedListCacheStore()
let pageRefreshingToken = 0
let disposed = false
const pageRequestToken = useRequestToken({ isActive: () => !disposed })
const importFetchConcurrency = 3
const pageBusy = computed(() => loading.value || catalogLoading.value || actionLoading.value || pageRefreshing.value)

type ImportFetchSummary = {
  requestedCount: number
  successCount: number
  failureCount: number
  errors: Array<{ source_name?: string; message: string }>
}
type FetchNowResult = {
  success: boolean
  error?: string
}

const emit = defineEmits<{
  'open-source': [source: { id: number; name: string; kind: 'subscriptions' | 'recommendations' }]
}>()

const sourceByNormalizedURL = computed(() => {
  const sourceMap = new Map<string, Source>()
  for (const source of sources.value) {
    sourceMap.set(source.normalized_url, source)
  }
  return sourceMap
})

function nextPageRequestToken() {
  return pageRequestToken.next()
}

function invalidatePageRequests() {
  pageRequestToken.invalidate()
}

function pageRequestIsCurrent(token?: number) {
  return pageRequestToken.isCurrent(token)
}

function beginPageRefresh(token: number) {
  pageRefreshingToken = token
  pageRefreshing.value = true
}

function finishPageRefresh(token: number) {
  if (pageRefreshingToken !== token) {
    return
  }
  pageRefreshingToken = 0
  pageRefreshing.value = false
}

function invalidateSubscriptionFeedCaches(sourceIDs: number[] = []) {
  feedListCache.invalidate('subscriptions:subscriptions:0')
  for (const sourceID of sourceIDs) {
    if (sourceID > 0) {
      feedListCache.invalidate(`source:subscriptions:${sourceID}`)
    }
  }
}

async function loadSources(options: { silent?: boolean; notify?: boolean; token?: number } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  try {
    const nextSources = await listSources()
    if (!pageRequestIsCurrent(options.token)) {
      return
    }
    sources.value = nextSources
  } catch (err) {
    if (pageRequestIsCurrent(options.token) && options.notify !== false) {
      showNotice('warning', `刷新失败：订阅源列表加载失败。详细原因：${formatAPIError(err)}`)
    }
    throw err
  } finally {
    if (pageRequestIsCurrent(options.token) && !options.silent) {
      loading.value = false
    }
  }
}

async function loadCatalog(
  options: { silent?: boolean; notify?: boolean; token?: number; failurePrefix?: string } = {},
) {
  if (!options.silent) {
    catalogLoading.value = true
  }
  try {
    const result = await listSourceCatalog({ q: catalogQuery.value || undefined, limit: 200, offset: 0 })
    if (!pageRequestIsCurrent(options.token)) {
      return
    }
    catalog.value = result.entries
  } catch (err) {
    if (pageRequestIsCurrent(options.token) && options.notify !== false) {
      showNotice(
        'warning',
        `${options.failurePrefix ?? '刷新失败'}：推荐源目录加载失败。详细原因：${formatAPIError(err)}`,
      )
    }
    throw err
  } finally {
    if (pageRequestIsCurrent(options.token) && !options.silent) {
      catalogLoading.value = false
    }
  }
}

async function refreshPage(options: PageRefreshOptions = {}) {
  if (pageBusy.value) {
    return
  }
  const token = nextPageRequestToken()
  beginPageRefresh(token)
  let releaseRefreshLayoutFreeze: (() => void) | undefined
  if (options.onRefreshSettled) {
    releaseRefreshLayoutFreeze = refreshLayoutFreeze.capture()
    options.onRefreshSettled(releaseRefreshLayoutFreeze)
  }
  const query = catalogQuery.value.trim()
  try {
    if (!options.suppressStartNotice) {
      showNotice(
        'running',
        query ? `抓取中：正在更新“${query}”的推荐源搜索结果` : '抓取中：正在更新订阅管理数据',
      )
    }
    await Promise.all([
      loadSources({ silent: true, notify: false, token }),
      loadCatalog({ silent: true, notify: false, token }),
    ])
    if (!pageRequestIsCurrent(token)) {
      return
    }
    const fetchResult = await fetchActiveSources()
    if (!pageRequestIsCurrent(token)) {
      return
    }
    invalidateSubscriptionFeedCaches(fetchResult.sources.map((source) => source.id))
    await loadSources({ silent: true, notify: false, token })
    if (!pageRequestIsCurrent(token)) {
      return
    }
    const fetchNotice = subscriptionManagementFetchNotice(fetchResult)
    showNotice(fetchNotice.type, fetchNotice.message, undefined, options.noticeDelayMS)
  } catch (err) {
    if (pageRequestIsCurrent(token)) {
      showNotice(
        'warning',
        `刷新异常：订阅管理数据未完整更新。详细原因：${formatAPIError(err)}`,
        undefined,
        options.noticeDelayMS,
      )
    }
  } finally {
    if (!pageRequestIsCurrent(token)) {
      releaseRefreshLayoutFreeze?.()
      finishPageRefresh(token)
      return
    }
    finishPageRefresh(token)
  }
}

async function fetchImportedSources(importedSources: Source[], token: number): Promise<ImportFetchSummary> {
  const activeSources = importedSources.filter((source) => source.status === 'active')
  const summary: ImportFetchSummary = {
    requestedCount: activeSources.length,
    successCount: 0,
    failureCount: 0,
    errors: [],
  }
  if (!activeSources.length) {
    return summary
  }

  let cursor = 0
  const workerCount = Math.min(importFetchConcurrency, activeSources.length)
  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (pageRequestIsCurrent(token) && cursor < activeSources.length) {
        const source = activeSources[cursor]
        cursor += 1
        try {
          await fetchSource(source.id)
          if (!pageRequestIsCurrent(token)) {
            return
          }
          summary.successCount += 1
        } catch (err) {
          if (!pageRequestIsCurrent(token)) {
            return
          }
          summary.failureCount += 1
          summary.errors.push({
            source_name: source.name,
            message: formatAPIError(err),
          })
        }
      }
    }),
  )
  return summary
}

function importNoticeType(result: ImportSourcesResult, fetchSummary: ImportFetchSummary) {
  return result.failure_count > 0 || fetchSummary.failureCount > 0 ? 'warning' : 'success'
}

function importNoticeMessage(prefix: string, result: ImportSourcesResult, fetchSummary: ImportFetchSummary) {
  const parts = [`${prefix} ${result.success_count} 个来源`]
  if (result.failure_count > 0) {
    const importErrors = result.errors.map((item) => ({
      source_name: item.reference,
      message: item.message,
    }))
    const reason = importErrors.length ? `。导入失败原因：${formatSourceFetchErrors(importErrors)}` : ''
    parts.push(`${result.failure_count} 个导入失败${reason}`)
  }
  if (fetchSummary.requestedCount > 0) {
    parts.push(`已抓取 ${fetchSummary.successCount} 个`)
  }
  if (fetchSummary.failureCount > 0) {
    const reason = fetchSummary.errors.length
      ? `。抓取失败原因：${formatSourceFetchErrors(fetchSummary.errors)}`
      : ''
    parts.push(`${fetchSummary.failureCount} 个抓取失败${reason}`)
  }
  return parts.join('，')
}

async function handleImportURLs() {
  if (pageBusy.value) {
    return
  }
  const urls = urlInput.value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
  if (!urls.length) {
    return
  }
  const token = nextPageRequestToken()
  actionLoading.value = true
  let importCompleted = false
  try {
    const result = await importURLSources(urls)
    if (!pageRequestIsCurrent(token)) {
      return
    }
    const fetchSummary = await fetchImportedSources(result.sources, token)
    if (!pageRequestIsCurrent(token)) {
      return
    }
    invalidateSubscriptionFeedCaches(result.sources.map((source) => source.id))
    importCompleted = true
    await Promise.all([
      loadSources({ silent: true, notify: false, token }),
      loadCatalog({ silent: true, notify: false, token }),
    ])
    if (!pageRequestIsCurrent(token)) {
      return
    }
    showNotice(importNoticeType(result, fetchSummary), importNoticeMessage('已导入', result, fetchSummary))
    urlInput.value = ''
  } catch (err) {
    if (pageRequestIsCurrent(token)) {
      showNotice(
        'warning',
        importCompleted
          ? `导入已完成，但订阅管理数据刷新失败。详细原因：${formatAPIError(err)}`
          : `导入失败：${formatAPIError(err)}`,
      )
    }
  } finally {
    if (pageRequestIsCurrent(token)) {
      actionLoading.value = false
    }
  }
}

async function handleImportOPML(event: Event) {
  if (pageBusy.value) {
    return
  }
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  if (!file) {
    return
  }
  const token = nextPageRequestToken()
  opmlFile.value = file
  actionLoading.value = true
  let importCompleted = false
  try {
    const result = await importOPMLSource(file)
    if (!pageRequestIsCurrent(token)) {
      return
    }
    const fetchSummary = await fetchImportedSources(result.sources, token)
    if (!pageRequestIsCurrent(token)) {
      return
    }
    invalidateSubscriptionFeedCaches(result.sources.map((source) => source.id))
    importCompleted = true
    await Promise.all([
      loadSources({ silent: true, notify: false, token }),
      loadCatalog({ silent: true, notify: false, token }),
    ])
    if (!pageRequestIsCurrent(token)) {
      return
    }
    showNotice(importNoticeType(result, fetchSummary), importNoticeMessage('已从 OPML 导入', result, fetchSummary))
  } catch (err) {
    if (pageRequestIsCurrent(token)) {
      showNotice(
        'warning',
        importCompleted
          ? `导入已完成，但订阅管理数据刷新失败。详细原因：${formatAPIError(err)}`
          : `导入失败：${formatAPIError(err)}`,
      )
    }
  } finally {
    if (pageRequestIsCurrent(token)) {
      actionLoading.value = false
      input.value = ''
    }
  }
}

function sourceForCatalog(entry: SourceCatalogEntry) {
  return sourceByNormalizedURL.value.get(entry.normalized_url)
}

function catalogStatus(entry: SourceCatalogEntry) {
  const source = sourceForCatalog(entry)
  return source?.status ?? entry.source_status
}

function catalogToggleLabel(entry: SourceCatalogEntry) {
  return catalogStatus(entry) === 'active' ? '关闭' : '开启'
}

function catalogHealthLabel(status: SourceCatalogEntry['health_status']) {
  if (status === 'healthy') {
    return '健康'
  }
  if (status === 'degraded') {
    return '不稳定'
  }
  if (status === 'unreachable') {
    return '不可达'
  }
  return '未知'
}

function openCatalogSource(entry: SourceCatalogEntry) {
  const source = sourceForCatalog(entry)
  const subscribed = catalogStatus(entry) === 'active' && Boolean(source || entry.source_id)
  emit('open-source', {
    id: subscribed ? source?.id ?? entry.source_id ?? entry.id : entry.id,
    name: source?.name ?? entry.name,
    kind: subscribed ? 'subscriptions' : 'recommendations',
  })
}

async function fetchNow(source: Source, token: number): Promise<FetchNowResult> {
  if (!pageRequestIsCurrent(token)) {
    return { success: false, error: '页面状态已更新，本次抓取已取消' }
  }
  showNotice('running', `抓取中：正在抓取 ${source.name} 的最新内容`)
  try {
    await fetchSource(source.id)
    return { success: true }
  } catch (err) {
    return { success: false, error: formatAPIError(err) }
  }
}

async function toggleCatalogSource(entry: SourceCatalogEntry) {
  if (pageBusy.value) {
    return
  }
  const token = nextPageRequestToken()
  actionLoading.value = true
  let sourceStatusUpdated = false
  try {
    const existing = sourceForCatalog(entry)
    if (existing) {
      const nextStatus = existing.status === 'active' ? 'inactive' : 'active'
      const updated = await updateSourceStatus(existing.id, nextStatus)
      if (!pageRequestIsCurrent(token)) {
        return
      }
      sourceStatusUpdated = true
      let fetchResult: FetchNowResult = { success: true }
      if (updated.status === 'active') {
        fetchResult = await fetchNow(updated, token)
        if (!pageRequestIsCurrent(token)) {
          return
        }
      }
      invalidateSubscriptionFeedCaches([updated.id])
      const noticeType = updated.status === 'active' && !fetchResult.success ? 'warning' : 'success'
      const noticeMessage =
        updated.status === 'active' && !fetchResult.success
          ? `${updated.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`
          : `${updated.name} 已${updated.status === 'active' ? '开启并抓取最新内容' : '关闭'}`
      await Promise.all([
        loadSources({ silent: true, notify: false, token }),
        loadCatalog({ silent: true, notify: false, token }),
      ])
      if (!pageRequestIsCurrent(token)) {
        return
      }
      showNotice(noticeType, noticeMessage)
      return
    }

    const result = await importCatalogSources([entry.id])
    if (!pageRequestIsCurrent(token)) {
      return
    }
    const imported = result.sources[0]
    sourceStatusUpdated = Boolean(imported || result.success_count > 0)
    let fetchResult: FetchNowResult = { success: true }
    if (imported) {
      fetchResult = await fetchNow(imported, token)
      if (!pageRequestIsCurrent(token)) {
        return
      }
    }
    invalidateSubscriptionFeedCaches(imported ? [imported.id] : [])
    const noticeType = !fetchResult.success ? 'warning' : 'success'
    const noticeMessage = !fetchResult.success
      ? `${entry.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`
      : `${entry.name} 已开启并抓取最新内容`
    await Promise.all([
      loadSources({ silent: true, notify: false, token }),
      loadCatalog({ silent: true, notify: false, token }),
    ])
    if (!pageRequestIsCurrent(token)) {
      return
    }
    showNotice(noticeType, noticeMessage)
  } catch (err) {
    if (pageRequestIsCurrent(token)) {
      showNotice(
        'warning',
        sourceStatusUpdated
          ? `操作已完成，但订阅管理数据刷新失败。详细原因：${formatAPIError(err)}`
          : `操作失败：${entry.name} 状态未更新。详细原因：${formatAPIError(err)}`,
      )
    }
  } finally {
    if (pageRequestIsCurrent(token)) {
      actionLoading.value = false
    }
  }
}

onMounted(() => {
  void refreshPage().catch(() => undefined)
})

onUnmounted(() => {
  disposed = true
  invalidatePageRequests()
  finishPageRefresh(pageRefreshingToken)
  noticeRuntime.dispose()
  refreshLayoutFreeze.release()
})

defineExpose({ refreshPage, clearNotice })
</script>

<template>
  <section ref="sourcesPageRef" class="sources-page" :style="refreshLayoutFreeze.style.value">
    <Teleport to="body">
      <div
        v-if="notice"
        class="sources-toast"
        :class="`sources-toast--${notice.type}`"
        role="status"
        aria-live="polite"
      >
        {{ notice.message }}
      </div>
    </Teleport>

    <div class="sources-toolbar">
      <div class="sources-toolbar__group">
        <input v-model="catalogQuery" class="sources-input" type="search" placeholder="搜索订阅源" />
        <button
          class="sources-button"
          type="button"
          :disabled="pageBusy"
          @click="() => loadCatalog({ failurePrefix: '搜索失败' })"
        >
          <IconSearch />
          <span>搜索</span>
        </button>
      </div>
    </div>

    <section class="sources-panel">
      <div v-if="catalogLoading || loading || (pageRefreshing && !catalog.length)" class="sources-empty">加载中</div>
      <div v-else class="sources-list">
        <div
          v-for="entry in catalog"
          :key="entry.id"
          class="source-row source-row--catalog source-row--clickable"
          role="button"
          tabindex="0"
          @click="openCatalogSource(entry)"
          @keydown.enter.prevent="openCatalogSource(entry)"
          @keydown.space.prevent="openCatalogSource(entry)"
        >
          <div>
            <div class="source-row__title">{{ entry.name }}</div>
            <div class="source-row__meta">
              {{ entry.category }} · {{ catalogHealthLabel(entry.health_status) }}
            </div>
            <div class="source-row__meta">{{ entry.feed_url }}</div>
          </div>
          <div class="source-row__actions">
            <button
              class="sources-button"
              :class="{ 'sources-button--active': catalogStatus(entry) === 'active' }"
              type="button"
              :disabled="pageBusy"
              @click.stop="toggleCatalogSource(entry)"
            >
              {{ catalogToggleLabel(entry) }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <section class="sources-layout sources-layout--imports">
      <article class="sources-panel">
        <label class="sources-button sources-button--file">
          <IconUpload />
          <span>导入 OPML</span>
          <input type="file" accept=".opml,.xml" hidden :disabled="pageBusy" @change="handleImportOPML" />
        </label>
      </article>

      <article class="sources-panel">
        <textarea v-model="urlInput" class="sources-textarea" rows="8" placeholder="每行一个订阅地址"></textarea>
        <div class="sources-toolbar__group">
          <button class="sources-button" type="button" :disabled="pageBusy" @click="handleImportURLs">
            导入 URL
          </button>
        </div>
      </article>
    </section>
  </section>
</template>
