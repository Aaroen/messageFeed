<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { PageViewExpose } from '@/composables/usePageOutletState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

withDefaults(
  defineProps<{
    contentStyle?: StyleValue
    innerStyle?: StyleValue
  }>(),
  {
    contentStyle: undefined,
    innerStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'content-ref', element: HTMLElement | null): void
  (event: 'view-ref', view: PageViewExpose | null): void
  (event: 'content-scroll', value: Event): void
  (event: 'touch-start', value: TouchEvent): void
  (event: 'touch-move', value: TouchEvent): void
  (event: 'touch-end', value: TouchEvent): void
  (event: 'touch-cancel', value: TouchEvent): void
  (event: 'open-source', source: ReaderSource): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()

type ComponentRefWithExpose = ComponentPublicInstance & {
  $?: {
    exposeProxy?: unknown
    exposed?: unknown
  }
}

function hasPageViewExpose(value: unknown): value is PageViewExpose {
  if (!value || typeof value !== 'object') {
    return false
  }

  const candidate = value as PageViewExpose
  return typeof candidate.refreshPage === 'function' || typeof candidate.clearNotice === 'function'
}

function resolvePageViewExpose(value: Element | ComponentPublicInstance | null) {
  if (!value || value instanceof Element) {
    return null
  }

  if (hasPageViewExpose(value)) {
    return value
  }

  const internal = (value as ComponentRefWithExpose).$
  if (hasPageViewExpose(internal?.exposeProxy)) {
    return internal.exposeProxy
  }
  if (hasPageViewExpose(internal?.exposed)) {
    return internal.exposed
  }

  return null
}

function setContentRef(value: Element | ComponentPublicInstance | null) {
  emit('content-ref', value instanceof HTMLElement ? value : null)
}

function setViewRef(value: Element | ComponentPublicInstance | null) {
  emit('view-ref', resolvePageViewExpose(value))
}

function handleOpenSource(source: ReaderSource) {
  emit('open-source', source)
}

function handleOpenItem(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
  emit('open-item', item, sourceKind, originRect)
}
</script>

<template>
  <section
    :ref="setContentRef"
    class="app-content app-content--page"
    :style="contentStyle"
    @scroll.passive="(event) => emit('content-scroll', event)"
    @touchstart.passive="(event) => emit('touch-start', event)"
    @touchmove="(event) => emit('touch-move', event)"
    @touchend.passive="(event) => emit('touch-end', event)"
    @touchcancel.passive="(event) => emit('touch-cancel', event)"
  >
    <div class="page-content-inner" :style="innerStyle">
      <router-view v-slot="{ Component }">
        <component :is="Component" :ref="setViewRef" @open-source="handleOpenSource" @open-item="handleOpenItem" />
      </router-view>
    </div>
  </section>
</template>
