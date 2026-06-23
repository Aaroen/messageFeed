type RuntimeCleaner = () => void

type AppRuntimeCleanupOptions = {
  swipe: {
    resetFeedPagerTransition: RuntimeCleaner
    resetSwipeTransition: RuntimeCleaner
  }
  navigation: {
    reset: RuntimeCleaner
  }
  feedRefresh: {
    resetRefreshCompletion: RuntimeCleaner
  }
  chrome: {
    reset: RuntimeCleaner
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
    resetClickSuppression: RuntimeCleaner
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
    () => options.swipe.resetFeedPagerTransition(),
    () => options.swipe.resetSwipeTransition(),
    () => options.navigation.reset(),
    () => options.feedRefresh.resetRefreshCompletion(),
    () => options.chrome.reset(),
    () => options.route.clearTimer(),
    () => options.readerRouteSync.clearTimer(),
    () => options.readerMotion.resetSourceContentMotion(),
    () => options.detailSourceTransition.clearRectCapture(),
    () => options.pagePull.invalidateRefreshCompletion(),
    () => options.pagePull.reset(),
    () => options.shell.resetClickSuppression(),
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
