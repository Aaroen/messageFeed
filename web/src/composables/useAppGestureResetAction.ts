type AppGestureResetActionOptions = {
  clearGesturePointer: () => void
  resetNavigationGesture: () => void
  resetFeedViewSwipeTracking: () => void
  resetFeedViewPointerTracking: () => void
  clearFeedViewStartedWithHiddenChrome: () => void
  resetReaderBackSwipeCandidate: () => void
}

export function useAppGestureResetAction(options: AppGestureResetActionOptions) {
  function resetGestureTracking() {
    options.clearGesturePointer()
    options.resetNavigationGesture()
    options.resetFeedViewSwipeTracking()
    options.resetFeedViewPointerTracking()
    options.clearFeedViewStartedWithHiddenChrome()
    options.resetReaderBackSwipeCandidate()
  }

  return {
    resetGestureTracking,
  }
}
