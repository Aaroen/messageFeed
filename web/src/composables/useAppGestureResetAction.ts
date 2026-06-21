type AppGestureResetActionOptions = {
  resetNavigationGesture: () => void
  resetFeedViewSwipeTracking: () => void
  clearFeedViewStartedWithHiddenChrome: () => void
  resetReaderBackSwipeCandidate: () => void
}

export function useAppGestureResetAction(options: AppGestureResetActionOptions) {
  function resetGestureTracking() {
    options.resetNavigationGesture()
    options.resetFeedViewSwipeTracking()
    options.clearFeedViewStartedWithHiddenChrome()
    options.resetReaderBackSwipeCandidate()
  }

  return {
    resetGestureTracking,
  }
}
