type ReadableRef<T> = {
  readonly value: T
}

type GestureOriginController = {
  begin: (startX: number, startY: number, navigationProgress: number) => void
  delta: (clientX: number, clientY: number) => { deltaX: number; deltaY: number }
}

type NavigationGestureController = {
  clearActiveSwipes: () => void
}

type FeedPagerPointerController = {
  beginPointerTracking: (pointerId: number) => void
  isActivePointer: (pointerId: number) => boolean
  cancelPointerCandidate: () => void
  tryBeginDrag: (deltaX: number) => { started: boolean; blocked: boolean }
  updateDragDelta: (
    deltaX: number,
    options?: { resetBlockedDirection?: boolean },
  ) => { blocked: boolean }
  resolveDragCommitPath: (deltaX: number, horizontal: boolean, switchDistance: number) => string | null
  endPointerTracking: () => void
}

type FeedPointerSwipeHandlersOptions = {
  isFeedRoute: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
  navigationProgress: ReadableRef<number>
  viewSwipeCandidateActive: ReadableRef<boolean>
  viewSwipeActive: ReadableRef<boolean>
  viewDragOffset: ReadableRef<number>
  viewSwitchDistance: number
  gestureOrigin: GestureOriginController
  navigationGesture: NavigationGestureController
  feedPagerTransition: FeedPagerPointerController
  canStartViewSwipe: (clientX: number) => boolean
  finishCommittedListReturnForGesture: () => void
  isViewHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  suppressFollowingClick: () => void
  showTopChromeForViewSwipe: () => void
  beginViewSwipeTransition: (deltaX: number) => void
  syncViewSwipeTransition: (offset: number) => void
  finishViewSwipe: (nextPath: string | null) => void
}

export function useFeedPointerSwipeHandlers(options: FeedPointerSwipeHandlersOptions) {
  function handleFeedPointerDown(event: PointerEvent) {
    if (
      !options.isFeedRoute.value ||
      options.navigationVisible.value ||
      event.isPrimary === false ||
      event.pointerType === 'mouse'
    ) {
      return
    }

    options.finishCommittedListReturnForGesture()

    if (!options.canStartViewSwipe(event.clientX)) {
      return
    }

    options.gestureOrigin.begin(event.clientX, event.clientY, options.navigationProgress.value)
    options.navigationGesture.clearActiveSwipes()
    options.feedPagerTransition.beginPointerTracking(event.pointerId)
  }

  function handleFeedPointerMove(event: PointerEvent) {
    if (!options.feedPagerTransition.isActivePointer(event.pointerId) || event.pointerType === 'mouse') {
      return
    }

    const { deltaX, deltaY } = options.gestureOrigin.delta(event.clientX, event.clientY)

    if (options.viewSwipeCandidateActive.value && !options.viewSwipeActive.value) {
      if (!options.isViewHorizontalSwipe(deltaX, deltaY)) {
        return
      }

      const dragStart = options.feedPagerTransition.tryBeginDrag(deltaX)
      if (dragStart.blocked) {
        options.feedPagerTransition.cancelPointerCandidate()
        return
      }

      if (dragStart.started) {
        options.suppressFollowingClick()
        options.showTopChromeForViewSwipe()
        options.beginViewSwipeTransition(deltaX)
        ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
      } else {
        return
      }
    }

    if (!options.viewSwipeActive.value || !options.isViewHorizontalSwipe(deltaX, deltaY)) {
      return
    }

    const dragUpdate = options.feedPagerTransition.updateDragDelta(deltaX, { resetBlockedDirection: true })
    if (dragUpdate.blocked) {
      options.syncViewSwipeTransition(options.viewDragOffset.value)
      return
    }

    options.syncViewSwipeTransition(options.viewDragOffset.value)
  }

  function handleFeedPointerUp(event: PointerEvent) {
    if (!options.feedPagerTransition.isActivePointer(event.pointerId) || event.pointerType === 'mouse') {
      return
    }

    if (!options.viewSwipeActive.value) {
      options.feedPagerTransition.endPointerTracking()
      return
    }

    const { deltaX, deltaY } = options.gestureOrigin.delta(event.clientX, event.clientY)
    const horizontal = options.isViewHorizontalSwipe(deltaX, deltaY)

    const nextPath = options.feedPagerTransition.resolveDragCommitPath(
      deltaX,
      horizontal,
      options.viewSwitchDistance,
    )
    if (nextPath) {
      options.suppressFollowingClick()
      options.finishViewSwipe(nextPath)
    } else {
      options.finishViewSwipe(null)
    }

    options.feedPagerTransition.endPointerTracking()
  }

  function handleFeedPointerCancel(event: PointerEvent) {
    if (!options.feedPagerTransition.isActivePointer(event.pointerId) || event.pointerType === 'mouse') {
      return
    }

    const hadViewSwipe = options.viewSwipeActive.value
    options.feedPagerTransition.endPointerTracking()
    if (hadViewSwipe) {
      options.finishViewSwipe(null)
    }
  }

  return {
    handleFeedPointerDown,
    handleFeedPointerMove,
    handleFeedPointerUp,
    handleFeedPointerCancel,
  }
}
