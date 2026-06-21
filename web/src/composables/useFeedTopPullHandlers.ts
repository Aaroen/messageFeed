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
  feedTopChromeIsVisiblyOpen: ReadableRef<boolean>
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

function clamp(value: number, min = 0, max = 1) {
  return Math.min(max, Math.max(min, value))
}

export function useFeedTopPullHandlers(options: FeedTopPullHandlersOptions) {
  function handleFeedTopPullStart(startedWithVisibleChrome = false) {
    if (options.isFeedRoute.value && options.feedPullRefreshing()) {
      return
    }

    options.topPull.begin(
      startedWithVisibleChrome || options.feedTopChromeIsVisiblyOpen.value,
      options.topChromeProgress.value,
    )
    options.beginRefreshingChrome()
  }

  function handleFeedTopPullMove(distance: number) {
    if (!options.topPull.pulling.value || (options.isFeedRoute.value && options.feedPullRefreshing())) {
      return
    }

    if (!options.topPull.startedWithChrome.value && options.feedTopChromeIsVisiblyOpen.value) {
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
      clamp(options.topPull.startProgress.value - distance / options.feedHeaderHeight.value),
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

    if (options.topChromeProgress.value <= 0.04) {
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
