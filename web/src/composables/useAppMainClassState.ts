import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type AppMainClassStateOptions = {
  isFeedRoute: ReadableRef<boolean>
  feedChromeHidden: ReadableRef<boolean>
  feedPullActive: ReadableRef<boolean>
  feedPullRefreshing: () => boolean
  pagePullActive: ReadableRef<boolean>
  freezeFeedBodyDuringTopRefresh: ReadableRef<boolean>
  feedRefreshSettling: ReadableRef<boolean>
  feedChromeSettling: ReadableRef<boolean>
  readerBackDragging: ReadableRef<boolean>
  viewSettling: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailReturningToFeed: ReadableRef<boolean>
  detailChromeVisible: ReadableRef<boolean>
}

export function useAppMainClassState(options: AppMainClassStateOptions) {
  const mainClass = computed(() => ({
    'app-main--feed': options.isFeedRoute.value,
    'app-main--page': !options.isFeedRoute.value,
    'app-main--tabs-hidden': options.feedChromeHidden.value,
    'app-main--refreshing': options.feedPullActive.value || options.pagePullActive.value,
    'app-main--pull-dragging': options.feedPullActive.value && !options.feedPullRefreshing(),
    'app-main--top-refresh-contained': options.freezeFeedBodyDuringTopRefresh.value,
    'app-main--refresh-settling': options.feedRefreshSettling.value,
    'app-main--chrome-settling': options.feedChromeSettling.value && !options.readerBackDragging.value,
    'app-main--view-settling': options.viewSettling.value,
    'app-main--detail-reader': options.detailReaderOpen.value && !options.detailReturningToFeed.value,
    'app-main--detail-chrome': options.detailChromeVisible.value,
  }))

  return {
    mainClass,
  }
}
