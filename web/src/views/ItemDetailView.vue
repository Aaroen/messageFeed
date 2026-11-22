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
import FeedItemTime from '@/components/FeedItemTime.vue'
import { useThemeState } from '@/composables/useThemeState'
import { escapeHTML, sanitizeDetailHTML } from '@/utils/readerContent'

type ItemStateKey = 'read' | 'favorite' | 'hidden'

const route = useRoute()
const { dark } = useThemeState()
const item = ref<FeedItem | null>(null)
const loading = ref(false)
const actionBusy = ref<ItemStateKey | ''>('')
const error = ref('')
const actionError = ref('')

const itemID = computed(() => Number.parseInt(route.params.id?.toString() ?? '0', 10))
const detailSrcdoc = computed(() => {
  const current = item.value
  const source =
    current?.content_html ||
    current?.content_snippet ||
    `<p>${escapeHTML(current?.content_text || current?.summary || '暂无正文。')}</p>`
  const theme = dark.value ? 'dark' : 'light'
  return `<!doctype html>
<html data-theme="${theme}">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>
  :root { color-scheme: ${theme}; }
  body {
    margin: 0;
    background: transparent;
    color: #16191f;
    font: 16px/1.78 ui-sans-serif, -apple-system, BlinkMacSystemFont, "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif;
    overflow-wrap: anywhere;
  }
  h1, h2, h3, h4, h5, h6 {
    margin: 1.5em 0 0.6em;
    line-height: 1.38;
    font-weight: 680;
    color: #111827;
  }
  h1 { font-size: 1.45em; }
  h2 { font-size: 1.28em; }
  h3 { font-size: 1.14em; }
  p { margin: 0 0 1.05em; }
  ul, ol { margin: 0.7em 0 1.1em; padding-left: 1.5em; }
  li { margin: 0.35em 0; }
  hr { margin: 1.6em 0; border: 0; border-top: 1px solid rgba(15, 23, 42, 0.12); }
  img, video, iframe {
    max-width: 100%;
    height: auto;
    border-radius: 8px;
  }
  figure { margin: 1.2em 0; }
  figcaption { margin-top: 0.5em; color: #5a6270; font-size: 0.875em; text-align: center; }
  pre, code {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  pre {
    margin: 1.1em 0;
    padding: 14px 16px;
    border: 1px solid rgba(15, 23, 42, 0.1);
    border-radius: 10px;
    background: #eceef2;
    font-size: 0.875em;
    line-height: 1.62;
  }
  :not(pre) > code {
    padding: 2px 6px;
    border-radius: 5px;
    background: #eceef2;
    font-size: 0.9em;
  }
  a {
    color: #2563eb;
    text-underline-offset: 3px;
  }
  blockquote {
    margin: 1.1em 0;
    padding: 0.25em 0 0.25em 1em;
    border-left: 3px solid rgba(37, 99, 235, 0.4);
    color: #5a6270;
  }
  table {
    width: 100%;
    margin: 1.2em 0;
    border-collapse: collapse;
    font-size: 0.92em;
  }
  th, td {
    padding: 8px 10px;
    border: 1px solid rgba(15, 23, 42, 0.12);
    text-align: left;
  }
  th {
    background: #eceef2;
    font-weight: 640;
  }
  html[data-theme="dark"] body { color: #f1f3f7; }
  html[data-theme="dark"] h1,
  html[data-theme="dark"] h2,
  html[data-theme="dark"] h3,
  html[data-theme="dark"] h4,
  html[data-theme="dark"] h5,
  html[data-theme="dark"] h6 { color: #f8fafc; }
  html[data-theme="dark"] hr { border-top-color: rgba(255, 255, 255, 0.12); }
  html[data-theme="dark"] figcaption { color: #9ca6b8; }
  html[data-theme="dark"] pre {
    border-color: rgba(255, 255, 255, 0.12);
    background: #242933;
  }
  html[data-theme="dark"] :not(pre) > code { background: #242933; }
  html[data-theme="dark"] a { color: #60a5fa; }
  html[data-theme="dark"] blockquote {
    border-left-color: rgba(96, 165, 250, 0.5);
    color: #9ca6b8;
  }
  html[data-theme="dark"] th,
  html[data-theme="dark"] td { border-color: rgba(255, 255, 255, 0.12); }
  html[data-theme="dark"] th { background: #242933; }
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
        <FeedItemTime :item="item" />
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
