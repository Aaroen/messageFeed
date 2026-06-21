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

const sources = ref<Source[]>([])
const catalog = ref<SourceCatalogEntry[]>([])
const catalogQuery = ref('')
const urlInput = ref('')
const opmlFile = ref<File | null>(null)
const loading = ref(false)
const catalogLoading = ref(false)
const actionLoading = ref(false)
const notice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
const motionTimings = useMotionTimings()
let noticeTimer = 0
const importFetchConcurrency = 3

type ImportFetchSummary = {
  requestedCount: number
  successCount: number
  failureCount: number
}
type FetchNowResult = {
  success: boolean
  error?: string
}

const emit = defineEmits<{
  openSource: [source: { id: number; name: string; kind: 'subscriptions' | 'recommendations' }]
}>()

const sourceByNormalizedURL = computed(() => {
  const sourceMap = new Map<string, Source>()
  for (const source of sources.value) {
    sourceMap.set(source.normalized_url, source)
  }
  return sourceMap
})

function showNotice(type: 'success' | 'warning', message: string, durationMS?: number, delayMS = 0) {
  const normalized = message.trim()
  if (!normalized) {
    notice.value = null
    return
  }
  window.clearTimeout(noticeTimer)
  const show = () => {
    notice.value = { type, message: normalized }
    const duration = durationMS ?? motionTimings.noticeDuration(type)
    if (duration > 0) {
      noticeTimer = window.setTimeout(() => {
        notice.value = null
      }, duration)
    }
  }
  if (delayMS > 0) {
    noticeTimer = window.setTimeout(show, delayMS)
    return
  }
  show()
}

function formatFetchErrors(errors: Array<{ source_name?: string; message: string }> = []) {
  const details = errors
    .map((item) => {
      const name = item.source_name?.trim() || '未知来源'
      const message = item.message.trim()
      return message ? `${name}：${message}` : name
    })
    .filter(Boolean)
    .slice(0, 3)
  if (!details.length) {
    return '服务未返回具体错误原因'
  }
  const overflow = errors.length > details.length ? `；另有 ${errors.length - details.length} 个失败来源` : ''
  return `${details.join('；')}${overflow}`
}

async function loadSources(options: { silent?: boolean; notify?: boolean } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  try {
    sources.value = await listSources()
  } catch (err) {
    if (options.notify !== false) {
      showNotice('warning', `刷新失败：订阅源列表加载失败。详细原因：${formatAPIError(err)}`)
    }
    throw err
  } finally {
    if (!options.silent) {
      loading.value = false
    }
  }
}

async function loadCatalog(options: { silent?: boolean; notify?: boolean } = {}) {
  if (!options.silent) {
    catalogLoading.value = true
  }
  try {
    const result = await listSourceCatalog({ q: catalogQuery.value || undefined, limit: 200, offset: 0 })
    catalog.value = result.entries
  } catch (err) {
    if (options.notify !== false) {
      showNotice('warning', `刷新失败：推荐源目录加载失败。详细原因：${formatAPIError(err)}`)
    }
    throw err
  } finally {
    if (!options.silent) {
      catalogLoading.value = false
    }
  }
}

async function refreshPage(options: { noticeDelayMS?: number; suppressStartNotice?: boolean } = {}) {
  if (loading.value || catalogLoading.value || actionLoading.value) {
    return
  }
  const query = catalogQuery.value.trim()
  try {
    if (!options.suppressStartNotice) {
      showNotice('success', query ? `抓取中：正在更新“${query}”的推荐源搜索结果` : '抓取中：正在更新订阅管理数据', 0)
    }
    await Promise.all([
      loadSources({ silent: true, notify: false }),
      loadCatalog({ silent: true, notify: false }),
    ])
    const fetchResult = await fetchActiveSources()
    await loadSources({ silent: true, notify: false })
    if (fetchResult.failure_count > 0) {
      const prefix = fetchResult.success_count > 0 ? '刷新异常' : '刷新失败'
      showNotice(
        'warning',
        `${prefix}：推荐源目录已更新；已抓取 ${fetchResult.success_count} 个订阅源，${fetchResult.failure_count} 个失败。失败原因：${formatFetchErrors(fetchResult.errors)}`,
        undefined,
        options.noticeDelayMS,
      )
      return
    }
    showNotice('success', '刷新成功：已更新订阅管理数据', undefined, options.noticeDelayMS)
  } catch (err) {
    showNotice('warning', `刷新异常：订阅管理数据未完整更新。详细原因：${formatAPIError(err)}`, undefined, options.noticeDelayMS)
  }
}

