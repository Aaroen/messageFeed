<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import AppFeedOutlet from '@/components/AppFeedOutlet.vue'
import AppPageOutlet from '@/components/AppPageOutlet.vue'
import AppTopChromeOutlet from '@/components/AppTopChromeOutlet.vue'
import type { PageViewExpose } from '@/composables/usePageOutletState'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

type FeedTab = {
  key: string
  label: string
  path: string
}

withDefaults(
  defineProps<{
    mainClass?: ClassValue
    mainStyle?: StyleValue
    swipePhase?: string
    swipeDirection?: string | null
    swipeProgress?: number
    swipeIsBlocked?: boolean
    topChromePhase?: ChromePhase
    feedHeaderProgress?: number
    headerClass?: ClassValue
    headerStyle?: StyleValue
    feedHeaderActive?: boolean
    detailReaderOpen?: boolean
    detailHeaderVisible?: boolean
    detailHeaderLayerStyle?: StyleValue
    detailTitle?: string
    detailHeaderTitleStyle?: StyleValue
    detailHeaderPreviousTitle?: string
    detailHeaderPreviousTextStyle?: StyleValue
    detailHeaderCurrentTextStyle?: StyleValue
    isFeedRoute?: boolean
    feedTabs?: FeedTab[]
    activeKey?: string | symbol | null
    feedTabsLayerHidden?: boolean
    feedTabsLayerStyle?: StyleValue
    viewSwipeTargetVisible?: boolean
    feedTabsTargetLayerStyle?: StyleValue
    viewSwipeTargetKey?: string | null
    feedPullActive?: boolean
    feedPullRefreshing?: boolean
    pullStatusStyle?: StyleValue
    pullIconStyle?: StyleValue
    pullStatusText?: string
    pullStatusMeta?: string
    pageTitle?: string
    pagePullActive?: boolean
    pageTitleLayerStyle?: StyleValue
    pagePullStatusStyle?: StyleValue
    pagePullRefreshing?: boolean
    pagePullIconStyle?: StyleValue
    pagePullStatusText?: string
    pagePullStatusMeta?: string
    sourceReaderOpen?: boolean
    viewSettling?: boolean
    feedTrackStyle?: StyleValue
    feedScrollTop?: number
    topChromeProgress?: number
    feedHeaderHeight?: number
    freezeBodyDuringTopRefresh?: boolean
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
    pageContentInnerStyle?: StyleValue
  }>(),
  {
    mainClass: '',
    mainStyle: undefined,
    swipePhase: 'idle',
    swipeDirection: null,
    swipeProgress: 0,
    swipeIsBlocked: false,
    topChromePhase: 'visible',
    feedHeaderProgress: 1,
    headerClass: '',
    headerStyle: undefined,
    feedHeaderActive: false,
    detailReaderOpen: false,
    detailHeaderVisible: false,
    detailHeaderLayerStyle: undefined,
    detailTitle: '',
    detailHeaderTitleStyle: undefined,
    detailHeaderPreviousTitle: '',
    detailHeaderPreviousTextStyle: undefined,
    detailHeaderCurrentTextStyle: undefined,
    isFeedRoute: false,
    feedTabs: () => [],
    activeKey: null,
    feedTabsLayerHidden: true,
    feedTabsLayerStyle: undefined,
    viewSwipeTargetVisible: false,
    feedTabsTargetLayerStyle: undefined,
    viewSwipeTargetKey: null,
    feedPullActive: false,
    feedPullRefreshing: false,
    pullStatusStyle: undefined,
    pullIconStyle: undefined,
    pullStatusText: '',
    pullStatusMeta: '',
    pageTitle: '',
    pagePullActive: false,
    pageTitleLayerStyle: undefined,
    pagePullStatusStyle: undefined,
    pagePullRefreshing: false,
    pagePullIconStyle: undefined,
    pagePullStatusText: '',
    pagePullStatusMeta: '',
    sourceReaderOpen: false,
    viewSettling: false,
    feedTrackStyle: undefined,
    feedScrollTop: 0,
    topChromeProgress: 1,
    feedHeaderHeight: 86,
    freezeBodyDuringTopRefresh: false,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
    pageContentInnerStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'navigate', path: string): void
  (event: 'feed-content-ref', element: HTMLElement | null): void
  (event: 'feed-content-scroll', value: Event): void
  (event: 'feed-pointer-down', value: PointerEvent): void
  (event: 'feed-pointer-move', value: PointerEvent): void
  (event: 'feed-pointer-up', value: PointerEvent): void
  (event: 'feed-pointer-cancel', value: PointerEvent): void
  (event: 'feed-top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'feed-top-pull-move', distance: number): void
  (event: 'feed-top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
  (event: 'page-content-ref', element: HTMLElement | null): void
  (event: 'page-view-ref', view: PageViewExpose | null): void
  (event: 'page-content-scroll', value: Event): void
  (event: 'page-touch-start', value: TouchEvent): void
  (event: 'page-touch-move', value: TouchEvent): void
  (event: 'page-touch-end', value: TouchEvent): void
  (event: 'page-touch-cancel', value: TouchEvent): void
  (event: 'open-source', source: ReaderSource): void
}>()
</script>

<template>
  <main
    class="app-main"
    :class="mainClass"
    :style="mainStyle"
    :data-swipe-phase="swipePhase"
    :data-swipe-direction="swipeDirection || undefined"
    :data-swipe-progress="swipeProgress.toFixed(3)"
    :data-swipe-blocked="swipeIsBlocked ? 'true' : undefined"
  >
    <AppTopChromeOutlet
      :phase="topChromePhase"
      :progress="feedHeaderProgress"
      :root-class="headerClass"
      :root-style="headerStyle"
      :feed-header-active="feedHeaderActive"
      :detail-reader-open="detailReaderOpen"
      :detail-header-visible="detailHeaderVisible"
      :detail-header-layer-style="detailHeaderLayerStyle"
      :detail-title="detailTitle"
      :detail-header-title-style="detailHeaderTitleStyle"
      :detail-header-previous-title="detailHeaderPreviousTitle"
      :detail-header-previous-text-style="detailHeaderPreviousTextStyle"
      :detail-header-current-text-style="detailHeaderCurrentTextStyle"
      :is-feed-route="isFeedRoute"
      :feed-tabs="feedTabs"
      :active-key="activeKey"
      :feed-tabs-layer-hidden="feedTabsLayerHidden"
      :feed-tabs-layer-style="feedTabsLayerStyle"
      :view-swipe-target-visible="viewSwipeTargetVisible"
      :feed-tabs-target-layer-style="feedTabsTargetLayerStyle"
      :view-swipe-target-key="viewSwipeTargetKey"
      :feed-pull-active="feedPullActive"
      :pull-status-style="pullStatusStyle"
      :pull-refreshing="feedPullRefreshing"
      :pull-icon-style="pullIconStyle"
      :pull-status-text="pullStatusText"
      :pull-status-meta="pullStatusMeta"
      :page-title="pageTitle"
      :page-pull-active="pagePullActive"
      :page-title-layer-style="pageTitleLayerStyle"
      :page-pull-status-style="pagePullStatusStyle"
      :page-pull-refreshing="pagePullRefreshing"
      :page-pull-icon-style="pagePullIconStyle"
      :page-pull-status-text="pagePullStatusText"
      :page-pull-status-meta="pagePullStatusMeta"
      @navigate="(path) => emit('navigate', path)"
    />

    <AppFeedOutlet
      v-if="isFeedRoute"
      :active-key="activeKey"
      :detail-reader-open="detailReaderOpen"
      :source-reader-open="sourceReaderOpen"
      :view-settling="viewSettling"
      :feed-track-style="feedTrackStyle"
      :feed-scroll-top="feedScrollTop"
      :top-chrome-progress="topChromeProgress"
      :feed-header-height="feedHeaderHeight"
      :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
      :morphing-item-id="morphingItemId"
      :morphing-height-lock-item-id="morphingHeightLockItemId"
      :morphing-item-height="morphingItemHeight"
      :feed-item-preview-progress="feedItemPreviewProgress"
      @content-ref="(element) => emit('feed-content-ref', element)"
      @content-scroll="(event) => emit('feed-content-scroll', event)"
      @pointer-down="(event) => emit('feed-pointer-down', event)"
      @pointer-move="(event) => emit('feed-pointer-move', event)"
      @pointer-up="(event) => emit('feed-pointer-up', event)"
      @pointer-cancel="(event) => emit('feed-pointer-cancel', event)"
      @top-pull-start="(startedWithVisibleChrome) => emit('feed-top-pull-start', startedWithVisibleChrome)"
      @top-pull-move="(distance) => emit('feed-top-pull-move', distance)"
      @top-pull-end="(shouldRefresh) => emit('feed-top-pull-end', shouldRefresh)"
      @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
    />
    <AppPageOutlet
      v-else
      :inner-style="pageContentInnerStyle"
      @content-ref="(element) => emit('page-content-ref', element)"
      @view-ref="(view) => emit('page-view-ref', view)"
      @content-scroll="(event) => emit('page-content-scroll', event)"
      @touch-start="(event) => emit('page-touch-start', event)"
      @touch-move="(event) => emit('page-touch-move', event)"
      @touch-end="(event) => emit('page-touch-end', event)"
      @touch-cancel="(event) => emit('page-touch-cancel', event)"
      @open-source="(source) => emit('open-source', source)"
    />
  </main>
</template>
