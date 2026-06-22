<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'

withDefaults(
  defineProps<{
    item?: FeedItem | null
    loading?: boolean
    error?: string
    displayDate?: string
    srcdoc?: string
    inlineMetaStyle?: StyleValue
    inlineSourceStyle?: StyleValue
    frameStyle?: StyleValue
    actionsStyle?: StyleValue
  }>(),
  {
    item: null,
    loading: false,
    error: '',
    displayDate: '',
    srcdoc: '',
    inlineMetaStyle: undefined,
    inlineSourceStyle: undefined,
    frameStyle: undefined,
    actionsStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'inline-source-ref', element: HTMLElement | null): void
  (event: 'frame-ref', element: HTMLIFrameElement | null): void
  (event: 'frame-load'): void
}>()

function domElement(value: Element | ComponentPublicInstance | null) {
  return value instanceof HTMLElement ? value : null
}

function setInlineSourceRef(value: Element | ComponentPublicInstance | null) {
  emit('inline-source-ref', domElement(value))
}

function setFrameRef(value: Element | ComponentPublicInstance | null) {
  const element = domElement(value)
  emit('frame-ref', element instanceof HTMLIFrameElement ? element : null)
}
</script>

<template>
  <a-alert v-if="error" type="warning" show-icon :content="error" />
  <section v-if="item" class="reader-detail__surface">
    <div class="reader-detail__inline-meta" :style="inlineMetaStyle">
      <span
        :ref="setInlineSourceRef"
        class="reader-detail__inline-source"
        :style="inlineSourceStyle"
      >
        {{ item.source_name || '未知来源' }}
      </span>
      <span>{{ displayDate }}</span>
    </div>
    <iframe
      :ref="setFrameRef"
      class="reader-detail__frame"
      :style="frameStyle"
      title="条目正文"
      sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox"
      :srcdoc="srcdoc"
      @load="emit('frame-load')"
    />
    <div class="reader-detail__actions" :style="actionsStyle">
      <a :href="item.url" target="_blank" rel="noreferrer">阅读原文</a>
    </div>
  </section>
  <section v-else class="empty-surface">
    <div class="empty-surface__mark">读</div>
    <h2>{{ loading ? '正在加载条目' : '暂无条目内容' }}</h2>
    <p>请稍候。</p>
  </section>
</template>
