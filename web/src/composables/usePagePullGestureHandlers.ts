type ReadableRef<T> = {
  readonly value: T
}

type PagePullRefreshController = {
  readonly gestureDistance: ReadableRef<number>
  readonly gestureCandidate: ReadableRef<boolean>
  readonly gestureTracking: ReadableRef<boolean>
  readonly refreshing: ReadableRef<boolean>
  beginGestureCandidate: (startX: number, startY: number) => void
  beginGestureTracking: () => void
  finishGestureTracking: () => void
  gestureDelta: (clientX: number, clientY: number) => { deltaX: number; deltaY: number }
  updateGestureDistance: (deltaY: number) => number
  setGestureOffset: (offset: number) => void
}

type PagePullGestureHandlersOptions = {
  isFeedRoute: ReadableRef<boolean>
  refreshThreshold: number
  pullRefresh: PagePullRefreshController
  currentContentScrollTop: () => number
  isControlTarget: (target: EventTarget | null) => boolean
  shouldCancelTopPull: (deltaX: number, deltaY: number) => boolean
  shouldWaitForTopPull: (deltaX: number, deltaY: number) => boolean
  setTopChromeVisible: (visible: boolean) => void
  finishFeedTopPull: () => void
  settlePullOffset: () => void
  refreshCurrentPageFromPull: () => void | Promise<void>
}

function pageRubberBandOffset(distance: number) {
  if (distance <= 0) {
    return 0
  }
  return Math.min(22, Math.log1p(distance) * 4.8)
}

export function usePagePullGestureHandlers(options: PagePullGestureHandlersOptions) {
  function resetPageTopPullTracking() {
    options.pullRefresh.finishGestureTracking()
  }

  function handlePageTouchStart(event: TouchEvent) {
    if (
      options.isFeedRoute.value ||
      event.touches.length !== 1 ||
      options.pullRefresh.refreshing.value ||
      options.currentContentScrollTop() > 0 ||
      options.isControlTarget(event.target)
    ) {
      resetPageTopPullTracking()
      return
    }

    const touch = event.touches[0]
    options.pullRefresh.beginGestureCandidate(touch.clientX, touch.clientY)
  }

  function handlePageTouchMove(event: TouchEvent) {
    if (
      options.isFeedRoute.value ||
      event.touches.length !== 1 ||
      options.currentContentScrollTop() > 0 ||
      (!options.pullRefresh.gestureCandidate.value && !options.pullRefresh.gestureTracking.value)
    ) {
      return
    }

    const touch = event.touches[0]
    const { deltaX, deltaY } = options.pullRefresh.gestureDelta(touch.clientX, touch.clientY)

    if (!options.pullRefresh.gestureTracking.value) {
      if (options.shouldCancelTopPull(deltaX, deltaY)) {
        resetPageTopPullTracking()
        return
      }

      if (options.shouldWaitForTopPull(deltaX, deltaY)) {
        return
      }

      options.pullRefresh.beginGestureTracking()
      options.setTopChromeVisible(true)
    }

    if (options.pullRefresh.gestureTracking.value) {
      event.preventDefault()
      options.pullRefresh.updateGestureDistance(deltaY)
      options.pullRefresh.setGestureOffset(pageRubberBandOffset(deltaY))
    }
  }

  function handlePageTouchEnd() {
    if (options.pullRefresh.gestureTracking.value) {
      const shouldRefresh = options.pullRefresh.gestureDistance.value >= options.refreshThreshold
      options.finishFeedTopPull()
      options.setTopChromeVisible(true)
      options.settlePullOffset()
      if (shouldRefresh) {
        void options.refreshCurrentPageFromPull()
      }
    } else if (options.pullRefresh.gestureCandidate.value) {
      options.finishFeedTopPull()
    }
    resetPageTopPullTracking()
  }

  function handlePageTouchCancel() {
    if (options.pullRefresh.gestureTracking.value || options.pullRefresh.gestureCandidate.value) {
      options.finishFeedTopPull()
      options.setTopChromeVisible(true)
      options.settlePullOffset()
    }
    resetPageTopPullTracking()
  }

  return {
    resetPageTopPullTracking,
    handlePageTouchStart,
    handlePageTouchMove,
    handlePageTouchEnd,
    handlePageTouchCancel,
  }
}
