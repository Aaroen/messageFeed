import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppTopChromeOutletState } from '@/composables/useAppTopChromeOutletState'
import type { PageViewExpose } from '@/composables/usePageOutletState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

type AppMainOutletBindingOptions = {
  mainClass: ReadableRef<ClassValue>
  mainStyle: ReadableRef<StyleValue>
  swipePhase: ReadableRef<string>
  swipeDirection: ReadableRef<string | null>
  swipeProgress: ReadableRef<number>
  swipeIsBlocked: ReadableRef<boolean>
  topChrome: AppTopChromeOutletState
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
    const topChrome = options.topChrome

    return {
      mainClass: options.mainClass.value,
      mainStyle: options.mainStyle.value,
      swipePhase: options.swipePhase.value,
      swipeDirection: options.swipeDirection.value,
      swipeProgress: options.swipeProgress.value,
      swipeIsBlocked: options.swipeIsBlocked.value,
      topChromePhase: topChrome.phase.value,
      feedHeaderProgress: topChrome.progress.value,
      headerClass: topChrome.rootClass.value,
      headerStyle: topChrome.rootStyle.value,
      feedHeaderActive: topChrome.feedHeaderActive.value,
      detailReaderOpen: topChrome.detailReaderOpen.value,
      detailHeaderVisible: topChrome.detailHeaderVisible.value,
      detailHeaderLayerStyle: topChrome.detailHeaderLayerStyle.value,
      detailTitle: topChrome.detailTitle.value,
      detailHeaderTitleStyle: topChrome.detailHeaderTitleStyle.value,
      detailHeaderPreviousTitle: topChrome.detailHeaderPreviousTitle.value,
      detailHeaderPreviousTextStyle: topChrome.detailHeaderPreviousTextStyle.value,
      detailHeaderCurrentTextStyle: topChrome.detailHeaderCurrentTextStyle.value,
      isFeedRoute: topChrome.isFeedRoute.value,
      feedTabs: topChrome.feedTabs,
      activeKey: topChrome.activeKey.value,
      feedTabsLayerHidden: topChrome.feedTabsLayerHidden.value,
      feedTabsLayerStyle: topChrome.feedTabsLayerStyle.value,
      viewSwipeTargetVisible: topChrome.viewSwipeTargetVisible.value,
      feedTabsTargetLayerStyle: topChrome.feedTabsTargetLayerStyle.value,
      viewSwipeTargetKey: topChrome.viewSwipeTargetKey.value,
      feedPullActive: topChrome.feedPullActive.value,
      feedPullRefreshing: topChrome.feedPullRefreshing.value,
      pullStatusStyle: topChrome.pullStatusStyle.value,
      pullIconStyle: topChrome.pullIconStyle.value,
      pullStatusText: topChrome.pullStatusText.value,
      pullStatusMeta: topChrome.pullStatusMeta.value,
      pageTitle: topChrome.pageTitle.value,
      pagePullActive: topChrome.pagePullActive.value,
      pageTitleLayerStyle: topChrome.pageTitleLayerStyle.value,
      pagePullStatusStyle: topChrome.pagePullStatusStyle.value,
      pagePullRefreshing: topChrome.pagePullRefreshing.value,
      pagePullIconStyle: topChrome.pagePullIconStyle.value,
      pagePullStatusText: topChrome.pagePullStatusText.value,
      pagePullStatusMeta: topChrome.pagePullStatusMeta.value,
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
