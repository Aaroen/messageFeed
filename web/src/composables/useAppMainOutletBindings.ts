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
  feedContentStyle: ReadableRef<StyleValue>
  pageContentStyle: ReadableRef<StyleValue>
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
      topChrome: {
        chrome: {
          phase: topChrome.chrome.phase.value,
          progress: topChrome.chrome.progress.value,
          rootClass: topChrome.chrome.rootClass.value,
          rootStyle: topChrome.chrome.rootStyle.value,
        },
        feed: {
          active: topChrome.feed.active.value,
          detailReaderOpen: topChrome.feed.detailReaderOpen.value,
          detailHeaderVisible: topChrome.feed.detailHeaderVisible.value,
          detailHeaderLayerStyle: topChrome.feed.detailHeaderLayerStyle.value,
          detailTitle: topChrome.feed.detailTitle.value,
          detailHeaderTitleStyle: topChrome.feed.detailHeaderTitleStyle.value,
          detailHeaderPreviousTitle: topChrome.feed.detailHeaderPreviousTitle.value,
          detailHeaderPreviousTextStyle: topChrome.feed.detailHeaderPreviousTextStyle.value,
          detailHeaderCurrentTextStyle: topChrome.feed.detailHeaderCurrentTextStyle.value,
          isFeedRoute: topChrome.feed.isFeedRoute.value,
          feedTabs: topChrome.feed.feedTabs,
          activeKey: topChrome.feed.activeKey.value,
          feedTabsLayerHidden: topChrome.feed.feedTabsLayerHidden.value,
          feedTabsLayerStyle: topChrome.feed.feedTabsLayerStyle.value,
          viewSwipeTargetVisible: topChrome.feed.viewSwipeTargetVisible.value,
          feedTabsTargetLayerStyle: topChrome.feed.feedTabsTargetLayerStyle.value,
          viewSwipeTargetKey: topChrome.feed.viewSwipeTargetKey.value,
          feedPullActive: topChrome.feed.feedPullActive.value,
          feedPullRefreshing: topChrome.feed.feedPullRefreshing.value,
          pullStatusStyle: topChrome.feed.pullStatusStyle.value,
          pullIconStyle: topChrome.feed.pullIconStyle.value,
          pullStatusText: topChrome.feed.pullStatusText.value,
          pullStatusMeta: topChrome.feed.pullStatusMeta.value,
        },
        page: {
          title: topChrome.page.title.value,
          pullActive: topChrome.page.pullActive.value,
          titleLayerStyle: topChrome.page.titleLayerStyle.value,
          pullStatusStyle: topChrome.page.pullStatusStyle.value,
          pullRefreshing: topChrome.page.pullRefreshing.value,
          pullIconStyle: topChrome.page.pullIconStyle.value,
          pullStatusText: topChrome.page.pullStatusText.value,
          pullStatusMeta: topChrome.page.pullStatusMeta.value,
        },
      },
      sourceReaderOpen: options.sourceReaderOpen.value,
      feedContentStyle: options.feedContentStyle.value,
      pageContentStyle: options.pageContentStyle.value,
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
