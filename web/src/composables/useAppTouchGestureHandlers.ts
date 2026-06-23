type ReadableRef<T> = {
  readonly value: T
}

type GestureOriginController = {
  begin: (startX: number, startY: number, navigationProgress: number) => void
  originX: () => number
  navigationProgress: () => number
  delta: (clientX: number, clientY: number) => { deltaX: number; deltaY: number }
}

type NavigationGestureController = {
  setCandidates: (candidates: { edgeSwipe?: boolean; closeSwipe?: boolean }) => void
  cancelCandidates: () => void
  hasCandidate: () => boolean
  hasActiveSwipe: () => boolean
  edgeSwipeCandidate: () => boolean
  closeSwipeCandidate: () => boolean
  beginEdgeSwipe: () => void
  beginCloseSwipe: () => void
  edgeSwipe: () => boolean
  closeSwipe: () => boolean
  dragStarted: () => boolean
}

type NavigationDrawerController = {
  beginDrag: () => void
  updateOpeningDrag: (deltaX: number) => void
  updateClosingDrag: (startProgress: number, deltaX: number) => void
}

type FeedPagerTouchController = {
  resetViewSwipeTracking: () => void
  beginViewSwipeCandidate: () => void
  cancelViewSwipeCandidate: () => void
  tryBeginDrag: (deltaX: number) => { started: boolean; blocked: boolean }
  updateDragDelta: (deltaX: number) => { blocked: boolean }
  resolveDragCommitPath: (deltaX: number, horizontal: boolean, switchDistance: number) => string | null
  endPointerTracking: () => void
}

type ReaderBackSwipeCandidateTarget = 'source' | 'page'

type AppTouchGestureHandlersOptions = {
  navigationVisible: ReadableRef<boolean>
  navigationOpen: ReadableRef<boolean>
  navigationProgress: ReadableRef<number>
  sourceReaderOpen: ReadableRef<boolean>
  isFeedRoute: ReadableRef<boolean>
  viewSwipeCandidateActive: ReadableRef<boolean>
  viewSwipeActive: ReadableRef<boolean>
  viewDragOffset: ReadableRef<number>
  readerBackSwipeCandidateActive: ReadableRef<boolean>
  readerBackSwipeTrackingActive: ReadableRef<boolean>
  navigationOpenDistance: number
  viewSwitchDistance: number
  gestureOrigin: GestureOriginController
  navigationGesture: NavigationGestureController
  navigationDrawer: NavigationDrawerController
  feedPagerTransition: FeedPagerTouchController
  finishCommittedListReturnForGesture: () => void
  resetGestureTracking: () => void
  detailBlocksGestures: () => boolean
  beginDetailGestureCandidate: (startX: number, startY: number) => void
  beginReaderBackSwipeCandidateState: (target: ReaderBackSwipeCandidateTarget) => void
  resetReaderBackSwipeCandidateState: () => void
  updateBackSwipe: (deltaX: number, deltaY: number, fromDetailFrame: boolean, currentX: number) => boolean
  finishBackSwipe: (deltaX: number, deltaY: number) => void
  cancelBackSwipe: () => void
  canStartNavigationOpen: (clientX: number) => boolean
  canStartViewSwipe: (clientX: number) => boolean
  isHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  isViewHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  isNavigationDrag: (deltaX: number, deltaY: number) => boolean
  settleNavigation: (open: boolean) => void
  showTopChromeForViewSwipe: () => void
  beginViewSwipeTransition: (deltaX: number) => void
  syncViewSwipeTransition: (offset: number) => void
  suppressFollowingClick: () => void
  finishViewSwipe: (nextPath: string | null) => void
}

