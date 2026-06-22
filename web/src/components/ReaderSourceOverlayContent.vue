<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'
import { IconMenuUnfold } from '@arco-design/web-vue/es/icon'

import type { FeedItem } from '@/api/feed'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'
import RefreshStatusLayer from '@/components/RefreshStatusLayer.vue'
import ReaderSourceChromeContent from '@/components/ReaderSourceChromeContent.vue'
import ReaderSourceFeed from '@/components/ReaderSourceFeed.vue'
import ReaderSourceNotice from '@/components/ReaderSourceNotice.vue'
import TopChrome from '@/components/TopChrome.vue'

type SourceNotice = {
  type: 'running' | 'success' | 'warning'
  message: string
}

withDefaults(
  defineProps<{
    notice?: SourceNotice | null
    topChromePhase?: ChromePhase
    topChromeProgress?: number
    topChromeContentCollapsed?: boolean
    headerStyle?: StyleValue
    sourceName?: string
    sourceMeta?: string
    titleTextStyle?: StyleValue
    titleLayerStyle?: StyleValue
    mainLayerStyle?: StyleValue
    pullStatusStyle?: StyleValue
    pullIconStyle?: StyleValue
    pullActive?: boolean
    pullRefreshing?: boolean
    pullStatusText?: string
    pullStatusMeta?: string
    toggleActive?: boolean
    toggleLabel?: string
    toggleDisabled?: boolean
    contentStyle?: StyleValue
    readerSource?: ReaderSource | null
    scrollTop?: number
    headerHeight?: number
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    morphingPreviewProgress?: number
    backgroundRefresh?: boolean
  }>(),
  {
    notice: null,
    topChromePhase: 'visible',
    topChromeProgress: 1,
    topChromeContentCollapsed: false,
    headerStyle: undefined,
    sourceName: '',
    sourceMeta: '',
    titleTextStyle: undefined,
    titleLayerStyle: undefined,
    mainLayerStyle: undefined,
    pullStatusStyle: undefined,
    pullIconStyle: undefined,
    pullActive: false,
    pullRefreshing: false,
    pullStatusText: '',
    pullStatusMeta: '',
    toggleActive: false,
    toggleLabel: '',
    toggleDisabled: false,
    contentStyle: undefined,
    readerSource: null,
    scrollTop: 0,
    headerHeight: 86,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    morphingPreviewProgress: 0,
    backgroundRefresh: false,
  },
)

const emit = defineEmits<{
  (event: 'content-ref', element: HTMLElement | null): void
  (event: 'content-scroll', value: Event): void
  (event: 'open-navigation'): void
  (event: 'toggle-subscription'): void
  (event: 'top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'top-pull-move', distance: number): void
  (event: 'top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()

function domElement(value: Element | ComponentPublicInstance | null) {
  return value instanceof HTMLElement ? value : null
}

function setContentRef(value: Element | ComponentPublicInstance | null) {
  emit('content-ref', domElement(value))
}
</script>

<template>
  <ReaderSourceNotice :notice="notice" />
  <TopChrome
    variant="source"
    :phase="topChromePhase"
    :progress="topChromeProgress"
    :root-style="headerStyle"
  >
    <button class="reader-back-button" type="button" aria-label="打开导航" @click="emit('open-navigation')">
      <IconMenuUnfold />
    </button>
    <ReaderSourceChromeContent
      :source-name="sourceName"
      :source-meta="sourceMeta"
      :title-text-style="titleTextStyle"
      :title-layer-style="titleLayerStyle"
      :main-layer-style="mainLayerStyle"
      :main-hidden="pullActive"
      :toggle-active="toggleActive"
      :toggle-label="toggleLabel"
      :toggle-disabled="toggleDisabled"
      @toggle-subscription="emit('toggle-subscription')"
    />
    <RefreshStatusLayer
      root-class="reader-source-refresh-layer"
      :hidden="!pullActive"
      :root-style="pullStatusStyle"
      :refreshing="pullRefreshing"
      :icon-style="pullIconStyle"
      :title="pullStatusText"
      :meta="pullStatusMeta"
    />
  </TopChrome>
  <div
    :ref="setContentRef"
    class="reader-overlay__content reader-overlay__content--source"
    :style="contentStyle"
    @scroll.passive="(event) => emit('content-scroll', event)"
  >
    <ReaderSourceFeed
      :reader-source="readerSource"
      :scroll-top="scrollTop"
      :top-chrome-progress="topChromeProgress"
      :top-chrome-content-collapsed="topChromeContentCollapsed"
      :header-height="headerHeight"
      :morphing-item-id="morphingItemId"
      :morphing-height-lock-item-id="morphingHeightLockItemId"
      :morphing-item-height="morphingItemHeight"
      :morphing-preview-progress="morphingPreviewProgress"
      :background-refresh="backgroundRefresh"
      @top-pull-start="(startedWithVisibleChrome) => emit('top-pull-start', startedWithVisibleChrome)"
      @top-pull-move="(distance) => emit('top-pull-move', distance)"
      @top-pull-end="(shouldRefresh) => emit('top-pull-end', shouldRefresh)"
      @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
    />
  </div>
</template>
