<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import ReaderSourceOverlayContent from '@/components/ReaderSourceOverlayContent.vue'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type SourceNotice = {
  type: 'running' | 'success' | 'warning'
  message: string
}

withDefaults(
  defineProps<{
    sourceNotice?: SourceNotice | null
    topChromePhase?: ChromePhase
    topChromeProgress?: number
    topChromeContentCollapsed?: boolean
    sourceHeaderStyle?: StyleValue
    sourceChromeInteractive?: boolean
    sourceName?: string
    sourceMeta?: string
    sourceTitleTextStyle?: StyleValue
    sourceTitleLayerStyle?: StyleValue
    sourceMainLayerStyle?: StyleValue
    sourcePullStatusStyle?: StyleValue
    sourcePullIconStyle?: StyleValue
    sourcePullActive?: boolean
    sourcePullRefreshing?: boolean
    pullStatusText?: string
    pullStatusMeta?: string
    sourceToggleActive?: boolean
    sourceToggleLabel?: string
    sourceToggleDisabled?: boolean
    sourceContentStyle?: StyleValue
    readerSource?: ReaderSource | null
    sourceScrollTop?: number
    feedHeaderHeight?: number
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
    sourceBackgroundRefresh?: boolean
  }>(),
  {
    sourceNotice: null,
    topChromePhase: 'visible',
    topChromeProgress: 1,
    topChromeContentCollapsed: false,
    sourceHeaderStyle: undefined,
    sourceChromeInteractive: true,
    sourceName: '',
    sourceMeta: '',
    sourceTitleTextStyle: undefined,
    sourceTitleLayerStyle: undefined,
    sourceMainLayerStyle: undefined,
    sourcePullStatusStyle: undefined,
    sourcePullIconStyle: undefined,
    sourcePullActive: false,
    sourcePullRefreshing: false,
    pullStatusText: '',
    pullStatusMeta: '',
    sourceToggleActive: false,
    sourceToggleLabel: '',
    sourceToggleDisabled: false,
    sourceContentStyle: undefined,
    readerSource: null,
    sourceScrollTop: 0,
    feedHeaderHeight: 86,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
    sourceBackgroundRefresh: false,
  },
)

const emit = defineEmits<{
  (event: 'source-content-ref', element: HTMLElement | null): void
  (event: 'source-content-scroll', value: Event): void
  (event: 'open-navigation'): void
  (event: 'toggle-source-subscription'): void
  (event: 'top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'top-pull-move', distance: number): void
  (event: 'top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()
</script>

<template>
  <ReaderSourceOverlayContent
    :notice="sourceNotice"
    :top-chrome-phase="topChromePhase"
    :top-chrome-progress="topChromeProgress"
    :top-chrome-content-collapsed="topChromeContentCollapsed"
    :header-style="sourceHeaderStyle"
    :chrome-interactive="sourceChromeInteractive"
    :source-name="sourceName"
    :source-meta="sourceMeta"
    :title-text-style="sourceTitleTextStyle"
    :title-layer-style="sourceTitleLayerStyle"
    :main-layer-style="sourceMainLayerStyle"
    :pull-status-style="sourcePullStatusStyle"
    :pull-icon-style="sourcePullIconStyle"
    :pull-active="sourcePullActive"
    :pull-refreshing="sourcePullRefreshing"
    :pull-status-text="pullStatusText"
    :pull-status-meta="pullStatusMeta"
    :toggle-active="sourceToggleActive"
    :toggle-label="sourceToggleLabel"
    :toggle-disabled="sourceToggleDisabled"
    :content-style="sourceContentStyle"
    :reader-source="readerSource"
    :scroll-top="sourceScrollTop"
    :header-height="feedHeaderHeight"
    :morphing-item-id="morphingItemId"
    :morphing-height-lock-item-id="morphingHeightLockItemId"
    :morphing-item-height="morphingItemHeight"
    :morphing-preview-progress="feedItemPreviewProgress"
    :background-refresh="sourceBackgroundRefresh"
    @content-ref="(element) => emit('source-content-ref', element)"
    @content-scroll="(event) => emit('source-content-scroll', event)"
    @open-navigation="emit('open-navigation')"
    @toggle-subscription="emit('toggle-source-subscription')"
    @top-pull-start="(startedWithVisibleChrome) => emit('top-pull-start', startedWithVisibleChrome)"
    @top-pull-move="(distance) => emit('top-pull-move', distance)"
    @top-pull-end="(shouldRefresh) => emit('top-pull-end', shouldRefresh)"
    @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
  />
</template>
