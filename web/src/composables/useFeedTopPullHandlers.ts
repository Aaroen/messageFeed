import { clampProgress } from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type TopPullController = {
  readonly pulling: ReadableRef<boolean>
  readonly startedWithChrome: ReadableRef<boolean>
  readonly startProgress: ReadableRef<number>
  begin: (startedWithChrome: boolean, startProgress?: number) => void
  finish: () => void
  markStartedWithChrome: () => void
  resetStartedWithChrome: () => void
}

type FeedTopPullHandlersOptions = {
  isFeedRoute: ReadableRef<boolean>
  topPull: TopPullController
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  feedHeaderHeight: ReadableRef<number>
  feedPullRefreshing: () => boolean
  currentContentScrollTop: () => number
  beginRefreshingChrome: () => void
  setRefreshingProgress: (progress: number, options?: { contentCollapsed?: boolean }) => void
  commitRefreshingChrome: (startedWithChrome: boolean) => void
  recordRefreshStartedWithChrome: (startedWithChrome: boolean) => void
  collapseTopChrome: () => void
  setTopChromeVisible: (visible: boolean) => void
}

export function useFeedTopPullHandlers(options: FeedTopPullHandlersOptions) {
  function topChromeProgress() {
    return clampProgress(options.topChromeProgress.value)
  }

  function handleFeedTopPullStart(startedWithVisibleChrome = false) {
    if (options.isFeedRoute.value && options.feedPullRefreshing()) {
      return
    }

    const chromeConsumesContentSpace = !options.feedContentCollapsed.value
    const startedWithLayoutChrome = startedWithVisibleChrome || chromeConsumesContentSpace
    options.topPull.begin(startedWithLayoutChrome, startedWithLayoutChrome ? topChromeProgress() : 0)
    options.beginRefreshingChrome()
  }

  function handleFeedTopPullMove(distance: number) {
    if (!options.topPull.pulling.value || (options.isFeedRoute.value && options.feedPullRefreshing())) {
      return
    }

    if (!options.topPull.startedWithChrome.value && !options.feedContentCollapsed.value) {
      options.topPull.markStartedWithChrome()
    }

    if (!options.topPull.startedWithChrome.value && options.currentContentScrollTop() > 0) {
      return
    }

    if (options.topPull.startedWithChrome.value) {
      options.setRefreshingProgress(1, { contentCollapsed: false })
      return
    }

    options.setRefreshingProgress(
      clampProgress(options.topPull.startProgress.value - distance / options.feedHeaderHeight.value),
    )
  }

  function handleFeedTopPullEnd(shouldRefresh = false) {
    if (!options.topPull.pulling.value) {
      options.topPull.resetStartedWithChrome()
      return
    }

    const startedWithChrome = options.topPull.startedWithChrome.value
    options.topPull.finish()

    if (shouldRefresh) {
      options.recordRefreshStartedWithChrome(startedWithChrome)
      options.commitRefreshingChrome(startedWithChrome)
      return
    }

    if (!startedWithChrome || topChromeProgress() <= 0.04) {
      options.collapseTopChrome()
      options.topPull.resetStartedWithChrome()
      return
    }

    options.setTopChromeVisible(true)
    options.topPull.resetStartedWithChrome()
  }

  return {
    handleFeedTopPullStart,
    handleFeedTopPullMove,
    handleFeedTopPullEnd,
  }
}
