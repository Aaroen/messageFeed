type ReadableRef<T> = {
  readonly value: T
}

type GestureOriginController = {
  begin: (startX: number, startY: number, navigationProgress: number) => void
  originX: () => number
  navigationProgress: () => number
  delta: (clientX: number, clientY: number) => { deltaX: number; deltaY: number }
  setActivePointerId: (pointerId: number | null) => void
  isActivePointer: (pointerId: number) => boolean
  clearActivePointer: () => void
}

type NavigationGestureController = {
  setCandidates: (candidates: { edgeSwipe?: boolean; closeSwipe?: boolean }) => void
  hasCandidate: () => boolean
  edgeSwipeCandidate: () => boolean
  closeSwipeCandidate: () => boolean
  beginEdgeSwipe: () => void
  beginCloseSwipe: () => void
  hasActiveSwipe: () => boolean
  edgeSwipe: () => boolean
  closeSwipe: () => boolean
  dragStarted: () => boolean
}

type NavigationDrawerController = {
  beginDrag: () => void
  updateOpeningDrag: (deltaX: number) => void
  updateClosingDrag: (startProgress: number, deltaX: number) => void
}

type NavigationPointerHandlersOptions = {
  navigationOpen: ReadableRef<boolean>
  navigationProgress: ReadableRef<number>
  viewSwipeActive: ReadableRef<boolean>
  navigationOpenDistance: number
  gestureOrigin: GestureOriginController
  navigationGesture: NavigationGestureController
  navigationDrawer: NavigationDrawerController
  finishCommittedListReturnForGesture: () => void
  canStartNavigationOpen: (clientX: number) => boolean
  cancelViewSwipeCandidate: () => void
  isNavigationDrag: (deltaX: number, deltaY: number) => boolean
  isHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  isViewHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  settleNavigation: (open: boolean) => void
  resetGestureTracking: () => void
}

export function useNavigationPointerHandlers(options: NavigationPointerHandlersOptions) {
  function ignoresPointerGesture(event: PointerEvent) {
    return event.pointerType === 'mouse' || event.pointerType === 'touch'
  }

  function handleWindowPointerDown(event: PointerEvent) {
    if (ignoresPointerGesture(event) || event.isPrimary === false) {
      return
    }

    options.finishCommittedListReturnForGesture()

    options.gestureOrigin.begin(event.clientX, event.clientY, options.navigationProgress.value)
    options.navigationGesture.setCandidates({
      edgeSwipe: options.canStartNavigationOpen(options.gestureOrigin.originX()),
      closeSwipe: options.navigationOpen.value,
    })
    options.gestureOrigin.setActivePointerId(
      options.navigationGesture.hasCandidate() ? event.pointerId : null,
    )
  }

  function handleWindowPointerMove(event: PointerEvent) {
    if (!options.gestureOrigin.isActivePointer(event.pointerId)) {
      return
    }

    const { deltaX, deltaY } = options.gestureOrigin.delta(event.clientX, event.clientY)

    if (options.navigationGesture.edgeSwipeCandidate() && deltaX > 8 && options.isNavigationDrag(deltaX, deltaY)) {
      options.navigationGesture.beginEdgeSwipe()
      options.cancelViewSwipeCandidate()
      options.navigationDrawer.beginDrag()
    }

    if (options.navigationGesture.closeSwipeCandidate() && deltaX < -8 && options.isNavigationDrag(deltaX, deltaY)) {
      options.navigationGesture.beginCloseSwipe()
      options.cancelViewSwipeCandidate()
      options.navigationDrawer.beginDrag()
    }

    if (options.navigationGesture.hasActiveSwipe()) {
      event.preventDefault()
    }

    if (options.navigationGesture.edgeSwipe()) {
      options.navigationDrawer.updateOpeningDrag(deltaX)
    } else if (options.navigationGesture.closeSwipe()) {
      options.navigationDrawer.updateClosingDrag(options.gestureOrigin.navigationProgress(), deltaX)
    }
  }

  function handleWindowPointerUp(event: PointerEvent) {
    if (!options.gestureOrigin.isActivePointer(event.pointerId)) {
      return
    }

    const { deltaX, deltaY } = options.gestureOrigin.delta(event.clientX, event.clientY)
    const horizontal = options.viewSwipeActive.value
      ? options.isViewHorizontalSwipe(deltaX, deltaY)
      : options.isHorizontalSwipe(deltaX, deltaY)

    if (options.navigationGesture.edgeSwipe() && options.navigationGesture.dragStarted()) {
      options.settleNavigation(
        horizontal &&
          (deltaX >= options.navigationOpenDistance || options.navigationProgress.value >= 0.42),
      )
    }

    if (options.navigationGesture.closeSwipe() && options.navigationGesture.dragStarted()) {
      options.settleNavigation(
        !(horizontal &&
          (deltaX <= -options.navigationOpenDistance || options.navigationProgress.value <= 0.58)),
      )
    }

    options.gestureOrigin.clearActivePointer()
    options.resetGestureTracking()
  }

  function handleWindowPointerCancel(event: PointerEvent) {
    if (!options.gestureOrigin.isActivePointer(event.pointerId)) {
      return
    }

    options.gestureOrigin.clearActivePointer()
    const hadNavigationGesture = options.navigationGesture.hasActiveSwipe()
    options.resetGestureTracking()
    if (hadNavigationGesture) {
      options.settleNavigation(options.navigationProgress.value >= 0.42)
    }
  }

  return {
    handleWindowPointerDown,
    handleWindowPointerMove,
    handleWindowPointerUp,
    handleWindowPointerCancel,
  }
}