export function useAppTouchGestureHandlers(options: AppTouchGestureHandlersOptions) {
  function handleTouchStart(event: TouchEvent) {
    if (event.touches.length !== 1) {
      return
    }

    options.finishCommittedListReturnForGesture()
    if (options.readerBackSwipeTrackingActive.value) {
      options.cancelBackSwipe()
      options.resetGestureTracking()
    }

    const touch = event.touches[0]
    options.gestureOrigin.begin(touch.clientX, touch.clientY, options.navigationProgress.value)

    if (options.navigationVisible.value) {
      options.navigationGesture.setCandidates({ closeSwipe: options.navigationOpen.value })
      options.feedPagerTransition.resetViewSwipeTracking()
      options.resetReaderBackSwipeCandidateState()
      return
    }

    if (options.detailBlocksGestures()) {
      options.beginDetailGestureCandidate(touch.clientX, touch.clientY)
      return
    }
    if (options.sourceReaderOpen.value) {
      options.beginReaderBackSwipeCandidateState('source')
      return
    }
    if (!options.isFeedRoute.value && !options.navigationVisible.value) {
      options.beginReaderBackSwipeCandidateState('page')
    }

    options.navigationGesture.setCandidates({
      edgeSwipe: options.canStartNavigationOpen(options.gestureOrigin.originX()),
      closeSwipe: options.navigationOpen.value,
    })
    if (options.canStartViewSwipe(options.gestureOrigin.originX())) {
      options.feedPagerTransition.beginViewSwipeCandidate()
    } else {
      options.feedPagerTransition.resetViewSwipeTracking()
    }
  }

  function handleTouchMove(event: TouchEvent) {
    if (
      !options.navigationGesture.hasCandidate() &&
      !options.viewSwipeCandidateActive.value &&
      !options.readerBackSwipeCandidateActive.value &&
      !options.navigationGesture.hasActiveSwipe() &&
      !options.viewSwipeActive.value &&
      !options.readerBackSwipeTrackingActive.value
    ) {
      return
    }

    const touch = event.touches[0]
    const { deltaX, deltaY } = options.gestureOrigin.delta(touch.clientX, touch.clientY)
    const horizontal = options.isHorizontalSwipe(deltaX, deltaY)
    const viewHorizontal = options.isViewHorizontalSwipe(deltaX, deltaY)

    if (options.readerBackSwipeCandidateActive.value || options.readerBackSwipeTrackingActive.value) {
      const handledBackSwipe = options.updateBackSwipe(deltaX, deltaY, false, touch.clientX)
      if (!handledBackSwipe) {
        return
      }
      event.preventDefault()
      return
    }

    if (
      options.navigationGesture.edgeSwipeCandidate() &&
      deltaX > 8 &&
      options.isNavigationDrag(deltaX, deltaY)
    ) {
      options.navigationGesture.beginEdgeSwipe()
      options.feedPagerTransition.cancelViewSwipeCandidate()
      options.navigationDrawer.beginDrag()
    }

    if (
      options.navigationGesture.closeSwipeCandidate() &&
      deltaX < -8 &&
      options.isNavigationDrag(deltaX, deltaY)
    ) {
      options.navigationGesture.beginCloseSwipe()
      options.feedPagerTransition.cancelViewSwipeCandidate()
      options.navigationDrawer.beginDrag()
    }

    if (options.navigationGesture.hasActiveSwipe() || options.viewSwipeActive.value) {
      event.preventDefault()
    }

    if (options.navigationGesture.edgeSwipe()) {
      options.navigationDrawer.updateOpeningDrag(deltaX)
      return
    }

    if (options.navigationGesture.closeSwipe()) {
      options.navigationDrawer.updateClosingDrag(options.gestureOrigin.navigationProgress(), deltaX)
      return
    }

    if (options.viewSwipeCandidateActive.value && viewHorizontal) {
      const dragStart = options.feedPagerTransition.tryBeginDrag(deltaX)
      if (dragStart.blocked) {
        options.feedPagerTransition.cancelViewSwipeCandidate()
        return
      }
      if (dragStart.started) {
        options.navigationGesture.cancelCandidates()
        options.showTopChromeForViewSwipe()
        options.beginViewSwipeTransition(deltaX)
      } else {
        return
      }
    }

    if (options.viewSwipeActive.value) {
      options.feedPagerTransition.updateDragDelta(deltaX)
      options.syncViewSwipeTransition(options.viewDragOffset.value)
      return
    }

    void horizontal
  }

  function handleTouchEnd(event: TouchEvent) {
    if (
      !options.navigationGesture.hasCandidate() &&
      !options.viewSwipeCandidateActive.value &&
      !options.readerBackSwipeCandidateActive.value &&
      !options.navigationGesture.hasActiveSwipe() &&
      !options.viewSwipeActive.value &&
      !options.readerBackSwipeTrackingActive.value
    ) {
      return
    }

    const touch = event.changedTouches[0]
    const { deltaX, deltaY } = options.gestureOrigin.delta(touch.clientX, touch.clientY)
    const horizontal = options.isHorizontalSwipe(deltaX, deltaY)

    if (options.readerBackSwipeTrackingActive.value) {
      options.finishBackSwipe(deltaX, deltaY)
      options.resetGestureTracking()
      return
    }

    if (!options.navigationGesture.hasActiveSwipe() && !options.viewSwipeActive.value) {
      options.resetGestureTracking()
      return
    }

    if (options.navigationGesture.edgeSwipe()) {
      if (options.navigationGesture.dragStarted()) {
        options.settleNavigation(
          horizontal &&
            (deltaX >= options.navigationOpenDistance || options.navigationProgress.value >= 0.42),
        )
      }
    }

    if (options.navigationGesture.closeSwipe()) {
      if (options.navigationGesture.dragStarted()) {
        options.settleNavigation(
          !(horizontal &&
            (deltaX <= -options.navigationOpenDistance || options.navigationProgress.value <= 0.58)),
        )
      }
    }

    if (options.viewSwipeActive.value) {
      options.suppressFollowingClick()
      options.finishViewSwipe(
        options.feedPagerTransition.resolveDragCommitPath(deltaX, horizontal, options.viewSwitchDistance),
      )
    }

    options.resetGestureTracking()
  }

  function handleTouchCancel() {
    const hadNavigationGesture = options.navigationGesture.hasActiveSwipe()
    const hadViewGesture = options.viewSwipeActive.value
    const hadBackGesture = options.readerBackSwipeTrackingActive.value
    if (hadBackGesture) {
      options.cancelBackSwipe()
    }
    if (hadViewGesture) {
      options.finishViewSwipe(null)
    }
    options.resetGestureTracking()
    if (hadNavigationGesture && options.navigationVisible.value) {
      options.settleNavigation(options.navigationProgress.value >= 0.42)
    }
    options.feedPagerTransition.endPointerTracking()
  }

  return {
    handleTouchStart,
    handleTouchMove,
    handleTouchEnd,
    handleTouchCancel,
  }
}
