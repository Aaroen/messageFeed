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
  fetchSource,
  updateSourceStatus,
  type Source,
  type SourceCatalogEntry,
} from '@/api/feed'

const sources = ref<Source[]>([])
const catalog = ref<SourceCatalogEntry[]>([])
const catalogQuery = ref('')
const urlInput = ref('')
const opmlFile = ref<File | null>(null)
const loading = ref(false)
const catalogLoading = ref(false)
const actionLoading = ref(false)
const notice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
let noticeTimer = 0

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

function showNotice(type: 'success' | 'warning', message: string) {
  notice.value = { type, message }
  window.clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => {
    notice.value = null
  }, 2600)
}

async function loadSources(options: { silent?: boolean } = {}) {
  if (!options.silent) {
    loading.value = true
  }
  try {
    sources.value = await listSources()
  } catch (err) {
    showNotice('warning', formatAPIError(err))
  } finally {
    if (!options.silent) {
      loading.value = false
    }
  }
}

async function loadCatalog(options: { silent?: boolean } = {}) {
  if (!options.silent) {
    catalogLoading.value = true
  }
  try {
    const result = await listSourceCatalog({ q: catalogQuery.value || undefined, limit: 200, offset: 0 })
    catalog.value = result.entries
  } catch (err) {
    showNotice('warning', formatAPIError(err))
  } finally {
    if (!options.silent) {
      catalogLoading.value = false
    }
  }
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
    showNotice('success', `已导入 ${result.success_count} 个来源`)
    urlInput.value = ''
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', formatAPIError(err))
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
    showNotice('success', `已从 OPML 导入 ${result.success_count} 个来源`)
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', formatAPIError(err))
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
  emit('openSource', {
    id: source?.id ?? entry.source_id ?? entry.id,
    name: source?.name ?? entry.name,
    kind: source || entry.source_id ? 'subscriptions' : 'recommendations',
  })
}

async function fetchNow(source: Source) {
  try {
    await fetchSource(source.id)
  } catch (err) {
    showNotice('warning', formatAPIError(err))
  }
}

async function toggleCatalogSource(entry: SourceCatalogEntry) {
  actionLoading.value = true
  try {
    const existing = sourceForCatalog(entry)
    if (existing) {
      const nextStatus = existing.status === 'active' ? 'inactive' : 'active'
      const updated = await updateSourceStatus(existing.id, nextStatus)
      if (updated.status === 'active') {
        await fetchNow(updated)
      }
      showNotice('success', `${updated.name} 已${updated.status === 'active' ? '开启' : '关闭'}`)
      await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
      return
    }

    const result = await importCatalogSources([entry.id])
    const imported = result.sources[0]
    if (imported) {
      await fetchNow(imported)
    }
    showNotice('success', `${entry.name} 已开启`)
    await Promise.all([loadSources({ silent: true }), loadCatalog({ silent: true })])
  } catch (err) {
    showNotice('warning', formatAPIError(err))
  } finally {
    actionLoading.value = false
  }
}

onMounted(() => {
  loadSources()
  loadCatalog()
})

onUnmounted(() => {
  window.clearTimeout(noticeTimer)
})
</script>

<template>
  <section class="sources-page">
    <div
      v-if="notice"
      class="sources-toast"
      :class="`sources-toast--${notice.type}`"
      role="status"
      aria-live="polite"
    >
      {{ notice.message }}
    </div>

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
