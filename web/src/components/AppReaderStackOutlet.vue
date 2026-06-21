<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import AppReaderStackContent from '@/components/AppReaderStackContent.vue'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type SourceNotice = {
  type: 'success' | 'warning'
  message: string
}

const props = withDefaults(
  defineProps<{
    sourceMounted?: boolean
    sourceUnderDetail?: boolean
    sourceStyle?: StyleValue
    sourceTitleRevealMounted?: boolean
    sourceTitleRevealVisible?: boolean
    sourceTitleRevealStyle?: StyleValue
    sourceTitle?: string
    sourceMeta?: string
    sourceNameMorphMounted?: boolean
    sourceNameMorphVisible?: boolean
    sourceNameMorphStyle?: StyleValue
    sourceNameMorphText?: string
    detailOpen?: boolean
    detailMotionSettling?: boolean
    detailReturningFeed?: boolean
    detailStyle?: StyleValue
    sourceNotice?: SourceNotice | null
    topChromePhase?: ChromePhase
    topChromeProgress?: number
    sourceHeaderStyle?: StyleValue
    sourceName?: string
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
    sourceRefreshNonce?: number
    sourceScrollTop?: number
    feedHeaderHeight?: number
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
    sourceBackgroundRefresh?: boolean
    detailEntrySettling?: boolean
    detailChromeSettling?: boolean
    detailTransitionStyle?: StyleValue
    detailItem?: FeedItem | null
    detailMorphVisible?: boolean
    detailMorphTextStyle?: StyleValue
    detailMorphSourceLabelStyle?: StyleValue
    detailDisplayDate?: string
    detailMorphSummaryVisible?: boolean
    detailPreviewSummary?: string
    detailContentStyle?: StyleValue
    detailLoading?: boolean
    detailError?: string
    detailSrcdoc?: string
    detailInlineSourceStyle?: StyleValue
    detailProgressVisible?: boolean
    detailProgressDragging?: boolean
    detailReadingProgress?: number
    detailProgressStyle?: StyleValue
    detailProgressFillStyle?: StyleValue
    detailProgressThumbStyle?: StyleValue
  }>(),
  {
    sourceMounted: false,
    sourceUnderDetail: false,
    sourceStyle: undefined,
    sourceTitleRevealMounted: false,
    sourceTitleRevealVisible: false,
    sourceTitleRevealStyle: undefined,
    sourceTitle: '',
    sourceMeta: '',
    sourceNameMorphMounted: false,
    sourceNameMorphVisible: false,
    sourceNameMorphStyle: undefined,
    sourceNameMorphText: '',
    detailOpen: false,
    detailMotionSettling: false,
    detailReturningFeed: false,
    detailStyle: undefined,
    sourceNotice: null,
    topChromePhase: 'visible',
    topChromeProgress: 1,
    sourceHeaderStyle: undefined,
    sourceName: '',
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
    sourceRefreshNonce: 0,
    sourceScrollTop: 0,
    feedHeaderHeight: 86,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
    sourceBackgroundRefresh: false,
    detailEntrySettling: false,
    detailChromeSettling: false,
    detailTransitionStyle: undefined,
    detailItem: null,
    detailMorphVisible: false,
    detailMorphTextStyle: undefined,
    detailMorphSourceLabelStyle: undefined,
    detailDisplayDate: '',
    detailMorphSummaryVisible: false,
    detailPreviewSummary: '',
    detailContentStyle: undefined,
    detailLoading: false,
    detailError: '',
    detailSrcdoc: '',
    detailInlineSourceStyle: undefined,
    detailProgressVisible: false,
    detailProgressDragging: false,
    detailReadingProgress: 0,
    detailProgressStyle: undefined,
    detailProgressFillStyle: undefined,
    detailProgressThumbStyle: undefined,
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
  (event: 'detail-content-ref', element: HTMLElement | null): void
  (event: 'detail-content-scroll', value: Event): void
  (event: 'detail-inline-source-ref', element: HTMLElement | null): void
  (event: 'detail-frame-ref', element: HTMLIFrameElement | null): void
  (event: 'detail-frame-load'): void
  (event: 'detail-progress-drag-start'): void
  (event: 'detail-progress-drag-end'): void
  (event: 'detail-progress-change', progress: number): void
}>()
</script>

<template>
  <AppReaderStackContent
    v-bind="props"
    @source-content-ref="(element) => emit('source-content-ref', element)"
    @source-content-scroll="(event) => emit('source-content-scroll', event)"
    @open-navigation="emit('open-navigation')"
    @toggle-source-subscription="emit('toggle-source-subscription')"
    @top-pull-start="(startedWithVisibleChrome) => emit('top-pull-start', startedWithVisibleChrome)"
    @top-pull-move="(distance) => emit('top-pull-move', distance)"
    @top-pull-end="(shouldRefresh) => emit('top-pull-end', shouldRefresh)"
    @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
    @detail-content-ref="(element) => emit('detail-content-ref', element)"
    @detail-content-scroll="(event) => emit('detail-content-scroll', event)"
    @detail-inline-source-ref="(element) => emit('detail-inline-source-ref', element)"
    @detail-frame-ref="(element) => emit('detail-frame-ref', element)"
    @detail-frame-load="emit('detail-frame-load')"
    @detail-progress-drag-start="emit('detail-progress-drag-start')"
    @detail-progress-drag-end="emit('detail-progress-drag-end')"
    @detail-progress-change="(progress) => emit('detail-progress-change', progress)"
  />
</template>
