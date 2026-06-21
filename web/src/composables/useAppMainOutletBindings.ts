import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppReaderDetailHeaderState } from '@/composables/useAppReaderDetailHeaderState'
import type { PageViewExpose } from '@/composables/usePageOutletState'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

type FeedTab = {
  key: string
  label: string
  path: string
}

type AppMainOutletBindingOptions = {
  mainClass: ReadableRef<ClassValue>
  mainStyle: ReadableRef<StyleValue>
  swipePhase: ReadableRef<string>
  swipeDirection: ReadableRef<string | null>
  swipeProgress: ReadableRef<number>
  swipeIsBlocked: ReadableRef<boolean>
  topChromePhase: ReadableRef<ChromePhase>
  feedHeaderProgress: ReadableRef<number>
  headerClass: ReadableRef<ClassValue>
  headerStyle: ReadableRef<StyleValue>
  isFeedRoute: ReadableRef<boolean>
  readerDetailHeader: AppReaderDetailHeaderState
  feedTabs: FeedTab[]
  activeKey: () => string | symbol | null | undefined
  feedTabsLayerHidden: ReadableRef<boolean>
  feedTabsLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetVisible: ReadableRef<boolean>
  feedTabsTargetLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetKey: ReadableRef<string | null>
  feedPullActive: ReadableRef<boolean>
  feedPullRefreshing: () => boolean
  pullStatusStyle: ReadableRef<StyleValue>
  pullIconStyle: ReadableRef<StyleValue>
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  pageTitle: ReadableRef<string>
  pagePullActive: ReadableRef<boolean>
  pageTitleLayerStyle: ReadableRef<StyleValue>
  pagePullStatusStyle: ReadableRef<StyleValue>
  pagePullRefreshing: ReadableRef<boolean>
  pagePullIconStyle: ReadableRef<StyleValue>
  pagePullStatusText: ReadableRef<string>
  pagePullStatusMeta: ReadableRef<string>
  sourceReaderOpen: ReadableRef<boolean>
  viewSettling: ReadableRef<boolean>
  feedTrackStyle: ReadableRef<StyleValue>
  feedScrollTop: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  freezeFeedBodyDuringTopRefresh: ReadableRef<boolean>
  morphingItemId: ReadableRef<number | null>
  morphingHeightLockItemId: ReadableRef<number | null>
  morphingItemHeight: ReadableRef<number | null>
  feedItemPreviewProgress: ReadableRef<number>
  pageContentInnerStyle: ReadableRef<StyleValue>
  navigateTo: (path: string) => void
  setFeedContentElement: (element: HTMLElement | null) => void
  handleFeedContentScroll: (event: Event) => void
  handleFeedPointerDown: (event: PointerEvent) => void
  handleFeedPointerMove: (event: PointerEvent) => void
  handleFeedPointerUp: (event: PointerEvent) => void
  handleFeedPointerCancel: (event: PointerEvent) => void
  handleFeedTopPullStart: (startedWithVisibleChrome: boolean) => void
  handleFeedTopPullMove: (distance: number) => void
  handleFeedTopPullEnd: (shouldRefresh: boolean) => void
  openItemReader: (item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) => void
  setPageContentElement: (element: HTMLElement | null) => void
  setPageViewInstance: (view: PageViewExpose | null) => void
  handlePageContentScroll: (event: Event) => void
  handlePageTouchStart: (event: TouchEvent) => void
  handlePageTouchMove: (event: TouchEvent) => void
  handlePageTouchEnd: (event: TouchEvent) => void
  handlePageTouchCancel: (event: TouchEvent) => void
  openSourceReader: (source: ReaderSource) => void
}

