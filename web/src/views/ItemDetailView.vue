<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { formatAPIError } from '@/api/client'
import {
  type FeedItem,
  getFeedItem,
  setItemFavorite,
  setItemHidden,
  setItemRead,
} from '@/api/feed'
import ActionBar from '@/components/ActionBar.vue'
import { escapeHTML, formatItemDate, sanitizeDetailHTML } from '@/utils/readerContent'

type ItemStateKey = 'read' | 'favorite' | 'hidden'

const route = useRoute()
const item = ref<FeedItem | null>(null)
const loading = ref(false)
const actionBusy = ref<ItemStateKey | ''>('')
const error = ref('')
const actionError = ref('')

const itemID = computed(() => Number.parseInt(route.params.id?.toString() ?? '0', 10))
const displayDate = computed(() => formatItemDate(item.value?.published_at || item.value?.fetched_at))
const detailSrcdoc = computed(() => {
  const current = item.value
  const source =
    current?.content_html ||
    current?.content_snippet ||
    `<p>${escapeHTML(current?.content_text || current?.summary || '暂无正文。')}</p>`
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>
  :root { color-scheme: light dark; }
  body {
    margin: 0;
    background: transparent;
    color: #172033;
    font: 16px/1.72 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    overflow-wrap: anywhere;
  }
  img, video, iframe { max-width: 100%; height: auto; }
  pre, code { white-space: pre-wrap; overflow-wrap: anywhere; }
  a { color: #1d4ed8; }
  blockquote { margin: 1em 0; padding-left: 1em; border-left: 3px solid #d1d5db; color: #4b5563; }
  @media (prefers-color-scheme: dark) {
    body { color: #dbe6f3; }
    a { color: #93c5fd; }
    blockquote { border-left-color: #475569; color: #a9b6c6; }
  }
</style>
</head>
<body>${sanitizeDetailHTML(source)}</body>
</html>`
})

async function loadItem() {
  if (!Number.isFinite(itemID.value) || itemID.value < 1) {
    error.value = '条目不存在。'
    item.value = null
    return
  }
  loading.value = true
  error.value = ''
  actionError.value = ''
  try {
    item.value = await getFeedItem(itemID.value)
  } catch (err) {
    item.value = null
    error.value = formatAPIError(err)
  } finally {
    loading.value = false
  }
}

async function updateItemState(key: ItemStateKey) {
  const current = item.value
  if (!current || actionBusy.value) {
    return
  }
  actionBusy.value = key
  actionError.value = ''
  try {
    if (key === 'read') {
      const state = await setItemRead(current.id, !current.is_read)
      item.value = { ...current, is_read: state.is_read }
      return
    }
    if (key === 'favorite') {
      const state = await setItemFavorite(current.id, !current.is_favorite)
      item.value = { ...current, is_favorite: state.is_favorite }
      return
    }
    const state = await setItemHidden(current.id, !current.is_hidden)
    item.value = { ...current, is_hidden: state.is_hidden }
  } catch (err) {
    actionError.value = `状态更新失败：${formatAPIError(err)}`
  } finally {
    actionBusy.value = ''
  }
}

onMounted(loadItem)
watch(itemID, loadItem)
</script>

<template>
  <main class="item-detail-page">
    <section v-if="item" class="item-detail-page__surface">
      <div class="item-detail-page__meta">
        <span>{{ item.source_name || '未知来源' }}</span>
        <span>{{ displayDate }}</span>
      </div>
      <h1>{{ item.title }}</h1>
      <div class="item-detail-page__toolbar">
        <ActionBar
          :is-read="item.is_read"
          :is-favorite="item.is_favorite"
          :is-hidden="item.is_hidden"
          :busy-key="actionBusy"
          @toggle-read="updateItemState('read')"
          @toggle-favorite="updateItemState('favorite')"
          @toggle-hidden="updateItemState('hidden')"
        />
        <a :href="item.url" target="_blank" rel="noreferrer">阅读原文</a>
      </div>
      <a-alert v-if="actionError" type="warning" show-icon :content="actionError" />
      <iframe class="item-detail-page__frame" title="条目正文" sandbox="allow-popups" :srcdoc="detailSrcdoc" />
    </section>
    <section v-else class="empty-surface">
      <div class="empty-surface__mark">读</div>
      <h2>{{ loading ? '正在加载条目' : '未找到条目' }}</h2>
      <p>{{ error || '请稍候。' }}</p>
    </section>
  </main>
</template>
