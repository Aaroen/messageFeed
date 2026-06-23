type RuntimeCleaner = () => void

type AppRuntimeCleanupOptions = {
  swipe: {
    clearFeedPagerTimers: RuntimeCleaner
    clearSwipeTransitionTimer: RuntimeCleaner
  }
  navigation: {
    clearTimer: RuntimeCleaner
  }
  feedRefresh: {
    resetRefreshCompletion: RuntimeCleaner
  }
  chrome: {
    clearTimer: RuntimeCleaner
  }
  route: {
    clearTimer: RuntimeCleaner
  }
  readerRouteSync: {
    clearTimer: RuntimeCleaner
  }
  readerMotion: {
    resetSourceContentMotion: RuntimeCleaner
  }
  detailSourceTransition: {
    clearRectCapture: RuntimeCleaner
  }
  pagePull: {
    invalidateRefreshCompletion: RuntimeCleaner
    reset: RuntimeCleaner
  }
  shell: {
    clearClickSuppressionTimer: RuntimeCleaner
  }
  sourceSubscription: {
    clearRuntime: RuntimeCleaner
  }
  readerDetailFrames: {
    clear: RuntimeCleaner
  }
  readerSessionScrollRestore: {
    clearTimers: RuntimeCleaner
  }
  readerStack: {
    clearTimers: RuntimeCleaner
    clearBackSwipeStretchAnchorTimer: RuntimeCleaner
  }
  readerSession: {
    clearTimer: RuntimeCleaner
  }
}

export function useAppRuntimeCleanup(options: AppRuntimeCleanupOptions) {
  const clearRuntimeTimers: RuntimeCleaner[] = [
    () => options.swipe.clearFeedPagerTimers(),
    () => options.swipe.clearSwipeTransitionTimer(),
    () => options.navigation.clearTimer(),
    () => options.feedRefresh.resetRefreshCompletion(),
    () => options.chrome.clearTimer(),
    () => options.route.clearTimer(),
    () => options.readerRouteSync.clearTimer(),
    () => options.readerMotion.resetSourceContentMotion(),
    () => options.detailSourceTransition.clearRectCapture(),
    () => options.pagePull.invalidateRefreshCompletion(),
    () => options.pagePull.reset(),
    () => options.shell.clearClickSuppressionTimer(),
    () => options.sourceSubscription.clearRuntime(),
    () => options.readerDetailFrames.clear(),
    () => options.readerSessionScrollRestore.clearTimers(),
    () => options.readerStack.clearTimers(),
    () => options.readerStack.clearBackSwipeStretchAnchorTimer(),
    () => options.readerSession.clearTimer(),
  ]

  return {
    clearRuntimeTimers,
  }
}