export function useAppMainOutletBindings(options: AppMainOutletBindingOptions) {
  const props = computed(() => {
    const readerDetailHeader = options.readerDetailHeader

    return {
      mainClass: options.mainClass.value,
      mainStyle: options.mainStyle.value,
      swipePhase: options.swipePhase.value,
      swipeDirection: options.swipeDirection.value,
      swipeProgress: options.swipeProgress.value,
      swipeIsBlocked: options.swipeIsBlocked.value,
      topChromePhase: options.topChromePhase.value,
      feedHeaderProgress: options.feedHeaderProgress.value,
      headerClass: options.headerClass.value,
      headerStyle: options.headerStyle.value,
      feedHeaderActive: options.isFeedRoute.value || readerDetailHeader.chromeVisible.value,
      detailReaderOpen: readerDetailHeader.readerOpen.value,
      detailHeaderVisible: readerDetailHeader.visible.value,
      detailHeaderLayerStyle: readerDetailHeader.layerStyle.value,
      detailTitle: readerDetailHeader.title.value,
      detailHeaderTitleStyle: readerDetailHeader.titleStyle.value,
      detailHeaderPreviousTitle: readerDetailHeader.previousTitle.value,
      detailHeaderPreviousTextStyle: readerDetailHeader.previousTextStyle.value,
      detailHeaderCurrentTextStyle: readerDetailHeader.currentTextStyle.value,
      isFeedRoute: options.isFeedRoute.value,
      feedTabs: options.feedTabs,
      activeKey: options.activeKey() ?? null,
      feedTabsLayerHidden: options.feedTabsLayerHidden.value,
      feedTabsLayerStyle: options.feedTabsLayerStyle.value,
      viewSwipeTargetVisible: options.viewSwipeTargetVisible.value,
      feedTabsTargetLayerStyle: options.feedTabsTargetLayerStyle.value,
      viewSwipeTargetKey: options.viewSwipeTargetKey.value,
      feedPullActive: options.feedPullActive.value,
      feedPullRefreshing: options.feedPullRefreshing(),
      pullStatusStyle: options.pullStatusStyle.value,
      pullIconStyle: options.pullIconStyle.value,
      pullStatusText: options.pullStatusText.value,
      pullStatusMeta: options.pullStatusMeta.value,
      pageTitle: options.pageTitle.value,
      pagePullActive: options.pagePullActive.value,
      pageTitleLayerStyle: options.pageTitleLayerStyle.value,
      pagePullStatusStyle: options.pagePullStatusStyle.value,
      pagePullRefreshing: options.pagePullRefreshing.value,
      pagePullIconStyle: options.pagePullIconStyle.value,
      pagePullStatusText: options.pagePullStatusText.value,
      pagePullStatusMeta: options.pagePullStatusMeta.value,
      sourceReaderOpen: options.sourceReaderOpen.value,
      viewSettling: options.viewSettling.value,
      feedTrackStyle: options.feedTrackStyle.value,
      feedScrollTop: options.feedScrollTop.value,
      topChromeProgress: options.topChromeProgress.value,
      feedHeaderHeight: options.feedHeaderHeight.value,
      freezeBodyDuringTopRefresh: options.freezeFeedBodyDuringTopRefresh.value,
      morphingItemId: options.morphingItemId.value,
      morphingHeightLockItemId: options.morphingHeightLockItemId.value,
      morphingItemHeight: options.morphingItemHeight.value,
      feedItemPreviewProgress: options.feedItemPreviewProgress.value,
      pageContentInnerStyle: options.pageContentInnerStyle.value,
    }
  })

  const listeners = {
    navigate: options.navigateTo,
    'feed-content-ref': options.setFeedContentElement,
    'feed-content-scroll': options.handleFeedContentScroll,
    'feed-pointer-down': options.handleFeedPointerDown,
    'feed-pointer-move': options.handleFeedPointerMove,
    'feed-pointer-up': options.handleFeedPointerUp,
    'feed-pointer-cancel': options.handleFeedPointerCancel,
    'feed-top-pull-start': options.handleFeedTopPullStart,
    'feed-top-pull-move': options.handleFeedTopPullMove,
    'feed-top-pull-end': options.handleFeedTopPullEnd,
    'open-item': options.openItemReader,
    'page-content-ref': options.setPageContentElement,
    'page-view-ref': options.setPageViewInstance,
    'page-content-scroll': options.handlePageContentScroll,
    'page-touch-start': options.handlePageTouchStart,
    'page-touch-move': options.handlePageTouchMove,
    'page-touch-end': options.handlePageTouchEnd,
    'page-touch-cancel': options.handlePageTouchCancel,
    'open-source': options.openSourceReader,
  }

  return {
    props,
    listeners,
  }
}
