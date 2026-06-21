import { computed, ref } from 'vue'

export type FeedSwipeSurface = 'feed:subscriptions' | 'feed:recommendations'

type FeedPagerTransitionOptions = {
  getActiveKey: () => string | symbol | null | undefined
  getWindowWidth: () => number
  isFeedRoute: () => boolean
  isDetailReaderOpen: () => boolean
  dragThreshold?: number
}

type FeedPagerSwipeFinishResult = {
  committed: boolean
  settlePayload: {
    progress: number
    isBlocked: boolean
  }
  shouldRevealChromeFirst: boolean
}

function clamp(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

export function useFeedPagerTransition(options: FeedPagerTransitionOptions) {
  const dragThreshold = options.dragThreshold ?? 8
  const dragOffset = ref(0)
  const settling = ref(false)
  let settlingTimer = 0
  let delayedCommitTimer = 0
  let startedWithHiddenChrome = false
  let activePointerId: number | null = null

  const activeIndex = computed(() => (options.getActiveKey() === 'recommendations' ? 1 : 0))
  const activeSurface = computed<FeedSwipeSurface>(() =>
    activeIndex.value === 0 ? 'feed:subscriptions' : 'feed:recommendations',
  )

  const trackStyle = computed(() => ({
    transform: `translate3d(calc(${-activeIndex.value * 100}% + ${cssPx(dragOffset.value)}), 0, 0)`,
  }))

  const swipeProgress = computed(() =>
    clamp(Math.abs(dragOffset.value) / Math.max(1, Math.min(options.getWindowWidth(), 320))),
  )

  const targetKey = computed(() => {
    if (dragOffset.value < -dragThreshold && activeIndex.value === 0) {
      return 'recommendations'
    }
    if (dragOffset.value > dragThreshold && activeIndex.value === 1) {
      return 'subscriptions'
    }
    return ''
  })

  const targetVisible = computed(() => options.isFeedRoute() && !options.isDetailReaderOpen() && Boolean(targetKey.value))
  const targetProgress = computed(() => (targetVisible.value ? clamp((swipeProgress.value - 0.26) / 0.48) : 0))

  function surfaceFromOffset(offset: number): FeedSwipeSurface | null {
    if (offset < -dragThreshold && activeIndex.value === 0) {
      return 'feed:recommendations'
    }
    if (offset > dragThreshold && activeIndex.value === 1) {
      return 'feed:subscriptions'
    }
    return null
  }

  function isBlockedDragDirection(deltaX: number) {
    return (activeIndex.value === 0 && deltaX > 0) || (activeIndex.value === 1 && deltaX < 0)
  }

  function canStartDrag(deltaX: number) {
    return surfaceFromOffset(deltaX) !== null
  }

  function commitPath(deltaX: number, horizontal: boolean, switchDistance: number) {
    if (activeIndex.value === 0 && horizontal && deltaX <= -switchDistance) {
      return '/recommendations'
    }
    if (activeIndex.value === 1 && horizontal && deltaX >= switchDistance) {
      return '/subscriptions'
    }
    return null
  }

  function directionFromOffset(offset: number) {
    return offset < 0 ? ('left' as const) : ('right' as const)
  }

  function swipeTransitionBeginPayload(offset: number) {
    return {
      from: activeSurface.value,
      to: surfaceFromOffset(offset),
      direction: directionFromOffset(offset),
      progress: swipeProgress.value,
    }
  }

  function swipeTransitionUpdatePayload(offset: number) {
    const targetSurface = surfaceFromOffset(offset)
    return {
      to: targetSurface,
      direction: directionFromOffset(offset),
      progress: swipeProgress.value,
      isBlocked: targetSurface === null,
    }
  }

  function finishSwipeResult(nextPath: string | null): FeedPagerSwipeFinishResult {
    const committed = Boolean(nextPath)
    setSettling(true)
    clearDelayedCommitTimer()
    clearSettlingTimer()
    return {
      committed,
      settlePayload: {
        progress: committed ? 1 : 0,
        isBlocked: false,
      },
      shouldRevealChromeFirst: committed && consumeStartedWithHiddenChrome(),
    }
  }

  function settleFinishedSwipe(delay: number) {
    setDragOffset(0)
    scheduleSettlingEnd(delay)
  }

  function beginProgrammaticNavigation() {
    setSettling(true)
    setDragOffset(0)
  }

  function settleProgrammaticNavigation(delay: number) {
    clearDelayedCommitTimer()
    scheduleSettlingEnd(delay)
  }

  function beginDragCandidate() {
    setSettling(false)
  }

  function beginPointerTracking(pointerId: number) {
    activePointerId = pointerId
    beginDragCandidate()
  }

  function isActivePointer(pointerId: number) {
    return activePointerId === pointerId
  }

  function clearPointerTracking() {
    activePointerId = null
  }

  function setDragOffset(nextOffset: number) {
    dragOffset.value = Number.isFinite(nextOffset) ? nextOffset : 0
  }

  function resetDragOffset() {
    setDragOffset(0)
  }

  function setSettling(nextSettling: boolean) {
    settling.value = nextSettling
  }

  function markStartedWithHiddenChrome() {
    startedWithHiddenChrome = true
  }

  function clearStartedWithHiddenChrome() {
    startedWithHiddenChrome = false
  }

  function consumeStartedWithHiddenChrome() {
    const started = startedWithHiddenChrome
    startedWithHiddenChrome = false
    return started
  }

  function clearSettlingTimer() {
    window.clearTimeout(settlingTimer)
  }

  function clearDelayedCommitTimer() {
    window.clearTimeout(delayedCommitTimer)
  }

  function scheduleSettlingEnd(delay: number) {
    clearSettlingTimer()
    settlingTimer = window.setTimeout(() => {
      setSettling(false)
    }, Math.max(0, delay))
  }

  function scheduleDelayedCommit(delay: number, commit: () => void) {
    clearDelayedCommitTimer()
    delayedCommitTimer = window.setTimeout(commit, Math.max(0, delay))
  }

  function setDragDelta(deltaX: number) {
    if (activeIndex.value === 0) {
      setDragOffset(Math.min(0, Math.max(deltaX, -options.getWindowWidth())))
      return
    }
    setDragOffset(Math.max(0, Math.min(deltaX, options.getWindowWidth())))
  }

  function reset() {
    clearSettlingTimer()
    clearDelayedCommitTimer()
    clearStartedWithHiddenChrome()
    clearPointerTracking()
    dragOffset.value = 0
    settling.value = false
  }

  return {
    dragThreshold,
    dragOffset,
    settling,
    activeIndex,
    activeSurface,
    trackStyle,
    swipeProgress,
    targetKey,
    targetVisible,
    targetProgress,
    surfaceFromOffset,
    isBlockedDragDirection,
    canStartDrag,
    commitPath,
    swipeTransitionBeginPayload,
    swipeTransitionUpdatePayload,
    finishSwipeResult,
    settleFinishedSwipe,
    beginProgrammaticNavigation,
    settleProgrammaticNavigation,
    beginDragCandidate,
    beginPointerTracking,
    isActivePointer,
    clearPointerTracking,
    resetDragOffset,
    markStartedWithHiddenChrome,
    clearStartedWithHiddenChrome,
    consumeStartedWithHiddenChrome,
    clearSettlingTimer,
    clearDelayedCommitTimer,
    scheduleSettlingEnd,
    scheduleDelayedCommit,
    setDragDelta,
    reset,
  }
}
