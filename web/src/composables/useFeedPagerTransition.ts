import { computed, readonly, ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

export type FeedSwipeSurface = 'feed:subscriptions' | 'feed:recommendations' | 'feed:agent'

const feedPagerPages = [
  { key: 'subscriptions', path: '/subscriptions', surface: 'feed:subscriptions' },
  { key: 'recommendations', path: '/recommendations', surface: 'feed:recommendations' },
  { key: 'agent', path: '/agent', surface: 'feed:agent' },
] as const

const lastFeedPagerIndex = feedPagerPages.length - 1

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
  startedWithHiddenChrome: boolean
}

type FeedPagerProgrammaticNavigationResult = {
  animated: boolean
}

type FeedPagerDragStartResult = {
  started: boolean
  blocked: boolean
}

type FeedPagerDragUpdateOptions = {
  resetBlockedDirection?: boolean
}

type FeedPagerDragUpdateResult = {
  blocked: boolean
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function blockedDragOffset(deltaX: number, windowWidth: number) {
  if (!Number.isFinite(deltaX) || deltaX === 0) {
    return 0
  }

  const distance = Math.abs(deltaX)
  const maxOffset = Math.min(Math.max(windowWidth * 0.1, 22), 36)
  const offset = Math.min(maxOffset, Math.log1p(distance) * 8)
  return deltaX > 0 ? offset : -offset
}

export function useFeedPagerTransition(options: FeedPagerTransitionOptions) {
  const dragThreshold = options.dragThreshold ?? 8
  const dragOffset = ref(0)
  const settling = ref(false)
  const viewSwipeCandidateActive = ref(false)
  const viewSwipeActive = ref(false)
  const transitionBaseIndex = ref<number | null>(null)
  let settlingTimer = 0
  let delayedCommitTimer = 0
  let settlingTimerToken = 0
  let delayedCommitTimerToken = 0
  let startedWithHiddenChrome = false
  let activePointerId: number | null = null

  const activeIndex = computed(() => feedKeyIndex(options.getActiveKey()))
  const trackBaseIndex = computed(() => transitionBaseIndex.value ?? activeIndex.value)
  const activeSurface = computed<FeedSwipeSurface>(() => feedPagerPages[activeIndex.value].surface)

  const trackStyle = computed(() => ({
    transform: `translate3d(calc(${-trackBaseIndex.value * 100}% + ${cssPx(dragOffset.value)}), 0, 0)`,
    transition: settling.value ? 'transform var(--motion-normal) var(--ease-standard)' : undefined,
  }))

  const swipeProgress = computed(() =>
    clampProgress(Math.abs(dragOffset.value) / Math.max(1, Math.min(options.getWindowWidth(), 320))),
  )

  const targetKey = computed(() => {
    if (dragOffset.value < -dragThreshold && activeIndex.value < lastFeedPagerIndex) {
      return feedPagerPages[activeIndex.value + 1].key
    }
    if (dragOffset.value > dragThreshold && activeIndex.value > 0) {
      return feedPagerPages[activeIndex.value - 1].key
    }
    return ''
  })

  const targetVisible = computed(() => options.isFeedRoute() && !options.isDetailReaderOpen() && Boolean(targetKey.value))
  const targetProgress = computed(() =>
    targetVisible.value ? clampProgress((swipeProgress.value - 0.26) / 0.48) : 0,
  )

  function surfaceFromOffset(offset: number): FeedSwipeSurface | null {
    if (offset < -dragThreshold && activeIndex.value < lastFeedPagerIndex) {
      return feedPagerPages[activeIndex.value + 1].surface
    }
    if (offset > dragThreshold && activeIndex.value > 0) {
      return feedPagerPages[activeIndex.value - 1].surface
    }
    return null
  }

  function isBlockedDragDirection(deltaX: number) {
    return (
      (activeIndex.value === 0 && deltaX > 0) ||
      (activeIndex.value === lastFeedPagerIndex && deltaX < 0)
    )
  }

  function canStartDrag(deltaX: number) {
    return surfaceFromOffset(deltaX) !== null
  }

  function commitPath(deltaX: number, horizontal: boolean, switchDistance: number) {
    if (!horizontal) {
      return null
    }
    if (deltaX <= -switchDistance && activeIndex.value < lastFeedPagerIndex) {
      return feedPagerPages[activeIndex.value + 1].path
    }
    if (deltaX >= switchDistance && activeIndex.value > 0) {
      return feedPagerPages[activeIndex.value - 1].path
    }
    return null
  }

  function feedPathIndex(path: string) {
    const match = feedPagerPages.find((page) => page.path === path)
    return match ? feedPagerPages.indexOf(match) : null
  }

  function feedKeyIndex(key: string | symbol | null | undefined) {
    const text = typeof key === 'string' ? key : ''
    const index = feedPagerPages.findIndex((page) => page.key === text)
    if (index >= 0) {
      return index
    }
    return 0
  }

  function targetOffsetForPath(path: string) {
    const targetIndex = feedPathIndex(path)
    if (targetIndex === null || targetIndex === activeIndex.value) {
      return null
    }
    return (activeIndex.value - targetIndex) * options.getWindowWidth()
  }

  function resolveDragCommitPath(deltaX: number, horizontal: boolean, switchDistance: number) {
    if (!viewSwipeActive.value) {
      return null
    }
    return commitPath(deltaX, horizontal, switchDistance)
  }

  function directionFromOffset(offset: number) {
    if (Math.abs(offset) <= 0.5) {
      return null
    }
    return offset < 0 ? ('left' as const) : ('right' as const)
  }

  function swipeTransitionBeginPayload(offset: number) {
    const targetSurface = surfaceFromOffset(offset)
    return {
      from: activeSurface.value,
      to: targetSurface,
      direction: directionFromOffset(offset),
      progress: swipeProgress.value,
      isBlocked: targetSurface === null,
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
    const targetOffset = nextPath ? targetOffsetForPath(nextPath) : null
    const committed = targetOffset !== null
    const swipeStartedWithHiddenChrome = consumeStartedWithHiddenChrome()
    setSettling(true)
    clearDelayedCommitTimer()
    clearSettlingTimer()
    if (committed) {
      lockTransitionBaseIndex()
      setDragOffset(targetOffset)
    } else {
      unlockTransitionBaseIndex()
      setDragOffset(0)
    }
    return {
      committed,
      settlePayload: {
        progress: committed ? 1 : 0,
        isBlocked: false,
      },
      startedWithHiddenChrome: swipeStartedWithHiddenChrome,
    }
  }

  function settleFinishedSwipe(delay: number, commit?: () => Promise<unknown> | unknown) {
    if (commit) {
      scheduleDelayedRouteCommit(delay, commit)
      return
    }
    scheduleSettlingEnd(delay)
  }

  function beginProgrammaticNavigation() {
    clearTimers()
    unlockTransitionBaseIndex()
    setSettling(true)
    setDragOffset(0)
  }

  function beginProgrammaticFeedSwitch(path: string): FeedPagerProgrammaticNavigationResult {
    if (!options.isFeedRoute() || options.isDetailReaderOpen()) {
      return {
        animated: false,
      }
    }

    const targetOffset = targetOffsetForPath(path)
    if (targetOffset === null) {
      return {
        animated: false,
      }
    }

    clearTimers()
    lockTransitionBaseIndex()
    setSettling(true)
    setDragOffset(targetOffset)
    return {
      animated: true,
    }
  }

  function settleProgrammaticNavigation(delay: number) {
    clearDelayedCommitTimer()
    scheduleSettlingEnd(delay)
  }

  function commitProgrammaticNavigation(delay: number, commit: () => Promise<unknown> | unknown) {
    scheduleDelayedRouteCommit(delay, commit)
  }

  function beginDragCandidate() {
    clearTimers()
    setSettling(false)
  }

  function beginViewSwipeCandidate() {
    viewSwipeCandidateActive.value = true
    viewSwipeActive.value = false
  }

  function cancelViewSwipeCandidate() {
    viewSwipeCandidateActive.value = false
  }

  function beginViewSwipe() {
    viewSwipeActive.value = true
    viewSwipeCandidateActive.value = false
  }

  function tryBeginDrag(deltaX: number): FeedPagerDragStartResult {
    const blocked = isBlockedDragDirection(deltaX)
    if (blocked) {
      beginViewSwipe()
      setDragDelta(deltaX)
      return {
        started: true,
        blocked: true,
      }
    }

    if (!canStartDrag(deltaX)) {
      return {
        started: false,
        blocked: false,
      }
    }

    beginViewSwipe()
    return {
      started: true,
      blocked: false,
    }
  }

  function resetViewSwipeTracking() {
    viewSwipeCandidateActive.value = false
    viewSwipeActive.value = false
  }

  function beginPointerTracking(pointerId: number) {
    activePointerId = pointerId
    beginDragCandidate()
    beginViewSwipeCandidate()
  }

  function isActivePointer(pointerId: number) {
    return activePointerId === pointerId
  }

  function clearPointerTracking() {
    activePointerId = null
  }

  function cancelPointerCandidate() {
    clearPointerTracking()
    cancelViewSwipeCandidate()
  }

  function endPointerTracking() {
    resetViewSwipeTracking()
    clearStartedWithHiddenChrome()
    clearPointerTracking()
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

  function lockTransitionBaseIndex() {
    transitionBaseIndex.value = activeIndex.value
  }

  function unlockTransitionBaseIndex() {
    transitionBaseIndex.value = null
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
    if (typeof window !== 'undefined' && settlingTimer !== 0) {
      window.clearTimeout(settlingTimer)
    }
    settlingTimer = 0
    settlingTimerToken += 1
  }

  function clearDelayedCommitTimer() {
    if (typeof window !== 'undefined' && delayedCommitTimer !== 0) {
      window.clearTimeout(delayedCommitTimer)
    }
    delayedCommitTimer = 0
    delayedCommitTimerToken += 1
  }

  function scheduleSettlingEnd(delay: number) {
    clearSettlingTimer()
    const token = settlingTimerToken + 1
    settlingTimerToken = token
    settlingTimer = window.setTimeout(() => {
      if (token !== settlingTimerToken) {
        return
      }
      settlingTimer = 0
      setSettling(false)
    }, Math.max(0, delay))
  }

  function scheduleDelayedCommit(delay: number, commit: () => void) {
    clearDelayedCommitTimer()
    const token = delayedCommitTimerToken + 1
    delayedCommitTimerToken = token
    delayedCommitTimer = window.setTimeout(() => {
      if (token !== delayedCommitTimerToken) {
        return
      }
      delayedCommitTimer = 0
      commit()
    }, Math.max(0, delay))
  }

  function scheduleDelayedRouteCommit(delay: number, commit: () => Promise<unknown> | unknown) {
    scheduleDelayedCommit(delay, () => {
      let result: Promise<unknown> | unknown
      try {
        result = commit()
      } catch {
        setSettling(false)
        setDragOffset(0)
        unlockTransitionBaseIndex()
        return
      }

      Promise.resolve(result)
        .catch(() => undefined)
        .finally(() => {
          setSettling(false)
          setDragOffset(0)
          unlockTransitionBaseIndex()
        })
    })
  }

  function clearTimers() {
    clearSettlingTimer()
    clearDelayedCommitTimer()
  }

  function setDragDelta(deltaX: number) {
    if (isBlockedDragDirection(deltaX)) {
      setDragOffset(blockedDragOffset(deltaX, options.getWindowWidth()))
      return
    }

    const windowWidth = options.getWindowWidth()
    const minOffset = activeIndex.value < lastFeedPagerIndex ? -windowWidth : 0
    const maxOffset = activeIndex.value > 0 ? windowWidth : 0
    setDragOffset(Math.min(maxOffset, Math.max(minOffset, deltaX)))
  }

  function updateDragDelta(deltaX: number, updateOptions: FeedPagerDragUpdateOptions = {}): FeedPagerDragUpdateResult {
    const blocked = isBlockedDragDirection(deltaX)
    if (updateOptions.resetBlockedDirection && blocked) {
      resetDragOffset()
      return {
        blocked: true,
      }
    }

    setDragDelta(deltaX)
    return {
      blocked,
    }
  }

  function reset() {
    clearTimers()
    clearStartedWithHiddenChrome()
    clearPointerTracking()
    resetViewSwipeTracking()
    unlockTransitionBaseIndex()
    dragOffset.value = 0
    settling.value = false
  }

  return {
    dragThreshold,
    dragOffset: readonly(dragOffset),
    settling: readonly(settling),
    viewSwipeCandidateActive: readonly(viewSwipeCandidateActive),
    viewSwipeActive: readonly(viewSwipeActive),
    activeIndex,
    activeSurface,
    trackStyle,
    targetKey,
    targetVisible,
    targetProgress,
    resolveDragCommitPath,
    swipeTransitionBeginPayload,
    swipeTransitionUpdatePayload,
    finishSwipeResult,
    settleFinishedSwipe,
    beginProgrammaticNavigation,
    beginProgrammaticFeedSwitch,
    settleProgrammaticNavigation,
    commitProgrammaticNavigation,
    beginViewSwipeCandidate,
    cancelViewSwipeCandidate,
    tryBeginDrag,
    resetViewSwipeTracking,
    beginPointerTracking,
    isActivePointer,
    cancelPointerCandidate,
    endPointerTracking,
    resetDragOffset,
    markStartedWithHiddenChrome,
    clearStartedWithHiddenChrome,
    scheduleDelayedCommit,
    clearTimers,
    updateDragDelta,
    reset,
  }
}
