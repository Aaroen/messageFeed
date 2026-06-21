import type { PageRefreshOptions } from '@/composables/usePageOutletState'
import type { AppPagePullState } from '@/composables/useAppPagePullState'
import { usePagePullGestureHandlers } from '@/composables/usePagePullGestureHandlers'
import { usePagePullRefreshAction } from '@/composables/usePagePullRefreshAction'

type ReadableRef<T> = {
  readonly value: T
}

type PageRefreshAction = (options?: PageRefreshOptions) => Promise<void> | void

type AppPagePullInteractionsOptions = {
  pagePull: AppPagePullState
  isFeedRoute: ReadableRef<boolean>
  topRefreshNoticeDelay: number
  currentRefreshPage: () => PageRefreshAction | null
  hasRefreshPage: () => boolean
  currentContentScrollTop: () => number
  isControlTarget: (target: EventTarget | null) => boolean
  shouldCancelTopPull: (deltaX: number, deltaY: number) => boolean
  shouldWaitForTopPull: (deltaX: number, deltaY: number) => boolean
  setTopChromeVisible: (visible: boolean) => void
  finishFeedTopPull: () => void
  settlePullOffset: () => void
  collapseTopChrome: () => void
}

export function useAppPagePullInteractions(options: AppPagePullInteractionsOptions) {
  const pullRefresh = options.pagePull.pullRefresh
  const refreshAction = usePagePullRefreshAction({
    refreshing: options.pagePull.refreshing,
    noticeDelayMS: options.topRefreshNoticeDelay,
    currentRefreshPage: options.currentRefreshPage,
    beginRefreshing: pullRefresh.beginRefreshing,
    settleRefreshCompletion: pullRefresh.settleRefreshCompletion,
    collapseTopChrome: options.collapseTopChrome,
  })

  const gestureHandlers = usePagePullGestureHandlers({
    isFeedRoute: options.isFeedRoute,
    refreshThreshold: pullRefresh.threshold,
    pullRefresh,
    currentContentScrollTop: options.currentContentScrollTop,
    hasRefreshPage: options.hasRefreshPage,
    isControlTarget: options.isControlTarget,
    shouldCancelTopPull: options.shouldCancelTopPull,
    shouldWaitForTopPull: options.shouldWaitForTopPull,
    setTopChromeVisible: options.setTopChromeVisible,
    finishFeedTopPull: options.finishFeedTopPull,
    settlePullOffset: options.settlePullOffset,
    refreshCurrentPageFromPull: refreshAction.refreshCurrentPageFromPull,
  })

  return {
    refreshCurrentPageFromPull: refreshAction.refreshCurrentPageFromPull,
    resetPageTopPullTracking: gestureHandlers.resetPageTopPullTracking,
    handlePageTouchStart: gestureHandlers.handlePageTouchStart,
    handlePageTouchMove: gestureHandlers.handlePageTouchMove,
    handlePageTouchEnd: gestureHandlers.handlePageTouchEnd,
    handlePageTouchCancel: gestureHandlers.handlePageTouchCancel,
  }
}
