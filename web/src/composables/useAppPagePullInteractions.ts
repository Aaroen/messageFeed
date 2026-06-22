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
  clearCurrentPageNotice: () => void
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
    clearCurrentPageNotice: options.clearCurrentPageNotice,
    beginRefreshing: pullRefresh.beginRefreshing,
    holdCompletionRefreshing: options.pagePull.holdCompletionRefreshing,
    releaseCompletionRefreshing: options.pagePull.releaseCompletionRefreshing,
    settleRefreshCompletion: pullRefresh.settleRefreshCompletion,
    collapseTopChrome: options.collapseTopChrome,
  })

  const gestureHandlers = usePagePullGestureHandlers({
    isFeedRoute: options.isFeedRoute,
    refreshThreshold: pullRefresh.threshold,
    pullRefresh,
    currentContentScrollTop: options.currentContentScrollTop,
    hasRefreshPage: options.hasRefreshPage,
    clearCurrentPageNotice: options.clearCurrentPageNotice,
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
    invalidateRefreshCompletion: refreshAction.invalidateRefreshCompletion,
    resetPageTopPullTracking: gestureHandlers.resetPageTopPullTracking,
    handlePageTouchStart: gestureHandlers.handlePageTouchStart,
    handlePageTouchMove: gestureHandlers.handlePageTouchMove,
    handlePageTouchEnd: gestureHandlers.handlePageTouchEnd,
    handlePageTouchCancel: gestureHandlers.handlePageTouchCancel,
  }
}