async function fetchImportedSources(importedSources: Source[]): Promise<ImportFetchSummary> {
  const activeSources = importedSources.filter((source) => source.status === 'active')
  const summary: ImportFetchSummary = {
    requestedCount: activeSources.length,
    successCount: 0,
    failureCount: 0,
  }
  if (!activeSources.length) {
    return summary
  }

  let cursor = 0
  const workerCount = Math.min(importFetchConcurrency, activeSources.length)
  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (cursor < activeSources.length) {
        const source = activeSources[cursor]
        cursor += 1
        try {
          await fetchSource(source.id)
          summary.successCount += 1
        } catch {
          summary.failureCount += 1
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
    parts.push(`${result.failure_count} 个导入失败`)
  }
  if (fetchSummary.requestedCount > 0) {
    parts.push(`已抓取 ${fetchSummary.successCount} 个`)
  }
  if (fetchSummary.failureCount > 0) {
    parts.push(`${fetchSummary.failureCount} 个抓取失败`)
  }
  return parts.join('，')
}

async function handleImportURLs() {
  const urls = urlInput.value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
  if (!urls.length) {
    return
  }
  actionLoading.value = true
  try {
    const result = await importURLSources(urls)
    const fetchSummary = await fetchImportedSources(result.sources)
    showNotice(importNoticeType(result, fetchSummary), importNoticeMessage('已导入', result, fetchSummary))
    urlInput.value = ''
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', `导入失败：${formatAPIError(err)}`)
  } finally {
    actionLoading.value = false
  }
}

async function handleImportOPML(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  if (!file) {
    return
  }
  opmlFile.value = file
  actionLoading.value = true
  try {
    const result = await importOPMLSource(file)
    const fetchSummary = await fetchImportedSources(result.sources)
    showNotice(importNoticeType(result, fetchSummary), importNoticeMessage('已从 OPML 导入', result, fetchSummary))
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', `导入失败：${formatAPIError(err)}`)
  } finally {
    actionLoading.value = false
    input.value = ''
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

function openCatalogSource(entry: SourceCatalogEntry) {
  const source = sourceForCatalog(entry)
  const subscribed = catalogStatus(entry) === 'active' && Boolean(source || entry.source_id)
  emit('openSource', {
    id: subscribed ? source?.id ?? entry.source_id ?? entry.id : entry.id,
    name: source?.name ?? entry.name,
    kind: subscribed ? 'subscriptions' : 'recommendations',
  })
}

async function fetchNow(source: Source): Promise<FetchNowResult> {
  showNotice('success', `抓取中：正在抓取 ${source.name} 的最新内容`, 0)
  try {
    await fetchSource(source.id)
    return { success: true }
  } catch (err) {
    return { success: false, error: formatAPIError(err) }
  }
}

async function toggleCatalogSource(entry: SourceCatalogEntry) {
  actionLoading.value = true
  try {
    const existing = sourceForCatalog(entry)
    if (existing) {
      const nextStatus = existing.status === 'active' ? 'inactive' : 'active'
      const updated = await updateSourceStatus(existing.id, nextStatus)
      let fetchResult: FetchNowResult = { success: true }
      if (updated.status === 'active') {
        fetchResult = await fetchNow(updated)
      }
      if (updated.status === 'active' && !fetchResult.success) {
        showNotice('warning', `${updated.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`)
      } else {
        showNotice('success', `${updated.name} 已${updated.status === 'active' ? '开启并抓取最新内容' : '关闭'}`)
      }
      await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
      return
    }

    const result = await importCatalogSources([entry.id])
    const imported = result.sources[0]
    let fetchResult: FetchNowResult = { success: true }
    if (imported) {
      fetchResult = await fetchNow(imported)
    }
    if (!fetchResult.success) {
      showNotice('warning', `${entry.name} 已开启，但抓取失败。详细原因：${fetchResult.error || '服务未返回具体错误原因'}`)
    } else {
      showNotice('success', `${entry.name} 已开启并抓取最新内容`)
    }
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', `操作失败：${entry.name} 状态未更新。详细原因：${formatAPIError(err)}`)
  } finally {
    actionLoading.value = false
  }
}

onMounted(() => {
  void refreshPage().catch(() => undefined)
})

onUnmounted(() => {
  window.clearTimeout(noticeTimer)
})

defineExpose({ refreshPage })
</script>

<template>
  <section class="sources-page">
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
        <button class="sources-button" type="button" :disabled="catalogLoading" @click="() => loadCatalog()">
          <IconSearch />
          <span>搜索</span>
        </button>
      </div>
    </div>

    <section class="sources-panel">
      <div v-if="catalogLoading || loading" class="sources-empty">加载中</div>
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
            <div class="source-row__meta">{{ entry.category }} · {{ entry.health_status }}</div>
            <div class="source-row__meta">{{ entry.feed_url }}</div>
          </div>
          <div class="source-row__actions">
            <button
              class="sources-button"
              :class="{ 'sources-button--active': catalogStatus(entry) === 'active' }"
              type="button"
              :disabled="actionLoading"
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
          <input type="file" accept=".opml,.xml" hidden @change="handleImportOPML" />
        </label>
      </article>

      <article class="sources-panel">
        <textarea v-model="urlInput" class="sources-textarea" rows="8" placeholder="每行一个订阅地址"></textarea>
        <div class="sources-toolbar__group">
          <button class="sources-button" type="button" :disabled="actionLoading" @click="handleImportURLs">
            导入 URL
          </button>
        </div>
      </article>
    </section>
  </section>
</template>
