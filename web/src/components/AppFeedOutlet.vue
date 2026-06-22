<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind } from '@/composables/useReaderSession'
import FeedPager from '@/components/FeedPager.vue'

withDefaults(
  defineProps<{
    activeKey?: string | symbol | null
    detailReaderOpen?: boolean
    sourceReaderOpen?: boolean
    contentStyle?: StyleValue
    feedTrackStyle?: StyleValue
    feedScrollTop?: number
    topChromeProgress?: number
    topChromeContentCollapsed?: boolean
    feedHeaderHeight?: number
    freezeBodyDuringTopRefresh?: boolean
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
  }>(),
  {
    activeKey: null,
    detailReaderOpen: false,
    sourceReaderOpen: false,
    contentStyle: undefined,
    feedTrackStyle: undefined,
    feedScrollTop: 0,
    topChromeProgress: 1,
    topChromeContentCollapsed: false,
    feedHeaderHeight: 86,
    freezeBodyDuringTopRefresh: false,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
  },
)

const emit = defineEmits<{
  (event: 'content-ref', element: HTMLElement | null): void
  (event: 'content-scroll', value: Event): void
  (event: 'pointer-down', value: PointerEvent): void
  (event: 'pointer-move', value: PointerEvent): void
  (event: 'pointer-up', value: PointerEvent): void
  (event: 'pointer-cancel', value: PointerEvent): void
  (event: 'top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'top-pull-move', distance: number): void
  (event: 'top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()

function setContentRef(value: Element | ComponentPublicInstance | null) {
  emit('content-ref', value instanceof HTMLElement ? value : null)
}
</script>

<template>
  <section
    :ref="setContentRef"
    class="app-content app-content--feed"
    :style="contentStyle"
    @scroll.passive="(event) => emit('content-scroll', event)"
    @pointerdown="(event) => emit('pointer-down', event)"
    @pointermove="(event) => emit('pointer-move', event)"
    @pointerup="(event) => emit('pointer-up', event)"
    @pointercancel="(event) => emit('pointer-cancel', event)"
  >
    <FeedPager
      :active-key="activeKey"
      :detail-reader-open="detailReaderOpen"
      :source-reader-open="sourceReaderOpen"
      :feed-track-style="feedTrackStyle"
      :feed-scroll-top="feedScrollTop"
      :top-chrome-progress="topChromeProgress"
      :top-chrome-content-collapsed="topChromeContentCollapsed"
      :feed-header-height="feedHeaderHeight"
      :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
      :morphing-item-id="morphingItemId"
      :morphing-height-lock-item-id="morphingHeightLockItemId"
      :morphing-item-height="morphingItemHeight"
      :feed-item-preview-progress="feedItemPreviewProgress"
      @top-pull-start="(startedWithVisibleChrome) => emit('top-pull-start', startedWithVisibleChrome)"
      @top-pull-move="(distance) => emit('top-pull-move', distance)"
      @top-pull-end="(shouldRefresh) => emit('top-pull-end', shouldRefresh)"
      @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
    />
  </section>
</template>
